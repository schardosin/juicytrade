from pydantic import BaseModel
from typing import Optional, List, Dict, Any
from datetime import datetime

class StockQuote(BaseModel):
    """Standardized stock quote model."""
    symbol: str
    ask: Optional[float] = None
    bid: Optional[float] = None
    timestamp: str

class OptionContract(BaseModel):
    """Standardized option contract model."""
    symbol: str
    underlying_symbol: str
    expiration_date: str
    strike_price: float
    type: str  # "call" or "put"
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
    data: Dict[str, Any]

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
    order_type: str = "limit"
    time_in_force: str = "day"
    limit_price: Optional[float] = None
