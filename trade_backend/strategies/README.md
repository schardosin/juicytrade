# Automated Trading Strategies

This module implements a complete automated trading system that allows users to create, validate, and execute Python-based trading strategies within the Juicy Trade platform.

## 🏗️ Architecture Overview

The automated trading system consists of several key components:

- **`BaseStrategy`** - Abstract base class that all strategies must inherit from
- **`StrategyDataProvider`** - Provides unified access to market data (live/backtesting modes)
- **`OrderExecutor`** - Handles order placement and management
- **`StrategyExecutionEngine`** - Core component managing strategy lifecycle
- **`StrategyValidator`** - Multi-layer validation system for security
- **`StrategyRegistry`** - Manages strategy registration and storage
- **`api_endpoints.py`** - REST API for strategy management

## 🚀 Getting Started

### 1. Strategy Template

Your strategy classes must inherit from `BaseStrategy`:

```python
from strategies.base_strategy import BaseStrategy, StrategyResult, MarketData

class MyTradingStrategy(BaseStrategy):
    def initialize(self):
        """Called once when strategy starts"""
        self.symbol = 'AAPL'
        self.subscribe_symbol(self.symbol)

    def on_market_data(self, symbol: str, data: MarketData) -> Optional[StrategyResult]:
        """Called whenever new market data arrives"""
        if self.should_buy(data):
            return StrategyResult(
                action='BUY',
                symbol=symbol,
                quantity=100,
                reason='My trading signal'
            )
        return None

    def get_strategy_metadata(self):
        return {
            'name': 'My Strategy',
            'description': 'My awesome strategy',
            'risk_level': 'MEDIUM',
            'preferred_symbols': ['AAPL', 'MSFT']
        }
```

### 2. Required Methods

Your strategy class **must** implement these methods:

- **`initialize()`** - Set up subscriptions and initial state
- **`on_market_data(symbol, data)`** - Process market data and return trade signals
- **`get_strategy_metadata()`** - Return strategy configuration for UI

### 3. Optional Methods

Your strategy class **may** override these methods:

- **`before_market_open()`** - Pre-market logic
- **`after_market_close()`** - End-of-day logic
- **`on_position_update(symbol, data)`** - Handle position changes
- **`on_order_status(order_id, status, details)`** - Handle order updates
- **`should_stop_strategy()`** - Check if strategy should stop

## 📊 Strategy Interface

### MarketData Structure

Your `on_market_data()` method receives `MarketData` objects:

```python
@dataclass
class MarketData:
    symbol: str
    timestamp: datetime
    price: Optional[float]          # Current price
    bid: Optional[float]           # Bid price
    ask: Optional[float]           # Ask price
    volume: Optional[int]          # Volume
    delta: Optional[float]         # Option Greeks
    gamma: Optional[float]
    theta: Optional[float]
    vega: Optional[float]
```

### StrategyResult Structure

Return `StrategyResult` objects to execute trades:

```python
StrategyResult(
    action='BUY',          # 'BUY', 'SELL', 'HOLD', 'CLOSE_POSITION'
    symbol='AAPL',         # Trading symbol
    quantity=100,          # Number of shares/contracts
    order_type='MARKET',   # 'MARKET', 'LIMIT', 'STOP'
    limit_price=150.0,     # Required for LIMIT orders
    stop_price=145.0,      # Required for STOP orders
    reason='Signal',       # Trade reasoning
    rule_id='my_rule'      # Identifies which rule fired
)
```

## 🛡️ Security & Validation

### Upload Process

1. **File Validation** - Check file size, type, content
2. **Syntax Check** - Ensure valid Python syntax
3. **Import Security** - Only allow safe imports
4. **Interface Validation** - Verify strategy implements required methods
5. **Runtime Safety** - Test strategy instantiation in sandbox

### Safe Imports

Strategies can only import these modules:
- Standard library: `datetime`, `time`, `math`, `statistics`, `collections`
- Data libraries: `pandas`, `numpy`, `scipy`
- Strategy framework: `BaseStrategy`, `StrategyResult`, etc.

### Banned Code Patterns

These patterns are **not allowed** in strategies:
- `import os`, `import sys` (system access)
- `eval()`, `exec()` (code injection)
- `__import__()` (dynamic imports)
- File operations: `open()`, `file()`
- System calls: `subprocess`, `socket`

## 🎯 API Usage

### Strategy Upload

```bash
curl -X POST "http://localhost:8000/api/strategies/upload" \
  -F "file=@/path/to/strategy.py" \
  -F "name=My Strategy" \
  -F "description=My awesome trading strategy"
```

### List User Strategies

```bash
curl -X GET "http://localhost:8000/api/strategies/my" \
  -H "Authorization: Bearer <token>"
```

