"""
CSV Metadata Extractor

Extracts metadata from CSV files by analyzing their content structure,
date ranges, and data quality. Does not rely on filename parsing.
"""

import json
import logging
from datetime import datetime, date
from pathlib import Path
from typing import Dict, List, Optional, Any, Tuple
import hashlib

from .csv_reader import CSVReader, CSVFormat
from .import_models import (
    DBNMetadata, SymbolInfo, DateRange, DataType, AssetType
)

logger = logging.getLogger(__name__)


class CSVMetadataExtractor:
    """
    Extracts comprehensive metadata from CSV files by analyzing content.
    """
    
    def __init__(self):
        """Initialize CSV metadata extractor"""
        self.csv_reader = CSVReader()
        self.cache_dir = Path("metadata_cache")
        self.cache_dir.mkdir(exist_ok=True)
    
    def get_file_metadata(self, file_path: Path, symbol: str, 
                         force_refresh: bool = False) -> DBNMetadata:
        """
        Extract comprehensive metadata from CSV file.
        
        Args:
            file_path: Path to CSV file
            symbol: Symbol name provided by user
            force_refresh: Force refresh of cached metadata
            
        Returns:
            DBNMetadata object with CSV-specific information
        """
        try:
            # Check cache first
            if not force_refresh:
                cached_metadata = self._load_cached_metadata(file_path, symbol)
                if cached_metadata:
                    logger.info(f"Using cached metadata for {file_path}")
                    return cached_metadata
            
            logger.info(f"Extracting metadata from CSV file: {file_path}")
            
            # Get basic file info
            file_stat = file_path.stat()
            
            # Detect CSV format
            csv_format, parser_config = self.csv_reader.detect_format(file_path)
            
            # Get sample data for preview
            headers, sample_rows = self.csv_reader.get_sample_data(file_path, 10)
            
            # Analyze file content
            record_count = self.csv_reader.get_record_count(file_path)
            start_date, end_date = self.csv_reader.get_date_range(file_path, symbol)
            
            # Determine asset type from symbol
            asset_type = self._determine_asset_type(symbol)
            
            # Create date range
            date_range = None
            if start_date and end_date:
                date_range = DateRange(start_date=start_date, end_date=end_date)
            
            # Create symbol info
            symbol_info = SymbolInfo(
                symbol=symbol,
                asset_type=asset_type,
                date_range=date_range,
                record_count=record_count,
                data_types=[DataType.OHLCV],  # CSV files typically contain OHLCV data
                underlying_symbol=self._extract_underlying_symbol(symbol)
            )
            
            # Analyze data quality
            data_quality = self._analyze_data_quality(file_path, symbol, sample_size=1000)
            
            # Create metadata object
            metadata = DBNMetadata(
                filename=file_path.name,
                file_path=str(file_path),
                file_size=file_stat.st_size,
                created_at=datetime.fromtimestamp(file_stat.st_ctime),
                modified_at=datetime.fromtimestamp(file_stat.st_mtime),
                
                # CSV-specific metadata
                dataset=f"CSV_{csv_format.upper()}",
                schema="csv_ohlcv",
                stype_in="csv",
                stype_out="ohlcv",
                start_timestamp=datetime.combine(start_date, datetime.min.time()) if start_date else None,
                end_timestamp=datetime.combine(end_date, datetime.max.time()) if end_date else None,
                
                # Extracted information
                symbols=[symbol_info],
                total_records=record_count,
                data_types=[DataType.OHLCV],
                asset_types=[asset_type],
                overall_date_range=date_range,
                
                # Parsing information
                parsed_from_filename=False,  # We don't use filename parsing
                metadata_extraction_error=None
            )
            
            # Add CSV-specific fields
            metadata.csv_format = csv_format.value
            metadata.csv_headers = headers
            metadata.csv_sample_data = sample_rows[:5]  # Store first 5 rows for preview
            metadata.csv_parser_config = parser_config
            metadata.data_quality = data_quality
            
            # Cache the metadata
            self._cache_metadata(file_path, symbol, metadata)
            
            logger.info(f"Successfully extracted metadata: {record_count} records, "
                       f"date range: {start_date} to {end_date}")
            
            return metadata
            
        except Exception as e:
            logger.error(f"Error extracting CSV metadata: {e}")
            
            # Return minimal metadata on error
            file_stat = file_path.stat()
            return DBNMetadata(
                filename=file_path.name,
                file_path=str(file_path),
                file_size=file_stat.st_size,
                created_at=datetime.fromtimestamp(file_stat.st_ctime),
                modified_at=datetime.fromtimestamp(file_stat.st_mtime),
                dataset="CSV_UNKNOWN",
                symbols=[],
                total_records=0,
                data_types=[],
                asset_types=[],
                parsed_from_filename=False,
                metadata_extraction_error=str(e)
            )
    
    def analyze_csv_structure(self, file_path: Path) -> Dict[str, Any]:
        """
        Analyze CSV structure without requiring symbol input.
        Used for initial file analysis before user provides symbol.
        
        Args:
            file_path: Path to CSV file
            
        Returns:
            Dictionary with structure analysis
        """
        try:
            # Get basic file info
            file_stat = file_path.stat()
            
            # Detect format
            csv_format, parser_config = self.csv_reader.detect_format(file_path)
            
            # Get sample data
            headers, sample_rows = self.csv_reader.get_sample_data(file_path, 10)
            
            # Get record count
            record_count = self.csv_reader.get_record_count(file_path)
            
            # Try to extract date range with dummy symbol
            try:
                start_date, end_date = self.csv_reader.get_date_range(file_path, "TEMP")
            except Exception:
                start_date, end_date = None, None
            
            return {
                'filename': file_path.name,
                'file_size': file_stat.st_size,
                'modified_at': datetime.fromtimestamp(file_stat.st_mtime),
                'csv_format': csv_format.value,
                'headers': headers,
                'sample_data': sample_rows[:5],
                'record_count': record_count,
                'start_date': start_date.isoformat() if start_date else None,
                'end_date': end_date.isoformat() if end_date else None,
                'parser_config': parser_config,
                'needs_symbol_input': True  # Always true for CSV files
            }
            
        except Exception as e:
            logger.error(f"Error analyzing CSV structure: {e}")
            return {
                'filename': file_path.name,
                'error': str(e),
                'needs_symbol_input': True
            }
    
    def _determine_asset_type(self, symbol: str) -> AssetType:
        """
        Determine asset type from symbol.
        
        Args:
            symbol: Symbol string
            
        Returns:
            AssetType enum value
        """
        symbol_upper = symbol.upper()
        
        # Common index symbols
        if symbol_upper in ['SPX', 'SPY', 'QQQ', 'IWM', 'DIA', 'VIX', 'NDX', 'RUT']:
            return AssetType.EQUITIES
        
        # Forex patterns
        if '/' in symbol or any(symbol_upper.endswith(curr) for curr in ['USD', 'EUR', 'GBP', 'JPY', 'CHF', 'CAD', 'AUD']):
            return AssetType.FOREX
        
        # Futures patterns (common suffixes)
        if any(symbol_upper.endswith(suffix) for suffix in ['H3', 'M3', 'U3', 'Z3', 'F4', 'G4', 'H4', 'J4', 'K4', 'M4', 'N4', 'Q4', 'U4', 'V4', 'X4', 'Z4']):
            return AssetType.FUTURES
        
        # Options patterns (long symbols with strikes/expiry)
        if len(symbol) > 10 or any(char in symbol for char in ['C', 'P']) and any(char.isdigit() for char in symbol):
            return AssetType.OPTIONS
        
        # Default to equities for most cases
        return AssetType.EQUITIES
    
    def _extract_underlying_symbol(self, symbol: str) -> str:
        """
        Extract underlying symbol for partitioning.
        
        Args:
            symbol: Full symbol
            
        Returns:
            Underlying symbol for partitioning
        """
        # For most cases, the symbol itself is the underlying
        # This could be enhanced for complex option symbols
        return symbol.upper()
    
    def _analyze_data_quality(self, file_path: Path, symbol: str, sample_size: int = 1000) -> Dict[str, Any]:
        """
        Analyze data quality by sampling records.
        
        Args:
            file_path: Path to CSV file
            symbol: Symbol name
            sample_size: Number of records to sample
            
        Returns:
            Dictionary with data quality metrics
        """
        try:
            quality_metrics = {
                'total_records_sampled': 0,
                'valid_records': 0,
                'invalid_records': 0,
                'missing_prices': 0,
                'zero_volume_records': 0,
                'date_gaps': [],
                'price_anomalies': [],
                'sample_price_range': {'min': None, 'max': None}
            }
            
            prices = []
            previous_date = None
            
            # Sample records for quality analysis
            for i, record in enumerate(self.csv_reader.stream_records(file_path, symbol)):
                if i >= sample_size:
                    break
                
                quality_metrics['total_records_sampled'] += 1
                
                if record:
                    quality_metrics['valid_records'] += 1
                    
                    # Check for missing prices
                    if not record.close or record.close <= 0:
                        quality_metrics['missing_prices'] += 1
                    else:
                        prices.append(record.close)
                    
                    # Check for zero volume
                    if record.volume == 0:
                        quality_metrics['zero_volume_records'] += 1
                    
                    # Check for date gaps (simplified)
                    if previous_date and record.date:
                        days_diff = (record.date - previous_date).days
                        if days_diff > 3:  # Weekend + 1 day gap
                            quality_metrics['date_gaps'].append({
                                'from': previous_date.isoformat(),
                                'to': record.date.isoformat(),
                                'days': days_diff
                            })
                    
                    previous_date = record.date
                else:
                    quality_metrics['invalid_records'] += 1
            
            # Calculate price range
            if prices:
                quality_metrics['sample_price_range']['min'] = min(prices)
                quality_metrics['sample_price_range']['max'] = max(prices)
            
            # Calculate quality score (0-100)
            if quality_metrics['total_records_sampled'] > 0:
                valid_ratio = quality_metrics['valid_records'] / quality_metrics['total_records_sampled']
                quality_metrics['quality_score'] = int(valid_ratio * 100)
            else:
                quality_metrics['quality_score'] = 0
            
            return quality_metrics
            
        except Exception as e:
            logger.error(f"Error analyzing data quality: {e}")
            return {'error': str(e), 'quality_score': 0}
    
    def _get_cache_key(self, file_path: Path, symbol: str) -> str:
        """
        Generate cache key for file and symbol combination.
        
        Args:
            file_path: Path to file
            symbol: Symbol name
            
        Returns:
            Cache key string
        """
        # Include file path, symbol, and modification time in cache key
        file_stat = file_path.stat()
        cache_input = f"{file_path}_{symbol}_{file_stat.st_mtime}_{file_stat.st_size}"
        return hashlib.md5(cache_input.encode()).hexdigest()
    
    def _cache_metadata(self, file_path: Path, symbol: str, metadata: DBNMetadata):
        """
        Cache metadata to disk.
        
        Args:
            file_path: Path to source file
            symbol: Symbol name
            metadata: Metadata to cache
        """
        try:
            cache_key = self._get_cache_key(file_path, symbol)
            cache_file = self.cache_dir / f"csv_{cache_key}.json"
            
            # Convert metadata to dict for JSON serialization
            metadata_dict = metadata.model_dump()
            
            with open(cache_file, 'w') as f:
                json.dump(metadata_dict, f, indent=2, default=str)
            
            logger.debug(f"Cached metadata for {file_path} with symbol {symbol}")
            
        except Exception as e:
            logger.warning(f"Error caching metadata: {e}")
    
    def _load_cached_metadata(self, file_path: Path, symbol: str) -> Optional[DBNMetadata]:
        """
        Load cached metadata from disk.
        
        Args:
            file_path: Path to source file
            symbol: Symbol name
            
        Returns:
            Cached metadata or None if not found/invalid
        """
        try:
            cache_key = self._get_cache_key(file_path, symbol)
            cache_file = self.cache_dir / f"csv_{cache_key}.json"
            
            if not cache_file.exists():
                return None
            
            with open(cache_file, 'r') as f:
                metadata_dict = json.load(f)
            
            # Convert back to DBNMetadata object
            return DBNMetadata(**metadata_dict)
            
        except Exception as e:
            logger.debug(f"Error loading cached metadata: {e}")
            return None
    
    def get_cache_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the metadata cache.
        
        Returns:
            Dictionary with cache statistics
        """
        try:
            cache_files = list(self.cache_dir.glob("csv_*.json"))
            total_size = sum(f.stat().st_size for f in cache_files)
            
            return {
                'cache_directory': str(self.cache_dir),
                'cached_files': len(cache_files),
                'total_cache_size_bytes': total_size,
                'total_cache_size_mb': round(total_size / (1024 * 1024), 2)
            }
            
        except Exception as e:
            logger.error(f"Error getting cache stats: {e}")
            return {'error': str(e)}
    
    def clear_cache(self, older_than_days: Optional[int] = None):
        """
        Clear metadata cache.
        
        Args:
            older_than_days: Only clear cache files older than this many days
        """
        try:
            removed_count = 0
            
            for cache_file in self.cache_dir.glob("csv_*.json"):
                should_remove = True
                
                if older_than_days:
                    file_age_days = (datetime.now().timestamp() - cache_file.stat().st_mtime) / (24 * 3600)
                    should_remove = file_age_days > older_than_days
                
                if should_remove:
                    cache_file.unlink()
                    removed_count += 1
            
            logger.info(f"Cleared {removed_count} cache files")
            
        except Exception as e:
            logger.error(f"Error clearing cache: {e}")


# Global instance
csv_metadata_extractor = CSVMetadataExtractor()
