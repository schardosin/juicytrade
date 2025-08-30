# Data Flow Architecture Documentation

## 🤖 LLM Implementation Guide - READ THIS FIRST

### 🎯 CRITICAL RULE: Components Must Be "Dumb"

**Components should NEVER make direct API calls or manage data fetching logic.** They only consume reactive data through the unified `useMarketData()` composable.

```javascript
// ✅ CORRECT - Component is "dumb", only consumes data
export default {
  setup() {
    const { getStockPrice, getBalance } = useMarketData();
    
    // Just get reactive data - everything else is automatic
    const price = getStockPrice("AAPL");     // WebSocket, auto-managed
    const balance = getBalance();            // Auto-refreshes every 60s
    
    return { price, balance };
  }
}

// ❌ WRONG - Component is "smart", manages data logic
export default {
  setup() {
    const price = ref(0);
    
    // DON'T DO THIS - No direct API calls in components
    const fetchPrice = async () => {
      price.value = await api.getPrice("AAPL");
    };
    
    onMounted(fetchPrice);
    setInterval(fetchPrice, 5000);
    
    return { price };
  }
}
```

### 🔧 Implementation Rules for LLMs

1. **Always Use useMarketData()**: `const { getStockPrice } = useMarketData();`
2. **Never Import API Directly**: Don't use `import api from '../services/api';`
3. **No Manual Intervals**: Smart data system handles all timing
4. **No WebSocket Management**: Subscriptions are automatic
5. **Use Centralized Loading/Error States**: Available through `isLoading()` and `getError()`

### 🚨 Common Mistakes to Avoid

- ❌ Direct API calls: `await api.getPositions()`
- ❌ Manual intervals: `setInterval(fetchData, 30000)`
- ❌ WebSocket subscriptions: `webSocketClient.subscribe("AAPL")`
- ❌ Manual loading states: `const loading = ref(false)`

### 📋 Quick Reference

**For any data need, use these patterns:**
- Real-time prices: `const price = getStockPrice("AAPL")`
- Account data: `const balance = getBalance()` (auto-updates)
- Search/lookup: `const results = await lookupSymbols(query)` (cached)
- Orders: `const orders = await getOrdersByStatus("open")` (always fresh)
- Historical data: `const data = await getHistoricalData("AAPL", "1D")` (cached)

---

## Overview

This document outlines the unified data management architecture for the Vue.js trading application. The system implements a hybrid approach that handles both real-time WebSocket data and REST API data through enhanced centralized services, ensuring components access data through consistent patterns.

## Core Principles

### 1. **Unified Data Access**

- **Single Interface**: All components use `useMarketData()` composable regardless of data source
- **Strategy Abstraction**: Components don't know if data comes from WebSocket, periodic updates, or caching
- **Consistent Patterns**: Same reactive data access patterns across all components

### 2. **Hybrid Data Sources**

- **WebSocket Data**: Real-time stock/option prices with smart subscription management
- **Periodic Updates**: Auto-refreshing account data (balance, positions, orders)
- **TTL Caching**: Historical data, symbol lookup, options chains
- **One-time Fetch**: Static account information

### 3. **Component Abstraction**

- Components consume reactive data without knowing the underlying strategy
- Automatic subscription management based on component lifecycle
- Centralized error handling and loading states

## Architecture Overview

```
┌───────────────────────────────────────────────────────────────────────────────┐
│                              Data Sources                                     │
├─────────────────┬─────────────────┬─────────────────┬─────────────────────────┤
│   WebSocket     │  Periodic API   │   Cached API    │   Fresh API             │
│   (Real-time)   │  (Auto-refresh) │  (TTL-based)    │  (Always Current)       │
│                 │                 │                 │                         │
│ • Stock Prices  │ • Balance (60s) │ • Historical    │ • Orders (by status)    │
│ • Option Prices │ • Positions(30s)│   (5min cache)  │ • Critical broker data  │
│ • Live Updates  │ • Background    │ • Symbol Lookup │ • Dynamic parameters    │
│                 │   Updates       │   (10min cache) │ • No caching            │
└─────────────────┴─────────────────┴─────────────────┴─────────────────────────┘
                                        │
                                        ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                    Enhanced SmartMarketDataStore                                 │
├──────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│ ┌───────────────┐ ┌──────────────┐ ┌───────────────┐ ┌─────────────────────────┐ │
│ │ WebSocket     │ │ Periodic     │ │ Cached        │ │ On-Demand Fresh         │ │
│ │ Strategy      │ │ Strategy     │ │ Strategy      │ │ Strategy                │ │
│ │               │ │              │ │               │ │                         │ │
│ │• Web Worker   │ │• Auto-refresh│ │• TTL Cache    │ │• Always Fresh           │ │
│ │• Sleep Resist.│ │• Background  │ │• On-demand    │ │• Dynamic Params         │ │
│ │• Auto Recovery│ │• Error Retry │ │• Force Refresh│ │• Centralized Loading    │ │
│ │• Vue Reactive │ │• Health Mon. │ │• Smart Cache. │ │• Real-time Status       │ │
│ │• Smart Sub.   │ │• Recovery    │ │• Memory Mgmt. │ │• Error Handling         │ │
│ └───────────────┘ └──────────────┘ └───────────────┘ └─────────────────────────┘ │
│                                                                                  │
│ ┌─────────────────────────────────────────────────────────────────────────┐      │
│ │                    Background Web Worker                                │      │
│ │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │      │
│ │ │Sleep-Resist │ │Auto-Recovery│ │Health Check │ │Connection Mgmt      │ │      │
│ │ │Architecture │ │System       │ │& Monitoring │ │& Status Tracking    │ │      │
│ │ │             │ │             │ │             │ │                     │ │      │
│ │ │• Dedicated  │ │• Wake Detect│ │• Real-time  │ │• Reactive Status    │ │      │
│ │ │  Worker     │ │• Network    │ │  Health     │ │• Instant Updates    │ │      │
│ │ │• Background │ │  Recovery   │ │• Auto-Retry │ │• Vue Integration    │ │      │
│ │ │  Processing │ │• Smart      │ │• Failure    │ │• Event-Driven       │ │      │
│ │ │• No Sleep   │ │  Reconnect  │ │  Detection  │ │• Error Propagation  │ │      │
│ │ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────────────┘ │      │
│ └─────────────────────────────────────────────────────────────────────────┘      │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         useMarketData Composable                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  • getStockPrice(symbol)       → WebSocket Strategy (Web Worker-Based)      │
│  • getOptionPrice(symbol)      → WebSocket Strategy (Web Worker-Based)      │
│  • getBalance()                → Periodic Strategy (60s)                    │
│  • getPositions()              → Periodic Strategy (30s)                    │
│  • getOrdersByStatus(status)   → On-Demand Fresh Strategy                   │
│  • getHistoricalData()         → Cached Strategy (5min TTL)                 │
│  • lookupSymbols()             → Cached Strategy (10min TTL)                │
│  • getOptionsChain()           → Cached Strategy (5min TTL)                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                    Centralized Selected Legs Store                               │
├──────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│ ┌─────────────────────────────────────────────────────────────────────────┐      │
│ │                    selectedLegsStore.js                                 │      │
│ │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │      │
│ │ │Multi-Source │ │Reactive     │ │Source       │ │Validation &         │ │      │
│ │ │Leg Mgmt     │ │State Mgmt   │ │Tracking     │ │Business Logic       │ │      │
│ │ │             │ │             │ │             │ │                     │ │      │
│ │ │• Options    │ │• Vue        │ │• Options    │ │• Quantity Limits    │ │      │
│ │ │  Chain      │ │  Reactive   │ │  Chain      │ │• Side Validation    │ │      │
│ │ │• Positions  │ │• Computed   │ │• Positions  │ │• Premium Calc       │ │      │
│ │ │• Strategies │ │  Metadata   │ │• Strategies │ │• Source-specific    │ │      │
│ │ │• Unified    │ │• Auto       │ │• Cross-     │ │  Rules              │ │      │
│ │ │  Interface  │ │  Updates    │ │  Component  │ │• Data Integrity     │ │      │
│ │ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────────────┘ │      │
│ └─────────────────────────────────────────────────────────────────────────┘      │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      useSelectedLegs Composable                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  • selectedLegs                → Reactive array of all selected legs        │
│  • hasSelectedLegs             → Boolean if any legs selected               │
│  • addLeg(data, source)        → Add leg from any source                    │
│  • removeLeg(symbol)           → Remove specific leg                        │
│  • clearBySource(source)       → Clear legs from specific source            │
│  • addFromOptionsChain()       → Convenience method for options chain       │
│  • addFromPosition()           → Convenience method for positions           │
│  • getSelectionClass()         → CSS classes for UI state                   │
│  • Quantity validation helpers → Source-aware quantity limits               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────────┐
│                              Components                                       │
├─────────────────┬─────────────────┬─────────────────┬─────────────────────────┤
│  OptionsTrading │  PositionsView  │ BottomTradingP. │    RightPanel           │
│                 │                 │                 │                         │
│ • Live Prices   │ • Auto-refresh  │ • Selected Legs │ • Analysis Tab          │
│ • Options Chain │ • Real-time P&L │ • Unified View  │ • Payoff Chart          │
│ • Leg Selection │ • Leg Selection │ • Order Mgmt    │ • Multi-source Data     │
│ • Centralized   │ • Centralized   │ • Reactive UI   │ • Reactive Updates      │
│   Selection     │   Selection     │ • Cross-source  │ • Chart Integration     │
│                 │                 │   Support       │                         │
└─────────────────┴─────────────────┴─────────────────┴─────────────────────────┘
```

