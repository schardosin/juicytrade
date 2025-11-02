# Juicy Trade

A sophisticated options trading application with a modular, multi-provider architecture, featuring a FastAPI backend and a Vue.js frontend. The application is designed to support multiple brokerage providers, with a clear separation between live and paper trading environments.

## Architecture

The application is divided into two main components:

1.  **`trade_backend`**: A FastAPI application that serves as the backend for the trading application. It provides a standardized API for interacting with different brokerage providers, handles user authentication, and manages real-time data streaming.

2.  **`trade_app`**: A Vue.js single-page application (SPA) that provides the user interface for the trading application. It communicates with the backend via a RESTful API and WebSockets for real-time data updates.

This architecture allows for a clean separation of concerns between the frontend and backend, making it easier to develop and maintain the application.

### Multi-Provider Architecture

The backend is designed to support multiple brokerage providers, with a clear distinction between live and paper trading accounts. This allows for a flexible and powerful trading experience, where you can use live data feeds for market analysis while executing trades in a paper account for testing.

- **Provider Manager (`provider_manager.py`)**: Initializes all available providers, including live and paper variants (e.g., `alpaca`, `alpaca_paper`, `tradier`, `tradier_paper`, `tastytrade`, `tastytrade_paper`).
- **Provider Configuration (`provider_config.py`)**: Manages the routing of different operations (e.g., `stock_quotes`, `orders`, `positions`, `expiration_dates`, `next_market_date`) to specific providers. This configuration is stored in `provider_config.json` and can be updated via the API.
- **Streaming Manager (`streaming_manager.py`)**: Manages real-time data streaming from multiple providers concurrently, aggregating the data into a single feed for the frontend.

## Authentication System

Juicy Trade features a comprehensive, flexible authentication system that supports multiple authentication methods to secure your trading platform. The system is designed to be configurable and production-ready with proper security measures.

### Supported Authentication Methods

#### 1. **OAuth Authentication** (Recommended for Production)
- **Google OAuth**: Secure authentication using Google accounts
- **GitHub OAuth**: Authentication via GitHub accounts  
- **Microsoft OAuth**: Authentication using Microsoft/Azure accounts
- **User Authorization**: Configurable email/domain whitelisting for access control
- **Secure Cookies**: HTTPS-compatible session management

#### 2. **Simple Authentication**
- **Username/Password**: Basic authentication with configurable credentials
- **Session Management**: JWT-based session tokens
- **Development Friendly**: Easy setup for development environments

#### 3. **Token-Based Authentication**
- **API Token**: Bearer token authentication for API access
- **JWT Integration**: Secure token validation and management

#### 4. **Header-Based Authentication**
- **Reverse Proxy**: Integration with reverse proxy authentication
- **Enterprise Ready**: Compatible with enterprise SSO solutions

#### 5. **Disabled Authentication**
- **Development Mode**: No authentication required (development only)
- **Quick Setup**: Immediate access for testing and development

### Authentication Configuration

#### Environment Variables

Configure authentication using environment variables in your `.env` file or Docker environment:

```bash
# Authentication Method (required)
AUTH_METHOD=oauth  # Options: oauth, simple, token, header, disabled

# OAuth Configuration (for AUTH_METHOD=oauth)
AUTH_OAUTH_PROVIDER=google  # Options: google, github, microsoft
AUTH_OAUTH_CLIENT_ID=your_oauth_client_id
AUTH_OAUTH_CLIENT_SECRET=your_oauth_client_secret
AUTH_OAUTH_REDIRECT_URI=https://yourdomain.com/auth/oauth/callback

# User Authorization (CRITICAL for production security)
AUTH_OAUTH_ALLOWED_EMAILS=admin@yourcompany.com,trader@yourcompany.com
AUTH_OAUTH_ALLOWED_DOMAINS=yourcompany.com,yourdomain.com

# Simple Authentication (for AUTH_METHOD=simple)
AUTH_SIMPLE_USERNAME=admin
AUTH_SIMPLE_PASSWORD=your_secure_password

# JWT Configuration
AUTH_JWT_SECRET_KEY=your_jwt_secret_key_change_in_production
AUTH_JWT_EXPIRE_MINUTES=1440  # 24 hours
AUTH_JWT_ALGORITHM=HS256

# Session Configuration
AUTH_SESSION_COOKIE_NAME=juicytrade_session
AUTH_SESSION_MAX_AGE=86400  # 24 hours

# Security Settings
AUTH_SECURE_COOKIES=true   # Enable for HTTPS (production)
AUTH_COOKIE_DOMAIN=yourdomain.com  # Set for production
AUTH_ENABLE_CSRF=false     # Disable for API-first applications
```

