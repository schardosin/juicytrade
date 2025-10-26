"""
Parquet Writer

Handles conversion of DBN data to partitioned Parquet format for efficient
backtesting queries. Provides optimized storage with proper partitioning
by date, symbol, and asset type.
"""

import logging
from datetime import datetime, date, timedelta
from pathlib import Path
from typing import Dict, List, Optional, Any, Iterator
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from concurrent.futures import ThreadPoolExecutor
import os
import threading
import tempfile
import shutil

from ...path_manager import path_manager
from .import_models import (
    ImportFilters, DataType, AssetType, ImportProgress,
    SymbolInfo, DateRange
)
from .csv_reader import CSVRecord

logger = logging.getLogger(__name__)


class ParquetWriter:
    """
    Handles conversion of DBN data to partitioned Parquet format.
    """
    
    def __init__(self):
        """Initialize Parquet writer."""
        # Set up output directory structure
        self.parquet_dir = path_manager.data_dir / "parquet"
        self.parquet_dir.mkdir(exist_ok=True)
        
        # Create subdirectories for different asset types
        self.options_dir = self.parquet_dir / "options"
        self.equities_dir = self.parquet_dir / "equities"
        self.futures_dir = self.parquet_dir / "futures"
        self.forex_dir = self.parquet_dir / "forex"
        
        for dir_path in [self.options_dir, self.equities_dir, self.futures_dir, self.forex_dir]:
            dir_path.mkdir(exist_ok=True)
        
        # File locking mechanism to prevent concurrent writes to same partition
        self._file_locks = {}
        self._locks_lock = threading.Lock()
    
    def _get_file_lock(self, file_path: Path) -> threading.Lock:
        """
        Get or create a lock for a specific file path.
        
        Args:
            file_path: Path to the file
            
        Returns:
            Threading lock for the file
        """
        file_key = str(file_path)
        
        with self._locks_lock:
            if file_key not in self._file_locks:
                self._file_locks[file_key] = threading.Lock()
            return self._file_locks[file_key]
    
    def _atomic_write_parquet(self, df: pd.DataFrame, output_path: Path) -> bool:
        """
        Atomically write DataFrame to parquet file using temporary file.
        
        Args:
            df: DataFrame to write
            output_path: Final output path
            
        Returns:
            True if successful, False otherwise
        """
        try:
            # Create temporary file in the same directory
            temp_dir = output_path.parent
            temp_dir.mkdir(parents=True, exist_ok=True)
            
            with tempfile.NamedTemporaryFile(
                dir=temp_dir, 
                suffix='.parquet.tmp', 
                delete=False
            ) as temp_file:
                temp_path = Path(temp_file.name)
            
            # Create PyArrow table with explicit schema to avoid dictionary encoding issues
            schema = self._create_consistent_schema(df)
            table = pa.Table.from_pandas(df, schema=schema)
            
            pq.write_table(
                table,
                temp_path,
                compression='snappy',
                use_dictionary=False,  # Disable dictionary encoding to avoid compatibility issues
                write_statistics=True
            )
            
            # Atomic move to final location
            shutil.move(str(temp_path), str(output_path))
            
            return True
            
        except Exception as e:
            logger.error(f"Error in atomic write to {output_path}: {e}")
            # Clean up temp file if it exists
            try:
                if 'temp_path' in locals() and temp_path.exists():
                    temp_path.unlink()
            except Exception:
                pass
            return False
    
    def convert_dbn_to_parquet(self, 
                              records_iterator: Iterator[Any],
                              metadata: Dict[str, Any],
                              progress_callback: Optional[callable] = None) -> List[str]:
        """
        Convert ALL DBN records to partitioned Parquet files without any filtering.
        
        Args:
            records_iterator: Iterator of DBN records (all records, no filtering)
            metadata: Metadata about the source file
            progress_callback: Optional progress callback
            
        Returns:
            List of output file paths created
        """
        logger.info("Starting complete DBN to Parquet conversion (no filtering)")
        
        output_paths = []
        processed_records = 0
        batch_size = 10000  # Process in chunks
        
        # Determine asset type from metadata
        file_asset_type = self._determine_asset_type_from_metadata(metadata)
        
        try:
            # Use chunked processing for memory efficiency
            current_batch = {}  # Group by (underlying_symbol, date, asset_type)
            
            for record in records_iterator:
                try:
                    # Extract record information (records already have symbols from DBN reader)
                    record_info = self._extract_record_info_with_context(
                        record, file_asset_type
                    )
                    
                    if not record_info:
                        continue
                    
                    symbol = record_info['symbol']
                    underlying_symbol = record_info['underlying_symbol']
                    record_date = record_info['date']
                    asset_type = record_info['asset_type']
                    
                    # Group by underlying symbol and date (no filtering)
                    key = (underlying_symbol, record_date, asset_type)
                    if key not in current_batch:
                        current_batch[key] = []
                    
                    current_batch[key].append(record_info)
                    processed_records += 1
                    
                    # Process batch when it gets large enough
                    if processed_records % batch_size == 0:
                        batch_output_paths = self._process_batch(current_batch, metadata)
                        output_paths.extend(batch_output_paths)
                        current_batch.clear()  # Free memory
                        
                        # Progress update
                        if progress_callback:
                            progress_callback(
                                processed_records, 
                                None,  # Total unknown for streaming
                                f"Processed {processed_records:,} records"
                            )
                
                except Exception as e:
                    logger.warning(f"Error processing record: {e}")
                    continue
            
            # Process final batch
            if current_batch:
                batch_output_paths = self._process_batch(current_batch, metadata)
                output_paths.extend(batch_output_paths)
            
            logger.info(f"Successfully converted {processed_records:,} records to {len(output_paths)} Parquet files")
            
            return output_paths
            
        except Exception as e:
            logger.error(f"Error in complete DBN to Parquet conversion: {e}")
            raise
    

    def _process_batch(self, batch: Dict[tuple, List[Dict[str, Any]]], metadata: Dict[str, Any]) -> List[str]:
        """
        Process a batch of records and write to Parquet files.
        
        Args:
            batch: Dictionary mapping (underlying_symbol, date, asset_type) to records
            metadata: Source metadata
            
        Returns:
            List of output file paths created
        """
        output_paths = []
        
        for (underlying_symbol, record_date, asset_type), records in batch.items():
            try:
                output_path = self._write_symbol_date_partition_to_parquet(
                    underlying_symbol, record_date, asset_type, records, metadata
                )
                
                if output_path:
                    output_paths.append(output_path)
                    
            except Exception as e:
                logger.error(f"Error writing batch partition for {underlying_symbol} {record_date}: {e}")
                continue
        
        return output_paths

    def _extract_record_info(self, record: Any, file_asset_type: Optional[AssetType] = None, symbol_mapping: Optional[Dict[int, str]] = None) -> Optional[Dict[str, Any]]:
        """
        Extract relevant information from a DBN record.
        
        Args:
            record: DBN record
            file_asset_type: Asset type determined from file metadata
            symbol_mapping: Optional mapping from instrument_id to readable symbol
            
        Returns:
            Dictionary with extracted information or None if extraction fails
        """
        try:
            # Extract basic fields - this will need to be adapted based on actual DBN record structure
            record_info = {}
            
            # Symbol - use mapping if available, with fallback for unmapped records
            symbol = None
            if hasattr(record, 'instrument_id') and symbol_mapping:
                instrument_id = record.instrument_id
                symbol = symbol_mapping.get(instrument_id)
                if not symbol:
                    # Fallback: use instrument_id as symbol instead of skipping
                    symbol = f"UNKNOWN_{instrument_id}"
                    logger.debug(f"Using fallback symbol for unmapped instrument_id: {instrument_id}")
            else:
                # Fallback to existing logic
                symbol = getattr(record, 'symbol', None) or str(getattr(record, 'instrument_id', ''))
            
            if not symbol:
                return None
            record_info['symbol'] = symbol
            
            # Extract underlying symbol for partitioning
            underlying_symbol = self._extract_underlying_symbol(symbol)
            record_info['underlying_symbol'] = underlying_symbol
            
            # Timestamp and date
            timestamp = getattr(record, 'ts_event', None) or getattr(record, 'timestamp', None)
            if timestamp:
                dt = datetime.fromtimestamp(timestamp / 1e9)
                record_info['timestamp'] = dt
                record_info['date'] = dt.date()
            else:
                # Fallback to current date
                record_info['date'] = date.today()
                record_info['timestamp'] = datetime.now()
            
            # Asset type determination - use file metadata if available
            record_info['asset_type'] = file_asset_type or self._determine_asset_type(symbol)
            
            # Price data with proper DBN scaling (1e-9 according to Databento docs)
            # Apply scaling during import so aggregation queries are clean
            if hasattr(record, 'open'):
                raw_open = getattr(record, 'open', 0)
                record_info['open'] = self._scale_dbn_price(raw_open)
            if hasattr(record, 'high'):
                raw_high = getattr(record, 'high', 0)
                record_info['high'] = self._scale_dbn_price(raw_high)
            if hasattr(record, 'low'):
                raw_low = getattr(record, 'low', 0)
                record_info['low'] = self._scale_dbn_price(raw_low)
            if hasattr(record, 'close'):
                raw_close = getattr(record, 'close', 0)
                record_info['close'] = self._scale_dbn_price(raw_close)
            if hasattr(record, 'volume'):
                record_info['volume'] = int(getattr(record, 'volume', 0))
            
            # Trade data
            if hasattr(record, 'price'):
                record_info['price'] = float(getattr(record, 'price', 0))
            if hasattr(record, 'size'):
                record_info['size'] = int(getattr(record, 'size', 0))
            
            # Quote data
            if hasattr(record, 'bid_px'):
                record_info['bid'] = float(getattr(record, 'bid_px', 0))
            if hasattr(record, 'ask_px'):
                record_info['ask'] = float(getattr(record, 'ask_px', 0))
            if hasattr(record, 'bid_sz'):
                record_info['bid_size'] = int(getattr(record, 'bid_sz', 0))
            if hasattr(record, 'ask_sz'):
                record_info['ask_size'] = int(getattr(record, 'ask_sz', 0))
            
            return record_info
            
        except Exception as e:
            logger.warning(f"Error extracting record info: {e}")
            return None
    
    def _scale_dbn_price(self, raw_price: Any) -> float:
        """
        Scale DBN price according to Databento documentation.
        
        According to Databento docs: "every 1 unit corresponds to 1e-9, 
        i.e. 1/1,000,000,000 or 0.000000001"
        
        UNDEF_PRICE (9223372036854775807 = INT64_MAX) represents null/undefined prices.
        
        Args:
            raw_price: Raw price value from DBN (string or int)
            
        Returns:
            Scaled price as float (0.0 for null/undefined prices)
        """
        try:
            if raw_price is None or raw_price == 0:
                return 0.0
            
            # Convert to int if it's a string
            if isinstance(raw_price, str):
                price_int = int(raw_price)
            else:
                price_int = int(raw_price)
            
            # Handle UNDEF_PRICE (INT64_MAX) as per Databento specification
            # "UNDEF_PRICE is used to denote a null or undefined price.
            # It will be equal to 9223372036854775807 (INT64_MAX)"
            UNDEF_PRICE = 9223372036854775807  # INT64_MAX
            if price_int == UNDEF_PRICE:
                return 0.0  # Return 0 for null/undefined prices
            
            # Apply 1e-9 scaling as per Databento documentation
            scaled_price = price_int / 1e9
            
            return float(scaled_price)
            
        except (ValueError, TypeError) as e:
            logger.warning(f"Error scaling DBN price {raw_price}: {e}")
            return 0.0
    
    def _determine_asset_type_from_metadata(self, metadata: Dict[str, Any]) -> Optional[AssetType]:
        """
        Determine asset type from file metadata.
        
        Args:
            metadata: File metadata dictionary
            
        Returns:
            AssetType enum value or None if cannot be determined
        """
        # Check dataset field
        dataset = metadata.get('dataset', '').lower()
        if 'opra' in dataset:
            return AssetType.OPTIONS
        elif 'sip' in dataset:
            return AssetType.EQUITIES
        
        # Check asset_types from metadata
        asset_types = metadata.get('asset_types', [])
        if asset_types and len(asset_types) > 0:
            # Convert string to AssetType enum if needed
            first_asset_type = asset_types[0]
            if isinstance(first_asset_type, str):
                try:
                    return AssetType(first_asset_type)
                except ValueError:
                    pass
            elif isinstance(first_asset_type, AssetType):
                return first_asset_type
        
        # Check filename for hints
        filename = metadata.get('filename', '').lower()
        if 'opra' in filename:
            return AssetType.OPTIONS
        elif 'sip' in filename:
            return AssetType.EQUITIES
        
        return None
    
    def _extract_underlying_symbol(self, symbol: str) -> str:
        """
        Extract underlying symbol from option symbol.
        
        Args:
            symbol: Full option symbol (e.g., "SPXW  250804C05500000")
            
        Returns:
            Underlying symbol (e.g., "SPXW")
        """
        try:
            # For option symbols like "SPXW  250804C05500000", extract the underlying
            if ' ' in symbol:
                # Split on spaces and take the first part
                underlying = symbol.split()[0].strip()
                if underlying:
                    return underlying
            
            # For symbols without spaces, try to extract alphabetic prefix
            # This handles cases like "SPXW250804C05500000"
            alphabetic_part = ''
            for char in symbol:
                if char.isalpha():
                    alphabetic_part += char
                else:
                    break
            
            if alphabetic_part:
                return alphabetic_part
            
            # Fallback: return the original symbol
            return symbol
            
        except Exception as e:
            logger.warning(f"Error extracting underlying symbol from {symbol}: {e}")
            return symbol
    
    def _determine_asset_type(self, symbol: str) -> AssetType:
        """
        Determine asset type from symbol.
        
        Args:
            symbol: Symbol string
            
        Returns:
            AssetType enum value
        """
        # Basic heuristics for asset type detection
        if len(symbol) > 10 or (any(char in symbol for char in ['C', 'P']) and any(char.isdigit() for char in symbol)):
            return AssetType.OPTIONS
        elif len(symbol) <= 5 and symbol.isalpha():
            return AssetType.EQUITIES
        elif '/' in symbol or any(symbol.endswith(suffix) for suffix in ['USD', 'EUR', 'GBP']):
            return AssetType.FOREX
        else:
            return AssetType.EQUITIES
    
    
    def _write_partition_to_parquet(self, 
                                   symbol: str, 
                                   record_date: date, 
                                   asset_type: AssetType,
                                   records: List[Dict[str, Any]],
                                   metadata: Dict[str, Any]) -> Optional[str]:
        """
        Write a partition of records to Parquet file.
        
        Args:
            symbol: Symbol for this partition
            record_date: Date for this partition
            asset_type: Asset type
            records: List of record dictionaries
            metadata: Source metadata
            
        Returns:
            Output file path or None if failed
        """
        try:
            if not records:
                return None
            
            # Convert records to DataFrame
            df = pd.DataFrame(records)
            
            # Ensure consistent data types
            df = self._normalize_dataframe(df)
            
            # Determine output path
            output_path = self._get_partition_path(symbol, record_date, asset_type, metadata)
            output_path.parent.mkdir(parents=True, exist_ok=True)
            
            # Write to Parquet with compression
            table = pa.Table.from_pandas(df)
            pq.write_table(
                table, 
                output_path,
                compression='snappy',
                use_dictionary=True,
                write_statistics=True
            )
            
            logger.debug(f"Wrote {len(records)} records to {output_path}")
            return str(output_path)
            
        except Exception as e:
            logger.error(f"Error writing partition to Parquet: {e}")
            return None
    
    def _normalize_dataframe(self, df: pd.DataFrame) -> pd.DataFrame:
        """
        Normalize DataFrame data types for consistent Parquet storage.
        
        Args:
            df: Input DataFrame
            
        Returns:
            Normalized DataFrame
        """
        # Filter out invalid timestamps (out of pandas range)
        if 'timestamp' in df.columns:
            try:
                # Convert to datetime and filter out invalid timestamps
                df['timestamp'] = pd.to_datetime(df['timestamp'], errors='coerce')
                
                # Filter out timestamps outside valid range (1677-2262)
                valid_mask = (
                    (df['timestamp'] >= pd.Timestamp('1677-01-01')) & 
                    (df['timestamp'] <= pd.Timestamp('2262-01-01'))
                )
                
                if not valid_mask.all():
                    invalid_count = (~valid_mask).sum()
                    logger.warning(f"Filtering out {invalid_count} records with invalid timestamps")
                    df = df[valid_mask].copy()
                    
            except Exception as e:
                logger.error(f"Error normalizing timestamps: {e}")
                # Fallback: use current timestamp for invalid ones
                df['timestamp'] = pd.Timestamp.now()
        
        # Ensure date is string type for consistent schema compatibility
        if 'date' in df.columns:
            try:
                # Convert to datetime first for validation
                df['date'] = pd.to_datetime(df['date'], errors='coerce')
                
                # Filter out invalid dates (NaT values)
                valid_date_mask = df['date'].notna()
                if not valid_date_mask.all():
                    invalid_count = (~valid_date_mask).sum()
                    logger.warning(f"Filtering out {invalid_count} records with invalid dates")
                    df = df[valid_date_mask].copy()
                
                # Convert to string format for consistent schema compatibility
                df['date'] = df['date'].dt.strftime('%Y-%m-%d')
                    
            except Exception as e:
                logger.error(f"Error normalizing dates: {e}")
        
        # Ensure expiration is string type (for options)
        if 'expiration' in df.columns:
            try:
                df['expiration'] = pd.to_datetime(df['expiration'], errors='coerce')
                # Convert to string format for consistency
                df['expiration'] = df['expiration'].dt.strftime('%Y-%m-%d')
            except Exception as e:
                logger.warning(f"Error normalizing expiration dates: {e}")
                df['expiration'] = df['expiration'].astype(str)
        
        # Ensure numeric columns are proper types (CBBO + legacy OHLCV)
        numeric_columns = [
            'open', 'high', 'low', 'close', 'price', 'bid', 'ask',  # Legacy
            'bid_px', 'ask_px', 'strike'  # CBBO + Options
        ]
        for col in numeric_columns:
            if col in df.columns:
                df[col] = pd.to_numeric(df[col], errors='coerce')
        
        # Ensure integer columns (CBBO + legacy)
        int_columns = [
            'volume', 'size', 'bid_size', 'ask_size',  # Legacy
            'bid_sz', 'ask_sz'  # CBBO
        ]
        for col in int_columns:
            if col in df.columns:
                df[col] = pd.to_numeric(df[col], errors='coerce').fillna(0).astype('int64')
        
        # Ensure string columns with consistent types (including options metadata)
        string_columns = [
            'symbol', 'asset_type', 'underlying_symbol',  # Core
            'option_type'  # Options
        ]
        for col in string_columns:
            if col in df.columns:
                # Convert to string and ensure consistent categorical encoding
                df[col] = df[col].astype(str)
        
        return df
    
    def _ensure_schema_compatibility(self, existing_df: pd.DataFrame, new_df: pd.DataFrame) -> pd.DataFrame:
        """
        Ensure new DataFrame is compatible with existing DataFrame schema.
        
        Args:
            existing_df: Existing DataFrame from file
            new_df: New DataFrame to append
            
        Returns:
            New DataFrame with compatible schema
        """
        try:
            # Ensure all columns from existing DataFrame exist in new DataFrame
            for col in existing_df.columns:
                if col not in new_df.columns:
                    # Add missing column with appropriate default value
                    if existing_df[col].dtype == 'object':
                        new_df[col] = ''
                    elif pd.api.types.is_numeric_dtype(existing_df[col]):
                        new_df[col] = 0
                    elif pd.api.types.is_datetime64_any_dtype(existing_df[col]):
                        new_df[col] = pd.NaT
                    else:
                        new_df[col] = None
            
            # Ensure column order matches existing DataFrame
            new_df = new_df.reindex(columns=existing_df.columns, fill_value=None)
            
            # Ensure data types match existing DataFrame with special handling for date columns
            for col in existing_df.columns:
                if col in new_df.columns:
                    try:
                        # Special handling for date columns - convert both to strings for compatibility
                        if col == 'date':
                            # Convert both existing and new data to string format for compatibility
                            if len(existing_df) > 0:
                                existing_df[col] = existing_df[col].astype(str)
                            if len(new_df) > 0:
                                new_df[col] = new_df[col].astype(str)
                        elif col == 'expiration':
                            # Handle expiration date column - convert to strings
                            if len(existing_df) > 0:
                                existing_df[col] = existing_df[col].astype(str)
                            if len(new_df) > 0:
                                new_df[col] = new_df[col].astype(str)
                        elif col in ['underlying', 'underlying_symbol', 'symbol', 'asset_type', 'option_type']:
                            # Handle string fields that may have dictionary encoding issues
                            if len(existing_df) > 0:
                                existing_df[col] = existing_df[col].astype(str)
                            if len(new_df) > 0:
                                new_df[col] = new_df[col].astype(str)
                        elif existing_df[col].dtype == 'object' or pd.api.types.is_string_dtype(existing_df[col]):
                            # Handle string columns to avoid dictionary encoding issues
                            # Convert both to string to ensure compatibility
                            new_df[col] = new_df[col].astype(str)
                            # Also ensure existing data is string type (not categorical/dictionary)
                            if hasattr(existing_df[col], 'cat'):
                                existing_df[col] = existing_df[col].astype(str)
                        else:
                            # Handle other data types
                            new_df[col] = new_df[col].astype(existing_df[col].dtype)
                    except Exception as e:
                        logger.warning(f"Could not convert column {col} to match existing type: {e}")
                        # Keep original type if conversion fails
                        pass
            
            return new_df
            
        except Exception as e:
            logger.error(f"Error ensuring schema compatibility: {e}")
            return new_df
    
    def _create_consistent_schema(self, df: pd.DataFrame) -> pa.Schema:
        """
        Create a consistent PyArrow schema for the DataFrame to avoid dictionary encoding issues.
        
        Args:
            df: DataFrame to create schema for
            
        Returns:
            PyArrow schema with consistent types
        """
        try:
            schema_fields = []
            
            for col in df.columns:
                if col == 'timestamp':
                    schema_fields.append(pa.field(col, pa.timestamp('ns')))
                elif col == 'date':
                    # Use string type for date columns to avoid conversion issues
                    schema_fields.append(pa.field(col, pa.string()))
                elif col == 'expiration':
                    # Use string type for expiration columns to avoid conversion issues
                    schema_fields.append(pa.field(col, pa.string()))
                elif col in ['bid_px', 'ask_px', 'strike']:
                    schema_fields.append(pa.field(col, pa.float64()))
                elif col in ['bid_sz', 'ask_sz', 'volume', 'size']:
                    schema_fields.append(pa.field(col, pa.int64()))
                elif col in ['symbol', 'underlying_symbol', 'asset_type', 'option_type', 'underlying']:
                    # Use string type instead of dictionary to avoid encoding issues
                    schema_fields.append(pa.field(col, pa.string()))
                elif col in ['year', 'month', 'day']:
                    schema_fields.append(pa.field(col, pa.int32()))
                else:
                    # Default to string for unknown columns
                    schema_fields.append(pa.field(col, pa.string()))
            
            return pa.schema(schema_fields)
            
        except Exception as e:
            logger.warning(f"Error creating consistent schema: {e}")
            # Fallback to inferred schema
            return pa.Table.from_pandas(df).schema
    
    def _get_partition_path(self, 
                           symbol: str, 
                           record_date: date, 
                           asset_type: AssetType,
                           metadata: Dict[str, Any]) -> Path:
        """
        Get the output path for a partition.
        
        Args:
            symbol: Symbol
            record_date: Date
            asset_type: Asset type
            metadata: Source metadata
            
        Returns:
            Path for the partition file
        """
        # Determine base directory by asset type
        if asset_type == AssetType.OPTIONS:
            base_dir = self.options_dir
        elif asset_type == AssetType.EQUITIES:
            base_dir = self.equities_dir
        elif asset_type == AssetType.FUTURES:
            base_dir = self.futures_dir
        else:
            base_dir = self.forex_dir
        
        # Create partitioned path: asset_type/year=YYYY/month=MM/day=DD/symbol=SYMBOL/data.parquet
        year = record_date.year
        month = record_date.month
        day = record_date.day
        
        partition_path = (
            base_dir / 
            f"year={year}" / 
            f"month={month:02d}" / 
            f"day={day:02d}" / 
            f"symbol={symbol}" / 
            "data.parquet"
        )
        
        return partition_path
    
    def _write_symbol_date_partition_to_parquet(self,
                                               underlying_symbol: str,
                                               record_date: date,
                                               asset_type: AssetType,
                                               records: List[Dict[str, Any]],
                                               metadata: Dict[str, Any]) -> Optional[str]:
        """
        Write or append a symbol-date partition of records to Parquet file.
        If the file already exists, appends new records to existing data.
        
        CRITICAL FIX: This method now properly handles concurrent writes and ensures
        true appending without data loss during batch processing using file locking
        and atomic write operations.

        Args:
            underlying_symbol: Underlying symbol for this partition (e.g., "SPXW")
            record_date: Date for this partition
            asset_type: Asset type
            records: List of record dictionaries
            metadata: Source metadata

        Returns:
            Output file path or None if failed
        """
        try:
            if not records:
                return None

            # Convert new records to DataFrame
            new_df = pd.DataFrame(records)
            new_df = self._normalize_dataframe(new_df)

            # Determine output path for symbol-date partitioning
            output_path = self._get_symbol_date_partition_path(underlying_symbol, record_date, asset_type, metadata)
            output_path.parent.mkdir(parents=True, exist_ok=True)

            # Get file-specific lock to prevent concurrent writes to same partition
            file_lock = self._get_file_lock(output_path)
            
            with file_lock:
                # Critical section: read, combine, and write atomically
                if output_path.exists():
                    # File exists - read existing data and append
                    try:
                        # Read with consistent schema to avoid dictionary encoding issues
                        existing_table = pq.read_table(output_path)
                        existing_df = existing_table.to_pandas()
                        
                        # Remove partition columns that PyArrow automatically adds from directory structure
                        # These are created from the Hive-style partitioning and shouldn't be in the data
                        partition_columns = ['underlying', 'year', 'month', 'day']
                        for col in partition_columns:
                            if col in existing_df.columns:
                                existing_df = existing_df.drop(columns=[col])
                        
                        # Convert all string columns to string type to avoid dictionary issues
                        for col in existing_df.columns:
                            if existing_df[col].dtype == 'object' or pd.api.types.is_string_dtype(existing_df[col]):
                                existing_df[col] = existing_df[col].astype(str)

                        # Ensure schema compatibility before combining
                        new_df = self._ensure_schema_compatibility(existing_df, new_df)

                        # Combine existing and new data
                        combined_df = pd.concat([existing_df, new_df], ignore_index=True)

                        # Atomic write using temporary file
                        if self._atomic_write_parquet(combined_df, output_path):
                            logger.info(f"✅ Appended {len(records)} records to {output_path} (total: {len(combined_df)} records)")
                            
                            # Verify record count after write
                            try:
                                verify_table = pq.read_table(output_path)
                                # Use len() instead of metadata.num_rows for PyArrow compatibility
                                actual_count = len(verify_table)
                                if actual_count != len(combined_df):
                                    logger.error(f"❌ RECORD COUNT MISMATCH: Expected {len(combined_df)}, got {actual_count}")
                                else:
                                    logger.debug(f"✅ Record count verified: {actual_count}")
                            except Exception as e:
                                logger.warning(f"Could not verify record count: {e}")
                        else:
                            logger.error(f"❌ Atomic write failed for {output_path}")
                            return None

                    except Exception as e:
                        logger.warning(f"Error reading existing file {output_path}, overwriting: {e}")
                        # Fall back to overwriting if read fails
                        if self._atomic_write_parquet(new_df, output_path):
                            logger.info(f"✅ Wrote {len(records)} records to {output_path} (fallback overwrite)")
                        else:
                            logger.error(f"❌ Fallback atomic write failed for {output_path}")
                            return None
                else:
                    # File doesn't exist - write new file
                    if self._atomic_write_parquet(new_df, output_path):
                        logger.info(f"✅ Wrote {len(records)} records to {output_path} (new file)")
                    else:
                        logger.error(f"❌ Atomic write failed for new file {output_path}")
                        return None

            return str(output_path)

        except Exception as e:
            logger.error(f"❌ Error writing/appending symbol-date partition to Parquet: {e}")
            return None
    
    def _get_symbol_date_partition_path(self, 
                                       underlying_symbol: str,
                                       record_date: date, 
                                       asset_type: AssetType,
                                       metadata: Dict[str, Any]) -> Path:
        """
        Get the output path for a symbol-date partition.
        
        Args:
            underlying_symbol: Underlying symbol (e.g., "SPXW")
            record_date: Date
            asset_type: Asset type
            metadata: Source metadata
            
        Returns:
            Path for the partition file
        """
        # Determine base directory by asset type
        if asset_type == AssetType.OPTIONS:
            base_dir = self.options_dir
        elif asset_type == AssetType.EQUITIES:
            base_dir = self.equities_dir
        elif asset_type == AssetType.FUTURES:
            base_dir = self.futures_dir
        else:
            base_dir = self.forex_dir
        
        # Create symbol-date partitioned path: asset_type/underlying=SYMBOL/year=YYYY/month=MM/day=DD/data.parquet
        year = record_date.year
        month = record_date.month
        day = record_date.day
        
        partition_path = (
            base_dir / 
            f"underlying={underlying_symbol}" /
            f"year={year}" / 
            f"month={month:02d}" / 
            f"day={day:02d}" / 
            "data.parquet"
        )
        
        return partition_path
    
    def get_existing_partitions(self, asset_type: Optional[AssetType] = None) -> List[Dict[str, Any]]:
        """
        Get information about existing Parquet partitions.
        
        Args:
            asset_type: Optional filter by asset type
            
        Returns:
            List of partition information dictionaries
        """
        partitions = []
        
        # Determine directories to scan
        if asset_type:
            if asset_type == AssetType.OPTIONS:
                dirs_to_scan = [self.options_dir]
            elif asset_type == AssetType.EQUITIES:
                dirs_to_scan = [self.equities_dir]
            elif asset_type == AssetType.FUTURES:
                dirs_to_scan = [self.futures_dir]
            else:
                dirs_to_scan = [self.forex_dir]
        else:
            dirs_to_scan = [self.options_dir, self.equities_dir, self.futures_dir, self.forex_dir]
        
        for base_dir in dirs_to_scan:
            if not base_dir.exists():
                continue
                
            # Scan for parquet files
            for parquet_file in base_dir.rglob("*.parquet"):
                try:
                    # Parse path to extract partition information
                    parts = parquet_file.parts
                    
                    # Find year, month, day, symbol parts
                    partition_info = {
                        'file_path': str(parquet_file),
                        'asset_type': base_dir.name,
                        'file_size': parquet_file.stat().st_size
                    }
                    
                    for part in parts:
                        if part.startswith('year='):
                            partition_info['year'] = int(part.split('=')[1])
                        elif part.startswith('month='):
                            partition_info['month'] = int(part.split('=')[1])
                        elif part.startswith('day='):
                            partition_info['day'] = int(part.split('=')[1])
                        elif part.startswith('symbol='):
                            partition_info['symbol'] = part.split('=')[1]
                    
                    # Get record count from Parquet metadata
                    try:
                        parquet_file_obj = pq.ParquetFile(parquet_file)
                        partition_info['record_count'] = parquet_file_obj.metadata.num_rows
                    except Exception:
                        partition_info['record_count'] = 0
                    
                    partitions.append(partition_info)
                    
                except Exception as e:
                    logger.warning(f"Error processing partition {parquet_file}: {e}")
        
        return partitions
    
    def delete_partitions(self, 
                         symbol: Optional[str] = None,
                         date_range: Optional[DateRange] = None,
                         asset_type: Optional[AssetType] = None) -> int:
        """
        Delete existing partitions matching criteria.
        
        Args:
            symbol: Optional symbol filter
            date_range: Optional date range filter
            asset_type: Optional asset type filter
            
        Returns:
            Number of partitions deleted
        """
        partitions = self.get_existing_partitions(asset_type)
        deleted_count = 0
        
        for partition in partitions:
            should_delete = True
            
            # Apply filters
            if symbol and partition.get('symbol') != symbol:
                should_delete = False
            
            if date_range and 'year' in partition and 'month' in partition and 'day' in partition:
                partition_date = date(partition['year'], partition['month'], partition['day'])
                if not (date_range.start_date <= partition_date <= date_range.end_date):
                    should_delete = False
            
            if should_delete:
                try:
                    Path(partition['file_path']).unlink()
                    deleted_count += 1
                    logger.info(f"Deleted partition: {partition['file_path']}")
                except Exception as e:
                    logger.error(f"Error deleting partition {partition['file_path']}: {e}")
        
        return deleted_count
    
    def get_storage_stats(self) -> Dict[str, Any]:
        """
        Get storage statistics for Parquet data.
        
        Returns:
            Dictionary with storage statistics
        """
        stats = {
            'total_partitions': 0,
            'total_size_bytes': 0,
            'total_records': 0,
            'by_asset_type': {}
        }
        
        for asset_type in [AssetType.OPTIONS, AssetType.EQUITIES, AssetType.FUTURES, AssetType.FOREX]:
            partitions = self.get_existing_partitions(asset_type)
            
            asset_stats = {
                'partitions': len(partitions),
                'size_bytes': sum(p.get('file_size', 0) for p in partitions),
                'records': sum(p.get('record_count', 0) for p in partitions),
                'symbols': len(set(p.get('symbol') for p in partitions if p.get('symbol')))
            }
            
            stats['by_asset_type'][asset_type.value] = asset_stats
            stats['total_partitions'] += asset_stats['partitions']
            stats['total_size_bytes'] += asset_stats['size_bytes']
            stats['total_records'] += asset_stats['records']
        
        # Add human-readable sizes
        stats['total_size_mb'] = round(stats['total_size_bytes'] / (1024 * 1024), 2)
        stats['total_size_gb'] = round(stats['total_size_bytes'] / (1024 * 1024 * 1024), 2)
        
        return stats
    
    def _write_date_partition_to_parquet(self, 
                                        record_date: date, 
                                        asset_type: AssetType,
                                        records: List[Dict[str, Any]],
                                        metadata: Dict[str, Any]) -> Optional[str]:
        """
        Write a date partition of records to Parquet file (all symbols for a given date).
        
        Args:
            record_date: Date for this partition
            asset_type: Asset type
            records: List of record dictionaries
            metadata: Source metadata
            
        Returns:
            Output file path or None if failed
        """
        try:
            if not records:
                return None
            
            # Convert records to DataFrame
            df = pd.DataFrame(records)
            
            # Ensure consistent data types
            df = self._normalize_dataframe(df)
            
            # Determine output path for date-based partitioning
            output_path = self._get_date_partition_path(record_date, asset_type, metadata)
            output_path.parent.mkdir(parents=True, exist_ok=True)
            
            # Write to Parquet with compression
            table = pa.Table.from_pandas(df)
            pq.write_table(
                table, 
                output_path,
                compression='snappy',
                use_dictionary=True,
                write_statistics=True
            )
            
            logger.info(f"Wrote {len(records)} records to {output_path}")
            return str(output_path)
            
        except Exception as e:
            logger.error(f"Error writing date partition to Parquet: {e}")
            return None
    
    def _get_date_partition_path(self, 
                                record_date: date, 
                                asset_type: AssetType,
                                metadata: Dict[str, Any]) -> Path:
        """
        Get the output path for a date-based partition.
        
        Args:
            record_date: Date
            asset_type: Asset type
            metadata: Source metadata
            
        Returns:
            Path for the partition file
        """
        # Determine base directory by asset type
        if asset_type == AssetType.OPTIONS:
            base_dir = self.options_dir
        elif asset_type == AssetType.EQUITIES:
            base_dir = self.equities_dir
        elif asset_type == AssetType.FUTURES:
            base_dir = self.futures_dir
        else:
            base_dir = self.forex_dir
        
        # Create date-based partitioned path: asset_type/year=YYYY/month=MM/day=DD/data.parquet
        year = record_date.year
        month = record_date.month
        day = record_date.day
        
        partition_path = (
            base_dir / 
            f"year={year}" / 
            f"month={month:02d}" / 
            f"day={day:02d}" / 
            "data.parquet"
        )
        
        return partition_path
    
    def _build_date_aware_mapping(self, metadata: Dict[str, Any]) -> Dict[int, List[tuple]]:
        """
        Build date-aware symbol mapping from metadata.
        
        Args:
            metadata: File metadata containing native mappings
            
        Returns:
            Dictionary mapping instrument_id to list of (start_date, end_date, symbol) tuples
        """
        date_aware_mapping = {}
        
        try:
            # Extract native mappings from metadata
            native_mappings = metadata.get('native_mappings')
            if not native_mappings:
                logger.warning("No native mappings found in metadata")
                return date_aware_mapping
            
            # Process each symbol's date ranges
            for readable_symbol, date_ranges in native_mappings.items():
                if isinstance(date_ranges, list):
                    for date_range_info in date_ranges:
                        if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                            instrument_id = int(date_range_info['symbol'])
                            start_date = date_range_info.get('start_date')
                            end_date = date_range_info.get('end_date')
                            
                            if instrument_id not in date_aware_mapping:
                                date_aware_mapping[instrument_id] = []
                            
                            date_aware_mapping[instrument_id].append((start_date, end_date, readable_symbol))
            
            logger.info(f"Built date-aware mapping for {len(date_aware_mapping)} instrument IDs")
            
        except Exception as e:
            logger.error(f"Error building date-aware mapping: {e}")
        
        return date_aware_mapping
    
    def _extract_record_info_with_context(self, 
                                         record: Any, 
                                         file_asset_type: Optional[AssetType] = None) -> Optional[Dict[str, Any]]:
        """
        Extract record information and convert to unified CBBO-style schema.
        
        This method handles both DBN records (CBBO/OHLCV) and CSV records,
        converting all data to the unified CBBO schema for consistent storage:
        - DBN CBBO records: Use native bid_px/ask_px/bid_sz/ask_sz
        - DBN OHLCV records: Convert close to bid_px/ask_px, volume to bid_sz/ask_sz
        - CSV OHLCV records: Convert close to bid_px/ask_px, volume to bid_sz/ask_sz
        
        Args:
            record: DBN record or CSVRecord
            file_asset_type: Asset type determined from file metadata
            
        Returns:
            Dictionary with extracted information in unified CBBO schema or None if extraction fails
        """
        try:
            # Handle CSV records
            if isinstance(record, CSVRecord):
                return self._extract_csv_record_info(record, file_asset_type)
            
            # Handle DBN records - extract basic fields
            record_info = {}
            
            # Use symbol from native DBN mapping (already populated by DBN reader)
            symbol = getattr(record, 'symbol', None)
            if not symbol:
                # Use instrument_id as fallback identifier 
                instrument_id = getattr(record, 'instrument_id', None)
                if instrument_id:
                    symbol = str(instrument_id)
                else:
                    logger.debug("Record has no symbol or instrument_id")
                    return None
            
            record_info['symbol'] = symbol
            
            # Get timestamp and handle invalid values
            timestamp = getattr(record, 'ts_event', None) or getattr(record, 'timestamp', None)
            if timestamp and timestamp != 18446744073709551615:  # Filter out UINT64_MAX invalid timestamps
                try:
                    dt = datetime.fromtimestamp(timestamp / 1e9)
                    # Validate the resulting datetime is reasonable
                    if dt.year >= 1970 and dt.year <= 2030:
                        record_info['timestamp'] = dt
                        record_info['date'] = dt.date()
                    else:
                        # Invalid year, skip this record
                        logger.debug(f"Skipping record with invalid year: {dt.year}")
                        return None
                except (ValueError, OSError, OverflowError):
                    # Invalid timestamp, skip this record
                    logger.debug(f"Skipping record with invalid timestamp: {timestamp}")
                    return None
            else:
                # Invalid or missing timestamp, skip this record
                logger.debug(f"Skipping record with invalid/missing timestamp: {timestamp}")
                return None
            
            # Extract underlying symbol for partitioning
            underlying_symbol = self._extract_underlying_symbol(symbol)
            record_info['underlying_symbol'] = underlying_symbol
            
            # Asset type determination - use file metadata if available
            record_info['asset_type'] = file_asset_type or self._determine_asset_type(symbol)
            
            # UNIFIED SCHEMA: Convert all DBN data to CBBO-style format
            
            # Check for CBBO fields - first try levels array (where CBBO data is actually stored)
            if hasattr(record, 'levels') and record.levels and len(record.levels) > 0:
                # This is a CBBO record - extract bid/ask data from levels array
                level = record.levels[0]  # Use first level (best bid/ask)
                
                # Extract bid price from levels
                if hasattr(level, 'bid_px'):
                    raw_bid = getattr(level, 'bid_px', 0)
                    record_info['bid_px'] = self._scale_dbn_price(raw_bid)
                else:
                    record_info['bid_px'] = 0.0
                
                # Extract ask price from levels
                if hasattr(level, 'ask_px'):
                    raw_ask = getattr(level, 'ask_px', 0)
                    record_info['ask_px'] = self._scale_dbn_price(raw_ask)
                else:
                    record_info['ask_px'] = 0.0
                
                # Extract bid/ask sizes from levels
                if hasattr(level, 'bid_sz'):
                    record_info['bid_sz'] = int(getattr(level, 'bid_sz', 0))
                else:
                    record_info['bid_sz'] = 0
                
                if hasattr(level, 'ask_sz'):
                    record_info['ask_sz'] = int(getattr(level, 'ask_sz', 0))
                else:
                    record_info['ask_sz'] = 0
                    
            # Fallback: Check for direct CBBO fields (bid_px, ask_px, bid_sz, ask_sz)
            elif hasattr(record, 'bid_px') or hasattr(record, 'ask_px'):
                # This is a CBBO record with direct fields - extract native bid/ask data
                if hasattr(record, 'bid_px'):
                    raw_bid = getattr(record, 'bid_px', 0)
                    record_info['bid_px'] = self._scale_dbn_price(raw_bid)
                else:
                    record_info['bid_px'] = 0.0
                
                if hasattr(record, 'ask_px'):
                    raw_ask = getattr(record, 'ask_px', 0)
                    record_info['ask_px'] = self._scale_dbn_price(raw_ask)
                else:
                    record_info['ask_px'] = 0.0
                
                # Extract bid/ask sizes
                if hasattr(record, 'bid_sz'):
                    record_info['bid_sz'] = int(getattr(record, 'bid_sz', 0))
                else:
                    record_info['bid_sz'] = 0
                
                if hasattr(record, 'ask_sz'):
                    record_info['ask_sz'] = int(getattr(record, 'ask_sz', 0))
                else:
                    record_info['ask_sz'] = 0
                    
            elif hasattr(record, 'close'):
                # This is an OHLCV record - convert to CBBO format
                raw_close = getattr(record, 'close', 0)
                close_price = self._scale_dbn_price(raw_close)
                
                # Use close price as both bid and ask (no spread for backtesting)
                record_info['bid_px'] = close_price
                record_info['ask_px'] = close_price
                
                # Convert volume to bid/ask sizes
                volume = int(getattr(record, 'volume', 0))
                record_info['bid_sz'] = volume // 2  # Split volume between bid and ask
                record_info['ask_sz'] = volume // 2
                
            else:
                # Fallback - no price data available
                record_info['bid_px'] = 0.0
                record_info['ask_px'] = 0.0
                record_info['bid_sz'] = 0
                record_info['ask_sz'] = 0
            
            # Extract options metadata for options data
            # CRITICAL FIX: Always try to extract options metadata for option-like symbols
            # regardless of file_asset_type, since symbol detection is more reliable
            if file_asset_type == AssetType.OPTIONS or self._determine_asset_type(symbol) == AssetType.OPTIONS:
                options_metadata = self._extract_options_metadata(symbol)
                if options_metadata:
                    record_info.update(options_metadata)
                    # Ensure the asset type is correctly set for options
                    record_info['asset_type'] = AssetType.OPTIONS
                else:
                    # Options symbol but metadata extraction failed
                    logger.debug(f"Options metadata extraction failed for symbol: {symbol}")
                    record_info['strike'] = None
                    record_info['option_type'] = None
                    record_info['expiration'] = None
            else:
                # For non-options data, set options fields to None
                record_info['strike'] = None
                record_info['option_type'] = None
                record_info['expiration'] = None
            
            return record_info
            
        except Exception as e:
            logger.warning(f"Error extracting record info with unified schema: {e}")
            return None
    
    def _extract_csv_record_info(self, record: CSVRecord, 
                                file_asset_type: Optional[AssetType] = None) -> Optional[Dict[str, Any]]:
        """
        Extract record information from CSV record and convert to unified CBBO-style schema.
        
        This converts OHLCV data to the unified schema used by CBBO data:
        - close price becomes both bid_px and ask_px (no spread for backtesting)
        - volume becomes both bid_sz and ask_sz
        - Maintains compatibility with options CBBO data
        
        Args:
            record: CSVRecord object
            file_asset_type: Asset type determined from file metadata
            
        Returns:
            Dictionary with extracted information in unified schema or None if extraction fails
        """
        try:
            if not record or not record.symbol:
                return None
            
            # Extract underlying symbol for partitioning
            underlying_symbol = self._extract_underlying_symbol(record.symbol)
            
            # Asset type determination - use file metadata if available
            asset_type = file_asset_type or self._determine_asset_type(record.symbol)
            
            record_info = {
                'symbol': record.symbol,
                'underlying_symbol': underlying_symbol,
                'timestamp': record.timestamp,
                'date': record.date,
                'asset_type': asset_type
            }
            
            # Convert OHLCV to unified CBBO-style schema
            # For backtesting equities, we use close price as both bid and ask (no spread)
            if record.close is not None:
                close_price = float(record.close)
                record_info['bid_px'] = close_price  # Use close as bid
                record_info['ask_px'] = close_price  # Use close as ask (no spread for backtesting)
            else:
                record_info['bid_px'] = 0.0
                record_info['ask_px'] = 0.0
            
            # Convert volume to bid/ask sizes
            if record.volume is not None:
                volume = int(record.volume)
                record_info['bid_sz'] = volume // 2  # Split volume between bid and ask
                record_info['ask_sz'] = volume // 2
            else:
                record_info['bid_sz'] = 0
                record_info['ask_sz'] = 0
            
            # For equities, we don't have options metadata
            # These fields will be None/empty for non-options data
            if asset_type != AssetType.OPTIONS:
                record_info['strike'] = None
                record_info['option_type'] = None
                record_info['expiration'] = None
            
            return record_info
            
        except Exception as e:
            logger.warning(f"Error extracting CSV record info: {e}")
            return None
    
    def _extract_options_metadata(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        Extract options metadata (strike, type, expiration) from option symbol.
        
        This method parses standard option symbols like:
        - "SPY   250822C00640000" -> strike=640.0, type="call", expiration=2025-08-22
        - "SPXW  250827P06375000" -> strike=6375.0, type="put", expiration=2025-08-27
        
        Args:
            symbol: Full option symbol
            
        Returns:
            Dictionary with strike, option_type, expiration or None if parsing fails
        """
        try:
            if not symbol or len(symbol) < 15:
                return None
            
            # Standard option symbol format: "SYMBOL  YYMMDDCPPPPPPP"
            # Where: SYMBOL = underlying, YY = year, MM = month, DD = day, 
            #        C = call/put (C/P), PPPPPPP = strike price * 1000
            
            # Remove extra spaces and normalize
            clean_symbol = ' '.join(symbol.split())
            
            # Split on spaces to separate underlying from option details
            parts = clean_symbol.split()
            if len(parts) < 2:
                return None
            
            underlying = parts[0]
            option_part = parts[1]  # e.g., "250822C00640000"
            
            if len(option_part) < 15:
                return None
            
            # Extract date part (first 6 characters: YYMMDD)
            date_part = option_part[:6]  # e.g., "250822"
            
            # Extract option type (7th character: C or P)
            option_type_char = option_part[6]  # e.g., "C"
            
            # Extract strike part (remaining 8 characters)
            strike_part = option_part[7:]  # e.g., "00640000"
            
            # Parse expiration date
            try:
                year = 2000 + int(date_part[:2])  # Convert YY to YYYY
                month = int(date_part[2:4])
                day = int(date_part[4:6])
                expiration = date(year, month, day)
            except (ValueError, TypeError):
                logger.warning(f"Could not parse expiration date from {date_part} in symbol {symbol}")
                return None
            
            # Parse option type
            if option_type_char.upper() == 'C':
                option_type = 'call'
            elif option_type_char.upper() == 'P':
                option_type = 'put'
            else:
                logger.warning(f"Unknown option type '{option_type_char}' in symbol {symbol}")
                return None
            
            # Parse strike price (divide by 1000 to get actual strike)
            try:
                strike_raw = int(strike_part)
                strike = float(strike_raw) / 1000.0
            except (ValueError, TypeError):
                logger.warning(f"Could not parse strike price from {strike_part} in symbol {symbol}")
                return None
            
            return {
                'strike': strike,
                'option_type': option_type,
                'expiration': expiration
            }
            
        except Exception as e:
            logger.warning(f"Error extracting options metadata from symbol {symbol}: {e}")
            return None
