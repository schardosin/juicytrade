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
    SymbolRequest, OrderRequest, MultiLegOrderRequest
)
from .provider_manager import provider_manager
from .provider_config import provider_config_manager
from .streaming_manager import streaming_manager

# Configure logging
logging.basicConfig(
    level=logging.INFO,
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
    
    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket connected. Total connections: {len(self.active_connections)}")
    
    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
        logger.info(f"WebSocket disconnected. Total connections: {len(self.active_connections)}")
    
    async def broadcast(self, message: dict):
        disconnected = []
        for connection in self.active_connections:
            try:
                await connection.send_json(message)
            except Exception as e:
                logger.warning(f"Failed to send message to WebSocket: {e}")
                disconnected.append(connection)
        
        # Clean up disconnected clients
        for conn in disconnected:
            self.disconnect(conn)

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

# === Account & Portfolio Endpoints ===

@app.get("/positions", response_model=ApiResponse)
async def get_positions():
    """Get all current positions."""
    try:
        positions = await provider_manager.get_positions()
        positions_data = [position.dict() for position in positions]
        
        return ApiResponse(
            success=True,
            data={
                "positions": positions_data,
                "total_positions": len(positions)
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
    asyncio.create_task(handle_streaming_data())
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
                logger.info(f"Broadcasting price update for {market_data.symbol}")
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