#### OAuth Setup Guide

**1. Google OAuth Setup:**
```bash
# 1. Go to Google Cloud Console (console.cloud.google.com)
# 2. Create a new project or select existing
# 3. Enable Google+ API
# 4. Create OAuth 2.0 credentials
# 5. Add authorized redirect URI: https://yourdomain.com/auth/oauth/callback

AUTH_METHOD=oauth
AUTH_OAUTH_PROVIDER=google
AUTH_OAUTH_CLIENT_ID=your_google_client_id
AUTH_OAUTH_CLIENT_SECRET=your_google_client_secret
AUTH_OAUTH_REDIRECT_URI=https://yourdomain.com/auth/oauth/callback
```

**2. GitHub OAuth Setup:**
```bash
# 1. Go to GitHub Settings → Developer settings → OAuth Apps
# 2. Create a new OAuth App
# 3. Set Authorization callback URL: https://yourdomain.com/auth/oauth/callback

AUTH_METHOD=oauth
AUTH_OAUTH_PROVIDER=github
AUTH_OAUTH_CLIENT_ID=your_github_client_id
AUTH_OAUTH_CLIENT_SECRET=your_github_client_secret
AUTH_OAUTH_REDIRECT_URI=https://yourdomain.com/auth/oauth/callback
```

#### User Authorization (CRITICAL Security Feature)

**⚠️ IMPORTANT**: For production deployments, you MUST configure user authorization to prevent unauthorized access:

**Option 1: Specific Email Addresses**
```bash
AUTH_OAUTH_ALLOWED_EMAILS=john@company.com,jane@company.com,admin@company.com
```

**Option 2: Domain-Based Authorization**
```bash
AUTH_OAUTH_ALLOWED_DOMAINS=yourcompany.com,yourdomain.com
```

**Option 3: Combined Authorization (Most Flexible)**
```bash
AUTH_OAUTH_ALLOWED_EMAILS=external.consultant@gmail.com,contractor@freelance.com
AUTH_OAUTH_ALLOWED_DOMAINS=yourcompany.com
```

**Security Behavior:**
- **Authorized Users**: Can access the trading platform normally
- **Unauthorized Users**: Receive a clear "Access Denied" page with their email displayed
- **No Configuration**: If no restrictions are set, ANY user with the OAuth provider can access (NOT recommended for production)
- **Logging**: All unauthorized access attempts are logged with email addresses for security monitoring

### Production Deployment

#### Docker Configuration Example

```yaml
# docker-compose-prod.yml
services:
  juicytrade:
    image: your-registry/juicytrade:latest
    environment:
      # Authentication Configuration
      - AUTH_METHOD=oauth
      - AUTH_OAUTH_PROVIDER=google
      - AUTH_OAUTH_CLIENT_ID=${AUTH_OAUTH_CLIENT_ID}
      - AUTH_OAUTH_CLIENT_SECRET=${AUTH_OAUTH_CLIENT_SECRET}
      - AUTH_OAUTH_REDIRECT_URI=https://yourdomain.com/auth/oauth/callback
      
      # CRITICAL: User Authorization
      - AUTH_OAUTH_ALLOWED_EMAILS=${AUTH_OAUTH_ALLOWED_EMAILS}
      - AUTH_OAUTH_ALLOWED_DOMAINS=${AUTH_OAUTH_ALLOWED_DOMAINS}
      
      # Security Settings
      - AUTH_JWT_SECRET_KEY=${AUTH_JWT_SECRET_KEY}
      - AUTH_SECURE_COOKIES=true
      - AUTH_COOKIE_DOMAIN=yourdomain.com
      - AUTH_ENABLE_CSRF=false
```

#### Nginx Configuration

The authentication system requires proper nginx routing for OAuth callbacks:

```nginx
# Proxy authentication requests to backend
location /auth/ {
    proxy_pass http://backend:8008/auth/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host $host;
}
```

### Authentication API Endpoints

The authentication system provides several API endpoints:

