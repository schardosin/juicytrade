import os
import asyncio
import uvicorn
from fastapi import FastAPI, BackgroundTasks, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import List, Dict, Optional, Set
from alpaca.data.live import OptionDataStream, StockDataStream
from alpaca.data.historical import StockHistoricalDataClient
from alpaca.data.requests import StockLatestQuoteRequest
from alpaca.trading.client import TradingClient
from alpaca.trading.stream import TradingStream
from dotenv import load_dotenv
import logging
from datetime import datetime, date
import requests
import json

# Function to fetch initial stock prices
async def fetch_initial_prices(symbols: List[str]):
    client = StockHistoricalDataClient(ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE)
    request_params = StockLatestQuoteRequest(symbol_or_symbols=symbols)
    try:
        latest_quotes = client.get_stock_latest_quote(request_params)
        for symbol, quote in latest_quotes.items():
            stock_prices[symbol] = {
                "ask": quote.ask_price,
                "bid": quote.bid_price,
                "timestamp": datetime.now().isoformat()
            }
        logger.info(f"Fetched initial prices for {len(symbols)} symbols")
    except Exception as e:
        logger.error(f"Error fetching initial prices: {e}")

# --- Basic Configuration ---

# Configure logging to provide timestamps and clear messages
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger("streaming_service")
logger.setLevel(logging.INFO)

# Load environment variables from a .env file
load_dotenv()

# It's critical to use LIVE keys for streaming, especially for the stock data stream.
ALPACA_API_KEY_LIVE = os.getenv("APCA_API_KEY_ID_LIVE")
ALPACA_API_SECRET_LIVE = os.getenv("APCA_API_SECRET_KEY_LIVE")
ALPACA_API_KEY_PAPER = os.getenv("APCA_API_KEY_ID_PAPER")
ALPACA_API_SECRET_PAPER = os.getenv("APCA_API_SECRET_KEY_PAPER")
ALPACA_BASE_URL_LIVE = os.getenv("ALPACA_BASE_URL_LIVE")
ALPACA_BASE_URL_PAPER = os.getenv("ALPACA_BASE_URL_PAPER")

def get_api_credentials(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_API_KEY_PAPER, ALPACA_API_SECRET_PAPER
    else:
        return ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE

def get_base_url(is_paper_trade=True):
    if is_paper_trade:
        return ALPACA_BASE_URL_PAPER
    else:
        return ALPACA_BASE_URL_LIVE

def get_options_chain(symbol, expiry, strategy_type):
    url = f"{get_base_url(is_paper_trade=True)}/v2/options/contracts"
    api_key, api_secret = get_api_credentials(is_paper_trade=True)
    option_type = "call" if strategy_type == "CALL Butterfly" else "put" if strategy_type == "PUT Butterfly" else None
    params = {
        "underlying_symbols": symbol,
        "expiration_date": expiry,
        "root_symbol": symbol,
        "type": option_type,
        "limit": 1000
    }
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
        "accept": "application/json"
    }
    resp = requests.get(url, headers=headers, params=params)
    try:
        response_data = resp.json()
        if resp.status_code == 200:
            contracts = response_data.get("option_contracts", [])
            return contracts
    except Exception as e:
        logger.error(f"Error fetching options chain: {e}")
    return []

# --- FastAPI Application Setup ---

app = FastAPI(title="Alpaca Streaming Service")

# Add CORS middleware to allow all origins, methods, and headers.
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- WebSocket Connection Manager ---

class ConnectionManager:
    def __init__(self):
        self.active_connections: Dict[WebSocket, Set[str]] = {}
        self.symbol_subscribers: Dict[str, Set[WebSocket]] = {}
    
    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections[websocket] = set()
        logger.info(f"WebSocket client connected. Total connections: {len(self.active_connections)}")
    
    def disconnect(self, websocket: WebSocket):
        # Remove from all symbol subscriptions
        if websocket in self.active_connections:
            subscribed_symbols = self.active_connections[websocket]
            for symbol in subscribed_symbols:
                if symbol in self.symbol_subscribers:
                    self.symbol_subscribers[symbol].discard(websocket)
                    if len(self.symbol_subscribers[symbol]) == 0:
                        del self.symbol_subscribers[symbol]
            del self.active_connections[websocket]
        logger.info(f"WebSocket client disconnected. Total connections: {len(self.active_connections)}")
    
    def subscribe_symbol(self, websocket: WebSocket, symbol: str):
        if websocket in self.active_connections:
            self.active_connections[websocket].add(symbol)
            if symbol not in self.symbol_subscribers:
                self.symbol_subscribers[symbol] = set()
            self.symbol_subscribers[symbol].add(websocket)
            logger.info(f"Client subscribed to {symbol}. Total subscribers for {symbol}: {len(self.symbol_subscribers[symbol])}")
    
    async def broadcast_price_update(self, symbol: str, price_data: dict):
        if symbol in self.symbol_subscribers:
            message = {
                "type": "price_update",
                "symbol": symbol,
                "data": price_data
            }
            disconnected = []
            for websocket in self.symbol_subscribers[symbol]:
                try:
                    await websocket.send_text(json.dumps(message))
                except Exception as e:
                    logger.warning(f"Failed to send message to WebSocket client: {e}")
                    disconnected.append(websocket)
            
            # Clean up disconnected clients
            for ws in disconnected:
                self.disconnect(ws)
    
    def get_stats(self):
        return {
            "total_connections": len(self.active_connections),
            "total_subscriptions": sum(len(subs) for subs in self.active_connections.values()),
            "symbols_with_subscribers": list(self.symbol_subscribers.keys())
        }

# Global connection manager instance
manager = ConnectionManager()

# --- Auto-subscription Helper Functions ---

