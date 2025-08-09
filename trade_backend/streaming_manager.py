import asyncio
import logging
import time
from typing import List, Dict, Optional, Set

from .provider_config import provider_config_manager
from .provider_manager import provider_manager
from .models import MarketData
from .utils.symbol_converter import SymbolConverter

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
    """
    
    def __init__(self):
        self._providers = {}
        self._latest_cache = LatestValueCache()
        self._provider_subscriptions: Set[str] = set()
        self._is_connected = False
        self._shutdown_event = asyncio.Event()
        
    async def connect(self):
        """Connects to all streaming providers specified in the configuration."""
        try:
            logger.info("🔄 Starting streaming manager connection...")
            config = provider_config_manager.get_config()
            
            required_providers = set()
            if config.get("streaming_quotes"):
                required_providers.add(config["streaming_quotes"])
            if config.get("streaming_greeks"):
                required_providers.add(config["streaming_greeks"])
            
            for provider_name in required_providers:
                if provider_name and provider_name not in self._providers:
                    provider = provider_manager._providers.get(provider_name)
                    if provider:
                        provider.set_streaming_cache(self._latest_cache)
                        self._providers[provider_name] = provider
                        connected = await self._connect_with_retry(provider)
                        if connected:
                            logger.info(f"✅ Provider {provider_name} connected for streaming.")
                        else:
                            logger.error(f"❌ Failed to connect provider {provider_name}.")
            
            self._is_connected = any(p.is_streaming_connected() for p in self._providers.values())
            logger.info(f"✅ Streaming manager connected. Active providers: {list(self._providers.keys())}")
            return True
        except Exception as e:
            logger.error(f"❌ Error connecting streaming manager: {e}")
            return False

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
        """Aggregates all client subscriptions and updates the provider."""
        # 1. Aggregate all symbols from all clients
        global_new_symbols = set()
        for client_subs in client_subscriptions.values():
            global_new_symbols.update(client_subs)
            
        # 2. Calculate the difference
        symbols_to_add = global_new_symbols - self._provider_subscriptions
        symbols_to_remove = self._provider_subscriptions - global_new_symbols
        
        # 3. Unsubscribe from symbols that are no longer needed
        if symbols_to_remove:
            logger.info(f"🧹 Unsubscribing from {len(symbols_to_remove)} symbols.")
            await self._unsubscribe_symbols_safe(list(symbols_to_remove))
        
        # 4. Subscribe to new symbols
        if symbols_to_add:
            logger.info(f"📡 Subscribing to {len(symbols_to_add)} new symbols.")
            await self._subscribe_symbols_safe(list(symbols_to_add))
            
        # 5. Update the global state
        self._provider_subscriptions = global_new_symbols
        logger.info(f"Global subscriptions updated. Total: {len(self._provider_subscriptions)} symbols.")

    async def _subscribe_symbols_safe(self, symbols: List[str]):
        """Safely subscribes to symbols on all relevant providers."""
        config = provider_config_manager.get_config()
        quotes_provider_name = config.get("streaming_quotes")
        
        if quotes_provider_name and quotes_provider_name in self._providers:
            provider = self._providers[quotes_provider_name]
            try:
                provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, quotes_provider_name)
                await provider.subscribe_to_symbols(provider_symbols)
            except Exception as e:
                logger.error(f"❌ Error subscribing on {quotes_provider_name}: {e}")

    async def _unsubscribe_symbols_safe(self, symbols: List[str]):
        """Safely unsubscribes from symbols on all relevant providers."""
        config = provider_config_manager.get_config()
        quotes_provider_name = config.get("streaming_quotes")
        
        if quotes_provider_name and quotes_provider_name in self._providers:
            provider = self._providers[quotes_provider_name]
            if hasattr(provider, 'unsubscribe_from_symbols'):
                try:
                    provider_symbols = SymbolConverter.batch_convert_to_provider_format(symbols, quotes_provider_name)
                    await provider.unsubscribe_from_symbols(provider_symbols)
                except Exception as e:
                    logger.error(f"❌ Error unsubscribing on {quotes_provider_name}: {e}")

    def get_subscription_status(self) -> Dict[str, any]:
        """Gets the current global subscription status."""
        return {
            "provider_subscriptions": list(self._provider_subscriptions),
            "total_subscriptions": len(self._provider_subscriptions),
            "is_connected": self._is_connected,
        }

    async def disconnect(self):
        """Disconnects all streaming providers."""
        logger.info("🛑 Disconnecting streaming manager...")
        self._shutdown_event.set()
        disconnect_tasks = [p.disconnect_streaming() for p in self._providers.values()]
        await asyncio.gather(*disconnect_tasks, return_exceptions=True)
        self._providers.clear()
        self._provider_subscriptions.clear()
        self._is_connected = False
        logger.info("✅ Streaming manager disconnected.")

# Singleton instance
streaming_manager = StreamingManager()
