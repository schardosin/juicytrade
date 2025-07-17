from abc import ABC, abstractmethod
from typing import List, Dict, Optional, Any, Set
from datetime import datetime
import asyncio
import logging

from ..models import (
    StockQuote, OptionContract, Position, Order, 
    ExpirationDate, MarketData, ApiResponse, SymbolSearchResult, Account
)

logger = logging.getLogger(__name__)

class BaseProvider(ABC):
    """
    Abstract Base Class for trading data providers.
    
    This interface defines the contract that all trading providers must implement.
    It ensures consistency across different providers (Alpaca, Public, etc.).
    """
    
    def __init__(self, name: str):
        self.name = name
        self.is_connected = False
        self._subscribed_symbols: Set[str] = set()
        self._streaming_queue: Optional[asyncio.Queue] = None
        
    # === Market Data Methods ===
    
    @abstractmethod
    async def get_stock_quote(self, symbol: str) -> Optional[StockQuote]:
        """
        Get the latest stock quote for a symbol.
        
        Args:
            symbol: Stock symbol (e.g., "SPY")
            
        Returns:
            StockQuote object or None if not found
        """
        pass
    
    @abstractmethod
    async def get_stock_quotes(self, symbols: List[str]) -> Dict[str, StockQuote]:
        """
        Get stock quotes for multiple symbols.
        
        Args:
            symbols: List of stock symbols
            
        Returns:
            Dictionary mapping symbols to StockQuote objects
        """
        pass
    
    @abstractmethod
    async def get_expiration_dates(self, symbol: str) -> List[str]:
        """
        Get available expiration dates for options on a symbol.
        
        Args:
            symbol: Underlying symbol (e.g., "SPY")
            
        Returns:
            List of expiration dates in YYYY-MM-DD format
        """
        pass
    
    @abstractmethod
    async def get_options_chain(self, symbol: str, expiry: str, option_type: Optional[str] = None) -> List[OptionContract]:
        """
        Get options chain for a symbol and expiration date.
        
        Args:
            symbol: Underlying symbol
            expiry: Expiration date in YYYY-MM-DD format
            option_type: Optional filter for "call" or "put"
            
        Returns:
            List of OptionContract objects
        """
        pass
    
    @abstractmethod
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
        pass
    
    @abstractmethod
    async def get_options_greeks_batch(self, option_symbols: List[str]) -> Dict[str, Dict]:
        """
        Get Greeks for multiple option symbols in batch.
        
        Args:
            option_symbols: List of option symbols
            
        Returns:
            Dictionary mapping option symbols to Greeks data
        """
        pass
    
    @abstractmethod
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
        pass
    
    @abstractmethod
    async def get_next_market_date(self) -> str:
        """
        Get the next trading date.
        
        Returns:
            Next market date in YYYY-MM-DD format
        """
        pass
    
    @abstractmethod
    async def lookup_symbols(self, query: str) -> List[SymbolSearchResult]:
        """
        Search for symbols matching the query.
        
        Args:
            query: Search term (symbol or company name)
            
        Returns:
            List of SymbolSearchResult objects
        """
        pass
    
    @abstractmethod
    async def get_historical_bars(self, symbol: str, timeframe: str, 
                                start_date: str = None, end_date: str = None, 
                                limit: int = 500) -> List[Dict[str, Any]]:
        """
        Get historical OHLCV bars for charting.
        
        Args:
            symbol: Stock symbol (e.g., "AAPL")
            timeframe: Time interval ("1m", "5m", "15m", "30m", "1h", "4h", "D", "W", "M")
            start_date: Start date in YYYY-MM-DD format (optional)
            end_date: End date in YYYY-MM-DD format (optional)
            limit: Maximum number of bars to return (default 500)
            
        Returns:
            List of OHLCV dictionaries in Lightweight Charts format:
            [{"time": "2024-01-15", "open": 150.25, "high": 152.80, "low": 149.90, "close": 151.45, "volume": 1234567}]
        """
        pass
    
    # === Account & Portfolio Methods ===
    
    @abstractmethod
    async def get_positions(self) -> List[Position]:
        """
        Get all current positions.
        
        Returns:
            List of Position objects
        """
        pass
    
    @abstractmethod
    async def get_orders(self, status: str = "open") -> List[Order]:
        """
        Get orders with optional status filter.
        
        Args:
            status: Order status filter ("open", "filled", "canceled", "all")
            
        Returns:
            List of Order objects
        """
        pass
    
    @abstractmethod
    async def get_account(self) -> Optional[Account]:
        """
        Get account information including balance and buying power.
        
        Returns:
            Account object with account details or None if not available
        """
        pass
    
    # === Order Management Methods ===
    
    @abstractmethod
    async def place_order(self, order_data: Dict[str, Any]) -> Order:
        """
        Place a trading order.
        
        Args:
            order_data: Order parameters
            
        Returns:
            Order object representing the placed order
        """
        pass

    @abstractmethod
    async def place_multi_leg_order(self, order_data: Dict[str, Any]) -> Order:
        """
        Place a multi-leg trading order.
        
        Args:
            order_data: Order parameters for a multi-leg order
            
        Returns:
            Order object representing the placed order
        """
        pass
    
    @abstractmethod
    async def cancel_order(self, order_id: str) -> bool:
        """
        Cancel an existing order.
        
        Args:
            order_id: ID of the order to cancel
            
        Returns:
            True if successful, False otherwise
        """
        pass
    
    # === Streaming Methods ===
    
    @abstractmethod
    async def connect_streaming(self) -> bool:
        """
        Connect to the provider's streaming service.
        
        Returns:
            True if connection successful, False otherwise
        """
        pass
    
    @abstractmethod
    async def disconnect_streaming(self) -> bool:
        """
        Disconnect from the provider's streaming service.
        
        Returns:
            True if disconnection successful, False otherwise
        """
        pass
    
    @abstractmethod
    async def subscribe_to_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """
        Subscribe to real-time data for symbols.
        
        Args:
            symbols: List of symbols to subscribe to
            data_types: Types of data to subscribe to (e.g., ["quotes", "trades"])
            
        Returns:
            True if subscription successful, False otherwise
        """
        pass
    
    @abstractmethod
    async def unsubscribe_from_symbols(self, symbols: List[str], data_types: List[str] = None) -> bool:
        """
        Unsubscribe from real-time data for symbols.
        
        Args:
            symbols: List of symbols to unsubscribe from
            data_types: Types of data to unsubscribe from (e.g., ["quotes", "trades"])
            
        Returns:
            True if unsubscription successful, False otherwise
        """
        pass
    
    def set_streaming_queue(self, queue: asyncio.Queue):
        """Set the queue for streaming data."""
        self._streaming_queue = queue
    
    # === Utility Methods ===
    
    def get_subscribed_symbols(self) -> Set[str]:
        """Get currently subscribed symbols."""
        return self._subscribed_symbols.copy()
    
    def is_streaming_connected(self) -> bool:
        """Check if streaming connection is active."""
        return self.is_connected
    
    async def health_check(self) -> Dict[str, Any]:
        """
        Perform a health check on the provider.
        
        Returns:
            Dictionary with health status information
        """
        return {
            "provider": self.name,
            "connected": self.is_connected,
            "subscribed_symbols": len(self._subscribed_symbols),
            "timestamp": datetime.now().isoformat()
        }
    
    # === Helper Methods for Subclasses ===
    
    def _create_api_response(self, success: bool, data: Any = None, error: str = None, message: str = None) -> ApiResponse:
        """Helper method to create standardized API responses."""
        return ApiResponse(
            success=success,
            data=data,
            error=error,
            message=message,
            timestamp=datetime.now().isoformat()
        )
    
    def _log_error(self, operation: str, error: Exception):
        """Helper method for consistent error logging."""
        logger.error(f"{self.name} provider error in {operation}: {str(error)}", exc_info=True)
    
    def _log_info(self, message: str):
        """Helper method for consistent info logging."""
        logger.info(f"{self.name} provider: {message}")