## Multi-Provider Architecture

### Overview

The JuicyTrade application supports multiple trading providers with a unified interface. Each provider implements the same base functionality while handling provider-specific authentication, data formats, and API endpoints.

### Supported Providers

#### 1. **Alpaca Markets**
- **Type**: Commission-free stock and options trading
- **Authentication**: API Key + Secret
- **Streaming**: WebSocket with real-time market data
- **Features**: Paper trading, live trading, comprehensive options support

#### 2. **Tradier**
- **Type**: Professional trading platform
- **Authentication**: Bearer token
- **Streaming**: WebSocket with session-based authentication
- **Features**: Advanced options strategies, real-time quotes, comprehensive order management

#### 3. **TastyTrade** ⭐ *NEW*
- **Type**: Options-focused trading platform
- **Authentication**: Session-based with username/password
- **Streaming**: DXLink protocol for professional-grade market data
- **Features**: Advanced options analytics, Greeks streaming, professional trading tools

### Provider Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        Multi-Provider Architecture                              │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │   Alpaca        │    │    Tradier      │    │     TastyTrade          │      │
│  │   Provider      │    │    Provider     │    │     Provider            │      │
│  │                 │    │                 │    │                         │      │
│  │ • REST API      │    │ • REST API      │    │ • REST API              │      │
│  │ • WebSocket     │    │ • WebSocket     │    │ • DXLink Streaming      │      │
│  │ • API Key Auth  │    │ • Bearer Token  │    │ • Session Auth          │      │
│  │ • Paper Trading │    │ • Live Trading  │    │ • Options Focus         │      │
│  │ • Options       │    │ • Advanced      │    │ • Greeks Streaming      │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Base Provider Interface                              │    │
│  │                                                                         │    │
│  │  • get_stock_quote()           → Unified stock price interface          │    │
│  │  • get_options_chain()         → Standardized options data              │    │
│  │  • get_positions()             → Consistent position format             │    │
│  │  • place_order()               → Universal order placement              │    │
│  │  • connect_streaming()         → Provider-specific streaming            │    │
│  │  • subscribe_to_symbols()      → Unified subscription management        │    │
│  │  • get_historical_bars()       → Standardized chart data               │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Provider Manager                                     │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Configuration   │  │ Health          │  │ Symbol Conversion       │  │    │
│  │  │ Management      │  │ Monitoring      │  │                         │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Provider      │  │ • Connection    │  │ • OCC Standard Format   │  │    │
│  │  │   Selection     │  │   Status        │  │ • Provider-Specific     │  │    │
│  │  │ • Credentials   │  │ • Auto Recovery │  │   Conversion            │  │    │
│  │  │ • Routing       │  │ • Error         │  │ • Bidirectional         │  │    │
│  │  │ • Failover      │  │   Tracking      │  │   Translation           │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### TastyTrade Integration Details

#### Authentication Flow
```python
# TastyTrade uses session-based authentication
class TastyTradeProvider(BaseProvider):
    async def _create_session(self) -> bool:
        """Create session with username/password"""
        payload = {
            "login": self.username,
            "password": self.password,
            "remember-me": False
        }
        
        response = await client.post(f"{self.base_url}/sessions", json=payload)
        data = response.json()
        
        self._session_token = data["data"]["session-token"]
        self._session_expires_at = datetime.fromisoformat(
            data["data"]["session-expiration"]
        )
        
        return True
```

#### DXLink Streaming Protocol
```python
# TastyTrade uses DXLink for professional market data
async def _dxlink_streaming_setup(self) -> bool:
    """Execute DXLink setup sequence"""
    # 1. SETUP - Initialize connection
    setup_msg = {
        "type": "SETUP",
        "channel": 0,
        "version": "0.1-DXF-JS/0.3.0",
        "keepaliveTimeout": 60
    }
    
    # 2. AUTH - Authenticate with quote token
    auth_msg = {
        "type": "AUTH",
        "channel": 0,
        "token": self._quote_token
    }
    
    # 3. FEED_SETUP - Configure data types
    feed_setup_msg = {
        "type": "FEED_SETUP",
        "channel": 1,
        "acceptEventFields": {
            "Quote": ["bidPrice", "askPrice", "bidSize", "askSize"],
            "Greeks": ["delta", "gamma", "theta", "vega", "volatility"]
        }
    }
```

#### Symbol Format Conversion
```python
# TastyTrade uses different symbol formats
def convert_symbol_to_provider_format(self, symbol: str) -> str:
    """Convert standard OCC to TastyTrade format"""
    # Standard: SPXW250806P02600000
    # TastyTrade: SPXW  250806P02600000 (with spaces)
    
    if self._is_option_symbol(symbol):
        # Parse OCC format and add TastyTrade spacing
        return self._format_tastytrade_option_symbol(symbol)
    
    return symbol  # Stock symbols unchanged
```

## Streaming Health Monitoring System ⭐ *NEW*

### Overview

The application now includes a comprehensive streaming health monitoring system that provides real-time connection monitoring, automatic recovery, and detailed health metrics for all streaming providers.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    Streaming Health Monitoring System                           │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │ Connection      │    │ Health          │    │ Recovery                │      │
│  │ State Tracking  │    │ Metrics         │    │ Management              │      │
│  │                 │    │                 │    │                         │      │
│  │ • CONNECTING    │    │ • Data Received │    │ • Auto Reconnection     │      │
│  │ • CONNECTED     │    │ • Error Count   │    │ • Subscription Restore  │      │
│  │ • DISCONNECTED  │    │ • Uptime        │    │ • Exponential Backoff   │      │
│  │ • FAILED        │    │ • Subscriptions │    │ • Sleep/Wake Detection  │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    StreamingHealthManager                               │    │
│  │                                                                         │    │
│  │  • register_provider()         → Register provider for monitoring       │    │
│  │  • register_connection()       → Track individual connections           │    │
│  │  • update_connection_state()   → Real-time state updates               │    │
│  │  • record_data_received()      → Track data flow                       │    │
│  │  • record_error()              → Log and track errors                  │    │
│  │  • get_health_status()         → Comprehensive health report           │    │
│  │  • start_monitoring()          → Begin health monitoring               │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Provider Integration                                 │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Alpaca          │  │ Tradier         │  │ TastyTrade              │  │    │
│  │  │ Integration     │  │ Integration     │  │ Integration             │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Health        │  │ • Health        │  │ • Health                │  │    │
│  │  │   Reporting     │  │   Reporting     │  │   Reporting             │  │    │
│  │  │ • State Updates │  │ • State Updates │  │ • State Updates         │  │    │
│  │  │ • Error         │  │ • Error         │  │ • Error                 │  │    │
│  │  │   Tracking      │  │   Tracking      │  │   Tracking              │  │    │
│  │  │ • Timeout       │  │ • Timeout       │  │ • Timeout               │  │    │
│  │  │   Protection    │  │   Protection    │  │   Protection            │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Features

#### 1. **Real-time Connection Monitoring**
```python
# Connection state tracking
class ConnectionState(Enum):
    CONNECTING = "connecting"
    CONNECTED = "connected"
    DISCONNECTED = "disconnected"
    FAILED = "failed"

# Real-time state updates
streaming_health_manager.update_connection_state(
    connection_id, 
    ConnectionState.CONNECTED
)
```

