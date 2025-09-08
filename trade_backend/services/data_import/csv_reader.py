"""
CSV Reader

Handles reading and parsing of CSV files from various trading platforms,
with support for TradeStation format and extensible architecture for other formats.
"""

import csv
import logging
from datetime import datetime, date
from pathlib import Path
from typing import Dict, List, Optional, Any, Iterator, Tuple
import pandas as pd
from enum import Enum

logger = logging.getLogger(__name__)


class CSVFormat(str, Enum):
    """Supported CSV formats"""
    TRADESTATION = "tradestation"
    GENERIC = "generic"
    AUTO_DETECT = "auto_detect"


class CSVRecord:
    """Standardized CSV record structure"""
    
    def __init__(self, **kwargs):
        # Core fields
        self.symbol = kwargs.get('symbol')
        self.timestamp = kwargs.get('timestamp')
        self.date = kwargs.get('date')
        
        # OHLCV data
        self.open = kwargs.get('open')
        self.high = kwargs.get('high')
        self.low = kwargs.get('low')
        self.close = kwargs.get('close')
        self.volume = kwargs.get('volume', 0)
        
        # Additional fields
        self.up_volume = kwargs.get('up_volume')
        self.down_volume = kwargs.get('down_volume')
        
        # Raw data for debugging
        self.raw_data = kwargs.get('raw_data', {})


class TradeStationCSVParser:
    """Parser for TradeStation CSV format"""
    
    EXPECTED_HEADERS = ['Date', 'Time', 'Open', 'High', 'Low', 'Close', 'Up', 'Down']
    
    @classmethod
    def can_parse(cls, headers: List[str]) -> bool:
        """Check if this parser can handle the given headers"""
        # Check if all expected headers are present (case-insensitive)
        headers_lower = [h.lower().strip('"') for h in headers]
        expected_lower = [h.lower() for h in cls.EXPECTED_HEADERS]
        
        return all(expected in headers_lower for expected in expected_lower)
    
    @classmethod
    def parse_row(cls, row: Dict[str, str], symbol: str) -> Optional[CSVRecord]:
        """Parse a single CSV row into a CSVRecord"""
        try:
            # Parse date and time
            date_str = row.get('Date', '').strip('"')
            time_str = row.get('Time', '').strip('"')
            
            if not date_str or not time_str:
                return None
            
            # Parse date (MM/DD/YYYY format)
            try:
                date_obj = datetime.strptime(date_str, '%m/%d/%Y').date()
            except ValueError:
                # Try alternative formats
                try:
                    date_obj = datetime.strptime(date_str, '%Y-%m-%d').date()
                except ValueError:
                    logger.warning(f"Could not parse date: {date_str}")
                    return None
            
            # Parse time (HH:MM format)
            try:
                time_obj = datetime.strptime(time_str, '%H:%M').time()
            except ValueError:
                logger.warning(f"Could not parse time: {time_str}")
                return None
            
            # Combine date and time
            timestamp = datetime.combine(date_obj, time_obj)
            
            # Parse price data
            open_price = float(row.get('Open', '0').strip('"'))
            high_price = float(row.get('High', '0').strip('"'))
            low_price = float(row.get('Low', '0').strip('"'))
            close_price = float(row.get('Close', '0').strip('"'))
            
            # Parse volume data
            up_volume = int(float(row.get('Up', '0').strip('"')))
            down_volume = int(float(row.get('Down', '0').strip('"')))
            total_volume = up_volume + down_volume
            
            return CSVRecord(
                symbol=symbol,
                timestamp=timestamp,
                date=date_obj,
                open=open_price,
                high=high_price,
                low=low_price,
                close=close_price,
                volume=total_volume,
                up_volume=up_volume,
                down_volume=down_volume,
                raw_data=row
            )
            
        except (ValueError, KeyError) as e:
            logger.warning(f"Error parsing CSV row: {e}, row: {row}")
            return None


