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
    ExpirationDate, MarketData, ApiResponse
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

    async def get_next_market_date(self) -> str:
        raise NotImplementedError("get_next_market_date is not implemented for PublicProvider")

    async def get_positions(self) -> List[Position]:
        raise NotImplementedError("get_positions is not implemented for PublicProvider")

    async def get_orders(self, status: str = "open") -> List[Order]:
        raise NotImplementedError("get_orders is not implemented for PublicProvider")

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

    async def unsubscribe_from_symbols(self, symbols: List[str]) -> bool:
        return True

    async def get_streaming_data(self) -> Optional[MarketData]:
        return None