async def auto_subscribe_position_symbols(positions):
    """
    Automatically subscribe to streaming for symbols found in positions.
    """
    global subscribed_options, subscribed_stocks
    
    symbols_to_add_options = set()
    symbols_to_add_stocks = set()
    
    for position in positions:
        symbol = position.get("symbol")
        asset_class = position.get("asset_class")
        underlying_symbol = position.get("underlying_symbol")
        
        if not symbol:
            continue
            
        if asset_class == "us_option":
            symbols_to_add_options.add(symbol)
            # Also add underlying symbol for stock streaming
            if underlying_symbol:
                symbols_to_add_stocks.add(underlying_symbol)
        else:
            symbols_to_add_stocks.add(symbol)
    
    # Check if we need to add new symbols
    new_options = symbols_to_add_options - subscribed_options
    new_stocks = symbols_to_add_stocks - subscribed_stocks
    
    if new_options or new_stocks:
        logger.info(f"Auto-subscribing to position symbols - Options: {new_options}, Stocks: {new_stocks}")
        
        # Update subscription sets
        subscribed_options.update(new_options)
        subscribed_stocks.update(new_stocks)
        
        # Fetch initial prices for new stock symbols
        if new_stocks:
            await fetch_initial_prices(list(new_stocks))
        
        # Restart streaming to include new symbols
        await reset_streaming()
        
        logger.info(f"Auto-subscription complete. Total subscriptions - Options: {len(subscribed_options)}, Stocks: {len(subscribed_stocks)}")
    else:
        logger.info("No new symbols to auto-subscribe from positions")

# --- Global State Management ---

# Dictionaries to hold the latest price data
option_prices: Dict[str, Dict] = {}
stock_prices: Dict[str, Dict] = {}

# Dictionary to hold open orders data
open_orders: Dict[str, Dict] = {}

# Sets to keep track of currently subscribed symbols
subscribed_options: Set[str] = set()
subscribed_stocks: Set[str] = set()

# Global references to the stream clients and the running asyncio task
option_stream: Optional[OptionDataStream] = None
stock_stream: Optional[StockDataStream] = None
trading_stream: Optional[TradingStream] = None
stream_task: Optional[asyncio.Task] = None
trading_stream_task: Optional[asyncio.Task] = None


# --- Pydantic Models for API Requests ---

class SymbolRequest(BaseModel):
    symbols: List[str]

class SymbolResponse(BaseModel):
    success: bool
    message: str
    symbols: List[str]


# --- Real-Time Data Handlers ---

async def option_quote_handler(data):
    """This function is called for every new option quote received."""
    symbol = data.symbol
    price_data = {
        "ask": data.ask_price,
        "bid": data.bid_price,
        "timestamp": datetime.now().isoformat()
    }
    option_prices[symbol] = price_data
    
    # Broadcast to WebSocket clients
    await manager.broadcast_price_update(symbol, price_data)
    
async def stock_quote_handler(data):
    """This function is called for every new stock quote received."""
    symbol = data.symbol
    price_data = {
        "ask": data.ask_price,
        "bid": data.bid_price,
        "timestamp": datetime.now().isoformat()
    }
    stock_prices[symbol] = price_data
    
    # Broadcast to WebSocket clients
    await manager.broadcast_price_update(symbol, price_data)
    
# --- Trading Stream Handlers ---

async def trade_update_handler(data):
    """Handle trade updates from Alpaca TradingStream."""
    try:
        order_id = data.order.id
        order_status = data.order.status.value
        
        # Get symbol - for multi-leg orders, symbol might be None, so get from legs
        symbol = data.order.symbol
        if not symbol and hasattr(data.order, 'legs') and data.order.legs:
            symbol = f"Multi-leg ({len(data.order.legs)} legs)"
        
        # Add detailed logging to debug what events we're receiving
        logger.info(f"TRADE UPDATE RECEIVED: Order ID: {order_id}, Status: {order_status}, Symbol: {symbol}")
        logger.info(f"TRADE UPDATE DETAILS: Event: {data.event}, Timestamp: {data.timestamp}")
        logger.info(f"TRADE UPDATE RAW DATA: Has legs: {hasattr(data.order, 'legs')}, Legs count: {len(data.order.legs) if hasattr(data.order, 'legs') and data.order.legs else 0}")
        
        # Create order data structure
        order_data = {
            "id": order_id,
            "asset": data.order.symbol,
            "order_type": f"{data.order.order_type.value.title()} @ {data.order.limit_price or '-'}",
            "side": data.order.side.value if hasattr(data.order.side, 'value') else str(data.order.side),
            "qty": float(data.order.qty),
            "filled_qty": float(data.order.filled_qty) if data.order.filled_qty else 0.0,
            "avg_fill_price": float(data.order.filled_avg_price) if data.order.filled_avg_price else None,
            "status": order_status,
            "source": "access_key",  # Default source
            "submitted_at": data.order.submitted_at.isoformat() if data.order.submitted_at else datetime.now().isoformat(),
            "filled_at": data.order.filled_at.isoformat() if data.order.filled_at else None,
            "timestamp": datetime.now().isoformat()
        }
        
        # Convert order_id to string for consistent key usage
        order_id_str = str(order_id)
        order_data["id"] = order_id_str
        
        # Handle multi-leg orders
        if hasattr(data.order, 'legs') and data.order.legs:
            # This is a multi-leg order, create a parent entry
            order_data["asset"] = f"{len(data.order.legs)}-Leg Order"
            order_data["side"] = "-"
            
            # Store the parent order
            open_orders[order_id_str] = order_data
            
            # Store individual legs
            for i, leg in enumerate(data.order.legs):
                leg_id = f"{order_id}_leg_{i}"
                leg_data = {
                    "id": leg_id,
                    "parent_id": order_id_str,
                    "asset": leg.symbol,
                    "order_type": f"{data.order.order_type.value.title()} @ -",
                    "side": leg.side.value if hasattr(leg.side, 'value') else str(leg.side),
                    "qty": float(leg.qty),
                    "filled_qty": 0.0,  # Individual leg fill info may not be available
                    "avg_fill_price": None,
                    "status": order_status,
                    "source": "access_key",
                    "submitted_at": order_data["submitted_at"],
                    "filled_at": order_data["filled_at"],
                    "timestamp": datetime.now().isoformat()
                }
                open_orders[leg_id] = leg_data
        else:
            # Single-leg order
            open_orders[order_id_str] = order_data
        
        # Remove filled or cancelled orders from open_orders
        if order_status in ['filled', 'canceled', 'expired', 'rejected']:
            # Remove the order and any legs
            orders_to_remove = [order_id_str]
            for oid in list(open_orders.keys()):
                if oid.startswith(f"{order_id}_leg_"):
                    orders_to_remove.append(oid)
            
            for oid in orders_to_remove:
                if oid in open_orders:
                    del open_orders[oid]
        
        # Broadcast updated open orders to all WebSocket clients
        await broadcast_open_orders()
        
        logger.info(f"ORDER UPDATE: {order_id} - {data.order.symbol} - {order_status}")
        
    except Exception as e:
        logger.error(f"Error processing trade update: {e}", exc_info=True)

