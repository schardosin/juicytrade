import logging
from typing import Dict, List, Optional, Any

from .providers.base_provider import BaseProvider
from .providers.alpaca_provider import AlpacaProvider
from .providers.public_provider import PublicProvider
from .providers.tradier_provider import TradierProvider
from .providers.tastytrade_provider import TastyTradeProvider
from .provider_config import provider_config_manager
from .provider_credential_store import ProviderCredentialStore
from .provider_types import get_provider_types, validate_credentials, apply_defaults
from .config import settings
from .models import StockQuote, OptionContract, Position, Order, MarketData, SymbolSearchResult, Account, PositionGroup

logger = logging.getLogger(__name__)

class ProviderManager:
    def __init__(self):
        self._providers: Dict[str, BaseProvider] = {}
        self.credential_store = ProviderCredentialStore()
        self._initialize_active_providers()

    def _initialize_active_providers(self):
        """Initialize only active provider instances from credential store"""
        logger.info("🔄 Initializing active provider instances...")
        
        # Clear existing providers
        self._providers.clear()
        
        # Get active instances from credential store
        active_instances = self.credential_store.get_active_instances()
        
        if not active_instances:
            logger.warning("⚠️ No active provider instances found. Please configure provider instances through the API.")
            return
        
        # Initialize each active instance
        for instance_id, instance_data in active_instances.items():
            try:
                provider_type = instance_data.get('provider_type')
                account_type = instance_data.get('account_type')
                credentials = instance_data.get('credentials', {})
                display_name = instance_data.get('display_name', instance_id)
                
                # Apply default values to credentials
                credentials = apply_defaults(provider_type, account_type, credentials)
                
                # Create provider instance
                provider = self._create_provider_instance(provider_type, account_type, credentials)
                
                if provider:
                    self._providers[instance_id] = provider
                    logger.info(f"✅ Initialized provider instance: {display_name} ({instance_id})")
                else:
                    logger.error(f"❌ Failed to create provider instance: {instance_id}")
                    
            except Exception as e:
                logger.error(f"❌ Error initializing provider instance {instance_id}: {e}")
        
        logger.info(f"🎯 Initialized {len(self._providers)} active provider instances")


    def _create_provider_instance(self, provider_type: str, account_type: str, credentials: Dict[str, str]) -> Optional[BaseProvider]:
        """Create a provider instance based on type and credentials"""
        try:
            if provider_type == "alpaca":
                return AlpacaProvider(
                    api_key=credentials.get('api_key'),
                    api_secret=credentials.get('api_secret'),
                    base_url=credentials.get('base_url'),
                    data_url=credentials.get('data_url'),
                    use_paper=(account_type == "paper")
                )
            elif provider_type == "tradier":
                return TradierProvider(
                    account_id=credentials.get('account_id'),
                    api_key=credentials.get('api_key'),
                    base_url=credentials.get('base_url'),
                    stream_url=credentials.get('stream_url')
                )
            elif provider_type == "public":
                return PublicProvider(
                    api_secret=credentials.get('api_secret'),
                    account_id=credentials.get('account_id')
                )
            elif provider_type == "tastytrade":
                return TastyTradeProvider(
                    username=credentials.get('username'),
                    password=credentials.get('password'),
                    account_id=credentials.get('account_id'),
                    base_url=credentials.get('base_url')
                )
            else:
                logger.error(f"❌ Unknown provider type: {provider_type}")
                return None
                
        except Exception as e:
            logger.error(f"❌ Error creating {provider_type} provider: {e}")
            return None

    def get_available_provider_instances(self) -> Dict[str, Dict[str, Any]]:
        """Get all available provider instances with their metadata"""
        instances = {}
        for instance_id, provider in self._providers.items():
            instance_data = self.credential_store.get_instance(instance_id)
            if instance_data:
                instances[instance_id] = {
                    'provider_type': instance_data.get('provider_type'),
                    'account_type': instance_data.get('account_type'),
                    'display_name': instance_data.get('display_name'),
                    'active': instance_data.get('active', False),
                    'created_at': instance_data.get('created_at'),
                    'updated_at': instance_data.get('updated_at')
                }
        return instances

    async def test_provider_connection(self, provider_type: str, account_type: str, credentials: Dict[str, str]) -> Dict[str, Any]:
        """Test a provider connection without saving credentials"""
        try:
            # Apply defaults to credentials
            test_credentials = apply_defaults(provider_type, account_type, credentials)
            
            # Create temporary provider instance
            provider = self._create_provider_instance(provider_type, account_type, test_credentials)
            
            if not provider:
                return {
                    'success': False,
                    'message': f'Failed to create {provider_type} provider instance',
                    'details': {'error': 'Provider creation failed'}
                }
            
            # Test the connection with a simple health check
            health_result = await provider.health_check()
            
            if health_result.get('status') == 'healthy':
                return {
                    'success': True,
                    'message': f'{provider_type.title()} connection successful',
                    'details': health_result
                }
            else:
                return {
                    'success': False,
                    'message': f'{provider_type.title()} connection failed',
                    'details': health_result
                }
                
        except Exception as e:
            logger.error(f"❌ Error testing {provider_type} connection: {e}")
            return {
                'success': False,
                'message': f'Connection test failed: {str(e)}',
                'details': {'error': str(e)}
            }

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

    def get_provider(self, instance_id: str) -> Optional[BaseProvider]:
        """Get a provider instance by its instance ID."""
        provider = self._providers.get(instance_id)
        if not provider:
            logger.error(f"Provider instance '{instance_id}' not found or not initialized.")
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

    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20, type: str = None, underlying_symbol: str = None) -> List[OptionContract]:
        provider = self._get_provider("options_chain")
        if provider:
            return await provider.get_options_chain_basic(symbol, expiry, underlying_price, strike_count, type, underlying_symbol)
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
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.get_positions()
        return []

    async def get_positions_enhanced(self) -> List[PositionGroup]:
        """Get enhanced positions with order chain grouping and strategy detection."""
        provider = self._get_provider("trade_account")
        if provider and hasattr(provider, 'get_positions_enhanced'):
            return await provider.get_positions_enhanced()
        return []

    async def get_orders(self, status: str = "open") -> List[Order]:
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.get_orders(status)
        return []

    async def get_account(self) -> Optional[Account]:
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.get_account()
        return None

    async def place_order(self, order_data: Dict[str, Any]) -> Optional[Order]:
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.place_order(order_data)
        return None

    async def preview_order(self, order_data: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.preview_order(order_data)
        return None

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Optional[Order]:
        provider = self._get_provider("trade_account")
        if provider:
            return await provider.place_multi_leg_order(order_data)
        return None

    async def cancel_order(self, order_id: str) -> bool:
        provider = self._get_provider("trade_account")
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
            # Map weekly symbols to their standard equivalents for historical data
            # Brokers typically don't provide separate historical data for weekly symbols
            chart_symbol = self._map_symbol_for_historical_data(symbol)
            if chart_symbol != symbol:
                logger.info(f"📊 Mapping symbol for historical data: {symbol} → {chart_symbol}")
            return await provider.get_historical_bars(chart_symbol, timeframe, start_date, end_date, limit)
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

    def _map_symbol_for_historical_data(self, symbol: str) -> str:
        """Map symbols to their standard equivalents for historical data."""
        # Weekly symbols that need to be mapped to their standard equivalents
        # Brokers typically don't provide separate historical data for weekly symbols
        weekly_symbol_map = {
            "SPXW": "SPX",  # SPX Weeklys → SPX
            # Add other weekly symbols here if needed
            # "NDXP": "NDX",  # Example: NDX Weeklys (if they exist)
            # "RUTW": "RUT",  # Example: RUT Weeklys (if they exist)
        }
        
        # Return mapped symbol or original if no mapping exists
        return weekly_symbol_map.get(symbol, symbol)

provider_manager = ProviderManager()
