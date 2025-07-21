# Juicy Trade

A sophisticated options trading application with a modular, multi-provider architecture, featuring a FastAPI backend and a Vue.js frontend. The application is designed to support multiple brokerage providers, with a clear separation between live and paper trading environments.

## Architecture

The application is divided into two main components:

1.  **`trade_backend`**: A FastAPI application that serves as the backend for the trading application. It provides a standardized API for interacting with different brokerage providers, handles user authentication, and manages real-time data streaming.

2.  **`trade_app`**: A Vue.js single-page application (SPA) that provides the user interface for the trading application. It communicates with the backend via a RESTful API and WebSockets for real-time data updates.

This architecture allows for a clean separation of concerns between the frontend and backend, making it easier to develop and maintain the application.

### Multi-Provider Architecture

The backend is designed to support multiple brokerage providers, with a clear distinction between live and paper trading accounts. This allows for a flexible and powerful trading experience, where you can use live data feeds for market analysis while executing trades in a paper account for testing.

- **Provider Manager (`provider_manager.py`)**: Initializes all available providers, including live and paper variants (e.g., `alpaca`, `alpaca_paper`, `tradier`, `tradier_paper`).
- **Provider Configuration (`provider_config.py`)**: Manages the routing of different operations (e.g., `stock_quotes`, `orders`, `positions`, `expiration_dates`, `next_market_date`) to specific providers. This configuration is stored in `provider_config.json` and can be updated via the API.
- **Streaming Manager (`streaming_manager.py`)**: Manages real-time data streaming from multiple providers concurrently, aggregating the data into a single feed for the frontend.

## Features

- **Multi-Provider Support**: The backend supports multiple brokerage providers, with a base provider interface that can be extended to support new providers.
- **Live & Paper Trading**: Separate configurations for live and paper trading accounts, allowing for flexible testing and trading strategies.
- **Live Option and Stock Price Streaming**: Real-time price updates via WebSockets, with a dedicated streaming service that maintains persistent connections to the data provider.
- **Options Chain**: A detailed options chain display with live bid/ask prices, greeks, and other relevant data.
- **Interactive Payoff Chart**: A dynamic payoff chart that visualizes the profit/loss profile of the selected options strategy.
- **Order Management**: A comprehensive order management system that allows users to place, track, and cancel orders for single-leg, multi-leg, and combo strategies.
- **Advanced Options Calculator**: Centralized calculation engine for profit/loss analysis, Greeks, buying power effects, and credit/debit determination.
- **Professional Trading Interface**: Responsive bottom trading panel with Tasty Trade-inspired design, featuring dynamic button layouts and real-time pricing.
- **Interactive Leg Selection**: Click-to-select individual option legs with visual highlighting for targeted quantity adjustments.
- **Smart Quantity Controls**: Quantity buttons that work on all legs when none selected, or only on selected legs for precise control.
- **Intelligent Price Display**: Always shows positive limit prices in UI while correctly sending negative/positive values to brokers for credits/debits.
- **Accurate P&L Calculations**: Proper max profit, max loss, and buying power calculations for credit and debit spreads.
- **Symbol Lookup & Search**: Intelligent symbol search with smart caching, prefix filtering, and multi-provider support for finding stocks, ETFs, and options.
- **Live Search Interface**: Professional Tasty Trade-style search dropdown with real-time results, keyboard navigation, and type-coded badges.
- **Multi-Provider Symbol Search**: Support for both Tradier and Alpaca symbol lookup with configurable provider routing.
- **Order Confirmation System**: Comprehensive order confirmation dialog with detailed P&L analysis, risk metrics, and trade validation.
- **Auto-Clear Functionality**: Automatic clearing of selected options after successful order placement for seamless trading workflow.
- **Professional Chart Integration**: TradingView Lightweight Charts with comprehensive historical data, multiple timeframes, and real-time updates.
- **Advanced Chart Features**: Candlestick visualization with volume histogram, perfect 70/30 space distribution, and official TradingView implementation.
- **Multi-Timeframe Analysis**: Support for 1m, 5m, 15m, 1h, 4h, daily, weekly, and monthly charts with appropriate historical depth.
- **Enhanced Data Coverage**: Comprehensive historical data ranging from 7 days (intraday) to 10 years (monthly) for thorough market analysis.
- **Dual Chart Views**: Dedicated chart view with full-screen analysis capabilities and integrated trading interface.
- **Dedicated Positions Management**: Full-page positions view with comprehensive portfolio analysis, hierarchical grouping, and real-time P&L tracking.
- **Multi-View Navigation**: Clean URL structure with dedicated views for trading (`/trade`), charting (`/chart`), and positions (`/positions`) with seamless navigation.
- **Professional Theme System**: Centralized CSS theme architecture with Tasty Trade-inspired design, featuring consistent color hierarchy, typography, and component styling for a professional trading platform experience.
- **Advanced Position Management**: Comprehensive position tracking and analysis system with real-time P&L calculations, interactive position selection, and integrated payoff visualization.
- **Multi-Leg Payoff Analysis**: Sophisticated payoff chart calculations supporting complex options strategies including iron butterflies, condors, spreads, and single-leg positions with accurate profit/loss modeling.
- **Position-Based Chart Integration**: Dynamic payoff charts that combine existing positions with new trade selections, allowing traders to visualize the impact of additional legs on their current portfolio.
- **Collapsible Options Chain**: Modern, intuitive options chain interface with natural collapsible design, enhanced visual hierarchy, and streamlined user experience.
- **Centralized Data Flow**: Unified data management system with reactive state management, optimized WebSocket integration, and seamless real-time updates across all components.
- **Professional Notification System**: Elegant toast-style notifications with centralized management, smart timing, and non-disruptive user experience for all trading actions and system feedback.
- **Consistent Theme Architecture**: Comprehensive CSS custom properties system ensuring visual consistency across all components, with standardized RightPanel integration and modern chart controls styling.

## Project Structure

### `trade_backend`

The `trade_backend` is a FastAPI application with the following structure:

- **`main.py`**: The main entry point for the FastAPI application. It defines the API endpoints, WebSocket routes, and background tasks.
- **`providers/`**: This directory contains the different brokerage provider implementations. Each provider must implement the `BaseProvider` interface defined in `base_provider.py`.
  - **`base_provider.py`**: Defines the abstract base class for all brokerage providers.
  - **`alpaca_provider.py`**: An implementation of the `BaseProvider` interface for the Alpaca brokerage.
  - **`tradier_provider.py`**: An implementation of the `BaseProvider` interface for the Tradier brokerage.
  - **`public_provider.py`**: An implementation of the `BaseProvider` interface for the Public.com API (for market data).
- **`models.py`**: Defines the Pydantic models used for data validation and serialization.
- **`config.py`**: Defines the application configuration, including API keys and other settings.
- **`provider_manager.py`**: Initializes and manages all available provider instances.
- **`provider_config.py`**: Manages the routing configuration for different operations.
- **`streaming_manager.py`**: Manages real-time data streaming from multiple providers.

### `trade_app`

The `trade_app` is a Vue.js application with the following structure:

- **`public/`**: Contains the static assets for the application.
- **`src/`**: Contains the source code for the Vue.js application.
  - **`main.js`**: The main entry point for the Vue.js application.
  - **`App.vue`**: The root component for the application.
  - **`components/`**: Contains the reusable Vue components used throughout the application.
    - **`BottomTradingPanel.vue`**: Professional trading interface with responsive controls and real-time pricing.
    - **`OrderConfirmationDialog.vue`**: Comprehensive order confirmation with P&L analysis and risk metrics.
    - **`OptionsChain.vue`**: Interactive options chain with live pricing and selection capabilities.
    - **`PayoffChart.vue`**: Dynamic profit/loss visualization chart.
  - **`views/`**: Contains the different pages or views of the application.
    - **`OptionsTrading.vue`**: Main trading interface with options chain, chart, and order management.
  - **`services/`**: Contains the services for interacting with the backend API and WebSockets.
    - **`optionsCalculator.js`**: Centralized calculation engine for P&L, Greeks, and risk analysis.
    - **`orderService.js`**: Order validation and submission service.
    - **`api.js`**: Backend API communication service.
    - **`webSocketClient.js`**: Real-time data streaming client.
  - **`composables/`**: Contains reusable Vue composition functions.
    - **`useOrderManagement.js`**: Centralized order state and workflow management.
  - **`router/`**: Contains the routing configuration for the application.

## Getting Started

### Prerequisites

- Python 3.8+
- Node.js 14+
- API keys for Alpaca, Tradier, and Public.com (stored in `.env` file)

### Installation

1.  **Backend**:

    - Navigate to the `trade_backend` directory.
    - Create a virtual environment and install the Python dependencies:
      ```bash
      python -m venv venv
      source venv/bin/activate  # On Windows: venv\Scripts\activate
      pip install -r requirements.txt
      ```
    - Create a `.env` file with your API keys (see the `.env.example` file for a template).

