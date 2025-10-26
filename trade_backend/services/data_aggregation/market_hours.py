"""
Market hours utilities for filtering trading data.
"""

import logging
from datetime import datetime, time
from typing import Tuple
import pytz

logger = logging.getLogger(__name__)


class MarketHoursFilter:
    """Handles market hours filtering logic."""
    
    def __init__(self):
        """Initialize market hours filter."""
        self.market_timezone = pytz.timezone('America/New_York')
        self.market_open = time(9, 30)  # 9:30 AM EST
        self.market_close = time(16, 0)  # 4:00 PM EST
    
    def get_market_hours_sql_filter(self) -> str:
        """
        Generate SQL WHERE clause for market hours filtering.
        
        Returns:
            SQL WHERE clause string for DuckDB
        """
        return """
        (
            -- Convert to EST/EDT and check market hours
            EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') > 9
            OR (
                EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') = 9 
                AND EXTRACT(minute FROM timestamp AT TIME ZONE 'America/New_York') >= 30
            )
        )
        AND EXTRACT(hour FROM timestamp AT TIME ZONE 'America/New_York') < 16
        AND EXTRACT(dow FROM timestamp AT TIME ZONE 'America/New_York') BETWEEN 1 AND 5
        """
    
    def is_market_hours(self, timestamp: datetime) -> bool:
        """
        Check if a timestamp falls within market hours.
        
        Args:
            timestamp: Timestamp to check (assumed to be in UTC)
            
        Returns:
            True if within market hours, False otherwise
        """
        try:
            # Convert to market timezone
            if timestamp.tzinfo is None:
                # Assume UTC if no timezone info
                timestamp = pytz.utc.localize(timestamp)
            
            market_time = timestamp.astimezone(self.market_timezone)
            
            # Check if it's a weekday (Monday=0, Sunday=6)
            if market_time.weekday() >= 5:  # Saturday or Sunday
                return False
            
            # Check if within market hours
            current_time = market_time.time()
            return self.market_open <= current_time < self.market_close
            
        except Exception as e:
            logger.warning(f"Error checking market hours for {timestamp}: {e}")
            return False
    
    def get_market_hours_bounds(self) -> Tuple[time, time]:
        """
        Get market open and close times.
        
        Returns:
            Tuple of (market_open_time, market_close_time)
        """
        return self.market_open, self.market_close
    
    def format_market_hours_info(self) -> str:
        """
        Get human-readable market hours information.
        
        Returns:
            Formatted string describing market hours
        """
        return f"Market Hours: {self.market_open.strftime('%I:%M %p')} - {self.market_close.strftime('%I:%M %p')} EST/EDT, Monday-Friday"


def get_market_hours_filter() -> MarketHoursFilter:
    """
    Get a market hours filter instance.
    
    Returns:
        MarketHoursFilter instance
    """
    return MarketHoursFilter()
