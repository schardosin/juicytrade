import asyncio
import requests
import websockets
import json
import logging
import time
from typing import List, Dict, Optional, Any, Tuple
from datetime import datetime, timedelta
from collections import OrderedDict

from .base_provider import BaseProvider
from ..models import StockQuote, OptionContract, Position, Order, MarketData, SymbolSearchResult, Account, PositionGroup, HistoricalTrade
from ..config import settings

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
        self._connection_ready = asyncio.Event()
        self._symbol_cache = SymbolLookupCache()
        self._lastday_price_cache = {}
        self._lastday_price_cache_date = None

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
        """Enhanced connection with better error handling and recovery"""
        try:
            # Clear previous connection state
            self.is_connected = False
            self._connection_ready.clear()
            
            # Create new session
            if not await self._create_session():
                logger.error("Failed to create Tradier session")
                return False

            # Close existing connection if any
            if self._stream_connection:
                try:
                    await self._stream_connection.close()
                except:
                    pass
                self._stream_connection = None

            # Establish new WebSocket connection
            logger.info(f"Connecting to Tradier WebSocket: {self.stream_url}")
            self._stream_connection = await websockets.connect(
                self.stream_url,
                ping_interval=30,  # Send ping every 30 seconds
                ping_timeout=10,   # Wait 10 seconds for pong
                close_timeout=10   # Wait 10 seconds for close
            )
            
            logger.info("✅ Connected to Tradier WebSocket successfully")
            self._connection_ready.set()
            
            # Start stream handler
            asyncio.create_task(self._stream_handler())
            
            self.is_connected = True
            return True
            
        except Exception as e:
            logger.error(f"❌ Failed to connect to Tradier WebSocket: {e}")
            self.is_connected = False
            return False

    async def _stream_handler(self):
        """Enhanced stream handler with better error handling and debugging"""
        logger.info("🎯 Tradier stream handler started")
        message_count = 0
        quote_count = 0
        
        try:
            async for message in self._stream_connection:
                message_count += 1
                try:
                    data = json.loads(message)
                    message_type = data.get('type', 'unknown')
                    
                    # Log every 100 messages to show activity
                    if message_count % 100 == 0:
                        logger.debug(f"📊 Tradier: Processed {message_count} messages, {quote_count} quotes")
                    
                    if message_type == 'quote':
                        quote_count += 1
                        symbol = data.get('symbol')
                        
                        # Log first few quotes for debugging
                        if quote_count <= 5:
                            logger.debug(f"📈 Tradier quote #{quote_count}: {symbol} - bid: {data.get('bid')}, ask: {data.get('ask')}, last: {data.get('last')}")
                        
                        # Validate that we're subscribed to this symbol
                        if symbol not in self._subscribed_symbols:
                            # Only warn for stock symbols, options are expected to have many symbols
                            if not self._is_option_symbol(symbol):
                                logger.debug(f"Received data for unsubscribed symbol: {symbol}")
                            continue
                        
                        # Create market data with proper price handling
                        price_data = {
                            'bid': data.get('bid'),
                            'ask': data.get('ask'),
                            'last': data.get('last'),
                            'volume': data.get('volume'),
                            'timestamp': data.get('biddate') or data.get('timestamp')
                        }
                        
                        market_data = MarketData(
                            symbol=symbol,
                            data_type='quote',
                            timestamp=str(data.get('biddate') or data.get('timestamp', '')),
                            data=price_data
                        )
                        await self._streaming_queue.put(market_data)
                        
                        # Log every 50th quote to show data flow
                        if quote_count % 50 == 0:
                            logger.debug(f"📡 Tradier: Queued quote #{quote_count} for {symbol}")
                        
                    elif message_type == 'heartbeat':
                        # Handle heartbeat messages
                        logger.debug("💓 Received heartbeat from Tradier")
                    else:
                        # Log unknown message types for first few
                        if message_count <= 10:
                            logger.debug(f"📨 Tradier message #{message_count}: type={message_type}")
                        
                except json.JSONDecodeError as e:
                    logger.warning(f"Failed to decode JSON message: {e}")
                    continue
                except Exception as e:
                    logger.error(f"Error processing message #{message_count}: {e}")
                    continue
                    
        except websockets.exceptions.ConnectionClosed as e:
            logger.warning(f"🔌 Tradier WebSocket connection closed: {e}")
            self.is_connected = False
        except Exception as e:
            logger.error(f"❌ Error in Tradier stream handler: {e}")
            self.is_connected = False
        finally:
            logger.info(f"� Tradier stream handler stopped - processed {message_count} messages, {quote_count} quotes")

    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        logger.info(f"🔔 Tradier: subscribe_to_symbols called with {len(symbols)} symbols")
        
        # Check if we need to reconnect
        if not self.is_connected or not self._session_id or not self._stream_connection:
            logger.info("Tradier stream not connected, attempting to reconnect...")
            if not await self.connect_streaming():
                logger.error("Failed to reconnect to Tradier stream. Cannot subscribe.")
                return False
        
        await self._connection_ready.wait() # Wait until connection is ready
        
        payload = {
            "symbols": symbols,
            "sessionid": self._session_id,
            "filter": ["quote"] # For now, only quotes
        }
        try:
            logger.info(f"🔔 Tradier: Sending subscription payload with {len(symbols)} symbols")
            await self._stream_connection.send(json.dumps(payload))
            # IMPORTANT: Replace the entire subscription set, don't add to it
            self._subscribed_symbols = set(symbols)
            return True
        except Exception as e:
            logger.error(f"Failed to subscribe to symbols on Tradier: {e}")
            # Try to reconnect on failure
            self.is_connected = False
            return False

    def is_streaming_connected(self) -> bool:
        """Check if streaming connection is active and healthy"""
        try:
            # Basic checks first
            if not self.is_connected or self._session_id is None:
                return False
            
            # Check WebSocket connection state
            if self._stream_connection is None:
                return False
            
            # Check if WebSocket is in OPEN state (1)
            # WebSocket states: CONNECTING=0, OPEN=1, CLOSING=2, CLOSED=3
            if hasattr(self._stream_connection, 'state'):
                return self._stream_connection.state == 1  # OPEN state
            else:
                # Fallback to closed property check
                return not self._stream_connection.closed
                
        except Exception as e:
            logger.debug(f"Exception in is_streaming_connected: {e}")
            return False

    async def get_streaming_data(self) -> Optional[MarketData]:
        """Not used in this new architecture - data is put directly into manager's queue"""
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

    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20) -> List[OptionContract]:
        """Fast loading - basic options data without Greeks, ATM-focused by strike count."""
        try:
            url = f"{self.base_url}/v1/markets/options/chains"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            params = {
                "symbol": symbol,
                "expiration": expiry,
                "greeks": "false"  # Skip expensive Greeks calculation
            }
            
            resp = requests.get(url, headers=headers, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            contracts = data.get("options", {}).get("option", [])
            
            # Get underlying price if not provided
            if underlying_price is None:
                try:
                    quote = await self.get_stock_quote(symbol)
                    underlying_price = (quote.bid + quote.ask) / 2 if quote and quote.bid and quote.ask else None
                except:
                    underlying_price = None
            
            # Smart filtering - focus on ATM strikes by count
            filtered_contracts = []
            if underlying_price and contracts:
                # Get all unique strikes and sort them
                strikes = sorted(list(set(float(contract.get("strike", 0)) for contract in contracts)))
                
                # Find the ATM strike (closest to underlying price)
                atm_strike = min(strikes, key=lambda x: abs(x - underlying_price))
                atm_index = strikes.index(atm_strike)
                
                # Calculate how many strikes to take on each side
                strikes_per_side = strike_count // 2
                
                # Get the range of strikes around ATM
                start_index = max(0, atm_index - strikes_per_side)
                end_index = min(len(strikes), atm_index + strikes_per_side + 1)
                
                # Select strikes in the range
                selected_strikes = set(strikes[start_index:end_index])
                
                # Filter contracts to only include selected strikes
                for contract in contracts:
                    strike = float(contract.get("strike", 0))
                    if strike in selected_strikes:
                        filtered_contracts.append(contract)
                
                contracts = filtered_contracts
            
            # Transform without Greeks (faster)
            result = []
            for contract in contracts:
                transformed_contract = self._transform_option_contract_basic(contract)
                if transformed_contract:
                    result.append(transformed_contract)
            
            return result
        except Exception as e:
            self._log_error(f"get_options_chain_basic for {symbol} {expiry}", e)
            return []

    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """Get Greeks for multiple option symbols in batch."""
        try:
            # Tradier doesn't have a dedicated Greeks endpoint, so we need to get full option data
            # Group symbols by underlying and expiration for efficient API calls
            symbol_groups = {}
            
            for option_symbol in option_symbols:
                parsed = self._parse_option_symbol(option_symbol)
                if parsed:
                    key = f"{parsed['underlying']}_{parsed['expiry']}"
                    if key not in symbol_groups:
                        symbol_groups[key] = {
                            'underlying': parsed['underlying'],
                            'expiry': parsed['expiry'],
                            'symbols': []
                        }
                    symbol_groups[key]['symbols'].append(option_symbol)
            
            # Fetch Greeks for each group
            greeks_data = {}
            for group in symbol_groups.values():
                try:
                    contracts = await self.get_options_chain(
                        group['underlying'], 
                        group['expiry']
                    )
                    
                    # Extract Greeks for requested symbols
                    for contract in contracts:
                        if contract.symbol in group['symbols']:
                            greeks_data[contract.symbol] = {
                                'delta': contract.delta,
                                'theta': contract.theta,
                                'gamma': contract.gamma,
                                'vega': contract.vega,
                                'implied_volatility': contract.implied_volatility
                            }
                except Exception as e:
                    self._log_error(f"get_options_greeks_batch for group {group['underlying']}", e)
                    continue
            
            return greeks_data
        except Exception as e:
            self._log_error(f"get_options_greeks_batch", e)
            return {}

    async def get_options_chain_smart(self, symbol: str, expiry: str, underlying_price: float = None, 
                                   atm_range: int = 20, include_greeks: bool = False, 
                                   strikes_only: bool = False) -> List[OptionContract]:
        """Smart options chain with configurable loading."""
        try:
            if include_greeks:
                # Use full options chain with Greeks
                return await self.get_options_chain(symbol, expiry)
            else:
                # Use basic fast loading
                return await self.get_options_chain_basic(symbol, expiry, underlying_price, atm_range)
        except Exception as e:
            self._log_error(f"get_options_chain_smart for {symbol} {expiry}", e)
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

    def _transform_option_contract_basic(self, raw_contract: Dict[str, Any]) -> Optional[OptionContract]:
        """Transform Tradier option contract to our standard model without Greeks (faster)."""
        try:
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
                # Greeks are None for basic loading (will be loaded separately)
                implied_volatility=None,
                delta=None,
                gamma=None,
                theta=None,
                vega=None,
            )
        except Exception as e:
            self._log_error("transform_option_contract_basic", e)
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
            symbol = raw_position.get("symbol")
            cost_basis = float(raw_position.get("cost_basis", 0))
            quantity = float(raw_position.get("quantity", 1))
            
            # Calculate average entry price
            if self._is_option_symbol(symbol):
                # For options: cost_basis is total cost, divide by quantity and then by 100 to get per-contract price
                avg_entry_price = (cost_basis / quantity) / 100 if quantity != 0 else 0
            else:
                # For stocks: cost_basis is total cost, divide by quantity to get per-share price
                avg_entry_price = cost_basis / quantity if quantity != 0 else 0
            
            return Position(
                symbol=symbol,
                qty=quantity,
                side="long" if quantity > 0 else "short",
                cost_basis=cost_basis,
                # Tradier does not provide these fields directly in the positions endpoint
                market_value=0,
                unrealized_pl=0,
                unrealized_plpc=0,
                current_price=0,
                avg_entry_price=avg_entry_price,
                asset_class="us_equity" if not self._is_option_symbol(symbol) else "us_option"
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
                order_status = order.get("status")
                
                # Filter orders based on status
                if status == "all":
                    # Include all orders
                    include_order = True
                elif status == "canceled":
                    # Include both canceled and rejected orders
                    include_order = order_status in ["canceled", "cancelled", "rejected"]
                else:
                    # Exact status match for other statuses
                    include_order = order_status == status
                
                if include_order:
                    transformed_order = self._transform_order(order)
                    if transformed_order:
                        result.append(transformed_order)
            
            return result
        except Exception as e:
            self._log_error(f"get_orders with status {status}", e)
            return []

    async def get_account(self) -> Optional[Account]:
        """Get account information including balance and buying power."""
        try:
            url = f"{self.base_url}/v1/accounts/{self.account_id}/balances"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            resp = requests.get(url, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            
            balances = data.get("balances", {})
            if balances:
                return self._transform_account(balances)
            return None
        except Exception as e:
            self._log_error("get_account", e)
            return None

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
    
    def _transform_account(self, raw_balances: Dict[str, Any]) -> Optional[Account]:
        """Transform Tradier account balances to our standard model."""
        try:
            # Extract account type to determine which sub-object to use for buying power
            account_type = raw_balances.get("account_type", "").lower()
            
            # Get buying power based on account type
            stock_buying_power = None
            options_buying_power = None
            cash_available = None
            
            if account_type == "margin" and "margin" in raw_balances:
                margin_data = raw_balances["margin"]
                stock_buying_power = float(margin_data.get("stock_buying_power", 0)) if margin_data.get("stock_buying_power") else None
                options_buying_power = float(margin_data.get("option_buying_power", 0)) if margin_data.get("option_buying_power") else None
            elif account_type == "cash" and "cash" in raw_balances:
                cash_data = raw_balances["cash"]
                cash_available = float(cash_data.get("cash_available", 0)) if cash_data.get("cash_available") else None
                # For cash accounts, buying power is typically the same as available cash
                stock_buying_power = cash_available
                options_buying_power = cash_available
            elif account_type == "pdt" and "pdt" in raw_balances:
                pdt_data = raw_balances["pdt"]
                stock_buying_power = float(pdt_data.get("stock_buying_power", 0)) if pdt_data.get("stock_buying_power") else None
                options_buying_power = float(pdt_data.get("option_buying_power", 0)) if pdt_data.get("option_buying_power") else None
            
            # If cash_available wasn't set above, use total_cash as fallback
            if cash_available is None:
                cash_available = float(raw_balances.get("total_cash", 0)) if raw_balances.get("total_cash") else None
            
            # Determine overall buying power (prefer options buying power, then stock buying power)
            buying_power = options_buying_power or stock_buying_power
            
            return Account(
                account_id=self.account_id,
                account_number=raw_balances.get("account_number", self.account_id),
                status="active",  # Tradier doesn't provide account status in balances
                currency="USD",
                buying_power=buying_power,
                cash=cash_available,
                portfolio_value=float(raw_balances.get("total_equity", 0)) if raw_balances.get("total_equity") else None,
                equity=float(raw_balances.get("total_equity", 0)) if raw_balances.get("total_equity") else None,
                day_trading_buying_power=buying_power,  # Use same as buying_power for day trading
                regt_buying_power=stock_buying_power,
                options_buying_power=options_buying_power,
                pattern_day_trader=account_type == "pdt",  # True if account type is PDT
                trading_blocked=None,     # Tradier doesn't provide this in balances
                transfers_blocked=None,   # Tradier doesn't provide this in balances
                account_blocked=None,     # Tradier doesn't provide this in balances
                created_at=None,          # Tradier doesn't provide this in balances
                multiplier=None,          # Tradier doesn't provide this in balances
                long_market_value=float(raw_balances.get("long_market_value", 0)) if raw_balances.get("long_market_value") else None,
                short_market_value=float(raw_balances.get("short_market_value", 0)) if raw_balances.get("short_market_value") else None,
                initial_margin=float(raw_balances.get("current_requirement", 0)) if raw_balances.get("current_requirement") else None,
                maintenance_margin=None,  # Tradier doesn't provide this separately
                daytrade_count=None,      # Tradier doesn't provide this in balances
                options_approved_level=None,  # Tradier doesn't provide this in balances
                options_trading_level=None     # Tradier doesn't provide this in balances
            )
        except Exception as e:
            self._log_error("transform_account", e)
            return None
    
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
    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """
        Tradier doesn't support true unsubscribing, so we need to track what should be unsubscribed
        and only subscribe to the remaining symbols when a new subscription is made.
        """
        logger.info(f"🗑️ Tradier: Marking {len(symbols)} symbols for unsubscription")
        
        # Remove from our tracked subscribed symbols
        for symbol in symbols:
            self._subscribed_symbols.discard(symbol)
        
        # Since Tradier doesn't support true unsubscribing, we'll re-subscribe to only the remaining symbols
        if self._subscribed_symbols:
            logger.info(f"🔄 Tradier: Re-subscribing to remaining {len(self._subscribed_symbols)} symbols")
            return await self.subscribe_to_symbols(list(self._subscribed_symbols))
        else:
            logger.info("🔄 Tradier: No symbols remaining, clearing all subscriptions")
            # We can't truly clear all subscriptions in Tradier, but we've cleared our tracking
            return True

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
                description=raw_security.get("description") or "",  # Handle None values
                exchange=raw_security.get("exchange") or "",        # Handle None values
                type=raw_security.get("type") or ""                 # Handle None values
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

    # === Enhanced Positions with History Integration ===
    
    async def get_positions_enhanced(self) -> Dict[str, Any]:
        """Get enhanced positions with simplified static data structure."""
        try:
            logger.info("🔍 Getting enhanced positions with static broker data...")
            
            # 1. Get current positions
            current_positions = await self.get_positions()
            if not current_positions:
                logger.info("No current positions found")
                return {"enhanced": True, "symbol_groups": []}
            
            # 2. Get historical trades and current orders for grouping
            historical_trades = await self._get_order_history(days_back=30)
            current_orders = await self.get_orders(status="all")
            
            logger.info(f"📊 Retrieved {len(historical_trades)} historical trades and {len(current_orders)} current orders")
            
            # 3. Create simplified hierarchical grouping (Symbol -> Strategies -> Legs)
            symbol_groups = await self._create_simplified_hierarchical_groups(
                current_positions, historical_trades, current_orders
            )
            
            logger.info(f"✅ Created {len(symbol_groups)} symbol groups with static data")
            return {"enhanced": True, "symbol_groups": symbol_groups}
            
        except Exception as e:
            self._log_error("get_positions_enhanced", e)
            return {"enhanced": True, "symbol_groups": []}
    
    async def _get_order_history(self, days_back: int = 30) -> List[HistoricalTrade]:
        """Get order history from Tradier history API."""
        try:
            from datetime import datetime, timedelta
            
            url = f"{self.base_url}/v1/accounts/{self.account_id}/history"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json"
            }
            
            # Calculate date range
            end_date = datetime.now()
            start_date = end_date - timedelta(days=days_back)
            
            params = {
                "type": "trade",  # Only get trade transactions
                "start": start_date.strftime('%Y-%m-%d'),
                "end": end_date.strftime('%Y-%m-%d'),
                "limit": 1000
            }
            
            logger.info(f"🔍 Requesting history from {start_date.strftime('%Y-%m-%d')} to {end_date.strftime('%Y-%m-%d')}")
            
            resp = requests.get(url, headers=headers, params=params)
            resp.raise_for_status()
            data = resp.json()
            
            logger.info(f"📊 Raw history response keys: {list(data.keys())}")
            
            # Log the actual history structure for debugging
            if "history" in data:
                history_data = data["history"]
                logger.info(f"📊 History data type: {type(history_data)}")
                if isinstance(history_data, dict):
                    logger.info(f"📊 History dict keys: {list(history_data.keys())}")
                    if not history_data:
                        logger.info("📊 History dict is empty - no trade data available")
                elif isinstance(history_data, list):
                    logger.info(f"📊 History list length: {len(history_data)}")
                else:
                    logger.info(f"📊 History data: {history_data}")
            
            # Extract history data - try different response structures
            events = []
            if "history" in data:
                history = data["history"]
                if isinstance(history, dict):
                    events = history.get("event", [])
                elif isinstance(history, list):
                    events = history
            elif "events" in data:
                events = data["events"]
            elif "event" in data:
                events = data["event"]
            
            if isinstance(events, dict):
                events = [events]
            
            logger.info(f"📊 Found {len(events)} raw events")
            
            # Transform to HistoricalTrade objects
            trades = []
            for event in events:
                logger.debug(f"Processing event: {event.get('type', 'unknown')} - {event}")
                trade = self._transform_historical_trade(event)
                if trade:
                    trades.append(trade)
            
            logger.info(f"📊 Retrieved {len(trades)} historical trades")
            return trades
            
        except Exception as e:
            self._log_error("_get_order_history", e)
            return []
    
    def _transform_historical_trade(self, raw_event: Dict[str, Any]) -> Optional[HistoricalTrade]:
        """Transform Tradier history event to HistoricalTrade."""
        try:
            # Only process trade events
            if raw_event.get("type") != "trade":
                return None
            
            trade = raw_event.get("trade", {})
            if not trade:
                return None
            
            return HistoricalTrade(
                id=str(trade.get("id", "")),
                symbol=trade.get("symbol", ""),
                side="buy" if float(trade.get("quantity", 0)) > 0 else "sell",
                qty=abs(float(trade.get("quantity", 0))),
                price=float(trade.get("price", 0)),
                date=trade.get("date", ""),
                commission=float(trade.get("commission", 0)) if trade.get("commission") else None,
                asset_class="us_option" if self._is_option_symbol(trade.get("symbol", "")) else "us_equity"
            )
        except Exception as e:
            self._log_error("_transform_historical_trade", e)
            return None
    
    async def _get_current_prices(self, symbols: List[str]) -> Dict[str, float]:
        """Get current market prices for symbols."""
        try:
            if not symbols:
                return {}
            
            # Separate stocks and options
            stock_symbols = [s for s in symbols if not self._is_option_symbol(s)]
            option_symbols = [s for s in symbols if self._is_option_symbol(s)]
            
            prices = {}
            
            # Get stock prices
            if stock_symbols:
                stock_quotes = await self.get_stock_quotes(stock_symbols)
                for symbol, quote in stock_quotes.items():
                    if quote and quote.bid and quote.ask:
                        prices[symbol] = (quote.bid + quote.ask) / 2
                    elif quote and quote.bid:
                        prices[symbol] = quote.bid
                    elif quote and quote.ask:
                        prices[symbol] = quote.ask
            
            # Get option prices (simplified - would need options chain calls for full implementation)
            # For now, we'll use the close price from positions or set to 0
            for symbol in option_symbols:
                prices[symbol] = 0  # Will be updated with actual option prices if available
            
            return prices
            
        except Exception as e:
            self._log_error("_get_current_prices", e)
            return {}
    
    def _group_positions_by_order_chains(self, positions: List[Position], 
                                       history: List[HistoricalTrade], 
                                       current_orders: List[Order]) -> List[PositionGroup]:
        """Group positions by their originating orders using current orders and historical data."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime, timedelta
            
            logger.info("📊 Grouping positions by actual order chains")
            logger.info(f"📊 Using {len(current_orders)} current orders and {len(history)} historical trades")
            
            # Create a mapping of symbols to orders for faster lookup
            # For options orders, we need to map by the actual option symbols from legs
            symbol_to_orders = {}
            for order in current_orders:
                # For multi-leg orders, map each leg's option symbol to the order
                if order.legs and len(order.legs) > 0:
                    for leg in order.legs:
                        if leg.get("symbol"):  # This should be the option symbol
                            option_symbol = leg["symbol"]
                            if option_symbol not in symbol_to_orders:
                                symbol_to_orders[option_symbol] = []
                            symbol_to_orders[option_symbol].append(order)
                else:
                    # For single-leg orders, use the order symbol directly
                    # This could be either underlying (for stocks) or option symbol
                    if order.symbol not in symbol_to_orders:
                        symbol_to_orders[order.symbol] = []
                    symbol_to_orders[order.symbol].append(order)
            
            # Create a mapping of symbols to historical trades
            symbol_to_trades = {}
            for trade in history:
                if trade.symbol not in symbol_to_trades:
                    symbol_to_trades[trade.symbol] = []
                symbol_to_trades[trade.symbol].append(trade)
            
            groups = {}
            
            # Group positions based on their associated orders
            for position in positions:
                # Find orders for this position's symbol
                position_orders = symbol_to_orders.get(position.symbol, [])
                position_trades = symbol_to_trades.get(position.symbol, [])
                
                # Determine underlying and asset class
                if self._is_option_symbol(position.symbol):
                    parsed = self._parse_option_symbol(position.symbol)
                    underlying = parsed["underlying"] if parsed else position.symbol
                    asset_class = "options"
                    expiry = parsed["expiry"] if parsed else None
                else:
                    underlying = position.symbol
                    asset_class = "stocks"
                    expiry = None
                
                # Try to group by order relationships
                group_key = self._find_or_create_group_key(
                    position, position_orders, position_trades, underlying, asset_class, expiry
                )
                
                if group_key not in groups:
                    # Get the most recent order/trade date for this group
                    order_date = None
                    if position_orders:
                        order_date = max(order.submitted_at for order in position_orders if order.submitted_at)
                    elif position_trades:
                        order_date = max(trade.date for trade in position_trades)
                    
                    groups[group_key] = {
                        "underlying": underlying,
                        "asset_class": asset_class,
                        "expiry": expiry,
                        "positions": [],
                        "orders": [],
                        "trades": [],
                        "trade_date": order_date
                    }
                
                groups[group_key]["positions"].append(position)
                groups[group_key]["orders"].extend(position_orders)
                groups[group_key]["trades"].extend(position_trades)
            
            logger.info(f"📊 Created {len(groups)} order-based position groups")
            return self._convert_groups_to_position_groups(groups)
                
        except Exception as e:
            self._log_error("_group_positions_by_order_chains", e)
            return []
    
    def _find_or_create_group_key(self, position: Position, position_orders: List[Order], 
                                position_trades: List[HistoricalTrade], underlying: str, 
                                asset_class: str, expiry: Optional[str]) -> str:
        """Find or create a group key for a position based on its orders and trades."""
        try:
            # If we have orders for this position, try to group by order timing/relationship
            if position_orders:
                # Sort orders by submission time to find the earliest
                sorted_orders = sorted(position_orders, key=lambda o: o.submitted_at or "")
                earliest_order = sorted_orders[0] if sorted_orders else None
                
                if earliest_order and earliest_order.submitted_at:
                    # Use the date part of the earliest order as part of the group key
                    order_date = earliest_order.submitted_at[:10]  # Extract YYYY-MM-DD
                    
                    # For multi-leg orders, group by the order ID or submission time
                    if earliest_order.legs and len(earliest_order.legs) > 1:
                        return f"{underlying}_{order_date}_multileg_{earliest_order.id}"
                    else:
                        # For single-leg orders, group by underlying, expiry, and date
                        if asset_class == "options" and expiry:
                            return f"{underlying}_{expiry}_{order_date}"
                        else:
                            return f"{underlying}_{order_date}"
            
            # If we have historical trades, use trade date for grouping
            elif position_trades:
                # Sort trades by date to find the earliest
                sorted_trades = sorted(position_trades, key=lambda t: t.date)
                earliest_trade = sorted_trades[0] if sorted_trades else None
                
                if earliest_trade:
                    trade_date = earliest_trade.date[:10] if len(earliest_trade.date) >= 10 else earliest_trade.date
                    
                    if asset_class == "options" and expiry:
                        return f"{underlying}_{expiry}_{trade_date}"
                    else:
                        return f"{underlying}_{trade_date}"
            
            # Fallback: group by underlying and expiry only
            if asset_class == "options" and expiry:
                return f"{underlying}_{expiry}_individual"
            else:
                return f"{underlying}_individual"
                
        except Exception as e:
            self._log_error(f"_find_or_create_group_key for {position.symbol}", e)
            # Fallback to simple grouping
            return f"{underlying}_fallback_{position.symbol}"
    
    def _group_with_historical_data(self, positions: List[Position], 
                                  history: List[HistoricalTrade]) -> List[PositionGroup]:
        """Group positions using historical trade data."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime
            
            # Create groups based on underlying symbol and expiry, refined by historical data
            groups = {}
            
            # Create a map of symbol to historical trades for faster lookup
            symbol_to_trades = {}
            for trade in history:
                if trade.symbol not in symbol_to_trades:
                    symbol_to_trades[trade.symbol] = []
                symbol_to_trades[trade.symbol].append(trade)
            
            # Group positions by underlying and expiry, then refine by historical data
            for position in positions:
                # Determine underlying symbol for grouping
                if self._is_option_symbol(position.symbol):
                    parsed = self._parse_option_symbol(position.symbol)
                    underlying = parsed["underlying"] if parsed else position.symbol
                    asset_class = "options"
                    expiry = parsed["expiry"] if parsed else None
                else:
                    underlying = position.symbol
                    asset_class = "stocks"
                    expiry = None
                
                # Get historical trades for this position
                position_trades = symbol_to_trades.get(position.symbol, [])
                
                # Create group key - prioritize grouping by underlying and expiry for options
                if asset_class == "options" and expiry:
                    # For options, group by underlying and expiry first
                    group_key = f"{underlying}_{expiry}"
                else:
                    # For stocks or options without expiry, group by underlying
                    group_key = underlying
                
                # If we have historical trades, try to refine the grouping by date
                if position_trades:
                    # Find the most recent trade date
                    most_recent_trade = max(position_trades, key=lambda t: t.date)
                    trade_date = most_recent_trade.date
                    # Add date to group key to separate orders from different days
                    group_key = f"{group_key}_{trade_date}"
                
                if group_key not in groups:
                    groups[group_key] = {
                        "underlying": underlying,
                        "asset_class": asset_class,
                        "expiry": expiry,
                        "positions": [],
                        "trades": [],
                        "trade_date": position_trades[0].date if position_trades else None
                    }
                
                groups[group_key]["positions"].append(position)
                groups[group_key]["trades"].extend(position_trades)
            
            return self._convert_groups_to_position_groups(groups)
            
        except Exception as e:
            self._log_error("_group_with_historical_data", e)
            return []
    
    def _group_without_historical_data(self, positions: List[Position]) -> List[PositionGroup]:
        """Group positions intelligently without historical data using strategy detection."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime
            
            # Separate options and stocks
            option_positions = [p for p in positions if self._is_option_symbol(p.symbol)]
            stock_positions = [p for p in positions if not self._is_option_symbol(p.symbol)]
            
            groups = {}
            
            # Group stock positions individually
            for position in stock_positions:
                group_key = f"stock_{position.symbol}"
                groups[group_key] = {
                    "underlying": position.symbol,
                    "asset_class": "stocks",
                    "expiry": None,
                    "positions": [position],
                    "trades": [],
                    "trade_date": None
                }
            
            # Group option positions by underlying and expiration, then try to detect strategies
            option_groups = {}
            for position in option_positions:
                parsed = self._parse_option_symbol(position.symbol)
                if not parsed:
                    continue
                    
                underlying = parsed["underlying"]
                expiry = parsed["expiry"]
                group_key = f"{underlying}_{expiry}"
                
                if group_key not in option_groups:
                    option_groups[group_key] = {
                        "underlying": underlying,
                        "asset_class": "options",
                        "expiry": expiry,
                        "positions": [],
                        "trades": [],
                        "trade_date": None
                    }
                
                option_groups[group_key]["positions"].append(position)
            
            # Now try to split large option groups into logical strategy groups
            for group_key, group_data in option_groups.items():
                if len(group_data["positions"]) <= 4:
                    # Small group, keep as is
                    groups[group_key] = group_data
                else:
                    # Large group, try to split into logical strategies
                    split_groups = self._split_into_strategy_groups(group_key, group_data)
                    groups.update(split_groups)
            
            return self._convert_groups_to_position_groups(groups)
            
        except Exception as e:
            self._log_error("_group_without_historical_data", e)
            return []
    
    def _split_into_strategy_groups(self, base_key: str, group_data: Dict[str, Any]) -> Dict[str, Dict[str, Any]]:
        """Split a large position group into smaller strategy-based groups."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            
            positions = group_data["positions"]
            
            # If we have exactly 6 positions, this is likely one Iron Condor (4 legs) + 2 other positions
            # Let's try to detect the Iron Condor first and group the rest logically
            if len(positions) == 6:
                logger.info(f"📊 Analyzing 6-position group for {base_key}")
                
                # Try to identify Iron Condor (4 legs)
                iron_condor_positions = self._identify_iron_condors(positions)
                
                if iron_condor_positions and len(iron_condor_positions) > 0:
                    # We found an Iron Condor, create one group for it
                    ic_positions = iron_condor_positions[0]  # Take the first (and likely only) Iron Condor
                    
                    # Find remaining positions
                    used_symbols = set(p.symbol for p in ic_positions)
                    remaining_positions = [p for p in positions if p.symbol not in used_symbols]
                    
                    logger.info(f"📊 Found Iron Condor with {len(ic_positions)} legs, {len(remaining_positions)} remaining")
                    
                    split_groups = {}
                    
                    # Create Iron Condor group
                    split_groups[f"{base_key}_iron_condor"] = {
                        **group_data,
                        "positions": ic_positions
                    }
                    
                    # Group remaining positions (likely a spread or individual positions)
                    if len(remaining_positions) == 2:
                        # Likely a spread
                        split_groups[f"{base_key}_spread"] = {
                            **group_data,
                            "positions": remaining_positions
                        }
                    else:
                        # Individual positions
                        for i, pos in enumerate(remaining_positions):
                            split_groups[f"{base_key}_single_{i}"] = {
                                **group_data,
                                "positions": [pos]
                            }
                    
                    return split_groups
            
            # Fallback: if we can't identify clear patterns, split into logical chunks
            # Try to keep it simple - group in pairs (common for spreads)
            split_groups = {}
            group_counter = 0
            
            for i in range(0, len(positions), 2):
                chunk = positions[i:i+2]
                split_groups[f"{base_key}_group_{group_counter}"] = {
                    **group_data,
                    "positions": chunk
                }
                group_counter += 1
            
            logger.info(f"📊 Split {len(positions)} positions into {len(split_groups)} groups")
            return split_groups
            
        except Exception as e:
            self._log_error(f"_split_into_strategy_groups for {base_key}", e)
            # Fallback: return original group
            return {base_key: group_data}
    
    def _identify_iron_condors(self, positions: List[Position]) -> List[List[Position]]:
        """Try to identify Iron Condor patterns in positions."""
        try:
            # Parse all positions
            parsed_positions = []
            for pos in positions:
                parsed = self._parse_option_symbol(pos.symbol)
                if parsed:
                    parsed_positions.append({
                        "position": pos,
                        "strike": parsed["strike"],
                        "type": parsed["type"],
                        "qty": pos.qty
                    })
            
            # Group by type
            calls = [p for p in parsed_positions if p["type"] == "call"]
            puts = [p for p in parsed_positions if p["type"] == "put"]
            
            iron_condors = []
            
            # Look for Iron Condor patterns: 2 calls + 2 puts with specific strike relationships
            if len(calls) >= 2 and len(puts) >= 2:
                # Sort by strike
                calls.sort(key=lambda x: x["strike"])
                puts.sort(key=lambda x: x["strike"])
                
                # Try to match Iron Condor patterns
                for i in range(len(calls) - 1):
                    for j in range(len(puts) - 1):
                        call1, call2 = calls[i], calls[i + 1]
                        put1, put2 = puts[j], puts[j + 1]
                        
                        # Check if this could be an Iron Condor
                        # Typical pattern: sell call at higher strike, buy call at even higher strike
                        #                  sell put at lower strike, buy put at even lower strike
                        if (call1["strike"] < call2["strike"] and 
                            put1["strike"] < put2["strike"] and
                            put2["strike"] < call1["strike"]):  # puts below calls
                            
                            ic_positions = [
                                call1["position"], call2["position"],
                                put1["position"], put2["position"]
                            ]
                            iron_condors.append(ic_positions)
            
            return iron_condors
            
        except Exception as e:
            self._log_error("_identify_iron_condors", e)
            return []
    
    def _convert_groups_to_position_groups(self, groups: Dict[str, Dict[str, Any]]) -> List[PositionGroup]:
        """Convert group dictionaries to PositionGroup objects."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime
            
            position_groups = []
            for group_key, group_data in groups.items():
                try:
                    # Calculate days to expiration
                    dte = None
                    if group_data["expiry"]:
                        try:
                            expiry_date = datetime.strptime(group_data["expiry"], "%Y-%m-%d")
                            dte = (expiry_date - datetime.now()).days
                        except:
                            dte = None
                    
                    # Detect strategy for options
                    strategy = "Single Position"
                    if group_data["asset_class"] == "options" and len(group_data["positions"]) > 1:
                        # Create legs for strategy detection
                        legs = []
                        for pos in group_data["positions"]:
                            legs.append({
                                "symbol": pos.symbol,
                                "side": "buy_to_open" if pos.qty > 0 else "sell_to_open",
                                "qty": abs(pos.qty)
                            })
                        
                        try:
                            # Import the strategy detection function
                            from ..utils.optionsStrategies import detectStrategy
                            strategy = detectStrategy(legs)
                        except (ImportError, Exception) as e:
                            logger.warning(f"Could not detect strategy: {e}")
                            # Fallback if import fails
                            strategy = f"{len(legs)}-Leg Strategy"
                    elif group_data["asset_class"] == "options":
                        strategy = "Single Option"
                    elif group_data["asset_class"] == "stocks":
                        strategy = "Stock Position"
                    
                    # Calculate totals (will be updated with real prices later)
                    total_qty = sum(pos.qty for pos in group_data["positions"])
                    total_cost_basis = sum(pos.cost_basis for pos in group_data["positions"])
                    
                    # Get order date from trades
                    order_date = None
                    if group_data["trades"]:
                        order_date = min(trade.date for trade in group_data["trades"])
                    
                    position_group = PositionGroup(
                        id=group_key,
                        symbol=group_data["underlying"],
                        strategy=strategy,
                        asset_class=group_data["asset_class"],
                        total_qty=total_qty,
                        total_cost_basis=total_cost_basis,
                        total_market_value=0,  # Will be calculated later
                        total_unrealized_pl=0,  # Will be calculated later
                        total_unrealized_plpc=0,  # Will be calculated later
                        pl_day=0,  # Will be calculated later
                        pl_open=0,  # Will be calculated later
                        legs=group_data["positions"],
                        order_date=order_date,
                        expiration_date=group_data["expiry"],
                        dte=dte
                    )
                    
                    position_groups.append(position_group)
                    
                except Exception as e:
                    self._log_error(f"Error creating position group for {group_key}", e)
                    continue
            
            return position_groups
            
        except Exception as e:
            self._log_error("_convert_groups_to_position_groups", e)
            return []
            
            # Convert final groups to PositionGroup objects
            position_groups = []
            for group_key, group_data in final_groups.items():
                try:
                    # Calculate days to expiration
                    dte = None
                    if group_data["expiry"]:
                        try:
                            expiry_date = datetime.strptime(group_data["expiry"], "%Y-%m-%d")
                            dte = (expiry_date - datetime.now()).days
                        except:
                            dte = None
                    
                    # Detect strategy for options
                    strategy = "Single Position"
                    if group_data["asset_class"] == "options" and len(group_data["positions"]) > 1:
                        # Create legs for strategy detection
                        legs = []
                        for pos in group_data["positions"]:
                            legs.append({
                                "symbol": pos.symbol,
                                "side": "buy_to_open" if pos.qty > 0 else "sell_to_open",
                                "qty": abs(pos.qty)
                            })
                        
                        try:
                            # Import the strategy detection function
                            from ..utils.optionsStrategies import detectStrategy
                            strategy = detectStrategy(legs)
                        except (ImportError, Exception) as e:
                            logger.warning(f"Could not detect strategy: {e}")
                            # Fallback if import fails
                            strategy = f"{len(legs)}-Leg Strategy"
                    elif group_data["asset_class"] == "options":
                        strategy = "Single Option"
                    elif group_data["asset_class"] == "stocks":
                        strategy = "Stock Position"
                    
                    # Calculate totals (will be updated with real prices later)
                    total_qty = sum(pos.qty for pos in group_data["positions"])
                    total_cost_basis = sum(pos.cost_basis for pos in group_data["positions"])
                    
                    # Get order date from trades
                    order_date = None
                    if group_data["trades"]:
                        order_date = min(trade.date for trade in group_data["trades"])
                    
                    position_group = PositionGroup(
                        id=group_key,
                        symbol=group_data["underlying"],
                        strategy=strategy,
                        asset_class=group_data["asset_class"],
                        total_qty=total_qty,
                        total_cost_basis=total_cost_basis,
                        total_market_value=0,  # Will be calculated later
                        total_unrealized_pl=0,  # Will be calculated later
                        total_unrealized_plpc=0,  # Will be calculated later
                        pl_day=0,  # Will be calculated later
                        pl_open=0,  # Will be calculated later
                        legs=group_data["positions"],
                        order_date=order_date,
                        expiration_date=group_data["expiry"],
                        dte=dte
                    )
                    
                    position_groups.append(position_group)
                    
                except Exception as e:
                    self._log_error(f"Error creating position group for {group_key}", e)
                    continue
            
            return position_groups
            
        except Exception as e:
            self._log_error("_group_positions_by_order_chains", e)
            return []
    
    def _create_hierarchical_groups(self, positions: List[Position], 
                                  history: List[HistoricalTrade], 
                                  current_orders: List[Order],
                                  current_prices: Dict[str, float]) -> List[Dict[str, Any]]:
        """Create Tasty Trade-style hierarchical grouping: Symbol -> Strategies -> Legs."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime
            
            logger.info("📊 Creating hierarchical symbol groups")
            
            # Step 1: Group positions by underlying symbol
            symbol_groups = {}
            
            for position in positions:
                # Determine underlying symbol
                if self._is_option_symbol(position.symbol):
                    parsed = self._parse_option_symbol(position.symbol)
                    underlying = parsed["underlying"] if parsed else position.symbol
                    asset_class = "options"
                else:
                    underlying = position.symbol
                    asset_class = "stocks"
                
                if underlying not in symbol_groups:
                    symbol_groups[underlying] = {
                        "symbol": underlying,
                        "asset_class": asset_class,
                        "total_pl_day": 0,
                        "total_pl_open": 0,
                        "total_qty": 0,
                        "total_cost_basis": 0,
                        "total_market_value": 0,
                        "strategies": []
                    }
                
                # Add position to symbol group
                symbol_groups[underlying]["total_qty"] += position.qty
                symbol_groups[underlying]["total_cost_basis"] += position.cost_basis
            
            # Step 2: Within each symbol, group positions by strategy/order chains
            for underlying, symbol_group in symbol_groups.items():
                # Get positions for this underlying
                underlying_positions = [
                    pos for pos in positions 
                    if (self._parse_option_symbol(pos.symbol)["underlying"] if self._is_option_symbol(pos.symbol) else pos.symbol) == underlying
                ]
                
                # Group these positions by strategy using existing logic
                strategy_groups = self._group_positions_by_order_chains(
                    underlying_positions, history, current_orders
                )
                
                # Convert strategy groups to the hierarchical format
                for strategy_group in strategy_groups:
                    # Calculate P&L for this strategy
                    self._calculate_group_pnl(strategy_group, current_prices)
                    
                    strategy_data = {
                        "name": strategy_group.strategy,
                        "pl_day": strategy_group.pl_day,
                        "pl_open": strategy_group.pl_open,
                        "total_qty": strategy_group.total_qty,
                        "cost_basis": strategy_group.total_cost_basis,
                        "market_value": strategy_group.total_market_value,
                        "dte": strategy_group.dte,
                        "legs": []
                    }
                    
                    # Add individual legs
                    for leg in strategy_group.legs:
                        # Update leg with current price
                        current_price = current_prices.get(leg.symbol, 0)
                        if current_price > 0:
                            leg.current_price = current_price
                            if self._is_option_symbol(leg.symbol):
                                leg.market_value = current_price * leg.qty * 100
                            else:
                                leg.market_value = current_price * leg.qty
                            leg.unrealized_pl = leg.market_value - leg.cost_basis
                        
                        leg_data = {
                            "symbol": leg.symbol,
                            "qty": leg.qty,
                            "current_price": leg.current_price,
                            "cost_basis": leg.cost_basis,
                            "market_value": leg.market_value,
                            "unrealized_pl": leg.unrealized_pl,
                            "asset_class": leg.asset_class
                        }
                        strategy_data["legs"].append(leg_data)
                    
                    symbol_group["strategies"].append(strategy_data)
                    
                    # Aggregate to symbol level
                    symbol_group["total_pl_day"] += strategy_group.pl_day
                    symbol_group["total_pl_open"] += strategy_group.pl_open
                    symbol_group["total_market_value"] += strategy_group.total_market_value
            
            # Convert to list format
            result = list(symbol_groups.values())
            logger.info(f"✅ Created {len(result)} hierarchical symbol groups")
            return result
            
        except Exception as e:
            self._log_error("_create_hierarchical_groups", e)
            return []

    async def _create_simplified_hierarchical_groups(self, positions: List[Position], 
                                             history: List[HistoricalTrade], 
                                             current_orders: List[Order]) -> List[Dict[str, Any]]:
        """Create simplified hierarchical grouping with only static broker data."""
        try:
            from ..utils.optionsStrategies import detectStrategy
            from datetime import datetime
            
            logger.info("📊 Creating simplified hierarchical symbol groups")
            
            # Step 1: Group positions by underlying symbol
            symbol_groups = {}
            
            for position in positions:
                # Determine underlying symbol
                if self._is_option_symbol(position.symbol):
                    parsed = self._parse_option_symbol(position.symbol)
                    underlying = parsed["underlying"] if parsed else position.symbol
                    asset_class = "options"
                else:
                    underlying = position.symbol
                    asset_class = "stocks"
                
                if underlying not in symbol_groups:
                    symbol_groups[underlying] = {
                        "symbol": underlying,
                        "asset_class": asset_class,
                        "positions_by_expiry": {}
                    }
                
                # Group by expiry for options, or use "stock" for stocks
                if self._is_option_symbol(position.symbol):
                    parsed = self._parse_option_symbol(position.symbol)
                    expiry_key = parsed["expiry"] if parsed else "unknown"
                else:
                    expiry_key = "stock"
                
                if expiry_key not in symbol_groups[underlying]["positions_by_expiry"]:
                    symbol_groups[underlying]["positions_by_expiry"][expiry_key] = []
                
                symbol_groups[underlying]["positions_by_expiry"][expiry_key].append(position)
            
            # Step 2: Convert to final format with strategies
            result = []
            for underlying, symbol_group in symbol_groups.items():
                strategies = []
                
                for expiry_key, expiry_positions in symbol_group["positions_by_expiry"].items():
                    # Detect strategy for this group of positions
                    strategy_name = self._detect_strategy_name(expiry_positions)
                    
                    # Calculate DTE for options
                    dte = None
                    if expiry_key != "stock":
                        try:
                            expiry_date = datetime.strptime(expiry_key, "%Y-%m-%d")
                            dte = (expiry_date - datetime.now()).days
                        except:
                            dte = None
                    
                    # Calculate strategy totals (static data only)
                    strategy_total_qty = sum(pos.qty for pos in expiry_positions)
                    strategy_cost_basis = sum(pos.cost_basis for pos in expiry_positions)
                    
                    # Create legs with only static broker data
                    legs = []
                    for pos in expiry_positions:
                        # Calculate avg_entry_price from cost_basis
                        if self._is_option_symbol(pos.symbol):
                            # For options: cost_basis / qty / 100 (Tradier specific calculation)
                            avg_entry_price = abs(pos.cost_basis / pos.qty / 100) if pos.qty != 0 else 0
                        else:
                            # For stocks: cost_basis / qty
                            avg_entry_price = abs(pos.cost_basis / pos.qty) if pos.qty != 0 else 0
                        
                        # Fetch lastday_price for each position
                        lastday_price = await self._get_lastday_price(pos.symbol)
                        
                        legs.append({
                            "symbol": pos.symbol,
                            "qty": pos.qty,
                            "avg_entry_price": avg_entry_price,
                            "cost_basis": pos.cost_basis,
                            "asset_class": pos.asset_class,
                            "lastday_price": lastday_price
                        })
                    
                    strategy = {
                        "name": strategy_name,
                        "total_qty": strategy_total_qty,
                        "cost_basis": strategy_cost_basis,
                        "dte": dte,
                        "legs": legs
                    }
                    strategies.append(strategy)
                
                result.append({
                    "symbol": underlying,
                    "asset_class": symbol_group["asset_class"],
                    "strategies": strategies
                })
            
            logger.info(f"✅ Created {len(result)} simplified symbol groups")
            return result
            
        except Exception as e:
            self._log_error("_create_simplified_hierarchical_groups", e)
            return []

    def _detect_strategy_name(self, positions: List[Position]) -> str:
        """Detect strategy name based on positions using centralized strategy detection."""
        try:
            if len(positions) == 1:
                if self._is_option_symbol(positions[0].symbol):
                    return "Single Option"
                else:
                    return "Stock Position"
            
            # For multi-leg strategies, use the centralized strategy detection
            option_positions = [pos for pos in positions if self._is_option_symbol(pos.symbol)]
            
            if len(option_positions) >= 2:
                # Convert positions to legs format expected by detectStrategy
                legs = []
                for pos in option_positions:
                    # Determine side based on quantity (positive = buy, negative = sell)
                    side = "buy_to_open" if pos.qty > 0 else "sell_to_open"
                    legs.append({
                        "symbol": pos.symbol,
                        "side": side,
                        "qty": abs(pos.qty)
                    })
                
                # Use centralized strategy detection
                try:
                    from ..utils.optionsStrategies import detectStrategy
                    strategy = detectStrategy(legs)
                    return strategy
                except ImportError as e:
                    logger.warning(f"Could not import detectStrategy: {e}")
                    # Fallback to generic naming
                    pass
            
            # Fallback to generic naming
            if len(positions) == 2:
                return "2-Leg Strategy"
            elif len(positions) == 4:
                return "4-Leg Strategy"
            elif len(positions) == 6:
                return "6-Leg Strategy"
            else:
                return f"{len(positions)}-Leg Strategy"
                
        except Exception as e:
            self._log_error("_detect_strategy_name", e)
            return "Unknown Strategy"

    async def _get_lastday_price(self, symbol: str) -> Optional[float]:
        """Get the previous day's closing price for a given symbol with caching."""
        try:
            today = datetime.now().date()
            
            # Check if we need to refresh the cache
            if self._lastday_price_cache_date != today:
                self._lastday_price_cache = {}
                self._lastday_price_cache_date = today
            
            # Check if the price is in the cache
            if symbol in self._lastday_price_cache:
                return self._lastday_price_cache[symbol]
            
            # If not in cache, fetch the price
            previous_day = (today - timedelta(days=1)).strftime('%Y-%m-%d')
            bars = await self.get_historical_bars(symbol, timeframe='D', end_date=previous_day, limit=1)
            
            if bars and len(bars) > 0:
                price = bars[0]['close']
                # Cache the result
                self._lastday_price_cache[symbol] = price
                return price
            else:
                logger.warning(f"No historical data found for {symbol} on {previous_day}")
                return None
        except Exception as e:
            self._log_error(f"_get_lastday_price for {symbol}", e)
            return None
