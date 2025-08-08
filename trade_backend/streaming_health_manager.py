import asyncio
import logging
import time
import json
from typing import Dict, List, Optional, Set, Any, Callable
from datetime import datetime, timedelta
from enum import Enum
from dataclasses import dataclass, asdict
import weakref

logger = logging.getLogger(__name__)

class ConnectionState(Enum):
    """Connection states for health monitoring."""
    DISCONNECTED = "disconnected"
    CONNECTING = "connecting"
    CONNECTED = "connected"
    DEGRADED = "degraded"
    FAILED = "failed"
    RECOVERING = "recovering"

class HealthStatus(Enum):
    """Overall health status."""
    HEALTHY = "healthy"
    WARNING = "warning"
    CRITICAL = "critical"
    FAILED = "failed"

@dataclass
class ConnectionMetrics:
    """Metrics for a single connection."""
    provider_name: str
    connection_type: str  # "websocket", "http_stream", etc.
    state: ConnectionState
    connected_at: Optional[float] = None
    last_data_time: Optional[float] = None
    last_ping_time: Optional[float] = None
    last_pong_time: Optional[float] = None
    message_count: int = 0
    error_count: int = 0
    reconnection_count: int = 0
    last_error: Optional[str] = None
    subscribed_symbols: Set[str] = None
    
    def __post_init__(self):
        if self.subscribed_symbols is None:
            self.subscribed_symbols = set()
    
    @property
    def uptime_seconds(self) -> float:
        """Get connection uptime in seconds."""
        if self.connected_at:
            return time.time() - self.connected_at
        return 0
    
    @property
    def time_since_last_data(self) -> float:
        """Get time since last data in seconds."""
        if self.last_data_time and self.last_data_time > 0:
            return time.time() - self.last_data_time
        # Return a large but finite number instead of inf to prevent logging issues
        return 999999.0
    
    @property
    def is_stale(self) -> bool:
        """Check if connection is stale (no data for too long)."""
        return self.time_since_last_data > 120  # 2 minutes
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        data = asdict(self)
        data['state'] = self.state.value
        data['subscribed_symbols'] = list(self.subscribed_symbols)
        data['uptime_seconds'] = self.uptime_seconds
        data['time_since_last_data'] = self.time_since_last_data
        data['is_stale'] = self.is_stale
        return data

class TimeoutWrapper:
    """Universal timeout wrapper for all async operations."""
    
    @staticmethod
    async def execute(coro, timeout: float, operation_name: str = "operation"):
        """Execute coroutine with timeout protection."""
        try:
            return await asyncio.wait_for(coro, timeout=timeout)
        except asyncio.TimeoutError:
            logger.warning(f"⏰ Timeout after {timeout}s for {operation_name}")
            raise
        except Exception as e:
            logger.error(f"❌ Error in {operation_name}: {e}")
            raise

class CircuitBreaker:
    """Circuit breaker pattern for connection failure isolation."""
    
    def __init__(self, failure_threshold: int = 5, recovery_timeout: float = 60.0):
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout
        self.failure_count = 0
        self.last_failure_time = 0
        self.state = "closed"  # closed, open, half_open
    
    def can_execute(self) -> bool:
        """Check if operation can be executed."""
        if self.state == "closed":
            return True
        elif self.state == "open":
            if time.time() - self.last_failure_time > self.recovery_timeout:
                self.state = "half_open"
                return True
            return False
        else:  # half_open
            return True
    
    def record_success(self):
        """Record successful operation."""
        self.failure_count = 0
        self.state = "closed"
    
    def record_failure(self):
        """Record failed operation."""
        self.failure_count += 1
        self.last_failure_time = time.time()
        
        if self.failure_count >= self.failure_threshold:
            self.state = "open"
            logger.warning(f"🔴 Circuit breaker opened after {self.failure_count} failures")

