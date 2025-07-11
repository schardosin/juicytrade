# Trading Backend - Multi-Provider Architecture

A clean, decoupled trading backend that supports multiple data providers through a standardized interface. This architecture separates concerns and makes it easy to add new trading data providers.

## Architecture Overview

```
trading_backend/
├── main.py              # FastAPI application with provider factory
├── config.py            # Configuration management
├── models.py            # Standardized Pydantic data models
├── __init__.py          # Package initialization
├── README.md            # This file
└── providers/
    ├── __init__.py
    ├── base_provider.py  # Abstract base class interface
    └── alpaca_provider.py # Alpaca implementation
```

## Key Components

### 1. BaseProvider Interface (`providers/base_provider.py`)

- Abstract base class defining the contract for all providers
- Ensures consistent API across different data sources
- Methods for market data, positions, orders, and streaming

### 2. Standardized Models (`models.py`)

- Pydantic models for data consistency
- Provider-agnostic data structures
- Automatic validation and serialization

### 3. Provider Implementations (`providers/`)

- **AlpacaProvider**: Complete Alpaca integration
- **Future providers**: Easy to add (PublicProvider, etc.)

### 4. Configuration (`config.py`)

- Environment-based configuration
- Provider selection and credentials
- Server settings

## Features

### ✅ **Market Data**

- Stock quotes (single and batch)
- Options chains with full contract details
- Expiration dates for options
- Market calendar information

### ✅ **Account & Portfolio**

- Current positions with P&L
- Order history and status
- Multi-leg order support

### ✅ **Real-time Streaming**

- WebSocket connections
- Live price updates
- Subscription management

### ✅ **Provider Abstraction**

- Easy to switch between providers
- Consistent API regardless of data source
- Standardized error handling

## Usage

### Running the Server

```bash
# Using Python module
python -m trading_backend.main

# Or directly
cd trading_backend
python main.py
```

### Configuration

Set environment variables in `.env`:

```env
# Provider selection
PROVIDER=alpaca

# Alpaca credentials
APCA_API_KEY_ID_LIVE=your_live_key
APCA_API_SECRET_KEY_LIVE=your_live_secret
APCA_API_KEY_ID_PAPER=your_paper_key
APCA_API_SECRET_KEY_PAPER=your_paper_secret

# URLs
ALPACA_BASE_URL_LIVE=https://api.alpaca.markets
ALPACA_BASE_URL_PAPER=https://paper-api.alpaca.markets
```

### API Endpoints

#### Market Data

- `GET /prices/stocks?symbols=SPY,QQQ` - Stock quotes
- `GET /expiration_dates?symbol=SPY` - Option expiration dates
- `GET /options_chain?symbol=SPY&expiry=2024-01-19` - Options chain
- `GET /next_market_date` - Next trading date

#### Account & Portfolio

- `GET /positions` - Current positions
- `GET /orders?status=open` - Orders with filtering
- `GET /open_orders` - Open orders (backward compatibility)

#### Streaming

- `POST /subscribe/stocks` - Subscribe to stock streaming
- `POST /subscribe/options` - Subscribe to option streaming
- `WebSocket /ws` - Real-time data stream

#### System

- `GET /` - Service status and health
- `GET /health` - Detailed health check

## Adding New Providers

To add a new provider (e.g., Public):

1. **Create Provider Class**:

```python
# providers/public_provider.py
from .base_provider import BaseProvider

class PublicProvider(BaseProvider):
    def __init__(self, api_key: str):
        super().__init__("Public")
        self.api_key = api_key

    async def get_stock_quote(self, symbol: str):
        # Implement Public API call
        # Transform to StockQuote model
        pass

    # Implement other required methods...
```

2. **Update Provider Factory**:

```python
# main.py
def create_provider() -> BaseProvider:
    if settings.provider.lower() == "alpaca":
        return AlpacaProvider(...)
    elif settings.provider.lower() == "public":
        return PublicProvider(...)
    else:
        raise ValueError(f"Unsupported provider: {settings.provider}")
```

3. **Add Configuration**:

```python
# config.py
class Settings(BaseSettings):
    # ... existing settings ...
    public_api_key: str = os.getenv("PUBLIC_API_KEY", "")
```

## Data Models

### StockQuote

```python
{
    "symbol": "SPY",
    "ask": 450.25,
    "bid": 450.20,
    "timestamp": "2024-01-15T10:30:00"
}
```

### OptionContract

```python
{
    "symbol": "SPY240119C00450000",
    "underlying_symbol": "SPY",
    "expiration_date": "2024-01-19",
    "strike_price": 450.0,
    "type": "call",
    "bid": 2.50,
    "ask": 2.55,
    "delta": 0.52,
    "gamma": 0.03,
    "theta": -0.05,
    "vega": 0.12
}
```

### Position

```python
{
    "symbol": "SPY",
    "qty": 100,
    "side": "long",
    "market_value": 45000.0,
    "cost_basis": 44500.0,
    "unrealized_pl": 500.0,
    "current_price": 450.0,
    "avg_entry_price": 445.0,
    "asset_class": "us_equity"
}
```

## Benefits of This Architecture

### 🔧 **Maintainability**

- Clear separation of concerns
- Provider-specific logic isolated
- Easy to debug and extend

### 🚀 **Scalability**

- Add new providers without changing core logic
- Standardized data models prevent inconsistencies
- Modular design supports team development

### 🛡️ **Reliability**

- Consistent error handling across providers
- Standardized logging and monitoring
- Provider health checks

### 🔄 **Flexibility**

- Switch providers via configuration
- A/B test different data sources
- Fallback provider support (future)

## Migration from Old Backend

The new architecture maintains API compatibility with the existing frontend:

1. **Same Endpoints**: All existing endpoints work unchanged
2. **Same Data Format**: Response formats are preserved
3. **Enhanced Features**: Better error handling and logging
4. **Cleaner Code**: Easier to maintain and extend

## Development

### Running Tests

```bash
# Install dependencies
pip install -r requirements.txt

# Run the server
python -m trading_backend.main

# Test endpoints
curl http://localhost:8008/health
curl http://localhost:8008/prices/stocks?symbols=SPY
```

### Adding Features

1. Add method to `BaseProvider` interface
2. Implement in all provider classes
3. Add API endpoint in `main.py`
4. Update models if needed

This architecture provides a solid foundation for a multi-provider trading system that's both powerful and maintainable.
