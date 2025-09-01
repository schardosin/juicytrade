"""
Mock providers for strategy parameter extraction.

These lightweight mock classes are used when we need to instantiate strategies
temporarily to extract their metadata without requiring full system dependencies.
"""

from typing import Dict, List, Any, Optional
from .data_provider import StrategyDataProvider


class MockDataProvider(StrategyDataProvider):
    """
    Mock data provider for metadata extraction.
    
    This is a minimal implementation used only for getting strategy parameters.
    """
    
    def __init__(self):
        super().__init__()
        self.is_connected = True
    
    def subscribe_symbol(self, symbol: str, callback: Optional[callable] = None) -> bool:
        """Mock subscription - always succeeds."""
        self.subscriptions.add(symbol)
        return True
    
    def unsubscribe_symbol(self, symbol: str) -> bool:
        """Mock unsubscription - always succeeds."""
        self.subscriptions.discard(symbol)
        return True
    
    def get_current_price(self, symbol: str) -> Optional[float]:
        """Mock price - returns a default value."""
        return 100.0
    
    def get_recent_candles(self, symbol: str, limit: int) -> List[Dict[str, Any]]:
        """Mock candles - returns empty list."""
        return []
    
    def get_options_chain(self, symbol: str, expiry: str = None, strikes_around_atm: int = 10) -> List[Dict]:
        """Mock options chain - returns empty list."""
        return []
    
    def get_greeks_data(self, symbol: str) -> Optional[Dict[str, Any]]:
        """Mock Greeks - returns None."""
        return None


class MockOrderExecutor:
    """
    Mock order executor for metadata extraction.
    
    This is a minimal implementation used only for getting strategy parameters.
    """
    
    def __init__(self):
        self.is_connected = True
        self.orders = []
    
    async def submit_order(self, order_data: Dict[str, Any]) -> Dict[str, Any]:
        """Mock order submission - always succeeds."""
        return {
            "success": True,
            "order_id": "mock_order_123",
            "status": "filled"
        }
    
    async def cancel_order(self, order_id: str) -> bool:
        """Mock order cancellation - always succeeds."""
        return True
    
    def get_order_status(self, order_id: str) -> Optional[Dict[str, Any]]:
        """Mock order status - returns filled."""
        return {
            "order_id": order_id,
            "status": "filled",
            "filled_quantity": 100
        }
    
    def get_positions(self) -> List[Dict[str, Any]]:
        """Mock positions - returns empty list."""
        return []
    
    def get_account_balance(self) -> float:
        """Mock account balance."""
        return 100000.0