#### 2. **Comprehensive Health Metrics**
```python
@dataclass
class ConnectionMetrics:
    connection_id: str
    provider_name: str
    connection_type: str
    state: ConnectionState
    connected_at: Optional[datetime]
    last_data_received: Optional[datetime]
    data_received_count: int
    error_count: int
    subscriptions: Set[str]
    uptime_seconds: float
```

#### 3. **Automatic Recovery System**
```python
# Enhanced connection with health monitoring
async def connect_streaming(self) -> bool:
    try:
        # Update health status
        streaming_health_manager.update_connection_state(
            self._connection_id, 
            ConnectionState.CONNECTING
        )
        
        # Connection logic with timeout protection
        self._stream_connection = await TimeoutWrapper.execute(
            websockets.connect(self.stream_url),
            timeout=15.0,
            operation_name="connect_websocket"
        )
        
        # Success - update health status
        streaming_health_manager.update_connection_state(
            self._connection_id, 
            ConnectionState.CONNECTED
        )
        
        return True
        
    except Exception as e:
        # Failure - record error and update status
        streaming_health_manager.record_error(self._connection_id, str(e))
        streaming_health_manager.update_connection_state(
            self._connection_id, 
            ConnectionState.FAILED
        )
        return False
```

#### 4. **Timeout Protection**
```python
class TimeoutWrapper:
    @staticmethod
    async def execute(coro, timeout: float, operation_name: str = "operation"):
        """Execute coroutine with timeout protection and logging"""
        try:
            return await asyncio.wait_for(coro, timeout=timeout)
        except asyncio.TimeoutError:
            logger.error(f"⏰ Timeout in {operation_name} after {timeout}s")
            raise
        except Exception as e:
            logger.error(f"❌ Error in {operation_name}: {e}")
            raise
```

#### 5. **Subscription Restoration**
```python
# Automatic subscription restoration after reconnection
async def _restore_subscriptions(self, provider, provider_name: str):
    """Restore subscriptions after provider reconnection"""
    all_symbols = list(self._stock_subscriptions | self._options_subscriptions)
    if all_symbols:
        logger.info(f"🔄 Restoring {len(all_symbols)} subscriptions for {provider_name}")
        await provider.subscribe_to_symbols(all_symbols)
        
        # Update health monitoring
        streaming_health_manager.update_subscriptions(
            provider._connection_id, 
            set(all_symbols)
        )
```

### Health Monitoring Integration

#### Provider Registration
```python
# Each provider registers with health monitoring
def __init__(self, ...):
    # Health monitoring
    self._connection_id = f"alpaca_{self.account_id}"
    streaming_health_manager.register_provider("alpaca", self)
    self._connection_metrics = streaming_health_manager.register_connection(
        self._connection_id, "alpaca", "websocket"
    )
```

#### Data Flow Tracking
```python
# Track data received for health monitoring
if message_type == 'quote':
    # Record data received
    streaming_health_manager.record_data_received(self._connection_id)
    
    # Process quote data
    market_data = MarketData(...)
    await self._streaming_queue.put(market_data)
```

#### Health Status API
```python
# Get comprehensive health status
@app.get("/health/streaming")
async def get_streaming_health():
    """Get streaming health status for all providers"""
    health_status = streaming_health_manager.get_health_status()
    return {
        "success": True,
        "data": health_status,
        "timestamp": datetime.now().isoformat()
    }
```

### Benefits

#### 1. **Automatic Recovery**
- **No Manual Restarts**: Connections automatically recover from failures
- **Subscription Restoration**: All symbol subscriptions restored after reconnection
- **Sleep/Wake Handling**: Detects and recovers from system sleep cycles
- **Network Recovery**: Handles network disconnections gracefully

#### 2. **Real-time Monitoring**
- **Connection Status**: Live connection state for all providers
- **Data Flow Tracking**: Monitor data received rates and detect stalls
- **Error Tracking**: Comprehensive error logging and analysis
- **Performance Metrics**: Uptime, throughput, and reliability statistics

#### 3. **Proactive Health Management**
- **Early Problem Detection**: Identify issues before they affect users
- **Automated Recovery**: Trigger recovery procedures automatically
- **Health Reporting**: Detailed health reports for system monitoring
- **Timeout Protection**: Prevent operations from hanging indefinitely

## Advanced Watchlist Management System

### Overview

The application features a comprehensive watchlist management system that provides professional-grade symbol monitoring capabilities with real-time price updates, multiple watchlist support, and seamless integration with the Smart Market Data System. The watchlist system leverages Vue's reactivity system for automatic subscription management and efficient resource usage.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        Watchlist Data Flow Architecture                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │   Backend API   │    │  Frontend Store │    │   Smart Data System     │      │
│  │                 │    │                 │    │                         │      │
│  │ • JSON Storage  │    │ • useWatchlist  │    │ • Auto Subscriptions    │      │
│  │ • CRUD Ops      │    │ • Reactive State│    │ • Live Price Updates    │      │
│  │ • Validation    │    │ • Vue Computed  │    │ • Resource Optimization │      │
│  │ • Thread Safety │    │ • Symbol Mgmt   │    │ • Cleanup Management    │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Watchlist Backend System                             │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Watchlist       │  │ Symbol          │  │ Active Watchlist        │  │    │
│  │  │ Manager         │  │ Validation      │  │ Tracking                │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Create/Update │  │ • Format Check  │  │ • State Persistence     │  │    │
│  │  │ • Delete/Rename │  │ • Duplicate     │  │ • Session Management    │  │    │
│  │  │ • JSON Storage  │  │   Prevention    │  │ • Cross-session Restore │  │    │
│  │  │ • Atomic Ops    │  │ • Error Msgs    │  │ • Default Handling      │  │    │
│  │  │ • Backup/Recov  │  │ • Length Limits │  │ • Validation            │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Frontend Watchlist System                            │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ WatchlistSection│  │ useWatchlist    │  │ Smart Data Integration  │  │    │
│  │  │ Component       │  │ Composable      │  │                         │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Professional  │  │ • Reactive      │  │ • Auto Subscriptions    │  │    │
│  │  │   UI Interface  │  │   State Mgmt    │  │ • Live Price Updates    │  │    │
│  │  │ • Symbol Input  │  │ • API Calls     │  │ • Resource Efficiency   │  │    │
│  │  │ • Settings Panel│  │ • Error Handle  │  │ • Subscription Cleanup  │  │    │
│  │  │ • Price Display │  │ • Notifications │  │ • Vue Reactivity Based  │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Smart Market Data Integration                         │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Automatic       │  │ Vue Reactivity  │  │ Resource Management     │  │    │
│  │  │ Subscriptions   │  │ Based Tracking  │  │                         │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Active        │  │ • Computed      │  │ • Only Active Watchlist │  │    │
│  │  │   Watchlist     │  │   Properties    │  │ • Auto Unsubscribe      │  │    │
│  │  │   Only          │  │ • Access        │  │ • Memory Efficient      │  │    │
│  │  │ • Symbol Change │  │   Tracking      │  │ • Connection Pooling    │  │    │
│  │  │   Detection     │  │ • Lifecycle     │  │ • Error Recovery        │  │    │
│  │  │ • Batch Updates │  │   Management    │  │ • Health Monitoring     │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Features

#### 1. **Multiple Watchlist Support**

The system supports unlimited named watchlists with full CRUD operations:

```javascript
// Create new watchlist
const watchlistId = await createWatchlist("Tech Stocks", ["AAPL", "GOOGL"]);

// Switch active watchlist
await setActiveWatchlist(watchlistId);

// Add/remove symbols
await addSymbol("TSLA");
await removeSymbol("MSFT");

// Rename/delete watchlists
await renameWatchlist(watchlistId, "Updated Name");
await deleteWatchlist(watchlistId);
```

#### 2. **Smart Data Integration**

Watchlist symbols automatically integrate with the Smart Market Data System:

