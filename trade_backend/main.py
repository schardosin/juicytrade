import os
import debugpy
import asyncio
import uvicorn
import logging
import time
from contextlib import asynccontextmanager

# Check for debug mode at the very start
if os.environ.get("DEBUG_STRATEGY"):
    try:
        debug_port = 5678
        debugpy.listen(("0.0.0.0", debug_port))
        # The logging is configured later, so we use print here
        print(f"🚀 Strategy Debug Mode is ON. Waiting for debugger to attach on port {debug_port}...")
        debugpy.wait_for_client()
        print("✅ Debugger attached!")
    except Exception as e:
        print(f"❌ Error starting debugpy: {e}")
from typing import Dict, List, Optional, Any

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect, BackgroundTasks, Request
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .auth.config import auth_config
from .auth.middleware import AuthenticationMiddleware, require_auth
from .auth.endpoints import auth_router
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
from .strategies.api_endpoints import router as strategies_router
from .strategies.execution_engine import StrategyExecutionEngine
from .services.data_import.import_manager import import_manager
from .services.data_import.import_models import (
    ImportRequest, ImportJobStatus, ImportFilters, DateRange,
    ImportFileType, CSVFormat, MultiFileImportRequest, ImportQueueStatus
)
from .services.data_aggregation import (
    DataAggregationService, AggregationRequest, AggregatedData, TimeFrame,
    get_aggregation_service
)
from datetime import datetime, date, timezone

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