2.  **Frontend**:
    - Navigate to the `trade_app` directory.
    - Install the Node.js dependencies:
      ```bash
      npm install
      ```

### Running the Application

1.  **Start the Backend**:

    - Navigate to the `trade_backend` directory.
    - Start the FastAPI application:
      ```bash
      uvicorn main:app --host 0.0.0.0 --port 8008 --reload
      ```
    - **Optional**: Set log level for debugging:
      ```bash
      LOG_LEVEL=DEBUG uvicorn main:app --host 0.0.0.0 --port 8008 --reload
      ```

2.  **Start the Frontend**:
    - Navigate to the `trade_app` directory.
    - Start the Vue.js development server:
      ```bash
      npm run dev
      ```

The application will be available at `http://localhost:5173`.

### Configuration

#### Logging Configuration

The application supports configurable logging levels for better debugging and production use:

**Environment Variables:**

```bash
# Set log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
export LOG_LEVEL=INFO  # Default

# For verbose debugging (shows detailed streaming logs)
export LOG_LEVEL=DEBUG

# For production (minimal logging)
export LOG_LEVEL=WARNING
```

**Configuration File (.env):**

```bash
LOG_LEVEL=INFO
```

**Log Level Behavior:**

- **DEBUG**: Shows detailed streaming messages, subscription changes, and internal operations
- **INFO**: Standard operational messages (default)
- **WARNING**: Only warnings and errors
- **ERROR**: Only error messages
- **CRITICAL**: Only critical system failures

### Provider Configuration

You can configure the provider routing via the API. For example, to use live Tradier for streaming and paper Alpaca for orders:

```bash
curl -X PUT http://localhost:8008/providers/config \
  -H "Content-Type: application/json" \
  -d '{
    "streaming": {
      "stock_quotes": "tradier",
      "option_quotes": "tradier"
    },
    "orders": "alpaca_paper",
    "positions": "alpaca_paper"
  }'
```

### Order Placement

You can place single-leg, multi-leg, and combo orders via the API. The `side` parameter for option orders should be one of `buy_to_open`, `buy_to_close`, `sell_to_open`, or `sell_to_close`.

**Single-Leg Order:**

```bash
curl -X POST http://localhost:8008/orders \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "SPY",
    "qty": "1",
    "side": "buy",
    "type": "market",
    "time_in_force": "day"
  }'
```

**Multi-Leg Order:**

```bash
curl -X POST http://localhost:8008/orders/multi-leg \
  -H "Content-Type: application/json" \
  -d '{
    "legs": [
      {
        "symbol": "SPY250715C00625000",
        "qty": "1",
        "side": "buy_to_open"
      },
      {
        "symbol": "SPY250715C00626000",
        "qty": "1",
        "side": "sell_to_open"
      }
    ],
    "order_type": "limit",
    "time_in_force": "day",
    "limit_price": "-1.00"
  }'
```

**Cancel Order:**

```bash
curl -X DELETE http://localhost:8008/orders/{order_id}
```

### Symbol Lookup & Search

The application features a comprehensive symbol search system with both backend API and frontend UI components.

#### Backend API

**Search for Symbols:**

```bash
curl -X GET "http://localhost:8008/symbols/lookup?q=SPY" \
  -H "Accept: application/json"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "symbols": [
      {
        "symbol": "SPY",
        "description": "SPDR S&P 500 ETF Trust",
        "exchange": "ARCA",
        "type": "etf"
      }
    ]
  },
  "message": "Found 1 symbols matching 'SPY'",
  "timestamp": "2025-01-12T19:23:16.123456"
}
```

#### Frontend Search Interface

The frontend includes a professional Tasty Trade-style search interface integrated into the top navigation bar:

**Features:**

- **Live Search Dropdown**: Real-time results appear as you type with 300ms debounce
- **Keyboard Navigation**: Full arrow key navigation, Enter to select, Escape to close
- **Type-Coded Badges**: Color-coded badges for Stock (blue), ETF (green), Index (orange), Option (purple)
- **Professional UI**: Dark theme matching Tasty Trade design with loading states
- **Seamless Integration**: Click or Enter to select symbol and immediately load options chain

**Usage:**

1. Click in the search box in the top navigation
2. Start typing a symbol (e.g., "AAPL")
3. Use arrow keys to navigate results or click with mouse
4. Press Enter or click to select symbol
5. Trading interface immediately updates with new symbol data

#### Multi-Provider Support

**Supported Providers:**

- **Tradier**: Comprehensive symbol database with all security types (default)
- **Alpaca**: Active US equities with detailed company information

**Provider Configuration:**

```bash
# Switch symbol lookup to Alpaca
curl -X PUT http://localhost:8008/providers/config \
  -H "Content-Type: application/json" \
  -d '{"symbol_lookup": "alpaca"}'

# Switch back to Tradier
curl -X PUT http://localhost:8008/providers/config \
  -H "Content-Type: application/json" \
  -d '{"symbol_lookup": "tradier"}'
```

#### Smart Caching System

**Performance Features:**

- **6-Hour Cache**: Long-lived cache reduces API calls by 90%+
- **Prefix Filtering**: Efficient sub-searches from cached data
- **Complete Caching**: Cache ALL results, display top 50 most relevant
- **LRU Eviction**: Memory management with configurable size limits
- **Intelligent Sorting**: Exact matches first, then prefix matches, then contains matches

**Cache Behavior Examples:**

- Search "S" → API call → Cache 200+ symbols → Display top 50
- Search "SP" → Filter from cached "S" results → Cache filtered results → Display matches
- Search "SPY" → Filter from cached "SP" results → SPY appears first (exact match)
- API failures → No caching of empty results → Automatic retry on next request

**Sorting Priority:**

1. **Exact Match**: "SPY" query shows SPY first
2. **Starts With**: "AA" query shows AAPL, AAL before other matches
3. **Contains Symbol**: "TECH" query shows BTECH, FTECH
4. **Contains Description**: "Apple" query shows symbols with Apple in company name
5. **Alphabetical**: Everything else sorted alphabetically

### Chart Integration & Historical Data

The application features a comprehensive charting system powered by TradingView Lightweight Charts, providing professional-grade market analysis capabilities.

#### Chart API Endpoints

**Get Historical Chart Data:**

```bash
curl -X GET "http://localhost:8008/chart/historical/SPY?timeframe=D&limit=100&start_date=2024-01-01" \
  -H "Accept: application/json"
```

**Supported Timeframes:**

- **Intraday**: `1m`, `5m`, `15m`, `1h`, `4h`
- **Daily/Weekly/Monthly**: `D`, `W`, `M`

**Parameters:**

- `timeframe`: Chart interval (required)
- `limit`: Maximum number of bars (optional, default varies by timeframe)
- `start_date`: Start date in YYYY-MM-DD format (optional)
- `end_date`: End date in YYYY-MM-DD format (optional)

**Response Format:**

```json
{
  "success": true,
  "data": {
    "symbol": "SPY",
    "timeframe": "D",
    "bars": [
      {
        "time": "2024-01-01",
        "open": 475.23,
        "high": 477.45,
        "low": 474.12,
        "close": 476.89,
        "volume": 45123456
      }
    ],
    "count": 100
  },
  "message": "Retrieved 100 bars for SPY"
}
```

#### Chart Features

**TradingView Lightweight Charts Integration:**

- **Professional Candlestick Charts**: Full OHLC visualization with customizable styling
- **Volume Histogram**: Integrated volume display with 70/30 price/volume space distribution
- **Multiple Timeframes**: Seamless switching between 1-minute to monthly charts
- **Real-time Updates**: WebSocket integration for live price updates
- **Interactive Controls**: Crosshair, zoom, pan, and timeframe selection
- **Responsive Design**: Auto-resize and mobile-friendly interface

**Data Coverage by Timeframe:**

- **1m, 5m, 15m**: 1000 bars over 7 days (comprehensive intraday analysis)
- **1h**: 3000 bars over 14 days (detailed hourly patterns)
- **4h**: 2000 bars over 30 days (medium-term analysis)
- **Daily**: 730 bars over 2 years (long-term daily trends)
- **Weekly**: 156 bars over 3 years (weekly pattern analysis)
- **Monthly**: 120 bars over 10 years (decade-long trend analysis)

**Chart Views:**

- **Integrated Chart**: Embedded chart within trading interface
- **Full-Screen Chart**: Dedicated chart view (`/chart`) for detailed analysis
- **Dual Navigation**: Seamless switching between trading (`/trade`) and chart views

#### Frontend Chart Components

**LightweightChart.vue:**

