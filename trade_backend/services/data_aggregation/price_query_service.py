"""
Enhanced Price Query Service

Provides intelligent price lookups with "closest timestamp before" functionality.
This service solves the Iron Condor strategy's price lookup issues by finding
the most recent price data before a requested timestamp.
"""

import logging
import time
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional, List, Dict, Any, Tuple
import pandas as pd

from ...path_manager import path_manager
from .centralized_data_service import get_centralized_data_service

logger = logging.getLogger(__name__)


class PriceData:
    """Single price data point with bid/ask information."""
    
    def __init__(self, symbol: str, timestamp: datetime, bid: float, ask: float, 
                 bid_size: float, ask_size: float, strike: Optional[float] = None,
                 option_type: Optional[str] = None, expiration: Optional[str] = None):
        self.symbol = symbol
        self.timestamp = timestamp
        self.bid = bid
        self.ask = ask
        self.bid_size = bid_size
        self.ask_size = ask_size
        self.mid = (bid + ask) / 2.0 if bid > 0 and ask > 0 else 0.0
        self.spread = ask - bid if bid > 0 and ask > 0 else 0.0
        
        # Options metadata
        self.strike = strike
        self.option_type = option_type
        self.expiration = expiration
    
    def __repr__(self):
        return (f"PriceData({self.symbol}, {self.timestamp}, "
                f"bid={self.bid}, ask={self.ask}, mid={self.mid:.4f})")