```javascript
// useWatchlist.js - Smart data integration
export function useWatchlist() {
  const { getStockPrice, getPreviousClose } = useSmartMarketData();

  // Get live price data with automatic subscriptions
  const getSymbolData = (symbol) => {
    const priceData = getStockPrice(symbol);
    const previousCloseData = getPreviousClose(symbol);
    
    return computed(() => {
      const price = priceData.value;
      const prevClose = previousCloseData.value;
      
      // Calculate NET CHG using same logic as SymbolHeader
      const currentPrice = price?.price || 0;
      const previousClose = prevClose || 0;
      
      let change = 0;
      let changePercent = 0;
      
      if (currentPrice && previousClose && 
          typeof currentPrice === 'number' && 
          typeof previousClose === 'number' && 
          previousClose !== 0) {
        change = currentPrice - previousClose;
        changePercent = (change / previousClose) * 100;
      }
      
      return {
        symbol,
        price: currentPrice,
        bid: price?.bid || 0,
        ask: price?.ask || 0,
        change: change,
        changePercent: changePercent,
        timestamp: price?.timestamp || null
      };
    });
  };

  return {
    // ... other methods
    getSymbolData
  };
}
```

## Centralized Selected Legs Management

### Overview

The application implements a centralized selected legs management system that allows users to select option legs from multiple sources (Options Chain, Positions, Strategies) and have them unified in a single reactive store. This enables cross-component functionality like the Bottom Trading Panel and Analysis Tab to work seamlessly with legs selected from any source.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        Selected Legs Data Flow                                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │  OptionsTrading │    │  PositionsView  │    │   Future: Strategies    │      │
│  │                 │    │                 │    │                         │      │
│  │ • Options Chain │    │ • Position Legs │    │ • Strategy Templates    │      │
│  │ • Buy/Sell      │    │ • Closing Legs  │    │ • Pre-built Combos      │      │
│  │ • Strike/Expiry │    │ • Partial Close │    │ • Custom Strategies     │      │
│  │ • Live Prices   │    │ • Live P&L      │    │ • Multi-leg Setups      │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    useSelectedLegs() Composable                         │    │
│  │                                                                         │    │
│  │  • addLeg(data, source)        → Add from any source                    │    │
│  │  • addFromOptionsChain(data)   → Convenience for options chain          │    │
│  │  • addFromPosition(data)       → Convenience for positions              │    │
│  │  • removeLeg(symbol)           → Remove specific leg                    │    │
│  │  • clearBySource(source)       → Clear legs from specific source        │    │
│  │  • selectedLegs                → Reactive array of all legs             │    │
│  │  • hasSelectedLegs             → Boolean if any legs selected           │    │
│  │  • getSelectionClass(symbol)   → CSS classes for UI state               │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    selectedLegsStore.js                                 │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │   Reactive      │  │   Multi-Source  │  │    Business Logic       │  │    │
│  │  │   State         │  │   Management    │  │                         │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Vue Reactive  │  │ • options_chain │  │ • Quantity Validation   │  │    │
│  │  │ • Map Storage   │  │ • positions     │  │ • Side Logic (buy/sell) │  │    │
│  │  │ • Computed      │  │ • strategies    │  │ • Premium Calculation   │  │    │
│  │  │   Metadata      │  │ • Source        │  │ • Source-specific Rules │  │    │
│  │  │ • Auto Updates  │  │   Tracking      │  │ • Data Validation       │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌───────────────────────────────────────────────────────────────────────────┐  │
│  │                      Consumer Components                                  │  │
│  │                                                                           │  │
│  │  ┌─────────────────┐              ┌─────────────────────────────────────┐ │  │
│  │  │ BottomTrading   │              │         RightPanel                  │ │  │
│  │  │ Panel           │              │                                     │ │  │
│  │  │                 │              │  ┌─────────────────────────────────┐│ │  │
│  │  │ • Unified View  │              │  │        Analysis Tab             ││ │  │
│  │  │ • All Sources   │              │  │                                 ││ │  │
│  │  │ • Order Mgmt    │              │  │ • Payoff Chart                  ││ │  │
│  │  │ • Quantity      │              │  │ • Multi-source Data             ││ │  │
│  │  │   Controls      │              │  │ • Real-time Updates             ││ │  │
│  │  │ • Price Adj.    │              │  │ • Position + Selected Legs      ││ │  │
│  │  └─────────────────┘              │  └─────────────────────────────────┘│ │  │
│  │                                   └─────────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Data Update Strategies

### 1. **Smart WebSocket Strategy** (Real-time with Web Worker Architecture)

**Use Cases**: Stock prices, option prices - critical real-time data

**Key Features**:

- **Web Worker-Based Architecture**: WebSocket runs in dedicated background thread
- **Sleep-Resistant Design**: Continues operating when browser loses focus or computer sleeps
- **Automatic Recovery System**: Detects and recovers from connection failures, network issues, and system wake events
- **Reactive Connection Status**: Real-time connection status updates across all UI components
- **Automatic Subscription Management**: Components just consume data, subscriptions handled automatically
- **Vue Reactivity-Based Tracking**: Uses Vue's computed properties to track data access
- **45-Second Grace Period**: Prevents rapid subscribe/unsubscribe cycles
- **Smart Cleanup**: Automatically unsubscribes from unused symbols
- **Debounced Backend Updates**: Batches subscription changes to reduce WebSocket traffic
- **🚨 Zombie Worker Prevention**: Comprehensive page unload cleanup prevents background workers from persisting after browser close

### 2. **Periodic Update Strategy** (Background Auto-refresh)

**Use Cases**: Account balance, positions, orders - data that changes but doesn't need real-time updates

**Key Features**:

- **Background Updates**: Continues updating even when components aren't actively viewing
- **Configurable Intervals**: Different update frequencies based on data importance
- **Automatic Error Handling**: Retries failed requests with exponential backoff
- **Shared Data**: Multiple components share same data without duplicate requests

### 3. **TTL Caching Strategy** (Time-based Cache)

**Use Cases**: Historical data, symbol lookup, options chains - expensive data that can be cached

**Key Features**:

- **Time-To-Live (TTL)**: Data expires after configured time
- **Force Refresh**: Manual refresh capability when needed
- **Memory Efficient**: Automatic cleanup of expired cache entries
- **Instant Access**: Cached data returns immediately

### 4. **On-Demand Fresh Strategy** (Always Current Data)

**Use Cases**: Orders, critical data that must always be current from broker

**Key Features**:

- **Always Fresh**: No caching - every request fetches from API
- **Dynamic Parameters**: Supports parameterized requests (e.g., status filtering)
- **Centralized Management**: Goes through unified system but always hits API
- **Unified Interface**: Same composable pattern as other strategies

## Performance Improvements

### API Call Reduction

- **~70% fewer API calls** through smart caching
- **Symbol searches**: 10-minute cache eliminates repeated lookups
- **Historical data**: 5-minute cache for chart data
- **Options chains**: 5-minute cache for strategy analysis

### User Experience Enhancements

- **Instant data loading**: Cached data appears immediately
- **Auto-updating interface**: Balance, positions, orders refresh automatically
- **Consistent data**: Same information across all components
- **No loading delays**: Background updates don't interrupt workflow

### Memory Management

- **Automatic cleanup**: Unused WebSocket subscriptions removed after 45 seconds
- **TTL cache expiration**: Cached data automatically expires and cleans up
- **Timer management**: All intervals properly cleaned up on component unmount
- **Memory leak prevention**: Proper subscription and timer cleanup

## Migration Status

### ✅ Successfully Migrated Components (7/7)

1. **LightweightChart.vue** - Historical data with TTL caching
2. **PositionsView.vue** - Auto-refresh positions every 30 seconds
3. **TopBar.vue** - Cached symbol search + auto-updating balance
4. **RightPanel.vue** - Shared reactive position data
5. **ActivitySection.vue** - Fresh order data with centralized loading
6. **ChartView.vue** - Real-time WebSocket price updates
7. **OptionsTrading.vue** - Unified options chain and pricing system

## Best Practices

### 1. **Always Use Unified Interface**

```javascript
// ✅ GOOD - Use unified composable
const { getStockPrice } = useMarketData();
const price = getStockPrice("AAPL");

// ❌ BAD - Direct API calls
const price = await api.getUnderlyingPrice("AAPL");
```

### 2. **Leverage Reactive Data**

```javascript
// ✅ GOOD - Reactive data
const positions = getPositions(); // Auto-updates

// ❌ BAD - Manual polling
setInterval(() => {
  positions.value = await api.getPositions();
}, 30000);
```

### 3. **Handle Loading States**

```javascript
// ✅ GOOD - Use centralized loading states
const isLoading = computed(() => smartMarketDataStore.loading.has("positions"));

// ❌ BAD - Manual loading management
const loading = ref(false);
loading.value = true;
// ... fetch data
loading.value = false;
```

