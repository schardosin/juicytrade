"""
Example Moving Average Strategy - Action-Based Framework

This is a simple example strategy that demonstrates how to use the new action-based
BaseStrategy framework. It shows how to convert traditional indicator-based strategies
to use the powerful action system.

Strategy Description:
This strategy generates buy/sell signals based on moving average crossovers using
the action-based framework:
- Uses TimeAction to start monitoring at market open
- Uses MonitorAction to watch for crossover conditions
- Uses TradeAction to execute trades
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
    
    This strategy demonstrates how to implement traditional technical analysis
    strategies using the new action-based system:
    
    1. Wait for market open (TimeAction)
    2. Monitor for MA crossover conditions (MonitorAction)
    3. Execute trades when conditions are met (TradeAction)
    4. Manage risk with stop-loss and take-profit (MonitorAction)
    5. Handle position management and state tracking
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        """
        Initialize the Moving Average Strategy.
        
        Args:
            strategy_id: Unique identifier for this strategy instance
            data_provider: Data provider for market data
            order_executor: Order executor for trade execution
            config: Strategy configuration parameters
        """
        super().__init__(strategy_id, data_provider, order_executor, config)
        
        # Initialize strategy-specific attributes
        self.symbol = None
        self.fast_period = None
        self.slow_period = None
        self.stop_loss_pct = None
        self.take_profit_pct = None
        
        self.log_info(f"MovingAverageStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Initialize the moving average strategy with actions"""
        
        # Get strategy parameters from config
        self.symbol = self.get_config_value("symbol", "SPY")
        self.fast_period = self.get_config_value("fast_period", 10)
        self.slow_period = self.get_config_value("slow_period", 30)
        self.stop_loss_pct = self.get_config_value("stop_loss_pct", 2.0)
        self.take_profit_pct = self.get_config_value("take_profit_pct", 5.0)
        
        # Initialize strategy state
        self.set_state("symbol", self.symbol)
        self.set_state("fast_period", self.fast_period)
        self.set_state("slow_period", self.slow_period)
        self.set_state("price_history", [])
        self.set_state("fast_ma_history", [])
        self.set_state("slow_ma_history", [])
        self.set_state("current_position", 0)
        self.set_state("entry_price", None)
        self.set_state("last_crossover", None)
        
        # Action 1: Wait for market open to start monitoring
        self.add_time_action(
            trigger_time="09:30",  # Market open
            callback=self.start_monitoring,
            name="wait_for_market_open"
        )
        
        self.log_info(f"Moving Average Strategy initialized for {self.symbol}")
        self.log_info(f"Parameters: Fast={self.fast_period}, Slow={self.slow_period}")
        
        self.add_checkpoint("strategy_initialized", {
            "symbol": self.symbol,
            "fast_period": self.fast_period,
            "slow_period": self.slow_period
        })
    
    async def start_monitoring(self, context: ActionContext):
        """Start monitoring for moving average crossovers"""
        self.log_info("Market opened - starting MA crossover monitoring")
        
        # Action 2: Monitor for price updates and calculate MAs
        self.add_monitor_action(
            name="price_monitor",
            condition=lambda ctx: self.update_indicators(ctx),
            callback=self.check_crossover_signals,
            continuous=True
        )
        
        # Action 3: Monitor existing positions for risk management
        self.add_monitor_action(
            name="risk_monitor",
            condition=lambda ctx: self.check_risk_management(ctx),
            callback=self.handle_risk_exit,
            continuous=True
        )
        
        self.add_checkpoint("monitoring_started")
    
    def update_indicators(self, context: ActionContext) -> bool:
        """Update price history and calculate moving averages"""
        try:
            # Get current price (mock implementation)
            current_price = self.get_mock_price(context)
            if current_price is None:
                return False
            
            # Update price history
            price_history = self.get_state("price_history", [])
            price_history.append(current_price)
            
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
    
    async def check_crossover_signals(self, context: ActionContext):
        """Check for moving average crossover signals"""
        try:
            fast_ma_history = self.get_state("fast_ma_history", [])
            slow_ma_history = self.get_state("slow_ma_history", [])
            
            if len(fast_ma_history) < 2 or len(slow_ma_history) < 2:
                return  # Need at least 2 points for crossover
            
            # Current and previous values
            fast_ma = fast_ma_history[-1]
            slow_ma = slow_ma_history[-1]
            prev_fast_ma = fast_ma_history[-2]
            prev_slow_ma = slow_ma_history[-2]
            
            current_position = self.get_state("current_position", 0)
            
            # Check for bullish crossover (buy signal)
            if (prev_fast_ma <= prev_slow_ma and fast_ma > slow_ma and current_position <= 0):
                self.log_info(f"BULLISH CROSSOVER: Fast MA ({fast_ma:.2f}) crossed above Slow MA ({slow_ma:.2f})")
                
                # Close short position if we have one
                if current_position < 0:
                    await self.close_position(context, "Cover short on bullish crossover")
                
                # Open long position
                await self.open_long_position(context, fast_ma, slow_ma)
                
                self.set_state("last_crossover", "bullish")
                self.add_checkpoint("bullish_crossover", {
                    "fast_ma": fast_ma,
                    "slow_ma": slow_ma,
                    "timestamp": context.current_time.isoformat()
                })
            
            # Check for bearish crossover (sell signal)
            elif (prev_fast_ma >= prev_slow_ma and fast_ma < slow_ma and current_position >= 0):
                self.log_info(f"BEARISH CROSSOVER: Fast MA ({fast_ma:.2f}) crossed below Slow MA ({slow_ma:.2f})")
                
                # Close long position if we have one
                if current_position > 0:
                    await self.close_position(context, "Close long on bearish crossover")
                
                # Open short position (optional - can be disabled)
                if self.get_config_value("allow_short", False):
                    await self.open_short_position(context, fast_ma, slow_ma)
                
                self.set_state("last_crossover", "bearish")
                self.add_checkpoint("bearish_crossover", {
                    "fast_ma": fast_ma,
                    "slow_ma": slow_ma,
                    "timestamp": context.current_time.isoformat()
                })
                
        except Exception as e:
            self.log_error(f"Error checking crossover signals: {e}")
    
    async def open_long_position(self, context: ActionContext, fast_ma: float, slow_ma: float):
        """Open a long position"""
        try:
            # Calculate position size
            position_size = self.calculate_position_size()
            
            if position_size <= 0:
                self.log_warning("Cannot open position - insufficient capital or invalid size")
                return
            
            # Add trade action to buy
            self.add_trade_action(
                name="buy_long_position",
                trade_type="BUY",
                symbol=self.symbol,
                quantity=position_size
            )
            
            # Update state
            current_price = self.get_state("price_history", [])[-1] if self.get_state("price_history") else 0
            self.set_state("current_position", position_size)
            self.set_state("entry_price", current_price)
            self.set_state("position_type", "long")
            
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
        """Close current position"""
        try:
            current_position = self.get_state("current_position", 0)
            if current_position == 0:
                return  # No position to close
            
            # Determine trade action
            if current_position > 0:
                # Close long position
                trade_type = "SELL"
                quantity = current_position
            else:
                # Close short position
                trade_type = "BUY_TO_COVER"
                quantity = abs(current_position)
            
            # Add trade action
            self.add_trade_action(
                name="close_position",
                trade_type=trade_type,
                symbol=self.symbol,
                quantity=quantity
            )
            
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
            
            # Reset position state
            self.set_state("current_position", 0)
            self.set_state("entry_price", None)
            self.set_state("position_type", None)
            
            self.add_checkpoint("position_closed", {
                "reason": reason,
                "pnl": pnl if 'pnl' in locals() else 0,
                "timestamp": context.current_time.isoformat()
            })
            
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
    
    async def handle_risk_exit(self, context: ActionContext):
        """Handle risk management exit"""
        exit_reason = self.get_state("exit_reason", "Risk management exit")
        await self.close_position(context, exit_reason)
    
    def calculate_position_size(self) -> int:
        """Calculate position size based on risk management"""
        try:
            # Simple position sizing - could be enhanced
            account_balance = self.get_config_value("account_balance", 10000)
            risk_per_trade = self.get_config_value("risk_per_trade_pct", 1.0) / 100
            
            price_history = self.get_state("price_history", [])
            if not price_history:
                return 0
            
            current_price = price_history[-1]
            risk_amount = account_balance * risk_per_trade
            stop_distance = current_price * (self.stop_loss_pct / 100)
            
            if stop_distance <= 0:
                return 0
            
            position_size = int(risk_amount / stop_distance)
            
            # Apply limits
            min_size = self.get_config_value("min_position_size", 10)
            max_size = self.get_config_value("max_position_size", 1000)
            
            return max(min_size, min(position_size, max_size))
            
        except Exception as e:
            self.log_error(f"Error calculating position size: {e}")
            return 0
    
    def calculate_ma(self, price_history: List[float], period: int) -> float:
        """Calculate simple moving average"""
        if len(price_history) < period:
            return price_history[-1] if price_history else 0
        
        return sum(price_history[-period:]) / period
    
    def get_mock_price(self, context: ActionContext) -> Optional[float]:
        """Get mock price data for testing (replace with real data provider)"""
        try:
            # Mock price generation for testing
            import random
            base_price = 100.0
            
            # Get previous price for continuity
            price_history = self.get_state("price_history", [])
            if price_history:
                base_price = price_history[-1]
            
            # Add some random movement
            change_pct = random.uniform(-0.01, 0.01)  # ±1% change
            new_price = base_price * (1 + change_pct)
            
            return round(new_price, 2)
            
        except Exception as e:
            self.log_error(f"Error getting mock price: {e}")
            return None
    
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
            "risk_level": "MEDIUM",
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


