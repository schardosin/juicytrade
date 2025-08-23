import asyncio
import logging
import time
from typing import List, Dict, Optional, Set

from .provider_config import provider_config_manager
from .provider_manager import provider_manager
from .models import MarketData
from .utils.symbol_converter import SymbolConverter
from .ivx_streaming_manager import ivx_streaming_manager

logger = logging.getLogger(__name__)

class LatestValueCache:
    """
    Latest-value cache for real-time market data.
    Only stores the most recent value for each symbol, discarding old data.
    """
    
    def __init__(self):
        self._data: Dict[str, MarketData] = {}
        self._lock = asyncio.Lock()
        self._update_callbacks: List[callable] = []
    
    async def update(self, market_data: MarketData):
        """Update cache with new market data - maximum performance, no deduplication."""
        async with self._lock:
            symbol = market_data.symbol
            
            if not market_data.timestamp_ms:
                market_data.timestamp_ms = time.time() * 1000
            
            self._data[symbol] = market_data
            
            for callback in self._update_callbacks:
                try:
                    await callback(market_data)
                except Exception as e:
                    logger.error(f"❌ Error in cache update callback: {e}")
            
            return True
    
    async def get_latest(self, symbol: str) -> Optional[MarketData]:
        """Get the latest data for a symbol."""
        async with self._lock:
            return self._data.get(symbol)
    
    def add_update_callback(self, callback: callable):
        """Add callback to be called when data is updated."""
        self._update_callbacks.append(callback)

