from pydantic import BaseModel
from typing import Optional, List, Dict, Any
from datetime import datetime

class StockQuote(BaseModel):
    """Standardized stock quote model."""
    symbol: str
    ask: Optional[float] = None
    bid: Optional[float] = None
    last: Optional[float] = None
    timestamp: str

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

class Position(BaseModel):
    """Standardized position model."""
    symbol: str
    qty: float
    side: str  # "long" or "short"
    market_value: float
    cost_basis: float
    unrealized_pl: float
    unrealized_plpc: Optional[float] = None
    current_price: float
    avg_entry_price: float
    asset_class: str  # "us_equity", "us_option", etc.
    lastday_price: Optional[float] = None  # Previous day's closing price for daily P/L calculation
    date_acquired: Optional[str] = None  # Date when position was acquired (for 0DTE detection)
    
    # Option-specific fields
    underlying_symbol: Optional[str] = None
    option_type: Optional[str] = None  # "call" or "put"
    strike_price: Optional[float] = None
    expiry_date: Optional[str] = None

class Order(BaseModel):
    """Standardized order model."""
    id: str
    symbol: str
    asset_class: str
    side: str  # "buy" or "sell"
    action: Optional[str] = None
    order_type: str  # "market", "limit", etc.
    qty: float
    filled_qty: float
    limit_price: Optional[float] = None
    stop_price: Optional[float] = None
    avg_fill_price: Optional[float] = None
    status: str  # "new", "filled", "canceled", etc.
    time_in_force: str  # "day", "gtc", etc.
    submitted_at: str
    filled_at: Optional[str] = None
    
    # Multi-leg order support
    legs: Optional[List[Dict[str, Any]]] = None

class ExpirationDate(BaseModel):
    """Standardized expiration date model."""
    date: str
    days_to_expiry: Optional[int] = None

class MarketData(BaseModel):
    """Standardized market data model for streaming."""
    symbol: str
    data_type: str  # "quote", "trade", etc.
    timestamp: str
    timestamp_ms: Optional[int] = None  # Unix timestamp in milliseconds for easy comparison
    data: Dict[str, Any]
    
    def __init__(self, **data):
        super().__init__(**data)
        # Auto-generate timestamp_ms if not provided
        if self.timestamp_ms is None:
            import time
            self.timestamp_ms = int(time.time() * 1000)
    
    @property
    def age_seconds(self) -> float:
        """Get age of this data in seconds."""
        if self.timestamp_ms:
            import time
            return (time.time() * 1000 - self.timestamp_ms) / 1000
        return 0
    
    @property
    def is_fresh(self, max_age_seconds: float = 30.0) -> bool:
        """Check if data is fresh (not older than max_age_seconds)."""
        return self.age_seconds <= max_age_seconds

class SymbolSearchResult(BaseModel):
    """Standardized symbol search result model."""
    symbol: str
    description: str
    exchange: str
    type: str  # "stock", "etf", "index", "option"

class Account(BaseModel):
    """Standardized account information model."""
    account_id: str
    account_number: Optional[str] = None
    status: str
    currency: str = "USD"
    buying_power: Optional[float] = None
    cash: Optional[float] = None
    portfolio_value: Optional[float] = None
    equity: Optional[float] = None
    day_trading_buying_power: Optional[float] = None
    regt_buying_power: Optional[float] = None
    options_buying_power: Optional[float] = None
    pattern_day_trader: Optional[bool] = None
    trading_blocked: Optional[bool] = None
    transfers_blocked: Optional[bool] = None
    account_blocked: Optional[bool] = None
    created_at: Optional[str] = None
    multiplier: Optional[str] = None
    long_market_value: Optional[float] = None
    short_market_value: Optional[float] = None
    initial_margin: Optional[float] = None
    maintenance_margin: Optional[float] = None
    daytrade_count: Optional[int] = None
    options_approved_level: Optional[int] = None
    options_trading_level: Optional[int] = None

class ApiResponse(BaseModel):
    """Standardized API response wrapper."""
    success: bool
    data: Optional[Any] = None
    error: Optional[str] = None
    message: Optional[str] = None
    timestamp: str = datetime.now().isoformat()

# Request models for API endpoints
class SymbolRequest(BaseModel):
    symbols: List[str]

class OrderRequest(BaseModel):
    symbol: str
    side: str
    qty: float
    order_type: str = "market"
    time_in_force: str = "day"
    limit_price: Optional[float] = None
    stop_price: Optional[float] = None

class MultiLegOrderRequest(BaseModel):
    legs: List[Dict[str, Any]]
    action: Optional[str] = None
    qty: int = 1
    order_type: str = "limit"
    time_in_force: str = "day"
    limit_price: Optional[float] = None

class PositionGroup(BaseModel):
    """Position group model for order chain grouping."""
    id: str  # Order chain ID or group identifier
    symbol: str  # Underlying symbol (e.g., SPY)
    strategy: str  # Strategy name (e.g., "Iron Condor", "Call Spread")
    asset_class: str  # "options" or "stocks"
    total_qty: float
    total_cost_basis: float
    total_market_value: float
    total_unrealized_pl: float
    total_unrealized_plpc: float
    pl_day: float  # P&L for the day
    pl_open: float  # P&L since position opened
    legs: List[Position]  # Individual position legs
    order_date: Optional[str] = None
    expiration_date: Optional[str] = None  # For options
    dte: Optional[int] = None  # Days to expiration
    order_chain_id: Optional[str] = None  # Original order ID that created this group
    
class HistoricalTrade(BaseModel):
    """Historical trade data from broker history API."""
    id: str
    symbol: str
    side: str  # "buy" or "sell"
    qty: float
    price: float
    date: str
    order_id: Optional[str] = None
    commission: Optional[float] = None
    asset_class: Optional[str] = None

# Provider Instance Management Models
class CreateProviderInstanceRequest(BaseModel):
    """Request model for creating a new provider instance."""
    provider_type: str
    account_type: str
    display_name: str
    credentials: Dict[str, str]

class UpdateProviderInstanceRequest(BaseModel):
    """Request model for updating a provider instance."""
    display_name: Optional[str] = None
    credentials: Optional[Dict[str, str]] = None

class ProviderInstanceResponse(BaseModel):
    """Response model for provider instance data."""
    instance_id: str
    active: bool
    provider_type: str
    account_type: str
    display_name: str
    created_at: int
    updated_at: int
    # Note: credentials are not included in response for security

class TestProviderConnectionRequest(BaseModel):
    """Request model for testing provider connection."""
    provider_type: str
    account_type: str
    credentials: Dict[str, str]

class TestProviderConnectionResponse(BaseModel):
    """Response model for provider connection test."""
    success: bool
    message: str
    details: Optional[Dict[str, Any]] = None