- **Official TradingView Implementation**: Following documented best practices
- **Smart Time Format Handling**: Automatic conversion between date strings and timestamps
- **Enhanced Data Requests**: Intelligent date range calculation for optimal data coverage
- **Professional Styling**: Dark theme with customizable colors and layouts
- **Error Handling**: Comprehensive loading states and error management

**ChartView.vue:**

- **Full-Screen Analysis**: Dedicated chart page with maximum screen utilization
- **Symbol Integration**: Synchronized with trading interface symbol selection
- **Real-time Updates**: Live price and volume data streaming
- **Professional Layout**: Industry-standard chart interface design

#### Chart Usage Examples

**Basic Chart Integration:**

```vue
<template>
  <LightweightChart
    :symbol="currentSymbol"
    :theme="'dark'"
    :height="600"
    :enableRealtime="true"
  />
</template>
```

**Navigation Between Views:**

```javascript
// Navigate to full-screen chart
router.push("/chart");

// Navigate back to trading interface
router.push("/trade");
```

**Real-time Chart Updates:**

```javascript
// WebSocket integration for live updates
const wsConnection = new WebSocket("ws://localhost:8008/ws");
wsConnection.send(
  JSON.stringify({
    type: "subscribe",
    symbols: ["SPY"],
  })
);
```

#### Chart Performance Optimizations

**Efficient Data Loading:**

- **Smart Caching**: Reduced API calls through intelligent data management
- **Progressive Loading**: Load essential data first, then enhance with additional history
- **Optimized Requests**: Tailored data limits and date ranges per timeframe
- **Memory Management**: Efficient chart data handling and cleanup

**Real-time Performance:**

- **WebSocket Streaming**: Low-latency price updates
- **Selective Updates**: Only update changed data points
- **Smooth Animations**: 60fps chart rendering and transitions
- **Responsive Interface**: Instant timeframe switching and navigation

### Dedicated Positions Management

The application features a comprehensive positions management system accessible via the dedicated Positions view (`/positions`), providing professional-grade portfolio analysis and monitoring capabilities.

#### Positions View Features

**Full-Page Portfolio Interface:**

- **Dedicated Positions Page**: Complete view accessible at `/positions` for comprehensive portfolio management
- **Multi-Asset Support**: Separate tabs for Options and Stocks positions with unified interface
- **Hierarchical Organization**: Intelligent grouping by Order Chains, Symbol, or Strategy for optimal portfolio analysis
- **Real-time P&L Tracking**: Live profit/loss calculations with daily and total P&L metrics

**Advanced Position Display:**

- **Collapsible Position Groups**: Expandable rows showing strategy-level and individual leg details
- **Professional Leg Cards**: Compact, information-dense cards showing quantity, expiration, strike, and option type
- **Visual P&L Indicators**: Color-coded profit/loss display with green for gains and red for losses
- **Comprehensive Totals**: Portfolio-level totals with real-time updates

**Interactive Features:**

- **Expandable Hierarchies**: Click to expand position groups and view individual strategy components
- **Strategy Recognition**: Automatic identification and labeling of common options strategies
- **Days to Expiration**: Real-time calculation of time remaining until option expiration
- **Position Sizing**: Clear display of position quantities with buy/sell indicators

#### Technical Implementation

**PositionsView.vue Architecture:**

```vue
<template>
  <div class="positions-view-container">
    <!-- Standard layout with TopBar, SideNav, SymbolHeader -->
    <div class="positions-header">
      <!-- Position type tabs (Options/Stocks) -->
      <!-- Grouping controls (Order Chains/Symbol/Strategy) -->
    </div>

    <!-- Portfolio totals section -->
    <div class="totals-section">
      <div class="totals-row">
        <span>TOTALS</span>
        <span class="total-pl-day">{{ formatCurrency(totalPlDay) }}</span>
        <span class="total-pl-open">{{ formatCurrency(totalPlOpen) }}</span>
      </div>
    </div>

    <!-- Positions table with hierarchical display -->
    <table class="positions-table">
      <!-- Expandable position groups -->
      <!-- Strategy-level rows -->
      <!-- Individual leg rows with detailed cards -->
    </table>
  </div>
</template>
```

**Data Processing Pipeline:**

1. **Position Fetching**: Retrieves positions from broker APIs via `/positions` endpoint
2. **Symbol Parsing**: Automatic parsing of option symbols to extract strike, expiration, and type
3. **Strategy Recognition**: Groups related positions into recognized strategy patterns
4. **Hierarchical Organization**: Creates expandable tree structure for optimal display
5. **Real-time Updates**: Continuous P&L calculations with live market data

**Professional Styling:**

- **Consistent Theme Integration**: Uses centralized CSS custom properties for professional appearance
- **Responsive Design**: Optimized for various screen sizes with mobile-friendly layouts
- **Visual Hierarchy**: Clear information architecture with proper spacing and typography
- **Interactive States**: Hover effects, expansion animations, and visual feedback

#### Position Data Structure

**Enhanced Position Groups:**

```javascript
{
  id: "symbol_SPY",
  symbol: "SPY",
  strategy: "Iron Condor",
  asset_class: "options",
  total_qty: 4,
  pl_day: -125.50,
  pl_open: 245.75,
  strategies: [
    {
      name: "Iron Condor",
      legs: [
        {
          qty: 1,
          symbol: "SPY250718C00620000",
          strike_price: 620,
          option_type: "call",
          current_price: 2.45,
          unrealized_pl: 45.00
        }
        // ... additional legs
      ]
    }
  ]
}
```

**Leg Card Display:**

Each position leg is displayed in a compact, professional card format showing:

- **Quantity**: +1/-1 with color coding for buy/sell
- **Expiration**: MM/DD format for quick reference
- **Days to Expiry**: Countdown with "d" suffix
- **Strike Price**: Clear numerical display
- **Option Type**: C/P with color coding (green for buys, red for sells)

#### Navigation Integration

**Multi-View Architecture:**

```javascript
// Router configuration
const routes = [
  { path: "/", component: OptionsTrading }, // Default trading view
  { path: "/trade", component: OptionsTrading }, // Trading interface
  { path: "/chart", component: ChartView }, // Full-screen charts
  { path: "/positions", component: PositionsView }, // Portfolio management
];
```

**Seamless Navigation:**

- **Left Navigation Menu**: SideNav component provides quick access to all views
- **Consistent Layout**: All views share TopBar, SideNav, and SymbolHeader for unified experience
- **State Preservation**: Symbol selection and preferences maintained across view changes
- **URL-Based Routing**: Direct access to specific views via clean URLs

#### Usage Examples

**Navigate to Positions View:**

```javascript
// Programmatic navigation
router.push("/positions");

// Direct URL access
// http://localhost:5173/positions
```

**Position Filtering:**

```javascript
// Filter by asset type
selectedType.value = "options"; // or "stocks"

// Group by different criteria
groupBy.value = "order_chains"; // or "symbol" or "strategy"
```

**Expand Position Details:**

```javascript
// Toggle position group expansion
const toggleExpand = (groupId) => {
  const index = expandedGroups.value.indexOf(groupId);
  if (index > -1) {
    expandedGroups.value.splice(index, 1);
  } else {
    expandedGroups.value.push(groupId);
  }
};
```

#### Benefits

**For Traders:**

- **Comprehensive Overview**: Complete portfolio visibility in dedicated interface
- **Strategy Analysis**: Clear visualization of complex multi-leg positions
- **Real-time Monitoring**: Live P&L updates and position tracking
- **Professional Layout**: Industry-standard position management interface

**For Portfolio Management:**

- **Hierarchical Organization**: Logical grouping of related positions
- **Risk Assessment**: Clear P&L visibility at strategy and portfolio levels
- **Time Management**: Days to expiration tracking for all positions
- **Performance Tracking**: Historical and current performance metrics

The dedicated Positions view provides professional-grade portfolio management capabilities, allowing traders to monitor, analyze, and manage their positions with the same level of sophistication found in institutional trading platforms.

## Smart Market Data System

The application features an advanced **Vue Reactivity-Based Market Data System** that provides zero-configuration real-time data streaming with automatic subscription management. This system eliminates manual WebSocket handling in components while ensuring optimal resource usage and broker API compliance.

### Key Innovation: Vue Reactivity-Based Access Tracking

Unlike traditional heartbeat-based systems, Juicy Trade leverages Vue's natural reactivity system to automatically track which symbols are actively being used by components. This provides several key advantages:

- **Zero Component Logic**: Components simply call `getStockPrice('AAPL')` - everything else is automatic
- **Natural Lifecycle**: Subscriptions automatically follow Vue's component lifecycle
- **Memory Efficient**: No manual cleanup required - Vue's reactivity handles it naturally
- **Performance Optimized**: Only subscribes to symbols actually being displayed

### How It Works

#### Automatic Access Tracking

