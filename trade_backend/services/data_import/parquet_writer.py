"""
Parquet Writer

Handles conversion of DBN data to partitioned Parquet format for efficient
backtesting queries. Provides optimized storage with proper partitioning
by date, symbol, and asset type.
"""

import logging
from datetime import datetime, date
from pathlib import Path
from typing import Dict, List, Optional, Any, Iterator
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from concurrent.futures import ThreadPoolExecutor
import os

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
        
        Args:
            raw_price: Raw price value from DBN (string or int)
            
        Returns:
            Scaled price as float
        """
        try:
            if raw_price is None or raw_price == 0:
                return 0.0
            
            # Convert to int if it's a string
            if isinstance(raw_price, str):
                price_int = int(raw_price)
            else:
                price_int = int(raw_price)
            
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
        # Ensure timestamp is datetime
        if 'timestamp' in df.columns:
            df['timestamp'] = pd.to_datetime(df['timestamp'])
        
        # Ensure date is date type
        if 'date' in df.columns:
            df['date'] = pd.to_datetime(df['date']).dt.date
        
        # Ensure numeric columns are proper types
        numeric_columns = ['open', 'high', 'low', 'close', 'price', 'bid', 'ask']
        for col in numeric_columns:
            if col in df.columns:
                df[col] = pd.to_numeric(df[col], errors='coerce')
        
        # Ensure integer columns
        int_columns = ['volume', 'size', 'bid_size', 'ask_size']
        for col in int_columns:
            if col in df.columns:
                df[col] = pd.to_numeric(df[col], errors='coerce').fillna(0).astype('int64')
        
        # Ensure string columns
        string_columns = ['symbol', 'asset_type']
        for col in string_columns:
            if col in df.columns:
                df[col] = df[col].astype(str)
        
        return df
    
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

            if output_path.exists():
                # File exists - read existing data and append
                try:
                    existing_table = pq.read_table(output_path)
                    existing_df = existing_table.to_pandas()

                    # Combine existing and new data
                    combined_df = pd.concat([existing_df, new_df], ignore_index=True)

                    # Write combined data
                    table = pa.Table.from_pandas(combined_df)
                    pq.write_table(
                        table,
                        output_path,
                        compression='snappy',
                        use_dictionary=True,
                        write_statistics=True
                    )

                    logger.info(f"Appended {len(records)} records to {output_path} (total: {len(combined_df)} records)")

                except Exception as e:
                    logger.warning(f"Error reading existing file {output_path}, overwriting: {e}")
                    # Fall back to overwriting if read fails
                    table = pa.Table.from_pandas(new_df)
                    pq.write_table(
                        table,
                        output_path,
                        compression='snappy',
                        use_dictionary=True,
                        write_statistics=True
                    )
                    logger.info(f"Wrote {len(records)} records to {output_path} (fallback overwrite)")
            else:
                # File doesn't exist - write new file
                table = pa.Table.from_pandas(new_df)
                pq.write_table(
                    table,
                    output_path,
                    compression='snappy',
                    use_dictionary=True,
                    write_statistics=True
                )
                logger.info(f"Wrote {len(records)} records to {output_path} (new file)")

            return str(output_path)

        except Exception as e:
            logger.error(f"Error writing/appending symbol-date partition to Parquet: {e}")
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
        Extract record information from DBN record with native symbol mapping.
        Records come with symbols already populated by the native DBN library.
        
        Args:
            record: DBN record (with symbols already mapped by native DBN library)
            file_asset_type: Asset type determined from file metadata
            
        Returns:
            Dictionary with extracted information or None if extraction fails
        """
        try:
            # Handle CSV records differently
            if isinstance(record, CSVRecord):
                return self._extract_csv_record_info(record, file_asset_type)
            
            # Extract basic fields for DBN records
            record_info = {}
            
            # Use symbol from native DBN mapping (already populated by DBN reader)
            symbol = getattr(record, 'symbol', None)
            if not symbol:
                # This should not happen with native mapping, but provide fallback
                instrument_id = getattr(record, 'instrument_id', None)
                if instrument_id:
                    symbol = f"UNMAPPED_{instrument_id}"
                    logger.warning(f"No native symbol found for instrument_id {instrument_id}")
                else:
                    logger.warning("Record has no symbol or instrument_id")
                    return None
            
            record_info['symbol'] = symbol
            
            # Get timestamp
            timestamp = getattr(record, 'ts_event', None) or getattr(record, 'timestamp', None)
            if timestamp:
                dt = datetime.fromtimestamp(timestamp / 1e9)
                record_info['timestamp'] = dt
                record_info['date'] = dt.date()
            else:
                # Fallback to current date
                record_info['date'] = date.today()
                record_info['timestamp'] = datetime.now()
            
            # Extract underlying symbol for partitioning
            underlying_symbol = self._extract_underlying_symbol(symbol)
            record_info['underlying_symbol'] = underlying_symbol
            
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
            logger.warning(f"Error extracting record info with native mapping: {e}")
            return None
    
    def _extract_csv_record_info(self, record: CSVRecord, 
                                file_asset_type: Optional[AssetType] = None) -> Optional[Dict[str, Any]]:
        """
        Extract record information from CSV record.
        
        Args:
            record: CSVRecord object
            file_asset_type: Asset type determined from file metadata
            
        Returns:
            Dictionary with extracted information or None if extraction fails
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
            
            # OHLCV data - use standard schema only
            if record.open is not None:
                record_info['open'] = float(record.open)
            if record.high is not None:
                record_info['high'] = float(record.high)
            if record.low is not None:
                record_info['low'] = float(record.low)
            if record.close is not None:
                record_info['close'] = float(record.close)
            if record.volume is not None:
                record_info['volume'] = int(record.volume)
            
            # Note: TradeStation up_volume and down_volume are already summed into volume
            # We don't save them separately to maintain schema compatibility with DBN files
            
            return record_info
            
        except Exception as e:
            logger.warning(f"Error extracting CSV record info: {e}")
            return None
