import logging
from typing import Dict, List, Optional, Any

from .providers.base_provider import BaseProvider
from .providers.alpaca_provider import AlpacaProvider
from .providers.public_provider import PublicProvider
from .providers.tradier_provider import TradierProvider
from .provider_config import provider_config_manager
from .config import settings
from .models import StockQuote, OptionContract, Position, Order, MarketData, SymbolSearchResult, Account

logger = logging.getLogger(__name__)

class ProviderManager:
    def __init__(self):
        self._providers: Dict[str, BaseProvider] = {}
        self._initialize_providers()

    def _initialize_providers(self):
        # Initialize Alpaca Live
        try:
            self._providers['alpaca'] = AlpacaProvider(
                api_key=settings.alpaca_api_key_live,
                api_secret=settings.alpaca_api_secret_live,
                base_url=settings.alpaca_base_url_live,
                data_url=settings.alpaca_data_url,
                use_paper=False
            )
            logger.info("Alpaca provider initialized.")
        except Exception as e:
            logger.error(f"Failed to initialize Alpaca provider: {e}")

        # Initialize Alpaca Paper
        try:
            self._providers['alpaca_paper'] = AlpacaProvider(
                api_key=settings.alpaca_api_key_paper,
                api_secret=settings.alpaca_api_secret_paper,
                base_url=settings.alpaca_base_url_paper,
                data_url=settings.alpaca_data_url,
                use_paper=True
            )
            logger.info("Alpaca Paper provider initialized.")
        except Exception as e:
            logger.error(f"Failed to initialize Alpaca Paper provider: {e}")

        # Initialize Public
        try:
            self._providers['public'] = PublicProvider(
                api_secret=settings.public_secret_key,
                account_id=settings.public_account_id
            )
            logger.info("Public provider initialized.")
        except Exception as e:
            logger.error(f"Failed to initialize Public provider: {e}")

        # Initialize Tradier Live
        try:
            self._providers['tradier'] = TradierProvider(
                account_id=settings.tradier_account_id,
                api_key=settings.tradier_secret_key,
                base_url=settings.tradier_base_url_live,
                stream_url=settings.tradier_stream_url_live
            )
            logger.info("Tradier provider initialized.")
        except Exception as e:
            logger.error(f"Failed to initialize Tradier provider: {e}")

        # Initialize Tradier Paper
        try:
            self._providers['tradier_paper'] = TradierProvider(
                account_id=settings.tradier_account_id_paper,
                api_key=settings.tradier_secret_key_paper,
                base_url=settings.tradier_base_url_paper,
                stream_url=settings.tradier_stream_url_paper
            )
            logger.info("Tradier Paper provider initialized.")
        except Exception as e:
            logger.error(f"Failed to initialize Tradier Paper provider: {e}")

    def _get_provider(self, operation: str) -> Optional[BaseProvider]:
        config = provider_config_manager.get_config()
        provider_name = config.get(operation)
        if not provider_name:
            logger.error(f"No provider configured for operation: {operation}")
            return None
        
        provider = self._providers.get(provider_name)
        if not provider:
            logger.error(f"Provider '{provider_name}' not initialized.")
        return provider

    async def get_expiration_dates(self, symbol: str) -> List[str]:
        provider = self._get_provider("expiration_dates")
        if provider:
            return await provider.get_expiration_dates(symbol)
        return []

    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        provider = self._get_provider("stock_quotes")
        if provider:
            return await provider.get_stock_quote(symbol)
        return None

    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        provider = self._get_provider("options_chain")
        if provider:
            return await provider.get_options_chain(symbol, expiry, option_type)
        return []

    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20) -> List[OptionContract]:
        provider = self._get_provider("options_chain")
        if provider:
            return await provider.get_options_chain_basic(symbol, expiry, underlying_price, strike_count)
        return []

    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        provider = self._get_provider("options_chain")
        if provider:
            return await provider.get_options_greeks_batch(option_symbols)
        return {}

    async def get_options_chain_smart(self, symbol: str, expiry: str, underlying_price: float = None, 
                                   atm_range: int = 20, include_greeks: bool = False, 
                                   strikes_only: bool = False) -> List[OptionContract]:
        provider = self._get_provider("options_chain")
        if provider:
            return await provider.get_options_chain_smart(symbol, expiry, underlying_price, atm_range, include_greeks, strikes_only)
        return []

    async def get_positions(self) -> List[Position]:
        provider = self._get_provider("positions")
        if provider:
            return await provider.get_positions()
        return []

    async def get_orders(self, status: str = "open") -> List[Order]:
        provider = self._get_provider("orders")
        if provider:
            return await provider.get_orders(status)
        return []

    async def get_account(self) -> Optional[Account]:
        provider = self._get_provider("account")
        if provider:
            return await provider.get_account()
        return None

    async def place_order(self, order_data: Dict[str, Any]) -> Optional[Order]:
        provider = self._get_provider("orders")
        if provider:
            return await provider.place_order(order_data)
        return None

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Optional[Order]:
        provider = self._get_provider("orders")
        if provider:
            return await provider.place_multi_leg_order(order_data)
        return None

    async def cancel_order(self, order_id: str) -> bool:
        provider = self._get_provider("orders")
        if provider:
            return await provider.cancel_order(order_id)
        return False

    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        provider = self._get_provider("symbol_lookup")
        if provider:
            return await provider.lookup_symbols(query)
        return []

    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        provider = self._get_provider("historical_data")
        if provider:
            return await provider.get_historical_bars(symbol, timeframe, start_date, end_date, limit)
        return []

    async def get_next_market_date(self) -> str:
        provider = self._get_provider("market_calendar")
        if provider:
            return await provider.get_next_market_date()
        return ""
        
    async def health_check(self) -> Dict[str, Any]:
        health_status = {}
        for name, provider in self._providers.items():
            health_status[name] = await provider.health_check()
        return health_status

provider_manager = ProviderManager()