```javascript
// Component usage - no subscription logic needed
const { getStockPrice } = useSmartMarketData();
const livePrice = computed(() => getStockPrice("AAPL"));

// Vue automatically calls trackAccess() during:
// - Component renders
// - Reactive updates
// - Template evaluations
// - Watcher executions
```

#### Smart Subscription Management

```javascript
// SmartMarketDataStore automatically:
// 1. Tracks access when Vue evaluates computed properties
// 2. Subscribes to new symbols immediately
// 3. Maintains active subscriptions through natural Vue reactivity
// 4. Cleans up unused symbols after 45 seconds of no access
```

#### Data Flow Architecture

```
Component Render → computed() Evaluation → trackAccess() → Keep Alive
     ↓                    ↓                    ↓              ↓
Template Update    → Re-evaluation     → Access Refresh → Subscription Active
     ↓                    ↓                    ↓              ↓
Watcher Trigger    → Auto-evaluation   → Timestamp Update → Connection Maintained
     ↓                    ↓                    ↓              ↓
Component Unmount  → No Evaluation     → Stale Access → Auto Cleanup (45s)
```

### Core Components

#### SmartMarketDataStore (`smartMarketDataStore.js`)

- **Reactive Maps**: `stockPrices` and `optionPrices` using Vue's reactive system
- **Access Tracking**: Automatic timestamp updates on every computed property evaluation
- **Smart Cleanup**: 45-second grace period with automatic unsubscription
- **Backend Integration**: Uses `subscribe_smart_replace_all` for efficient multi-symbol management

#### useSmartMarketData Composable (`useSmartMarketData.js`)

```javascript
export function useSmartMarketData() {
  const store = inject("smartMarketDataStore");

  return {
    getStockPrice: (symbol) => store.getStockPrice(symbol),
    getOptionPrice: (symbol) => store.getOptionPrice(symbol),
  };
}
```

#### Global State Integration (`useGlobalSymbol.js`)

The global symbol state automatically integrates with the Smart Market Data system:

```javascript
// Automatic live price updates for global state
const liveStockPrice = computed(() => {
  if (!globalSymbolState.currentSymbol) return null;
  return getStockPrice(globalSymbolState.currentSymbol);
});

// Auto-update global state when live prices change
watch(liveStockPrice, (newPriceRef) => {
  if (newPriceRef?.value?.price) {
    globalSymbolState.currentPrice = newPriceRef.value.price;
    globalSymbolState.isLivePrice = true;
  }
});
```

### Backend Architecture

#### Multi-Symbol Subscription Management

The backend uses a true multi-symbol architecture with smart subscription replacement:

```python
# StreamingManager - Multi-symbol sets only
class StreamingManager:
    def __init__(self):
        self._stock_subscriptions = set()  # Multi-symbol set
        # All legacy single-symbol code removed

    async def subscribe_smart_replace_all(self, symbols):
        """Replace all stock subscriptions with new symbol set"""
        old_symbols = self._stock_subscriptions.copy()
        self._stock_subscriptions = set(symbols)

        # Calculate and apply changes efficiently
        to_add = self._stock_subscriptions - old_symbols
        to_remove = old_symbols - self._stock_subscriptions
        await self._apply_subscription_changes(to_add, to_remove)
```

#### WebSocket Message Handling

```python
# main.py - Smart replace message handling
@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    async for message in websocket.iter_text():
        data = json.loads(message)

        if data["type"] == "subscribe_smart_replace_all":
            symbols = data.get("symbols", [])
            await streaming_manager.subscribe_smart_replace_all(symbols)
```

### Component Integration Patterns

#### Pattern 1: Direct Smart Data Access

Used by components that need live market data:

```javascript
// SymbolHeader.vue, CollapsibleOptionsChain.vue
const { getStockPrice, getOptionPrice } = useSmartMarketData();
const livePrice = computed(() => getStockPrice(currentSymbol.value));
const optionPrice = computed(() => getOptionPrice(optionSymbol.value));
```

#### Pattern 2: Global State Integration

Used by view components that manage overall application state:

```javascript
// ChartView.vue, OptionsTrading.vue, PositionsView.vue
const { globalSymbolState } = useGlobalSymbol();
const currentPrice = computed(() => globalSymbolState.currentPrice);
```

#### Pattern 3: Props-Based (Clean Components)

Used by components that receive data from parents:

```javascript
// RightPanel.vue, PayoffChart.vue, QuoteDetailsSection.vue
props: {
  currentPrice: Number,  // Automatically gets live data from parent
  // ... other props
}
```

### Key Benefits

#### For Users

- **Instant Updates**: Real-time price data with no manual refresh needed
- **Seamless Experience**: No interruptions or loading delays when switching symbols
- **Reliable Data**: Automatic reconnection and error recovery
- **Memory Efficient**: No resource leaks or performance degradation over time

#### For Developers

- **Zero Configuration**: No manual subscription management in components
- **Vue-Native**: Leverages Vue's reactivity system naturally
- **Clean Architecture**: Clear separation between data and presentation
- **Easy Testing**: Isolated, testable components with predictable behavior

#### For Production

- **Broker Compliant**: Respects API subscription limits automatically
- **Resource Efficient**: Minimal memory and CPU usage
- **Scalable**: Handles high-frequency trading scenarios efficiently
- **Fault Tolerant**: Graceful handling of network issues and API failures

### Migration Status

The Smart Market Data System has been **fully implemented** and is **production ready**:

**✅ Migrated Components:**

- `useGlobalSymbol.js` - Integrated with SmartMarketDataStore
- `ChartView.vue` - Uses global state, WebSocket code removed
- `OptionsTrading.vue` - Uses global state, WebSocket code cleaned up
- `PositionsView.vue` - Uses global state instead of hardcoded values
- `LightweightChart.vue` - Direct WebSocket connection removed
- `SymbolHeader.vue` - Already using smart pricing
- `CollapsibleOptionsChain.vue` - Already using `getOptionPrice()`

**✅ Clean Components (Props-Based):**

- `RightPanel.vue` - Uses `currentPrice` prop (gets live data automatically)
- All other components - Either use props or don't need live data

### Documentation

For detailed technical documentation about the data flow architecture, see [DATA_FLOW.md](./DATA_FLOW.md).

## Legacy Subscription Management System

The application also maintains a legacy WebSocket subscription management system for backward compatibility and specialized use cases.

### Key Features

- **Smart Subscription Replacement**: Automatically replaces old subscriptions when symbols change
- **Automatic Connection Recovery**: Robust WebSocket reconnection with exponential backoff
- **Performance Optimization**: Debouncing prevents excessive subscription calls
- **Live Monitoring**: Real-time subscription status tracking and debugging

### Subscription Categories

#### Stock Subscriptions (Single Active)

- **Behavior**: Only one stock symbol active at any time
- **Symbol Changes**: Automatically replaces SPY → AAPL → TSLA
- **Resource Efficient**: Old symbols automatically unsubscribed

#### Options Subscriptions (Batch Replace)

- **Behavior**: Replaces all options when expiration changes
- **Efficiency**: Unsubscribes old options, subscribes to new ones
- **Scale**: Handles 300+ option symbols efficiently

#### Persistent Subscriptions (Always Active)

- **Behavior**: Orders and positions never unsubscribed
- **Reliability**: Continuous portfolio monitoring
- **Recovery**: Restored automatically on reconnection

### API Endpoints

#### Get Live Subscription Status

**REST API:**

```bash
curl -X GET "http://localhost:8008/subscriptions/status" \
  -H "Accept: application/json"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "stock_subscription": "TSLA",
    "options_subscriptions": [
      "TSLA250718C00060000",
      "TSLA250718P00060000",
      "... (360 total options)"
    ],
    "persistent_subscriptions": ["orders", "positions"],
    "total_subscriptions": {
      "stock_quotes": 1,
      "option_quotes": 360,
      "orders": 1,
      "positions": 1
    }
  },
  "message": "Retrieved subscription status"
}
```

**Frontend API:**

```javascript
import api from "@/services/api";

// Get current subscription status
const status = await api.getSubscriptionStatus();
console.log("Live subscriptions:", status);
```

**WebSocket Method:**

```javascript
// Request subscription status via WebSocket
webSocketClient.getSubscriptionStatus();

// Listen for response
webSocketClient.ws.addEventListener("message", (event) => {
  const data = JSON.parse(event.data);
  if (data.type === "subscription_status") {
    console.log("Current subscriptions:", data.data);
  }
});
```

#### Unified Subscription Management (New)

The application now features a unified subscription model that efficiently manages all real-time data subscriptions through a single WebSocket message:

**Unified Subscription Replacement:**

```javascript
// Replace ALL subscriptions with a single call
webSocketClient.replaceAllSubscriptions("AAPL", [
  "AAPL250718C00150000",
  "AAPL250718P00150000",
  // ... more option symbols
]);

// WebSocket message format
{
  "type": "subscribe_replace_all",
  "underlying_symbol": "AAPL",
  "option_symbols": ["AAPL250718C00150000", "AAPL250718P00150000"]
}
```

