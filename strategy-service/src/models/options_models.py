"""
Options Framework Models

This module extends the existing models.py with options-specific helper classes
that complement the existing OptionContract and MultiLegOrderRequest models.
"""

from pydantic import BaseModel
from typing import List, Optional, Dict, Any
from datetime import datetime, date
import re


class OptionContract(BaseModel):
    """Standardized option contract model."""
    symbol: str
    underlying_symbol: str
    expiration_date: str
    strike_price: float
    type: str  # "call" or "put"
    root_symbol: Optional[str] = None  # Root symbol from provider (e.g., SPXW, SPY)
    bid: Optional[float] = None
    ask: Optional[float] = None
    close_price: Optional[float] = None
    volume: Optional[int] = None
    open_interest: Optional[int] = None
    implied_volatility: Optional[float] = None
    delta: Optional[float] = None
    gamma: Optional[float] = None
    theta: Optional[float] = None
    vega: Optional[float] = None


class OptionsChain(BaseModel):
    """
    Container for options chain data at a specific point in time.
    
    This complements the existing OptionContract model by providing
    a structured way to work with complete options chains.
    """
    underlying: str              # "SPXW"
    expiration: str             # "2013-04-05"
    timestamp: datetime         # Point in time for this chain
    contracts: List[OptionContract]
    
    def get_calls(self) -> List[OptionContract]:
        """Get all call options from the chain."""
        return [c for c in self.contracts if c.type.lower() == "call"]
    
    def get_puts(self) -> List[OptionContract]:
        """Get all put options from the chain."""
        return [c for c in self.contracts if c.type.lower() == "put"]
    
    def get_strikes_range(self, min_strike: float, max_strike: float) -> List[OptionContract]:
        """Get contracts within a specific strike range."""
        return [c for c in self.contracts if min_strike <= c.strike_price <= max_strike]
    
    def get_atm_contracts(self, underlying_price: float, strike_count: int = 10) -> List[OptionContract]:
        """Get contracts around the at-the-money strike."""
        # Sort by distance from underlying price
        sorted_contracts = sorted(
            self.contracts, 
            key=lambda c: abs(c.strike_price - underlying_price)
        )
        return sorted_contracts[:strike_count]
    
    def get_contract_by_strike(self, strike: float, option_type: str) -> Optional[OptionContract]:
        """Get specific contract by strike and type."""
        for contract in self.contracts:
            if (contract.strike_price == strike and 
                contract.type.lower() == option_type.lower()):
                return contract
        return None


class OptionsLeg(BaseModel):
    """
    Individual leg of a multi-leg options order.
    
    This model is designed to work seamlessly with the existing
    MultiLegOrderRequest structure used by your providers.
    """
    contract: OptionContract
    action: str      # "buy", "sell", "buy_to_open", "sell_to_close"
    quantity: int
    
    def to_provider_leg(self) -> Dict[str, Any]:
        """
        Convert to the leg format expected by existing MultiLegOrderRequest.
        
        This ensures compatibility with your existing provider infrastructure.
        """
        return {
            "symbol": self.contract.symbol,
            "side": self.action.lower(),
            "qty": self.quantity,
            "asset_class": "us_option"
        }


class OptionsOrder(BaseModel):
    """
    Multi-leg options order that integrates with existing order infrastructure.
    
    This model bridges strategy-level options logic with the existing
    MultiLegOrderRequest system used by your providers.
    """
    legs: List[OptionsLeg]
    order_type: str = "market"  # "market", "limit", "net_debit", "net_credit"
    limit_price: Optional[float] = None
    time_in_force: str = "day"
    
    def calculate_net_debit_credit(self) -> float:
        """
        Calculate net cost (positive = debit, negative = credit).
        
        Uses close_price for backtesting, bid/ask for live trading.
        """
        net = 0.0
        for leg in self.legs:
            # Determine price to use (backtest vs live)
            if leg.contract.close_price is not None:
                # Backtesting: use close price
                price = leg.contract.close_price
            elif leg.contract.bid and leg.contract.ask:
                # Live trading: use mid price
                price = (leg.contract.bid + leg.contract.ask) / 2
            elif leg.contract.bid:
                price = leg.contract.bid
            elif leg.contract.ask:
                price = leg.contract.ask
            else:
                price = 0.0
            
            # Calculate net based on action
            if leg.action.lower() in ["buy", "buy_to_open"]:
                net += price * leg.quantity * 100  # Options multiplier
            else:
                net -= price * leg.quantity * 100
                
        return net
    
    def to_multi_leg_request(self) -> Dict[str, Any]:
        """
        Convert to existing MultiLegOrderRequest format.
        
        This ensures seamless integration with your existing provider system.
        """
        return {
            "legs": [leg.to_provider_leg() for leg in self.legs],
            "order_type": self.order_type,
            "time_in_force": self.time_in_force,
            "limit_price": self.limit_price,
            "qty": 1  # Multi-leg orders typically use qty=1
        }


