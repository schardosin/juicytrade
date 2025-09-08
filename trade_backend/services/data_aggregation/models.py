"""
Data models for the aggregation service.
"""

from enum import Enum
from datetime import datetime, date
from typing import Optional, List
from pydantic import BaseModel, Field


class TimeFrame(str, Enum):
    """Supported timeframes for aggregation."""
    FIVE_MIN = "5min"
    FIFTEEN_MIN = "15min"
    THIRTY_MIN = "30min"
    ONE_HOUR = "1hr"
    DAILY = "daily"


class AggregationRequest(BaseModel):
    """Request model for data aggregation."""
    symbol: str = Field(..., description="Symbol to aggregate (e.g., 'SPY', 'SPXW')")
    timeframe: TimeFrame = Field(default=TimeFrame.FIVE_MIN, description="Aggregation timeframe")
    start_date: Optional[str] = Field(None, description="Start date (YYYY-MM-DD)")
    end_date: Optional[str] = Field(None, description="End date (YYYY-MM-DD)")
    market_hours_only: bool = Field(default=True, description="Filter to market hours (9:30 AM - 4:00 PM EST)")
    limit: Optional[int] = Field(None, description="Maximum number of records to return")


class OHLCVData(BaseModel):
    """OHLCV data point."""
    timestamp: datetime = Field(..., description="Timestamp for this data point")
    open: float = Field(..., description="Opening price")
    high: float = Field(..., description="Highest price")
    low: float = Field(..., description="Lowest price")
    close: float = Field(..., description="Closing price")
    volume: int = Field(..., description="Volume traded")


class AggregatedData(BaseModel):
    """Response model for aggregated data."""
    symbol: str = Field(..., description="Symbol")
    timeframe: TimeFrame = Field(..., description="Aggregation timeframe used")
    start_date: date = Field(..., description="Actual start date of data")
    end_date: date = Field(..., description="Actual end date of data")
    market_hours_only: bool = Field(..., description="Whether market hours filtering was applied")
    record_count: int = Field(..., description="Number of data points returned")
    data: List[OHLCVData] = Field(..., description="OHLCV data points")


class MarketHours(BaseModel):
    """Market hours configuration."""
    open_hour: int = Field(default=9, description="Market open hour (EST)")
    open_minute: int = Field(default=30, description="Market open minute")
    close_hour: int = Field(default=16, description="Market close hour (EST)")
    close_minute: int = Field(default=0, description="Market close minute")
    timezone: str = Field(default="America/New_York", description="Market timezone")


class AggregationStats(BaseModel):
    """Statistics about aggregation performance."""
    query_time_ms: float = Field(..., description="Query execution time in milliseconds")
    records_processed: int = Field(..., description="Number of raw records processed")
    records_returned: int = Field(..., description="Number of aggregated records returned")
    cache_hit: bool = Field(default=False, description="Whether result was served from cache")