#### Get Authentication Status
```bash
GET /auth/status
```
Returns current authentication status and user information.

#### Get Authentication Configuration  
```bash
GET /auth/config
```
Returns public authentication configuration (method, provider, etc.).

#### Login (Simple/Token Authentication)
```bash
POST /auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}
```

#### Logout
```bash
POST /auth/logout
```
Clears session and logs out the current user.

#### OAuth Authorization Flow
```bash
GET /auth/oauth/authorize?next=/dashboard
```
Initiates OAuth flow with optional redirect destination.

#### Get Current User
```bash
GET /auth/user
```
Returns detailed information about the currently authenticated user.

### Security Features

#### Session Management
- **JWT Tokens**: Secure, stateless session management
- **Configurable Expiration**: Customizable session duration
- **Secure Cookies**: HTTPS-compatible cookie settings
- **Domain Restriction**: Proper cookie domain configuration for production

#### OAuth Security
- **State Parameter**: CSRF protection for OAuth flows
- **Secure Redirects**: Validated redirect URLs
- **Token Validation**: Proper OAuth token exchange and validation
- **User Authorization**: Email/domain-based access control

#### Production Security
- **HTTPS Enforcement**: Secure cookie settings for production
- **Domain Validation**: Proper cookie domain configuration
- **Access Logging**: Comprehensive logging of authentication events
- **Error Handling**: Secure error messages that don't leak sensitive information

### Development vs Production

#### Development Configuration
```bash
# Quick setup for development
AUTH_METHOD=disabled  # No authentication required
# OR
AUTH_METHOD=simple
AUTH_SIMPLE_USERNAME=admin
AUTH_SIMPLE_PASSWORD=admin123
```

#### Production Configuration
```bash
# Secure production setup
AUTH_METHOD=oauth
AUTH_OAUTH_PROVIDER=google
AUTH_OAUTH_CLIENT_ID=production_client_id
AUTH_OAUTH_CLIENT_SECRET=production_client_secret
AUTH_OAUTH_REDIRECT_URI=https://yourdomain.com/auth/oauth/callback
AUTH_OAUTH_ALLOWED_EMAILS=admin@yourcompany.com
AUTH_JWT_SECRET_KEY=your_production_jwt_secret
AUTH_SECURE_COOKIES=true
AUTH_COOKIE_DOMAIN=yourdomain.com
```

## Features

- **Multi-Provider Support**: The backend supports multiple brokerage providers, with a base provider interface that can be extended to support new providers.
- **Live & Paper Trading**: Separate configurations for live and paper trading accounts, allowing for flexible testing and trading strategies.
- **Comprehensive Authentication System**: Flexible authentication supporting OAuth (Google, GitHub, Microsoft), simple username/password, token-based, and enterprise SSO integration with configurable user authorization and production-ready security features.
- **Sleep-Resistant WebSocket Architecture**: Advanced Web Worker-based real-time streaming that continues operating when browser loses focus or computer sleeps, with automatic recovery from network issues and system wake events.
- **Connection Limit Protection**: Robust connection management with automatic cleanup of stale WebSocket connections, preventing "connection limit exceeded" errors after sleep/wake cycles. Features exponential backoff retry logic and proactive connection validation.
- **Zombie Worker Prevention** ⭐ *CRITICAL*: Comprehensive Web Worker lifecycle management system that prevents background workers from persisting after browser close, eliminating performance degradation and resource conflicts through immediate cleanup on page unload.
- **Streaming Health Monitoring** ⭐ *NEW*: Comprehensive real-time connection monitoring system with automatic recovery, health metrics tracking, and proactive connection management for all streaming providers.
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
- **Advanced Watchlist Management**: Comprehensive watchlist system with multiple watchlist support, real-time price updates, smart data integration, and professional trading interface for monitoring multiple symbols simultaneously.
- **Centralized Selected Legs Management**: Sophisticated multi-source option leg selection system enabling seamless trading workflow across all components with unified analysis and order management.
- **Historical Data Caching System** ⭐ *NEW*: Live-updating historical data cache system optimized for Daily/6M charts with instant loading, real-time today's candle updates, and zero network delays for the most common chart configuration.

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
- **Authentication**: OAuth2 (client_id, client_secret, refresh_token). Access tokens are short-lived (15 minutes) and are automatically refreshed using the refresh_token.
- **Streaming**: DXLink protocol for professional-grade market data (quote tokens retrieved via OAuth2-authenticated API)
- **Features**: Advanced options analytics, Greeks streaming, professional trading tools

