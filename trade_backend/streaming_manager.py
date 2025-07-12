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

    async def get_data(self) -> Optional[MarketData]:
        return await self._streaming_queue.get()

streaming_manager = StreamingManager()