class PriceQueryService:
    """
    Enhanced price query service with "closest timestamp before" functionality.
    
    This service provides intelligent price lookups that find the most recent
    price data before a requested timestamp, solving the exact timestamp
    matching issues in trading strategies.
    """
    
    def __init__(self, max_lookback_minutes: int = 5):
        """
        Initialize the price query service using CentralizedDataService.
        
        Args:
            max_lookback_minutes: Maximum time to look back for closest price (default: 5 minutes)
        """
        self.centralized_service = get_centralized_data_service()
        self.max_lookback_minutes = max_lookback_minutes
        
        logger.info(f"PriceQueryService initialized with CentralizedDataService, {max_lookback_minutes}min lookback")
    
    
    def get_price_before(self, symbol: str, target_time: datetime, 
                        max_lookback_minutes: Optional[int] = None) -> Optional[PriceData]:
        """
        Get the closest price BEFORE the target timestamp using CentralizedDataService.
        
        This is the core method that implements "closest timestamp before" logic.
        If you request a price at 13:30:00, it will find the most recent price
        before that time (e.g., 13:29:58.961).
        
        Args:
            symbol: Symbol to get price for
            target_time: Target timestamp to look before
            max_lookback_minutes: Override default lookback time
            
        Returns:
            PriceData for the closest timestamp before target_time, or None if not found
        """
        lookback_minutes = max_lookback_minutes or self.max_lookback_minutes
        start_time = time.time()
        
        try:
            # Convert timezone-aware datetime to timezone-naive for compatibility with CBBO data
            if target_time.tzinfo is not None:
                target_time = target_time.replace(tzinfo=None)
            
            logger.debug(f"Getting price before {target_time} for {symbol}")
            
            # Calculate lookback window
            lookback_start = target_time - timedelta(minutes=lookback_minutes)
            
            # Extract underlying symbol for file path lookup
            underlying = symbol.split()[0] if ' ' in symbol else symbol
            
            # Use CentralizedDataService to get exact file path
            target_date = target_time.date().strftime('%Y-%m-%d')
            exact_path = self.centralized_service.get_exact_symbol_date_path(underlying, target_date)
            
            if not exact_path:
                logger.warning(f"No data file found for underlying: {underlying} on {target_date}")
                return None
            
            # Build "closest before" query using CentralizedDataService connection
            sql = self._build_closest_before_query_with_file(symbol, target_time, lookback_start, exact_path)
            
            logger.debug(f"Executing closest-before query via CentralizedDataService:\n{sql}")
            
            # Execute query using CentralizedDataService connection (no connection closing issues!)
            result = self.centralized_service.conn.execute(sql).fetchone()
            
            if not result:
                logger.debug(f"No price found before {target_time} for {symbol} "
                           f"(looked back {lookback_minutes} minutes)")
                return None
            
            # Convert to PriceData object
            price_data = PriceData(
                symbol=symbol,
                timestamp=result[0],  # timestamp
                bid=float(result[1]) if result[1] and result[1] > 0 else 0.0,  # bid_px
                ask=float(result[2]) if result[2] and result[2] > 0 else 0.0,  # ask_px
                bid_size=float(result[3]) if result[3] and result[3] > 0 else 0.0,  # bid_sz
                ask_size=float(result[4]) if result[4] and result[4] > 0 else 0.0,  # ask_sz
                strike=float(result[5]) if result[5] else None,  # strike
                option_type=result[6] if result[6] else None,  # option_type
                expiration=result[7] if result[7] else None   # expiration
            )
            
            # Calculate how far back we found the price
            time_diff = (target_time - price_data.timestamp).total_seconds()
            
            # Calculate execution time
            execution_time = (time.time() - start_time) * 1000
            
            logger.info(f"✅ Found price for {symbol} at {price_data.timestamp} "
                       f"({time_diff:.1f}s before {target_time}) in {execution_time:.1f}ms")
            
            return price_data
            
        except Exception as e:
            logger.error(f"Error getting price before {target_time} for {symbol}: {e}")
            return None
        # No finally block - CentralizedDataService manages its own connection!
    
    def get_prices_before_batch(self, symbols: List[str], target_time: datetime,
                               max_lookback_minutes: Optional[int] = None) -> Dict[str, Optional[PriceData]]:
        """
        Get closest prices BEFORE target timestamp for multiple symbols at once.
        
        This is optimized for Iron Condor strategies that need prices for 4 option legs
        simultaneously. Much more efficient than individual queries.
        
        Args:
            symbols: List of symbols to get prices for
            target_time: Target timestamp to look before
            max_lookback_minutes: Override default lookback time
            
        Returns:
            Dictionary mapping symbol to PriceData (or None if not found)
        """
        lookback_minutes = max_lookback_minutes or self.max_lookback_minutes
        start_time = time.time()
        
        try:
            logger.info(f"Getting batch prices before {target_time} for {len(symbols)} symbols")
            
            # Calculate lookback window
            lookback_start = target_time - timedelta(minutes=lookback_minutes)
            
            # Group symbols by underlying to optimize file access
            symbol_groups = self._group_symbols_by_underlying(symbols)
            
            results = {}
            
            for underlying, symbol_list in symbol_groups.items():
                try:
                    # Get file paths for this underlying
                    file_paths = self._get_symbol_file_paths(underlying)
                    if not file_paths:
                        logger.warning(f"No data files found for underlying: {underlying}")
                        for symbol in symbol_list:
                            results[symbol] = None
                        continue
                    
                    # Create DuckDB view from Parquet files
                    target_date = target_time.date().strftime('%Y-%m-%d')
                    self._create_parquet_view(file_paths, target_date, target_date)
                    
                    # Build batch query for all symbols in this underlying
                    sql = self._build_batch_closest_before_query(symbol_list, target_time, lookback_start)
                    
                    logger.debug(f"Executing batch query for {underlying}:\n{sql}")
                    
                    # Execute query and process results using CentralizedDataService
                    query_results = self.centralized_service.conn.execute(sql).fetchall()
                    
                    # Convert results to PriceData objects
                    for row in query_results:
                        symbol = row[8]  # symbol is the 9th column (index 8)
                        price_data = PriceData(
                            symbol=symbol,
                            timestamp=row[0],  # timestamp
                            bid=float(row[1]) if row[1] and row[1] > 0 else 0.0,  # bid_px
                            ask=float(row[2]) if row[2] and row[2] > 0 else 0.0,  # ask_px
                            bid_size=float(row[3]) if row[3] and row[3] > 0 else 0.0,  # bid_sz
                            ask_size=float(row[4]) if row[4] and row[4] > 0 else 0.0,  # ask_sz
                            strike=float(row[5]) if row[5] else None,  # strike
                            option_type=row[6] if row[6] else None,  # option_type
                            expiration=row[7] if row[7] else None   # expiration
                        )
                        results[symbol] = price_data
                    
                    # Mark symbols not found as None
                    for symbol in symbol_list:
                        if symbol not in results:
                            results[symbol] = None
                            logger.debug(f"No price found before {target_time} for {symbol}")
                
                except Exception as e:
                    logger.error(f"Error in batch query for underlying {underlying}: {e}")
                    # Mark all symbols in this group as None
                    for symbol in symbol_list:
                        results[symbol] = None
            
            # Calculate execution time
            execution_time = (time.time() - start_time) * 1000
            found_count = sum(1 for v in results.values() if v is not None)
            
            logger.info(f"✅ Batch query completed: {found_count}/{len(symbols)} prices found "
                       f"in {execution_time:.1f}ms")
            
            return results
            
        except Exception as e:
            logger.error(f"Error in batch price query: {e}")
            return {symbol: None for symbol in symbols}
        finally:
            # Clean up view
            try:
                self.centralized_service.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def _build_closest_before_query_with_file(self, symbol: str, target_time: datetime, 
                                            lookback_start: datetime, file_path: str) -> str:
        """
        Build SQL query to find closest timestamp before target time using exact file path.
        
        Args:
            symbol: Specific symbol to query for
            target_time: Target timestamp to look before
            lookback_start: Earliest time to consider
            file_path: Exact path to parquet file
            
        Returns:
            SQL query string
        """
        # Convert timezone-aware datetime to timezone-naive for compatibility with CBBO data
        if target_time.tzinfo is not None:
            target_time = target_time.replace(tzinfo=None)
        if lookback_start.tzinfo is not None:
            lookback_start = lookback_start.replace(tzinfo=None)
        
        sql = f"""
        SELECT 
            timestamp,
            bid_px,
            ask_px,
            bid_sz,
            ask_sz,
            strike,
            option_type,
            expiration
        FROM read_parquet('{file_path}')
        WHERE symbol = '{symbol}'
          AND timestamp < '{target_time.isoformat()}'
          AND timestamp >= '{lookback_start.isoformat()}'
        ORDER BY timestamp DESC
        LIMIT 1
        """
        
        return sql.strip()

    def _build_closest_before_query(self, symbol: str, target_time: datetime, lookback_start: datetime) -> str:
        """
        Build SQL query to find closest timestamp before target time for a specific symbol.
        
        Args:
            symbol: Specific symbol to query for
            target_time: Target timestamp to look before
            lookback_start: Earliest time to consider
            
        Returns:
            SQL query string
        """
        # Convert timezone-aware datetime to timezone-naive for compatibility with CBBO data
        if target_time.tzinfo is not None:
            target_time = target_time.replace(tzinfo=None)
        if lookback_start.tzinfo is not None:
            lookback_start = lookback_start.replace(tzinfo=None)
        
        sql = f"""
        SELECT 
            timestamp,
            bid_px,
            ask_px,
            bid_sz,
            ask_sz,
            strike,
            option_type,
            expiration
        FROM parquet_data
        WHERE symbol = '{symbol}'
          AND timestamp < '{target_time.isoformat()}'
          AND timestamp >= '{lookback_start.isoformat()}'
        ORDER BY timestamp DESC
        LIMIT 1
        """
        
        return sql.strip()
    
    def _build_batch_closest_before_query(self, symbols: List[str], target_time: datetime, 
                                        lookback_start: datetime) -> str:
        """
        Build SQL query to find closest timestamps before target time for multiple symbols.
        
        Args:
            symbols: List of symbols to query
            target_time: Target timestamp to look before
            lookback_start: Earliest time to consider
            
        Returns:
            SQL query string
        """
        # Convert timezone-aware datetime to timezone-naive for compatibility with CBBO data
        if target_time.tzinfo is not None:
            target_time = target_time.replace(tzinfo=None)
        if lookback_start.tzinfo is not None:
            lookback_start = lookback_start.replace(tzinfo=None)
        
        # Create symbol list for SQL IN clause
        symbol_list = "', '".join(symbols)
        
        sql = f"""
        WITH ranked_prices AS (
            SELECT 
                timestamp,
                bid_px,
                ask_px,
                bid_sz,
                ask_sz,
                strike,
                option_type,
                expiration,
                symbol,
                ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY timestamp DESC) as rn
            FROM parquet_data
            WHERE symbol IN ('{symbol_list}')
              AND timestamp < '{target_time.isoformat()}'
              AND timestamp >= '{lookback_start.isoformat()}'
        )
        SELECT 
            timestamp,
            bid_px,
            ask_px,
            bid_sz,
            ask_sz,
            strike,
            option_type,
            expiration,
            symbol
        FROM ranked_prices
        WHERE rn = 1
        ORDER BY symbol
        """
        
        return sql.strip()
    
    def _group_symbols_by_underlying(self, symbols: List[str]) -> Dict[str, List[str]]:
        """
        Group symbols by their underlying asset for efficient batch queries.
        
        Args:
            symbols: List of symbols to group
            
        Returns:
            Dictionary mapping underlying symbol to list of option symbols
        """
        groups = {}
        
        for symbol in symbols:
            # Extract underlying from option symbol (e.g., "SPY   250812C00643000" -> "SPY")
            underlying = symbol.split()[0] if ' ' in symbol else symbol
            
            if underlying not in groups:
                groups[underlying] = []
            groups[underlying].append(symbol)
        
        return groups
    
    def _get_symbol_wildcard_path(self, symbol: str) -> str:
        """
        Get wildcard path for a symbol using DuckDB's partition pruning.

        Instead of collecting file paths manually, return a wildcard path that
        DuckDB can use for efficient file pruning based on query filters.

        Args:
            symbol: Symbol to find files for

        Returns:
            Wildcard path string for DuckDB read_parquet
        """
        # Check all asset type directories
        asset_dirs = ['options', 'equities', 'futures', 'forex']

        for asset_dir in asset_dirs:
            symbol_dir = self.centralized_service.parquet_dir / asset_dir / f"underlying={symbol}"

            if symbol_dir.exists():
                # Use wildcard path, let DuckDB handle file discovery and partitioning
                wildcard_path = str(symbol_dir / "**" / "data.parquet")
                logger.debug(f"Using wildcard path for {symbol}: {wildcard_path}")
                return wildcard_path

        logger.warning(f"No data directory found for symbol {symbol}")
        return ""

    def _get_symbol_file_paths(self, symbol: str) -> List[str]:
        """
        LEGACY: Get all Parquet file paths for a symbol (underlying).
        Only kept for backward compatibility with methods not yet updated.

        Args:
            symbol: Symbol/underlying to find files for

        Returns:
            List of file paths
        """
        file_paths = []

        # Check all asset type directories
        # Priority: options > equities > futures > forex
        asset_dirs = ['options', 'equities', 'futures', 'forex']

        for asset_dir in asset_dirs:
            symbol_dir = self.centralized_service.parquet_dir / asset_dir / f"underlying={symbol}"

            if symbol_dir.exists():
                # Find all data.parquet files recursively
                parquet_files = list(symbol_dir.rglob("data.parquet"))
                if parquet_files:
                    file_paths.extend([str(f) for f in parquet_files])
                    logger.debug(f"Using {asset_dir} data for {symbol} ({len(parquet_files)} files)")
                    # Use the first asset type found to avoid schema conflicts
                    break

        return file_paths
    
    def _create_parquet_view(self, file_paths: List[str], 
                           start_date: Optional[str] = None,
                           end_date: Optional[str] = None):
        """
        Create a DuckDB view from Parquet files with date filtering.
        
        Args:
            file_paths: List of Parquet file paths
            start_date: Optional start date filter (YYYY-MM-DD)
            end_date: Optional end date filter (YYYY-MM-DD)
        """
        # Build file list for DuckDB
        files_str = ", ".join([f"'{path}'" for path in file_paths])
        
        # Base query
        base_query = f"SELECT * FROM read_parquet([{files_str}])"
        
        # Add date filtering if specified
        where_conditions = []
        if start_date:
            where_conditions.append(f"date >= '{start_date}'")
        if end_date:
            where_conditions.append(f"date <= '{end_date}'")
        
        if where_conditions:
            base_query += " WHERE " + " AND ".join(where_conditions)
        
        # Create view
        create_view_sql = f"CREATE OR REPLACE VIEW parquet_data AS {base_query}"

        logger.debug(f"Creating parquet view with {len(file_paths)} files")
        self.centralized_service.conn.execute(create_view_sql)

    def _create_parquet_view_from_wildcard(self, wildcard_path: str,
                                         start_date: Optional[str] = None,
                                         end_date: Optional[str] = None):
        """
        Create a DuckDB view from a wildcard path, letting DuckDB handle file discovery.

        Args:
            wildcard_path: Wildcard path like "parquet/options/underlying=SPXW/**/*.parquet"
            start_date: Optional start date filter (YYYY-MM-DD)
            end_date: Optional end date filter (YYYY-MM-DD)
        """
        # Base query using wildcard - DuckDB will handle partition pruning
        base_query = f"SELECT * FROM read_parquet('{wildcard_path}')"

        # Add date filtering if specified
        where_conditions = []
        if start_date:
            where_conditions.append(f"date >= '{start_date}'")
        if end_date:
            where_conditions.append(f"date <= '{end_date}'")

        if where_conditions:
            base_query += " WHERE " + " AND ".join(where_conditions)

        # Create view
        create_view_sql = f"CREATE OR REPLACE VIEW parquet_data AS {base_query}"

        logger.debug(f"Creating parquet view from wildcard: {wildcard_path}")
        self.centralized_service.conn.execute(create_view_sql)
    
    def close(self):
        """Close method for compatibility. CentralizedDataService manages its own connection."""
        logger.info("PriceQueryService close() called - CentralizedDataService manages connections")


# Global service instance
_price_query_service: Optional[PriceQueryService] = None


def get_price_query_service() -> PriceQueryService:
    """
    Get the global price query service instance.
    
    Returns:
        PriceQueryService instance
    """
    global _price_query_service
    
    if _price_query_service is None:
        _price_query_service = PriceQueryService()
    
    return _price_query_service


def close_price_query_service():
    """Close the global price query service."""
    global _price_query_service
    
    if _price_query_service is not None:
        _price_query_service.close()
        _price_query_service = None
