import asyncio
import requests
import json
import logging
import time
import httpx
import websockets
from typing import List, Dict, Optional, Any, Tuple
from datetime import datetime, timedelta

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
)
from ..config import settings

logger = logging.getLogger(__name__)

class DXLinkCandleClient:
    """Dedicated client for DXLink Candle Events to fetch historical data."""
    
    def __init__(self, dxlink_url: str, quote_token: str):
        self.dxlink_url = dxlink_url
        self.quote_token = quote_token
        self.websocket = None
        self.channel_id = 1  # Use channel 1 for candle data
        
    async def get_candles(self, symbol: str, timeframe: str, from_time: int, limit: int = 500) -> List[Dict]:
        """Get historical candles via DXLink streaming."""
        try:
            logger.info(f"DXLink: Getting candles for {symbol} {timeframe} from {from_time}")
            
            # Connect to DXLink WebSocket
            self.websocket = await websockets.connect(
                self.dxlink_url,
                ping_interval=30,
                ping_timeout=10,
                close_timeout=5
            )
            
            # Execute DXLink setup sequence
            if not await self._dxlink_setup_sequence():
                logger.error("DXLink setup sequence failed")
                return []
            
            # Subscribe and collect candles
            candles = await self._subscribe_and_collect_candles(symbol, timeframe, from_time, limit)
            
            logger.info(f"DXLink: Collected {len(candles)} candles for {symbol}")
            return candles
            
        except Exception as e:
            logger.error(f"DXLink candle collection error: {e}")
            return []
        finally:
            await self._disconnect()
    
    async def _dxlink_setup_sequence(self) -> bool:
        """Execute the full DXLink setup sequence."""
        try:
            # 1. SETUP
            setup_msg = {
                "type": "SETUP",
                "channel": 0,
                "version": "0.1-DXF-JS/0.3.0",
                "keepaliveTimeout": 60,
                "acceptKeepaliveTimeout": 60
            }
            await self.websocket.send(json.dumps(setup_msg))
            
            # Wait for SETUP response
            response = await asyncio.wait_for(self.websocket.recv(), timeout=10)
            setup_response = json.loads(response)
            if setup_response.get("type") != "SETUP":
                logger.error(f"Unexpected SETUP response: {setup_response}")
                return False
            
            # 2. Wait for AUTH_STATE: UNAUTHORIZED
            auth_state = await asyncio.wait_for(self.websocket.recv(), timeout=10)
            auth_response = json.loads(auth_state)
            if auth_response.get("type") != "AUTH_STATE" or auth_response.get("state") != "UNAUTHORIZED":
                logger.error(f"Unexpected AUTH_STATE response: {auth_response}")
                return False
            
            # 3. AUTHORIZE
            auth_msg = {
                "type": "AUTH",
                "channel": 0,
                "token": self.quote_token
            }
            await self.websocket.send(json.dumps(auth_msg))
            
            # Wait for AUTH_STATE: AUTHORIZED
            auth_success = await asyncio.wait_for(self.websocket.recv(), timeout=10)
            auth_success_response = json.loads(auth_success)
            if (auth_success_response.get("type") != "AUTH_STATE" or 
                auth_success_response.get("state") != "AUTHORIZED"):
                logger.error(f"Authorization failed: {auth_success_response}")
                return False
            
            # 4. CHANNEL_REQUEST
            channel_msg = {
                "type": "CHANNEL_REQUEST",
                "channel": self.channel_id,
                "service": "FEED",
                "parameters": {"contract": "AUTO"}
            }
            await self.websocket.send(json.dumps(channel_msg))
            
            # Wait for CHANNEL_OPENED
            channel_response = await asyncio.wait_for(self.websocket.recv(), timeout=10)
            channel_opened = json.loads(channel_response)
            if channel_opened.get("type") != "CHANNEL_OPENED":
                logger.error(f"Channel open failed: {channel_opened}")
                return False
            
            # 5. FEED_SETUP for Candle events
            feed_setup_msg = {
                "type": "FEED_SETUP",
                "channel": self.channel_id,
                "acceptAggregationPeriod": 0.1,
                "acceptDataFormat": "COMPACT",
                "acceptEventFields": {
                    "Candle": ["eventType", "eventSymbol", "time", "open", "high", "low", "close", "volume"]
                }
            }
            await self.websocket.send(json.dumps(feed_setup_msg))
            
            # Wait for FEED_CONFIG
            feed_response = await asyncio.wait_for(self.websocket.recv(), timeout=10)
            feed_config = json.loads(feed_response)
            if feed_config.get("type") != "FEED_CONFIG":
                logger.error(f"Feed setup failed: {feed_config}")
                return False
            
            logger.info("DXLink setup sequence completed successfully")
            return True
            
        except asyncio.TimeoutError:
            logger.error("DXLink setup sequence timeout")
            return False
        except Exception as e:
            logger.error(f"DXLink setup sequence error: {e}")
            return False
    
    async def _subscribe_and_collect_candles(self, symbol: str, timeframe: str, from_time: int, limit: int) -> List[Dict]:
        """Subscribe to candle events and collect historical data with optimized performance."""
        try:
            # Create candle symbol format: SYMBOL{=PERIODtype}
            candle_symbol = f"{symbol}{{={timeframe}}}"
            
            # FEED_SUBSCRIPTION with fromTime for historical data and trading session parameters
            subscription_msg = {
                "type": "FEED_SUBSCRIPTION",
                "channel": self.channel_id,
                "reset": True,
                "add": [{
                    "type": "Candle",
                    "symbol": candle_symbol,
                    "fromTime": from_time,
                    "tho": "true",  # Trading hours only - exclude extended session
                    "a": "s"        # Align candles to trading session (not midnight)
                }]
            }
            await self.websocket.send(json.dumps(subscription_msg))
            
            logger.info(f"DXLink: Subscribed to {candle_symbol} from timestamp {from_time}")
            
            # Optimized data collection with adaptive timeout strategy and deduplication
            candles_dict = {}  # Use dict to deduplicate by timestamp
            consecutive_empty_messages = 0
            max_empty_messages = 2  # Very aggressive - stop after 2 empty messages
            start_time = time.time()
            last_data_time = time.time()
            initial_burst_complete = False
            
            # Track data flow patterns to detect completion
            no_new_data_iterations = 0
            max_no_data_iterations = 2  # Very aggressive - stop after 2 iterations without new data
            
            while len(candles_dict) < limit and no_new_data_iterations < max_no_data_iterations:
                try:
                    # Short timeout - we want to detect completion quickly
                    timeout = 1.0
                    message = await asyncio.wait_for(self.websocket.recv(), timeout=timeout)
                    data = json.loads(message)
                    
                    if data.get("type") == "FEED_DATA":
                        feed_data = data.get("data", [])
                        
                        # Process candle events
                        new_candles = self._process_feed_data(feed_data)
                        if new_candles:
                            # Deduplicate by timestamp - keep the most recent version of each candle
                            candles_before = len(candles_dict)
                            for candle in new_candles:
                                timestamp = candle.get('time')
                                if timestamp:
                                    # Always keep the latest version (DXLink sends updates for current candle)
                                    candles_dict[timestamp] = candle
                            
                            # Check if we actually got new unique candles
                            candles_after = len(candles_dict)
                            if candles_after > candles_before:
                                # We got new unique data, reset counters
                                no_new_data_iterations = 0
                                consecutive_empty_messages = 0
                                last_data_time = time.time()
                                
                                # Detect initial burst completion (when we get < 10 candles at once)
                                if not initial_burst_complete and len(new_candles) < 10:
                                    initial_burst_complete = True
                                    logger.info(f"DXLink: Initial burst complete, got {candles_after} unique candles")
                                
                                # Log progress
                                elapsed = time.time() - start_time
                                if candles_after % 50 == 0 or elapsed > 2:
                                    logger.info(f"DXLink: Received {len(new_candles)} candles, total unique: {candles_after} (elapsed: {elapsed:.1f}s)")
                                    start_time = time.time()  # Reset timer
                            else:
                                # Got candles but they were duplicates - increment counter
                                no_new_data_iterations += 1
                                logger.debug(f"DXLink: Got {len(new_candles)} duplicate candles, no new unique data (iteration {no_new_data_iterations})")
                        else:
                            # No candles in this message
                            no_new_data_iterations += 1
                            consecutive_empty_messages += 1
                    else:
                        # Handle other message types (keepalive, etc.)
                        logger.debug(f"DXLink: Received {data.get('type')} message")
                        no_new_data_iterations += 1
                        consecutive_empty_messages += 1
                        
                except asyncio.TimeoutError:
                    no_new_data_iterations += 1
                    consecutive_empty_messages += 1
                    time_since_last_data = time.time() - last_data_time
                    
                    logger.debug(f"DXLink: Timeout waiting for data (no_new_data: {no_new_data_iterations}, time since last: {time_since_last_data:.1f}s)")
                    
                    # Very aggressive completion detection
                    if candles_dict and no_new_data_iterations >= 1:
                        logger.info(f"DXLink: No new data for {no_new_data_iterations} iterations with {len(candles_dict)} unique candles, assuming complete")
                        break
                    continue
            
            # Convert dict back to list and sort by timestamp
            candles = list(candles_dict.values())
            candles.sort(key=lambda x: x.get('time', 0))
            
            # Don't remove the last candle automatically - let the API consumer decide
            # The deduplication already handles live updates properly
            
            # Apply limit
            if limit and len(candles) > limit:
                candles = candles[-limit:]
            
            total_elapsed = time.time() - (start_time if 'start_time' in locals() else time.time())
            logger.info(f"DXLink: Final candle count: {len(candles)} (total time: {total_elapsed:.1f}s)")
            return candles
            
        except Exception as e:
            logger.error(f"DXLink candle collection error: {e}")
            return []
    
    def _process_feed_data(self, feed_data: List) -> List[Dict]:
        """Process FEED_DATA messages and extract candle events."""
        candles = []
        
        try:
            # DXLink COMPACT format: flat array with candle data
            # Format: ['Candle', 'SYMBOL{=timeframe}', timestamp, open, high, low, close, volume, 'Candle', ...]
            for item in feed_data:
                if isinstance(item, list):
                    # Parse the flat array - each candle is 8 consecutive elements
                    i = 0
                    while i + 7 < len(item):
                        if item[i] == "Candle":
                            candle = {
                                "eventType": item[i],        # "Candle"
                                "eventSymbol": item[i + 1],  # "MSFT{=d}"
                                "time": item[i + 2],         # timestamp in ms
                                "open": item[i + 3],         # open price
                                "high": item[i + 4],         # high price
                                "low": item[i + 5],          # low price
                                "close": item[i + 6],        # close price
                                "volume": item[i + 7]        # volume
                            }
                            candles.append(candle)
                            i += 8  # Move to next candle (8 fields per candle)
                        else:
                            i += 1  # Skip non-candle data
        except Exception as e:
            logger.error(f"Error processing feed data: {e}")
            logger.error(f"Feed data structure: {feed_data}")
        
        return candles
    
    async def _disconnect(self):
        """Clean disconnect from DXLink."""
        try:
            if self.websocket:
                await self.websocket.close()
                self.websocket = None
                logger.info("DXLink WebSocket disconnected")
        except Exception as e:
            logger.error(f"Error disconnecting DXLink: {e}")

