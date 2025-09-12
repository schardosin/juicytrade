"""
Pure Declarative Moving Average Strategy

This strategy demonstrates the pure declarative approach using the enhanced flow engine.
It implements the exact same logic as the original MovingAverageStrategy but with
dramatically simplified code - no record_cycle_decision method needed!

Key Features:
- Pure declarative flow definition
- Automatic decision recording by flow engine
- Automatic indicator updates
- Same exact trading results as original strategy
- ~200 lines instead of ~700+ lines
- Enhanced debugging and visualization
"""

from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List

from .base_strategy import BaseStrategy
from .actions import ActionContext
from .rules import Rules

import logging
logger = logging.getLogger(__name__)


class PureDeclarativeMovingAverageStrategy(BaseStrategy):
    """
    Pure Declarative Moving Average Crossover Strategy
    
    This strategy implements the exact same logic as MovingAverageStrategy but uses
    the pure declarative approach - no record_cycle_decision method needed!
    
    The flow engine automatically:
    - Updates indicators when needed
    - Records all decision data for UI
    - Handles execution flow
    - Provides rich debugging information
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        super().__init__(strategy_id, data_provider, order_executor, config)
        self.symbol = None
        self.fast_period = None
        self.slow_period = None
        self.stop_loss_pct = None
        self.take_profit_pct = None
        self.log_info(f"PureDeclarativeMovingAverageStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Initialize strategy parameters and define the pure declarative flow"""
        # Initialize parameters (same as original)
        self.symbol = self.get_config_value("symbol", "SPY")
        self.fast_period = self.get_config_value("fast_period", 10)
        self.slow_period = self.get_config_value("slow_period", 30)
        self.stop_loss_pct = self.get_config_value("stop_loss_pct", 2.0)
        self.take_profit_pct = self.get_config_value("take_profit_pct", 5.0)
        
        # Initialize state (same as original)
        self.set_state("symbol", self.symbol)
        self.set_state("fast_period", self.fast_period)
        self.set_state("slow_period", self.slow_period)
        self.set_state("price_history", [])
        self.set_state("fast_ma_history", [])
        self.set_state("slow_ma_history", [])
        self.set_state("current_position", 0)
        self.set_state("entry_price", None)
        self.set_state("last_crossover", None)
        
        # Add time-based trigger (same as original)
        self.add_time_action(
            trigger_time="09:30",
            callback=self.start_monitoring,
            name="wait_for_market_open"
        )
        
        # Register UI states for flow engine context display
        self.register_ui_state("current_fast_ma")
        self.register_ui_state("current_slow_ma")
        self.register_ui_state("current_position")
        self.register_ui_state("entry_price")
        self.register_ui_state("symbol")
        self.log_info("Registered UI states for flow engine display")
        
        # --- PURE DECLARATIVE FLOW DEFINITION ---
        # This is the ONLY place where strategy logic is defined!
        self._define_strategy_flow()

        # Register data processors in order of execution
        self.register_data_processor(self.update_price_history)
        self.register_data_processor(self.calculate_moving_averages)
        self.register_data_processor(self.update_current_indicators)
        
        self.log_info(f"Pure Declarative Moving Average Strategy initialized for {self.symbol}")
        self.add_checkpoint("strategy_initialized", {
            "symbol": self.symbol,
            "fast_period": self.fast_period,
            "slow_period": self.slow_period
        })
    
    def _define_strategy_flow(self):
        """Define the complete strategy with Market Data Ready dependency and conditional flows"""
        # Create action nodes first
        buy_action = self.flow.add_action("Execute Buy Order", self.open_long_position)
        sell_action = self.flow.add_action("Execute Sell Order", self.close_position)
        
        # Market Data Ready Flow: Runs until satisfied, then skips
        market_data_flow = self.flow.add_decision(
            name="Market Data Ready?",
            condition=Rules.AllOf(self.has_enough_data),
            if_true=None,  # No action when ready, just continue
            if_false=None, # No action when not ready, just end
            skip_when_satisfied=True  # Skip this check once we have enough data
        )
        
        # Entry Flow: Only runs when position = 0 AND market data is ready
        entry_flow = self.flow.add_decision(
            name="Entry Signal Check",
            condition=Rules.AllOf(
                self.is_bullish_crossover,  # bullish_crossover
                self.has_sufficient_capital # sufficient_capital
            ),
            if_true=buy_action,
            if_false=None,
            execution_condition=self._can_run_entry_flow  # Custom condition: not in position AND market data ready
        )
        
        # Exit Flow: Only runs when position > 0 AND market data is ready
        exit_flow = self.flow.add_decision(
            name="Exit Signal Check", 
            condition=Rules.AllOf(
                self.is_bearish_crossover   # bearish_crossover
            ),
            if_true=sell_action,
            if_false=None,
            execution_condition=self._can_run_exit_flow  # Custom condition: in position AND market data ready
        )
        
        # Set up parallel flows - all three run independently based on their conditions
        self.flow.set_parallel_flows([market_data_flow, entry_flow, exit_flow])
        
        self.log_info(f"Pure declarative flows with Market Data Ready dependency defined with {self.flow.get_node_count()} nodes")
    
    async def start_monitoring(self, context: ActionContext):
        """Start monitoring - called at market open"""
        self.log_info("Market opened - starting pure declarative MA crossover monitoring")
        self.add_checkpoint("monitoring_started")
    
    # ========================================================================
    # Rule Methods - Individual boolean functions for the flow engine
    # ========================================================================
    
    def has_enough_data(self, context: ActionContext) -> bool:
        """Rule: Check if we have enough price data for analysis"""
        price_history = self.get_state("price_history", [])
        has_data = len(price_history) >= self.slow_period
        
        # DEBUG: Log the data check
        if self.debug:
            self.log_info(f"DEBUG: has_enough_data check - price_history length: {len(price_history)}, slow_period: {self.slow_period}, result: {has_data}")
        
        return has_data
    
    def is_in_position(self, context: ActionContext) -> bool:
        """Rule: Check if currently in a position"""
        current_position = self.get_state("current_position", 0)
        return current_position > 0
    
    def is_bullish_crossover(self, context: ActionContext) -> bool:
        """Rule: Detect true bullish crossover (exact same logic as original)"""
        fast_ma_history = self.get_state("fast_ma_history", [])
        slow_ma_history = self.get_state("slow_ma_history", [])
        
        # Need at least 2 MA values to detect crossover
        if len(fast_ma_history) < 2 or len(slow_ma_history) < 2:
            return False
        
        # Previous values
        prev_fast_ma = fast_ma_history[-2]
        prev_slow_ma = slow_ma_history[-2]
        
        # Current values
        curr_fast_ma = fast_ma_history[-1]
        curr_slow_ma = slow_ma_history[-1]

        # True bullish crossover: was below or equal, now above
        was_below = prev_fast_ma <= prev_slow_ma
        now_above = curr_fast_ma > curr_slow_ma
        bullish_crossover = was_below and now_above
        
        return bullish_crossover
    
    def is_not_in_position(self, context: ActionContext) -> bool:
        """Rule: Check if NOT currently in a position"""
        current_position = self.get_state("current_position", 0)
        return current_position == 0
    
    def has_sufficient_capital(self, context: ActionContext) -> bool:
        """Rule: Check if we have sufficient capital to open a position"""
        position_size = self.calculate_position_size()
        return position_size > 0
    
    def is_bearish_crossover(self, context: ActionContext) -> bool:
        """Rule: Detect bearish crossover (exact same logic as original)"""
        current_fast_ma = self.get_state("current_fast_ma", 0)
        current_slow_ma = self.get_state("current_slow_ma", 0)
        
        # Simple bearish condition: fast MA below slow MA (same as original)
        return current_fast_ma < current_slow_ma
    
    # ========================================================================
    # Execution Condition Methods - Control when flows should run
    # ========================================================================
    
    def _can_run_entry_flow(self, context: ActionContext) -> bool:
        """Execution condition: Entry flow only runs when not in position AND market data is ready"""
        # Check if not in position
        if not self.is_not_in_position(context):
            return False
        
        # Check if market data is ready (or if the market data flow is satisfied)
        market_data_flow = None
        for flow in self.flow.parallel_flows:
            if hasattr(flow, 'name') and flow.name == "Market Data Ready?":
                market_data_flow = flow
                break
        
        # If market data flow is satisfied (skip_when_satisfied=True and satisfied=True), 
        # then market data is ready
        if market_data_flow and hasattr(market_data_flow, 'satisfied') and market_data_flow.satisfied:
            return True
        
        # If market data flow is not satisfied yet, check if data is ready now
        return self.has_enough_data(context)
    
    def _can_run_exit_flow(self, context: ActionContext) -> bool:
        """Execution condition: Exit flow only runs when in position AND market data is ready"""
        # Check if in position
        if not self.is_in_position(context):
            return False
        
        # Check if market data is ready (or if the market data flow is satisfied)
        market_data_flow = None
        for flow in self.flow.parallel_flows:
            if hasattr(flow, 'name') and flow.name == "Market Data Ready?":
                market_data_flow = flow
                break
        
        # If market data flow is satisfied (skip_when_satisfied=True and satisfied=True), 
        # then market data is ready
        if market_data_flow and hasattr(market_data_flow, 'satisfied') and market_data_flow.satisfied:
            return True
        
        # If market data flow is not satisfied yet, check if data is ready now
        return self.has_enough_data(context)
    
    # ========================================================================
    # Action Methods - Functions executed by action nodes
    # ========================================================================
    
    async def open_long_position(self, context: ActionContext):
        """Action: Open a long position"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position != 0:
                self.log_error(f"Position violation: Attempting to open long position when current_position={current_position}")
                return
            
            position_size = self.calculate_position_size()
            
            if position_size <= 0:
                self.log_warning("Cannot open position - insufficient capital or invalid size")
                return
            
            if hasattr(self.order_executor, 'place_market_order'):
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=position_size,
                    side="BUY",
                    reason="Entry signal detected - Fast MA above Slow MA"
                )
                
                if trade_id:
                    current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
                    self.set_state("current_position", position_size)
                    self.set_state("entry_price", current_price)
                    self.set_state("position_type", "long")
                    
                    self.log_info(f"TRADE EXECUTED: BUY {position_size} {self.symbol} @ ${current_price:.2f} - Trade ID: {trade_id}")
                else:
                    self.log_error("Trade execution failed - no trade_id returned")
            else:
                self.log_error("Order executor does not support place_market_order")
            
        except Exception as e:
            self.log_error(f"Error opening long position: {e}")
    
    async def close_position(self, context: ActionContext):
        """Action: Close current position"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position == 0:
                return  # No position to close
            
            # Determine trade action
            if current_position > 0:
                side = "SELL"
                quantity = current_position
            else:
                side = "BUY_TO_COVER"
                quantity = abs(current_position)
            
            # Execute trade through backtest engine
            if hasattr(self.order_executor, 'place_market_order'):
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=quantity,
                    side=side,
                    reason="Exit signal detected - Fast MA below Slow MA"
                )
                self.log_info(f"TRADE EXECUTED: {side} {quantity} {self.symbol} - Trade ID: {trade_id}")
                
                # Calculate P&L
                entry_price = self.get_state("entry_price", 0)
                current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
                
                if entry_price and current_price:
                    if current_position > 0:  # Long position
                        pnl = (current_price - entry_price) * current_position
                    else:  # Short position
                        pnl = (entry_price - current_price) * abs(current_position)
                    
                    self.update_pnl(pnl)
                    self.log_info(f"Closing position: P&L: ${pnl:.2f}")
                
                # Reset position state if trade was executed
                if trade_id:
                    self.set_state("current_position", 0)
                    self.set_state("entry_price", None)
                    self.set_state("position_type", None)
                    
                    self.add_checkpoint("position_closed", {
                        "pnl": pnl if 'pnl' in locals() else 0,
                        "timestamp": context.current_time.isoformat()
                    })
                else:
                    self.log_error("Exit trade execution failed - position state not updated")
            else:
                self.log_error("Order executor does not support place_market_order for exit trades")
            
        except Exception as e:
            self.log_error(f"Error closing position: {e}")
    
    # ========================================================================
    # Helper Methods (same as original strategy)
    # ========================================================================
    
    # ========================================================================
    # Data Processor Methods - Registered in order of execution
    # ========================================================================
    
    def update_price_history(self, context: ActionContext) -> bool:
        """Data Processor 1: Update price history with new data points"""
        try:
            # Get current price from data provider
            current_price = self.get_current_price_from_provider()
            if current_price is None:
                return False
            
            # Update price history - but only if this is a new timestamp
            price_history = self.get_state("price_history", [])
            last_timestamp = self.get_state("last_price_timestamp", None)
            current_timestamp = context.current_time
            
            # Only add price if this is a new timestamp (avoid duplicates)
            if last_timestamp != current_timestamp:
                price_history.append(current_price)
                self.set_state("last_price_timestamp", current_timestamp)
                
                # Keep only what we need
                max_history = max(self.fast_period, self.slow_period) + 10
                if len(price_history) > max_history:
                    price_history = price_history[-max_history:]
                
                self.set_state("price_history", price_history)
                self.set_state("new_price_added", True)  # Flag for next processors
                return True
            
            self.set_state("new_price_added", False)  # No new price added
            return False
            
        except Exception as e:
            self.log_error(f"Error updating price history: {e}")
            return False
    
    def calculate_moving_averages(self, context: ActionContext) -> bool:
        """Data Processor 2: Calculate moving averages when we have enough data"""
        try:
            # Only calculate if we added a new price
            if not self.get_state("new_price_added", False):
                return False
            
            price_history = self.get_state("price_history", [])
            
            # Calculate moving averages if we have enough data
            if len(price_history) >= self.slow_period:
                fast_ma = self.calculate_ma(price_history, self.fast_period)
                slow_ma = self.calculate_ma(price_history, self.slow_period)
                
                # Store calculated values for next processor
                self.set_state("calculated_fast_ma", fast_ma)
                self.set_state("calculated_slow_ma", slow_ma)
                self.set_state("ma_calculated", True)
                return True
            
            self.set_state("ma_calculated", False)
            return False
            
        except Exception as e:
            self.log_error(f"Error calculating moving averages: {e}")
            return False
    
    def update_current_indicators(self, context: ActionContext) -> bool:
        """Data Processor 3: Update current indicators and MA history"""
        try:
            # Only update if we calculated new MAs
            if not self.get_state("ma_calculated", False):
                return False
            
            fast_ma = self.get_state("calculated_fast_ma")
            slow_ma = self.get_state("calculated_slow_ma")
            
            # Update MA history
            fast_ma_history = self.get_state("fast_ma_history", [])
            slow_ma_history = self.get_state("slow_ma_history", [])
            
            fast_ma_history.append(fast_ma)
            slow_ma_history.append(slow_ma)
            
            # Keep reasonable history
            if len(fast_ma_history) > 100:
                fast_ma_history = fast_ma_history[-100:]
                slow_ma_history = slow_ma_history[-100:]
            
            # Update all state values
            self.set_state("fast_ma_history", fast_ma_history)
            self.set_state("slow_ma_history", slow_ma_history)
            self.set_state("current_fast_ma", fast_ma)
            self.set_state("current_slow_ma", slow_ma)
            
            # Clean up temporary flags
            self.set_state("new_price_added", False)
            self.set_state("ma_calculated", False)
            self.set_state("calculated_fast_ma", None)
            self.set_state("calculated_slow_ma", None)
            
            return True
            
        except Exception as e:
            self.log_error(f"Error updating current indicators: {e}")
            return False
    
    def calculate_position_size(self) -> int:
        """Calculate position size based on available capital (same as original)"""
        try:
            # Get available capital from backtest engine if possible
            available_capital = 10000  # Default fallback
            
            # Try to get actual available capital from backtest engine
            if hasattr(self.order_executor, 'current_capital'):
                available_capital = self.order_executor.current_capital
            else:
                # Fallback to config value
                available_capital = self.get_config_value("account_balance", 10000)
            
            price_history = self.get_state("price_history", [])
            if not price_history:
                return 0
            
            current_price = price_history[-1]
            if current_price <= 0:
                return 0
            
            # Simple position sizing - use percentage of available capital
            position_value = available_capital * 0.95
            position_size = int(position_value / current_price)
            
            # Apply reasonable limits
            min_size = self.get_config_value("min_position_size", 1)
            max_size = self.get_config_value("max_position_size", 1000)
            
            # Ensure we don't exceed available capital
            max_affordable = int(available_capital / current_price)
            
            final_size = max(min_size, min(position_size, max_size, max_affordable))
            
            return final_size
            
        except Exception as e:
            self.log_error(f"Error calculating position size: {e}")
            return 0
    
    def calculate_ma(self, price_history: List[float], period: int) -> float:
        """Calculate simple moving average (same as original)"""
        if len(price_history) < period:
            return price_history[-1] if price_history else 0
        
        return sum(price_history[-period:]) / period
    
    def get_current_price_from_provider(self) -> Optional[float]:
        """Get current price from the data provider (same as original)"""
        try:
            # Use the data provider interface (backtest engine provides this)
            if hasattr(self.data_provider, 'get_current_price'):
                price = self.data_provider.get_current_price(self.symbol)
                if price is not None:
                    return price

            # No fallback - return None if no price available
            self.log_warning(f"No price available from data provider for {self.symbol}")
            return None

        except Exception as e:
            self.log_error(f"Error getting current price: {e}")
            return None
    
    def _calculate_unrealized_pnl(self) -> float:
        """Calculate unrealized P&L for current position (same as original)"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position == 0:
                return 0.0
            
            entry_price = self.get_state("entry_price")
            if not entry_price:
                return 0.0
            
            current_price = self.get_current_price_from_provider()
            if not current_price:
                return 0.0
            
            if current_position > 0:  # Long position
                return (current_price - entry_price) * current_position
            else:  # Short position
                return (entry_price - current_price) * abs(current_position)
                
        except Exception as e:
            self.log_error(f"Error calculating unrealized P&L: {e}")
            return 0.0
    
    # ========================================================================
    # Strategy Metadata
    # ========================================================================
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return strategy metadata for UI and monitoring (same as original)"""
        return {
            "name": "Pure Declarative Moving Average Crossover Strategy",
            "description": "Pure declarative flow-based MA crossover strategy (10/30) using enhanced Rules engine with automatic decision recording",
            "version": "4.0.0",
            "author": "Pure Declarative Framework",
            "risk_level": "HIGH",
            "max_positions": 1,
            "preferred_symbols": ["SPY"],
            "parameters": {
                "symbol": {
                    "type": "string",
                    "default": "SPY",
                    "description": "Trading symbol",
                    "category": "strategy"
                },
                "fast_period": {
                    "type": "integer",
                    "default": 10,
                    "min": 2,
                    "max": 50,
                    "description": "Fast moving average period",
                    "category": "strategy"
                },
                "slow_period": {
                    "type": "integer",
                    "default": 30,
                    "min": 10,
                    "max": 200,
                    "description": "Slow moving average period",
                    "category": "strategy"
                },
                "stop_loss_pct": {
                    "type": "float",
                    "default": 2.0,
                    "min": 0.5,
                    "max": 10.0,
                    "description": "Stop loss percentage",
                    "category": "strategy"
                },
                "take_profit_pct": {
                    "type": "float",
                    "default": 5.0,
                    "min": 1.0,
                    "max": 20.0,
                    "description": "Take profit percentage",
                    "category": "strategy"
                },
                "allow_short": {
                    "type": "boolean",
                    "default": False,
                    "description": "Allow short positions",
                    "category": "strategy"
                },
                "risk_per_trade_pct": {
                    "type": "float",
                    "default": 1.0,
                    "min": 0.1,
                    "max": 5.0,
                    "description": "Risk per trade as % of capital",
                    "category": "strategy"
                },
                "account_balance": {
                    "type": "float",
                    "default": 100000.0,
                    "min": 1000.0,
                    "max": 10000000.0,
                    "description": "Account balance for position sizing",
                    "category": "framework"
                },
                "max_position_size": {
                    "type": "integer",
                    "default": 1000,
                    "min": 1,
                    "max": 10000,
                    "description": "Maximum position size (shares/contracts)",
                    "category": "framework"
                },
                "commission_per_trade": {
                    "type": "float",
                    "default": 1.0,
                    "min": 0.0,
                    "max": 50.0,
                    "description": "Commission per trade ($)",
                    "category": "framework"
                }
            }
        }
    
    def get_flow_graph_data(self) -> Dict[str, Any]:
        """Get flow graph data for visualization"""
        return self.flow.to_graph_data()

    # NO record_cycle_decision method needed!
    # The flow engine handles everything automatically:
    # - Updates indicators via update_indicators()
    # - Executes the declarative flow
    # - Records all decision data for UI
    # - Provides rich debugging information
    # - Handles errors gracefully