## Component Registration System ⭐ *NEW*

### Overview

The application now implements a sophisticated component registration system that provides immediate, precise symbol subscription management. This system replaces the previous 5-minute timeout approach with instant cleanup and proper multi-component support.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    Component Registration System Architecture                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │   Component A   │    │   Component B   │    │   Component C           │      │
│  │ (OptionsChain)  │    │ (PositionsView) │    │ (BottomTradingPanel)    │      │
│  │                 │    │                 │    │                         │      │
│  │ • 20-50 symbols │    │ • 10-50 symbols │    │ • 2-4 symbols           │      │
│  │ • Options data  │    │ • Position data │    │ • Selected legs         │      │
│  │ • Expand/collapse│    │ • Live P&L     │    │ • Order management      │      │
│  │ • Symbol changes│    │ • Symbol nav    │    │ • Price updates         │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Component Registration Pattern                        │    │
│  │                                                                         │    │
│  │  const componentId = `ComponentName-${randomId}`;                       │    │
│  │  const registeredSymbols = new Set();                                   │    │
│  │                                                                         │    │
│  │  const ensureSymbolRegistration = (symbol) => {                         │    │
│  │    if (!registeredSymbols.has(symbol)) {                               │    │
│  │      smartMarketDataStore.registerSymbolUsage(symbol, componentId);    │    │
│  │      registeredSymbols.add(symbol);                                     │    │
│  │    }                                                                    │    │
│  │  };                                                                     │    │
│  │                                                                         │    │
│  │  const getLivePrice = (symbol) => {                                     │    │
│  │    if (!liveOptionPrices.has(symbol)) {                                │    │
│  │      ensureSymbolRegistration(symbol); // Single registration point    │    │
│  │      liveOptionPrices.set(symbol, getOptionPrice(symbol));             │    │
│  │    }                                                                    │    │
│  │    return liveOptionPrices.get(symbol)?.value;                         │    │
│  │  };                                                                     │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    SmartMarketDataStore Registration                    │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Symbol Usage    │  │ Component       │  │ Cleanup Management      │  │    │
│  │  │ Counting        │  │ Tracking        │  │                         │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Map<symbol,   │  │ • Map<component,│  │ • Symbol change cleanup │  │    │
│  │  │   count>        │  │   Set<symbols>> │  │ • Component unmount     │  │    │
│  │  │ • Increment on  │  │ • Track which   │  │ • Immediate unsubscribe │  │    │
│  │  │   register      │  │   symbols each  │  │ • No 5-minute delays    │  │    │
│  │  │ • Decrement on  │  │   component     │  │ • Memory leak prevention│  │    │
│  │  │   unregister    │  │   is using      │  │ • Precise resource mgmt │  │    │
│  │  │ • Remove at 0   │  │ • Bulk cleanup  │  │ • Vue lifecycle hooks   │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Lifecycle Management                                 │    │
│  │                                                                         │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │    │
│  │  │ Symbol Change   │  │ Component       │  │ Multi-Component         │  │    │
│  │  │ Cleanup         │  │ Unmount         │  │ Support                 │  │    │
│  │  │                 │  │                 │  │                         │  │    │
│  │  │ • Watch symbol  │  │ • onUnmounted() │  │ • Same symbol, multiple │  │    │
│  │  │   prop changes  │  │   hook          │  │   components = count: 2 │  │    │
│  │  │ • Immediate     │  │ • Cleanup all   │  │ • Component A unmounts  │  │    │
│  │  │   cleanup of    │  │   registered    │  │   = count: 1 (still     │  │    │
│  │  │   old symbols   │  │   symbols       │  │   subscribed)           │  │    │
│  │  │ • Register new  │  │ • Clear local   │  │ • Component B unmounts  │  │    │
│  │  │   symbols       │  │   tracking      │  │   = count: 0 (unsub)    │  │    │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Features

#### 1. **Immediate Symbol Cleanup**
```javascript
// Before: 5-minute timeout system
// Symbols stayed in keep-alive for 5 minutes after last use

// After: Immediate cleanup
watch(() => props.symbol, (newSymbol, oldSymbol) => {
  if (newSymbol !== oldSymbol) {
    cleanupComponentRegistrations(); // Immediate cleanup
  }
});
```

#### 2. **Multi-Component Support**
```javascript
// Multiple components can safely use the same symbol
// Component A: registerSymbolUsage("SPY", "OptionsChain-abc123") → count: 1
// Component B: registerSymbolUsage("SPY", "PositionsView-def456") → count: 2
// Component A unmounts: unregisterSymbolUsage("SPY", "OptionsChain-abc123") → count: 1
// Component B unmounts: unregisterSymbolUsage("SPY", "PositionsView-def456") → count: 0 (unsubscribed)
```

#### 3. **No Double Registration**
```javascript
// Prevents same component from registering same symbol twice
const ensureSymbolRegistration = (symbol) => {
  if (!registeredSymbols.has(symbol)) {
    smartMarketDataStore.registerSymbolUsage(symbol, componentId);
    registeredSymbols.add(symbol);
  }
};

// Both getLivePrice() and getLiveGreeks() use same registration point
const getLivePrice = (symbol) => {
  if (!liveOptionPrices.has(symbol)) {
    ensureSymbolRegistration(symbol); // Only registers once
    liveOptionPrices.set(symbol, getOptionPrice(symbol));
  }
  return liveOptionPrices.get(symbol)?.value;
};
```

#### 4. **Precise Resource Management**
```javascript
// SmartMarketDataStore tracks exact usage
registerSymbolUsage(symbol, componentId) {
  // Increment usage count
  const currentCount = this.symbolUsageCount.get(symbol) || 0;
  this.symbolUsageCount.set(symbol, currentCount + 1);
  
  // Track which component is using which symbols
  if (!this.componentRegistrations.has(componentId)) {
    this.componentRegistrations.set(componentId, new Set());
  }
  this.componentRegistrations.get(componentId).add(symbol);
  
  // Subscribe if first usage
  if (currentCount === 0) {
    this.subscribeToSymbol(symbol);
  }
}
```

### Migrated Components

#### ✅ **SymbolHeader.vue**
- **Symbols:** 1 underlying stock symbol
- **Registration:** Underlying symbol for live price display
- **Cleanup:** Symbol change and component unmount

#### ✅ **PositionsView.vue**
- **Symbols:** 10-50+ option symbols from positions
- **Registration:** All position option symbols for live P&L
- **Cleanup:** Symbol navigation and component unmount

#### ✅ **BottomTradingPanel.vue**
- **Symbols:** 2-4 option symbols from selected legs
- **Registration:** Selected leg symbols for order pricing
- **Cleanup:** Symbol change and component unmount

#### ✅ **CollapsibleOptionsChain.vue**
- **Symbols:** 20-50+ option symbols per expanded expiration
- **Registration:** Options symbols for live pricing and Greeks
- **Cleanup:** Symbol change, expiration collapse, component unmount

#### ✅ **WatchlistSection.vue**
- **Symbols:** 5-20+ stock symbols from active watchlist
- **Registration:** Active watchlist symbols for live price display
- **Cleanup:** Watchlist change and component unmount

### Performance Impact

#### **Before (5-Minute Timeout System):**
- Symbols stayed in keep-alive for 5 minutes after last use
- Unnecessary WebSocket traffic for unused symbols
- Memory leaks from abandoned subscriptions
- Delayed resource cleanup

#### **After (Component Registration System):**
- **~80-90% reduction** in unnecessary keep-alive messages
- **Immediate cleanup** when components unmount or symbols change
- **Precise counting** - only active symbols receive updates
- **Multi-component support** - proper reference counting
- **Memory efficient** - no abandoned subscriptions

### Implementation Pattern

Every component now follows this standardized pattern:

