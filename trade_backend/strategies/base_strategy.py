"""
Base Strategy Class - Action-Based Framework

This module provides the unified strategy framework that combines the power of action-based
trading logic with a familiar BaseStrategy interface. All trading strategies should inherit
from this class.

Features:
- Action-based multi-step trading logic
- Time-based triggers and scheduling
- State management with checkpoints
- Comprehensive error handling and retry logic
- Market hours awareness
- Dry-run and debugging support
"""

import asyncio
import logging
from abc import ABC, abstractmethod
from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List, Callable, Union

from .actions import (
    Action, ActionQueue, ActionExecutor, ActionContext, ActionResult,
    TimeAction, MonitorAction, TradeAction, ConditionalAction, ExpirationAction
)
from .strategy_state import StrategyState, StateValidationLevel
from .time_manager import TimeScheduler, MarketType, TradingSession
from .flow_engine import FlowEngine

logger = logging.getLogger(__name__)

# ============================================================================
# Base Strategy Class - Action-Based Framework
# ============================================================================

class BaseStrategy(ABC):
    """
    Base class for all trading strategies using the action-based framework.
    
    This class provides a powerful foundation for complex trading strategies that need:
    - Time-based triggers (e.g., execute at 1:30 PM)
    - Condition monitoring (e.g., wait for price to reach 0.15)
    - Multi-step logic with state management
    - Dynamic strategy switching
    - Comprehensive logging and debugging
    - Error handling with retry mechanisms
    
    Example Usage:
        class MyStrategy(BaseStrategy):
            async def initialize_strategy(self):
                # Wait for market open
                self.add_time_action("09:30", self.start_trading)
                
                # Set initial state
                self.set_state("max_positions", 5)
            
            async def start_trading(self, context):
                # Add monitoring action
                self.add_monitor_action(
                    "price_monitor",
                    condition=lambda ctx: self.check_price_condition(ctx),
                    callback=self.execute_trade
                )
    """
    
    def __init__(
        self,
        strategy_id: str,
        data_provider=None,
        order_executor=None,
        config: Dict[str, Any] = None,
        dry_run: bool = False,
        debug: bool = False,
        market_type: MarketType = MarketType.STOCK
    ):
        self.strategy_id = strategy_id
        self.data_provider = data_provider
        self.order_executor = order_executor
        self.config = config or {}
        self.dry_run = dry_run
        self.debug = debug
        
        # Action system
        self.action_queue = ActionQueue()
        self.action_executor = ActionExecutor(debug_mode=debug)
        
        # State management
        validation_level = StateValidationLevel.DEVELOPMENT if debug else StateValidationLevel.BASIC
        self.state = StrategyState(strategy_id, validation_level)
        
        # Time management
        self.time_scheduler = TimeScheduler(market_type)
        
        # Declarative Flow Engine
        self.flow = FlowEngine(self)
        self.data_update_processors = []
        
        # Symbol registration for additional data loading
        self.additional_symbols = set()
        
        # UI state registration for flow engine context display
        self.ui_states = set()
        
        # Execution state
        self.is_running = False
        self.is_paused = False
        self.start_time: Optional[datetime] = None
        self.last_update: Optional[datetime] = None
        
        # Performance tracking
        self.execution_stats = {
            "actions_executed": 0,
            "actions_successful": 0,
            "actions_failed": 0,
            "trades_executed": 0,
            "total_pnl": 0.0,
            "error_count": 0
        }
        
        # Logging
        self.logger = logging.getLogger(f"BaseStrategy.{strategy_id}")
        
        # ARCHITECTURAL FIX: Virtual date for strategy agnosticism
        self._virtual_date: Optional[str] = None  # Set by backtest engine
        
        logger.info(f"BaseStrategy initialized: {strategy_id} (dry_run={dry_run}, debug={debug})")
    
    # ========================================================================
    # Abstract Methods - Must be implemented by user strategies
    # ========================================================================
    
    @abstractmethod
    async def initialize_strategy(self):
        """
        Initialize the strategy and set up initial actions.
        
        This is where you define your strategy's action sequence:
        - Add time-based triggers
        - Set up monitoring conditions
        - Configure initial state
        - Subscribe to market data
        
        Example:
            # Wait for 1:30 PM
            self.add_time_action("13:30", self.start_monitoring)
            
            # Set up initial state
            self.set_state("max_positions", 5)
            self.set_state("risk_per_trade", 0.02)
        """
        pass
    
    @abstractmethod
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """
        Return strategy metadata for UI and monitoring.
        
        Returns:
            Dictionary with strategy information:
            {
                "name": "Strategy Name",
                "description": "Strategy description",
                "version": "1.0.0",
                "author": "Author Name",
                "risk_level": "MEDIUM",
                "max_positions": 5,
                "preferred_symbols": ["SPY", "QQQ"],
                "parameters": {...}
            }
        """
        pass
    
    # ========================================================================
    # Backward Compatibility Methods (for old BaseStrategy interface)
    # ========================================================================
    
    async def initialize(self):
        """
        Backward compatibility method - calls initialize_strategy()
        """
        await self.initialize_strategy()
    
    async def on_market_data(self, data: Dict[str, Any]):
        """
        Backward compatibility method for market data handling.
        Override this if you need direct market data processing.
        """
        pass
    
    async def on_trade_update(self, trade: Dict[str, Any]):
        """
        Backward compatibility method for trade updates.
        Override this if you need direct trade update processing.
        """
        pass
    
    # ========================================================================
    # Action Management
    # ========================================================================
    
    def add_action(self, action: Action):
        """Add action to execution queue"""
        # Set dry_run mode if strategy is in dry_run
        if self.dry_run and hasattr(action, 'dry_run'):
            action.dry_run = True
        
        self.action_queue.add_action(action)
        self.logger.info(f"Added action: {action}")
    
    def add_time_action(
        self,
        trigger_time: Union[str, datetime],
        callback: Callable,
        name: str = "",
        **kwargs
    ) -> TimeAction:
        """Add time-based action"""
        action = TimeAction(
            name=name or f"Time trigger {trigger_time}",
            trigger_time=trigger_time,
            on_trigger=callback,
            dry_run=self.dry_run,
            **kwargs
        )
        self.add_action(action)
        return action
    
    def add_monitor_action(
        self,
        name: str,
        condition: Callable[[ActionContext], bool],
        callback: Optional[Callable] = None,
        continuous: bool = False,
        **kwargs
    ) -> MonitorAction:
        """Add monitoring action"""
        action = MonitorAction(
            name=name,
            condition=condition,
            on_condition_met=callback,
            continuous=continuous,
            dry_run=self.dry_run,
            **kwargs
        )
        self.add_action(action)
        return action
    
    def add_trade_action(
        self,
        name: str,
        trade_type: str,
        symbol: str,
        quantity: int,
        **kwargs
    ) -> TradeAction:
        """Add trade execution action"""
        action = TradeAction(
            name=name,
            trade_type=trade_type,
            symbol=symbol,
            quantity=quantity,
            dry_run=self.dry_run,
            **kwargs
        )
        self.add_action(action)
        return action
    
    def add_conditional_action(
        self,
        name: str,
        conditions: List[tuple],
        default_action: Optional[Action] = None,
        **kwargs
    ) -> ConditionalAction:
        """Add conditional action"""
        action = ConditionalAction(
            name=name,
            conditions=conditions,
            default_action=default_action,
            dry_run=self.dry_run,
            **kwargs
        )
        self.add_action(action)
        return action
    
    def add_expiration_action(
        self,
        name: str,
        expiration_date: datetime,
        callback: Optional[Callable] = None,
        **kwargs
    ) -> ExpirationAction:
        """Add expiration handling action"""
        action = ExpirationAction(
            name=name,
            expiration_date=expiration_date,
            on_expiration=callback,
            dry_run=self.dry_run,
            **kwargs
        )
        self.add_action(action)
        return action
    
    # ========================================================================
    # Strategy Execution
    # ========================================================================
    
    async def start(self):
        """Start strategy execution"""
        if self.is_running:
            self.logger.warning("Strategy is already running")
            return
        
        try:
            self.logger.info("Starting strategy")
            
            # Initialize strategy
            await self.initialize_strategy()
            
            # Set execution state
            self.is_running = True
            self.is_paused = False
            self.start_time = datetime.now()
            self.last_update = datetime.now()
            
            # Add initial checkpoint
            self.state.add_checkpoint("strategy_started")
            
            self.logger.info("Strategy started successfully")
            
        except Exception as e:
            self.logger.error(f"Failed to start strategy: {e}")
            self.execution_stats["error_count"] += 1
            raise
    
    async def stop(self):
        """Stop strategy execution"""
        if not self.is_running:
            self.logger.warning("Strategy is not running")
            return
        
        try:
            self.logger.info("Stopping strategy")
            
            # Set execution state
            self.is_running = False
            self.is_paused = False
            
            # Add final checkpoint
            self.state.add_checkpoint("strategy_stopped")
            
            # Cleanup
            await self.cleanup()
            
            self.logger.info("Strategy stopped successfully")
            
        except Exception as e:
            self.logger.error(f"Error stopping strategy: {e}")
            self.execution_stats["error_count"] += 1
            raise
    
    async def pause(self):
        """Pause strategy execution"""
        if not self.is_running:
            self.logger.warning("Strategy is not running")
            return
        
        self.is_paused = True
        self.state.add_checkpoint("strategy_paused")
        self.logger.info("Strategy paused")
    
    async def resume(self):
        """Resume strategy execution"""
        if not self.is_running:
            self.logger.warning("Strategy is not running")
            return
        
        self.is_paused = False
        self.state.add_checkpoint("strategy_resumed")
        self.logger.info("Strategy resumed")
    
    async def execute_cycle(self):
        """Execute one cycle of strategy actions"""
        if not self.is_running or self.is_paused:
            return
        
        try:
            # Create action context
            # CRITICAL FIX: Use backtest engine's current_time if available (for proper timestamps)
            current_time = None
            if hasattr(self.data_provider, 'current_time') and self.data_provider.current_time:
                # Use backtest engine's current time for proper historical timestamps
                current_time = self.data_provider.current_time
            else:
                # Fallback to time scheduler for live trading
                current_time = self.time_scheduler.get_current_time()
            
            context = ActionContext(
                strategy_state=self.state._data,
                market_data=await self.get_market_data(),
                current_time=current_time,
                positions=await self.get_positions(),
                account_info=await self.get_account_info(),
                debug_mode=self.debug,
                virtual_date=self._virtual_date  # Pass virtual date to actions
            )
            
            # Check scheduled time events
            ready_events = self.time_scheduler.check_scheduled_events()
            for event in ready_events:
                try:
                    if asyncio.iscoroutinefunction(event["callback"]):
                        await event["callback"](context)
                    else:
                        event["callback"](context)
                except Exception as e:
                    self.logger.error(f"Error executing scheduled event {event['name']}: {e}")
            
            # CRITICAL FIX: Execute all active actions FIRST (especially TimeActions)
            # This ensures TimeActions set state flags before flow engine checks them
            active_actions = self.action_queue.get_active_actions()
            
            for action in active_actions:
                try:
                    # Set current action context
                    self.state.set_current_action(action.name)
                    
                    # Execute action
                    result = await self.action_executor.execute_action(action, context)
                    
                    # Update statistics
                    self.execution_stats["actions_executed"] += 1
                    if result.success:
                        self.execution_stats["actions_successful"] += 1
                    else:
                        self.execution_stats["actions_failed"] += 1
                    
                    # Handle trade actions
                    if isinstance(action, TradeAction) and result.success and not result.dry_run:
                        self.execution_stats["trades_executed"] += 1
                    
                    # Clear current action context
                    self.state.clear_current_action()
                    
                except Exception as e:
                    self.logger.error(f"Error executing action {action.name}: {e}")
                    self.execution_stats["error_count"] += 1
            
            # **FRAMEWORK ENHANCEMENT**: Record decision for this data point AFTER actions
            # This allows TimeActions to set state flags before flow engine checks them
            try:
                await self.record_cycle_decision(context)
            except Exception as e:
                self.logger.error(f"Error recording cycle decision: {e}")
            
            # Call legacy market data handler if overridden
            if hasattr(self, '_has_market_data_handler'):
                await self.on_market_data(context.market_data)
            
            # Update last execution time
            self.last_update = datetime.now()
            
        except Exception as e:
            self.logger.error(f"Error in execution cycle: {e}")
            self.execution_stats["error_count"] += 1
    
    async def record_cycle_decision(self, context: ActionContext):
        """
        Record a decision for this execution cycle.
        
        This method is called automatically during each execute_cycle() to allow
        strategies to record decisions at every data point for debugging and analysis.
        
        For pure declarative strategies, this method automatically executes the flow engine.
        For traditional strategies, override this method to implement custom decision recording.
        
        Args:
            context: ActionContext containing current market data, time, positions, etc.
        """
        # Check if this is a pure declarative strategy (has flow nodes but no custom record_cycle_decision)
        if (hasattr(self, 'flow') and 
            self.flow.get_node_count() > 0 and 
            not self._has_custom_record_cycle_decision()):
            
            # Pure declarative strategy - execute the flow engine
            await self.execute_declarative_flow(context)
        else:
            # Traditional strategy - default implementation does nothing
            # Strategies can override this method for custom decision recording
            pass
    
    def _has_custom_record_cycle_decision(self) -> bool:
        """Check if the strategy has overridden record_cycle_decision"""
        # Get the method from the strategy class
        strategy_method = getattr(self.__class__, 'record_cycle_decision', None)
        base_method = getattr(BaseStrategy, 'record_cycle_decision', None)
        
        # If the method is different from the base class method, it's been overridden
        return strategy_method is not base_method
    
    async def execute_declarative_flow(self, context: ActionContext):
        """
        Execute the declarative flow for pure declarative strategies.
        
        This method handles:
        1. Updating indicators (if the strategy has an update_indicators method)
        2. Executing the flow engine with automatic decision recording
        3. Error handling and logging
        """
        try:
            # Execute all registered data processors
            for processor in self.data_update_processors:
                try:
                    processor(context)
                except Exception as e:
                    self.logger.error(f"Error executing data processor '{processor.__name__}': {e}")

            # Execute the flow engine (which will automatically record decisions)
            await self.flow.execute(context)
            
        except Exception as e:
            self.logger.error(f"Error executing declarative flow: {e}")
            import traceback
            self.logger.error(f"Traceback: {traceback.format_exc()}")
            
            # Record the error in decision timeline
            self.add_decision_timeline({
                "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                "rule_description": "Flow Execution Error",
                "result": False,
                "context_values": {"error": str(e)},
                "evaluation_details": {"error_type": "flow_execution_error"},
                "next_action": "Error - Flow Stopped"
            })

    def register_data_processor(self, processor: Callable):
        """
        Register a data processor to be called on each execution cycle.
        
        Args:
            processor: A callable that accepts an ActionContext and returns a boolean.
        """
        if not callable(processor):
            raise TypeError("Processor must be a callable function or method.")
        self.data_update_processors.append(processor)
        self.logger.info(f"Data processor '{processor.__name__}' registered.")
            
    async def cleanup(self):
        """Cleanup resources when strategy stops"""
        # Override in subclasses for custom cleanup
        pass
    
    # ========================================================================
    # Market Data and Position Management
    # ========================================================================
    
    async def get_market_data(self) -> Dict[str, Any]:
        """Get current market data"""
        if self.data_provider:
            # This would be implemented based on your data provider interface
            return {}
        return {}
    
    async def get_positions(self) -> Dict[str, Any]:
        """Get current positions"""
        if self.order_executor and hasattr(self.order_executor, 'get_position_info'):
            return self.order_executor.get_position_info(strategy_id=self.strategy_id)
        return {}
    
    async def get_account_info(self) -> Dict[str, Any]:
        """Get account information"""
        if self.order_executor:
            # This would be implemented based on your order executor interface
            return {}
        return {}
    
    # ========================================================================
    # Helper Methods for Strategy Development
    # ========================================================================
    
    def schedule_at_market_open(self, callback: Callable, session: TradingSession = TradingSession.REGULAR):
        """Schedule callback at market open"""
        return self.time_scheduler.schedule_at_market_open(callback, session)
    
    def schedule_at_market_close(self, callback: Callable, session: TradingSession = TradingSession.REGULAR):
        """Schedule callback at market close"""
        return self.time_scheduler.schedule_at_market_close(callback, session)
    
    def schedule_at_time(self, target_time: Union[str, datetime], callback: Callable):
        """Schedule callback at specific time"""
        return self.time_scheduler.schedule_at_time(target_time, callback)
    
    def is_market_open(self) -> bool:
        """Check if market is currently open"""
        return self.time_scheduler.is_market_open()
    
    def is_regular_hours(self) -> bool:
        """Check if market is in regular trading hours"""
        return self.time_scheduler.is_regular_hours()
    
    def get_current_session(self) -> TradingSession:
        """Get current trading session"""
        return self.time_scheduler.get_current_session()
    
    # ========================================================================
    # Position Management Framework Integration
    # ========================================================================
    
    def has_open_positions(self, underlying: str = "") -> bool:
        """
        Check if strategy has any open positions.
        
        Args:
            underlying: Filter by underlying symbol (optional)
            
        Returns:
            True if any positions exist
        """
        if self.order_executor and hasattr(self.order_executor, 'has_open_positions'):
            return self.order_executor.has_open_positions(strategy_id=self.strategy_id, underlying=underlying)
        return False
    
    def get_position_for_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get position information for a specific symbol.
        
        Args:
            symbol: Symbol to get position for
            
        Returns:
            Position information or None if no position
        """
        if self.order_executor and hasattr(self.order_executor, 'get_position_info'):
            return self.order_executor.get_position_info(symbol=symbol, strategy_id=self.strategy_id)
        return None
    
    def get_all_positions(self, underlying: str = "") -> Dict[str, Any]:
        """
        Get all positions for this strategy.
        
        Args:
            underlying: Filter by underlying symbol (optional)
            
        Returns:
            Dictionary with 'positions' and 'combo_positions' keys
        """
        if self.order_executor and hasattr(self.order_executor, 'get_position_info'):
            return self.order_executor.get_position_info(strategy_id=self.strategy_id)
        return {"positions": {}, "combo_positions": {}}
    
    def get_position_count(self, underlying: str = "") -> int:
        """
        Get count of open positions.
        
        Args:
            underlying: Filter by underlying symbol (optional)
            
        Returns:
            Number of open positions
        """
        all_positions = self.get_all_positions(underlying)
        position_count = len(all_positions.get("positions", {}))
        combo_count = len(all_positions.get("combo_positions", {}))
        return position_count + combo_count
    
    def is_max_positions_reached(self, max_positions: int = None, underlying: str = "") -> bool:
        """
        Check if maximum position limit has been reached.
        
        Args:
            max_positions: Maximum positions allowed (if None, uses config)
            underlying: Filter by underlying symbol (optional)
            
        Returns:
            True if at max positions
        """
        if max_positions is None:
            max_positions = self.config.get("max_positions", 999)
        
        current_count = self.get_position_count(underlying)
        return current_count >= max_positions
    
    def can_open_new_position(self, underlying: str = "", required_margin: float = 0.0) -> bool:
        """
        Check if strategy can open a new position.
        
        Args:
            underlying: Filter by underlying symbol (optional)
            required_margin: Required margin for new position (optional)
            
        Returns:
            True if new position can be opened
        """
        # Check position limits
        if self.is_max_positions_reached(underlying=underlying):
            self.log_info(f"Cannot open new position: at max positions limit")
            return False
        
        # Add other checks here (margin, risk limits, etc.)
        return True
    
    def get_position_summary(self) -> Dict[str, Any]:
        """
        Get comprehensive position summary for this strategy.
        
        Returns:
            Dictionary with position statistics and details
        """
        all_positions = self.get_all_positions()
        positions = all_positions.get("positions", {})
        combo_positions = all_positions.get("combo_positions", {})
        
        # Calculate statistics
        total_positions = len(positions) + len(combo_positions)
        
        # Group by underlying
        by_underlying = {}
        for symbol, pos in positions.items():
            underlying = self._extract_underlying_from_symbol(symbol)
            if underlying not in by_underlying:
                by_underlying[underlying] = {"single_leg": 0, "multi_leg": 0}
            by_underlying[underlying]["single_leg"] += 1
        
        for symbol, pos in combo_positions.items():
            # Handle ComboPosition objects vs dictionaries
            if hasattr(pos, 'underlying_symbol'):
                underlying = pos.underlying_symbol
            elif isinstance(pos, dict):
                underlying = pos.get("underlying_symbol", "UNKNOWN")
            else:
                underlying = "UNKNOWN"
            
            if underlying not in by_underlying:
                by_underlying[underlying] = {"single_leg": 0, "multi_leg": 0}
            by_underlying[underlying]["multi_leg"] += 1
        
        return {
            "strategy_id": self.strategy_id,
            "total_positions": total_positions,
            "single_leg_positions": len(positions),
            "multi_leg_positions": len(combo_positions),
            "by_underlying": by_underlying,
            "max_positions": self.config.get("max_positions", "unlimited"),
            "can_open_new": self.can_open_new_position(),
            "position_details": {
                "single_leg": list(positions.keys()),
                "multi_leg": list(combo_positions.keys())
            }
        }
    
    def _extract_underlying_from_symbol(self, symbol: str) -> str:
        """Extract underlying symbol from position symbol"""
        # For options symbols, extract the underlying
        if len(symbol) > 6 and ('C' in symbol or 'P' in symbol):
            # OCC format: AAPL240119C00150000
            import re
            match = re.match(r'^([A-Z]+)', symbol)
            if match:
                return match.group(1)
        
        # For stock symbols or combo symbols
        if '_COMBO_' in symbol:
            return symbol.split('_COMBO_')[0]
        
        return symbol
    
    # ========================================================================
    # Symbol Registration for Data Loading
    # ========================================================================
    
    def register_additional_symbol(self, symbol: str):
        """
        Register an additional symbol for data loading during backtests.
        
        This allows strategies to load data for symbols beyond the primary
        underlying symbol. Useful for:
        - Options strategies needing different option symbol formats (SPX -> SPXW)
        - Multi-asset strategies requiring correlated symbols
        - Volatility analysis requiring VIX data
        
        Args:
            symbol: Additional symbol to load data for
            
        Example:
            # For SPX options strategy that needs SPXW options data
            if self.config.get("underlying") == "SPX":
                self.register_additional_symbol("SPXW")
        """
        self.additional_symbols.add(symbol)
        self.logger.info(f"Registered additional symbol for data loading: {symbol}")
    
    def get_additional_symbols(self) -> List[str]:
        """
        Get list of additional symbols registered by this strategy.
        
        Returns:
            List of additional symbol strings that should be loaded
            for this strategy during backtests
        """
        return list(self.additional_symbols)
    
    def get_all_required_symbols(self, primary_symbol: str = None) -> List[str]:
        """
        Get all symbols required by this strategy (primary + additional).
        
        Args:
            primary_symbol: The primary underlying symbol (if not provided,
                          will try to get from config["underlying"] or config["symbol"])
        
        Returns:
            List of all symbols this strategy needs for data loading
        """
        # Get primary symbol from parameter or config
        if not primary_symbol:
            primary_symbol = (
                self.config.get("underlying") or 
                self.config.get("symbol") or 
                "SPY"  # Default fallback
            )
        
        # Combine primary and additional symbols
        all_symbols = {primary_symbol}
        all_symbols.update(self.additional_symbols)
        
        return list(all_symbols)
    
    # ========================================================================
    # UI State Registration for Flow Engine Context Display
    # ========================================================================
    
    def register_ui_state(self, state_name: str):
        """
        Register a state variable to be exposed to the UI through the flow engine.
        
        This allows strategies to specify which state variables should be displayed
        in the UI's market context during flow engine execution, replacing the
        hardcoded list in the flow engine.
        
        Args:
            state_name: Name of the state variable to expose to UI
            
        Example:
            # For moving average strategy
            self.register_ui_state("current_fast_ma")
            self.register_ui_state("current_slow_ma")
            self.register_ui_state("current_position")
            
            # For vertical spread strategy  
            self.register_ui_state("short_leg")
            self.register_ui_state("long_leg")
            self.register_ui_state("price_vertical")
        """
        self.ui_states.add(state_name)
        self.logger.info(f"Registered UI state for flow engine display: {state_name}")
    
    def get_ui_states(self) -> List[str]:
        """
        Get list of state variables registered for UI display.
        
        Returns:
            List of state variable names that should be displayed in UI
        """
        return list(self.ui_states)
    
    # ========================================================================
    # State Management Helpers
    # ========================================================================
    
    def set_state(self, key: str, value: Any, reason: Optional[str] = None):
        """Set strategy state value"""
        return self.state.set(key, value, reason)
    
    def get_state(self, key: str, default: Any = None) -> Any:
        """Get strategy state value"""
        return self.state.get(key, default)
    
    def add_checkpoint(self, name: str, metadata: Optional[Dict[str, Any]] = None):
        """Add state checkpoint"""
        return self.state.add_checkpoint(name, metadata=metadata)
    
    def restore_checkpoint(self, name: str):
        """Restore from checkpoint"""
        return self.state.restore_from_checkpoint(name)
    
    # ========================================================================
    # Backward Compatibility - Legacy Methods
    # ========================================================================
    
    def get_status(self) -> Dict[str, Any]:
        """Get comprehensive strategy status (backward compatible)"""
        return {
            "name": self.__class__.__name__,
            "strategy_id": self.strategy_id,
            "is_running": self.is_running,
            "is_paused": self.is_paused,
            "dry_run": self.dry_run,
            "debug": self.debug,
            "start_time": self.start_time.isoformat() if self.start_time else None,
            "last_update": self.last_update.isoformat() if self.last_update else None,
            "execution_stats": self.execution_stats.copy(),
            "action_queue": {
                "pending_actions": len(self.action_queue.get_active_actions()),
                "completed_actions": len(self.action_queue.get_completed_actions())
            },
            "state_summary": self.state.get_summary(),
            "market_info": self.time_scheduler.get_market_summary(),
            "current_session": self.get_current_session().value,
            # Legacy fields for backward compatibility
            "positions": 0,  # Simplified for sync method
            "trades": self.execution_stats["trades_executed"],
            "pnl": self.execution_stats["total_pnl"],
            "error_count": self.execution_stats["error_count"],
            "config": self.config
        }
    
    def update_pnl(self, amount: float):
        """Update strategy P&L (backward compatible)"""
        self.execution_stats["total_pnl"] += amount
        self.last_update = datetime.now()
    
    def add_trade(self, trade: Dict[str, Any]):
        """Add a trade to the strategy's trade history (backward compatible)"""
        trade["timestamp"] = datetime.now().isoformat()
        self.execution_stats["trades_executed"] += 1
        self.last_update = datetime.now()
    
    def get_config_value(self, key: str, default: Any = None) -> Any:
        """Get configuration value (backward compatible)"""
        return self.config.get(key, default)
    
    def set_config_value(self, key: str, value: Any):
        """Set configuration value (backward compatible)"""
        self.config[key] = value
    
    # ========================================================================
    # Action Execution Helpers
    # ========================================================================
    
    def get_action_log(self) -> List[Dict[str, Any]]:
        """Get detailed action execution log"""
        log_entries = []
        
        for action in self.action_queue.actions:
            entry = {
                "name": action.name,
                "type": action.__class__.__name__,
                "status": action.status.value,
                "created_at": action.created_at.isoformat() if action.created_at else None,
                "started_at": action.started_at.isoformat() if action.started_at else None,
                "completed_at": action.completed_at.isoformat() if action.completed_at else None,
                "execution_log": action.execution_log.copy()
            }
            
            # Add enhanced debugging info for MonitorAction
            if hasattr(action, 'rule_description'):
                entry["rule_description"] = action.rule_description
            if hasattr(action, 'condition_parameters'):
                entry["condition_parameters"] = action.condition_parameters
            
            if action.result:
                result_data = {
                    "success": action.result.success,
                    "error": action.result.error,
                    "dry_run": action.result.dry_run,
                    "details": action.result.details
                }
                
                # Ensure action result data is JSON serializable
                if action.result.data:
                    serialized_data = {}
                    for key, value in action.result.data.items():
                        if hasattr(value, 'isoformat'):
                            serialized_data[key] = value.isoformat()
                        else:
                            serialized_data[key] = value
                    result_data["data"] = serialized_data
                
                # Add enhanced decision tracking data
                if action.result.decision_data:
                    result_data["decision_data"] = action.result.decision_data
                if action.result.rule_evaluations:
                    result_data["rule_evaluations"] = action.result.rule_evaluations
                if action.result.context_snapshot:
                    result_data["context_snapshot"] = action.result.context_snapshot
                
                entry["result"] = result_data
            
            log_entries.append(entry)
        
        return log_entries
    
    def add_decision_timeline(self, decision_data: Dict[str, Any]):
        """
        Add decision data to the strategy's decision timeline for debugging.
        
        This method allows strategies to record detailed decision-making data
        that can be used for backtesting analysis and debugging.
        
        Args:
            decision_data: Dictionary containing decision information:
                - timestamp: When the decision was made
                - rule_description: Human-readable description of the rule
                - result: Boolean result of the rule evaluation
                - context_values: Market data and indicator values at decision time
                - parameters: Rule parameters used in evaluation
                - evaluation_details: Additional details about the evaluation
                - check_number: Sequential number for this decision point
        """
        # Ensure we have a decision timeline list in state
        if not hasattr(self, '_decision_timeline'):
            self._decision_timeline = []
        
        # Add timestamp if not provided
        if 'timestamp' not in decision_data:
            decision_data['timestamp'] = datetime.now().isoformat()
        
        # Add strategy context
        decision_data['strategy_id'] = self.strategy_id
        decision_data['strategy_name'] = self.__class__.__name__
        
        # Store the decision data
        self._decision_timeline.append(decision_data)
        
        # Keep only the last 10000 entries to prevent memory issues
        if len(self._decision_timeline) > 10000:
            self._decision_timeline = self._decision_timeline[-10000:]
        
        # Log for debugging if enabled
        if self.debug:
            self.logger.debug(f"Decision recorded: {decision_data.get('rule_description', 'Unknown rule')} -> {decision_data.get('result', 'No result')}")

    def get_decision_timeline(self) -> List[Dict[str, Any]]:
        """Get timeline of all strategy decisions for debugging"""
        timeline = []
        
        # Get decisions from the new decision timeline
        if hasattr(self, '_decision_timeline'):
            timeline.extend(self._decision_timeline)
        
        # Also get decisions from action results (backward compatibility)
        for action in self.action_queue.actions:
            if action.result and action.result.decision_data:
                timeline_entry = {
                    "timestamp": action.result.decision_data.get("timestamp"),
                    "action_name": action.name,
                    "action_type": action.__class__.__name__,
                    "rule_description": action.result.decision_data.get("rule_description", ""),
                    "result": action.result.decision_data.get("result", False),
                    "context_values": action.result.decision_data.get("context_values", {}),
                    "parameters": action.result.decision_data.get("parameters", {}),
                    "error": action.result.decision_data.get("error"),
                    "check_number": action.result.decision_data.get("check_number", 0),
                    "evaluation_details": action.result.decision_data.get("evaluation_details", {}),
                    "strategy_id": self.strategy_id,
                    "strategy_name": self.__class__.__name__
                }
                timeline.append(timeline_entry)
        
        # Sort by timestamp
        timeline.sort(key=lambda x: x.get("timestamp", ""))
        return timeline
    
    def get_checkpoints(self) -> List[Dict[str, Any]]:
        """Get strategy checkpoints"""
        checkpoints = self.state.list_checkpoints()
        # Ensure all datetime objects are serialized
        for checkpoint in checkpoints:
            if 'timestamp' in checkpoint and hasattr(checkpoint['timestamp'], 'isoformat'):
                checkpoint['timestamp'] = checkpoint['timestamp'].isoformat()
            elif 'timestamp' in checkpoint:
                checkpoint['timestamp'] = str(checkpoint['timestamp'])
        return checkpoints
    
    def get_state_history(self) -> List[Dict[str, Any]]:
        """Get state change history"""
        history = []
        for change in self.state.state_history[-50:]:  # Last 50 changes
            change_dict = change.to_dict()
            # Ensure datetime objects are serialized
            if 'timestamp' in change_dict and hasattr(change_dict['timestamp'], 'isoformat'):
                change_dict['timestamp'] = change_dict['timestamp'].isoformat()
            elif 'timestamp' in change_dict:
                change_dict['timestamp'] = str(change_dict['timestamp'])
            history.append(change_dict)
        return history
    
    # ========================================================================
    # Utility Methods
    # ========================================================================
    
    def log_info(self, message: str):
        """Log info message"""
        self.logger.info(message)
    
    def log_warning(self, message: str):
        """Log warning message"""
        self.logger.warning(message)
    
    def log_error(self, message: str):
        """Log error message"""
        self.logger.error(message)
        self.execution_stats["error_count"] += 1
    
    # ========================================================================
    # ARCHITECTURAL FIX: Virtual Date Management
    # ========================================================================
    
    def _set_virtual_date(self, virtual_date: str):
        """
        Set virtual date for strategy (called by backtest engine).
        
        This allows the strategy to know what date it should think it's "living" on
        during backtesting without doing any timezone calculations.
        
        Args:
            virtual_date: Date string in format "YYYY-MM-DD" (e.g., "2025-08-12")
        """
        self._virtual_date = virtual_date
    
    def get_virtual_date(self) -> Optional[str]:
        """
        Get the virtual date if set by backtest engine.
        
        Returns:
            Virtual date string ("YYYY-MM-DD") or None if not in backtest
        """
        return self._virtual_date
    
    def __str__(self):
        return f"BaseStrategy(id={self.strategy_id}, running={self.is_running}, actions={len(self.action_queue.actions)})"
    
    def __repr__(self):
        return self.__str__()


