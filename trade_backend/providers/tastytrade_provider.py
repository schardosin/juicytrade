import asyncio
import requests
import json
import logging
import time
import httpx
from typing import List, Dict, Optional, Any, Tuple
from datetime import datetime, timedelta

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
)
from ..config import settings

logger = logging.getLogger(__name__)

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
        """Get historical OHLCV bars for charting."""
        raise NotImplementedError("TastyTrade get_historical_bars not yet implemented")
    
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
