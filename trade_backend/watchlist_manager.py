import json
import os
import uuid
from datetime import datetime
from typing import Dict, List, Optional, Any
from pathlib import Path
import logging
from .path_manager import path_manager

logger = logging.getLogger(__name__)

class WatchlistManager:
    """
    Watchlist Manager - Handles watchlist operations with JSON file storage
    
    Similar to provider_config.py pattern but for watchlist management.
    Provides CRUD operations for watchlists and symbol management.
    """
    
    def __init__(self, config_file: str = "watchlist.json"):
        """Initialize the watchlist manager with JSON file storage."""
        self.config_file = config_file
        self._watchlists = {}
        self._active_watchlist = "default"
        self._version = "1.0"
        self._load_config()
    
    def _load_config(self):
        """Load watchlist configuration from JSON file."""
        try:
            config_path = path_manager.get_config_file_path(self.config_file)
            if config_path.exists():
                with open(config_path, 'r') as f:
                    data = json.load(f)
                    self._watchlists = data.get('watchlists', {})
                    self._active_watchlist = data.get('active_watchlist', 'default')
                    self._version = data.get('version', '1.0')
                    logger.info(f"📋 Loaded {len(self._watchlists)} watchlists from {config_path}")
            else:
                # Create default configuration
                self._create_default_config()
                logger.info("📋 Created default watchlist configuration")
        except Exception as e:
            logger.error(f"❌ Error loading watchlist config: {e}")
            self._create_default_config()
    
    def _create_default_config(self):
        """Create default watchlist configuration."""
        now = datetime.utcnow().isoformat() + 'Z'
        
        self._watchlists = {
            "default": {
                "id": "default",
                "name": "My Watchlist",
                "symbols": ["SPY", "QQQ", "AAPL"],
                "created_at": now,
                "updated_at": now
            }
        }
        self._active_watchlist = "default"
        self._version = "1.0"
        self._save_config()
    
    def _save_config(self):
        """Save watchlist configuration to JSON file with atomic write."""
        try:
            # Prepare data structure
            data = {
                "watchlists": self._watchlists,
                "active_watchlist": self._active_watchlist,
                "version": self._version,
                "last_updated": datetime.utcnow().isoformat() + 'Z'
            }
            
            # Get config path
            config_path = path_manager.get_config_file_path(self.config_file)
            
            # Atomic write - write to temp file first, then rename
            temp_file = config_path.with_suffix('.tmp')
            with open(temp_file, 'w') as f:
                json.dump(data, f, indent=2)
            
            # Atomic rename
            temp_file.replace(config_path)
            logger.debug(f"💾 Saved watchlist config to {config_path}")
            
        except Exception as e:
            logger.error(f"❌ Error saving watchlist config: {e}")
            raise
    
    def get_all_watchlists(self) -> Dict[str, Any]:
        """Get all watchlists with metadata."""
        return {
            "watchlists": self._watchlists.copy(),
            "active_watchlist": self._active_watchlist,
            "total_watchlists": len(self._watchlists),
            "version": self._version
        }
    
    def get_watchlist(self, watchlist_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific watchlist by ID."""
        return self._watchlists.get(watchlist_id)
    
    def create_watchlist(self, name: str, symbols: Optional[List[str]] = None) -> str:
        """
        Create a new watchlist.
        
        Args:
            name: Display name for the watchlist
            symbols: Optional list of initial symbols
            
        Returns:
            str: The ID of the created watchlist
        """
        if not name or not name.strip():
            raise ValueError("Watchlist name cannot be empty")
        
        # Generate unique ID
        watchlist_id = self._generate_watchlist_id(name)
        
        # Check if ID already exists (shouldn't happen with UUID, but be safe)
        if watchlist_id in self._watchlists:
            raise ValueError(f"Watchlist with ID '{watchlist_id}' already exists")
        
        # Create watchlist
        now = datetime.utcnow().isoformat() + 'Z'
        self._watchlists[watchlist_id] = {
            "id": watchlist_id,
            "name": name.strip(),
            "symbols": symbols or [],
            "created_at": now,
            "updated_at": now
        }
        
        self._save_config()
        logger.info(f"✅ Created watchlist '{name}' with ID '{watchlist_id}'")
        return watchlist_id
    
    def update_watchlist(self, watchlist_id: str, name: Optional[str] = None, 
                        symbols: Optional[List[str]] = None) -> bool:
        """
        Update an existing watchlist.
        
        Args:
            watchlist_id: ID of the watchlist to update
            name: New name (optional)
            symbols: New symbol list (optional)
            
        Returns:
            bool: True if updated successfully
        """
        if watchlist_id not in self._watchlists:
            raise ValueError(f"Watchlist '{watchlist_id}' not found")
        
        watchlist = self._watchlists[watchlist_id]
        updated = False
        
        if name is not None and name.strip():
            watchlist["name"] = name.strip()
            updated = True
        
        if symbols is not None:
            # Validate and clean symbols
            clean_symbols = [s.strip().upper() for s in symbols if s.strip()]
            watchlist["symbols"] = clean_symbols
            updated = True
        
        if updated:
            watchlist["updated_at"] = datetime.utcnow().isoformat() + 'Z'
            self._save_config()
            logger.info(f"✅ Updated watchlist '{watchlist_id}'")
        
        return updated
    
    def delete_watchlist(self, watchlist_id: str) -> bool:
        """
        Delete a watchlist.
        
        Args:
            watchlist_id: ID of the watchlist to delete
            
        Returns:
            bool: True if deleted successfully
        """
        if watchlist_id not in self._watchlists:
            raise ValueError(f"Watchlist '{watchlist_id}' not found")
        
        # Don't allow deleting the last watchlist
        if len(self._watchlists) <= 1:
            raise ValueError("Cannot delete the last remaining watchlist")
        
        # If deleting the active watchlist, switch to another one
        if self._active_watchlist == watchlist_id:
            # Find another watchlist to make active
            remaining_ids = [wid for wid in self._watchlists.keys() if wid != watchlist_id]
            self._active_watchlist = remaining_ids[0] if remaining_ids else "default"
        
        # Delete the watchlist
        del self._watchlists[watchlist_id]
        self._save_config()
        
        logger.info(f"✅ Deleted watchlist '{watchlist_id}'")
        return True
    
    def add_symbol(self, watchlist_id: str, symbol: str) -> bool:
        """
        Add a symbol to a watchlist.
        
        Args:
            watchlist_id: ID of the watchlist
            symbol: Symbol to add
            
        Returns:
            bool: True if added successfully
        """
        if watchlist_id not in self._watchlists:
            raise ValueError(f"Watchlist '{watchlist_id}' not found")
        
        if not symbol or not symbol.strip():
            raise ValueError("Symbol cannot be empty")
        
        symbol = symbol.strip().upper()
        watchlist = self._watchlists[watchlist_id]
        
        # Check if symbol already exists
        if symbol in watchlist["symbols"]:
            raise ValueError(f"Symbol '{symbol}' already exists in watchlist")
        
        # Add symbol
        watchlist["symbols"].append(symbol)
        watchlist["updated_at"] = datetime.utcnow().isoformat() + 'Z'
        
        self._save_config()
        logger.info(f"✅ Added '{symbol}' to watchlist '{watchlist_id}'")
        return True
    
    def remove_symbol(self, watchlist_id: str, symbol: str) -> bool:
        """
        Remove a symbol from a watchlist.
        
        Args:
            watchlist_id: ID of the watchlist
            symbol: Symbol to remove
            
        Returns:
            bool: True if removed successfully
        """
        if watchlist_id not in self._watchlists:
            raise ValueError(f"Watchlist '{watchlist_id}' not found")
        
        if not symbol or not symbol.strip():
            raise ValueError("Symbol cannot be empty")
        
        symbol = symbol.strip().upper()
        watchlist = self._watchlists[watchlist_id]
        
        # Check if symbol exists
        if symbol not in watchlist["symbols"]:
            raise ValueError(f"Symbol '{symbol}' not found in watchlist")
        
        # Remove symbol
        watchlist["symbols"].remove(symbol)
        watchlist["updated_at"] = datetime.utcnow().isoformat() + 'Z'
        
        self._save_config()
        logger.info(f"✅ Removed '{symbol}' from watchlist '{watchlist_id}'")
        return True
    
    def set_active_watchlist(self, watchlist_id: str) -> bool:
        """
        Set the active watchlist.
        
        Args:
            watchlist_id: ID of the watchlist to make active
            
        Returns:
            bool: True if set successfully
        """
        if watchlist_id not in self._watchlists:
            raise ValueError(f"Watchlist '{watchlist_id}' not found")
        
        self._active_watchlist = watchlist_id
        self._save_config()
        
        logger.info(f"✅ Set active watchlist to '{watchlist_id}'")
        return True
    
    def get_active_watchlist(self) -> Optional[Dict[str, Any]]:
        """Get the currently active watchlist."""
        return self._watchlists.get(self._active_watchlist)
    
    def get_active_watchlist_id(self) -> str:
        """Get the ID of the currently active watchlist."""
        return self._active_watchlist
    
    def _generate_watchlist_id(self, name: str) -> str:
        """Generate a unique ID for a watchlist."""
        # Create a clean ID based on name, but ensure uniqueness with UUID
        clean_name = "".join(c.lower() for c in name if c.isalnum())[:20]
        unique_suffix = str(uuid.uuid4())[:8]
        return f"{clean_name}_{unique_suffix}"
    
    def validate_symbol(self, symbol: str) -> bool:
        """
        Basic symbol validation.
        
        Args:
            symbol: Symbol to validate
            
        Returns:
            bool: True if symbol appears valid
        """
        if not symbol or not symbol.strip():
            return False
        
        symbol = symbol.strip().upper()
        
        # Basic validation - alphanumeric, 1-10 characters
        if not symbol.isalnum() or len(symbol) < 1 or len(symbol) > 10:
            return False
        
        return True
    
    def get_all_symbols(self) -> List[str]:
        """Get all unique symbols across all watchlists."""
        all_symbols = set()
        for watchlist in self._watchlists.values():
            all_symbols.update(watchlist["symbols"])
        return sorted(list(all_symbols))
    
    def search_watchlists(self, query: str) -> List[Dict[str, Any]]:
        """
        Search watchlists by name or symbols.
        
        Args:
            query: Search query
            
        Returns:
            List of matching watchlists
        """
        if not query or not query.strip():
            return list(self._watchlists.values())
        
        query = query.strip().lower()
        matches = []
        
        for watchlist in self._watchlists.values():
            # Search in name
            if query in watchlist["name"].lower():
                matches.append(watchlist)
                continue
            
            # Search in symbols
            for symbol in watchlist["symbols"]:
                if query in symbol.lower():
                    matches.append(watchlist)
                    break
        
        return matches

# Create global instance
watchlist_manager = WatchlistManager()
