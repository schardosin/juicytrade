"""
Import Queue Manager

Manages a FIFO queue of import jobs, processing files sequentially
to avoid resource conflicts and provide ordered execution.
"""

import json
import logging
import uuid
import threading
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional, Any
from concurrent.futures import ThreadPoolExecutor

from ...path_manager import path_manager
from .import_models import (
    ImportQueueItem, ImportQueueStatus, QueueStatus, 
    MultiFileImportRequest, ImportRequest, ImportFileType,
    CSVFormat, TimestampConvention
)

logger = logging.getLogger(__name__)


class ImportQueue:
    """
    Manages a FIFO queue of import jobs with sequential processing.
    """
    
    def __init__(self, import_manager):
        """
        Initialize the import queue.
        
        Args:
            import_manager: Reference to the ImportManager instance
        """
        self.import_manager = import_manager
        
        # Queue storage
        self.queue_dir = path_manager.data_dir / "import_queue"
        self.queue_dir.mkdir(exist_ok=True)
        
        # Queue state
        self._queue_items: Dict[str, ImportQueueItem] = {}
        self._queue_lock = threading.Lock()
        self._processing_item: Optional[ImportQueueItem] = None
        
        # Processing control
        self._executor = ThreadPoolExecutor(max_workers=1)  # Sequential processing
        self._processing = False
        
        # Load existing queue items
        self._load_queue_from_disk()
        
        # Start queue processor
        self._start_queue_processor()
    
    def add_files_to_queue(self, request: MultiFileImportRequest) -> List[str]:
        """
        Add multiple files to the import queue.
        
        Args:
            request: Multi-file import request
            
        Returns:
            List of queue IDs for the added items
        """
        queue_ids = []
        batch_id = str(uuid.uuid4())
        
        with self._queue_lock:
            # Get current max position
            max_position = 0
            if self._queue_items:
                max_position = max(item.queue_position for item in self._queue_items.values())
            
            for i, file_config in enumerate(request.files):
                queue_id = str(uuid.uuid4())
                
                # Create queue item
                queue_item = ImportQueueItem(
                    queue_id=queue_id,
                    filename=file_config['filename'],
                    queue_position=max_position + i + 1,
                    queue_status=QueueStatus.QUEUED,
                    created_at=datetime.now(timezone.utc),
                    batch_id=batch_id,
                    batch_name=request.batch_name,
                    overwrite_existing=request.overwrite_existing,
                    
                    # File-specific configuration
                    file_type=file_config.get('file_type'),
                    csv_symbol=file_config.get('csv_symbol'),
                    csv_format=file_config.get('csv_format'),
                    timestamp_convention=file_config.get('timestamp_convention')
                )
                
                self._queue_items[queue_id] = queue_item
                queue_ids.append(queue_id)
                
                # Save to disk
                self._save_queue_item(queue_item)
        
        logger.info(f"Added {len(queue_ids)} files to import queue (batch: {batch_id})")
        
        # Trigger queue processing
        self._trigger_queue_processing()
        
        return queue_ids
    
    def get_queue_status(self) -> ImportQueueStatus:
        """
        Get current status of the import queue.
        
        Returns:
            ImportQueueStatus with current state
        """
        with self._queue_lock:
            # Sort items by queue position
            sorted_items = sorted(self._queue_items.values(), key=lambda x: x.queue_position)
            
            # Count items by status
            completed_items = sum(1 for item in sorted_items if item.queue_status == QueueStatus.COMPLETED)
            failed_items = sum(1 for item in sorted_items if item.queue_status == QueueStatus.FAILED)
            queued_items = sum(1 for item in sorted_items if item.queue_status == QueueStatus.QUEUED)
            
            return ImportQueueStatus(
                queue_items=sorted_items,
                current_processing=self._processing_item,
                total_items=len(sorted_items),
                completed_items=completed_items,
                failed_items=failed_items,
                queued_items=queued_items
            )
    
    def remove_from_queue(self, queue_id: str) -> bool:
        """
        Remove an item from the queue.
        
        Args:
            queue_id: Queue ID to remove
            
        Returns:
            True if removed, False if not found or currently processing
        """
        with self._queue_lock:
            if queue_id not in self._queue_items:
                return False
            
            item = self._queue_items[queue_id]
            
            # Don't allow removal of currently processing item
            if item.queue_status == QueueStatus.PROCESSING:
                return False
            
            # Remove from memory and disk
            del self._queue_items[queue_id]
            self._delete_queue_item(queue_id)
            
            logger.info(f"Removed queue item {queue_id} ({item.filename})")
            return True
    
    def _start_queue_processor(self):
        """
        Start the background queue processor.
        """
        self._executor.submit(self._process_queue_continuously)
    
    def _process_queue_continuously(self):
        """
        Continuously process the queue (runs in background thread).
        """
        while True:
            try:
                # Check if there's anything to process
                next_item = self._get_next_queued_item()
                
                if next_item:
                    self._process_queue_item(next_item)
                else:
                    # No items to process, wait a bit
                    import time
                    time.sleep(2)
                    
            except Exception as e:
                logger.error(f"Error in queue processor: {e}")
                import time
                time.sleep(5)  # Wait longer on error
    
    def _get_next_queued_item(self) -> Optional[ImportQueueItem]:
        """
        Get the next item in queue that needs processing.
        
        Returns:
            Next queued item or None if nothing to process
        """
        with self._queue_lock:
            # Don't start new items if already processing
            if self._processing_item:
                return None
            
            # Find the next queued item (lowest position number)
            queued_items = [
                item for item in self._queue_items.values() 
                if item.queue_status == QueueStatus.QUEUED
            ]
            
            if not queued_items:
                return None
            
            # Sort by position and return first
            queued_items.sort(key=lambda x: x.queue_position)
            return queued_items[0]
    
    def _process_queue_item(self, queue_item: ImportQueueItem):
        """
        Process a single queue item.
        
        Args:
            queue_item: Item to process
        """
        import asyncio
        
        try:
            # Mark as processing
            with self._queue_lock:
                queue_item.queue_status = QueueStatus.PROCESSING
                queue_item.started_at = datetime.now(timezone.utc)
                self._processing_item = queue_item
                self._save_queue_item(queue_item)
            
            logger.info(f"Processing queue item {queue_item.queue_id}: {queue_item.filename}")
            
            # Create import request from queue item
            import_request = self._create_import_request(queue_item)
            
            # Start the import job (run async function in sync context)
            job_id = asyncio.run(self.import_manager.start_import_job(import_request))
            
            # Update queue item with job ID
            with self._queue_lock:
                queue_item.job_id = job_id
                self._save_queue_item(queue_item)
            
            # Wait for job completion
            self._wait_for_job_completion(queue_item, job_id)
            
        except Exception as e:
            logger.error(f"Error processing queue item {queue_item.queue_id}: {e}")
            
            # Mark as failed
            with self._queue_lock:
                queue_item.queue_status = QueueStatus.FAILED
                queue_item.error_message = str(e)
                queue_item.completed_at = datetime.now(timezone.utc)
                self._processing_item = None
                self._save_queue_item(queue_item)
        
        finally:
            # Clear processing state
            with self._queue_lock:
                self._processing_item = None
    
    def _create_import_request(self, queue_item: ImportQueueItem) -> ImportRequest:
        """
        Create an ImportRequest from a queue item.
        
        Args:
            queue_item: Queue item to convert
            
        Returns:
            ImportRequest for the import manager
        """
        job_name = f"{queue_item.batch_name or 'Queue Import'} - {queue_item.filename}"
        
        return ImportRequest(
            filename=queue_item.filename,
            job_name=job_name,
            overwrite_existing=queue_item.overwrite_existing,
            file_type=queue_item.file_type,
            csv_symbol=queue_item.csv_symbol,
            csv_format=queue_item.csv_format,
            timestamp_convention=queue_item.timestamp_convention
        )
    
    def _wait_for_job_completion(self, queue_item: ImportQueueItem, job_id: str):
        """
        Wait for an import job to complete and update queue item status.
        
        Args:
            queue_item: Queue item being processed
            job_id: Import job ID to monitor
        """
        import asyncio
        import time
        
        while True:
            try:
                # Check job status
                job_info = asyncio.run(self.import_manager.get_job_status(job_id))
                
                if job_info.is_finished:
                    # Job completed
                    with self._queue_lock:
                        if job_info.status.value == 'completed':
                            queue_item.queue_status = QueueStatus.COMPLETED
                        else:
                            queue_item.queue_status = QueueStatus.FAILED
                            queue_item.error_message = job_info.error_message
                        
                        queue_item.completed_at = datetime.now(timezone.utc)
                        self._save_queue_item(queue_item)
                    
                    logger.info(f"Queue item {queue_item.queue_id} completed with status: {queue_item.queue_status}")
                    break
                
                # Still running, wait and check again
                time.sleep(2)
                
            except Exception as e:
                logger.error(f"Error checking job status for {job_id}: {e}")
                time.sleep(5)
    
    def _trigger_queue_processing(self):
        """
        Trigger queue processing (called when new items are added).
        """
        # The continuous processor will pick up new items automatically
        pass
    
    def _save_queue_item(self, queue_item: ImportQueueItem):
        """
        Save a queue item to disk.
        
        Args:
            queue_item: Item to save
        """
        try:
            queue_file = self.queue_dir / f"{queue_item.queue_id}.json"
            
            with open(queue_file, 'w') as f:
                json.dump(queue_item.model_dump(), f, indent=2, default=str)
                
        except Exception as e:
            logger.error(f"Error saving queue item {queue_item.queue_id}: {e}")
    
    def _delete_queue_item(self, queue_id: str):
        """
        Delete a queue item from disk.
        
        Args:
            queue_id: Queue ID to delete
        """
        try:
            queue_file = self.queue_dir / f"{queue_id}.json"
            if queue_file.exists():
                queue_file.unlink()
        except Exception as e:
            logger.error(f"Error deleting queue item {queue_id}: {e}")
    
    def _load_queue_from_disk(self):
        """
        Load existing queue items from disk.
        """
        try:
            for queue_file in self.queue_dir.glob("*.json"):
                try:
                    with open(queue_file, 'r') as f:
                        queue_data = json.load(f)
                    
                    queue_item = ImportQueueItem(**queue_data)
                    
                    # Reset processing items to queued (they were interrupted)
                    if queue_item.queue_status == QueueStatus.PROCESSING:
                        queue_item.queue_status = QueueStatus.QUEUED
                        queue_item.started_at = None
                        queue_item.job_id = None
                    
                    self._queue_items[queue_item.queue_id] = queue_item
                    
                except Exception as e:
                    logger.warning(f"Error loading queue item {queue_file}: {e}")
            
            logger.info(f"Loaded {len(self._queue_items)} queue items from disk")
            
        except Exception as e:
            logger.error(f"Error loading queue from disk: {e}")