async def broadcast_open_orders():
    """Broadcast current open orders to all connected WebSocket clients."""
    # Get fresh data from API instead of using potentially stale cache
    try:
        fresh_orders_response = get_open_orders()
        message = {
            "type": "open_orders_update",
            "data": fresh_orders_response
        }
        
        logger.info(f"Broadcasting fresh open orders: {fresh_orders_response.get('total_orders', 0)} orders")
        
        # Send to all connected clients
        disconnected = []
        for websocket in manager.active_connections:
            try:
                await websocket.send_text(json.dumps(message))
            except Exception as e:
                logger.warning(f"Failed to send open orders update to WebSocket client: {e}")
                disconnected.append(websocket)
        
        # Clean up disconnected clients
        for ws in disconnected:
            manager.disconnect(ws)
            
    except Exception as e:
        logger.error(f"Error broadcasting open orders: {e}")


# --- Streaming Logic and Lifecycle Management ---

async def run_streaming():
    """
    Initializes and runs the data streams based on the global subscription sets.
    This function is designed to be run as a cancellable asyncio task.
    """
    global option_stream, stock_stream

    if not ALPACA_API_KEY_LIVE or not ALPACA_API_SECRET_LIVE:
        logger.error("LIVE API credentials not found. Streaming will not start.")
        return

    try:
        tasks_to_run = []

        if subscribed_options:
            logger.info(f"Initializing OptionDataStream for {len(subscribed_options)} symbols.")
            option_stream = OptionDataStream(ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE)
            for symbol in subscribed_options:
                option_stream.subscribe_quotes(option_quote_handler, symbol)
            tasks_to_run.append(option_stream._run_forever())

        if subscribed_stocks:
            logger.info(f"Initializing StockDataStream for {len(subscribed_stocks)} symbols.")
            stock_stream = StockDataStream(ALPACA_API_KEY_LIVE, ALPACA_API_SECRET_LIVE)
            for symbol in subscribed_stocks:
                stock_stream.subscribe_quotes(stock_quote_handler, symbol)
            tasks_to_run.append(stock_stream._run_forever())

        if tasks_to_run:
            logger.info("Starting data stream(s)...")
            await asyncio.gather(*tasks_to_run)
        else:
            logger.info("No symbols are subscribed. Stream not starting.")

    except asyncio.CancelledError:
        logger.info("Streaming task was successfully cancelled.")
    except Exception as e:
        logger.error(f"Error in streaming task: {e}", exc_info=True)
    finally:
        logger.info("Streaming task has finished.")
        option_stream = None
        stock_stream = None

async def reset_streaming():
    """
    Resets the streaming connection by closing existing streams and starting a new one.
    """
    global option_stream, stock_stream, stream_task
    
    logger.info("Resetting streaming connection")
    
    # Close existing streams
    if option_stream:
        await option_stream.close()
    if stock_stream:
        await stock_stream.close()
    
    # Cancel existing streaming task
    if stream_task and not stream_task.done():
        stream_task.cancel()
        try:
            await stream_task
        except asyncio.CancelledError:
            pass
    
    # Start new streaming task
    stream_task = asyncio.create_task(run_streaming())
    logger.info("Started new streaming background task")

async def start_streaming_background():
    """
    Manages the lifecycle of the streaming task.
    """
    await reset_streaming()

# --- Trading Stream Management ---

async def run_trading_stream():
    """
    Initializes and runs the TradingStream for order updates.
    """
    global trading_stream
    
    if not ALPACA_API_KEY_PAPER or not ALPACA_API_SECRET_PAPER:
        logger.error("PAPER API credentials not found. Trading stream will not start.")
        return
    
    try:
        logger.info("Initializing TradingStream for order updates...")
        trading_stream = TradingStream(ALPACA_API_KEY_PAPER, ALPACA_API_SECRET_PAPER, paper=True)
        
        # Subscribe to trade updates
        trading_stream.subscribe_trade_updates(trade_update_handler)
        
        logger.info("Starting TradingStream...")
        await trading_stream._run_forever()
        
    except asyncio.CancelledError:
        logger.info("Trading stream task was successfully cancelled.")
    except Exception as e:
        logger.error(f"Error in trading stream task: {e}", exc_info=True)
    finally:
        logger.info("Trading stream task has finished.")
        trading_stream = None

async def start_trading_stream():
    """
    Start the trading stream for order updates.
    """
    global trading_stream_task
    
    # Cancel existing trading stream task if running
    if trading_stream_task and not trading_stream_task.done():
        trading_stream_task.cancel()
        try:
            await trading_stream_task
        except asyncio.CancelledError:
            pass
    
    # Start new trading stream task
    trading_stream_task = asyncio.create_task(run_trading_stream())
    logger.info("Started trading stream background task")

async def stop_trading_stream():
    """
    Stop the trading stream.
    """
    global trading_stream, trading_stream_task
    
    logger.info("Stopping trading stream...")
    
    # Close trading stream
    if trading_stream:
        await trading_stream.close()
    
    # Cancel trading stream task
    if trading_stream_task and not trading_stream_task.done():
        trading_stream_task.cancel()
        try:
            await trading_stream_task
        except asyncio.CancelledError:
            pass
    
    logger.info("Trading stream stopped")


# --- FastAPI Application Lifecycle Events ---

