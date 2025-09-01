"""
Data Provider Interface for Strategy Data Access

This module provides unified access to market data for trading strategies,
supporting both live trading and backtesting modes.
"""

from abc import ABC, abstractmethod
from typing import Dict, List, Any, Optional, Set, Callable
from datetime import datetime, timedelta
import asyncio
import logging

# Import with try/except to handle both direct execution and module import
try:
    from ..models import StockQuote, OptionContract
    from ..provider_manager import provider_manager
except ImportError:
    # Fallback for direct execution or testing
    import sys
    from pathlib import Path
    sys.path.append(str(Path(__file__).parent.parent))
    from models import StockQuote, OptionContract
    from provider_manager import provider_manager

logger = logging.getLogger(__name__)


class StrategyDataProvider(ABC):
    """
    Abstract base class for strategy data providers.

    This defines the interface that strategies use to access market data,
    regardless of whether they're running in live or backtesting mode.
    """

    def __init__(self):
        self.subscriptions: Set[str] = set()
        self.data_callbacks: Dict[str, Callable] = {}
        self.is_connected = False

    @abstractmethod
    def subscribe_symbol(self, symbol: str, callback: Optional[Callable] = None) -> bool:
        """
        Subscribe to real-time data for a symbol.

        Args:
            symbol: Symbol to subscribe to
            callback: Optional callback for data updates

        Returns:
            True if subscription successful
        """
        pass

    @abstractmethod
    def unsubscribe_symbol(self, symbol: str) -> bool:
        """
        Unsubscribe from real-time data for a symbol.

        Args:
            symbol: Symbol to unsubscribe from

        Returns:
            True if unsubscription successful
        """
        pass

    @abstractmethod
    def get_current_price(self, symbol: str) -> Optional[float]:
        """
        Get current market price for a symbol.

        Args:
            symbol: Symbol to get price for

        Returns:
            Current market price or None
        """
        pass

    @abstractmethod
    def get_recent_candles(self, symbol: str, limit: int) -> List[Dict[str, Any]]:
        """
        Get recent price candles for technical analysis.

        Args:
            symbol: Symbol to get candles for
            limit: Number of candles to retrieve

        Returns:
            List of OHLCV candle dictionaries
        """
        pass

    @abstractmethod
    def get_options_chain(self, symbol: str, expiry: str = None, strikes_around_atm: int = 10) -> List[Dict]:
        """
        Get options chain data.

        Args:
            symbol: Underlying symbol
            expiry: Expiration date
            strikes_around_atm: Number of strikes around ATM

        Returns:
            List of option contract data
        """
        pass

    @abstractmethod
    def get_greeks_data(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get option Greeks data.

        Args:
            symbol: Option symbol

        Returns:
            Greeks data dictionary
        """
        pass

    def get_subscribed_symbols(self) -> List[str]:
        """Get list of currently subscribed symbols."""
        return list(self.subscriptions)

    async def start(self):
        """Start the data provider."""
        self.is_connected = True
        logger.info(f"StrategyDataProvider started with {len(self.subscriptions)} subscriptions")

    async def stop(self):
        """Stop the data provider."""
        self.is_connected = False
        self.subscriptions.clear()
        self.data_callbacks.clear()
        logger.info("StrategyDataProvider stopped")

    def _notify_callback(self, symbol: str, data: Dict[str, Any]):
        """Notify registered callback of data update."""
        if symbol in self.data_callbacks:
            callback = self.data_callbacks[symbol]
            try:
                callback(symbol, data)
            except Exception as e:
                logger.error(f"Error in data callback for {symbol}: {e}")


class LiveDataProvider(StrategyDataProvider):
    """
    Live data provider that connects to real-time market data.

    This provider integrates with the existing streaming system and handles
    real-time symbol subscriptions and data delivery.
    """

    def __init__(self):
        super().__init__()
        self._symbol_cache = {}  # Cache for fast access
        self._quote_subscriptions = set()
        self._greeks_subscriptions = set()
        self._streaming_queue = None
        self._data_task = None

    async def start(self):
        """Start live data streaming."""
        await super().start()

        # Get reference to the streaming system
        from ..streaming_manager import streaming_manager
        self.streaming_manager = streaming_manager

        # Create streaming queue for this strategy
        self._streaming_queue = asyncio.Queue()
        self.streaming_manager.add_strategy_queue(self._streaming_queue, id(self))

        # Start data processing task
        self._data_task = asyncio.create_task(self._process_streaming_data())

        logger.info("LiveDataProvider started - connected to streaming system")

    async def stop(self):
        """Stop live data streaming."""
        # Clean up subscriptions
        for symbol in self.get_subscribed_symbols():
            await self.unsubscribe_symbol(symbol)

        # Remove from streaming system
        if self.streaming_manager and hasattr(self.streaming_manager, 'remove_strategy_queue'):
            self.streaming_manager.remove_strategy_queue(id(self))

        # Cancel data processing task
        if self._data_task and not self._data_task.done():
            self._data_task.cancel()
            try:
                await self._data_task
            except asyncio.CancelledError:
                pass

        await super().stop()

    def subscribe_symbol(self, symbol: str, callback: Optional[Callable] = None) -> bool:
        """
        Subscribe to real-time data for a symbol.

        For options symbols, automatically subscribes to both quotes and Greeks.
        """
        if symbol in self.subscriptions:
            logger.debug(f"Already subscribed to {symbol}")
            return True

        try:
            # Register callback
            if callback:
                self.data_callbacks[symbol] = callback

            # Determine subscription type based on symbol
            if self._is_option_symbol(symbol):
                # Options: subscribe to quotes and Greeks
                success = self.streaming_manager.subscribe_to_symbols([symbol],
                                                                   data_types=['quote', 'greeks'])
                if success:
                    self._greeks_subscriptions.add(symbol)
            else:
                # Stocks: subscribe to quotes
                success = self.streaming_manager.subscribe_to_symbols([symbol],
                                                                   data_types=['quote'])

            if success:
                self.subscriptions.add(symbol)
                logger.info(f"Subscribed to live data for {symbol}")
                return True
            else:
                logger.error(f"Failed to subscribe to {symbol}")
                return False

        except Exception as e:
            logger.error(f"Error subscribing to {symbol}: {e}")
            return False

    def unsubscribe_symbol(self, symbol: str) -> bool:
        """
        Unsubscribe from real-time data for a symbol.
        """
        if symbol not in self.subscriptions:
            return True

        try:
            # Determine unsubsciption type
            if symbol in self._greeks_subscriptions:
                success = self.streaming_manager.unsubscribe_from_symbols([symbol],
                                                                       data_types=['quote', 'greeks'])
                self._greeks_subscriptions.remove(symbol)
            else:
                success = self.streaming_manager.unsubscribe_from_symbols([symbol],
                                                                       data_types=['quote'])

            if success:
                self.subscriptions.discard(symbol)
                self._symbol_cache.pop(symbol, None)
                self.data_callbacks.pop(symbol, None)
                logger.info(f"Unsubscribed from live data for {symbol}")
                return True
            else:
                logger.error(f"Failed to unsubscribe from {symbol}")
                return False

        except Exception as e:
            logger.error(f"Error unsubscribing from {symbol}: {e}")
            return False

    def get_current_price(self, symbol: str) -> Optional[float]:
        """
        Get current market price.

        Uses cached data for fast access.
        """
        if symbol in self._symbol_cache:
            cached_data = self._symbol_cache[symbol]
            if cached_data.get('price'):
                return cached_data['price']

        # Fallback to provider system
        try:
            if self._is_option_symbol(symbol):
                # Get option quote
                provider = provider_manager.get_provider_for_operation('option_quotes')
                if provider:
                    option_data = provider.get_stock_quote(symbol)  # Note: options use get_stock_quote too
                    if option_data and option_data.bid and option_data.ask:
                        price = (option_data.bid + option_data.ask) / 2
                        return price
            else:
                # Get stock quote
                provider = provider_manager.get_provider_for_operation('stock_quotes')
                if provider:
                    quote = provider.get_stock_quote(symbol)
                    if quote and quote.bid and quote_data.ask:
                        price = (quote.bid + quote.ask) / 2
                        return price
        except Exception as e:
            logger.error(f"Error getting price for {symbol}: {e}")

        return None

    def get_recent_candles(self, symbol: str, limit: int = 50) -> List[Dict[str, Any]]:
        """
        Get recent candles from the existing historical data system.
        """
        try:
            # Use the existing historical cache for live candles
            from ..services.ivx_cache import ivx_cache

            # Determine timeframe and appropriate data source
            if self._is_option_symbol(symbol):
                # For options, get equity bars (underlying)
                underlying_symbol = symbol.split('_')[0]  # Extract underlying
                bars = ivx_cache.get_cached_equity_bars(underlying_symbol, 'D', limit)
            else:
                # For stocks, get direct bars
                bars = ivx_cache.get_cached_equity_bars(symbol, 'D', limit)

            if bars:
                return bars[-limit:]  # Return most recent
            else:
                logger.warning(f"No cached bars found for {symbol}")
                return []

        except Exception as e:
            logger.error(f"Error getting candles for {symbol}: {e}")
            return []

    def get_options_chain(self, symbol: str, expiry: str = None, strikes_around_atm: int = 10) -> List[Dict]:
        """
        Get options chain data.
        """
        try:
            # Use existing options chain provider
            provider = provider_manager.get_provider_for_operation('options_chains')
            if provider:
                contracts = provider.get_options_chain_basic(symbol, expiry, None, strikes_around_atm)
                return contracts
        except Exception as e:
            logger.error(f"Error getting options chain for {symbol}: {e}")

        return []

    def get_greeks_data(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get Greeks data for an option.
        """
        try:
            if symbol in self._symbol_cache:
                cached_data = self._symbol_cache[symbol]
                return {
                    'delta': cached_data.get('delta'),
                    'gamma': cached_data.get('gamma'),
                    'theta': cached_data.get('theta'),
                    'vega': cached_data.get('vega'),
                    'implied_volatility': cached_data.get('iv')
                }

            # Fallback to provider system
            provider = provider_manager.get_provider_for_operation('greeks')
            if provider:
                greeks_list = provider.get_options_greeks_batch([symbol])
                if symbol in greeks_list:
                    return provider.get_options_greeks_batch([symbol])[symbol]

        except Exception as e:
            logger.error(f"Error getting Greeks for {symbol}: {e}")

        return None

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        # Simple check for OCC format with numbers and C/P
        return len(symbol) > 10 and ('C' in symbol or 'P' in symbol) and any(c.isdigit() for c in symbol)

    async def _process_streaming_data(self):
        """Process streaming data from the queue."""
        try:
            while self.is_connected:
                try:
                    # Wait for data from streaming queue
                    market_data = await asyncio.wait_for(
                        self._streaming_queue.get(),
                        timeout=1.0
                    )

                    if market_data:
                        symbol = market_data.symbol
                        data_dict = market_data.data

                        # Update symbol cache
                        self._symbol_cache[symbol] = {
                            'price': data_dict.get('price'),
                            'bid': data_dict.get('bid'),
                            'ask': data_dict.get('ask'),
                            'volume': data_dict.get('volume'),
                            'delta': data_dict.get('delta'),
                            'gamma': data_dict.get('gamma'),
                            'theta': data_dict.get('theta'),
                            'vega': data_dict.get('vega'),
                            'iv': data_dict.get('implied_volatility'),
                            'last_update': datetime.now()
                        }

                        # Notify callback if registered
                        self._notify_callback(symbol, data_dict)

                except asyncio.TimeoutError:
                    continue  # Normal timeout, keep processing
                except Exception as e:
                    logger.error(f"Error processing streaming data: {e}")
                    await asyncio.sleep(1.0)  # Brief pause on error

        except asyncio.CancelledError:
            logger.info("Live data processing task cancelled")
        except Exception as e:
            logger.error(f"Critical error in live data processing: {e}")
        finally:
            logger.info("Live data processing task stopped")


class BacktestDataProvider(StrategyDataProvider):
    """
    Backtesting data provider that plays historical data.

    This provider reads historical data files and plays them back in chronological
    order, simulating live market conditions for backtesting.
    """

    def __init__(self, start_date: str, end_date: str, speed_multiplier: float = 1.0):
        super().__init__()
        self.start_date = datetime.fromisoformat(start_date.replace('Z', '+00:00'))
        self.end_date = datetime.fromisoformat(end_date.replace('Z', '+00:00'))
        self.speed_multiplier = speed_multiplier

        # Backtesting state
        self.current_time = self.start_date
        self.historical_data = {}  # symbol -> list of MarketData
        self.time_cursor = {}  # symbol -> current data index

        # Simulation control
        self.is_paused = False
        self.backtest_task = None

    async def start(self):
        """Start backtest data playback."""
        await super().start()
        logger.info(f"BacktestDataProvider started: {self.start_date} to {self.end_date}")

    async def stop(self):
        """Stop backtesting."""
        self.is_paused = True

        if self.backtest_task and not self.backtest_task.done():
            self.backtest_task.cancel()
            try:
                await self.backtest_task
            except asyncio.CancelledError:
                pass

        await super().stop()

    def subscribe_symbol(self, symbol: str, callback: Optional[Callable] = None) -> bool:
        """
        Subscribe to historical data for a symbol.

        In backtesting mode, this loads historical data for the symbol.
        """
        if symbol in self.subscriptions:
            return True

        try:
            # Register callback
            if callback:
                self.data_callbacks[symbol] = callback

            # Load historical data for this symbol
            if self._load_historical_data(symbol):
                self.subscriptions.add(symbol)
                self.time_cursor[symbol] = 0
                logger.info(f"Subscribed to backtest data for {symbol}")
                return True
            else:
                logger.error(f"Failed to load backtest data for {symbol}")
                return False

        except Exception as e:
            logger.error(f"Error subscribing to backtest data for {symbol}: {e}")
            return False

    def unsubscribe_symbol(self, symbol: str) -> bool:
        """
        Unsubscribe from backtesting data.
        """
        if symbol not in self.subscriptions:
            return True

        # Clean up data
        self.subscriptions.discard(symbol)
        self.historical_data.pop(symbol, None)
        self.time_cursor.pop(symbol, None)
        self.data_callbacks.pop(symbol, None)

        logger.info(f"Unsubscribed from backtest data for {symbol}")
        return True

    def get_current_price(self, symbol: str) -> Optional[float]:
        """
        Get price at current backtest time.
        """
        if symbol not in self.historical_data:
            return None

        data_list = self.historical_data[symbol]
        cursor = self.time_cursor[symbol]

        if cursor < len(data_list):
            current_data = data_list[cursor]
            return current_data.price

        return None

    def get_recent_candles(self, symbol: str, limit: int = 50) -> List[Dict[str, Any]]:
        """
        Get historical candles up to current backtest time.
        """
        if symbol not in self.historical_data:
            return []

        data_list = self.historical_data[symbol]
        cursor = self.time_cursor[symbol]
        start_index = max(0, cursor - limit)

        candles = []
        for i in range(start_index, cursor):
            data = data_list[i]

            # Convert MarketData to candle format
            candle = {
                'time': data.timestamp.isoformat(),
                'open': data.price,  # Simplified: use current price for OHLC
                'high': data.price,
                'low': data.price,
                'close': data.price,
                'volume': data.volume or 0
            }
            candles.append(candle)

        return candles

    def get_options_chain(self, symbol: str, expiry: str = None, strikes_around_atm: int = 10) -> List[Dict]:
        """
        Get options chain at current backtest time.
        """
        # Simplified implementation - return empty list
        # In a full implementation, this would replay historical options data
        logger.warning("Options chain data not implemented for backtesting")
        return []

    def get_greeks_data(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Get Greeks data at current backtest time.
        """
        if symbol not in self.historical_data:
            return None

        data_list = self.historical_data[symbol]
        cursor = self.time_cursor[symbol]

        if cursor < len(data_list):
            current_data = data_list[cursor]
            return {
                'delta': current_data.delta,
                'gamma': current_data.gamma,
                'theta': current_data.theta,
                'vega': current_data.vega,
                'implied_volatility': current_data.implied_volatility
            }

        return None

    def _load_historical_data(self, symbol: str) -> bool:
        """
        Load historical data for a symbol.

        In a real implementation, this would read from files or database.
        For now, we create mock data.
        """
        try:
            # Generate mock historical data for the backtest period
            data_points = []
            current_time = self.start_date
            base_price = 150.0  # Example base price

            while current_time <= self.end_date:
                # Simulate price movement
                price_change = (0.01 * (1 if time.time() % 2 == 0 else -1))  # ±1% random
                price = base_price * (1 + price_change)

                # Create MarketData object
                market_data = MarketData(
                    symbol=symbol,
                    timestamp=current_time,
                    price=price,
                    bid=price * 0.999,  # Slight spread
                    ask=price * 1.001,
                    bid_size=100,
                    ask_size=100,
                    volume=10000
                )

                data_points.append(market_data)

                # Advance time (e.g., every minute)
                current_time += timedelta(minutes=1)
                base_price = price  # Update base price

            self.historical_data[symbol] = data_points
            logger.info(f"Loaded {len(data_points)} historical data points for {symbol}")
            return True

        except Exception as e:
            logger.error(f"Error loading historical data for {symbol}: {e}")
            return False

    async def advance_time(self) -> bool:
        """
        Advance the backtest timeline and trigger data updates.

        Returns:
            False if backtest has ended, True otherwise
        """
        if self.current_time >= self.end_date or self.is_paused:
            return False

        # Advance time (apply speed multiplier)
        time_advance = timedelta(minutes=1) * self.speed_multiplier
        self.current_time += time_advance

        # Update all subscribed symbols to current time point
        for symbol in self.subscriptions:
            if symbol in self.historical_data:
                data_list = self.historical_data[symbol]
                cursor = self.time_cursor[symbol]

                # Find first data point at or after current time
                while (cursor < len(data_list) and
                       data_list[cursor].timestamp < self.current_time):
                    # Notify callback of this data point
                    data_point = data_list[cursor]
                    self._notify_callback(symbol, data_point.data)

                    cursor += 1

                self.time_cursor[symbol] = cursor

        return True

    def pause(self):
        """Pause backtesting."""
        self.is_paused = True
        logger.info("Backtesting paused")

    def resume(self):
        """Resume backtesting."""
        self.is_paused = False
        logger.info("Backtesting resumed")

    def set_speed(self, multiplier: float):
        """Set backtesting speed multiplier."""
        self.speed_multiplier = max(0.1, multiplier)  # Minimum 0.1x speed
        logger.info(f"Backtesting speed set to {self.speed_multiplier}x")