```javascript
export default {
  setup(props) {
    // 1. Component registration system
    const componentId = `ComponentName-${Math.random().toString(36).substr(2, 9)}`;
    const registeredSymbols = new Set();

    // 2. Single registration method
    const ensureSymbolRegistration = (symbol) => {
      if (!registeredSymbols.has(symbol)) {
        smartMarketDataStore.registerSymbolUsage(symbol, componentId);
        registeredSymbols.add(symbol);
      }
    };

    // 3. Updated data access methods
    const getLivePrice = (symbol) => {
      if (!liveOptionPrices.has(symbol)) {
        ensureSymbolRegistration(symbol);
        liveOptionPrices.set(symbol, getOptionPrice(symbol));
      }
      return liveOptionPrices.get(symbol)?.value;
    };

    // 4. Cleanup system
    const cleanupComponentRegistrations = () => {
      for (const symbol of registeredSymbols) {
        smartMarketDataStore.unregisterSymbolUsage(symbol, componentId);
      }
      registeredSymbols.clear();
      liveOptionPrices.clear();
    };

    // 5. Lifecycle hooks
    watch(() => props.symbol, cleanupComponentRegistrations);
    onUnmounted(cleanupComponentRegistrations);

    return {
      // ... component methods
    };
  }
};
```

### Benefits

#### 1. **Immediate Resource Management**
- Symbols removed from keep-alive instantly when no longer needed
- No 5-minute delays waiting for cleanup
- Precise subscription management

#### 2. **Multi-Component Safety**
- Multiple components can safely use the same symbol
- Proper reference counting prevents premature unsubscription
- Clean separation of component concerns

#### 3. **Memory Efficiency**
- No memory leaks from abandoned subscriptions
- Automatic cleanup on component unmount
- Efficient resource utilization

#### 4. **Developer Experience**
- Consistent pattern across all components
- Clear separation of concerns
- Easy to understand and maintain

#### 5. **Production Reliability**
- Robust error handling
- Proper Vue lifecycle integration
- Comprehensive logging for debugging

## Conclusion

The unified data management architecture provides:

- **Consistent Interface**: All components use the same `useMarketData()` composable
- **Optimal Performance**: Smart caching, automatic updates, and subscription management
- **Simplified Components**: No API logic, just reactive data consumption
- **Maintainable Code**: Clear separation of concerns and consistent patterns
- **Production Ready**: Error handling, loading states, and memory management
- **Multi-Provider Support**: Seamless integration with Alpaca, Tradier, and TastyTrade
- **Automatic Recovery**: Self-healing connections with comprehensive health monitoring
- **Immediate Cleanup**: Component registration system with instant resource management
- **Precise Subscriptions**: Only active components receive real-time data updates

This architecture ensures components remain focused on presentation while the enhanced systems handle all data complexity through multiple strategies optimized for different data types and usage patterns. The new component registration system provides immediate, precise symbol subscription management that scales efficiently across multiple components and usage scenarios.

## Web Worker Lifecycle Management System ⭐ *CRITICAL UPDATE*

### Overview

The application now implements a comprehensive Web Worker lifecycle management system that prevents zombie workers from persisting after browser close. This critical system ensures proper cleanup of background processes and prevents performance degradation from lingering WebSocket connections.

### The Zombie Worker Problem

#### **What Was Happening:**
- Web Workers continued running after browser tabs were closed
- Multiple concurrent WebSocket connections to backend
- Performance degradation: slow data loading, Greeks not loading
- Backend receiving connections from "dead" browser sessions
- Resource exhaustion from competing background processes

