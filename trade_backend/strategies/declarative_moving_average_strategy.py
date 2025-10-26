"""
Declarative Moving Average Strategy - Flow Engine Example

This strategy demonstrates the new declarative flow engine by implementing the exact same
logic as the existing MovingAverageStrategy, but using a declarative graph-based approach
instead of imperative logic in record_cycle_decision.

Key Features:
- Declarative flow definition using self.flow
- Structured rules using Rules helper
- Graph-based execution flow
- Identical trading results to the original strategy
- Automatic graph visualization support

Strategy Logic (identical to original):
- Wait for market open (9:30 AM)
- Calculate 10/30 period moving averages
- Enter long on bullish crossover (fast MA crosses above slow MA)
- Exit on bearish crossover (fast MA crosses below slow MA)
- Risk management with stop loss and take profit
"""

import logging
from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List

from .base_strategy import BaseStrategy
from .actions import ActionContext
from .rules import Rules
from .flow_engine import RuleCondition

logger = logging.getLogger(__name__)


class DeclarativeMovingAverageStrategy(BaseStrategy):
    """
    Declarative Moving Average Crossover Strategy using Flow Engine
    
    This strategy implements the exact same logic as MovingAverageStrategy but uses
    the new declarative flow engine to define the strategy as a graph of rules and actions.
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        super().__init__(strategy_id, data_provider, order_executor, config)
        self.symbol = None
        self.fast_period = None
        self.slow_period = None
        self.stop_loss_pct = None
        self.take_profit_pct = None
        self.log_info(f"DeclarativeMovingAverageStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Initialize strategy parameters and define the declarative flow"""
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
        
        # --- DECLARATIVE FLOW DEFINITION ---
        # Define the strategy flow as a graph of interconnected rules and actions
        self._define_strategy_flow()
        
        self.log_info(f"Declarative Moving Average Strategy initialized for {self.symbol}")
        self.add_checkpoint("strategy_initialized", {
            "symbol": self.symbol,
            "fast_period": self.fast_period,
            "slow_period": self.slow_period
        })
    
    def _define_strategy_flow(self):
        """Define the declarative strategy flow graph"""
        # Root decision: Check if market data is ready
        root_decision = self.flow.add_decision(
            name="Is Market Ready?",
            condition=Rules.AllOf(self.has_enough_data),
            
            # If there IS enough data, proceed to position analysis
            if_true=self.flow.add_decision(
                name="Analyze Current Position",
                condition=Rules.AllOf(self.is_in_position),
                
                # Path if NOT in position -> Check for entry signal
                if_false=self.flow.add_decision(
                    name="Check Entry Signal",
                    condition=Rules.AllOf(
                        self.is_bullish_crossover
                        # Note: We're already in the "NOT in position" branch, so no need to check again
                    ),
                    if_true=self.flow.add_action("Execute Buy Order", self.open_long_position)
                ),
                
                # Path if we ARE in position -> Check for exit signal
                if_true=self.flow.add_decision(
                    name="Check Exit Signal",
                    condition=Rules.AllOf(  # Exit only on bearish crossover (same as original)
                        self.is_bearish_crossover
                    ),
                    if_true=self.flow.add_action("Execute Sell Order", self.close_position)
                )
            )
            # if_false (not enough data) -> flow ends naturally
        )
        
        # Set the root node for execution
        self.flow.set_root_node(root_decision)
        
        self.log_info(f"Declarative flow defined with {self.flow.get_node_count()} nodes")
    
    async def start_monitoring(self, context: ActionContext):
        """Start monitoring - called at market open"""
        self.log_info("Market opened - starting declarative MA crossover monitoring")
        self.add_checkpoint("monitoring_started")
    
    # ========================================================================
    # Rule Methods - Individual boolean functions for the flow engine
    # ========================================================================
    
    def has_enough_data(self, context: ActionContext) -> bool:
        """Rule: Check if we have enough price data for analysis"""
        price_history = self.get_state("price_history", [])
        return len(price_history) >= self.slow_period
    
    def is_in_position(self, context: ActionContext) -> bool:
        """Rule: Check if currently in a position"""
        current_position = self.get_state("current_position", 0)
        return current_position > 0
    
    def is_bullish_crossover(self, context: ActionContext) -> bool:
        """Rule: Detect true bullish crossover (fast MA crosses above slow MA)"""
        fast_ma_history = self.get_state("fast_ma_history", [])
        slow_ma_history = self.get_state("slow_ma_history", [])
        
        # Need at least 2 MA values to detect crossover
        if len(fast_ma_history) < 2 or len(slow_ma_history) < 2:
            self.log_info(f"🔍 CROSSOVER DEBUG: Not enough MA history - fast: {len(fast_ma_history)}, slow: {len(slow_ma_history)}")
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
        is_crossover = was_below and now_above
        
        # Debug logging can be enabled if needed
        if self.debug:
            self.log_info(f"Crossover check: Prev: Fast={prev_fast_ma:.2f}, Slow={prev_slow_ma:.2f} | Curr: Fast={curr_fast_ma:.2f}, Slow={curr_slow_ma:.2f}")
            self.log_info(f"Crossover result: was_below={was_below}, now_above={now_above}, is_crossover={is_crossover}")
        
        return is_crossover
    
    def is_bearish_crossover(self, context: ActionContext) -> bool:
        """Rule: Detect bearish crossover (fast MA crosses below slow MA)"""
        current_fast_ma = self.get_state("current_fast_ma", 0)
        current_slow_ma = self.get_state("current_slow_ma", 0)
        
        # Simple bearish condition: fast MA below slow MA (same as original)
        return current_fast_ma < current_slow_ma
    
    def check_risk_management(self, context: ActionContext) -> bool:
        """Rule: Check if risk management rules are triggered"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position == 0:
                return False  # No position to manage
            
            entry_price = self.get_state("entry_price")
            if not entry_price:
                return False
            
            price_history = self.get_state("price_history", [])
            if not price_history:
                return False
            
            current_price = price_history[-1]
            
            # Calculate P&L percentage
            if current_position > 0:  # Long position
                pnl_pct = (current_price - entry_price) / entry_price * 100
            else:  # Short position
                pnl_pct = (entry_price - current_price) / entry_price * 100
            
            # Check stop loss
            if pnl_pct <= -self.stop_loss_pct:
                self.set_state("exit_reason", f"Stop loss triggered: {pnl_pct:.1f}%")
                return True
            
            # Check take profit
            if pnl_pct >= self.take_profit_pct:
                self.set_state("exit_reason", f"Take profit triggered: {pnl_pct:.1f}%")
                return True
            
            return False
            
        except Exception as e:
            self.log_error(f"Error checking risk management: {e}")
            return False
    
    # ========================================================================
    # Action Methods - Functions executed by action nodes
    # ========================================================================
    
    async def open_long_position(self, context: ActionContext):
        """Action: Open a long position"""
        try:
            # Check current position state before opening
            current_position = self.get_state("current_position", 0)
            if current_position != 0:
                self.log_error(f"🚨 POSITION VIOLATION: Attempting to open long position when current_position={current_position}")
                return
            
            # Calculate position size
            position_size = self.calculate_position_size()
            
            if position_size <= 0:
                self.log_warning("Cannot open position - insufficient capital or invalid size")
                return
            
            self.log_info(f"🚀 DECLARATIVE: Attempting to execute BUY order for {position_size} shares of {self.symbol}")
            
            # Execute trade through backtest engine - EXACTLY like original strategy
            if hasattr(self.order_executor, 'place_market_order'):
                self.log_info(f"🚀 DECLARATIVE: Calling place_market_order with symbol={self.symbol}, quantity={position_size}, side=BUY")
                
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=position_size,
                    side="BUY",
                    reason="Entry signal detected - Fast MA above Slow MA"
                )
                
                self.log_info(f"🚀 DECLARATIVE: place_market_order returned trade_id: {trade_id}")
                
                if trade_id:
                    current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
                    self.set_state("current_position", position_size)
                    self.set_state("entry_price", current_price)
                    self.set_state("position_type", "long")
                    
                    self.log_info(f"✅ DECLARATIVE TRADE EXECUTED: BUY {position_size} {self.symbol} @ ${current_price:.2f} - Trade ID: {trade_id}")
                    self.log_info(f"✅ DECLARATIVE POSITION STATE UPDATED: current_position={position_size}, entry_price=${current_price:.2f}")
                else:
                    self.log_error("🚨 DECLARATIVE: Trade execution failed - no trade_id returned")
            else:
                self.log_error("🚨 DECLARATIVE: Order executor does not support place_market_order")
                # Log what methods are available
                if self.order_executor:
                    available_methods = [method for method in dir(self.order_executor) if not method.startswith('_')]
                    self.log_info(f"Available order executor methods: {available_methods}")
            
        except Exception as e:
            self.log_error(f"🚨 DECLARATIVE: Error opening long position: {e}")
            import traceback
            self.log_error(f"🚨 DECLARATIVE: Traceback: {traceback.format_exc()}")
    
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
            
            # Get exit reason
            exit_reason = self.get_state("exit_reason", "Exit signal detected - Fast MA below Slow MA")
            
            # Execute trade through backtest engine
            if hasattr(self.order_executor, 'place_market_order'):
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=quantity,
                    side=side,
                    reason=exit_reason
                )
                self.log_info(f"✅ TRADE EXECUTED: {side} {quantity} {self.symbol} - Trade ID: {trade_id}")
                
                # Calculate P&L
                entry_price = self.get_state("entry_price", 0)
                current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
                
                if entry_price and current_price:
                    if current_position > 0:  # Long position
                        pnl = (current_price - entry_price) * current_position
                    else:  # Short position
                        pnl = (entry_price - current_price) * abs(current_position)
                    
                    self.update_pnl(pnl)
                    self.log_info(f"Closing position: {exit_reason}, P&L: ${pnl:.2f}")
                
                # Reset position state if trade was executed
                if trade_id:
                    self.set_state("current_position", 0)
                    self.set_state("entry_price", None)
                    self.set_state("position_type", None)
                    self.set_state("exit_reason", None)  # Clear exit reason
                    
                    self.log_info(f"✅ POSITION STATE RESET: current_position=0, position closed")
                    
                    self.add_checkpoint("position_closed", {
                        "reason": exit_reason,
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
    
    def update_indicators(self, context: ActionContext) -> bool:
        """Update price history and calculate moving averages (same as original)"""
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
            
            # Calculate moving averages if we have enough data
            if len(price_history) >= self.slow_period:
                fast_ma = self.calculate_ma(price_history, self.fast_period)
                slow_ma = self.calculate_ma(price_history, self.slow_period)
                
                # Update MA history
                fast_ma_history = self.get_state("fast_ma_history", [])
                slow_ma_history = self.get_state("slow_ma_history", [])
                
                fast_ma_history.append(fast_ma)
                slow_ma_history.append(slow_ma)
                
                # Keep reasonable history
                if len(fast_ma_history) > 100:
                    fast_ma_history = fast_ma_history[-100:]
                    slow_ma_history = slow_ma_history[-100:]
                
                self.set_state("fast_ma_history", fast_ma_history)
                self.set_state("slow_ma_history", slow_ma_history)
                self.set_state("current_fast_ma", fast_ma)
                self.set_state("current_slow_ma", slow_ma)
                
                if self.debug:
                    self.log_info(f"Updated MAs: Fast={fast_ma:.2f}, Slow={slow_ma:.2f}, Price={current_price:.2f}")
                
                return True  # Indicators updated successfully
            
            return False  # Not enough data yet
            
        except Exception as e:
            self.log_error(f"Error updating indicators: {e}")
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
            
            self.log_info(f"Position sizing: Available=${available_capital:.2f}, Price=${current_price:.2f}, Size={final_size} shares, Value=${final_size * current_price:.2f}")
            
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
    
    def _calculate_unrealized_pnl(self, current_price: float) -> float:
        """Calculate unrealized P&L for current position (same as original)"""
        try:
            current_position = self.get_state("current_position", 0)
            entry_price = self.get_state("entry_price")
            
            if current_position == 0 or not entry_price:
                return 0.0
            
            if current_position > 0:  # Long position
                return (current_price - entry_price) * current_position
            else:  # Short position
                return (entry_price - current_price) * abs(current_position)
                
        except Exception as e:
            self.log_error(f"Error calculating unrealized P&L: {e}")
            return 0.0
    
    # ========================================================================
    # Decision Recording - Enhanced for Flow Engine
    # ========================================================================
    
    async def record_cycle_decision(self, context: ActionContext):
        """
        Record decision for this execution cycle using the declarative flow engine.
        
        This method now updates indicators and then executes the declarative flow,
        while still recording detailed decision data for debugging.
        """
        try:
            # First, update indicators with current price data
            self.update_indicators(context)
            
            # Get current market context for decision tracking
            current_price = self.get_current_price_from_provider()
            if current_price is None:
                return  # Skip if no price data available
            
            current_fast_ma = self.get_state("current_fast_ma", 0)
            current_slow_ma = self.get_state("current_slow_ma", 0)
            current_position = self.get_state("current_position", 0)
            price_history = self.get_state("price_history", [])
            
            # Increment decision point counter
            decision_count = self.get_state("decision_count", 0) + 1
            self.set_state("decision_count", decision_count)
            
            # Check if we have enough data for full analysis
            has_enough_data = len(price_history) >= self.slow_period
            
            if not has_enough_data:
                # Record decision for insufficient data case
                self.add_decision_timeline({
                    "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                    "rule_description": "Declarative Flow - Insufficient Data for Trading",
                    "result": False,
                    "context_values": {
                        "current_price": current_price,
                        "fast_ma": None,
                        "slow_ma": None,
                        "ma_difference": None,
                        "current_position": current_position,
                        "price_history_length": len(price_history),
                        "unrealized_pnl": 0,
                        "flow_executed": False
                    },
                    "parameters": {
                        "fast_period": self.fast_period,
                        "slow_period": self.slow_period,
                        "symbol": self.symbol,
                        "stop_loss_pct": self.stop_loss_pct,
                        "take_profit_pct": self.take_profit_pct
                    },
                    "evaluation_details": {
                        "data_status": "insufficient",
                        "price_points": len(price_history),
                        "required_points": self.slow_period,
                        "indicators_ready": False,
                        "can_trade": False,
                        "action_taken": "none",
                        "flow_engine": "declarative"
                    },
                    "check_number": decision_count
                })
            else:
                # We have enough data - execute the declarative flow
                await self.flow.execute(context)
                
                # Record comprehensive decision data
                fast_ma_history = self.get_state("fast_ma_history", [])
                slow_ma_history = self.get_state("slow_ma_history", [])
                
                # Analyze crossover conditions
                has_crossover_data = len(fast_ma_history) >= 2 and len(slow_ma_history) >= 2
                bullish_crossover = False
                crossover_analysis = {}
                
                if has_crossover_data:
                    prev_fast_ma = fast_ma_history[-2]
                    prev_slow_ma = slow_ma_history[-2]
                    
                    # True bullish crossover: was below, now above
                    bullish_crossover = (
                        prev_fast_ma <= prev_slow_ma and
                        current_fast_ma > current_slow_ma
                    )
                    
                    crossover_analysis = {
                        "has_crossover_data": True,
                        "previous_fast_ma": prev_fast_ma,
                        "previous_slow_ma": prev_slow_ma,
                        "current_fast_ma": current_fast_ma,
                        "current_slow_ma": current_slow_ma,
                        "was_fast_below_slow": prev_fast_ma <= prev_slow_ma,
                        "is_fast_above_slow": current_fast_ma > current_slow_ma,
                        "true_bullish_crossover": bullish_crossover,
                        "crossover_description": f"Fast MA: {prev_fast_ma:.3f} → {current_fast_ma:.3f}, Slow MA: {prev_slow_ma:.3f} → {current_slow_ma:.3f}"
                    }
                else:
                    crossover_analysis = {
                        "has_crossover_data": False,
                        "reason": "Need at least 2 MA values for crossover detection",
                        "ma_history_length": len(fast_ma_history),
                        "true_bullish_crossover": False
                    }
                
                # Determine what action was taken (if any)
                action_taken = "none"
                if bullish_crossover and current_position == 0:
                    action_taken = "entry_signal_detected"
                elif current_fast_ma < current_slow_ma and current_position > 0:
                    action_taken = "exit_signal_detected"
                
                self.add_decision_timeline({
                    "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                    "rule_description": "Declarative Flow - Complete Market Analysis",
                    "result": has_enough_data,
                    "context_values": {
                        "current_price": current_price,
                        "fast_ma": current_fast_ma,
                        "slow_ma": current_slow_ma,
                        "ma_difference": current_fast_ma - current_slow_ma,
                        "current_position": current_position,
                        "position_size_calculation": self.calculate_position_size(),
                        "flow_executed": True,
                        "unrealized_pnl": self._calculate_unrealized_pnl(current_price) if current_position != 0 else 0,
                        "crossover_analysis": crossover_analysis
                    },
                    "parameters": {
                        "fast_period": self.fast_period,
                        "slow_period": self.slow_period,
                        "symbol": self.symbol,
                        "stop_loss_pct": self.stop_loss_pct,
                        "take_profit_pct": self.take_profit_pct
                    },
                    "evaluation_details": {
                        "flow_type": "declarative",
                        "rules_evaluated": "graph_based",
                        "decision_state": {
                            "has_enough_data": has_enough_data,
                            "bullish_crossover": bullish_crossover,
                            "bearish_crossover": bool(current_fast_ma < current_slow_ma),
                            "in_position": bool(current_position > 0),
                            "risk_management_triggered": bool(self.check_risk_management(context))
                        },
                        "action_taken": action_taken,
                        "crossover_detection": crossover_analysis,
                        "flow_nodes": self.flow.get_node_count()
                    },
                    "check_number": decision_count
                })
                
        except Exception as e:
            self.log_error(f"Error recording cycle decision: {e}")
            import traceback
            self.log_error(f"Traceback: {traceback.format_exc()}")
    
    async def cleanup_strategy(self):
        """Cleanup method called when strategy stops"""
        try:
            # Call parent cleanup
            await super().cleanup_strategy()
        except Exception as e:
            self.log_error(f"Error during strategy cleanup: {e}")
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return strategy metadata"""
        # Get default values for metadata
        default_symbol = getattr(self, 'symbol', 'SPY')
        default_fast = getattr(self, 'fast_period', 10)
        default_slow = getattr(self, 'slow_period', 30)
        
        return {
            "name": "Declarative Moving Average Crossover Strategy",
            "description": f"Declarative flow-based MA crossover strategy ({default_fast}/{default_slow}) using Rules engine",
            "version": "3.0.0",
            "author": "Declarative Flow Engine",
            "risk_level": "HIGH",
            "max_positions": 1,
            "preferred_symbols": [default_symbol],
            "flow_engine": "declarative",
            "parameters": {
                "symbol": {
                    "type": "string",
                    "default": "SPY",
                    "description": "Trading symbol"
                },
                "fast_period": {
                    "type": "integer",
                    "default": 10,
                    "min": 2,
                    "max": 50,
                    "description": "Fast moving average period"
                },
                "slow_period": {
                    "type": "integer",
                    "default": 30,
                    "min": 10,
                    "max": 200,
                    "description": "Slow moving average period"
                },
                "stop_loss_pct": {
                    "type": "float",
                    "default": 2.0,
                    "min": 0.5,
                    "max": 10.0,
                    "description": "Stop loss percentage"
                },
                "take_profit_pct": {
                    "type": "float",
                    "default": 5.0,
                    "min": 1.0,
                    "max": 20.0,
                    "description": "Take profit percentage"
                },
                "allow_short": {
                    "type": "boolean",
                    "default": False,
                    "description": "Allow short positions"
                },
                "risk_per_trade_pct": {
                    "type": "float",
                    "default": 1.0,
                    "min": 0.1,
                    "max": 5.0,
                    "description": "Risk per trade as % of capital"
                }
            }
        }
    
    def get_flow_graph_data(self) -> Dict[str, Any]:
        """Get the declarative flow graph data for visualization"""
        return self.flow.to_graph_data()
