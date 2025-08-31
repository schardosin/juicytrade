"""
Automated Trading Strategies Module

This module contains the core implementation for the automated trading system,
including strategy execution, data management, and user strategy validation.
"""

from .base_strategy import BaseStrategy, StrategyResult
from .data_provider import StrategyDataProvider, LiveDataProvider, BacktestDataProvider
from .execution_engine import StrategyExecutionEngine
from .strategy_registry import StrategyRegistry
from .strategy_validator import StrategyValidator
from .order_executor import OrderExecutor

__all__ = [
    'BaseStrategy',
    'StrategyResult',
    'StrategyDataProvider',
    'LiveDataProvider',
    'BacktestDataProvider',
    'StrategyExecutionEngine',
    'StrategyRegistry',
    'StrategyValidator',
    'OrderExecutor'
]