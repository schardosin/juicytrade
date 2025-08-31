"""
Strategy Registry

Manages strategy registration, storage, and retrieval.
This is a mock implementation for development - would need proper database integration for production.
"""

import asyncio
import logging
import uuid
from typing import Dict, List, Any, Optional
from datetime import datetime

logger = logging.getLogger(__name__)

class StrategyRegistry:
    """Mock strategy registry for development."""
    
    def __init__(self):
        # In-memory storage (would be database in production)
        self.strategies = {}
        self.user_strategies = {}
        
    async def register_strategy(
        self, 
        user_id: str, 
        strategy_file: bytes, 
        filename: str, 
        strategy_name: str, 
        description: str = ""
    ) -> Dict[str, Any]:
        """Register a new strategy."""
        try:
            strategy_id = str(uuid.uuid4())
            
            # Mock strategy metadata
            strategy_data = {
                "strategy_id": strategy_id,
                "user_id": user_id,
                "name": strategy_name,
                "description": description,
                "filename": filename,
                "file_size": len(strategy_file),
                "author": "User",
                "version": "1.0.0",
                "risk_level": "medium",
                "max_positions": 5,
                "preferred_symbols": ["SPY", "QQQ"],
                "created_at": datetime.now().isoformat(),
                "last_used": None,
                "validation_count": 1,
                "success_count": 0,
                "error_count": 0,
                "file_content": strategy_file  # Store for later use
            }
            
            # Store strategy
            self.strategies[strategy_id] = strategy_data
            
            # Add to user's strategies
            if user_id not in self.user_strategies:
                self.user_strategies[user_id] = []
            self.user_strategies[user_id].append(strategy_id)
            
            logger.info(f"Strategy registered: {strategy_name} -> {strategy_id}")
            
            return {
                "success": True,
                "strategy_id": strategy_id,
                "validation_details": {
                    "syntax_check": "passed",
                    "imports_check": "passed",
                    "base_class_check": "passed"
                }
            }
            
        except Exception as e:
            logger.error(f"Strategy registration failed: {e}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def get_user_strategies(self, user_id: str) -> List[Dict[str, Any]]:
        """Get all strategies for a user."""
        try:
            if user_id not in self.user_strategies:
                return []
            
            user_strategy_ids = self.user_strategies[user_id]
            strategies = []
            
            for strategy_id in user_strategy_ids:
                if strategy_id in self.strategies:
                    strategy_data = self.strategies[strategy_id].copy()
                    # Remove file content from response
                    strategy_data.pop('file_content', None)
                    strategies.append(strategy_data)
            
            return strategies
            
        except Exception as e:
            logger.error(f"Error getting user strategies: {e}")
            return []
    
    async def get_strategy_details(self, strategy_id: str) -> Optional[Dict[str, Any]]:
        """Get detailed information about a strategy."""
        try:
            if strategy_id not in self.strategies:
                return None
            
            strategy_data = self.strategies[strategy_id].copy()
            # Remove file content from response
            strategy_data.pop('file_content', None)
            return strategy_data
            
        except Exception as e:
            logger.error(f"Error getting strategy details: {e}")
            return None
    
    async def get_strategy(self, strategy_id: str) -> Optional[Any]:
        """Get strategy class instance (mock)."""
        try:
            if strategy_id not in self.strategies:
                return None
            
            # Return a mock strategy class
            class MockStrategy:
                def __init__(self, registry_strategies):
                    self.strategy_id = strategy_id
                    self.name = registry_strategies[strategy_id]["name"]
                
                async def initialize(self):
                    pass
                
                async def on_market_data(self, data):
                    pass
                
                async def cleanup(self):
                    pass
            
            return MockStrategy(self.strategies)
            
        except Exception as e:
            logger.error(f"Error getting strategy class: {e}")
            return None
    
    async def update_strategy(self, strategy_id: str, updates: Dict[str, Any]) -> Dict[str, Any]:
        """Update strategy metadata."""
        try:
            if strategy_id not in self.strategies:
                return {"success": False, "error": "Strategy not found"}
            
            # Update allowed fields
            allowed_fields = ["name", "description", "config"]
            for field, value in updates.items():
                if field in allowed_fields:
                    self.strategies[strategy_id][field] = value
            
            self.strategies[strategy_id]["updated_at"] = datetime.now().isoformat()
            
            return {"success": True}
            
        except Exception as e:
            logger.error(f"Error updating strategy: {e}")
            return {"success": False, "error": str(e)}
    
    async def delete_strategy(self, strategy_id: str) -> bool:
        """Delete a strategy."""
        try:
            if strategy_id not in self.strategies:
                return False
            
            # Remove from strategies
            strategy_data = self.strategies.pop(strategy_id)
            user_id = strategy_data["user_id"]
            
            # Remove from user's strategies
            if user_id in self.user_strategies:
                self.user_strategies[user_id] = [
                    sid for sid in self.user_strategies[user_id] 
                    if sid != strategy_id
                ]
            
            logger.info(f"Strategy deleted: {strategy_id}")
            return True
            
        except Exception as e:
            logger.error(f"Error deleting strategy: {e}")
            return False
    
    async def health_check(self) -> Dict[str, Any]:
        """Get registry health status."""
        try:
            total_strategies = len(self.strategies)
            total_users = len(self.user_strategies)
            
            return {
                "status": "healthy",
                "total_strategies": total_strategies,
                "total_users": total_users,
                "timestamp": datetime.now().isoformat()
            }
            
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            return {
                "status": "error",
                "error": str(e),
                "timestamp": datetime.now().isoformat()
            }
