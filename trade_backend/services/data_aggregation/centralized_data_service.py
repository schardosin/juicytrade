"""
Centralized Data Service - Single DuckDB connection for all data operations.

This service centralizes all Parquet data access to eliminate file handle issues
and provide efficient, consistent data querying with exact path discovery.
"""

import logging
import time
from datetime import datetime, date
from pathlib import Path
from typing import Optional, List, Dict, Any
import pandas as pd
import duckdb

from ...path_manager import path_manager
from ...strategies.options_models import OptionsChain, OptionContract

logger = logging.getLogger(__name__)


class CentralizedDataService:
    """
    Single, centralized data service for all DuckDB operations.

    This service maintains one DuckDB connection for the entire application
    and uses exact date paths to avoid wildcard file handle explosions.

    Key differences from distributed services:
    - Single connection, no conflicts
    - Exact paths, no mass file discovery
    - Centralized queries, consistent resource usage
    """

    def __init__(self):
        """Initialize the centralized data service with one persistent connection."""
        self.conn = duckdb.connect()
        self.parquet_dir = path_manager.data_dir / "parquet"

        # Configure DuckDB for optimal performance
        self._configure_duckdb()

        logger.info("CentralizedDataService initialized with single persistent connection")

    def _configure_duckdb(self):
        """Configure DuckDB settings for optimal performance."""
        try:
            # Enable parallel processing
            self.conn.execute("SET threads TO 4")

            # Optimize memory usage
            self.conn.execute("SET memory_limit = '2GB'")

            logger.debug("DuckDB configured for centralized operations")

        except Exception as e:
            logger.warning(f"Error configuring DuckDB: {e}")

    def get_exact_symbol_date_path(self, symbol: str, date_str: str) -> Optional[str]:
        """
        Get the exact Parquet file path for a symbol and date.

        This builds exact paths like:
        parquet/options/underlying=SPX/year=2025/month=08/day=12/data.parquet

        Instead of wildcards, this avoids opening hundreds of files.

        Args:
            symbol: Symbol to find data for
            date_str: Date in YYYY-MM-DD format

        Returns:
            Exact path to data file, or None if not found
        """
        year, month, day = date_str.split('-')

        # Check all asset types
        asset_dirs = ['options', 'equities', 'futures', 'forex']

        for asset_dir in asset_dirs:
            symbol_dir = self.parquet_dir / asset_dir / f"underlying={symbol}"
            target_dir = symbol_dir / f"year={year}" / f"month={month}" / f"day={day}"
            target_file = target_dir / "data.parquet"

            if target_file.exists():
                logger.debug(f"Found exact path: {target_file}")
                return str(target_file)

        logger.debug(f"No data file found for {symbol} on {date_str}")
        return None

    def query_symbol_at_date(self, symbol: str, date_str: str, filters: Optional[str] = None) -> pd.DataFrame:
        """
        Query a symbol's data for a specific date using DuckDB with exact file path.
        DuckDB is much more efficient than pandas for parquet, but we ensure it
        only opens exactly one file by providing the exact path.

        Args:
            symbol: Symbol to query
            date_str: Date in YYYY-MM-DD format
            filters: Optional additional WHERE clause

        Returns:
            DataFrame with query results

        Raises:
            ValueError: If data file not found
        """
        # Get exact path - MUST be exact single file
        exact_path = self.get_exact_symbol_date_path(symbol, date_str)
        if not exact_path:
            raise ValueError(f"No data found for {symbol} on {date_str}")

        logger.debug(f"Querying single parquet file with DuckDB: {exact_path}")

        try:
            start_time = time.time()

            # Build DuckDB query for single file
            base_query = f"SELECT * FROM read_parquet('{exact_path}')"
            if filters:
                base_query += f" WHERE {filters}"

            logger.debug(f"DUCKDB query: {base_query}")

            # Execute with DuckDB - only opens ONE file
            df = self.conn.execute(base_query).df()

            # Convert date column to string format for consistency
            if 'date' in df.columns and df['date'].dtype != 'object':
                df['date'] = df['date'].astype(str)

            execution_time = (time.time() - start_time) * 1000
            logger.debug(f"DuckDB query completed: {len(df)} rows in {execution_time:.1f}ms")
            return df

        except Exception as e:
            logger.error(f"Error querying {exact_path}: {e}")
            raise

    def get_options_chain(self, symbol: str, expiration: str, current_time: datetime,
                         strikes_around_atm: Optional[int] = None,
                         underlying_symbol: Optional[str] = None,
                         underlying_price: Optional[float] = None) -> Optional[OptionsChain]:
        """
        Get options chain for a symbol and expiration using exact date path.

        This replaces the distributed services approach with a single, efficient
        query using exact paths to avoid file handle exhaustion.

        Args:
            symbol: Options symbol (like SPXW)
            expiration: Expiration date (YYYY-MM-DD)
            current_time: Current timestamp
            strikes_around_atm: Number of strikes to get around at-the-money
            underlying_symbol: Underlying symbol for price lookup
            underlying_price: Known underlying price

        Returns:
            OptionsChain or None if not found
        """
        try:
            date_str = current_time.date().strftime('%Y-%m-%d')

            # Get underlying price for ATM calculations
            if underlying_price is None:
                underlying_price = self._get_underlying_price_at_time(
                    underlying_symbol or symbol,
                    current_time
                )
                if underlying_price is None:
                    logger.warning(f"No underlying price found for pricing strikes")
                    return None

            # OPTIMIZATION: Only query contracts active in the specific minute
            # IMPORTANT: Convert to UTC format (-00:00) since database stores timestamps in UTC
            from datetime import timedelta
            minute_start = current_time.replace(second=0, microsecond=0)
            minute_end = minute_start + timedelta(minutes=1)

            # Format to UTC ISO string (database doesn't handle timezone offsets properly)
            minute_start_utc = minute_start.strftime('%Y-%m-%dT%H:%M:%S-00:00')
            minute_end_utc = minute_end.strftime('%Y-%m-%dT%H:%M:%S-00:00')

            # Base filters with timestamp constraint
            base_filters = f"""
                expiration = '{expiration}'
                AND strike IS NOT NULL
                AND option_type IS NOT NULL
                AND timestamp >= '{minute_start_utc}'
                AND timestamp < '{minute_end_utc}'
            """

            # Build complete filter query
            filters = base_filters

            if strikes_around_atm:
                # First get available strikes in this minute to calculate ATM range
                strikes_df = self.query_symbol_at_date(symbol, date_str,
                    f"{base_filters} AND strike IS NOT NULL"
                )

                if not strikes_df.empty:
                    # Find closest strike to underlying price
                    strikes_df['price_diff'] = (strikes_df['strike'] - underlying_price).abs()
                    closest_strike = strikes_df.loc[strikes_df['price_diff'].idxmin(), 'strike']

                    # Get range around the ATM strike
                    strike_min = closest_strike - (strikes_around_atm * 50)  # Rough range
                    strike_max = closest_strike + (strikes_around_atm * 50)

                    filters += f" AND strike >= {strike_min} AND strike <= {strike_max}"

            # Execute main query
            options_df = self.query_symbol_at_date(symbol, date_str, filters)

            if options_df.empty:
                logger.warning(f"No options data found for {symbol} exp={expiration}")
                return None

            # Convert to OptionContract objects
            contracts = []
            for _, row in options_df.iterrows():
                try:
                    # Get price data for this contract
                    contract_symbol = str(row['symbol'])

                    # Try to get price from the same date (will fail if no listing, but that's OK)
                    try:
                        contract_price = self._get_price_at_time(contract_symbol, current_time)
                        bid = contract_price['bid'] if contract_price else 0.0
                        ask = contract_price['ask'] if contract_price else 0.0
                    except:
                        bid = ask = 0.0

                    contract = OptionContract(
                        symbol=contract_symbol,
                        strike_price=float(row['strike']),
                        type=str(row['option_type']),
                        expiration_date=str(row['expiration']),
                        bid=bid,
                        ask=ask,
                        close_price=0.0,  # Not available in CBBO data
                        volume=0,  # Not available in CBBO data
                        underlying_symbol=symbol
                    )
                    contracts.append(contract)

                except Exception as e:
                    logger.warning(f"Error creating contract for {row.get('symbol')}: {e}")
                    continue

            if not contracts:
                logger.warning(f"No valid contracts found")
                return None

            # Sort by type and strike
            contracts.sort(key=lambda x: (x.type, x.strike_price))

            chain = OptionsChain(
                underlying=symbol,
                expiration=expiration,
                timestamp=current_time,
                contracts=contracts
            )

            logger.info(f"Built options chain for {symbol} exp={expiration} with {len(contracts)} contracts")
            return chain

        except Exception as e:
            logger.error(f"Error getting options chain for {symbol} exp={expiration}: {e}")
            return None

    def _get_underlying_price_at_time(self, symbol: str, target_time: datetime) -> Optional[float]:
        """
        Get underlying price at a specific time.

        Args:
            symbol: Underlying symbol
            target_time: Target timestamp

        Returns:
            Price as float, or None if not found
        """
        try:
            date_str = target_time.date().strftime('%Y-%m-%d')

            # Get data for underlying symbol
            df = self.query_symbol_at_date(symbol, date_str)

            if df.empty:
                return None

            # Find closest timestamp before target time
            df['timestamp_diff'] = (pd.to_datetime(df['timestamp']) - target_time).dt.total_seconds()
            before_mask = df['timestamp_diff'] <= 0

            if not before_mask.any():
                return None

            closest_row = df[before_mask].loc[df[before_mask]['timestamp_diff'].idxmax()]

            # Use mid price
            bid = closest_row.get('bid_px', 0)
            ask = closest_row.get('ask_px', 0)

            if bid > 0 and ask > 0:
                return (bid + ask) / 2.0
            elif ask > 0:
                return ask
            elif bid > 0:
                return bid

            return None

        except Exception as e:
            logger.warning(f"Error getting underlying price for {symbol}: {e}")
            return None

    def _get_price_at_time(self, symbol: str, target_time: datetime) -> Optional[Dict[str, float]]:
        """
        Get price data (bid/ask) for a symbol at a specific time.

        Args:
            symbol: Symbol to get price for
            target_time: Target timestamp

        Returns:
            Dict with 'bid' and 'ask' keys, or None if not found
        """
        try:
            date_str = target_time.date().strftime('%Y-%m-%d')

            # Get data for symbol
            df = self.query_symbol_at_date(symbol, date_str)

            if df.empty:
                return None

            # Find closest timestamp before target time
            df['timestamp_diff'] = (pd.to_datetime(df['timestamp']) - target_time).dt.total_seconds()
            before_mask = df['timestamp_diff'] <= 0

            if not before_mask.any():
                return None

            closest_row = df[before_mask].loc[df[before_mask]['timestamp_diff'].idxmax()]

            bid = closest_row.get('bid_px', 0.0)
            ask = closest_row.get('ask_px', 0.0)

            return {'bid': bid, 'ask': ask}

        except Exception as e:
            logger.debug(f"Error getting price for {symbol}: {e}")
            return None

    def get_aggregated_data(self, symbol: str, timeframe: str, start_date: Optional[str] = None,
                          end_date: Optional[str] = None) -> pd.DataFrame:
        """
        Get aggregated OHLCV data for a symbol and timeframe.

        This is a simplified version for the centralized service.
        For full aggregation logic, the existing aggregation_service should be used.

        Args:
            symbol: Symbol to aggregate
            timeframe: Timeframe for aggregation
            start_date: Start date filter
            end_date: End date filter

        Returns:
            DataFrame with aggregated data
        """
        # This would need the TimeFrameCalculator from the existing service
        # For now, return empty DataFrame to maintain interface
        logger.warning("Aggregated data method not fully implemented in centralized service")
        return pd.DataFrame()

    def shut_down(self):
        """Shut down the centralized data service."""
        try:
            self.conn.close()
            logger.info("CentralizedDataService shut down")
        except Exception as e:
            logger.error(f"Error shutting down CentralizedDataService: {e}")


# Global singleton instance
_centralized_data_service: Optional[CentralizedDataService] = None


def get_centralized_data_service() -> CentralizedDataService:
    """
    Get the global centralized data service instance.

    Returns:
        The singleton CentralizedDataService instance
    """
    global _centralized_data_service

    if _centralized_data_service is None:
        _centralized_data_service = CentralizedDataService()

    return _centralized_data_service


def shutdown_centralized_data_service():
    """Shut down the global centralized data service."""
    global _centralized_data_service

    if _centralized_data_service is not None:
        _centralized_data_service.shut_down()
        _centralized_data_service = None
