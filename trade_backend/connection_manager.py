import asyncio
import logging
import json
import time
from typing import List, Dict, Optional
from fastapi import WebSocket, WebSocketDisconnect

logger = logging.getLogger(__name__)

class ConnectionManager:
    """
    WebSocket connection manager with proper lifecycle management,
    health monitoring, and graceful shutdown support.
    """
    
    def __init__(self, streaming_manager, shutdown_manager):
        self.active_connections: List[WebSocket] = []
        self.streaming_task: Optional[asyncio.Task] = None
        self.is_streaming = False
        self.is_shutting_down = False
        
        # Dependencies
        self.streaming_manager = streaming_manager
        self.shutdown_manager = shutdown_manager
        
        # Health monitoring
        self._connection_health: Dict[int, Dict] = {}
        self._last_broadcast_time = time.time()
        self._broadcast_count = 0
        
        # Performance tracking
        self._stats = {
            'total_connections': 0,
            'total_messages_sent': 0,
            'total_errors': 0,
            'streaming_task_restarts': 0
        }
        
    async def connect(self, websocket: WebSocket):
        """Enhanced WebSocket connection with health tracking and connection limit protection"""
        if self.is_shutting_down:
            logger.warning("🚫 Rejecting new connection - shutdown in progress")
            await websocket.close(code=1001, reason="Server shutting down")
            return
            
        try:
            # Check for stale connections before accepting new ones
            await self._cleanup_stale_connections()
            
            # Validate connection limits to prevent Alpaca 429 errors
            if len(self.active_connections) >= 5:  # Conservative limit for Alpaca
                logger.warning(f"⚠️ Connection limit reached ({len(self.active_connections)}), cleaning up stale connections")
                await self._cleanup_stale_connections()
                
                # If still at limit after cleanup, reject new connection
                if len(self.active_connections) >= 5:
                    logger.error("❌ Connection limit exceeded after cleanup - rejecting new connection")
                    await websocket.close(code=1008, reason="Connection limit exceeded")
                    return
            
            await websocket.accept()
            self.active_connections.append(websocket)
            self.shutdown_manager.register_websocket(websocket)
            
            # Initialize health tracking
            connection_id = id(websocket)
            self._connection_health[connection_id] = {
                'connected_at': time.time(),
                'last_message': time.time(),
                'message_count': 0,
                'error_count': 0
            }
            
            self._stats['total_connections'] += 1
            
            logger.info(f"✅ WebSocket connected. Total connections: {len(self.active_connections)}")
            
            # Start streaming task if this is the first connection and not already streaming
            await self._ensure_streaming_task()
                
        except Exception as e:
            logger.error(f"❌ Error connecting WebSocket: {e}")
            await self._cleanup_connection(websocket)
    
    async def _ensure_streaming_task(self):
        """Ensure streaming task is running if we have connections"""
        if (len(self.active_connections) > 0 and 
            not self.is_streaming and 
            not self.is_shutting_down):
            
            logger.info("🚀 Starting streaming task for WebSocket connections")
            self.streaming_task = asyncio.create_task(self._handle_streaming_data_enhanced())
            self.is_streaming = True
    
    def disconnect(self, websocket: WebSocket):
        """Enhanced WebSocket disconnection with cleanup"""
        try:
            if websocket in self.active_connections:
                self.active_connections.remove(websocket)
                
            # Cleanup health tracking
            connection_id = id(websocket)
            self._connection_health.pop(connection_id, None)
            
            # Unregister from shutdown manager
            self.shutdown_manager.unregister_websocket(websocket)
            
            logger.info(f"🔌 WebSocket disconnected. Total connections: {len(self.active_connections)}")
            
            # Stop streaming task if no connections remain
            if len(self.active_connections) == 0 and self.is_streaming:
                logger.info("🛑 Stopping streaming task - no active connections")
                self._stop_streaming_task()
                
        except Exception as e:
            logger.error(f"❌ Error disconnecting WebSocket: {e}")
    
    def _stop_streaming_task(self):
        """Safely stop the streaming task"""
        try:
            if self.streaming_task and not self.streaming_task.done():
                self.streaming_task.cancel()
            self.is_streaming = False
            self.streaming_task = None
            logger.debug("🛑 Streaming task stopped")
        except Exception as e:
            logger.error(f"❌ Error stopping streaming task: {e}")
    
    async def _cleanup_connection(self, websocket: WebSocket):
        """Clean up a failed connection"""
        try:
            if hasattr(websocket, 'client_state') and websocket.client_state.name != "DISCONNECTED":
                await asyncio.wait_for(websocket.close(), timeout=2.0)
        except (asyncio.TimeoutError, Exception):
            pass  # Connection might already be closed or timeout
        finally:
            self.disconnect(websocket)
    
    async def _cleanup_stale_connections(self):
        """Clean up stale or disconnected WebSocket connections"""
        try:
            stale_connections = []
            current_time = time.time()
            
            for connection in self.active_connections[:]:  # Copy to avoid modification during iteration
                connection_id = id(connection)
                
                # Check if connection is actually disconnected
                try:
                    if hasattr(connection, 'client_state'):
                        if connection.client_state.name == "DISCONNECTED":
                            stale_connections.append(connection)
                            continue
                    
                    # Check for connections that haven't sent messages recently (stale)
                    if connection_id in self._connection_health:
                        last_message_time = self._connection_health[connection_id].get('last_message', 0)
                        time_since_message = current_time - last_message_time
                        
                        # Consider connections stale if no activity for 5 minutes
                        if time_since_message > 300:
                            logger.info(f"🧹 Marking connection as stale (no activity for {time_since_message:.0f}s)")
                            stale_connections.append(connection)
                            continue
                    
                    # Try to ping the connection to verify it's alive
                    try:
                        await asyncio.wait_for(connection.ping(), timeout=1.0)
                    except (asyncio.TimeoutError, Exception):
                        logger.info("🧹 Connection failed ping test - marking as stale")
                        stale_connections.append(connection)
                        
                except Exception as e:
                    logger.warning(f"⚠️ Error checking connection health: {e}")
                    stale_connections.append(connection)
            
            # Clean up stale connections
            if stale_connections:
                logger.info(f"🧹 Cleaning up {len(stale_connections)} stale connections")
                cleanup_tasks = []
                for conn in stale_connections:
                    cleanup_tasks.append(asyncio.create_task(self._cleanup_connection(conn)))
                
                if cleanup_tasks:
                    await asyncio.gather(*cleanup_tasks, return_exceptions=True)
                    
                logger.info(f"✅ Cleaned up stale connections. Active connections: {len(self.active_connections)}")
            
        except Exception as e:
            logger.error(f"❌ Error during stale connection cleanup: {e}")
    
    async def broadcast(self, message: dict):
        """Enhanced broadcast with connection health checking and error handling"""
        if not self.active_connections or self.is_shutting_down:
            return
            
        disconnected = []
        successful_sends = 0
        current_time = time.time()
        
        for connection in self.active_connections[:]:  # Copy to avoid modification during iteration
            try:
                # Check connection state
                if (hasattr(connection, 'client_state') and 
                    connection.client_state.name != "CONNECTED"):
                    disconnected.append(connection)
                    continue
                
                # Send message with timeout
                await asyncio.wait_for(connection.send_json(message), timeout=2.0)
                successful_sends += 1
                self._stats['total_messages_sent'] += 1
                
                # Update health tracking
                connection_id = id(connection)
                if connection_id in self._connection_health:
                    self._connection_health[connection_id]['last_message'] = current_time
                    self._connection_health[connection_id]['message_count'] += 1
                    
            except asyncio.TimeoutError:
                logger.warning(f"⚠️ WebSocket send timeout")
                disconnected.append(connection)
                self._stats['total_errors'] += 1
            except Exception as e:
                logger.warning(f"⚠️ Failed to send message to WebSocket: {e}")
                disconnected.append(connection)
                self._stats['total_errors'] += 1
        
        # Clean up disconnected clients
        cleanup_tasks = []
        for conn in disconnected:
            cleanup_tasks.append(asyncio.create_task(self._cleanup_connection(conn)))
        
        if cleanup_tasks:
            await asyncio.gather(*cleanup_tasks, return_exceptions=True)
        
        # Update broadcast statistics
        self._broadcast_count += 1
        self._last_broadcast_time = current_time
        
        # Log broadcast statistics periodically
        if self._broadcast_count % 100 == 0:
            logger.debug(f"📊 Broadcast stats: {successful_sends}/{len(self.active_connections)} successful, "
                        f"total messages: {self._stats['total_messages_sent']}, "
                        f"errors: {self._stats['total_errors']}")
    
    async def _handle_streaming_data_enhanced(self):
        """Streaming data handler with better error handling and shutdown awareness"""
        logger.info("🚀 Starting streaming data handler...")
        message_count = 0
        error_count = 0
        last_log_time = time.time()
        consecutive_errors = 0
        max_consecutive_errors = 10
        
        try:
            while (self.is_streaming and 
                   len(self.active_connections) > 0 and 
                   not self.shutdown_manager.is_shutting_down()):
                
                try:
                    # Get latest data from cache instead of queue
                    latest_data = await self.streaming_manager.get_all_latest_data()
                    
                    # Process each symbol's latest data
                    if latest_data:
                        for symbol, market_data in latest_data.items():
                            if market_data and self.active_connections and not self.is_shutting_down:
                                message_count += 1
                                consecutive_errors = 0  # Reset error counter on success
                                
                                # Determine message type based on data content
                                message_type = "price_update"
                                if hasattr(market_data, 'data_type'):
                                    if market_data.data_type == "greeks":
                                        message_type = "greeks_update"
                                elif isinstance(market_data.data, dict):
                                    # Check if data contains Greeks fields
                                    greeks_fields = {'delta', 'gamma', 'theta', 'vega', 'implied_volatility'}
                                    if greeks_fields.intersection(market_data.data.keys()):
                                        message_type = "greeks_update"
                                
                                # Broadcast data with appropriate message type
                                await self.broadcast({
                                    "type": message_type,
                                    "symbol": market_data.symbol,
                                    "data": market_data.data,
                                    "timestamp": market_data.timestamp
                                })
                        
                        # Log statistics periodically
                        current_time = time.time()
                        if current_time - last_log_time > 60:  # Every minute
                            logger.info(f"📊 Streaming stats: {message_count} messages processed, "
                                      f"{error_count} errors, {len(self.active_connections)} connections")
                            last_log_time = current_time
                    else:
                        # No data available, sleep to prevent busy waiting
                        await asyncio.sleep(0.5)
                        continue
                    
                    # Sleep briefly to prevent overwhelming the system
                    await asyncio.sleep(0.1)
                        
                except asyncio.CancelledError:
                    logger.info("🛑 Streaming data handler cancelled")
                    break
                except Exception as e:
                    error_count += 1
                    consecutive_errors += 1
                    logger.error(f"❌ Error in streaming data handler: {e}")
                    
                    # If too many consecutive errors, take a longer break and check connection health
                    if consecutive_errors >= max_consecutive_errors:
                        logger.warning(f"⚠️ {consecutive_errors} consecutive streaming errors, "
                                     f"checking connection health and taking extended break")
                        await self._check_streaming_health()
                        await asyncio.sleep(5)
                        consecutive_errors = 0  # Reset after break
                    else:
                        await asyncio.sleep(1)
                        
        except Exception as e:
            logger.error(f"❌ Fatal error in streaming data handler: {e}")
            self._stats['streaming_task_restarts'] += 1
        finally:
            logger.info(f"🏁 Streaming data handler stopped - "
                       f"processed {message_count} messages with {error_count} errors")
            self.is_streaming = False
    
    async def _check_streaming_health(self):
        """Check streaming manager health and attempt recovery if needed"""
        try:
            if hasattr(self.streaming_manager, 'is_connected'):
                if not self.streaming_manager.is_connected:
                    logger.warning("⚠️ Streaming manager disconnected, attempting recovery")
                    # Could trigger reconnection logic here if available
        except Exception as e:
            logger.error(f"❌ Error checking streaming health: {e}")
    
    async def handle_websocket_message(self, websocket: WebSocket, data: dict):
        """Enhanced WebSocket message handling with better error handling"""
        if self.is_shutting_down:
            await websocket.send_json({
                "type": "error",
                "message": "Server is shutting down"
            })
            return
            
        try:
            message_type = data.get("type")
            
            # Update connection health
            connection_id = id(websocket)
            if connection_id in self._connection_health:
                self._connection_health[connection_id]['last_message'] = time.time()
            
            # Skip processing if no message type (common on initial connection)
            if message_type is None:
                logger.debug("📝 Received message without type field - likely connection initialization")
                return
            
            # Handle different message types
            if message_type == "subscribe_smart_replace_all":
                await self._handle_smart_subscription(websocket, data)
            elif message_type == "subscribe_replace_all":
                await self._handle_unified_subscription(websocket, data)
            elif message_type == "subscribe_persistent":
                await self._handle_persistent_subscription(websocket, data)
            elif message_type == "get_subscription_status":
                await self._handle_subscription_status(websocket)
            elif message_type == "ping":
                await self._handle_ping(websocket)
            elif message_type == "get_connection_stats":
                await self._handle_connection_stats(websocket)
            else:
                logger.warning(f"⚠️ Unknown message type: {message_type}")
                await websocket.send_json({
                    "type": "error",
                    "message": f"Unknown message type: {message_type}"
                })
                
        except Exception as e:
            logger.error(f"❌ Error handling WebSocket message: {e}")
            try:
                await websocket.send_json({
                    "type": "error",
                    "message": f"Error processing message: {str(e)}"
                })
            except:
                # If we can't send error message, connection is probably dead
                await self._cleanup_connection(websocket)
    
    async def _handle_smart_subscription(self, websocket: WebSocket, data: dict):
        """Handle smart subscription replacement"""
        try:
            stock_symbols = data.get("stock_symbols", [])
            option_symbols = data.get("option_symbols", [])
            
            logger.info(f"🔄 WebSocket: Smart subscription - stocks: {len(stock_symbols)}, options: {len(option_symbols)}")
            
            await self.streaming_manager.replace_all_subscriptions_multi_stock(stock_symbols, option_symbols)
            
            await websocket.send_json({
                "type": "subscription_confirmed",
                "subscription_type": "smart_replace_all",
                "stock_symbols": stock_symbols,
                "option_symbols": option_symbols,
                "message": f"Smart subscriptions replaced - stocks: {len(stock_symbols)}, options: {len(option_symbols)}"
            })
            
        except Exception as e:
            logger.error(f"❌ Error handling smart subscription: {e}")
            await websocket.send_json({
                "type": "error",
                "message": f"Error handling smart subscription: {str(e)}"
            })
    
    async def _handle_unified_subscription(self, websocket: WebSocket, data: dict):
        """Handle unified subscription replacement"""
        try:
            underlying_symbol = data.get("underlying_symbol")
            option_symbols = data.get("option_symbols", [])
            
            logger.info(f"🔄 WebSocket: Unified subscription - underlying: {underlying_symbol}, options: {len(option_symbols)}")
            
            if underlying_symbol:
                await self.streaming_manager.replace_all_subscriptions(underlying_symbol, option_symbols)
                await websocket.send_json({
                    "type": "subscription_confirmed",
                    "subscription_type": "all_replace",
                    "underlying_symbol": underlying_symbol,
                    "option_symbols": option_symbols,
                    "message": f"All subscriptions replaced - underlying: {underlying_symbol}, options: {len(option_symbols)}"
                })
            
        except Exception as e:
            logger.error(f"❌ Error handling unified subscription: {e}")
            await websocket.send_json({
                "type": "error",
                "message": f"Error handling unified subscription: {str(e)}"
            })
    
    async def _handle_persistent_subscription(self, websocket: WebSocket, data: dict):
        """Handle persistent subscription (for orders, positions, etc.)"""
        try:
            symbols = data.get("symbols", [])
            
            logger.info(f"🔄 WebSocket: Persistent subscription - symbols: {symbols}")
            
            # Ensure persistent subscriptions for background data
            if hasattr(self.streaming_manager, 'ensure_persistent_subscriptions'):
                await self.streaming_manager.ensure_persistent_subscriptions(symbols)
            
            await websocket.send_json({
                "type": "subscription_confirmed",
                "subscription_type": "persistent",
                "symbols": symbols,
                "message": f"Persistent subscriptions ensured for {len(symbols)} symbols"
            })
            
        except Exception as e:
            logger.error(f"❌ Error handling persistent subscription: {e}")
            await websocket.send_json({
                "type": "error",
                "message": f"Error handling persistent subscription: {str(e)}"
            })
    
    async def _handle_subscription_status(self, websocket: WebSocket):
        """Handle subscription status request"""
        try:
            status = self.streaming_manager.get_subscription_status()
            await websocket.send_json({
                "type": "subscription_status",
                "data": status
            })
        except Exception as e:
            logger.error(f"❌ Error handling subscription status: {e}")
            await websocket.send_json({
                "type": "error",
                "message": f"Error getting subscription status: {str(e)}"
            })
    
    async def _handle_ping(self, websocket: WebSocket):
        """Handle ping message"""
        try:
            await websocket.send_json({
                "type": "pong",
                "timestamp": time.time()
            })
        except Exception as e:
            logger.error(f"❌ Error handling ping: {e}")
    
    async def _handle_connection_stats(self, websocket: WebSocket):
        """Handle connection statistics request"""
        try:
            connection_id = id(websocket)
            connection_health = self._connection_health.get(connection_id, {})
            
            stats = {
                "connection_count": len(self.active_connections),
                "is_streaming": self.is_streaming,
                "connection_health": connection_health,
                "manager_stats": self._stats,
                "last_broadcast_time": self._last_broadcast_time,
                "broadcast_count": self._broadcast_count
            }
            
            await websocket.send_json({
                "type": "connection_stats",
                "data": stats
            })
        except Exception as e:
            logger.error(f"❌ Error handling connection stats: {e}")
    
    def get_stats(self) -> dict:
        """Get connection manager statistics"""
        return {
            "active_connections": len(self.active_connections),
            "is_streaming": self.is_streaming,
            "is_shutting_down": self.is_shutting_down,
            "stats": self._stats.copy(),
            "last_broadcast_time": self._last_broadcast_time,
            "broadcast_count": self._broadcast_count
        }
