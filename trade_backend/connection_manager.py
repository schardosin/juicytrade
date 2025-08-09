import asyncio
import logging
import json
import time
from typing import List, Dict, Optional, Set
from fastapi import WebSocket, WebSocketDisconnect

from .config import settings

logger = logging.getLogger(__name__)

class ConnectionManager:
    """
    Manages WebSocket connections, including per-client subscriptions
    and keep-alive tracking.
    """
    
    def __init__(self, streaming_manager, shutdown_manager):
        self.active_connections: List[WebSocket] = []
        self.client_subscriptions: Dict[WebSocket, Set[str]] = {}
        self.client_keepalives: Dict[WebSocket, float] = {}
        self.symbol_keepalives: Dict[str, float] = {}  # Track per-symbol keep-alives
        self.is_shutting_down = False
        
        # Dependencies
        self.streaming_manager = streaming_manager
        self.shutdown_manager = shutdown_manager
        
        # Register to receive data updates from the streaming manager's cache
        self.streaming_manager._latest_cache.add_update_callback(self.broadcast_market_data)
        
        # Background tasks will be started when the first connection is made
        self._client_keepalive_task = None
        self._symbol_keepalive_task = None

    async def connect(self, websocket: WebSocket):
        """Accepts a new WebSocket connection and initializes its state."""
        if self.is_shutting_down:
            await websocket.close(code=1001, reason="Server shutting down")
            return
            
        await websocket.accept()
        self.active_connections.append(websocket)
        self.client_subscriptions[websocket] = set()
        self.client_keepalives[websocket] = time.time()
        self.shutdown_manager.register_websocket(websocket)
        
        # Start background tasks if this is the first connection
        if len(self.active_connections) == 1:
            if self._client_keepalive_task is None or self._client_keepalive_task.done():
                self._client_keepalive_task = asyncio.create_task(self._check_client_keepalives())
            if self._symbol_keepalive_task is None or self._symbol_keepalive_task.done():
                self._symbol_keepalive_task = asyncio.create_task(self._check_symbol_keepalives())
        
        logger.info(f"✅ WebSocket connected. Total connections: {len(self.active_connections)}")
    
    async def disconnect(self, websocket: WebSocket):
        """Handles disconnection of a WebSocket client."""
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
        
        self.client_subscriptions.pop(websocket, None)
        self.client_keepalives.pop(websocket, None)
        self.shutdown_manager.unregister_websocket(websocket)
        
        # Notify the streaming manager to update global subscriptions
        await self.streaming_manager.update_global_subscriptions(self.client_subscriptions)
        
        logger.info(f"🔌 WebSocket disconnected. Total connections: {len(self.active_connections)}")
    
    async def broadcast_market_data(self, market_data):
        """Broadcasts market data only to clients subscribed to the symbol."""
        if self.is_shutting_down:
            return

        message_type = "price_update"
        if hasattr(market_data, 'data_type') and market_data.data_type == "greeks":
            message_type = "greeks_update"

        message = {
            "type": message_type,
            "symbol": market_data.symbol,
            "data": market_data.data,
            "timestamp": market_data.timestamp
        }

        # Create a list of tasks to send messages concurrently
        send_tasks = []
        for connection, subscriptions in self.client_subscriptions.items():
            if market_data.symbol in subscriptions:
                send_tasks.append(self._send_json_safe(connection, message))
        
        if send_tasks:
            await asyncio.gather(*send_tasks)

    async def _send_json_safe(self, websocket: WebSocket, message: dict):
        """Safely sends a JSON message to a single client."""
        try:
            await websocket.send_json(message)
        except (WebSocketDisconnect, RuntimeError):
            await self._cleanup_connection(websocket)
        except Exception as e:
            logger.error(f"Error sending message to client: {e}")
            await self._cleanup_connection(websocket)

    async def _cleanup_connection(self, websocket: WebSocket):
        """Gracefully cleans up a single connection."""
        logger.info(f"🧹 Cleaning up connection: {id(websocket)}")
        try:
            await websocket.close()
        except Exception:
            pass  # Ignore errors on close, connection is likely already dead
        finally:
            await self.disconnect(websocket)

    async def handle_websocket_message(self, websocket: WebSocket, data: dict):
        """Handles incoming messages from a WebSocket client."""
        if self.is_shutting_down:
            return

        # Any message from the client serves as a keep-alive
        self.client_keepalives[websocket] = time.time()

        message_type = data.get("type")
        if message_type == "subscribe_smart_replace_all":
            await self._handle_smart_subscription(websocket, data)
        elif message_type == "keepalive":
            await self._handle_keepalive(websocket, data)
        else:
            logger.warning(f"⚠️ Unknown message type: {message_type}")

    async def _handle_smart_subscription(self, websocket: WebSocket, data: dict):
        """Updates the subscription set for a specific client."""
        stock_symbols = set(data.get("stock_symbols", []))
        option_symbols = set(data.get("option_symbols", []))
        all_symbols = stock_symbols.union(option_symbols)
        
        # Initialize symbol keep-alives for new symbols
        current_time = time.time()
        for symbol in all_symbols:
            if symbol not in self.symbol_keepalives:
                self.symbol_keepalives[symbol] = current_time
        
        self.client_subscriptions[websocket] = all_symbols
        logger.info(f"Client {id(websocket)} subscribed to {len(all_symbols)} symbols.")
        
        # Notify the streaming manager to update its global subscription list
        await self.streaming_manager.update_global_subscriptions(self.client_subscriptions)
        
        await self._send_json_safe(websocket, {
            "type": "subscription_confirmed",
            "stock_symbols": list(stock_symbols),
            "option_symbols": list(option_symbols)
        })

    async def _handle_keepalive(self, websocket: WebSocket, data: dict):
        """Handles a keep-alive message from the client."""
        symbols = set(data.get("symbols", []))
        current_time = time.time()
        
        # Update client keep-alive
        self.client_keepalives[websocket] = current_time
        
        # Update symbol keep-alives
        for symbol in symbols:
            self.symbol_keepalives[symbol] = current_time
        
        # Update client subscriptions
        self.client_subscriptions[websocket] = symbols
        
        logger.debug(f"Received keep-alive from {id(websocket)} for {len(symbols)} symbols.")
        await self._send_json_safe(websocket, {"type": "pong"})

    async def _check_client_keepalives(self):
        """Periodically checks for stale clients and disconnects them."""
        while not self.is_shutting_down:
            await asyncio.sleep(300)  # Check every 5 minutes for client timeouts
            
            stale_clients = []
            current_time = time.time()
            client_timeout = 300  # 5 minutes for client timeout
            
            for client, last_seen in self.client_keepalives.items():
                if current_time - last_seen > client_timeout:
                    logger.info(f"Client {id(client)} timed out. Last seen {current_time - last_seen:.0f}s ago.")
                    stale_clients.append(client)
            
            if stale_clients:
                cleanup_tasks = [self._cleanup_connection(client) for client in stale_clients]
                await asyncio.gather(*cleanup_tasks)

    async def _check_symbol_keepalives(self):
        """Periodically checks for stale symbols and removes them from subscriptions."""
        while not self.is_shutting_down:
            await asyncio.sleep(1)  # Check every second for symbol timeouts
            
            current_time = time.time()
            symbol_timeout = settings.subscription_keepalive_timeout
            stale_symbols = []
            
            # Find symbols that haven't received keep-alives
            for symbol, last_seen in self.symbol_keepalives.items():
                if current_time - last_seen > symbol_timeout:
                    logger.info(f"Symbol {symbol} timed out. Last keep-alive {current_time - last_seen:.0f}s ago.")
                    stale_symbols.append(symbol)
            
            if stale_symbols:
                # Remove stale symbols from all client subscriptions
                subscriptions_changed = False
                for client, subscriptions in self.client_subscriptions.items():
                    original_size = len(subscriptions)
                    subscriptions.difference_update(stale_symbols)
                    if len(subscriptions) != original_size:
                        subscriptions_changed = True
                        logger.info(f"Removed {original_size - len(subscriptions)} stale symbols from client {id(client)}")
                
                # Clean up symbol keep-alive tracking
                for symbol in stale_symbols:
                    self.symbol_keepalives.pop(symbol, None)
                
                # Update global subscriptions if any changes were made
                if subscriptions_changed:
                    await self.streaming_manager.update_global_subscriptions(self.client_subscriptions)
