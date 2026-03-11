"""
Strategy State Management System

This module provides comprehensive state management for action-based strategies:
- Strategy state persistence and retrieval
- Checkpoint system for progress tracking
- State validation and consistency checks
- State history and audit trail
- Recovery mechanisms for interrupted strategies
"""

import logging
from datetime import datetime
from typing import Any, Dict, List, Optional, Set, Union
from dataclasses import dataclass, field
from enum import Enum
import json
import copy

logger = logging.getLogger(__name__)

# ============================================================================
# State Management Core
# ============================================================================

class StateValidationLevel(Enum):
    """Validation strictness levels"""
    NONE = "none"           # No validation
    BASIC = "basic"         # Basic type and structure checks
    STRICT = "strict"       # Full validation with business rules
    DEVELOPMENT = "development"  # Extra validation for development mode

@dataclass
class Checkpoint:
    """Represents a strategy execution checkpoint"""
    name: str
    timestamp: datetime
    state_snapshot: Dict[str, Any]
    action_name: Optional[str] = None
    action_result: Optional[Dict[str, Any]] = None
    metadata: Dict[str, Any] = field(default_factory=dict)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert checkpoint to dictionary"""
        return {
            "name": self.name,
            "timestamp": self.timestamp.isoformat(),
            "state_snapshot": self.state_snapshot,
            "action_name": self.action_name,
            "action_result": self.action_result,
            "metadata": self.metadata
        }
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'Checkpoint':
        """Create checkpoint from dictionary"""
        return cls(
            name=data["name"],
            timestamp=datetime.fromisoformat(data["timestamp"]),
            state_snapshot=data["state_snapshot"],
            action_name=data.get("action_name"),
            action_result=data.get("action_result"),
            metadata=data.get("metadata", {})
        )

@dataclass
class StateChange:
    """Represents a state change event"""
    timestamp: datetime
    field: str
    old_value: Any
    new_value: Any
    action_name: Optional[str] = None
    reason: Optional[str] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert state change to dictionary"""
        return {
            "timestamp": self.timestamp.isoformat(),
            "field": self.field,
            "old_value": self.old_value,
            "new_value": self.new_value,
            "action_name": self.action_name,
            "reason": self.reason
        }