class GenericCSVParser:
    """Generic CSV parser for common OHLCV formats"""
    
    COMMON_HEADERS = {
        'date': ['date', 'datetime', 'timestamp', 'time'],
        'open': ['open', 'o'],
        'high': ['high', 'h'],
        'low': ['low', 'l'],
        'close': ['close', 'c'],
        'volume': ['volume', 'vol', 'v']
    }
    
    @classmethod
    def can_parse(cls, headers: List[str]) -> bool:
        """Check if this parser can handle the given headers"""
        headers_lower = [h.lower().strip('"') for h in headers]
        
        # Check if we have at least date and close price
        has_date = any(date_header in headers_lower for date_header in cls.COMMON_HEADERS['date'])
        has_close = any(close_header in headers_lower for close_header in cls.COMMON_HEADERS['close'])
        
        return has_date and has_close
    
    @classmethod
    def map_headers(cls, headers: List[str]) -> Dict[str, str]:
        """Map CSV headers to standard field names"""
        headers_lower = [h.lower().strip('"') for h in headers]
        mapping = {}
        
        for field, possible_headers in cls.COMMON_HEADERS.items():
            for header in headers_lower:
                if header in possible_headers:
                    original_header = headers[headers_lower.index(header)]
                    mapping[field] = original_header
                    break
        
        return mapping
    
    @classmethod
    def parse_row(cls, row: Dict[str, str], symbol: str, header_mapping: Dict[str, str]) -> Optional[CSVRecord]:
        """Parse a single CSV row into a CSVRecord"""
        try:
            # Parse timestamp
            date_header = header_mapping.get('date')
            if not date_header:
                return None
            
            date_str = row.get(date_header, '').strip('"')
            if not date_str:
                return None
            
            # Try various date formats
            timestamp = None
            date_obj = None
            
            for date_format in ['%Y-%m-%d %H:%M:%S', '%Y-%m-%d %H:%M', '%Y-%m-%d', 
                               '%m/%d/%Y %H:%M:%S', '%m/%d/%Y %H:%M', '%m/%d/%Y']:
                try:
                    timestamp = datetime.strptime(date_str, date_format)
                    date_obj = timestamp.date()
                    break
                except ValueError:
                    continue
            
            if not timestamp:
                logger.warning(f"Could not parse date: {date_str}")
                return None
            
            # Parse price data
            open_price = None
            high_price = None
            low_price = None
            close_price = None
            volume = 0
            
            if 'open' in header_mapping:
                open_price = float(row.get(header_mapping['open'], '0').strip('"'))
            if 'high' in header_mapping:
                high_price = float(row.get(header_mapping['high'], '0').strip('"'))
            if 'low' in header_mapping:
                low_price = float(row.get(header_mapping['low'], '0').strip('"'))
            if 'close' in header_mapping:
                close_price = float(row.get(header_mapping['close'], '0').strip('"'))
            if 'volume' in header_mapping:
                volume = int(float(row.get(header_mapping['volume'], '0').strip('"')))
            
            return CSVRecord(
                symbol=symbol,
                timestamp=timestamp,
                date=date_obj,
                open=open_price,
                high=high_price,
                low=low_price,
                close=close_price,
                volume=volume,
                raw_data=row
            )
            
        except (ValueError, KeyError) as e:
            logger.warning(f"Error parsing CSV row: {e}, row: {row}")
            return None


