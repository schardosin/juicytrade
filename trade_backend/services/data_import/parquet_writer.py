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
                              filters: Optional[ImportFilters] = None,
                              progress_callback: Optional[callable] = None,
                              symbol_mapping: Optional[Dict[int, str]] = None) -> List[str]:
        """
        Convert DBN records to partitioned Parquet files.
        
        Args:
            records_iterator: Iterator of DBN records
            metadata: Metadata about the source file
            filters: Optional filters to apply
            progress_callback: Optional progress callback
            symbol_mapping: Optional mapping from instrument_id to readable symbol
            
        Returns:
            List of output file paths created
        """
        logger.info("Starting DBN to Parquet conversion")
        
        output_paths = []
        records_by_date = {}  # Changed: Group by date only, not symbol+date
        total_records = 0
        processed_records = 0
        
        # Determine asset type from metadata
        file_asset_type = self._determine_asset_type_from_metadata(metadata)
        
        try:
            # Group records by underlying symbol and date for optimal partitioning
            records_by_symbol_date = {}
            
            for record in records_iterator:
                try:
                    # Extract record information with symbol mapping
                    record_info = self._extract_record_info(record, file_asset_type, symbol_mapping)
                    if not record_info:
                        continue
                    
                    symbol = record_info['symbol']
                    underlying_symbol = record_info['underlying_symbol']
                    record_date = record_info['date']
                    asset_type = record_info['asset_type']
                    
                    # Apply filters
                    if filters and not self._record_matches_filters(record_info, filters):
                        continue
                    
                    # Group by underlying symbol and date for better management
                    key = (underlying_symbol, record_date, asset_type)
                    if key not in records_by_symbol_date:
                        records_by_symbol_date[key] = []
                    
                    records_by_symbol_date[key].append(record_info)
                    total_records += 1
                    
                    # Progress update
                    if progress_callback and total_records % 10000 == 0:
                        progress_callback(total_records, None, f"Grouping records: {underlying_symbol}")
                
                except Exception as e:
                    logger.warning(f"Error processing record: {e}")
                    continue
            
            logger.info(f"Grouped {total_records} records into {len(records_by_symbol_date)} symbol-date partitions")
            
            # Convert each group to Parquet
            for i, ((underlying_symbol, record_date, asset_type), records) in enumerate(records_by_symbol_date.items()):
                try:
                    output_path = self._write_symbol_date_partition_to_parquet(
                        underlying_symbol, record_date, asset_type, records, metadata
                    )
                    
                    if output_path:
                        output_paths.append(output_path)
                        processed_records += len(records)
                    
                    # Progress update
                    if progress_callback:
                        progress_callback(
                            processed_records, 
                            total_records, 
                            f"Writing partition: {underlying_symbol} {record_date} ({len(records)} records)"
                        )
                
                except Exception as e:
                    logger.error(f"Error writing partition for {underlying_symbol} {record_date}: {e}")
                    continue
            
            logger.info(f"Successfully converted {processed_records} records to {len(output_paths)} Parquet files")
            return output_paths
            
        except Exception as e:
            logger.error(f"Error in DBN to Parquet conversion: {e}")
            raise
    
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
            
            # Symbol - use mapping if available, skip unmapped records
            symbol = None
            if hasattr(record, 'instrument_id') and symbol_mapping:
                instrument_id = record.instrument_id
                symbol = symbol_mapping.get(instrument_id)
                if not symbol:
                    # Skip records without proper symbol mapping
                    return None
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
            
            # Price data (adapt based on actual record structure)
            if hasattr(record, 'open'):
                record_info['open'] = float(getattr(record, 'open', 0))
            if hasattr(record, 'high'):
                record_info['high'] = float(getattr(record, 'high', 0))
            if hasattr(record, 'low'):
                record_info['low'] = float(getattr(record, 'low', 0))
            if hasattr(record, 'close'):
                record_info['close'] = float(getattr(record, 'close', 0))
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
    
    def _record_matches_filters(self, record_info: Dict[str, Any], filters: ImportFilters) -> bool:
        """
        Check if record matches import filters.
        
        Args:
            record_info: Extracted record information
            filters: Import filters
            
        Returns:
            True if record matches filters
        """
        # Symbol filter
        if filters.symbols and record_info['symbol'] not in filters.symbols:
            return False
        
        # Date filter
        if filters.date_range:
            record_date = record_info['date']
            if not (filters.date_range.start_date <= record_date <= filters.date_range.end_date):
                return False
        
        # Asset type filter
        if filters.asset_types and record_info['asset_type'] not in filters.asset_types:
            return False
        
        return True
    
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
        Write a symbol-date partition of records to Parquet file.
        
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
            
            # Convert records to DataFrame
            df = pd.DataFrame(records)
            
            # Ensure consistent data types
            df = self._normalize_dataframe(df)
            
            # Determine output path for symbol-date partitioning
            output_path = self._get_symbol_date_partition_path(underlying_symbol, record_date, asset_type, metadata)
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
            logger.error(f"Error writing symbol-date partition to Parquet: {e}")
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
