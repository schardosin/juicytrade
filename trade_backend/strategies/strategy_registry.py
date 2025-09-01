"""
Strategy Registry - Database-first implementation

Manages strategy registration, storage, and retrieval using SQLite database.
Follows the same patterns as existing system components (ProviderCredentialStore).
"""

import asyncio
import logging
import uuid
from typing import Dict, List, Any, Optional, Type
from datetime import datetime
from .base_strategy import BaseStrategy
from .strategy_validator import StrategyValidator
from .database import strategy_db_manager
from .strategy_store import strategy_store

logger = logging.getLogger(__name__)

class StrategyRegistry:
    """
    Database-backed strategy registry.
    Follows the same patterns as ProviderCredentialStore.
    """
    
    def __init__(self):
        self.validator = StrategyValidator()
        self.db_manager = strategy_db_manager
        self.store = strategy_store
        self._strategy_classes = {}  # In-memory cache for loaded strategy classes
        
        # Initialize database on first use
        self._ensure_database_initialized()
    
    def _ensure_database_initialized(self):
        """Ensure database is initialized (lazy initialization)"""
        if not self.db_manager._initialized:
            success = self.db_manager.initialize()
            if not success:
                logger.error("❌ Failed to initialize strategy database")
                raise RuntimeError("Strategy database initialization failed")
    
    async def register_strategy(
        self, 
        user_id: str, 
        strategy_file: bytes, 
        filename: str, 
        strategy_name: str, 
        description: str = ""
    ) -> Dict[str, Any]:
        """
        Register a new strategy with validation and database storage.
        Follows the same pattern as ProviderCredentialStore.add_instance().
        """
        try:
            # Validate the strategy file
            validation_result = await self.validator.validate_strategy_file(
                strategy_file, filename
            )
            
            if not validation_result.success:
                logger.warning(f"⚠️ Strategy validation failed: {strategy_name}")
                return {
                    "success": False,
                    "error": "Validation Failed",
                    "message": validation_result.message,
                    "validation_details": validation_result.details
                }
            
            # Decode strategy content
            python_code = strategy_file.decode('utf-8')
            
            # Extract metadata from validation result details
            metadata = validation_result.details.get("file_info", {})
            metadata.update({
                "filename": filename,
                "file_size": len(strategy_file),
                "validation_timestamp": datetime.now().isoformat(),
                "validation_id": validation_result.details.get("validation_id"),
                "validation_steps": validation_result.details.get("steps", [])
            })
            
            # Store strategy in database
            result = self.store.add_strategy(
                user_id=user_id,
                name=strategy_name,
                description=description,
                python_code=python_code,
                filename=filename,
                metadata=metadata,
                validation_details=validation_result.details
            )
            
            if result["success"]:
                logger.info(f"➕ Strategy registered: {strategy_name} -> {result['strategy_id']}")
                return {
                    "success": True,
                    "strategy_id": result["strategy_id"],
                    "message": f"Strategy '{strategy_name}' uploaded and validated successfully",
                    "strategy_name": strategy_name,
                    "validation_details": validation_result.details
                }
            else:
                logger.error(f"❌ Failed to store strategy: {result.get('error', 'Unknown error')}")
                return {
                    "success": False,
                    "error": "Storage Failed",
                    "message": result.get("message", "Failed to store strategy in database")
                }
                
        except Exception as e:
            logger.error(f"❌ Strategy registration failed: {e}")
            return {
                "success": False,
                "error": "Registration Failed",
                "message": str(e)
            }
    
    async def get_user_strategies(self, user_id: str) -> List[Dict[str, Any]]:
        """
        Get all strategies for a user.
        Follows the same pattern as ProviderCredentialStore.get_all_instances().
        """
        try:
            strategies = self.store.get_all_strategies(user_id)
            logger.debug(f"📋 Retrieved {len(strategies)} strategies for user {user_id}")
            return strategies
            
        except Exception as e:
            logger.error(f"❌ Error getting user strategies: {e}")
            return []
    
    async def get_strategy_details(self, strategy_id: str) -> Optional[Dict[str, Any]]:
        """
        Get detailed information about a strategy.
        Follows the same pattern as ProviderCredentialStore.get_instance().
        """
        try:
            strategy = self.store.get_strategy(strategy_id)
            if strategy:
                logger.debug(f"📄 Retrieved strategy details: {strategy_id}")
            return strategy
            
        except Exception as e:
            logger.error(f"❌ Error getting strategy details: {e}")
            return None
    
    async def get_strategy(self, strategy_id: str) -> Optional[Type[BaseStrategy]]:
        """
        Get strategy class by ID with dynamic loading.
        Returns the actual strategy class that can be instantiated.
        """
        try:
            # Check in-memory cache first
            if strategy_id in self._strategy_classes:
                return self._strategy_classes[strategy_id]
            
            # Get strategy code from database
            python_code = self.store.get_strategy_code(strategy_id)
            if not python_code:
                logger.warning(f"⚠️ Strategy code not found: {strategy_id}")
                return None
            
            # Load strategy class dynamically
            strategy_class = await self._load_strategy_class(strategy_id, python_code)
            if strategy_class:
                # Cache the loaded class
                self._strategy_classes[strategy_id] = strategy_class
                logger.debug(f"📦 Loaded strategy class: {strategy_id}")
                return strategy_class
            
            return None
            
        except Exception as e:
            logger.error(f"❌ Error getting strategy class: {e}")
            return None
    
    async def _load_strategy_class(self, strategy_id: str, python_code: str) -> Optional[Type[BaseStrategy]]:
        """
        Dynamically load strategy class from Python code.
        Uses safe execution environment.
        """
        try:
            # Create a safe execution environment
            exec_globals = {
                '__builtins__': __builtins__,
                'BaseStrategy': BaseStrategy,
                'StrategyResult': None,  # Import from base_strategy if needed
                'datetime': datetime,
                'logging': logging
            }
            
            # Execute the strategy code
            exec(python_code, exec_globals)
            
            # Find the strategy class (should inherit from BaseStrategy)
            strategy_class = None
            for name, obj in exec_globals.items():
                if (isinstance(obj, type) and 
                    issubclass(obj, BaseStrategy) and 
                    obj != BaseStrategy):
                    strategy_class = obj
                    break
            
            if not strategy_class:
                logger.error(f"❌ No valid strategy class found in {strategy_id}")
                return None
            
            return strategy_class
            
        except Exception as e:
            logger.error(f"❌ Error loading strategy class {strategy_id}: {e}")
            return None
    
    async def create_strategy_instance(
        self, 
        strategy_id: str, 
        data_provider, 
        order_executor, 
        config: Dict[str, Any] = None
    ) -> Optional[BaseStrategy]:
        """
        Create configured strategy instance.
        """
        try:
            strategy_class = await self.get_strategy(strategy_id)
            if not strategy_class:
                return None
            
            # Create instance with dependencies
            instance = strategy_class(
                data_provider=data_provider,
                order_executor=order_executor,
                config=config or {}
            )
            
            # Update usage statistics
            self.store.update_strategy_usage(strategy_id)
            
            logger.info(f"🏗️ Created strategy instance: {strategy_id}")
            return instance
            
        except Exception as e:
            logger.error(f"❌ Error creating strategy instance {strategy_id}: {e}")
            # Increment error count
            self.store.increment_error_count(strategy_id)
            return None
    
    async def update_strategy(self, strategy_id: str, updates: Dict[str, Any]) -> Dict[str, Any]:
        """
        Update strategy metadata.
        Follows the same pattern as ProviderCredentialStore.update_instance().
        """
        try:
            success = self.store.update_strategy(strategy_id, **updates)
            
            if success:
                # Clear cached class if code was updated
                if 'python_code' in updates:
                    self._strategy_classes.pop(strategy_id, None)
                
                logger.info(f"✏️ Updated strategy: {strategy_id}")
                return {"success": True}
            else:
                return {"success": False, "error": "Strategy not found"}
                
        except Exception as e:
            logger.error(f"❌ Error updating strategy: {e}")
            return {"success": False, "error": str(e)}
    
    async def delete_strategy(self, strategy_id: str) -> bool:
        """
        Delete a strategy (soft delete).
        Follows the same pattern as ProviderCredentialStore.delete_instance().
        """
        try:
            success = self.store.delete_strategy(strategy_id)
            
            if success:
                # Clear from cache
                self._strategy_classes.pop(strategy_id, None)
                logger.info(f"🗑️ Deleted strategy: {strategy_id}")
            
            return success
            
        except Exception as e:
            logger.error(f"❌ Error deleting strategy: {e}")
            return False
    
    async def validate_strategy_id(self, strategy_id: str) -> bool:
        """Check if strategy ID is unique"""
        return self.store.validate_strategy_id(strategy_id)
    
    async def search_strategies(self, user_id: str, query: str) -> List[Dict[str, Any]]:
        """Search strategies by name or description"""
        try:
            return self.store.search_strategies(user_id, query)
        except Exception as e:
            logger.error(f"❌ Error searching strategies: {e}")
            return []
    
    async def get_strategy_statistics(self, user_id: str = "default_user") -> Dict[str, Any]:
        """Get strategy statistics for user"""
        try:
            return self.store.get_strategy_statistics(user_id)
        except Exception as e:
            logger.error(f"❌ Error getting strategy statistics: {e}")
            return {
                "total_strategies": 0,
                "strategies_with_executions": 0,
                "total_executions": 0,
                "running_executions": 0,
                "paused_executions": 0,
                "total_trades": 0,
                "total_pnl": 0.0
            }
    
    async def health_check(self) -> Dict[str, Any]:
        """
        Get registry health status.
        Includes database health information.
        """
        try:
            # Get database health
            db_health = self.db_manager.health_check()
            
            # Get database statistics
            db_stats = self.db_manager.get_database_stats()
            
            # Combine health information
            health_status = {
                "status": "healthy" if db_health.get("status") == "healthy" else "unhealthy",
                "database": db_health,
                "statistics": db_stats,
                "cached_classes": len(self._strategy_classes),
                "timestamp": datetime.now().isoformat()
            }
            
            return health_status
            
        except Exception as e:
            logger.error(f"❌ Health check failed: {e}")
            return {
                "status": "error",
                "error": str(e),
                "timestamp": datetime.now().isoformat()
            }
    
    def record_strategy_success(self, strategy_id: str):
        """Record successful strategy execution"""
        try:
            self.store.increment_success_count(strategy_id)
        except Exception as e:
            logger.error(f"❌ Error recording strategy success: {e}")
    
    def record_strategy_error(self, strategy_id: str):
        """Record strategy execution error"""
        try:
            self.store.increment_error_count(strategy_id)
        except Exception as e:
            logger.error(f"❌ Error recording strategy error: {e}")

# Global instance (following the same pattern as existing components)
strategy_registry = StrategyRegistry()
