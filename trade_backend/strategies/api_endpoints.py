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
                "validation_details": validation_result.details
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
async def start_strategy(strategy_id: str, execution_request: StrategyExecutionRequest, request: Request):
    """
    Start executing a strategy.

    - **strategy_id**: Strategy to start
    - **execution_request**: Execution configuration

    Start strategy in live or backtesting mode.
    """
    try:
        # Get strategy instance
        strategy_instance = await get_strategy_registry().get_strategy(strategy_id)
        if not strategy_instance:
            raise HTTPException(status_code=404, detail="Strategy not found")

        # Get execution engine
        execution_engine = get_execution_engine(request)
        if not execution_engine:
            raise HTTPException(status_code=503, detail="Strategy execution engine not available")

        # Merge configurations
        config = execution_request.config or {}
        config['user_id'] = "default_user"
        config['strategy_id'] = strategy_id

        # Start execution
        success = await execution_engine.start_strategy(strategy_id, strategy_instance, config)

        if success:
            logger.info(f"Strategy started: {strategy_id}")
            return {
                "success": True,
                "message": "Strategy started successfully",
                "strategy_id": strategy_id,
                "mode": execution_request.mode
            }
        else:
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


@router.post("/{strategy_id}/backtest", response_model=Dict[str, Any])
async def run_backtest(strategy_id: str, backtest_config: Dict[str, Any]):
    """
    Run a backtest on a strategy.

    - **strategy_id**: Strategy to backtest
    - **backtest_config**: Backtest configuration

    Returns backtest results.
    """
    try:
        logger.info(f"Backtest request for {strategy_id}")

        # This is a placeholder for the actual backtest implementation
        # Would need to be expanded with actual backtesting logic

        # Simulate backtest results
        backtest_results = {
            "total_pnl": -125.50,
            "win_rate": 0.65,
            "total_trades": 28,
            "max_profit": 45.30,
            "max_loss": -75.25,
            "sharpe_ratio": 1.23,
            "period_start": backtest_config.get('start_date'),
            "period_end": backtest_config.get('end_date')
        }

        return {
            "success": True,
            "strategy_id": strategy_id,
            "results": backtest_results
        }

    except Exception as e:
        logger.error(f"Backtest error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Backtest failed: {str(e)}")