class CSVReader:
    """
    Main CSV reader that handles different CSV formats and provides
    a unified interface for streaming CSV records.
    """
    
    def __init__(self):
        """Initialize CSV reader with available parsers"""
        self.parsers = {
            CSVFormat.TRADESTATION: TradeStationCSVParser,
            CSVFormat.GENERIC: GenericCSVParser
        }
    
    def detect_format(self, file_path: Path) -> Tuple[CSVFormat, Dict[str, Any]]:
        """
        Detect CSV format by analyzing headers and sample data.
        
        Args:
            file_path: Path to CSV file
            
        Returns:
            Tuple of (detected_format, parser_config)
        """
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                # Read first line to get headers
                csv_reader = csv.reader(f)
                headers = next(csv_reader)
                
                logger.info(f"Detected CSV headers: {headers}")
                
                # Try TradeStation format first (most specific)
                if TradeStationCSVParser.can_parse(headers):
                    logger.info("Detected TradeStation CSV format")
                    return CSVFormat.TRADESTATION, {}
                
                # Try generic format
                elif GenericCSVParser.can_parse(headers):
                    header_mapping = GenericCSVParser.map_headers(headers)
                    logger.info(f"Detected Generic CSV format with mapping: {header_mapping}")
                    return CSVFormat.GENERIC, {'header_mapping': header_mapping}
                
                else:
                    logger.warning(f"Could not detect CSV format for headers: {headers}")
                    return CSVFormat.GENERIC, {'header_mapping': {}}
                    
        except Exception as e:
            logger.error(f"Error detecting CSV format: {e}")
            return CSVFormat.GENERIC, {'header_mapping': {}}
    
    def get_sample_data(self, file_path: Path, num_rows: int = 5) -> Tuple[List[str], List[List[str]]]:
        """
        Get sample data from CSV file for preview.
        
        Args:
            file_path: Path to CSV file
            num_rows: Number of sample rows to return
            
        Returns:
            Tuple of (headers, sample_rows)
        """
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                csv_reader = csv.reader(f)
                headers = next(csv_reader)
                
                sample_rows = []
                for i, row in enumerate(csv_reader):
                    if i >= num_rows:
                        break
                    sample_rows.append(row)
                
                return headers, sample_rows
                
        except Exception as e:
            logger.error(f"Error reading sample data: {e}")
            return [], []
    
    def stream_records(self, file_path: Path, symbol: str, 
                      csv_format: Optional[CSVFormat] = None,
                      timestamp_convention: Optional[str] = None) -> Iterator[CSVRecord]:
        """
        Stream CSV records from file.
        
        Args:
            file_path: Path to CSV file
            symbol: Symbol name for the data
            csv_format: Optional format specification (auto-detect if None)
            timestamp_convention: Timestamp convention ('begin_of_minute' or 'end_of_minute')
            
        Yields:
            CSVRecord objects
        """
        try:
            # Detect format if not specified
            if csv_format is None:
                csv_format, parser_config = self.detect_format(file_path)
            else:
                parser_config = {}
            
            parser_class = self.parsers.get(csv_format)
            if not parser_class:
                logger.error(f"No parser available for format: {csv_format}")
                return
            
            logger.info(f"Streaming CSV records from {file_path} using {csv_format} format")
            
            with open(file_path, 'r', encoding='utf-8') as f:
                csv_reader = csv.DictReader(f)
                
                processed_count = 0
                error_count = 0
                
                for row in csv_reader:
                    try:
                        if csv_format == CSVFormat.GENERIC:
                            record = parser_class.parse_row(row, symbol, parser_config.get('header_mapping', {}))
                        else:
                            record = parser_class.parse_row(row, symbol)
                        
                        if record:
                            # Apply timestamp convention adjustment if needed
                            if timestamp_convention == 'end_of_minute' and record.timestamp:
                                from datetime import timedelta
                                # Subtract 1 minute to convert from end-of-minute to begin-of-minute
                                record.timestamp = record.timestamp - timedelta(minutes=1)
                                record.date = record.timestamp.date()
                                logger.debug(f"Adjusted timestamp from end-of-minute to begin-of-minute: {record.timestamp}")
                            
                            processed_count += 1
                            yield record
                        else:
                            error_count += 1
                            
                    except Exception as e:
                        error_count += 1
                        logger.warning(f"Error processing row {processed_count + error_count}: {e}")
                        continue
                
                logger.info(f"CSV streaming completed. Processed: {processed_count}, Errors: {error_count}")
                
        except Exception as e:
            logger.error(f"Error streaming CSV records: {e}")
            raise
    
    def get_record_count(self, file_path: Path) -> int:
        """
        Get total record count in CSV file.
        
        Args:
            file_path: Path to CSV file
            
        Returns:
            Number of data records (excluding header)
        """
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                csv_reader = csv.reader(f)
                next(csv_reader)  # Skip header
                return sum(1 for _ in csv_reader)
        except Exception as e:
            logger.error(f"Error counting CSV records: {e}")
            return 0
    
    def get_date_range(self, file_path: Path, symbol: str) -> Tuple[Optional[date], Optional[date]]:
        """
        Get date range from CSV file by scanning all records.
        
        Args:
            file_path: Path to CSV file
            symbol: Symbol name for the data
            
        Returns:
            Tuple of (start_date, end_date)
        """
        try:
            dates = []
            
            for record in self.stream_records(file_path, symbol):
                if record and record.date:
                    dates.append(record.date)
            
            if dates:
                return min(dates), max(dates)
            else:
                return None, None
                
        except Exception as e:
            logger.error(f"Error getting date range: {e}")
            return None, None
