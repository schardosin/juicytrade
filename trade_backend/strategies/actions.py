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

@dataclass
class ActionContext:
    """Context passed to actions during execution"""
    strategy_state: Dict[str, Any]
    market_data: Dict[str, Any]
    current_time: datetime
    positions: Dict[str, Any]
    account_info: Dict[str, Any]
    debug_mode: bool = False

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
        
        # Not yet triggered
        return ActionResult(
            success=False,
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
        **kwargs
    ):
        super().__init__(name, **kwargs)
        self.condition = condition
        self.on_condition_met = on_condition_met
        self.continuous = continuous
        self.check_interval = check_interval
        self.condition_met = False
        self.check_count = 0
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Check condition and execute callback if met"""
        self.check_count += 1
        
        try:
            # Evaluate condition
            condition_result = self.condition(context)
            
            if context.debug_mode:
                self.log(f"Condition check #{self.check_count}: {condition_result}", "debug")
            
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
                            details=f"Condition met and callback executed"
                        )
                    except Exception as e:
                        return ActionResult(
                            success=False,
                            error=str(e),
                            details=f"Condition callback failed: {e}"
                        )
                else:
                    return ActionResult(
                        success=True,
                        data={"condition_met": True, "check_count": self.check_count},
                        details=f"Condition met (no callback)"
                    )
            
            elif condition_result and self.continuous:
                # Condition still met, continue monitoring
                return ActionResult(
                    success=True,
                    data={"condition_met": True, "continuous": True},
                    details=f"Condition continues to be met"
                )
            
            else:
                # Condition not met, keep waiting
                return ActionResult(
                    success=False,
                    details=f"Condition not met (check #{self.check_count})"
                )
                
        except Exception as e:
            return ActionResult(
                success=False,
                error=str(e),
                details=f"Condition evaluation failed: {e}"
            )

class TradeAction(Action):
    """Execute trading operations"""
    
    def __init__(
        self,
        name: str,
        trade_type: str,  # "BUY", "SELL", "BUY_TO_OPEN", etc.
        symbol: str,
        quantity: int,
        order_type: str = "MARKET",
        price: Optional[float] = None,
        stop_loss: Optional[float] = None,
        take_profit: Optional[float] = None,
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
    
    async def execute(self, context: ActionContext) -> ActionResult:
        """Execute the trade"""
        trade_details = {
            "type": self.trade_type,
            "symbol": self.symbol,
            "quantity": self.quantity,
            "order_type": self.order_type,
            "price": self.price,
            "stop_loss": self.stop_loss,
            "take_profit": self.take_profit
        }
        
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
                data={**trade_details, "order_id": f"ORDER_{datetime.now().timestamp()}"},
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