class SymbolLookupCache:
    """Smart caching for symbol lookup with prefix filtering."""
    
    def __init__(self, ttl_seconds: int = 21600, max_size: int = 500):
        self.ttl_seconds = ttl_seconds  # 6 hours (21600 seconds)
        self.max_size = max_size
        self.cache: Dict[str, Tuple[List[SymbolSearchResult], float]] = {}
    
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
        if len(self.cache) > self.max_size:
            # Simple LRU: remove oldest entries
            sorted_items = sorted(self.cache.items(), key=lambda x: x[1][1])
            items_to_remove = len(self.cache) - self.max_size + 1
            for key, _ in sorted_items[:items_to_remove]:
                del self.cache[key]
    
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
            # Cache the full results for better prefix filtering
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
                
                # Sort the filtered results by relevance
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

class TastyTradeProvider(BaseProvider):
    """
    TastyTrade provider implementation.
    
    Uses session-based authentication with 24-hour session tokens.
    For streaming, uses DXLink with separate quote tokens.
    """
    
    def __init__(self, username: str, password: str, base_url: str):
        super().__init__("TastyTrade")
        self.username = username
        self.password = password
        self.base_url = base_url
        self._session_token = None
        self._session_expires_at = None
        self._quote_token = None
        self._quote_token_expires_at = None
        self._dxlink_url = None
        self._stream_connection = None
        self._connection_ready = asyncio.Event()
        self._symbol_cache = SymbolLookupCache()
        
    async def _create_session(self) -> bool:
        """Create a new session with TastyTrade API."""
        try:
            url = f"{self.base_url}/sessions"
            headers = {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"  # Required by TastyTrade
            }
            
            payload = {
                "login": self.username,
                "password": self.password,
                "remember-me": False
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.post(url, headers=headers, json=payload)
                response.raise_for_status()
                
                data = response.json()
                session_data = data.get("data", {})
                
                self._session_token = session_data.get("session-token")
                session_expiration = session_data.get("session-expiration")
                
                if self._session_token and session_expiration:
                    # Parse expiration time
                    self._session_expires_at = datetime.fromisoformat(
                        session_expiration.replace('Z', '+00:00')
                    )
                    logger.info("TastyTrade session created successfully")
                    return True
                else:
                    logger.error("Failed to get session token from TastyTrade response")
                    return False
                    
        except Exception as e:
            logger.error(f"Error creating TastyTrade session: {e}")
            return False
    
    async def _ensure_valid_session(self) -> bool:
        """Ensure we have a valid session token."""
        if not self._session_token or not self._session_expires_at:
            return await self._create_session()
        
        # Check if session is about to expire (refresh 5 minutes early)
        if datetime.now(self._session_expires_at.tzinfo) >= (self._session_expires_at - timedelta(minutes=5)):
            logger.info("Session token expiring soon, refreshing...")
            return await self._create_session()
        
        return True
    
    async def _get_quote_token(self) -> bool:
        """Get quote token for DXLink streaming."""
        try:
            if not await self._ensure_valid_session():
                return False
            
            url = f"{self.base_url}/api-quote-tokens"
            headers = {
                "Authorization": self._session_token,  # No "Bearer" prefix for TastyTrade
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                quote_data = data.get("data", {})
                
                self._quote_token = quote_data.get("token")
                self._dxlink_url = quote_data.get("dxlink-url")
                
                if self._quote_token and self._dxlink_url:
                    # Quote tokens are valid for 24 hours
                    self._quote_token_expires_at = datetime.now() + timedelta(hours=24)
                    logger.info("TastyTrade quote token obtained successfully")
                    return True
                else:
                    logger.error("Failed to get quote token from TastyTrade")
                    return False
                    
        except Exception as e:
            logger.error(f"Error getting TastyTrade quote token: {e}")
            return False
    
    async def _make_authenticated_request(self, method: str, endpoint: str, **kwargs) -> Dict[str, Any]:
        """Make an authenticated request to TastyTrade API."""
        if not await self._ensure_valid_session():
            raise Exception("Failed to authenticate with TastyTrade")
        
        url = f"{self.base_url}{endpoint}"
        headers = kwargs.get("headers", {})
        headers.update({
            "Authorization": self._session_token,
            "Accept": "application/json",
            "User-Agent": "juicytrade/1.0"
        })
        
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.request(method, url, headers=headers, **kwargs)
            response.raise_for_status()
            return response.json()
    
    # === Market Data Methods ===
    
    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        """Get the latest stock quote for a symbol."""
        raise NotImplementedError("TastyTrade get_stock_quote not yet implemented")
    
    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        """Get stock quotes for multiple symbols."""
        raise NotImplementedError("TastyTrade get_stock_quotes not yet implemented")
    
    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """Get available expiration dates for options on a symbol."""
        raise NotImplementedError("TastyTrade get_expiration_dates not yet implemented")
    
    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        """Get options chain for a symbol and expiration date."""
        raise NotImplementedError("TastyTrade get_options_chain not yet implemented")
    
    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20) -> List[OptionContract]:
        """Get basic options chain (no Greeks) for fast loading, ATM-focused."""
        raise NotImplementedError("TastyTrade get_options_chain_basic not yet implemented")
    
    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """Get Greeks for multiple option symbols in batch."""
        raise NotImplementedError("TastyTrade get_options_greeks_batch not yet implemented")
    
    async def get_options_chain_smart(self, symbol: str, expiry: str, underlying_price: float = None, 
                                   atm_range: int = 20, include_greeks: bool = False, 
                                   strikes_only: bool = False) -> List[OptionContract]:
        """Get smart options chain with configurable loading."""
        raise NotImplementedError("TastyTrade get_options_chain_smart not yet implemented")
    
    async def get_next_market_date(self) -> str:
        """Get the next trading date."""
        raise NotImplementedError("TastyTrade get_next_market_date not yet implemented")
    
    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query using TastyTrade symbol search API with smart caching."""
        return await self._symbol_cache.search(query, self._api_lookup_symbols)
    
    async def _api_lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Make actual API call to TastyTrade symbol search endpoint."""
        try:
            # Ensure we have a valid session
            if not await self._ensure_valid_session():
                logger.error("Failed to authenticate for symbol lookup")
                return []
            
            url = f"{self.base_url}/symbols/search/{query}"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                
                # Handle both array response and potential wrapper object
                if isinstance(data, list):
                    items = data
                else:
                    # TastyTrade returns {'data': {'items': [...]}}
                    items = data.get('data', {}).get('items', [])
                
                # Transform to SymbolSearchResult objects
                results = []
                for item in items:
                    if isinstance(item, dict):
                        result = SymbolSearchResult(
                            symbol=item.get("symbol", ""),
                            description=item.get("description", ""),
                            exchange=item.get("listed-market", ""),
                            type=item.get("instrument-type", "")
                        )
                        results.append(result)
                
                # Sort by relevance (same logic as other providers)
                results.sort(key=self._create_symbol_sort_key(query))
                
                logger.info(f"Found {len(results)} symbols for query '{query}' from TastyTrade")
                return results
                
        except httpx.HTTPStatusError as e:
            if e.response.status_code == 401:
                # Session expired, try to re-authenticate
                logger.warning("TastyTrade session expired during symbol search")
                self._session_token = None
                self._session_expires_at = None
            logger.error(f"TastyTrade symbol lookup HTTP error for query '{query}': {e}")
            logger.error(f"TastyTrade symbol lookup HTTP response: {e.response.text if hasattr(e, 'response') else 'No response'}")
            return []
        except Exception as e:
            logger.error(f"TastyTrade symbol lookup error for query '{query}': {e}")
            return []
    
    def _create_symbol_sort_key(self, query: str):
        """Create sort key function for symbol search results."""
        def sort_key(result):
            symbol = result.symbol.upper()
            query_upper = query.upper()
            
            # Exact match = highest priority
            if symbol == query_upper:
                return (0, symbol)
            
            # Starts with query = second priority  
            if symbol.startswith(query_upper):
                return (1, len(symbol), symbol)
            
            # Contains query = third priority
            if query_upper in symbol:
                return (2, symbol.index(query_upper), len(symbol), symbol)
            
            # Description contains query = fourth priority
            description = result.description.upper()
            if query_upper in description:
                return (3, description.index(query_upper), len(description), symbol)
            
            return (4, symbol)
        
        return sort_key
    
    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """
        Get historical OHLCV bars for charting using DXLink Candle Events.
        
        This method:
        1. Maps timeframe to DXLink candle format
        2. Calculates appropriate fromTime based on parameters
        3. Establishes temporary DXLink connection
        4. Collects candle data via streaming
        5. Transforms to standard OHLCV format
        6. Cleans up connection
        """
        try:
            logger.info(f"TastyTrade: Getting historical bars for {symbol} {timeframe}")
            
            # 1. Validate inputs and map timeframe
            dxlink_timeframe = self._map_timeframe_to_dxlink(timeframe)
            if not dxlink_timeframe:
                logger.error(f"Unsupported timeframe: {timeframe}")
                return []
            
            logger.info(f"TastyTrade: Mapped {timeframe} -> {dxlink_timeframe}")
            
            # 2. Calculate fromTime for historical data
            from_time = self._calculate_from_time(start_date, timeframe)
            
            logger.info(f"TastyTrade: Using fromTime {from_time}")
            
            # 3. Ensure we have valid quote token
            if not await self._get_quote_token():
                logger.error("Failed to get quote token for historical data")
                return []
            
            logger.info(f"TastyTrade: Got quote token, DXLink URL: {self._dxlink_url}")
            
            # 4. Create DXLink candle client and collect data
            # If end_date is specified, we need to collect more data to ensure we have enough
            # bars within the date range after filtering
            collection_limit = limit
            if end_date:
                # Increase collection limit to account for potential filtering
                # Use a reasonable multiplier to ensure we get enough data
                collection_limit = max(limit * 3, 100)  # At least 3x the requested limit or 100
                logger.info(f"TastyTrade: Increased collection limit to {collection_limit} due to end_date filter")
            
            candle_client = DXLinkCandleClient(self._dxlink_url, self._quote_token)
            candles = await candle_client.get_candles(symbol, dxlink_timeframe, from_time, collection_limit)
            
            logger.info(f"TastyTrade: DXLink returned {len(candles)} raw candles")
            
            # 5. Transform to standard format and filter out null data
            result = []
            for candle in candles:
                transformed = self._transform_dxlink_candle(candle, timeframe)
                if transformed:
                    # Filter out bars with null or NaN OHLC data
                    import math
                    open_val = transformed.get('open')
                    high_val = transformed.get('high')
                    low_val = transformed.get('low')
                    close_val = transformed.get('close')
                    
                    # Check if all OHLC values are valid (not None and not NaN)
                    if (open_val is not None and not math.isnan(open_val) and
                        high_val is not None and not math.isnan(high_val) and
                        low_val is not None and not math.isnan(low_val) and
                        close_val is not None and not math.isnan(close_val)):
                        result.append(transformed)
                    else:
                        logger.debug(f"TastyTrade: Filtered out bar with null/NaN OHLC data: {transformed.get('time')}")
                else:
                    logger.warning(f"Failed to transform candle: {candle}")
            
            # If no data from DXLink, log the issue and return empty
            if not result:
                logger.warning(f"TastyTrade: No historical data received from DXLink for {symbol} {timeframe}")

            # 6. Sort by time and apply end_date filter if specified
            result.sort(key=lambda x: x['time'])
            
            # Apply end_date filter if specified
            if end_date:
                try:
                    # Parse end_date and filter out bars after this date
                    end_dt = datetime.strptime(end_date, '%Y-%m-%d')
                    end_date_str = end_dt.strftime('%Y-%m-%d')
                    
                    # Filter bars to only include those on or before end_date
                    filtered_result = []
                    for bar in result:
                        bar_date = bar['time'][:10]  # Extract date part (YYYY-MM-DD)
                        if bar_date <= end_date_str:
                            filtered_result.append(bar)
                    
                    result = filtered_result
                    logger.info(f"TastyTrade: Applied end_date filter {end_date}, {len(result)} bars remaining")
                    
                except Exception as e:
                    logger.error(f"Error applying end_date filter {end_date}: {e}")
            
            # Apply limit after filtering
            if limit and len(result) > limit:
                result = result[-limit:]  # Get most recent bars
            
            logger.info(f"TastyTrade: Retrieved {len(result)} historical bars for {symbol} ({timeframe})")
            return result
            
        except Exception as e:
            logger.error(f"TastyTrade get_historical_bars error for {symbol} {timeframe}: {e}")
            import traceback
            logger.error(f"TastyTrade get_historical_bars traceback: {traceback.format_exc()}")
            return []
    
    def _map_timeframe_to_dxlink(self, timeframe: str) -> Optional[str]:
        """Convert our timeframe to DXLink candle format."""
        timeframe_map = {
            '1m': '1m',
            '5m': '5m', 
            '15m': '15m',
            '30m': '30m',
            '1h': '1h',
            '4h': '4h',
            'D': 'd',      # Daily should be 'd', not '1d'
            'W': 'w',      # Weekly should be 'w', not '1w'  
            'M': 'mo'      # Monthly should be 'mo', not '1M'
        }
        return timeframe_map.get(timeframe)
    
    def _calculate_from_time(self, start_date: str, timeframe: str) -> int:
        """Calculate Unix timestamp for DXLink fromTime parameter."""
        try:
            if start_date:
                # Parse provided start date and convert to UTC midnight for DXLink
                start_dt = datetime.strptime(start_date, '%Y-%m-%d')
                
                # For DXLink, we need to send UTC midnight timestamp
                # Since DXLink sends data with UTC midnight timestamps, we should match that
                from datetime import timezone
                start_dt_utc = start_dt.replace(tzinfo=timezone.utc)
                
                logger.info(f"TastyTrade: Start date {start_date} -> UTC: {start_dt_utc.strftime('%Y-%m-%d %H:%M:%S %Z')}")
            else:
                # Calculate default lookback based on timeframe (same logic as Tradier/Alpaca)
                now = datetime.now()
                if timeframe in ['1m', '5m', '15m', '30m']:
                    # Intraday: last 5 days (same as Alpaca)
                    start_dt = now - timedelta(days=5)
                elif timeframe in ['1h', '4h']:
                    # Hourly: last 30 days
                    start_dt = now - timedelta(days=30)
                elif timeframe == 'D':
                    # Daily: last 365 days (same as Alpaca)
                    start_dt = now - timedelta(days=365)
                elif timeframe == 'W':
                    # Weekly: last 2 years
                    start_dt = now - timedelta(days=730)
                elif timeframe == 'M':
                    # Monthly: last 5 years
                    start_dt = now - timedelta(days=1825)
                else:
                    # Default: last 30 days
                    start_dt = now - timedelta(days=30)
                
                # Convert to UTC for consistency
                from datetime import timezone
                start_dt_utc = start_dt.replace(tzinfo=timezone.utc)
            
            # Convert to Unix timestamp (milliseconds since epoch for DXLink)
            # Subtract a small buffer to avoid edge cases with incomplete first candles
            buffer_hours = 24 if timeframe == 'D' else 1  # 1 day buffer for daily, 1 hour for others
            start_dt_utc_buffered = start_dt_utc - timedelta(hours=buffer_hours)
            from_time_ms = int(start_dt_utc_buffered.timestamp() * 1000)
            logger.info(f"TastyTrade: Using fromTime {from_time_ms} ({start_dt_utc_buffered.strftime('%Y-%m-%d %H:%M:%S %Z')}) with {buffer_hours}h buffer")
            return from_time_ms
            
        except Exception as e:
            logger.error(f"Error calculating fromTime: {e}")
            # Fallback: last 5 days in milliseconds (same as intraday default)
            fallback_dt = datetime.now() - timedelta(days=5)
            from datetime import timezone
            fallback_dt_utc = fallback_dt.replace(tzinfo=timezone.utc)
            return int(fallback_dt_utc.timestamp() * 1000)
    
    def _transform_dxlink_candle(self, candle_data: Dict, timeframe: str) -> Optional[Dict[str, Any]]:
        """Transform DXLink candle event to standard OHLCV format."""
        try:
            # Extract timestamp and convert to appropriate format
            timestamp = candle_data.get('time')
            if timestamp:
                # DXLink time is in milliseconds UTC, convert to datetime in UTC
                from datetime import timezone
                dt_utc = datetime.fromtimestamp(timestamp / 1000, tz=timezone.utc)
                
                # Format time based on timeframe
                if timeframe in ['1m', '5m', '15m', '30m', '1h', '4h']:
                    # Intraday - convert to Eastern Time and include time
                    from zoneinfo import ZoneInfo
                    dt_et = dt_utc.astimezone(ZoneInfo("America/New_York"))
                    time_str = dt_et.strftime('%Y-%m-%d %H:%M')
                else:
                    # Daily+ - DXLink sends midnight UTC timestamps, use UTC date directly
                    # This avoids timezone conversion issues where midnight UTC becomes previous day in ET
                    time_str = dt_utc.strftime('%Y-%m-%d')
            else:
                logger.warning("Missing timestamp in candle data")
                return None
            
            # Extract OHLCV data with proper NaN handling for volume
            volume_raw = candle_data.get('volume', 0)
            
            # Handle NaN volume (common for index symbols like SPX)
            if volume_raw == 'NaN' or volume_raw is None:
                volume = 0
            else:
                try:
                    volume = int(float(volume_raw))
                except (ValueError, TypeError):
                    volume = 0
            
            # Extract OHLC data and handle null values properly
            open_val = candle_data.get('open')
            high_val = candle_data.get('high')
            low_val = candle_data.get('low')
            close_val = candle_data.get('close')
            
            # Convert to float if not null, otherwise keep as None
            try:
                open_price = float(open_val) if open_val is not None else None
                high_price = float(high_val) if high_val is not None else None
                low_price = float(low_val) if low_val is not None else None
                close_price = float(close_val) if close_val is not None else None
            except (ValueError, TypeError):
                # If conversion fails, treat as null data
                open_price = high_price = low_price = close_price = None
            
            return {
                'time': time_str,
                'open': open_price,
                'high': high_price,
                'low': low_price,
                'close': close_price,
                'volume': volume
            }
            
        except Exception as e:
            logger.error(f"Error transforming DXLink candle: {e}")
            return None
    
    # === Account & Portfolio Methods ===
    
    async def get_positions(self) -> List[Position]:
        """Get all current positions."""
        raise NotImplementedError("TastyTrade get_positions not yet implemented")
    
    async def get_orders(self, status: str = "open") -> List[Order]:
        """Get orders with optional status filter."""
        raise NotImplementedError("TastyTrade get_orders not yet implemented")
    
    async def get_account(self) -> Optional[Account]:
        """Get account information including balance and buying power."""
        raise NotImplementedError("TastyTrade get_account not yet implemented")
    
    # === Order Management Methods ===
    
    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a trading order."""
        raise NotImplementedError("TastyTrade place_order not yet implemented")

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a multi-leg trading order."""
        raise NotImplementedError("TastyTrade place_multi_leg_order not yet implemented")
    
    async def cancel_order(self, order_id: str) -> bool:
        """Cancel an existing order."""
        raise NotImplementedError("TastyTrade cancel_order not yet implemented")
    
    # === Streaming Methods ===
    
    async def connect_streaming(self) -> bool:
        """Connect to DXLink streaming service."""
        try:
            # Get quote token for DXLink authentication
            if not await self._get_quote_token():
                logger.error("Failed to get quote token for streaming")
                return False
            
            # TODO: Implement DXLink WebSocket connection
            # This will require connecting to self._dxlink_url and using self._quote_token
            logger.info("TastyTrade streaming connection not yet implemented")
            self.is_connected = False
            return False
            
        except Exception as e:
            logger.error(f"Error connecting to TastyTrade streaming: {e}")
            return False
    
    async def disconnect_streaming(self) -> bool:
        """Disconnect from DXLink streaming service."""
        try:
            if self._stream_connection:
                # TODO: Implement DXLink WebSocket disconnection
                self._stream_connection = None
            
            self.is_connected = False
            self._connection_ready.clear()
            self._subscribed_symbols.clear()
            
            logger.info("TastyTrade streaming disconnected")
            return True
            
        except Exception as e:
            logger.error(f"Error disconnecting TastyTrade streaming: {e}")
            return False
    
    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """Subscribe to real-time data for symbols via DXLink."""
        logger.info(f"TastyTrade: subscribe_to_symbols called with {len(symbols)} symbols")
        
        # TODO: Implement DXLink symbol subscription
        # This will require DXLink protocol implementation
        logger.info("TastyTrade symbol subscription not yet implemented")
        return False
    
    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """Unsubscribe from real-time data for symbols via DXLink."""
        logger.info(f"TastyTrade: unsubscribe_from_symbols called with {len(symbols)} symbols")
        
        # TODO: Implement DXLink symbol unsubscription
        logger.info("TastyTrade symbol unsubscription not yet implemented")
        return False
    
    # === Utility Methods ===
    
    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol using TastyTrade format."""
        # TastyTrade uses OCC format similar to other providers
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])
    
    def _parse_option_symbol(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Parse TastyTrade option symbol to extract components."""
        try:
            # TastyTrade uses standard OCC format
            # Example: AAPL  220617P00150000
            # Format: ROOT(6) + YYMMDD(6) + C/P(1) + STRIKE(8)
            
            if len(symbol) < 21:
                return None
            
            # Extract components
            root = symbol[:6].strip()  # Remove padding spaces
            date_part = symbol[6:12]
            option_type = symbol[12]
            strike_part = symbol[13:21]
            
            # Parse expiry date
            year = 2000 + int(date_part[:2])
            month = int(date_part[2:4])
            day = int(date_part[4:6])
            expiry_date = f"{year}-{month:02d}-{day:02d}"
            
            # Parse strike price
            strike_price = float(strike_part) / 1000
            
            return {
                "underlying": root,
                "type": "call" if option_type == "C" else "put",
                "strike": strike_price,
                "expiry": expiry_date
            }
            
        except Exception as e:
            self._log_error(f"parse_option_symbol {symbol}", e)
            return None
    
    async def health_check(self) -> Dict[str, Any]:
        """Perform a health check on the TastyTrade provider."""
        try:
            session_valid = await self._ensure_valid_session()
            
            return {
                "provider": self.name,
                "connected": self.is_connected,
                "session_valid": session_valid,
                "session_expires_at": self._session_expires_at.isoformat() if self._session_expires_at else None,
                "subscribed_symbols": len(self._subscribed_symbols),
                "timestamp": datetime.now().isoformat()
            }
        except Exception as e:
            return {
                "provider": self.name,
                "connected": False,
                "session_valid": False,
                "error": str(e),
                "timestamp": datetime.now().isoformat()
            }