@app.on_event("startup")
async def startup_event():
    """Action to take when the application starts."""
    logger.info("Application started. Fetching initial prices for any pre-configured symbols.")
    if subscribed_stocks:
        await fetch_initial_prices(list(subscribed_stocks))
    
    # Start the trading stream for order updates
    await start_trading_stream()
    
    # Sync initial open orders from API
    await sync_open_orders_from_api()
    
    logger.info("Waiting for initial subscriptions to start streaming.")

@app.on_event("shutdown")
async def shutdown_event():
    """Action to take when the application shuts down."""
    global stream_task, option_stream, stock_stream, trading_stream, trading_stream_task
    logger.info("Application shutting down...")
    
    # Gracefully close connections to Alpaca's servers
    if option_stream:
        await option_stream.close()
    if stock_stream:
        await stock_stream.close()
    if trading_stream:
        await trading_stream.close()
        
    # Cancel the running tasks
    if stream_task and not stream_task.done():
        stream_task.cancel()
        try:
            await stream_task
        except asyncio.CancelledError:
            logger.info("Streaming task cancelled successfully on shutdown.")
    
    if trading_stream_task and not trading_stream_task.done():
        trading_stream_task.cancel()
        try:
            await trading_stream_task
        except asyncio.CancelledError:
            logger.info("Trading stream task cancelled successfully on shutdown.")


# --- API Endpoints ---

from fastapi import Body

class ButterflyOrderRequest(BaseModel):
    symbol: str
    expiry: str
    strategy_type: str
    legs: list
    order_price: float
    order_offset: float
    qty: int = 1
    time_in_force: str = "day"
    order_type: str = "limit"

@app.post("/place_butterfly_order")
async def place_butterfly_order(order: ButterflyOrderRequest):
    """
    Place a butterfly order using Alpaca API.
    """
    api_key, api_secret = get_api_credentials(is_paper_trade=True)
    order_url = f"{get_base_url(is_paper_trade=True)}/v2/orders"
    headers = {
        "APCA-API-KEY-ID": api_key,
        "APCA-API-SECRET-KEY": api_secret,
        "Content-Type": "application/json"
    }
    import json
    order_payload = {
        "order_class": "mleg",
        "time_in_force": order.time_in_force,
        "qty": str(order.qty),
        "type": order.order_type,
        "limit_price": f"{order.order_price:.2f}",
        "legs": order.legs
    }
    try:
        resp = requests.post(order_url, headers=headers, data=json.dumps(order_payload))
        if resp.status_code in (200, 201):
            return {"success": True, "order": resp.json()}
        else:
            return {"success": False, "error": f"Order failed: {resp.status_code} {resp.text}"}
    except Exception as e:
        return {"success": False, "error": str(e)}


@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """WebSocket endpoint for real-time price streaming."""
    await manager.connect(websocket)
    try:
        while True:
            data = await websocket.receive_text()
            message = json.loads(data)
            
            if message["type"] == "subscribe":
                symbols = message.get("symbols", [])
                new_options = set()
                new_stocks = set()
                
                for symbol in symbols:
                    manager.subscribe_symbol(websocket, symbol)
                    
                    # Determine if this is an option or stock symbol
                    # Option symbols are typically longer and contain specific patterns
                    if len(symbol) > 10 and any(c in symbol for c in ['C', 'P']) and any(c.isdigit() for c in symbol[-8:]):
                        # This looks like an option symbol
                        if symbol not in subscribed_options:
                            new_options.add(symbol)
                    else:
                        # This looks like a stock symbol
                        if symbol not in subscribed_stocks:
                            new_stocks.add(symbol)
                    
                    # Send current price if available
                    if symbol in stock_prices:
                        await websocket.send_text(json.dumps({
                            "type": "price_update",
                            "symbol": symbol,
                            "data": stock_prices[symbol]
                        }))
                    elif symbol in option_prices:
                        await websocket.send_text(json.dumps({
                            "type": "price_update", 
                            "symbol": symbol,
                            "data": option_prices[symbol]
                        }))
                
                # Add new symbols to global subscription sets and restart streaming
                if new_options or new_stocks:
                    logger.info(f"WebSocket: Adding new symbols to Alpaca subscriptions - Options: {new_options}, Stocks: {new_stocks}")
                    
                    # Update global subscription sets
                    subscribed_options.update(new_options)
                    subscribed_stocks.update(new_stocks)
                    
                    # Fetch initial prices for new stock symbols
                    if new_stocks:
                        await fetch_initial_prices(list(new_stocks))
                    
                    # Restart streaming to include new symbols
                    asyncio.create_task(reset_streaming())
                    
                    logger.info(f"WebSocket: Alpaca streaming restarted with new symbols. Total - Options: {len(subscribed_options)}, Stocks: {len(subscribed_stocks)}")
                        
                # Send confirmation
                await websocket.send_text(json.dumps({
                    "type": "subscription_confirmed",
                    "symbols": symbols,
                    "message": f"Subscribed to {len(symbols)} symbols and added {len(new_options)} options + {len(new_stocks)} stocks to Alpaca streaming"
                }))
            
            elif message["type"] == "get_positions":
                # Send current positions via WebSocket
                try:
                    logger.info("WebSocket: Received get_positions request")
                    # Call the positions function directly (it's not async)
                    positions_response = get_open_positions()
                    logger.info(f"WebSocket: Got positions response with {positions_response.get('total_positions', 0)} positions")
                    
                    # Auto-subscribe to streaming for position symbols
                    if positions_response.get("success") and positions_response.get("positions"):
                        await auto_subscribe_position_symbols(positions_response["positions"])
                    
                    response_message = {
                        "type": "positions_update",
                        "data": positions_response
                    }
                    logger.info("WebSocket: Sending positions response")
                    await websocket.send_text(json.dumps(response_message))
                    logger.info("WebSocket: Positions response sent successfully")
                except Exception as e:
                    logger.error(f"Error getting positions via WebSocket: {e}", exc_info=True)
                    await websocket.send_text(json.dumps({
                        "type": "positions_error",
                        "error": str(e)
                    }))
            
            elif message["type"] == "get_open_orders":
                # Send current open orders via WebSocket (backward compatibility)
                try:
                    logger.info("WebSocket: Received get_open_orders request")
                    # Call the orders function directly (it's not async)
                    orders_response = get_open_orders()
                    logger.info(f"WebSocket: Got open orders response with {orders_response.get('total_orders', 0)} orders")
                    
                    response_message = {
                        "type": "open_orders_update",
                        "data": orders_response
                    }
                    logger.info("WebSocket: Sending open orders response")
                    await websocket.send_text(json.dumps(response_message))
                    logger.info("WebSocket: Open orders response sent successfully")
                except Exception as e:
                    logger.error(f"Error getting open orders via WebSocket: {e}", exc_info=True)
                    await websocket.send_text(json.dumps({
                        "type": "open_orders_error",
                        "error": str(e)
                    }))
            
            elif message["type"] == "get_orders":
                # Send orders with filtering via WebSocket
                try:
                    status_filter = message.get("filter", "open")
                    date_filter = message.get("date_filter", "today")
                    logger.info(f"WebSocket: Received get_orders request with filter: {status_filter}, date: {date_filter}")
                    # Call the orders function directly (it's not async)
                    orders_response = get_orders(status_filter, date_filter)
                    logger.info(f"WebSocket: Got orders response with {orders_response.get('total_orders', 0)} orders")
                    
                    response_message = {
                        "type": "orders_update",
                        "data": orders_response,
                        "filter": status_filter,
                        "date_filter": date_filter
                    }
                    logger.info("WebSocket: Sending orders response")
                    await websocket.send_text(json.dumps(response_message))
                    logger.info("WebSocket: Orders response sent successfully")
                except Exception as e:
                    logger.error(f"Error getting orders via WebSocket: {e}", exc_info=True)
                    await websocket.send_text(json.dumps({
                        "type": "orders_error",
                        "error": str(e)
                    }))
                        
    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception as e:
        logger.error(f"WebSocket error: {e}")
        manager.disconnect(websocket)

