import asyncio
import signal
import logging
import time
from typing import Set, List, Optional
import os

logger = logging.getLogger(__name__)

class GracefulShutdownManager:
    """
    Enhanced shutdown manager that handles graceful cleanup of all resources
    including WebSocket connections, streaming tasks, and provider connections.
    """
    
    def __init__(self):
        self.shutdown_event = asyncio.Event()
        self.cleanup_tasks: Set[asyncio.Task] = set()
        self.websocket_connections: List = []
        self.streaming_manager = None
        self.connection_manager = None
        self._shutdown_timeout = 10.0  # Maximum time to wait for graceful shutdown
        self._force_exit_timeout = 15.0  # Maximum time before force exit
        
    def register_websocket(self, ws):
        """Register WebSocket connection for cleanup"""
        if ws not in self.websocket_connections:
            self.websocket_connections.append(ws)
            logger.debug(f"Registered WebSocket connection. Total: {len(self.websocket_connections)}")
        
    def unregister_websocket(self, ws):
        """Unregister WebSocket connection"""
        if ws in self.websocket_connections:
            self.websocket_connections.remove(ws)
            logger.debug(f"Unregistered WebSocket connection. Total: {len(self.websocket_connections)}")
            
    def set_streaming_manager(self, manager):
        """Set streaming manager for cleanup"""
        self.streaming_manager = manager
        
    def set_connection_manager(self, manager):
        """Set connection manager for cleanup"""
        self.connection_manager = manager
    
    def setup_signal_handlers(self):
        """Setup signal handlers for graceful shutdown"""
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        logger.info("🛡️ Signal handlers registered for graceful shutdown")
        
    def _signal_handler(self, signum, frame):
        """Enhanced signal handler with proper cleanup"""
        logger.info(f"🛑 Received signal {signum}, initiating graceful shutdown...")
        
        # Set shutdown event immediately
        self.shutdown_event.set()
        
        # Create cleanup task with timeout protection
        try:
            loop = asyncio.get_running_loop()
            
            # Cancel any existing cleanup tasks
            for task in self.cleanup_tasks.copy():
                if not task.done():
                    task.cancel()
            self.cleanup_tasks.clear()
            
            # Create new cleanup task
            cleanup_task = asyncio.create_task(self._comprehensive_cleanup())
            self.cleanup_tasks.add(cleanup_task)
            
            # Set up force exit timer
            force_exit_task = asyncio.create_task(self._force_exit_timer())
            self.cleanup_tasks.add(force_exit_task)
            
        except RuntimeError:
            logger.error("❌ No running event loop found during shutdown - forcing immediate exit")
            self._immediate_exit()
            
    async def _comprehensive_cleanup(self):
        """Comprehensive cleanup of all resources with timeout protection"""
        cleanup_start_time = time.time()
        
        try:
            logger.info("🧹 Starting comprehensive resource cleanup...")
            
            # Step 1: Stop accepting new connections and mark as shutting down
            await self._stop_new_connections()
            
            # Step 2: Close all WebSocket connections immediately
            await self._close_websockets()
            
            # Step 3: Disconnect streaming manager
            await self._disconnect_streaming()
            
            # Step 4: Cancel all running tasks with timeout
            await self._cancel_all_tasks()
            
            cleanup_time = time.time() - cleanup_start_time
            logger.info(f"✅ Graceful shutdown completed in {cleanup_time:.2f} seconds")
            
            # Exit cleanly
            await asyncio.sleep(0.1)  # Brief pause to ensure logs are written
            os._exit(0)
            
        except Exception as e:
            logger.error(f"❌ Error during comprehensive cleanup: {e}")
            self._immediate_exit()
    
    async def _stop_new_connections(self):
        """Stop accepting new connections"""
        try:
            if self.connection_manager:
                # Mark connection manager as shutting down
                self.connection_manager.is_shutting_down = True
                logger.info("🚫 Stopped accepting new connections")
        except Exception as e:
            logger.warning(f"⚠️ Error stopping new connections: {e}")
    
    async def _close_websockets(self):
        """Force close all WebSocket connections with timeout"""
        try:
            if not self.websocket_connections:
                logger.info("📭 No WebSocket connections to close")
                return
                
            logger.info(f"🔌 Closing {len(self.websocket_connections)} WebSocket connections...")
            
            close_tasks = []
            for ws in self.websocket_connections[:]:  # Copy to avoid modification during iteration
                try:
                    # Check if WebSocket is still open
                    if hasattr(ws, 'close') and not getattr(ws, 'closed', True):
                        close_task = asyncio.create_task(self._close_websocket_with_timeout(ws))
                        close_tasks.append(close_task)
                except Exception as e:
                    logger.warning(f"⚠️ Error preparing WebSocket close: {e}")
            
            if close_tasks:
                # Wait for all closes with timeout
                try:
                    await asyncio.wait_for(
                        asyncio.gather(*close_tasks, return_exceptions=True),
                        timeout=3.0
                    )
                except asyncio.TimeoutError:
                    logger.warning("⚠️ WebSocket close timeout - some connections may still be open")
                    
            self.websocket_connections.clear()
            logger.info("✅ All WebSocket connections processed")
            
        except Exception as e:
            logger.error(f"❌ Error closing WebSockets: {e}")
    
    async def _close_websocket_with_timeout(self, ws):
        """Close a single WebSocket with timeout"""
        try:
            await asyncio.wait_for(ws.close(), timeout=2.0)
        except asyncio.TimeoutError:
            logger.warning("⚠️ Individual WebSocket close timeout")
        except Exception as e:
            logger.warning(f"⚠️ Error closing individual WebSocket: {e}")
            
    async def _disconnect_streaming(self):
        """Disconnect streaming manager with timeout"""
        try:
            if self.streaming_manager:
                logger.info("📡 Disconnecting streaming manager...")
                await asyncio.wait_for(
                    self.streaming_manager.disconnect(),
                    timeout=5.0
                )
                logger.info("✅ Streaming manager disconnected")
            else:
                logger.info("📭 No streaming manager to disconnect")
        except asyncio.TimeoutError:
            logger.warning("⚠️ Streaming disconnect timeout")
        except Exception as e:
            logger.error(f"❌ Error disconnecting streaming: {e}")
            
    async def _cancel_all_tasks(self):
        """Cancel all running tasks with proper waiting and timeout"""
        try:
            loop = asyncio.get_running_loop()
            current_task = asyncio.current_task()
            
            # Get all tasks except current cleanup tasks
            all_tasks = [
                task for task in asyncio.all_tasks(loop) 
                if not task.done() 
                and task != current_task 
                and task not in self.cleanup_tasks
            ]
            
            if not all_tasks:
                logger.info("📭 No additional tasks to cancel")
                return
                
            logger.info(f"🛑 Cancelling {len(all_tasks)} running tasks...")
            
            # Cancel all tasks
            for task in all_tasks:
                if not task.done():
                    task.cancel()
            
            # Wait for cancellation with timeout
            try:
                await asyncio.wait_for(
                    asyncio.gather(*all_tasks, return_exceptions=True),
                    timeout=self._shutdown_timeout
                )
                logger.info("✅ All tasks cancelled successfully")
            except asyncio.TimeoutError:
                logger.warning(f"⚠️ Task cancellation timeout after {self._shutdown_timeout}s - some tasks may still be running")
                
        except Exception as e:
            logger.error(f"❌ Error cancelling tasks: {e}")
    
    async def _force_exit_timer(self):
        """Force exit after timeout to prevent hanging"""
        try:
            await asyncio.sleep(self._force_exit_timeout)
            logger.warning(f"⏰ Force exit timeout reached after {self._force_exit_timeout}s")
            self._immediate_exit()
        except asyncio.CancelledError:
            # Normal cancellation when cleanup completes
            pass
        except Exception as e:
            logger.error(f"❌ Error in force exit timer: {e}")
            self._immediate_exit()
    
    def _immediate_exit(self):
        """Immediate exit without cleanup"""
        logger.warning("🔥 Forcing immediate exit...")
        try:
            # Try to flush logs
            import sys
            sys.stdout.flush()
            sys.stderr.flush()
        except:
            pass
        os._exit(1)
    
    def is_shutting_down(self) -> bool:
        """Check if shutdown is in progress"""
        return self.shutdown_event.is_set()

# Global instance
shutdown_manager = GracefulShutdownManager()