# ============================================================================
# Example Strategy Implementation
# ============================================================================

class VerticalSpreadStrategy(BaseStrategy):
    """
    Example implementation of the vertical spread strategy using the action framework.
    
    This demonstrates the power of the action-based system:
    - Time-based triggers (1:30 PM start)
    - Dynamic monitoring (find verticals at 0.05, switch to better ones)
    - Conditional execution (enter at 0.15)
    - Expiration handling
    """
    
    async def initialize_strategy(self):
        """Initialize the vertical spread strategy"""
        # Set up initial state
        self.set_state("target_premium", 0.05)
        self.set_state("entry_premium", 0.15)
        self.set_state("max_positions", 3)
        self.set_state("current_vertical", None)
        
        # Action 1: Wait for 1:30 PM to start
        self.add_time_action(
            trigger_time="13:30",
            callback=self.start_monitoring,
            name="wait_for_start_time"
        )
        
        self.log_info("Vertical spread strategy initialized")
    
    async def start_monitoring(self, context: ActionContext):
        """Start monitoring for vertical spreads"""
        self.log_info("Starting vertical spread monitoring at 1:30 PM")
        
        # Action 2: Monitor for initial vertical at target premium
        self.add_monitor_action(
            name="find_initial_vertical",
            condition=lambda ctx: self.find_vertical_at_premium(ctx, self.get_state("target_premium")),
            callback=self.setup_vertical_monitoring,
            continuous=False
        )
        
        self.add_checkpoint("monitoring_started")
    
    async def setup_vertical_monitoring(self, context: ActionContext):
        """Set up monitoring for the found vertical"""
        vertical_data = self.find_vertical_at_premium(context, self.get_state("target_premium"))
        self.set_state("current_vertical", vertical_data)
        
        self.log_info(f"Found vertical: {vertical_data}")
        
        # Action 3: Monitor current vertical for entry premium
        self.add_monitor_action(
            name="monitor_entry_premium",
            condition=lambda ctx: self.check_entry_premium(ctx),
            callback=self.enter_trade,
            continuous=True
        )
        
        # Action 4: Simultaneously monitor for better verticals
        self.add_monitor_action(
            name="monitor_better_verticals",
            condition=lambda ctx: self.find_better_vertical(ctx),
            callback=self.switch_vertical,
            continuous=True
        )
        
        self.add_checkpoint("vertical_monitoring_setup", {"vertical": vertical_data})
    
    async def enter_trade(self, context: ActionContext):
        """Enter the vertical spread trade"""
        vertical = self.get_state("current_vertical")
        
        self.log_info(f"Entering trade for vertical: {vertical}")
        
        # Action 5: Execute the trade
        self.add_trade_action(
            name="enter_vertical_spread",
            trade_type="BUY_TO_OPEN",
            symbol=vertical["symbol"],
            quantity=1
        )
        
        # Action 6: Set up expiration monitoring
        expiration_date = vertical["expiration_date"]
        self.add_expiration_action(
            name="monitor_expiration",
            expiration_date=expiration_date,
            callback=self.handle_expiration
        )
        
        self.add_checkpoint("trade_entered", {"vertical": vertical})
    
    async def switch_vertical(self, context: ActionContext):
        """Switch to a better vertical"""
        better_vertical = self.find_better_vertical(context)
        old_vertical = self.get_state("current_vertical")
        
        self.log_info(f"Switching from {old_vertical} to {better_vertical}")
        
        self.set_state("current_vertical", better_vertical)
        self.add_checkpoint("vertical_switched", {
            "old_vertical": old_vertical,
            "new_vertical": better_vertical
        })
    
    async def handle_expiration(self, context: ActionContext):
        """Handle option expiration"""
        vertical = self.get_state("current_vertical")
        
        self.log_info(f"Handling expiration for vertical: {vertical}")
        
        # Let it expire (no action needed for this strategy)
        self.add_checkpoint("expiration_handled", {"vertical": vertical})
    
    def find_vertical_at_premium(self, context: ActionContext, target_premium: float) -> Optional[Dict]:
        """Find vertical spread at target premium"""
        # Mock implementation - in real strategy, this would analyze options chain
        if context.debug_mode:
            self.log_info(f"Searching for vertical at premium {target_premium}")
        
        # Simulate finding a vertical
        return {
            "symbol": "SPY_VERTICAL_SPREAD",
            "premium": target_premium,
            "strike_spread": 5,
            "expiration_date": context.current_time + timedelta(days=30)
        }
    
    def check_entry_premium(self, context: ActionContext) -> bool:
        """Check if vertical has reached entry premium"""
        vertical = self.get_state("current_vertical")
        entry_premium = self.get_state("entry_premium")
        
        if not vertical:
            return False
        
        # Mock implementation - check if premium reached target
        current_premium = vertical["premium"] * 3  # Simulate price movement
        
        if context.debug_mode:
            self.log_info(f"Current premium: {current_premium}, target: {entry_premium}")
        
        return current_premium >= entry_premium
    
    def find_better_vertical(self, context: ActionContext) -> Optional[Dict]:
        """Find a better vertical spread closer to current price"""
        current_vertical = self.get_state("current_vertical")
        target_premium = self.get_state("target_premium")
        
        if not current_vertical:
            return None
        
        # Mock implementation - simulate finding a better vertical
        # In real implementation, this would analyze the options chain
        
        if context.debug_mode:
            self.log_info("Searching for better vertical spreads")
        
        # Simulate occasionally finding a better vertical
        import random
        if random.random() < 0.1:  # 10% chance
            return {
                "symbol": "SPY_BETTER_VERTICAL",
                "premium": target_premium,
                "strike_spread": 5,
                "expiration_date": context.current_time + timedelta(days=30),
                "distance_to_price": 2.5  # Closer to current price
            }
        
        return None
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return strategy metadata"""
        return {
            "name": "Vertical Spread Strategy",
            "description": "Dynamic vertical spread strategy with premium monitoring",
            "version": "1.0.0",
            "author": "Action Framework",
            "risk_level": "MEDIUM",
            "max_positions": 3,
            "preferred_symbols": ["SPY", "QQQ", "IWM"],
            "parameters": {
                "target_premium": {
                    "type": "float",
                    "default": 0.05,
                    "min": 0.01,
                    "max": 0.20,
                    "description": "Target premium for initial vertical identification"
                },
                "entry_premium": {
                    "type": "float", 
                    "default": 0.15,
                    "min": 0.05,
                    "max": 0.50,
                    "description": "Premium level to enter the trade"
                },
                "max_positions": {
                    "type": "integer",
                    "default": 3,
                    "min": 1,
                    "max": 10,
                    "description": "Maximum number of concurrent positions"
                }
            }
        }