# Create FastAPI app
app = FastAPI(
    title="Trading Backend API",
    description="Multi-provider trading backend with standardized API",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware with environment-based configuration
cors_origins = os.getenv("CORS_ALLOW_ORIGINS", "http://localhost:3001")
allowed_origins = [origin.strip() for origin in cors_origins.split(",") if origin.strip()]

app.add_middleware(
    CORSMiddleware,
    allow_origins=allowed_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

logger.info(f"🌐 CORS configured with allowed origins: {allowed_origins}")

# Add authentication middleware (only if enabled)
if auth_config.is_enabled():
    logger.info(f"🔐 Authentication enabled: {auth_config.method.value}")
    app.add_middleware(AuthenticationMiddleware)
else:
    logger.info("🔓 Authentication disabled")

# WebSocket connection manager
manager = ConnectionManager(streaming_manager, shutdown_manager)
shutdown_manager.set_connection_manager(manager)

# === Register Authentication Router ===
app.include_router(auth_router)

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
                data = {symbol_list[0]: quote.model_dump() if quote else None}
            else:
                # This part needs to be adapted for the manager if we want multi-symbol quotes
                # For now, we'll just loop, but a batch method in the manager would be better.
                data = {}
                for symbol in symbol_list:
                    quote = await provider_manager.get_stock_quote(symbol)
                    data[symbol] = quote.model_dump() if quote else None
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
            contract_dict = contract.model_dump()
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
        contracts_data = [contract.model_dump() for contract in contracts]
        
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
        
        return ApiResponse(
            success=True,
            data={
                "cache_info": cache_info,
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

@app.get("/api/ivx/{symbol}", response_model=ApiResponse)
async def get_ivx_data(symbol: str):
    """Get IVx data for a symbol across all available expirations."""
    try:
        logger.info(f"📊 API request for IVx data for {symbol}")
        
        # Check cache first
        cached_data = ivx_cache.get(symbol)
        if cached_data:
            logger.info(f"📦 Returning cached IVx data for {symbol} ({len(cached_data)} expirations)")
            return ApiResponse(
                success=True,
                data={
                    "symbol": symbol,
                    "expirations": cached_data,
                    "cached": True,
                    "calculation_time": None
                },
                message=f"Retrieved cached IVx data for {symbol} ({len(cached_data)} expirations)"
            )
        
        # Calculate fresh data
        logger.info(f"🔄 Calculating fresh IVx data for {symbol}")
        start_time = time.time()
        
        # Get the options provider (same as streaming version)
        provider = provider_manager._get_provider("options_chain")
        if not provider:
            return ApiResponse(
                success=False,
                data={"symbol": symbol, "expirations": [], "error": "No options provider available"},
                message=f"No options provider available for {symbol}"
            )
        
        # Get all available expirations
        expiration_dates = await provider.get_expiration_dates(symbol)
        if not expiration_dates:
            return ApiResponse(
                success=False,
                data={"symbol": symbol, "expirations": [], "error": "No expiration dates available"},
                message=f"No expiration dates available for {symbol}"
            )
        
        # Get underlying price (with fallback for indices like SPX)
        logger.info(f"🔍 Getting stock quote for {symbol}")
        quote = await provider.get_stock_quote(symbol)
        logger.info(f"🔍 Quote result: {quote}")
        
        underlying_price = None
        if quote is not None:
            # Try to get price from last, then bid/ask midpoint, then individual bid or ask
            if quote.last is not None:
                underlying_price = quote.last
                logger.info(f"📈 Using last price for {symbol}: ${underlying_price}")
            elif quote.bid is not None and quote.ask is not None:
                underlying_price = (quote.bid + quote.ask) / 2
                logger.info(f"📈 Using bid/ask midpoint for {symbol}: ${underlying_price} (bid: ${quote.bid}, ask: ${quote.ask})")
            elif quote.bid is not None:
                underlying_price = quote.bid
                logger.info(f"📈 Using bid price for {symbol}: ${underlying_price}")
            elif quote.ask is not None:
                underlying_price = quote.ask
                logger.info(f"📈 Using ask price for {symbol}: ${underlying_price}")
        
        # Fallback for indices like SPX where direct quotes might fail
        if underlying_price is None:
            logger.warning(f"⚠️ Direct quote failed for {symbol}, trying fallback via options chain")
            try:
                # Get the first expiration to find underlying price from options chain
                first_expiry = expiration_dates[0]['date'] if expiration_dates else None
                if first_expiry:
                    # Use the expiration symbol for options chain lookup
                    first_exp_symbol = expiration_dates[0].get('symbol', symbol)
                    logger.info(f"� Trying options chain fallback for {symbol} using expiry {first_expiry} and symbol {first_exp_symbol}")
                    
                    fallback_options = await provider.get_options_chain_basic(
                        symbol=first_exp_symbol,
                        expiry=first_expiry,
                        strike_count=5,
                        underlying_symbol=symbol
                    )
                    
                    if fallback_options and len(fallback_options) > 0:
                        # Some providers include underlying price in options response
                        if hasattr(fallback_options[0], 'underlying_price') and fallback_options[0].underlying_price:
                            underlying_price = fallback_options[0].underlying_price
                            logger.info(f"📈 Using underlying price from options chain for {symbol}: ${underlying_price}")
                        else:
                            # Estimate from ATM strikes - find middle strike as approximation
                            strikes = sorted([opt.strike_price for opt in fallback_options])
                            if strikes:
                                underlying_price = strikes[len(strikes) // 2]  # Use middle strike as approximation
                                logger.info(f"📈 Estimated underlying price from strikes for {symbol}: ${underlying_price}")
            except Exception as e:
                logger.error(f"❌ Options chain fallback failed for {symbol}: {e}")
        
        if underlying_price is None:
            logger.error(f"❌ No valid price available for {symbol}: quote={quote}")
            return ApiResponse(
                success=False,
                data={"symbol": symbol, "expirations": [], "error": "No valid price available"},
                message=f"No valid price available for {symbol}"
            )
        
        # Calculate IVx for each expiration (PARALLEL PROCESSING - same as streaming)
        ivx_results = []
        logger.info(f"🔄 Processing {len(expiration_dates)} expiration dates for {symbol}")
        
        # Use the same parallel processing approach as streaming
        import asyncio
        
        # Worker function to process a single expiration (extracted from streaming logic)
        async def process_single_expiration(expiration_date):
            try:
                # Extract date string and other info from expiration dict if needed
                if isinstance(expiration_date, dict):
                    date_str = expiration_date.get('date')
                    exp_type = expiration_date.get('type')
                    date_str_symbol = expiration_date.get('symbol', symbol)
                    if not date_str:
                        logger.warning(f"⚠️ Expiration dict missing 'date' field: {expiration_date}")
                        return None
                else:
                    date_str = expiration_date
                    exp_type = None
                    date_str_symbol = symbol
                
                logger.info(f"📅 Processing expiration: {date_str}")
                
                # Get basic options chain (same as streaming - step 1)
                options_chain = await provider.get_options_chain_basic(
                    symbol=date_str_symbol,
                    expiry=date_str,
                    underlying_price=underlying_price,
                    strike_count=20,
                    type=exp_type,
                    underlying_symbol=symbol
                )
                
                if not options_chain:
                    logger.warning(f"No options chain for {symbol} {date_str}")
                    return None

                # Find ATM strike (same as streaming - step 2)
                atm_strike = None
                min_diff = float('inf')
                for contract in options_chain:
                    diff = abs(contract.strike_price - underlying_price)
                    if diff < min_diff:
                        min_diff = diff
                        atm_strike = contract.strike_price
                
                if not atm_strike:
                    logger.warning(f"Could not find ATM strike for {symbol} {date_str}")
                    return None
                
                # Find ATM call and put contracts (same as streaming - step 3)
                atm_call = next((c for c in options_chain if c.strike_price == atm_strike and c.type == 'call'), None)
                atm_put = next((c for c in options_chain if c.strike_price == atm_strike and c.type == 'put'), None)
                
                if not atm_call or not atm_put:
                    logger.warning(f"Could not find ATM call/put for {symbol} {date_str} at strike {atm_strike}")
                    return None
                
                # Get live greeks/IV data for ATM options (same as streaming - step 4)
                symbols_to_fetch = [atm_call.symbol, atm_put.symbol]
                
                try:
                    greeks_results_dict = await provider.get_streaming_greeks_batch(symbols_to_fetch)
                except asyncio.TimeoutError:
                    logger.warning(f"Timeout fetching greeks for {symbol} {date_str}")
                    return None
                except Exception as e:
                    logger.error(f"Error fetching greeks for {symbol} {date_str}: {e}")
                    return None
                
                # Extract valid IVs (same as streaming - step 5)
                valid_ivs = [
                    greeks.get('implied_volatility') 
                    for greeks in greeks_results_dict.values() 
                    if greeks and greeks.get('implied_volatility') is not None
                ]
                
                if not valid_ivs:
                    logger.warning(f"Failed to get IV for ATM options for {symbol} {date_str}")
                    return None
                
                # Calculate IVx (average of ATM call and put IV - same as streaming)
                ivx = sum(valid_ivs) / len(valid_ivs)
                
                # Calculate DTE and expected move (same as streaming)
                exp_date = datetime.strptime(date_str, "%Y-%m-%d").date()
                today = datetime.now().date()
                dte = (exp_date - today).days
                
                if dte <= 0:
                    logger.warning(f"Skipping expired expiration {symbol} {date_str} (DTE: {dte})")
                    return None
                
                # Calculate expected move (same as streaming)
                def calculate_expected_move(underlying_price: float, ivx: float, dte: int) -> float:
                    """Calculate the expected move in dollars."""
                    import math
                    return underlying_price * ivx * math.sqrt(dte / 365)
                
                expected_move = calculate_expected_move(underlying_price, ivx, dte)
                
                # Create result (same format as streaming)
                result = {
                    "expiration_date": date_str,
                    "days_to_expiration": dte,
                    "ivx_percent": round(ivx * 100, 2),
                    "expected_move_dollars": round(expected_move, 2),
                    "calculation_method": "atm_iv_average",  # Same as streaming
                    "options_count": len(options_chain)
                }
                return result
                
            except Exception as e:
                logger.error(f"❌ Error calculating IVx for {symbol} {expiration_date}: {e}")
                return None
        
        # Process expirations in parallel with concurrency control (increased for better performance)
        concurrency_limit = 10  # Increased from 5 to reduce batching pauses
        semaphore = asyncio.Semaphore(concurrency_limit)
        
        async def worker(expiration_date):
            async with semaphore:
                return await process_single_expiration(expiration_date)
        
        # Create tasks for all expirations
        tasks = [worker(exp) for exp in expiration_dates]
        
        # Process all tasks and collect results as they complete
        completed_count = 0
        
        for future in asyncio.as_completed(tasks):
            try:
                result = await future
                completed_count += 1
                
                if result:
                    ivx_results.append(result)
                else:
                    logger.warning(f"IVx calculation failed for an expiration ({completed_count}/{len(expiration_dates)})")
            
            except Exception as e:
                completed_count += 1
                logger.error(f"Error processing an expiration task: {e}")
        
        logger.info(f"Successfully calculated IVx for {len(ivx_results)}/{len(expiration_dates)} expirations for {symbol}")
        
        calculation_time = time.time() - start_time
        
        # Cache the results
        if ivx_results:
            ivx_cache.set(symbol, ivx_results)
            logger.info(f"💾 Cached IVx data for {symbol} ({len(ivx_results)} expirations)")
        
        return ApiResponse(
            success=True,
            data={
                "symbol": symbol,
                "expirations": ivx_results,
                "cached": False,
                "calculation_time": round(calculation_time, 2)
            },
            message=f"Calculated IVx data for {symbol} ({len(ivx_results)} expirations in {calculation_time:.2f}s)"
        )
        
    except Exception as e:
        logger.error(f"❌ Error getting IVx data for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/symbols/lookup", response_model=ApiResponse)
async def lookup_symbols(q: str):
    """Search for symbols matching the query."""
    try:
        results = await provider_manager.lookup_symbols(q)
        return ApiResponse(
            success=True,
            data={"symbols": [result.model_dump() for result in results]},
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

@app.get("/symbol/{symbol}/range/52week", response_model=ApiResponse)
async def get_52_week_range(symbol: str):
    """Get 52-week high and low for a symbol."""
    try:
        from datetime import datetime, timedelta
        
        # Get 1 year of daily data
        end_date = datetime.now()
        start_date = end_date - timedelta(days=365)
        
        bars = await provider_manager.get_historical_bars(
            symbol, "D",
            start_date.strftime('%Y-%m-%d'),
            end_date.strftime('%Y-%m-%d'),
            limit=365
        )
        
        if not bars:
            return ApiResponse(
                success=False,
                data=None,
                message=f"No historical data available for {symbol}"
            )
        
        # Calculate 52-week high and low
        high_bar = max(bars, key=lambda x: x.get('high', 0))
        low_bar = min(bars, key=lambda x: x.get('low', float('inf')))
        
        return ApiResponse(
            success=True,
            data={
                "symbol": symbol,
                "high": high_bar.get('high'),
                "low": low_bar.get('low'),
                "high_date": high_bar.get('time'),
                "low_date": low_bar.get('time'),
                "period_days": len(bars)
            },
            message=f"Retrieved 52-week range for {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting 52-week range for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/symbol/{symbol}/volume/average", response_model=ApiResponse)
async def get_average_volume(symbol: str, days: int = 20):
    """Get average volume for a symbol over specified number of days."""
    try:
        from datetime import datetime, timedelta
        
        # Get extra days to ensure we have enough trading days
        end_date = datetime.now()
        start_date = end_date - timedelta(days=days * 2)  # Get double to account for weekends/holidays
        
        bars = await provider_manager.get_historical_bars(
            symbol, "D",
            start_date.strftime('%Y-%m-%d'),
            end_date.strftime('%Y-%m-%d'),
            limit=days * 2
        )
        
        if not bars:
            return ApiResponse(
                success=False,
                data=None,
                message=f"No historical data available for {symbol}"
            )
        
        # Filter out bars with zero volume and take the most recent 'days' bars
        volume_bars = [bar for bar in bars if bar.get('volume', 0) > 0]
        recent_bars = volume_bars[-days:] if len(volume_bars) >= days else volume_bars
        
        if not recent_bars:
            return ApiResponse(
                success=False,
                data=None,
                message=f"No volume data available for {symbol}"
            )
        
        # Calculate average volume
        total_volume = sum(bar.get('volume', 0) for bar in recent_bars)
        average_volume = total_volume / len(recent_bars)
        
        return ApiResponse(
            success=True,
            data={
                "symbol": symbol,
                "average_volume": round(average_volume),
                "period_days": len(recent_bars),
                "requested_days": days
            },
            message=f"Retrieved {len(recent_bars)}-day average volume for {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting average volume for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Account & Portfolio Endpoints ===

@app.get("/positions", response_model=ApiResponse)
async def get_positions(request: Request):
    """Get enhanced positions with hierarchical grouping (Symbol -> Strategies -> Legs)."""
    # Require authentication for sensitive position data
    if auth_config.is_enabled():
        require_auth(request)
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
                data=account.model_dump(),
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
async def list_import_files():
    """List available import files (DBN and CSV) with basic information only - fast loading."""
    try:
        files_info = []
        
        # Ensure directory exists
        import_manager.dbn_files_dir.mkdir(exist_ok=True)
        
        # Get DBN files - basic info only
        for dbn_file in import_manager.dbn_files_dir.glob("*.dbn"):
            try:
                file_stat = dbn_file.stat()
                file_info = {
                    'filename': dbn_file.name,
                    'file_path': str(dbn_file),
                    'file_size': file_stat.st_size,
                    'modified_at': datetime.fromtimestamp(file_stat.st_mtime),
                    'file_type': 'dbn',
                    'needs_symbol_input': False,
                    'size_mb': round(file_stat.st_size / (1024 * 1024), 2),
                    'size_gb': round(file_stat.st_size / (1024 * 1024 * 1024), 2)
                }
                files_info.append(file_info)
            except Exception as e:
                logger.warning(f"Error processing DBN file {dbn_file}: {e}")
                continue
        
        # Get CSV files - basic info only
        for csv_file in import_manager.dbn_files_dir.glob("*.csv"):
            try:
                file_stat = csv_file.stat()
                file_info = {
                    'filename': csv_file.name,
                    'file_path': str(csv_file),
                    'file_size': file_stat.st_size,
                    'modified_at': datetime.fromtimestamp(file_stat.st_mtime),
                    'file_type': 'csv',
                    'needs_symbol_input': True,
                    'size_mb': round(file_stat.st_size / (1024 * 1024), 2),
                    'size_gb': round(file_stat.st_size / (1024 * 1024 * 1024), 2)
                }
                files_info.append(file_info)
            except Exception as e:
                logger.warning(f"Error processing CSV file {csv_file}: {e}")
                continue
        
        # Sort by modification time (newest first)
        files_info.sort(key=lambda x: x['modified_at'], reverse=True)
        
        dbn_count = sum(1 for f in files_info if f['file_type'] == 'dbn')
        csv_count = sum(1 for f in files_info if f['file_type'] == 'csv')
        
        return ApiResponse(
            success=True,
            data=files_info,
            message=f"Found {len(files_info)} import files ({dbn_count} DBN, {csv_count} CSV) - basic info only"
        )
    except Exception as e:
        logger.error(f"Error listing import files: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/files/detailed", response_model=ApiResponse)
async def list_import_files_detailed():
    """List available import files (DBN and CSV) with full metadata information - slower but complete."""
    try:
        files_info = await import_manager.list_available_files()
        
        # Convert to dict format for JSON serialization
        files_data = []
        for file_info in files_info:
            file_dict = file_info.dict()
            files_data.append(file_dict)
        
        return ApiResponse(
            success=True,
            data=files_data,
            message=f"Found {len(files_data)} import files ({sum(1 for f in files_data if f.get('file_type') == 'dbn')} DBN, {sum(1 for f in files_data if f.get('file_type') == 'csv')} CSV) - with metadata"
        )
    except Exception as e:
        logger.error(f"Error listing import files with metadata: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/imported-data", response_model=ApiResponse)
async def get_imported_data(expand: bool = False):
    """Get list of imported datasets by symbol with optional detailed information."""
    try:
        if expand:
            # Get detailed symbol-level data (slower, includes record counts, file sizes, etc.)
            symbol_datasets = import_manager.get_symbol_level_data()
            message = f"Retrieved {len(symbol_datasets)} imported symbol datasets with detailed information"
        else:
            # Get basic symbol-level data (faster, just symbol names and basic info)
            symbol_datasets = import_manager.get_symbol_level_data_basic()
            message = f"Retrieved {len(symbol_datasets)} imported symbol datasets (basic info only)"
        
        return ApiResponse(
            success=True,
            data=symbol_datasets,
            message=message
        )
    except Exception as e:
        logger.error(f"Error getting imported data: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/metadata/{filename}", response_model=ApiResponse)
async def get_file_metadata_api(filename: str, symbol: Optional[str] = None, force_refresh: bool = False):
    """Get detailed metadata for a specific import file (API endpoint)."""
    try:
        # Detect file type
        file_type = import_manager._detect_file_type(filename)
        
        if file_type == ImportFileType.CSV and not symbol:
            # For CSV files without symbol, return basic structure info
            file_path = import_manager.dbn_files_dir / filename
            if not file_path.exists():
                raise HTTPException(status_code=404, detail=f"File not found: {filename}")
            
            # Get CSV structure analysis
            from .services.data_import.csv_metadata_extractor import csv_metadata_extractor
            csv_analysis = csv_metadata_extractor.analyze_csv_structure(file_path)
            
            # Create a basic metadata response for CSV files
            basic_metadata = {
                "filename": filename,
                "file_path": str(file_path),
                "file_size": file_path.stat().st_size,
                "file_type": "csv",
                "csv_format": csv_analysis.get('csv_format', 'generic'),
                "csv_headers": csv_analysis.get('headers', []),
                "csv_sample_data": csv_analysis.get('sample_data', []),
                "record_count": csv_analysis.get('record_count', 0),
                "start_date": csv_analysis.get('start_date'),
                "end_date": csv_analysis.get('end_date'),
                "needs_symbol_input": True,
                "message": "CSV file detected. Symbol input will be required for import."
            }
            
            return ApiResponse(
                success=True,
                data=basic_metadata,
                message=f"Retrieved basic structure for CSV file {filename}"
            )
        else:
            # For DBN files or CSV files with symbol, get full metadata
            metadata = await import_manager.get_file_metadata(filename, symbol, force_refresh)
            
            return ApiResponse(
                success=True,
                data=metadata.dict(),
                message=f"Retrieved metadata for {filename}"
            )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
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

@app.get("/api/data-import/jobs", response_model=ApiResponse)
async def list_import_jobs_api(
    status: Optional[str] = None,
    limit: int = 50
):
    """List import jobs with optional status filtering (API endpoint)."""
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
        jobs_data = [job.model_dump() for job in jobs]
        
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


@app.delete("/api/data-import/imported-data/{symbol}", response_model=ApiResponse)
async def delete_imported_dataset(symbol: str, asset_type: Optional[str] = None):
    """Delete imported data for a specific symbol and optionally specific asset type."""
    try:
        deleted = import_manager.delete_imported_data(symbol, asset_type)
        
        if deleted:
            if asset_type:
                message = f"Successfully deleted {asset_type} data for symbol '{symbol}'"
            else:
                message = f"Successfully deleted all data for symbol '{symbol}'"
            
            return ApiResponse(
                success=True,
                data={"symbol": symbol, "asset_type": asset_type, "deleted": True},
                message=message
            )
        else:
            if asset_type:
                message = f"No {asset_type} data found for symbol '{symbol}' or deletion failed"
            else:
                message = f"No data found for symbol '{symbol}' or deletion failed"
            
            return ApiResponse(
                success=False,
                data={"symbol": symbol, "asset_type": asset_type, "deleted": False},
                message=message
            )
    except Exception as e:
        logger.error(f"Error deleting imported data for {symbol} (asset_type: {asset_type}): {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Data Import Queue Endpoints ===

@app.post("/api/data-import/queue", response_model=ApiResponse)
async def add_files_to_import_queue(request: MultiFileImportRequest):
    """Add multiple files to the import queue for sequential processing."""
    try:
        queue_ids = import_manager.import_queue.add_files_to_queue(request)
        
        return ApiResponse(
            success=True,
            data={
                "queue_ids": queue_ids,
                "batch_name": request.batch_name,
                "total_files": len(request.files)
            },
            message=f"Added {len(request.files)} files to import queue"
        )
    except Exception as e:
        logger.error(f"Error adding files to import queue: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data-import/queue/status", response_model=ApiResponse)
async def get_import_queue_status():
    """Get current status of the import queue."""
    try:
        queue_status = import_manager.import_queue.get_queue_status()
        
        return ApiResponse(
            success=True,
            data=queue_status.model_dump(),
            message=f"Retrieved queue status - {queue_status.total_items} items, {queue_status.queued_items} queued, {queue_status.completed_items} completed"
        )
    except Exception as e:
        logger.error(f"Error getting import queue status: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/api/data-import/queue/batch", response_model=ApiResponse)
async def add_batch_to_import_queue(request: dict):
    """Add multiple files to the import queue as a batch."""
    try:
        jobs = request.get('jobs', [])
        if not jobs:
            raise HTTPException(status_code=400, detail="No jobs provided in batch")
        
        # Process each job in the batch
        job_ids = []
        for job_data in jobs:
            # Convert job data to ImportRequest
            import_request = ImportRequest(
                filename=job_data['filename'],
                job_name=job_data.get('job_name', f"Import {job_data['filename']}"),
                overwrite_existing=job_data.get('overwrite_existing', False),
                file_type=job_data.get('file_type'),
                csv_symbol=job_data.get('csv_symbol'),
                csv_format=job_data.get('csv_format'),
                timestamp_convention=job_data.get('timestamp_convention')
            )
            
            # Start the import job
            job_id = await import_manager.start_import_job(import_request)
            job_ids.append(job_id)
        
        return ApiResponse(
            success=True,
            data={
                "job_ids": job_ids,
                "total_jobs": len(job_ids),
                "batch_size": len(jobs)
            },
            message=f"Successfully added {len(jobs)} files to import queue"
        )
    except Exception as e:
        logger.error(f"Error adding batch to import queue: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/api/data-import/queue/{queue_id}", response_model=ApiResponse)
async def remove_from_import_queue(queue_id: str):
    """Remove an item from the import queue."""
    try:
        removed = import_manager.import_queue.remove_from_queue(queue_id)
        
        if removed:
            return ApiResponse(
                success=True,
                data={"queue_id": queue_id, "removed": True},
                message=f"Queue item {queue_id} removed successfully"
            )
        else:
            return ApiResponse(
                success=False,
                data={"queue_id": queue_id, "removed": False},
                message=f"Could not remove queue item {queue_id} (not found or currently processing)"
            )
    except Exception as e:
        logger.error(f"Error removing from import queue {queue_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# === Data Aggregation Endpoints ===

@app.get("/api/data/aggregated/{symbol}", response_model=ApiResponse)
async def get_aggregated_data(
    symbol: str,
    timeframe: str = "5min",
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    market_hours_only: bool = True,
    limit: Optional[int] = None
):
    """Get aggregated OHLCV data for a symbol with flexible timeframes."""
    try:
        # Create aggregation request
        request = AggregationRequest(
            symbol=symbol,
            timeframe=timeframe,
            start_date=start_date,
            end_date=end_date,
            market_hours_only=market_hours_only,
            limit=limit
        )
        
        # Get aggregation service and process request
        aggregation_service = get_aggregation_service()
        result = aggregation_service.get_aggregated_data(request)
        
        return ApiResponse(
            success=True,
            data=result.dict(),
            message=f"Retrieved {result.record_count} {result.timeframe} bars for {symbol}"
        )
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting aggregated data for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data/symbols", response_model=ApiResponse)
async def get_available_symbols():
    """Get list of available symbols in the aggregated data."""
    try:
        aggregation_service = get_aggregation_service()
        symbols = aggregation_service.get_available_symbols()
        
        return ApiResponse(
            success=True,
            data={
                "symbols": symbols,
                "total_symbols": len(symbols)
            },
            message=f"Found {len(symbols)} available symbols"
        )
        
    except Exception as e:
        logger.error(f"Error getting available symbols: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data/symbols/{symbol}/date-range", response_model=ApiResponse)
async def get_symbol_date_range(symbol: str):
    """Get the available date range for a specific symbol."""
    try:
        aggregation_service = get_aggregation_service()
        date_range = aggregation_service.get_symbol_date_range(symbol)
        
        if not date_range:
            raise HTTPException(status_code=404, detail=f"Symbol '{symbol}' not found")
        
        return ApiResponse(
            success=True,
            data=date_range,
            message=f"Retrieved date range for {symbol}"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting date range for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data/timeframes", response_model=ApiResponse)
async def get_supported_timeframes():
    """Get supported timeframes for data aggregation."""
    try:
        aggregation_service = get_aggregation_service()
        timeframes = aggregation_service.get_supported_timeframes()
        
        return ApiResponse(
            success=True,
            data={
                "timeframes": timeframes,
                "total_timeframes": len(timeframes)
            },
            message="Retrieved supported timeframes"
        )
        
    except Exception as e:
        logger.error(f"Error getting supported timeframes: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/api/data/market-hours", response_model=ApiResponse)
async def get_market_hours_info():
    """Get market hours information."""
    try:
        from .services.data_aggregation.market_hours import get_market_hours_filter
        
        market_hours_filter = get_market_hours_filter()
        open_time, close_time = market_hours_filter.get_market_hours_bounds()
        
        return ApiResponse(
            success=True,
            data={
                "market_open": open_time.strftime('%H:%M'),
                "market_close": close_time.strftime('%H:%M'),
                "timezone": "America/New_York",
                "description": market_hours_filter.format_market_hours_info(),
                "trading_days": "Monday-Friday"
            },
            message="Retrieved market hours information"
        )
        
    except Exception as e:
        logger.error(f"Error getting market hours info: {e}")
        raise HTTPException(status_code=500, detail=str(e))

# Legacy endpoint for backward compatibility
@app.get("/data/aggregated/{symbol}", response_model=ApiResponse)
async def get_aggregated_data_legacy(
    symbol: str,
    timeframe: str = "5min",
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    market_hours_only: bool = True,
    limit: Optional[int] = None
):
    """Get aggregated OHLCV data for a symbol (legacy endpoint)."""
    return await get_aggregated_data(
        symbol=symbol,
        timeframe=timeframe,
        start_date=start_date,
        end_date=end_date,
        market_hours_only=market_hours_only,
        limit=limit
    )

# === WebSocket Authentication Function ===

async def authenticate_websocket(websocket: WebSocket) -> bool:
    """
    Authenticate WebSocket connection using various methods.
    Returns True if authenticated, False otherwise.
    """
    try:
        # Method 1: Check for JWT token in query parameters
        query_params = dict(websocket.query_params)
        token = query_params.get('token')
        
        if token:
            # Validate JWT token
            try:
                import jwt
                payload = jwt.decode(
                    token,
                    auth_config.jwt_secret_key,
                    algorithms=[auth_config.jwt_algorithm]
                )
                
                username = payload.get("sub")
                expires_at = datetime.fromtimestamp(payload.get("exp", 0), tz=timezone.utc)
                
                if username and expires_at > datetime.now(timezone.utc):
                    logger.info(f"🔐 WebSocket authenticated via JWT token for user: {username}")
                    return True
                    
            except jwt.InvalidTokenError as e:
                logger.warning(f"🔒 Invalid JWT token in WebSocket connection: {e}")
        
        # Method 2: Check for session cookie in headers
        cookie_header = None
        try:
            # Use proper Starlette Headers API for direct access
            cookie_header = websocket.headers.get('cookie')
        except Exception as e:
            logger.debug(f"🔒 Error accessing cookie header directly: {e}")
            # Fallback: try to get cookie from the headers dict
            try:
                headers_dict = dict(websocket.headers)
                cookie_header = headers_dict.get('cookie')
            except Exception as e2:
                logger.debug(f"🔒 Fallback cookie access also failed: {e2}")
        
        if cookie_header:
            # Parse cookies safely
            cookies = {}
            try:
                for cookie_str in cookie_header.split(';'):
                    cookie_str = cookie_str.strip()
                    if '=' in cookie_str:
                        try:
                            # Split only on the first '=' to handle cookies with '=' in values
                            parts = cookie_str.split('=', 1)
                            if len(parts) == 2:
                                key, val = parts
                                cookies[key.strip()] = val.strip()
                        except (ValueError, IndexError) as e:
                            # Skip malformed cookies
                            logger.debug(f"🔒 Skipping malformed cookie: {cookie_str} - {e}")
                            continue
            except Exception as e:
                logger.warning(f"🔒 Error parsing cookies: {e}")
                cookies = {}
            
            session_token = cookies.get(auth_config.session_cookie_name)
            if session_token:
                try:
                    import jwt
                    payload = jwt.decode(
                        session_token,
                        auth_config.jwt_secret_key,
                        algorithms=[auth_config.jwt_algorithm]
                    )
                    
                    username = payload.get("sub")
                    expires_at = datetime.fromtimestamp(payload.get("exp", 0))
                    current_time = datetime.now(timezone.utc)
                    
                    if username and expires_at > current_time:
                        logger.info(f"🔐 WebSocket authenticated via session cookie for user: {username}")
                        return True
                        
                except jwt.InvalidTokenError as e:
                    logger.warning(f"🔒 Invalid session token in WebSocket connection: {e}")
                except Exception as e:
                    logger.warning(f"🔒 Error validating session token: {e}")
        
        # Method 3: Check for Authorization header
        auth_header = None
        try:
            # Use proper Starlette Headers API for direct access
            auth_header = websocket.headers.get('authorization')
        except Exception as e:
            logger.debug(f"🔒 Error accessing authorization header: {e}")
            # Fallback: try to get authorization from the headers dict
            try:
                headers_dict = dict(websocket.headers)
                auth_header = headers_dict.get('authorization')
            except Exception as e2:
                logger.debug(f"🔒 Fallback authorization access also failed: {e2}")
        
        if auth_header:
            if auth_header.startswith("Bearer "):
                token = auth_header.split(" ", 1)[1]
                try:
                    import jwt
                    payload = jwt.decode(
                        token,
                        auth_config.jwt_secret_key,
                        algorithms=[auth_config.jwt_algorithm]
                    )
                    
                    username = payload.get("sub")
                    expires_at = datetime.fromtimestamp(payload.get("exp", 0))
                    
                    if username and expires_at > datetime.now(timezone.utc):
                        logger.info(f"🔐 WebSocket authenticated via Authorization header for user: {username}")
                        return True
                        
                except jwt.InvalidTokenError as e:
                    logger.warning(f"🔒 Invalid Bearer token in WebSocket connection: {e}")
            
            elif auth_header.startswith("Basic ") and auth_config.method.value == "simple":
                # Basic authentication for simple auth method
                import base64
                try:
                    encoded_credentials = auth_header.split(" ", 1)[1]
                    credentials = base64.b64decode(encoded_credentials).decode("utf-8")
                    
                    # Safe credential parsing - handle malformed credentials
                    if ':' not in credentials:
                        logger.warning(f"🔒 Basic auth credentials missing colon separator")
                        return False
                    
                    credential_parts = credentials.split(":", 1)
                    if len(credential_parts) != 2:
                        logger.warning(f"🔒 Basic auth credentials malformed")
                        return False
                    
                    username, password = credential_parts
                    
                    if (username == auth_config.simple_username and 
                        password == auth_config.simple_password):
                        logger.info(f"🔐 WebSocket authenticated via Basic auth for user: {username}")
                        return True
                    else:
                        logger.warning(f"🔒 Basic auth credentials don't match")
                        
                except Exception as e:
                    logger.error(f"🔒 Exception in Basic auth processing: {e}")
        
        logger.warning("🔒 WebSocket connection failed authentication - no valid credentials found")
        return False
        
    except Exception as e:
        logger.error(f"❌ WebSocket authentication error: {e}")
        return False

# === WebSocket Endpoint ===

@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """WebSocket endpoint with authentication protection and better error handling."""
    
    # Authenticate WebSocket connection if authentication is enabled
    if auth_config.is_enabled():
        try:
            # Check for authentication via query parameters or headers
            authenticated = await authenticate_websocket(websocket)
            if not authenticated:
                logger.warning("🔒 Unauthenticated WebSocket connection attempt rejected")
                await websocket.close(code=1008, reason="Authentication required")
                return
        except Exception as e:
            logger.error(f"❌ WebSocket authentication error: {e}")
            await websocket.close(code=1011, reason="Authentication failed")
            return
    
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
