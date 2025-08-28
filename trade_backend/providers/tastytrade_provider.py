import asyncio
import requests
import json
import logging
import time
import httpx
import websockets
import websockets.exceptions
from typing import List, Dict, Optional, Any, Tuple, Set
from asyncio import Future
from datetime import datetime, timedelta

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
)
from ..config import settings
from ..streaming_health_manager import streaming_health_manager, ConnectionState, TimeoutWrapper
from ..services.ivx_calculator import calculate_expected_move, find_atm_strike

logger = logging.getLogger(__name__)

# Configure httpx logging to reduce verbosity
logging.getLogger("httpx").setLevel(logging.WARNING)

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


class TastyTradeProvider(BaseProvider):
    """
    TastyTrade provider implementation.
    
    Uses session-based authentication with 24-hour session tokens.
    For streaming, uses DXLink with separate quote tokens.
    """
    
    def __init__(self, username: str, password: str, account_id: str, base_url: str):
        super().__init__("TastyTrade")
        self.username = username
        self.password = password
        self.account_id = account_id
        self.base_url = base_url
        self._session_token = None
        self._session_expires_at = None
        self._quote_token = None
        self._quote_token_expires_at = None
        self._dxlink_url = None
        self._stream_connection = None
        self._connection_ready = asyncio.Event()
        
        # Streaming infrastructure
        self._streaming_queue = None
        self._streaming_task = None
        self._shutdown_event = asyncio.Event()
        self._recv_lock = asyncio.Lock()
        self._subscribed_symbols = set()
        
        # Request/response mechanism for streaming
        self._greeks_requests: Dict[str, Future] = {}
        self._quote_requests: Dict[str, Future] = {}
        
        # Health monitoring
        self._connection_id = f"tastytrade_{self.account_id}"
        streaming_health_manager.register_provider("tastytrade", self)
        self._connection_metrics = streaming_health_manager.register_connection(
            self._connection_id, "tastytrade", "websocket"
        )
        
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
        """Get the latest stock quote for a symbol via streaming."""
        quote_data = await self.get_streaming_quote(symbol)
        if not quote_data:
            return None

        return StockQuote(
            symbol=symbol,
            bid=quote_data.get('bid'),
            ask=quote_data.get('ask'),
            bid_size=quote_data.get('bid_size'),
            ask_size=quote_data.get('ask_size'),
            last_price=None, # Not in quote data
            timestamp=datetime.now().isoformat()
        )

    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        """Get stock quotes for multiple symbols."""
        tasks = [self.get_stock_quote(s) for s in symbols]
        results = await asyncio.gather(*tasks)
        return {s.symbol: s for s in results if s}

    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """Get available expiration dates for options on a symbol with universal enhanced structure."""
        try:
            # UI to Tasty mapping
            type_mapping = {
                "regular": "monthly",
                "end-of-month": "eom",
                "weekly": "weekly",
                "quarterly": "quarterly"
            }

            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            url = f"{self.base_url}/option-chains/{symbol}"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                items = data.get("data", {}).get("items", [])
                
                # Always return enhanced structure for all symbols
                enhanced_dates = []
                # Group by expiration date and root symbol
                date_symbol_map = {}
                
                for item in items:
                    exp_date = item.get("expiration-date")
                    root_symbol = item.get("root-symbol", symbol)
                    exp_type = item.get("expiration-type", "Standard")
                    
                    if exp_date:
                        key = f"{exp_date}-{root_symbol}"
                        if key not in date_symbol_map:
                            # Determine type based on expiration type from TastyTrade
                            symbol_type = type_mapping.get(exp_type.lower(), exp_type)
                            
                            date_symbol_map[key] = {
                                "date": exp_date,
                                "symbol": root_symbol,
                                "type": symbol_type
                            }
                
                # Convert to list and sort by date, then by symbol
                enhanced_dates = list(date_symbol_map.values())
                enhanced_dates.sort(key=lambda x: (x["date"], x["symbol"]))
                
                return enhanced_dates
                
        except Exception as e:
            self._log_error(f"get_expiration_dates for {symbol}", e)
            return []
    
    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20, type: str = None, underlying_symbol: str = None, include_greeks: bool = False) -> List[OptionContract]:
        """Fast loading - basic options data without Greeks, ATM-focused by strike count."""
        try:
            # Tasty to UI mapping
            reverse_type_mapping = {
                "monthly": "regular",
                "eom": "end-of-month",
                "weekly": "weekly",
                "quarterly": "quarterly"
            }

            # Determine the symbol to use for the API call. Use underlying_symbol if provided, otherwise use symbol.
            api_symbol = underlying_symbol if underlying_symbol else symbol

            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")

            # 1. Get basic option contracts
            url = f"{self.base_url}/option-chains/{api_symbol}"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json, text/plain, */*",
                "User-Agent": "juicytrade/1.0"
            }

            params = {}
            if symbol:
                params['root-symbol'] = symbol

            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers, params=params)
                response.raise_for_status()

                data = response.json()
                items = data.get("data", {}).get("items", [])

                # Determine type filter, for tastytrade monthly is called regular
                typeFilter = None
                if type:
                    typeFilter = reverse_type_mapping.get(type.lower())

                # Filter contracts
                filtered_contracts_raw = []
                option_symbols = []
                for item in items:
                    # Filter by expiry if specified
                    if expiry and item.get("expiration-date") != expiry:
                        continue

                    # The 'type' parameter from the frontend refers to 'weekly' or 'monthly',
                    # which is 'expiration-type' in TastyTrade's response.
                    if type and item.get("expiration-type", "").lower() != typeFilter:
                        continue

                    filtered_contracts_raw.append(item)
                    option_symbols.append(item.get("symbol", ""))

                if not filtered_contracts_raw:
                    return []
                
                # 2. Transform contracts with pricing data
                contracts = []
                for item in filtered_contracts_raw:
                    transformed_contract = self._transform_tastytrade_option_contract(item, None)
                    if transformed_contract:
                        contracts.append(transformed_contract)

            if not contracts:
                return []

            # Explicitly filter by root_symbol as the API might not be precise enough
            contracts = [
                contract for contract in contracts if contract.root_symbol.upper() == symbol.upper()
            ]
            logger.info(f"TastyTrade: Filtered to {len(contracts)} contracts for root symbol {symbol}")

            if not contracts:
                return []

            # Get underlying price if not provided
            if underlying_price is None:
                try:
                    # Use api_symbol to get quote for the underlying
                    quote = await self.get_stock_quote(api_symbol)
                    underlying_price = (quote.bid + quote.ask) / 2 if quote and quote.bid and quote.ask else None
                except:
                    underlying_price = None

            # Smart filtering - focus on ATM strikes by count
            if underlying_price and contracts:
                # Get all unique strikes and sort them
                strikes = sorted(list(set(contract.strike_price for contract in contracts)))

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
                filtered_contracts = [
                    contract for contract in contracts
                    if contract.strike_price in selected_strikes
                ]

                contracts = filtered_contracts

            # Transform to basic format (remove Greeks for faster loading)
            result = []
            for contract in contracts:
                basic_contract = self._transform_to_basic_contract(contract)
                if basic_contract:
                    result.append(basic_contract)

            logger.info(f"TastyTrade: Returning {len(result)} basic options for {symbol} {expiry}")
            return result

        except Exception as e:
            self._log_error(f"get_options_chain_basic for {symbol} {expiry}", e)
            return []
    
    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """Get Greeks for multiple option symbols in batch."""
        try:
            # TastyTrade doesn't have a dedicated Greeks endpoint, so we need to get full option data
            # Group symbols by underlying and expiration for efficient API calls
            symbol_groups = {}
            
            for option_symbol in option_symbols:
                # Convert standard OCC format to TastyTrade format for parsing
                tastytrade_symbol = self.convert_symbol_to_provider_format(option_symbol)
                parsed = self._parse_option_symbol(tastytrade_symbol)
                if parsed:
                    key = f"{parsed['underlying']}_{parsed['expiry']}"
                    if key not in symbol_groups:
                        symbol_groups[key] = {
                            'underlying': parsed['underlying'],
                            'expiry': parsed['expiry'],
                            'symbols': []
                        }
                    # Store the original standard OCC symbol for the response
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
                        # contract.symbol is already in standard OCC format (converted by the provider)
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
    
    async def get_next_market_date(self) -> str:
        """Get the next trading date."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            # TastyTrade doesn't have a dedicated market calendar endpoint
            # We'll use a simple approach: check if today is a weekday and not a holiday
            from datetime import datetime, timedelta
            
            today = datetime.now().date()
            next_date = today + timedelta(days=1)
            
            # Skip weekends
            while next_date.weekday() >= 5:  # 5 = Saturday, 6 = Sunday
                next_date += timedelta(days=1)
            
            return next_date.strftime('%Y-%m-%d')
            
        except Exception as e:
            self._log_error("get_next_market_date", e)
            # Fallback: return next weekday
            from datetime import datetime, timedelta
            today = datetime.now().date()
            next_date = today + timedelta(days=1)
            while next_date.weekday() >= 5:
                next_date += timedelta(days=1)
            return next_date.strftime('%Y-%m-%d')
    
    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query using TastyTrade symbol search API with smart caching."""
        return await self._api_lookup_symbols(query)
    
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
                        instrument_type = item.get("instrument-type", "")
                        ui_type = instrument_type
                        if instrument_type == "Equity":
                            ui_type = "Stock"
                        elif instrument_type == "Index":
                            ui_type = "Index"

                        result = SymbolSearchResult(
                            symbol=item.get("symbol", ""),
                            description=item.get("description", ""),
                            exchange=item.get("listed-market", ""),
                            type=ui_type
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
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            url = f"{self.base_url}/accounts/{self.account_id}/positions"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                items = data.get("data", {}).get("items", [])
                
                # Transform TastyTrade positions to our standard model
                positions = []
                for item in items:
                    position = self._transform_tastytrade_position(item)
                    if position:
                        positions.append(position)
                
                return positions
                
        except Exception as e:
            self._log_error("get_positions", e)
            return []
    
    async def get_orders(self, status: str = "open") -> List[Order]:
        """Get orders with optional status filter."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            # Use different endpoints based on status
            if status == "open":
                # Use live orders endpoint for open orders
                url = f"{self.base_url}/accounts/{self.account_id}/orders/live"
            else:
                # Use all orders endpoint for other statuses
                url = f"{self.base_url}/accounts/{self.account_id}/orders"
            
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                items = data.get("data", {}).get("items", [])
                
                # Transform TastyTrade orders to our standard model
                orders = []
                for item in items:
                    order = self._transform_tastytrade_order(item)
                    if order:
                        # Apply additional status filtering for non-live endpoints
                        if status != "open" and status != "all":
                            order_status = order.status.lower() if order.status else ""
                            if status == "filled" and order_status != "filled":
                                continue
                            elif status == "cancelled" and order_status != "cancelled":
                                continue
                        
                        orders.append(order)
                
                return orders
                
        except Exception as e:
            self._log_error(f"get_orders with status {status}", e)
            return []
    
    async def get_account(self) -> Optional[Account]:
        """Get account information including balance and buying power."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            url = f"{self.base_url}/accounts/{self.account_id}/balances"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.get(url, headers=headers)
                response.raise_for_status()
                
                data = response.json()
                account_data = data.get("data", {})
                
                # Transform TastyTrade account to our standard model
                return self._transform_tastytrade_account(account_data)
                
        except Exception as e:
            self._log_error("get_account", e)
            return None
    
    # === Order Management Methods ===
    
    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a trading order."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            # Transform our standard order format to TastyTrade format
            tastytrade_order = self._transform_to_tastytrade_order(order_data)
            
            # Log the order being sent for debugging in JSON format
            logger.info(f"TastyTrade: Sending order payload: {json.dumps(tastytrade_order, indent=2)}")
            
            url = f"{self.base_url}/accounts/{self.account_id}/orders"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "Content-Type": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.post(url, headers=headers, json=tastytrade_order)
                
                # Log response details for debugging
                logger.info(f"TastyTrade: Order response status: {response.status_code}")
                if response.status_code != 200:
                    logger.error(f"TastyTrade: Order response body: {response.text}")
                
                # Handle 422 responses specially - they contain error details in the body
                if response.status_code == 422:
                    try:
                        response_data = response.json()
                        error_data = response_data.get("error", {})
                        
                        if error_data:
                            error_messages = []
                            
                            # Extract only the detailed error messages from errors array
                            errors_array = error_data.get("errors", [])
                            for error in errors_array:
                                error_message = error.get("message", "")
                                if error_message:
                                    error_messages.append(error_message)
                            
                            error_msg = "; ".join(error_messages) if error_messages else "Order validation failed"
                            raise Exception(f"Order validation failed: {error_msg}")
                    except json.JSONDecodeError:
                        logger.error("Failed to parse 422 response JSON")
                        raise Exception("Order validation failed - unable to parse error details")
                
                response.raise_for_status()
                
                data = response.json()
                # TastyTrade nests the actual order data under data.order
                order_data = data.get("data", {}).get("order", {})
                
                # Transform TastyTrade order response to our standard model
                return self._transform_tastytrade_order(order_data)
                
        except Exception as e:
            self._log_error("place_order", e)
            raise

    async def preview_order(self, order_data: Dict[str, Any]) -> Dict[str, Any]:
        """Preview a multi-leg trading order using the dry-run endpoint."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")

            tastytrade_order = self._transform_to_tastytrade_multi_leg_order(order_data)
            
            # Log the payload being sent to TastyTrade in JSON format
            logger.info(f"TastyTrade: Sending dry-run payload: {json.dumps(tastytrade_order, indent=2)}")
            
            url = f"{self.base_url}/accounts/{self.account_id}/orders/dry-run"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "Content-Type": "application/json",
                "User-Agent": "juicytrade/1.0"
            }

            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.post(url, headers=headers, json=tastytrade_order)
                
                # Handle 422 responses specially - they contain error details in the body
                if response.status_code == 422:
                    try:
                        response_data = response.json()
                        error_data = response_data.get("error", {})
                        
                        if error_data:
                            error_messages = []
                            
                            # Extract only the detailed error messages from errors array
                            errors_array = error_data.get("errors", [])
                            for error in errors_array:
                                error_message = error.get("message", "")
                                if error_message:
                                    error_messages.append(error_message)
                            
                            return {
                                "status": "error",
                                "validation_errors": error_messages if error_messages else ["Order validation failed"],
                                "commission": 0,
                                "cost": 0,
                                "fees": 0,
                                "order_cost": 0,
                                "margin_change": 0,
                                "buying_power_effect": 0,
                                "day_trades": 0,
                                "estimated_total": 0
                            }
                    except Exception as e:
                        logger.error(f"Error parsing 422 response: {e}")
                        return {
                            "status": "error",
                            "validation_errors": ["Order validation failed - unable to parse error details"],
                            "commission": 0,
                            "cost": 0,
                            "fees": 0,
                            "order_cost": 0,
                            "margin_change": 0,
                            "buying_power_effect": 0,
                            "day_trades": 0,
                            "estimated_total": 0
                        }
                
                # For other status codes, raise for status as usual
                response.raise_for_status()
                
                response_data = response.json()
                data = response_data.get("data", {})
                error_data = response_data.get("error", {})
                
                # Check for errors first
                if error_data:
                    error_messages = []
                    
                    # Extract only the detailed error messages from errors array
                    errors_array = error_data.get("errors", [])
                    for error in errors_array:
                        error_message = error.get("message", "")
                        if error_message:
                            error_messages.append(error_message)
                    
                    return {
                        "status": "error",
                        "validation_errors": error_messages,
                        "commission": 0,
                        "cost": 0,
                        "fees": 0,
                        "order_cost": 0,
                        "margin_change": 0,
                        "buying_power_effect": 0,
                        "day_trades": 0,
                        "estimated_total": 0
                    }
                
                # Check for warnings (keep existing logic for backward compatibility)
                warnings = data.get("warnings", [])
                if warnings:
                    # Filter out informational warnings
                    error_warnings = [w for w in warnings if w.get("message") not in ["Your order will begin working during next valid session."]]
                    if error_warnings:
                        return {
                            "status": "error",
                            "validation_errors": [w.get("message") for w in error_warnings],
                            "commission": 0,
                            "cost": 0,
                            "fees": 0,
                            "order_cost": 0,
                            "margin_change": 0,
                            "buying_power_effect": 0,
                            "day_trades": 0,
                            "estimated_total": 0
                        }
                
                fee_calculation = data.get("fee-calculation", {})
                buying_power_effect = data.get("buying-power-effect", {})
                
                price_effect = data.get("order", {}).get("price-effect", "")
                order_price = float(data.get("order", {}).get("price", 0))
                total_fees = float(fee_calculation.get("total-fees", 0))
                commission = float(fee_calculation.get("commission", 0))
                fees = total_fees - commission
                margin_change = float(buying_power_effect.get("change-in-margin-requirement", 0))
                buying_power_impact = float(buying_power_effect.get("impact", 0))

                # Calculate estimated_total based on price-effect
                if price_effect.lower() == "credit":
                    estimated_total = (order_price * 100) - total_fees
                else:
                    estimated_total = (order_price * 100) + total_fees

                return {
                    "status": "ok",
                    "commission": commission,
                    "cost": order_price,
                    "fees": fees,
                    "order_cost": order_price,
                    "margin_change": margin_change,
                    "buying_power_effect": buying_power_impact,
                    "day_trades": 0, # Not provided in dry-run
                    "validation_errors": [],
                    "estimated_total": estimated_total
                }

        except Exception as e:
            self._log_error("preview_order", e)
            return {
                "status": "error",
                "validation_errors": [f"Preview failed: {str(e)}"],
                "commission": 0,
                "cost": 0,
                "fees": 0,
                "order_cost": 0,
                "margin_change": 0,
                "buying_power_effect": 0,
                "day_trades": 0,
                "estimated_total": 0
            }

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a multi-leg trading order."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            # Transform our standard multi-leg order format to TastyTrade format
            tastytrade_order = self._transform_to_tastytrade_multi_leg_order(order_data)
            
            # Log the order being sent for debugging
            logger.info(f"TastyTrade: Sending multi-leg order: {tastytrade_order}")
            
            url = f"{self.base_url}/accounts/{self.account_id}/orders"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "Content-Type": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.post(url, headers=headers, json=tastytrade_order)
                
                # Log response details for debugging
                logger.info(f"TastyTrade: Order response status: {response.status_code}")
                if response.status_code != 200:
                    logger.error(f"TastyTrade: Order response body: {response.text}")
                
                response.raise_for_status()
                
                data = response.json()
                # TastyTrade nests the actual order data under data.order
                order_data = data.get("data", {}).get("order", {})
                
                # Transform TastyTrade order response to our standard model
                return self._transform_tastytrade_order(order_data)
                
        except Exception as e:
            self._log_error("place_multi_leg_order", e)
            raise
    
    async def cancel_order(self, order_id: str) -> bool:
        """Cancel an existing order."""
        try:
            if not await self._ensure_valid_session():
                raise Exception("Failed to authenticate with TastyTrade")
            
            url = f"{self.base_url}/accounts/{self.account_id}/orders/{order_id}"
            headers = {
                "Authorization": self._session_token,
                "Accept": "application/json",
                "User-Agent": "juicytrade/1.0"
            }
            
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.delete(url, headers=headers)
                response.raise_for_status()
                
                # TastyTrade returns success if the order was cancelled
                return True
                
        except Exception as e:
            self._log_error(f"cancel_order {order_id}", e)
            return False
    
    # === Streaming Methods ===
    
    async def _ensure_healthy_connection(self) -> bool:
        """Ensure we have a healthy streaming connection, reconnect if needed."""
        try:
            # Check if we're already connected and healthy
            if self.is_connected and self._stream_connection:
                # Test the connection with a ping
                try:
                    await asyncio.wait_for(self._stream_connection.ping(), timeout=5.0)
                    logger.debug("TastyTrade: Connection health check passed")
                    return True
                except (asyncio.TimeoutError, websockets.exceptions.ConnectionClosed, Exception):
                    logger.warning("TastyTrade: Connection health check failed, reconnecting...")
                    self.is_connected = False
                    self._connection_ready.clear()
            
            # Connection is not healthy, attempt to reconnect
            logger.info("TastyTrade: Establishing new streaming connection...")
            return await self.connect_streaming()
            
        except Exception as e:
            logger.error(f"TastyTrade: Error ensuring healthy connection: {e}")
            return False

    async def connect_streaming(self) -> bool:
        """Connect to DXLink streaming service with health monitoring."""
        try:
            # Update health status
            streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.CONNECTING)
            
            # Clean up any existing connection first
            # Cancel existing streaming task to avoid concurrent recv calls/leaks
            if self._streaming_task and not self._streaming_task.done():
                try:
                    self._streaming_task.cancel()
                    # Wait briefly for the task to cancel to avoid overlapping recv() calls
                    try:
                        await asyncio.wait_for(self._streaming_task, timeout=5.0)
                    except Exception:
                        pass
                except Exception:
                    pass
                self._streaming_task = None

            if self._stream_connection:
                try:
                    await TimeoutWrapper.execute(
                        self._stream_connection.close(),
                        timeout=5.0,
                        operation_name="close_existing_connection"
                    )
                except:
                    pass
                self._stream_connection = None
            
            # Reset connection state
            self.is_connected = False
            self._connection_ready.clear()
            
            # Get fresh quote token for DXLink authentication
            if not await TimeoutWrapper.execute(
                self._get_quote_token(),
                timeout=30.0,
                operation_name="get_quote_token"
            ):
                logger.error("Failed to get quote token for streaming")
                streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.FAILED)
                return False
            
            # Create DXLink streaming connection with retry logic
            max_retries = 3
            for attempt in range(max_retries):
                try:
                    logger.info(f"TastyTrade: Connection attempt {attempt + 1}/{max_retries}")
                    self._stream_connection = await TimeoutWrapper.execute(
                        websockets.connect(
                            self._dxlink_url,
                            ping_interval=30,
                            ping_timeout=10,
                            close_timeout=5
                        ),
                        timeout=15.0,
                        operation_name=f"connect_websocket_attempt_{attempt + 1}"
                    )
                    break
                except Exception as e:
                    logger.warning(f"TastyTrade: Connection attempt {attempt + 1} failed: {e}")
                    streaming_health_manager.record_error(self._connection_id, str(e))
                    if attempt == max_retries - 1:
                        raise
                    await asyncio.sleep(2 ** attempt)  # Exponential backoff
            
            # Execute DXLink setup sequence
            if not await TimeoutWrapper.execute(
                self._dxlink_streaming_setup(),
                timeout=60.0,
                operation_name="dxlink_setup"
            ):
                logger.error("DXLink streaming setup failed")
                streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.FAILED)
                return False
            
            self.is_connected = True
            self._connection_ready.set()
            streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.CONNECTED)
            
            # Start the streaming data processing task
            await self._start_streaming_task()
            
            logger.info("TastyTrade streaming connected successfully")
            return True
            
        except Exception as e:
            logger.error(f"Error connecting to TastyTrade streaming: {e}")
            self.is_connected = False
            self._connection_ready.clear()
            streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.FAILED)
            streaming_health_manager.record_error(self._connection_id, str(e))
            return False
    
    async def disconnect_streaming(self) -> bool:
        """Disconnect from DXLink streaming service."""
        try:
            if self._stream_connection:
                await self._stream_connection.close()
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
        """Subscribe to real-time data for symbols via DXLink using correct streamer symbols."""
        logger.info(f"TastyTrade: subscribe_to_symbols called with {len(symbols)} symbols, data_types: {data_types}")
        
        try:
            # Check if connection is healthy, reconnect if needed
            if not await self._ensure_healthy_connection():
                logger.error("TastyTrade: Failed to establish healthy connection")
                return False
            
            # Wait for connection to be ready
            await self._connection_ready.wait()
            
            # Default to both Quote and Greeks if no data_types specified (backward compatibility)
            if data_types is None:
                data_types = ["Quote", "Greeks"]
            
            # Prepare subscriptions using correct streaming symbols
            subscriptions = []
            
            for symbol in symbols:
                # Convert to streaming symbol format for DXLink
                if self._is_option_symbol(symbol):
                    streaming_symbol = self._convert_to_streamer_symbol(symbol)
                    logger.debug(f"TastyTrade: Using streamer symbol for {symbol}: {streaming_symbol}")
                else:
                    streaming_symbol = symbol  # Stock symbols use as-is
                    logger.debug(f"TastyTrade: Using stock symbol as-is: {streaming_symbol}")
                
                # Add Quote subscription only if requested
                if "Quote" in data_types:
                    subscriptions.append({
                        "type": "Quote",
                        "symbol": streaming_symbol
                    })
                    logger.debug(f"TastyTrade: Added Quote subscription for {streaming_symbol}")
                
                # Add Greeks subscription for option symbols only if requested
                if "Greeks" in data_types and self._is_option_symbol(symbol):
                    subscriptions.append({
                        "type": "Greeks",
                        "symbol": streaming_symbol
                    })
                    logger.debug(f"TastyTrade: Added Greeks subscription for {streaming_symbol}")
            
            # Send subscription message
            subscription_msg = {
                "type": "FEED_SUBSCRIPTION",
                "channel": 1,
                "reset": False,
                "add": subscriptions
            }
            
            logger.info(f"TastyTrade: Sending subscription message with {len(subscriptions)} subscriptions for data_types: {data_types}")
            await self._stream_connection.send(json.dumps(subscription_msg))
            
            # Update subscribed symbols
            self._subscribed_symbols.update(symbols)
            
            logger.info(f"TastyTrade: Subscribed to {len(symbols)} symbols with {len(subscriptions)} subscriptions")
            return True
            
        except Exception as e:
            logger.error(f"Error subscribing to TastyTrade symbols: {e}")
            # Mark connection as unhealthy so it gets reconnected next time
            self.is_connected = False
            self._connection_ready.clear()
            return False
    
    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """Unsubscribe from real-time data for symbols via DXLink."""
        logger.info(f"TastyTrade: unsubscribe_from_symbols called with {len(symbols)} symbols and data_types: {data_types}")
        
        try:
            if not self.is_connected:
                return True  # Already disconnected

            # If no data_types are specified, default to both for backward compatibility
            if data_types is None:
                data_types = ["Quote", "Greeks"]

            # Convert symbols to TastyTrade format and prepare unsubscriptions
            subscriptions = []
            
            for symbol in symbols:
                # Convert to streaming symbol format for DXLink
                if self._is_option_symbol(symbol):
                    streaming_symbol = self._convert_to_streamer_symbol(symbol)
                    logger.debug(f"TastyTrade: Using streamer symbol for unsubscription: {streaming_symbol}")
                else:
                    streaming_symbol = symbol  # Stock symbols use as-is
                
                if "Quote" in data_types:
                    subscriptions.append({
                        "type": "Quote",
                        "symbol": streaming_symbol
                    })
                
                if "Greeks" in data_types and self._is_option_symbol(symbol):
                    subscriptions.append({
                        "type": "Greeks",
                        "symbol": streaming_symbol
                    })
            
            if not subscriptions:
                logger.info("TastyTrade: No subscriptions to remove.")
                return True
            
            # Send unsubscription message
            unsubscription_msg = {
                "type": "FEED_SUBSCRIPTION",
                "channel": 1,
                "reset": False,
                "remove": subscriptions
            }
            
            await self._stream_connection.send(json.dumps(unsubscription_msg))
            
            # Update subscribed symbols
            self._subscribed_symbols.difference_update(symbols)
            
            logger.info(f"TastyTrade: Unsubscribed from {len(symbols)} symbols")
            return True
            
        except Exception as e:
            logger.error(f"Error unsubscribing from TastyTrade symbols: {e}")
            return False

    async def get_streaming_greeks_batch(self, symbols: List[str], timeout: int = 5) -> Dict[str, Dict]:
        """
        Subscribes to a list of option symbols and waits for their greeks data.
        This is more efficient than calling get_streaming_greeks for each symbol individually.
        """
        await self._ensure_healthy_connection()

        futures = {symbol: asyncio.get_running_loop().create_future() for symbol in symbols}
        for symbol, future in futures.items():
            self._greeks_requests[symbol] = future

        await self.subscribe_to_symbols(symbols, data_types=['Greeks'])

        tasks = [asyncio.wait_for(future, timeout=timeout) for future in futures.values()]
        
        results = {}
        try:
            greeks_list = await asyncio.gather(*tasks, return_exceptions=True)
            for i, symbol in enumerate(symbols):
                result = greeks_list[i]
                if isinstance(result, Exception):
                    logger.warning(f"Timeout or error waiting for greeks for {symbol}: {result}")
                    results[symbol] = None
                else:
                    results[symbol] = result
        except asyncio.TimeoutError:
            logger.warning(f"Timeout waiting for greeks for batch of {len(symbols)} symbols")
        finally:
            await self.unsubscribe_from_symbols(symbols, data_types=['Greeks'])
            for symbol in symbols:
                if symbol in self._greeks_requests:
                    del self._greeks_requests[symbol]
        
        return results
    
    # === Utility Methods ===
    
    def _convert_to_streamer_symbol(self, symbol: str) -> str:
        """Convert standard OCC symbol to TastyTrade DXLink streamer format.
        
        Examples:
        - SPXW250806P02600000 -> .SPXW250806P2600
        - SPY250808C00643000 -> .SPY250808C643
        """
        try:
            if not self._is_option_symbol(symbol):
                return symbol
            
            # Parse the standard OCC symbol
            # Format: ROOT + YYMMDD + C/P + STRIKE(8 digits)
            if len(symbol) < 15:
                logger.warning(f"TastyTrade: Symbol too short for option parsing: {symbol}")
                return symbol
            
            # Find where the date starts (6 consecutive digits)
            for i in range(len(symbol) - 14):  # Need at least 15 chars after position i
                date_part = symbol[i:i+6]
                if date_part.isdigit():
                    # Found the date part
                    root = symbol[:i]
                    date = date_part  # YYMMDD
                    option_type = symbol[i+6]  # C or P
                    strike_part = symbol[i+7:i+15]  # 8 digits
                    
                    # Convert strike to actual dollar amount (OCC format is dollars * 1000)
                    try:
                        strike_raw = int(strike_part)
                        # Divide by 1000 to get actual strike price
                        strike_dollars = strike_raw / 1000
                        # Convert to clean string (remove .0 if it's a whole number)
                        if strike_dollars == int(strike_dollars):
                            strike_clean = str(int(strike_dollars))
                        else:
                            strike_clean = str(strike_dollars)
                        
                        # Build streamer symbol: .ROOT + DATE + TYPE + STRIKE
                        streamer_symbol = f".{root}{date}{option_type}{strike_clean}"
                        
                        logger.debug(f"TastyTrade: Converted {symbol} -> {streamer_symbol}")
                        return streamer_symbol
                        
                    except ValueError:
                        logger.error(f"TastyTrade: Invalid strike price in symbol: {symbol}")
                        return symbol
            
            logger.warning(f"TastyTrade: Could not parse option symbol: {symbol}")
            return symbol
            
        except Exception as e:
            logger.error(f"TastyTrade: Error converting symbol to streamer format: {e}")
            return symbol

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol using TastyTrade format."""
        # TastyTrade uses OCC format similar to other providers
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])

    def _normalize_option_type(self, option_type: str) -> str:
        """Normalize option type to 'call' or 'put'."""
        if not option_type:
            return "call"  # Default
        lower_type = option_type.lower()
        if lower_type == "c" or lower_type == "call":
            return "call"
        if lower_type == "p" or lower_type == "put":
            return "put"
        return "call" # Fallback
    
    def _get_provider_type(self) -> str:
        """Override to return 'tastytrade' for symbol conversion."""
        return "tastytrade"
    
    def convert_symbol_to_standard_format(self, symbol: str) -> str:
        """Convert TastyTrade symbol to standard OCC format.
        
        Handles both:
        - TastyTrade API symbols with spaces: "SPXW  250808C06290000" -> "SPXW250808C06290000"
        - TastyTrade streamer symbols: ".SPXW250808P6330" -> "SPXW250808P06330000"
        """
        try:
            # Handle TastyTrade API symbols with extra spaces
            if not symbol.startswith('.') and '  ' in symbol:
                # Remove extra spaces from TastyTrade API symbols
                cleaned_symbol = ' '.join(symbol.split())  # Normalize all whitespace to single spaces
                cleaned_symbol = cleaned_symbol.replace(' ', '')  # Remove all spaces
                logger.debug(f"TastyTrade: Cleaned API symbol {symbol} -> {cleaned_symbol}")
                return cleaned_symbol
            
            # Handle streamer symbols (start with dot)
            if symbol.startswith('.'):
                # Remove the leading dot
                symbol_no_dot = symbol[1:]
                
                # Parse the streamer symbol format: ROOT + YYMMDD + C/P + STRIKE
                if len(symbol_no_dot) < 10:
                    logger.warning(f"TastyTrade: Streamer symbol too short: {symbol}")
                    return symbol
                
                # Find where the date starts (6 consecutive digits)
                for i in range(len(symbol_no_dot) - 9):  # Need at least 10 chars after position i
                    date_part = symbol_no_dot[i:i+6]
                    if date_part.isdigit():
                        # Found the date part
                        root = symbol_no_dot[:i]
                        date = date_part  # YYMMDD
                        option_type = symbol_no_dot[i+6]  # C or P
                        strike_part = symbol_no_dot[i+7:]  # Strike price
                        
                        # Convert strike back to 8-digit OCC format (multiply by 1000)
                        try:
                            strike_dollars = float(strike_part)
                            strike_raw = int(strike_dollars * 1000)
                            strike_formatted = f"{strike_raw:08d}"  # 8 digits with leading zeros
                            
                            # Build standard OCC symbol: ROOT + DATE + TYPE + STRIKE
                            standard_symbol = f"{root}{date}{option_type}{strike_formatted}"
                            
                            logger.debug(f"TastyTrade: Converted streamer {symbol} -> standard {standard_symbol}")
                            return standard_symbol
                            
                        except ValueError:
                            logger.error(f"TastyTrade: Invalid strike price in streamer symbol: {symbol}")
                            return symbol
                
                logger.warning(f"TastyTrade: Could not parse streamer symbol: {symbol}")
                return symbol
            
            # If no special handling needed, return as-is
            return symbol
            
        except Exception as e:
            logger.error(f"TastyTrade: Error converting symbol to standard format: {e}")
            return symbol
    
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
    
    def _transform_tastytrade_option_contract(self, raw_contract: Dict[str, Any], pricing_data: Dict[str, Any] = None) -> Optional[OptionContract]:
        """Transform TastyTrade option contract to our standard model with optional pricing data."""
        try:
            symbol = raw_contract.get("symbol", "")
            
            # Convert TastyTrade symbol to standard OCC format for UI consistency
            standard_symbol = self.convert_symbol_to_standard_format(symbol)
            
            return OptionContract(
                symbol=standard_symbol,  # UI gets standard OCC format
                underlying_symbol=raw_contract.get("underlying-symbol", ""),
                expiration_date=raw_contract.get("expiration-date", ""),
                strike_price=float(raw_contract.get("strike-price", 0)),
                type=self._normalize_option_type(raw_contract.get("option-type", "")),
                root_symbol=raw_contract.get("root-symbol", ""),  # Use root-symbol directly from API
                bid=None,
                ask=None,
                close_price=None,
                volume=None,
                open_interest=None,
                # Greeks from pricing data if available
                implied_volatility=None,
                delta=None,
                gamma=None,
                theta=None,
                vega=None,
            )
        except Exception as e:
            self._log_error("transform_tastytrade_option_contract", e)
            return None
    
    def _transform_to_basic_contract(self, contract: OptionContract) -> Optional[OptionContract]:
        """Transform option contract to basic format without Greeks (faster)."""
        try:
            return OptionContract(
                symbol=contract.symbol,
                underlying_symbol=contract.underlying_symbol,
                expiration_date=contract.expiration_date,
                strike_price=contract.strike_price,
                type=contract.type,
                root_symbol=contract.root_symbol,  # Preserve root_symbol
                bid=contract.bid,
                ask=contract.ask,
                close_price=contract.close_price,
                volume=contract.volume,
                open_interest=contract.open_interest,
                # Greeks are None for basic loading (will be loaded separately)
                implied_volatility=None,
                delta=None,
                gamma=None,
                theta=None,
                vega=None,
            )
        except Exception as e:
            self._log_error("transform_to_basic_contract", e)
            return None

    def _transform_to_tastytrade_order(self, order_data: Dict[str, Any]) -> Dict[str, Any]:
        """Transform our standard order format to TastyTrade format."""
        try:
            # Map our order types to TastyTrade format
            order_type_map = {
                "market": "Market",
                "limit": "Limit",
                "stop": "Stop",
                "stop_limit": "Stop Limit"
            }
            
            # Map our actions to TastyTrade format - but we'll handle sell dynamically
            action_map = {
                "buy": "Buy to Open",
                "sell": "Sell to Close",  # Will be overridden below for short sells
                "buy_to_open": "Buy to Open",
                "sell_to_open": "Sell to Open",
                "buy_to_close": "Buy to Close",
                "sell_to_close": "Sell to Close"
            }
            
            # Determine the correct action for sell orders
            side = order_data.get("side", "buy").lower()
            if side == "sell":
                # Check if this is a short sell
                is_short_sell = order_data.get("is_short_sell", False)
                if is_short_sell:
                    action = "Sell to Open"  # Short selling
                else:
                    action = "Sell to Close"  # Closing existing position
            else:
                action = action_map.get(side, "Buy to Open")
            
            # Map our time in force to TastyTrade format
            tif_map = {
                "day": "Day",
                "gtc": "GTC",
                "ioc": "IOC",
                "fok": "FOK"
            }
            
            # Determine instrument type
            symbol = order_data.get("symbol", "")
            instrument_type = "Equity Option" if self._is_option_symbol(symbol) else "Equity"
            
            # Build TastyTrade order
            tastytrade_order = {
                "order-type": order_type_map.get(order_data.get("order_type", "limit").lower(), "Limit"),
                "time-in-force": tif_map.get(order_data.get("time_in_force", "day").lower(), "Day"),
                "legs": [
                    {
                        "instrument-type": instrument_type,
                        "action": action,
                        "quantity": int(order_data.get("quantity", 1)),
                        "symbol": symbol
                    }
                ]
            }
            
            # Add price for limit orders
            order_type = order_data.get("order_type", "").lower()
            if order_type in ["limit", "stop_limit"]:
                # Check multiple possible price fields
                price = (order_data.get("price") or 
                        order_data.get("limit_price") or 
                        order_data.get("net_price") or
                        order_data.get("premium"))
                
                if price is not None:  # Allow 0 price
                    price_float = float(price)
                    
                    # TastyTrade requires positive prices and correct price-effect
                    tastytrade_order["price"] = abs(price_float)  # Always make price positive
                    
                    # Determine price-effect based on order side and price sign
                    side = order_data.get("side", "buy").lower()
                    if price_float < 0:
                        # Negative price explicitly indicates credit
                        tastytrade_order["price-effect"] = "Credit"
                    elif side in ["sell", "sell_to_open", "sell_to_close"]:
                        # Selling typically receives credit (you get money)
                        tastytrade_order["price-effect"] = "Credit"
                    else:
                        # Buying typically pays debit (you pay money)
                        tastytrade_order["price-effect"] = "Debit"
                else:
                    # If no price provided, this is likely an error - log it
                    logger.warning(f"TastyTrade: No price found in order data for limit order: {order_data}")
                    # Don't set a default price - let TastyTrade reject it with a proper error
                    raise ValueError("Limit order requires a price but none was provided")
            
            # Add stop price for stop orders
            if order_data.get("stop_price"):
                tastytrade_order["stop-trigger"] = float(order_data["stop_price"])
            
            return tastytrade_order
            
        except Exception as e:
            self._log_error("transform_to_tastytrade_order", e)
            raise
    
    def _transform_to_tastytrade_multi_leg_order(self, order_data: Dict[str, Any]) -> Dict[str, Any]:
        """Transform our standard multi-leg order format to TastyTrade format."""
        try:
            # Map our order types to TastyTrade format
            order_type_map = {
                "market": "Market",
                "limit": "Limit",
                "stop": "Stop",
                "stop_limit": "Stop Limit"
            }
            
            # Map our actions to TastyTrade format
            action_map = {
                "buy": "Buy to Open",
                "sell": "Sell to Close",
                "buy_to_open": "Buy to Open",
                "sell_to_open": "Sell to Open",
                "buy_to_close": "Buy to Close",
                "sell_to_close": "Sell to Close"
            }
            
            # Map our time in force to TastyTrade format
            tif_map = {
                "day": "Day",
                "gtc": "GTC",
                "ioc": "IOC",
                "fok": "FOK"
            }
            
            # Build legs with proper symbol conversion
            legs = []
            order_legs = order_data.get("legs") or []
            
            # If no legs provided, this is a single-leg order - create leg from order data
            if not order_legs:
                original_symbol = order_data.get("symbol", "")
                
                # Convert symbol to TastyTrade format if it's an option
                if self._is_option_symbol(original_symbol):
                    tastytrade_symbol = self.convert_symbol_to_provider_format(original_symbol)
                    instrument_type = "Equity Option"
                else:
                    tastytrade_symbol = original_symbol
                    instrument_type = "Equity"
                
                # Determine the correct action for sell orders
                side = order_data.get("side", "buy").lower()
                if side == "sell":
                    # Check if this is a short sell
                    is_short_sell = order_data.get("is_short_sell", False)
                    if is_short_sell:
                        action = "Sell to Open"  # Short selling
                    else:
                        action = "Sell to Close"  # Closing existing position
                else:
                    action = action_map.get(side, "Buy to Open")
                
                legs.append({
                    "instrument-type": instrument_type,
                    "action": action,
                    "quantity": int(order_data.get("qty", 1)),
                    "symbol": tastytrade_symbol
                })
            else:
                # Process multi-leg order
                for leg in order_legs:
                    original_symbol = leg.get("symbol", "")
                
                    # Convert symbol to TastyTrade format if it's an option
                    if self._is_option_symbol(original_symbol):
                        tastytrade_symbol = self.convert_symbol_to_provider_format(original_symbol)
                        instrument_type = "Equity Option"
                    else:
                        tastytrade_symbol = original_symbol
                        instrument_type = "Equity"
                    
                    legs.append({
                        "instrument-type": instrument_type,
                        "action": action_map.get(leg.get("side", "buy").lower(), "Buy to Open"),
                        "quantity": int(leg.get("quantity", 1)),
                        "symbol": tastytrade_symbol
                    })
            
            # Build TastyTrade multi-leg order
            tastytrade_order = {
                "order-type": order_type_map.get(order_data.get("order_type", "limit").lower(), "Limit"),
                "time-in-force": tif_map.get(order_data.get("time_in_force", "day").lower(), "Day"),
                "legs": legs
            }
            
            # Add price and price-effect for limit orders (required for TastyTrade)
            order_type = order_data.get("order_type", "limit").lower()
            
            if order_type in ["limit", "stop_limit"]:  # Both limit and stop_limit need price
                # Check multiple possible price fields
                price = (order_data.get("price") or 
                        order_data.get("limit_price") or 
                        order_data.get("net_price") or
                        order_data.get("premium"))
                
                if price is not None:  # Allow 0 price
                    price_float = float(price)
                    
                    # TastyTrade requires positive prices and correct price-effect
                    tastytrade_order["price"] = abs(price_float)  # Always make price positive
                    
                    # Determine price-effect based on order side and price sign
                    side = order_data.get("side")
                    if price_float < 0:
                        # Negative price explicitly indicates credit
                        tastytrade_order["price-effect"] = "Credit"
                    elif side and side.lower() in ["sell", "sell_to_open", "sell_to_close"]:
                        # Selling typically receives credit (you get money)
                        tastytrade_order["price-effect"] = "Credit"
                    else:
                        # For multi-leg orders or buy orders, determine from price sign
                        # If price is positive and we don't have a clear sell side, default to Debit
                        tastytrade_order["price-effect"] = "Debit"
                else:
                    # If no price provided, this is likely an error - log it
                    logger.warning(f"TastyTrade: No price found in order data for limit order: {order_data}")
                    # Don't set a default price - let TastyTrade reject it with a proper error
                    raise ValueError("Limit order requires a price but none was provided")
            elif order_type == "market":
                # Market orders don't need price but may need price-effect for multi-leg
                order_legs_check = order_data.get("legs") or []
                if len(order_legs_check) > 1:
                    tastytrade_order["price-effect"] = order_data.get("price_effect", "Debit")
            
            # Log the final TastyTrade order payload
            logger.info(f"TastyTrade: Final multi-leg order payload: {tastytrade_order}")
            
            return tastytrade_order
            
        except Exception as e:
            self._log_error("transform_to_tastytrade_multi_leg_order", e)
            raise
    
    def _transform_tastytrade_order(self, raw_order: Dict[str, Any]) -> Optional[Order]:
        """Transform TastyTrade order to our standard model."""
        try:
            # Map TastyTrade status to our standard status
            status_map = {
                "Live": "open",
                "Routed": "open",  # TastyTrade uses "Routed" for submitted orders
                "Filled": "filled",
                "Cancelled": "cancelled",
                "Rejected": "rejected",
                "Expired": "expired"
            }
            
            # Map TastyTrade order type to our standard format
            order_type_map = {
                "Market": "market",
                "Limit": "limit",
                "Stop": "stop",
                "Stop Limit": "stop_limit"
            }
            
            # Extract legs information
            legs = raw_order.get("legs", [])
            
            # For single-leg orders, use the first leg
            if legs:
                first_leg = legs[0]
                symbol = first_leg.get("symbol", "")
                # Convert TastyTrade symbol to standard OCC format for UI consistency
                standard_symbol = self.convert_symbol_to_standard_format(symbol)
                quantity = first_leg.get("quantity", 0)
                side = self._map_tastytrade_action_to_side(first_leg.get("action", ""))
                
                # Determine asset class from instrument type
                instrument_type = first_leg.get("instrument-type", "")
                if instrument_type == "Equity Option":
                    asset_class = "us_option"
                elif instrument_type == "Equity":
                    asset_class = "us_equity"
                else:
                    asset_class = "unknown"
            else:
                standard_symbol = ""
                quantity = 0
                side = "buy"
                asset_class = "unknown"
            
            # Calculate total filled quantity from all legs
            total_filled_qty = 0
            for leg in legs:
                filled_qty = leg.get("quantity", 0) - leg.get("remaining-quantity", 0)
                total_filled_qty += filled_qty
            
            # Parse submitted timestamp
            received_at = raw_order.get("received-at")
            if received_at:
                # Convert ISO timestamp to our format
                try:
                    from datetime import datetime
                    dt = datetime.fromisoformat(received_at.replace('Z', '+00:00'))
                    submitted_at = dt.isoformat()
                except:
                    submitted_at = received_at
            else:
                submitted_at = datetime.now().isoformat()
            
            # Handle limit_price with credit/debit conversion (same as Tradier)
            limit_price = float(raw_order.get("price")) if raw_order.get("price") else None
            price_effect = raw_order.get("price-effect", "").lower()
            
            # Convert credit orders to negative limit_price for UI consistency
            if price_effect == "credit" and limit_price is not None:
                limit_price = -abs(limit_price)
            
            # Handle avg_fill_price with same credit/debit logic
            avg_fill_price = float(raw_order.get("filled-price")) if raw_order.get("filled-price") else None
            if price_effect == "credit" and avg_fill_price is not None:
                avg_fill_price = -abs(avg_fill_price)
            
            return Order(
                id=str(raw_order.get("id", "")),
                symbol=standard_symbol,  # UI gets standard OCC format
                asset_class=asset_class,
                side=side,
                order_type=order_type_map.get(raw_order.get("order-type", ""), "limit"),
                qty=float(quantity),
                filled_qty=float(total_filled_qty),
                limit_price=limit_price,
                stop_price=float(raw_order.get("stop-trigger")) if raw_order.get("stop-trigger") else None,
                avg_fill_price=avg_fill_price,
                status=status_map.get(raw_order.get("status", ""), "unknown"),
                time_in_force=raw_order.get("time-in-force", "Day").lower(),
                submitted_at=submitted_at,
                filled_at=raw_order.get("updated-at") if raw_order.get("status") == "Filled" else None,
                legs=self._transform_tastytrade_legs(legs) if len(legs) > 1 else None
            )
            
        except Exception as e:
            self._log_error("transform_tastytrade_order", e)
            return None
    
    def _map_tastytrade_action_to_side(self, action: str) -> str:
        """Map TastyTrade action to our standard side."""
        action_lower = action.lower()
        if "buy" in action_lower:
            return "buy"
        elif "sell" in action_lower:
            return "sell"
        else:
            return "buy"  # Default
    
    def _transform_tastytrade_legs(self, legs: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Transform TastyTrade legs to our standard format."""
        transformed_legs = []
        for leg in legs:
            symbol = leg.get("symbol", "")
            # Convert TastyTrade symbol to standard OCC format for UI consistency
            standard_symbol = self.convert_symbol_to_standard_format(symbol)
            
            transformed_legs.append({
                "symbol": standard_symbol,  # UI gets standard OCC format
                "quantity": leg.get("quantity", 0),
                "side": self._map_tastytrade_action_to_side(leg.get("action", "")),
                "instrument_type": leg.get("instrument-type", "")
            })
        return transformed_legs
    
    def _transform_tastytrade_position(self, raw_position: Dict[str, Any]) -> Optional[Position]:
        """Transform TastyTrade position to our standard model."""
        try:
            symbol = raw_position.get("symbol", "")
            
            # Convert TastyTrade symbol to standard OCC format for UI consistency
            standard_symbol = self.convert_symbol_to_standard_format(symbol)
            
            # Map TastyTrade instrument-type to our standard asset_class
            instrument_type = raw_position.get("instrument-type", "")
            if instrument_type == "Equity":
                asset_class = "us_equity"
            elif instrument_type == "Equity Option":
                asset_class = "us_option"
            else:
                asset_class = "unknown"
            
            # Determine side based on quantity
            qty = raw_position.get("quantity", 0)
            side = "long" if qty >= 0 else "short"
            
            # Get TastyTrade-specific fields
            avg_open_price = float(raw_position.get("average-open-price", 0)) if raw_position.get("average-open-price") else 0
            close_price = float(raw_position.get("close-price", 0)) if raw_position.get("close-price") else 0
            quantity = float(qty)
            multiplier = float(raw_position.get("multiplier", 1))
            
            # Calculate cost basis (total amount paid/received for the position)
            cost_basis = avg_open_price * quantity * multiplier
            
            # Calculate current market value
            market_value = close_price * quantity * multiplier
            
            # Calculate unrealized P/L
            unrealized_pl = market_value - cost_basis
            
            # Handle cost-effect (Credit means we received money, so cost_basis should be negative)
            cost_effect = raw_position.get("cost-effect", "")
            if cost_effect == "Credit":
                cost_basis = -abs(cost_basis)  # Make cost basis negative for credits
            
            return Position(
                symbol=standard_symbol,  # UI gets standard OCC format
                qty=float(qty),
                side=side,
                market_value=market_value,
                cost_basis=cost_basis,
                unrealized_pl=unrealized_pl,
                current_price=close_price,
                avg_entry_price=avg_open_price,  # Use average-open-price directly
                asset_class=asset_class,
                lastday_price=float(raw_position.get("average-daily-market-close-price", 0)) if raw_position.get("average-daily-market-close-price") else None,
                date_acquired=raw_position.get("created-at")  # Use created-at as acquisition date
            )
            
        except Exception as e:
            self._log_error("transform_tastytrade_position", e)
            return None
    
    def _transform_tastytrade_account(self, raw_account: Dict[str, Any]) -> Optional[Account]:
        """Transform TastyTrade account balances to our standard model."""
        try:
            return Account(
                account_id=self.account_id,  # Use the account_id from provider instance
                account_number=raw_account.get("account-number", ""),
                status="active",  # TastyTrade accounts are active if we can fetch balances
                buying_power=raw_account.get("equity-buying-power"),
                cash=raw_account.get("cash-balance"),
                day_trading_buying_power=raw_account.get("day-trading-buying-power"),
                equity=raw_account.get("margin-equity"),
                initial_margin=raw_account.get("reg-t-margin-requirement"),
                maintenance_margin=raw_account.get("maintenance-requirement"),
                portfolio_value=raw_account.get("net-liquidating-value")
            )
            
        except Exception as e:
            self._log_error("transform_tastytrade_account", e)
            return None

    async def get_all_expirations_ivx(self, symbol: str, underlying_price: Optional[float] = None) -> List[Dict[str, Any]]:
        """Get IVx data for all expirations for a given symbol using streaming."""
        logger.info(f"TastyTrade: Getting all expirations IVx for {symbol}")
        
        # 1. Get expirations and underlying price
        expirations = await self.get_expiration_dates(symbol)
        if not expirations:
            logger.warning(f"No expirations found for {symbol}")
            return []

        if underlying_price is None:
            quote = await self.get_stock_quote(symbol)
            if quote and quote.bid and quote.ask:
                underlying_price = (quote.bid + quote.ask) / 2
            else:
                logger.warning(f"Could not get underlying price for {symbol}")
                return []

        # 2. Ensure streaming is connected
        await self._ensure_healthy_connection()

        # 3. Create tasks to fetch IVx for each expiration concurrently
        tasks = [
            self._get_ivx_for_expiration(symbol, exp_date_obj, underlying_price)
            for exp_date_obj in expirations
        ]
        
        results = await asyncio.gather(*tasks, return_exceptions=True)

        # 4. Filter out None results and log errors
        ivx_results = []
        for i, res in enumerate(results):
            if isinstance(res, Exception):
                logger.error(f"Error getting IVx for expiration {expirations[i]['date']}: {res}")
            elif res is not None:
                ivx_results.append(res)
        
        logger.info(f"Successfully retrieved IVx for {len(ivx_results)} expirations for {symbol}")
        return ivx_results

    async def get_all_expirations_ivx_streaming(self, symbol: str, underlying_price: Optional[float] = None, stream_callback=None) -> List[Dict[str, Any]]:
        """
        Get IVx data for all expirations with a fluid, concurrent streaming implementation.
        
        This method uses a semaphore to limit concurrency, ensuring that a fixed number of
        expirations are processed in parallel. As soon as one finishes, the next one starts,
        preventing slow or failing expirations from blocking the entire stream.
        
        Args:
            symbol: The underlying symbol
            underlying_price: Current underlying price (will fetch if None)
            stream_callback: Async callback to stream partial results
        
        Returns:
            List of IVx data dictionaries.
        """
        logger.info(f"TastyTrade: Starting fluid IVx streaming for {symbol}")
        
        # 1. Get expirations and underlying price
        expirations = await self.get_expiration_dates(symbol)
        if not expirations:
            logger.warning(f"No expirations found for {symbol}")
            return []

        if underlying_price is None:
            quote = await self.get_stock_quote(symbol)
            if quote and quote.bid and quote.ask:
                underlying_price = (quote.bid + quote.ask) / 2
            else:
                logger.warning(f"Could not get underlying price for {symbol}")
                return []

        # 2. Ensure streaming is connected
        await self._ensure_healthy_connection()

        # 3. Sort expirations and set up concurrency controls
        sorted_expirations = sorted(expirations, key=lambda x: x["date"])
        total_expirations = len(sorted_expirations)
        concurrency_limit = 5  # Process 5 expirations in parallel
        semaphore = asyncio.Semaphore(concurrency_limit)
        
        logger.info(f"TastyTrade: Processing {total_expirations} expirations with concurrency limit of {concurrency_limit}")

        # 4. Worker function to process a single expiration
        async def worker(exp_date_obj):
            async with semaphore:
                return await self._get_ivx_for_expiration(symbol, exp_date_obj, underlying_price)

        # 5. Create and process tasks
        tasks = [worker(exp) for exp in sorted_expirations]
        completed_count = 0
        all_results = []

        for future in asyncio.as_completed(tasks):
            try:
                result = await future
                completed_count += 1
                
                if result:
                    all_results.append(result)
                    logger.info(f"TastyTrade: Completed IVx calculation {completed_count}/{total_expirations} for {result['expiration_date']}")
                    if stream_callback:
                        await stream_callback(symbol, result, completed_count, total_expirations)
                else:
                    logger.warning(f"TastyTrade: IVx calculation failed for an expiration ({completed_count}/{total_expirations})")
            
            except Exception as e:
                completed_count += 1
                logger.error(f"TastyTrade: Error processing an expiration task: {e}")

        logger.info(f"TastyTrade: Successfully retrieved IVx for {len(all_results)}/{total_expirations} expirations for {symbol}")
        return all_results

    async def _get_ivx_for_expiration(self, symbol: str, exp_date_obj: dict, underlying_price: float) -> Optional[Dict[str, Any]]:
        """Helper to get IVx for a single expiration date."""
        exp_date = exp_date_obj['date']
        exp_root_symbol = exp_date_obj.get('symbol', symbol)
        exp_type = exp_date_obj.get('type')
        
        logger.debug(f"Processing IVx for {exp_root_symbol} {exp_date} ({exp_type})")

        # 1. Get basic option chain to find strikes
        options_chain = await self.get_options_chain_basic(
            symbol=exp_root_symbol,
            expiry=exp_date,
            underlying_price=underlying_price,
            strike_count=20,
            type=exp_type,
            underlying_symbol=symbol
        )
        if not options_chain:
            logger.warning(f"No options chain found for {exp_root_symbol} {exp_date}")
            return None

        # 2. Find ATM strike
        atm_strike_val = find_atm_strike(options_chain, underlying_price)
        if not atm_strike_val:
            logger.warning(f"Could not find ATM strike for {exp_root_symbol} {exp_date}")
            return None

        # 3. Find ATM contracts (call and put)
        atm_call = next((c for c in options_chain if c.strike_price == atm_strike_val and c.type == 'call'), None)
        atm_put = next((c for c in options_chain if c.strike_price == atm_strike_val and c.type == 'put'), None)

        if not atm_call or not atm_put:
            logger.warning(f"Could not find ATM call/put for {exp_root_symbol} {exp_date} at strike {atm_strike_val}")
            return None

        # 4. Fetch IV for both using the batch streaming greeks helper
        symbols_to_fetch = [atm_call.symbol, atm_put.symbol]
        try:
            greeks_results_dict = await self.get_streaming_greeks_batch(symbols_to_fetch)
        except asyncio.TimeoutError:
            logger.warning(f"Timeout fetching greeks for {exp_root_symbol} {exp_date}")
            return None

        valid_ivs = [
            greeks.get('implied_volatility') 
            for greeks in greeks_results_dict.values() 
            if greeks and greeks.get('implied_volatility') is not None
        ]
        
        if not valid_ivs:
            logger.warning(f"Failed to get IV for ATM options for {exp_root_symbol} {exp_date}")
            return None

        # 5. Calculate IVx (average of ATM call and put IV)
        ivx = sum(valid_ivs) / len(valid_ivs)

        # 6. Calculate DTE and Expected Move
        dte = (datetime.strptime(exp_date, "%Y-%m-%d").date() - datetime.now().date()).days
        expected_move = calculate_expected_move(underlying_price, ivx, dte)

        return {
            "expiration_date": exp_date,
            "ivx_percent": round(ivx * 100, 2),
            "expected_move_dollars": round(expected_move, 2)
        }

    async def get_streaming_quote(self, symbol: str, timeout: int = 10) -> Optional[Dict]:
        """Subscribes to a symbol and waits for its quote data via streaming."""
        await self._ensure_healthy_connection()

        future = asyncio.get_running_loop().create_future()
        self._quote_requests[symbol] = future

        await self.subscribe_to_symbols([symbol], data_types=['Quote'])

        try:
            quote = await asyncio.wait_for(future, timeout=timeout)
            return quote
        except asyncio.TimeoutError:
            logger.warning(f"Timeout waiting for quote for {symbol}")
            return None
        finally:
            await self.unsubscribe_from_symbols([symbol], data_types=['Quote'])
            if symbol in self._quote_requests:
                del self._quote_requests[symbol]

    async def get_streaming_greeks(self, symbol: str, timeout: int = 5) -> Optional[Dict]:
        """Subscribes to an option symbol and waits for its greeks data."""
        await self._ensure_healthy_connection()

        future = asyncio.get_running_loop().create_future()
        self._greeks_requests[symbol] = future

        await self.subscribe_to_symbols([symbol], data_types=['Greeks'])

        try:
            greeks = await asyncio.wait_for(future, timeout=timeout)
            return greeks
        except asyncio.TimeoutError:
            logger.warning(f"Timeout waiting for greeks for {symbol}")
            return None
        finally:
            await self.unsubscribe_from_symbols([symbol], data_types=['Greeks'])
            if symbol in self._greeks_requests:
                del self._greeks_requests[symbol]

    async def _dxlink_streaming_setup(self) -> bool:
        """Execute DXLink setup sequence for streaming connection."""
        try:
            # 1. SETUP
            setup_msg = {
                "type": "SETUP",
                "channel": 0,
                "version": "0.1-DXF-JS/0.3.0",
                "keepaliveTimeout": 60,
                "acceptKeepaliveTimeout": 60
            }
            await self._stream_connection.send(json.dumps(setup_msg))
            
            # Wait for SETUP response
            async with self._recv_lock:
                response = await asyncio.wait_for(self._stream_connection.recv(), timeout=10)
            setup_response = json.loads(response)
            if setup_response.get("type") != "SETUP":
                logger.error(f"Unexpected SETUP response: {setup_response}")
                return False
            
            # 2. Wait for AUTH_STATE: UNAUTHORIZED
            async with self._recv_lock:
                auth_state = await asyncio.wait_for(self._stream_connection.recv(), timeout=10)
            auth_response = json.loads(auth_state)
            if auth_response.get("type") != "AUTH_STATE" or auth_response.get("state") != "UNAUTHORIZED":
                logger.error(f"Unexpected AUTH_STATE response: {auth_response}")
                return False
            
            # 3. AUTHORIZE
            auth_msg = {
                "type": "AUTH",
                "channel": 0,
                "token": self._quote_token
            }
            await self._stream_connection.send(json.dumps(auth_msg))
            
            # Wait for AUTH_STATE: AUTHORIZED
            async with self._recv_lock:
                auth_success = await asyncio.wait_for(self._stream_connection.recv(), timeout=10)
            auth_success_response = json.loads(auth_success)
            if (auth_success_response.get("type") != "AUTH_STATE" or 
                auth_success_response.get("state") != "AUTHORIZED"):
                logger.error(f"Authorization failed: {auth_success_response}")
                return False
            
            # 4. CHANNEL_REQUEST
            channel_msg = {
                "type": "CHANNEL_REQUEST",
                "channel": 1,
                "service": "FEED",
                "parameters": {"contract": "AUTO"}
            }
            await self._stream_connection.send(json.dumps(channel_msg))
            
            # Wait for CHANNEL_OPENED
            async with self._recv_lock:
                channel_response = await asyncio.wait_for(self._stream_connection.recv(), timeout=10)
            channel_opened = json.loads(channel_response)
            if channel_opened.get("type") != "CHANNEL_OPENED":
                logger.error(f"Channel open failed: {channel_opened}")
                return False
            
            # 5. FEED_SETUP for Quote and Greeks events
            feed_setup_msg = {
                "type": "FEED_SETUP",
                "channel": 1,
                "acceptAggregationPeriod": 0.1,
                "acceptDataFormat": "COMPACT",
                "acceptEventFields": {
                    "Quote": ["eventType", "eventSymbol", "bidPrice", "askPrice", "bidSize", "askSize"],
                    "Greeks": ["eventType", "eventSymbol", "delta", "gamma", "theta", "vega", "volatility"]
                }
            }
            await self._stream_connection.send(json.dumps(feed_setup_msg))
            
            # Wait for FEED_CONFIG
            async with self._recv_lock:
                feed_response = await asyncio.wait_for(self._stream_connection.recv(), timeout=10)
            feed_config = json.loads(feed_response)
            if feed_config.get("type") != "FEED_CONFIG":
                logger.error(f"Feed setup failed: {feed_config}")
                return False
            
            logger.info("DXLink streaming setup completed successfully")
            return True
            
        except asyncio.TimeoutError:
            logger.error("DXLink streaming setup timeout")
            return False
        except Exception as e:
            logger.error(f"DXLink streaming setup error: {e}")
            return False

    def _process_greeks_feed_data(self, feed_data: List) -> Dict[str, Dict]:
        """Process FEED_DATA messages and extract Greeks events."""
        greeks_data = {}
        
        try:
            # DXLink COMPACT format: flat array with Greeks data
            # Format: ['Greeks', 'SYMBOL', delta, gamma, theta, vega, volatility, 'Greeks', ...]
            for item in feed_data:
                if isinstance(item, list):
                    # Parse the flat array - each Greeks event has 6 consecutive elements
                    i = 0
                    while i + 5 < len(item):
                        if item[i] == "Greeks":
                            symbol = item[i + 1]
                            greeks = {
                                "delta": item[i + 2],
                                "gamma": item[i + 3],
                                "theta": item[i + 4],
                                "vega": item[i + 5],
                                "implied_volatility": item[i + 6] if i + 6 < len(item) else None
                            }
                            greeks_data[symbol] = greeks
                            logger.debug(f"TastyTrade: Processed Greeks for {symbol}")
                            i += 7  # Move to next Greeks event (7 fields per event)
                        else:
                            i += 1  # Skip non-Greeks data
            
            if greeks_data:
                logger.debug(f"TastyTrade: Total Greeks processed: {len(greeks_data)} symbols")
            
        except Exception as e:
            logger.error(f"Error processing Greeks feed data: {e}")
            logger.error(f"Feed data structure: {feed_data}")
        
        return greeks_data

    def set_streaming_queue(self, queue):
        """Set the streaming queue for sending market data."""
        self._streaming_queue = queue
    
    def is_streaming_connected(self) -> bool:
        """Check if streaming connection is active."""
        return self.is_connected and self._stream_connection is not None
    
    async def _start_streaming_task(self):
        """Start the streaming data processing task."""
        if self._streaming_task is None or self._streaming_task.done():
            self._streaming_task = asyncio.create_task(self._process_streaming_data())
            logger.info("TastyTrade: Started streaming data processing task")
    
    async def _process_streaming_data(self):
        """Process incoming streaming data and send to queue with automatic recovery."""
        logger.info("TastyTrade: Starting streaming data processor")

        # Store current subscriptions for recovery
        current_subscriptions = set()
        recovery_attempt = 0
        max_recovery_attempts = 5

        # Start periodic keepalive task
        keepalive_task = asyncio.create_task(self._periodic_keepalive())

        try:
            while not self._shutdown_event.is_set():
                try:
                    # Main streaming loop: read messages while connected
                    while self.is_connected and self._stream_connection:
                        try:
                            # Serialize recv() calls to avoid concurrent recv errors
                            async with self._recv_lock:
                                message = await asyncio.wait_for(self._stream_connection.recv(), timeout=10.0)

                            data = json.loads(message)

                            # Update health manager that we received data
                            streaming_health_manager.record_data_received(self._connection_id)

                            logger.debug(f"TastyTrade: Received message type: {data.get('type')}")

                            if data.get("type") == "FEED_DATA":
                                # Process both Quote and Greeks events from the feed data
                                feed_data = data.get("data", [])
                                logger.debug(f"TastyTrade: Processing FEED_DATA with {len(feed_data)} items")
                                await self._process_feed_events(feed_data)
                            elif data.get("type") == "KEEPALIVE":
                                # Acknowledge keepalive from server
                                logger.debug("TastyTrade: Received keepalive from server")
                            else:
                                logger.debug(f"TastyTrade: Received non-FEED_DATA message: {data}")

                            # Reset recovery attempt counter on successful data processing
                            recovery_attempt = 0

                        except asyncio.TimeoutError:
                            # Timeout is normal - the periodic keepalive task handles keepalives
                            logger.debug("TastyTrade: Streaming timeout (normal - keepalive task handles connection maintenance)")
                            continue

                        except websockets.exceptions.ConnectionClosed as e:
                            # WebSocket connection was closed
                            close_code = getattr(e, 'code', None)
                            close_reason = getattr(e, 'reason', 'Unknown')

                            if close_code == 1000:
                                logger.warning(f"TastyTrade: Connection closed normally by server (code: {close_code}, reason: {close_reason})")
                            else:
                                logger.error(f"TastyTrade: Connection closed unexpectedly (code: {close_code}, reason: {close_reason})")

                            # All connection closures should trigger recovery
                            await self._handle_connection_loss(f"WebSocket closed (code: {close_code})", current_subscriptions)
                            break

                        except json.JSONDecodeError as e:
                            logger.error(f"TastyTrade: Invalid JSON received: {e}")
                            # JSON errors are not connection issues - continue processing
                            continue

                        except Exception as e:
                            logger.error(f"TastyTrade: Error processing streaming message: {e}")
                            # Check if this is a connection-related error
                            error_str = str(e).lower()
                            if any(keyword in error_str for keyword in ['connection', 'socket', 'network', 'timeout', 'closed']):
                                await self._handle_connection_loss(f"Connection error: {e}", current_subscriptions)
                                break
                            else:
                                # Non-connection error - log and continue
                                streaming_health_manager.record_error(self._connection_id, str(e))
                                continue

                    # If we exit the inner loop, attempt recovery
                    if not self._shutdown_event.is_set() and recovery_attempt < max_recovery_attempts:
                        recovery_attempt += 1
                        logger.info(f"TastyTrade: Attempting recovery #{recovery_attempt}/{max_recovery_attempts}")

                        # Attempt to recover the connection
                        if await self._attempt_connection_recovery(current_subscriptions, recovery_attempt):
                            logger.info("TastyTrade: Recovery successful, resuming streaming")
                            continue
                        else:
                            logger.error(f"TastyTrade: Recovery attempt #{recovery_attempt} failed")

                            # Wait before next attempt with exponential backoff
                            delay = min(5.0 * (2 ** (recovery_attempt - 1)), 60.0)
                            logger.info(f"TastyTrade: Waiting {delay:.1f}s before next recovery attempt")
                            await asyncio.sleep(delay)
                    else:
                        # Max recovery attempts reached or shutdown requested
                        if recovery_attempt >= max_recovery_attempts:
                            logger.error("TastyTrade: Max recovery attempts reached, stopping streaming processor")
                        break

                except asyncio.CancelledError:
                    logger.info("TastyTrade: Streaming processor cancelled")
                    break
                except Exception as e:
                    logger.error(f"TastyTrade: Fatal error in streaming processor: {e}")
                    streaming_health_manager.record_error(self._connection_id, f"Fatal error: {e}")

                    # Wait before attempting recovery
                    await asyncio.sleep(5.0)

        finally:
            # Cancel keepalive task
            if keepalive_task and not keepalive_task.done():
                keepalive_task.cancel()
                try:
                    await keepalive_task
                except asyncio.CancelledError:
                    pass

        logger.info("TastyTrade: Streaming data processor stopped")
    
    async def _periodic_keepalive(self):
        """Send periodic DXLink keepalive messages to maintain connection."""
        logger.info("TastyTrade: Starting periodic keepalive task")
        
        try:
            while not self._shutdown_event.is_set():
                # Wait for 30 seconds between keepalives
                await asyncio.sleep(30.0)
                
                # Only send keepalive if we're connected
                if self.is_connected and self._stream_connection:
                    try:
                        keepalive_msg = {
                            "type": "KEEPALIVE",
                            "channel": 0
                        }
                        await self._stream_connection.send(json.dumps(keepalive_msg))
                        logger.debug("TastyTrade: Periodic DXLink keepalive sent")
                    except Exception as e:
                        logger.warning(f"TastyTrade: Periodic keepalive failed: {e}")
                        # Don't trigger recovery here - let the main loop handle it
                        break
                else:
                    # Not connected, stop keepalive task
                    logger.debug("TastyTrade: Not connected, stopping keepalive task")
                    break
                    
        except asyncio.CancelledError:
            logger.info("TastyTrade: Periodic keepalive task cancelled")
        except Exception as e:
            logger.error(f"TastyTrade: Error in periodic keepalive task: {e}")
        finally:
            logger.info("TastyTrade: Periodic keepalive task stopped")
    
    async def _process_feed_events(self, feed_data: List):
        """Process FEED_DATA events and send to streaming cache or queue."""
        try:
            if not self._streaming_queue and not hasattr(self, '_streaming_cache'):
                logger.warning("TastyTrade: No streaming queue or cache available for feed events")
                return
            
            # Process Quote events
            quote_data = self._process_quote_feed_data(feed_data)
            if quote_data:
                logger.debug(f"TastyTrade: Processing {len(quote_data)} quote updates")
                for symbol, quote in quote_data.items():
                    standard_symbol = self.convert_symbol_to_standard_format(symbol)
                    
                    if standard_symbol in self._quote_requests and not self._quote_requests[standard_symbol].done():
                        self._quote_requests[standard_symbol].set_result(quote)

                    market_data = MarketData(
                        symbol=standard_symbol,
                        data=quote,
                        data_type="quote",
                        timestamp=datetime.now().isoformat()
                    )
                    await self._send_to_cache_or_queue(market_data)
                    logger.debug(f"TastyTrade: Sent quote update for {standard_symbol}: {quote}")
            
            # Process Greeks events
            greeks_data = self._process_greeks_feed_data(feed_data)
            if greeks_data:
                logger.debug(f"TastyTrade: Processing {len(greeks_data)} Greeks updates")
                for symbol, greeks in greeks_data.items():
                    standard_symbol = self.convert_symbol_to_standard_format(symbol)
                    
                    if standard_symbol in self._greeks_requests and not self._greeks_requests[standard_symbol].done():
                        self._greeks_requests[standard_symbol].set_result(greeks)

                    market_data = MarketData(
                        symbol=standard_symbol,
                        data=greeks,
                        data_type="greeks",
                        timestamp=datetime.now().isoformat()
                    )
                    await self._send_to_cache_or_queue(market_data)
                    logger.debug(f"TastyTrade: Sent Greeks update for {standard_symbol}")
                
        except Exception as e:
            logger.error(f"TastyTrade: Error processing feed events: {e}")
    
    def _process_quote_feed_data(self, feed_data: List) -> Dict[str, Dict]:
        """Process FEED_DATA messages and extract Quote events."""
        quote_data = {}
        
        try:
            # DXLink COMPACT format: flat array with Quote data
            # Format: ['Quote', 'SYMBOL', bidPrice, askPrice, bidSize, askSize, 'Quote', ...]
            for item in feed_data:
                if isinstance(item, list):
                    # Parse the flat array - each Quote event has 6 consecutive elements
                    i = 0
                    while i + 5 < len(item):
                        if item[i] == "Quote":
                            symbol = item[i + 1]
                            quote = {
                                "bid": item[i + 2],
                                "ask": item[i + 3],
                                "bid_size": item[i + 4],
                                "ask_size": item[i + 5]
                            }
                            quote_data[symbol] = quote
                            i += 6  # Move to next Quote event (6 fields per event)
                        else:
                            i += 1  # Skip non-Quote data
        except Exception as e:
            logger.error(f"Error processing Quote feed data: {e}")
            logger.error(f"Feed data structure: {feed_data}")
        
        return quote_data

    async def _send_to_cache_or_queue(self, market_data: MarketData):
        """Send market data to cache if available, otherwise to queue."""
        if hasattr(self, '_streaming_cache') and self._streaming_cache:
            await self._streaming_cache.update(market_data)
        elif self._streaming_queue:
            await self._streaming_queue.put(market_data)

    async def _handle_connection_loss(self, reason: str, current_subscriptions: set):
        """Handle connection loss by updating state and preparing for recovery."""
        logger.warning(f"TastyTrade: Connection lost - {reason}")
        
        # Update connection state
        self.is_connected = False
        self._connection_ready.clear()
        streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.FAILED)
        streaming_health_manager.record_error(self._connection_id, reason)
        
        # Store current subscriptions for recovery
        if hasattr(self, '_subscribed_symbols') and self._subscribed_symbols:
            current_subscriptions.update(self._subscribed_symbols)
            logger.info(f"TastyTrade: Stored {len(current_subscriptions)} subscriptions for recovery")
        
        # Clean up connection
        if self._stream_connection:
            try:
                await self._stream_connection.close()
            except:
                pass
            self._stream_connection = None
    
    async def _attempt_connection_recovery(self, current_subscriptions: set, attempt_number: int) -> bool:
        """Attempt to recover the streaming connection and restore subscriptions."""
        try:
            logger.info(f"TastyTrade: Starting connection recovery attempt #{attempt_number}")
            
            # Update health manager
            streaming_health_manager.update_connection_state(self._connection_id, ConnectionState.RECOVERING)
            
            # Attempt to reconnect
            if await self.connect_streaming():
                logger.info("TastyTrade: Connection recovery successful")
                
                # Restore subscriptions if we had any
                if current_subscriptions:
                    logger.info(f"TastyTrade: Restoring {len(current_subscriptions)} subscriptions")
                    try:
                        # Convert set back to list for subscription
                        symbols_list = list(current_subscriptions)
                        
                        # Resubscribe to all symbols (both quotes and Greeks)
                        success = await self.subscribe_to_symbols(symbols_list)
                        
                        if success:
                            logger.info(f"TastyTrade: Successfully restored {len(symbols_list)} subscriptions")
                        else:
                            logger.warning("TastyTrade: Failed to restore some subscriptions")
                            
                    except Exception as e:
                        logger.error(f"TastyTrade: Error restoring subscriptions: {e}")
                        # Don't fail recovery just because subscription restoration failed
                
                return True
            else:
                logger.error("TastyTrade: Connection recovery failed")
                return False
                
        except Exception as e:
            logger.error(f"TastyTrade: Error during connection recovery: {e}")
            streaming_health_manager.record_error(self._connection_id, f"Recovery error: {e}")
            return False

    async def get_positions_enhanced(self) -> Dict[str, Any]:
        """Get enhanced positions grouped by date_acquired (same order timing)."""
        try:
            logger.debug("🔍 TastyTrade: Getting enhanced positions grouped by acquisition date...")

            # 1. Get current positions only (no additional API calls needed)
            current_positions = await self.get_positions()
            if not current_positions:
                logger.info("TastyTrade: No current positions found")
                return {"enhanced": True, "symbol_groups": []}
            
            # 2. Group by date_acquired instead of expensive order chain analysis
            symbol_groups = await self._create_date_based_hierarchical_groups(current_positions)

            logger.debug(f"✅ TastyTrade: Created {len(symbol_groups)} symbol groups using date_acquired grouping")
            return {"enhanced": True, "symbol_groups": symbol_groups}
            
        except Exception as e:
            self._log_error("get_positions_enhanced", e)
            return {"enhanced": True, "symbol_groups": []}

    async def _create_date_based_hierarchical_groups(self, positions: List[Position]) -> List[Dict[str, Any]]:
        """Create hierarchical groups based on date_acquired (same order timing)."""
        try:
            from datetime import datetime
            
            logger.debug("📊 TastyTrade: Creating date-based hierarchical symbol groups")
            
            # Step 1: Group positions by underlying symbol first
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
                        "positions": []
                    }
                
                symbol_groups[underlying]["positions"].append(position)
            
            # Step 2: Within each underlying, group by date_acquired
            result = []
            for underlying, symbol_group in symbol_groups.items():
                strategies = await self._group_by_date_acquired(symbol_group["positions"])
                
                result.append({
                    "symbol": underlying,
                    "asset_class": symbol_group["asset_class"],
                    "strategies": strategies
                })
            
            logger.debug(f"✅ TastyTrade: Created {len(result)} date-based symbol groups")
            return result
            
        except Exception as e:
            self._log_error("_create_date_based_hierarchical_groups", e)
            return []

    async def _group_by_date_acquired(self, positions: List[Position]) -> List[Dict[str, Any]]:
        """Group positions by their date_acquired timestamp."""
        try:
            from datetime import datetime
            
            logger.debug(f"📊 TastyTrade: Grouping {len(positions)} positions by date_acquired")
            
            # Group by date_acquired
            date_groups = {}
            
            for position in positions:
                # Use date_acquired as the grouping key
                date_key = position.date_acquired or "unknown"
                
                # For more precise grouping, we can truncate to minute precision
                # This handles cases where positions from the same order might have slightly different timestamps
                if date_key != "unknown":
                    try:
                        # Parse the date and truncate to minute precision for grouping
                        dt = datetime.fromisoformat(date_key.replace('Z', '+00:00'))
                        # Group by minute precision (same order should be within the same minute)
                        date_key = dt.strftime('%Y-%m-%d %H:%M')
                    except (ValueError, TypeError):
                        logger.warning(f"TastyTrade: Could not parse date_acquired: {date_key}")
                        # Keep original value if parsing fails
                        pass
                
                if date_key not in date_groups:
                    date_groups[date_key] = []
                
                date_groups[date_key].append(position)
            
            logger.debug(f"📊 TastyTrade: Created {len(date_groups)} date-based groups")
            
            # Convert to strategy format
            strategies = []
            for date_key, date_positions in date_groups.items():
                strategy_name = self._detect_strategy_name(date_positions)
                
                # Calculate DTE if options are present
                dte = self._calculate_dte_for_positions(date_positions)
                
                # Calculate strategy totals (static data only)
                strategy_total_qty = sum(pos.qty for pos in date_positions)
                strategy_cost_basis = sum(pos.cost_basis for pos in date_positions)
                
                # Create legs with static broker data
                legs = await self._convert_positions_to_legs(date_positions)
                
                strategy = {
                    "name": strategy_name,
                    "total_qty": strategy_total_qty,
                    "cost_basis": strategy_cost_basis,
                    "dte": dte,
                    "legs": legs,
                    "date_acquired": date_key  # Include the grouping date for reference
                }
                strategies.append(strategy)
            
            logger.debug(f"📊 TastyTrade: Created {len(strategies)} strategies from date grouping")
            return strategies
            
        except Exception as e:
            self._log_error("_group_by_date_acquired", e)
            return []

    def _calculate_dte_for_positions(self, positions: List[Position]) -> Optional[int]:
        """Calculate days to expiration for a group of positions."""
        try:
            from datetime import datetime
            
            # Find the earliest expiration date among option positions
            expiry_dates = []
            for pos in positions:
                if self._is_option_symbol(pos.symbol):
                    parsed = self._parse_option_symbol(pos.symbol)
                    if parsed and parsed.get("expiry"):
                        expiry_dates.append(parsed["expiry"])
            
            if expiry_dates:
                # Use the earliest expiration date
                earliest_expiry = min(expiry_dates)
                try:
                    expiry_date = datetime.strptime(earliest_expiry, "%Y-%m-%d")
                    dte = (expiry_date - datetime.now()).days
                    return dte
                except ValueError:
                    return None
            
            return None
            
        except Exception as e:
            self._log_error("_calculate_dte_for_positions", e)
            return None

    async def _convert_positions_to_legs(self, positions: List[Position]) -> List[Dict[str, Any]]:
        """Convert positions to leg format with static broker data."""
        try:
            legs = []
            for pos in positions:
                legs.append({
                    "symbol": pos.symbol,
                    "qty": pos.qty,
                    "avg_entry_price": pos.avg_entry_price,
                    "cost_basis": pos.cost_basis,
                    "asset_class": pos.asset_class,
                    "lastday_price": pos.lastday_price,
                    "date_acquired": pos.date_acquired  # Include for reference
                })
            
            return legs
            
        except Exception as e:
            self._log_error("_convert_positions_to_legs", e)
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
                    logger.warning(f"TastyTrade: Could not import detectStrategy: {e}")
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

    async def test_credentials(self) -> Dict[str, Any]:
        """
        Test TastyTrade credentials by making a real API call to validate authentication.
        This method attempts to fetch account information, which requires valid credentials.
        """
        try:
            logger.info("🔍 Testing TastyTrade credentials...")
            account_info = await self.get_account()
            if account_info and account_info.account_id:
                logger.info("✅ TastyTrade credentials are valid.")
                return {"success": True, "message": "Successfully connected to TastyTrade."}
            else:
                logger.error("❌ TastyTrade credentials validation failed: No account info returned.")
                return {"success": False, "message": "Invalid credentials or no account info found."}
        except httpx.HTTPStatusError as e:
            if e.response.status_code == 401:
                logger.error(f"❌ TastyTrade API authentication failed (401): {e.response.text}")
                return {"success": False, "message": "Authentication failed: Invalid username or password."}
            else:
                logger.error(f"❌ HTTP error during TastyTrade credential test: {e}")
                return {"success": False, "message": f"API error: {e.response.status_code} - {e.response.text}"}
        except Exception as e:
            logger.error(f"❌ Unexpected error during TastyTrade credential test: {e}")
            return {"success": False, "message": f"An unexpected error occurred: {str(e)}"}