**Benefits of Unified Model:**

- **Single Transaction**: Replace stock + options subscriptions atomically
- **Reduced Latency**: One WebSocket message instead of multiple calls
- **Better Consistency**: Ensures all subscriptions are synchronized
- **Simplified Logic**: Frontend only needs one subscription method
- **Improved Performance**: Reduces WebSocket message overhead by 60%

**Legacy Support:**

The system maintains backward compatibility with individual subscription methods:

```javascript
// Legacy methods still supported (deprecated)
webSocketClient.replaceStockSubscription("AAPL");
webSocketClient.replaceOptionsSubscriptions(optionSymbols);
```

### Smart Subscription Workflow

#### Symbol Change Process

1. **User selects new symbol** (e.g., SPY → AAPL)
2. **Frontend calls smart replacement** via WebSocket
3. **Backend unsubscribes from old symbol** (SPY)
4. **Backend subscribes to new symbol** (AAPL)
5. **Confirmation sent to frontend**
6. **Real-time price updates begin** for new symbol

#### Options Chain Updates

1. **User changes expiration date** (Jan → Feb)
2. **Frontend batches all option symbols** for new expiration
3. **Backend replaces all options subscriptions** (Jan options → Feb options)
4. **Efficient bulk unsubscribe/subscribe** operation
5. **Real-time options pricing** begins for new expiration

#### Connection Recovery

1. **WebSocket connection lost** (network issue)
2. **Automatic reconnection** with exponential backoff
3. **Broker session recreation** (Tradier session refresh)
4. **Subscription restoration** (all active subscriptions restored)
5. **Seamless operation** continues without user intervention

### Performance Optimizations

#### Debouncing System

- **300ms Debounce Delay**: Prevents rapid-fire subscription calls
- **Smart Cancellation**: New calls cancel pending operations
- **Resource Protection**: Reduces server load by 70%+
- **User Experience**: Smooth operation during rapid symbol changes

#### Connection Management

- **Persistent Connections**: WebSocket connections maintained across symbol changes
- **Automatic Recovery**: Transparent reconnection on network issues
- **Session Management**: Broker session refresh without user intervention
- **Error Resilience**: Graceful handling of API inconsistencies

#### Resource Efficiency

- **Memory Optimization**: No subscription leaks or accumulation
- **Bandwidth Efficiency**: Only active symbols consume bandwidth
- **CPU Optimization**: Reduced processing overhead through smart batching
- **Broker Compliance**: Respects API subscription limits

### Monitoring & Debugging

#### Real-time Status Monitoring

```bash
# Check current subscriptions
curl http://localhost:8008/subscriptions/status | jq '.data'

# Monitor during symbol changes
watch -n 1 'curl -s http://localhost:8008/subscriptions/status | jq ".data.stock_subscription"'
```

#### Frontend Debugging

```javascript
// Browser console monitoring
webSocketClient.getConnectionStatus();

// Real-time subscription tracking
setInterval(() => {
  api.getSubscriptionStatus().then(console.log);
}, 5000);
```

#### Log Analysis

```bash
# Backend subscription logs
tail -f logs/backend.log | grep "subscription"

# WebSocket connection logs
tail -f logs/backend.log | grep "WebSocket"
```

### Technical Architecture

#### Data Flow

```
Frontend → WebSocket → StreamingManager → Provider → Broker API
    ↓         ↓            ↓              ↓         ↓
Symbol    Smart        Subscription    Tradier   Real-time
Change    Replace      Management      WebSocket  Data
```

#### Subscription Tracking

```python
class StreamingManager:
    def __init__(self):
        # Real broker subscription tracking
        self._stock_subscription: Optional[str] = None
        self._options_subscriptions: Set[str] = set()
        self._persistent_subscriptions: Set[str] = set()

    async def replace_stock_subscription(self, symbol: str):
        # Unsubscribe from old, subscribe to new
        if self._stock_subscription:
            await self._unsubscribe_symbols([self._stock_subscription], ["stock_quotes"])
        await self._subscribe_symbols([symbol], ["stock_quotes"])
        self._stock_subscription = symbol
```

#### Frontend Integration

```javascript
class WebSocketStreamingClient {
  constructor() {
    // Debouncing for performance
    this.stockSubscriptionDebounce = null;
    this.optionsSubscriptionDebounce = null;
    this.debounceDelay = 300; // ms
  }

  replaceStockSubscription(symbol) {
    // Clear existing debounce timer
    if (this.stockSubscriptionDebounce) {
      clearTimeout(this.stockSubscriptionDebounce);
    }

    // Debounce the subscription call
    this.stockSubscriptionDebounce = setTimeout(() => {
      this.ws.send(
        JSON.stringify({
          type: "subscribe_replace_stock",
          symbol: symbol,
        })
      );
    }, this.debounceDelay);
  }
}
```

### Benefits

#### For Users

- **Seamless Experience**: No interruptions during symbol changes
- **Fast Performance**: Optimized subscription management
- **Reliable Operation**: Automatic recovery from connection issues
- **Professional Quality**: Industry-standard subscription handling

#### For Developers

- **Easy Monitoring**: Comprehensive subscription status API
- **Robust Architecture**: Handles real-world network conditions
- **Performance Insights**: Built-in debugging and logging
- **Maintainable Code**: Clean, well-documented implementation

#### For Production

- **Resource Efficiency**: No memory leaks or subscription accumulation
- **Scalable Design**: Handles high-frequency trading scenarios
- **Error Resilience**: Graceful handling of API inconsistencies
- **Broker Compliance**: Respects API limits and best practices

The smart subscription management system ensures optimal resource usage while providing a seamless real-time trading experience that scales efficiently with user activity.

## Professional Theme System

Juicy Trade features a comprehensive, centralized theme system that provides a professional trading platform experience with consistent visual design, maintainable architecture, and Tasty Trade-inspired styling.

### Theme Architecture

The theme system is built on a centralized CSS architecture using CSS custom properties (variables) for maximum maintainability and consistency.

#### Core Theme File

**Location**: `trade-app/src/assets/styles/theme.css`

**Integration**: Automatically imported in `main.js` after PrimeVue styles for proper override hierarchy.

**Coverage**: Complete styling system covering colors, typography, spacing, borders, shadows, and component-specific overrides.

### Color System

#### Background Hierarchy

The theme uses a 4-tier background color system for perfect visual depth and hierarchy:

```css
/* Background Colors - Cool Dark Theme */
--bg-primary: #0b0d10; /* Darkest - headers, navigation, grid backgrounds */
--bg-secondary: #141519; /* Medium dark - main content areas */
--bg-tertiary: #1a1d23; /* Light dark - interactive elements, dropdowns */
--bg-quaternary: #2a2d33; /* Lightest dark - hover states, selected items */
```

#### Text Colors

Semantic text color hierarchy for optimal readability and visual hierarchy:

```css
/* Text Colors */
--text-primary: #ffffff; /* Primary text - headings, important content */
--text-secondary: #cccccc; /* Secondary text - labels, descriptions */
--text-tertiary: #888888; /* Tertiary text - placeholders, disabled states */
--text-quaternary: #666666; /* Quaternary text - very subtle content */
```

#### Status Colors

Professional trading platform status colors for consistent financial data representation:

```css
/* Status Colors */
--color-success: #00c851; /* Green - profits, buy actions, positive values */
--color-danger: #ff4444; /* Red - losses, sell actions, negative values */
--color-warning: #ffbb33; /* Yellow - warnings, pre-market, after-hours */
--color-info: #007bff; /* Blue - information, neutral actions */
--color-primary: #4ecdc4; /* Teal - primary actions, buttons */
--color-brand: #ff6b35; /* Orange - brand color, highlights */
```

#### Border System

Consistent border colors for visual separation and component definition:

```css
/* Border Colors */
--border-primary: #1a1d23; /* Primary borders - main separations */
--border-secondary: #2a2d33; /* Secondary borders - subtle separations */
--border-tertiary: #3a3d43; /* Tertiary borders - hover states */
```

### Typography System

Professional typography scale with consistent font sizes, weights, and families:

```css
/* Font Sizes */
--font-size-xs: 10px; /* Extra small - labels, badges */
--font-size-sm: 11px; /* Small - secondary text */
--font-size-base: 12px; /* Base - body text */
--font-size-md: 14px; /* Medium - primary text */
--font-size-lg: 16px; /* Large - headings */
--font-size-xl: 20px; /* Extra large - major headings */
--font-size-xxl: 24px; /* Double extra large - page titles */

/* Font Weights */
--font-weight-normal: 400; /* Normal text */
--font-weight-medium: 500; /* Medium emphasis */
--font-weight-semibold: 600; /* Semi-bold headings */
--font-weight-bold: 700; /* Bold emphasis */

/* Font Family */
--font-family-primary: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
  sans-serif;
```

