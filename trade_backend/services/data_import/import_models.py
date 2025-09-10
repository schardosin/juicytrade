"""
Data Import Models

Pydantic models for the data import system, defining the structure for
DBN file metadata, import requests, job status, and related data types.
"""

from datetime import datetime, date
from typing import Dict, List, Optional, Any, Union
from enum import Enum
from pydantic import BaseModel, Field, validator
from pathlib import Path


class ImportFileType(str, Enum):
    """Supported import file types"""
    DBN = "dbn"
    CSV = "csv"


class CSVFormat(str, Enum):
    """Supported CSV formats"""
    TRADESTATION = "tradestation"
    GENERIC = "generic"
    AUTO_DETECT = "auto_detect"


class TimestampConvention(str, Enum):
    """Timestamp convention for CSV data"""
    BEGIN_OF_MINUTE = "begin_of_minute"  # 9:30 represents 9:30-9:31 bar (standard)
    END_OF_MINUTE = "end_of_minute"      # 9:31 represents 9:30-9:31 bar (TradeStation)


class DataType(str, Enum):
    """Supported data types from DBN files"""
    OHLCV = "ohlcv"
    TRADES = "trades"
    QUOTES = "quotes"
    BOOK = "book"
    GREEKS = "greeks"
    STATS = "stats"


class ImportJobStatus(str, Enum):
    """Import job status values"""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class AssetType(str, Enum):
    """Asset types supported"""
    OPTIONS = "options"
    EQUITIES = "equities"
    FUTURES = "futures"
    FOREX = "forex"


class DateRange(BaseModel):
    """Date range for filtering data"""
    start_date: date
    end_date: date
    
    @validator('end_date')
    def end_after_start(cls, v, values):
        if 'start_date' in values and v < values['start_date']:
            raise ValueError('end_date must be after start_date')
        return v
    
    def __str__(self) -> str:
        return f"{self.start_date} to {self.end_date}"


class SymbolInfo(BaseModel):
    """Information about a symbol in the DBN file"""
    symbol: str
    asset_type: AssetType
    date_range: DateRange
    record_count: int
    data_types: List[DataType]
    underlying_symbol: Optional[str] = None  # For options: the underlying stock symbol (e.g., 'SPXW')
    
    class Config:
        use_enum_values = True


class DBNMetadata(BaseModel):
    """Metadata extracted from a DBN file"""
    filename: str
    file_path: str
    file_size: int
    created_at: datetime
    modified_at: datetime
    
    # DBN-specific metadata
    dataset: Optional[str] = None
    schema: Optional[str] = None
    stype_in: Optional[str] = None
    stype_out: Optional[str] = None
    start_timestamp: Optional[datetime] = None
    end_timestamp: Optional[datetime] = None
    
    # Extracted information
    symbols: List[SymbolInfo] = []
    total_records: int = 0
    data_types: List[DataType] = []
    asset_types: List[AssetType] = []
    overall_date_range: Optional[DateRange] = None
    
    # Parsing information
    parsed_from_filename: bool = False
    metadata_extraction_error: Optional[str] = None
    
    class Config:
        use_enum_values = True


class ImportFileInfo(BaseModel):
    """Basic information about an import file (DBN or CSV)"""
    filename: str
    file_path: str
    file_size: int
    modified_at: datetime
    file_type: ImportFileType
    metadata: Optional[DBNMetadata] = None
    metadata_cached: bool = False
    metadata_cache_time: Optional[datetime] = None
    
    # CSV-specific fields
    csv_format: Optional[CSVFormat] = None
    csv_headers: Optional[List[str]] = None
    csv_sample_data: Optional[List[List[str]]] = None
    needs_symbol_input: bool = False
    csv_structure_analysis: Optional[Dict[str, Any]] = None
    
    @property
    def size_mb(self) -> float:
        """File size in MB"""
        return round(self.file_size / (1024 * 1024), 2)
    
    @property
    def size_gb(self) -> float:
        """File size in GB"""
        return round(self.file_size / (1024 * 1024 * 1024), 2)


# Backward compatibility alias
DBNFileInfo = ImportFileInfo


class ImportFilters(BaseModel):
    """Filters for selective data import"""
    symbols: Optional[List[str]] = None
    date_range: Optional[DateRange] = None
    data_types: Optional[List[DataType]] = None
    asset_types: Optional[List[AssetType]] = None
    
    class Config:
        use_enum_values = True


