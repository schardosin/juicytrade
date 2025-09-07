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
    ImportFilters, ImportSummary, DBNFileInfo, DBNMetadata
)
from .metadata_extractor import metadata_extractor
from .dbn_reader import DBNReader
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
        self.parquet_writer = ParquetWriter()
        
        # Job management
        self._active_jobs: Dict[str, ImportJobInfo] = {}
        self._job_lock = threading.Lock()
        self._executor = ThreadPoolExecutor(max_workers=max_concurrent_jobs)
        
        # Load existing jobs
        self._load_existing_jobs()
    
    async def list_available_files(self) -> List[DBNFileInfo]:
        """
        List available DBN files with metadata information.
        
        Returns:
            List of DBNFileInfo objects
        """
        logger.info(f"Listing DBN files in {self.dbn_files_dir}")
        
        # Ensure directory exists
        self.dbn_files_dir.mkdir(exist_ok=True)
        
        # Use metadata extractor to get files with cached metadata
        files_info = metadata_extractor.list_files_with_metadata(self.dbn_files_dir)
        
        logger.info(f"Found {len(files_info)} DBN files")
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
    
    async def get_file_metadata(self, filename: str, force_refresh: bool = False) -> DBNMetadata:
        """
        Get detailed metadata for a specific file.
        
        Args:
            filename: Name of the DBN file
            force_refresh: Force refresh of cached metadata
            
        Returns:
            DBNMetadata object
        """
        file_path = self.dbn_files_dir / filename
        
        if not file_path.exists():
            raise FileNotFoundError(f"DBN file not found: {filename}")
        
        return metadata_extractor.get_file_metadata(file_path, force_refresh)
    
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
            raise FileNotFoundError(f"DBN file not found: {request.filename}")
        
        # Generate job ID
        job_id = str(uuid.uuid4())
        
        # Create job info
        job_info = ImportJobInfo(
            job_id=job_id,
            job_name=request.job_name or f"Import {request.filename}",
            filename=request.filename,
            status=ImportJobStatus.PENDING,
            created_at=datetime.utcnow(),
            filters=request.filters,
            output_format=request.output_format,
            overwrite_existing=request.overwrite_existing
        )
        
        # Store job info
        with self._job_lock:
            self._active_jobs[job_id] = job_info
        
        # Save job to disk
        self._save_job(job_info)
        
        # Submit job to executor
        future = self._executor.submit(self._execute_import_job, job_id)
        
        logger.info(f"Started import job {job_id} for file {request.filename}")
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
                    job_info.completed_at = datetime.utcnow()
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
        This is the rewritten, simplified, and corrected implementation based on the working test.
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
            
            from collections import defaultdict
            batches = defaultdict(list)
            processed_records = 0
            output_paths = []

            # Manually iterate and batch records
            for record in self.dbn_reader.stream_records(file_path):
                processed_records += 1
                record_info = self.parquet_writer._extract_record_info_with_context(record)
                if record_info:
                    key = (
                        record_info['underlying_symbol'],
                        record_info['date'],
                        record_info['asset_type']
                    )
                    batches[key].append(record_info)

                if processed_records % 100000 == 0:
                    logger.info(f"Job {job_id}: Processed {processed_records} records...")
                    job_info.progress.processed_records = processed_records
                    self._save_job(job_info)

            # Write batches to Parquet
            for (underlying_symbol, record_date, asset_type), records in batches.items():
                output_path = self.parquet_writer._write_symbol_date_partition_to_parquet(
                    underlying_symbol, record_date, asset_type, records,
                    {'dataset': 'OPRA', 'filename': job_info.filename}
                )
                if output_path:
                    output_paths.append(output_path)

            job_info.output_paths = output_paths
            job_info.status = ImportJobStatus.COMPLETED
            logger.info(f"Job {job_id} completed. Created {len(output_paths)} files.")

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
                            time_since_start = datetime.utcnow() - job_info.started_at
                            if time_since_start.total_seconds() > 3600:  # 1 hour timeout
                                job_info.status = ImportJobStatus.FAILED
                                job_info.error_message = "Job interrupted (system restart)"
                                job_info.completed_at = datetime.utcnow()
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
            cutoff_time = datetime.utcnow().timestamp() - (days_old * 24 * 3600)
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
        
        # Get DBN files stats
        dbn_stats = {
            'total_files': 0,
            'total_size_bytes': 0,
            'total_size_mb': 0,
            'total_size_gb': 0
        }
        
        if self.dbn_files_dir.exists():
            for dbn_file in self.dbn_files_dir.glob("*.dbn"):
                try:
                    size = dbn_file.stat().st_size
                    dbn_stats['total_files'] += 1
                    dbn_stats['total_size_bytes'] += size
                except Exception:
                    pass
            
            dbn_stats['total_size_mb'] = round(dbn_stats['total_size_bytes'] / (1024 * 1024), 2)
            dbn_stats['total_size_gb'] = round(dbn_stats['total_size_bytes'] / (1024 * 1024 * 1024), 2)
        
        # Get metadata cache stats
        cache_stats = metadata_extractor.get_cache_stats()
        
        return {
            'dbn_files': dbn_stats,
            'parquet_data': parquet_stats,
            'metadata_cache': cache_stats,
            'directories': {
                'dbn_files_dir': str(self.dbn_files_dir),
                'parquet_dir': str(self.parquet_writer.parquet_dir),
                'jobs_dir': str(self.jobs_dir)
            }
        }


# Global instance
import_manager = ImportManager()
