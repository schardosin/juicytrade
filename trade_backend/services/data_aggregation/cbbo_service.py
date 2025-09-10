"""
CBBO (Consolidated Best Bid and Offer) data service.

This service provides raw CBBO data for backtesting without aggregation.
CBBO data contains bid_px, ask_px, bid_sz, ask_sz for each timestamp.
"""

import logging
import time
from datetime import datetime, date
from pathlib import Path
from typing import Optional, List, Dict, Any
import pandas as pd
import duckdb

from ...path_manager import path_manager

logger = logging.getLogger(__name__)


class CBBOData:
    """Single CBBO data point."""
    
    def __init__(self, timestamp: datetime, bid: float, ask: float, 
                 bid_size: float, ask_size: float):
        self.timestamp = timestamp
        self.bid = bid
        self.ask = ask
        self.bid_size = bid_size
        self.ask_size = ask_size
        self.mid = (bid + ask) / 2.0 if bid > 0 and ask > 0 else 0.0
        self.spread = ask - bid if bid > 0 and ask > 0 else 0.0


class CBBODataService:
    """
    CBBO data service for backtesting.
    
    Provides raw CBBO data without aggregation - just bid, ask, and mid price
    for each timestamp. No OHLCV conversion needed.
    """
    
    def __init__(self):
        """Initialize the CBBO service."""
        self.conn = duckdb.connect()
        self.parquet_dir = path_manager.data_dir / "parquet"
        
        # Configure DuckDB for optimal performance
        self._configure_duckdb()
        
        logger.info("CBBODataService initialized")
    
    def _configure_duckdb(self):
        """Configure DuckDB settings for optimal performance."""
        try:
            # Enable parallel processing
            self.conn.execute("SET threads TO 4")
            
            # Optimize memory usage
            self.conn.execute("SET memory_limit = '2GB'")
            
            logger.debug("DuckDB configured for CBBO data access")
            
        except Exception as e:
            logger.warning(f"Error configuring DuckDB: {e}")
    
    def get_cbbo_data(self, symbol: str, start_date: Optional[str] = None, 
                      end_date: Optional[str] = None, 
                      start_time: Optional[datetime] = None,
                      end_time: Optional[datetime] = None,
                      limit: Optional[int] = None) -> List[CBBOData]:
        """
        Get raw CBBO data for a symbol and date range.
        
        Args:
            symbol: Symbol to get data for
            start_date: Start date (YYYY-MM-DD)
            end_date: End date (YYYY-MM-DD)
            start_time: Start datetime (for intraday filtering)
            end_time: End datetime (for intraday filtering)
            limit: Optional result limit
            
        Returns:
            List of CBBOData points
            
        Raises:
            ValueError: If symbol data not found
            Exception: If query fails
        """
        start_query_time = time.time()
        
        try:
            logger.info(f"Getting CBBO data for {symbol}")
            
            # Build file paths for the symbol
            file_paths = self._get_symbol_file_paths(symbol)
            if not file_paths:
                raise ValueError(f"No CBBO data found for symbol: {symbol}")
            
            # Create DuckDB view from Parquet files
            self._create_parquet_view(file_paths, start_date, end_date)
            
            # Build and execute query
            sql = self._build_cbbo_query(start_time, end_time, limit)
            
            logger.debug(f"Executing CBBO query:\n{sql}")
            
            # Execute query and get results
            result_df = self.conn.execute(sql).df()
            
            if result_df.empty:
                logger.warning(f"No CBBO data returned for {symbol}")
                return []
            
            # Convert to CBBOData objects
            cbbo_data = []
            for _, row in result_df.iterrows():
                cbbo_data.append(CBBOData(
                    timestamp=row['timestamp'],
                    bid=float(row['bid_px']) if row['bid_px'] > 0 else 0.0,
                    ask=float(row['ask_px']) if row['ask_px'] > 0 else 0.0,
                    bid_size=float(row['bid_sz']) if row['bid_sz'] > 0 else 0.0,
                    ask_size=float(row['ask_sz']) if row['ask_sz'] > 0 else 0.0
                ))
            
            # Calculate execution time
            execution_time = (time.time() - start_query_time) * 1000
            
            logger.info(
                f"CBBO data retrieved: {len(cbbo_data)} records "
                f"in {execution_time:.1f}ms"
            )
            
            return cbbo_data
            
        except Exception as e:
            logger.error(f"Error getting CBBO data for {symbol}: {e}")
            raise
        finally:
            # Clean up view
            try:
                self.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def get_cbbo_at_time(self, symbol: str, target_time: datetime) -> Optional[CBBOData]:
        """
        Get CBBO data for a specific timestamp.
        
        Args:
            symbol: Symbol to get data for
            target_time: Target timestamp
            
        Returns:
            CBBOData for the timestamp, or None if not found
        """
        try:
            # Get data for the target date with a small time window
            start_time = target_time.replace(second=0, microsecond=0)
            end_time = target_time.replace(second=59, microsecond=999999)
            
            cbbo_data = self.get_cbbo_data(
                symbol=symbol,
                start_time=start_time,
                end_time=end_time,
                limit=1000  # Get multiple records to find closest match
            )
            
            if not cbbo_data:
                return None
            
            # Find the closest timestamp
            closest_data = min(cbbo_data, 
                             key=lambda x: abs((x.timestamp - target_time).total_seconds()))
            
            return closest_data
            
        except Exception as e:
            logger.error(f"Error getting CBBO data at {target_time} for {symbol}: {e}")
            return None
    
    def _get_symbol_file_paths(self, symbol: str) -> List[str]:
        """
        Get all CBBO Parquet file paths for a symbol.
        
        Args:
            symbol: Symbol to find files for
            
        Returns:
            List of file paths
        """
        file_paths = []
        
        # Check all asset type directories for CBBO data
        # Priority: options > equities > futures > forex
        asset_dirs = ['options', 'equities', 'futures', 'forex']
        
        for asset_dir in asset_dirs:
            symbol_dir = self.parquet_dir / asset_dir / f"underlying={symbol}"
            
            if symbol_dir.exists():
                # Find all data.parquet files recursively
                parquet_files = list(symbol_dir.rglob("data.parquet"))
                if parquet_files:
                    file_paths.extend([str(f) for f in parquet_files])
                    logger.info(f"Using {asset_dir} CBBO data for {symbol} ({len(parquet_files)} files)")
                    # Use the first asset type found to avoid schema conflicts
                    break
        
        logger.debug(f"Found {len(file_paths)} CBBO files for symbol {symbol}")
        return file_paths
    
    def _create_parquet_view(self, file_paths: List[str], 
                           start_date: Optional[str] = None,
                           end_date: Optional[str] = None):
        """
        Create a DuckDB view from CBBO Parquet files.
        
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
        
        logger.debug(f"Creating CBBO parquet view with {len(file_paths)} files")
        self.conn.execute(create_view_sql)
    
    def _build_cbbo_query(self, start_time: Optional[datetime] = None,
                         end_time: Optional[datetime] = None,
                         limit: Optional[int] = None) -> str:
        """
        Build CBBO data query.
        
        Args:
            start_time: Optional start datetime
            end_time: Optional end datetime
            limit: Optional result limit
            
        Returns:
            SQL query string
        """
        # Base query - just select CBBO fields, no aggregation
        sql = """
        SELECT 
            timestamp,
            bid_px,
            ask_px,
            bid_sz,
            ask_sz
        FROM parquet_data
        WHERE 1=1
        """
        
        # Add time filtering if specified
        if start_time:
            sql += f" AND timestamp >= '{start_time.isoformat()}'"
        if end_time:
            sql += f" AND timestamp <= '{end_time.isoformat()}'"
        
        # Order by timestamp
        sql += " ORDER BY timestamp"
        
        # Add limit if specified
        if limit:
            sql += f" LIMIT {limit}"
        
        return sql.strip()
    
    def get_available_symbols(self) -> List[str]:
        """
        Get list of available symbols with CBBO data.
        
        Returns:
            List of available symbol names
        """
        symbols = set()
        
        # Scan all asset type directories
        for asset_dir in ['options', 'equities', 'futures', 'forex']:
            asset_path = self.parquet_dir / asset_dir
            
            if asset_path.exists():
                # Look for underlying=SYMBOL directories
                for symbol_dir in asset_path.glob("underlying=*"):
                    symbol = symbol_dir.name.split('=')[1]
                    symbols.add(symbol)
        
        return sorted(list(symbols))
    
    def get_symbol_date_range(self, symbol: str) -> Optional[Dict[str, str]]:
        """
        Get the available date range for a symbol's CBBO data.
        
        Args:
            symbol: Symbol to check
            
        Returns:
            Dictionary with start_date and end_date, or None if symbol not found
        """
        try:
            file_paths = self._get_symbol_file_paths(symbol)
            if not file_paths:
                return None
            
            # Create temporary view and query date range
            self._create_parquet_view(file_paths)
            
            result = self.conn.execute("""
                SELECT 
                    MIN(date) as start_date,
                    MAX(date) as end_date
                FROM parquet_data
            """).fetchone()
            
            if result and result[0] and result[1]:
                return {
                    'start_date': str(result[0]),
                    'end_date': str(result[1])
                }
            
            return None
            
        except Exception as e:
            logger.error(f"Error getting CBBO date range for {symbol}: {e}")
            return None
        finally:
            try:
                self.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def close(self):
        """Close the DuckDB connection."""
        try:
            self.conn.close()
            logger.info("CBBODataService closed")
        except Exception as e:
            logger.error(f"Error closing CBBODataService: {e}")


# Global service instance
_cbbo_service: Optional[CBBODataService] = None


def get_cbbo_service() -> CBBODataService:
    """
    Get the global CBBO service instance.
    
    Returns:
        CBBODataService instance
    """
    global _cbbo_service
    
    if _cbbo_service is None:
        _cbbo_service = CBBODataService()
    
    return _cbbo_service


def close_cbbo_service():
    """Close the global CBBO service."""
    global _cbbo_service
    
    if _cbbo_service is not None:
        _cbbo_service.close()
        _cbbo_service = None
