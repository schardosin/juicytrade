"""
Example Moving Average Crossover Strategy

This is a complete example of how to implement a trading strategy using the BaseStrategy framework.
Users can copy this structure and modify the trading logic for their own strategies.

Strategy Description:
This strategy generates buy/sell signals based on moving average crossovers.
- Enter long when fast MA crosses above slow MA
- Exit long when fast MA crosses below slow MA
- Includes trend confirmation and position management

Key Features:
- Technical indicator calculation
- Risk management
- Rule-based decision making
- UI feedback for rule evaluation
- Position sizing based on strategy parameters
"""

from typing import Dict, List, Any, Optional
from datetime import datetime
from .base_strategy import BaseStrategy, StrategyResult, MarketData, StrategyMetadata


class MovingAverageCrossoverStrategy(BaseStrategy):
    """
    Moving Average Crossover Strategy Implementation

    This strategy demonstrates:
    1. Complex rule evaluation
    2. Multiple trading signals
    3. Technical indicator calculation
    4. Position management
    5. Risk controls
    6. UI feedback for rule states
    """

    def __init__(self, data_provider=None, order_executor=None, config: Dict[str, Any] = None):
        super().__init__(data_provider, order_executor, config)

        # Strategy-specific state
        self.price_history = []
        self.fast_ma_history = []
        self.slow_ma_history = []
        self.current_position = None
        self.entry_price = None

        # Strategy parameters from configuration
        self.fast_period = self.config.get('fast_period', 5)
        self.slow_period = self.config.get('slow_period', 20)
        self.stop_loss_pct = self.config.get('stop_loss_pct', 2.0)
        self.take_profit_pct = self.config.get('take_profit_pct', 5.0)
        self.risk_per_trade_pct = self.config.get('risk_per_trade_pct', 1.0)

    def initialize(self) -> None:
        """
        Strategy initialization - called once when strategy starts.

        This is where to:
        - Subscribe to required symbols
        - Initialize strategy-specific state
        - Load historical data if needed
        - Set up any required indicators
        """
        # Get symbol from configuration
        self.symbol = self.config.get('symbol', 'AAPL')

        # Subscribe to real-time market data
        self.subscribe_symbol(self.symbol)

        # Set strategy metadata for UI display
        self.metadata.update({
            'name': 'Moving Average Crossover Strategy',
            'description': 'Trades based on fast/slow moving average crossovers',
            'rules': [
                {'id': 'ma_crossover', 'description': 'Wait for fast MA to cross above slow MA'},
                {'id': 'trend_confirmation', 'description': 'Require upward trend confirmation'},
                {'id': 'volume_filter', 'description': 'Apply volume-based filtering'},
                {'id': 'position_size', 'description': 'Calculate appropriate position size'},
                {'id': 'risk_management', 'description': 'Apply stop-loss and take-profit rules'},
                {'id': 'exit_signal', 'description': 'Generate exit signals when conditions met'}
            ]
        })

        # Load historical data for initial indicator calculation
        try:
            historical_data = self.data_provider.get_recent_candles(self.symbol, limit=self.slow_period + 10)
            if historical_data:
                # Initialize price history with recent closes
                self.price_history = [candle['close'] for candle in historical_data if isinstance(candle, dict)]
                self.log_info(f"Loaded {len(self.price_history)} historical prices for indicator calculation")
        except Exception as e:
            self.log_warning(f"Could not load historical data: {e}")

    def on_market_data(self, symbol: str, data: MarketData) -> Optional[StrategyResult]:
        """
        Main market data processing function.

        This is called whenever new market data arrives for subscribed symbols.
        Contains the core trading logic and rule evaluation.

        Args:
            symbol: The symbol this data is for
            data: MarketData object with current market information

        Returns:
            StrategyResult if strategy wants to execute a trade, None otherwise
        """
        try:
            if symbol != self.symbol:
                return None  # Ignore data for other symbols

            # Update internal price history
            self.price_history.append(data.price)
            if len(self.price_history) > self.slow_period * 2:
                self.price_history = self.price_history[-self.slow_period * 2:]

            # Not enough data yet
            if len(self.price_history) < self.slow_period:
                self.log_info(f"Collecting price data... {len(self.price_history)}/{self.slow_period}")
                return None

            # Calculate technical indicators
            fast_ma = self._calculate_moving_average(self.price_history, self.fast_period)
            slow_ma = self._calculate_moving_average(self.price_history, self.slow_period)

            # Store for visualization
            self.fast_ma_history.append(fast_ma)
            self.slow_ma_history.append(slow_ma)

            # Keep reasonable history length
            if len(self.fast_ma_history) > 100:
                self.fast_ma_history = self.fast_ma_history[-100:]
                self.slow_ma_history = self.slow_ma_history[-100:]

            # Evaluate trading rules
            signal = self._evaluate_trading_rules(
                symbol, data, fast_ma, slow_ma
            )

            if signal:
                self.log_info(f"Generated signal: {signal.action} {signal.symbol} x{signal.quantity}")

            return signal

        except Exception as e:
            self.log_error(f"Error processing market data: {e}")
            return None

    def on_position_update(self, symbol: str, position_data: Dict[str, Any]) -> None:
        """
        Handle position updates from broker.

        Called when position changes occur (fills, partial fills, etc.)

        Args:
            symbol: Position symbol
            position_data: Updated position information
        """
        super().on_position_update(symbol, position_data)

        # Update strategy-specific position tracking
        quantity = position_data.get('quantity', 0)
        price = position_data.get('avg_price', 0)

        if quantity != 0:
            self.current_position = quantity
            if self.entry_price is None:
                self.entry_price = price
        else:
            # Position closed
            self.current_position = None
            self.entry_price = None

        self.log_info(f"Position update: {symbol} = {quantity} shares @ {price}")

    def on_order_status(self, order_id: str, status: str, details: Dict[str, Any]) -> None:
        """
        Handle order status updates.

        Called when order status changes.

        Args:
            order_id: Order identifier
            status: New order status
            details: Additional order details
        """
        super().on_order_status(order_id, status, details)

        if status == 'FILLED':
            # Update position tracking
            side = details.get('side', '').lower()
            quantity = details.get('quantity', 0)
            price = details.get('avg_fill_price', 0)

            if side == 'buy' and not self.current_position:
                self.current_position = quantity
                self.entry_price = price
                self.log_info(f"Position opened: {quantity} shares @ {price}")
            elif side == 'sell' and self.current_position:
                self.current_position = 0
                self.entry_price = None
                pnl = (price - self.entry_price) * quantity if self.entry_price else 0
                self.log_info(f"Position closed: {quantity} shares @ {price}, P&L: ${pnl:.2f}")

    def get_strategy_metadata(self) -> StrategyMetadata:
        """
        Return comprehensive strategy metadata for UI display and monitoring.

        This information is used by the trading platform to:
        - Display strategy information in UI
        - Show configuration options
        - Report on strategy performance and rule states
        - Validate strategy parameters
        """
        return StrategyMetadata(
            name=f"Moving Average Crossover ({self.fast_period}/{self.slow_period})",
            description=f"Trades based on {self.fast_period}-period MA crossing above/below {self.slow_period}-period MA",
            author="Strategy Framework Example",
            version="1.0.0",
            risk_level="MEDIUM",
            max_positions=1,
            max_daily_loss_pct=25.0,  # Stop strategy if daily loss exceeds 25%
            position_size_pct=self.risk_per_trade_pct,
            preferred_symbols=[self.symbol],
            asset_classes=["equity"],
            rules=[
                {'id': 'ma_crossover', 'description': 'Fast MA crosses above/below slow MA'},
                {'id': 'trend_confirmation', 'description': 'Upward/downward trend confirmation'},
                {'id': 'volume_filter', 'description': 'Minimum volume requirement'},
                {'id': 'position_size', 'description': 'Risk-based position sizing'},
                {'id': 'risk_management', 'description': 'Stop-loss and take-profit rules'},
                {'id': 'exit_signal', 'description': 'Exit position when conditions met'}
            ],
            parameters={
                'fast_period': {
                    'type': 'integer',
                    'default': 5,
                    'min': 2,
                    'max': 20,
                    'description': 'Fast moving average period'
                },
                'slow_period': {
                    'type': 'integer',
                    'default': 20,
                    'min': 10,
                    'max': 50,
                    'description': 'Slow moving average period'
                },
                'stop_loss_pct': {
                    'type': 'float',
                    'default': 2.0,
                    'min': 0.5,
                    'max': 10.0,
                    'description': 'Stop loss percentage'
                },
                'take_profit_pct': {
                    'type': 'float',
                    'default': 5.0,
                    'min': 1.0,
                    'max': 20.0,
                    'description': 'Take profit percentage'
                },
                'risk_per_trade_pct': {
                    'type': 'float',
                    'default': 1.0,
                    'min': 0.1,
                    'max': 5.0,
                    'description': 'Risk per trade as % of capital'
                },
                'enable_volume_filter': {
                    'type': 'boolean',
                    'default': True,
                    'description': 'Apply minimum volume filter'
                }
            }
        )

    # ============================================================================
    # Private helper methods
    # ============================================================================

    def _calculate_moving_average(self, price_history: List[float], period: int) -> float:
        """
        Calculate simple moving average.

        Args:
            price_history: List of recent prices
            period: Number of periods for the moving average

        Returns:
            Calculated moving average
        """
        if len(price_history) < period:
            return price_history[-1] if price_history else 0

        return sum(price_history[-period:]) / period

    def _evaluate_trading_rules(self, symbol: str, data: MarketData,
                               fast_ma: float, slow_ma: float) -> Optional[StrategyResult]:
        """
        Evaluate all trading rules and determine if a trade should be executed.

        Args:
            symbol: Trading symbol
            data: Current market data
            fast_ma: Fast moving average
            slow_ma: Slow moving average

        Returns:
            StrategyResult if trade should be executed, None otherwise
        """
        current_price = data.price
        current_volume = data.volume

        # =====================================================================
        # Rule 1: Moving Average Crossover Check
        # =====================================================================
        crossover_signal = self._check_ma_crossover(fast_ma, slow_ma)

        if crossover_signal['signal'] == 'BUY':
            self.update_rule_state('ma_crossover', True, f"Fast MA ({fast_ma:.2f}) > Slow MA ({slow_ma:.2f})")
        elif crossover_signal['signal'] == 'SELL':
            self.update_rule_state('ma_crossover', True, f"Fast MA ({fast_ma:.2f}) < Slow MA ({slow_ma:.2f})")

        if not crossover_signal['signal']:
            self.update_rule_state('ma_crossover', False, "Waiting for crossover")
            self.update_rule_state('trend_confirmation', False)
            return None

        # =====================================================================
        # Rule 2: Trend Confirmation
        # =====================================================================
        trend_confirmed = self._confirm_trend(crossover_signal['signal'])

        if not trend_confirmed:
            self.update_rule_state('trend_confirmation', False, "Trend not confirmed")
            return None

        self.update_rule_state('trend_confirmation', True, "Trend confirmed")

        # =====================================================================
        # Rule 3: Volume Filter (if enabled)
        # =====================================================================
        if self.config.get('enable_volume_filter', True):
            if not self._check_volume_filter(current_volume, data):
                self.update_rule_state('volume_filter', False, f"Volume too low: {current_volume}")
                return None

        self.update_rule_state('volume_filter', True, "Volume filter passed")

        # =====================================================================
        # Rule 4: Position Size Calculation
        # =====================================================================
        position_quantity = self.calculate_position_size(
            symbol,
            self.risk_per_trade_pct * self.config.get('account_balance', 10000)  # Assume $10k default
        )

        if position_quantity <= 0:
            self.update_rule_state('position_size', False, "Insufficient capital")
            return None

        self.update_rule_state('position_size', True, f"Position size: {position_quantity} shares")

        # =====================================================================
        # Rule 5: Risk Management - Exit signals if we have a position
        # =====================================================================
        exit_signal = self._check_risk_management(current_price)
        if exit_signal:
            self.update_rule_state('risk_management', True, exit_signal['reason'])

            return StrategyResult(
                action='CLOSE_POSITION',
                symbol=symbol,
                quantity=abs(self.current_position),  # Close entire position
                reason=f"Exit signal: {exit_signal['reason']}"
            )

        # =====================================================================
        # Rule 6: Generate Entry Signal
        # =====================================================================
        signal = crossover_signal['signal']

        # Reverse signal if we're closing a short position (simplified logic)
        if self.current_position and ((signal == 'BUY' and self.current_position < 0) or
                                    (signal == 'SELL' and self.current_position > 0)):
            reason = f"Exit {signal} signal for existing position"
            self.update_rule_state('exit_signal', True, reason)
        else:
            # Entry signal for new position
            reason = f"Entry {signal} signal on MA crossover"
            self.update_rule_state('exit_signal', False)

        return StrategyResult(
            action=signal,
            symbol=symbol,
            quantity=position_quantity,
            order_type='MARKET',
            reason=reason,
            rule_id='ma_crossover'
        )

    def _check_ma_crossover(self, fast_ma: float, slow_ma: float) -> Dict[str, str]:
        """
        Check for moving average crossover signal.

        Args:
            fast_ma: Fast moving average
            slow_ma: Slow moving average

        Returns:
            Dictionary with crossover signal information
        """
        if len(self.fast_ma_history) < 2 or len(self.slow_ma_history) < 2:
            return {'signal': None, 'description': 'Not enough data', 'strength': 0}

        # Get previous values
        prev_fast = self.fast_ma_history[-2]
        prev_slow = self.slow_ma_history[-2]

        # Determine crossover type and strength
        if fast_ma > slow_ma and prev_fast <= prev_slow:
            # Bullish crossover
            strength = self._calculate_crossover_strength(fast_ma, slow_ma, 'bullish')
            return {'signal': 'BUY', 'description': 'Bullish crossover', 'strength': strength}
        elif fast_ma < slow_ma and prev_fast >= prev_slow:
            # Bearish crossover
            strength = self._calculate_crossover_strength(fast_ma, slow_ma, 'bearish')
            return {'signal': 'SELL', 'description': 'Bearish crossover', 'strength': strength}

        return {'signal': None, 'description': 'No crossover', 'strength': 0}

    def _calculate_crossover_strength(self, fast_ma: float, slow_ma: float, direction: str) -> float:
        """
        Calculate the strength of a crossover signal.

        Args:
            fast_ma: Fast moving average
            slow_ma: Slow moving average
            direction: 'bullish' or 'bearish'

        Returns:
            Crossover strength (0.0 to 1.0)
        """
        # Simple strength calculation based on MA difference
        diff_pct = abs(fast_ma - slow_ma) / slow_ma

        # Strength increases with percentage difference, capped at 100%
        strength = min(diff_pct * 100, 1.0)  # Convert to fraction

        # Adjust based on trend direction consistency
        if len(self.fast_ma_history) >= 5:
            # Check if the last 5 MAs are consistently trending in the signal direction
            fast_trend = (self.fast_ma_history[-1] > self.fast_ma_history[-5]) if direction == 'bullish' else (self.fast_ma_history[-1] < self.fast_ma_history[-5])
            slow_trend = (self.slow_ma_history[-1] > self.slow_ma_history[-5]) if direction == 'bullish' else (self.slow_ma_history[-1] < self.slow_ma_history[-5])

            if fast_trend and slow_trend:
                strength *= 1.2  # Boost strength for consistent trend
            else:
                strength *= 0.8  # Reduce strength for conflicting trend

        return strength

    def _confirm_trend(self, signal: str) -> bool:
        """
        Confirm that the crossover aligns with the broader trend.

        Args:
            signal: 'BUY' or 'SELL'

        Returns:
            True if trend is confirmed, False otherwise
        """
        if len(self.price_history) < 10:
            return False  # Not enough data

        # Calculate trend slope over last 10 periods
        recent_prices = self.price_history[-10:]
        if len(recent_prices) < 2:
            return False

        # Simple linear regression slope approximation
        n = len(recent_prices)
        x = list(range(n))

        slope = sum((x_i - sum(x)/n) * (y_i - sum(recent_prices)/n) for x_i, y_i in zip(x, recent_prices))
        slope /= sum((x_i - sum(x)/n) ** 2 for x_i in x)

        # For BUY signals, we want positive slope
        # For SELL signals, we want negative slope
        expected_sign = 1 if signal == 'BUY' else -1

        return slope * expected_sign > 0

    def _check_volume_filter(self, volume: float, data: MarketData) -> bool:
        """
        Apply volume filtering to avoid low liquidity trades.

        Args:
            volume: Current volume
            data: Market data

        Returns:
            True if volume filter passes, False otherwise
        """
        # Get minimum volume from configuration
        min_volume = self.config.get('min_volume', 1000)

        # Basic volume check
        if volume < min_volume:
            return False

        # Additional liquidity checks
        spread = (data.ask - data.bid) / data.bid
        max_spread_pct = self.config.get('max_spread_pct', 0.01)  # 1% maximum spread

        if spread > max_spread_pct:
            self.log_warning(f"Wide spread detected: {spread * 100:.2f}%")
            return False

        return True

    def _check_risk_management(self, current_price: float) -> Optional[Dict[str, str]]:
        """
        Check for risk management signals (stop-loss, take-profit).

        Args:
            current_price: Current market price

        Returns:
            Dict with exit signal or None
        """
        if not self.current_position or not self.entry_price:
            return None

        # Calculate current P&L percentage
        pnl_pct = (current_price - self.entry_price) / self.entry_price

        # Check stop loss
        if self.current_position > 0:  # Long position
            if pnl_pct <= -self.stop_loss_pct / 100:
                return {'reason': f"Stop loss triggered: {pnl_pct * 100:.1f}%"}
            elif pnl_pct >= self.take_profit_pct / 100:
                return {'reason': f"Take profit triggered: +{pnl_pct * 100:.1f}%"}
        else:  # Short position
            if pnl_pct >= self.stop_loss_pct / 100:  # P&L is negative for profit on shorts
                return {'reason': f"Stop loss triggered: {pnl_pct * 100:.1f}%"}
            elif pnl_pct <= -self.take_profit_pct / 100:
                return {'reason': f"Take profit triggered: {pnl_pct * 100:.1f}%"}

        return None

    def calculate_position_size(self, symbol: str, risk_amount: float) -> int:
        """
        Calculate position size based on risk management.

        Args:
            symbol: Trading symbol
            risk_amount: Amount of capital to risk on this trade

        Returns:
            Number of shares to trade
        """
        # Get current price
        current_price = self.get_current_price(symbol)
        if not current_price:
            return 0

        # Calculate stop loss distance
        stop_distance = current_price * (self.stop_loss_pct / 100)

        # Calculate position size based on risk
        max_quantity = int(risk_amount / stop_distance)

        # Apply limits
        min_quantity = self.config.get('min_position_size', 10)
        max_quantity = min(max_quantity, self.config.get('max_position_size', 1000))

        return max(min_quantity, min(max_quantity, max_quantity))

    def should_stop_strategy(self) -> bool:
        """
        Check if strategy should be stopped.

        Returns:
            True if strategy should be stopped, False otherwise
        """
        # Check daily loss limit
        max_daily_loss = self.get_strategy_metadata().max_daily_loss_pct
        if max_daily_loss and self.current_pnl < -max_daily_loss * self.config.get('account_balance', 10000):
            self.log_warning(f"Strategy stopped due to daily loss limit: ${self.current_pnl:.2f}")
            return True

        # Check for excessive consecutive losses (simple example)
        recent_losses = getattr(self, '_recent_losses', 0)
        if recent_losses >= self.config.get('max_consecutive_losses', 5):
            self.log_warning(f"Strategy stopped due to {recent_losses} consecutive losses")
            return True

        return False

    def log_info(self, message: str) -> None:
        """Log an information message with strategy context."""
        self.logger.info(f"MAStrategy[{self.symbol}]: {message}")

    def log_warning(self, message: str) -> None:
        """Log a warning message with strategy context."""
        self.logger.warning(f"MAStrategy[{self.symbol}]: {message}")

    def log_error(self, message: str) -> None:
        """Log an error message with strategy context."""
        self.logger.error(f"MAStrategy[{self.symbol}]: {message}")