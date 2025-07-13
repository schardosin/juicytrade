import asyncio
import requests
import websockets
import json
import logging
import time
from typing import List, Dict, Optional, Any, Tuple
from datetime import datetime
from collections import OrderedDict

from .base_provider import BaseProvider
from ..models import StockQuote, OptionContract, Position, Order, MarketData, SymbolSearchResult

logger = logging.getLogger(__name__)

class SymbolLookupCache:
    """Smart caching for symbol lookup with prefix filtering."""
    
    def __init__(self, ttl_seconds: int = 21600, max_size: int = 500):
        self.ttl_seconds = ttl_seconds  # 6 hours (21600 seconds)
        self.max_size = max_size
        self.cache: OrderedDict[str, Tuple[List[SymbolSearchResult], float]] = OrderedDict()
    
    def _cleanup_expired(self):
        """Remove expired entries from cache."""
        current_time = time.time()
        expired_keys = [
            key for key, (_, timestamp) in self.cache.items()
            if current_time - timestamp > self.ttl_seconds
        ]
        for key in expired_keys:
            del self.cache[key]
    
    def _enforce_size_limit(self):
        """Enforce max cache size using LRU eviction."""
        while len(self.cache) > self.max_size:
            self.cache.popitem(last=False)  # Remove oldest (LRU)
    
    async def search(self, query: str, api_call_func) -> List[SymbolSearchResult]:
        """
        Smart search with caching and prefix filtering.
        
        Args:
            query: Search query
            api_call_func: Function to call API if cache miss
            
        Returns:
            List of SymbolSearchResult objects
        """
        self._cleanup_expired()
        normalized_query = query.lower().strip()
        
        # 1. Check for exact match in cache
        if normalized_query in self.cache:
            results, _ = self.cache[normalized_query]
            # Move to end (mark as recently used)
            self.cache.move_to_end(normalized_query)
            # Return top 50 for display
            return results[:50] if len(results) > 50 else results
        
        # 2. Try to filter from cached broader search
        filtered_results = self._try_filter_from_cache(normalized_query)
        if filtered_results is not None:
            # Cache the filtered results for future exact matches
            self.cache[normalized_query] = (filtered_results, time.time())
            self._enforce_size_limit()
            # Return top 50 for display
            return filtered_results[:50] if len(filtered_results) > 50 else filtered_results
        
        # 3. Make API call and cache results (only if successful)
        api_results = await api_call_func(query)
        
        # Only cache if we got actual results (don't cache empty results from API failures)
        if api_results:
            # Cache the full results (not just top 50) for better prefix filtering
            self.cache[normalized_query] = (api_results, time.time())
            self._enforce_size_limit()
        
        # Return top 50 for display
        return api_results[:50] if len(api_results) > 50 else api_results
    
    def _try_filter_from_cache(self, query: str) -> Optional[List[SymbolSearchResult]]:
        """
        Try to filter results from a cached broader search.
        
        Args:
            query: Current search query
            
        Returns:
            Filtered results if found, None otherwise
        """
        current_time = time.time()
        
        # Look for cached results from shorter queries that could contain our results
        for cached_query, (results, timestamp) in self.cache.items():
            # Skip expired entries
            if current_time - timestamp > self.ttl_seconds:
                continue
            
            # Check if this cached query is a prefix of our query
            if len(cached_query) < len(query) and query.startswith(cached_query):
                # Filter the cached results to match our query
                filtered = [
                    result for result in results
                    if (query in result.symbol.lower() or 
                        query in result.description.lower())
                ]
                
                # Sort the filtered results by relevance (same logic as API)
                def sort_key(result):
                    symbol = result.symbol.upper()
                    query_upper = query.upper()
                    
                    # Exact match gets highest priority
                    if symbol == query_upper:
                        return (0, symbol)
                    
                    # Starts with query gets second priority
                    if symbol.startswith(query_upper):
                        return (1, len(symbol), symbol)
                    
                    # Contains query gets third priority
                    if query_upper in symbol:
                        return (2, symbol.index(query_upper), len(symbol), symbol)
                    
                    # Description contains query gets fourth priority
                    description = result.description.upper()
                    if query_upper in description:
                        return (3, description.index(query_upper), len(description), symbol)
                    
                    # Everything else
                    return (4, symbol)
                
                filtered.sort(key=sort_key)
                return filtered
        
        return None

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
        self._symbol_cache = SymbolLookupCache()

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
        """Get latest stock quote for a symbol."""
        try:
            quotes = await self.get_stock_quotes([symbol])
            return quotes.get(symbol)
        except Exception as e:
            self._log_error(f"get_stock_quote for {symbol}", e)
            return None

    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        """Get stock quotes for multiple symbols."""
        try:
            url = f"{self.base_url}/v1/markets/quotes"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            params = {
                "symbols": ",".join(symbols)
            }
            
            resp = requests.post(url, headers=headers, data=params)
            resp.raise_for_status()
            data = resp.json()
            
            quotes = data.get("quotes", {}).get("quote", [])
            if isinstance(quotes, dict):
                quotes = [quotes]
            
            result = {}
            for quote in quotes:
                transformed_quote = self._transform_stock_quote(quote)
                if transformed_quote:
                    result[transformed_quote.symbol] = transformed_quote
            
            return result
        except Exception as e:
            self._log_error(f"get_stock_quotes for {symbols}", e)
            return {}

    def _transform_stock_quote(self, raw_quote: Dict[str, Any]) -> Optional[StockQuote]:
        """Transform Tradier stock quote to our standard model."""
        try:
            return StockQuote(
                symbol=raw_quote.get("symbol", ""),
                ask=float(raw_quote.get("ask", 0)) if raw_quote.get("ask") else None,
                bid=float(raw_quote.get("bid", 0)) if raw_quote.get("bid") else None,
                timestamp=datetime.fromtimestamp(raw_quote.get("trade_date") / 1000).isoformat() if raw_quote.get("trade_date") else datetime.now().isoformat()
            )
        except Exception as e:
            self._log_error("transform_stock_quote", e)
            return None
    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """Get available expiration dates for options on a symbol."""
        try:
            url = f"{self.base_url}/v1/markets/options/expirations"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            params = {
                "symbol": symbol,
                "includeAllRoots": "true"
            }
            
            resp = requests.get(url, headers=headers, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            expirations = data.get("expirations", {}).get("date", [])
            if isinstance(expirations, str):
                return [expirations]
            return expirations
        except Exception as e:
            self._log_error(f"get_expiration_dates for {symbol}", e)
            return []

    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        """Get options chain for a symbol and expiration."""
        try:
            url = f"{self.base_url}/v1/markets/options/chains"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            params = {
                "symbol": symbol,
                "expiration": expiry,
                "greeks": "true"
            }
            
            resp = requests.get(url, headers=headers, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            contracts = data.get("options", {}).get("option", [])
            
            result = []
            for contract in contracts:
                if option_type and contract.get("option_type") != option_type.lower():
                    continue
                
                transformed_contract = self._transform_option_contract(contract)
                if transformed_contract:
                    result.append(transformed_contract)
            
            return result
        except Exception as e:
            self._log_error(f"get_options_chain for {symbol} {expiry}", e)
            return []

    def _transform_option_contract(self, raw_contract: Dict[str, Any]) -> Optional[OptionContract]:
        """Transform Tradier option contract to our standard model."""
        try:
            greeks = raw_contract.get("greeks", {})
            return OptionContract(
                symbol=raw_contract.get("symbol", ""),
                underlying_symbol=raw_contract.get("underlying", ""),
                expiration_date=raw_contract.get("expiration_date", ""),
                strike_price=float(raw_contract.get("strike", 0)),
                type=raw_contract.get("option_type", "").lower(),
                bid=float(raw_contract.get("bid", 0)) if raw_contract.get("bid") else None,
                ask=float(raw_contract.get("ask", 0)) if raw_contract.get("ask") else None,
                close_price=float(raw_contract.get("close", 0)) if raw_contract.get("close") else None,
                volume=int(raw_contract.get("volume", 0)) if raw_contract.get("volume") else None,
                open_interest=int(raw_contract.get("open_interest", 0)) if raw_contract.get("open_interest") else None,
                implied_volatility=float(greeks.get("mid_iv", 0)) if greeks.get("mid_iv") else None,
                delta=float(greeks.get("delta", 0)) if greeks.get("delta") else None,
                gamma=float(greeks.get("gamma", 0)) if greeks.get("gamma") else None,
                theta=float(greeks.get("theta", 0)) if greeks.get("theta") else None,
                vega=float(greeks.get("vega", 0)) if greeks.get("vega") else None,
            )
        except Exception as e:
            self._log_error("transform_option_contract", e)
            return None

    async def get_next_market_date(self) -> str:
        """Get the next trading date."""
        try:
            url = f"{self.base_url}/v1/markets/calendar"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            resp = requests.get(url, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            
            days = data.get("calendar", {}).get("days", {}).get("day", [])
            if isinstance(days, dict):
                days = [days]

            for day in days:
                if day.get("status") == "open":
                    return day.get("date")
            
            # Fallback to today if no open market date is found
            return datetime.now().strftime("%Y-%m-%d")
        except Exception as e:
            self._log_error("get_next_market_date", e)
            return datetime.now().strftime("%Y-%m-%d")

    async def get_positions(self) -> List[Position]:
        """Get all current positions."""
        try:
            url = f"{self.base_url}/v1/accounts/{self.account_id}/positions"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            resp = requests.get(url, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            
            positions = data.get("positions", {}).get("position", [])
            if isinstance(positions, dict):
                positions = [positions]
            
            result = []
            for position in positions:
                transformed_position = self._transform_position(position)
                if transformed_position:
                    result.append(transformed_position)
            
            return result
        except Exception as e:
            self._log_error("get_positions", e)
            return []

    def _transform_position(self, raw_position: Dict[str, Any]) -> Optional[Position]:
        """Transform Tradier position to our standard model."""
        try:
            return Position(
                symbol=raw_position.get("symbol"),
                qty=float(raw_position.get("quantity", 0)),
                side="long" if float(raw_position.get("quantity", 0)) > 0 else "short",
                cost_basis=float(raw_position.get("cost_basis", 0)),
                # Tradier does not provide these fields directly in the positions endpoint
                market_value=0,
                unrealized_pl=0,
                unrealized_plpc=0,
                current_price=0,
                avg_entry_price=float(raw_position.get("cost_basis", 0)) / float(raw_position.get("quantity", 1)),
                asset_class="us_equity" if not self._is_option_symbol(raw_position.get("symbol")) else "us_option"
            )
        except Exception as e:
            self._log_error("transform_position", e)
            return None

    async def get_orders(self, status: str = "open") -> List[Order]:
        """Get orders with optional status filter."""
        try:
            url = f"{self.base_url}/v1/accounts/{self.account_id}/orders"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            resp = requests.get(url, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            
            orders = data.get("orders", {}).get("order", [])
            if isinstance(orders, dict):
                orders = [orders]
            
            result = []
            for order in orders:
                if status == "all" or order.get("status") == status:
                    transformed_order = self._transform_order(order)
                    if transformed_order:
                        result.append(transformed_order)
            
            return result
        except Exception as e:
            self._log_error(f"get_orders with status {status}", e)
            return []

    def _transform_order(self, raw_order: Dict[str, Any]) -> Optional[Order]:
        """Transform Tradier order to our standard model."""
        try:
            return Order(
                id=str(raw_order.get("id")),
                symbol=raw_order.get("symbol"),
                asset_class=raw_order.get("class"),
                side=raw_order.get("side"),
                order_type=raw_order.get("type"),
                qty=float(raw_order.get("quantity", 0)),
                filled_qty=float(raw_order.get("exec_quantity", 0)),
                limit_price=float(raw_order.get("price", 0)) if raw_order.get("price") else None,
                stop_price=None,
                avg_fill_price=float(raw_order.get("avg_fill_price", 0)) if raw_order.get("avg_fill_price") else None,
                status=raw_order.get("status"),
                time_in_force=raw_order.get("duration"),
                submitted_at=raw_order.get("create_date"),
                filled_at=raw_order.get("transaction_date"),
                legs=[self._transform_order_leg(leg) for leg in raw_order.get("leg", [])] if raw_order.get("leg") else None
            )
        except Exception as e:
            self._log_error("transform_order", e)
            return None

    def _transform_order_leg(self, raw_leg: Dict[str, Any]) -> Dict[str, Any]:
        """Transform Tradier order leg to our standard format."""
        return {
            "symbol": raw_leg.get("option_symbol"),
            "side": raw_leg.get("side"),
            "qty": float(raw_leg.get("quantity", 0))
        }
    
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
        """Cancel an existing order."""
        try:
            url = f"{self.base_url}/v1/accounts/{self.account_id}/orders/{order_id}"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            resp = requests.delete(url, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            
            return data.get("order", {}).get("status") == "ok"
        except Exception as e:
            self._log_error(f"cancel_order {order_id}", e)
            return False
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

    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query using Tradier API."""
        return await self._symbol_cache.search(query, self._api_lookup_symbols)
    
    async def _api_lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Make actual API call to Tradier for symbol lookup."""
        try:
            url = f"{self.base_url}/v1/markets/lookup"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            params = {
                "q": query
            }
            
            resp = requests.get(url, headers=headers, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            # Handle the response structure
            securities = data.get("securities", {})
            if isinstance(securities, dict):
                security_list = securities.get("security", [])
            else:
                security_list = securities
            
            # Ensure it's a list
            if isinstance(security_list, dict):
                security_list = [security_list]
            
            # Sort results by relevance before limiting
            def sort_key(security):
                symbol = security.get("symbol", "").upper()
                query_upper = query.upper()
                
                # Exact match gets highest priority
                if symbol == query_upper:
                    return (0, symbol)
                
                # Starts with query gets second priority
                if symbol.startswith(query_upper):
                    return (1, len(symbol), symbol)
                
                # Contains query gets third priority
                if query_upper in symbol:
                    return (2, symbol.index(query_upper), len(symbol), symbol)
                
                # Description contains query gets fourth priority
                description = security.get("description", "").upper()
                if query_upper in description:
                    return (3, description.index(query_upper), len(description), symbol)
                
                # Everything else
                return (4, symbol)
            
            # Sort the security list
            security_list.sort(key=sort_key)
            
            # Transform ALL results first (don't limit yet)
            all_results = []
            for security in security_list:  # Transform ALL results
                transformed = self._transform_symbol_search_result(security)
                if transformed:
                    all_results.append(transformed)
            
            logger.info(f"Found {len(all_results)} total symbols for query '{query}', caching all results")
            
            # Return ALL results for caching, limiting will be done in the cache search method
            return all_results
            
        except Exception as e:
            self._log_error(f"lookup_symbols for query '{query}'", e)
            return []
    
    def _transform_symbol_search_result(self, raw_security: Dict[str, Any]) -> Optional[SymbolSearchResult]:
        """Transform Tradier security search result to our standard model."""
        try:
            return SymbolSearchResult(
                symbol=raw_security.get("symbol", ""),
                description=raw_security.get("description", ""),
                exchange=raw_security.get("exchange", ""),
                type=raw_security.get("type", "")
            )
        except Exception as e:
            self._log_error("transform_symbol_search_result", e)
            return None

    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """
        Get historical OHLCV bars for charting using appropriate Tradier endpoint.
        
        Uses /v1/markets/timesales for intraday (1m, 5m, 15m)
        Uses /v1/markets/history for daily/weekly/monthly (D, W, M)
        """
        try:
            # Determine which endpoint to use based on timeframe
            intraday_timeframes = ['1m', '5m', '15m', '30m', '1h', '4h']
            daily_timeframes = ['D', 'W', 'M']
            
            if timeframe in intraday_timeframes:
                return await self._get_intraday_bars(symbol, timeframe, start_date, end_date, limit)
            elif timeframe in daily_timeframes:
                return await self._get_daily_bars(symbol, timeframe, start_date, end_date, limit)
            else:
                self._log_error(f"get_historical_bars for {symbol}", 
                              Exception(f"Unsupported timeframe: {timeframe}"))
                return []
                
        except Exception as e:
            self._log_error(f"get_historical_bars for {symbol} {timeframe}", e)
            return []
    
    async def _get_intraday_bars(self, symbol: str, timeframe: str, 
                               start_date: str = None, end_date: str = None, 
                               limit: int = 500) -> List[Dict[str, Any]]:
        """Get intraday bars using /v1/markets/timesales endpoint."""
        try:
            url = f"{self.base_url}/v1/markets/timesales"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            # Map our timeframe to Tradier interval
            interval_map = {
                '1m': '1min',
                '5m': '5min', 
                '15m': '15min',
                '30m': '15min',  # Tradier doesn't have 30min, use 15min
                '1h': '15min',   # Tradier doesn't have 1h, use 15min
                '4h': '15min'    # Tradier doesn't have 4h, use 15min
            }
            
            params = {
                "symbol": symbol,
                "interval": interval_map.get(timeframe, '1min'),
                "session_filter": "all"
            }
            
            # Add date range if provided
            if start_date:
                params["start"] = f"{start_date} 09:30"
            if end_date:
                params["end"] = f"{end_date} 16:00"
            
            response = requests.get(url, headers=headers, params=params)
            response.raise_for_status()
            data = response.json()
            
            # Extract timesales data
            series_data = data.get("series", {}).get("data", [])
            
            # Transform to Lightweight Charts format
            result = []
            for bar in series_data:
                # Convert timestamp to date string for Lightweight Charts
                timestamp = bar.get("timestamp")
                if timestamp:
                    time_str = datetime.fromtimestamp(timestamp).strftime('%Y-%m-%d %H:%M')
                else:
                    time_str = bar.get("time", "")
                
                result.append({
                    "time": time_str,
                    "open": float(bar.get("open", 0)),
                    "high": float(bar.get("high", 0)),
                    "low": float(bar.get("low", 0)),
                    "close": float(bar.get("close", 0)),
                    "volume": int(bar.get("volume", 0))
                })
            
            # Apply limit if specified
            if limit and len(result) > limit:
                result = result[-limit:]  # Get most recent bars
            
            self._log_info(f"Retrieved {len(result)} intraday bars for {symbol} ({timeframe})")
            return result
            
        except Exception as e:
            self._log_error(f"_get_intraday_bars for {symbol} {timeframe}", e)
            return []
    
    async def _get_daily_bars(self, symbol: str, timeframe: str, 
                            start_date: str = None, end_date: str = None, 
                            limit: int = 500) -> List[Dict[str, Any]]:
        """Get daily/weekly/monthly bars using /v1/markets/history endpoint."""
        try:
            url = f"{self.base_url}/v1/markets/history"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            # Map our timeframe to Tradier interval
            interval_map = {
                'D': 'daily',
                'W': 'weekly', 
                'M': 'monthly'
            }
            
            params = {
                "symbol": symbol,
                "interval": interval_map.get(timeframe, 'daily'),
                "session_filter": "all"
            }
            
            # Add date range if provided
            if start_date:
                params["start"] = start_date
            if end_date:
                params["end"] = end_date
            
            response = requests.get(url, headers=headers, params=params)
            response.raise_for_status()
            data = response.json()
            
            # Extract history data
            history_data = data.get("history", {})
            if isinstance(history_data, dict):
                bars = history_data.get("day", [])
            else:
                bars = []
            
            # Transform to Lightweight Charts format
            result = []
            for bar in bars:
                result.append({
                    "time": bar.get("date", ""),
                    "open": float(bar.get("open", 0)),
                    "high": float(bar.get("high", 0)),
                    "low": float(bar.get("low", 0)),
                    "close": float(bar.get("close", 0)),
                    "volume": int(bar.get("volume", 0))
                })
            
            # Apply limit if specified
            if limit and len(result) > limit:
                result = result[-limit:]  # Get most recent bars
            
            self._log_info(f"Retrieved {len(result)} daily bars for {symbol} ({timeframe})")
            return result
            
        except Exception as e:
            self._log_error(f"_get_daily_bars for {symbol} {timeframe}", e)
            return []

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])