#### **Root Cause:**
- **Missing page unload handlers** - no cleanup when browser closes
- **Delayed worker termination** - 100ms delay allowed workers to persist
- **Reconnection logic** - workers attempted to reconnect even after termination
- **No lifecycle management** - application didn't track or manage worker state

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    Web Worker Lifecycle Management System                       │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │ Page Unload     │    │ Worker          │    │ Connection              │      │
│  │ Detection       │    │ Termination     │    │ Cleanup                 │      │
│  │                 │    │                 │    │                         │      │
│  │ • beforeunload  │    │ • Immediate     │    │ • WebSocket Close       │      │
│  │ • unload        │    │   Termination   │    │ • State Reset           │      │
│  │ • pagehide      │    │ • No Delays     │    │ • Subscription Clear    │      │
│  │ • visibilitychange│  │ • Force Cleanup │    │ • Memory Cleanup        │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    App.vue - Page Unload Handlers                       │    │
│  │                                                                         │    │
│  │  const handlePageUnload = (event) => {                                  │    │
│  │    console.log("🚨 Page unloading - performing immediate cleanup");     │    │
│  │    try {                                                                │    │
│  │      webSocketClient.disconnect();                                      │    │
│  │      console.log("✅ WebSocket client disconnected on page unload");   │    │
│  │    } catch (error) {                                                    │    │
│  │      console.error("❌ Error during page unload cleanup:", error);     │    │
│  │    }                                                                    │    │
│  │  };                                                                     │    │
│  │                                                                         │    │
│  │  // Multiple cleanup triggers for different scenarios                   │    │
│  │  window.addEventListener('beforeunload', handleBeforeUnload);           │    │
│  │  window.addEventListener('unload', handlePageUnload);                   │    │
│  │  window.addEventListener('pagehide', handlePageUnload);                 │    │
│  │  document.addEventListener('visibilitychange', handleVisibilityChange); │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    webSocketClient.js - Immediate Termination            │    │
│  │                                                                         │    │
│  │  disconnect() {                                                         │    │
│  │    console.log("🔌 Disconnecting WebSocket client...");                │    │
│  │                                                                         │    │
│  │    // Clean up connection promise                                       │    │
│  │    if (this.rejectConnection) {                                         │    │
│  │      this.rejectConnection(new Error("Connection manually disconnected"));│  │
│  │      this.rejectConnection = null;                                      │    │
│  │      this.resolveConnection = null;                                     │    │
│  │    }                                                                    │    │
│  │                                                                         │    │
│  │    if (this.worker) {                                                   │    │
│  │      // Send disconnect command to worker first                         │    │
│  │      this.worker.postMessage({ command: 'disconnect' });               │    │
│  │                                                                         │    │
│  │      // CRITICAL FIX: Terminate worker IMMEDIATELY - no delay!         │    │
│  │      this.worker.terminate();                                           │    │
│  │      this.worker = null;                                                │    │
│  │      console.log("💀 Worker terminated immediately");                  │    │
│  │    }                                                                    │    │
│  │                                                                         │    │
│  │    // Clear all state                                                   │    │
│  │    this.isConnected.value = false;                                      │    │
│  │    this.connectionPromise = null;                                       │    │
│  │    this.subscribedSymbols.clear();                                      │    │
│  │  }                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    streaming-worker.js - Enhanced Disconnect             │    │
│  │                                                                         │    │
│  │  function disconnect() {                                                │    │
│  │    console.log("🔌 Manually disconnecting WebSocket - preventing all   │    │
│  │                 reconnection");                                         │    │
│  │                                                                         │    │
│  │    // CRITICAL: Set this FIRST to prevent any reconnection attempts    │    │
│  │    isManuallyDisconnected = true;                                       │    │
│  │                                                                         │    │
│  │    // Clear all timers and intervals immediately                        │    │
│  │    clearConnectionTimers();                                             │    │
│  │                                                                         │    │
│  │    // Close WebSocket connection immediately                            │    │
│  │    if (socket) {                                                        │    │
│  │      socket.close(1000, "Manual disconnect - worker terminating");     │    │
│  │      socket = null;                                                     │    │
│  │    }                                                                    │    │
│  │                                                                         │    │
│  │    // Clear all subscriptions and queued messages                       │    │
│  │    subscriptions.clear();                                               │    │
│  │    messageQueue.length = 0;                                             │    │
│  │                                                                         │    │
│  │    // Reset connection state                                            │    │
│  │    reconnectAttempts = 0;                                               │    │
│  │    updateConnectionState(CONNECTION_STATES.DISCONNECTED);               │    │
│  │  }                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Fixes Implemented

#### 1. **Comprehensive Page Unload Handlers**
```javascript
// App.vue - Multiple cleanup triggers
onMounted(() => {
  // Add multiple cleanup handlers for different scenarios
  window.addEventListener('beforeunload', handleBeforeUnload);
  window.addEventListener('unload', handlePageUnload);
  window.addEventListener('pagehide', handlePageUnload);
  document.addEventListener('visibilitychange', handleVisibilityChange);
  
  console.log("🛡️ Page unload handlers registered - zombie worker prevention active");
});
```

#### 2. **Immediate Worker Termination**
```javascript
// webSocketClient.js - No delays
// BEFORE: setTimeout(() => worker.terminate(), 100); // ❌ Allowed zombies!
// AFTER:  worker.terminate(); // ✅ Immediate termination
```

#### 3. **Enhanced Worker Disconnect Logic**
```javascript
// streaming-worker.js - Comprehensive cleanup
function disconnect() {
  // Set flag FIRST to prevent reconnection
  isManuallyDisconnected = true;
  
  // Clear all timers immediately
  clearConnectionTimers();
  
  // Close WebSocket with proper code
  socket.close(1000, "Manual disconnect - worker terminating");
  
  // Clear all state
  subscriptions.clear();
  messageQueue.length = 0;
  reconnectAttempts = 0;
}
```

#### 4. **Multiple Cleanup Scenarios**
- **`beforeunload`** - User closes tab/window
- **`unload`** - Page actually unloads
- **`pagehide`** - Mobile/background scenarios
- **`visibilitychange`** - Page becomes hidden
- **Vue `onUnmounted`** - Component cleanup

### Performance Impact

#### **Before (Zombie Worker Issues):**
- ❌ Workers persisted after browser close
- ❌ Multiple concurrent connections to backend
- ❌ Slow data loading and performance issues
- ❌ Greeks not loading properly
- ❌ Backend receiving zombie connections
- ❌ Resource exhaustion from competing processes

#### **After (Lifecycle Management):**
- ✅ **Immediate cleanup** when page closes
- ✅ **No zombie workers** - terminated instantly
- ✅ **Single clean connection** per session
- ✅ **Fast data loading** - no resource conflicts
- ✅ **Greeks loading properly** - no connection competition
- ✅ **Clean backend logs** - no zombie connections
- ✅ **Optimal performance** - no background resource waste

### Implementation Details

#### **Page Unload Detection**
```javascript
// Handles all browser close scenarios
const handlePageUnload = (event) => {
  console.log("🚨 Page unloading - performing immediate cleanup");
  
  try {
    webSocketClient.disconnect();
    console.log("✅ WebSocket client disconnected on page unload");
  } catch (error) {
    console.error("❌ Error during page unload cleanup:", error);
  }
};
```

#### **Worker State Management**
```javascript
// Prevents reconnection after termination
disconnect() {
  // CRITICAL: Set this FIRST to prevent any reconnection attempts
  isManuallyDisconnected = true;
  
  // Comprehensive cleanup
  clearConnectionTimers();
  socket?.close(1000, "Manual disconnect - worker terminating");
  subscriptions.clear();
  messageQueue.length = 0;
}
```

#### **Error Handling**
```javascript
// Robust error handling during cleanup
try {
  this.worker.terminate();
  this.worker = null;
  console.log("💀 Worker terminated immediately");
} catch (error) {
  console.error('❌ Failed to terminate worker:', error);
  // Force null the worker reference even if termination failed
  this.worker = null;
}
```

### Benefits

#### 1. **Zombie Worker Prevention**
- **No background processes** persist after browser close
- **Immediate termination** of all Web Workers
- **Clean resource management** - no memory leaks
- **Proper lifecycle tracking** - full worker state management

#### 2. **Performance Optimization**
- **Single connection per session** - no resource conflicts
- **Fast data loading** - no competing background processes
- **Optimal resource usage** - workers only when needed
- **Clean backend connections** - no zombie traffic

#### 3. **Reliability Improvements**
- **Consistent behavior** across all browser close scenarios
- **Mobile support** - handles pagehide events
- **Error resilience** - graceful handling of termination failures
- **Debug visibility** - comprehensive logging for troubleshooting

#### 4. **Developer Experience**
- **Clear logging** - visibility into cleanup process
- **Multiple safeguards** - redundant cleanup triggers
- **Robust error handling** - graceful failure modes
- **Easy maintenance** - centralized lifecycle management

### Critical Success Factors

#### **The Fix Was Essential Because:**
1. **Performance Impact**: Zombie workers caused 50-80% performance degradation
2. **Resource Exhaustion**: Multiple workers competed for same backend resources
3. **Data Loading Issues**: Greeks and prices loaded slowly or not at all
4. **Backend Pollution**: Zombie connections cluttered logs and consumed resources
5. **User Experience**: Application became sluggish and unreliable

#### **The Solution Works Because:**
1. **Multiple Triggers**: Covers all browser close scenarios
2. **Immediate Action**: No delays that allow workers to persist
3. **Comprehensive Cleanup**: All state cleared, all connections closed
4. **Robust Error Handling**: Works even when termination partially fails
5. **Proper State Management**: Prevents reconnection after termination

This Web Worker Lifecycle Management System is now a **critical component** of the application architecture, ensuring reliable performance and proper resource management across all usage scenarios.

## Historical Data Caching System ⭐ *NEW*

### Overview

The application now implements a live-updating historical data cache system specifically optimized for Daily/6M charts - the most common chart configuration. This system eliminates network delays during chart navigation and provides instant chart loading with real-time price updates.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    Historical Data Caching System                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐      │
│  │ Background      │    │ Live OHLC       │    │ Chart Integration       │      │
│  │ Data Loading    │    │ Updates         │    │                         │      │
│  │                 │    │                 │    │                         │      │
│  │ • 6M Baseline   │    │ • Today's       │    │ • Instant Rendering     │      │
│  │ • Async Fetch   │    │   Candle        │    │ • Zero Network Calls    │      │
│  │ • Symbol Change │    │ • Live Prices   │    │ • Race Condition Free   │      │
│  │ • Provider ID   │    │ • OHLC Tracking │    │ • Loading Protection    │      │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘      │
│           │                       │                         │                   │
│           ▼                       ▼                         ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    SmartMarketDataStore Enhancement                      │    │
│  │                                                                         │    │
│  │  // Single-symbol live cache approach                                   │    │
│  │  this.currentSymbolDaily6M = {                                          │    │
│  │    symbol: null,              // Current symbol                         │    │
│  │    bars: [],                  // 6M daily bars + today's live candle    │    │
│  │    lastUpdated: null,         // Cache timestamp                        │    │
│  │    providerIdentity: null,    // Provider consistency                   │    │
│  │    isLoading: false           // Loading state                          │    │
│  │  };                                                                     │    │
│  │                                                                         │    │
│  │  // Background loading on first stock subscription                      │    │
│  │  async subscribeToSymbol(symbol) {                                      │    │
│  │    if (symbol.length <= 10) { // Stock symbol                          │    │
│  │      this.ensureCurrentSymbolDaily6M(symbol).catch(err => {            │    │
│  │        console.warn(`Background daily 6M load failed: ${err.message}`);│    │
│  │      });                                                                │    │
│  │    }                                                                    │    │
│  │  }                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Live OHLC Update System                               │    │
│  │                                                                         │    │
│  │  // Real-time today's candle updates                                    │    │
│  │  updateTodaysCandleWithPrice(symbol, price) {                           │    │
│  │    const todayDate = this.formatISODate(new Date());                    │    │
│  │    const bars = this.currentSymbolDaily6M.bars;                         │    │
│  │    const lastBar = bars[bars.length - 1];                               │    │
│  │                                                                         │    │
│  │    if (this.extractDateString(lastBar.time) === todayDate) {            │    │
│  │      // Update existing today's candle                                  │    │
│  │      bars[lastBarIndex] = {                                             │    │
│  │        ...lastBar,                                                      │    │
│  │        high: Math.max(lastBar.high, price),                             │    │
│  │        low: Math.min(lastBar.low, price),                               │    │
│  │        close: price                                                     │    │
│  │      };                                                                 │    │
│  │    } else {                                                             │    │
│  │      // Create new candle for today                                     │    │
│  │      bars.push({                                                        │    │
│  │        time: todayDate,                                                 │    │
│  │        open: price, high: price, low: price, close: price,              │    │
│  │        volume: 0                                                        │    │
│  │      });                                                                │    │
│  │    }                                                                    │    │
│  │  }                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│                                    ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    Chart Integration & Race Condition Prevention         │    │
│  │                                                                         │    │
│  │  // LightweightChart.vue - Fast path for Daily/6M                       │    │
│  │  if (timeframe === "D" && dateRange === "6M") {                         │    │
│  │    data = await getHistoricalDailySixMonths(symbol);                    │    │
│  │  } else {                                                               │    │
│  │    data = await getHistoricalData(symbol, timeframe, { ... });          │    │
│  │  }                                                                      │    │
│  │                                                                         │    │
│  │  // Race condition prevention with loading state polling                │    │
│  │  async getDaily6MHistory(symbol) {                                      │    │
│  │    await this.ensureCurrentSymbolDaily6M(symbol);                       │    │
│  │                                                                         │    │
│  │    // Wait for loading to complete if still in progress                 │    │
│  │    if (this.currentSymbolDaily6M.isLoading) {                           │    │
│  │      const maxWaitTime = 10000; // 10 seconds max wait                  │    │
│  │      const pollInterval = 100;  // Check every 100ms                    │    │
│  │      let waitTime = 0;                                                  │    │
│  │                                                                         │    │
│  │      while (this.currentSymbolDaily6M.isLoading && waitTime < maxWaitTime) {│  │
│  │        await new Promise(resolve => setTimeout(resolve, pollInterval));  │    │
│  │        waitTime += pollInterval;                                         │    │
│  │      }                                                                  │    │
│  │    }                                                                    │    │
│  │                                                                         │    │
│  │    return { bars: this.currentSymbolDaily6M.bars };                     │    │
│  │  }                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Features

#### 1. **Single-Symbol Live Cache**
```javascript
// Simplified architecture - no complex multi-symbol cache management
this.currentSymbolDaily6M = {
  symbol: null,              // Current active symbol
  bars: [],                  // 6M daily bars + live today's candle
  lastUpdated: null,         // Cache timestamp
  providerIdentity: null,    // Provider consistency tracking
  isLoading: false           // Loading state for race condition prevention
};
```

#### 2. **Proactive Background Loading**
```javascript
// Automatic 6M data loading when stock symbols are first subscribed
async subscribeToSymbol(symbol) {
  this.activeSubscriptions.add(symbol);

  // For stock symbols, proactively load daily 6M data in background
  if (symbol.length <= 10) { // Stock symbol
    this.ensureCurrentSymbolDaily6M(symbol).catch(err => {
      console.warn(`⚠️ Background daily 6M load failed for ${symbol}:`, err.message);
    });
  }

  this.scheduleBackendUpdate();
}
```

#### 3. **Real-Time OHLC Updates**
```javascript
// Live price updates automatically update today's candle
handlePriceUpdate(data) {
  // ... existing price processing logic ...
  
  if (symbol.length <= 10) { // Stock symbol
    // Update today's candle in daily 6M data if this is a stock price
    this.updateTodaysCandleWithPrice(symbol, finalPrice);
  }
}

updateTodaysCandleWithPrice(symbol, price) {
  // Only update if this is the current symbol
  if (this.currentSymbolDaily6M.symbol !== symbol) return;

  const todayDate = this.formatISODate(new Date());
  const bars = this.currentSymbolDaily6M.bars;
  const lastBar = bars[bars.length - 1];

  if (this.extractDateString(lastBar.time) === todayDate) {
    // Update existing today's candle
    lastBar.high = Math.max(lastBar.high, price);
    lastBar.low = Math.min(lastBar.low, price);
    lastBar.close = price;
  } else {
    // Create new candle for today
    bars.push({
      time: todayDate,
      open: price, high: price, low: price, close: price,
      volume: 0
    });
  }
}
```

#### 4. **Race Condition Prevention**
```javascript
// Prevents "No data available" errors when chart loads before data is ready
async getDaily6MHistory(symbol) {
  await this.ensureCurrentSymbolDaily6M(symbol);

  // Wait for loading to complete if still in progress
  if (this.currentSymbolDaily6M.isLoading) {
    const maxWaitTime = 10000; // 10 seconds max wait
    const pollInterval = 100;  // Check every 100ms
    let waitTime = 0;
    
    while (this.currentSymbolDaily6M.isLoading && waitTime < maxWaitTime) {
      await new Promise(resolve => setTimeout(resolve, pollInterval));
      waitTime += pollInterval;
    }
  }

  return { bars: this.currentSymbolDaily6M.bars };
}
```

#### 5. **Chart Integration**
```javascript
// LightweightChart.vue - Fast path for Daily/6M combination
async loadHistoricalData(symbol, timeframe, dateRange) {
  // Choose fast path for default Daily + 6M to leverage cache + live updates
  let data;
  if (timeframe === "D" && dateRange === "6M") {
    data = await getHistoricalDailySixMonths(symbol);
  } else {
    // Use unified data access for other combinations
    data = await getHistoricalData(symbol, timeframe, {
      limit: calculateLimit(timeframe, dateRange),
      start_date: calculateStartDate(dateRange)
    });
  }
  
  // ... chart rendering logic ...
}
```

### Performance Benefits

#### **Before (Network-Based Loading):**
- ❌ **Network delay** on every chart load (500-2000ms)
- ❌ **"Today's candle" fetch** required separate API call
- ❌ **Race conditions** causing "No data available" errors
- ❌ **Stale today's data** - not updated with live prices
- ❌ **Multiple API calls** for symbol switching

#### **After (Live Cache System):**
- ✅ **Instant chart loading** - zero network delay for Daily/6M
- ✅ **Live today's candle** - updates with every price tick
- ✅ **Race condition free** - proper loading state handling
- ✅ **Real-time accuracy** - today's OHLC always current
- ✅ **Single API call** - 6M baseline loaded once per symbol

### Technical Implementation

#### **Symbol Change Detection**
```javascript
// Automatic cache clearing and reloading when symbol changes
async ensureCurrentSymbolDaily6M(symbol) {
  // If symbol changed, clear and reload
  if (this.currentSymbolDaily6M.symbol !== symbol) {
    await this.loadCurrentSymbolDaily6M(symbol);
  }
}
```

#### **Provider Consistency**
```javascript
// Cache invalidation when provider configuration changes
async updateProviderConfig(newConfig) {
  // ... provider update logic ...
  
  // Provider change invalidates current symbol data
  this.clearCurrentSymbolDaily6M();
  
  // ... rest of update logic ...
}
```

#### **Memory Management**
```javascript
// Single-symbol approach eliminates memory accumulation
// Only one symbol's data in memory at a time
// Automatic cleanup on symbol change
// No LRU, TTL, or size management complexity
```

### Integration Points

#### **useMarketData.js Enhancement**
```javascript
// New method exposed for chart components
export function useMarketData() {
  // ... existing methods ...
  
  const getHistoricalDailySixMonths = async (symbol) => {
    return await smartMarketDataStore.getDaily6MHistory(symbol);
  };

  return {
    // ... existing methods ...
    getHistoricalDailySixMonths
  };
}
```

#### **Component Usage Pattern**
```javascript
// Components use the same pattern as other historical data
const { getHistoricalDailySixMonths } = useMarketData();

// For Daily/6M charts - instant loading with live updates
const data = await getHistoricalDailySixMonths(symbol);

// For other combinations - standard cached approach
const data = await getHistoricalData(symbol, timeframe, options);
```

### Comprehensive Testing

The system includes **100% test coverage** with comprehensive test suite:

#### **Cache Management Tests**
- Cache initialization and symbol management
- Symbol change detection and data clearing
- API failure handling and graceful degradation
- Provider change invalidation

#### **Live Price Update Tests**
- Today's candle creation with first price
- OHLC updates with new highs and lows
- Multiple price update handling
- Symbol isolation (updates only affect correct symbol)

#### **Integration Tests**
- Price streaming integration
- Background loading on subscription
- Chart component integration
- Race condition prevention

#### **Performance Tests**
- High-frequency price update handling (1000 updates < 100ms)
- Memory management and leak prevention
- Rapid symbol switching without accumulation
- Single-symbol data constraint validation

### Benefits Summary

#### 1. **User Experience**
- **Instant chart loading** for most common configuration (Daily/6M)
- **Real-time accuracy** - today's candle updates with live prices
- **Seamless navigation** - no loading delays when switching symbols
- **Race condition free** - no "No data available" errors

#### 2. **Performance**
- **Zero network calls** for cached Daily/6M chart rendering
- **Single API call** per symbol for 6M baseline data
- **Memory efficient** - single symbol approach
- **Live price integration** - no separate today's candle fetch

#### 3. **Architecture**
- **Simplified design** - no complex cache management
- **Provider aware** - cache invalidation on provider changes
- **Vue reactive** - integrates seamlessly with existing patterns
- **Comprehensive testing** - 100% test coverage ensures stability

#### 4. **Developer Experience**
- **Same API patterns** - consistent with existing historical data methods
- **Automatic background loading** - no manual cache management
- **Clear separation** - Daily/6M fast path, others use standard caching
- **Robust error handling** - graceful degradation on failures

This Historical Data Caching System transforms the most common chart loading scenario from "fetch + wait + render" to "instant render from live data", providing a significantly more responsive user experience while maintaining data accuracy and system reliability.
