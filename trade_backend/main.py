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
    SymbolRequest, OrderRequest, MultiLegOrderRequest, SymbolSearchResult, Account,
    CreateProviderInstanceRequest, UpdateProviderInstanceRequest, ProviderInstanceResponse,
    TestProviderConnectionRequest, TestProviderConnectionResponse
)
from .watchlist_models import (
    CreateWatchlistRequest, UpdateWatchlistRequest, AddSymbolRequest,
    SetActiveWatchlistRequest, WatchlistResponse, WatchlistsResponse,
    WatchlistSymbolsResponse, SearchWatchlistsRequest
)
from .provider_manager import provider_manager
from .provider_config import provider_config_manager
from .provider_types import get_provider_types
from .streaming_manager import streaming_manager
from .connection_manager import ConnectionManager
from .shutdown_manager import shutdown_manager
from .watchlist_manager import watchlist_manager
from .greeks_manager import greeks_manager
from .services.ivx_calculator import calculate_ivx_data
from .services.ivx_cache import ivx_cache
from .ivx_streaming_manager import IVxStreamingManager
from .strategies.api_endpoints import router as strategies_router
from .strategies.execution_engine import StrategyExecutionEngine
from .services.data_import.import_manager import import_manager
from .services.data_import.import_models import (
    ImportRequest, ImportJobStatus, ImportFilters, DateRange
)
from datetime import datetime, date

