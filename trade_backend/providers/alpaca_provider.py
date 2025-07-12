import asyncio
import requests
from typing import List, Dict, Optional, Any, Set
from datetime import datetime, date
import logging
import json
import os

from alpaca.data.live import OptionDataStream, StockDataStream
from alpaca.data.enums import DataFeed
from alpaca.data.historical import StockHistoricalDataClient
from alpaca.data.requests import StockLatestQuoteRequest
from alpaca.trading.client import TradingClient
from alpaca.trading.stream import TradingStream
from alpaca.trading.requests import (
    GetOrdersRequest, GetOptionContractsRequest, MarketOrderRequest, LimitOrderRequest, OptionLegRequest
)
from alpaca.trading.enums import OrderSide, TimeInForce, OrderClass
from alpaca.trading.models import Order as AlpacaOrder

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
    
    def __init__(self, api_key: str, api_secret: str, 
                 base_url: str, data_url: str, use_paper: bool = True):
        super().__init__("Alpaca")
        
        # Store credentials
        self.api_key = api_key
        self.api_secret = api_secret
        self.base_url = base_url
        self.data_url = data_url
        self.use_paper = use_paper
        
        # Initialize clients
        self._init_clients()
        
        # Streaming components
        self.option_stream: Optional[OptionDataStream] = None
        self.stock_stream: Optional[StockDataStream] = None
        self.trading_stream: Optional[TradingStream] = None
        self._streaming_queue = asyncio.Queue()
        
        # Quote cache for after-hours pricing
        self._quote_cache: Dict[str, Dict[str, Any]] = {}
        self._cache_ttl_minutes = 15  # Cache TTL in minutes
        
        # Expiration dates cache (daily TTL)
        self._expiration_cache: Dict[str, Dict[str, Any]] = {}
        self._cache_file_path = "cache/alpaca_expiration_cache.json"
        
        # Load cache from disk on startup
        self._load_cache_from_disk()
        
    def _init_clients(self):
        """Initialize Alpaca API clients."""
        try:
            # Historical data client (always use live for market data)
            self.historical_client = StockHistoricalDataClient(
                self.api_key, self.api_secret
            )
            
            # Trading client (use paper or live based on config)
            self.trading_client = TradingClient(
                self.api_key, self.api_secret, paper=self.use_paper
            )
                
            self._log_info("Clients initialized successfully")
        except Exception as e:
            self._log_error("client initialization", e)
            raise
    
    def _get_api_credentials(self) -> tuple:
        """Get appropriate API credentials based on paper/live setting."""
        return self.api_key, self.api_secret
    
    def _get_base_url(self) -> str:
        """Get appropriate base URL based on paper/live setting."""
        return self.base_url
    
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
        """Get available expiration dates for options using snapshots endpoint with daily caching."""
        try:
            # Check cache first
            cached_dates = self._get_cached_expiration_dates(symbol)
            if cached_dates:
                self._log_info(f"Using cached expiration dates for {symbol} ({len(cached_dates)} dates)")
                return cached_dates
            
            # Cache miss - fetch from API
            self._log_info(f"Cache miss - fetching expiration dates for {symbol} from API")
            
            expiration_dates = set()
            page_token = None
            page_count = 0
            
            while True:
                page_count += 1
                # Use the data API snapshots endpoint
                url = f"{self.data_url}/v1beta1/options/snapshots/{symbol}"
                api_key, api_secret = self._get_api_credentials()
                
                params = {
                    "type": "call"  # Only fetch calls to reduce data volume - expiration dates are the same for calls and puts
                }
                if page_token:
                    params["page_token"] = page_token
                
                headers = {
                    "APCA-API-KEY-ID": api_key,
                    "APCA-API-SECRET-KEY": api_secret,
                    "accept": "application/json"
                }
                
                response = requests.get(url, headers=headers, params=params)
                
                if response.status_code == 200:
                    data = response.json()
                    snapshots = data.get("snapshots", {})
                    
                    self._log_info(f"Page {page_count}: Retrieved {len(snapshots)} option snapshots for {symbol}")
                    
                    # Extract expiration dates from option symbols
                    page_dates = set()
                    for option_symbol in snapshots.keys():
                        expiry_date = self._extract_expiry_from_symbol(option_symbol)
                        if expiry_date:
                            expiration_dates.add(expiry_date)
                            page_dates.add(expiry_date)
                    
                    self._log_info(f"Page {page_count}: Found {len(page_dates)} unique expiration dates")
                    
                    # Check for next page token
                    next_page_token = data.get("next_page_token")
                    if next_page_token:
                        page_token = next_page_token
                        self._log_info(f"Found next page token, continuing to page {page_count + 1}")
                    else:
                        self._log_info("No more pages available")
                        break
                else:
                    self._log_error(f"get_expiration_dates API call page {page_count}", 
                                  Exception(f"HTTP {response.status_code}: {response.text}"))
                    break
            
            sorted_dates = sorted(list(expiration_dates))
            self._log_info(f"Retrieved {len(sorted_dates)} total expiration dates for {symbol} across {page_count} pages")
            
            # Cache the results for daily use
            self._cache_expiration_dates(symbol, sorted_dates)
            
            return sorted_dates
                
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
                
                # First pass: transform contracts and identify missing quotes
                result = []
                symbols_needing_quotes = []
                
                for contract in contracts:
                    try:
                        option_contract = self._transform_option_contract(contract)
                        if option_contract:
                            result.append(option_contract)
                            
                            # Check if bid/ask are missing and add to symbols needing quotes
                            if (option_contract.bid is None or option_contract.ask is None) and option_contract.symbol:
                                symbols_needing_quotes.append(option_contract.symbol)
                                
                    except Exception as e:
                        self._log_error(f"transform_option_contract", e)
                        continue
                
                # Second pass: fetch latest quotes for contracts with missing bid/ask (only when market is closed)
                if symbols_needing_quotes:
                    market_open = await self.is_market_open()
                    
                    if not market_open:
                        # Market is closed - fetch latest quotes to improve pricing accuracy
                        self._log_info(f"Market closed - fetching latest quotes for {len(symbols_needing_quotes)} option contracts")
                        latest_quotes = await self.get_latest_option_quotes(symbols_needing_quotes)
                        
                        # Update contracts with latest quote data
                        updated_count = 0
                        for contract in result:
                            if contract.symbol in latest_quotes:
                                quote_data = latest_quotes[contract.symbol]
                                
                                # Update bid/ask if they were missing and we have fresh data
                                if contract.bid is None and quote_data.get("bid") is not None:
                                    contract.bid = quote_data["bid"]
                                    updated_count += 1
                                if contract.ask is None and quote_data.get("ask") is not None:
                                    contract.ask = quote_data["ask"]
                        
                        self._log_info(f"Updated {updated_count} contracts with latest quotes")
                    else:
                        # Market is open - rely on websocket stream for live pricing
                        self._log_info(f"Market open - skipping quote fetch for {len(symbols_needing_quotes)} contracts (relying on live stream)")
                
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
    
    async def is_market_open(self) -> bool:
        """Check if the market is currently open."""
        try:
            url = f"{self._get_base_url()}/v2/calendar"
            api_key, api_secret = self._get_api_credentials()
            
            today = date.today().strftime("%Y%m%d")  # Format: YYYYMMDD as shown in your example
            params = {
                "start": today,
                "end": today
            }
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                if data and len(data) > 0:
                    market_day = data[0]
                    
                    # Check if today is a trading day
                    today_formatted = date.today().strftime("%Y-%m-%d")
                    if market_day.get("date") != today_formatted:
                        return False
                    
                    # Get market hours (format: "09:30", "16:00" in EST)
                    market_open = market_day.get("open")  # e.g., "09:30"
                    market_close = market_day.get("close")  # e.g., "16:00"
                    
                    if market_open and market_close:
                        from datetime import datetime, time
                        import pytz
                        
                        # Get current time in EST
                        est = pytz.timezone('US/Eastern')
                        now_est = datetime.now(est)
                        current_time = now_est.time()
                        
                        # Parse market open/close times (EST)
                        open_hour, open_minute = map(int, market_open.split(':'))
                        close_hour, close_minute = map(int, market_close.split(':'))
                        
                        open_time = time(open_hour, open_minute)
                        close_time = time(close_hour, close_minute)
                        
                        # Check if current time is within market hours
                        is_open = open_time <= current_time <= close_time
                        
                        self._log_info(f"Market status check: Current EST time {current_time}, Market hours {open_time}-{close_time}, Open: {is_open}")
                        return is_open
                
                return False  # Not a trading day
            else:
                # If we can't determine market status, assume closed for safety
                self._log_error("is_market_open API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return False
                
        except Exception as e:
            self._log_error("is_market_open", e)
            # If we can't determine market status, assume closed for safety
            return False
    
    async def get_latest_option_quotes(self, symbols: List[str]) -> Dict[str, Dict[str, float]]:
        """Get latest option quotes for multiple symbols."""
        try:
            if not symbols:
                return {}
            
            # Check if market is open - if closed, use cache for efficiency
            market_open = await self.is_market_open()
            
            if not market_open:
                # Market is closed - check cache first for stable after-hours quotes
                cached_results = {}
                symbols_to_fetch = []
                
                for symbol in symbols:
                    cached_quote = self._get_cached_quote(symbol)
                    if cached_quote:
                        cached_results[symbol] = cached_quote
                    else:
                        symbols_to_fetch.append(symbol)
                
                if cached_results:
                    self._log_info(f"Market closed - using cached quotes for {len(cached_results)} symbols")
                
                # Only fetch fresh data for symbols not in cache
                if symbols_to_fetch:
                    fresh_results = await self._fetch_fresh_quotes(symbols_to_fetch)
                    # Cache the fresh results since market is closed
                    self._cache_quotes(fresh_results)
                    cached_results.update(fresh_results)
                
                return cached_results
            else:
                # Market is open - always fetch fresh data (no caching during trading hours)
                self._log_info(f"Market open - fetching fresh quotes for {len(symbols)} symbols")
                return await self._fetch_fresh_quotes(symbols)
                
        except Exception as e:
            self._log_error(f"get_latest_option_quotes for {len(symbols)} symbols", e)
            return {}
    
    async def _fetch_fresh_quotes(self, symbols: List[str]) -> Dict[str, Dict[str, float]]:
        """Fetch fresh quotes from API (no caching)."""
        try:
            if not symbols:
                return {}
            
            # Alpaca API has a limit of 100 symbols per request, so we need to batch
            batch_size = 100
            all_results = {}
            
            # Process symbols in batches
            for i in range(0, len(symbols), batch_size):
                batch_symbols = symbols[i:i + batch_size]
                batch_result = await self._get_latest_option_quotes_batch(batch_symbols)
                all_results.update(batch_result)
            
            self._log_info(f"Fetched fresh quotes for {len(all_results)} option symbols across {len(range(0, len(symbols), batch_size))} batches")
            return all_results
                
        except Exception as e:
            self._log_error(f"_fetch_fresh_quotes for {len(symbols)} symbols", e)
            return {}
    
    def _get_cached_quote(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Get cached quote if available and not expired."""
        try:
            if symbol in self._quote_cache:
                cached_data = self._quote_cache[symbol]
                cached_time = datetime.fromisoformat(cached_data.get("cached_at", ""))
                
                # Check if cache is still valid (within TTL)
                time_diff = datetime.now() - cached_time
                if time_diff.total_seconds() < (self._cache_ttl_minutes * 60):
                    return {
                        "bid": cached_data.get("bid"),
                        "ask": cached_data.get("ask"),
                        "bid_size": cached_data.get("bid_size"),
                        "ask_size": cached_data.get("ask_size"),
                        "timestamp": cached_data.get("timestamp")
                    }
                else:
                    # Cache expired, remove it
                    del self._quote_cache[symbol]
            
            return None
        except Exception as e:
            self._log_error(f"_get_cached_quote for {symbol}", e)
            return None
    
    def _cache_quotes(self, quotes: Dict[str, Dict[str, Any]]) -> None:
        """Cache quotes with timestamp."""
        try:
            current_time = datetime.now().isoformat()
            
            for symbol, quote_data in quotes.items():
                self._quote_cache[symbol] = {
                    **quote_data,
                    "cached_at": current_time
                }
            
            self._log_info(f"Cached {len(quotes)} quotes for after-hours use")
        except Exception as e:
            self._log_error("_cache_quotes", e)
    
    async def _get_latest_option_quotes_batch(self, symbols: List[str]) -> Dict[str, Dict[str, float]]:
        """Get latest option quotes for a batch of symbols (max 100)."""
        try:
            if not symbols:
                return {}
            
            # Use the data API URL from configuration
            url = f"{self.data_url}/v1beta1/options/quotes/latest"
            api_key, api_secret = self._get_api_credentials()
            
            # Join symbols with comma for the API call
            symbols_param = ",".join(symbols)
            
            params = {
                "symbols": symbols_param
            }
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                quotes = data.get("quotes", {})
                
                result = {}
                for symbol, quote_data in quotes.items():
                    if quote_data:
                        result[symbol] = {
                            "bid": float(quote_data.get("bp", 0)) if quote_data.get("bp") else None,
                            "ask": float(quote_data.get("ap", 0)) if quote_data.get("ap") else None,
                            "bid_size": int(quote_data.get("bs", 0)) if quote_data.get("bs") else None,
                            "ask_size": int(quote_data.get("as", 0)) if quote_data.get("as") else None,
                            "timestamp": quote_data.get("t", "")
                        }
                
                return result
            else:
                self._log_error(f"get_latest_option_quotes_batch API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return {}
                
        except Exception as e:
            self._log_error(f"get_latest_option_quotes_batch for {len(symbols)} symbols", e)
            return {}
    
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
            order_type = order_data.get("order_type")
            side = order_data["side"].split('_')[0] # "buy_to_open" -> "buy"
            request_params = None

            if order_type == "market":
                request_params = MarketOrderRequest(
                    symbol=order_data["symbol"],
                    qty=order_data["qty"],
                    side=side,
                    time_in_force=order_data["time_in_force"]
                )
            elif order_type == "limit":
                request_params = LimitOrderRequest(
                    symbol=order_data["symbol"],
                    qty=order_data["qty"],
                    side=side,
                    time_in_force=order_data["time_in_force"],
                    limit_price=order_data["limit_price"]
                )
            
            if request_params:
                order = self.trading_client.submit_order(order_data=request_params)
                return self._transform_order(order)
            else:
                raise ValueError(f"Unsupported or missing order type: {order_type}")

        except Exception as e:
            self._log_error("place_order", e)
            raise

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a multi-leg trading order."""
        try:
            order_legs = [
                OptionLegRequest(
                    symbol=leg["symbol"],
                    side=OrderSide(leg["side"].split('_')[0]),
                    ratio_qty=int(leg["qty"])
                )
                for leg in order_data["legs"]
            ]

            if order_data["order_type"] == "limit":
                request_params = LimitOrderRequest(
                    qty=order_data["qty"],
                    order_class=OrderClass.MLEG,
                    time_in_force=TimeInForce(order_data["time_in_force"]),
                    legs=order_legs,
                    limit_price=order_data["limit_price"]
                )
            else: # market
                 request_params = MarketOrderRequest(
                    qty=order_data["qty"],
                    order_class=OrderClass.MLEG,
                    time_in_force=TimeInForce(order_data["time_in_force"]),
                    legs=order_legs
                )

            order = self.trading_client.submit_order(order_data=request_params)
            return self._transform_order(order)
        except Exception as e:
            self._log_error("place_multi_leg_order", e)
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
            self.option_stream = OptionDataStream(self.api_key, self.api_secret)
            self.stock_stream = StockDataStream(self.api_key, self.api_secret)
            self.trading_stream = TradingStream(
                self.api_key,
                self.api_secret,
                paper=self.use_paper
            )
            
            # Start the streams in a separate thread
            import threading
            threading.Thread(target=self.option_stream.run, daemon=True).start()
            threading.Thread(target=self.stock_stream.run, daemon=True).start()
            
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
            stock_symbols = [s for s in symbols if not self._is_option_symbol(s)]
            if self.stock_stream and stock_symbols:
                self._log_info(f"Subscribing to {len(stock_symbols)} stock symbols: {stock_symbols}")
                self.stock_stream.subscribe_quotes(self._stock_quote_handler, *stock_symbols)

            # Subscribe to option quotes
            option_symbols = [s for s in symbols if self._is_option_symbol(s)]
            if self.option_stream and option_symbols:
                self._log_info(f"Subscribing to {len(option_symbols)} option symbols: {option_symbols}")
                self.option_stream.subscribe_quotes(self._option_quote_handler, *option_symbols)

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
    
    def _extract_expiry_from_symbol(self, symbol: str) -> Optional[str]:
        """Extract expiration date from option symbol (e.g., SPY250711C00582000 -> 2025-07-11)."""
        try:
            if len(symbol) >= 9:
                # Extract date part from symbol (positions 3-8: YYMMDD)
                date_part = symbol[3:9]
                
                if len(date_part) == 6 and date_part.isdigit():
                    year = 2000 + int(date_part[:2])
                    month = int(date_part[2:4])
                    day = int(date_part[4:6])
                    
                    # Validate date components
                    if 1 <= month <= 12 and 1 <= day <= 31:
                        return f"{year}-{month:02d}-{day:02d}"
        except Exception as e:
            self._log_error(f"_extract_expiry_from_symbol {symbol}", e)
        return None
    
    def _get_cached_expiration_dates(self, symbol: str) -> Optional[List[str]]:
        """Get cached expiration dates if available and not expired (daily TTL)."""
        try:
            if symbol in self._expiration_cache:
                cached_data = self._expiration_cache[symbol]
                cached_date = cached_data.get("cached_date", "")
                today = date.today().strftime("%Y-%m-%d")
                
                # Check if cache is still valid (same day)
                if cached_date == today:
                    return cached_data.get("expiration_dates", [])
                else:
                    # Cache expired (different day), remove it
                    del self._expiration_cache[symbol]
            
            return None
        except Exception as e:
            self._log_error(f"_get_cached_expiration_dates for {symbol}", e)
            return None
    
    def _cache_expiration_dates(self, symbol: str, expiration_dates: List[str]) -> None:
        """Cache expiration dates with daily TTL and save to disk."""
        try:
            today = date.today().strftime("%Y-%m-%d")
            
            self._expiration_cache[symbol] = {
                "expiration_dates": expiration_dates,
                "cached_date": today
            }
            
            # Save cache to disk
            self._save_cache_to_disk()
            
            self._log_info(f"Cached {len(expiration_dates)} expiration dates for {symbol} (valid until next day)")
        except Exception as e:
            self._log_error(f"_cache_expiration_dates for {symbol}", e)
    
    def _load_cache_from_disk(self) -> None:
        """Load expiration dates cache from disk on startup."""
        try:
            if os.path.exists(self._cache_file_path):
                with open(self._cache_file_path, 'r') as f:
                    cache_data = json.load(f)
                    
                # Validate and load cache data
                today = date.today().strftime("%Y-%m-%d")
                valid_cache = {}
                
                for symbol, data in cache_data.items():
                    cached_date = data.get("cached_date", "")
                    if cached_date == today:
                        # Cache is still valid for today
                        valid_cache[symbol] = data
                    # Expired cache entries are automatically excluded
                
                self._expiration_cache = valid_cache
                
                if valid_cache:
                    self._log_info(f"Loaded {len(valid_cache)} cached expiration date entries from disk")
                else:
                    self._log_info("No valid cached expiration dates found on disk")
            else:
                self._log_info("No cache file found - starting with empty cache")
                
        except Exception as e:
            self._log_error("_load_cache_from_disk", e)
            # Start with empty cache if loading fails
            self._expiration_cache = {}
    
    def _save_cache_to_disk(self) -> None:
        """Save expiration dates cache to disk."""
        try:
            # Create cache directory if it doesn't exist
            cache_dir = os.path.dirname(self._cache_file_path)
            if cache_dir and not os.path.exists(cache_dir):
                os.makedirs(cache_dir, exist_ok=True)
            
            # Save cache to file
            with open(self._cache_file_path, 'w') as f:
                json.dump(self._expiration_cache, f, indent=2)
                
            self._log_info(f"Saved {len(self._expiration_cache)} cache entries to disk")
            
        except Exception as e:
            self._log_error("_save_cache_to_disk", e)
    
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
            self._log_info(f"Received option quote for {data.symbol}: {data}")
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
