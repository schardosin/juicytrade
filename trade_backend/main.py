import asyncio
import uvicorn
import logging
from contextlib import asynccontextmanager
from typing import Dict, List, Optional, Any

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .models import (
    StockQuote, OptionContract, Position, Order, ApiResponse,
    SymbolRequest, OrderRequest, MultiLegOrderRequest, SymbolSearchResult, Account
)
from .provider_manager import provider_manager
from .provider_config import provider_config_manager
from .streaming_manager import streaming_manager

# Configure logging
logging.basicConfig(
    level=settings.log_level.upper(),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("trading_backend")

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Application lifespan manager.
    """
    # Startup
    logger.info("Starting trading backend...")
    try:
        # Connect to streaming if needed
        logger.info("Connecting to streaming...")
        await streaming_manager.connect()
        logger.info("Provider streaming connected")
        
    except Exception as e:
        logger.error(f"Failed to initialize providers: {e}")
        raise
    
    yield
    
    # Shutdown
    logger.info("Shutting down trading backend...")
    await streaming_manager.disconnect()
    logger.info("Provider streaming disconnected")

# Create FastAPI app
app = FastAPI(
    title="Trading Backend API",
    description="Multi-provider trading backend with standardized API",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# WebSocket connection manager
class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []
        self.streaming_task = None
        self.is_streaming = False
    
    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket connected. Total connections: {len(self.active_connections)}")
        
        # Start streaming task if this is the first connection
        if len(self.active_connections) == 1 and not self.is_streaming:
            logger.info("Starting streaming task for first WebSocket connection")
            self.streaming_task = asyncio.create_task(self._handle_streaming_data())
            self.is_streaming = True
    
    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
        logger.info(f"WebSocket disconnected. Total connections: {len(self.active_connections)}")
        
        # Stop streaming task if no connections remain
        if len(self.active_connections) == 0 and self.is_streaming:
            logger.info("Stopping streaming task - no active connections")
            if self.streaming_task and not self.streaming_task.done():
                self.streaming_task.cancel()
            self.is_streaming = False
            self.streaming_task = None
    
    async def broadcast(self, message: dict):
        if not self.active_connections:
            return
            
        disconnected = []
        for connection in self.active_connections:
            try:
                # Check if connection is still alive
                if connection.client_state.name != "CONNECTED":
                    disconnected.append(connection)
                    continue
                    
                await connection.send_json(message)
            except Exception as e:
                logger.warning(f"Failed to send message to WebSocket: {e}")
                disconnected.append(connection)
        
        # Clean up disconnected clients
        for conn in disconnected:
            self.disconnect(conn)
    
    async def _handle_streaming_data(self):
        """Internal streaming task - only one instance should run."""
        logger.info("🚀 Starting streaming data handler...")
        try:
            while self.is_streaming and len(self.active_connections) > 0:
                try:
                    market_data = await streaming_manager.get_data()
                    if market_data and self.active_connections:
                        # Only broadcast if we have active connections
                        await self.broadcast({
                            "type": "price_update",
                            "symbol": market_data.symbol,
                            "data": market_data.data,
                            "timestamp": market_data.timestamp
                        })
                except Exception as e:
                    logger.error(f"Error in streaming data handler: {e}")
                    await asyncio.sleep(1)
        except asyncio.CancelledError:
            logger.info("🛑 Streaming data handler cancelled")
        except Exception as e:
            logger.error(f"Fatal error in streaming data handler: {e}")
        finally:
            logger.info("🏁 Streaming data handler stopped")
            self.is_streaming = False

manager = ConnectionManager()

# === API Endpoints ===

@app.get("/", response_model=ApiResponse)
async def root():
    """Root endpoint with service status."""
    health = await provider_manager.health_check()
    
    return ApiResponse(
        success=True,
        data={
            "service": "Trading Backend API",
            "version": "1.0.0",
            "provider_health": health
        },
        message="Service is running"
    )

@app.get("/health", response_model=ApiResponse)
async def health_check():
    """Health check endpoint."""
    health = await provider_manager.health_check()
    return ApiResponse(
        success=True,
        data=health,
        message="Health check completed"
    )

# === Provider Configuration Endpoints ===

@app.get("/providers/config", response_model=ApiResponse)
async def get_provider_config():
    """Get current provider routing configuration."""
    config = provider_config_manager.get_config()
    return ApiResponse(success=True, data=config)

@app.put("/providers/config", response_model=ApiResponse)
async def update_provider_config(new_config: Dict[str, Any]):
    """Update provider routing configuration."""
    provider_config_manager.update_config(new_config)
    return ApiResponse(success=True, message="Provider config updated.")

@app.post("/providers/config/reset", response_model=ApiResponse)
async def reset_provider_config():
    """Reset provider routing to default."""
    provider_config_manager.reset_config()
    return ApiResponse(success=True, message="Provider config reset to default.")

@app.get("/providers/available", response_model=ApiResponse)
async def get_available_providers():
    """Get available providers and their capabilities."""
    providers = provider_config_manager.get_available_providers()
    return ApiResponse(success=True, data=providers)

# === Market Data Endpoints ===

@app.get("/prices/stocks", response_model=ApiResponse)
async def get_stock_prices(symbols: Optional[str] = None):
    """Get stock prices for one or more symbols."""
    try:
        if symbols:
            symbol_list = [s.strip() for s in symbols.split(',')]
            if len(symbol_list) == 1:
                quote = await provider_manager.get_stock_quote(symbol_list[0])
                data = {symbol_list[0]: quote.dict() if quote else None}
            else:
                # This part needs to be adapted for the manager if we want multi-symbol quotes
                # For now, we'll just loop, but a batch method in the manager would be better.
                data = {}
                for symbol in symbol_list:
                    quote = await provider_manager.get_stock_quote(symbol)
                    data[symbol] = quote.dict() if quote else None
        else:
            data = {}
        
        return ApiResponse(
            success=True,
            data=data,
            message=f"Retrieved prices for {len(data)} symbols"
        )
    except Exception as e:
        logger.error(f"Error getting stock prices: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/expiration_dates", response_model=ApiResponse)
async def get_expiration_dates(symbol: str):
    """Get available expiration dates for options on a symbol."""
    try:
        dates = await provider_manager.get_expiration_dates(symbol)
        return ApiResponse(
            success=True,
            data={
                "expiration_dates": dates,
                "total_dates": len(dates)
            },
            message=f"Found {len(dates)} expiration dates for {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting expiration dates: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/options_chain", response_model=ApiResponse)
async def get_options_chain(symbol: str, expiry: str, strategy_type: Optional[str] = None):
    """Get options chain for a symbol and expiration date."""
    try:
        # Map strategy_type to option_type if needed
        option_type = None
        if strategy_type:
            if "CALL" in strategy_type.upper():
                option_type = "call"
            elif "PUT" in strategy_type.upper():
                option_type = "put"
        
        contracts = await provider_manager.get_options_chain(symbol, expiry, option_type)
        
        # Convert to dict format for API response
        contracts_data = [contract.dict() for contract in contracts]
        
        return ApiResponse(
            success=True,
            data=contracts_data,
            message=f"Retrieved {len(contracts)} option contracts"
        )
    except Exception as e:
        logger.error(f"Error getting options chain: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/options_chain_basic", response_model=ApiResponse)
async def get_options_chain_basic(
    symbol: str, 
    expiry: str, 
    underlying_price: Optional[float] = None,
    strike_count: int = 20
):
    """Fast options chain loading - basic data only, ATM-focused by strike count."""
    try:
        contracts = await provider_manager.get_options_chain_basic(
            symbol, expiry, underlying_price, strike_count
        )
        
        # Convert to dict format for API response
        contracts_data = [contract.dict() for contract in contracts]
        
        return ApiResponse(
            success=True,
            data=contracts_data,
            message=f"Retrieved {len(contracts)} basic option contracts ({strike_count} strikes around ATM)"
        )
    except Exception as e:
        logger.error(f"Error getting basic options chain: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/options_greeks", response_model=ApiResponse)
async def get_options_greeks(symbols: str):
    """Get Greeks for specific option symbols (comma-separated)."""
    try:
        symbol_list = [s.strip() for s in symbols.split(',') if s.strip()]
        
        if not symbol_list:
            return ApiResponse(
                success=True,
                data={},
                message="No symbols provided"
            )
        
        greeks_data = await provider_manager.get_options_greeks_batch(symbol_list)
        
        return ApiResponse(
            success=True,
            data=greeks_data,
            message=f"Retrieved Greeks for {len(symbol_list)} option symbols"
        )
    except Exception as e:
        logger.error(f"Error getting options Greeks: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/options_chain_smart", response_model=ApiResponse)
async def get_options_chain_smart(
    symbol: str,
    expiry: str, 
    underlying_price: Optional[float] = None,
    atm_range: int = 20,
    include_greeks: bool = False,
    strikes_only: bool = False
):
    """Smart options chain with configurable loading."""
    try:
        contracts = await provider_manager.get_options_chain_smart(
            symbol, expiry, underlying_price, atm_range, include_greeks, strikes_only
        )
        
        # Convert to dict format for API response
        contracts_data = [contract.dict() for contract in contracts]
        
        return ApiResponse(
            success=True,
            data=contracts_data,
            message=f"Retrieved {len(contracts)} smart option contracts"
        )
    except Exception as e:
        logger.error(f"Error getting smart options chain: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/full_options_chain", response_model=ApiResponse)
async def get_full_options_chain(symbol: str, expiry: str):
    """Get complete options chain (calls and puts) for a symbol and expiration."""
    try:
        contracts = await provider_manager.get_options_chain(symbol, expiry)
        contracts_data = [contract.dict() for contract in contracts]
        
        return ApiResponse(
            success=True,
            data={
                "options_chain": contracts_data,
                "total_contracts": len(contracts)
            },
            message=f"Retrieved {len(contracts)} option contracts"
        )
    except Exception as e:
        logger.error(f"Error getting full options chain: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/next_market_date", response_model=ApiResponse)
async def get_next_market_date():
    """Get the next market trading date."""
    try:
        # This should probably be routed as well, but for now we'll leave it to the default
        next_date = await provider_manager.get_next_market_date()
        return ApiResponse(
            success=True,
            data={"next_market_date": next_date},
            message="Retrieved next market date"
        )
    except Exception as e:
        logger.error(f"Error getting next market date: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/subscriptions/status", response_model=ApiResponse)
async def get_subscription_status():
    """Get current WebSocket subscription status."""
    try:
        status = streaming_manager.get_subscription_status()
        return ApiResponse(
            success=True,
            data=status,
            message="Retrieved subscription status"
        )
    except Exception as e:
        logger.error(f"Error getting subscription status: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/symbols/lookup", response_model=ApiResponse)
async def lookup_symbols(q: str):
    """Search for symbols matching the query."""
    try:
        results = await provider_manager.lookup_symbols(q)
        return ApiResponse(
            success=True,
            data={"symbols": [result.dict() for result in results]},
            message=f"Found {len(results)} symbols matching '{q}'"
        )
    except Exception as e:
        logger.error(f"Error looking up symbols: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/chart/historical/{symbol}", response_model=ApiResponse)
async def get_historical_chart_data(
    symbol: str,
    timeframe: str = "D",
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    limit: int = 500
):
    """Get historical OHLCV data for charts."""
    try:
        bars = await provider_manager.get_historical_bars(
            symbol, timeframe, start_date, end_date, limit
        )
        
        return ApiResponse(
            success=True,
            data={
                "symbol": symbol,
                "timeframe": timeframe,
                "bars": bars,
                "count": len(bars)
            },
            message=f"Retrieved {len(bars)} bars for {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting historical chart data: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/chart/intraday/{symbol}", response_model=ApiResponse)
async def get_intraday_chart_data(symbol: str, interval: str = "5m"):
    """Get recent intraday data for real-time chart initialization."""
    try:
        from datetime import datetime, timedelta
        
        # Get last 2 days of intraday data
        end_date = datetime.now()
        start_date = end_date - timedelta(days=2)
        
        bars = await provider_manager.get_historical_bars(
            symbol, interval, 
            start_date.strftime('%Y-%m-%d'),
            end_date.strftime('%Y-%m-%d'),
            limit=200
        )
        
        return ApiResponse(
            success=True,
            data={
                "symbol": symbol,
                "interval": interval,
                "bars": bars,
                "count": len(bars)
            },
            message=f"Retrieved {len(bars)} intraday bars for {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting intraday chart data: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Account & Portfolio Endpoints ===

@app.get("/positions", response_model=ApiResponse)
async def get_positions():
    """Get enhanced positions with order chain grouping and strategy detection."""
    try:
        # Try to get enhanced positions first
        try:
            position_groups = await provider_manager.get_positions_enhanced()
            if position_groups:
                return ApiResponse(
                    success=True,
                    data={
                        "position_groups": [group.dict() for group in position_groups],
                        "total_groups": len(position_groups),
                        "enhanced": True
                    },
                    message=f"Retrieved {len(position_groups)} position groups with strategy detection"
                )
        except AttributeError:
            # Provider doesn't support enhanced positions, fall back to regular positions
            logger.info("Provider doesn't support enhanced positions, using regular positions")
        
        # Fallback to regular positions
        positions = await provider_manager.get_positions()
        positions_data = [position.dict() for position in positions]
        
        return ApiResponse(
            success=True,
            data={
                "positions": positions_data,
                "total_positions": len(positions),
                "enhanced": False
            },
            message=f"Retrieved {len(positions)} positions"
        )
    except Exception as e:
        logger.error(f"Error getting positions: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/orders", response_model=ApiResponse)
async def get_orders(status: str = "open", date_filter: str = "today"):
    """Get orders with optional filtering."""
    try:
        orders = await provider_manager.get_orders(status)
        orders_data = [order.dict() for order in orders]
        
        return ApiResponse(
            success=True,
            data={
                "orders": orders_data,
                "total_orders": len(orders)
            },
            message=f"Retrieved {len(orders)} orders with status '{status}'"
        )
    except Exception as e:
        logger.error(f"Error getting orders: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/open_orders", response_model=ApiResponse)
async def get_open_orders():
    """Get current open orders (backward compatibility)."""
    return await get_orders(status="open")

@app.get("/account", response_model=ApiResponse)
async def get_account():
    """Get account information including balance and buying power."""
    try:
        account = await provider_manager.get_account()
        if account:
            return ApiResponse(
                success=True,
                data=account.dict(),
                message="Retrieved account information"
            )
        else:
            raise HTTPException(status_code=404, detail="Account information not available")
    except Exception as e:
        logger.error(f"Error getting account: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/orders", response_model=ApiResponse)
async def place_order(order_request: OrderRequest):
    """Place a new trading order."""
    try:
        order = await provider_manager.place_order(order_request.dict())
        if order:
            return ApiResponse(
                success=True,
                data=order.dict(),
                message="Order placed successfully."
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to place order.")
    except Exception as e:
        logger.error(f"Error placing order: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/orders/multi-leg", response_model=ApiResponse)
async def place_multi_leg_order(order_request: MultiLegOrderRequest):
    """Place a new multi-leg trading order."""
    try:
        order = await provider_manager.place_multi_leg_order(order_request.dict())
        if order:
            return ApiResponse(
                success=True,
                data=order.dict(),
                message="Multi-leg order placed successfully."
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to place multi-leg order.")
    except Exception as e:
        logger.error(f"Error placing multi-leg order: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/orders/{order_id}", response_model=ApiResponse)
async def cancel_order(order_id: str):
    """Cancel an existing order."""
    try:
        logger.info(f"Attempting to cancel order: {order_id}")
        result = await provider_manager.cancel_order(order_id)
        
        if result:
            return ApiResponse(
                success=True,
                data={"order_id": order_id, "status": "cancelled"},
                message=f"Order {order_id} cancelled successfully"
            )
        else:
            raise HTTPException(status_code=400, detail=f"Failed to cancel order {order_id}")
    except Exception as e:
        logger.error(f"Error cancelling order {order_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Streaming Endpoints ===

@app.post("/subscribe/stocks", response_model=ApiResponse)
async def subscribe_stocks(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Subscribe to stock price streaming."""
    try:
        await streaming_manager.subscribe(request.symbols, ["quotes"])
        # Start background task to handle streaming data
        background_tasks.add_task(handle_streaming_data)
        
        return ApiResponse(
            success=True,
            data={"subscribed_symbols": request.symbols},
            message=f"Subscribed to {len(request.symbols)} stock symbols"
        )
    except Exception as e:
        logger.error(f"Error subscribing to stocks: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/subscribe/options", response_model=ApiResponse)
async def subscribe_options(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Subscribe to option price streaming."""
    try:
        await streaming_manager.subscribe(request.symbols, ["quotes"])
        background_tasks.add_task(handle_streaming_data)
        
        return ApiResponse(
            success=True,
            data={"subscribed_symbols": request.symbols},
            message=f"Subscribed to {len(request.symbols)} option symbols"
        )
    except Exception as e:
        logger.error(f"Error subscribing to options: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === WebSocket Endpoint ===

@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """WebSocket endpoint for real-time data streaming."""
    await manager.connect(websocket)
    try:
        while True:
            # Wait for client messages
            data = await websocket.receive_json()
            
            if data.get("type") == "subscribe":
                symbols = data.get("symbols", [])
                if symbols:
                    await streaming_manager.subscribe(symbols)
                    await websocket.send_json({
                        "type": "subscription_confirmed",
                        "symbols": symbols,
                        "message": f"Subscribed to {len(symbols)} symbols"
                    })
            
            elif data.get("type") == "subscribe_replace_all":
                underlying_symbol = data.get("underlying_symbol")
                option_symbols = data.get("option_symbols", [])
                logger.info(f"🔄 WebSocket: Received unified subscription replacement - underlying: {underlying_symbol}, options: {len(option_symbols)} symbols")
                
                if underlying_symbol:
                    await streaming_manager.replace_all_subscriptions(underlying_symbol, option_symbols)
                    await websocket.send_json({
                        "type": "subscription_confirmed",
                        "subscription_type": "all_replace",
                        "underlying_symbol": underlying_symbol,
                        "option_symbols": option_symbols,
                        "message": f"All subscriptions replaced - underlying: {underlying_symbol}, options: {len(option_symbols)} symbols"
                    })
                    logger.info(f"✅ WebSocket: All subscriptions replaced - underlying: {underlying_symbol}, options: {len(option_symbols)} symbols")
            
            elif data.get("type") == "subscribe_replace_stock":
                # Legacy support - deprecated
                symbol = data.get("symbol")
                logger.warning(f"🔄 WebSocket: Received deprecated stock subscription replacement request for {symbol}")
                if symbol:
                    await streaming_manager.replace_stock_subscription(symbol)
                    await websocket.send_json({
                        "type": "subscription_confirmed",
                        "subscription_type": "stock_replace",
                        "symbol": symbol,
                        "message": f"Stock subscription replaced with {symbol} (deprecated - use subscribe_replace_all)"
                    })
                    logger.info(f"✅ WebSocket: Stock subscription replaced with {symbol} (deprecated)")
            
            elif data.get("type") == "subscribe_replace_options":
                # Legacy support - deprecated
                symbols = data.get("symbols", [])
                logger.warning(f"🔄 WebSocket: Received deprecated options subscription replacement request for {len(symbols)} symbols")
                await streaming_manager.replace_options_subscriptions(symbols)
                await websocket.send_json({
                    "type": "subscription_confirmed",
                    "subscription_type": "options_replace",
                    "symbols": symbols,
                    "message": f"Options subscriptions replaced with {len(symbols)} symbols (deprecated - use subscribe_replace_all)"
                })
                logger.info(f"✅ WebSocket: Options subscriptions replaced with {len(symbols)} symbols (deprecated)")
            
            elif data.get("type") == "subscribe_persistent":
                data_types = data.get("data_types", ["orders", "positions"])
                await streaming_manager.ensure_persistent_subscriptions(data_types)
                await websocket.send_json({
                    "type": "subscription_confirmed",
                    "subscription_type": "persistent",
                    "data_types": data_types,
                    "message": f"Persistent subscriptions ensured for {data_types}"
                })
            
            elif data.get("type") == "get_subscription_status":
                status = streaming_manager.get_subscription_status()
                await websocket.send_json({
                    "type": "subscription_status",
                    "data": status
                })
            
            elif data.get("type") == "get_positions":
                positions = await provider_manager.get_positions()
                await websocket.send_json({
                    "type": "positions_update",
                        "data": {
                            "success": True,
                            "positions": [pos.dict() for pos in positions],
                            "total_positions": len(positions)
                        }
                    })
            
            elif data.get("type") == "get_orders":
                status_filter = data.get("filter", "open")
                orders = await provider_manager.get_orders(status_filter)
                await websocket.send_json({
                    "type": "orders_update",
                        "data": {
                            "success": True,
                            "orders": [order.dict() for order in orders],
                            "total_orders": len(orders)
                        },
                        "filter": status_filter
                    })
                    
    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception as e:
        logger.error(f"WebSocket error: {e}")
        manager.disconnect(websocket)

# === Background Tasks ===

async def handle_streaming_data():
    """Background task to handle streaming data and broadcast to WebSocket clients."""
    logger.info("Starting to handle streaming data...")
    while True:
        try:
            market_data = await streaming_manager.get_data()
            if market_data:
                #logger.info(f"Broadcasting price update for {market_data.symbol}")
                # Broadcast to all connected WebSocket clients
                await manager.broadcast({
                    "type": "price_update",
                    "symbol": market_data.symbol,
                    "data": market_data.data,
                    "timestamp": market_data.timestamp
                })
        except Exception as e:
            logger.error(f"Error handling streaming data: {e}")
            await asyncio.sleep(1)

# === Main Execution ===

if __name__ == "__main__":
    uvicorn.run(
        "trading_backend.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.reload
    )
