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
  - **`views/`**: Contains the different pages or views of the application.
  - **`services/`**: Contains the services for interacting with the backend API and WebSockets.
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

2.  **Start the Frontend**:
    - Navigate to the `trade_app` directory.
    - Start the Vue.js development server:
      ```bash
      npm run dev
      ```

The application will be available at `http://localhost:5173`.

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
