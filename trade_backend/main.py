import asyncio
import uvicorn
import logging
from contextlib import asynccontextmanager
from typing import Dict, List, Optional

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .models import (
    StockQuote, OptionContract, Position, Order, ApiResponse,
    SymbolRequest, OrderRequest, MultiLegOrderRequest
)
from .providers.base_provider import BaseProvider
from .providers.alpaca_provider import AlpacaProvider

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("trading_backend")

# Global provider instance
provider: Optional[BaseProvider] = None

def create_provider() -> BaseProvider:
    """
    Factory function to create the appropriate provider based on configuration.
    """
    if settings.provider.lower() == "alpaca":
        return AlpacaProvider(
            api_key_live=settings.alpaca_api_key_live,
            api_secret_live=settings.alpaca_api_secret_live,
            api_key_paper=settings.alpaca_api_key_paper,
            api_secret_paper=settings.alpaca_api_secret_paper,
            base_url_live=settings.alpaca_base_url_live,
            base_url_paper=settings.alpaca_base_url_paper,
            use_paper=settings.use_paper_trading
        )
    else:
        raise ValueError(f"Unsupported provider: {settings.provider}")

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Application lifespan manager.
    """
    global provider
    
    # Startup
    logger.info("Starting trading backend...")
    try:
        provider = create_provider()
        logger.info(f"Initialized {provider.name} provider")
        
        # Connect to streaming if needed
        await provider.connect_streaming()
        logger.info("Provider streaming connected")
        
    except Exception as e:
        logger.error(f"Failed to initialize provider: {e}")
        raise
    
    yield
    
    # Shutdown
    logger.info("Shutting down trading backend...")
    if provider:
        await provider.disconnect_streaming()
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
    health = await provider.health_check() if provider else {"status": "no provider"}
    
    return ApiResponse(
        success=True,
        data={
            "service": "Trading Backend API",
            "version": "1.0.0",
            "provider": settings.provider,
            "provider_health": health
        },
        message="Service is running"
    )

@app.get("/health", response_model=ApiResponse)
async def health_check():
    """Health check endpoint."""
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    health = await provider.health_check()
    return ApiResponse(
        success=True,
        data=health,
        message="Health check completed"
    )

# === Market Data Endpoints ===

@app.get("/prices/stocks", response_model=ApiResponse)
async def get_stock_prices(symbols: Optional[str] = None):
    """Get stock prices for one or more symbols."""
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        if symbols:
            symbol_list = [s.strip() for s in symbols.split(',')]
            if len(symbol_list) == 1:
                quote = await provider.get_stock_quote(symbol_list[0])
                data = {symbol_list[0]: quote.dict() if quote else None}
            else:
                quotes = await provider.get_stock_quotes(symbol_list)
                data = {symbol: quote.dict() for symbol, quote in quotes.items()}
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        dates = await provider.get_expiration_dates(symbol)
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        # Map strategy_type to option_type if needed
        option_type = None
        if strategy_type:
            if "CALL" in strategy_type.upper():
                option_type = "call"
            elif "PUT" in strategy_type.upper():
                option_type = "put"
        
        contracts = await provider.get_options_chain(symbol, expiry, option_type)
        
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        contracts = await provider.get_options_chain(symbol, expiry)
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        next_date = await provider.get_next_market_date()
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        positions = await provider.get_positions()
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
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        orders = await provider.get_orders(status)
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

# === Streaming Endpoints ===

@app.post("/subscribe/stocks", response_model=ApiResponse)
async def subscribe_stocks(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Subscribe to stock price streaming."""
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        success = await provider.subscribe_to_symbols(request.symbols, ["quotes"])
        
        if success:
            # Start background task to handle streaming data
            background_tasks.add_task(handle_streaming_data)
            
            return ApiResponse(
                success=True,
                data={"subscribed_symbols": request.symbols},
                message=f"Subscribed to {len(request.symbols)} stock symbols"
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to subscribe to symbols")
            
    except Exception as e:
        logger.error(f"Error subscribing to stocks: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/subscribe/options", response_model=ApiResponse)
async def subscribe_options(request: SymbolRequest, background_tasks: BackgroundTasks):
    """Subscribe to option price streaming."""
    if not provider:
        raise HTTPException(status_code=503, detail="Provider not initialized")
    
    try:
        success = await provider.subscribe_to_symbols(request.symbols, ["quotes"])
        
        if success:
            background_tasks.add_task(handle_streaming_data)
            
            return ApiResponse(
                success=True,
                data={"subscribed_symbols": request.symbols},
                message=f"Subscribed to {len(request.symbols)} option symbols"
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to subscribe to symbols")
            
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
                if symbols and provider:
                    await provider.subscribe_to_symbols(symbols)
                    await websocket.send_json({
                        "type": "subscription_confirmed",
                        "symbols": symbols,
                        "message": f"Subscribed to {len(symbols)} symbols"
                    })
            
            elif data.get("type") == "get_positions":
                if provider:
                    positions = await provider.get_positions()
                    await websocket.send_json({
                        "type": "positions_update",
                        "data": {
                            "success": True,
                            "positions": [pos.dict() for pos in positions],
                            "total_positions": len(positions)
                        }
                    })
            
            elif data.get("type") == "get_orders":
                if provider:
                    status_filter = data.get("filter", "open")
                    orders = await provider.get_orders(status_filter)
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
    if not provider:
        return
    
    while provider.is_streaming_connected():
        try:
            market_data = await provider.get_streaming_data()
            if market_data:
                # Broadcast to all connected WebSocket clients
                await manager.broadcast({
                    "type": "price_update",
                    "symbol": market_data.symbol,
                    "data": market_data.data,
                    "timestamp": market_data.timestamp
                })
            else:
                # Small delay if no data available
                await asyncio.sleep(0.1)
                
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
