"""
Strategy data store for database operations.
Follows the same patterns as ProviderCredentialStore.
"""

import json
import hashlib
import logging
import time
import uuid
from typing import Dict, List, Optional, Any
from datetime import datetime
from sqlalchemy.orm import Session
from sqlalchemy import and_, or_, desc, asc
from .database import strategy_db_manager
from .models import Strategy, StrategyExecution, StrategyTrade, StrategyPerformance

logger = logging.getLogger(__name__)

class StrategyStore:
    """
    Database storage for strategies.
    Follows the same patterns as ProviderCredentialStore.
    """
    
    def __init__(self):
        self.db_manager = strategy_db_manager
    
    def _generate_strategy_id(self, user_id: str, name: str) -> str:
        """Generate unique strategy ID (similar to ProviderCredentialStore.generate_instance_id)"""
        base_name = name.lower().replace(' ', '_').replace('(', '').replace(')', '')
        base_id = f"strategy_{user_id}_{base_name}"
        
        # Ensure uniqueness
        counter = 1
        strategy_id = base_id
        while not self.validate_strategy_id(strategy_id):
            strategy_id = f"{base_id}_{counter}"
            counter += 1
        
        return strategy_id
    
    def _calculate_file_hash(self, content: str) -> str:
        """Calculate SHA-256 hash of strategy content"""
        return hashlib.sha256(content.encode('utf-8')).hexdigest()
    
    def validate_strategy_id(self, strategy_id: str) -> bool:
        """Check if strategy ID is unique (similar to ProviderCredentialStore.validate_instance_id)"""
        try:
            with self.db_manager.get_session() as session:
                existing = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                return existing is None
        except Exception as e:
            logger.error(f"❌ Error validating strategy ID {strategy_id}: {e}")
            return False
    
    def get_all_strategies(self, user_id: str = "default_user") -> List[Dict[str, Any]]:
        """Get all strategies for user (similar to get_all_instances)"""
        try:
            with self.db_manager.get_session() as session:
                strategies = session.query(Strategy).filter(
                    Strategy.user_id == user_id,
                    Strategy.is_active == True
                ).order_by(desc(Strategy.created_at)).all()
                
                return [strategy.to_dict() for strategy in strategies]
        except Exception as e:
            logger.error(f"❌ Error getting strategies for user {user_id}: {e}")
            return []
    
    def get_active_strategies(self, user_id: str = "default_user") -> List[Dict[str, Any]]:
        """Get only strategies that have been used recently (similar to get_active_instances)"""
        try:
            with self.db_manager.get_session() as session:
                # Get strategies used in the last 30 days or with active executions
                thirty_days_ago = datetime.now().timestamp() - (30 * 24 * 60 * 60)
                
                strategies = session.query(Strategy).filter(
                    Strategy.user_id == user_id,
                    Strategy.is_active == True,
                    or_(
                        Strategy.last_used >= datetime.fromtimestamp(thirty_days_ago),
                        Strategy.success_count > 0
                    )
                ).order_by(desc(Strategy.last_used)).all()
                
                return [strategy.to_dict() for strategy in strategies]
        except Exception as e:
            logger.error(f"❌ Error getting active strategies for user {user_id}: {e}")
            return []
    
    def get_strategy(self, strategy_id: str) -> Optional[Dict[str, Any]]:
        """Get specific strategy (similar to get_instance)"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                return strategy.to_dict() if strategy else None
        except Exception as e:
            logger.error(f"❌ Error getting strategy {strategy_id}: {e}")
            return None
    
    def get_strategy_code(self, strategy_id: str) -> Optional[str]:
        """Get strategy Python code"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                return strategy.python_code if strategy else None
        except Exception as e:
            logger.error(f"❌ Error getting strategy code {strategy_id}: {e}")
            return None
    
    def add_strategy(self, user_id: str, name: str, description: str, 
                    python_code: str, filename: str, metadata: Dict[str, Any],
                    validation_details: Optional[Dict] = None) -> Dict[str, Any]:
        """Add new strategy (similar to add_instance)"""
        try:
            # Generate unique strategy ID
            strategy_id = self._generate_strategy_id(user_id, name)
            
            # Calculate file hash
            file_hash = self._calculate_file_hash(python_code)
            file_size = len(python_code.encode('utf-8'))
            
            with self.db_manager.get_session() as session:
                strategy = Strategy(
                    strategy_id=strategy_id,
                    user_id=user_id,
                    name=name,
                    description=description,
                    python_code=python_code,
                    filename=filename,
                    file_size=file_size,
                    file_hash=file_hash,
                    strategy_metadata=metadata,
                    validation_status="passed" if validation_details and validation_details.get("success") else "failed",
                    validation_details=validation_details,
                    author=metadata.get("author", "User"),
                    version=metadata.get("version", "1.0.0"),
                    risk_level=metadata.get("risk_level", "medium"),
                    max_positions=metadata.get("max_positions", 1),
                    preferred_symbols=metadata.get("preferred_symbols", []),
                    validation_count=1
                )
                
                session.add(strategy)
                session.commit()
                
                logger.info(f"➕ Added strategy: {strategy_id}")
                return {
                    "success": True,
                    "strategy_id": strategy_id,
                    "message": f"Strategy '{name}' added successfully"
                }
                
        except Exception as e:
            logger.error(f"❌ Error adding strategy: {e}")
            return {
                "success": False,
                "error": str(e),
                "message": "Failed to add strategy"
            }
    
    def update_strategy(self, strategy_id: str, **updates) -> bool:
        """Update existing strategy (similar to update_instance)"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                if not strategy:
                    logger.warning(f"⚠️ Strategy not found: {strategy_id}")
                    return False
                
                # Update fields
                for key, value in updates.items():
                    if hasattr(strategy, key):
                        setattr(strategy, key, value)
                
                # Update file hash if code changed
                if 'python_code' in updates:
                    strategy.file_hash = self._calculate_file_hash(updates['python_code'])
                    strategy.file_size = len(updates['python_code'].encode('utf-8'))
                
                strategy.updated_at = datetime.now()
                session.commit()
                
                logger.info(f"✏️ Updated strategy: {strategy_id}")
                return True
                
        except Exception as e:
            logger.error(f"❌ Error updating strategy {strategy_id}: {e}")
            return False
    
    def delete_strategy(self, strategy_id: str) -> bool:
        """Delete strategy (soft delete, similar to delete_instance)"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                if strategy:
                    # Soft delete
                    strategy.is_active = False
                    strategy.updated_at = datetime.now()
                    session.commit()
                    
                    logger.info(f"🗑️ Deleted strategy: {strategy_id}")
                    return True
                else:
                    logger.warning(f"⚠️ Strategy not found for deletion: {strategy_id}")
                    return False
                    
        except Exception as e:
            logger.error(f"❌ Error deleting strategy {strategy_id}: {e}")
            return False
    
    def update_strategy_usage(self, strategy_id: str) -> bool:
        """Update last used timestamp"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                if strategy:
                    strategy.last_used = datetime.now()
                    session.commit()
                    return True
                    
                return False
                
        except Exception as e:
            logger.error(f"❌ Error updating strategy usage {strategy_id}: {e}")
            return False
    
    def increment_success_count(self, strategy_id: str) -> bool:
        """Increment success count"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                if strategy:
                    strategy.success_count += 1
                    strategy.last_used = datetime.now()
                    session.commit()
                    return True
                    
                return False
                
        except Exception as e:
            logger.error(f"❌ Error incrementing success count {strategy_id}: {e}")
            return False
    
    def increment_error_count(self, strategy_id: str) -> bool:
        """Increment error count"""
        try:
            with self.db_manager.get_session() as session:
                strategy = session.query(Strategy).filter(
                    Strategy.strategy_id == strategy_id,
                    Strategy.is_active == True
                ).first()
                
                if strategy:
                    strategy.error_count += 1
                    strategy.last_used = datetime.now()
                    session.commit()
                    return True
                    
                return False
                
        except Exception as e:
            logger.error(f"❌ Error incrementing error count {strategy_id}: {e}")
            return False
    
    def get_strategies_by_user(self, user_id: str) -> List[Dict[str, Any]]:
        """Get all strategies for specific user"""
        return self.get_all_strategies(user_id)
    
    def search_strategies(self, user_id: str, query: str) -> List[Dict[str, Any]]:
        """Search strategies by name or description"""
        try:
            with self.db_manager.get_session() as session:
                search_term = f"%{query.lower()}%"
                
                strategies = session.query(Strategy).filter(
                    Strategy.user_id == user_id,
                    Strategy.is_active == True,
                    or_(
                        Strategy.name.ilike(search_term),
                        Strategy.description.ilike(search_term)
                    )
                ).order_by(desc(Strategy.created_at)).all()
                
                return [strategy.to_dict() for strategy in strategies]
                
        except Exception as e:
            logger.error(f"❌ Error searching strategies: {e}")
            return []
    
    def get_strategy_statistics(self, user_id: str = "default_user") -> Dict[str, Any]:
        """Get strategy statistics for user"""
        try:
            with self.db_manager.get_session() as session:
                total_strategies = session.query(Strategy).filter(
                    Strategy.user_id == user_id,
                    Strategy.is_active == True
                ).count()
                
                # Get strategies with executions
                strategies_with_executions = session.query(Strategy).join(StrategyExecution).filter(
                    Strategy.user_id == user_id,
                    Strategy.is_active == True
                ).distinct().count()
                
                # Get total executions
                total_executions = session.query(StrategyExecution).filter(
                    StrategyExecution.user_id == user_id
                ).count()
                
                # Get running executions
                running_executions = session.query(StrategyExecution).filter(
                    StrategyExecution.user_id == user_id,
                    StrategyExecution.status == "running"
                ).count()
                
                # Get total trades
                total_trades = session.query(StrategyTrade).join(StrategyExecution).filter(
                    StrategyExecution.user_id == user_id
                ).count()
                
                # Get total P&L
                total_pnl = session.query(StrategyExecution).filter(
                    StrategyExecution.user_id == user_id
                ).with_entities(StrategyExecution.total_pnl).all()
                
                total_pnl_sum = sum(pnl[0] or 0 for pnl in total_pnl)
                
                return {
                    "total_strategies": total_strategies,
                    "strategies_with_executions": strategies_with_executions,
                    "total_executions": total_executions,
                    "running_executions": running_executions,
                    "paused_executions": 0,  # TODO: Add paused status
                    "total_trades": total_trades,
                    "total_pnl": total_pnl_sum
                }
                
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

# Global instance
strategy_store = StrategyStore()
