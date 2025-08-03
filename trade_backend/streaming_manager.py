import asyncio
import logging
import time
from typing import List, Dict, Optional, Set

from .provider_config import provider_config_manager
from .provider_manager import provider_manager
from .models import MarketData

logger = logging.getLogger(__name__)

class StreamingManager:
    """
    Streaming manager with robust connection management,
    health monitoring, automatic recovery, and graceful shutdown support.
    """
    
    def __init__(self):
        self._providers = {}
        self._streaming_queue = asyncio.Queue()
        self._subscriptions: Dict[str, Set[str]] = {}
        
        # Enhanced subscription tracking
        self._stock_subscriptions: Set[str] = set()
        self._options_subscriptions: Set[str] = set()
        self._persistent_subscriptions: Set[str] = set()
        
        # Connection management
        self._is_connected = False
        self._shutdown_event = asyncio.Event()
        self._health_check_task = None
        self._last_data_time = time.time()
        self._connection_attempts = 0
        self._max_connection_attempts = 5
        
        # Health monitoring
        self._health_stats = {
            'total_messages': 0,
            'last_message_time': None,
            'connection_uptime': 0,
            'reconnection_count': 0,
            'last_error': None,
            'error_count': 0
        }
        
    async def connect(self):
        """Enhanced connection with health monitoring and retry logic"""
        try:
            logger.info("🔄 Starting enhanced streaming manager connection...")
            
            config = provider_config_manager.get_config()
            
            # Connect streaming quotes provider with retry logic
            quotes_provider_name = config.get("streaming_quotes")
            if quotes_provider_name and quotes_provider_name not in self._providers:
                provider = provider_manager._providers.get(quotes_provider_name)
                if provider:
                    provider.set_streaming_queue(self._streaming_queue)
                    self._providers[quotes_provider_name] = provider
                    
                    # Enhanced connection with retry
                    connected = await self._connect_with_retry(provider)
                    if connected:
                        logger.info(f"✅ Provider {quotes_provider_name} connected for quotes streaming")
                        self._is_connected = True
                        self._connection_attempts = 0
                        
                        # Start health monitoring
                        self._health_check_task = asyncio.create_task(self._health_monitor())
                        
                        # Update health stats
                        self._health_stats['connection_uptime'] = time.time()
                    else:
                        logger.error(f"❌ Failed to connect provider {quotes_provider_name} after {self._max_connection_attempts} attempts")
                        return False
            
            # Connect trade account provider
            trade_account_provider_name = config.get("trade_account")
            if trade_account_provider_name and trade_account_provider_name not in self._providers:
                provider = provider_manager._providers.get(trade_account_provider_name)
                if provider:
                    provider.set_streaming_queue(self._streaming_queue)
                    self._providers[trade_account_provider_name] = provider
                    await provider.connect_streaming()
                    logger.info(f"✅ Provider {trade_account_provider_name} connected for trade account streaming")
            
            logger.info("✅ Enhanced streaming manager connected successfully")
            return True
            
        except Exception as e:
            logger.error(f"❌ Error connecting enhanced streaming manager: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
            return False
    
    async def _connect_with_retry(self, provider, max_retries=None):
        """Connect provider with exponential backoff retry logic"""
        if max_retries is None:
            max_retries = self._max_connection_attempts
            
        for attempt in range(max_retries):
            try:
                self._connection_attempts = attempt + 1
                logger.info(f"🔄 Connecting {provider.name} (attempt {attempt + 1}/{max_retries})")
                
                connected = await asyncio.wait_for(
                    provider.connect_streaming(),
                    timeout=15.0
                )
                
                if connected:
                    logger.info(f"✅ {provider.name} connected successfully")
                    return True
                else:
                    logger.warning(f"⚠️ {provider.name} connection returned False")
                    
            except asyncio.TimeoutError:
                logger.warning(f"⚠️ Connection timeout for {provider.name} (attempt {attempt + 1})")
            except Exception as e:
                logger.error(f"❌ Connection error for {provider.name}: {e}")
                self._health_stats['last_error'] = str(e)
                self._health_stats['error_count'] += 1
            
            if attempt < max_retries - 1:
                # Exponential backoff with jitter
                delay = min(2 ** attempt + (time.time() % 1), 30)  # Max 30 seconds
                logger.info(f"⏳ Waiting {delay:.1f}s before retry...")
                await asyncio.sleep(delay)
        
        return False
    
    async def _health_monitor(self):
        """Monitor connection health and trigger recovery when needed"""
        logger.info("🏥 Starting health monitor...")
        
        try:
            while not self._shutdown_event.is_set():
                await asyncio.sleep(30)  # Check every 30 seconds
                
                current_time = time.time()
                time_since_data = current_time - self._last_data_time
                
                # Check if we've received data recently
                if time_since_data > 120:  # No data for 2 minutes
                    logger.warning(f"⚠️ No streaming data received for {time_since_data:.0f} seconds")
                    await self._attempt_recovery("no_data_timeout")
                
                # Check provider connection status
                await self._check_provider_health()
                
                # Update uptime
                if self._health_stats['connection_uptime']:
                    uptime = current_time - self._health_stats['connection_uptime']
                    if uptime > 300:  # Log uptime every 5 minutes
                        logger.debug(f"📊 Streaming uptime: {uptime/60:.1f} minutes, "
                                   f"messages: {self._health_stats['total_messages']}")
                        
        except asyncio.CancelledError:
            logger.info("🛑 Health monitor cancelled")
        except Exception as e:
            logger.error(f"❌ Error in health monitor: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
    
    async def _check_provider_health(self):
        """Check health of all connected providers"""
        try:
            for provider_name, provider in self._providers.items():
                if hasattr(provider, 'is_streaming_connected'):
                    if not provider.is_streaming_connected():
                        logger.warning(f"⚠️ Provider {provider_name} connection lost")
                        await self._recover_provider(provider, provider_name)
        except Exception as e:
            logger.error(f"❌ Error checking provider health: {e}")
    
    async def _attempt_recovery(self, reason: str):
        """Attempt to recover from connection issues with enhanced sleep/wake handling"""
        try:
            logger.info(f"🔄 Attempting recovery due to: {reason}")
            self._health_stats['reconnection_count'] += 1
            
            # For sleep/wake scenarios, be more aggressive with cleanup
            if reason in ["system_wakeup", "no_data_timeout"]:
                logger.info("🧹 Sleep/wake detected - performing aggressive connection cleanup")
                
                # Force disconnect all providers first to clear stale connections
                for provider_name, provider in self._providers.items():
                    try:
                        logger.info(f"🛑 Force disconnecting {provider_name} for cleanup")
                        await asyncio.wait_for(provider.disconnect_streaming(), timeout=5.0)
                    except Exception as e:
                        logger.warning(f"⚠️ Error force disconnecting {provider_name}: {e}")
                
                # Wait longer for cleanup after sleep/wake
                await asyncio.sleep(2.0)
            
            # Try to reconnect all providers
            for provider_name, provider in self._providers.items():
                await self._recover_provider(provider, provider_name)
                
        except Exception as e:
            logger.error(f"❌ Error during recovery attempt: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
    
    async def _recover_provider(self, provider, provider_name: str):
        """Recover a specific provider connection"""
        try:
            logger.info(f"🔄 Recovering provider {provider_name}...")
            
            # Disconnect first
            try:
                await asyncio.wait_for(provider.disconnect_streaming(), timeout=5.0)
            except asyncio.TimeoutError:
                logger.warning(f"⚠️ Disconnect timeout for {provider_name}")
            except Exception as e:
                logger.warning(f"⚠️ Error disconnecting {provider_name}: {e}")
            
            # Reconnect with retry
            connected = await self._connect_with_retry(provider, max_retries=3)
            if connected:
                # Restore subscriptions
                await self._restore_subscriptions(provider, provider_name)
                logger.info(f"✅ Provider {provider_name} recovered successfully")
            else:
                logger.error(f"❌ Failed to recover provider {provider_name}")
                
        except Exception as e:
            logger.error(f"❌ Error recovering provider {provider_name}: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
    
    async def _restore_subscriptions(self, provider, provider_name: str):
        """Restore subscriptions after provider reconnection"""
        try:
            all_symbols = list(self._stock_subscriptions | self._options_subscriptions)
            if all_symbols:
                logger.info(f"🔄 Restoring {len(all_symbols)} subscriptions for {provider_name}")
                await provider.subscribe_to_symbols(all_symbols)
                
                # Update internal tracking
                if "quotes" not in self._subscriptions:
                    self._subscriptions["quotes"] = set()
                self._subscriptions["quotes"].update(all_symbols)
                
                logger.info(f"✅ Subscriptions restored for {provider_name}")
        except Exception as e:
            logger.error(f"❌ Error restoring subscriptions for {provider_name}: {e}")

    async def disconnect(self):
        """Enhanced disconnect with proper cleanup and timeout protection"""
        try:
            logger.info("🛑 Disconnecting enhanced streaming manager...")
            
            # Set shutdown event
            self._shutdown_event.set()
            
            # Cancel health monitor
            if self._health_check_task and not self._health_check_task.done():
                self._health_check_task.cancel()
                try:
                    await asyncio.wait_for(self._health_check_task, timeout=3.0)
                except (asyncio.TimeoutError, asyncio.CancelledError):
                    pass
            
            # Disconnect all providers with timeout
            disconnect_tasks = []
            for provider_name, provider in self._providers.items():
                disconnect_task = asyncio.create_task(
                    self._disconnect_provider_with_timeout(provider, provider_name)
                )
                disconnect_tasks.append(disconnect_task)
            
            if disconnect_tasks:
                await asyncio.wait_for(
                    asyncio.gather(*disconnect_tasks, return_exceptions=True),
                    timeout=10.0
                )
            
            # Clear state
            self._providers.clear()
            self._subscriptions.clear()
            self._stock_subscriptions.clear()
            self._options_subscriptions.clear()
            self._is_connected = False
            
            logger.info("✅ Enhanced streaming manager disconnected")
            
        except asyncio.TimeoutError:
            logger.warning("⚠️ Streaming manager disconnect timeout")
        except Exception as e:
            logger.error(f"❌ Error disconnecting streaming manager: {e}")
    
    async def _disconnect_provider_with_timeout(self, provider, provider_name: str):
        """Disconnect provider with timeout protection"""
        try:
            await asyncio.wait_for(provider.disconnect_streaming(), timeout=5.0)
            logger.info(f"✅ Provider {provider_name} disconnected")
        except asyncio.TimeoutError:
            logger.warning(f"⚠️ Disconnect timeout for provider {provider_name}")
        except Exception as e:
            logger.error(f"❌ Error disconnecting provider {provider_name}: {e}")

    async def get_data(self) -> Optional[MarketData]:
        """Enhanced get_data with proper timeout, health tracking, and shutdown awareness"""
        try:
            # Check if we're shutting down
            if self._shutdown_event.is_set():
                return None
            
            # Get data with timeout
            data = await asyncio.wait_for(self._streaming_queue.get(), timeout=2.0)
            
            if data:
                # Update health tracking
                current_time = time.time()
                self._last_data_time = current_time
                self._health_stats['total_messages'] += 1
                self._health_stats['last_message_time'] = current_time
                
                return data
            
            return None
            
        except asyncio.TimeoutError:
            # Timeout is normal when no data is available
            return None
        except Exception as e:
            if not self._shutdown_event.is_set():
                logger.error(f"❌ Error getting streaming data: {e}")
                self._health_stats['last_error'] = str(e)
                self._health_stats['error_count'] += 1
            return None
    
    # Enhanced subscription methods with better error handling
    
    async def replace_all_subscriptions_multi_stock(self, stock_symbols: List[str], option_symbols: List[str]):
        """Enhanced subscription replacement with better error handling and recovery"""
        try:
            if self._shutdown_event.is_set():
                logger.warning("⚠️ Shutdown in progress, ignoring subscription request")
                return
                
            logger.info(f"🔄 Enhanced StreamingManager: Replacing all subscriptions - "
                       f"stocks: {len(stock_symbols)}, options: {len(option_symbols)}")
            
            # Step 1: Unsubscribe from current symbols
            all_current_symbols = list(self._stock_subscriptions | self._options_subscriptions)
            
            if all_current_symbols:
                logger.info(f"🧹 Unsubscribing from {len(all_current_symbols)} current symbols")
                await self._unsubscribe_symbols_safe(all_current_symbols)
            
            # Step 2: Subscribe to new symbols
            all_new_symbols = stock_symbols + option_symbols
            if all_new_symbols:
                logger.info(f"📡 Subscribing to {len(all_new_symbols)} new symbols")
                await self._subscribe_symbols_safe(all_new_symbols)
            
            # Step 3: Update tracking
            old_stock_count = len(self._stock_subscriptions)
            old_options_count = len(self._options_subscriptions)
            
            self._stock_subscriptions = set(stock_symbols)
            self._options_subscriptions = set(option_symbols)
            
            logger.info(f"✅ All subscriptions replaced. Stocks: {old_stock_count} -> {len(stock_symbols)}, "
                       f"Options: {old_options_count} -> {len(option_symbols)}")
            
        except Exception as e:
            logger.error(f"❌ Error in replace_all_subscriptions_multi_stock: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
    
    async def _subscribe_symbols_safe(self, symbols: List[str]):
        """Safe symbol subscription with error handling and recovery"""
        try:
            config = provider_config_manager.get_config()
            quotes_provider_name = config.get("streaming_quotes")
            
            if quotes_provider_name and quotes_provider_name in self._providers:
                provider = self._providers[quotes_provider_name]
                
                # Check if provider is connected
                if hasattr(provider, 'is_streaming_connected') and not provider.is_streaming_connected():
                    logger.warning(f"⚠️ Provider {quotes_provider_name} not connected, attempting recovery")
                    await self._recover_provider(provider, quotes_provider_name)
                
                # Subscribe with timeout
                success = await asyncio.wait_for(
                    provider.subscribe_to_symbols(symbols),
                    timeout=15.0
                )
                
                if success:
                    # Update internal tracking
                    if "quotes" not in self._subscriptions:
                        self._subscriptions["quotes"] = set()
                    self._subscriptions["quotes"].update(symbols)
                    logger.info(f"✅ Successfully subscribed to {len(symbols)} symbols")
                else:
                    logger.warning(f"⚠️ Subscription returned False for {len(symbols)} symbols")
                
        except asyncio.TimeoutError:
            logger.error(f"⚠️ Subscription timeout for {len(symbols)} symbols")
            # Attempt recovery on timeout
            await self._attempt_recovery("subscription_timeout")
        except Exception as e:
            logger.error(f"❌ Error subscribing to symbols: {e}")
            self._health_stats['last_error'] = str(e)
            self._health_stats['error_count'] += 1
    
    async def _unsubscribe_symbols_safe(self, symbols: List[str]):
        """Safe symbol unsubscription with error handling"""
        try:
            config = provider_config_manager.get_config()
            quotes_provider_name = config.get("streaming_quotes")
            
            if quotes_provider_name and quotes_provider_name in self._providers:
                provider = self._providers[quotes_provider_name]
                
                if hasattr(provider, 'unsubscribe_from_symbols'):
                    await asyncio.wait_for(
                        provider.unsubscribe_from_symbols(symbols),
                        timeout=10.0
                    )
                
                # Update internal tracking
                if "quotes" in self._subscriptions:
                    self._subscriptions["quotes"] -= set(symbols)
                    
        except asyncio.TimeoutError:
            logger.warning(f"⚠️ Unsubscription timeout for {len(symbols)} symbols")
        except Exception as e:
            logger.error(f"❌ Error unsubscribing from symbols: {e}")
    
    # Legacy methods for backward compatibility
    
    async def replace_all_subscriptions(self, underlying_symbol: str, option_symbols: List[str]):
        """Legacy method - redirects to enhanced multi-stock version"""
        await self.replace_all_subscriptions_multi_stock([underlying_symbol], option_symbols)
    
    def get_subscription_status(self) -> Dict[str, any]:
        """Get current subscription status with enhanced health information"""
        return {
            "stock_subscriptions": list(self._stock_subscriptions),
            "options_subscriptions": list(self._options_subscriptions),
            "persistent_subscriptions": list(self._persistent_subscriptions),
            "total_subscriptions": {
                data_type: len(symbols) for data_type, symbols in self._subscriptions.items()
            },
            "is_connected": self._is_connected,
            "health_stats": self._health_stats.copy(),
            "connection_attempts": self._connection_attempts,
            "shutdown_in_progress": self._shutdown_event.is_set()
        }
    
    def get_health_stats(self) -> Dict[str, any]:
        """Get detailed health statistics"""
        current_time = time.time()
        uptime = (current_time - self._health_stats['connection_uptime'] 
                 if self._health_stats['connection_uptime'] else 0)
        
        return {
            **self._health_stats,
            "uptime_seconds": uptime,
            "uptime_minutes": uptime / 60,
            "is_connected": self._is_connected,
            "active_providers": len(self._providers),
            "queue_size": self._streaming_queue.qsize(),
            "time_since_last_data": current_time - self._last_data_time
        }

# Create streaming manager instance
streaming_manager = StreamingManager()
