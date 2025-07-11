"""
Trading Backend - Multi-Provider Trading API

A clean, decoupled trading backend that supports multiple data providers
through a standardized interface. Currently supports Alpaca with easy
extensibility for additional providers.

Key Features:
- Provider-agnostic API design
- Standardized data models
- Real-time streaming support
- WebSocket integration
- Clean separation of concerns

Usage:
    from trading_backend.main import app
    # or
    python -m trading_backend.main
"""

__version__ = "1.0.0"
__author__ = "Trading Backend Team"

from .config import settings
from .models import *
from .providers import BaseProvider

__all__ = [
    "settings",
    "BaseProvider",
    # Models
    "StockQuote",
    "OptionContract", 
    "Position",
    "Order",
    "ApiResponse",
    "SymbolRequest",
    "OrderRequest",
    "MultiLegOrderRequest"
]