#### 4. **Public.com**
- **Type**: Market data provider
- **Authentication**: API Key
- **Features**: Real-time quotes, historical data, symbol lookup

## Project Structure

### `trade_backend`

The `trade_backend` is a FastAPI application with the following structure:

- **`main.py`**: The main entry point for the FastAPI application. It defines the API endpoints, WebSocket routes, and background tasks.
- **`providers/`**: This directory contains the different brokerage provider implementations. Each provider must implement the `BaseProvider` interface defined in `base_provider.py`.
  - **`base_provider.py`**: Defines the abstract base class for all brokerage providers.
  - **`alpaca_provider.py`**: An implementation of the `BaseProvider` interface for the Alpaca brokerage.
  - **`tradier_provider.py`**: An implementation of the `BaseProvider` interface for the Tradier brokerage.
  - **`tastytrade_provider.py`** ⭐ *NEW*: An implementation of the `BaseProvider` interface for the TastyTrade brokerage.
  - **`public_provider.py`**: An implementation of the `BaseProvider` interface for the Public.com API (for market data).
- **`models.py`**: Defines the Pydantic models used for data validation and serialization.
- **`config.py`**: Defines the application configuration, including API keys and other settings.
- **`provider_manager.py`**: Initializes and manages all available provider instances.
- **`provider_config.py`**: Manages the routing configuration for different operations.
- **`streaming_manager.py`**: Manages real-time data streaming from multiple providers.
- **`streaming_health_manager.py`** ⭐ *NEW*: Comprehensive health monitoring system for all streaming connections.

### `trade_app`

The `trade_app` is a Vue.js application with the following structure:

- **`public/`**: Contains the static assets for the application.
- **`src/`**: Contains the source code for the Vue.js application.
  - **`main.js`**: The main entry point for the Vue.js application.
  - **`App.vue`**: The root component for the application.
  - **`components/`**: Contains the reusable Vue components used throughout the application.
    - **`BottomTradingPanel.vue`**: Professional trading interface with responsive controls and real-time pricing.
    - **`OrderConfirmationDialog.vue`**: Comprehensive order confirmation with P&L analysis and risk metrics.
    - **`CollapsibleOptionsChain.vue`**: Modern collapsible options chain interface.
    - **`PayoffChart.vue`**: Dynamic profit/loss visualization chart.
    - **`WatchlistSection.vue`**: Advanced watchlist management component.
    - **`SystemRecoveryIndicator.vue`**: Real-time system health and recovery status display.
  - **`views/`**: Contains the different pages or views of the application.
    - **`OptionsTrading.vue`**: Main trading interface with options chain, chart, and order management.
    - **`ChartView.vue`**: Dedicated full-screen chart analysis view.
    - **`PositionsView.vue`**: Comprehensive portfolio management interface.
  - **`services/`**: Contains the services for interacting with the backend API and WebSockets.
    - **`optionsCalculator.js`**: Centralized calculation engine for P&L, Greeks, and risk analysis.
    - **`orderService.js`**: Order validation and submission service.
    - **`api.js`**: Backend API communication service.
    - **`webSocketClient.js`**: Real-time data streaming client.
    - **`smartMarketDataStore.js`**: Advanced reactive market data management system.
    - **`selectedLegsStore.js`**: Centralized selected legs management store.
    - **`notificationService.js`**: Professional notification system.
  - **`composables/`**: Contains reusable Vue composition functions.
    - **`useOrderManagement.js`**: Centralized order state and workflow management.
    - **`useSmartMarketData.js`**: Smart market data access composable.
    - **`useSelectedLegs.js`**: Multi-source leg selection management.
    - **`useWatchlist.js`**: Comprehensive watchlist management.
    - **`useNotifications.js`**: Notification system integration.
  - **`router/`**: Contains the routing configuration for the application.

## Getting Started

### Prerequisites

- Python 3.8+
- Node.js 14+
- API keys for your chosen providers (stored in `.env` file or configured via UI)

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

#### UI-Based Provider Management (Recommended)

Juicy Trade features a comprehensive **Provider Management System** accessible through the Settings dialog, eliminating the need for manual `.env` file editing. This system provides complete UI configurability for all trading provider connections.

