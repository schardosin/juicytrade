"""
Action-Based Strategy Framework

This module implements a comprehensive action system for trading strategies that allows:
- Time-based triggers (e.g., execute at 1:30 PM)
- Condition monitoring (e.g., wait for price to reach 0.15)
- Trade execution with dry-run support
- Complex multi-step strategies with state management
- Comprehensive logging and debugging
- Error handling with retry mechanisms
"""

import asyncio
import logging
from abc import ABC, abstractmethod
from datetime import datetime, time
from typing import Any, Dict, List, Optional, Callable, Union
from enum import Enum
from dataclasses import dataclass, field

logger = logging.getLogger(__name__)

# ============================================================================
# Core Action Framework
# ============================================================================

class ActionStatus(Enum):
    """Action execution status"""
    PENDING = "pending"
    EXECUTING = "executing"
    COMPLETED = "completed"
    FAILED = "failed"
    ABORTED = "aborted"
    SKIPPED = "skipped"

@dataclass
class ActionResult:
    """Result of action execution"""
    success: bool
    data: Optional[Dict[str, Any]] = None
    error: Optional[str] = None
    dry_run: bool = False
    aborted: bool = False
    retry_count: int = 0
    execution_time: Optional[float] = None
    details: Optional[str] = None
    # Enhanced decision tracking
    decision_data: Optional[Dict[str, Any]] = None
    rule_evaluations: Optional[List[Dict[str, Any]]] = None
    context_snapshot: Optional[Dict[str, Any]] = None

@dataclass
class ActionContext:
    """Context passed to actions during execution"""
    strategy_state: Dict[str, Any]
    market_data: Dict[str, Any]
    current_time: datetime
    positions: Dict[str, Any]
    account_info: Dict[str, Any]
    debug_mode: bool = False

    def get_snapshot(self) -> Dict[str, Any]:
        """Returns a serializable snapshot of the context."""
        return {
            "strategy_state": self.strategy_state,
            "market_data": self.market_data,
            "current_time": self.current_time.isoformat(),
            "positions": self.positions,
            "account_info": self.account_info,
        }

    def get_snapshot(self) -> Dict[str, Any]:
        """Returns a serializable snapshot of the context."""
        return {
            "strategy_state": self.strategy_state,
            "market_data": self.market_data,
            "current_time": self.current_time.isoformat(),
            "positions": self.positions,
            "account_info": self.account_info,
        }

class Rule:
    def __init__(self, name: str, condition: Callable[[ActionContext], bool]):
        self.name = name
        self.condition = condition

    def evaluate(self, context: ActionContext) -> "Decision":
        from .models import Decision
        try:
            result = self.condition(context)
            return Decision(
                rule_name=self.name,
                result=bool(result),
                context_snapshot=context.get_snapshot() if hasattr(context, 'get_snapshot') else {}
            )
        except Exception as e:
            logger.error(f"Error evaluating rule '{self.name}': {e}")
            return Decision(
                rule_name=self.name,
                result=False,
                context_snapshot=context.get_snapshot() if hasattr(context, 'get_snapshot') else {},
                reason=str(e)
            )

