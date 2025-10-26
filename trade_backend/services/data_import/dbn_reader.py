"""
DBN Reader

Handles reading Databento binary (DBN) files using the databento library's
built-in functionality for optimal performance and accuracy.

This module leverages the databento library's native capabilities for symbol mapping
and metadata extraction, ensuring compatibility with DBN CLI behavior and eliminating
phantom records through proper date-based symbol resolution.

Key features:
- Stream records with automatic symbol mapping
- Convert to DataFrame using databento's optimized methods  
- Extract metadata and symbol information
- Date-aware symbol resolution (prevents phantom records)
"""

import logging
import functools
from datetime import datetime, date, timedelta
from pathlib import Path
from typing import Dict, List, Optional, Iterator, Any, TYPE_CHECKING
import os

if TYPE_CHECKING:
    import pandas as pd

try:
    import databento as db
    from databento import DBNStore
    DATABENTO_AVAILABLE = True
except ImportError:
    DATABENTO_AVAILABLE = False
    db = None
    # Create a placeholder for type hints when databento is not available
    if TYPE_CHECKING:
        from databento import DBNStore
    else:
        DBNStore = None

from .import_models import (
    DBNMetadata, SymbolInfo, DataType, AssetType, 
    DateRange, ImportFilters
)

logger = logging.getLogger(__name__)