class ImportRequest(BaseModel):
    """Request to start a data import job"""
    filename: str
    filters: Optional[ImportFilters] = None
    output_format: str = Field(default="parquet", description="Output format (currently only parquet)")
    overwrite_existing: bool = Field(default=False, description="Overwrite existing data")
    job_name: Optional[str] = None
    
    # CSV-specific fields
    file_type: Optional[ImportFileType] = None
    csv_symbol: Optional[str] = None  # Required for CSV files
    csv_format: Optional[CSVFormat] = None
    timestamp_convention: Optional[TimestampConvention] = Field(
        default=TimestampConvention.BEGIN_OF_MINUTE,
        description="Timestamp convention for CSV data"
    )
    
    @validator('output_format')
    def validate_output_format(cls, v):
        if v.lower() != 'parquet':
            raise ValueError('Currently only parquet output format is supported')
        return v.lower()
    
    @validator('csv_symbol')
    def validate_csv_symbol(cls, v, values):
        file_type = values.get('file_type')
        filename = values.get('filename', '')
        
        # Check if this is a CSV file (either by file_type or filename extension)
        is_csv_file = (
            file_type == ImportFileType.CSV or 
            file_type == "csv" or 
            filename.lower().endswith('.csv')
        )
        
        if is_csv_file and not v:
            raise ValueError('csv_symbol is required for CSV file imports')
        return v


class ImportProgress(BaseModel):
    """Progress information for an import job"""
    total_records: int = 0
    processed_records: int = 0
    current_symbol: Optional[str] = None
    current_date: Optional[date] = None
    estimated_completion: Optional[datetime] = None
    
    # Day-based progress tracking fields
    progress_percentage: Optional[float] = None  # Override calculated percentage with day-based progress
    current_month: Optional[str] = None
    trading_day_in_month: Optional[int] = None
    total_months: Optional[int] = None
    current_month_index: Optional[int] = None
    status_message: Optional[str] = None
    
    # Symbol tracking for frontend overlay matching
    primary_symbols: Optional[List[str]] = None
    
    def get_progress_percentage(self) -> float:
        """Get progress as percentage (0-100), preferring day-based over record-based"""
        # Use day-based progress if available
        if self.progress_percentage is not None:
            return round(self.progress_percentage, 2)
        
        # Fallback to record-based progress
        if self.total_records == 0:
            return 0.0
        return round((self.processed_records / self.total_records) * 100, 2)


class ImportJobInfo(BaseModel):
    """Complete information about an import job"""
    job_id: str
    job_name: Optional[str] = None
    filename: str
    status: ImportJobStatus
    created_at: datetime
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    
    # Request details
    filters: Optional[ImportFilters] = None
    output_format: str = "parquet"
    overwrite_existing: bool = False
    
    # CSV-specific fields
    file_type: Optional[ImportFileType] = None
    csv_symbol: Optional[str] = None
    csv_format: Optional[CSVFormat] = None
    timestamp_convention: Optional[TimestampConvention] = None
    
    # Progress and results
    progress: ImportProgress = ImportProgress()
    error_message: Optional[str] = None
    output_paths: List[str] = []
    
    # Statistics
    symbols_processed: List[str] = []
    total_output_size: int = 0
    
    class Config:
        use_enum_values = True
    
    @property
    def duration_seconds(self) -> Optional[float]:
        """Job duration in seconds"""
        if not self.started_at:
            return None
        end_time = self.completed_at or datetime.utcnow()
        return (end_time - self.started_at).total_seconds()
    
    @property
    def is_active(self) -> bool:
        """Whether the job is currently active"""
        return self.status in [ImportJobStatus.PENDING, ImportJobStatus.RUNNING]
    
    @property
    def is_finished(self) -> bool:
        """Whether the job has finished (successfully or not)"""
        return self.status in [ImportJobStatus.COMPLETED, ImportJobStatus.FAILED, ImportJobStatus.CANCELLED]


class ImportSummary(BaseModel):
    """Summary of import operations"""
    total_jobs: int = 0
    active_jobs: int = 0
    completed_jobs: int = 0
    failed_jobs: int = 0
    total_files_imported: int = 0
    total_data_size: int = 0
    
    @property
    def success_rate(self) -> float:
        """Success rate as percentage"""
        if self.total_jobs == 0:
            return 0.0
        return round((self.completed_jobs / self.total_jobs) * 100, 2)