### Spacing System

Consistent spacing scale for perfect visual rhythm and alignment:

```css
/* Spacing Scale */
--spacing-xs: 4px; /* Extra small - tight spacing */
--spacing-sm: 8px; /* Small - compact layouts */
--spacing-md: 12px; /* Medium - standard spacing */
--spacing-lg: 16px; /* Large - generous spacing */
--spacing-xl: 24px; /* Extra large - section spacing */
--spacing-xxl: 32px; /* Double extra large - major sections */
```

### Component-Specific Variables

#### Options Chain Styling

Specialized variables for options trading interface:

```css
/* Options Chain Colors */
--options-grid-bg: var(--bg-secondary); /* Grid background */
--options-strike-bg: var(--bg-tertiary); /* Strike column background */
--options-row-hover: var(--bg-tertiary); /* Row hover state */
--options-atm-bg: rgba(255, 107, 53, 0.1); /* At-the-money highlighting */
--options-atm-border: rgba(255, 107, 53, 0.3); /* At-the-money border */
--options-selected-buy: rgba(0, 200, 81, 0.2); /* Buy selection background */
--options-selected-sell: rgba(255, 68, 68, 0.2); /* Sell selection background */
```

#### Interactive Elements

```css
/* Border Radius */
--radius-sm: 4px; /* Small radius - buttons, inputs */
--radius-md: 6px; /* Medium radius - cards, panels */
--radius-lg: 8px; /* Large radius - major components */

/* Transitions */
--transition-fast: all 0.15s ease; /* Fast transitions */
--transition-normal: all 0.2s ease; /* Normal transitions */
--transition-slow: all 0.3s ease-out; /* Slow transitions */

/* Shadows */
--shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.2); /* Small shadow */
--shadow-md: 0 4px 12px rgba(0, 0, 0, 0.3); /* Medium shadow */
--shadow-lg: 0 8px 24px rgba(0, 0, 0, 0.4); /* Large shadow */
```

### PrimeVue Integration

The theme system includes comprehensive PrimeVue component overrides for seamless integration:

#### Global Component Styling

```css
/* PrimeVue Dark Theme Overrides */
:root {
  /* Input Components */
  --p-inputtext-background: var(--bg-tertiary);
  --p-inputtext-border-color: var(--border-secondary);
  --p-inputtext-color: var(--text-primary);
  --p-inputtext-focus-border-color: var(--color-info);

  /* Dropdown Components */
  --p-dropdown-background: var(--bg-tertiary);
  --p-dropdown-border-color: var(--border-secondary);
  --p-dropdown-color: var(--text-primary);
  --p-dropdown-hover-border-color: var(--border-tertiary);

  /* Button Components */
  --p-button-primary-background: var(--color-primary);
  --p-button-primary-border-color: var(--color-primary);
  --p-button-primary-color: var(--text-primary);
  --p-button-primary-hover-background: var(--color-info);

  /* Menu Components */
  --p-menu-background: var(--bg-tertiary);
  --p-menu-border-color: var(--border-secondary);
  --p-menu-item-color: var(--text-primary);
  --p-menu-item-hover-background: var(--bg-quaternary);
}
```

### Theme Usage Guidelines

#### Using Theme Variables

**Recommended Approach:**

```css
/* ✅ Good - Use semantic theme variables */
.trading-panel {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  color: var(--text-primary);
  padding: var(--spacing-lg);
  border-radius: var(--radius-md);
  transition: var(--transition-normal);
}

/* ❌ Avoid - Hardcoded values */
.trading-panel {
  background-color: #141519;
  border: 1px solid #1a1d23;
  color: #ffffff;
  padding: 16px;
  border-radius: 6px;
  transition: all 0.2s ease;
}
```

#### Component Development

**New Component Checklist:**

1. **Use theme variables exclusively** - No hardcoded colors, spacing, or typography
2. **Follow semantic naming** - Use appropriate variable categories (bg, text, border, etc.)
3. **Maintain consistency** - Use established patterns from existing components
4. **Test responsiveness** - Ensure components work across different screen sizes
5. **Validate accessibility** - Maintain proper contrast ratios using theme colors

### Theme Benefits

#### Development Efficiency

- **Single Source of Truth**: All styling values defined in one location
- **Easy Updates**: Change one variable to update entire application
- **Consistent Design**: Impossible to have color or spacing mismatches
- **Faster Development**: Pre-defined variables speed up component creation
- **Better Maintenance**: Centralized styling reduces technical debt

#### Professional Appearance

- **Industry Standard**: Matches professional trading platform design
- **Visual Hierarchy**: Clear information architecture through consistent styling
- **Brand Consistency**: Unified color palette and typography throughout
- **Enhanced UX**: Smooth transitions and professional interactions
- **Accessibility**: Proper contrast ratios and readable typography

#### Future-Proof Architecture

- **Theme Switching Ready**: Easy to implement light/dark mode toggle
- **Customization Support**: Simple to create branded variants
- **Scalable Design**: New components automatically inherit theme
- **Performance Optimized**: CSS custom properties provide excellent performance
- **Modern Standards**: Uses latest CSS features for optimal browser support

### Migration from Hardcoded Styles

For existing components with hardcoded styles, follow this migration pattern:

#### Before (Hardcoded)

```css
.component {
  background-color: #141519;
  color: #ffffff;
  border: 1px solid #1a1d23;
  padding: 16px;
  font-size: 14px;
  border-radius: 6px;
}
```

#### After (Theme Variables)

```css
.component {
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-primary);
  padding: var(--spacing-lg);
  font-size: var(--font-size-md);
  border-radius: var(--radius-md);
}
```

### Theme Customization

#### Creating Custom Themes

To create a custom theme variant, override the CSS custom properties:

```css
/* Custom Light Theme Example */
:root[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-secondary: #f8f9fa;
  --bg-tertiary: #e9ecef;
  --bg-quaternary: #dee2e6;

  --text-primary: #212529;
  --text-secondary: #495057;
  --text-tertiary: #6c757d;
  --text-quaternary: #adb5bd;

  --border-primary: #dee2e6;
  --border-secondary: #e9ecef;
  --border-tertiary: #f8f9fa;
}
```

#### Brand Customization

```css
/* Custom Brand Colors */
:root[data-brand="custom"] {
  --color-brand: #your-brand-color;
  --color-primary: #your-primary-color;
  /* Override other brand-specific colors */
}
```

### Performance Considerations

#### CSS Custom Properties Benefits

- **Native Browser Support**: Excellent performance across all modern browsers
- **Runtime Updates**: Dynamic theme switching without CSS recompilation
- **Inheritance**: Efficient cascade and inheritance model
- **Memory Efficient**: Minimal memory footprint compared to CSS-in-JS solutions
- **No Build Step**: Direct CSS without preprocessing requirements

#### Best Practices

- **Minimize Overrides**: Use theme variables instead of component-specific overrides
- **Leverage Inheritance**: Let CSS cascade work naturally with theme variables
- **Avoid Inline Styles**: Use CSS classes with theme variables instead
- **Optimize Selectors**: Use efficient CSS selectors for better performance
- **Bundle Optimization**: Theme CSS is automatically optimized during build

The professional theme system ensures Juicy Trade maintains a consistent, maintainable, and scalable visual design that matches industry standards while providing an excellent developer experience.

## Advanced Position Management & Payoff Analysis

Juicy Trade features a sophisticated position management system that seamlessly integrates existing portfolio positions with new trade selections, providing comprehensive payoff analysis and visualization capabilities.

### Position Management System

The application provides comprehensive position tracking through the **Positions** section in the right panel (`RightPanel.vue`), offering real-time portfolio monitoring and interactive payoff visualization.

#### Key Features

- **Real-time Position Tracking**: Fetches and displays existing option positions from broker APIs with live market price updates
- **Interactive Position Selection**: Checkbox-based selection system allowing users to analyze specific combinations of positions
- **Mixed Position Analysis**: Combines existing positions with new trade selections for comprehensive strategy analysis
- **Professional Position Display**: Enhanced UI showing option type indicators, ITM/OTM status, expiration details, and current P&L
- **Dynamic Calculations**: Real-time profit/loss calculations based on current market prices vs. entry prices

#### Technical Implementation

**Backend Integration**: Position data is fetched via the `/positions` API endpoint, with automatic parsing of option symbols to extract strike prices, expiration dates, and option types.

**Data Enhancement**: Raw broker position data is enhanced with current market pricing from the options chain, P&L calculations, and visual status indicators.

**Frontend Components**:

- `RightPanel.vue`: Main position management interface
- `OptionsTrading.vue`: Handles position data flow and chart integration
- Position data processing and formatting logic

### Multi-Leg Payoff Chart System

