"""
Import Manager

Orchestrates the data import process, managing async jobs, progress tracking,
and coordination between DBN reading, metadata extraction, and Parquet writing.
"""

import asyncio
import json
import logging
import uuid
from datetime import datetime, date, timezone
from pathlib import Path
from typing import Dict, List, Optional, Any, Callable
import threading
from concurrent.futures import ThreadPoolExecutor

from ...path_manager import path_manager
from .import_models import (
    ImportRequest, ImportJobInfo, ImportJobStatus, ImportProgress,
    ImportFilters, ImportSummary, ImportFileInfo, DBNMetadata,
    ImportFileType, CSVFormat
)
from .metadata_extractor import metadata_extractor
from .csv_metadata_extractor import csv_metadata_extractor
from .dbn_reader import DBNReader
from .csv_reader import CSVReader
from .parquet_writer import ParquetWriter

logger = logging.getLogger(__name__)


class ImportManager:
    """
    Manages data import operations with async job processing.
    """
    
    def __init__(self, max_concurrent_jobs: int = 2):
        """
        Initialize import manager.
        
        Args:
            max_concurrent_jobs: Maximum number of concurrent import jobs
        """
        self.max_concurrent_jobs = max_concurrent_jobs
        
        # Set up directories
        self.dbn_files_dir = path_manager.data_dir / "dbn_files"
        self.jobs_dir = path_manager.data_dir / "import_jobs"
        self.jobs_dir.mkdir(exist_ok=True)
        
        # Initialize components
        self.dbn_reader = DBNReader()
        self.csv_reader = CSVReader()
        self.parquet_writer = ParquetWriter()
        
        # Job management
        self._active_jobs: Dict[str, ImportJobInfo] = {}
        self._job_lock = threading.Lock()
        self._executor = ThreadPoolExecutor(max_workers=max_concurrent_jobs)
        
        # Load existing jobs
        self._load_existing_jobs()
    
    async def list_available_files(self) -> List[ImportFileInfo]:
        """
        List available import files (DBN and CSV) with metadata information.
        
        Returns:
            List of ImportFileInfo objects
        """
        logger.info(f"Listing import files in {self.dbn_files_dir}")
        
        # Ensure directory exists
        self.dbn_files_dir.mkdir(exist_ok=True)
        
        files_info = []
        
        # Get DBN files with metadata - handle errors gracefully
        try:
            dbn_files = metadata_extractor.list_files_with_metadata(self.dbn_files_dir)
            for dbn_file in dbn_files:
                try:
                    # Convert DBNFileInfo to ImportFileInfo
                    import_file = ImportFileInfo(
                        filename=dbn_file.filename,
                        file_path=dbn_file.file_path,
                        file_size=dbn_file.file_size,
                        modified_at=dbn_file.modified_at,
                        file_type=ImportFileType.DBN,
                        metadata=dbn_file.metadata,
                        metadata_cached=dbn_file.metadata_cached,
                        metadata_cache_time=dbn_file.metadata_cache_time
                    )
                    files_info.append(import_file)
                except Exception as e:
                    logger.warning(f"Error processing DBN file {dbn_file.filename}: {e}")
                    continue
        except Exception as e:
            logger.warning(f"Error loading DBN files with metadata, falling back to basic listing: {e}")
            
            # Fallback: List DBN files without metadata
            try:
                for dbn_file in self.dbn_files_dir.glob("*.dbn"):
                    try:
                        file_stat = dbn_file.stat()
                        import_file = ImportFileInfo(
                            filename=dbn_file.name,
                            file_path=str(dbn_file),
                            file_size=file_stat.st_size,
                            modified_at=datetime.fromtimestamp(file_stat.st_mtime),
                            file_type=ImportFileType.DBN,
                            metadata=None,
                            metadata_cached=False,
                            metadata_cache_time=None
                        )
                        files_info.append(import_file)
                    except Exception as e:
                        logger.warning(f"Error processing DBN file {dbn_file}: {e}")
                        continue
            except Exception as e:
                logger.error(f"Error scanning DBN files: {e}")
        
        # Get CSV files with basic analysis
        try:
            for csv_file in self.dbn_files_dir.glob("*.csv"):
                try:
                    file_stat = csv_file.stat()
                    
                    # Analyze CSV structure
                    csv_analysis = csv_metadata_extractor.analyze_csv_structure(csv_file)
                    
                    import_file = ImportFileInfo(
                        filename=csv_file.name,
                        file_path=str(csv_file),
                        file_size=file_stat.st_size,
                        modified_at=datetime.fromtimestamp(file_stat.st_mtime),
                        file_type=ImportFileType.CSV,
                        needs_symbol_input=True,
                        csv_structure_analysis=csv_analysis,
                        csv_format=CSVFormat(csv_analysis.get('csv_format', 'generic')),
                        csv_headers=csv_analysis.get('headers', []),
                        csv_sample_data=csv_analysis.get('sample_data', [])
                    )
                    files_info.append(import_file)
                    
                except Exception as e:
                    logger.warning(f"Error analyzing CSV file {csv_file}: {e}")
                    continue
        except Exception as e:
            logger.error(f"Error scanning CSV files: {e}")
        
        # Sort by modification time (newest first)
        files_info.sort(key=lambda x: x.modified_at, reverse=True)
        
        logger.info(f"Found {len(files_info)} import files ({sum(1 for f in files_info if f.file_type == ImportFileType.DBN)} DBN, {sum(1 for f in files_info if f.file_type == ImportFileType.CSV)} CSV)")
        return files_info
    
    async def list_dbn_files_only(self) -> List[Dict[str, Any]]:
        """
        List only .dbn files without any metadata processing for fast loading.
        
        Returns:
            List of basic file information dictionaries
        """
        logger.info(f"Listing .dbn files only in {self.dbn_files_dir}")
        
        # Ensure directory exists
        self.dbn_files_dir.mkdir(exist_ok=True)
        
        files_info = []
        
        # Scan for .dbn files only
        for dbn_file in self.dbn_files_dir.glob("*.dbn"):
            try:
                stat = dbn_file.stat()
                file_info = {
                    'filename': dbn_file.name,
                    'file_path': str(dbn_file),
                    'file_size': stat.st_size,
                    'modified_at': datetime.fromtimestamp(stat.st_mtime),
                    'size_mb': round(stat.st_size / (1024 * 1024), 2),
                    'size_gb': round(stat.st_size / (1024 * 1024 * 1024), 2)
                }
                files_info.append(file_info)
            except Exception as e:
                logger.warning(f"Error getting info for {dbn_file}: {e}")
                continue
        
        # Sort by modification time (newest first)
        files_info.sort(key=lambda x: x['modified_at'], reverse=True)
        
        logger.info(f"Found {len(files_info)} .dbn files")
        return files_info
    
    async def get_file_metadata(self, filename: str, symbol: str = None, 
                               force_refresh: bool = False) -> DBNMetadata:
        """
        Get detailed metadata for a specific file.
        
        Args:
            filename: Name of the file
            symbol: Symbol name (required for CSV files)
            force_refresh: Force refresh of cached metadata
            
        Returns:
            DBNMetadata object
        """
        file_path = self.dbn_files_dir / filename
        
        if not file_path.exists():
            raise FileNotFoundError(f"File not found: {filename}")
        
        # Determine file type
        file_type = self._detect_file_type(filename)
        
        if file_type == ImportFileType.DBN:
            return metadata_extractor.get_file_metadata(file_path, force_refresh)
        elif file_type == ImportFileType.CSV:
            if not symbol:
                raise ValueError("Symbol is required for CSV file metadata extraction")
            return csv_metadata_extractor.get_file_metadata(file_path, symbol, force_refresh)
        else:
            raise ValueError(f"Unsupported file type: {filename}")
    
    async def start_import_job(self, request: ImportRequest) -> str:
        """
        Start a new import job.
        
        Args:
            request: Import request details
            
        Returns:
            Job ID for tracking the import
        """
        # Validate request
        file_path = self.dbn_files_dir / request.filename
        if not file_path.exists():
            raise FileNotFoundError(f"File not found: {request.filename}")
        
        # Detect file type if not provided
        if not request.file_type:
            request.file_type = self._detect_file_type(request.filename)
        
        # Validate CSV-specific requirements
        if request.file_type == ImportFileType.CSV and not request.csv_symbol:
            raise ValueError("csv_symbol is required for CSV file imports")
        
        # Generate job ID
        job_id = str(uuid.uuid4())
        
        # Create job info
        job_info = ImportJobInfo(
            job_id=job_id,
            job_name=request.job_name or f"Import {request.filename}",
            filename=request.filename,
            status=ImportJobStatus.PENDING,
            created_at=datetime.now(timezone.utc),
            filters=request.filters,
            output_format=request.output_format,
            overwrite_existing=request.overwrite_existing
        )
        
        # Store CSV-specific info in job
        if request.file_type == ImportFileType.CSV:
            job_info.csv_symbol = request.csv_symbol
            job_info.csv_format = request.csv_format
            job_info.file_type = request.file_type
            job_info.timestamp_convention = request.timestamp_convention
        
        # Store job info
        with self._job_lock:
            self._active_jobs[job_id] = job_info
        
        # Save job to disk
        self._save_job(job_info)
        
        # Submit job to executor
        future = self._executor.submit(self._execute_import_job, job_id)
        
        logger.info(f"Started import job {job_id} for file {request.filename} (type: {request.file_type})")
        return job_id
    
    async def get_job_status(self, job_id: str) -> ImportJobInfo:
        """
        Get status of an import job.
        
        Args:
            job_id: Job ID to check
            
        Returns:
            ImportJobInfo with current status
        """
        with self._job_lock:
            if job_id in self._active_jobs:
                return self._active_jobs[job_id]
        
        # Try to load from disk
        job_file = self.jobs_dir / f"{job_id}.json"
        if job_file.exists():
            try:
                with open(job_file, 'r') as f:
                    job_data = json.load(f)
                return ImportJobInfo(**job_data)
            except Exception as e:
                logger.error(f"Error loading job {job_id}: {e}")
        
        raise ValueError(f"Job not found: {job_id}")
    
    async def list_import_jobs(self, 
                              status_filter: Optional[ImportJobStatus] = None,
                              limit: int = 50) -> List[ImportJobInfo]:
        """
        List import jobs with optional filtering.
        
        Args:
            status_filter: Optional status filter
            limit: Maximum number of jobs to return
            
        Returns:
            List of ImportJobInfo objects
        """
        jobs = []
        
        # Get active jobs
        with self._job_lock:
            jobs.extend(self._active_jobs.values())
        
        # Load jobs from disk
        for job_file in self.jobs_dir.glob("*.json"):
            if len(jobs) >= limit:
                break
                
            try:
                with open(job_file, 'r') as f:
                    job_data = json.load(f)
                
                job_info = ImportJobInfo(**job_data)
                
                # Skip if already in active jobs
                if job_info.job_id in self._active_jobs:
                    continue
                
                # Apply status filter
                if status_filter and job_info.status != status_filter:
                    continue
                
                jobs.append(job_info)
                
            except Exception as e:
                logger.warning(f"Error loading job file {job_file}: {e}")
        
        # Sort by created_at descending
        jobs.sort(key=lambda x: x.created_at, reverse=True)
        
        return jobs[:limit]
    
    async def cancel_import_job(self, job_id: str) -> bool:
        """
        Cancel a running import job.
        
        Args:
            job_id: Job ID to cancel
            
        Returns:
            True if job was cancelled, False otherwise
        """
        with self._job_lock:
            if job_id in self._active_jobs:
                job_info = self._active_jobs[job_id]
                
                if job_info.is_active:
                    job_info.status = ImportJobStatus.CANCELLED
                    job_info.completed_at = datetime.now(timezone.utc)
                    self._save_job(job_info)
                    
                    logger.info(f"Cancelled import job {job_id}")
                    return True
        
        return False
    
    async def get_import_summary(self) -> ImportSummary:
        """
        Get summary of import operations.
        
        Returns:
            ImportSummary with statistics
        """
        jobs = await self.list_import_jobs(limit=1000)  # Get more jobs for accurate stats
        
        summary = ImportSummary()
        summary.total_jobs = len(jobs)
        
        for job in jobs:
            if job.status == ImportJobStatus.COMPLETED:
                summary.completed_jobs += 1
                summary.total_files_imported += 1
                summary.total_data_size += job.total_output_size
            elif job.status == ImportJobStatus.FAILED:
                summary.failed_jobs += 1
            elif job.is_active:
                summary.active_jobs += 1
        
        return summary
    
    def _execute_import_job(self, job_id: str):
        """
        Execute an import job (runs in thread pool).
        This implementation uses streaming batch processing for memory efficiency
        with append mode to prevent data loss.
        """
        job_info = self._active_jobs.get(job_id)
        if not job_info:
            logger.error(f"Job {job_id} not found for execution.")
            return

        try:
            job_info.status = ImportJobStatus.RUNNING
            job_info.started_at = datetime.now(timezone.utc)
            self._save_job(job_info)
            logger.info(f"Executing job {job_id} for {job_info.filename}")

            file_path = self.dbn_files_dir / job_info.filename
            
            # Detect file type
            file_type = getattr(job_info, 'file_type', None) or self._detect_file_type(job_info.filename)

            from collections import defaultdict
            batches = defaultdict(list)
            processed_records = 0
            total_records_written = 0
            output_paths = []
            batch_size_threshold = 500000  # Process in chunks for memory efficiency

            def _write_and_clear_batches():
                nonlocal total_records_written
                logger.info(f"Job {job_id}: Writing {len(batches)} batches to Parquet...")
                batch_output_paths = []
                for (underlying_symbol, record_date, asset_type), records in list(batches.items()):
                    # Create metadata based on file type
                    if file_type == ImportFileType.CSV:
                        csv_format = getattr(job_info, 'csv_format', None)
                        if csv_format:
                            csv_format_str = csv_format.upper() if hasattr(csv_format, 'upper') else str(csv_format).upper()
                        else:
                            csv_format_str = 'GENERIC'
                        
                        metadata = {
                            'dataset': f'CSV_{csv_format_str}',
                            'filename': job_info.filename,
                            'symbol': getattr(job_info, 'csv_symbol', 'UNKNOWN')
                        }
                    else:
                        metadata = {
                            'dataset': 'OPRA',
                            'filename': job_info.filename
                        }
                    
                    output_path = self.parquet_writer._write_symbol_date_partition_to_parquet(
                        underlying_symbol, record_date, asset_type, records, metadata
                    )
                    if output_path:
                        batch_output_paths.append(output_path)
                        total_records_written += len(records)
                    del batches[(underlying_symbol, record_date, asset_type)]

                output_paths.extend(batch_output_paths)
                logger.info(f"Job {job_id}: Wrote {len(batch_output_paths)} files, {total_records_written:,} total records")

            # Stream records based on file type
            if file_type == ImportFileType.CSV:
                csv_symbol = getattr(job_info, 'csv_symbol', None)
                if not csv_symbol:
                    raise ValueError("CSV symbol not found in job info")
                
                # Get timestamp convention from job info
                timestamp_convention = getattr(job_info, 'timestamp_convention', None)
                if timestamp_convention:
                    timestamp_convention = timestamp_convention.value if hasattr(timestamp_convention, 'value') else timestamp_convention
                
                records_iterator = self.csv_reader.stream_records(
                    file_path, 
                    csv_symbol, 
                    timestamp_convention=timestamp_convention
                )
            else:
                records_iterator = self.dbn_reader.stream_records(file_path)

            # Process records in batches
            for record in records_iterator:
                processed_records += 1
                record_info = self.parquet_writer._extract_record_info_with_context(record)
                if record_info:
                    key = (
                        record_info['underlying_symbol'],
                        record_info['date'],
                        record_info['asset_type']
                    )
                    batches[key].append(record_info)

                # Process batch when it reaches threshold
                if processed_records % batch_size_threshold == 0:
                    logger.info(f"Job {job_id}: Processed {processed_records:,} records, memory threshold reached. Writing batches...")
                    _write_and_clear_batches()
                    job_info.progress.processed_records = processed_records
                    self._save_job(job_info)

            # Write any remaining records in final batches
            if batches:
                logger.info(f"Job {job_id}: Writing final batches...")
                _write_and_clear_batches()

            job_info.output_paths = list(set(output_paths))  # Remove duplicates
            job_info.status = ImportJobStatus.COMPLETED
            job_info.progress.processed_records = processed_records
            job_info.progress.total_records = processed_records
            job_info.total_output_size = total_records_written

            logger.info(f"Job {job_id} completed. Processed {processed_records:,} records, wrote {total_records_written:,} records to {len(job_info.output_paths)} unique files.")

        except Exception as e:
            logger.exception(f"Job {job_id} failed: {e}")
            job_info.status = ImportJobStatus.FAILED
            job_info.error_message = str(e)
        finally:
            job_info.completed_at = datetime.now(timezone.utc)
            self._save_job(job_info)
            with self._job_lock:
                self._active_jobs.pop(job_id, None)
    
    def _check_existing_data(self, job_info: ImportJobInfo, metadata: DBNMetadata):
        """
        Check for existing data that would be overwritten.
        
        Args:
            job_info: Job information
            metadata: File metadata
        """
        # This is a placeholder for checking existing Parquet data
        # In a full implementation, you'd check for overlapping partitions
        # and either skip them or warn the user
        pass
    
    def _save_job(self, job_info: ImportJobInfo):
        """
        Save job information to disk.
        
        Args:
            job_info: Job information to save
        """
        try:
            job_file = self.jobs_dir / f"{job_info.job_id}.json"
            with open(job_file, 'w') as f:
                json.dump(job_info.model_dump(), f, indent=2, default=str)
        except Exception as e:
            logger.error(f"Error saving job {job_info.job_id}: {e}")
    
    def _load_existing_jobs(self):
        """
        Load existing jobs from disk.
        """
        try:
            for job_file in self.jobs_dir.glob("*.json"):
                try:
                    with open(job_file, 'r') as f:
                        job_data = json.load(f)
                    
                    job_info = ImportJobInfo(**job_data)
                    
                    # Only load active jobs into memory
                    if job_info.is_active:
                        with self._job_lock:
                            self._active_jobs[job_info.job_id] = job_info
                        
                        # Mark stale active jobs as failed (they were interrupted)
                        if job_info.started_at:
                            time_since_start = datetime.now(timezone.utc) - job_info.started_at
                            if time_since_start.total_seconds() > 3600:  # 1 hour timeout
                                job_info.status = ImportJobStatus.FAILED
                                job_info.error_message = "Job interrupted (system restart)"
                                job_info.completed_at = datetime.now(timezone.utc)
                                self._save_job(job_info)
                                
                                with self._job_lock:
                                    self._active_jobs.pop(job_info.job_id, None)
                
                except Exception as e:
                    logger.warning(f"Error loading job file {job_file}: {e}")
            
            logger.info(f"Loaded {len(self._active_jobs)} active import jobs")
            
        except Exception as e:
            logger.error(f"Error loading existing jobs: {e}")
    
    def cleanup_old_jobs(self, days_old: int = 30):
        """
        Clean up old job files.
        
        Args:
            days_old: Remove job files older than this many days
        """
        try:
            cutoff_time = datetime.now(timezone.utc).timestamp() - (days_old * 24 * 3600)
            removed_count = 0
            
            for job_file in self.jobs_dir.glob("*.json"):
                try:
                    if job_file.stat().st_mtime < cutoff_time:
                        job_file.unlink()
                        removed_count += 1
                except Exception as e:
                    logger.warning(f"Error removing old job file {job_file}: {e}")
            
            if removed_count > 0:
                logger.info(f"Cleaned up {removed_count} old job files")
                
        except Exception as e:
            logger.error(f"Error cleaning up old jobs: {e}")
    
    def get_storage_stats(self) -> Dict[str, Any]:
        """
        Get comprehensive storage statistics.
        
        Returns:
            Dictionary with storage statistics
        """
        # Get Parquet storage stats
        parquet_stats = self.parquet_writer.get_storage_stats()
        
        # Get import files stats (DBN and CSV)
        import_files_stats = {
            'dbn_files': {'total_files': 0, 'total_size_bytes': 0},
            'csv_files': {'total_files': 0, 'total_size_bytes': 0},
            'total_files': 0,
            'total_size_bytes': 0,
            'total_size_mb': 0,
            'total_size_gb': 0
        }
        
        if self.dbn_files_dir.exists():
            # Count DBN files
            for dbn_file in self.dbn_files_dir.glob("*.dbn"):
                try:
                    size = dbn_file.stat().st_size
                    import_files_stats['dbn_files']['total_files'] += 1
                    import_files_stats['dbn_files']['total_size_bytes'] += size
                    import_files_stats['total_size_bytes'] += size
                except Exception:
                    pass
            
            # Count CSV files
            for csv_file in self.dbn_files_dir.glob("*.csv"):
                try:
                    size = csv_file.stat().st_size
                    import_files_stats['csv_files']['total_files'] += 1
                    import_files_stats['csv_files']['total_size_bytes'] += size
                    import_files_stats['total_size_bytes'] += size
                except Exception:
                    pass
            
            import_files_stats['total_files'] = (
                import_files_stats['dbn_files']['total_files'] + 
                import_files_stats['csv_files']['total_files']
            )
            import_files_stats['total_size_mb'] = round(import_files_stats['total_size_bytes'] / (1024 * 1024), 2)
            import_files_stats['total_size_gb'] = round(import_files_stats['total_size_bytes'] / (1024 * 1024 * 1024), 2)
        
        # Get metadata cache stats
        dbn_cache_stats = metadata_extractor.get_cache_stats()
        csv_cache_stats = csv_metadata_extractor.get_cache_stats()
        
        return {
            'import_files': import_files_stats,
            'parquet_data': parquet_stats,
            'metadata_cache': {
                'dbn_cache': dbn_cache_stats,
                'csv_cache': csv_cache_stats
            },
            'directories': {
                'import_files_dir': str(self.dbn_files_dir),
                'parquet_dir': str(self.parquet_writer.parquet_dir),
                'jobs_dir': str(self.jobs_dir)
            }
        }
    
    def get_symbol_level_data(self) -> List[Dict[str, Any]]:
        """
        Get symbol-level data by scanning the Parquet directory structure.
        
        Returns:
            List of dictionaries with symbol-level information
        """
        symbol_data = []
        
        try:
            # Scan each asset type directory
            asset_type_dirs = {
                'options': self.parquet_writer.options_dir,
                'equities': self.parquet_writer.equities_dir,
                'futures': self.parquet_writer.futures_dir,
                'forex': self.parquet_writer.forex_dir
            }
            
            for asset_type, base_dir in asset_type_dirs.items():
                if not base_dir.exists():
                    continue
                
                # Look for underlying=SYMBOL directories
                for underlying_dir in base_dir.glob("underlying=*"):
                    try:
                        # Extract symbol name from directory
                        symbol = underlying_dir.name.split('=')[1]
                        
                        # Scan date structure to find date ranges and count records
                        date_info = self._analyze_symbol_date_structure(underlying_dir)
                        
                        if date_info['partition_count'] > 0:
                            symbol_entry = {
                                'id': f"symbol_{symbol}_{asset_type}",
                                'symbol': symbol,
                                'asset_type': asset_type.upper(),
                                'start_date': date_info['start_date'],
                                'end_date': date_info['end_date'],
                                'record_count': date_info['record_count'],
                                'file_size': date_info['file_size'],
                                'partition_count': date_info['partition_count'],
                                'imported_at': datetime.now().isoformat()
                            }
                            symbol_data.append(symbol_entry)
                            
                    except Exception as e:
                        logger.warning(f"Error processing symbol directory {underlying_dir}: {e}")
                        continue
            
            # Sort by symbol name
            symbol_data.sort(key=lambda x: x['symbol'])
            
            logger.info(f"Found {len(symbol_data)} symbols with imported data")
            return symbol_data
            
        except Exception as e:
            logger.error(f"Error getting symbol-level data: {e}")
            return []
    
    def _detect_file_type(self, filename: str) -> ImportFileType:
        """
        Detect file type from filename extension.
        
        Args:
            filename: Name of the file
            
        Returns:
            ImportFileType enum value
        """
        filename_lower = filename.lower()
        if filename_lower.endswith('.dbn'):
            return ImportFileType.DBN
        elif filename_lower.endswith('.csv'):
            return ImportFileType.CSV
        else:
            raise ValueError(f"Unsupported file type: {filename}")
    
    def _analyze_symbol_date_structure(self, symbol_dir: Path) -> Dict[str, Any]:
        """
        Analyze the date structure for a symbol to extract date ranges and statistics.
        
        Args:
            symbol_dir: Path to the underlying=SYMBOL directory
            
        Returns:
            Dictionary with date range and statistics information
        """
        date_info = {
            'start_date': None,
            'end_date': None,
            'record_count': 0,
            'file_size': 0,
            'partition_count': 0
        }
        
        try:
            dates = []
            
            # Scan year directories
            for year_dir in symbol_dir.glob("year=*"):
                try:
                    year = int(year_dir.name.split('=')[1])
                    
                    # Scan month directories
                    for month_dir in year_dir.glob("month=*"):
                        try:
                            month = int(month_dir.name.split('=')[1])
                            
                            # Scan day directories
                            for day_dir in month_dir.glob("day=*"):
                                try:
                                    day = int(day_dir.name.split('=')[1])
                                    
                                    # Check if data.parquet exists
                                    parquet_file = day_dir / "data.parquet"
                                    if parquet_file.exists():
                                        dates.append(date(year, month, day))
                                        date_info['partition_count'] += 1
                                        
                                        # Get file size
                                        try:
                                            file_size = parquet_file.stat().st_size
                                            date_info['file_size'] += file_size
                                            
                                            # Get record count from Parquet metadata
                                            import pyarrow.parquet as pq
                                            parquet_file_obj = pq.ParquetFile(parquet_file)
                                            date_info['record_count'] += parquet_file_obj.metadata.num_rows
                                            
                                        except Exception as e:
                                            logger.debug(f"Error reading Parquet metadata for {parquet_file}: {e}")
                                            
                                except (ValueError, Exception) as e:
                                    logger.debug(f"Error processing day directory {day_dir}: {e}")
                                    continue
                                    
                        except (ValueError, Exception) as e:
                            logger.debug(f"Error processing month directory {month_dir}: {e}")
                            continue
                            
                except (ValueError, Exception) as e:
                    logger.debug(f"Error processing year directory {year_dir}: {e}")
                    continue
            
            # Determine date range
            if dates:
                dates.sort()
                date_info['start_date'] = dates[0].isoformat()
                date_info['end_date'] = dates[-1].isoformat()
            
            return date_info
            
        except Exception as e:
            logger.error(f"Error analyzing symbol date structure for {symbol_dir}: {e}")
            return date_info


    def delete_imported_data(self, symbol: str) -> bool:
        """
        Delete all imported data for a specific symbol.
        
        Args:
            symbol: Symbol to delete data for
            
        Returns:
            True if data was deleted, False if no data found
        """
        try:
            import shutil
            deleted_any = False
            
            # Check each asset type directory
            asset_type_dirs = {
                'options': self.parquet_writer.options_dir,
                'equities': self.parquet_writer.equities_dir,
                'futures': self.parquet_writer.futures_dir,
                'forex': self.parquet_writer.forex_dir
            }
            
            for asset_type, base_dir in asset_type_dirs.items():
                if not base_dir.exists():
                    continue
                
                # Look for underlying=SYMBOL directory
                symbol_dir = base_dir / f"underlying={symbol}"
                if symbol_dir.exists():
                    logger.info(f"Deleting {asset_type} data for symbol {symbol} at {symbol_dir}")
                    shutil.rmtree(symbol_dir)
                    deleted_any = True
            
            if deleted_any:
                logger.info(f"Successfully deleted all imported data for symbol '{symbol}'")
            else:
                logger.warning(f"No imported data found for symbol '{symbol}'")
            
            return deleted_any
            
        except Exception as e:
            logger.error(f"Error deleting imported data for symbol '{symbol}': {e}")
            return False


# Global instance
import_manager = ImportManager()
