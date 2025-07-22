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
        self._stock_subscriptions: Set[str] = set()     # Multiple stocks
        self._options_subscriptions: Set[str] = set()
        self._persistent_subscriptions: Set[str] = set()

    async def connect(self):
        """Connect to streaming providers and set up data flow."""
        config = provider_config_manager.get_config()
        
        # Connect streaming quotes provider
        quotes_provider_name = config.get("streaming_quotes")
        if quotes_provider_name and quotes_provider_name not in self._providers:
            provider = provider_manager._providers.get(quotes_provider_name)
            if provider:
                provider.set_streaming_queue(self._streaming_queue)
                self._providers[quotes_provider_name] = provider
                await provider.connect_streaming()
                logger.info(f"Provider {quotes_provider_name} connected for quotes streaming.")
        
        # Connect trade account provider for positions/orders streaming
        trade_account_provider_name = config.get("trade_account")
        if trade_account_provider_name and trade_account_provider_name not in self._providers:
            provider = provider_manager._providers.get(trade_account_provider_name)
            if provider:
                provider.set_streaming_queue(self._streaming_queue)
                self._providers[trade_account_provider_name] = provider
                await provider.connect_streaming()
                logger.info(f"Provider {trade_account_provider_name} connected for trade account streaming.")

    async def disconnect(self):
        for provider in self._providers.values():
            await provider.disconnect_streaming()

    async def subscribe(self, symbols: List[str], data_types: List[str] = None):
        """Legacy method - use replace_all_subscriptions for new code"""
        if not data_types:
            data_types = ["quotes"]

        config = provider_config_manager.get_config().get("streaming", {})
        
        for data_type in data_types:
            provider_name = config.get(data_type)
            if provider_name and provider_name in self._providers:
                provider = self._providers[provider_name]
                
                if data_type not in self._subscriptions:
                    self._subscriptions[data_type] = set()
                
                new_symbols = set(symbols) - self._subscriptions[data_type]
                if new_symbols:
                    await provider.subscribe_to_symbols(list(new_symbols))
                    self._subscriptions[data_type].update(new_symbols)

    async def _subscribe_symbols(self, symbols: List[str]):
        """Internal method to subscribe to symbols using streaming quotes provider"""
        config = provider_config_manager.get_config()
        quotes_provider_name = config.get("streaming_quotes")
        
        if quotes_provider_name and quotes_provider_name in self._providers:
            provider = self._providers[quotes_provider_name]
            
            if "quotes" not in self._subscriptions:
                self._subscriptions["quotes"] = set()
            
            new_symbols = set(symbols) - self._subscriptions["quotes"]
            if new_symbols:
                logger.info(f"Subscribing to {len(new_symbols)} symbols via {quotes_provider_name} streaming quotes provider")
                await provider.subscribe_to_symbols(list(new_symbols))
                self._subscriptions["quotes"].update(new_symbols)

    async def _unsubscribe_symbols(self, symbols: List[str]):
        """Internal method to unsubscribe from symbols using streaming quotes provider"""
        config = provider_config_manager.get_config()
        quotes_provider_name = config.get("streaming_quotes")
        
        if quotes_provider_name and quotes_provider_name in self._providers:
            provider = self._providers[quotes_provider_name]
            
            if "quotes" in self._subscriptions:
                symbols_to_remove = set(symbols) & self._subscriptions["quotes"]
                if symbols_to_remove:
                    logger.info(f"Unsubscribing from {len(symbols_to_remove)} symbols via {quotes_provider_name} streaming quotes provider")
                    # Check if provider has unsubscribe method
                    if hasattr(provider, 'unsubscribe_from_symbols'):
                        await provider.unsubscribe_from_symbols(list(symbols_to_remove))
                    self._subscriptions["quotes"] -= symbols_to_remove

    async def replace_all_subscriptions(self, underlying_symbol: str, option_symbols: List[str]):
        """Unified method to replace ALL subscriptions with new underlying + options symbols"""
        logger.info(f"🔄 StreamingManager: Replacing all subscriptions - underlying: {underlying_symbol}, options: {len(option_symbols)} symbols")
        
        # Step 1: Unsubscribe from all current symbols
        all_current_symbols = []
        all_current_symbols.extend(list(self._stock_subscriptions))
        all_current_symbols.extend(list(self._options_subscriptions))
        
        if all_current_symbols:
            logger.info(f"🧹 StreamingManager: Unsubscribing from {len(all_current_symbols)} current symbols")
            await self._unsubscribe_symbols(all_current_symbols)
        
        # Step 2: Subscribe to all new symbols
        all_new_symbols = [underlying_symbol] + option_symbols
        if all_new_symbols:
            logger.info(f"📡 StreamingManager: Subscribing to {len(all_new_symbols)} new symbols")
            await self._subscribe_symbols(all_new_symbols)
        
        # Step 3: Update tracking
        old_stock_count = len(self._stock_subscriptions)
        old_options_count = len(self._options_subscriptions)
        self._stock_subscriptions = {underlying_symbol}
        self._options_subscriptions = set(option_symbols)
        
        logger.info(f"✅ StreamingManager: All subscriptions replaced. Stocks: {old_stock_count} -> 1 ({underlying_symbol}), Options: {old_options_count} -> {len(option_symbols)}")

    async def replace_all_subscriptions_multi_stock(self, stock_symbols: List[str], option_symbols: List[str]):
        """Replace ALL subscriptions with multiple stock symbols + options symbols"""
        logger.info(f"🔄 StreamingManager: Replacing all subscriptions (multi-stock) - stocks: {len(stock_symbols)}, options: {len(option_symbols)} symbols")
        
        # Step 1: Unsubscribe from all current symbols
        all_current_symbols = []
        all_current_symbols.extend(list(self._stock_subscriptions))
        all_current_symbols.extend(list(self._options_subscriptions))
        
        if all_current_symbols:
            logger.info(f"🧹 StreamingManager: Unsubscribing from {len(all_current_symbols)} current symbols")
            await self._unsubscribe_symbols(all_current_symbols)
        
        # Step 2: Subscribe to all new symbols
        all_new_symbols = stock_symbols + option_symbols
        if all_new_symbols:
            logger.info(f"📡 StreamingManager: Subscribing to {len(all_new_symbols)} new symbols")
            await self._subscribe_symbols(all_new_symbols)
        
        # Step 3: Update tracking
        old_stock_count = len(self._stock_subscriptions)
        old_options_count = len(self._options_subscriptions)
        
        # Update subscriptions
        self._stock_subscriptions = set(stock_symbols)
        self._options_subscriptions = set(option_symbols)
        
        logger.info(f"✅ StreamingManager: All subscriptions replaced (multi-stock). Stocks: {old_stock_count} -> {len(stock_symbols)}, Options: {old_options_count} -> {len(option_symbols)}")

    async def replace_stock_subscription(self, symbol: str):
        """Legacy method - use replace_all_subscriptions for new code"""
        logger.warning("replace_stock_subscription is deprecated - use replace_all_subscriptions")
        await self.replace_all_subscriptions(symbol, [])

    async def replace_options_subscriptions(self, symbols: List[str]):
        """Legacy method - use replace_all_subscriptions for new code"""
        logger.warning("replace_options_subscriptions is deprecated - use replace_all_subscriptions")
        # Use first current stock or SPY as fallback
        current_stock = next(iter(self._stock_subscriptions), "SPY")
        await self.replace_all_subscriptions(current_stock, symbols)

    async def unsubscribe_all(self):
        """Unsubscribe from all current subscriptions"""
        logger.info("🔄 StreamingManager: Unsubscribing from all subscriptions")
        
        # Get all current symbols
        all_current_symbols = []
        all_current_symbols.extend(list(self._stock_subscriptions))
        all_current_symbols.extend(list(self._options_subscriptions))
        
        if all_current_symbols:
            await self._unsubscribe_symbols(all_current_symbols)
        
        # Clear tracking
        self._stock_subscriptions.clear()
        self._options_subscriptions.clear()
        
        logger.info("✅ StreamingManager: All subscriptions cleared")

    async def ensure_persistent_subscriptions(self, data_types: List[str]):
        """Ensure persistent subscriptions are active (orders, positions)"""
        logger.info(f"Ensuring persistent subscriptions for: {data_types}")
        
        config = provider_config_manager.get_config()
        trade_account_provider_name = config.get("trade_account")
        
        if trade_account_provider_name and trade_account_provider_name in self._providers:
            provider = self._providers[trade_account_provider_name]
            
            for data_type in data_types:
                if data_type not in self._persistent_subscriptions:
                    # For persistent subscriptions, we don't need specific symbols
                    # The trade account provider will handle account-level subscriptions
                    if hasattr(provider, 'subscribe_to_account_updates'):
                        await provider.subscribe_to_account_updates([data_type])
                    self._persistent_subscriptions.add(data_type)
                    logger.info(f"Ensured persistent subscription for {data_type} via {trade_account_provider_name}")

    def get_subscription_status(self) -> Dict[str, any]:
        """Get current subscription status for debugging"""
        return {
            "stock_subscriptions": list(self._stock_subscriptions),
            "options_subscriptions": list(self._options_subscriptions),
            "persistent_subscriptions": list(self._persistent_subscriptions),
            "total_subscriptions": {
                data_type: len(symbols) for data_type, symbols in self._subscriptions.items()
            }
        }

    async def _resubscribe_current_symbols(self, provider):
        """Re-subscribe to current symbols after provider reconnection"""
        try:
            # Get all current symbols that should be subscribed
            all_current_symbols = []
            all_current_symbols.extend(list(self._stock_subscriptions))
            all_current_symbols.extend(list(self._options_subscriptions))
            
            if all_current_symbols:
                logger.info(f"🔄 Re-subscribing to {len(all_current_symbols)} symbols after {provider.name} reconnection")
                await provider.subscribe_to_symbols(all_current_symbols)
                
                # Update internal subscriptions tracking
                if "quotes" not in self._subscriptions:
                    self._subscriptions["quotes"] = set()
                self._subscriptions["quotes"].update(all_current_symbols)
                
                logger.info(f"✅ Successfully re-subscribed to {len(all_current_symbols)} symbols")
            else:
                logger.info("No symbols to re-subscribe after reconnection")
                
        except Exception as e:
            logger.error(f"❌ Error re-subscribing to symbols after {provider.name} reconnection: {e}")

    async def get_data(self) -> Optional[MarketData]:
        return await self._streaming_queue.get()

streaming_manager = StreamingManager()