@app.get("/")
async def root():
    """Root endpoint to check the status of the service."""
    websocket_stats = manager.get_stats()
    return {
        "service_status": "running",
        "options_subscribed": list(subscribed_options),
        "stocks_subscribed": list(subscribed_stocks),
        "is_streaming_task_active": stream_task is not None and not stream_task.done(),
        "is_trading_stream_active": trading_stream_task is not None and not trading_stream_task.done(),
        "total_option_prices": len(option_prices),
        "total_stock_prices": len(stock_prices),
        "total_open_orders": len(open_orders),
        "websocket_connections": websocket_stats["total_connections"],
        "websocket_subscriptions": websocket_stats["total_subscriptions"],
        "symbols_with_websocket_subscribers": websocket_stats["symbols_with_subscribers"]
    }

@app.get("/subscriptions")
async def get_subscriptions():
    """Get current subscription status and streaming information."""
    return {
        "options": {
            "subscribed_symbols": list(subscribed_options),
            "symbols_with_data": list(option_prices.keys()),
            "count": len(subscribed_options)
        },
        "stocks": {
            "subscribed_symbols": list(subscribed_stocks),
            "symbols_with_data": list(stock_prices.keys()),
            "count": len(subscribed_stocks)
        },
        "streaming": {
            "is_active": stream_task is not None and not stream_task.done(),
            "total_subscriptions": len(subscribed_options) + len(subscribed_stocks)
        }
    }

