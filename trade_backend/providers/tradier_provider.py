import asyncio
import requests
import websockets
import json
import logging
from typing import List, Dict, Optional, Any
from datetime import datetime

from .base_provider import BaseProvider
from ..models import StockQuote, OptionContract, Position, Order, MarketData

logger = logging.getLogger(__name__)

class TradierProvider(BaseProvider):
    def __init__(self, account_id: str, api_key: str, base_url: str, stream_url: str):
        super().__init__("Tradier")
        self.account_id = account_id
        self.api_key = api_key
        self.base_url = base_url
        self.stream_url = stream_url
        self._session_id = None
        self._stream_connection = None
        self._streaming_queue = asyncio.Queue()
        self._connection_ready = asyncio.Event()

    async def _create_session(self) -> bool:
        url = f"{self.base_url}/v1/markets/events/session"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Accept": "application/json"
        }
        try:
            response = requests.post(url, headers=headers)
            response.raise_for_status()
            if response.text:
                try:
                    data = response.json()
                    self._session_id = data.get("stream", {}).get("sessionid")
                except json.JSONDecodeError:
                    logger.error(f"Failed to decode JSON from Tradier session response: {response.text}")
                    self._session_id = None
            else:
                self._session_id = None
            
            if self._session_id:
                logger.info("Tradier session created successfully.")
                return True
            else:
                logger.error("Failed to create Tradier session: 'sessionid' not in response.")
                return False
        except requests.exceptions.RequestException as e:
            logger.error(f"Error creating Tradier session: {e}")
            return False

    async def connect_streaming(self) -> bool:
        if not await self._create_session():
            return False

        try:
            self._stream_connection = await websockets.connect(self.stream_url)
            logger.info("Connected to Tradier WebSocket.")
            self._connection_ready.set() # Signal that the connection is ready
            asyncio.create_task(self._stream_handler())
            self.is_connected = True
            return True
        except Exception as e:
            logger.error(f"Failed to connect to Tradier WebSocket: {e}")
            return False

    async def _stream_handler(self):
        try:
            async for message in self._stream_connection:
                data = json.loads(message)
                if data['type'] == 'quote':
                    market_data = MarketData(
                        symbol=data['symbol'],
                        data_type='quote',
                        timestamp=str(data['biddate']),
                        data={'bid': data['bid'], 'ask': data['ask']}
                    )
                    await self._streaming_queue.put(market_data)
        except websockets.exceptions.ConnectionClosed as e:
            logger.warning(f"Tradier WebSocket connection closed: {e}")
            self.is_connected = False
        except Exception as e:
            logger.error(f"Error in Tradier stream handler: {e}")
            self.is_connected = False

    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        await self._connection_ready.wait() # Wait until connection is ready
        if not self.is_connected or not self._session_id:
            logger.error("Not connected to Tradier stream. Cannot subscribe.")
            return False

        payload = {
            "symbols": symbols,
            "sessionid": self._session_id,
            "filter": ["quote"] # For now, only quotes
        }
        try:
            await self._stream_connection.send(json.dumps(payload))
            self._subscribed_symbols.update(symbols)
            logger.info(f"Subscribed to {len(symbols)} symbols on Tradier.")
            return True
        except Exception as e:
            logger.error(f"Failed to subscribe to symbols on Tradier: {e}")
            return False

    async def get_streaming_data(self) -> Optional[MarketData]:
        try:
            return await self._streaming_queue.get()
        except asyncio.QueueEmpty:
            return None

    # --- Methods below are not implemented for Tradier yet ---
    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        raise NotImplementedError("get_stock_quote is not implemented for TradierProvider")
    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        raise NotImplementedError("get_stock_quotes is not implemented for TradierProvider")
    async def get_expiration_dates(self, symbol: str) -> List[str]:
        raise NotImplementedError("get_expiration_dates is not implemented for TradierProvider")
    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        raise NotImplementedError("get_options_chain is not implemented for TradierProvider")
    async def get_next_market_date(self) -> str:
        raise NotImplementedError("get_next_market_date is not implemented for TradierProvider")
    async def get_positions(self) -> List[Position]:
        raise NotImplementedError("get_positions is not implemented for TradierProvider")
    async def get_orders(self, status: str = "open") -> List[Order]:
        raise NotImplementedError("get_orders is not implemented for TradierProvider")
    
    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a trading order."""
        try:
            order_url = f"{self.base_url}/v1/accounts/{self.account_id}/orders"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }

            if self._is_option_symbol(order_data["symbol"]):
                 payload = {
                    "class": "option",
                    "symbol": self._parse_option_symbol(order_data["symbol"])["underlying"],
                    "option_symbol": order_data["symbol"],
                    "side": order_data["side"],
                    "quantity": str(order_data["qty"]),
                    "type": order_data["order_type"],
                    "duration": order_data["time_in_force"],
                }
            else:
                payload = {
                    "class": "equity",
                    "symbol": order_data["symbol"],
                    "side": order_data["side"],
                    "quantity": str(order_data["qty"]),
                    "type": order_data["order_type"],
                    "duration": order_data["time_in_force"],
                }

            if order_data.get("limit_price"):
                payload["price"] = f"{order_data['limit_price']:.2f}"

            resp = requests.post(order_url, headers=headers, data=payload)
            resp.raise_for_status()
            order_response = resp.json()
            
            order_id = order_response.get("order", {}).get("id")
            if order_id:
                return Order(
                    id=str(order_id),
                    symbol=order_data["symbol"],
                    asset_class="us_option" if self._is_option_symbol(order_data["symbol"]) else "us_equity",
                    side=order_data["side"],
                    order_type=order_data["order_type"],
                    qty=order_data["qty"],
                    filled_qty=0,
                    limit_price=order_data.get("limit_price"),
                    status="submitted",
                    time_in_force=order_data["time_in_force"],
                    submitted_at=datetime.now().isoformat(),
                )
            else:
                raise Exception(f"Failed to place order: {order_response}")

        except Exception as e:
            self._log_error("place_order", e)
            raise

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a multi-leg trading order."""
        try:
            order_url = f"{self.base_url}/v1/accounts/{self.account_id}/orders"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }

            underlying_symbol = self._parse_option_symbol(order_data["legs"][0]["symbol"])["underlying"]
            
            order_type = "debit" if order_data["limit_price"] > 0 else "credit"

            payload = {
                "class": "multileg",
                "symbol": underlying_symbol,
                "type": order_type,
                "duration": order_data["time_in_force"],
                "price": f"{abs(order_data['limit_price']):.2f}",
            }

            for i, leg in enumerate(order_data["legs"]):
                payload[f"option_symbol[{i}]"] = leg["symbol"]
                payload[f"side[{i}]"] = leg["side"]
                payload[f"quantity[{i}]"] = str(leg["qty"])

            resp = requests.post(order_url, headers=headers, data=payload)
            resp.raise_for_status()
            order_response = resp.json()
            
            order_id = order_response.get("order", {}).get("id")
            if order_id:
                return Order(
                    id=str(order_id),
                    symbol="Multi-leg",
                    asset_class="us_option",
                    side=order_data["order_type"],
                    order_type=order_data["order_type"],
                    qty=order_data["qty"],
                    filled_qty=0,
                    limit_price=order_data["limit_price"],
                    status="submitted",
                    time_in_force=order_data["time_in_force"],
                    submitted_at=datetime.now().isoformat(),
                    legs=order_data["legs"]
                )
            else:
                raise Exception(f"Failed to place multi-leg order: {order_response}")

        except Exception as e:
            self._log_error("place_multi_leg_order", e)
            raise

    async def cancel_order(self, order_id: str) -> bool:
        raise NotImplementedError("cancel_order is not implemented for TradierProvider")
    async def disconnect_streaming(self) -> bool:
        if self._stream_connection:
            await self._stream_connection.close()
            self.is_connected = False
            logger.info("Disconnected from Tradier WebSocket.")
        return True
    async def unsubscribe_from_symbols(self, symbols: List[str]) -> bool:
        # Tradier doesn't support unsubscribing, but we can send a new list of symbols
        current_symbols = self._subscribed_symbols.copy()
        current_symbols.difference_update(symbols)
        return await self.subscribe_to_symbols(list(current_symbols))

    def _parse_option_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Parse option symbol to extract components."""
        try:
            # Find the start of the date part of the symbol
            for i, char in enumerate(symbol):
                if char.isdigit():
                    underlying = symbol[:i]
                    rest = symbol[i:]
                    date_part = rest[:6]
                    option_type = rest[6]
                    strike_part = rest[7:]
                    
                    # Parse expiry date
                    year = 2000 + int(date_part[:2])
                    month = int(date_part[2:4])
                    day = int(date_part[4:6])
                    expiry_date = f"{year}-{month:02d}-{day:02d}"
                    
                    # Parse strike price
                    strike_price = float(strike_part) / 1000
                    
                    return {
                        "underlying": underlying,
                        "type": "call" if option_type == "C" else "put",
                        "strike": strike_price,
                        "expiry": expiry_date
                    }
            return None
        except Exception as e:
            self._log_error(f"parse_option_symbol {symbol}", e)
        return None

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])
