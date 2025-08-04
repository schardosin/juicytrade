from pydantic import BaseModel, Field, validator
from typing import List, Optional, Dict, Any
from datetime import datetime

class WatchlistBase(BaseModel):
    """Base watchlist model with common fields."""
    name: str = Field(..., min_length=1, max_length=100, description="Watchlist display name")
    symbols: List[str] = Field(default_factory=list, description="List of symbols in the watchlist")

class CreateWatchlistRequest(WatchlistBase):
    """Request model for creating a new watchlist."""
    
    @validator('name')
    def validate_name(cls, v):
        if not v or not v.strip():
            raise ValueError('Watchlist name cannot be empty')
        return v.strip()
    
    @validator('symbols')
    def validate_symbols(cls, v):
        if not v:
            return []
        
        # Clean and validate symbols
        clean_symbols = []
        for symbol in v:
            if isinstance(symbol, str) and symbol.strip():
                clean_symbol = symbol.strip().upper()
                if clean_symbol.isalnum() and 1 <= len(clean_symbol) <= 10:
                    clean_symbols.append(clean_symbol)
        
        return clean_symbols

class UpdateWatchlistRequest(BaseModel):
    """Request model for updating a watchlist."""
    name: Optional[str] = Field(None, min_length=1, max_length=100, description="New watchlist name")
    symbols: Optional[List[str]] = Field(None, description="New symbol list")
    
    @validator('name')
    def validate_name(cls, v):
        if v is not None and (not v or not v.strip()):
            raise ValueError('Watchlist name cannot be empty')
        return v.strip() if v else v
    
    @validator('symbols')
    def validate_symbols(cls, v):
        if v is None:
            return v
        
        # Clean and validate symbols
        clean_symbols = []
        for symbol in v:
            if isinstance(symbol, str) and symbol.strip():
                clean_symbol = symbol.strip().upper()
                if clean_symbol.isalnum() and 1 <= len(clean_symbol) <= 10:
                    clean_symbols.append(clean_symbol)
        
        return clean_symbols

class AddSymbolRequest(BaseModel):
    """Request model for adding a symbol to a watchlist."""
    symbol: str = Field(..., min_length=1, max_length=10, description="Symbol to add")
    
    @validator('symbol')
    def validate_symbol(cls, v):
        if not v or not v.strip():
            raise ValueError('Symbol cannot be empty')
        
        clean_symbol = v.strip().upper()
        if not clean_symbol.isalnum():
            raise ValueError('Symbol must be alphanumeric')
        
        if len(clean_symbol) < 1 or len(clean_symbol) > 10:
            raise ValueError('Symbol must be 1-10 characters long')
        
        return clean_symbol

class SetActiveWatchlistRequest(BaseModel):
    """Request model for setting the active watchlist."""
    watchlist_id: str = Field(..., min_length=1, description="ID of the watchlist to make active")

class WatchlistResponse(BaseModel):
    """Response model for a single watchlist."""
    id: str
    name: str
    symbols: List[str]
    created_at: str
    updated_at: str
    is_active: Optional[bool] = False

class WatchlistsResponse(BaseModel):
    """Response model for multiple watchlists."""
    watchlists: Dict[str, Dict[str, Any]]
    active_watchlist: str
    total_watchlists: int
    version: str

class WatchlistSymbolsResponse(BaseModel):
    """Response model for watchlist symbols with live data."""
    watchlist_id: str
    watchlist_name: str
    symbols: List[str]
    total_symbols: int
    last_updated: str

class SearchWatchlistsRequest(BaseModel):
    """Request model for searching watchlists."""
    query: str = Field(..., min_length=1, description="Search query")
    
    @validator('query')
    def validate_query(cls, v):
        if not v or not v.strip():
            raise ValueError('Search query cannot be empty')
        return v.strip()
