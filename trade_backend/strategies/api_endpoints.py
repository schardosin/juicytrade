"""
Strategy Management API Endpoints

This module contains the REST API endpoints for managing user strategies:
- Strategy upload and validation
- Strategy listing and management
- Strategy execution control
- Real-time monitoring

These endpoints integrate with the frontend to provide a complete strategy management interface.
"""

import asyncio
import logging
from typing import List, Dict, Any, Optional
from fastapi import APIRouter, HTTPException, Form, File, UploadFile, Request
from datetime import datetime
from pydantic import BaseModel, Field

# No authentication needed for now - single user system
from .strategy_registry import StrategyRegistry
from .strategy_validator import StrategyValidator
from .execution_engine import StrategyExecutionEngine

logger = logging.getLogger(__name__)

# Initialize strategy management system (singleton pattern)
_strategy_registry = None
_strategy_validator = None

def get_strategy_registry():
    global _strategy_registry
    if _strategy_registry is None:
        _strategy_registry = StrategyRegistry()
    return _strategy_registry

def get_strategy_validator():
    global _strategy_validator
    if _strategy_validator is None:
        _strategy_validator = StrategyValidator()
    return _strategy_validator

def get_execution_engine(request=None):
    """Get the strategy execution engine from app state"""
    from fastapi import Request
    if request and hasattr(request.app.state, 'strategy_execution_engine'):
        return request.app.state.strategy_execution_engine
    # Fallback for when request is not available
    return None

router = APIRouter(prefix="/api/strategies", tags=["strategies"])


# ============================================================================
# Pydantic Models for API Requests/Responses
# ============================================================================

class StrategyUploadRequest(BaseModel):
    """Request model for strategy upload."""
    name: str = Field(..., min_length=1, max_length=100)
    description: str = Field("", max_length=500)

class StrategyUpdateRequest(BaseModel):
    """Request model for strategy update."""
    name: Optional[str] = Field(None, min_length=1, max_length=100)
    description: Optional[str] = Field(None, max_length=500)
    config: Optional[Dict[str, Any]] = None

class StrategyExecutionRequest(BaseModel):
    """Request model for strategy execution."""
    strategy_id: str = Field(..., min_length=1)
    mode: str = Field("live")
    config: Optional[Dict[str, Any]] = None

class StrategyMetadata(BaseModel):
    """Strategy metadata response model."""
    strategy_id: str
    name: str
    description: str
    author: str
    version: str
    risk_level: str
    max_positions: int
    preferred_symbols: List[str]
    created_at: str
    last_used: str
    validation_count: int
    success_count: int
    error_count: int

class ValidationResult(BaseModel):
    """Validation result response model."""
    success: bool
    message: str
    details: Dict[str, Any] = {}
    validation_steps: List[Dict[str, Any]] = []

class StrategyStatus(BaseModel):
    """Strategy execution status model."""
    strategy_id: str
    is_running: bool
    is_paused: bool
    created_at: str
    last_activity: str
    trades_count: int
    pnl: float
    win_rate: float
    error_count: int

class ExecutionStats(BaseModel):
    """Execution engine statistics model."""
    total_strategies: int
    running_strategies: int
    paused_strategies: int
    total_pnl: float
    total_trades: int
    uptime_seconds: float


# ============================================================================
# Strategy Upload & Management Endpoints
# ============================================================================

