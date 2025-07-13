import asyncio
import logging
from typing import List, Dict, Optional, Set

from .provider_config import provider_config_manager
from .provider_manager import provider_manager
from .models import MarketData

logger = logging.getLogger(__name__)

class StreamingManager:
    def __init__(self):
        self._providers = {}
        self._streaming_queue = asyncio.Queue()
        self._subscriptions: Dict[str, Set[str]] = {} # { "stock_quotes": {"AAPL", "SPY"}, ... }
        
        # Smart subscription tracking
        self._stock_subscription: Optional[str] = None
        self._options_subscriptions: Set[str] = set()
        self._persistent_subscriptions: Set[str] = set()

    async def connect(self):
        config = provider_config_manager.get_config().get("streaming", {})
        for stream_type, provider_name in config.items():
            if provider_name not in self._providers:
                provider = provider_manager._providers.get(provider_name)
                if provider:
                    self._providers[provider_name] = provider
                    await provider.connect_streaming()
                    asyncio.create_task(self._data_aggregator(provider))

    async def _data_aggregator(self, provider):
        while provider.is_streaming_connected():
            data = await provider.get_streaming_data()
            if data:
                await self._streaming_queue.put(data)
            await asyncio.sleep(0.01)

    async def disconnect(self):
        for provider in self._providers.values():
            await provider.disconnect_streaming()

    async def subscribe(self, symbols: List[str], data_types: List[str] = None):
        if not data_types:
            data_types = ["stock_quotes", "option_quotes"]

        config = provider_config_manager.get_config().get("streaming", {})
        
        for data_type in data_types:
            provider_name = config.get(data_type)
            if provider_name and provider_name in self._providers:
                provider = self._providers[provider_name]
                
                if data_type not in self._subscriptions:
                    self._subscriptions[data_type] = set()
                
                new_symbols = set(symbols) - self._subscriptions[data_type]
                if new_symbols:
                    await provider.subscribe_to_symbols(list(new_symbols), [data_type])
                    self._subscriptions[data_type].update(new_symbols)

    async def _subscribe_symbols(self, symbols: List[str], data_types: List[str]):
        """Internal method to subscribe to symbols"""
        config = provider_config_manager.get_config().get("streaming", {})
        
        for data_type in data_types:
            provider_name = config.get(data_type)
            if provider_name and provider_name in self._providers:
                provider = self._providers[provider_name]
                
                if data_type not in self._subscriptions:
                    self._subscriptions[data_type] = set()
                
                new_symbols = set(symbols) - self._subscriptions[data_type]
                if new_symbols:
                    logger.info(f"Subscribing to {len(new_symbols)} symbols for {data_type}")
                    await provider.subscribe_to_symbols(list(new_symbols), [data_type])
                    self._subscriptions[data_type].update(new_symbols)

    async def _unsubscribe_symbols(self, symbols: List[str], data_types: List[str]):
        """Internal method to unsubscribe from symbols"""
        config = provider_config_manager.get_config().get("streaming", {})
        
        for data_type in data_types:
            provider_name = config.get(data_type)
            if provider_name and provider_name in self._providers:
                provider = self._providers[provider_name]
                
                if data_type in self._subscriptions:
                    symbols_to_remove = set(symbols) & self._subscriptions[data_type]
                    if symbols_to_remove:
                        logger.info(f"Unsubscribing from {len(symbols_to_remove)} symbols for {data_type}")
                        # Check if provider has unsubscribe method
                        if hasattr(provider, 'unsubscribe_from_symbols'):
                            await provider.unsubscribe_from_symbols(list(symbols_to_remove), [data_type])
                        self._subscriptions[data_type] -= symbols_to_remove

    async def replace_stock_subscription(self, symbol: str):
        """Replace current stock subscription with new symbol"""
        logger.info(f"Replacing stock subscription: {self._stock_subscription} -> {symbol}")
        
        # Unsubscribe from current stock if exists
        if self._stock_subscription:
            await self._unsubscribe_symbols([self._stock_subscription], ["stock_quotes"])
        
        # Subscribe to new stock
        await self._subscribe_symbols([symbol], ["stock_quotes"])
        self._stock_subscription = symbol

    async def replace_options_subscriptions(self, symbols: List[str]):
        """Replace all current options subscriptions with new symbols"""
        logger.info(f"Replacing options subscriptions: {len(self._options_subscriptions)} -> {len(symbols)} symbols")
        
        # Unsubscribe from all current options
        if self._options_subscriptions:
            await self._unsubscribe_symbols(list(self._options_subscriptions), ["option_quotes"])
        
        # Subscribe to new options
        if symbols:
            await self._subscribe_symbols(symbols, ["option_quotes"])
        
        self._options_subscriptions = set(symbols)

    async def ensure_persistent_subscriptions(self, data_types: List[str]):
        """Ensure persistent subscriptions are active (orders, positions)"""
        logger.info(f"Ensuring persistent subscriptions for: {data_types}")
        
        for data_type in data_types:
            if data_type not in self._persistent_subscriptions:
                # For persistent subscriptions, we don't need specific symbols
                # The provider will handle account-level subscriptions
                config = provider_config_manager.get_config().get("streaming", {})
                provider_name = config.get(data_type)
                if provider_name and provider_name in self._providers:
                    provider = self._providers[provider_name]
                    if hasattr(provider, 'subscribe_to_account_updates'):
                        await provider.subscribe_to_account_updates([data_type])
                    self._persistent_subscriptions.add(data_type)

    def get_subscription_status(self) -> Dict[str, any]:
        """Get current subscription status for debugging"""
        return {
            "stock_subscription": self._stock_subscription,
            "options_subscriptions": list(self._options_subscriptions),
            "persistent_subscriptions": list(self._persistent_subscriptions),
            "total_subscriptions": {
                data_type: len(symbols) for data_type, symbols in self._subscriptions.items()
            }
        }

    async def get_data(self) -> Optional[MarketData]:
        return await self._streaming_queue.get()

streaming_manager = StreamingManager()