# ============================================================================
# Alternative Simple Moving Average Strategy (Minimal Example)
# ============================================================================

class SimpleMovingAverageStrategy(BaseStrategy):
    """
    Simplified version of the MA strategy for demonstration purposes.
    Shows the minimal implementation needed for an action-based strategy.
    """
    
    def __init__(self, strategy_id: str, data_provider, order_executor, config: Dict[str, Any]):
        """
        Initialize the Simple Moving Average Strategy.
        
        Args:
            strategy_id: Unique identifier for this strategy instance
            data_provider: Data provider for market data
            order_executor: Order executor for trade execution
            config: Strategy configuration parameters
        """
        super().__init__(strategy_id, data_provider, order_executor, config)
        
        # Initialize strategy-specific attributes
        self.symbol = None
        
        self.log_info(f"SimpleMovingAverageStrategy instance created with ID: {strategy_id}")
    
    async def initialize_strategy(self):
        """Minimal strategy initialization"""
        self.symbol = self.get_config_value("symbol", "SPY")
        
        # Simple action: Buy at market open, sell at market close
        self.add_time_action("09:30", self.buy_signal, "market_open_buy")
        self.add_time_action("15:30", self.sell_signal, "market_close_sell")
        
        self.log_info(f"Simple MA Strategy initialized for {self.symbol}")
    
    async def buy_signal(self, context: ActionContext):
        """Simple buy signal"""
        self.add_trade_action(
            name="daily_buy",
            trade_type="BUY",
            symbol=self.symbol,
            quantity=100
        )
        self.log_info("Executed daily buy signal")
    
    async def sell_signal(self, context: ActionContext):
        """Simple sell signal"""
        self.add_trade_action(
            name="daily_sell",
            trade_type="SELL",
            symbol=self.symbol,
            quantity=100
        )
        self.log_info("Executed daily sell signal")
    
    def get_strategy_metadata(self) -> Dict[str, Any]:
        """Return simple strategy metadata"""
        return {
            "name": "Simple Moving Average Strategy",
            "description": "Minimal example: Buy at open, sell at close",
            "version": "1.0.0",
            "author": "Action Framework",
            "risk_level": "LOW",
            "max_positions": 1,
            "preferred_symbols": [self.symbol],
            "parameters": {
                "symbol": {
                    "type": "string",
                    "default": "SPY",
                    "description": "Trading symbol"
                }
            }
        }