class StrategyState:
    """
    Comprehensive strategy state management with validation and checkpoints
    """
    
    def __init__(
        self,
        strategy_id: str,
        validation_level: StateValidationLevel = StateValidationLevel.BASIC,
        max_checkpoints: int = 100,
        max_history: int = 1000
    ):
        self.strategy_id = strategy_id
        self.validation_level = validation_level
        self.max_checkpoints = max_checkpoints
        self.max_history = max_history
        
        # Core state data
        self._data: Dict[str, Any] = {}
        self._metadata: Dict[str, Any] = {
            "created_at": datetime.now().isoformat(),
            "last_updated": datetime.now().isoformat(),
            "version": 1
        }
        
        # Checkpoints and history
        self.checkpoints: List[Checkpoint] = []
        self.state_history: List[StateChange] = []
        
        # Validation
        self.validation_errors: List[str] = []
        self.validation_warnings: List[str] = []
        
        # State tracking
        self._locked_fields: Set[str] = set()
        self._required_fields: Set[str] = set()
        self._field_types: Dict[str, type] = {}
        
        # Current action context
        self.current_action: Optional[str] = None
        
        logger.info(f"Initialized strategy state for {strategy_id} with validation level {validation_level.value}")
    
    # ========================================================================
    # Core State Operations
    # ========================================================================
    
    def get(self, key: str, default: Any = None) -> Any:
        """Get state value with dot notation support"""
        try:
            keys = key.split('.')
            value = self._data
            
            for k in keys:
                if isinstance(value, dict) and k in value:
                    value = value[k]
                else:
                    return default
            
            return value
        except Exception as e:
            logger.warning(f"Error getting state key '{key}': {e}")
            return default
    
    def set(self, key: str, value: Any, reason: Optional[str] = None) -> bool:
        """Set state value with validation and history tracking"""
        try:
            # Check if field is locked
            if key in self._locked_fields:
                error_msg = f"Cannot modify locked field: {key}"
                self.validation_errors.append(error_msg)
                logger.error(error_msg)
                return False
            
            # Get old value for history
            old_value = self.get(key)
            
            # Validate new value
            if not self._validate_field(key, value):
                return False
            
            # Set the value using dot notation
            self._set_nested(key, value)
            
            # Record state change
            change = StateChange(
                timestamp=datetime.now(),
                field=key,
                old_value=old_value,
                new_value=value,
                action_name=self.current_action,
                reason=reason
            )
            self.state_history.append(change)
            
            # Trim history if needed
            if len(self.state_history) > self.max_history:
                self.state_history = self.state_history[-self.max_history:]
            
            # Update metadata
            self._metadata["last_updated"] = datetime.now().isoformat()
            self._metadata["version"] += 1
            
            logger.debug(f"State updated: {key} = {value}")
            return True
            
        except Exception as e:
            error_msg = f"Error setting state key '{key}': {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def _set_nested(self, key: str, value: Any):
        """Set nested dictionary value using dot notation"""
        keys = key.split('.')
        current = self._data
        
        # Navigate to the parent of the target key
        for k in keys[:-1]:
            if k not in current:
                current[k] = {}
            current = current[k]
        
        # Set the final value
        current[keys[-1]] = value
    
    def delete(self, key: str, reason: Optional[str] = None) -> bool:
        """Delete state key with validation"""
        try:
            if key in self._locked_fields:
                error_msg = f"Cannot delete locked field: {key}"
                self.validation_errors.append(error_msg)
                logger.error(error_msg)
                return False
            
            if key in self._required_fields:
                error_msg = f"Cannot delete required field: {key}"
                self.validation_errors.append(error_msg)
                logger.error(error_msg)
                return False
            
            old_value = self.get(key)
            if old_value is not None:
                # Record deletion
                change = StateChange(
                    timestamp=datetime.now(),
                    field=key,
                    old_value=old_value,
                    new_value=None,
                    action_name=self.current_action,
                    reason=reason or "deleted"
                )
                self.state_history.append(change)
                
                # Delete the key
                self._delete_nested(key)
                
                logger.debug(f"State key deleted: {key}")
                return True
            
            return False
            
        except Exception as e:
            error_msg = f"Error deleting state key '{key}': {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def _delete_nested(self, key: str):
        """Delete nested dictionary key using dot notation"""
        keys = key.split('.')
        current = self._data
        
        # Navigate to the parent
        for k in keys[:-1]:
            if k not in current:
                return  # Key doesn't exist
            current = current[k]
        
        # Delete the final key
        if keys[-1] in current:
            del current[keys[-1]]
    
    def update(self, data: Dict[str, Any], reason: Optional[str] = None) -> bool:
        """Update multiple state values"""
        success = True
        for key, value in data.items():
            if not self.set(key, value, reason):
                success = False
        return success
    
    def clear(self, reason: Optional[str] = None) -> bool:
        """Clear all state data (except locked fields)"""
        try:
            # Get all keys except locked ones
            keys_to_delete = []
            for key in self._get_all_keys(self._data):
                if key not in self._locked_fields:
                    keys_to_delete.append(key)
            
            # Delete each key
            for key in keys_to_delete:
                self.delete(key, reason or "state_cleared")
            
            logger.info(f"State cleared for strategy {self.strategy_id}")
            return True
            
        except Exception as e:
            error_msg = f"Error clearing state: {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def _get_all_keys(self, data: Dict[str, Any], prefix: str = "") -> List[str]:
        """Get all keys from nested dictionary"""
        keys = []
        for key, value in data.items():
            full_key = f"{prefix}.{key}" if prefix else key
            keys.append(full_key)
            
            if isinstance(value, dict):
                keys.extend(self._get_all_keys(value, full_key))
        
        return keys
    
    # ========================================================================
    # Checkpoint Management
    # ========================================================================
    
    def add_checkpoint(
        self,
        name: str,
        action_name: Optional[str] = None,
        action_result: Optional[Dict[str, Any]] = None,
        metadata: Optional[Dict[str, Any]] = None
    ) -> bool:
        """Add a checkpoint with current state snapshot"""
        try:
            checkpoint = Checkpoint(
                name=name,
                timestamp=datetime.now(),
                state_snapshot=copy.deepcopy(self._data),
                action_name=action_name or self.current_action,
                action_result=action_result,
                metadata=metadata or {}
            )
            
            self.checkpoints.append(checkpoint)
            
            # Trim checkpoints if needed
            if len(self.checkpoints) > self.max_checkpoints:
                self.checkpoints = self.checkpoints[-self.max_checkpoints:]
            
            logger.info(f"Checkpoint added: {name}")
            return True
            
        except Exception as e:
            error_msg = f"Error adding checkpoint '{name}': {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def get_checkpoint(self, name: str) -> Optional[Checkpoint]:
        """Get checkpoint by name (returns most recent if multiple)"""
        for checkpoint in reversed(self.checkpoints):
            if checkpoint.name == name:
                return checkpoint
        return None
    
    def restore_from_checkpoint(self, name: str) -> bool:
        """Restore state from checkpoint"""
        try:
            checkpoint = self.get_checkpoint(name)
            if not checkpoint:
                error_msg = f"Checkpoint not found: {name}"
                self.validation_errors.append(error_msg)
                logger.error(error_msg)
                return False
            
            # Backup current state
            self.add_checkpoint(f"pre_restore_{datetime.now().timestamp()}")
            
            # Restore state
            self._data = copy.deepcopy(checkpoint.state_snapshot)
            
            # Record restoration
            change = StateChange(
                timestamp=datetime.now(),
                field="__state__",
                old_value="current_state",
                new_value=f"restored_from_{name}",
                action_name=self.current_action,
                reason=f"restored from checkpoint {name}"
            )
            self.state_history.append(change)
            
            logger.info(f"State restored from checkpoint: {name}")
            return True
            
        except Exception as e:
            error_msg = f"Error restoring from checkpoint '{name}': {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def list_checkpoints(self) -> List[Dict[str, Any]]:
        """List all checkpoints with summary info"""
        return [
            {
                "name": cp.name,
                "timestamp": cp.timestamp.isoformat(),
                "action_name": cp.action_name,
                "metadata": cp.metadata
            }
            for cp in self.checkpoints
        ]
    
    # ========================================================================
    # Validation System
    # ========================================================================
    
    def set_field_type(self, field: str, field_type: type):
        """Set expected type for a field"""
        self._field_types[field] = field_type
    
    def set_required_field(self, field: str):
        """Mark field as required"""
        self._required_fields.add(field)
    
    def lock_field(self, field: str):
        """Lock field to prevent modifications"""
        self._locked_fields.add(field)
    
    def unlock_field(self, field: str):
        """Unlock field to allow modifications"""
        self._locked_fields.discard(field)
    
    def _validate_field(self, key: str, value: Any) -> bool:
        """Validate field value based on validation level"""
        if self.validation_level == StateValidationLevel.NONE:
            return True
        
        errors = []
        
        # Type validation
        if key in self._field_types:
            expected_type = self._field_types[key]
            if not isinstance(value, expected_type):
                errors.append(f"Field '{key}' expected type {expected_type.__name__}, got {type(value).__name__}")
        
        # Development mode extra validation
        if self.validation_level == StateValidationLevel.DEVELOPMENT:
            # Check for common issues
            if isinstance(value, str) and len(value) == 0:
                self.validation_warnings.append(f"Field '{key}' is empty string")
            
            if value is None and key in self._required_fields:
                errors.append(f"Required field '{key}' cannot be None")
        
        # Strict validation
        if self.validation_level == StateValidationLevel.STRICT:
            # Business rule validation would go here
            pass
        
        if errors:
            self.validation_errors.extend(errors)
            logger.error(f"Validation failed for field '{key}': {errors}")
            return False
        
        return True
    
    def validate_state(self) -> bool:
        """Validate entire state"""
        self.validation_errors.clear()
        self.validation_warnings.clear()
        
        if self.validation_level == StateValidationLevel.NONE:
            return True
        
        # Check required fields
        for field in self._required_fields:
            if self.get(field) is None:
                self.validation_errors.append(f"Required field missing: {field}")
        
        # Validate all field types
        for key, expected_type in self._field_types.items():
            value = self.get(key)
            if value is not None and not isinstance(value, expected_type):
                self.validation_errors.append(f"Field '{key}' type mismatch: expected {expected_type.__name__}, got {type(value).__name__}")
        
        is_valid = len(self.validation_errors) == 0
        
        if not is_valid:
            logger.warning(f"State validation failed: {self.validation_errors}")
        
        return is_valid
    
    def get_validation_report(self) -> Dict[str, Any]:
        """Get comprehensive validation report"""
        return {
            "is_valid": len(self.validation_errors) == 0,
            "errors": self.validation_errors.copy(),
            "warnings": self.validation_warnings.copy(),
            "validation_level": self.validation_level.value,
            "required_fields": list(self._required_fields),
            "locked_fields": list(self._locked_fields),
            "field_types": {k: v.__name__ for k, v in self._field_types.items()}
        }
    
    # ========================================================================
    # State Persistence and Recovery
    # ========================================================================
    
    def to_dict(self) -> Dict[str, Any]:
        """Export state to dictionary"""
        return {
            "strategy_id": self.strategy_id,
            "data": self._data,
            "metadata": self._metadata,
            "checkpoints": [cp.to_dict() for cp in self.checkpoints],
            "state_history": [change.to_dict() for change in self.state_history[-100:]],  # Last 100 changes
            "validation_config": {
                "validation_level": self.validation_level.value,
                "required_fields": list(self._required_fields),
                "locked_fields": list(self._locked_fields),
                "field_types": {k: v.__name__ for k, v in self._field_types.items()}
            }
        }
    
    def from_dict(self, data: Dict[str, Any]) -> bool:
        """Import state from dictionary"""
        try:
            self.strategy_id = data["strategy_id"]
            self._data = data["data"]
            self._metadata = data["metadata"]
            
            # Restore checkpoints
            self.checkpoints = [Checkpoint.from_dict(cp) for cp in data.get("checkpoints", [])]
            
            # Restore state history
            history_data = data.get("state_history", [])
            self.state_history = [
                StateChange(
                    timestamp=datetime.fromisoformat(change["timestamp"]),
                    field=change["field"],
                    old_value=change["old_value"],
                    new_value=change["new_value"],
                    action_name=change.get("action_name"),
                    reason=change.get("reason")
                )
                for change in history_data
            ]
            
            # Restore validation config
            validation_config = data.get("validation_config", {})
            self.validation_level = StateValidationLevel(validation_config.get("validation_level", "basic"))
            self._required_fields = set(validation_config.get("required_fields", []))
            self._locked_fields = set(validation_config.get("locked_fields", []))
            
            # Restore field types
            field_types = validation_config.get("field_types", {})
            self._field_types = {}
            for field, type_name in field_types.items():
                # Map type names back to types
                type_map = {
                    "str": str, "int": int, "float": float, "bool": bool,
                    "list": list, "dict": dict, "NoneType": type(None)
                }
                if type_name in type_map:
                    self._field_types[field] = type_map[type_name]
            
            logger.info(f"State imported for strategy {self.strategy_id}")
            return True
            
        except Exception as e:
            error_msg = f"Error importing state: {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    def to_json(self) -> str:
        """Export state to JSON string"""
        return json.dumps(self.to_dict(), indent=2, default=str)
    
    def from_json(self, json_str: str) -> bool:
        """Import state from JSON string"""
        try:
            data = json.loads(json_str)
            return self.from_dict(data)
        except Exception as e:
            error_msg = f"Error parsing JSON: {e}"
            self.validation_errors.append(error_msg)
            logger.error(error_msg)
            return False
    
    # ========================================================================
    # Context Management
    # ========================================================================
    
    def set_current_action(self, action_name: str):
        """Set current action context"""
        self.current_action = action_name
        logger.debug(f"Current action set to: {action_name}")
    
    def clear_current_action(self):
        """Clear current action context"""
        self.current_action = None
    
    # ========================================================================
    # Utility Methods
    # ========================================================================
    
    def get_summary(self) -> Dict[str, Any]:
        """Get state summary for monitoring"""
        return {
            "strategy_id": self.strategy_id,
            "data_keys": list(self._data.keys()),
            "checkpoint_count": len(self.checkpoints),
            "history_count": len(self.state_history),
            "validation_errors": len(self.validation_errors),
            "validation_warnings": len(self.validation_warnings),
            "last_updated": self._metadata.get("last_updated"),
            "version": self._metadata.get("version"),
            "current_action": self.current_action
        }
    
    def __str__(self):
        return f"StrategyState(id={self.strategy_id}, keys={len(self._data)}, checkpoints={len(self.checkpoints)})"
    
    def __repr__(self):
        return self.__str__()