class StreamingHealthManager:
    """
    Comprehensive streaming health management system.
    
    Features:
    - Real-time connection health monitoring
    - Automatic failure detection and recovery
    - Circuit breaker pattern for failing connections
    - Universal timeout protection
    - Graceful shutdown management
    - Performance metrics and alerting
    """
    
    def __init__(self):
        self.connections: Dict[str, ConnectionMetrics] = {}
        self.circuit_breakers: Dict[str, CircuitBreaker] = {}
        self.recovery_tasks: Dict[str, asyncio.Task] = {}
        self.health_monitor_task: Optional[asyncio.Task] = None
        self.shutdown_event = asyncio.Event()
        self.is_monitoring = False
        
        # Health thresholds
        self.data_timeout = 120.0  # 2 minutes without data = stale
        self.ping_interval = 30.0  # Ping every 30 seconds
        self.ping_timeout = 10.0   # Ping response timeout
        self.reconnect_delay = 5.0 # Base reconnect delay
        self.max_reconnect_delay = 300.0  # Max 5 minutes
        
        # Provider references (weak references to avoid circular dependencies)
        self.providers: Dict[str, Any] = {}
        
        # Metrics
        self.start_time = time.time()
        self.total_reconnections = 0
        self.total_failures = 0
        
    def register_provider(self, provider_name: str, provider_instance):
        """Register a provider for health monitoring."""
        self.providers[provider_name] = weakref.ref(provider_instance)
        logger.info(f"📋 Registered provider {provider_name} for health monitoring")
    
    def register_connection(self, connection_id: str, provider_name: str, 
                          connection_type: str = "websocket") -> ConnectionMetrics:
        """Register a new connection for monitoring."""
        metrics = ConnectionMetrics(
            provider_name=provider_name,
            connection_type=connection_type,
            state=ConnectionState.DISCONNECTED
        )
        self.connections[connection_id] = metrics
        self.circuit_breakers[connection_id] = CircuitBreaker()
        
        logger.info(f"📋 Registered connection {connection_id} ({provider_name})")
        return metrics
    
    def update_connection_state(self, connection_id: str, state: ConnectionState):
        """Update connection state."""
        if connection_id in self.connections:
            old_state = self.connections[connection_id].state
            self.connections[connection_id].state = state
            
            if state == ConnectionState.CONNECTED:
                self.connections[connection_id].connected_at = time.time()
                self.circuit_breakers[connection_id].record_success()
            elif state == ConnectionState.FAILED:
                self.circuit_breakers[connection_id].record_failure()
                self.total_failures += 1
            
            if old_state != state:
                logger.info(f"🔄 Connection {connection_id}: {old_state.value} → {state.value}")
    
    def record_data_received(self, connection_id: str):
        """Record that data was received on a connection."""
        if connection_id in self.connections:
            self.connections[connection_id].last_data_time = time.time()
            self.connections[connection_id].message_count += 1
    
    def record_error(self, connection_id: str, error: str):
        """Record an error for a connection."""
        if connection_id in self.connections:
            self.connections[connection_id].error_count += 1
            self.connections[connection_id].last_error = error
            logger.error(f"❌ Connection {connection_id} error: {error}")
    
    def update_subscriptions(self, connection_id: str, symbols: Set[str]):
        """Update subscribed symbols for a connection."""
        if connection_id in self.connections:
            self.connections[connection_id].subscribed_symbols = symbols.copy()
    
    async def start_monitoring(self):
        """Start the health monitoring system."""
        if self.is_monitoring:
            logger.warning("⚠️ Health monitoring already running")
            return
        
        self.is_monitoring = True
        self.shutdown_event.clear()
        
        # Start health monitor task
        self.health_monitor_task = asyncio.create_task(self._health_monitor_loop())
        
        logger.info("🏥 Streaming health monitoring started")
    
    async def stop_monitoring(self):
        """Stop the health monitoring system."""
        logger.info("🛑 Stopping streaming health monitoring...")
        
        self.shutdown_event.set()
        self.is_monitoring = False
        
        # Cancel health monitor task
        if self.health_monitor_task and not self.health_monitor_task.done():
            self.health_monitor_task.cancel()
            try:
                await asyncio.wait_for(self.health_monitor_task, timeout=5.0)
            except (asyncio.TimeoutError, asyncio.CancelledError):
                pass
        
        # Cancel all recovery tasks
        for task in list(self.recovery_tasks.values()):
            if not task.done():
                task.cancel()
        
        # Wait for recovery tasks to complete
        if self.recovery_tasks:
            try:
                await asyncio.wait_for(
                    asyncio.gather(*self.recovery_tasks.values(), return_exceptions=True),
                    timeout=10.0
                )
            except asyncio.TimeoutError:
                logger.warning("⚠️ Recovery tasks did not complete within timeout")
        
        self.recovery_tasks.clear()
        logger.info("✅ Streaming health monitoring stopped")
    
    async def _health_monitor_loop(self):
        """Main health monitoring loop."""
        logger.info("🏥 Health monitor loop started")
        
        try:
            while not self.shutdown_event.is_set():
                try:
                    # Check all connections
                    await self._check_all_connections()
                    
                    # Perform periodic maintenance
                    await self._perform_maintenance()
                    
                    # Wait before next check
                    await asyncio.sleep(10.0)  # Check every 10 seconds
                    
                except asyncio.CancelledError:
                    break
                except Exception as e:
                    logger.error(f"❌ Error in health monitor loop: {e}")
                    await asyncio.sleep(5.0)  # Brief pause on error
                    
        except asyncio.CancelledError:
            logger.info("🛑 Health monitor loop cancelled")
        except Exception as e:
            logger.error(f"❌ Fatal error in health monitor loop: {e}")
        finally:
            logger.info("🏁 Health monitor loop stopped")
    
    async def _check_all_connections(self):
        """Check health of all registered connections."""
        current_time = time.time()
        
        for connection_id, metrics in self.connections.items():
            try:
                await self._check_connection_health(connection_id, metrics, current_time)
            except Exception as e:
                logger.error(f"❌ Error checking connection {connection_id}: {e}")
    
    async def _check_connection_health(self, connection_id: str, metrics: ConnectionMetrics, current_time: float):
        """Check health of a single connection."""
        # Skip if already recovering
        if connection_id in self.recovery_tasks and not self.recovery_tasks[connection_id].done():
            return
        
        # Add startup grace period - don't check for stale data immediately after connection
        startup_grace_period = 60.0  # 60 seconds grace period after connection
        connection_age = current_time - (metrics.connected_at or current_time)
        
        # Check for stale connections (but only after grace period)
        if (metrics.state == ConnectionState.CONNECTED and 
            connection_age > startup_grace_period and 
            metrics.is_stale):
            
            logger.warning(f"⚠️ Connection {connection_id} is stale (no data for {metrics.time_since_last_data:.1f}s)")
            self.update_connection_state(connection_id, ConnectionState.DEGRADED)
            
            # Trigger recovery if very stale (but only after extended grace period)
            if metrics.time_since_last_data > 300:  # 5 minutes
                logger.error(f"❌ Connection {connection_id} is very stale, triggering recovery")
                await self._trigger_recovery(connection_id)
        
        # Check for failed connections
        elif metrics.state == ConnectionState.FAILED:
            await self._trigger_recovery(connection_id)
        
        # Perform periodic ping for healthy connections (but only after grace period)
        elif (metrics.state == ConnectionState.CONNECTED and 
              connection_age > startup_grace_period):
            if not metrics.last_ping_time or (current_time - metrics.last_ping_time) > self.ping_interval:
                await self._perform_ping(connection_id, metrics)
    
    async def _perform_ping(self, connection_id: str, metrics: ConnectionMetrics):
        """Perform ping check on a connection."""
        try:
            provider_ref = self.providers.get(metrics.provider_name)
            if not provider_ref:
                return
            
            provider = provider_ref()
            if not provider:
                return
            
            # Update ping time
            metrics.last_ping_time = time.time()
            
            # Perform provider-specific health check
            if hasattr(provider, '_stream_connection') and provider._stream_connection:
                try:
                    # Try to ping the WebSocket connection
                    await TimeoutWrapper.execute(
                        provider._stream_connection.ping(),
                        timeout=self.ping_timeout,
                        operation_name=f"ping {connection_id}"
                    )
                    metrics.last_pong_time = time.time()
                    logger.debug(f"💓 Ping successful for {connection_id}")
                    
                except Exception as e:
                    logger.warning(f"⚠️ Ping failed for {connection_id}: {e}")
                    self.update_connection_state(connection_id, ConnectionState.DEGRADED)
                    
        except Exception as e:
            logger.error(f"❌ Error performing ping for {connection_id}: {e}")
    
    async def _trigger_recovery(self, connection_id: str):
        """Trigger recovery for a failed connection."""
        if connection_id in self.recovery_tasks and not self.recovery_tasks[connection_id].done():
            logger.debug(f"🔄 Recovery already in progress for {connection_id}")
            return
        
        # Check circuit breaker
        if not self.circuit_breakers[connection_id].can_execute():
            logger.warning(f"🔴 Circuit breaker open for {connection_id}, skipping recovery")
            return
        
        logger.info(f"🔄 Triggering recovery for {connection_id}")
        self.update_connection_state(connection_id, ConnectionState.RECOVERING)
        
        # Start recovery task
        self.recovery_tasks[connection_id] = asyncio.create_task(
            self._recover_connection(connection_id)
        )
    
    async def _recover_connection(self, connection_id: str):
        """Recover a failed connection."""
        metrics = self.connections[connection_id]
        provider_ref = self.providers.get(metrics.provider_name)
        
        if not provider_ref:
            logger.error(f"❌ No provider reference for {connection_id}")
            return
        
        provider = provider_ref()
        if not provider:
            logger.error(f"❌ Provider instance not available for {connection_id}")
            return
        
        try:
            logger.info(f"🔄 Starting recovery for {connection_id}")
            
            # Calculate backoff delay
            delay = min(
                self.reconnect_delay * (2 ** metrics.reconnection_count),
                self.max_reconnect_delay
            )
            
            logger.info(f"⏳ Waiting {delay:.1f}s before reconnection attempt")
            await asyncio.sleep(delay)
            
            # Attempt disconnection first (cleanup)
            try:
                await TimeoutWrapper.execute(
                    provider.disconnect_streaming(),
                    timeout=10.0,
                    operation_name=f"disconnect {connection_id}"
                )
            except Exception as e:
                logger.warning(f"⚠️ Error during disconnect for {connection_id}: {e}")
            
            # Wait a moment for cleanup
            await asyncio.sleep(1.0)
            
            # Attempt reconnection
            success = await TimeoutWrapper.execute(
                provider.connect_streaming(),
                timeout=30.0,
                operation_name=f"reconnect {connection_id}"
            )
            
            if success:
                logger.info(f"✅ Successfully recovered {connection_id}")
                self.update_connection_state(connection_id, ConnectionState.CONNECTED)
                metrics.reconnection_count += 1
                self.total_reconnections += 1
                
                # Restore subscriptions if available
                if metrics.subscribed_symbols:
                    try:
                        await TimeoutWrapper.execute(
                            provider.subscribe_to_symbols(list(metrics.subscribed_symbols)),
                            timeout=15.0,
                            operation_name=f"resubscribe {connection_id}"
                        )
                        logger.info(f"✅ Restored {len(metrics.subscribed_symbols)} subscriptions for {connection_id}")
                    except Exception as e:
                        logger.error(f"❌ Failed to restore subscriptions for {connection_id}: {e}")
                
            else:
                logger.error(f"❌ Failed to recover {connection_id}")
                self.update_connection_state(connection_id, ConnectionState.FAILED)
                
        except Exception as e:
            logger.error(f"❌ Error during recovery for {connection_id}: {e}")
            self.update_connection_state(connection_id, ConnectionState.FAILED)
            self.record_error(connection_id, str(e))
        
        finally:
            # Clean up recovery task
            if connection_id in self.recovery_tasks:
                del self.recovery_tasks[connection_id]
    
    async def _perform_maintenance(self):
        """Perform periodic maintenance tasks."""
        try:
            # Clean up completed recovery tasks
            completed_tasks = [
                conn_id for conn_id, task in self.recovery_tasks.items()
                if task.done()
            ]
            
            for conn_id in completed_tasks:
                del self.recovery_tasks[conn_id]
            
            # Log periodic health summary
            if int(time.time()) % 300 == 0:  # Every 5 minutes
                await self._log_health_summary()
                
        except Exception as e:
            logger.error(f"❌ Error during maintenance: {e}")
    
    async def _log_health_summary(self):
        """Log a summary of system health."""
        try:
            total_connections = len(self.connections)
            healthy_connections = sum(
                1 for m in self.connections.values()
                if m.state == ConnectionState.CONNECTED and not m.is_stale
            )
            
            uptime = time.time() - self.start_time
            
            logger.info(
                f"📊 Health Summary: {healthy_connections}/{total_connections} healthy, "
                f"uptime: {uptime/3600:.1f}h, reconnections: {self.total_reconnections}, "
                f"failures: {self.total_failures}"
            )
            
        except Exception as e:
            logger.error(f"❌ Error logging health summary: {e}")
    
    def get_health_status(self) -> Dict[str, Any]:
        """Get comprehensive health status."""
        try:
            current_time = time.time()
            
            # Calculate overall health
            total_connections = len(self.connections)
            if total_connections == 0:
                overall_status = HealthStatus.HEALTHY
            else:
                healthy_count = sum(
                    1 for m in self.connections.values()
                    if m.state == ConnectionState.CONNECTED and not m.is_stale
                )
                
                health_ratio = healthy_count / total_connections
                
                if health_ratio >= 0.8:
                    overall_status = HealthStatus.HEALTHY
                elif health_ratio >= 0.5:
                    overall_status = HealthStatus.WARNING
                elif health_ratio > 0:
                    overall_status = HealthStatus.CRITICAL
                else:
                    overall_status = HealthStatus.FAILED
            
            # Collect connection details
            connections_status = {}
            for conn_id, metrics in self.connections.items():
                connections_status[conn_id] = metrics.to_dict()
            
            return {
                "overall_status": overall_status.value,
                "monitoring_active": self.is_monitoring,
                "uptime_seconds": current_time - self.start_time,
                "total_connections": total_connections,
                "healthy_connections": sum(
                    1 for m in self.connections.values()
                    if m.state == ConnectionState.CONNECTED and not m.is_stale
                ),
                "total_reconnections": self.total_reconnections,
                "total_failures": self.total_failures,
                "active_recoveries": len(self.recovery_tasks),
                "connections": connections_status,
                "timestamp": current_time
            }
            
        except Exception as e:
            logger.error(f"❌ Error getting health status: {e}")
            return {
                "overall_status": HealthStatus.FAILED.value,
                "error": str(e),
                "timestamp": time.time()
            }
    
    async def force_recovery_all(self):
        """Force recovery of all connections (emergency function)."""
        logger.warning("🚨 Force recovery triggered for all connections")
        
        for connection_id in list(self.connections.keys()):
            try:
                await self._trigger_recovery(connection_id)
            except Exception as e:
                logger.error(f"❌ Error forcing recovery for {connection_id}: {e}")
    
    async def graceful_shutdown_all_connections(self, timeout: float = 30.0):
        """Gracefully shutdown all connections with timeout protection."""
        logger.info("🛑 Starting graceful shutdown of all connections...")
        
        shutdown_tasks = []
        
        for connection_id, metrics in self.connections.items():
            provider_ref = self.providers.get(metrics.provider_name)
            if provider_ref:
                provider = provider_ref()
                if provider and hasattr(provider, 'disconnect_streaming'):
                    task = asyncio.create_task(
                        TimeoutWrapper.execute(
                            provider.disconnect_streaming(),
                            timeout=10.0,
                            operation_name=f"shutdown {connection_id}"
                        )
                    )
                    shutdown_tasks.append(task)
        
        if shutdown_tasks:
            try:
                await asyncio.wait_for(
                    asyncio.gather(*shutdown_tasks, return_exceptions=True),
                    timeout=timeout
                )
                logger.info("✅ All connections shut down gracefully")
            except asyncio.TimeoutError:
                logger.warning("⚠️ Graceful shutdown timeout - some connections may not have closed properly")
        
        # Clear all connection states
        for metrics in self.connections.values():
            metrics.state = ConnectionState.DISCONNECTED

# Global instance
streaming_health_manager = StreamingHealthManager()