**Access Provider Settings:**
1. Click the Settings icon in the top navigation bar
2. Navigate to the "Providers" section in the left sidebar
3. Use the dual-tab interface for complete provider management

**Provider Management Features:**

- **Provider Instances Tab**: Complete CRUD interface for managing provider connections
  - **3-Step Add Provider Wizard**: Provider Type → Account Type → Credentials
  - **Visual Provider Cards**: Icons, descriptions, and supported account types
  - **Real-time Status Management**: Toggle active/inactive with visual feedback
  - **Connection Testing**: Test credentials before saving
  - **Edit/Delete Operations**: Full management capabilities
  - **Security-First Design**: Required credentials not pre-filled, defaults populated for optional fields
  - **Connection Test on Update**: When updating a provider, the connection is tested before saving. If the test fails, the dialog remains open with an error message.

- **Service Routing Tab**: Configure which provider instances to use for different services
  - **Service Categories**: Trading Services and Market Data
  - **Dynamic Provider Selection**: Only shows active, compatible providers
  - **Real-time Validation**: Warns about configuration issues
  - **Status Indicators**: Visual feedback for each service configuration
  - **Manual Save Workflow**: Changes require explicit save action to prevent costly connection restarts
  - **Change Tracking**: Visual indicators for unsaved changes with reset functionality
  - **Always-Visible Save Controls**: Fixed save buttons at bottom for optimal user experience

**Supported Provider Types:**
- **Alpaca**: Live and Paper trading with comprehensive market data
- **Tradier**: Live and Paper trading with real-time streaming
- **TastyTrade** ⭐ *NEW*: Options-focused trading platform with DXLink streaming and advanced Greeks
- **Public.com**: Market data provider for quotes and historical data

**Security Features:**
- **Encrypted Credential Storage**: Secure JSON-based storage system
- **Multi-Instance Support**: Multiple instances of same provider type
- **Connection Validation**: Test before save functionality
- **Secure Editing**: Required credentials must be re-entered, optional fields show defaults

#### API-Based Configuration (Legacy)

You can still configure provider routing via the API for automation or advanced use cases:

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

## Streaming Health Monitoring System ⭐ *NEW*

Juicy Trade features a comprehensive streaming health monitoring system that provides real-time connection monitoring, automatic recovery, and detailed health metrics for all streaming providers.

### Key Features

#### Real-time Connection Monitoring

- **Connection State Tracking**: Live monitoring of connection states (CONNECTING, CONNECTED, DISCONNECTED, FAILED)
- **Health Metrics Collection**: Data received counts, error tracking, uptime monitoring, and subscription management
- **Provider Integration**: All providers (Alpaca, Tradier, TastyTrade) include comprehensive health monitoring
- **Performance Metrics**: Uptime, throughput, and reliability statistics for system optimization

#### Automatic Recovery System

- **Self-Healing Connections**: Automatic recovery from failures without manual intervention
- **Subscription Restoration**: Complete restoration of all active subscriptions after reconnection
- **Exponential Backoff**: Smart reconnection strategy with increasing delays to prevent server overload
- **Sleep/Wake Detection**: Intelligent detection and recovery from system sleep cycles

#### Proactive Health Management

- **Early Problem Detection**: Identify issues before they affect users through continuous monitoring
- **Automated Recovery**: Trigger recovery procedures automatically based on health metrics
- **Health Reporting**: Detailed health reports for system monitoring and diagnostics
- **Timeout Protection**: Prevent operations from hanging indefinitely with configurable timeouts

### Health Status API

#### Get Streaming Health Status

```bash
curl -X GET "http://localhost:8008/health/streaming" \
  -H "Accept: application/json"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "providers": {
      "alpaca": {
        "connection_id": "alpaca_12345",
        "state": "connected",
        "connected_at": "2025-01-12T19:23:16.123456",
        "last_data_received": "2025-01-12T19:25:30.789012",
        "data_received_count": 1247,
        "error_count": 0,
        "uptime_seconds": 3600.5,
        "subscriptions": ["AAPL", "SPY", "TSLA"]
      },
      "tradier": {
        "connection_id": "tradier_67890",
        "state": "connected",
        "connected_at": "2025-01-12T19:23:18.456789",
        "last_data_received": "2025-01-12T19:25:31.234567",
        "data_received_count": 892,
        "error_count": 1,
        "uptime_seconds": 3598.2,
        "subscriptions": ["SPY250718C00620000", "SPY250718P00620000"]
      }
    },
    "overall_health": {
      "is_healthy": true,
      "total_connections": 2,
      "active_connections": 2,
      "total_subscriptions": 5,
      "recovery_in_progress": false
    }
  },
  "timestamp": "2025-01-12T19:25:32.123456"
}
```

