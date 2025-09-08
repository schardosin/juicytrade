"""
Data Aggregation Service

Provides flexible timeframe aggregation for market data with market hours filtering.
Supports 5min, 15min, 30min, 1hr, and daily aggregations using DuckDB for performance.
"""

from .aggregation_service import DataAggregationService, get_aggregation_service
from .models import AggregationRequest, AggregatedData, TimeFrame

__all__ = [
    'DataAggregationService',
    'get_aggregation_service',
    'AggregationRequest', 
    'AggregatedData',
    'TimeFrame'
]
