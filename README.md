# Smart Iron Trade

A robust Iron Butterfly trading application built with Streamlit and FastAPI, featuring live option price streaming via Alpaca API.

## Architecture

The application consists of two main components:

1. **Streaming Service** - A FastAPI application that maintains persistent WebSocket connections to Alpaca's streaming API for option and stock quotes.
2. **Streamlit App** - A user interface for configuring and placing Iron Butterfly trades, with live price updates.

This architecture ensures that WebSocket connections persist across Streamlit page reloads, preventing connection limit errors and providing a smooth user experience.

## Setup

### Prerequisites

- Python 3.8+
- Alpaca API keys (stored in `.env` file)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/yourusername/smart_iron_trade.git
cd smart_iron_trade
```

2. Create a virtual environment and install dependencies:

```bash
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt
```

3. Create a `.env` file with your Alpaca API keys:

```
APCA_API_KEY_ID_PAPER=your_paper_api_key
APCA_API_SECRET_KEY_PAPER=your_paper_api_secret
APCA_API_KEY_ID_LIVE=your_live_api_key
APCA_API_SECRET_KEY_LIVE=your_live_api_secret
ALPACA_BASE_URL_PAPER=https://paper-api.alpaca.markets
ALPACA_BASE_URL_LIVE=https://api.alpaca.markets
ALPACA_DATA_URL=https://data.alpaca.markets
```

## Running the Application

### Step 1: Start the Streaming Service

Start the FastAPI streaming service first:

```bash
python streaming_service.py
```

This will start the service on http://localhost:8008. You can check the service status by visiting this URL in your browser.

### Step 2: Start the Streamlit App

In a new terminal, start the Streamlit app:

```bash
streamlit run app_with_service.py
```

This will open the Streamlit app in your default web browser.

## Features

- **Live Option and Stock Price Streaming**: Real-time price updates via Alpaca's WebSocket API
- **Butterfly Strategy Selection**: Choose between IRON Butterfly, CALL Butterfly, and PUT Butterfly
- **Dynamic Strike Selection**: Automatically selects strikes based on current price and leg width
- **Live Payoff Diagram**: Visual representation of the butterfly strategy's profit/loss profile
- **Order Placement**: Submit orders directly to Alpaca with confirmation dialog
- **Persistent Connections**: WebSocket connections persist across page reloads

## Troubleshooting

- **Connection Issues**: If you encounter connection issues, use the "Restart Streaming Service" button in the Streamlit app.
- **Service Not Running**: If the streaming service is not running, you'll see an error message in the Streamlit app. Make sure the streaming service is running on http://localhost:8000.

## Development

### Project Structure

- `streaming_service.py` - FastAPI service for persistent WebSocket connections
- `streaming_client.py` - Client library for interacting with the streaming service
- `app_with_service.py` - Streamlit app that uses the streaming client
- `app.py` - Original Streamlit app (without the streaming service)
- `stream_option_data.py` - Original streaming implementation (deprecated)

### Adding New Features

To add new features:

1. Update the streaming service if new data streams are needed
2. Update the streaming client to interact with the new service endpoints
3. Implement the feature in the Streamlit app

## License

MIT