class DBNReader:
    """
    Reader for Databento binary files with metadata extraction capabilities.
    """
    
    def __init__(self):
        if not DATABENTO_AVAILABLE:
            raise ImportError(
                "databento library is not available. Please install it with: pip install databento"
            )
        # Cache for date-aware symbol mappings (mimic databento's InstrumentMap caching)
        self._date_aware_mapping = None
    
    @functools.lru_cache(maxsize=10000)
    def _resolve_symbol_for_date(self, instrument_id: int, record_date: date) -> Optional[str]:
        """
        Resolve an instrument ID on a particular date to the mapped symbol.
        Uses date-based resolution like databento's InstrumentMap.resolve().
        
        This method is cached to improve performance for repeated lookups.
        
        Args:
            instrument_id: The instrument ID to resolve
            record_date: The date for which to resolve the symbol
            
        Returns:
            The symbol string if found, None otherwise
        """
        if not self._date_aware_mapping or instrument_id not in self._date_aware_mapping:
            return None
            
        # FLEXIBLE MATCHING: Try multiple strategies for options data
        mappings = self._date_aware_mapping[instrument_id]
        
        # Strategy 1: Exact date range match (original logic)
        for mapping in mappings:
            if mapping['start_date'] <= record_date < mapping['end_date']:
                return mapping['symbol']
        
        # Strategy 2: For options, if record_date is before all mappings, use the earliest mapping
        # This handles cases where we have January data but mappings start in March
        earliest_mapping = min(mappings, key=lambda m: m['start_date'])
        if record_date < earliest_mapping['start_date']:
            logger.debug(f"Using earliest mapping for {instrument_id} on {record_date}: {earliest_mapping['symbol']}")
            return earliest_mapping['symbol']
        
        # Strategy 3: If record_date is after all mappings, use the latest mapping
        latest_mapping = max(mappings, key=lambda m: m['end_date'])
        if record_date >= latest_mapping['end_date']:
            logger.debug(f"Using latest mapping for {instrument_id} on {record_date}: {latest_mapping['symbol']}")
            return latest_mapping['symbol']
        
        # Strategy 4: Find the closest mapping by date
        closest_mapping = min(mappings, key=lambda m: min(
            abs((record_date - m['start_date']).days),
            abs((record_date - m['end_date']).days)
        ))
        logger.debug(f"Using closest mapping for {instrument_id} on {record_date}: {closest_mapping['symbol']}")
        return closest_mapping['symbol']
    
    def extract_metadata(self, file_path: Path) -> DBNMetadata:
        """
        Extract metadata from a DBN file using fast, non-scanning approach.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            DBNMetadata object with extracted information
        """
        logger.info(f"Extracting fast metadata from DBN file: {file_path}")
        
        try:
            # Get basic file information
            stat = file_path.stat()
            file_size_gb = stat.st_size / (1024 * 1024 * 1024)
            
            metadata = DBNMetadata(
                filename=file_path.name,
                file_path=str(file_path),
                file_size=stat.st_size,
                created_at=datetime.fromtimestamp(stat.st_ctime),
                modified_at=datetime.fromtimestamp(stat.st_mtime)
            )
            
            logger.info(f"File size: {file_size_gb:.2f} GB")
            
            # Always start with filename parsing (fast and reliable)
            filename_metadata = self._parse_filename_metadata(file_path)
            if filename_metadata:
                metadata.parsed_from_filename = True
                # Apply filename metadata
                for key, value in filename_metadata.items():
                    if hasattr(metadata, key) and value is not None:
                        setattr(metadata, key, value)
            
            # Try to read native DBN metadata (fast - no data scanning)
            try:
                store = DBNStore.from_file(str(file_path))
                
                # Extract only native metadata (no data iteration)
                metadata.dataset = str(getattr(store.metadata, 'dataset', '')) if getattr(store.metadata, 'dataset', None) else metadata.dataset
                metadata.schema = str(getattr(store.metadata, 'schema', '')) if getattr(store.metadata, 'schema', None) else metadata.schema
                metadata.stype_in = str(getattr(store.metadata, 'stype_in', '')) if getattr(store.metadata, 'stype_in', None) else None
                metadata.stype_out = str(getattr(store.metadata, 'stype_out', '')) if getattr(store.metadata, 'stype_out', None) else None
                
                # Extract time range from native metadata (authoritative source)
                if hasattr(store.metadata, 'start') and store.metadata.start:
                    # Convert nanoseconds since epoch to datetime using UTC (timestamps are UTC midnight)
                    from datetime import timezone
                    start_dt = datetime.fromtimestamp(store.metadata.start / 1e9, tz=timezone.utc)
                    metadata.start_timestamp = start_dt
                    
                    if not metadata.overall_date_range:
                        metadata.overall_date_range = DateRange(
                            start_date=start_dt.date(),
                            end_date=start_dt.date()
                        )
                    else:
                        metadata.overall_date_range.start_date = start_dt.date()
                        
                if hasattr(store.metadata, 'end') and store.metadata.end:
                    # Convert nanoseconds since epoch to datetime using UTC (timestamps are UTC midnight)
                    from datetime import timezone
                    end_dt = datetime.fromtimestamp(store.metadata.end / 1e9, tz=timezone.utc)
                    metadata.end_timestamp = end_dt
                    
                    if metadata.overall_date_range:
                        # End timestamp represents the start of the day AFTER the last trading day
                        # So subtract one day to get the actual last trading day
                        actual_end_date = end_dt.date() - timedelta(days=1)
                        metadata.overall_date_range.end_date = actual_end_date
                
                # For large files, skip expensive operations but extract symbols from native mappings
                if file_size_gb > 1.0:  # Files larger than 1GB
                    logger.info(f"Large file ({file_size_gb:.2f} GB) - using fast metadata extraction")
                    
                    # Use basic info without data scanning
                    metadata.total_records = None  # Will be counted during import
                    
                    # Try to extract symbols from native metadata mappings (fast - no data scanning)
                    symbols_info = self._extract_from_native_metadata_fast(store)
                    metadata.symbols = symbols_info if symbols_info else []
                    
                    # Infer basic info from dataset and schema
                    if metadata.dataset and 'opra' in metadata.dataset.lower():
                        metadata.asset_types = [AssetType.OPTIONS]
                    elif metadata.dataset and 'sip' in metadata.dataset.lower():
                        metadata.asset_types = [AssetType.EQUITIES]
                    
                    if metadata.schema:
                        schema_lower = metadata.schema.lower()
                        if 'ohlcv' in schema_lower:
                            metadata.data_types = [DataType.OHLCV]
                        elif 'trade' in schema_lower:
                            metadata.data_types = [DataType.TRADES]
                        elif 'quote' in schema_lower:
                            metadata.data_types = [DataType.QUOTES]
                
                else:
                    # For smaller files, we can afford more detailed extraction
                    logger.info(f"Small file ({file_size_gb:.2f} GB) - using detailed metadata extraction")
                    symbols_info = self._extract_symbols_info_fast(store)
                    metadata.symbols = symbols_info
                    
                    if symbols_info:
                        metadata.total_records = sum(symbol.record_count for symbol in symbols_info)
                        metadata.data_types = list(set(
                            data_type for symbol in symbols_info for data_type in symbol.data_types
                        ))
                        metadata.asset_types = list(set(symbol.asset_type for symbol in symbols_info))
                
                logger.info(f"Successfully extracted fast metadata for {file_path.name}")
                
            except Exception as e:
                logger.warning(f"Failed to extract native DBN metadata: {e}")
                metadata.metadata_extraction_error = str(e)
            
            return metadata
            
        except Exception as e:
            logger.error(f"Error extracting metadata from {file_path}: {e}")
            raise
    
    def _extract_symbols_info_fast(self, store: DBNStore) -> List[SymbolInfo]:
        """
        Fast symbol extraction for smaller files using limited sampling.
        
        Args:
            store: DBNStore instance
            
        Returns:
            List of SymbolInfo objects
        """
        symbols_info = []
        
        try:
            # First try native metadata
            native_symbols = self._extract_from_native_metadata_fast(store)
            if native_symbols:
                logger.info(f"Using fast native DBN metadata: found {len(native_symbols)} symbols")
                return native_symbols
            
            # Fallback to limited sampling (much smaller sample for speed)
            logger.info("Using fast sampling approach")
            symbols_data = self._sample_symbols_with_counts(store, sample_size=1000)  # Much smaller sample
            
            if not symbols_data:
                logger.warning("No symbols found in DBN file")
                return symbols_info
            
            # Create symbol info for each symbol found
            for symbol, estimated_count in symbols_data.items():
                # Determine asset type
                dataset_str = str(store.metadata.dataset).lower() if store.metadata.dataset else ''
                asset_type = AssetType.OPTIONS if 'opra' in dataset_str else self._determine_asset_type(symbol)
                
                # Use overall date range
                date_range = self._get_symbol_date_range(store, symbol)
                
                # Determine data types from schema
                data_types = self._get_symbol_data_types(store, symbol)
                
                symbol_info = SymbolInfo(
                    symbol=symbol,
                    asset_type=asset_type,
                    date_range=date_range,
                    record_count=estimated_count,
                    data_types=data_types
                )
                
                symbols_info.append(symbol_info)
                
        except Exception as e:
            logger.warning(f"Error extracting symbols info fast: {e}")
            
        return symbols_info
    
    def _extract_from_native_metadata_fast(self, store: DBNStore) -> Optional[List[SymbolInfo]]:
        """
        Fast extraction from native metadata without expensive record counting.
        
        Args:
            store: DBNStore instance
            
        Returns:
            List of SymbolInfo objects or None if native metadata is not available
        """
        try:
            # Check if we have native symbol mappings
            if not hasattr(store.metadata, 'mappings') or not store.metadata.mappings:
                return None
            
            logger.info("Found native DBN symbol mappings - using fast extraction")
            symbols_info = []
            
            # Get underlying symbol from native symbols list
            underlying_symbols = getattr(store.metadata, 'symbols', [])
            underlying_symbol = None
            if underlying_symbols:
                raw_symbol = underlying_symbols[0]
                if '.OPT' in raw_symbol:
                    underlying_symbol = raw_symbol.replace('.OPT', '')
                else:
                    underlying_symbol = raw_symbol
            
            # Get date range from metadata
            date_range = self._get_symbol_date_range(store, '')
            
            # Determine asset type from dataset
            dataset_str = str(store.metadata.dataset).lower() if store.metadata.dataset else ''
            asset_type = AssetType.OPTIONS if 'opra' in dataset_str else AssetType.EQUITIES
            
            # Get data types from schema
            data_types = self._get_symbol_data_types(store, '')
            
            # Process mappings - convert each symbol mapping to SymbolInfo
            for symbol, date_ranges in store.metadata.mappings.items():
                if isinstance(date_ranges, list) and date_ranges:
                    # Extract instrument IDs from date ranges
                    instrument_ids = []
                    for date_range_info in date_ranges:
                        if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                            try:
                                instrument_id = int(date_range_info['symbol'])
                                instrument_ids.append(instrument_id)
                            except (ValueError, TypeError):
                                continue
                    
                    if instrument_ids:
                        # Determine underlying symbol (e.g., SPXW from SPXW  250807C06330000)
                        symbol_underlying = None
                        if symbol and ' ' in symbol:
                            symbol_underlying = symbol.split()[0]
                        
                        symbol_info = SymbolInfo(
                            symbol=symbol,
                            instrument_ids=instrument_ids,
                            asset_type=asset_type,
                            date_range=date_range,
                            record_count=0,  # Fast path - no counting
                            data_types=data_types,
                            underlying_symbol=symbol_underlying or underlying_symbol
                        )
                        symbols_info.append(symbol_info)
            
            logger.info(f"Fast extracted {len(symbols_info)} symbols from native metadata, "
                       f"underlying: {underlying_symbol}")
            
            return symbols_info
            
        except Exception as e:
            logger.warning(f"Error in fast extraction from native metadata: {e}")
            return None

    def _extract_symbols_info(self, store: DBNStore) -> List[SymbolInfo]:
        """
        Extract symbol information from DBN store using native metadata when available.
        
        Args:
            store: DBNStore instance
            
        Returns:
            List of SymbolInfo objects
        """
        symbols_info = []
        
        try:
            # First try to use native DBN metadata
            native_symbols = self._extract_from_native_metadata(store)
            if native_symbols:
                logger.info(f"Using native DBN metadata: found {len(native_symbols)} symbols")
                return native_symbols
            
            # Fallback to sampling if native metadata is not available
            logger.info("Native metadata not available, falling back to sampling")
            symbols_data = self._sample_symbols_with_counts(store, sample_size=10000)
            
            if not symbols_data:
                logger.warning("No symbols found in DBN file")
                return symbols_info
            
            # Create symbol info for each symbol found
            for symbol, estimated_count in symbols_data.items():
                # Determine asset type - for OPRA data, it's options
                dataset_str = str(store.metadata.dataset).lower() if store.metadata.dataset else ''
                asset_type = AssetType.OPTIONS if 'opra' in dataset_str else self._determine_asset_type(symbol)
                
                # Use overall date range for each symbol
                date_range = self._get_symbol_date_range(store, symbol)
                
                # Determine data types from schema
                data_types = self._get_symbol_data_types(store, symbol)
                
                symbol_info = SymbolInfo(
                    symbol=symbol,
                    asset_type=asset_type,
                    date_range=date_range,
                    record_count=estimated_count,
                    data_types=data_types
                )
                
                symbols_info.append(symbol_info)
                
        except Exception as e:
            logger.warning(f"Error extracting symbols info: {e}")
            # Return empty list if we can't extract symbol information
            
        return symbols_info
    
    def _extract_from_native_metadata(self, store: DBNStore) -> Optional[List[SymbolInfo]]:
        """
        Extract symbol information from native DBN metadata.
        
        Args:
            store: DBNStore instance
            
        Returns:
            List of SymbolInfo objects or None if native metadata is not available
        """
        try:
            # Check if we have native symbol mappings
            if not hasattr(store.metadata, 'mappings') or not store.metadata.mappings:
                return None
            
            logger.info("Found native DBN symbol mappings")
            symbols_info = []
            
            # Get underlying symbol from native symbols list
            underlying_symbols = getattr(store.metadata, 'symbols', [])
            underlying_symbol = None
            if underlying_symbols:
                # Extract underlying symbol (e.g., 'SPXW.OPT' -> 'SPXW')
                raw_symbol = underlying_symbols[0]
                if '.OPT' in raw_symbol:
                    underlying_symbol = raw_symbol.replace('.OPT', '')
                else:
                    underlying_symbol = raw_symbol
            
            # Get date range from metadata
            date_range = self._get_symbol_date_range(store, '')
            
            # Determine asset type from dataset
            dataset_str = str(store.metadata.dataset).lower() if store.metadata.dataset else ''
            asset_type = AssetType.OPTIONS if 'opra' in dataset_str else AssetType.EQUITIES
            
            # Get data types from schema
            data_types = self._get_symbol_data_types(store, '')
            
            # Count actual records to get accurate total
            total_records = self._count_total_records(store)
            
            # Process each symbol in mappings
            symbol_counts = {}
            for symbol_str in store.metadata.mappings:
                # Clean up the symbol string and count occurrences
                clean_symbol = symbol_str.strip()
                if clean_symbol:
                    symbol_counts[clean_symbol] = symbol_counts.get(clean_symbol, 0) + 1
            
            # If we have actual record counts, distribute them proportionally
            if total_records > 0 and symbol_counts:
                # For now, assume roughly equal distribution
                # In a more sophisticated version, we'd count per symbol
                avg_records_per_symbol = total_records // len(symbol_counts)
                
                for symbol, _ in symbol_counts.items():
                    symbol_info = SymbolInfo(
                        symbol=symbol,
                        asset_type=asset_type,
                        date_range=date_range,
                        record_count=avg_records_per_symbol,
                        data_types=data_types,
                        underlying_symbol=underlying_symbol  # Add underlying symbol info
                    )
                    symbols_info.append(symbol_info)
            
            logger.info(f"Extracted {len(symbols_info)} symbols from native metadata, "
                       f"underlying: {underlying_symbol}, total records: {total_records}")
            
            return symbols_info
            
        except Exception as e:
            logger.warning(f"Error extracting from native metadata: {e}")
            return None
    
    def _count_total_records(self, store: DBNStore) -> int:
        """
        Count total records in the store efficiently.
        
        Args:
            store: DBNStore instance
            
        Returns:
            Total number of records
        """
        try:
            # Try to count all records
            count = 0
            for _ in store:
                count += 1
                # Log progress for large files
                if count % 50000 == 0:
                    logger.info(f"Counted {count} records so far...")
            
            logger.info(f"Total records counted: {count}")
            return count
            
        except Exception as e:
            logger.warning(f"Error counting total records: {e}")
            return 0
    
    def _sample_symbols_from_store(self, store: DBNStore, sample_size: int = 10000) -> List[str]:
        """
        Sample records from store to identify unique symbols.
        
        Args:
            store: DBNStore instance
            sample_size: Number of records to sample
            
        Returns:
            List of unique symbols found
        """
        symbols = set()
        
        try:
            # Read a sample of records to identify symbols
            count = 0
            for record in store:
                # DBN files use instrument_id directly
                if hasattr(record, 'instrument_id'):
                    symbols.add(str(record.instrument_id))
                elif hasattr(record, 'hd') and hasattr(record.hd, 'instrument_id'):
                    symbols.add(str(record.hd.instrument_id))
                elif hasattr(record, 'symbol'):
                    symbols.add(record.symbol)
                
                count += 1
                if count >= sample_size:
                    break
                    
        except Exception as e:
            logger.warning(f"Error sampling symbols: {e}")
            
        return list(symbols)
    
    def _sample_symbols_with_counts(self, store: DBNStore, sample_size: int = 10000) -> Dict[str, int]:
        """
        Sample records from store to identify unique symbols and estimate their counts.
        
        Args:
            store: DBNStore instance
            sample_size: Number of records to sample
            
        Returns:
            Dictionary mapping symbols to estimated record counts
        """
        symbol_counts = {}
        
        try:
            # Read a sample of records to count symbols
            count = 0
            for record in store:
                # DBN files use instrument_id directly
                symbol = None
                if hasattr(record, 'instrument_id'):
                    symbol = str(record.instrument_id)
                elif hasattr(record, 'hd') and hasattr(record.hd, 'instrument_id'):
                    symbol = str(record.hd.instrument_id)
                elif hasattr(record, 'symbol'):
                    symbol = record.symbol
                
                if symbol:
                    symbol_counts[symbol] = symbol_counts.get(symbol, 0) + 1
                
                count += 1
                if count >= sample_size:
                    break
            
            # If we sampled less than the full file, extrapolate the counts
            if count == sample_size:
                # Estimate total file size and scale up counts
                try:
                    # Try to get a rough estimate of total records by continuing to count
                    # for a bit more to get a better sample ratio
                    additional_count = 0
                    for record in store:
                        additional_count += 1
                        if additional_count >= 1000:  # Sample another 1000 records
                            break
                    
                    if additional_count > 0:
                        # Rough extrapolation based on sample
                        total_sampled = count + additional_count
                        scale_factor = max(1.0, total_sampled / count)
                        
                        # Scale up the counts
                        for symbol in symbol_counts:
                            symbol_counts[symbol] = int(symbol_counts[symbol] * scale_factor)
                            
                except Exception as e:
                    logger.warning(f"Error extrapolating counts: {e}")
                    # Just use the sampled counts as-is
                    
        except Exception as e:
            logger.warning(f"Error sampling symbols with counts: {e}")
            
        return symbol_counts
    
    def _determine_asset_type(self, symbol: str) -> AssetType:
        """
        Determine asset type from symbol string.
        
        Args:
            symbol: Symbol string
            
        Returns:
            AssetType enum value
        """
        # Basic heuristics for asset type detection
        # This could be enhanced with more sophisticated logic
        
        if len(symbol) > 10 or any(char in symbol for char in ['C', 'P']) and any(char.isdigit() for char in symbol):
            # Likely an options symbol (contains C/P and numbers)
            return AssetType.OPTIONS
        elif len(symbol) <= 5 and symbol.isalpha():
            # Likely an equity symbol
            return AssetType.EQUITIES
        elif '/' in symbol or any(symbol.endswith(suffix) for suffix in ['USD', 'EUR', 'GBP']):
            # Likely forex
            return AssetType.FOREX
        else:
            # Default to equities
            return AssetType.EQUITIES
    
    def _get_symbol_date_range(self, store: DBNStore, symbol: str) -> DateRange:
        """
        Get date range for a specific symbol.
        
        Args:
            store: DBNStore instance
            symbol: Symbol to get date range for
            
        Returns:
            DateRange for the symbol
        """
        # Use filename parsing instead of native metadata timestamps to avoid timezone issues
        # This is more reliable and matches what we show in the UI
        
        # Fallback to today's date if no range available
        today = date.today()
        return DateRange(start_date=today, end_date=today)
    
    def _get_symbol_data_types(self, store: DBNStore, symbol: str) -> List[DataType]:
        """
        Get available data types for a symbol.
        
        Args:
            store: DBNStore instance
            symbol: Symbol to check
            
        Returns:
            List of available DataType values
        """
        # This is a simplified implementation
        # In practice, you'd need to analyze the actual record types in the file
        
        data_types = []
        
        # Try to determine from schema or record types
        schema = str(getattr(store.metadata, 'schema', ''))
        
        if 'ohlcv' in schema.lower():
            data_types.append(DataType.OHLCV)
        if 'trade' in schema.lower():
            data_types.append(DataType.TRADES)
        if 'quote' in schema.lower():
            data_types.append(DataType.QUOTES)
        if 'book' in schema.lower():
            data_types.append(DataType.BOOK)
        
        # Default to OHLCV if nothing detected
        if not data_types:
            data_types.append(DataType.OHLCV)
            
        return data_types
    
    def _estimate_symbol_record_count(self, store: DBNStore, symbol: str) -> int:
        """
        Estimate record count for a specific symbol.
        
        Args:
            store: DBNStore instance
            symbol: Symbol to estimate for
            
        Returns:
            Estimated record count
        """
        # This is a placeholder implementation
        # In practice, you'd need to count records for each symbol
        
        try:
            # Try to get total record count and divide by number of symbols
            # This is a rough estimate
            total_records = getattr(store.metadata, 'record_count', 0)
            if total_records > 0:
                # Rough estimate assuming equal distribution
                return total_records // max(1, len(getattr(store.metadata, 'symbols', [symbol])))
        except Exception:
            pass
            
        return 0
    
    def _parse_filename_metadata(self, file_path: Path) -> Optional[Dict[str, Any]]:
        """
        Parse metadata from filename as fallback.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            Dictionary with parsed metadata or None
        """
        filename = file_path.stem
        metadata = {}
        
        try:
            # Example: opra-pillar-20250804-20250903.ohlcv-1m.dbn
            parts = filename.split('.')
            
            if len(parts) >= 2:
                # Extract data type from second part (e.g., "ohlcv-1m")
                data_part = parts[1]
                if 'ohlcv' in data_part.lower():
                    metadata['data_types'] = [DataType.OHLCV]
                elif 'trade' in data_part.lower():
                    metadata['data_types'] = [DataType.TRADES]
                elif 'quote' in data_part.lower():
                    metadata['data_types'] = [DataType.QUOTES]
            
            # Extract date range from first part
            main_part = parts[0]
            date_parts = main_part.split('-')
            
            # Look for date patterns (YYYYMMDD)
            dates = []
            for part in date_parts:
                if len(part) == 8 and part.isdigit():
                    try:
                        parsed_date = datetime.strptime(part, '%Y%m%d').date()
                        dates.append(parsed_date)
                    except ValueError:
                        continue
            
            if len(dates) >= 2:
                metadata['overall_date_range'] = DateRange(
                    start_date=min(dates),
                    end_date=max(dates)
                )
                metadata['start_timestamp'] = datetime.combine(min(dates), datetime.min.time())
                metadata['end_timestamp'] = datetime.combine(max(dates), datetime.max.time())
            
            # Extract dataset info
            if 'opra' in filename.lower():
                metadata['dataset'] = 'OPRA'
                metadata['asset_types'] = [AssetType.OPTIONS]
            elif 'sip' in filename.lower():
                metadata['dataset'] = 'SIP'
                metadata['asset_types'] = [AssetType.EQUITIES]
            
            return metadata if metadata else None
            
        except Exception as e:
            logger.warning(f"Error parsing filename metadata: {e}")
            return None
    
    def stream_records(self, file_path: Path) -> Iterator[Any]:
        """
        Stream records from a DBN file with symbol mapping applied using native metadata.
        
        This method applies symbol mapping to each record using the databento library's
        native mappings metadata, which provides date-aware symbol resolution.

        Args:
            file_path: Path to the DBN file.

        Yields:
            Individual records from the DBN file, with symbol attribute added.
        """
        logger.info(f"Streaming records from {file_path} with symbol mapping")

        try:
            # CRITICAL FIX: Clear cache and mappings for each file to prevent cross-file conflicts
            # This fixes the multi-file import issue where INSTR_ symbols were created
            if hasattr(self, '_resolve_symbol_for_date'):
                self._resolve_symbol_for_date.cache_clear()
            self._date_aware_mapping = {}
            
            # Load the DBN store
            store = DBNStore.from_file(str(file_path))
            
            # Use fast manual symbol mapping (to_df is too slow for large files)
            record_count = 0
            
            # Build simple lookup
            logger.info("Using fast manual symbol mapping")
            
            # Build date-aware instrument_id -> symbol mapping
            # Key insight: instrument IDs are reused across dates for different symbols
            self._date_aware_mapping = {}
            if hasattr(store.metadata, 'mappings') and store.metadata.mappings:
                from datetime import datetime, timezone
                
                for symbol, date_ranges in store.metadata.mappings.items():
                    if isinstance(date_ranges, list):
                        for date_range_info in date_ranges:
                            if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                                instrument_id = int(date_range_info['symbol'])
                                
                                # Extract date range - convert dates to timestamps for comparison
                                start_date = date_range_info.get('start_date')
                                end_date = date_range_info.get('end_date')
                                
                                if start_date and end_date:
                                    # Convert dates to timestamps (nanoseconds since epoch UTC)
                                    start_dt = datetime.combine(start_date, datetime.min.time()).replace(tzinfo=timezone.utc)
                                    end_dt = datetime.combine(end_date, datetime.max.time()).replace(tzinfo=timezone.utc)
                                    start_ts = int(start_dt.timestamp() * 1e9)
                                    end_ts = int(end_dt.timestamp() * 1e9)
                                    
                                    # Store mapping
                                    if instrument_id not in self._date_aware_mapping:
                                        self._date_aware_mapping[instrument_id] = []
                                    
                                    self._date_aware_mapping[instrument_id].append({
                                        'symbol': symbol,
                                        'start_ts': start_ts,
                                        'end_ts': end_ts,
                                        'start_date': start_date,
                                        'end_date': end_date
                                    })
                
                # Sort each instrument's mappings by start time for efficient lookup
                for instrument_id in self._date_aware_mapping:
                    self._date_aware_mapping[instrument_id].sort(key=lambda x: x['start_ts'])
            
            logger.info(f"Built date-aware mapping for {len(self._date_aware_mapping)} instrument IDs")
            
            for record in store:
                record_count += 1
                
                # Skip corrupted timestamps (UINT64_MAX values)
                timestamp = getattr(record, 'ts_event', None)
                if timestamp == 18446744073709551615:
                    continue
                
                # Apply date-aware symbol lookup (use DATE-based resolution like databento)
                instrument_id = getattr(record, 'instrument_id', None)
                timestamp = getattr(record, 'ts_event', 0)
                
                if instrument_id:
                    # Convert timestamp to date for databento-style resolution
                    from datetime import datetime, timezone
                    record_date = datetime.fromtimestamp(timestamp / 1e9, tz=timezone.utc).date()
                    
                    # Use cached date-based symbol resolution (like databento's InstrumentMap.resolve)
                    symbol = self._resolve_symbol_for_date(instrument_id, record_date)
                    
                    if symbol:
                        setattr(record, 'symbol', symbol)
                    else:
                        # Fallback to instrument ID for unmapped records
                        setattr(record, 'symbol', f"INSTR_{instrument_id}")
                else:
                    setattr(record, 'symbol', f"UNKNOWN_{record_count}")
                
                # Progress logging for large files
                if record_count % 1000000 == 0:
                    logger.info(f"Processed {record_count:,} records with fast mapping")

                yield record

            # Final statistics
            logger.info(f"Completed streaming {record_count:,} records with fast mapping")

        except Exception as e:
            logger.error(f"Error streaming records from {file_path}: {e}")
            raise
    
    def to_dataframe(self, file_path: Path, map_symbols: bool = True) -> 'pd.DataFrame':
        """
        Convert the entire DBN file to a pandas DataFrame with optional symbol mapping.
        
        This uses the databento library's to_df() method which can automatically
        handle symbol mapping when the DBN file contains native metadata.

        Args:
            file_path: Path to the DBN file.
            map_symbols: Whether to attempt symbol mapping (default: True).

        Returns:
            pandas DataFrame with symbol column added if mapping is available.
        """
        logger.info(f"Converting {file_path} to DataFrame using databento's to_df()")

        try:
            # Load the DBN store
            store = DBNStore.from_file(str(file_path))
            
            # Convert to DataFrame - the databento library handles symbol mapping
            # automatically if the DBN file contains the necessary metadata
            df = store.to_df()
            
            logger.info(f"Successfully converted to DataFrame: {len(df)} records, "
                       f"columns: {list(df.columns)}")
            
            # Check if symbol mapping was applied
            if 'symbol' in df.columns:
                sample_symbols = df['symbol'].unique()[:5]
                logger.info(f"Symbol mapping applied - sample symbols: {sample_symbols}")
            else:
                logger.info("No symbol column found - using instrument_id for identification")
            
            return df

        except Exception as e:
            logger.error(f"Error converting {file_path} to DataFrame: {e}")
            raise
    
    def get_symbol_mapping(self, file_path: Path) -> Dict[int, str]:
        """
        Extract symbol mapping from DBN file using the databento library's built-in functionality.
        
        This method provides a simple interface to get the symbol mappings, but the
        recommended approach is to use stream_records() or to_dataframe() which
        automatically handle symbol mapping with better performance.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            Dictionary mapping instrument_id to readable symbol
        """
        logger.info(f"Extracting symbol mapping from {file_path} using databento's built-in functionality")
        
        try:
            store = DBNStore.from_file(str(file_path))
            symbol_mapping = {}
            
            # Use the databento library's built-in symbol mapping
            if hasattr(store.metadata, 'mappings') and store.metadata.mappings:
                logger.info(f"Found {len(store.metadata.mappings)} native symbol mappings")
                
                # Build date-aware mapping instead of simple mapping to avoid conflicts
                date_aware_mapping = {}
                
                # Extract the mappings using the native metadata with date ranges
                for readable_symbol, date_ranges in store.metadata.mappings.items():
                    if isinstance(date_ranges, list):
                        for date_range_info in date_ranges:
                            if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                                instrument_id = int(date_range_info['symbol'])
                                start_date = date_range_info.get('start_date')
                                end_date = date_range_info.get('end_date')
                                
                                # Create date-aware mapping
                                if instrument_id not in date_aware_mapping:
                                    date_aware_mapping[instrument_id] = []
                                
                                date_aware_mapping[instrument_id].append((start_date, end_date, readable_symbol))
                

                
                logger.info(f"Built date-aware mapping for {len(date_aware_mapping)} instrument IDs")
                return date_aware_mapping
            else:
                logger.warning("No native symbol mappings found in DBN metadata")
                return {}
            
        except Exception as e:
            logger.error(f"Error extracting symbol mapping from {file_path}: {e}")
            return {}

    def get_file_info(self, file_path: Path) -> Dict[str, Any]:
        """
        Get basic file information without full metadata extraction.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            Dictionary with basic file information
        """
        try:
            stat = file_path.stat()
            
            return {
                'filename': file_path.name,
                'file_path': str(file_path),
                'file_size': stat.st_size,
                'modified_at': datetime.fromtimestamp(stat.st_mtime),
                'size_mb': round(stat.st_size / (1024 * 1024), 2),
                'size_gb': round(stat.st_size / (1024 * 1024 * 1024), 2)
            }
            
        except Exception as e:
            logger.error(f"Error getting file info for {file_path}: {e}")
            raise
    
    def get_available_symbols(self, file_path: Path) -> List[str]:
        """
        Get list of all available symbols in the DBN file using native metadata.
        
        This is a lightweight method to quickly get the symbols without processing
        the entire file. For full data processing, use stream_records() or to_dataframe().
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            List of symbol strings available in the file
        """
        logger.info(f"Getting available symbols from {file_path}")
        
        try:
            store = DBNStore.from_file(str(file_path))
            symbols = []
            
            # Extract symbols from native metadata
            if hasattr(store.metadata, 'mappings') and store.metadata.mappings:
                symbols = list(store.metadata.mappings.keys())
                logger.info(f"Found {len(symbols)} symbols in native metadata")
            else:
                logger.warning("No native symbol mappings found")
            
            return symbols
            
        except Exception as e:
            logger.error(f"Error getting available symbols from {file_path}: {e}")
            return []
    
    def _build_simple_mapping_from_native(self, store: 'DBNStore') -> Dict[int, str]:
        """
        Build simple symbol mapping using databento's native metadata functionality.
        
        This method extracts instrument_id to readable symbol mappings directly
        from the DBN file's embedded metadata, using the same approach as DBN CLI
        for maximum compatibility and data completeness.
        
        Args:
            store: DBNStore instance
            
        Returns:
            Dictionary mapping instrument_id to readable symbol
        """
        simple_mapping = {}
        
        try:
            if hasattr(store.metadata, 'mappings') and store.metadata.mappings:
                logger.info(f"Found {len(store.metadata.mappings)} native symbol mappings")
                
                # Build simple mapping like DBN CLI does - take the most recent/valid mapping
                for readable_symbol, date_ranges in store.metadata.mappings.items():
                    if isinstance(date_ranges, list):
                        for date_range_info in date_ranges:
                            if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                                instrument_id = int(date_range_info['symbol'])
                                
                                # Simple mapping: instrument_id -> readable_symbol
                                # If there are conflicts, the last one wins (like DBN CLI behavior)
                                simple_mapping[instrument_id] = readable_symbol
                
                logger.info(f"Built simple mapping for {len(simple_mapping)} instrument IDs")
            else:
                logger.warning("No native mappings found in DBN metadata")
                
        except Exception as e:
            logger.error(f"Error building simple symbol mapping from native metadata: {e}")
        
        return simple_mapping

    def _build_conflict_free_mapping_from_native(self, store: 'DBNStore') -> Dict[int, str]:
        """
        Build conflict-free symbol mapping that prevents phantom records.
        
        Unlike the simple mapping that uses "last wins" and creates conflicts,
        this mapping ensures each instrument_id maps to exactly one symbol,
        preventing the phantom record issue.
        
        Args:
            store: DBNStore instance
            
        Returns:
            Dictionary mapping instrument_id to readable symbol (conflict-free)
        """
        conflict_free_mapping = {}
        symbol_to_instruments = {}  # Track which instruments are mapped to each symbol
        
        try:
            if hasattr(store.metadata, 'mappings') and store.metadata.mappings:
                logger.info(f"Found {len(store.metadata.mappings)} native symbol mappings")
                
                # First pass: Build reverse mapping to detect conflicts
                for readable_symbol, date_ranges in store.metadata.mappings.items():
                    if isinstance(date_ranges, list):
                        for date_range_info in date_ranges:
                            if isinstance(date_range_info, dict) and 'symbol' in date_range_info:
                                instrument_id = int(date_range_info['symbol'])
                                
                                if readable_symbol not in symbol_to_instruments:
                                    symbol_to_instruments[readable_symbol] = []
                                symbol_to_instruments[readable_symbol].append(instrument_id)
                
                # Second pass: Create conflict-free mapping
                # For symbols with multiple instruments, choose the best one to avoid conflicts
                for readable_symbol, instrument_ids in symbol_to_instruments.items():
                    if len(instrument_ids) == 1:
                        # No conflict - safe to map
                        conflict_free_mapping[instrument_ids[0]] = readable_symbol
                    else:
                        # Conflict detected - use first instrument_id to prevent phantoms
                        first_instrument_id = instrument_ids[0]
                        conflict_free_mapping[first_instrument_id] = readable_symbol
                        
                        logger.debug(f"Symbol conflict for '{readable_symbol}': "
                                   f"instruments {instrument_ids}. Using {first_instrument_id}")
                
                logger.info(f"Built conflict-free mapping for {len(conflict_free_mapping)} instrument IDs")
            else:
                logger.warning("No native mappings found in DBN metadata")
                
        except Exception as e:
            logger.error(f"Error building conflict-free symbol mapping from native metadata: {e}")
        
        return conflict_free_mapping
    
    def _get_symbol_for_instrument_and_timestamp(self, symbol_mapping: Dict[int, List[tuple]], 
                                               instrument_id: int, timestamp: Optional[int]) -> Optional[str]:
        """
        Get the correct symbol for an instrument_id based on timestamp using date-aware mapping.
        
        Args:
            symbol_mapping: Dictionary mapping instrument_id to list of (start_date, end_date, symbol) tuples
            instrument_id: The instrument_id to look up
            timestamp: The timestamp of the record (nanoseconds since epoch)
            
        Returns:
            The correct symbol for this instrument_id at this timestamp, or None if not found
        """
        if instrument_id not in symbol_mapping:
            return None
        
        date_ranges = symbol_mapping[instrument_id]
        
        # If we have a timestamp, convert it to date for date-aware mapping
        if timestamp is not None:
            try:
                from datetime import datetime, timezone
                record_date = datetime.fromtimestamp(timestamp / 1e9, tz=timezone.utc).date()
                
                # Find the date range that contains this record's date
                for start_date, end_date, symbol in date_ranges:
                    if start_date and end_date and start_date <= record_date <= end_date:
                        return symbol
                
                # If no exact match found, use the most recent range
                from datetime import date
                sorted_ranges = sorted(date_ranges, key=lambda x: x[0] if x[0] else date.min, reverse=True)
                return sorted_ranges[0][2] if sorted_ranges else None
                
            except Exception as e:
                logger.debug(f"Error converting timestamp {timestamp} to date: {e}")
        
        # Fallback: use the first available symbol if timestamp processing fails
        return date_ranges[0][2] if date_ranges else None
