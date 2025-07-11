import asyncio
import requests
from typing import List, Dict, Optional, Any, Set
from datetime import datetime, date
import logging

from alpaca.data.live import OptionDataStream, StockDataStream
from alpaca.data.historical import StockHistoricalDataClient
from alpaca.data.requests import StockLatestQuoteRequest
from alpaca.trading.client import TradingClient
from alpaca.trading.stream import TradingStream
from alpaca.trading.requests import GetOrdersRequest

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse
)

logger = logging.getLogger(__name__)

class AlpacaProvider(BaseProvider):
    """
    Alpaca implementation of the BaseProvider interface.
    
    This class handles all Alpaca-specific logic and transforms
    Alpaca data into our standardized models.
    """
    
    def __init__(self, api_key_live: str, api_secret_live: str, 
                 api_key_paper: str, api_secret_paper: str,
                 base_url_live: str, base_url_paper: str,
                 use_paper: bool = True):
        super().__init__("Alpaca")
        
        # Store credentials
        self.api_key_live = api_key_live
        self.api_secret_live = api_secret_live
        self.api_key_paper = api_key_paper
        self.api_secret_paper = api_secret_paper
        self.base_url_live = base_url_live
        self.base_url_paper = base_url_paper
        self.use_paper = use_paper
        
        # Initialize clients
        self._init_clients()
        
        # Streaming components
        self.option_stream: Optional[OptionDataStream] = None
        self.stock_stream: Optional[StockDataStream] = None
        self.trading_stream: Optional[TradingStream] = None
        self._streaming_queue = asyncio.Queue()
        
    def _init_clients(self):
        """Initialize Alpaca API clients."""
        try:
            # Historical data client (always use live for market data)
            self.historical_client = StockHistoricalDataClient(
                self.api_key_live, self.api_secret_live
            )
            
            # Trading client (use paper or live based on config)
            if self.use_paper:
                self.trading_client = TradingClient(
                    self.api_key_paper, self.api_secret_paper, paper=True
                )
            else:
                self.trading_client = TradingClient(
                    self.api_key_live, self.api_secret_live, paper=False
                )
                
            self._log_info("Clients initialized successfully")
        except Exception as e:
            self._log_error("client initialization", e)
            raise
    
    def _get_api_credentials(self) -> tuple:
        """Get appropriate API credentials based on paper/live setting."""
        if self.use_paper:
            return self.api_key_paper, self.api_secret_paper
        else:
            return self.api_key_live, self.api_secret_live
    
    def _get_base_url(self) -> str:
        """Get appropriate base URL based on paper/live setting."""
        return self.base_url_paper if self.use_paper else self.base_url_live
    
    # === Market Data Methods ===
    
    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        """Get latest stock quote for a symbol."""
        try:
            request_params = StockLatestQuoteRequest(symbol_or_symbols=[symbol])
            latest_quotes = self.historical_client.get_stock_latest_quote(request_params)
            
            if symbol in latest_quotes:
                quote = latest_quotes[symbol]
                return StockQuote(
                    symbol=symbol,
                    ask=float(quote.ask_price) if quote.ask_price else None,
                    bid=float(quote.bid_price) if quote.bid_price else None,
                    timestamp=datetime.now().isoformat()
                )
            return None
        except Exception as e:
            self._log_error(f"get_stock_quote for {symbol}", e)
            return None
    
    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        """Get stock quotes for multiple symbols."""
        try:
            request_params = StockLatestQuoteRequest(symbol_or_symbols=symbols)
            latest_quotes = self.historical_client.get_stock_latest_quote(request_params)
            
            result = {}
            for symbol, quote in latest_quotes.items():
                result[symbol] = StockQuote(
                    symbol=symbol,
                    ask=float(quote.ask_price) if quote.ask_price else None,
                    bid=float(quote.bid_price) if quote.bid_price else None,
                    timestamp=datetime.now().isoformat()
                )
            return result
        except Exception as e:
            self._log_error(f"get_stock_quotes for {symbols}", e)
            return {}
    
    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """Get available expiration dates for options."""
        try:
            url = f"{self._get_base_url()}/v2/options/contracts"
            api_key, api_secret = self._get_api_credentials()
            
            params = {
                "underlying_symbols": symbol,
                "limit": 1000
            }
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                contracts = data.get("option_contracts", [])
                
                # Extract unique expiration dates
                expiration_dates = set()
                for contract in contracts:
                    if contract.get("expiration_date"):
                        expiration_dates.add(contract["expiration_date"])
                
                return sorted(list(expiration_dates))
            else:
                self._log_error(f"get_expiration_dates API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return []
                
        except Exception as e:
            self._log_error(f"get_expiration_dates for {symbol}", e)
            return []
    
    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        """Get options chain for symbol and expiration."""
        try:
            url = f"{self._get_base_url()}/v2/options/contracts"
            api_key, api_secret = self._get_api_credentials()
            
            params = {
                "underlying_symbols": symbol,
                "expiration_date": expiry,
                "root_symbol": symbol,
                "limit": 1000
            }
            
            if option_type:
                params["type"] = option_type
            
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                contracts = data.get("option_contracts", [])
                
                result = []
                for contract in contracts:
                    try:
                        option_contract = self._transform_option_contract(contract)
                        if option_contract:
                            result.append(option_contract)
                    except Exception as e:
                        self._log_error(f"transform_option_contract", e)
                        continue
                
                return result
            else:
                self._log_error(f"get_options_chain API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return []
                
        except Exception as e:
            self._log_error(f"get_options_chain for {symbol} {expiry}", e)
            return []
    
    async def get_next_market_date(self) -> str:
        """Get next market trading date."""
        try:
            url = f"{self._get_base_url()}/v2/calendar"
            api_key, api_secret = self._get_api_credentials()
            
            params = {
                "start": date.today().strftime("%Y-%m-%d"),
                "end": (date.today().replace(year=date.today().year + 1)).strftime("%Y-%m-%d")
            }
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                for day in data:
                    if day.get("date"):
                        market_date = day["date"]
                        if market_date >= date.today().strftime("%Y-%m-%d"):
                            return market_date
            
            # Fallback to today
            return date.today().strftime("%Y-%m-%d")
            
        except Exception as e:
            self._log_error("get_next_market_date", e)
            return date.today().strftime("%Y-%m-%d")
    
    # === Account & Portfolio Methods ===
    
    async def get_positions(self) -> List[Position]:
        """Get all current positions."""
        try:
            positions = self.trading_client.get_all_positions()
            
            result = []
            for position in positions:
                try:
                    transformed_position = self._transform_position(position)
                    if transformed_position:
                        result.append(transformed_position)
                except Exception as e:
                    self._log_error(f"transform_position for {position.symbol}", e)
                    continue
            
            return result
        except Exception as e:
            self._log_error("get_positions", e)
            return []
    
    async def get_orders(self, status: str = "open") -> List[Order]:
        """Get orders with status filter."""
        try:
            # Map our status to Alpaca's status
            if status == "open":
                alpaca_status = "open"
            elif status == "filled":
                alpaca_status = "closed"
            elif status == "canceled":
                alpaca_status = "closed"
            elif status == "all":
                alpaca_status = "all"
            else:
                alpaca_status = "open"
            
            try:
                request_params = {
                    "status": alpaca_status,
                    "limit": 1000,
                    "nested": True
                }
                request = GetOrdersRequest(**request_params)
                orders = self.trading_client.get_orders(filter=request)
            except Exception:
                # Fallback to simple get_orders
                orders = self.trading_client.get_orders()
            
            # Apply additional filtering if needed
            if status == "filled":
                filter_statuses = ["filled"]
                filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
            elif status == "canceled":
                filter_statuses = ["canceled", "cancelled", "expired", "rejected"]
                filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
            elif status == "open":
                filter_statuses = ["new", "accepted", "pending_new", "partially_filled", "held"]
                filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
            else:
                filtered_orders = orders
            
            result = []
            for order in filtered_orders:
                try:
                    transformed_order = self._transform_order(order)
                    if transformed_order:
                        result.append(transformed_order)
                except Exception as e:
                    self._log_error(f"transform_order for {order.id}", e)
                    continue
            
            return result
        except Exception as e:
            self._log_error(f"get_orders with status {status}", e)
            return []
    
    # === Order Management Methods ===
    
    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a trading order."""
        try:
            # This would implement order placement logic
            # For now, we'll raise NotImplementedError
            raise NotImplementedError("Order placement not yet implemented")
        except Exception as e:
            self._log_error("place_order", e)
            raise
    
    async def cancel_order(self, order_id: str) -> bool:
        """Cancel an existing order."""
        try:
            self.trading_client.cancel_order_by_id(order_id)
            return True
        except Exception as e:
            self._log_error(f"cancel_order {order_id}", e)
            return False
    
    # === Streaming Methods ===
    
    async def connect_streaming(self) -> bool:
        """Connect to Alpaca streaming services."""
        try:
            # Initialize streaming clients
            self.option_stream = OptionDataStream(self.api_key_live, self.api_secret_live)
            self.stock_stream = StockDataStream(self.api_key_live, self.api_secret_live)
            self.trading_stream = TradingStream(
                self.api_key_paper if self.use_paper else self.api_key_live,
                self.api_secret_paper if self.use_paper else self.api_secret_live,
                paper=self.use_paper
            )
            
            self.is_connected = True
            self._log_info("Streaming connection established")
            return True
        except Exception as e:
            self._log_error("connect_streaming", e)
            return False
    
    async def disconnect_streaming(self) -> bool:
        """Disconnect from streaming services."""
        try:
            if self.option_stream:
                await self.option_stream.close()
            if self.stock_stream:
                await self.stock_stream.close()
            if self.trading_stream:
                await self.trading_stream.close()
            
            self.is_connected = False
            self._log_info("Streaming connection closed")
            return True
        except Exception as e:
            self._log_error("disconnect_streaming", e)
            return False
    
    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """Subscribe to real-time data for symbols."""
        try:
            if not self.is_connected:
                await self.connect_streaming()
            
            # Add symbols to our tracking set
            self._subscribed_symbols.update(symbols)
            
            # Subscribe to stock quotes
            if self.stock_stream:
                for symbol in symbols:
                    if not self._is_option_symbol(symbol):
                        self.stock_stream.subscribe_quotes(self._stock_quote_handler, symbol)
            
            # Subscribe to option quotes
            if self.option_stream:
                for symbol in symbols:
                    if self._is_option_symbol(symbol):
                        self.option_stream.subscribe_quotes(self._option_quote_handler, symbol)
            
            self._log_info(f"Subscribed to {len(symbols)} symbols")
            return True
        except Exception as e:
            self._log_error(f"subscribe_to_symbols {symbols}", e)
            return False
    
    async def unsubscribe_from_symbols(self, symbols: List[str]) -> bool:
        """Unsubscribe from real-time data for symbols."""
        try:
            # Remove symbols from our tracking set
            self._subscribed_symbols.difference_update(symbols)
            
            # Note: Alpaca doesn't have direct unsubscribe methods,
            # so we'd need to restart the streams with the new symbol list
            self._log_info(f"Unsubscribed from {len(symbols)} symbols")
            return True
        except Exception as e:
            self._log_error(f"unsubscribe_from_symbols {symbols}", e)
            return False
    
    async def get_streaming_data(self) -> Optional[MarketData]:
        """Get next streaming market data."""
        try:
            if not self._streaming_queue.empty():
                return await self._streaming_queue.get()
            return None
        except Exception as e:
            self._log_error("get_streaming_data", e)
            return None
    
    # === Data Transformation Methods ===
    
    def _transform_option_contract(self, raw_contract: Dict[str, Any]) -> Optional[OptionContract]:
        """Transform Alpaca option contract to our standard model."""
        try:
            return OptionContract(
                symbol=raw_contract.get("symbol", ""),
                underlying_symbol=raw_contract.get("underlying_symbol", ""),
                expiration_date=raw_contract.get("expiration_date", ""),
                strike_price=float(raw_contract.get("strike_price", 0)),
                type=raw_contract.get("type", "").lower(),
                bid=float(raw_contract.get("bid", 0)) if raw_contract.get("bid") else None,
                ask=float(raw_contract.get("ask", 0)) if raw_contract.get("ask") else None,
                close_price=float(raw_contract.get("close_price", 0)) if raw_contract.get("close_price") else None,
                volume=int(raw_contract.get("volume", 0)) if raw_contract.get("volume") else None,
                open_interest=int(raw_contract.get("open_interest", 0)) if raw_contract.get("open_interest") else None,
                implied_volatility=float(raw_contract.get("implied_volatility", 0)) if raw_contract.get("implied_volatility") else None,
                delta=float(raw_contract.get("delta", 0)) if raw_contract.get("delta") else None,
                gamma=float(raw_contract.get("gamma", 0)) if raw_contract.get("gamma") else None,
                theta=float(raw_contract.get("theta", 0)) if raw_contract.get("theta") else None,
                vega=float(raw_contract.get("vega", 0)) if raw_contract.get("vega") else None,
            )
        except Exception as e:
            self._log_error("transform_option_contract", e)
            return None
    
    def _transform_position(self, raw_position) -> Optional[Position]:
        """Transform Alpaca position to our standard model."""
        try:
            position = Position(
                symbol=raw_position.symbol,
                qty=float(raw_position.qty),
                side="long" if float(raw_position.qty) > 0 else "short",
                market_value=float(raw_position.market_value) if raw_position.market_value else 0,
                cost_basis=float(raw_position.cost_basis) if raw_position.cost_basis else 0,
                unrealized_pl=float(raw_position.unrealized_pl) if raw_position.unrealized_pl else 0,
                unrealized_plpc=float(raw_position.unrealized_plpc) if raw_position.unrealized_plpc else None,
                current_price=float(raw_position.current_price) if raw_position.current_price else 0,
                avg_entry_price=float(raw_position.avg_entry_price) if raw_position.avg_entry_price else 0,
                asset_class=raw_position.asset_class.value if raw_position.asset_class else "unknown"
            )
            
            # Parse option-specific information if it's an option
            if raw_position.asset_class and raw_position.asset_class.value == "us_option":
                option_info = self._parse_option_symbol(raw_position.symbol)
                if option_info:
                    position.underlying_symbol = option_info["underlying"]
                    position.option_type = option_info["type"]
                    position.strike_price = option_info["strike"]
                    position.expiry_date = option_info["expiry"]
            
            return position
        except Exception as e:
            self._log_error("transform_position", e)
            return None
    
    def _transform_order(self, raw_order) -> Optional[Order]:
        """Transform Alpaca order to our standard model."""
        try:
            return Order(
                id=str(raw_order.id),
                symbol=raw_order.symbol or "Multi-leg",
                asset_class=raw_order.asset_class.value if raw_order.asset_class else "unknown",
                side=raw_order.side.value if hasattr(raw_order.side, 'value') else str(raw_order.side),
                order_type=raw_order.order_type.value if hasattr(raw_order.order_type, 'value') else str(raw_order.order_type),
                qty=float(raw_order.qty),
                filled_qty=float(raw_order.filled_qty) if raw_order.filled_qty else 0.0,
                limit_price=float(raw_order.limit_price) if raw_order.limit_price else None,
                stop_price=float(raw_order.stop_price) if raw_order.stop_price else None,
                avg_fill_price=float(raw_order.filled_avg_price) if raw_order.filled_avg_price else None,
                status=raw_order.status.value if hasattr(raw_order.status, 'value') else str(raw_order.status),
                time_in_force=raw_order.time_in_force.value if hasattr(raw_order.time_in_force, 'value') else str(raw_order.time_in_force),
                submitted_at=raw_order.submitted_at.isoformat() if raw_order.submitted_at else datetime.now().isoformat(),
                filled_at=raw_order.filled_at.isoformat() if raw_order.filled_at else None,
                legs=[self._transform_order_leg(leg) for leg in raw_order.legs] if hasattr(raw_order, 'legs') and raw_order.legs else None
            )
        except Exception as e:
            self._log_error("transform_order", e)
            return None
    
    def _transform_order_leg(self, raw_leg) -> Dict[str, Any]:
        """Transform Alpaca order leg to our standard format."""
        return {
            "symbol": raw_leg.symbol,
            "side": raw_leg.side.value if hasattr(raw_leg.side, 'value') else str(raw_leg.side),
            "qty": float(raw_leg.qty)
        }
    
    # === Helper Methods ===
    
    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])
    
    def _parse_option_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Parse option symbol to extract components."""
        try:
            if len(symbol) >= 15:
                underlying = symbol[:3]
                date_part = symbol[3:9]
                option_type = symbol[9]
                strike_part = symbol[10:]
                
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
        except Exception as e:
            self._log_error(f"parse_option_symbol {symbol}", e)
        return None
    
    # === Streaming Handlers ===
    
    async def _stock_quote_handler(self, data):
        """Handle incoming stock quote data."""
        try:
            market_data = MarketData(
                symbol=data.symbol,
                data_type="stock_quote",
                timestamp=datetime.now().isoformat(),
                data={
                    "ask": data.ask_price,
                    "bid": data.bid_price
                }
            )
            await self._streaming_queue.put(market_data)
        except Exception as e:
            self._log_error("stock_quote_handler", e)
    
    async def _option_quote_handler(self, data):
        """Handle incoming option quote data."""
        try:
            market_data = MarketData(
                symbol=data.symbol,
                data_type="option_quote",
                timestamp=datetime.now().isoformat(),
                data={
                    "ask": data.ask_price,
                    "bid": data.bid_price
                }
            )
            await self._streaming_queue.put(market_data)
        except Exception as e:
            self._log_error("option_quote_handler", e)
