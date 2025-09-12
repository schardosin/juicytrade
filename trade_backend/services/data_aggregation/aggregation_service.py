"""
Main data aggregation service using DuckDB for high-performance OHLCV aggregation.
"""

import logging
import time
from datetime import datetime, date
from pathlib import Path
from typing import Optional, List, Dict, Any
import pandas as pd
import duckdb

from ...path_manager import path_manager
from .models import (
    AggregationRequest, AggregatedData, OHLCVData, TimeFrame, 
    AggregationStats
)
from .market_hours import get_market_hours_filter
from .timeframe_utils import get_timeframe_calculator

logger = logging.getLogger(__name__)


class DataAggregationService:
    """
    High-performance data aggregation service using DuckDB.
    
    Provides flexible timeframe aggregation (5min, 15min, 30min, 1hr, daily)
    with market hours filtering and automatic price scaling correction.
    """
    
    def __init__(self):
        """Initialize the aggregation service."""
        self.conn = duckdb.connect()
        self.parquet_dir = path_manager.data_dir / "parquet"
        self.market_hours_filter = get_market_hours_filter()
        self.timeframe_calculator = get_timeframe_calculator()
        
        # Configure DuckDB for optimal performance
        self._configure_duckdb()
        
        logger.info("DataAggregationService initialized")
    
    def _configure_duckdb(self):
        """Configure DuckDB settings for optimal performance."""
        try:
            # Enable parallel processing
            self.conn.execute("SET threads TO 4")
            
            # Optimize memory usage
            self.conn.execute("SET memory_limit = '2GB'")
            
            # Enable progress bar for long queries (optional)
            self.conn.execute("SET enable_progress_bar = true")
            
            logger.debug("DuckDB configured for optimal performance")
            
        except Exception as e:
            logger.warning(f"Error configuring DuckDB: {e}")
    
    def get_aggregated_data(self, request: AggregationRequest) -> AggregatedData:
        """
        Get aggregated OHLCV data for a symbol and timeframe.
        
        Args:
            request: Aggregation request parameters
            
        Returns:
            AggregatedData with OHLCV bars
            
        Raises:
            ValueError: If symbol data not found or invalid parameters
            Exception: If aggregation fails
        """
        start_time = time.time()
        
        try:
            logger.info(f"Aggregating data for {request.symbol} ({request.timeframe})")
            
            # Validate timeframe
            timeframe = self.timeframe_calculator.validate_timeframe(request.timeframe)
            
            # Build file paths for the symbol
            file_paths = self._get_symbol_file_paths(request.symbol)
            if not file_paths:
                raise ValueError(f"No data found for symbol: {request.symbol}")
            
            # Create DuckDB view from Parquet files
            self._create_parquet_view(file_paths, request.start_date, request.end_date)
            
            # Generate and execute aggregation query
            sql = self._build_aggregation_query(
                timeframe, 
                request.market_hours_only,
                request.start_date,
                request.end_date,
                request.limit
            )
            
            logger.debug(f"Executing aggregation query:\n{sql}")
            
            # Execute query and get results
            result_df = self.conn.execute(sql).df()
            
            if result_df.empty:
                logger.warning(f"No data returned for {request.symbol} ({request.timeframe})")
                return self._create_empty_response(request, timeframe)
            
            # Convert to response format
            aggregated_data = self._convert_to_response(
                result_df, request.symbol, timeframe, request.market_hours_only
            )
            
            # Calculate execution time
            execution_time = (time.time() - start_time) * 1000
            
            logger.info(
                f"Aggregation completed: {len(aggregated_data.data)} bars "
                f"in {execution_time:.1f}ms"
            )
            
            return aggregated_data
            
        except Exception as e:
            logger.error(f"Error aggregating data for {request.symbol}: {e}")
            raise
        finally:
            # Clean up view
            try:
                self.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def _get_symbol_file_paths(self, symbol: str) -> List[str]:
        """
        Get all Parquet file paths for a symbol.
        
        Args:
            symbol: Symbol to find files for
            
        Returns:
            List of file paths
        """
        file_paths = []
        
        # Check all asset type directories in priority order
        # IMPORTANT: Only use ONE asset type to avoid schema conflicts
        asset_dirs = ['equities', 'options', 'futures', 'forex']
        
        for asset_dir in asset_dirs:
            symbol_dir = self.parquet_dir / asset_dir / f"underlying={symbol}"
            
            if symbol_dir.exists():
                # Find all data.parquet files recursively
                parquet_files = list(symbol_dir.rglob("data.parquet"))
                if parquet_files:
                    file_paths.extend([str(f) for f in parquet_files])
                    logger.info(f"Using {asset_dir} data for {symbol} ({len(parquet_files)} files)")
                    # CRITICAL: Only use the first asset type found to avoid schema conflicts
                    break
        
        logger.debug(f"Found {len(file_paths)} files for symbol {symbol}")
        return file_paths
    
    def _get_options_file_paths(self, symbol: str) -> List[str]:
        """
        Get OPTIONS-specific Parquet file paths for a symbol.
        
        This method specifically looks for options data, not equities data.
        
        Args:
            symbol: Symbol to find options files for
            
        Returns:
            List of options file paths
        """
        file_paths = []
        
        # Look specifically in the options directory
        symbol_dir = self.parquet_dir / "options" / f"underlying={symbol}"
        
        if symbol_dir.exists():
            # Find all data.parquet files recursively
            parquet_files = list(symbol_dir.rglob("data.parquet"))
            if parquet_files:
                file_paths.extend([str(f) for f in parquet_files])
                logger.info(f"Using options data for {symbol} ({len(parquet_files)} files)")
        
        logger.debug(f"Found {len(file_paths)} options files for symbol {symbol}")
        return file_paths
    
    def _create_parquet_view(self, file_paths: List[str], 
                           start_date: Optional[str] = None,
                           end_date: Optional[str] = None):
        """
        Create a DuckDB view from Parquet files with optional date filtering.
        
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
        self.conn.execute(create_view_sql)
    
    def _build_aggregation_query(self, 
                               timeframe: TimeFrame,
                               market_hours_only: bool,
                               start_date: Optional[str] = None,
                               end_date: Optional[str] = None,
                               limit: Optional[int] = None) -> str:
        """
        Build the complete aggregation SQL query.
        
        Args:
            timeframe: Target timeframe
            market_hours_only: Whether to filter market hours
            start_date: Optional start date
            end_date: Optional end date
            limit: Optional result limit
            
        Returns:
            Complete SQL query string
        """
        # Get base aggregation SQL
        sql = self.timeframe_calculator.get_aggregation_sql(timeframe, market_hours_only)
        
        # Add additional date filtering if needed (beyond file-level filtering)
        additional_filters = []
        if start_date:
            additional_filters.append(f"date >= '{start_date}'")
        if end_date:
            additional_filters.append(f"date <= '{end_date}'")
        
        if additional_filters:
            # Insert additional WHERE conditions
            where_clause = " AND " + " AND ".join(additional_filters)
            sql = sql.replace("WHERE 1=1", f"WHERE 1=1{where_clause}")
        
        # Add limit if specified
        if limit:
            sql += f" LIMIT {limit}"
        
        return sql
    
    def _convert_to_response(self, 
                           df: pd.DataFrame, 
                           symbol: str, 
                           timeframe: TimeFrame,
                           market_hours_only: bool) -> AggregatedData:
        """
        Convert DataFrame results to AggregatedData response.
        
        Args:
            df: Query result DataFrame
            symbol: Symbol name
            timeframe: Timeframe used
            market_hours_only: Whether market hours filtering was applied
            
        Returns:
            AggregatedData response object
        """
        # Convert DataFrame to OHLCV data points
        ohlcv_data = []
        for _, row in df.iterrows():
            ohlcv_data.append(OHLCVData(
                timestamp=row['time_bucket'],
                open=float(row['open']),
                high=float(row['high']),
                low=float(row['low']),
                close=float(row['close']),
                volume=int(row['volume'])
            ))
        
        # Calculate date range
        if ohlcv_data:
            start_date = ohlcv_data[0].timestamp.date()
            end_date = ohlcv_data[-1].timestamp.date()
        else:
            start_date = end_date = date.today()
        
        return AggregatedData(
            symbol=symbol,
            timeframe=timeframe,
            start_date=start_date,
            end_date=end_date,
            market_hours_only=market_hours_only,
            record_count=len(ohlcv_data),
            data=ohlcv_data
        )
    
    def _create_empty_response(self, 
                             request: AggregationRequest, 
                             timeframe: TimeFrame) -> AggregatedData:
        """
        Create an empty response when no data is found.
        
        Args:
            request: Original request
            timeframe: Validated timeframe
            
        Returns:
            Empty AggregatedData response
        """
        return AggregatedData(
            symbol=request.symbol,
            timeframe=timeframe,
            start_date=date.today(),
            end_date=date.today(),
            market_hours_only=request.market_hours_only,
            record_count=0,
            data=[]
        )
    
    def get_available_symbols(self) -> List[str]:
        """
        Get list of available symbols in the data.
        
        Returns:
            List of available symbol names
        """
        symbols = set()
        
        # Scan all asset type directories
        for asset_dir in ['equities', 'options', 'futures', 'forex']:
            asset_path = self.parquet_dir / asset_dir
            
            if asset_path.exists():
                # Look for underlying=SYMBOL directories
                for symbol_dir in asset_path.glob("underlying=*"):
                    symbol = symbol_dir.name.split('=')[1]
                    symbols.add(symbol)
        
        return sorted(list(symbols))
    
    def get_symbol_date_range(self, symbol: str) -> Optional[Dict[str, str]]:
        """
        Get the available date range for a symbol.
        
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
            logger.error(f"Error getting date range for {symbol}: {e}")
            return None
        finally:
            try:
                self.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def get_supported_timeframes(self) -> Dict[str, str]:
        """
        Get supported timeframes with descriptions.
        
        Returns:
            Dictionary mapping timeframe values to descriptions
        """
        return self.timeframe_calculator.get_supported_timeframes()
    
    def get_available_options_expirations(self, symbol: str, current_time: datetime) -> List[str]:
        """
        Get available options expirations for a symbol.
        
        Args:
            symbol: Symbol to get expirations for
            current_time: Current timestamp
            
        Returns:
            List of expiration dates (YYYY-MM-DD format)
        """
        try:
            # CRITICAL FIX: Use options-specific file path discovery
            file_paths = self._get_options_file_paths(symbol)
            if not file_paths:
                logger.warning(f"No options data found for symbol: {symbol}")
                return []
            
            target_date = current_time.date().strftime('%Y-%m-%d')
            
            # Create temporary view and query unique expirations
            self._create_parquet_view(file_paths, target_date, target_date)
            
            result = self.conn.execute("""
                SELECT DISTINCT expiration
                FROM parquet_data
                WHERE expiration IS NOT NULL
                ORDER BY expiration
            """).fetchall()
            
            expirations = [row[0] for row in result if row[0]]
            logger.info(f"Found {len(expirations)} expirations for {symbol}")
            return expirations
            
        except Exception as e:
            logger.error(f"Error getting expirations for {symbol}: {e}")
            return []
        finally:
            try:
                self.conn.execute("DROP VIEW IF EXISTS parquet_data")
            except:
                pass
    
    def get_options_chain(self, symbol: str, expiration: str, current_time: datetime):
        """
        Get options chain for a symbol and expiration.
        
        Args:
            symbol: Symbol to get chain for
            expiration: Expiration date (YYYY-MM-DD)
            current_time: Current timestamp
            
        Returns:
            OptionsChain object with contracts
        """
        try:
            from ..data_aggregation.price_query_service import get_price_query_service
            from ...strategies.options_models import OptionsChain
            from ...models import OptionContract
            
            # Use our new PriceQueryService for better price lookups
            price_service = get_price_query_service()
            
            # CRITICAL FIX: Use options-specific file path discovery
            file_paths = self._get_options_file_paths(symbol)
            if not file_paths:
                logger.warning(f"No options data found for symbol: {symbol}")
                return None
            
            target_date = current_time.date().strftime('%Y-%m-%d')
            
            # Create temporary view and query contracts for the expiration
            self._create_parquet_view(file_paths, target_date, target_date)
            
            result = self.conn.execute(f"""
                SELECT DISTINCT 
                    symbol,
                    strike,
                    option_type,
                    expiration
                FROM parquet_data
                WHERE expiration = '{expiration}'
                  AND strike IS NOT NULL
                  AND option_type IS NOT NULL
                ORDER BY option_type, strike
            """).fetchall()
            
            contracts = []
            for row in result:
                symbol_name, strike, option_type, exp = row
                
                # Get current price for this contract using our PriceQueryService
                price_data = price_service.get_price_before(symbol_name, current_time)
                
                bid = price_data.bid if price_data else 0.0
                ask = price_data.ask if price_data else 0.0
                
                contract = OptionContract(
                    symbol=symbol_name,
                    strike_price=float(strike),
                    type=option_type,
                    expiration_date=exp,
                    bid=bid,
                    ask=ask,
                    close_price=0.0,  # Not available in CBBO data
                    volume=0,  # Not available in CBBO data
                    underlying_symbol=symbol  # Add underlying symbol
                )
                contracts.append(contract)
            
            if contracts:
                chain = OptionsChain(
                    underlying=symbol,
                    expiration=expiration,
                    timestamp=current_time,
                    contracts=contracts
                )
                logger.info(f"Built options chain for {symbol} exp={expiration} with {len(contracts)} contracts")
                return chain
            else:
                logger.warning(f"No contracts found for {symbol} exp={expiration}")
                return None
            
        except Exception as e:
            logger.error(f"Error getting options chain for {symbol} exp={expiration}: {e}")
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
            logger.info("DataAggregationService closed")
        except Exception as e:
            logger.error(f"Error closing DataAggregationService: {e}")


# Global service instance
_aggregation_service: Optional[DataAggregationService] = None


def get_aggregation_service() -> DataAggregationService:
    """
    Get the global aggregation service instance.
    
    Returns:
        DataAggregationService instance
    """
    global _aggregation_service
    
    if _aggregation_service is None:
        _aggregation_service = DataAggregationService()
    
    return _aggregation_service


def close_aggregation_service():
    """Close the global aggregation service."""
    global _aggregation_service
    
    if _aggregation_service is not None:
        _aggregation_service.close()
        _aggregation_service = None