class OptionsSymbolParser:
    """
    Parser for options symbols in your data format: "SPXW  130405C01560000"
    
    This handles the specific format found in your parquet files and provides
    conversion utilities for working with different provider formats.
    """
    
    @staticmethod
    def parse_symbol(symbol: str) -> Dict[str, Any]:
        """
        Parse options symbol into components.
        
        Format: "SPXW  130405C01560000"
        - SPXW: underlying
        - 130405: expiration (YYMMDD)
        - C: option type (C=Call, P=Put)
        - 01560000: strike price (multiply by 0.001)
        
        Args:
            symbol: Options symbol string
            
        Returns:
            Dictionary with parsed components
        """
        # Remove extra spaces and normalize
        symbol = symbol.strip()
        
        # Pattern: underlying + spaces + YYMMDD + C/P + 8-digit strike
        pattern = r'^([A-Z]+)\s+(\d{6})([CP])(\d{8})$'
        match = re.match(pattern, symbol)
        
        if not match:
            raise ValueError(f"Invalid options symbol format: {symbol}")
        
        underlying, date_str, option_type, strike_str = match.groups()
        
        # Parse expiration date (YYMMDD -> YYYY-MM-DD)
        year = int("20" + date_str[:2])  # Assume 20XX
        month = int(date_str[2:4])
        day = int(date_str[4:6])
        expiration = date(year, month, day)
        
        # Parse strike price (8 digits, divide by 1000)
        strike = int(strike_str) / 1000.0
        
        # Convert option type
        option_type_full = "call" if option_type == "C" else "put"
        
        return {
            "underlying": underlying,
            "expiration": expiration.strftime("%Y-%m-%d"),
            "option_type": option_type_full,
            "strike": strike,
            "symbol": symbol
        }
    
    @staticmethod
    def create_option_contract(symbol: str, timestamp: datetime, 
                             close_price: float, volume: int = 0,
                             bid_price: Optional[float] = None,
                             ask_price: Optional[float] = None,
                             bid_size: Optional[int] = None,
                             ask_size: Optional[int] = None) -> OptionContract:
        """
        Create OptionContract from parquet data (OHLCV or CBBO).
        
        This converts your parquet data format into the existing OptionContract model.
        Supports both legacy OHLCV data and new CBBO data with bid/ask spreads.
        """
        parsed = OptionsSymbolParser.parse_symbol(symbol)
        
        return OptionContract(
            symbol=symbol,
            underlying_symbol=parsed["underlying"],
            expiration_date=parsed["expiration"],
            strike_price=parsed["strike"],
            type=parsed["option_type"],
            close_price=close_price,
            volume=volume,
            # CBBO data: bid/ask prices and sizes
            bid=bid_price,
            ask=ask_price,
            bid_size=bid_size,
            ask_size=ask_size
        )


class OptionsPosition(BaseModel):
    """
    Multi-leg options position tracking.
    
    This extends the existing Position model concept to handle complex
    multi-leg options positions with proper P&L calculation.
    """
    legs: List[Dict[str, Any]]  # Individual position legs
    strategy_type: str = "CUSTOM"  # "VERTICAL", "IRON_CONDOR", etc.
    underlying: str
    entry_timestamp: datetime
    net_entry_cost: float         # Total debit/credit at entry
    current_net_value: float = 0.0      # Current market value
    unrealized_pnl: float = 0.0
    
    def update_current_prices(self, chain: OptionsChain):
        """
        Update all leg prices and recalculate P&L.
        
        This method updates the position value based on current market prices
        from the options chain.
        """
        total_value = 0.0
        
        for leg in self.legs:
            # Find current price for this contract
            current_contract = chain.get_contract_by_strike(
                leg["strike_price"], 
                leg["option_type"]
            )
            
            if current_contract:
                # Use appropriate price based on data availability
                if current_contract.close_price is not None:
                    current_price = current_contract.close_price
                elif current_contract.bid and current_contract.ask:
                    current_price = (current_contract.bid + current_contract.ask) / 2
                else:
                    current_price = 0.0
                
                # Calculate position value
                if leg["quantity"] > 0:  # Long position
                    total_value += current_price * leg["quantity"] * 100
                else:  # Short position
                    total_value -= current_price * abs(leg["quantity"]) * 100
        
        self.current_net_value = total_value
        self.unrealized_pnl = self.current_net_value - self.net_entry_cost
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return {
            "legs": self.legs,
            "strategy_type": self.strategy_type,
            "underlying": self.underlying,
            "entry_timestamp": self.entry_timestamp.isoformat(),
            "net_entry_cost": self.net_entry_cost,
            "current_net_value": self.current_net_value,
            "unrealized_pnl": self.unrealized_pnl
        }