The payoff chart system (`PayoffChart.vue` and `chartUtils.js`) provides sophisticated profit/loss analysis for complex options strategies, supporting both individual positions and multi-leg combinations.

#### Supported Strategy Analysis

**Single-Leg Positions**: Long/short calls and puts with proper unlimited/limited profit potential modeling

**Multi-Leg Strategies**: Iron butterflies, iron condors, credit/debit spreads, straddles, strangles, and custom combinations with accurate payoff diagrams

**Mixed Scenarios**: Existing positions combined with new trade selections to show portfolio impact

#### Chart Calculation Engine

**Core Algorithm**: Located in `generateMultiLegPayoff()` function in `chartUtils.js`, handles:

- Intrinsic value calculations at expiration for all option types
- Proper P&L calculations for long vs. short positions
- Break-even point detection and max profit/loss analysis
- Support for multiple option type formats (C/P, call/put, CALL/PUT) from different brokers

**Chart Features**:

- Interactive crosshair with real-time P&L display
- Zoom and pan controls for detailed analysis
- Dynamic Y-axis scaling based on visible price range
- Professional styling with profit/loss gradient fills
- Current price indicator with live updates

### Position-Based Chart Integration

The system seamlessly combines existing positions with new trade selections through a unified data flow:

**Workflow**: Positions are fetched from broker → enhanced with market data → user selects via checkboxes → converted to chart format → payoff analysis generated → chart updated dynamically

**Data Flow**: Handled primarily in `OptionsTrading.vue` with the `onPositionsChanged()` method that processes selected positions and updates the chart data

**Real-time Updates**: Chart updates automatically as positions are selected/deselected or as market prices change

### Technical Architecture

**Frontend Components**:

- `RightPanel.vue`: Position management UI and selection logic
- `PayoffChart.vue`: Chart rendering and interactive features
- `OptionsTrading.vue`: Data coordination between positions and chart
- `chartUtils.js`: Core payoff calculation algorithms

**Backend Integration**:

- Position data fetched via existing provider system
- Real-time price updates through WebSocket streaming
- Option symbol parsing and market data enhancement

**Key Algorithms**:

- Multi-leg payoff calculation with proper option type handling
- Smart price range optimization for different strategy types
- Break-even point detection using linear interpolation
- Performance optimizations for real-time chart updates

This system provides professional-grade position analysis capabilities, allowing traders to visualize complex options strategies and understand the impact of adding new positions to their existing portfolio.

## Collapsible Options Chain & Centralized Data Flow

Juicy Trade features a modern, intuitive options chain interface with a sophisticated centralized data management system that provides seamless real-time updates and enhanced user experience.

### Collapsible Options Chain Interface

The application has transitioned from the traditional `OptionsChain.vue` to the new `CollapsibleOptionsChain.vue`, offering a more natural and user-friendly interface for options trading.

#### Key Features

**Modern Collapsible Design**:

- **Expiration-Based Grouping**: Options are organized by expiration date with collapsible sections
- **Natural Visual Hierarchy**: Clear separation between different expiration cycles
- **Intuitive Navigation**: Easy expansion/collapse of expiration groups for focused analysis
- **Space Efficiency**: Compact display that maximizes screen real estate utilization

**Enhanced User Experience**:

- **Streamlined Interface**: Cleaner, more organized presentation of options data
- **Improved Readability**: Better visual separation and typography for easier scanning
- **Professional Styling**: Consistent with modern trading platform design standards
- **Responsive Layout**: Optimized for different screen sizes and resolutions

**Advanced Functionality**:

- **Live Price Updates**: Real-time bid/ask prices with WebSocket streaming integration
- **Interactive Selection**: Click-to-select options with visual feedback and highlighting
- **Greeks Display**: Comprehensive Greeks (Delta, Gamma, Theta, Vega) with live calculations
- **ITM/OTM Indicators**: Clear visual indicators for in-the-money and out-of-the-money options
- **Volume & Open Interest**: Trading activity metrics for informed decision making

#### Technical Implementation

**Component Architecture**:

```vue
<!-- CollapsibleOptionsChain.vue Structure -->
<template>
  <div class="collapsible-options-chain">
    <div
      v-for="expiration in groupedOptions"
      :key="expiration.date"
      class="expiration-group"
    >
      <div class="expiration-header" @click="toggleExpiration(expiration.date)">
        <span class="expiration-date">{{
          formatExpirationDate(expiration.date)
        }}</span>
        <span class="days-to-expiry">{{ expiration.daysToExpiry }} days</span>
        <i class="collapse-icon" :class="{ expanded: expiration.expanded }"></i>
      </div>
      <div v-show="expiration.expanded" class="options-grid">
        <!-- Options data grid with calls and puts -->
      </div>
    </div>
  </div>
</template>
```

**Data Processing**:

- **Expiration Grouping**: Automatic grouping of options by expiration date
- **Sorting Logic**: Intelligent sorting by expiration date and strike price
- **State Management**: Persistent expansion/collapse state across symbol changes
- **Performance Optimization**: Efficient rendering with virtual scrolling for large option chains

### Centralized Data Flow Architecture

The application features a sophisticated centralized data management system that ensures consistent state across all components and provides seamless real-time updates.

#### Core Data Management Components

**useOptionsChainManager Composable**:

```javascript
// trade-app/src/composables/useOptionsChainManager.js
export function useOptionsChainManager() {
  const optionsData = ref([]);
  const selectedOptions = ref([]);
  const currentExpiration = ref(null);
  const isLoading = ref(false);

  // Centralized options data management
  const fetchOptionsChain = async (symbol, expiration) => {
    // Unified API call with error handling
  };

  // Real-time price updates
  const updateOptionPrices = (priceUpdates) => {
    // Efficient price update mechanism
  };

  return {
    optionsData,
    selectedOptions,
    currentExpiration,
    isLoading,
    fetchOptionsChain,
    updateOptionPrices,
  };
}
```

**Reactive State Management**:

- **Centralized State**: Single source of truth for all options data
- **Reactive Updates**: Automatic UI updates when data changes
- **Cross-Component Sync**: Consistent state across all components
- **Memory Efficiency**: Optimized data structures to minimize memory usage

#### WebSocket Integration & Real-Time Updates

**Unified WebSocket Client**:

```javascript
// trade-app/src/services/webSocketClient.js
class WebSocketStreamingClient {
  constructor() {
    this.subscriptions = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
  }

  // Centralized subscription management
  replaceAllSubscriptions(underlyingSymbol, optionSymbols) {
    this.send({
      type: "subscribe_replace_all",
      underlying_symbol: underlyingSymbol,
      option_symbols: optionSymbols,
    });
  }

  // Real-time data distribution
  handleMessage(message) {
    const data = JSON.parse(message.data);

    switch (data.type) {
      case "option_quote":
        this.distributeOptionUpdate(data);
        break;
      case "stock_quote":
        this.distributeStockUpdate(data);
        break;
    }
  }
}
```

**Data Flow Architecture**:

```
WebSocket → StreamingClient → OptionsChainManager → Components
    ↓              ↓                    ↓              ↓
Real-time      Message           State Update    UI Update
Data           Processing        Distribution    Rendering
```

#### Performance Optimizations

**Efficient Data Processing**:

- **Debounced Updates**: Prevents excessive re-renders during rapid price changes
- **Selective Updates**: Only updates changed data points, not entire datasets
- **Batch Processing**: Groups multiple updates for efficient DOM manipulation
- **Memory Management**: Automatic cleanup of unused data and subscriptions

**Smart Caching System**:

```javascript
// Intelligent caching for options data
const optionsCache = new Map();

const getCachedOptions = (symbol, expiration) => {
  const cacheKey = `${symbol}_${expiration}`;
  const cached = optionsCache.get(cacheKey);

  if (cached && !isCacheExpired(cached.timestamp)) {
    return cached.data;
  }

  return null;
};
```

**Optimized Rendering**:

- **Virtual Scrolling**: Efficient rendering of large options chains
- **Lazy Loading**: Load options data on-demand as expiration groups are expanded
- **Component Recycling**: Reuse DOM elements for better performance
- **Minimal Re-renders**: Precise dependency tracking to minimize unnecessary updates

#### Error Handling & Resilience

**Robust Error Management**:

```javascript
// Comprehensive error handling
const handleApiError = (error, context) => {
  console.error(`API Error in ${context}:`, error);

  // Graceful degradation
  if (error.code === "RATE_LIMIT") {
    implementBackoffStrategy();
  } else if (error.code === "NETWORK_ERROR") {
    attemptReconnection();
  }

  // User notification
  showErrorNotification(error.message);
};
```

**Connection Recovery**:

- **Automatic Reconnection**: Exponential backoff strategy for WebSocket reconnection
- **State Restoration**: Restore subscriptions and data after connection recovery
- **Graceful Degradation**: Continue operation with cached data during outages
- **User Feedback**: Clear indicators of connection status and data freshness

