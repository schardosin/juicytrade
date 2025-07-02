# Iron Butterfly Trade Vue App

This is a Vue 3 + PrimeVue application that replicates the functionality of the original Streamlit app for Iron Butterfly trading.

## Features

- **Real-time Data Streaming**: Live price updates for stocks and options
- **Interactive Configuration**: Configure butterfly trades with various parameters
- **Live Payoff Charts**: Real-time butterfly payoff diagrams using Chart.js
- **Order Management**: Place and confirm butterfly orders with detailed breakdowns
- **Multiple Strategy Types**: Support for IRON, CALL, and PUT butterfly strategies

## Technology Stack

- **Vue 3**: Modern reactive framework with Composition API
- **PrimeVue**: Comprehensive UI component library
- **Chart.js**: Interactive charting for payoff diagrams
- **Axios**: HTTP client for API communication
- **Vite**: Fast build tool and development server

## Setup

1. Install dependencies:

```bash
npm install
```

2. Start the development server:

```bash
npm run dev
```

3. Build for production:

```bash
npm run build
```

## Backend Integration

The app connects to the same backend API as the original Streamlit app:

- Base URL: `http://localhost:8008`
- Proxy configured in Vite to route `/api/*` requests to the backend

## Key Components

### Services

- `api.js`: Backend API integration
- `streamingClient.js`: Real-time data streaming client

### Utils

- `chartUtils.js`: Chart generation and payoff calculations

### Main App

- `App.vue`: Main application component with all trading functionality

## API Endpoints Used

- `GET /next_market_date`: Get next trading date
- `GET /prices/stocks`: Get stock prices
- `GET /options_chain`: Get options chain data
- `GET /prices/options`: Get option prices
- `POST /place_butterfly_order`: Place butterfly orders
- `POST /subscribe/stocks`: Subscribe to stock price updates
- `POST /subscribe/options`: Subscribe to option price updates

## Features Comparison with Streamlit App

✅ Service status monitoring
✅ Trade configuration (expiry, strategy, leg width, offset, symbol)
✅ Real-time underlying price display
✅ Options chain fetching and display
✅ Live butterfly legs table with pricing
✅ Interactive payoff chart with break-even points
✅ Order confirmation dialog with detailed breakdown
✅ Order placement and result handling
✅ Real-time price streaming and updates
✅ Service restart functionality

The Vue app provides the same functionality as the Streamlit app with a more modern, responsive UI.
