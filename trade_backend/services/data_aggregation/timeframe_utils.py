"""
Timeframe utilities for data aggregation.
"""

import logging
from typing import Dict, Tuple
from .models import TimeFrame

logger = logging.getLogger(__name__)


class TimeFrameCalculator:
    """Handles timeframe calculations and SQL generation."""
    
    def __init__(self):
        """Initialize timeframe calculator."""
        self.timeframe_configs = {
            TimeFrame.FIVE_MIN: {
                'minutes': 5,
                'sql_interval': "5 minutes",
                'description': '5-minute bars'
            },
            TimeFrame.FIFTEEN_MIN: {
                'minutes': 15,
                'sql_interval': "15 minutes",
                'description': '15-minute bars'
            },
            TimeFrame.THIRTY_MIN: {
                'minutes': 30,
                'sql_interval': "30 minutes",
                'description': '30-minute bars'
            },
            TimeFrame.ONE_HOUR: {
                'minutes': 60,
                'sql_interval': "1 hour",
                'description': '1-hour bars'
            },
            TimeFrame.DAILY: {
                'minutes': 390,  # 6.5 hours * 60 minutes (market day)
                'sql_interval': "1 day",
                'description': 'Daily bars'
            }
        }
    
    def get_aggregation_sql(self, timeframe: TimeFrame, market_hours_only: bool = True) -> str:
        """
        Generate SQL for timeframe aggregation.
        
        Args:
            timeframe: Target timeframe for aggregation
            market_hours_only: Whether to filter to market hours
            
        Returns:
            SQL query string for aggregation
        """
        config = self.timeframe_configs.get(timeframe)
        if not config:
            raise ValueError(f"Unsupported timeframe: {timeframe}")
        
        # Base aggregation logic
        if timeframe == TimeFrame.DAILY:
            # Daily aggregation: group by date
            time_bucket_sql = "DATE_TRUNC('day', timestamp AT TIME ZONE 'America/New_York')"
        else:
            # Intraday aggregation: use proper time buckets aligned to market open (9:30 AM)
            minutes = config['minutes']
            time_bucket_sql = f"""
            DATE_TRUNC('day', timestamp AT TIME ZONE 'America/New_York') + 
            INTERVAL '9 hours 30 minutes' + 
            (EXTRACT(epoch FROM timestamp AT TIME ZONE 'America/New_York' - DATE_TRUNC('day', timestamp AT TIME ZONE 'America/New_York') - INTERVAL '9 hours 30 minutes') / 60 / {minutes})::int * INTERVAL '{minutes} minutes'
            """
        
        # Market hours filter
        market_hours_filter = ""
        if market_hours_only:
            market_hours_filter = """
            AND (
                -- Market hours: 9:30 AM - 4:00 PM EST/EDT, weekdays only
                (
                    EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') > 9
                    OR (
                        EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') = 9 
                        AND EXTRACT(minute FROM timestamp AT TIME ZONE 'America/New_York') >= 30
                    )
                )
                AND EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') < 16
                AND EXTRACT(dow FROM timestamp AT TIME ZONE 'America/New_York') BETWEEN 1 AND 5
            )
            """
        
        # Complete SQL query
        sql = f"""
        SELECT 
            {time_bucket_sql} as time_bucket,
            FIRST(open / 1e9 ORDER BY timestamp) as open,
            MAX(high / 1e9) as high,
            MIN(low / 1e9) as low,
            LAST(close / 1e9 ORDER BY timestamp) as close,
            SUM(volume) as volume,
            COUNT(*) as record_count
        FROM parquet_data
        WHERE 1=1
        {market_hours_filter}
        GROUP BY time_bucket
        ORDER BY time_bucket
        """
        
        return sql.strip()
    
    def get_timeframe_info(self, timeframe: TimeFrame) -> Dict[str, any]:
        """
        Get information about a timeframe.
        
        Args:
            timeframe: Timeframe to get info for
            
        Returns:
            Dictionary with timeframe information
        """
        config = self.timeframe_configs.get(timeframe)
        if not config:
            raise ValueError(f"Unsupported timeframe: {timeframe}")
        
        return {
            'timeframe': timeframe,
            'minutes': config['minutes'],
            'description': config['description'],
            'sql_interval': config['sql_interval']
        }
    
    def get_supported_timeframes(self) -> Dict[str, str]:
        """
        Get all supported timeframes with descriptions.
        
        Returns:
            Dictionary mapping timeframe values to descriptions
        """
        return {
            tf.value: config['description'] 
            for tf, config in self.timeframe_configs.items()
        }
    
    def validate_timeframe(self, timeframe: str) -> TimeFrame:
        """
        Validate and convert timeframe string to enum.
        
        Args:
            timeframe: Timeframe string to validate
            
        Returns:
            TimeFrame enum value
            
        Raises:
            ValueError: If timeframe is not supported
        """
        try:
            return TimeFrame(timeframe)
        except ValueError:
            supported = list(self.get_supported_timeframes().keys())
            raise ValueError(f"Unsupported timeframe '{timeframe}'. Supported: {supported}")
    
    def estimate_result_size(self, timeframe: TimeFrame, days: int) -> int:
        """
        Estimate number of bars for a given timeframe and date range.
        
        Args:
            timeframe: Target timeframe
            days: Number of trading days
            
        Returns:
            Estimated number of bars
        """
        config = self.timeframe_configs.get(timeframe)
        if not config:
            return 0
        
        if timeframe == TimeFrame.DAILY:
            return days
        
        # Assume 6.5 hours of trading per day
        minutes_per_day = 390  # 6.5 * 60
        bars_per_day = minutes_per_day // config['minutes']
        
        return bars_per_day * days


def get_timeframe_calculator() -> TimeFrameCalculator:
    """
    Get a timeframe calculator instance.
    
    Returns:
        TimeFrameCalculator instance
    """
    return TimeFrameCalculator()