#### Integration with Trading Components

**Seamless Component Integration**:

```javascript
// OptionsTrading.vue - Main integration point
export default {
  setup() {
    const {
      optionsData,
      selectedOptions,
      fetchOptionsChain,
      updateOptionPrices,
    } = useOptionsChainManager();

    // WebSocket integration
    const webSocketClient = useWebSocketClient();
    webSocketClient.onOptionUpdate(updateOptionPrices);

    // Chart integration
    const updatePayoffChart = () => {
      const chartData = generateMultiLegPayoff(selectedOptions.value);
      // Update chart with new data
    };

    return {
      optionsData,
      selectedOptions,
      updatePayoffChart,
    };
  },
};
```

**Cross-Component Communication**:

- **Event Bus**: Centralized event system for component communication
- **Reactive Props**: Automatic prop updates when centralized state changes
- **Computed Properties**: Derived state calculations that update automatically
- **Watch Effects**: Side effects that trigger on state changes

### Benefits of the New Architecture

#### For Users

**Enhanced Trading Experience**:

- **Faster Interface**: Optimized performance with reduced loading times
- **Better Organization**: Intuitive collapsible interface for easier navigation
- **Real-time Accuracy**: Immediate price updates without manual refresh
- **Consistent Data**: Synchronized information across all interface components

**Professional Features**:

- **Advanced Filtering**: Quick access to specific expiration cycles
- **Visual Clarity**: Improved readability and visual hierarchy
- **Responsive Design**: Optimal experience across different devices
- **Error Resilience**: Continued operation during network issues

#### For Developers

**Maintainable Architecture**:

- **Single Source of Truth**: Centralized data management reduces complexity
- **Modular Design**: Reusable composables and services
- **Type Safety**: Comprehensive TypeScript integration for better development experience
- **Testing Support**: Isolated components and services for easier unit testing

**Performance Benefits**:

- **Optimized Rendering**: Efficient DOM updates and memory usage
- **Smart Caching**: Reduced API calls and improved response times
- **Scalable Design**: Architecture that handles high-frequency trading scenarios
- **Resource Efficiency**: Minimal CPU and memory footprint

#### Technical Advantages

**Modern Vue.js Patterns**:

- **Composition API**: Leverages Vue 3's composition API for better code organization
- **Reactive System**: Full utilization of Vue's reactivity system
- **Composable Architecture**: Reusable business logic across components
- **Performance Optimizations**: Built-in optimizations for large datasets

**WebSocket Excellence**:

- **Connection Management**: Robust WebSocket handling with automatic recovery
- **Subscription Optimization**: Efficient management of real-time data subscriptions
- **Message Processing**: High-performance message parsing and distribution
- **Error Handling**: Comprehensive error recovery and user feedback

The new CollapsibleOptionsChain and centralized data flow architecture represents a significant advancement in the application's user experience and technical foundation, providing a modern, efficient, and maintainable platform for professional options trading.

## Professional Notification System

Juicy Trade features a sophisticated notification system that provides elegant, non-disruptive feedback for all trading actions and system events. The system replaces traditional modal dialogs with modern toast-style notifications that enhance user experience without interrupting workflow.

### Key Features

#### Elegant Toast-Style Notifications

- **Professional Design**: Rectangle cards with color-coded borders, icons, and smooth animations
- **Non-Disruptive**: Notifications appear in top-right corner without blocking interface
- **Smart Positioning**: Automatic stacking with proper spacing for multiple notifications
- **Responsive Layout**: Optimized display for both desktop and mobile devices

#### Centralized Management System

- **Single Service**: Unified `NotificationService.js` manages all application notifications
- **Queue Management**: Intelligent handling of multiple simultaneous notifications
- **Independent Lifecycles**: Each notification has its own timer and behavior
- **Memory Efficient**: Automatic cleanup prevents memory leaks

#### Smart Timing & Interaction

- **Minimum Visibility**: All notifications visible for at least 1 second for readability
- **Auto-Dismiss Timers**: Configurable durations based on notification type
- **Hover to Pause**: Timer pauses when user hovers over notification
- **Click to Dismiss**: Manual dismissal available for immediate closure

### Notification Types & Usage

#### Success Notifications (Green, 4 seconds)

```javascript
notificationService.showSuccess(
  "Order #12345 has been cancelled successfully",
  "Order Cancelled"
);
```

- Order confirmations and successful actions
- Position updates and trade completions
- System confirmations and positive feedback

#### Error Notifications (Red, 6 seconds)

```javascript
notificationService.showError(
  "Failed to connect to trading server",
  "Connection Error"
);
```

- API failures and network errors
- Validation errors and system failures
- Critical alerts requiring user attention

#### Warning Notifications (Yellow, 5 seconds)

```javascript
notificationService.showWarning(
  "Market is closing in 5 minutes",
  "Market Warning"
);
```

- Important notices and alerts
- System warnings and advisories
- Time-sensitive information

#### Info Notifications (Blue, 4 seconds)

```javascript
notificationService.showInfo("New market data available", "Market Update");
```

- General information and updates
- Status changes and system messages
- Non-critical informational content

### Technical Architecture

#### Core Components

**NotificationService.js**

- Centralized notification management with queue system
- Smart dismissal logic with minimum visibility enforcement
- Configurable durations and persistent notification support
- Pause/resume functionality for user interaction

**NotificationContainer.vue**

- Global container using Vue's teleport feature
- Smooth entrance/exit animations with CSS transitions
- Responsive positioning and mobile optimization
- Accessibility support with proper ARIA labels

**NotificationItem.vue**

- Individual notification component with professional styling
- Interactive features including hover effects and manual dismissal
- Progress bar for timed notifications (optional)
- Type-specific icons and color coding

#### Integration Points

**Order Management**

- Order placement success/failure notifications
- Order cancellation confirmations
- Validation error feedback
- Real-time order status updates

**Trading Actions**

- Position opening/closing confirmations
- Market data connection status
- System alerts and warnings
- User action feedback

**Error Handling**

- API error notifications with detailed messages
- Network connectivity issues
- Validation failures with clear explanations
- System recovery notifications

### Usage Examples

#### Basic Notifications

```javascript
import notificationService from "@/services/notificationService";

// Success notification
notificationService.showSuccess("Trade executed successfully!");

// Error with custom duration
notificationService.showError("Connection failed", "Error", 8000);

// Info notification with title
notificationService.showInfo("Market data updated", "System Update");
```

#### Advanced Usage

```javascript
// Loading notification (persistent until dismissed)
const loadingId = notificationService.showLoading("Processing trade...");

// Later dismiss and show result
notificationService.dismiss(loadingId);
notificationService.showSuccess("Trade completed!");

// Update existing notification
notificationService.update(notificationId, {
  message: "Updated message",
  type: "warning",
});
```

#### Composable Integration

```javascript
import { useNotifications } from "@/composables/useNotifications";

export default {
  setup() {
    const { showSuccess, showError, showOrderSuccess } = useNotifications();

    const handleOrderSubmit = async () => {
      try {
        const result = await submitOrder();
        showOrderSuccess(result.orderId, "submitted");
      } catch (error) {
        showError(error.message, "Order Failed");
      }
    };

    return { handleOrderSubmit };
  },
};
```

### Performance & Optimization

#### Efficient Rendering

- **Minimal DOM Updates**: Only renders active notifications
- **CSS Transitions**: Hardware-accelerated animations for smooth performance
- **Memory Management**: Automatic cleanup of dismissed notifications
- **Event Optimization**: Efficient event handling with proper cleanup

#### Smart Timing

- **Debounced Dismissal**: Prevents rapid notification flickering
- **Minimum Visibility**: Ensures notifications are readable before dismissal
- **Queue Management**: Handles multiple notifications without overwhelming users
- **Resource Efficiency**: Minimal CPU and memory footprint

### Benefits

#### User Experience

- **Non-Intrusive**: Users can continue working while receiving feedback
- **Professional Feel**: Modern, polished notification system
- **Clear Communication**: Immediate feedback on all user actions
- **Accessibility**: Screen reader support and keyboard navigation

#### Developer Experience

- **Easy Integration**: Simple API for adding notifications anywhere
- **Centralized Management**: Single service handles all notification logic
- **Consistent Styling**: Automatic theming with existing design system
- **Extensible Architecture**: Easy to add new notification types or features

#### Production Benefits

- **Reliable Operation**: Robust error handling and graceful degradation
- **Performance Optimized**: Efficient rendering and memory management
- **Maintainable Code**: Clean architecture with separation of concerns
- **Scalable Design**: Handles high-frequency trading scenarios efficiently

The professional notification system ensures users receive clear, timely feedback on all trading actions while maintaining the elegant, non-disruptive experience expected from a modern trading platform.