class Action(ABC):
    """Base class for all strategy actions"""
    
    def __init__(
        self,
        name: str,
        description: str = "",
        dry_run: bool = False,
        retry_count: int = 3,
        retry_delay: float = 1.0,
        timeout: Optional[float] = None,
        prerequisites: Optional[List[str]] = None
    ):
        self.name = name
        self.description = description
        self.dry_run = dry_run
        self.retry_count = retry_count
        self.retry_delay = retry_delay
        self.timeout = timeout
        self.prerequisites = prerequisites or []
        
        # Execution state
        self.status = ActionStatus.PENDING
        self.created_at = datetime.now()
        self.started_at: Optional[datetime] = None
        self.completed_at: Optional[datetime] = None
        self.result: Optional[ActionResult] = None
        self.execution_log: List[str] = []
        
        # Callbacks
        self.on_success: Optional[Callable] = None
        self.on_failure: Optional[Callable] = None
        self.on_retry: Optional[Callable] = None
    
    @abstractmethod
    async def execute(self, context: ActionContext) -> ActionResult:
        """Execute the action with given context"""
        pass
    
    def log(self, message: str, level: str = "info"):
        """Add message to execution log"""
        timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
        log_entry = f"[{timestamp}] {message}"
        self.execution_log.append(log_entry)
        
        # Also log to system logger
        getattr(logger, level)(f"Action '{self.name}': {message}")
    
    def validate_prerequisites(self, context: ActionContext) -> List[str]:
        """Validate that prerequisites are met"""
        errors = []
        for prereq in self.prerequisites:
            if prereq not in context.strategy_state:
                errors.append(f"Missing prerequisite: {prereq}")
        return errors
    
    def is_completed(self) -> bool:
        """Check if action is completed"""
        return self.status in [ActionStatus.COMPLETED, ActionStatus.FAILED, ActionStatus.ABORTED]
    
    def mark_completed(self):
        """Mark action as completed"""
        self.status = ActionStatus.COMPLETED
        self.completed_at = datetime.now()
    
    def mark_failed(self, error: str):
        """Mark action as failed with error message"""
        self.status = ActionStatus.FAILED
        self.completed_at = datetime.now()
        if self.result:
            self.result.error = error
        else:
            self.result = ActionResult(success=False, error=error)
    
    def __str__(self):
        return f"{self.__class__.__name__}('{self.name}', status={self.status.value})"

# ============================================================================
# Specific Action Types
# ============================================================================

