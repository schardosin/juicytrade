"""
Framework-level query optimization engine.

This provides reusable SQL query building and optimization utilities that any
strategy or service can use to improve database query performance.
"""

import logging
from datetime import datetime, timedelta
from typing import Optional, List, Dict, Any, Tuple
import math

logger = logging.getLogger(__name__)


class QueryOptimizer:
    """
    Framework-level query optimization engine.
    
    Provides reusable SQL building utilities for common query patterns,
    optimized for performance and usable by any strategy or service.
    
    Features:
    - Timestamp range filtering with proper timezone handling
    - Strike range calculations for options data
    - Column selection optimization
    - Index-friendly query patterns
    - SQL injection prevention
    """
    
    @staticmethod
    def build_timestamp_filter(start_time: datetime, end_time: Optional[datetime] = None,
                              column_name: str = "timestamp", mode: str = "inclusive") -> str:
        """
        Build optimized timestamp filter clause.
        
        Args:
            start_time: Start timestamp (inclusive for "inclusive" mode, target time for "before" mode)
            end_time: End timestamp (exclusive), defaults based on mode
            column_name: Name of timestamp column in database
            mode: "inclusive" (start_time to start_time+1min) or "before" (start_time-1min to start_time)
            
        Returns:
            SQL WHERE clause for timestamp filtering
        """
        if mode == "before":
            # For "before" mode: look in the minute BEFORE the target time
            # If target is 11:20:00, look from 11:19:00 to 11:20:00
            if end_time is None:
                end_time = start_time
                start_time = start_time - timedelta(minutes=1)
        else:
            # Default "inclusive" mode: look from start_time to start_time + 1 minute
            if end_time is None:
                end_time = start_time + timedelta(minutes=1)
        
        # Convert to UTC format for database compatibility
        start_utc = start_time.strftime('%Y-%m-%dT%H:%M:%S-00:00')
        end_utc = end_time.strftime('%Y-%m-%dT%H:%M:%S-00:00')
        
        return f"{column_name} >= '{start_utc}' AND {column_name} < '{end_utc}'"
    
    @staticmethod
    def build_date_filter(date_str: str, column_name: str = "date") -> str:
        """
        Build date filter clause.
        
        Args:
            date_str: Date in YYYY-MM-DD format
            column_name: Name of date column in database
            
        Returns:
            SQL WHERE clause for date filtering
        """
        return f"{column_name} = '{date_str}'"
    
    @staticmethod
    def build_strike_range_filter(center_price: float, range_percent: float = 0.10,
                                 min_range: float = 100.0, column_name: str = "strike") -> str:
        """
        Build strike range filter for options data.
        
        Args:
            center_price: Center price (usually underlying price)
            range_percent: Percentage range around center (0.10 = 10%)
            min_range: Minimum absolute range in dollars
            column_name: Name of strike column in database
            
        Returns:
            SQL WHERE clause for strike filtering
        """
        # Calculate range
        percentage_range = center_price * range_percent
        actual_range = max(percentage_range, min_range)
        
        strike_min = center_price - actual_range
        strike_max = center_price + actual_range
        
        # Round to reasonable precision
        strike_min = math.floor(strike_min / 5) * 5  # Round down to nearest 5
        strike_max = math.ceil(strike_max / 5) * 5   # Round up to nearest 5
        
        return f"{column_name} >= {strike_min} AND {column_name} <= {strike_max}"
    
    @staticmethod
    def build_strike_list_filter(strikes: List[float], column_name: str = "strike") -> str:
        """
        Build strike filter for specific strike values.
        
        Args:
            strikes: List of specific strike prices
            column_name: Name of strike column in database
            
        Returns:
            SQL WHERE clause for strike filtering
        """
        if not strikes:
            return "1=0"  # No strikes = no results
        
        strikes_str = ", ".join([str(float(s)) for s in strikes])
        return f"{column_name} IN ({strikes_str})"
    
    @staticmethod
    def optimize_column_selection(required_fields: List[str], 
                                available_fields: Optional[List[str]] = None) -> str:
        """
        Build optimized column selection clause.
        
        Args:
            required_fields: List of required column names
            available_fields: Optional list of available columns (for validation)
            
        Returns:
            SQL SELECT clause with optimized column selection
        """
        if not required_fields:
            return "*"
        
        # Validate fields if available_fields provided
        if available_fields:
            validated_fields = []
            for field in required_fields:
                if field in available_fields:
                    validated_fields.append(field)
                else:
                    logger.warning(f"Field '{field}' not available, skipping")
            
            if not validated_fields:
                return "*"  # Fallback to all fields
            
            required_fields = validated_fields
        
        # Ensure we always have key fields for options data
        essential_fields = ["symbol", "timestamp", "date"]
        for field in essential_fields:
            if field not in required_fields:
                required_fields.append(field)
        
        return ", ".join(required_fields)
    
    @staticmethod
    def build_options_base_query(symbol: str, expiration: str, timestamp_range: Tuple[datetime, datetime],
                               required_fields: Optional[List[str]] = None,
                               strike_range: Optional[Tuple[float, float]] = None,
                               option_types: Optional[List[str]] = None,
                               timestamp_mode: str = "inclusive") -> str:
        """
        Build optimized base query for options data.
        
        Args:
            symbol: Options symbol (e.g., 'SPXW')
            expiration: Expiration date (YYYY-MM-DD)
            timestamp_range: Tuple of (start_time, end_time)
            required_fields: Optional list of required column names
            strike_range: Optional tuple of (min_strike, max_strike)
            option_types: Optional list of option types ('call', 'put')
            timestamp_mode: "inclusive" or "before" mode for timestamp filtering
            
        Returns:
            Complete SQL query string
        """
        # Build column selection
        if required_fields:
            columns = QueryOptimizer.optimize_column_selection(required_fields)
        else:
            # Default options fields
            columns = "symbol, strike, option_type, expiration, timestamp, bid_px, ask_px, date"
        
        # Build base query - will be completed with file path by caller
        query_parts = [f"SELECT {columns}", "FROM read_parquet('{file_path}')"]
        
        # Build WHERE conditions
        conditions = []
        
        # Expiration filter
        conditions.append(f"expiration = '{expiration}'")
        
        # Timestamp filter with mode support
        start_time, end_time = timestamp_range
        conditions.append(QueryOptimizer.build_timestamp_filter(start_time, end_time, mode=timestamp_mode))
        
        # Strike range filter
        if strike_range:
            min_strike, max_strike = strike_range
            conditions.append(f"strike >= {min_strike} AND strike <= {max_strike}")
        
        # Option type filter
        if option_types:
            types_str = ", ".join([f"'{t}'" for t in option_types])
            conditions.append(f"option_type IN ({types_str})")
        
        # Add basic data quality filters
        conditions.extend([
            "symbol IS NOT NULL",
            "strike IS NOT NULL",
            "option_type IS NOT NULL"
        ])
        
        # Combine conditions
        where_clause = " AND ".join(conditions)
        query_parts.append(f"WHERE {where_clause}")
        
        # Add ordering for consistent results
        query_parts.append("ORDER BY option_type, strike")
        
        return " ".join(query_parts)
    
    @staticmethod
    def build_expiration_discovery_query(symbol: str, date_str: str) -> str:
        """
        Build optimized query for discovering available expirations.
        
        Args:
            symbol: Options symbol
            date_str: Date to check for available expirations
            
        Returns:
            SQL query to find available expirations
        """
        return f"""
        SELECT DISTINCT expiration
        FROM read_parquet('{{file_path}}')
        WHERE date = '{date_str}'
          AND expiration IS NOT NULL
          AND expiration >= '{date_str}'
        ORDER BY expiration
        """
    
    @staticmethod
    def build_bulk_price_query(symbols: List[str], timestamp_range: Tuple[datetime, datetime]) -> str:
        """
        Build optimized query for bulk price lookups.
        
        Args:
            symbols: List of symbols to get prices for
            timestamp_range: Tuple of (start_time, end_time)
            
        Returns:
            SQL query for bulk price lookup
        """
        if not symbols:
            return "SELECT NULL WHERE 1=0"  # Empty result
        
        symbols_str = ", ".join([f"'{s}'" for s in symbols])
        start_time, end_time = timestamp_range
        timestamp_filter = QueryOptimizer.build_timestamp_filter(start_time, end_time)
        
        return f"""
        SELECT symbol, timestamp, bid_px, ask_px,
               (bid_px + ask_px) / 2.0 as mid_px
        FROM read_parquet('{{file_path}}')
        WHERE symbol IN ({symbols_str})
          AND {timestamp_filter}
          AND bid_px IS NOT NULL
          AND ask_px IS NOT NULL
        ORDER BY symbol, timestamp DESC
        """
    
    @staticmethod
    def estimate_query_selectivity(total_rows: int, filters: Dict[str, Any]) -> float:
        """
        Estimate query selectivity for performance planning.
        
        Args:
            total_rows: Total number of rows in dataset
            filters: Dictionary of applied filters with their selectivity
            
        Returns:
            Estimated selectivity (0.0 to 1.0)
        """
        selectivity = 1.0
        
        # Common selectivity estimates for options data
        selectivity_estimates = {
            'expiration': 0.1,      # ~10% of data for specific expiration
            'timestamp_minute': 0.0007,  # ~1/1440 for specific minute
            'timestamp_hour': 0.04,      # ~1/24 for specific hour
            'strike_range_10pct': 0.3,   # ~30% for 10% strike range
            'option_type': 0.5,          # ~50% for calls or puts
        }
        
        for filter_name, filter_value in filters.items():
            if filter_name in selectivity_estimates:
                filter_selectivity = selectivity_estimates[filter_name]
                selectivity *= filter_selectivity
                logger.debug(f"Applied {filter_name} selectivity: {filter_selectivity}")
        
        estimated_rows = int(total_rows * selectivity)
        logger.debug(f"Estimated query will return {estimated_rows} rows (selectivity: {selectivity:.4f})")
        
        return selectivity
    
    @staticmethod
    def suggest_query_optimization(estimated_rows: int, query_complexity: str = "medium") -> List[str]:
        """
        Suggest query optimizations based on estimated result size.
        
        Args:
            estimated_rows: Estimated number of result rows
            query_complexity: Query complexity level ("simple", "medium", "complex")
            
        Returns:
            List of optimization suggestions
        """
        suggestions = []
        
        if estimated_rows > 50000:
            suggestions.append("Consider adding more selective filters (timestamp, strike range)")
            suggestions.append("Use column selection to reduce data transfer")
        
        if estimated_rows > 100000:
            suggestions.append("Query may be too broad - consider breaking into smaller chunks")
            suggestions.append("Enable DuckDB query parallelization")
        
        if query_complexity == "complex" and estimated_rows > 10000:
            suggestions.append("Consider using temporary views for complex multi-stage queries")
        
        if not suggestions:
            suggestions.append("Query appears well-optimized for estimated result size")
        
        return suggestions