# Configure logging
logging.basicConfig(
    level=settings.log_level.upper(),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("trading_backend")

# Setup shutdown manager
shutdown_manager.setup_signal_handlers()
shutdown_manager.set_streaming_manager(streaming_manager)

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Enhanced application lifespan manager with proper cleanup.
    """
    # Startup
    logger.info("🚀 Starting enhanced trading backend...")
    try:
        # Connect to streaming manager
        logger.info("🔄 Connecting to streaming...")
        connected = await streaming_manager.connect()
        if connected:
            logger.info("✅ Streaming manager connected")
        else:
            logger.warning("⚠️ Streaming manager connection failed - continuing with limited functionality")
        
        # Start IVx streaming manager
        logger.info("🔄 Starting IVx streaming manager...")
        await ivx_streaming_manager.start()
        logger.info("✅ IVx streaming manager started")
        
        # Initialize Strategy Database
        logger.info("🔄 Initializing Strategy Database...")
        from .strategies.database import strategy_db_manager
        db_success = strategy_db_manager.initialize()
        if db_success:
            logger.info("✅ Strategy Database initialized")
        else:
            logger.error("❌ Strategy Database initialization failed")
        
        # Initialize Strategy Execution Engine
        logger.info("🔄 Initializing Strategy Execution Engine...")
        strategy_execution_engine = StrategyExecutionEngine()
        await strategy_execution_engine.initialize()
        
        # Make strategy engine available globally
        app.state.strategy_execution_engine = strategy_execution_engine
        logger.info("✅ Strategy Execution Engine initialized")
        
    except Exception as e:
        logger.error(f"❌ Failed to initialize streaming: {e}")
        # Don't raise - allow server to start even if streaming fails
    
    yield
    
    # Shutdown - this will be handled by the shutdown manager
    logger.info("🛑 Application lifespan shutdown - cleanup will be handled by shutdown manager")
    
    # Stop IVx streaming manager
    try:
        logger.info("🛑 Stopping IVx streaming manager...")
        await ivx_streaming_manager.stop()
        logger.info("✅ IVx streaming manager stopped")
    except Exception as e:
        logger.error(f"❌ Error stopping IVx streaming manager: {e}")

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
manager = ConnectionManager(streaming_manager, shutdown_manager)
shutdown_manager.set_connection_manager(manager)

# Initialize IVx streaming manager BEFORE starting streaming
ivx_streaming_manager = IVxStreamingManager(manager)
manager.set_ivx_streaming_manager(ivx_streaming_manager)

# CRITICAL: Set the global ivx_streaming_manager reference BEFORE streaming starts
from .ivx_streaming_manager import set_global_ivx_streaming_manager
set_global_ivx_streaming_manager(ivx_streaming_manager)

# === Register Strategy Router ===
app.include_router(strategies_router)

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

# === Setup Configuration Endpoints ===

@app.get("/setup/status", response_model=ApiResponse)
async def get_setup_status():
    """Check if mandatory routes are configured for the trading platform."""
    try:
        # Get current provider configuration
        config = provider_config_manager.get_config()
        
        # Define mandatory services that must be configured
        mandatory_services = [
            'trade_account',
            'options_chain',
            'historical_data',
            'symbol_lookup',
            'streaming_quotes'
        ]
        
        # The config might be directly the service routing object, or nested under service_routing
        service_routing = config.get('service_routing', config) if isinstance(config, dict) else {}
        
        # Check if we have service routing configuration
        if not service_routing or not isinstance(service_routing, dict):
            return ApiResponse(
                success=True,
                data={
                    "is_setup_complete": False,
                    "missing_mandatory_services": mandatory_services,
                    "configured_services": {},
                    "has_providers": False
                },
                message="No service routing configuration found"
            )
        
        # Check each mandatory service
        missing_services = []
        for service in mandatory_services:
            routed_provider = service_routing.get(service)
            if not routed_provider or routed_provider == "":
                missing_services.append(service)
        
        is_setup_complete = len(missing_services) == 0
        has_providers = bool(config.get('provider_instances', []))
        
        return ApiResponse(
            success=True,
            data={
                "is_setup_complete": is_setup_complete,
                "missing_mandatory_services": missing_services,
                "configured_services": service_routing,
                "has_providers": has_providers
            },
            message="Setup status checked successfully"
        )
        
    except Exception as e:
        logger.error(f"Error checking setup status: {e}")
        # If there's an error (like provider config not available), assume setup is incomplete
        return ApiResponse(
            success=True,
            data={
                "is_setup_complete": False,
                "missing_mandatory_services": [
                    'trade_account',
                    'options_chain', 
                    'historical_data',
                    'symbol_lookup',
                    'streaming_quotes'
                ],
                "configured_services": {},
                "has_providers": False
            },
            message=f"Error checking setup status: {str(e)}"
        )

# === Provider Configuration Endpoints ===

@app.get("/providers/config", response_model=ApiResponse)
async def get_provider_config():
    """Get current provider routing configuration."""
    config = provider_config_manager.get_config()
    return ApiResponse(success=True, data=config)

@app.put("/providers/config", response_model=ApiResponse)
async def update_provider_config(new_config: Dict[str, Any]):
    """Update provider routing configuration and automatically restart streaming."""
    try:
        # Update configuration
        provider_config_manager.update_config(new_config)
        
        # Check if streaming-related configuration changed
        streaming_keys = ["streaming_quotes", "streaming_greeks"]
        config_changed = any(key in new_config for key in streaming_keys)
        
        if config_changed:
            logger.info("🔄 Streaming configuration changed, restarting streaming connections...")
            
            # Restart streaming with new configuration
            restart_success = await streaming_manager.restart_with_new_config()
            
            if restart_success:
                return ApiResponse(
                    success=True, 
                    message="Provider config updated and streaming restarted successfully.",
                    data={"streaming_restarted": True}
                )
            else:
                return ApiResponse(
                    success=True,
                    message="Provider config updated, but streaming restart failed. Manual restart may be required.",
                    data={"streaming_restarted": False, "warning": "Streaming restart failed"}
                )
        else:
            return ApiResponse(
                success=True, 
                message="Provider config updated (no streaming restart needed).",
                data={"streaming_restarted": False}
            )
            
    except Exception as e:
        logger.error(f"Error updating provider config: {e}")
        return ApiResponse(
            success=False,
            message=f"Failed to update provider config: {str(e)}",
            data={"error": str(e)}
        )

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

# === Provider Instance Management Endpoints ===

@app.get("/providers/types", response_model=ApiResponse)
async def get_provider_types_endpoint():
    """Get available provider types and their field definitions."""
    try:
        provider_types = get_provider_types()
        return ApiResponse(
            success=True,
            data=provider_types,
            message="Retrieved provider type definitions"
        )
    except Exception as e:
        logger.error(f"Error getting provider types: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/providers/instances", response_model=ApiResponse)
async def get_provider_instances():
    """Get all provider instances with their status and credential information."""
    try:
        from .provider_types import get_visible_credentials, get_masked_credentials, get_default_credentials
        
        # Get all instances from credential store
        all_instances = provider_manager.credential_store.get_all_instances()
        
        # Convert to response format with enhanced credential information
        instances_data = {}
        for instance_id, instance_data in all_instances.items():
            provider_type = instance_data.get('provider_type')
            account_type = instance_data.get('account_type')
            
            instances_data[instance_id] = {
                'instance_id': instance_id,
                'active': instance_data.get('active', False),
                'provider_type': provider_type,
                'account_type': account_type,
                'display_name': instance_data.get('display_name'),
                'created_at': instance_data.get('created_at'),
                'updated_at': instance_data.get('updated_at'),
                'visible_credentials': get_visible_credentials(instance_data),
                'masked_credentials': get_masked_credentials(instance_data),
                'default_credentials': get_default_credentials(provider_type, account_type) if provider_type and account_type else {}
            }
        
        return ApiResponse(
            success=True,
            data=instances_data,
            message=f"Retrieved {len(instances_data)} provider instances"
        )
    except Exception as e:
        logger.error(f"Error getting provider instances: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/providers/instances", response_model=ApiResponse)
async def create_provider_instance(request: CreateProviderInstanceRequest):
    """Create a new provider instance."""
    try:
        from .provider_types import validate_credentials, apply_defaults
        import time
        
        # Validate credentials
        validation_errors = validate_credentials(
            request.provider_type, 
            request.account_type, 
            request.credentials
        )
        
        if validation_errors:
            raise HTTPException(
                status_code=400, 
                detail=f"Credential validation failed: {', '.join(validation_errors)}"
            )
        
        # Apply default values
        credentials_with_defaults = apply_defaults(
            request.provider_type, 
            request.account_type, 
            request.credentials
        )
        
        # Generate unique instance ID
        instance_id = provider_manager.credential_store.generate_instance_id(
            request.provider_type,
            request.account_type,
            request.display_name
        )
        
        # Add instance to credential store
        success = provider_manager.credential_store.add_instance(
            instance_id=instance_id,
            provider_type=request.provider_type,
            account_type=request.account_type,
            display_name=request.display_name,
            credentials=credentials_with_defaults
        )
        
        if not success:
            raise HTTPException(status_code=500, detail="Failed to create provider instance")
        
        # Reinitialize providers to include the new instance
        provider_manager._initialize_active_providers()
        
        return ApiResponse(
            success=True,
            data={"instance_id": instance_id},
            message=f"Provider instance '{request.display_name}' created successfully"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating provider instance: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.put("/providers/instances/{instance_id}", response_model=ApiResponse)
async def update_provider_instance(instance_id: str, request: UpdateProviderInstanceRequest):
    """Update an existing provider instance with smart credential handling."""
    try:
        from .provider_types import validate_credentials, apply_defaults, is_sensitive_field
        
        # Check if instance exists
        instance = provider_manager.credential_store.get_instance(instance_id)
        if not instance:
            raise HTTPException(status_code=404, detail="Provider instance not found")
        
        # Prepare updates
        updates = {}
        if request.display_name is not None:
            updates['display_name'] = request.display_name
        
        if request.credentials is not None:
            # Get existing credentials
            existing_credentials = instance.get('credentials', {})
            
            # Smart credential merging - only update changed fields
            merged_credentials = existing_credentials.copy()
            
            # Process each credential field
            for field_name, field_value in request.credentials.items():
                # Skip empty sensitive fields (they weren't changed)
                if is_sensitive_field(field_name) and not field_value:
                    continue
                
                # Skip masked placeholder values
                if field_value == '••••••••':
                    continue
                
                # Update the field
                merged_credentials[field_name] = field_value
            
            # Validate merged credentials
            validation_errors = validate_credentials(
                instance['provider_type'],
                instance['account_type'],
                merged_credentials
            )
            
            if validation_errors:
                raise HTTPException(
                    status_code=400,
                    detail=f"Credential validation failed: {', '.join(validation_errors)}"
                )
            
            # Apply defaults to merged credentials
            credentials_with_defaults = apply_defaults(
                instance['provider_type'],
                instance['account_type'],
                merged_credentials
            )
            updates['credentials'] = credentials_with_defaults
        
        # Update instance
        success = provider_manager.credential_store.update_instance(instance_id, **updates)
        
        if not success:
            raise HTTPException(status_code=500, detail="Failed to update provider instance")
        
        # Reinitialize providers if credentials were updated
        if request.credentials is not None:
            provider_manager._initialize_active_providers()
        
        return ApiResponse(
            success=True,
            data={"instance_id": instance_id},
            message="Provider instance updated successfully"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error updating provider instance {instance_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.put("/providers/instances/{instance_id}/toggle", response_model=ApiResponse)
async def toggle_provider_instance(instance_id: str):
    """Activate/deactivate a provider instance."""
    try:
        # Toggle the instance
        new_active_state = provider_manager.credential_store.toggle_instance(instance_id)
        
        if new_active_state is None:
            raise HTTPException(status_code=404, detail="Provider instance not found")
        
        # Reinitialize providers to reflect the change
        provider_manager._initialize_active_providers()
        
        action = "activated" if new_active_state else "deactivated"
        return ApiResponse(
            success=True,
            data={"instance_id": instance_id, "active": new_active_state},
            message=f"Provider instance {action} successfully"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error toggling provider instance {instance_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/providers/instances/{instance_id}", response_model=ApiResponse)
async def delete_provider_instance(instance_id: str):
    """Delete a provider instance."""
    try:
        # Check if instance exists
        instance = provider_manager.credential_store.get_instance(instance_id)
        if not instance:
            raise HTTPException(status_code=404, detail="Provider instance not found")
        
        # Delete the instance
        success = provider_manager.credential_store.delete_instance(instance_id)
        
        if not success:
            raise HTTPException(status_code=500, detail="Failed to delete provider instance")
        
        # Reinitialize providers to remove the deleted instance
        provider_manager._initialize_active_providers()
        
        return ApiResponse(
            success=True,
            data={"instance_id": instance_id},
            message="Provider instance deleted successfully"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting provider instance {instance_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/providers/instances/test", response_model=ApiResponse)
async def test_provider_connection(request: TestProviderConnectionRequest):
    """Test a provider connection without saving credentials."""
    try:
        # Test the connection using provider manager's new method
        result = await provider_manager.test_provider_credentials(
            request.provider_type,
            request.account_type,
            request.credentials
        )
        
        return ApiResponse(
            success=result['success'],
            data=result,
            message=result['message']
        )
        
    except Exception as e:
        logger.error(f"Error testing provider connection: {e}")
        return ApiResponse(
            success=False,
            data={'error': str(e)},
            message=f"Connection test failed: {str(e)}"
        )

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

@app.get("/options_chain_basic", response_model=ApiResponse)
async def get_options_chain_basic(
    symbol: str, 
    expiry: str, 
    underlying_price: Optional[float] = None,
    strike_count: int = 20,
    type: Optional[str] = None,
    underlying_symbol: Optional[str] = None
):
    """Fast options chain loading - basic data only, ATM-focused by strike count, no price data."""
    try:
        contracts = await provider_manager.get_options_chain_basic(
            symbol, expiry, underlying_price, strike_count, type, underlying_symbol
        )
        
        # Convert to dict format and remove price data for performance
        contracts_data = []
        for contract in contracts:
            contract_dict = contract.dict()
            # Remove price-related fields - UI gets these from streaming
            price_fields = ['price', 'bid', 'ask', 'last', 'mark', 'bid_size', 'ask_size', 'volume', 'open_interest']
            for field in price_fields:
                contract_dict.pop(field, None)
            contracts_data.append(contract_dict)
        
        return ApiResponse(
            success=True,
            data=contracts_data,
            message=f"Retrieved {len(contracts)} basic option contracts ({strike_count} strikes around ATM, no price data)"
        )
    except Exception as e:
        logger.error(f"Error getting basic options chain: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/options_greeks", response_model=ApiResponse)
async def get_options_greeks(symbols: str):
    """Get Greeks for specific option symbols (comma-separated) using smart Greeks manager."""
    try:
        symbol_list = [s.strip() for s in symbols.split(',') if s.strip()]
        
        if not symbol_list:
            return ApiResponse(
                success=True,
                data={},
                message="No symbols provided"
            )
        
        # Use the new Greeks manager for smart routing
        greeks_data = await greeks_manager.get_greeks_batch(symbol_list)
        
        return ApiResponse(
            success=True,
            data={
                "greeks": greeks_data,
                "strategy": greeks_manager.get_current_strategy(),
                "provider_info": greeks_manager.get_provider_info()
            },
            message=f"Retrieved Greeks for {len(symbol_list)} option symbols using {greeks_manager.get_current_strategy()} strategy"
        )
    except Exception as e:
        logger.error(f"Error getting options Greeks: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/greeks/{symbols}", response_model=ApiResponse)
async def get_greeks_batch(symbols: str):
    """Get Greeks for multiple option symbols using smart Greeks manager."""
    try:
        symbol_list = symbols.split(",")
        greeks_data = await greeks_manager.get_greeks_batch(symbol_list)
        
        return ApiResponse(
            success=True,
            data=greeks_data,
            message=f"Retrieved Greeks for {len(symbol_list)} symbols using {greeks_manager.get_current_strategy()} strategy",
            metadata={
                "strategy": greeks_manager.get_current_strategy(),
                "provider_info": greeks_manager.get_provider_info()
            }
        )
    except Exception as e:
        logger.error(f"Error in get_greeks_batch: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/provider-capabilities", response_model=ApiResponse)
async def get_provider_capabilities():
    """Get provider capabilities matrix including Greeks support."""
    try:
        from .provider_config import PROVIDER_CAPABILITIES
        
        # Enhanced capabilities with Greeks information
        enhanced_capabilities = {}
        for provider, caps in PROVIDER_CAPABILITIES.items():
            enhanced_capabilities[provider] = {
                **caps,
                "greeks_support": {
                    "api_greeks": "greeks" in caps.get("capabilities", {}).get("rest", []),
                    "streaming_greeks": "streaming_greeks" in caps.get("capabilities", {}).get("streaming", [])
                }
            }
        
        return ApiResponse(
            success=True,
            data=enhanced_capabilities,
            message="Retrieved provider capabilities with Greeks support information"
        )
    except Exception as e:
        logger.error(f"Error getting provider capabilities: {e}")
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
    """Smart options chain with configurable loading - no price data."""
    try:
        contracts = await provider_manager.get_options_chain_smart(
            symbol, expiry, underlying_price, atm_range, include_greeks, strikes_only
        )
        
        # Convert to dict format and remove price data for performance
        contracts_data = []
        for contract in contracts:
            contract_dict = contract.dict()
            # Remove price-related fields - UI gets these from streaming
            price_fields = ['price', 'bid', 'ask', 'last', 'mark', 'bid_size', 'ask_size', 'volume', 'open_interest']
            for field in price_fields:
                contract_dict.pop(field, None)
            contracts_data.append(contract_dict)
        
        return ApiResponse(
            success=True,
            data=contracts_data,
            message=f"Retrieved {len(contracts)} smart option contracts (no price data)"
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
    """Get current WebSocket subscription status with health information."""
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

@app.get("/health/streaming", response_model=ApiResponse)
async def get_streaming_health():
    """Get detailed streaming health statistics."""
    try:
        health_stats = streaming_manager.get_health_stats()
        connection_stats = manager.get_stats()
        
        return ApiResponse(
            success=True,
            data={
                "streaming_health": health_stats,
                "connection_health": connection_stats,
                "shutdown_in_progress": shutdown_manager.is_shutting_down()
            },
            message="Retrieved streaming health statistics"
        )
    except Exception as e:
        logger.error(f"Error getting streaming health: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/debug/ivx_cache", response_model=ApiResponse)
async def get_ivx_cache_status():
    """Get current IVX cache status for debugging."""
    try:
        cache_info = ivx_cache.get_cache_info()
        ivx_manager_status = ivx_streaming_manager.get_status() if ivx_streaming_manager else None
        
        return ApiResponse(
            success=True,
            data={
                "cache_info": cache_info,
                "ivx_manager_status": ivx_manager_status,
                "cache_behavior": {
                    "ttl_seconds": cache_info["ttl_seconds"],
                    "description": "Cache stores IVX data for 5 minutes. On first request, data is calculated and cached. Subsequent requests within 5 minutes return cached data immediately. After 5 minutes, cache expires and new calculation is triggered."
                }
            },
            message=f"Retrieved IVX cache status - {cache_info['total_cached_symbols']} symbols cached"
        )
    except Exception as e:
        logger.error(f"Error getting IVX cache status: {e}")
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
    """Get enhanced positions with hierarchical grouping (Symbol -> Strategies -> Legs)."""
    try:
        # Try to get enhanced hierarchical positions first
        try:
            result = await provider_manager.get_positions_enhanced()
            if isinstance(result, dict) and result.get("enhanced"):
                # New hierarchical structure
                return ApiResponse(
                    success=True,
                    data=result,
                    message=f"Retrieved {len(result.get('symbol_groups', []))} symbol groups with hierarchical structure"
                )
            elif isinstance(result, list):
                # Old position groups structure - convert for backward compatibility
                return ApiResponse(
                    success=True,
                    data={
                        "position_groups": [group.dict() for group in result],
                        "total_groups": len(result),
                        "enhanced": True
                    },
                    message=f"Retrieved {len(result)} position groups with strategy detection"
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

@app.post("/orders/preview", response_model=ApiResponse)
async def preview_order(order_request: MultiLegOrderRequest):
    """Preview an order to get cost estimates and validation."""
    try:
        preview_data = await provider_manager.preview_order(order_request.dict())
        if preview_data:
            success = preview_data.get('status') == 'ok'
            message = "Order preview generated successfully." if success else "Order preview failed."
            return ApiResponse(
                success=success,
                data=preview_data,
                message=message
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to generate order preview.")
    except Exception as e:
        logger.error(f"Error previewing order: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/orders/single-leg", response_model=ApiResponse)
async def place_single_leg_order(order_request: OrderRequest):
    """Place a new single-leg option order."""
    try:
        order = await provider_manager.place_order(order_request.dict())
        if order:
            return ApiResponse(
                success=True,
                data=order.dict(),
                message="Single-leg order placed successfully."
            )
        else:
            raise HTTPException(status_code=500, detail="Failed to place single-leg order.")
    except Exception as e:
        logger.error(f"Error placing single-leg order: {e}")
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

# === Watchlist Endpoints ===

@app.get("/watchlists", response_model=ApiResponse)
async def get_watchlists():
    """Get all watchlists with metadata."""
    try:
        data = watchlist_manager.get_all_watchlists()
        return ApiResponse(
            success=True,
            data=data,
            message=f"Retrieved {data['total_watchlists']} watchlists"
        )
    except Exception as e:
        logger.error(f"Error getting watchlists: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/watchlists/active", response_model=ApiResponse)
async def get_active_watchlist():
    """Get the currently active watchlist."""
    try:
        active_watchlist = watchlist_manager.get_active_watchlist()
        active_watchlist_id = watchlist_manager.get_active_watchlist_id()
        
        if not active_watchlist:
            raise HTTPException(status_code=404, detail="No active watchlist found")
        
        return ApiResponse(
            success=True,
            data={
                "active_watchlist_id": active_watchlist_id,
                "active_watchlist": active_watchlist
            },
            message="Retrieved active watchlist"
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting active watchlist: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.put("/watchlists/active", response_model=ApiResponse)
async def set_active_watchlist(request: SetActiveWatchlistRequest):
    """Set the active watchlist."""
    try:
        set_active = watchlist_manager.set_active_watchlist(request.watchlist_id)
        
        # Get the active watchlist
        active_watchlist = watchlist_manager.get_active_watchlist()
        
        return ApiResponse(
            success=True,
            data={
                "active_watchlist_id": request.watchlist_id,
                "active_watchlist": active_watchlist
            },
            message=f"Active watchlist set to '{request.watchlist_id}'"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error setting active watchlist: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/watchlists/{watchlist_id}", response_model=ApiResponse)
async def get_watchlist(watchlist_id: str):
    """Get a specific watchlist by ID."""
    try:
        watchlist = watchlist_manager.get_watchlist(watchlist_id)
        if not watchlist:
            raise HTTPException(status_code=404, detail=f"Watchlist '{watchlist_id}' not found")
        
        return ApiResponse(
            success=True,
            data=watchlist,
            message=f"Retrieved watchlist '{watchlist_id}'"
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting watchlist {watchlist_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/watchlists", response_model=ApiResponse)
async def create_watchlist(request: CreateWatchlistRequest):
    """Create a new watchlist."""
    try:
        watchlist_id = watchlist_manager.create_watchlist(
            name=request.name,
            symbols=request.symbols
        )
        
        # Get the created watchlist to return
        watchlist = watchlist_manager.get_watchlist(watchlist_id)
        
        return ApiResponse(
            success=True,
            data={
                "watchlist_id": watchlist_id,
                "watchlist": watchlist
            },
            message=f"Watchlist '{request.name}' created successfully"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error creating watchlist: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.put("/watchlists/{watchlist_id}", response_model=ApiResponse)
async def update_watchlist(watchlist_id: str, request: UpdateWatchlistRequest):
    """Update an existing watchlist."""
    try:
        updated = watchlist_manager.update_watchlist(
            watchlist_id=watchlist_id,
            name=request.name,
            symbols=request.symbols
        )
        
        if not updated:
            return ApiResponse(
                success=True,
                data={"watchlist_id": watchlist_id},
                message="No changes made to watchlist"
            )
        
        # Get the updated watchlist to return
        watchlist = watchlist_manager.get_watchlist(watchlist_id)
        
        return ApiResponse(
            success=True,
            data={
                "watchlist_id": watchlist_id,
                "watchlist": watchlist
            },
            message=f"Watchlist '{watchlist_id}' updated successfully"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error updating watchlist {watchlist_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/watchlists/{watchlist_id}", response_model=ApiResponse)
async def delete_watchlist(watchlist_id: str):
    """Delete a watchlist."""
    try:
        deleted = watchlist_manager.delete_watchlist(watchlist_id)
        
        return ApiResponse(
            success=True,
            data={"watchlist_id": watchlist_id, "deleted": deleted},
            message=f"Watchlist '{watchlist_id}' deleted successfully"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error deleting watchlist {watchlist_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/watchlists/{watchlist_id}/symbols", response_model=ApiResponse)
async def add_symbol_to_watchlist(watchlist_id: str, request: AddSymbolRequest):
    """Add a symbol to a watchlist."""
    try:
        # Basic symbol validation
        if not watchlist_manager.validate_symbol(request.symbol):
            raise HTTPException(status_code=400, detail=f"Invalid symbol: {request.symbol}")
        
        added = watchlist_manager.add_symbol(watchlist_id, request.symbol)
        
        # Get updated watchlist
        watchlist = watchlist_manager.get_watchlist(watchlist_id)
        
        return ApiResponse(
            success=True,
            data={
                "watchlist_id": watchlist_id,
                "symbol": request.symbol,
                "watchlist": watchlist
            },
            message=f"Symbol '{request.symbol}' added to watchlist"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error adding symbol to watchlist {watchlist_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/watchlists/{watchlist_id}/symbols/{symbol}", response_model=ApiResponse)
async def remove_symbol_from_watchlist(watchlist_id: str, symbol: str):
    """Remove a symbol from a watchlist."""
    try:
        removed = watchlist_manager.remove_symbol(watchlist_id, symbol)
        
        # Get updated watchlist
        watchlist = watchlist_manager.get_watchlist(watchlist_id)
        
        return ApiResponse(
            success=True,
            data={
                "watchlist_id": watchlist_id,
                "symbol": symbol,
                "watchlist": watchlist
            },
            message=f"Symbol '{symbol}' removed from watchlist"
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error removing symbol from watchlist {watchlist_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/watchlists/search", response_model=ApiResponse)
async def search_watchlists(request: SearchWatchlistsRequest):
    """Search watchlists by name or symbols."""
    try:
        results = watchlist_manager.search_watchlists(request.query)
        
        return ApiResponse(
            success=True,
            data={
                "query": request.query,
                "results": results,
                "total_results": len(results)
            },
            message=f"Found {len(results)} watchlists matching '{request.query}'"
        )
    except Exception as e:
        logger.error(f"Error searching watchlists: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/watchlists/symbols/all", response_model=ApiResponse)
async def get_all_watchlist_symbols():
    """Get all unique symbols across all watchlists."""
    try:
        symbols = watchlist_manager.get_all_symbols()
        
        return ApiResponse(
            success=True,
            data={
                "symbols": symbols,
                "total_symbols": len(symbols)
            },
            message=f"Retrieved {len(symbols)} unique symbols from all watchlists"
        )
    except Exception as e:
        logger.error(f"Error getting all watchlist symbols: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Data Import Endpoints ===

@app.get("/api/data-import/files", response_model=ApiResponse)
async def list_dbn_files():
    """List available DBN files without metadata processing for fast loading."""
    try:
        files_info = await import_manager.list_dbn_files_only()
        
        return ApiResponse(
            success=True,
            data=files_info,
            message=f"Found {len(files_info)} DBN files"
        )
    except Exception as e:
        logger.error(f"Error listing DBN files: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/imported-data", response_model=ApiResponse)
async def get_imported_data():
    """Get list of imported datasets by symbol."""
    try:
        # Get symbol-level data instead of asset-type aggregates
        symbol_datasets = import_manager.get_symbol_level_data()
        
        return ApiResponse(
            success=True,
            data=symbol_datasets,
            message=f"Retrieved {len(symbol_datasets)} imported symbol datasets"
        )
    except Exception as e:
        logger.error(f"Error getting imported data: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/metadata/{filename}", response_model=ApiResponse)
async def get_file_metadata_api(filename: str, force_refresh: bool = False):
    """Get detailed metadata for a specific DBN file (API endpoint)."""
    try:
        metadata = await import_manager.get_file_metadata(filename, force_refresh)
        
        return ApiResponse(
            success=True,
            data=metadata.dict(),
            message=f"Retrieved metadata for {filename}"
        )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting file metadata for {filename}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/api/data-import/jobs", response_model=ApiResponse)
async def start_import_job_api(request: ImportRequest):
    """Start a new data import job (API endpoint)."""
    try:
        job_id = await import_manager.start_import_job(request)
        
        return ApiResponse(
            success=True,
            data={
                "job_id": job_id,
                "filename": request.filename,
                "status": "pending"
            },
            message=f"Import job started for {request.filename}"
        )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error starting import job: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/jobs/{job_id}/status", response_model=ApiResponse)
async def get_import_job_status_api(job_id: str):
    """Get status and progress of an import job (API endpoint)."""
    try:
        job_info = await import_manager.get_job_status(job_id)
        
        return ApiResponse(
            success=True,
            data=job_info.model_dump(),
            message=f"Retrieved status for job {job_id}"
        )
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting job status for {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/api/data-import/jobs/{job_id}", response_model=ApiResponse)
async def cancel_import_job_api(job_id: str):
    """Cancel a running import job (API endpoint)."""
    try:
        cancelled = await import_manager.cancel_import_job(job_id)
        
        if cancelled:
            return ApiResponse(
                success=True,
                data={"job_id": job_id, "cancelled": True},
                message=f"Import job {job_id} cancelled successfully"
            )
        else:
            return ApiResponse(
                success=False,
                data={"job_id": job_id, "cancelled": False},
                message=f"Could not cancel job {job_id} (may not be active)"
            )
    except Exception as e:
        logger.error(f"Error cancelling import job {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/import/files", response_model=ApiResponse)
async def list_dbn_files_legacy():
    """List available DBN files with metadata information (legacy endpoint)."""
    try:
        files_info = await import_manager.list_available_files()
        
        # Convert to dict format for JSON serialization
        files_data = []
        for file_info in files_info:
            file_dict = file_info.dict()
            files_data.append(file_dict)
        
        return ApiResponse(
            success=True,
            data={
                "files": files_data,
                "total_files": len(files_data)
            },
            message=f"Found {len(files_data)} DBN files"
        )
    except Exception as e:
        logger.error(f"Error listing DBN files: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/import/metadata/{filename}", response_model=ApiResponse)
async def get_file_metadata(filename: str, force_refresh: bool = False):
    """Get detailed metadata for a specific DBN file."""
    try:
        metadata = await import_manager.get_file_metadata(filename, force_refresh)
        
        return ApiResponse(
            success=True,
            data=metadata.dict(),
            message=f"Retrieved metadata for {filename}"
        )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting file metadata for {filename}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/data/import/start", response_model=ApiResponse)
async def start_import_job(request: ImportRequest):
    """Start a new data import job."""
    try:
        job_id = await import_manager.start_import_job(request)
        
        return ApiResponse(
            success=True,
            data={
                "job_id": job_id,
                "filename": request.filename,
                "status": "pending"
            },
            message=f"Import job started for {request.filename}"
        )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error starting import job: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/import/jobs/{job_id}", response_model=ApiResponse)
async def get_import_job_status(job_id: str):
    """Get status and progress of an import job."""
    try:
        job_info = await import_manager.get_job_status(job_id)
        
        return ApiResponse(
            success=True,
            data=job_info.dict(),
            message=f"Retrieved status for job {job_id}"
        )
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting job status for {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/import/jobs", response_model=ApiResponse)
async def list_import_jobs(
    status: Optional[str] = None,
    limit: int = 50
):
    """List import jobs with optional status filtering."""
    try:
        # Convert string status to enum if provided
        status_filter = None
        if status:
            try:
                status_filter = ImportJobStatus(status.lower())
            except ValueError:
                raise HTTPException(status_code=400, detail=f"Invalid status: {status}")
        
        jobs = await import_manager.list_import_jobs(status_filter, limit)
        
        # Convert to dict format
        jobs_data = [job.dict() for job in jobs]
        
        return ApiResponse(
            success=True,
            data={
                "jobs": jobs_data,
                "total_jobs": len(jobs_data),
                "status_filter": status,
                "limit": limit
            },
            message=f"Retrieved {len(jobs_data)} import jobs"
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error listing import jobs: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/data/import/jobs/{job_id}", response_model=ApiResponse)
async def cancel_import_job(job_id: str):
    """Cancel a running import job."""
    try:
        cancelled = await import_manager.cancel_import_job(job_id)
        
        if cancelled:
            return ApiResponse(
                success=True,
                data={"job_id": job_id, "cancelled": True},
                message=f"Import job {job_id} cancelled successfully"
            )
        else:
            return ApiResponse(
                success=False,
                data={"job_id": job_id, "cancelled": False},
                message=f"Could not cancel job {job_id} (may not be active)"
            )
    except Exception as e:
        logger.error(f"Error cancelling import job {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/import/summary", response_model=ApiResponse)
async def get_import_summary():
    """Get summary statistics for import operations."""
    try:
        summary = await import_manager.get_import_summary()
        
        return ApiResponse(
            success=True,
            data=summary.dict(),
            message="Retrieved import summary statistics"
        )
    except Exception as e:
        logger.error(f"Error getting import summary: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/data/storage/stats", response_model=ApiResponse)
async def get_storage_stats():
    """Get comprehensive storage statistics for data import system."""
    try:
        stats = import_manager.get_storage_stats()
        
        return ApiResponse(
            success=True,
            data=stats,
            message="Retrieved storage statistics"
        )
    except Exception as e:
        logger.error(f"Error getting storage stats: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/data/cache/clear", response_model=ApiResponse)
async def clear_metadata_cache(filename: Optional[str] = None):
    """Clear metadata cache for all files or a specific file."""
    try:
        from .services.data_import.metadata_extractor import metadata_extractor
        from pathlib import Path
        
        if filename:
            file_path = import_manager.dbn_files_dir / filename
            if not file_path.exists():
                raise HTTPException(status_code=404, detail=f"File not found: {filename}")
            metadata_extractor.clear_cache(file_path)
            message = f"Cleared cache for {filename}"
        else:
            metadata_extractor.clear_cache()
            message = "Cleared all metadata cache"
        
        return ApiResponse(
            success=True,
            data={"filename": filename, "cleared": True},
            message=message
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error clearing metadata cache: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/api/data-import/cache", response_model=ApiResponse)
async def clear_api_metadata_cache():
    """Clear metadata cache through API endpoint."""
    try:
        from .services.data_import.metadata_extractor import metadata_extractor
        metadata_extractor.clear_cache()
        
        return ApiResponse(
            success=True,
            data={"cleared": True},
            message="Cleared all metadata cache"
        )
    except Exception as e:
        logger.error(f"Error clearing metadata cache: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === WebSocket Endpoint ===

@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """WebSocket endpoint with better error handling and shutdown awareness."""
    await manager.connect(websocket)
    try:
        while not shutdown_manager.is_shutting_down():
            try:
                # Wait for client messages with timeout to allow shutdown checking
                data = await asyncio.wait_for(websocket.receive_json(), timeout=1.0)
                
                # Validate data before processing
                if data is None:
                    logger.warning("⚠️ Received None data from WebSocket client")
                    continue
                
                if not isinstance(data, dict):
                    logger.warning(f"⚠️ Received non-dict data from WebSocket client: {type(data)}")
                    continue
                
                # Handle message through enhanced connection manager
                await manager.handle_websocket_message(websocket, data)
                
            except asyncio.TimeoutError:
                # Timeout is normal - allows us to check shutdown status
                continue
            except WebSocketDisconnect:
                logger.info("🔌 WebSocket client disconnected")
                break
            except Exception as e:
                logger.error(f"❌ Error processing WebSocket message: {e}")
                try:
                    await websocket.send_json({
                        "type": "error",
                        "message": f"Error processing message: {str(e)}"
                    })
                except:
                    # If we can't send error message, connection is probably dead
                    break
                    
    except WebSocketDisconnect:
        logger.info("🔌 WebSocket disconnected")
    except Exception as e:
        logger.error(f"❌ WebSocket error: {e}")
    finally:
        await manager.disconnect(websocket)

# === Main Execution ===

if __name__ == "__main__":
    uvicorn.run(
        "trading_backend.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.reload
    )