@router.post("/upload", response_model=Dict[str, Any])
async def upload_strategy(
    file: UploadFile = File(...),
    name: str = Form(..., min_length=1, max_length=100),
    description: str = Form("", max_length=500)
):
    """
    Upload and validate a new trading strategy.

    - **file**: Python file containing the strategy class
    - **name**: Human-readable strategy name
    - **description**: Strategy description (optional)

    Returns validation results and registration status.
    """
    try:
        logger.info(f"Strategy upload request: {name}")

        # Read and validate file content
        file_content = await file.read()
        filename = file.filename

        # Validate the strategy
        validation_result = await get_strategy_validator().validate_strategy_file(file_content, filename)

        if not validation_result.success:
            logger.warning(f"Strategy validation failed: {name}")
            return {
                "success": False,
                "error": "Validation Failed",
                "message": validation_result.message,
                "validation_details": {
                    "validation_steps": validation_result.validation_steps,
                    "errors": validation_result.errors,
                    "warnings": validation_result.warnings,
                    "suggestions": validation_result.suggestions,
                    **validation_result.details
                }
            }

        # Register the strategy
        user_id = "default_user"  # Single user system
        registration_result = await get_strategy_registry().register_strategy(
            user_id=user_id,
            strategy_file=file_content,
            filename=filename,
            strategy_name=name,
            description=description
        )

        if not registration_result.get("success"):
            logger.error(f"Strategy registration failed: {name}")
            raise HTTPException(
                status_code=400,
                detail=registration_result.get("error", "Registration failed")
            )

        logger.info(f"Strategy successfully registered: {name} -> {registration_result['strategy_id']}")
        return {
            "success": True,
            "message": f"Strategy '{name}' uploaded and validated successfully",
            "strategy_id": registration_result["strategy_id"],
            "strategy_name": name,
            "validation_details": registration_result.get("validation_details", {})
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Strategy upload error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Upload failed: {str(e)}")


@router.get("/my")
async def get_user_strategies():
    """
    Get all strategies for the current user.

    Returns a list of user's registered strategies with metadata.
    """
    try:
        user_id = "default_user"  # Single user system
        strategies = await get_strategy_registry().get_user_strategies(user_id)
        logger.info(f"Retrieved {len(strategies)} strategies for user {user_id}")
        return strategies

    except Exception as e:
        logger.error(f"Error fetching user strategies: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to fetch strategies: {str(e)}")


@router.get("/stats", response_model=ExecutionStats)
async def get_system_stats(request: Request):
    """
    Get system-wide trading statistics.

    Returns execution engine statistics.
    """
    try:
        execution_engine = get_execution_engine(request)
        if execution_engine:
            stats = execution_engine.get_execution_stats()
            return ExecutionStats(**stats)
        else:
            # Return default stats if engine not available
            return ExecutionStats(
                total_strategies=0,
                running_strategies=0,
                paused_strategies=0,
                total_pnl=0.0,
                total_trades=0,
                uptime_seconds=0.0
            )

    except Exception as e:
        logger.error(f"Error getting system stats: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get stats: {str(e)}")


@router.get("/registry/health", response_model=Dict[str, Any])
async def get_registry_health():
    """
    Get strategy registry health status.

    Returns health check results.
    """
    try:
        health = await get_strategy_registry().health_check()
        return health

    except Exception as e:
        logger.error(f"Error getting registry health: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Health check failed: {str(e)}")


@router.get("/{strategy_id}", response_model=StrategyMetadata)
async def get_strategy(strategy_id: str):
    """
    Get details for a specific strategy.

    - **strategy_id**: Unique strategy identifier

    Returns strategy metadata and current status.
    """
    try:
        strategy_metadata = await get_strategy_registry().get_strategy_details(strategy_id)

        if not strategy_metadata:
            raise HTTPException(status_code=404, detail="Strategy not found")

        return strategy_metadata

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting strategy details: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get strategy: {str(e)}")


@router.get("/{strategy_id}/parameters", response_model=Dict[str, Any])
async def get_strategy_parameters(strategy_id: str):
    """
    Get configurable parameters for a specific strategy.

    - **strategy_id**: Unique strategy identifier

    Returns strategy parameter schema for dynamic form generation.
    """
    try:
        logger.info(f"Getting parameters for strategy: {strategy_id}")
        
        # Get strategy class to access metadata
        strategy_class = await get_strategy_registry().get_strategy(strategy_id)
        
        if not strategy_class:
            logger.error(f"Strategy class not found for ID: {strategy_id}")
            raise HTTPException(status_code=404, detail=f"Strategy not found: {strategy_id}")

        logger.info(f"Found strategy class: {strategy_class.__name__}")

        # Create a temporary instance to get metadata
        # We need to provide minimal dependencies for metadata extraction
        from .mock_providers import MockDataProvider, MockOrderExecutor
        
        try:
            temp_instance = strategy_class(
                strategy_id=strategy_id,
                data_provider=MockDataProvider(),
                order_executor=MockOrderExecutor(),
                config={}
            )
            logger.info(f"Created temporary strategy instance")
        except Exception as instance_error:
            logger.error(f"Failed to create strategy instance: {str(instance_error)}")
            raise HTTPException(status_code=500, detail=f"Failed to create strategy instance: {str(instance_error)}")
        
        # Initialize the strategy so all attributes are set up
        try:
            await temp_instance.initialize_strategy()
            logger.info(f"Initialized strategy instance")
        except Exception as init_error:
            logger.error(f"Failed to initialize strategy: {str(init_error)}")
            raise HTTPException(status_code=500, detail=f"Failed to initialize strategy: {str(init_error)}")
        
        # Get strategy metadata which includes parameters
        try:
            metadata = temp_instance.get_strategy_metadata()
            logger.info(f"Retrieved strategy metadata: {list(metadata.keys())}")
        except Exception as metadata_error:
            logger.error(f"Failed to get strategy metadata: {str(metadata_error)}")
            raise HTTPException(status_code=500, detail=f"Failed to get strategy metadata: {str(metadata_error)}")
        
        # Extract parameters with enhanced information
        parameters = metadata.get("parameters", {})
        logger.info(f"Strategy parameters: {list(parameters.keys())}")
        
        # Add framework-level parameters that are always available
        framework_parameters = {
            "account_balance": {
                "type": "float",
                "default": 100000.0,
                "min": 1000.0,
                "max": 10000000.0,
                "description": "Account balance for position sizing",
                "category": "framework"
            },
            "max_position_size": {
                "type": "integer", 
                "default": 1000,
                "min": 1,
                "max": 10000,
                "description": "Maximum position size (shares/contracts)",
                "category": "framework"
            },
            "commission_per_trade": {
                "type": "float",
                "default": 1.0,
                "min": 0.0,
                "max": 50.0,
                "description": "Commission per trade ($)",
                "category": "framework"
            }
        }
        
        # Merge strategy parameters with framework parameters
        all_parameters = {**parameters, **framework_parameters}
        
        # Add category to strategy parameters if not present
        for param_name, param_config in all_parameters.items():
            if "category" not in param_config:
                param_config["category"] = "strategy"
        
        logger.info(f"Returning parameters for strategy {strategy_id}: {list(all_parameters.keys())}")
        
        return {
            "success": True,
            "strategy_id": strategy_id,
            "strategy_name": metadata.get("name", "Unknown Strategy"),
            "parameters": all_parameters,
            "metadata": {
                "name": metadata.get("name"),
                "description": metadata.get("description"),
                "version": metadata.get("version"),
                "risk_level": metadata.get("risk_level"),
                "max_positions": metadata.get("max_positions"),
                "preferred_symbols": metadata.get("preferred_symbols", [])
            }
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting strategy parameters for {strategy_id}: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to get parameters: {str(e)}")


# ============================================================================
# Strategy Configuration Management Endpoints (NEW)
# ============================================================================

@router.post("/{strategy_id}/configs", response_model=Dict[str, Any])
async def create_strategy_configuration(strategy_id: str, config_data: Dict[str, Any]):
    """
    Create a new configuration for a strategy.

    - **strategy_id**: Strategy to create configuration for
    - **config_data**: Configuration parameters and metadata

    Returns the created configuration.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyConfiguration
        import uuid
        
        # Validate strategy exists
        strategy_details = await get_strategy_registry().get_strategy_details(strategy_id)
        if not strategy_details:
            raise HTTPException(status_code=404, detail="Strategy not found")
        
        # Extract configuration data
        name = config_data.get("name", "")
        description = config_data.get("description", "")
        parameters = config_data.get("parameters", {})
        
        # Generate configuration ID
        config_id = f"config_{strategy_id}_{uuid.uuid4().hex[:8]}"
        
        # Create configuration
        with strategy_db_manager.get_session() as session:
            config = StrategyConfiguration(
                config_id=config_id,
                strategy_id=strategy_id,
                user_id="default_user",
                name=name if name else None,
                description=description if description else None,
                parameters=parameters
            )
            
            session.add(config)
            session.commit()
            session.refresh(config)
            
            logger.info(f"Created configuration {config_id} for strategy {strategy_id}")
            
            return {
                "success": True,
                "message": "Configuration created successfully",
                "configuration": config.to_dict()
            }
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating configuration: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create configuration: {str(e)}")


@router.get("/{strategy_id}/configs", response_model=List[Dict[str, Any]])
async def get_strategy_configurations(strategy_id: str):
    """
    Get all configurations for a strategy.

    - **strategy_id**: Strategy to get configurations for

    Returns list of configurations.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyConfiguration
        
        with strategy_db_manager.get_session() as session:
            configs = session.query(StrategyConfiguration).filter(
                StrategyConfiguration.strategy_id == strategy_id,
                StrategyConfiguration.user_id == "default_user",
                StrategyConfiguration.is_active == True
            ).order_by(StrategyConfiguration.created_at.desc()).all()
            
            return [config.to_dict() for config in configs]
    
    except Exception as e:
        logger.error(f"Error getting configurations: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get configurations: {str(e)}")


@router.get("/configs/{config_id}", response_model=Dict[str, Any])
async def get_configuration(config_id: str):
    """
    Get a specific configuration.

    - **config_id**: Configuration ID

    Returns configuration details.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyConfiguration
        
        with strategy_db_manager.get_session() as session:
            config = session.query(StrategyConfiguration).filter(
                StrategyConfiguration.config_id == config_id,
                StrategyConfiguration.user_id == "default_user"
            ).first()
            
            if not config:
                raise HTTPException(status_code=404, detail="Configuration not found")
            
            return config.to_dict()
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting configuration: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get configuration: {str(e)}")


@router.put("/configs/{config_id}", response_model=Dict[str, Any])
async def update_configuration(config_id: str, config_data: Dict[str, Any]):
    """
    Update a configuration.

    - **config_id**: Configuration to update
    - **config_data**: Updated configuration data

    Returns updated configuration.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyConfiguration
        from sqlalchemy.sql import func
        
        with strategy_db_manager.get_session() as session:
            config = session.query(StrategyConfiguration).filter(
                StrategyConfiguration.config_id == config_id,
                StrategyConfiguration.user_id == "default_user"
            ).first()
            
            if not config:
                raise HTTPException(status_code=404, detail="Configuration not found")
            
            # Update fields
            if "name" in config_data:
                config.name = config_data["name"] if config_data["name"] else None
            if "description" in config_data:
                config.description = config_data["description"] if config_data["description"] else None
            if "parameters" in config_data:
                config.parameters = config_data["parameters"]
            
            config.updated_at = func.now()
            
            session.commit()
            session.refresh(config)
            
            logger.info(f"Updated configuration {config_id}")
            
            return {
                "success": True,
                "message": "Configuration updated successfully",
                "configuration": config.to_dict()
            }
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error updating configuration: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to update configuration: {str(e)}")


@router.delete("/configs/{config_id}", response_model=Dict[str, Any])
async def delete_configuration(config_id: str):
    """
    Delete a configuration.

    - **config_id**: Configuration to delete

    Returns deletion confirmation.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyConfiguration
        
        with strategy_db_manager.get_session() as session:
            config = session.query(StrategyConfiguration).filter(
                StrategyConfiguration.config_id == config_id,
                StrategyConfiguration.user_id == "default_user"
            ).first()
            
            if not config:
                raise HTTPException(status_code=404, detail="Configuration not found")
            
            # Soft delete
            config.is_active = False
            
            session.commit()
            
            logger.info(f"Deleted configuration {config_id}")
            
            return {
                "success": True,
                "message": "Configuration deleted successfully",
                "config_id": config_id
            }
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting configuration: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete configuration: {str(e)}")


# ============================================================================
# Backtest Management Endpoints (NEW TEMPLATE-BASED)
# ============================================================================

@router.post("/{strategy_id}/backtest", response_model=Dict[str, Any])
async def run_strategy_backtest(strategy_id: str, backtest_request: Dict[str, Any]):
    """
    Run a backtest directly with strategy template and parameters.

    - **strategy_id**: Strategy template to backtest
    - **backtest_request**: Backtest configuration containing:
        - parameters: Strategy parameters (symbol, fast_period, etc.)
        - start_date: Start date (YYYY-MM-DD)
        - end_date: End date (YYYY-MM-DD)
        - initial_capital: Initial capital amount
        - speed_multiplier: Backtest speed (1=real-time, 1000=very fast)

    Returns backtest run details and starts execution.
    """
    try:
        from .database import strategy_db_manager
        from .models import BacktestRun
        from datetime import datetime
        import uuid
        
        # Extract request data - NEW TEMPLATE-BASED APPROACH
        parameters = backtest_request.get("parameters", {})
        start_date_str = backtest_request.get("start_date")
        end_date_str = backtest_request.get("end_date")
        initial_capital = backtest_request.get("initial_capital", 100000.0)
        speed_multiplier = backtest_request.get("speed_multiplier", 1000)
        timeframe = backtest_request.get("timeframe", "1min")  # Add timeframe parameter
        
        if not parameters:
            raise HTTPException(status_code=400, detail="parameters are required")
        if not start_date_str or not end_date_str:
            raise HTTPException(status_code=400, detail="start_date and end_date are required")
        
        # Validate strategy exists
        strategy_details = await get_strategy_registry().get_strategy_details(strategy_id)
        if not strategy_details:
            raise HTTPException(status_code=404, detail="Strategy not found")
        
        # Parse dates
        start_date = datetime.fromisoformat(start_date_str)
        end_date = datetime.fromisoformat(end_date_str)
        
        # Generate run ID
        run_id = f"run_{strategy_id}_{uuid.uuid4().hex[:8]}"
        
        # Create backtest run record with parameters stored directly
        with strategy_db_manager.get_session() as session:
            backtest_run = BacktestRun(
                run_id=run_id,
                config_id=None,  # No configuration needed in new architecture
                strategy_id=strategy_id,
                user_id="default_user",
                start_date=start_date,
                end_date=end_date,
                initial_capital=initial_capital,
                speed_multiplier=speed_multiplier,
                parameters=parameters,  # Store parameters directly with the run
                status="pending"
            )
            
            session.add(backtest_run)
            session.commit()
            session.refresh(backtest_run)
            
            logger.info(f"Created backtest run {run_id} for strategy {strategy_id} with parameters: {parameters}")
            
            # Start actual backtest execution
            try:
                # Get strategy class
                strategy_class = await get_strategy_registry().get_strategy(strategy_id)
                if not strategy_class:
                    raise HTTPException(status_code=404, detail="Strategy class not found")
                
                # Create strategy instance with parameters
                from .mock_providers import MockDataProvider, MockOrderExecutor
                
                strategy_instance = strategy_class(
                    strategy_id=strategy_id,
                    data_provider=MockDataProvider(),
                    order_executor=MockOrderExecutor(),
                    config=parameters
                )
                
                # Initialize the strategy
                await strategy_instance.initialize_strategy()
                
                # Create backtest engine with real provider manager
                from .backtest_engine import StrategyBacktestEngine
                
                # Get provider manager from the main application
                provider_manager = None
                try:
                    from ..provider_manager import provider_manager as global_provider_manager
                    provider_manager = global_provider_manager
                    logger.info("Using real provider manager for backtest")
                except ImportError:
                    logger.warning("Could not import provider manager, using mock data")
                
                backtest_engine = StrategyBacktestEngine(
                    initial_capital=initial_capital,
                    commission_per_trade=parameters.get("commission_per_trade", 1.0),
                    slippage_bps=2.0,
                    provider_manager=provider_manager,
                    timeframe=timeframe  # Pass timeframe to backtest engine
                )
                
                # Run the backtest
                symbols = [parameters.get("symbol", "SPY")]
                backtest_results = await backtest_engine.run_backtest(
                    strategy=strategy_instance,
                    start_date=start_date,
                    end_date=end_date,
                    symbols=symbols,
                    speed_multiplier=speed_multiplier
                )
                
                if not backtest_results.get("success"):
                    # Update run status to failed
                    backtest_run.status = "failed"
                    backtest_run.error_message = backtest_results.get("error", "Backtest execution failed")
                    session.commit()
                    
                    return {
                        "success": False,
                        "message": "Backtest execution failed",
                        "error": backtest_results.get("error", "Unknown error"),
                        "run_id": run_id
                    }
                
                # Serialize datetime objects in results before storing
                def serialize_datetime_objects(obj):
                    """Recursively serialize datetime objects in nested data structures"""
                    if hasattr(obj, 'isoformat'):
                        return obj.isoformat()
                    elif isinstance(obj, dict):
                        return {key: serialize_datetime_objects(value) for key, value in obj.items()}
                    elif isinstance(obj, list):
                        return [serialize_datetime_objects(item) for item in obj]
                    else:
                        return obj
                
                # Serialize all datetime objects in backtest results
                serialized_results = serialize_datetime_objects(backtest_results)
                
                # Update run with results
                backtest_run.status = "completed"
                backtest_run.results = serialized_results
                session.commit()
                
                logger.info(f"Backtest completed successfully for run {run_id}")
                
                return {
                    "success": True,
                    "message": "Backtest completed successfully",
                    "run_id": run_id,
                    "data": backtest_results,
                    "metrics": backtest_results.get("metrics", {}),
                    "trades": backtest_results.get("trades", []),
                    "equity_curve": backtest_results.get("equity_curve", []),
                    "checkpoints": backtest_results.get("checkpoints", []),
                    "action_log": backtest_results.get("action_log", [])
                }
                
            except Exception as backtest_error:
                logger.error(f"Backtest execution failed: {str(backtest_error)}")
                
                # Update run status to failed
                backtest_run.status = "failed"
                backtest_run.error_message = str(backtest_error)
                session.commit()
                
                return {
                    "success": False,
                    "message": "Backtest execution failed",
                    "error": str(backtest_error),
                    "run_id": run_id
                }
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error creating backtest run: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to create backtest run: {str(e)}")


@router.get("/backtest/runs", response_model=List[Dict[str, Any]])
async def get_backtest_runs(strategy_id: Optional[str] = None):
    """
    Get backtest runs, optionally filtered by strategy.

    - **strategy_id**: Optional strategy filter

    Returns list of backtest runs.
    """
    try:
        from .database import strategy_db_manager
        from .models import BacktestRun
        
        with strategy_db_manager.get_session() as session:
            query = session.query(BacktestRun).filter(
                BacktestRun.user_id == "default_user"
            )
            
            if strategy_id:
                query = query.filter(BacktestRun.strategy_id == strategy_id)
            
            runs = query.order_by(BacktestRun.created_at.desc()).all()
            
            return [run.to_dict() for run in runs]
    
    except Exception as e:
        logger.error(f"Error getting backtest runs: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get backtest runs: {str(e)}")


@router.get("/backtest/runs/{run_id}", response_model=Dict[str, Any])
async def get_backtest_run(run_id: str):
    """
    Get a specific backtest run.

    - **run_id**: Backtest run ID

    Returns backtest run details and results.
    """
    try:
        from .database import strategy_db_manager
        from .models import BacktestRun
        
        with strategy_db_manager.get_session() as session:
            run = session.query(BacktestRun).filter(
                BacktestRun.run_id == run_id,
                BacktestRun.user_id == "default_user"
            ).first()
            
            if not run:
                raise HTTPException(status_code=404, detail="Backtest run not found")
            
            return run.to_dict()
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting backtest run: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get backtest run: {str(e)}")

@router.post("/backtest/{strategy_id}", response_model=Dict[str, Any])
async def run_backtest(strategy_id: str, backtest_request: Dict[str, Any]):
    """
    Run a backtest for a given strategy.

    - **strategy_id**: Strategy to backtest
    - **backtest_request**: Backtest configuration

    Returns backtest results.
    """
    try:
        from .backtest_engine import StrategyBacktestEngine
        from .mock_providers import MockDataProvider, MockOrderExecutor
        
        # Get strategy class
        strategy_class = await get_strategy_registry().get_strategy(strategy_id)
        if not strategy_class:
            raise HTTPException(status_code=404, detail="Strategy not found")
        
        # Create strategy instance
        strategy_instance = strategy_class(
            strategy_id=strategy_id,
            data_provider=MockDataProvider(),
            order_executor=MockOrderExecutor(),
            config=backtest_request.get("parameters", {})
        )
        
        # Initialize the strategy
        await strategy_instance.initialize_strategy()
        
        # Create backtest engine
        backtest_engine = StrategyBacktestEngine(
            initial_capital=backtest_request.get("initial_capital", 100000.0),
            commission_per_trade=backtest_request.get("parameters", {}).get("commission_per_trade", 1.0),
            slippage_bps=2.0
        )
        
        # Run the backtest
        symbols = [backtest_request.get("parameters", {}).get("symbol", "SPY")]
        start_date = datetime.fromisoformat(backtest_request.get("start_date"))
        end_date = datetime.fromisoformat(backtest_request.get("end_date"))
        
        backtest_results = await backtest_engine.run_backtest(
            strategy=strategy_instance,
            start_date=start_date,
            end_date=end_date,
            symbols=symbols
        )
        
        return backtest_results
        
    except Exception as e:
        logger.error(f"Error running backtest: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to run backtest: {str(e)}")


@router.delete("/backtest/runs/{run_id}", response_model=Dict[str, Any])
async def delete_backtest_run(run_id: str):
    """
    Delete a backtest run.

    - **run_id**: Backtest run to delete

    Returns deletion confirmation.
    """
    try:
        from .database import strategy_db_manager
        from .models import BacktestRun
        
        with strategy_db_manager.get_session() as session:
            run = session.query(BacktestRun).filter(
                BacktestRun.run_id == run_id,
                BacktestRun.user_id == "default_user"
            ).first()
            
            if not run:
                raise HTTPException(status_code=404, detail="Backtest run not found")
            
            session.delete(run)
            session.commit()
            
            logger.info(f"Deleted backtest run {run_id}")
            
            return {
                "success": True,
                "message": "Backtest run deleted successfully",
                "run_id": run_id
            }
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting backtest run: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to delete backtest run: {str(e)}")


@router.put("/{strategy_id}", response_model=Dict[str, Any])
async def update_strategy(strategy_id: str, updates: StrategyUpdateRequest):
    """
    Update an existing strategy's metadata or code.

    - **strategy_id**: Strategy to update
    - **updates**: Fields to update

    Returns update confirmation.
    """
    try:
        # Perform update
        update_result = await get_strategy_registry().update_strategy(strategy_id, updates.dict(exclude_unset=True))

        if not update_result.get("success"):
            raise HTTPException(
                status_code=400,
                detail=update_result.get("error", "Update failed")
            )

        return {
            "success": True,
            "message": "Strategy updated successfully",
            "strategy_id": strategy_id
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error updating strategy: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Update failed: {str(e)}")


@router.delete("/{strategy_id}", response_model=Dict[str, Any])
async def delete_strategy(strategy_id: str):
    """
    Delete a strategy.

    - **strategy_id**: Strategy to delete

    Returns deletion confirmation.
    """
    try:
        # Delete strategy
        success = await get_strategy_registry().delete_strategy(strategy_id)

        if success:
            return {
                "success": True,
                "message": "Strategy deleted successfully",
                "strategy_id": strategy_id
            }
        else:
            raise HTTPException(status_code=400, detail="Failed to delete strategy")

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting strategy: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Deletion failed: {str(e)}")


# ============================================================================
# Strategy Execution Management Endpoints
# ============================================================================

@router.post("/{strategy_id}/start", response_model=Dict[str, Any])
async def start_strategy_with_parameters(strategy_id: str, start_request: Dict[str, Any], request: Request):
    """
    Start executing a strategy directly with parameters.

    - **strategy_id**: Strategy template to start
    - **start_request**: Execution configuration containing:
        - parameters: Strategy parameters (symbol, fast_period, etc.)
        - mode: Execution mode ('live', 'paper', 'simulation')
        - name: Instance name (optional)

    Start strategy in live or paper trading mode.
    """
    try:
        from .database import strategy_db_manager
        from .models import StrategyExecution
        from datetime import datetime
        import uuid
        
        # Extract request data - NEW TEMPLATE-BASED APPROACH
        parameters = start_request.get("parameters", {})
        mode = start_request.get("mode", "live")
        instance_name = start_request.get("name", f"{strategy_id} Instance")
        
        if not parameters:
            raise HTTPException(status_code=400, detail="parameters are required")
        
        # Validate strategy exists
        strategy_details = await get_strategy_registry().get_strategy_details(strategy_id)
        if not strategy_details:
            raise HTTPException(status_code=404, detail="Strategy not found")
        
        # Get strategy instance
        strategy_instance = await get_strategy_registry().get_strategy(strategy_id)
        if not strategy_instance:
            raise HTTPException(status_code=404, detail="Strategy not found")

        # Get execution engine
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        # Generate execution ID
        execution_id = f"exec_{strategy_id}_{uuid.uuid4().hex[:8]}"
        
        # Create execution record with parameters stored directly
        with strategy_db_manager.get_session() as session:
            strategy_execution = StrategyExecution(
                execution_id=execution_id,
                strategy_id=strategy_id,
                config_id=None,  # No configuration needed in new architecture
                user_id="default_user",
                mode=mode,
                status="running",
                configuration=parameters,  # Store parameters directly with the execution
                name=instance_name
            )
            
            session.add(strategy_execution)
            session.commit()
            session.refresh(strategy_execution)
            
            logger.info(f"Created strategy execution {execution_id} for strategy {strategy_id} with parameters: {parameters}")

        # Merge configurations for execution engine
        config = parameters.copy()
        config['user_id'] = "default_user"
        config['strategy_id'] = strategy_id
        config['execution_id'] = execution_id
        config['mode'] = mode

        # Start execution
        success = await execution_engine.start_strategy(execution_id, strategy_instance, config)

        if success:
            logger.info(f"Strategy started: {strategy_id} -> {execution_id}")
            return {
                "success": True,
                "message": "Strategy started successfully",
                "strategy_id": strategy_id,
                "execution_id": execution_id,
                "mode": mode,
                "parameters": parameters
            }
        else:
            # Update execution status to failed
            with strategy_db_manager.get_session() as session:
                execution = session.query(StrategyExecution).filter(
                    StrategyExecution.execution_id == execution_id
                ).first()
                if execution:
                    execution.status = "failed"
                    session.commit()
            
            raise HTTPException(status_code=400, detail="Failed to start strategy")

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error starting strategy: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to start strategy: {str(e)}")


@router.post("/{strategy_id}/stop", response_model=Dict[str, Any])
async def stop_strategy(strategy_id: str, request: Request):
    """
    Stop a running strategy.

    - **strategy_id**: Strategy to stop

    Returns stop confirmation.
    """
    try:
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        success = await execution_engine.stop_strategy(strategy_id)

        if success:
            logger.info(f"Strategy stopped: {strategy_id}")
            return {
                "success": True,
                "message": "Strategy stopped successfully",
                "strategy_id": strategy_id
            }
        else:
            raise HTTPException(status_code=400, detail="Strategy not found or not running")

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error stopping strategy: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to stop strategy: {str(e)}")


@router.post("/{strategy_id}/pause", response_model=Dict[str, Any])
async def pause_strategy(strategy_id: str, request: Request):
    """
    Pause a running strategy.

    - **strategy_id**: Strategy to pause

    Returns pause confirmation.
    """
    try:
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        success = await execution_engine.pause_strategy(strategy_id)

        if success:
            return {"success": True, "message": "Strategy paused", "strategy_id": strategy_id}
        else:
            raise HTTPException(status_code=400, detail="Strategy not found or not running")

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to pause strategy: {str(e)}")


@router.post("/{strategy_id}/resume", response_model=Dict[str, Any])
async def resume_strategy(strategy_id: str, request: Request):
    """
    Resume a paused strategy.

    - **strategy_id**: Strategy to resume

    Returns resume confirmation.
    """
    try:
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        success = await execution_engine.resume_strategy(strategy_id)

        if success:
            return {"success": True, "message": "Strategy resumed", "strategy_id": strategy_id}
        else:
            raise HTTPException(status_code=400, detail="Strategy not found or not paused")

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to resume strategy: {str(e)}")


@router.get("/{strategy_id}/status", response_model=StrategyStatus)
async def get_strategy_status(strategy_id: str, request: Request):
    """
    Get the current status of a strategy.

    - **strategy_id**: Strategy to query

    Returns execution status and performance metrics.
    """
    try:
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        status = execution_engine.get_strategy_status(strategy_id)

        if status is None:
            raise HTTPException(status_code=404, detail="Strategy not found")

        return StrategyStatus(**status)

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error getting strategy status: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Failed to get status: {str(e)}")


# ============================================================================
# Validation & Testing Endpoints
# ============================================================================

@router.post("/{strategy_id}/validate", response_model=ValidationResult)
async def validate_strategy(strategy_id: str):
    """
    Re-validate an existing strategy.

    - **strategy_id**: Strategy to validate

    Returns validation results.
    """
    try:
        # Get strategy file (simplified - would need actual file access)
        # This would need enhancement for actual file retrieval
        validation_result = await get_strategy_validator().validate_existing_strategy(strategy_id)

        return ValidationResult(
            success=validation_result.success,
            message=validation_result.message,
            details=validation_result.details
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error validating strategy: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Validation failed: {str(e)}")
