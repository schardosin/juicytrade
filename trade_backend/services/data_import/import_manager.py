"""
Import Manager

Orchestrates the data import process, managing async jobs, progress tracking,
and coordination between DBN reading, metadata extraction, and Parquet writing.
"""

import json
import logging
import uuid
from datetime import datetime, date, timezone, timedelta
from pathlib import Path
from typing import Dict, List, Optional, Any, Callable
import threading
from concurrent.futures import ThreadPoolExecutor
import calendar

from ...path_manager import path_manager
from .import_models import (
    ImportRequest, ImportJobInfo, ImportJobStatus,
    ImportSummary, ImportFileInfo, DBNMetadata,
    ImportFileType, CSVFormat
)
from .metadata_extractor import metadata_extractor
from .csv_metadata_extractor import csv_metadata_extractor
from .dbn_reader import DBNReader
from .csv_reader import CSVReader
from .parquet_writer import ParquetWriter
from .import_queue import ImportQueue

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
        # Note: DBNReader instances are created per-job to prevent cache contamination in concurrent processing
        self.csv_reader = CSVReader()
        self.parquet_writer = ParquetWriter()
        
        # Job management
        self._active_jobs: Dict[str, ImportJobInfo] = {}
        self._job_lock = threading.Lock()
        self._executor = ThreadPoolExecutor(max_workers=max_concurrent_jobs)
        
        # Load existing jobs
        self._load_existing_jobs()
        
        # Initialize import queue
        self.import_queue = ImportQueue(self)
    
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
        with append mode to prevent data loss and day-based progress tracking.
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

            # Get file metadata for date range (for progress calculation)
            import_start_date, import_end_date = self._get_import_date_range(job_info, file_path, file_type)
            
            from collections import defaultdict
            batches = defaultdict(list)
            processed_records = 0
            total_records_written = 0
            output_paths = []
            batch_size_threshold = 500000  # Process in chunks for memory efficiency
            
            # Progress tracking variables
            current_processing_date = None
            last_progress_update = 0
            progress_update_interval = 100000  # Update progress every 100k records

            def _write_and_clear_batches():
                nonlocal total_records_written
                logger.info(f"Job {job_id}: Writing {len(batches)} partition batches to Parquet...")
                batch_output_paths = []
                
                # Process each partition key separately to ensure proper consolidation
                for (underlying_symbol, record_date, asset_type), records in list(batches.items()):
                    try:
                        # Create metadata
                        metadata = self._create_write_metadata(job_info, file_type)
                        
                        logger.debug(f"Job {job_id}: Writing partition {underlying_symbol}/{record_date} with {len(records)} records")
                        
                        output_path = self.parquet_writer._write_symbol_date_partition_to_parquet(
                            underlying_symbol, record_date, asset_type, records, metadata
                        )
                        
                        if output_path:
                            batch_output_paths.append(output_path)
                            total_records_written += len(records)
                            logger.debug(f"Job {job_id}: Successfully wrote {len(records)} records to {output_path}")
                        else:
                            logger.error(f"Job {job_id}: Failed to write partition {underlying_symbol}/{record_date}")
                        
                        # Clear this batch from memory
                        del batches[(underlying_symbol, record_date, asset_type)]
                        
                    except Exception as e:
                        logger.error(f"Job {job_id}: Error writing partition {underlying_symbol}/{record_date}: {e}")
                        # Still remove the batch to prevent memory buildup
                        if (underlying_symbol, record_date, asset_type) in batches:
                            del batches[(underlying_symbol, record_date, asset_type)]

                output_paths.extend(batch_output_paths)
                logger.info(f"Job {job_id}: ✅ Wrote {len(batch_output_paths)} partition files, {total_records_written:,} total records written")

            # SCALABLE FIX: Stream with batches but consolidate same-partition writes
            # This maintains memory efficiency while preventing overwrites
            
            logger.info(f"Job {job_id}: Starting SCALABLE streaming import with batch consolidation")
            
            # Initialize record streaming
            records_iterator = self._get_records_iterator(job_info, file_path, file_type)

            # Process records in streaming batches with smart consolidation
            logger.info(f"Job {job_id}: Processing records in {batch_size_threshold:,} record batches")
            
            for record in records_iterator:
                processed_records += 1
                
                # Process record
                record_info = self._process_record(record, job_info, file_type)
                if record_info:
                    key = (
                        record_info['underlying_symbol'],
                        record_info['date'],
                        record_info['asset_type']
                    )
                    batches[key].append(record_info)
                    
                    # Update progress tracking based on current date
                    record_date = self._extract_date_from_record_info(record_info)
                    if record_date and import_start_date and import_end_date:
                        if current_processing_date != record_date:
                            current_processing_date = record_date
                            
                            # Calculate day-based progress
                            progress_info = self._calculate_trading_day_progress(
                                current_processing_date, import_start_date, import_end_date
                            )
                            
                            # Update job progress with day-based information
                            if not hasattr(job_info.progress, 'progress_percentage'):
                                job_info.progress.progress_percentage = progress_info['progress_percentage']
                            else:
                                job_info.progress.progress_percentage = progress_info['progress_percentage']
                            
                            job_info.progress.current_symbol = progress_info['status_message']
                            
                            # Track primary symbols being processed for frontend overlay matching
                            if job_info.progress.primary_symbols is None:
                                job_info.progress.primary_symbols = []
                            
                            # Add underlying symbols from current batches to primary symbols
                            current_symbols = set(job_info.progress.primary_symbols)
                            for (underlying_symbol, _, _) in batches.keys():
                                current_symbols.add(underlying_symbol)
                            job_info.progress.primary_symbols = list(current_symbols)
                            
                            logger.info(f"Job {job_id}: {progress_info['status_message']} - {progress_info['progress_percentage']}% complete")

                # Update progress periodically (every 100k records)
                if processed_records - last_progress_update >= progress_update_interval:
                    last_progress_update = processed_records
                    job_info.progress.processed_records = processed_records
                    self._save_job(job_info)

                # Process batch when it reaches threshold - but consolidate same partitions
                if processed_records % batch_size_threshold == 0:
                    logger.info(f"Job {job_id}: Processed {processed_records:,} records, writing consolidated batches...")
                    _write_and_clear_batches()
                    job_info.progress.processed_records = processed_records
                    self._save_job(job_info)

            # Write any remaining records in final batches
            if batches:
                logger.info(f"Job {job_id}: Writing final consolidated batches...")
                _write_and_clear_batches()

            job_info.output_paths = list(set(output_paths))  # Remove duplicates
            job_info.status = ImportJobStatus.COMPLETED
            job_info.progress.processed_records = processed_records
            job_info.progress.total_records = processed_records
            job_info.total_output_size = total_records_written
            
            # Ensure progress shows 100% when completed
            job_info.progress.progress_percentage = 100.0
            job_info.progress.current_symbol = "Import completed successfully!"

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
            
            # Convert job info to dict and handle set serialization
            job_data = job_info.model_dump()
            
            # Convert primary_symbols set to list for JSON serialization
            if hasattr(job_info.progress, 'primary_symbols') and job_info.progress.primary_symbols:
                if 'progress' not in job_data:
                    job_data['progress'] = {}
                job_data['progress']['primary_symbols'] = list(job_info.progress.primary_symbols)
            
            with open(job_file, 'w') as f:
                json.dump(job_data, f, indent=2, default=str)
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
    
    def get_symbol_level_data_basic(self) -> List[Dict[str, Any]]:
        """
        Get basic symbol-level data without expensive operations (fast loading).
        Only includes symbol names, asset types, and basic directory info.
        
        Returns:
            List of dictionaries with basic symbol information
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
                        
                        # Quick check if directory has any data (just check if any year directories exist)
                        has_data = any(underlying_dir.glob("year=*"))
                        
                        if has_data:
                            # Get basic date range without expensive operations
                            basic_date_info = self._get_basic_date_range(underlying_dir)
                            
                            symbol_entry = {
                                'id': f"symbol_{symbol}_{asset_type}",
                                'symbol': symbol,
                                'asset_type': asset_type.upper(),
                                'start_date': basic_date_info.get('start_date'),
                                'end_date': basic_date_info.get('end_date'),
                                # Placeholder values for detailed info (will be loaded later)
                                'record_count': None,
                                'file_size': None,
                                'partition_count': None,
                                'imported_at': datetime.now().isoformat(),
                                'loading_details': True  # Flag to indicate details are being loaded
                            }
                            symbol_data.append(symbol_entry)
                            
                    except Exception as e:
                        logger.warning(f"Error processing symbol directory {underlying_dir}: {e}")
                        continue
            
            # Sort by symbol name
            symbol_data.sort(key=lambda x: x['symbol'])
            
            logger.info(f"Found {len(symbol_data)} symbols with imported data (basic info)")
            return symbol_data
            
        except Exception as e:
            logger.error(f"Error getting basic symbol-level data: {e}")
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
    
    def _get_basic_date_range(self, symbol_dir: Path) -> Dict[str, Any]:
        """
        Get basic date range information without expensive operations (fast).
        Only looks at directory structure, doesn't read Parquet metadata.
        
        Args:
            symbol_dir: Path to the underlying=SYMBOL directory
            
        Returns:
            Dictionary with basic date range information
        """
        date_info = {
            'start_date': None,
            'end_date': None
        }
        
        try:
            dates = []
            
            # Scan year directories (fast directory listing)
            for year_dir in symbol_dir.glob("year=*"):
                try:
                    year = int(year_dir.name.split('=')[1])
                    
                    # Scan month directories
                    for month_dir in year_dir.glob("month=*"):
                        try:
                            month = int(month_dir.name.split('=')[1])
                            
                            # Scan day directories (just check existence, don't read files)
                            for day_dir in month_dir.glob("day=*"):
                                try:
                                    day = int(day_dir.name.split('=')[1])
                                    
                                    # Quick check if data.parquet exists (no file reading)
                                    parquet_file = day_dir / "data.parquet"
                                    if parquet_file.exists():
                                        dates.append(date(year, month, day))
                                        
                                except (ValueError, Exception) as e:
                                    logger.debug(f"Error processing day directory {day_dir}: {e}")
                                    continue
                                    
                        except (ValueError, Exception) as e:
                            logger.debug(f"Error processing month directory {month_dir}: {e}")
                            continue
                            
                except (ValueError, Exception) as e:
                    logger.debug(f"Error processing year directory {year_dir}: {e}")
                    continue
            
            # Determine date range (fast)
            if dates:
                dates.sort()
                date_info['start_date'] = dates[0].isoformat()
                date_info['end_date'] = dates[-1].isoformat()
            
            return date_info
            
        except Exception as e:
            logger.debug(f"Error getting basic date range for {symbol_dir}: {e}")
            return date_info


    def delete_imported_data(self, symbol: str, asset_type: str = None) -> bool:
        """
        Delete imported data for a specific symbol and optionally specific asset type.
        
        Args:
            symbol: Symbol to delete data for
            asset_type: Optional asset type to delete (if None, deletes all asset types)
            
        Returns:
            True if data was deleted, False if no data found
        """
        try:
            import shutil
            deleted_any = False
            
            # Asset type directory mapping
            asset_type_dirs = {
                'options': self.parquet_writer.options_dir,
                'equities': self.parquet_writer.equities_dir,
                'futures': self.parquet_writer.futures_dir,
                'forex': self.parquet_writer.forex_dir
            }
            
            # If specific asset type is provided, only delete from that type
            if asset_type:
                asset_type_lower = asset_type.lower()
                if asset_type_lower in asset_type_dirs:
                    base_dir = asset_type_dirs[asset_type_lower]
                    if base_dir.exists():
                        symbol_dir = base_dir / f"underlying={symbol}"
                        if symbol_dir.exists():
                            logger.info(f"Deleting {asset_type_lower} data for symbol {symbol} at {symbol_dir}")
                            shutil.rmtree(symbol_dir)
                            deleted_any = True
                        else:
                            logger.warning(f"No {asset_type_lower} data found for symbol '{symbol}'")
                    else:
                        logger.warning(f"Asset type directory does not exist: {base_dir}")
                else:
                    logger.error(f"Unknown asset type: {asset_type}")
                    return False
            else:
                # Delete from all asset types (legacy behavior)
                for asset_type_name, base_dir in asset_type_dirs.items():
                    if not base_dir.exists():
                        continue
                    
                    symbol_dir = base_dir / f"underlying={symbol}"
                    if symbol_dir.exists():
                        logger.info(f"Deleting {asset_type_name} data for symbol {symbol} at {symbol_dir}")
                        shutil.rmtree(symbol_dir)
                        deleted_any = True
            
            if deleted_any:
                if asset_type:
                    logger.info(f"Successfully deleted {asset_type} data for symbol '{symbol}'")
                else:
                    logger.info(f"Successfully deleted all imported data for symbol '{symbol}'")
            else:
                if asset_type:
                    logger.warning(f"No {asset_type} data found for symbol '{symbol}'")
                else:
                    logger.warning(f"No imported data found for symbol '{symbol}'")
            
            return deleted_any
            
        except Exception as e:
            logger.error(f"Error deleting imported data for symbol '{symbol}' (asset_type: {asset_type}): {e}")
            return False

    def _calculate_trading_day_progress(self, current_date: date, start_date: date, end_date: date) -> Dict[str, Any]:
        """
        Calculate trading day-based progress with month boundary adjustments.
        
        Args:
            current_date: Current date being processed
            start_date: Import start date
            end_date: Import end date
            
        Returns:
            Dictionary with progress information
        """
        try:
            # Calculate total months in the import range
            total_months = self._get_months_between(start_date, end_date)
            if total_months == 0:
                total_months = 1  # At least one month
            
            # Calculate which month we're currently in (0-based)
            current_month_index = self._get_months_between(start_date, current_date)
            
            # Each month represents 20% of progress (assuming 20 trading days per month)
            progress_per_month = 100.0 / total_months
            progress_per_day = progress_per_month / 20.0  # 20 trading days per month
            
            # Base progress from completed months
            base_progress = current_month_index * progress_per_month
            
            # Calculate trading day within current month
            trading_day_in_month = self._get_trading_day_in_month(current_date)
            
            # Add progress within current month (up to one month's worth)
            day_progress_in_month = min(trading_day_in_month * progress_per_day, progress_per_month)
            
            # Total progress (capped at 100%)
            total_progress = min(base_progress + day_progress_in_month, 100.0)
            
            # Format current date for display
            current_date_str = current_date.strftime("%B %d, %Y")
            
            return {
                'progress_percentage': round(total_progress, 1),
                'current_date': current_date_str,
                'current_month': current_date.strftime("%B %Y"),
                'trading_day_in_month': trading_day_in_month,
                'total_months': total_months,
                'current_month_index': current_month_index + 1,  # 1-based for display
                'status_message': f"Processing {current_date_str}"
            }
            
        except Exception as e:
            logger.warning(f"Error calculating trading day progress: {e}")
            return {
                'progress_percentage': 0.0,
                'current_date': str(current_date),
                'current_month': current_date.strftime("%B %Y"),
                'trading_day_in_month': 1,
                'total_months': 1,
                'current_month_index': 1,
                'status_message': f"Processing {current_date}"
            }
    
    def _get_months_between(self, start_date: date, end_date: date) -> int:
        """
        Calculate the number of months between two dates.
        
        Args:
            start_date: Start date
            end_date: End date
            
        Returns:
            Number of months between the dates
        """
        try:
            if end_date < start_date:
                return 0
            
            # Calculate months difference
            months = (end_date.year - start_date.year) * 12 + (end_date.month - start_date.month)
            
            # If we haven't reached the same day in the end month, don't count the partial month
            if end_date.day < start_date.day:
                months -= 1
            
            return max(0, months)
            
        except Exception as e:
            logger.warning(f"Error calculating months between {start_date} and {end_date}: {e}")
            return 1
    
    def _get_trading_day_in_month(self, current_date: date) -> int:
        """
        Calculate which trading day of the month this is (1-20).
        Assumes 20 trading days per month for simplicity.
        
        Args:
            current_date: Current date
            
        Returns:
            Trading day number in month (1-20)
        """
        try:
            # Simple approximation: use the day of month and scale to 1-20
            day_of_month = current_date.day
            
            # Get total days in this month
            _, days_in_month = calendar.monthrange(current_date.year, current_date.month)
            
            # Scale to 1-20 range (trading days)
            trading_day = max(1, min(20, int((day_of_month / days_in_month) * 20)))
            
            return trading_day
            
        except Exception as e:
            logger.warning(f"Error calculating trading day for {current_date}: {e}")
            return 1
    
    def _extract_date_from_record_info(self, record_info: Dict[str, Any]) -> Optional[date]:
        """
        Extract date from record info for progress tracking.
        
        Args:
            record_info: Record information dictionary
            
        Returns:
            Date object or None if extraction fails
        """
        try:
            if 'date' in record_info and record_info['date']:
                record_date = record_info['date']
                if isinstance(record_date, date):
                    return record_date
                elif isinstance(record_date, str):
                    # Try to parse string date
                    return datetime.strptime(record_date, '%Y-%m-%d').date()
            
            return None
            
        except Exception as e:
            logger.debug(f"Error extracting date from record info: {e}")
            return None

    def _get_import_date_range(self, job_info: ImportJobInfo, file_path: Path, file_type: ImportFileType) -> tuple:
        """
        Get import date range for progress tracking.
        
        Args:
            job_info: Job information
            file_path: Path to the import file
            file_type: Type of file being imported
            
        Returns:
            Tuple of (start_date, end_date) or (None, None) if extraction fails
        """
        try:
            if file_type == ImportFileType.CSV:
                csv_symbol = getattr(job_info, 'csv_symbol', None)
                metadata = csv_metadata_extractor.get_file_metadata(file_path, csv_symbol)
            else:
                metadata = metadata_extractor.get_file_metadata(file_path)
            
            # Extract date range for progress tracking
            import_start_date = None
            import_end_date = None
            
            if metadata and metadata.overall_date_range:
                import_start_date = metadata.overall_date_range.start_date
                import_end_date = metadata.overall_date_range.end_date
            elif metadata and hasattr(metadata, 'start_timestamp') and hasattr(metadata, 'end_timestamp'):
                if metadata.start_timestamp:
                    import_start_date = metadata.start_timestamp.date()
                if metadata.end_timestamp:
                    import_end_date = metadata.end_timestamp.date()
            
            return import_start_date, import_end_date
            
        except Exception as e:
            logger.warning(f"Could not extract metadata for progress tracking: {e}")
            return None, None

    def _get_records_iterator(self, job_info: ImportJobInfo, file_path: Path, file_type: ImportFileType):
        """
        Get records iterator based on file type.
        
        Args:
            job_info: Job information
            file_path: Path to the import file
            file_type: Type of file being imported
            
        Returns:
            Records iterator
        """
        if file_type == ImportFileType.CSV:
            csv_symbol = getattr(job_info, 'csv_symbol', None)
            if not csv_symbol:
                raise ValueError("CSV symbol not found in job info")
            
            # Get timestamp convention from job info
            timestamp_convention = getattr(job_info, 'timestamp_convention', None)
            if timestamp_convention:
                timestamp_convention = timestamp_convention.value if hasattr(timestamp_convention, 'value') else timestamp_convention
            
            return self.csv_reader.stream_records(
                file_path, 
                csv_symbol, 
                timestamp_convention=timestamp_convention
            )
        else:
            # CRITICAL FIX: Create new DBNReader instance for each job to prevent 
            # cache contamination between concurrent threads
            dbn_reader = DBNReader()
            return dbn_reader.stream_records(file_path)

    def _process_record(self, record, job_info: ImportJobInfo, file_type: ImportFileType):
        """
        Process a single record and extract record info.
        
        Args:
            record: Raw record from file
            job_info: Job information
            file_type: Type of file being imported
            
        Returns:
            Processed record info or None if processing fails
        """
        try:
            # Determine asset type for options metadata extraction
            file_asset_type = None
            if file_type == ImportFileType.DBN:
                # For DBN files, determine asset type from filename/dataset
                filename_lower = job_info.filename.lower()
                if 'opra' in filename_lower or 'cbbo' in filename_lower:
                    from .import_models import AssetType
                    file_asset_type = AssetType.OPTIONS
            
            return self.parquet_writer._extract_record_info_with_context(record, file_asset_type)
            
        except Exception as e:
            logger.debug(f"Error processing record: {e}")
            return None

    def _create_write_metadata(self, job_info: ImportJobInfo, file_type: ImportFileType) -> Dict[str, Any]:
        """
        Create metadata for Parquet write operations.
        
        Args:
            job_info: Job information
            file_type: Type of file being imported
            
        Returns:
            Metadata dictionary
        """
        if file_type == ImportFileType.CSV:
            csv_format = getattr(job_info, 'csv_format', None)
            if csv_format:
                csv_format_str = csv_format.upper() if hasattr(csv_format, 'upper') else str(csv_format).upper()
            else:
                csv_format_str = 'GENERIC'
            
            return {
                'dataset': f'CSV_{csv_format_str}',
                'filename': job_info.filename,
                'symbol': getattr(job_info, 'csv_symbol', 'UNKNOWN')
            }
        else:
            return {
                'dataset': 'OPRA',
                'filename': job_info.filename
            }

    def _update_progress_tracking(self, job_info: ImportJobInfo, record_info: Dict[str, Any], 
                                 import_start_date: Optional[date], import_end_date: Optional[date],
                                 batches: Dict, current_processing_date: Optional[date], job_id: str):
        """
        Update progress tracking based on current record.
        
        Args:
            job_info: Job information
            record_info: Current record information
            import_start_date: Import start date
            import_end_date: Import end date
            batches: Current batches dictionary
            current_processing_date: Current processing date
            job_id: Job ID for logging
        """
        try:
            # Update progress tracking based on current date
            record_date = self._extract_date_from_record_info(record_info)
            if record_date and import_start_date and import_end_date:
                if current_processing_date != record_date:
                    current_processing_date = record_date
                    
                    # Calculate day-based progress
                    progress_info = self._calculate_trading_day_progress(
                        current_processing_date, import_start_date, import_end_date
                    )
                    
                    # Update job progress with day-based information
                    job_info.progress.progress_percentage = progress_info['progress_percentage']
                    job_info.progress.current_symbol = progress_info['status_message']
                    
                    # Track primary symbols being processed for frontend overlay matching
                    if job_info.progress.primary_symbols is None:
                        job_info.progress.primary_symbols = []
                    
                    # Add underlying symbols from current batches to primary symbols
                    current_symbols = set(job_info.progress.primary_symbols)
                    for (underlying_symbol, _, _) in batches.keys():
                        current_symbols.add(underlying_symbol)
                    job_info.progress.primary_symbols = list(current_symbols)
                    
                    logger.debug(f"Job {job_id}: {progress_info['status_message']} - {progress_info['progress_percentage']}% complete")
                    
        except Exception as e:
            logger.debug(f"Error updating progress tracking: {e}")


# Global instance
import_manager = ImportManager()
