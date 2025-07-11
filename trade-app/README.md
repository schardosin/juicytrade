# Options Trading Platform

A professional options trading platform built with Vue 3, featuring a modern interface inspired by Tastytrade. This standalone application provides comprehensive options trading capabilities with real-time data streaming and advanced order management.

## Features

### Core Trading Features

- **Professional Options Chain**: Interactive options chain with real-time bid/ask prices and Greeks (Delta, Theta)
- **Multi-leg Strategy Support**: Build complex options strategies with up to multiple legs
- **Real-time Price Streaming**: Live price updates for underlying assets and options contracts
- **Risk Management**: Automatic calculation of maximum risk and maximum reward
- **Order Management**: Professional order ticket with limit/market orders and various time-in-force options

### User Interface

- **Tastytrade-inspired Design**: Modern dark theme with professional trading interface
- **Responsive Layout**: Optimized for desktop trading with resizable panels
- **Interactive Charts**: Real-time profit/loss visualization using Chart.js
- **Symbol Search**: Quick symbol lookup and switching
- **Account Information**: Display of net liquidation value and buying power

### Advanced Features

- **WebSocket Integration**: Real-time data streaming for live market updates
- **Order Confirmation**: Detailed order review before execution
- **Position Tracking**: Monitor open positions and P&L
- **Market Status**: Live market status indicators

## Technology Stack

- **Vue 3**: Modern reactive framework with Composition API
- **PrimeVue**: Professional UI component library
- **Chart.js**: Interactive charting for payoff diagrams
- **WebSocket**: Real-time data streaming
- **Vite**: Fast build tool and development server

## Quick Start

1. **Install dependencies:**

```bash
npm install
```

2. **Start development server:**

```bash
npm run dev
```

3. **Build for production:**

```bash
npm run build
```

4. **Preview production build:**

```bash
npm run preview
```

## Backend Integration

The platform connects to a Python backend service for:

- Real-time market data streaming
- Options chain data
- Order execution
- Account information

**Backend Requirements:**

- Python streaming service running on `localhost:8008`
- WebSocket endpoint for real-time data
- REST API for options data and order management

## Architecture

### Components Structure

```
src/
├── components/
│   ├── TopBar.vue           # Header with navigation and account info
│   ├── SideNav.vue          # Left navigation menu
│   ├── OptionsChain.vue     # Interactive options chain display
│   ├── OrderTicket.vue      # Order management panel
│   ├── PayoffChart.vue      # P&L visualization chart
│   └── Order*.vue          # Order confirmation dialogs
├── services/
│   ├── api.js              # REST API client
│   ├── webSocketClient.js  # WebSocket streaming client
│   └── orderService.js     # Order management service
├── utils/
│   └── chartUtils.js       # Chart generation utilities
└── views/
    └── OptionsTrading.vue  # Main trading interface
```

### Key Services

**API Service (`api.js`)**

- Options chain data retrieval
- Underlying asset pricing
- Account information

**WebSocket Client (`webSocketClient.js`)**

- Real-time price streaming
- Market data subscriptions
- Connection management

**Order Management (`useOrderManagement.js`)**

- Order validation and submission
- Risk calculations
- Order status tracking

## API Endpoints

### REST Endpoints

- `GET /api/underlying_price/{symbol}` - Get current underlying price
- `GET /api/options_chain` - Retrieve options chain data
- `POST /api/orders` - Place new orders

### WebSocket Events

- `price_update` - Real-time price updates
- `market_status` - Market open/close status
- `order_status` - Order execution updates

## Configuration

The application can be configured through environment variables:

```env
VITE_API_BASE_URL=http://localhost:8008
VITE_WS_URL=ws://localhost:8008/ws
```

## Development

### Project Structure

- Modern Vue 3 with Composition API
- TypeScript-ready (can be easily migrated)
- Component-based architecture
- Reactive state management
- Professional error handling

### Styling

- Dark theme optimized for trading
- Responsive design principles
- Professional color scheme
- Consistent spacing and typography

## Production Deployment

1. **Build the application:**

```bash
npm run build
```

2. **Deploy the `dist/` folder** to your web server

3. **Configure reverse proxy** for API calls to your backend service

## Browser Support

- Chrome/Chromium 88+
- Firefox 85+
- Safari 14+
- Edge 88+

## License

Private - All rights reserved

---

**Professional Options Trading Platform** - Built for serious traders who demand performance, reliability, and a superior user experience.