class TimeAction(Action):
    """Execute action at specific time"""
    
    def __init__(
        self,
        name: str,
        trigger_time: Union[str, time, datetime],
        on_trigger: Optional[Callable] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.trigger_time = self._parse_time(trigger_time)
        self.on_trigger = on_trigger
        self.triggered = False
    
    def _parse_time(self, time_input: Union[str, time, datetime]) -> time:
        """Parse various time formats"""
        if isinstance(time_input, str):
            # Parse "13:30" format
            hour, minute = map(int, time_input.split(':'))
            return time(hour, minute)
        elif isinstance(time_input, datetime):
            return time_input.time()
        elif isinstance(time_input, time):
            return time_input
        else:
            raise ValueError(f"Invalid time format: {time_input}")
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Check if trigger time has been reached"""
        current_time = context.current_time.time()
        
        if not self.triggered and current_time >= self.trigger_time:
            self.log(f"Time trigger reached: {self.trigger_time}")
            self.triggered = True
            
            if self.on_trigger:
                try:
                    if asyncio.iscoroutinefunction(self.on_trigger):
                        await self.on_trigger(context)
                    else:
                        self.on_trigger(context)
                    
                    return ActionResult(
                        success=True,
                        data={"triggered_at": current_time.isoformat()},
                        details=f"Time trigger executed at {current_time}"
                    )
                except Exception as e:
                    return ActionResult(
                        success=False,
                        error=str(e),
                        details=f"Time trigger callback failed: {e}"
                    )
            else:
                return ActionResult(
                    success=True,
                    data={"triggered_at": current_time.isoformat()},
                    details=f"Time trigger reached at {current_time}"
                )
        
        # CRITICAL FIX: Return success=True when waiting so action stays active
        # This prevents the action from being marked as failed and removed from queue
        return ActionResult(
            success=True,  # Changed from False to True
            data={"waiting": True, "target_time": self.trigger_time.isoformat(), "current_time": current_time.isoformat()},
            details=f"Waiting for {self.trigger_time}, current: {current_time}"
        )

class MonitorAction(Action):
    """Monitor conditions and execute callback when met"""
    
    def __init__(
        self,
        name: str,
        condition: Callable[[ActionContext], bool],
        on_condition_met: Optional[Callable] = None,
        continuous: bool = False,
        check_interval: float = 1.0,
        rule_description: str = "",
        condition_parameters: Optional[Dict[str, Any]] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.condition = condition
        self.on_condition_met = on_condition_met
        self.continuous = continuous
        self.check_interval = check_interval
        self.condition_met = False
        self.check_count = 0
        # Enhanced debugging info
        self.rule_description = rule_description or f"Monitor condition for {name}"
        self.condition_parameters = condition_parameters or {}
    
    def _capture_context_snapshot(self, context: ActionContext) -> Dict[str, Any]:
        """Capture relevant context data for debugging"""
        snapshot = {
            "timestamp": context.current_time.isoformat(),
            "market_data": {},
            "strategy_state": {},
            "positions": context.positions.copy() if context.positions else {},
            "account_info": context.account_info.copy() if context.account_info else {}
        }
        
        # Safely capture market data
        if context.market_data:
            for key, value in context.market_data.items():
                try:
                    # Only capture serializable data
                    if isinstance(value, (str, int, float, bool, type(None))):
                        snapshot["market_data"][key] = value
                    elif isinstance(value, dict):
                        snapshot["market_data"][key] = {k: v for k, v in value.items() 
                                                      if isinstance(v, (str, int, float, bool, type(None)))}
                except Exception:
                    snapshot["market_data"][key] = str(value)
        
        # Safely capture strategy state
        if context.strategy_state:
            for key, value in context.strategy_state.items():
                try:
                    if isinstance(value, (str, int, float, bool, type(None))):
                        snapshot["strategy_state"][key] = value
                    else:
                        snapshot["strategy_state"][key] = str(value)
                except Exception:
                    snapshot["strategy_state"][key] = str(value)
        
        return snapshot
    
    def _evaluate_condition_with_details(self, context: ActionContext) -> Dict[str, Any]:
        """Evaluate condition and capture detailed information"""
        evaluation_data = {
            "rule_name": self.name,
            "rule_description": self.rule_description,
            "parameters": self.condition_parameters.copy(),
            "timestamp": context.current_time.isoformat(),
            "check_number": self.check_count,
            "result": False,
            "error": None,
            "context_values": {},
            "evaluation_details": {}
        }
        
        try:
            # Capture key context values that might be relevant
            if context.market_data:
                price = context.market_data.get('price') or context.market_data.get('close')
                if price:
                    evaluation_data["context_values"]["current_price"] = price
                
                # Capture other common market data
                for key in ['open', 'high', 'low', 'close', 'volume', 'symbol']:
                    if key in context.market_data:
                        evaluation_data["context_values"][key] = context.market_data[key]
            
            # Capture relevant strategy state
            if context.strategy_state:
                for key in ['target_premium', 'entry_premium', 'max_positions', 'current_vertical']:
                    if key in context.strategy_state:
                        evaluation_data["context_values"][key] = context.strategy_state[key]
            
            # Evaluate the actual condition
            condition_result = self.condition(context)
            evaluation_data["result"] = bool(condition_result)
            
            # Add evaluation details based on common patterns
            if hasattr(self.condition, '__name__'):
                evaluation_data["evaluation_details"]["function_name"] = self.condition.__name__
            
            # Try to capture more details if it's a lambda or has specific attributes
            if hasattr(self.condition, '__code__'):
                evaluation_data["evaluation_details"]["code_info"] = {
                    "filename": self.condition.__code__.co_filename,
                    "line_number": self.condition.__code__.co_firstlineno,
                    "variable_names": list(self.condition.__code__.co_varnames)
                }
            
        except Exception as e:
            evaluation_data["result"] = False
            evaluation_data["error"] = str(e)
            evaluation_data["evaluation_details"]["exception"] = str(e)
        
        return evaluation_data
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Check condition and execute callback if met"""
        self.check_count += 1
        
        # Capture detailed evaluation data
        evaluation_data = self._evaluate_condition_with_details(context)
        condition_result = evaluation_data["result"]
        
        # Capture context snapshot for debugging
        context_snapshot = self._capture_context_snapshot(context)
        
        if context.debug_mode:
            self.log(f"Condition check #{self.check_count}: {condition_result}", "debug")
        
        try:
            if condition_result and not self.condition_met:
                self.condition_met = True
                self.log(f"Condition met after {self.check_count} checks")
                
                if self.on_condition_met:
                    try:
                        if asyncio.iscoroutinefunction(self.on_condition_met):
                            result = await self.on_condition_met(context)
                        else:
                            result = self.on_condition_met(context)
                        
                        return ActionResult(
                            success=True,
                            data={"condition_met": True, "check_count": self.check_count, "callback_result": result},
                            details=f"Condition met and callback executed",
                            decision_data=evaluation_data,
                            rule_evaluations=[evaluation_data],
                            context_snapshot=context_snapshot
                        )
                    except Exception as e:
                        return ActionResult(
                            success=False,
                            error=str(e),
                            details=f"Condition callback failed: {e}",
                            decision_data=evaluation_data,
                            rule_evaluations=[evaluation_data],
                            context_snapshot=context_snapshot
                        )
                else:
                    return ActionResult(
                        success=True,
                        data={"condition_met": True, "check_count": self.check_count},
                        details=f"Condition met (no callback)",
                        decision_data=evaluation_data,
                        rule_evaluations=[evaluation_data],
                        context_snapshot=context_snapshot
                    )
            
            elif condition_result and self.continuous:
                # Condition still met, continue monitoring
                return ActionResult(
                    success=True,
                    data={"condition_met": True, "continuous": True},
                    details=f"Condition continues to be met",
                    decision_data=evaluation_data,
                    rule_evaluations=[evaluation_data],
                    context_snapshot=context_snapshot
                )
            
            else:
                # Condition not met, but this is normal for continuous monitoring
                # Return success=True to indicate the monitoring is working correctly
                return ActionResult(
                    success=True,
                    data={"condition_met": False, "check_count": self.check_count, "monitoring": True},
                    details=f"Condition not met, continuing to monitor (check #{self.check_count})",
                    decision_data=evaluation_data,
                    rule_evaluations=[evaluation_data],
                    context_snapshot=context_snapshot
                )
                
        except Exception as e:
            return ActionResult(
                success=False,
                error=str(e),
                details=f"Condition evaluation failed: {e}",
                decision_data=evaluation_data,
                rule_evaluations=[evaluation_data],
                context_snapshot=context_snapshot
            )

class TradeAction(Action):
    """Execute trading operations"""
    
    def __init__(
        self,
        name: str,
        trade_type: str,  # "BUY", "SELL", "BUY_TO_OPEN", etc.
        symbol: Optional[str] = None,
        quantity: Optional[int] = None,
        order_type: str = "MARKET",
        price: Optional[float] = None,
        stop_loss: Optional[float] = None,
        take_profit: Optional[float] = None,
        legs: Optional[List[Dict[str, Any]]] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.trade_type = trade_type
        self.symbol = symbol
        self.quantity = quantity
        self.order_type = order_type
        self.price = price
        self.stop_loss = stop_loss
        self.take_profit = take_profit
        self.legs = legs

    async def execute(self, context: ActionContext) -> ActionResult:
        """Execute the trade"""
        if self.legs:
            trade_details = {
                "type": self.trade_type,
                "order_type": self.order_type,
                "price": self.price,
                "legs": self.legs
            }
            trade_description = f"{self.trade_type} {len(self.legs)}-leg option"
        else:
            trade_details = {
                "type": self.trade_type,
                "symbol": self.symbol,
                "quantity": self.quantity,
                "order_type": self.order_type,
                "price": self.price,
                "stop_loss": self.stop_loss,
                "take_profit": self.take_profit
            }
            trade_description = f"{self.trade_type} {self.quantity} {self.symbol}"
        
        if self.dry_run:
            self.log(f"DRY RUN: Would execute trade {trade_details}")
            return ActionResult(
                success=True,
                dry_run=True,
                data=trade_details,
                details=f"Dry run trade: {self.trade_type} {self.quantity} {self.symbol}"
            )
        else:
            # TODO: Integrate with actual order executor
            self.log(f"Executing trade: {trade_details}")
            
            # For now, simulate trade execution
            return ActionResult(
                success=True,
                data={**trade_details, "order_id": f"ORDER_{datetime.now().timestamp()}", "executed_at": datetime.now().isoformat()},
                details=f"Trade executed: {self.trade_type} {self.quantity} {self.symbol}"
            )

class ConditionalAction(Action):
    """Execute different actions based on conditions"""
    
    def __init__(
        self,
        name: str,
        conditions: List[tuple],  # [(condition_func, action), ...]
        default_action: Optional[Action] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.conditions = conditions
        self.default_action = default_action
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Evaluate conditions and execute appropriate action"""
        for i, (condition, action) in enumerate(self.conditions):
            try:
                if condition(context):
                    self.log(f"Condition {i+1} met, executing action: {action.name}")
                    result = await action.execute(context)
                    return ActionResult(
                        success=result.success,
                        data={"condition_index": i, "action_result": result.data},
                        error=result.error,
                        details=f"Executed conditional action {i+1}: {action.name}"
                    )
            except Exception as e:
                self.log(f"Condition {i+1} evaluation failed: {e}", "error")
        
        # No conditions met, execute default action if available
        if self.default_action:
            self.log(f"No conditions met, executing default action: {self.default_action.name}")
            result = await self.default_action.execute(context)
            return ActionResult(
                success=result.success,
                data={"default_action": True, "action_result": result.data},
                error=result.error,
                details=f"Executed default action: {self.default_action.name}"
            )
        
        return ActionResult(
            success=False,
            details="No conditions met and no default action specified"
        )

class ExpirationAction(Action):
    """Handle expiration events"""
    
    def __init__(
        self,
        name: str,
        expiration_date: datetime,
        on_expiration: Optional[Callable] = None,
        pre_expiration_actions: Optional[List[Action]] = None,
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.expiration_date = expiration_date
        self.on_expiration = on_expiration
        self.pre_expiration_actions = pre_expiration_actions or []
        self.expired = False
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Check for expiration and handle accordingly"""
        current_time = context.current_time
        
        if current_time >= self.expiration_date and not self.expired:
            self.expired = True
            self.log(f"Expiration reached: {self.expiration_date}")
            
            if self.on_expiration:
                try:
                    if asyncio.iscoroutinefunction(self.on_expiration):
                        result = await self.on_expiration(context)
                    else:
                        result = self.on_expiration(context)
                    
                    return ActionResult(
                        success=True,
                        data={"expired": True, "expiration_result": result},
                        details=f"Expiration handled at {current_time}"
                    )
                except Exception as e:
                    return ActionResult(
                        success=False,
                        error=str(e),
                        details=f"Expiration handler failed: {e}"
                    )
            else:
                return ActionResult(
                    success=True,
                    data={"expired": True},
                    details=f"Expiration reached at {current_time}"
                )
        
        # Not yet expired
        time_remaining = self.expiration_date - current_time
        return ActionResult(
            success=False,
            details=f"Time remaining until expiration: {time_remaining}"
        )

# ============================================================================
# Action Queue and Executor
# ============================================================================

class ActionQueue:
    """Manages action execution queue"""
    
    def __init__(self):
        self.actions: List[Action] = []
        self.completed_actions: List[Action] = []
        self.failed_actions: List[Action] = []
    
    def add_action(self, action: Action):
        """Add action to queue"""
        self.actions.append(action)
        logger.info(f"Added action to queue: {action}")
    
    def get_next_action(self) -> Optional[Action]:
        """Get next pending action"""
        for action in self.actions:
            if action.status == ActionStatus.PENDING:
                return action
        return None
    
    def get_active_actions(self) -> List[Action]:
        """Get all actions that are currently executing or pending"""
        return [a for a in self.actions if a.status in [ActionStatus.PENDING, ActionStatus.EXECUTING]]
    
    def get_completed_actions(self) -> List[Action]:
        """Get all completed actions"""
        return [a for a in self.actions if a.status == ActionStatus.COMPLETED]
    
    def clear_completed(self):
        """Remove completed actions from queue"""
        self.actions = [a for a in self.actions if a.status != ActionStatus.COMPLETED]

class ActionExecutor:
    """Executes actions with retry logic and error handling"""
    
    def __init__(self, debug_mode: bool = False):
        self.debug_mode = debug_mode
        self.execution_stats = {
            "total_executed": 0,
            "successful": 0,
            "failed": 0,
            "retried": 0,
            "aborted": 0
        }
    
    async def execute_action(self, action: Action, context: ActionContext) -> ActionResult:
        """Execute single action with retry logic"""
        action.status = ActionStatus.EXECUTING
        action.started_at = datetime.now()
        
        for attempt in range(action.retry_count + 1):
            try:
                # Validate prerequisites
                prereq_errors = action.validate_prerequisites(context)
                if prereq_errors:
                    return ActionResult(
                        success=False,
                        error=f"Prerequisites not met: {', '.join(prereq_errors)}",
                        details="Action prerequisites validation failed"
                    )
                
                # Execute action with timeout
                if action.timeout:
                    result = await asyncio.wait_for(
                        action.execute(context),
                        timeout=action.timeout
                    )
                else:
                    result = await action.execute(context)
                
                # Update execution stats
                self.execution_stats["total_executed"] += 1
                
                if result.success:
                    # For continuous monitoring actions, don't mark as completed
                    # unless they explicitly indicate completion
                    if isinstance(action, MonitorAction) and action.continuous and not result.data.get("condition_met", False):
                        # Keep monitoring action active for continuous monitoring
                        action.status = ActionStatus.EXECUTING
                        self.execution_stats["successful"] += 1
                        return result
                    # CRITICAL FIX: For TimeAction, don't mark as completed if just waiting
                    elif isinstance(action, TimeAction) and result.data and result.data.get("waiting", False):
                        # Keep TimeAction active while waiting for trigger time
                        action.status = ActionStatus.EXECUTING
                        self.execution_stats["successful"] += 1
                        return result
                    else:
                        # Normal completion for non-continuous actions or when condition is met
                        action.status = ActionStatus.COMPLETED
                        action.completed_at = datetime.now()
                        action.result = result
                        self.execution_stats["successful"] += 1
                        
                        # Execute success callback
                        if action.on_success:
                            try:
                                if asyncio.iscoroutinefunction(action.on_success):
                                    await action.on_success(result)
                                else:
                                    action.on_success(result)
                            except Exception as e:
                                action.log(f"Success callback failed: {e}", "error")
                        
                        return result
                else:
                    # Action returned failure, but no exception
                    if attempt < action.retry_count:
                        action.log(f"Action failed, retrying in {action.retry_delay}s (attempt {attempt + 1}/{action.retry_count})")
                        self.execution_stats["retried"] += 1
                        
                        if action.on_retry:
                            try:
                                if asyncio.iscoroutinefunction(action.on_retry):
                                    await action.on_retry(attempt + 1, result.error)
                                else:
                                    action.on_retry(attempt + 1, result.error)
                            except Exception as e:
                                action.log(f"Retry callback failed: {e}", "error")
                        
                        await asyncio.sleep(action.retry_delay)
                    else:
                        # Final failure
                        action.status = ActionStatus.FAILED
                        action.completed_at = datetime.now()
                        action.result = result
                        self.execution_stats["failed"] += 1
                        
                        if action.on_failure:
                            try:
                                if asyncio.iscoroutinefunction(action.on_failure):
                                    await action.on_failure(result)
                                else:
                                    action.on_failure(result)
                            except Exception as e:
                                action.log(f"Failure callback failed: {e}", "error")
                        
                        return result
                        
            except asyncio.TimeoutError:
                error_msg = f"Action timed out after {action.timeout}s"
                action.log(error_msg, "error")
                
                if attempt < action.retry_count:
                    action.log(f"Timeout, retrying in {action.retry_delay}s (attempt {attempt + 1}/{action.retry_count})")
                    await asyncio.sleep(action.retry_delay)
                else:
                    action.status = ActionStatus.ABORTED
                    action.completed_at = datetime.now()
                    self.execution_stats["aborted"] += 1
                    
                    return ActionResult(
                        success=False,
                        error=error_msg,
                        aborted=True,
                        details="Action aborted due to timeout"
                    )
                    
            except Exception as e:
                error_msg = str(e)
                action.log(f"Action execution error: {error_msg}", "error")
                
                if attempt < action.retry_count:
                    action.log(f"Exception occurred, retrying in {action.retry_delay}s (attempt {attempt + 1}/{action.retry_count})")
                    self.execution_stats["retried"] += 1
                    await asyncio.sleep(action.retry_delay)
                else:
                    action.status = ActionStatus.FAILED
                    action.completed_at = datetime.now()
                    action.result = ActionResult(
                        success=False,
                        error=error_msg,
                        retry_count=attempt + 1,
                        details="Action failed after all retry attempts"
                    )
                    self.execution_stats["failed"] += 1
                    
                    return action.result
        
        # Should never reach here
        return ActionResult(success=False, error="Unexpected execution path")
    
    def get_execution_stats(self) -> Dict[str, Any]:
        """Get execution statistics"""
        return self.execution_stats.copy()
