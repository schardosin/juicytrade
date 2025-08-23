import asyncio
import logging
import time
from typing import Dict, Set, Optional, List, Any
from datetime import datetime

from .services.ivx_cache import ivx_cache
from .services.ivx_calculator import calculate_ivx_data
from .provider_manager import provider_manager
from .models import MarketData

logger = logging.getLogger(__name__)

class IVxStreamingManager:
    """
    Manages IVx data streaming for subscribed symbols.
    Integrates with existing streaming infrastructure to provide real-time IVx updates.
    """
    
    def __init__(self, connection_manager):
        self.connection_manager = connection_manager
        self.subscribed_symbols: Set[str] = set()
        self.calculation_tasks: Dict[str, asyncio.Task] = {}
        self.calculation_intervals: Dict[str, float] = {}
        self.is_running = False
        self._shutdown_event = asyncio.Event()
        
        # Configuration
        self.default_update_interval = 300  # 5 minutes default
        self.market_hours_interval = 180   # 3 minutes during market hours
        self.after_hours_interval = 600    # 10 minutes after hours
        
        # Track calculation progress for streaming updates
        self.calculation_progress: Dict[str, Dict[str, Any]] = {}
        
    async def start(self):
        """Start the IVx streaming manager."""
        if self.is_running:
            return
            
        self.is_running = True
        self._shutdown_event.clear()
        logger.info("🚀 IVx Streaming Manager started")
        
    async def stop(self):
        """Stop the IVx streaming manager and cleanup tasks."""
        if not self.is_running:
            return
            
        logger.info("🛑 Stopping IVx Streaming Manager...")
        self.is_running = False
        self._shutdown_event.set()
        
        # Cancel all calculation tasks
        for symbol, task in self.calculation_tasks.items():
            if not task.done():
                task.cancel()
                logger.info(f"Cancelled IVx calculation task for {symbol}")
        
        self.calculation_tasks.clear()
        self.subscribed_symbols.clear()
        logger.info("✅ IVx Streaming Manager stopped")
        
    async def calculate_ivx_for_symbol(self, symbol: str):
        """
        Calculates IVx data for a single symbol, streaming progress and caching the result.
        This method is designed to be called by the StreamingManager when a symbol is subscribed.
        """
        if not self.is_running:
            return

        # If a calculation is already running, don't start another one.
        if symbol in self.calculation_tasks and not self.calculation_tasks[symbol].done():
            logger.info(f"IVx calculation for {symbol} is already in progress.")
            return

        # Check cache first before starting calculation
        cached_data = ivx_cache.get(symbol)
        if cached_data:
            logger.info(f"📦 Found cached IVx data for {symbol}, streaming immediately")
            # Stream cached data as partial data (same format as fresh calculations)
            for i, ivx_data in enumerate(cached_data):
                await self._broadcast_partial_ivx_data(symbol, ivx_data, i+1, len(cached_data))
            
            # Schedule automatic refresh when cache expires
            await self._schedule_cache_refresh(symbol)
            return

        # Start a new calculation task.
        task = asyncio.create_task(self._calculate_and_stream_ivx(symbol))
        self.calculation_tasks[symbol] = task
        logger.info(f"🔄 Started IVx calculation task for {symbol}")
    
    async def _calculate_and_stream_ivx(self, symbol: str):
        """Calculate IVx data and stream progress updates using provider-specific implementation."""
        try:
            logger.info(f"🔄 Starting IVx calculation for {symbol}")
            
            # Get the appropriate provider
            provider = await self._get_provider_for_symbol(symbol)
            if not provider:
                logger.error(f"No provider available for IVx calculation for {symbol}")
                return
            
            # Get underlying price
            underlying_price = await self._get_underlying_price(symbol, provider)
            if not underlying_price:
                logger.error(f"Could not get underlying price for {symbol}")
                return
            
            # Use provider-specific IVx calculation method
            logger.info(f"🔧 Using provider-specific IVx method for {symbol}")
            
            # Check if provider has the get_all_expirations_ivx method
            if hasattr(provider, 'get_all_expirations_ivx'):
                logger.info(f"📊 Provider {provider.name} has get_all_expirations_ivx method")
                
                # Call provider-specific method with timeout protection
                try:
                    # Set a reasonable timeout to prevent connection drops
                    ivx_results = await asyncio.wait_for(
                        provider.get_all_expirations_ivx(symbol, underlying_price),
                        timeout=60.0  # 60 second timeout
                    )
                except asyncio.TimeoutError:
                    logger.error(f"⏰ IVx calculation timeout for {symbol} after 60 seconds")
                    return
                except Exception as e:
                    logger.error(f"❌ Provider-specific IVx calculation failed for {symbol}: {e}")
                    return
                
                if ivx_results:
                    # Stream results as they come in (simulate streaming for provider methods)
                    for i, ivx_data in enumerate(ivx_results):
                        # Stream partial result
                        await self._broadcast_partial_ivx_data(symbol, ivx_data, i+1, len(ivx_results))
                    
                    # Cache the complete results
                    ivx_cache.set(symbol, ivx_results)
                    
                    logger.info(f"✅ Completed provider-specific IVx calculation for {symbol}: {len(ivx_results)} expirations")
                else:
                    logger.warning(f"⚠️ Provider-specific IVx calculation returned no results for {symbol}")
            else:
                logger.warning(f"⚠️ Provider {provider.name} does not have get_all_expirations_ivx method")
            
        except Exception as e:
            logger.error(f"❌ Error in provider-specific IVx calculation for {symbol}: {e}")
    
    async def _get_provider_for_symbol(self, symbol: str):
        """Get the appropriate provider for a symbol."""
        try:
            # Use the provider manager to get the options chain provider
            return provider_manager._get_provider("options_chain")
        except Exception as e:
            logger.error(f"Error getting provider for {symbol}: {e}")
            return None
    
    async def _get_underlying_price(self, symbol: str, provider) -> Optional[float]:
        """Get the current underlying price for a symbol."""
        try:
            quote = await provider.get_stock_quote(symbol)
            if quote and quote.bid and quote.ask:
                return (quote.bid + quote.ask) / 2
            return None
        except Exception as e:
            logger.error(f"Error getting underlying price for {symbol}: {e}")
            return None
    
    async def _get_update_interval(self) -> float:
        """Get the appropriate update interval based on market hours."""
        try:
            # For now, use a simple approach - could be enhanced to check actual market hours
            current_hour = datetime.now().hour
            
            # Market hours (9:30 AM - 4:00 PM ET roughly 9-16 in local time)
            if 9 <= current_hour <= 16:
                return self.market_hours_interval
            else:
                return self.after_hours_interval
                
        except Exception:
            return self.default_update_interval
    
    async def _broadcast_ivx_data(self, symbol: str, ivx_data: List[Dict[str, Any]], is_cached: bool = False, is_complete: bool = False):
        """Broadcast complete IVx data to subscribed clients."""
        message = {
            "type": "ivx_data",
            "symbol": symbol,
            "data": ivx_data,
            "timestamp": datetime.now().isoformat(),
            "is_cached": is_cached,
            "is_complete": is_complete,
            "total_expirations": len(ivx_data)
        }
        
        await self._send_to_subscribed_clients(symbol, message)
    
    async def _broadcast_partial_ivx_data(self, symbol: str, ivx_data: Dict[str, Any], completed: int, total: int):
        """Broadcast partial IVx data as calculations complete."""
        message = {
            "type": "ivx_partial_data",
            "symbol": symbol,
            "data": ivx_data,
            "timestamp": datetime.now().isoformat(),
            "progress": {
                "completed": completed,
                "total": total,
                "percentage": (completed / total) * 100
            }
        }
        
        await self._send_to_subscribed_clients(symbol, message)
    
    
    async def _schedule_cache_refresh(self, symbol: str):
        """Schedule automatic cache refresh when cache is about to expire."""
        try:
            # Calculate time until cache expires (5 minutes = 300 seconds)
            cache_ttl = ivx_cache.ttl_seconds
            
            # Schedule refresh slightly before expiration (30 seconds early)
            refresh_delay = cache_ttl - 30
            if refresh_delay <= 0:
                refresh_delay = cache_ttl * 0.9  # 90% of TTL if too close to expiration
            
            logger.info(f"⏰ Scheduling cache refresh for {symbol} in {refresh_delay} seconds")
            
            # Create a background task to refresh the cache
            refresh_task = asyncio.create_task(self._delayed_cache_refresh(symbol, refresh_delay))
            
            # Store the refresh task (replace any existing one)
            refresh_key = f"{symbol}_refresh"
            if refresh_key in self.calculation_tasks:
                old_task = self.calculation_tasks[refresh_key]
                if not old_task.done():
                    old_task.cancel()
            
            self.calculation_tasks[refresh_key] = refresh_task
            
        except Exception as e:
            logger.error(f"❌ Error scheduling cache refresh for {symbol}: {e}")
    
    async def _delayed_cache_refresh(self, symbol: str, delay: float):
        """Wait for the specified delay, then refresh the cache for the symbol."""
        try:
            await asyncio.sleep(delay)
            
            # Check if we're still running and the symbol is still relevant
            if not self.is_running:
                return
            
            # Check if there are still clients subscribed to this symbol
            has_subscribers = False
            for subscriptions in self.connection_manager.client_subscriptions.values():
                if symbol in subscriptions:
                    has_subscribers = True
                    break
            
            if not has_subscribers:
                logger.info(f"⏭️ Skipping cache refresh for {symbol} - no active subscribers")
                return
            
            logger.info(f"🔄 Auto-refreshing cache for {symbol}")
            
            # Force a new calculation (bypass cache check)
            task = asyncio.create_task(self._calculate_and_stream_ivx(symbol))
            self.calculation_tasks[symbol] = task
            
        except asyncio.CancelledError:
            logger.info(f"⏹️ Cache refresh cancelled for {symbol}")
        except Exception as e:
            logger.error(f"❌ Error in delayed cache refresh for {symbol}: {e}")
    
    async def _send_to_subscribed_clients(self, symbol: str, message: Dict[str, Any]):
        """Send message to all clients subscribed to the symbol."""
        if not self.connection_manager.active_connections:
            return
            
        # Create tasks to send to all subscribed clients
        send_tasks = []
        for connection, subscriptions in self.connection_manager.client_subscriptions.items():
            if symbol in subscriptions:
                send_tasks.append(self.connection_manager._send_json_safe(connection, message))
        
        if send_tasks:
            await asyncio.gather(*send_tasks, return_exceptions=True)
    
    def get_status(self) -> Dict[str, Any]:
        """Get current status of the IVx streaming manager."""
        return {
            "is_running": self.is_running,
            "subscribed_symbols": list(self.subscribed_symbols),
            "active_calculations": len([t for t in self.calculation_tasks.values() if not t.done()]),
            "calculation_progress": self.calculation_progress.copy()
        }

# Global instance - will be initialized in main.py
ivx_streaming_manager: Optional[IVxStreamingManager] = None

def set_global_ivx_streaming_manager(manager_instance: IVxStreamingManager):
    """Set the global IVx streaming manager instance."""
    global ivx_streaming_manager
    ivx_streaming_manager = manager_instance