### Start Strategy

```bash
curl -X POST "http://localhost:8000/api/strategies/strategy_id/start" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "live",
    "config": {
      "symbol": "AAPL",
      "fast_period": 5,
      "slow_period": 20
    }
  }'
```

### Monitor Strategy

```bash
curl -X GET "http://localhost:8000/api/strategies/strategy_id/status"
```

## 🔧 Configuration

### Strategy Parameters

Configure your strategy with these parameter types:

```python
parameters = {
    'fast_period': {
        'type': 'integer',
        'default': 5,
        'min': 2,
        'max': 20,
        'description': 'Fast moving average period'
    },
    'enable_filter': {
        'type': 'boolean',
        'default': True,
        'description': 'Enable volume filter'
    },
    'risk_pct': {
        'type': 'float',
        'default': 1.0,
        'min': 0.1,
        'max': 5.0,
        'description': 'Risk per trade (%)'
    }
}
```

### Risk Management

```python
metadata = {
    'risk_level': 'MEDIUM',
    'max_positions': 3,
    'position_size_pct': 5.0,  # % of capital per position
    'max_daily_loss_pct': 25.0 # Stop strategy at -25% daily loss
}
```

## 📈 Example: Complete Strategy

See `example_moving_average_strategy.py` for a complete working example that demonstrates:

- Complex trading logic with multiple rules
- Technical indicator calculations
- Risk management (stop-loss, take-profit)
- Position sizing based on risk
- UI feedback for rule states
- Proper error handling

## 🚀 Running Strategies

### 1. Upload Strategy
Upload your Python strategy file through the UI or API.

### 2. Validation
The system automatically validates your strategy for:
- ✅ Python syntax
- ✅ Security (safe imports only)
- ✅ Interface compliance
- ✅ Runtime safety

### 3. Configuration
Configure strategy parameters through the UI:
- Trading symbols
- Risk settings
- Time schedules
- Position limits

### 4. Execution
Start your strategy in live or backtesting mode:
- **Live Mode**: Trades with real money
- **Backtest Mode**: Tests on historical data

### 5. Monitoring
Monitor strategy performance in real-time:
- Rule evaluation states
- Trade execution
- P&L tracking
- Error notifications

## ⚡ Integration with Existing System

This trading strategy system integrates seamlessly with your existing Juicy Trade infrastructure:

- **Data Providers**: Uses existing `provider_manager` and streaming system
- **Order Management**: Integrates with existing brokers (Alpaca, Tradier, TastyTrade)
- **UI Components**: Extends existing Vue.js interface
- **Authentication**: Works with existing user management
- **Monitoring**: Uses existing health monitoring system

## 🎯 Best Practices

### Strategy Design
- Keep strategies focused on one primary trading idea
- Use meaningful parameter names and descriptions
- Include comprehensive error handling
- Document your trading logic clearly

### Risk Management
- Always implement position sizing based on risk
- Set appropriate stop-loss and take-profit levels
- Consider maximum drawdown limits
- Validate all inputs and market data

### Testing & Deployment
- Start with backtesting on multiple time periods
- Use a demo account before live trading
- Monitor performance metrics regularly
- Have manual override capabilities

## 🔧 Advanced Features

### Custom Data Calculations

```python
def calculate_atr(self, prices: List[float], period: int = 14) -> float:
    """Calculate Average True Range"""
    tr_values = []
    for i in range(1, len(prices)):
        tr = max(
            prices[i] - prices[i-1],
            abs(prices[i] - prices[i-1]),
            abs(prices[i-1] - prices[i])
        )
        tr_values.append(tr)

    return sum(tr_values[-period:]) / period
```

### Time-Based Logic

```python
def during_market_hours(self, timestamp: datetime) -> bool:
    """Check if current time is during market hours"""
    if hasattr(self, 'market_hours'):
        start = datetime.strptime(self.market_hours['start'], '%H:%M:%S').time()
        end = datetime.strptime(self.market_hours['end'], '%H:%M:%S').time()
        current = timestamp.time()
        return start <= current <= end
    return True  # Default to always trading
```

### Position Management

```python
def manage_positions(self):
    """Implement custom position sizing and management"""
    total_capital = self.config.get('account_balance', 10000)
    max_risk_per_trade = self.config.get('risk_per_trade_pct', 1.0)

    # Calculate position size based on risk
    risk_amount = total_capital * max_risk_per_trade / 100
    position_size = self.calculate_optimal_position_size(risk_amount)

    return min(position_size, self.config.get('max_position_size', 1000))
```

The automated trading system provides a robust, secure, and flexible framework for creating and executing algorithmic trading strategies. Start with the example strategy and gradually build more complex strategies as you become familiar with the framework.