import asyncio
import requests
from typing import List, Dict, Optional, Any, Set
from datetime import datetime, date, timedelta
import logging
import json
import os

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult
)
from ..config import settings

logger = logging.getLogger(__name__)

class PublicProvider(BaseProvider):
    """
    Public.com implementation of the BaseProvider interface.
    """

    def __init__(self, api_secret: str, account_id: str, base_url: str = "https://api.public.com"):
        super().__init__("Public")
        self.api_secret = api_secret
        self.account_id = account_id
        self.base_url = base_url
        self._access_token = None
        self._token_expiry = None

    async def _get_access_token(self) -> Optional[str]:
        """Get a new access token if needed."""
        if self._access_token and self._token_expiry and datetime.now() < self._token_expiry:
            return self._access_token

        url = f"{self.base_url}/userapiauthservice/personal/access-tokens"
        headers = {'Content-Type': 'application/json'}
        data = {
            "validityInMinutes": 1440,
            "secret": self.api_secret
        }
        try:
            response = requests.post(url, headers=headers, data=json.dumps(data))
            response.raise_for_status()
            token_data = response.json()
            self._access_token = token_data.get("accessToken")
            self._token_expiry = datetime.now() + timedelta(minutes=1430) # 10 min buffer
            self._log_info("Successfully obtained new Public API access token.")
            return self._access_token
        except requests.exceptions.RequestException as e:
            self._log_error("get_access_token", e)
            return None

    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """Get available expiration dates for options on a symbol."""
        token = await self._get_access_token()
        if not token:
            return []

        url = f"{self.base_url}/userapigateway/marketdata/{self.account_id}/option-expirations"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        data = {
            "instrument": {
                "symbol": symbol,
                "type": "EQUITY"
            }
        }
        try:
            response = requests.post(url, headers=headers, json=data)
            response.raise_for_status()
            expirations_data = response.json()
            return expirations_data.get("expirations", [])
        except requests.exceptions.RequestException as e:
            self._log_error(f"get_expiration_dates for {symbol}", e)
            return []

    # --- Methods below are not implemented for Public.com yet ---

    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        raise NotImplementedError("get_stock_quote is not implemented for PublicProvider")

    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        raise NotImplementedError("get_stock_quotes is not implemented for PublicProvider")

    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        raise NotImplementedError("get_options_chain is not implemented for PublicProvider")
    
    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20) -> List[OptionContract]:
        """
        Get basic options chain (no Greeks) for fast loading, ATM-focused.
        
        Args:
            symbol: Underlying symbol
            expiry: Expiration date in YYYY-MM-DD format
            underlying_price: Current underlying price for ATM filtering
            strike_count: Number of strikes around ATM to include (default 20)
            
        Returns:
            List of OptionContract objects without Greeks
        """
        raise NotImplementedError("get_options_chain_basic is not implemented for PublicProvider")
    
    async def get_options_chain_smart(self, symbol: str, expiry: str, underlying_price: float = None, 
                                   atm_range: int = 20, include_greeks: bool = False, 
                                   strikes_only: bool = False) -> List[OptionContract]:
        """
        Get smart options chain with configurable loading.
        
        Args:
            symbol: Underlying symbol
            expiry: Expiration date in YYYY-MM-DD format
            underlying_price: Current underlying price for ATM filtering
            atm_range: Range around ATM to include
            include_greeks: Whether to include Greeks calculation
            strikes_only: Whether to return only strike information
            
        Returns:
            List of OptionContract objects
        """
        raise NotImplementedError("get_options_chain_smart is not implemented for PublicProvider")
    
    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """
        Get Greeks for multiple option symbols in batch.
        
        Args:
            option_symbols: List of option symbols
            
        Returns:
            Dictionary mapping option symbols to Greeks data
        """
        raise NotImplementedError("get_options_greeks_batch is not implemented for PublicProvider")

    async def get_next_market_date(self) -> str:
        raise NotImplementedError("get_next_market_date is not implemented for PublicProvider")

    async def get_positions(self) -> List[Position]:
        raise NotImplementedError("get_positions is not implemented for PublicProvider")

    async def get_orders(self, status: str = "open") -> List[Order]:
        raise NotImplementedError("get_orders is not implemented for PublicProvider")
    
    async def get_account(self) -> Optional['Account']:
        """
        Get account information including balance and buying power.
        
        Returns:
            Account object with account details or None if not available
        """
        raise NotImplementedError("get_account is not implemented for PublicProvider")
    
    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """
        Get historical OHLCV bars for charting.
        
        Args:
            symbol: Stock symbol (e.g., "AAPL")
            timeframe: Time interval ("1m", "5m", "15m", "30m", "1h", "4h", "D", "W", "M")
            start_date: Start date in YYYY-MM-DD format (optional)
            end_date: End date in YYYY-MM-DD format (optional)
            limit: Maximum number of bars to return (default 500)
            
        Returns:
            List of OHLCV dictionaries in Lightweight Charts format:
            [{"time": "2024-01-15", "open": 150.25, "high": 152.80, "low": 149.90, "close": 151.45, "volume": 1234567}]
        """
        raise NotImplementedError("get_historical_bars is not implemented for PublicProvider")

    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        raise NotImplementedError("place_order is not implemented for PublicProvider")

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        raise NotImplementedError("place_multi_leg_order is not implemented for PublicProvider")

    async def cancel_order(self, order_id: str) -> bool:
        raise NotImplementedError("cancel_order is not implemented for PublicProvider")

    async def connect_streaming(self) -> bool:
        self._log_info("Streaming is not supported by PublicProvider.")
        return False

    async def disconnect_streaming(self) -> bool:
        return True

    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        self._log_info("Streaming is not supported by PublicProvider.")
        return False

    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        return True

    async def get_streaming_data(self) -> Optional[MarketData]:
        return None

    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query."""
        raise NotImplementedError("Symbol lookup not implemented for Public provider")
