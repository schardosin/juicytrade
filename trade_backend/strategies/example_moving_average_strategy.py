"""
Example Moving Average Strategy - Action-Based Framework

This is a simple example strategy that demonstrates how to use the new action-based
BaseStrategy framework. It shows how to convert traditional indicator-based strategies
to use the powerful action system.

Strategy Description:
This strategy generates buy/sell signals based on moving average crossovers using
the action-based framework:
- Uses TimeAction to start monitoring at market open
- Framework automatically calls record_cycle_decision() for each data point
- Executes trades directly through order executor
- Includes proper state management and checkpoints

Key Features:
- Action-based execution flow
- State management with checkpoints
- Time-based triggers
- Condition monitoring
- Risk management integration
"""

import logging
from datetime import datetime, timedelta
from typing import Dict, Any, Optional, List

from .base_strategy import BaseStrategy
from .actions import ActionContext

logger = logging.getLogger(__name__)


class MovingAverageStrategy(BaseStrategy):
    """
    Moving Average Crossover Strategy using Action-Based Framework
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        super().__init__(strategy_id, data_provider, order_executor, config)
        self.symbol = None
        self.fast_period = None
        self.slow_period = None
        self.stop_loss_pct = None
        self.take_profit_pct = None
        self.log_info(f"MovingAverageStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        self.symbol = self.get_config_value("symbol", "SPY")
        self.fast_period = self.get_config_value("fast_period", 10)
        self.slow_period = self.get_config_value("slow_period", 30)
        self.stop_loss_pct = self.get_config_value("stop_loss_pct", 2.0)
        self.take_profit_pct = self.get_config_value("take_profit_pct", 5.0)
        
        self.set_state("symbol", self.symbol)
        self.set_state("fast_period", self.fast_period)
        self.set_state("slow_period", self.slow_period)
        self.set_state("price_history", [])
        self.set_state("fast_ma_history", [])
        self.set_state("slow_ma_history", [])
        self.set_state("current_position", 0)
        self.set_state("entry_price", None)
        self.set_state("last_crossover", None)

        
        self.add_time_action(
            trigger_time="09:30",
            callback=self.start_monitoring,
            name="wait_for_market_open"
        )
        
        self.log_info(f"Moving Average Strategy initialized for {self.symbol}")
        self.add_checkpoint("strategy_initialized", {
            "symbol": self.symbol,
            "fast_period": self.fast_period,
            "slow_period": self.slow_period
        })
    
    async def start_monitoring(self, context: ActionContext):
        self.log_info("Market opened - starting MA crossover monitoring")
        # Framework automatically calls record_cycle_decision() every execution cycle
        # No need for monitor action - decision logic is handled in record_cycle_decision
        self.add_checkpoint("monitoring_started")
    
    def update_indicators(self, context: ActionContext) -> bool:
        """Update price history and calculate moving averages"""
        try:
            # Get current price from data provider
            current_price = self.get_current_price_from_provider()
            if current_price is None:
                return False
            
            # Update price history - but only if this is a new timestamp
            price_history = self.get_state("price_history", [])
            last_timestamp = self.get_state("last_price_timestamp", None)
            current_timestamp = context.current_time
            
            # Only add price if this is a new timestamp (avoid duplicates from multiple actions)
            if last_timestamp != current_timestamp:
                price_history.append(current_price)
                self.set_state("last_price_timestamp", current_timestamp)
                
                # Keep only what we need
                max_history = max(self.fast_period, self.slow_period) + 10
                if len(price_history) > max_history:
                    price_history = price_history[-max_history:]
                
                self.set_state("price_history", price_history)
            else:
                # Price already added for this timestamp, just return success
                pass
            
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
    
    async def open_long_position(self, context: ActionContext, reason: str):
        """Open a long position"""
        try:
            # CRITICAL FIX: Check current position state BEFORE opening new position
            current_position = self.get_state("current_position", 0)
            if current_position != 0:
                self.log_error(f"🚨 POSITION VIOLATION: Attempting to open long position when current_position={current_position}. This should never happen!")
                self.log_error(f"🚨 POSITION VIOLATION: Reason: {reason}")
                self.log_error(f"🚨 POSITION VIOLATION: Timestamp: {context.current_time}")
                return  # ABORT - do not open multiple positions
            
            # Calculate position size
            position_size = self.calculate_position_size()
            
            if position_size <= 0:
                self.log_warning("Cannot open position - insufficient capital or invalid size")
                return
            
            # CRITICAL FIX: Directly execute trade through backtest engine
            if hasattr(self.order_executor, 'place_market_order'):
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=position_size,
                    side="BUY",
                    reason=reason
                )
                self.log_info(f"✅ TRADE EXECUTED: BUY {position_size} {self.symbol} - Trade ID: {trade_id}")
                
                # CRITICAL FIX: Only update strategy state if trade was actually executed
                if trade_id:  # Trade ID returned means trade was executed
                    current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
                    self.set_state("current_position", position_size)
                    self.set_state("entry_price", current_price)
                    self.set_state("position_type", "long")
                    
                    self.log_info(f"✅ POSITION STATE UPDATED: current_position={position_size}, entry_price=${current_price:.2f}")
                else:
                    self.log_error("Trade execution failed - position state not updated")
                    return
            else:
                self.log_error("Order executor does not support place_market_order")
                return
            
            self.log_info(f"Opening LONG position: {position_size} shares at ${current_price:.2f}")
            
        except Exception as e:
            self.log_error(f"Error opening long position: {e}")
    
    async def open_short_position(self, context: ActionContext, fast_ma: float, slow_ma: float):
        """Open a short position"""
        try:
            # Calculate position size
            position_size = self.calculate_position_size()
            
            if position_size <= 0:
                self.log_warning("Cannot open position - insufficient capital or invalid size")
                return
            
            # Add trade action to sell short
            self.add_trade_action(
                name="sell_short_position",
                trade_type="SELL_SHORT",
                symbol=self.symbol,
                quantity=position_size
            )
            
            # Update state
            current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
            self.set_state("current_position", -position_size)  # Negative for short
            self.set_state("entry_price", current_price)
            self.set_state("position_type", "short")
            
            self.log_info(f"Opening SHORT position: {position_size} shares at ${current_price:.2f}")
            
        except Exception as e:
            self.log_error(f"Error opening short position: {e}")
    
    async def close_position(self, context: ActionContext, reason: str):
        """Close current position - FIXED: Use direct execution like open_long_position"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position == 0:
                return  # No position to close
            
            # Determine trade action
            if current_position > 0:
                # Close long position
                side = "SELL"
                quantity = current_position
            else:
                # Close short position
                side = "BUY_TO_COVER"
                quantity = abs(current_position)
            
            # CRITICAL FIX: Use direct execution through backtest engine (same as open_long_position)
            if hasattr(self.order_executor, 'place_market_order'):
                trade_id = self.order_executor.place_market_order(
                    symbol=self.symbol,
                    quantity=quantity,
                    side=side,
                    reason=reason
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
                    self.log_info(f"Closing position: {reason}, P&L: ${pnl:.2f}")
                
                # CRITICAL FIX: Only reset position state if trade was actually executed
                if trade_id:  # Trade ID returned means trade was executed
                    self.set_state("current_position", 0)
                    self.set_state("entry_price", None)
                    self.set_state("position_type", None)
                    
                    self.log_info(f"✅ POSITION STATE RESET: current_position=0, position closed")
                    
                    self.add_checkpoint("position_closed", {
                        "reason": reason,
                        "pnl": pnl if 'pnl' in locals() else 0,
                        "timestamp": context.current_time.isoformat()
                    })
                else:
                    self.log_error("Exit trade execution failed - position state not updated")
            else:
                self.log_error("Order executor does not support place_market_order for exit trades")
            
        except Exception as e:
            self.log_error(f"Error closing position: {e}")
    
    def check_risk_management(self, context: ActionContext) -> bool:
        """Check if risk management rules are triggered"""
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

    def calculate_position_size(self) -> int:
        """Calculate position size based on available capital"""
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
            
            # FIXED: Simple position sizing - use percentage of available capital
            # Use 95% of available capital to leave room for commissions and slippage
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
        """Calculate simple moving average"""
        if len(price_history) < period:
            return price_history[-1] if price_history else 0
        
        return sum(price_history[-period:]) / period
    
    def _calculate_unrealized_pnl(self, current_price: float) -> float:
        """Calculate unrealized P&L for current position"""
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
    
    def get_current_price_from_provider(self) -> Optional[float]:
        """Get current price from the data provider (backtest engine)"""
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
        
    async def record_cycle_decision(self, context: ActionContext):
        """
        Record decision for this execution cycle - called automatically by framework.
        
        REVERTED: This method now executes trades again so we can debug the multiple position issue.
        """
        try:
            # First, update indicators with current price data
            # This ensures we have fresh MA calculations for each data point
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
                    "rule_description": "Market Data Analysis - Insufficient Data for Trading",
                    "result": False,
                    "context_values": {
                        "current_price": current_price,
                        "fast_ma": None,
                        "slow_ma": None,
                        "ma_difference": None,
                        "current_position": current_position,
                        "price_history_length": len(price_history),
                        "unrealized_pnl": 0,
                        "trade_executed": False
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
                        "action_taken": "none"
                    },
                    "check_number": decision_count
                })
            else:
                # We have enough data, proceed with decision evaluation AND execution
                # Check entry conditions directly - FIXED: Proper crossover detection
                # Get previous MA values for crossover detection
                fast_ma_history = self.get_state("fast_ma_history", [])
                slow_ma_history = self.get_state("slow_ma_history", [])
                
                # We need at least 2 MA values to detect a crossover
                has_crossover_data = len(fast_ma_history) >= 2 and len(slow_ma_history) >= 2
                
                bullish_crossover = False
                if has_crossover_data:
                    prev_fast_ma = fast_ma_history[-2]  # Previous fast MA
                    prev_slow_ma = slow_ma_history[-2]  # Previous slow MA
                    
                    # True bullish crossover: was below, now above
                    bullish_crossover = (
                        prev_fast_ma <= prev_slow_ma and  # Was below or equal
                        current_fast_ma > current_slow_ma  # Now above
                    )
                
                is_entry_signal = (
                    bullish_crossover and  # TRUE crossover (not just above)
                    current_position == 0 and  # Not in position
                    self.calculate_position_size() > 0  # Sufficient capital
                )
                
                # REVERTED: Actually execute the trade if signal is TRUE (for debugging)
                trade_executed = False
                action_taken = "none"
                
                if is_entry_signal:
                    try:
                        # DEBUG: Log detailed state before trade execution
                        await self.open_long_position(context, "Entry signal detected - Fast MA above Slow MA")
                        trade_executed = True
                        action_taken = "open_long"
                        self.log_info(f"🚀 TRADE EXECUTED: Entry signal at ${current_price:.2f}")
                    except Exception as e:
                        self.log_error(f"Failed to execute entry trade: {e}")
                
                # Add detailed decision timeline entry for entry evaluation
                # Include crossover analysis details
                crossover_analysis = {}
                if has_crossover_data:
                    prev_fast_ma = fast_ma_history[-2]
                    prev_slow_ma = slow_ma_history[-2]
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
                
                self.add_decision_timeline({
                    "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                    "rule_description": "Entry Decision Chain Evaluation - Crossover Analysis",
                    "result": is_entry_signal,
                    "context_values": {
                        "current_price": current_price,
                        "fast_ma": current_fast_ma,
                        "slow_ma": current_slow_ma,
                        "ma_difference": current_fast_ma - current_slow_ma,
                        "current_position": current_position,
                        "position_size_calculation": self.calculate_position_size(),
                        "trade_executed": trade_executed,
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
                        "chain_type": "entry",
                        "rules_evaluated": 3,  # bullish_crossover, not_in_position, sufficient_capital
                        "decision_state": {
                            "bullish_crossover": bullish_crossover,  # TRUE crossover, not just above
                            "not_in_position": bool(current_position == 0),
                            "sufficient_capital": bool(self.calculate_position_size() > 0)
                        },
                        "action_taken": action_taken,
                        "crossover_detection": crossover_analysis
                    },
                    "check_number": decision_count
                })

                # If no entry signal, check for exit signal
                if not is_entry_signal:
                    # Check exit conditions directly - FIXED: Correct risk management logic
                    # Exit if: bearish crossover AND in position AND (risk management triggered OR normal bearish crossover)
                    risk_management_triggered = self.check_risk_management(context)
                    bearish_crossover = current_fast_ma < current_slow_ma
                    
                    is_exit_signal = (
                        bearish_crossover and  # Bearish crossover condition
                        current_position > 0  # In long position
                        # FIXED: Exit on bearish crossover OR risk management (removed the inverted logic)
                    )
                    
                    # Execute the exit trade if signal is TRUE
                    if is_exit_signal:
                        try:
                            await self.close_position(context, "Exit signal detected - Fast MA below Slow MA")
                            trade_executed = True
                            action_taken = "close_long"
                            self.log_info(f"🛑 TRADE EXECUTED: Exit signal at ${current_price:.2f}")
                        except Exception as e:
                            self.log_error(f"Failed to execute exit trade: {e}")
                    
                    # Add detailed decision timeline entry for exit evaluation
                    self.add_decision_timeline({
                        "timestamp": context.current_time.isoformat() if context.current_time else datetime.now().isoformat(),
                        "rule_description": "Exit Decision Chain Evaluation",
                        "result": is_exit_signal,
                        "context_values": {
                            "current_price": current_price,
                            "fast_ma": current_fast_ma,
                            "slow_ma": current_slow_ma,
                            "ma_difference": current_fast_ma - current_slow_ma,
                            "current_position": current_position,
                            "entry_price": self.get_state("entry_price"),
                            "unrealized_pnl": self._calculate_unrealized_pnl(current_price) if current_position != 0 else 0,
                            "trade_executed": trade_executed
                        },
                        "parameters": {
                            "fast_period": self.fast_period,
                            "slow_period": self.slow_period,
                            "symbol": self.symbol,
                            "stop_loss_pct": self.stop_loss_pct,
                            "take_profit_pct": self.take_profit_pct
                        },
                        "evaluation_details": {
                            "chain_type": "exit",
                            "rules_evaluated": 3,  # bearish_crossover, in_long_position, risk_management_check
                            "decision_state": {
                                "bearish_crossover": bool(current_fast_ma < current_slow_ma),
                                "in_long_position": bool(current_position > 0),
                                "risk_management_check": bool(not self.check_risk_management(context))
                            },
                            "risk_management_triggered": bool(self.check_risk_management(context)),
                            "action_taken": action_taken
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
        # Get default values for metadata (don't depend on initialized attributes)
        default_symbol = getattr(self, 'symbol', 'SPY')
        default_fast = getattr(self, 'fast_period', 10)
        default_slow = getattr(self, 'slow_period', 30)
        
        return {
            "name": "Moving Average Crossover Strategy",
            "description": f"Action-based MA crossover strategy ({default_fast}/{default_slow})",
            "version": "2.0.0",
            "author": "Action Framework Example",
            "risk_level": "HIGH",
            "max_positions": 1,
            "preferred_symbols": [default_symbol],
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