@app.post("/subscribe/options", response_model=SymbolResponse)
async def subscribe_options(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Endpoint to set the list of subscribed option symbols."""
    global subscribed_options
    
    new_symbol_set = set(request.symbols)
    if new_symbol_set == subscribed_options:
        return {"success": True, "message": "No change in option subscriptions.", "symbols": list(subscribed_options)}
    
    old_symbols = subscribed_options - new_symbol_set
    new_symbols = new_symbol_set - subscribed_options
    subscribed_options = new_symbol_set
    
    # Clear out price data for any symbols that were just removed
    for symbol in old_symbols:
        if symbol in option_prices:
            del option_prices[symbol]
    
    logger.info(f"Updated option subscriptions. Added: {new_symbols}, Removed: {old_symbols}")
    background_tasks.add_task(reset_streaming)
    
    return {"success": True, "message": f"Updated option subscriptions. Added: {len(new_symbols)}, Removed: {len(old_symbols)}. Restarting stream.", "symbols": list(subscribed_options)}

@app.post("/subscribe/stocks", response_model=SymbolResponse)
async def subscribe_stocks(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Endpoint to set the list of subscribed stock symbols."""
    global subscribed_stocks
    
    new_symbol_set = set(request.symbols)
    if new_symbol_set == subscribed_stocks:
        return {"success": True, "message": "No change in stock subscriptions.", "symbols": list(subscribed_stocks)}
        
    new_symbols = new_symbol_set - subscribed_stocks
    removed_symbols = subscribed_stocks - new_symbol_set
    subscribed_stocks = new_symbol_set
    
    # Clear out price data for any symbols that were just removed
    for symbol in removed_symbols:
        if symbol in stock_prices:
            del stock_prices[symbol]
            
    # Fetch initial prices for newly added symbols
    if new_symbols:
        await fetch_initial_prices(list(new_symbols))
            
    logger.info(f"Updated stock subscriptions. Added: {new_symbols}, Removed: {removed_symbols}")
    background_tasks.add_task(reset_streaming)
    
    return {"success": True, "message": f"Updated stock subscriptions. Added: {len(new_symbols)}, Removed: {len(removed_symbols)}. Restarting stream.", "symbols": list(subscribed_stocks)}

@app.get("/prices/options")
async def get_option_prices(symbols: Optional[str] = None, background_tasks: BackgroundTasks = None):
    """Retrieve latest prices for options, automatically subscribing to new symbols."""
    global subscribed_options
    
    if symbols:
        symbol_list = [s.strip() for s in symbols.split(',')]
        new_symbols = set(symbol_list) - subscribed_options
        
        if new_symbols:
            # Add new symbols to subscription set
            subscribed_options.update(new_symbols)
            logger.info(f"Auto-subscribing to new option symbols: {new_symbols}")
            
            # Restart streaming to include new symbols
            if background_tasks:
                background_tasks.add_task(reset_streaming)
            else:
                # If no background_tasks available, create task directly
                asyncio.create_task(reset_streaming())
        
        # Return current prices, including placeholders for newly added symbols
        result = {}
        for s in symbol_list:
            if s in option_prices:
                result[s] = option_prices[s]
            else:
                result[s] = {"message": "subscribed, waiting for data", "timestamp": datetime.now().isoformat()}
        return result
    
    return option_prices

@app.get("/prices/stocks")
async def get_stock_prices(symbols: Optional[str] = None, background_tasks: BackgroundTasks = None):
    """Retrieve latest prices for stocks, automatically subscribing to new symbols."""
    global subscribed_stocks
    
    if symbols:
        symbol_list = [s.strip() for s in symbols.split(',')]
        new_symbols = set(symbol_list) - subscribed_stocks
        
        if new_symbols:
            # Add new symbols to subscription set
            subscribed_stocks.update(new_symbols)
            logger.info(f"Auto-subscribing to new stock symbols: {new_symbols}")
            
            # Fetch initial prices for new symbols
            await fetch_initial_prices(list(new_symbols))
            
            # Restart streaming to include new symbols
            if background_tasks:
                background_tasks.add_task(reset_streaming)
            else:
                # If no background_tasks available, create task directly
                asyncio.create_task(reset_streaming())
        
        # Return current prices, including placeholders for newly added symbols
        result = {}
        for s in symbol_list:
            if s in stock_prices:
                result[s] = stock_prices[s]
            else:
                result[s] = {"message": "subscribed, waiting for data", "timestamp": datetime.now().isoformat()}
        return result
    
    return stock_prices

@app.post("/unsubscribe/options")
async def unsubscribe_options(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Remove option symbols from subscriptions."""
    global subscribed_options
    
    symbols_to_remove = set(request.symbols)
    removed_symbols = symbols_to_remove.intersection(subscribed_options)
    
    if not removed_symbols:
        return {"success": True, "message": "No symbols were subscribed.", "symbols": list(subscribed_options)}
    
    subscribed_options -= removed_symbols
    
    # Clear price data for removed symbols
    for symbol in removed_symbols:
        if symbol in option_prices:
            del option_prices[symbol]
    
    logger.info(f"Removed option subscriptions: {removed_symbols}")
    background_tasks.add_task(reset_streaming)
    
    return {"success": True, "message": f"Removed {len(removed_symbols)} option subscriptions.", "symbols": list(subscribed_options)}

@app.post("/unsubscribe/stocks")
async def unsubscribe_stocks(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Remove stock symbols from subscriptions."""
    global subscribed_stocks
    
    symbols_to_remove = set(request.symbols)
    removed_symbols = symbols_to_remove.intersection(subscribed_stocks)
    
    if not removed_symbols:
        return {"success": True, "message": "No symbols were subscribed.", "symbols": list(subscribed_stocks)}
    
    subscribed_stocks -= removed_symbols
    
    # Clear price data for removed symbols
    for symbol in removed_symbols:
        if symbol in stock_prices:
            del stock_prices[symbol]
    
    logger.info(f"Removed stock subscriptions: {removed_symbols}")
    background_tasks.add_task(reset_streaming)
    
    return {"success": True, "message": f"Removed {len(removed_symbols)} stock subscriptions.", "symbols": list(subscribed_stocks)}

@app.get("/options_chain")
async def api_get_options_chain(symbol: str, expiry: str, strategy_type: str):
    """Retrieve options chain for a given symbol, expiry, and strategy type."""
    return get_options_chain(symbol, expiry, strategy_type)

@app.get("/next_market_date")
def get_next_market_date():
    """
    Get the next market trading date from Alpaca calendar.
    """
    try:
        url = f"{get_base_url(is_paper_trade=False)}/v2/calendar"
        params = {
            "start": date.today().strftime("%Y-%m-%d"),
            "end": (date.today().replace(year=date.today().year + 1)).strftime("%Y-%m-%d")
        }
        api_key, api_secret = get_api_credentials(is_paper_trade=False)
        headers = {
            "APCA-API-KEY-ID": api_key,
            "APCA-API-SECRET-KEY": api_secret,
        }
        resp = requests.get(url, headers=headers, params=params)
        
        if resp.status_code == 200:
            data = resp.json()
            for day in data:
                if day.get("date"):
                    market_date = day["date"]
                    if market_date >= date.today().strftime("%Y-%m-%d"):
                        return {
                            "success": True,
                            "next_market_date": market_date,
                            "timestamp": datetime.now().isoformat()
                        }

        # Fallback to today's date if no future market date found
        return {
            "success": True,
            "next_market_date": date.today().strftime("%Y-%m-%d"),
            "timestamp": datetime.now().isoformat()
        }
        
    except Exception as e:
        logger.error(f"Error fetching next market date: {e}")
        return {
            "success": False,
            "error": str(e),
            "next_market_date": date.today().strftime("%Y-%m-%d"),
            "timestamp": datetime.now().isoformat()
        }

async def sync_open_orders_from_api():
    """
    Sync the in-memory open_orders cache with current data from Alpaca API.
    This is used for initial loading and periodic sync.
    """
    global open_orders
    
    try:
        api_key, api_secret = get_api_credentials(is_paper_trade=True)
        trading_client = TradingClient(api_key, api_secret, paper=True)
        
        # Get orders with various open statuses (not just "open")
        orders = trading_client.get_orders()  # Get all recent orders
        logger.info(f"Syncing: Fetched {len(orders)} total orders from Alpaca")
        
        # Filter for orders that are still open (not filled, canceled, etc.)
        open_statuses = ["new", "accepted", "pending_new", "partially_filled", "held"]
        open_orders_list = [order for order in orders if order.status.value.lower() in open_statuses]
        logger.info(f"Syncing: Found {len(open_orders_list)} orders with open statuses")
        
        # Clear existing cache and rebuild it
        open_orders.clear()
        
        for order in open_orders_list:
            order_info = {
                "id": str(order.id),
                "asset": order.symbol,
                "order_type": f"{order.order_type.value.title()} @ {order.limit_price or '-'}",
                "side": order.side.value if hasattr(order.side, 'value') else str(order.side),
                "qty": float(order.qty),
                "filled_qty": float(order.filled_qty) if order.filled_qty else 0.0,
                "avg_fill_price": float(order.filled_avg_price) if order.filled_avg_price else None,
                "status": order.status.value if hasattr(order.status, 'value') else str(order.status),
                "source": "access_key",
                "submitted_at": order.submitted_at.isoformat() if order.submitted_at else datetime.now().isoformat(),
                "filled_at": order.filled_at.isoformat() if order.filled_at else None,
                "timestamp": datetime.now().isoformat()
            }
            
            # Handle multi-leg orders
            if hasattr(order, 'legs') and order.legs:
                # This is a multi-leg order, create a parent entry
                order_info["asset"] = f"{len(order.legs)}-Leg Order"
                order_info["side"] = "-"
                
                # Store the parent order
                open_orders[str(order.id)] = order_info
                
                # Store individual legs
                for i, leg in enumerate(order.legs):
                    leg_id = f"{order.id}_leg_{i}"
                    leg_info = {
                        "id": leg_id,
                        "parent_id": str(order.id),
                        "asset": leg.symbol,
                        "order_type": f"{order.order_type.value.title()} @ -",
                        "side": leg.side.value if hasattr(leg.side, 'value') else str(leg.side),
                        "qty": float(leg.qty),
                        "filled_qty": 0.0,  # Individual leg fill info may not be available
                        "avg_fill_price": None,
                        "status": order_info["status"],
                        "source": "access_key",
                        "submitted_at": order_info["submitted_at"],
                        "filled_at": order_info["filled_at"],
                        "timestamp": datetime.now().isoformat()
                    }
                    open_orders[leg_id] = leg_info
            else:
                # Single-leg order
                open_orders[str(order.id)] = order_info
        
        logger.info(f"Synced {len(open_orders)} orders to in-memory cache")
        
        # Broadcast the updated orders to all WebSocket clients
        await broadcast_open_orders()
        
    except Exception as e:
        logger.error(f"Error syncing open orders from API: {e}")

@app.get("/open_orders")
def get_open_orders():
    """
    Retrieve current open orders directly from Alpaca API (backward compatibility).
    """
    return get_orders("open")

@app.get("/orders")
def get_orders(status_filter: str = "open", date_filter: str = "today"):
    """
    Retrieve orders from Alpaca API with status and date filtering.
    
    Args:
        status_filter: "open", "filled", "canceled", or "all"
        date_filter: "today", "yesterday", "week", "month", "all"
    """
    try:
        api_key, api_secret = get_api_credentials(is_paper_trade=True)
        trading_client = TradingClient(api_key, api_secret, paper=True)
        
        # Get all recent orders with expanded parameters
        from alpaca.trading.requests import GetOrdersRequest
        from alpaca.trading.enums import OrderStatus
        
        # Create request to get more comprehensive order history
        from datetime import datetime, timedelta
        
        # Convert date filter to actual date for Alpaca API
        def get_date_from_filter(date_filter: str):
            today = datetime.now().date()
            if date_filter == "today":
                return today
            elif date_filter == "yesterday":
                return today - timedelta(days=1)
            elif date_filter == "week":
                return today - timedelta(days=7)
            elif date_filter == "month":
                return today - timedelta(days=30)
            elif date_filter == "all":
                return None  # No date filter
            else:
                return today  # Default to today
        
        # Get the start date based on filter
        start_date = get_date_from_filter(date_filter)
        
        # Map our filter to Alpaca's status parameter
        if status_filter == "open":
            alpaca_status = "open"
        elif status_filter == "filled":
            alpaca_status = "closed"  # Filled orders are in "closed" status
        elif status_filter == "canceled":
            alpaca_status = "closed"  # Canceled orders are also in "closed" status
        elif status_filter == "all":
            alpaca_status = "all"
        else:
            alpaca_status = "open"  # Default to open
        
        try:
            # Use the correct status parameter for Alpaca API
            request_params = {
                "status": alpaca_status,  # Use Alpaca's status values
                "limit": 1000,           # Higher limit to get more orders
                "nested": True           # Include nested order details
            }
            
            # Add date filter if specified
            if start_date:
                request_params["after"] = start_date
            
            request = GetOrdersRequest(**request_params)
            
            orders = trading_client.get_orders(filter=request)
            logger.info(f"Fetched {len(orders)} orders with status='{alpaca_status}' for filter '{status_filter}'")
            
        except Exception as e:
            logger.warning(f"GetOrdersRequest failed: {e}, trying simple get_orders()")
            # Fallback to simple get_orders() call
            orders = trading_client.get_orders()
            logger.info(f"Fetched {len(orders)} orders with simple get_orders()")
        logger.info(f"API Request: Fetched {len(orders)} total orders from Alpaca for filter: {status_filter}")
        
        # Apply status filtering
        if status_filter == "open":
            filter_statuses = ["new", "accepted", "pending_new", "partially_filled", "held"]
            filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
        elif status_filter == "filled":
            filter_statuses = ["filled"]
            filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
        elif status_filter == "canceled":
            filter_statuses = ["canceled", "cancelled", "expired", "rejected"]
            filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
        elif status_filter == "all":
            filtered_orders = orders
        else:
            # Default to open orders for unknown filters
            filter_statuses = ["new", "accepted", "pending_new", "partially_filled", "held"]
            filtered_orders = [order for order in orders if order.status.value.lower() in filter_statuses]
        
        logger.info(f"API Request: Found {len(filtered_orders)} orders matching filter '{status_filter}'")
        
        order_data = []
        for order in filtered_orders:
            try:
                # Safe attribute access with null checking
                order_type_str = "Unknown"
                if order.order_type:
                    if hasattr(order.order_type, 'value'):
                        order_type_str = order.order_type.value.title()
                    else:
                        order_type_str = str(order.order_type).title()
                
                side_str = "Unknown"
                if order.side:
                    if hasattr(order.side, 'value'):
                        side_str = order.side.value
                    else:
                        side_str = str(order.side)
                
                status_str = "Unknown"
                if order.status:
                    if hasattr(order.status, 'value'):
                        status_str = order.status.value
                    else:
                        status_str = str(order.status)
                
                order_info = {
                    "id": str(order.id) if order.id else "unknown",
                    "asset": order.symbol if order.symbol else "Unknown",
                    "order_type": f"{order_type_str} @ {order.limit_price or '-'}",
                    "side": side_str,
                    "qty": float(order.qty) if order.qty else 0.0,
                    "filled_qty": float(order.filled_qty) if order.filled_qty else 0.0,
                    "avg_fill_price": float(order.filled_avg_price) if order.filled_avg_price else None,
                    "status": status_str,
                    "source": "access_key",
                    "submitted_at": order.submitted_at.isoformat() if order.submitted_at else datetime.now().isoformat(),
                    "filled_at": order.filled_at.isoformat() if order.filled_at else None,
                    "timestamp": datetime.now().isoformat()
                }
                
                logger.info(f"Processing order: ID={order.id}, Symbol={order.symbol}, Status={status_str}, Side={side_str}")
                
                # Handle multi-leg orders
                if hasattr(order, 'legs') and order.legs:
                    # This is a multi-leg order, create a parent entry
                    order_info["asset"] = f"{len(order.legs)}-Leg Order"
                    order_info["side"] = "-"
                    
                    # Add the parent order
                    order_data.append(order_info)
                    
                    # Add individual legs
                    for i, leg in enumerate(order.legs):
                        leg_info = {
                            "id": f"{order.id}_leg_{i}",
                            "parent_id": str(order.id),
                            "asset": leg.symbol,
                            "order_type": f"{order_type_str} @ -",
                            "side": leg.side.value if hasattr(leg.side, 'value') else str(leg.side),
                            "qty": float(leg.qty),
                            "filled_qty": 0.0,  # Individual leg fill info may not be available
                            "avg_fill_price": None,
                            "status": order_info["status"],
                            "source": "access_key",
                            "submitted_at": order_info["submitted_at"],
                            "filled_at": order_info["filled_at"],
                            "timestamp": datetime.now().isoformat()
                        }
                        order_data.append(leg_info)
                else:
                    # Single-leg order
                    order_data.append(order_info)
                    
            except Exception as e:
                logger.error(f"Error processing order {order.id}: {e}")
                continue
        
        return {
            "success": True,
            "orders": order_data,
            "total_orders": len(order_data),
            "timestamp": datetime.now().isoformat()
        }
        
    except Exception as e:
        logger.error(f"Error fetching open orders from API: {e}")
        return {
            "success": False,
            "error": str(e),
            "orders": [],
            "total_orders": 0,
            "timestamp": datetime.now().isoformat()
        }

@app.get("/positions")
def get_open_positions():
    """
    Retrieve all current open positions from Alpaca.
    Returns both stock and option positions with their details.
    """
    try:
        api_key, api_secret = get_api_credentials(is_paper_trade=True)
        trading_client = TradingClient(api_key, api_secret, paper=True)
        
        # Get all open positions
        positions = trading_client.get_all_positions()
        
        position_data = []
        for position in positions:
            position_info = {
                "symbol": position.symbol,
                "qty": float(position.qty),
                "side": "long" if float(position.qty) > 0 else "short",
                "market_value": float(position.market_value) if position.market_value else 0,
                "cost_basis": float(position.cost_basis) if position.cost_basis else 0,
                "unrealized_pl": float(position.unrealized_pl) if position.unrealized_pl else 0,
                "unrealized_plpc": float(position.unrealized_plpc) if position.unrealized_plpc else 0,
                "current_price": float(position.current_price) if position.current_price else 0,
                "avg_entry_price": float(position.avg_entry_price) if position.avg_entry_price else 0,
                "asset_class": position.asset_class.value if position.asset_class else "unknown"
            }
            
            # For options, try to parse additional information from the symbol
            if position.asset_class and position.asset_class.value == "us_option":
                # Parse option symbol to extract strike, expiry, type
                try:
                    # Option symbols are typically like: SPY250103C00620000
                    symbol_str = position.symbol
                    if len(symbol_str) >= 15:
                        underlying = symbol_str[:3]  # First 3 chars (e.g., SPY)
                        date_part = symbol_str[3:9]  # Next 6 chars (YYMMDD)
                        option_type = symbol_str[9]  # C or P
                        strike_part = symbol_str[10:]  # Strike price (8 digits)
                        
                        # Parse expiry date
                        year = 2000 + int(date_part[:2])
                        month = int(date_part[2:4])
                        day = int(date_part[4:6])
                        expiry_date = f"{year}-{month:02d}-{day:02d}"
                        
                        # Parse strike price (divide by 1000 to get actual price)
                        strike_price = float(strike_part) / 1000
                        
                        position_info.update({
                            "underlying_symbol": underlying,
                            "option_type": "call" if option_type == "C" else "put",
                            "strike_price": strike_price,
                            "expiry_date": expiry_date
                        })
                except Exception as e:
                    logger.warning(f"Could not parse option symbol {position.symbol}: {e}")
            
            position_data.append(position_info)
        
        return {
            "success": True,
            "positions": position_data,
            "total_positions": len(position_data),
            "timestamp": datetime.now().isoformat()
        }
        
    except Exception as e:
        logger.error(f"Error fetching positions: {e}")
        return {
            "success": False,
            "error": str(e),
            "positions": [],
            "total_positions": 0,
            "timestamp": datetime.now().isoformat()
        }


# --- Main Execution Block ---

if __name__ == "__main__":
    # To run this file:
    # 1. Save it as, for example, 'streaming_service.py'
    # 2. In your terminal, run: python streaming_service.py
    #
    # Ensure your .env file with API keys is in the same directory.
    uvicorn.run("streaming_service:app", host="0.0.0.0", port=8008, reload=True)