### Benefits

#### Automatic Recovery

- **No Manual Restarts**: Connections automatically recover from failures without user intervention
- **Subscription Restoration**: All symbol subscriptions restored after reconnection automatically
- **Sleep/Wake Handling**: Detects and recovers from system sleep cycles seamlessly
- **Network Recovery**: Handles network disconnections gracefully with intelligent retry logic

#### Real-time Monitoring

- **Connection Status**: Live connection state for all providers displayed in UI
- **Data Flow Tracking**: Monitor data received rates and detect stalls
- **Error Tracking**: Comprehensive error logging and analysis
- **Performance Metrics**: Uptime, throughput, and reliability statistics

## TastyTrade Integration ⭐ *NEW*

### Authentication & Setup (OAuth2-only)

TastyTrade uses OAuth2 for all API access. Username/password sessions are no longer supported. You must register an OAuth application, generate a long-lived refresh_token, and exchange it for short-lived access tokens (15 minutes).

Steps:
1. Create an OAuth application at my.tastytrade.com → Manage → My Profile → API → OAuth Applications. Save your client_id and client_secret.
2. Generate a personal grant (“Create Grant”) to obtain a refresh_token (long-lived, does not expire).
3. The backend will exchange the refresh_token for an access_token via POST {base_url}/oauth/token and automatically refresh it before expiry.

Configuration example:
```python
# TastyTrade provider configuration (OAuth2)
{
  "provider_type": "tastytrade",
  "account_type": "live",  # or "paper"
  "credentials": {
    "client_id": "your_client_id",            # optional for personal grant; include if available
    "client_secret": "your_client_secret",    # required
    "refresh_token": "your_refresh_token",    # required
    "account_id": "your_account_number",
    "base_url": "https://api.tastytrade.com"  # sandbox: https://api.cert.tastyworks.com
  }
}
```

Access token behavior:
- Access tokens expire after ~15 minutes. The backend automatically refreshes them using the refresh_token before expiry (proactive refresh window ~5 minutes).
- If an API call receives 401 due to expiry, the provider clears the token and will immediately refresh on the next call.

### DXLink Streaming Protocol

TastyTrade uses the advanced DXLink protocol for professional-grade market data:

- **Professional Market Data**: Institutional-quality real-time quotes and Greeks
- **Advanced Options Analytics**: Real-time Greeks streaming (Delta, Gamma, Theta, Vega)
- **High-Performance Streaming**: Optimized for high-frequency options trading
- **Symbol Format Conversion**: Automatic conversion between OCC standard and TastyTrade formats

Quote tokens:
- Retrieved via GET {base_url}/api-quote-tokens using the current OAuth2 Bearer access token.
- The provider manages quote token retrieval and reconnection logic automatically.

### Key Features

- **Options-Focused Platform**: Designed specifically for sophisticated options trading
- **Advanced Greeks**: Real-time streaming of all option Greeks with professional accuracy
- **Professional Analytics**: Comprehensive options analysis tools and metrics
- **Institutional-Grade Data**: High-quality market data suitable for professional trading

### DXLink Streaming Protocol

TastyTrade uses the advanced DXLink protocol for professional-grade market data:

- **Professional Market Data**: Institutional-quality real-time quotes and Greeks
- **Advanced Options Analytics**: Real-time Greeks streaming (Delta, Gamma, Theta, Vega)
- **High-Performance Streaming**: Optimized for high-frequency options trading
- **Symbol Format Conversion**: Automatic conversion between OCC standard and TastyTrade formats

### Key Features

- **Options-Focused Platform**: Designed specifically for sophisticated options trading
- **Advanced Greeks**: Real-time streaming of all option Greeks with professional accuracy
- **Professional Analytics**: Comprehensive options analysis tools and metrics
- **Institutional-Grade Data**: High-quality market data suitable for professional trading

For detailed technical documentation about the data flow architecture, see [DATA_FLOW.md](./DATA_FLOW.md).
