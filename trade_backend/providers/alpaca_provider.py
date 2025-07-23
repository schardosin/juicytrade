import asyncio
import requests
import time
from typing import List, Dict, Optional, Any, Set, Tuple
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
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
)

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
        
        # Initialize symbol lookup cache
        self._symbol_cache = SymbolLookupCache()
        
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
        try:
            # Get full options chain first
            all_contracts = await self.get_options_chain(symbol, expiry)
            
            if not all_contracts:
                return []
            
            # If no underlying price provided, try to get current stock quote
            if underlying_price is None:
                stock_quote = await self.get_stock_quote(symbol)
                if stock_quote and stock_quote.bid and stock_quote.ask:
                    underlying_price = (stock_quote.bid + stock_quote.ask) / 2
                else:
                    # Fallback: use middle strike as approximation
                    strikes = [c.strike_price for c in all_contracts if c.strike_price]
                    if strikes:
                        underlying_price = sorted(strikes)[len(strikes) // 2]
                    else:
                        # Return all contracts if we can't determine ATM
                        return all_contracts
            
            # Sort contracts by distance from ATM
            def distance_from_atm(contract):
                return abs(contract.strike_price - underlying_price)
            
            # Separate calls and puts
            calls = [c for c in all_contracts if c.type.lower() == 'call']
            puts = [c for c in all_contracts if c.type.lower() == 'put']
            
            # Sort each by distance from ATM
            calls.sort(key=distance_from_atm)
            puts.sort(key=distance_from_atm)
            
            # Take the closest strikes for each type
            strikes_per_side = strike_count // 2
            selected_calls = calls[:strikes_per_side]
            selected_puts = puts[:strikes_per_side]
            
            # Combine and sort by strike price
            result = selected_calls + selected_puts
            result.sort(key=lambda x: x.strike_price)
            
            self._log_info(f"Basic options chain for {symbol} {expiry}: {len(result)} contracts around ATM ${underlying_price:.2f}")
            return result
            
        except Exception as e:
            self._log_error(f"get_options_chain_basic for {symbol} {expiry}", e)
            return []
    
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
        try:
            if strikes_only:
                # For strikes only, we can use a more efficient approach
                return await self.get_options_chain_basic(symbol, expiry, underlying_price, atm_range)
            
            # Get basic chain first
            contracts = await self.get_options_chain_basic(symbol, expiry, underlying_price, atm_range)
            
            if not contracts:
                return []
            
            # If Greeks are requested, fetch them in batch
            if include_greeks:
                option_symbols = [c.symbol for c in contracts if c.symbol]
                if option_symbols:
                    greeks_data = await self.get_options_greeks_batch(option_symbols)
                    
                    # Update contracts with Greeks data
                    for contract in contracts:
                        if contract.symbol in greeks_data:
                            greeks = greeks_data[contract.symbol]
                            contract.delta = greeks.get('delta')
                            contract.gamma = greeks.get('gamma')
                            contract.theta = greeks.get('theta')
                            contract.vega = greeks.get('vega')
                            contract.implied_volatility = greeks.get('implied_volatility')
            
            self._log_info(f"Smart options chain for {symbol} {expiry}: {len(contracts)} contracts, Greeks: {include_greeks}")
            return contracts
            
        except Exception as e:
            self._log_error(f"get_options_chain_smart for {symbol} {expiry}", e)
            return []
    
    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """
        Get Greeks for multiple option symbols in batch.
        
        Args:
            option_symbols: List of option symbols
            
        Returns:
            Dictionary mapping option symbols to Greeks data
        """
        try:
            if not option_symbols:
                return {}
            
            # Alpaca doesn't have a dedicated Greeks endpoint, so we'll use the snapshots endpoint
            # which sometimes includes Greeks data
            url = f"{self.data_url}/v1beta1/options/snapshots"
            api_key, api_secret = self._get_api_credentials()
            
            # Process in batches of 100 (API limit)
            batch_size = 100
            all_greeks = {}
            
            for i in range(0, len(option_symbols), batch_size):
                batch_symbols = option_symbols[i:i + batch_size]
                symbols_param = ",".join(batch_symbols)
                
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
                    snapshots = data.get("snapshots", {})
                    
                    for symbol, snapshot in snapshots.items():
                        if snapshot:
                            # Extract Greeks if available
                            greeks_data = {}
                            
                            # Alpaca snapshot format may include Greeks
                            if 'greeks' in snapshot:
                                greeks = snapshot['greeks']
                                greeks_data = {
                                    'delta': float(greeks.get('delta', 0)) if greeks.get('delta') else None,
                                    'gamma': float(greeks.get('gamma', 0)) if greeks.get('gamma') else None,
                                    'theta': float(greeks.get('theta', 0)) if greeks.get('theta') else None,
                                    'vega': float(greeks.get('vega', 0)) if greeks.get('vega') else None,
                                    'implied_volatility': float(greeks.get('implied_volatility', 0)) if greeks.get('implied_volatility') else None
                                }
                            else:
                                # If no Greeks in snapshot, return empty dict for this symbol
                                greeks_data = {
                                    'delta': None,
                                    'gamma': None,
                                    'theta': None,
                                    'vega': None,
                                    'implied_volatility': None
                                }
                            
                            all_greeks[symbol] = greeks_data
                else:
                    self._log_error(f"get_options_greeks_batch API call", 
                                  Exception(f"HTTP {response.status_code}: {response.text}"))
            
            self._log_info(f"Retrieved Greeks for {len(all_greeks)} option symbols")
            return all_greeks
            
        except Exception as e:
            self._log_error(f"get_options_greeks_batch for {len(option_symbols)} symbols", e)
            return {}
    
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

    async def get_positions_enhanced(self) -> Dict[str, Any]:
        """Get enhanced positions with simplified static data structure."""
        try:
            logger.info("🔍 Getting enhanced positions with static broker data...")
            
            # 1. Get current positions
            current_positions = await self.get_positions()
            if not current_positions:
                logger.info("No current positions found")
                return {"enhanced": True, "symbol_groups": []}
            
            # 2. Extract symbols from positions for efficient order lookup
            position_symbols = [pos.symbol for pos in current_positions]
            logger.info(f"📊 Extracting orders for {len(position_symbols)} position symbols")
            
            # 3. Get closed orders for position symbols (much more efficient than full history)
            closed_orders = await self.get_orders_by_symbols(position_symbols, status="closed")
            
            # 4. Get all current orders for additional context
            current_orders = await self.get_orders(status="all")
            
            logger.info(f"📊 Retrieved {len(closed_orders)} closed orders for position symbols and {len(current_orders)} current orders")
            
            # 5. Create simplified hierarchical grouping (Symbol -> Strategies -> Legs)
            symbol_groups = self._create_simplified_hierarchical_groups(
                current_positions, closed_orders, current_orders
            )
            
            logger.info(f"✅ Created {len(symbol_groups)} symbol groups with enhanced order data")
            return {"enhanced": True, "symbol_groups": symbol_groups}
            
        except Exception as e:
            self._log_error("get_positions_enhanced", e)
            return {"enhanced": True, "symbol_groups": []}
    
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
    
    async def get_account(self) -> Optional[Account]:
        """Get account information including balance and buying power."""
        try:
            # Use the trading client to get account information
            account_info = self.trading_client.get_account()
            
            if account_info:
                return self._transform_account(account_info)
            return None
        except Exception as e:
            self._log_error("get_account", e)
            return None
    
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
        """Subscribe to real-time data for symbols - handles mixed stock/option symbols transparently."""
        try:
            if not self.is_connected:
                await self.connect_streaming()
            
            # Clear previous subscriptions and replace with new ones
            self._subscribed_symbols = set(symbols)
            
            # Separate symbols by type for internal routing
            stock_symbols = [s for s in symbols if not self._is_option_symbol(s)]
            option_symbols = [s for s in symbols if self._is_option_symbol(s)]
            
            # Subscribe to stock quotes using unified handler
            if stock_symbols and self.stock_stream:
                self._log_info(f"Subscribing to {len(stock_symbols)} stock symbols via unified handler")
                self.stock_stream.subscribe_quotes(self._unified_quote_handler, *stock_symbols)

            # Subscribe to option quotes using unified handler
            if option_symbols and self.option_stream:
                self._log_info(f"Subscribing to {len(option_symbols)} option symbols via unified handler")
                self.option_stream.subscribe_quotes(self._unified_quote_handler, *option_symbols)

            self._log_info(f"✅ Alpaca: Subscribed to {len(symbols)} symbols ({len(stock_symbols)} stocks, {len(option_symbols)} options)")
            return True
        except Exception as e:
            self._log_error(f"subscribe_to_symbols {symbols}", e)
            return False
    
    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """
        Unsubscribe from real-time data for symbols.
        
        Note: Alpaca doesn't support true unsubscribing from individual symbols,
        so we track what should be unsubscribed and manage subscriptions internally.
        """
        try:
            logger.info(f"🗑️ Alpaca: Marking {len(symbols)} symbols for unsubscription: {symbols}")
            
            # Remove symbols from our tracking set
            for symbol in symbols:
                self._subscribed_symbols.discard(symbol)
            
            logger.info(f"📋 Alpaca: Remaining subscribed symbols: {list(self._subscribed_symbols)}")
            
            # Instead of restarting streams (which causes connection limits),
            # we'll just track the unsubscribed symbols internally.
            # New subscriptions will only include the remaining symbols.
            logger.info(f"✅ Alpaca: Symbols marked for unsubscription. Next subscription will use remaining {len(self._subscribed_symbols)} symbols")
            
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
                asset_class=raw_position.asset_class.value if raw_position.asset_class else "unknown",
                lastday_price=float(raw_position.lastday_price) if hasattr(raw_position, 'lastday_price') and raw_position.lastday_price else None
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
    
    def _transform_account(self, raw_account) -> Optional[Account]:
        """Transform Alpaca account to our standard model."""
        try:
            return Account(
                account_id=str(raw_account.id),
                account_number=raw_account.account_number,
                status=raw_account.status.value if hasattr(raw_account.status, 'value') else str(raw_account.status),
                currency=raw_account.currency or "USD",
                buying_power=float(raw_account.buying_power) if raw_account.buying_power else None,
                cash=float(raw_account.cash) if raw_account.cash else None,
                portfolio_value=float(raw_account.portfolio_value) if raw_account.portfolio_value else None,
                equity=float(raw_account.equity) if raw_account.equity else None,
                day_trading_buying_power=float(raw_account.daytrading_buying_power) if raw_account.daytrading_buying_power else None,
                regt_buying_power=float(raw_account.regt_buying_power) if raw_account.regt_buying_power else None,
                options_buying_power=float(raw_account.options_buying_power) if raw_account.options_buying_power else None,
                pattern_day_trader=raw_account.pattern_day_trader,
                trading_blocked=raw_account.trading_blocked,
                transfers_blocked=raw_account.transfers_blocked,
                account_blocked=raw_account.account_blocked,
                created_at=raw_account.created_at.isoformat() if raw_account.created_at else None,
                multiplier=raw_account.multiplier,
                long_market_value=float(raw_account.long_market_value) if raw_account.long_market_value else None,
                short_market_value=float(raw_account.short_market_value) if raw_account.short_market_value else None,
                initial_margin=float(raw_account.initial_margin) if raw_account.initial_margin else None,
                maintenance_margin=float(raw_account.maintenance_margin) if raw_account.maintenance_margin else None,
                daytrade_count=raw_account.daytrade_count,
                options_approved_level=raw_account.options_approved_level,
                options_trading_level=raw_account.options_trading_level
            )
        except Exception as e:
            self._log_error("transform_account", e)
            return None
    
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
    
    async def _unified_quote_handler(self, data):
        """Unified handler for both stock and option quotes - provides consistent output."""
        try:
            # Determine if this is a stock or option symbol
            is_option = self._is_option_symbol(data.symbol)
            
            # Create standardized MarketData regardless of source stream
            market_data = MarketData(
                symbol=data.symbol,
                data_type="quote",  # Unified type for both stocks and options
                timestamp=datetime.now().isoformat(),
                data={
                    "ask": data.ask_price,
                    "bid": data.bid_price,
                    "last": getattr(data, 'last_price', None)
                }
            )
            
            # Put into unified queue - same format as Tradier
            await self._streaming_queue.put(market_data)
            
            # Log only for debugging (less verbose for options)
            if not is_option:
                self._log_info(f"📡 Alpaca unified handler: {data.symbol} -> bid: {data.bid_price}, ask: {data.ask_price}")
                
        except Exception as e:
            self._log_error("unified_quote_handler", e)
    
    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query using Alpaca assets endpoint with smart caching."""
        return await self._symbol_cache.search(query, self._api_lookup_symbols)
    
    async def _api_lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Make actual API call to Alpaca for symbol lookup."""
        try:
            url = f"{self._get_base_url()}/v2/assets"
            api_key, api_secret = self._get_api_credentials()
            
            # Alpaca assets endpoint parameters
            params = {
                "status": "active",  # Only active assets
                "asset_class": "us_equity",  # Focus on US equities
                "attributes": ""  # Can be used for additional filtering
            }
            
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                data = response.json()
                
                # Filter results based on query
                query_upper = query.upper()
                filtered_assets = []
                
                for asset in data:
                    symbol = asset.get("symbol", "").upper()
                    name = asset.get("name", "").upper()
                    
                    # Check if query matches symbol or name
                    if (query_upper in symbol or query_upper in name):
                        filtered_assets.append(asset)
                
                # Sort results by relevance
                def sort_key(asset):
                    symbol = asset.get("symbol", "").upper()
                    name = asset.get("name", "").upper()
                    
                    # Exact symbol match gets highest priority
                    if symbol == query_upper:
                        return (0, symbol)
                    
                    # Symbol starts with query gets second priority
                    if symbol.startswith(query_upper):
                        return (1, len(symbol), symbol)
                    
                    # Symbol contains query gets third priority
                    if query_upper in symbol:
                        return (2, symbol.index(query_upper), len(symbol), symbol)
                    
                    # Name contains query gets fourth priority
                    if query_upper in name:
                        return (3, name.index(query_upper), len(name), symbol)
                    
                    # Everything else
                    return (4, symbol)
                
                # Sort the filtered assets
                filtered_assets.sort(key=sort_key)
                
                # Transform ALL results to our standard model (cache everything)
                results = []
                for asset in filtered_assets:  # Transform ALL filtered results
                    transformed = self._transform_asset_to_symbol_result(asset)
                    if transformed:
                        results.append(transformed)
                
                logger.info(f"Found {len(results)} total symbols for query '{query}' from Alpaca assets, caching all results")
                
                # Return ALL results for caching, limiting will be done in the cache search method
                return results
            else:
                self._log_error(f"lookup_symbols API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return []
                
        except Exception as e:
            self._log_error(f"lookup_symbols for query '{query}'", e)
            return []
    
    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """Get historical OHLCV bars for charting using Alpaca StockHistoricalDataClient."""
        try:
            from alpaca.data.requests import StockBarsRequest
            from alpaca.data.timeframe import TimeFrame, TimeFrameUnit
            from datetime import datetime, timedelta
            from zoneinfo import ZoneInfo
            
            # Map our timeframe to Alpaca TimeFrame objects using amount and unit
            tf_map = {
                '1m': TimeFrame(amount=1, unit=TimeFrameUnit.Minute),
                '5m': TimeFrame(amount=5, unit=TimeFrameUnit.Minute),
                '15m': TimeFrame(amount=15, unit=TimeFrameUnit.Minute),
                '30m': TimeFrame(amount=30, unit=TimeFrameUnit.Minute),
                '1h': TimeFrame(amount=1, unit=TimeFrameUnit.Hour),
                '4h': TimeFrame(amount=4, unit=TimeFrameUnit.Hour),
                'D': TimeFrame(amount=1, unit=TimeFrameUnit.Day),
                'W': TimeFrame(amount=1, unit=TimeFrameUnit.Week),
                'M': TimeFrame(amount=1, unit=TimeFrameUnit.Month)
            }
            
            alpaca_timeframe = tf_map.get(timeframe, TimeFrame(amount=1, unit=TimeFrameUnit.Day))
            
            # Use timezone-aware datetime for NY timezone
            now = datetime.now(ZoneInfo("America/New_York"))
            
            # Set default start date if not provided
            if not start_date:
                if timeframe in ['1m', '5m', '15m', '30m', '1h', '4h']:
                    # For intraday, get last 5 days
                    start_dt = now - timedelta(days=5)
                else:
                    # For daily+, get last year
                    start_dt = now - timedelta(days=365)
            else:
                # Convert string date to timezone-aware datetime
                start_dt = datetime.strptime(start_date, '%Y-%m-%d').replace(tzinfo=ZoneInfo("America/New_York"))
            
            # Handle end date - only set if explicitly provided
            end_dt = None
            if end_date:
                end_dt = datetime.strptime(end_date, '%Y-%m-%d').replace(tzinfo=ZoneInfo("America/New_York"))
            
            # Create the request - only include end if provided
            request_params = {
                "symbol_or_symbols": [symbol],
                "timeframe": alpaca_timeframe,
                "start": start_dt,
                "limit": limit,
                "adjustment": "raw"
            }
            
            if end_dt:
                request_params["end"] = end_dt
            
            request = StockBarsRequest(**request_params)
            
            self._log_info(f"Requesting Alpaca bars for {symbol}, timeframe: {timeframe}, start: {start_dt}, end: {end_dt}, limit: {limit}")
            
            # Get the bars
            bars_response = self.historical_client.get_stock_bars(request)
            
            # Transform to Lightweight Charts format
            result = []
            
            # Access the data from the bars_response object
            if hasattr(bars_response, 'data') and symbol in bars_response.data:
                bars_list = bars_response.data[symbol]
                self._log_info(f"Received {len(bars_list)} bars from Alpaca for {symbol}")
                
                for bar in bars_list:
                    # Format time based on timeframe
                    if timeframe in ['1m', '5m', '15m', '30m', '1h', '4h']:
                        # Intraday - include time
                        time_str = bar.timestamp.strftime('%Y-%m-%d %H:%M')
                    else:
                        # Daily+ - date only
                        time_str = bar.timestamp.strftime('%Y-%m-%d')
                    
                    result.append({
                        'time': time_str,
                        'open': float(bar.open),
                        'high': float(bar.high),
                        'low': float(bar.low),
                        'close': float(bar.close),
                        'volume': int(bar.volume) if bar.volume else 0
                    })
            elif hasattr(bars_response, symbol):
                # Alternative access pattern
                bars_list = getattr(bars_response, symbol)
                self._log_info(f"Received {len(bars_list)} bars from Alpaca for {symbol} (alt access)")
                
                for bar in bars_list:
                    # Format time based on timeframe
                    if timeframe in ['1m', '5m', '15m', '30m', '1h', '4h']:
                        # Intraday - include time
                        time_str = bar.timestamp.strftime('%Y-%m-%d %H:%M')
                    else:
                        # Daily+ - date only
                        time_str = bar.timestamp.strftime('%Y-%m-%d')
                    
                    result.append({
                        'time': time_str,
                        'open': float(bar.open),
                        'high': float(bar.high),
                        'low': float(bar.low),
                        'close': float(bar.close),
                        'volume': int(bar.volume) if bar.volume else 0
                    })
            else:
                self._log_info(f"No bars found for symbol {symbol} in Alpaca response")
                self._log_info(f"Response type: {type(bars_response)}")
                self._log_info(f"Response attributes: {dir(bars_response)}")
            
            self._log_info(f"Transformed {len(result)} bars for {symbol} ({timeframe})")
            return result
            
        except Exception as e:
            self._log_error(f"get_historical_bars for {symbol} {timeframe}", e)
            import traceback
            self._log_error(f"Full traceback", Exception(traceback.format_exc()))
            return []

    def _transform_asset_to_symbol_result(self, asset: Dict[str, Any]) -> Optional[SymbolSearchResult]:
        """Transform Alpaca asset to our standard SymbolSearchResult model."""
        try:
            # Map Alpaca asset class to our type system
            asset_class = asset.get("class", "")
            if asset_class == "us_equity":
                symbol_type = "stock"
            elif asset_class == "us_option":
                symbol_type = "option"
            elif asset_class == "crypto":
                symbol_type = "crypto"
            else:
                symbol_type = asset_class.lower() if asset_class else "unknown"
            
            return SymbolSearchResult(
                symbol=asset.get("symbol", ""),
                description=asset.get("name", ""),
                exchange=asset.get("exchange", ""),
                type=symbol_type
            )
        except Exception as e:
            self._log_error("transform_asset_to_symbol_result", e)
            return None

    # === Stream Management Helper Methods ===
    
    async def _restart_streams_with_symbols(self, symbols: List[str]) -> bool:
        """Restart streaming connections with only the specified symbols."""
        try:
            logger.info(f"🔄 Alpaca: Restarting streams with {len(symbols)} symbols")
            
            # Stop current streams
            await self._stop_all_streams()
            
            # Wait a moment for cleanup
            await asyncio.sleep(0.5)
            
            # Reconnect streaming
            if not await self.connect_streaming():
                logger.error("Failed to reconnect streaming during restart")
                return False
            
            # Subscribe to the new symbol list
            return await self.subscribe_to_symbols(symbols)
            
        except Exception as e:
            self._log_error("_restart_streams_with_symbols", e)
            return False
    
    async def _stop_all_streams(self) -> bool:
        """Stop all streaming connections."""
        try:
            logger.info("🛑 Alpaca: Stopping all streams")
            
            if self.option_stream:
                try:
                    await self.option_stream.close()
                except Exception as e:
                    logger.warning(f"Error closing option stream: {e}")
                self.option_stream = None
            
            if self.stock_stream:
                try:
                    await self.stock_stream.close()
                except Exception as e:
                    logger.warning(f"Error closing stock stream: {e}")
                self.stock_stream = None
            
            if self.trading_stream:
                try:
                    await self.trading_stream.close()
                except Exception as e:
                    logger.warning(f"Error closing trading stream: {e}")
                self.trading_stream = None
            
            self.is_connected = False
            logger.info("✅ Alpaca: All streams stopped")
            return True
            
        except Exception as e:
            self._log_error("_stop_all_streams", e)
            return False

    # === Enhanced Positions Helper Methods ===
    
    async def _get_order_history(self, days_back: int = 30) -> List:
        """Get order history from Alpaca - simplified for now."""
        try:
            # For now, return empty list since Alpaca doesn't have a direct history API
            # In a full implementation, we would use the orders API with date filtering
            logger.info(f"📊 Alpaca order history not implemented - returning empty list")
            return []
        except Exception as e:
            self._log_error("_get_order_history", e)
            return []

    async def get_orders_by_symbols(self, symbols: List[str], status: str = "closed") -> List[Order]:
        """Get orders filtered by symbols and status - much more efficient than full history."""
        try:
            if not symbols:
                logger.info("No symbols provided for order lookup")
                return []
            
            logger.info(f"🔍 Getting {status} orders for {len(symbols)} symbols: {symbols}")
            
            # Use Alpaca's orders endpoint with symbol filtering
            url = f"{self._get_base_url()}/v2/orders"
            api_key, api_secret = self._get_api_credentials()
            
            # Join symbols with comma for the API call
            symbols_param = ",".join(symbols)
            
            params = {
                "status": status,
                "symbols": symbols_param,
                "limit": 1000,  # Get up to 1000 orders
                "nested": "true"  # Include nested order details
            }
            
            headers = {
                "APCA-API-KEY-ID": api_key,
                "APCA-API-SECRET-KEY": api_secret,
                "accept": "application/json"
            }
            
            response = requests.get(url, headers=headers, params=params)
            
            if response.status_code == 200:
                orders_data = response.json()
                logger.info(f"📊 Retrieved {len(orders_data)} orders for symbols")
                
                # Transform to our Order model
                result = []
                for order_data in orders_data:
                    try:
                        # Create a mock Alpaca order object for transformation
                        transformed_order = self._transform_order_from_dict(order_data)
                        if transformed_order:
                            result.append(transformed_order)
                    except Exception as e:
                        self._log_error(f"transform_order_from_dict for order {order_data.get('id')}", e)
                        continue
                
                logger.info(f"✅ Successfully transformed {len(result)} orders")
                return result
            else:
                self._log_error(f"get_orders_by_symbols API call", 
                              Exception(f"HTTP {response.status_code}: {response.text}"))
                return []
                
        except Exception as e:
            self._log_error(f"get_orders_by_symbols for {len(symbols)} symbols", e)
            return []

    def _transform_order_from_dict(self, order_data: Dict[str, Any]) -> Optional[Order]:
        """Transform Alpaca order dictionary to our standard Order model."""
        try:
            # Handle legs for multi-leg orders
            legs = None
            if order_data.get("legs"):
                legs = []
                for leg_data in order_data["legs"]:
                    legs.append({
                        "symbol": leg_data.get("symbol"),
                        "side": leg_data.get("side"),
                        "qty": float(leg_data.get("qty", 0))
                    })
            
            return Order(
                id=str(order_data.get("id", "")),
                symbol=order_data.get("symbol", "Multi-leg" if legs else ""),
                asset_class=order_data.get("asset_class", "unknown"),
                side=order_data.get("side", ""),
                order_type=order_data.get("order_type", ""),
                qty=float(order_data.get("qty", 0)),
                filled_qty=float(order_data.get("filled_qty", 0)),
                limit_price=float(order_data.get("limit_price")) if order_data.get("limit_price") else None,
                stop_price=float(order_data.get("stop_price")) if order_data.get("stop_price") else None,
                avg_fill_price=float(order_data.get("filled_avg_price")) if order_data.get("filled_avg_price") else None,
                status=order_data.get("status", ""),
                time_in_force=order_data.get("time_in_force", ""),
                submitted_at=order_data.get("submitted_at", ""),
                filled_at=order_data.get("filled_at"),
                legs=legs
            )
        except Exception as e:
            self._log_error("_transform_order_from_dict", e)
            return None
    
    def _create_simplified_hierarchical_groups(self, positions: List[Position], 
                                             history: List, 
                                             current_orders: List[Order]) -> List[Dict[str, Any]]:
        """Create simplified hierarchical grouping with only static broker data."""
        try:
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
                    
                    # Create legs with static broker data (Alpaca provides avg_entry_price directly)
                    legs = []
                    for pos in expiry_positions:
                        legs.append({
                            "symbol": pos.symbol,
                            "qty": pos.qty,
                            "avg_entry_price": pos.avg_entry_price,  # Alpaca provides this directly
                            "cost_basis": pos.cost_basis,
                            "asset_class": pos.asset_class,
                            "current_price": pos.current_price,
                            "lastday_price": pos.lastday_price,  # Include lastday_price for daily P/L calculation
                            "bid": getattr(pos, 'bid', None),
                            "ask": getattr(pos, 'ask', None)
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

