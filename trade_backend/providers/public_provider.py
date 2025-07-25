import asyncio
import requests
import uuid
from typing import List, Dict, Optional, Any, Set
from datetime import datetime, date, timedelta
import logging
import json
import os

from .base_provider import BaseProvider
from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
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

    # === Market Data Methods ===

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
        token = await self._get_access_token()
        if not token:
            return {}

        url = f"{self.base_url}/userapigateway/marketdata/{self.account_id}/quotes"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        # Prepare instruments list
        instruments = [{"symbol": symbol, "type": "EQUITY"} for symbol in symbols]
        data = {"instruments": instruments}
        
        try:
            response = requests.post(url, headers=headers, json=data)
            response.raise_for_status()
            quotes_data = response.json()
            
            result = {}
            for quote in quotes_data.get("quotes", []):
                if quote.get("outcome") == "SUCCESS":
                    symbol = quote.get("instrument", {}).get("symbol")
                    if symbol:
                        result[symbol] = self._transform_stock_quote(quote)
            
            return result
        except requests.exceptions.RequestException as e:
            self._log_error(f"get_stock_quotes for {symbols}", e)
            return {}

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

    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        """Get options chain for a symbol and expiration."""
        token = await self._get_access_token()
        if not token:
            return []

        url = f"{self.base_url}/userapigateway/marketdata/{self.account_id}/option-chain"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        data = {
            "instrument": {
                "symbol": symbol,
                "type": "EQUITY"
            },
            "expirationDate": expiry
        }
        
        try:
            response = requests.post(url, headers=headers, json=data)
            response.raise_for_status()
            chain_data = response.json()
            
            result = []
            
            # Process calls
            if not option_type or option_type.lower() == "call":
                for call in chain_data.get("calls", []):
                    if call.get("outcome") == "SUCCESS":
                        contract = self._transform_option_contract(call, "call")
                        if contract:
                            result.append(contract)
            
            # Process puts
            if not option_type or option_type.lower() == "put":
                for put in chain_data.get("puts", []):
                    if put.get("outcome") == "SUCCESS":
                        contract = self._transform_option_contract(put, "put")
                        if contract:
                            result.append(contract)
            
            return result
        except requests.exceptions.RequestException as e:
            self._log_error(f"get_options_chain for {symbol} {expiry}", e)
            return []

    async def get_options_chain_basic(self, symbol: str, expiry: str, underlying_price: float = None, strike_count: int = 20) -> List[OptionContract]:
        """Get basic options chain (no Greeks) for fast loading, ATM-focused."""
        # Public API doesn't have separate basic endpoint, use full chain
        contracts = await self.get_options_chain(symbol, expiry)
        
        if not contracts or not underlying_price:
            return contracts
        
        # Filter to ATM strikes
        def distance_from_atm(contract):
            return abs(contract.strike_price - underlying_price)
        
        # Separate calls and puts
        calls = [c for c in contracts if c.type.lower() == 'call']
        puts = [c for c in contracts if c.type.lower() == 'put']
        
        # Sort by distance from ATM
        calls.sort(key=distance_from_atm)
        puts.sort(key=distance_from_atm)
        
        # Take closest strikes
        strikes_per_side = strike_count // 2
        selected_calls = calls[:strikes_per_side]
        selected_puts = puts[:strikes_per_side]
        
        result = selected_calls + selected_puts
        result.sort(key=lambda x: x.strike_price)
        
        return result

    async def get_options_chain_smart(self, symbol: str, expiry: str, underlying_price: float = None, 
                                   atm_range: int = 20, include_greeks: bool = False, 
                                   strikes_only: bool = False) -> List[OptionContract]:
        """Get smart options chain with configurable loading."""
        if strikes_only:
            return await self.get_options_chain_basic(symbol, expiry, underlying_price, atm_range)
        
        contracts = await self.get_options_chain(symbol, expiry)
        
        if include_greeks and contracts:
            # Get Greeks for the contracts
            option_symbols = [c.symbol for c in contracts if c.symbol]
            if option_symbols:
                greeks_data = await self.get_options_greeks_batch(option_symbols)
                
                # Update contracts with Greeks
                for contract in contracts:
                    if contract.symbol in greeks_data:
                        greeks = greeks_data[contract.symbol]
                        contract.delta = greeks.get('delta')
                        contract.gamma = greeks.get('gamma')
                        contract.theta = greeks.get('theta')
                        contract.vega = greeks.get('vega')
                        contract.implied_volatility = greeks.get('implied_volatility')
        
        return contracts

    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """Get Greeks for multiple option symbols in batch."""
        # Public API may have a separate Greeks endpoint
        # For now, return empty dict - Greeks would come from options chain
        self._log_info(f"Greeks batch request for {len(option_symbols)} symbols - not implemented")
        return {}

    async def get_next_market_date(self) -> str:
        """Get the next trading date."""
        # Simple implementation - return next weekday
        today = datetime.now().date()
        next_date = today + timedelta(days=1)
        
        # Skip weekends
        while next_date.weekday() >= 5:  # Saturday = 5, Sunday = 6
            next_date += timedelta(days=1)
        
        return next_date.strftime("%Y-%m-%d")

    # === Account & Portfolio Methods ===

    async def get_account(self) -> Optional[Account]:
        """Get account information including balance and buying power."""
        token = await self._get_access_token()
        if not token:
            return None

        # First get account list to find our account
        accounts_url = f"{self.base_url}/userapigateway/trading/account"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        try:
            response = requests.get(accounts_url, headers=headers)
            response.raise_for_status()
            accounts_data = response.json()
            
            # Find our account
            target_account = None
            for account in accounts_data.get("accounts", []):
                if account.get("accountId") == self.account_id:
                    target_account = account
                    break
            
            if not target_account:
                self._log_error("get_account", Exception(f"Account {self.account_id} not found"))
                return None
            
            # Get portfolio data for buying power info
            portfolio_url = f"{self.base_url}/userapigateway/trading/{self.account_id}/portfolio/v2"
            portfolio_response = requests.get(portfolio_url, headers=headers)
            portfolio_response.raise_for_status()
            portfolio_data = portfolio_response.json()
            
            return self._transform_account(target_account, portfolio_data)
            
        except requests.exceptions.RequestException as e:
            self._log_error("get_account", e)
            return None

    async def get_positions(self) -> List[Position]:
        """Get all current positions."""
        token = await self._get_access_token()
        if not token:
            return []

        url = f"{self.base_url}/userapigateway/trading/{self.account_id}/portfolio/v2"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        try:
            response = requests.get(url, headers=headers)
            response.raise_for_status()
            portfolio_data = response.json()
            
            result = []
            for position_data in portfolio_data.get("positions", []):
                transformed_position = self._transform_position(position_data)
                if transformed_position:
                    result.append(transformed_position)
            
            return result
        except requests.exceptions.RequestException as e:
            self._log_error("get_positions", e)
            return []

    async def get_orders(self, status: str = "open") -> List[Order]:
        """Get orders with optional status filter."""
        token = await self._get_access_token()
        if not token:
            return []

        # Get orders from portfolio endpoint
        url = f"{self.base_url}/userapigateway/trading/{self.account_id}/portfolio/v2"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        try:
            response = requests.get(url, headers=headers)
            response.raise_for_status()
            portfolio_data = response.json()
            
            result = []
            for order_data in portfolio_data.get("orders", []):
                # Apply status filter
                order_status = order_data.get("status", "").lower()
                
                if status == "all":
                    include_order = True
                elif status == "open" or status == "pending":
                    # Both "open" and "pending" should include the same statuses
                    include_order = order_status in ["new", "partially_filled", "pending_replace", "pending_cancel"]
                elif status == "filled":
                    include_order = order_status == "filled"
                elif status == "canceled":
                    include_order = order_status in ["cancelled", "rejected", "expired"]
                else:
                    include_order = order_status == status.lower()
                
                if include_order:
                    transformed_order = self._transform_order(order_data)
                    if transformed_order:
                        result.append(transformed_order)
            
            return result
        except requests.exceptions.RequestException as e:
            self._log_error(f"get_orders with status {status}", e)
            return []

    # === Order Management Methods ===

    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a trading order."""
        token = await self._get_access_token()
        if not token:
            raise Exception("Failed to get access token")

        url = f"{self.base_url}/userapigateway/trading/{self.account_id}/order"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        # Generate unique order ID
        order_id = str(uuid.uuid4())
        
        # Map our order format to Public's format
        order_side, open_close_indicator = self._map_order_side(order_data["side"])
        instrument_type = self._get_instrument_type(order_data["symbol"])
        
        # Map time in force
        time_in_force = self._map_time_in_force(order_data["time_in_force"])
        
        # Build expiration object
        expiration = {"timeInForce": time_in_force}
        
        # If using GTD, we need to provide an expiration date
        # Default to 90 days from now for GTD orders
        if time_in_force == "GTD":
            gtd_date = datetime.now() + timedelta(days=90)
            # Format as ISO 8601 datetime with timezone
            expiration["expirationTime"] = gtd_date.strftime("%Y-%m-%dT00:00:00.000Z")
        
        payload = {
            "orderId": order_id,
            "instrument": {
                "symbol": order_data["symbol"],
                "type": instrument_type
            },
            "orderSide": order_side,
            "orderType": order_data["order_type"].upper(),
            "expiration": expiration,
            "quantity": str(order_data["qty"])
        }
        
        # Add limit price if needed
        if order_data.get("limit_price"):
            payload["limitPrice"] = str(order_data["limit_price"])
        
        # Add stop price if needed
        if order_data.get("stop_price"):
            payload["stopPrice"] = str(order_data["stop_price"])
        
        # Add open/close indicator for options
        if open_close_indicator:
            payload["openCloseIndicator"] = open_close_indicator
        
        # Add amount field (may be required by Public API)
        if order_data.get("limit_price") and order_data.get("qty"):
            amount = float(order_data["limit_price"]) * float(order_data["qty"])
            payload["amount"] = str(amount)
        
        try:
            response = requests.post(url, headers=headers, json=payload)
            response.raise_for_status()
            response_data = response.json()
            
            # Return order object
            return Order(
                id=response_data.get("orderId", order_id),
                symbol=order_data["symbol"],
                asset_class="us_option" if instrument_type == "OPTION" else "us_equity",
                side=order_data["side"],
                order_type=order_data["order_type"],
                qty=order_data["qty"],
                filled_qty=0,
                limit_price=order_data.get("limit_price"),
                stop_price=order_data.get("stop_price"),
                status="submitted",
                time_in_force=order_data["time_in_force"],
                submitted_at=datetime.now().isoformat()
            )
            
        except requests.exceptions.RequestException as e:
            self._log_error("place_order", e)
            raise

    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """Place a multi-leg trading order."""
        token = await self._get_access_token()
        if not token:
            raise Exception("Failed to get access token")

        url = f"{self.base_url}/userapigateway/trading/{self.account_id}/order/multileg"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        # Generate unique order ID
        order_id = str(uuid.uuid4())
        
        # Prepare legs
        legs = []
        for leg in order_data["legs"]:
            order_side, open_close_indicator = self._map_order_side(leg["side"])
            instrument_type = self._get_instrument_type(leg["symbol"])
            
            leg_data = {
                "instrument": {
                    "symbol": leg["symbol"],
                    "type": instrument_type
                },
                "side": order_side,
                "ratioQuantity": int(leg["qty"])
            }
            
            if open_close_indicator:
                leg_data["openCloseIndicator"] = open_close_indicator
            
            legs.append(leg_data)
        
        # Map time in force
        time_in_force = self._map_time_in_force(order_data["time_in_force"])
        
        # Build expiration object
        expiration = {"timeInForce": time_in_force}
        
        # If using GTD, we need to provide an expiration date
        # Default to 90 days from now for GTD orders
        if time_in_force == "GTD":
            gtd_date = datetime.now() + timedelta(days=90)
            # Format as ISO 8601 datetime with timezone
            expiration["expirationTime"] = gtd_date.strftime("%Y-%m-%dT00:00:00.000Z")
        
        payload = {
            "orderId": order_id,
            "quantity": str(order_data.get("qty", 1)),
            "type": order_data["order_type"].upper(),
            "limitPrice": str(order_data["limit_price"]),
            "expiration": expiration,
            "legs": legs
        }
        
        try:
            response = requests.post(url, headers=headers, json=payload)
            response.raise_for_status()
            response_data = response.json()
            
            # Return order object
            return Order(
                id=response_data.get("orderId", order_id),
                symbol="Multi-leg",
                asset_class="us_option",
                side=order_data["order_type"],
                order_type=order_data["order_type"],
                qty=order_data.get("qty", 1),
                filled_qty=0,
                limit_price=order_data["limit_price"],
                status="submitted",
                time_in_force=order_data["time_in_force"],
                submitted_at=datetime.now().isoformat(),
                legs=order_data["legs"]
            )
            
        except requests.exceptions.RequestException as e:
            self._log_error("place_multi_leg_order", e)
            raise

    async def cancel_order(self, order_id: str) -> bool:
        """Cancel an existing order."""
        token = await self._get_access_token()
        if not token:
            return False

        url = f"{self.base_url}/userapigateway/trading/{self.account_id}/order/{order_id}"
        headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }
        
        try:
            response = requests.delete(url, headers=headers)
            response.raise_for_status()
            return True
        except requests.exceptions.RequestException as e:
            self._log_error(f"cancel_order {order_id}", e)
            return False

    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """Get historical OHLCV bars for charting."""
        # Public API may not have historical bars endpoint
        self._log_info(f"Historical bars not available for Public provider")
        return []

    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """Search for symbols matching the query."""
        # Public API may have instruments endpoint for symbol search
        self._log_info(f"Symbol lookup not implemented for Public provider")
        return []

    # === Streaming Methods (Not Supported) ===

    async def connect_streaming(self) -> bool:
        self._log_info("Streaming is not supported by PublicProvider.")
        return False

    async def disconnect_streaming(self) -> bool:
        return True

    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        self._log_info("Streaming is not supported by PublicProvider.")
        return False

    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        return True

    async def get_streaming_data(self) -> Optional[MarketData]:
        return None

    # === Helper Methods ===

    def _transform_stock_quote(self, raw_quote: Dict[str, Any]) -> Optional[StockQuote]:
        """Transform Public stock quote to our standard model."""
        try:
            return StockQuote(
                symbol=raw_quote.get("instrument", {}).get("symbol", ""),
                ask=float(raw_quote.get("ask", 0)) if raw_quote.get("ask") else None,
                bid=float(raw_quote.get("bid", 0)) if raw_quote.get("bid") else None,
                timestamp=raw_quote.get("lastTimestamp", datetime.now().isoformat())
            )
        except Exception as e:
            self._log_error("transform_stock_quote", e)
            return None

    def _transform_option_contract(self, raw_contract: Dict[str, Any], option_type: str) -> Optional[OptionContract]:
        """Transform Public option contract to our standard model."""
        try:
            instrument = raw_contract.get("instrument", {})
            symbol = instrument.get("symbol", "")
            
            # Extract strike price from symbol (Public format needs to be determined)
            strike_price = self._extract_strike_from_symbol(symbol)
            
            return OptionContract(
                symbol=symbol,
                underlying_symbol=self._extract_underlying_from_symbol(symbol),
                expiration_date=self._extract_expiry_from_symbol(symbol),
                strike_price=strike_price,
                type=option_type,
                bid=float(raw_contract.get("bid", 0)) if raw_contract.get("bid") else None,
                ask=float(raw_contract.get("ask", 0)) if raw_contract.get("ask") else None,
                close_price=float(raw_contract.get("last", 0)) if raw_contract.get("last") else None,
                volume=int(raw_contract.get("volume", 0)) if raw_contract.get("volume") else None,
                open_interest=int(raw_contract.get("openInterest", 0)) if raw_contract.get("openInterest") else None,
                # Greeks would come from separate endpoint
                implied_volatility=None,
                delta=None,
                gamma=None,
                theta=None,
                vega=None,
            )
        except Exception as e:
            self._log_error("transform_option_contract", e)
            return None

    def _transform_position(self, raw_position: Dict[str, Any]) -> Optional[Position]:
        """Transform Public position to our standard model."""
        try:
            instrument = raw_position.get("instrument", {})
            symbol = instrument.get("symbol", "")
            quantity = float(raw_position.get("quantity", 0))
            
            # Determine asset class
            asset_class = "us_equity"
            if instrument.get("type") == "OPTION":
                asset_class = "us_option"
            
            # Extract cost basis info
            cost_basis_info = raw_position.get("costBasis", {})
            total_cost = float(cost_basis_info.get("totalCost", 0)) if cost_basis_info.get("totalCost") else 0
            unit_cost = float(cost_basis_info.get("unitCost", 0)) if cost_basis_info.get("unitCost") else 0
            
            # Extract current value and P&L
            current_value = float(raw_position.get("currentValue", 0)) if raw_position.get("currentValue") else 0
            
            # Extract gain info
            instrument_gain = raw_position.get("instrumentGain", {})
            gain_value = float(instrument_gain.get("gainValue", 0)) if instrument_gain.get("gainValue") else 0
            
            # Extract last price
            last_price_info = raw_position.get("lastPrice", {})
            current_price = float(last_price_info.get("lastPrice", 0)) if last_price_info.get("lastPrice") else 0
            
            return Position(
                symbol=symbol,
                qty=quantity,
                side="long" if quantity > 0 else "short",
                market_value=current_value,
                cost_basis=total_cost,
                unrealized_pl=gain_value,
                unrealized_plpc=0,  # Would need to calculate
                current_price=current_price,
                avg_entry_price=unit_cost,
                asset_class=asset_class
            )
        except Exception as e:
            self._log_error("transform_position", e)
            return None

    def _transform_order(self, raw_order: Dict[str, Any]) -> Optional[Order]:
        """Transform Public order to our standard model."""
        try:
            instrument = raw_order.get("instrument", {})
            symbol = instrument.get("symbol", "")
            
            # Map status
            status_map = {
                "NEW": "submitted",
                "FILLED": "filled",
                "CANCELLED": "canceled",
                "PARTIALLY_FILLED": "partially_filled",
                "REJECTED": "rejected",
                "EXPIRED": "expired"
            }
            
            status = status_map.get(raw_order.get("status", "").upper(), "unknown")
            
            # Handle legs for multi-leg orders
            legs = None
            if raw_order.get("legs"):
                legs = []
                for leg_data in raw_order["legs"]:
                    legs.append({
                        "symbol": leg_data.get("instrument", {}).get("symbol"),
                        "side": self._map_public_side_to_our_format(
                            leg_data.get("side"), 
                            leg_data.get("openCloseIndicator")
                        ),
                        "qty": int(leg_data.get("ratioQuantity", 0))
                    })
            
            return Order(
                id=raw_order.get("orderId", ""),
                symbol=symbol or "Multi-leg",
                asset_class="us_option" if instrument.get("type") == "OPTION" else "us_equity",
                side=self._map_public_side_to_our_format(
                    raw_order.get("side"), 
                    raw_order.get("openCloseIndicator")
                ),
                order_type=raw_order.get("type", "").lower(),
                qty=float(raw_order.get("quantity", 0)),
                filled_qty=float(raw_order.get("filledQuantity", 0)) if raw_order.get("filledQuantity") else 0,
                limit_price=float(raw_order.get("limitPrice", 0)) if raw_order.get("limitPrice") else None,
                stop_price=float(raw_order.get("stopPrice", 0)) if raw_order.get("stopPrice") else None,
                avg_fill_price=float(raw_order.get("averagePrice", 0)) if raw_order.get("averagePrice") else None,
                status=status,
                time_in_force=raw_order.get("expiration", {}).get("timeInForce", "").lower(),
                submitted_at=raw_order.get("createdAt", ""),
                filled_at=raw_order.get("closedAt"),
                legs=legs
            )
        except Exception as e:
            self._log_error("transform_order", e)
            return None

    def _transform_account(self, account_data: Dict[str, Any], portfolio_data: Dict[str, Any]) -> Optional[Account]:
        """Transform Public account data to our standard model."""
        try:
            buying_power_info = portfolio_data.get("buyingPower", {})
            
            # Parse options level from string format like "LEVEL_4" to integer
            options_level = account_data.get("optionsLevel")
            options_approved_level = None
            if options_level and isinstance(options_level, str):
                try:
                    # Extract number from "LEVEL_X" format
                    if options_level.startswith("LEVEL_"):
                        options_approved_level = int(options_level.split("_")[1])
                    else:
                        options_approved_level = int(options_level)
                except (ValueError, IndexError):
                    options_approved_level = None
            elif isinstance(options_level, int):
                options_approved_level = options_level
            
            # Calculate portfolio value and equity from available data
            cash = float(buying_power_info.get("cashOnlyBuyingPower", 0)) if buying_power_info.get("cashOnlyBuyingPower") else 0
            
            # Calculate total position value from positions
            total_position_value = 0
            long_market_value = 0
            short_market_value = 0
            
            positions = portfolio_data.get("positions", [])
            for position in positions:
                current_value = float(position.get("currentValue", 0)) if position.get("currentValue") else 0
                quantity = float(position.get("quantity", 0)) if position.get("quantity") else 0
                
                total_position_value += current_value
                
                if quantity > 0:
                    long_market_value += current_value
                elif quantity < 0:
                    short_market_value += abs(current_value)
            
            # Portfolio value = cash + total position value
            portfolio_value = cash + total_position_value
            
            # Equity is typically the same as portfolio value for cash accounts
            # For margin accounts, it would be portfolio_value - borrowed amount
            equity = portfolio_value
            
            # If we have specific equity data from Public, use that instead
            if portfolio_data.get("equity") and not isinstance(portfolio_data["equity"], list):
                try:
                    equity = float(portfolio_data["equity"])
                except (ValueError, TypeError):
                    pass  # Keep calculated equity
            elif portfolio_data.get("totalValue") and not isinstance(portfolio_data["totalValue"], list):
                try:
                    portfolio_value = float(portfolio_data["totalValue"])
                    equity = portfolio_value
                except (ValueError, TypeError):
                    pass  # Keep calculated values
            
            return Account(
                account_id=account_data.get("accountId", ""),
                account_number=account_data.get("accountId", ""),
                status="active",  # Public doesn't provide status in account data
                currency="USD",
                buying_power=float(buying_power_info.get("buyingPower", 0)) if buying_power_info.get("buyingPower") else None,
                cash=cash,
                portfolio_value=portfolio_value,
                equity=equity,
                day_trading_buying_power=float(buying_power_info.get("buyingPower", 0)) if buying_power_info.get("buyingPower") else None,
                regt_buying_power=float(buying_power_info.get("buyingPower", 0)) if buying_power_info.get("buyingPower") else None,
                options_buying_power=float(buying_power_info.get("optionsBuyingPower", 0)) if buying_power_info.get("optionsBuyingPower") else None,
                pattern_day_trader=False,  # Would need to determine
                trading_blocked=None,
                transfers_blocked=None,
                account_blocked=None,
                created_at=None,
                multiplier=None,
                long_market_value=long_market_value if long_market_value > 0 else None,
                short_market_value=short_market_value if short_market_value > 0 else None,
                initial_margin=None,
                maintenance_margin=None,
                daytrade_count=None,
                options_approved_level=options_approved_level,
                options_trading_level=None
            )
        except Exception as e:
            self._log_error("transform_account", e)
            return None

    def _map_order_side(self, side: str) -> tuple:
        """Map our side format to Public's format."""
        side_map = {
            "buy_to_open": ("BUY", "OPEN"),
            "buy_to_close": ("BUY", "CLOSE"), 
            "sell_to_open": ("SELL", "OPEN"),
            "sell_to_close": ("SELL", "CLOSE"),
            "buy": ("BUY", None),  # for stocks
            "sell": ("SELL", None)  # for stocks
        }
        return side_map.get(side.lower(), ("BUY", None))

    def _map_public_side_to_our_format(self, side: str, open_close_indicator: Optional[str] = None) -> str:
        """Map Public's side format back to our format."""
        if not side:
            return "buy"
        
        side = side.upper()
        if open_close_indicator:
            indicator = open_close_indicator.upper()
            if side == "BUY" and indicator == "OPEN":
                return "buy_to_open"
            elif side == "BUY" and indicator == "CLOSE":
                return "buy_to_close"
            elif side == "SELL" and indicator == "OPEN":
                return "sell_to_open"
            elif side == "SELL" and indicator == "CLOSE":
                return "sell_to_close"
        
        # Fallback for stocks or when no indicator
        return "buy" if side == "BUY" else "sell"

    def _map_time_in_force(self, time_in_force: str) -> str:
        """Map our time in force to Public's format."""
        tif_map = {
            "day": "DAY",
            "gtc": "GTD",  # Public uses GTD (Good Till Date) instead of GTC
            "ioc": "IOC",
            "fok": "FOK"
        }
        return tif_map.get(time_in_force.lower(), "DAY")

    def _get_instrument_type(self, symbol: str) -> str:
        """Determine if symbol is EQUITY or OPTION."""
        return "OPTION" if self._is_option_symbol(symbol) else "EQUITY"

    def _is_option_symbol(self, symbol: str) -> bool:
        """Check if symbol is an option symbol."""
        # Basic check - option symbols are typically longer and contain specific patterns
        return len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:])

    def _extract_strike_from_symbol(self, symbol: str) -> float:
        """Extract strike price from option symbol."""
        try:
            # This will need to be adjusted based on Public's actual option symbol format
            # For now, assume similar to standard format: AAPL250117C00150000
            if len(symbol) >= 15:
                strike_part = symbol[-8:]  # Last 8 digits
                return float(strike_part) / 1000  # Divide by 1000 to get actual strike
            return 0.0
        except Exception as e:
            self._log_error(f"extract_strike_from_symbol {symbol}", e)
            return 0.0

    def _extract_underlying_from_symbol(self, symbol: str) -> str:
        """Extract underlying symbol from option symbol."""
        try:
            # This will need to be adjusted based on Public's actual option symbol format
            # For now, assume first part before digits
            for i, char in enumerate(symbol):
                if char.isdigit():
                    return symbol[:i]
            return symbol
        except Exception as e:
            self._log_error(f"extract_underlying_from_symbol {symbol}", e)
            return symbol

    def _extract_expiry_from_symbol(self, symbol: str) -> str:
        """Extract expiration date from option symbol."""
        try:
            # This will need to be adjusted based on Public's actual option symbol format
            # For now, assume format like AAPL250117C00150000 where 250117 = Jan 17, 2025
            if len(symbol) >= 12:
                for i, char in enumerate(symbol):
                    if char.isdigit():
                        date_part = symbol[i:i+6]  # 6 digits for YYMMDD
                        if len(date_part) == 6:
                            year = 2000 + int(date_part[:2])
                            month = int(date_part[2:4])
                            day = int(date_part[4:6])
                            return f"{year}-{month:02d}-{day:02d}"
                        break
            return ""
        except Exception as e:
            self._log_error(f"extract_expiry_from_symbol {symbol}", e)
            return ""