class StreamingManager:
    """
    Manages streaming provider connections and aggregates subscriptions from all clients.
    Supports independent quote and Greeks streams from different providers.
    """
    
    def __init__(self):
        # Separate provider tracking for quotes and Greeks
        self._quote_providers = {}
        self._greeks_providers = {}
        self._latest_cache = LatestValueCache()
        
        # Separate subscription tracking
        self._quote_subscriptions: Set[str] = set()
        self._greeks_subscriptions: Set[str] = set()
        
        self._is_connected = False
        self._shutdown_event = asyncio.Event()
        
    async def connect(self):
        """Connects to all streaming providers specified in the configuration."""
        try:
            logger.info("🔄 Starting streaming manager connection...")
            config = provider_config_manager.get_config()
            
            # Connect quote providers
            quotes_provider_name = config.get("streaming_quotes")
            if quotes_provider_name:
                await self._connect_quote_provider(quotes_provider_name)
            
            # Connect Greeks providers (independent of quotes)
            greeks_provider_name = config.get("streaming_greeks")
            if greeks_provider_name:
                await self._connect_greeks_provider(greeks_provider_name)
            
            # Check overall connection status
            all_providers = set(self._quote_providers.values()) | set(self._greeks_providers.values())
            self._is_connected = any(p.is_streaming_connected() for p in all_providers)
            
            active_providers = list(self._quote_providers.keys()) + list(self._greeks_providers.keys())
            logger.info(f"✅ Streaming manager connected. Active providers: {active_providers}")
            return True
        except Exception as e:
            logger.error(f"❌ Error connecting streaming manager: {e}")
            return False

    async def _connect_quote_provider(self, provider_name: str):
        """Connect a provider for quote streaming."""
        if provider_name not in self._quote_providers:
            provider = provider_manager._providers.get(provider_name)
            if provider:
                provider.set_streaming_cache(self._latest_cache)
                self._quote_providers[provider_name] = provider
                connected = await self._connect_with_retry(provider)
                if connected:
                    logger.info(f"✅ {provider_name} connected for quote streaming.")
                else:
                    logger.error(f"❌ Failed to connect {provider_name} for quotes.")

    async def _connect_greeks_provider(self, provider_name: str):
        """Connect a provider for Greeks streaming."""
        if provider_name not in self._greeks_providers:
            # Check if this provider is already connected for quotes
            if provider_name in self._quote_providers:
                # Reuse the existing connection
                self._greeks_providers[provider_name] = self._quote_providers[provider_name]
                logger.info(f"✅ {provider_name} reusing connection for Greeks streaming.")
            else:
                # Create new connection for Greeks only
                provider = provider_manager._providers.get(provider_name)
                if provider:
                    provider.set_streaming_cache(self._latest_cache)
                    self._greeks_providers[provider_name] = provider
                    connected = await self._connect_with_retry(provider)
                    if connected:
                        logger.info(f"✅ {provider_name} connected for Greeks streaming.")
                    else:
                        logger.error(f"❌ Failed to connect {provider_name} for Greeks.")

    async def _connect_with_retry(self, provider, max_retries=3):
        """Connects a provider with exponential backoff retry logic."""
        for attempt in range(max_retries):
            try:
                logger.info(f"🔄 Connecting {provider.name} (attempt {attempt + 1}/{max_retries})")
                if await provider.connect_streaming():
                    logger.info(f"✅ {provider.name} connected successfully.")
                    return True
            except Exception as e:
                logger.error(f"❌ Connection error for {provider.name}: {e}")
            
            if attempt < max_retries - 1:
                delay = min(2 ** attempt, 30)
                logger.info(f"⏳ Waiting {delay}s before retry...")
                await asyncio.sleep(delay)
        return False

    async def update_global_subscriptions(self, client_subscriptions: Dict[any, Set[str]]):
        """Aggregates all client subscriptions and updates both quote and Greeks providers."""
        # 1. Aggregate all symbols from all clients
        global_new_symbols = set()
        for client_subs in client_subscriptions.values():
            global_new_symbols.update(client_subs)
        
        # 2. Separate stock and option symbols
        stock_symbols = {s for s in global_new_symbols if not self._is_option_symbol(s)}
        option_symbols = {s for s in global_new_symbols if self._is_option_symbol(s)}
        
        # 3. Update quote subscriptions (stocks + options)
        await self._update_quote_subscriptions(global_new_symbols)
        
        # 4. Update Greeks subscriptions (options only)
        await self._update_greeks_subscriptions(option_symbols)
        
        # 5. Trigger IVx calculations for stock symbols
        await self._update_ivx_subscriptions(stock_symbols)
        
        logger.info(f"Global subscriptions updated. Quotes: {len(global_new_symbols)}, Greeks: {len(option_symbols)}, IVx: {len(stock_symbols)} symbols.")

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])

    async def _update_quote_subscriptions(self, symbols: Set[str]):
        """Update quote subscriptions on quote providers."""
        symbols_to_add = symbols - self._quote_subscriptions
        symbols_to_remove = self._quote_subscriptions - symbols
        
        # Unsubscribe from removed symbols
        if symbols_to_remove:
            logger.info(f"🧹 Unsubscribing quotes from {len(symbols_to_remove)} symbols.")
            await self._unsubscribe_quotes_safe(list(symbols_to_remove))
        
        # Subscribe to new symbols
        if symbols_to_add:
            logger.info(f"📡 Subscribing quotes to {len(symbols_to_add)} new symbols.")
            await self._subscribe_quotes_safe(list(symbols_to_add))
        
        self._quote_subscriptions = symbols

    async def _update_greeks_subscriptions(self, option_symbols: Set[str]):
        """Update Greeks subscriptions on Greeks providers."""
        symbols_to_add = option_symbols - self._greeks_subscriptions
        symbols_to_remove = self._greeks_subscriptions - option_symbols
        
        # Unsubscribe from removed symbols
        if symbols_to_remove:
            logger.info(f"🧹 Unsubscribing Greeks from {len(symbols_to_remove)} symbols.")
            await self._unsubscribe_greeks_safe(list(symbols_to_remove))
        
        # Subscribe to new symbols
        if symbols_to_add:
            logger.info(f"📊 Subscribing Greeks to {len(symbols_to_add)} new symbols.")
            await self._subscribe_greeks_safe(list(symbols_to_add))
        
        self._greeks_subscriptions = option_symbols

    async def _subscribe_quotes_safe(self, symbols: List[str]):
        """Safely subscribes to quote data on quote providers."""
        for provider_name, provider in self._quote_providers.items():
            try:
                provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, provider_name)
                await provider.subscribe_to_symbols(provider_symbols)
                logger.debug(f"✅ Subscribed to quotes on {provider_name}: {len(symbols)} symbols")
            except Exception as e:
                logger.error(f"❌ Error subscribing quotes on {provider_name}: {e}")

    async def _unsubscribe_quotes_safe(self, symbols: List[str]):
        """Safely unsubscribes from quote data on quote providers."""
        for provider_name, provider in self._quote_providers.items():
            if hasattr(provider, 'unsubscribe_from_symbols'):
                try:
                    provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, provider_name)
                    await provider.unsubscribe_from_symbols(provider_symbols)
                    logger.debug(f"✅ Unsubscribed from quotes on {provider_name}: {len(symbols)} symbols")
                except Exception as e:
                    logger.error(f"❌ Error unsubscribing quotes on {provider_name}: {e}")

    async def _subscribe_greeks_safe(self, symbols: List[str]):
        """Safely subscribes to Greeks data on Greeks providers."""
        for provider_name, provider in self._greeks_providers.items():
            try:
                # Call subscribe_to_symbols with Greeks-only data types
                # This ensures we only get Greeks data, not duplicate quotes
                provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, provider_name)
                await provider.subscribe_to_symbols(provider_symbols, data_types=["Greeks"])
                logger.info(f"✅ Subscribed to Greeks-only streaming on {provider_name}: {len(symbols)} symbols")
            except Exception as e:
                logger.error(f"❌ Error subscribing Greeks on {provider_name}: {e}")

    async def _unsubscribe_greeks_safe(self, symbols: List[str]):
        """Safely unsubscribes from Greeks data on Greeks providers."""
        for provider_name, provider in self._greeks_providers.items():
            if hasattr(provider, 'unsubscribe_from_symbols'):
                try:
                    provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, provider_name)
                    await provider.unsubscribe_from_symbols(provider_symbols, data_types=["Greeks"])
                    logger.info(f"✅ Unsubscribed from Greeks-only streaming on {provider_name}: {len(symbols)} symbols")
                except Exception as e:
                    logger.error(f"❌ Error unsubscribing Greeks on {provider_name}: {e}")

    async def _update_ivx_subscriptions(self, stock_symbols: Set[str]):
        """Triggers IVx calculations for subscribed stock symbols."""
        if not ivx_streaming_manager:
            return

        for symbol in stock_symbols:
            # This will start the calculation if it's not already running.
            await ivx_streaming_manager.calculate_ivx_for_symbol(symbol)

    def get_subscription_status(self) -> Dict[str, any]:
        """Gets the current global subscription status."""
        return {
            "quote_subscriptions": list(self._quote_subscriptions),
            "greeks_subscriptions": list(self._greeks_subscriptions),
            "total_quote_subscriptions": len(self._quote_subscriptions),
            "total_greeks_subscriptions": len(self._greeks_subscriptions),
            "is_connected": self._is_connected,
            "quote_providers": list(self._quote_providers.keys()),
            "greeks_providers": list(self._greeks_providers.keys()),
        }

    def get_health_stats(self) -> Dict[str, any]:
        """Get detailed health statistics for streaming connections."""
        all_providers = set(self._quote_providers.values()) | set(self._greeks_providers.values())
        
        return {
            "is_connected": self._is_connected,
            "quote_providers": {
                name: provider.is_streaming_connected() if hasattr(provider, 'is_streaming_connected') else False
                for name, provider in self._quote_providers.items()
            },
            "greeks_providers": {
                name: provider.is_streaming_connected() if hasattr(provider, 'is_streaming_connected') else False
                for name, provider in self._greeks_providers.items()
            },
            "total_providers": len(all_providers),
            "subscription_counts": {
                "quotes": len(self._quote_subscriptions),
                "greeks": len(self._greeks_subscriptions)
            }
        }

    async def disconnect(self):
        """Disconnects all streaming providers."""
        logger.info("🛑 Disconnecting streaming manager...")
        self._shutdown_event.set()
        
        # Disconnect all providers (avoiding duplicates)
        all_providers = set(self._quote_providers.values()) | set(self._greeks_providers.values())
        disconnect_tasks = [p.disconnect_streaming() for p in all_providers]
        await asyncio.gather(*disconnect_tasks, return_exceptions=True)
        
        # Clear all state
        self._quote_providers.clear()
        self._greeks_providers.clear()
        self._quote_subscriptions.clear()
        self._greeks_subscriptions.clear()
        self._is_connected = False
        logger.info("✅ Streaming manager disconnected.")

    async def restart_with_new_config(self):
        """Restart streaming connections with new configuration while preserving subscriptions."""
        try:
            logger.info("🔄 Restarting streaming manager with new configuration...")
            
            # Store current subscriptions to restore them after restart
            current_quote_subscriptions = self._quote_subscriptions.copy()
            current_greeks_subscriptions = self._greeks_subscriptions.copy()
            
            logger.info(f"📋 Preserving {len(current_quote_subscriptions)} quote and {len(current_greeks_subscriptions)} Greeks subscriptions")
            
            # Disconnect all current connections
            await self.disconnect()
            
            # Reset shutdown event for reconnection
            self._shutdown_event.clear()
            
            # Reconnect with new configuration
            connected = await self.connect()
            
            if not connected:
                logger.error("❌ Failed to reconnect streaming manager with new configuration")
                return False
            
            # Restore subscriptions if we had any
            if current_quote_subscriptions or current_greeks_subscriptions:
                logger.info("🔄 Restoring previous subscriptions...")
                
                # Restore quote subscriptions
                if current_quote_subscriptions:
                    await self._update_quote_subscriptions(current_quote_subscriptions)
                
                # Restore Greeks subscriptions  
                if current_greeks_subscriptions:
                    await self._update_greeks_subscriptions(current_greeks_subscriptions)
                
                logger.info(f"✅ Restored {len(current_quote_subscriptions)} quote and {len(current_greeks_subscriptions)} Greeks subscriptions")
            
            logger.info("✅ Streaming manager restarted successfully with new configuration")
            return True
            
        except Exception as e:
            logger.error(f"❌ Error restarting streaming manager: {e}")
            return False

# Singleton instance
streaming_manager = StreamingManager()
