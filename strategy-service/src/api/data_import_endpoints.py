"""
Data Import API Endpoints

Provides HTTP endpoints for data import operations including file management,
import jobs, and queue processing.
"""

import logging
from datetime import datetime
from typing import Optional, List, Any
from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel

# Handle 'import' directory name by importing from the sibling package
from src.data.data_import.import_manager import import_manager
from src.data.data_import.import_models import (
    ImportRequest, ImportJobStatus, MultiFileImportRequest
)

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/data-import", tags=["data-import"])


class ApiResponse(BaseModel):
    """Standardized API response wrapper."""
    success: bool
    data: Optional[Any] = None
    error: Optional[str] = None
    message: Optional[str] = None
    timestamp: str = datetime.now().isoformat()


@router.get("/files", response_model=ApiResponse)
async def list_import_files():
    """List available import files (DBN and CSV) with basic information only - fast loading."""
    try:
        files_info = []
        
        import_manager.dbn_files_dir.mkdir(exist_ok=True)
        
        for dbn_file in import_manager.dbn_files_dir.glob("*.dbn"):
            try:
                file_stat = dbn_file.stat()
                file_info = {
                    'filename': dbn_file.name,
                    'file_path': str(dbn_file),
                    'file_size': file_stat.st_size,
                    'modified_at': datetime.fromtimestamp(file_stat.st_mtime),
                    'file_type': 'dbn',
                    'needs_symbol_input': False,
                    'size_mb': round(file_stat.st_size / (1024 * 1024), 2),
                    'size_gb': round(file_stat.st_size / (1024 * 1024 * 1024), 2)
                }
                files_info.append(file_info)
            except Exception as e:
                logger.warning(f"Error processing DBN file {dbn_file}: {e}")
                continue
        
        for csv_file in import_manager.dbn_files_dir.glob("*.csv"):
            try:
                file_stat = csv_file.stat()
                file_info = {
                    'filename': csv_file.name,
                    'file_path': str(csv_file),
                    'file_size': file_stat.st_size,
                    'modified_at': datetime.fromtimestamp(file_stat.st_mtime),
                    'file_type': 'csv',
                    'needs_symbol_input': True,
                    'size_mb': round(file_stat.st_size / (1024 * 1024), 2),
                    'size_gb': round(file_stat.st_size / (1024 * 1024 * 1024), 2)
                }
                files_info.append(file_info)
            except Exception as e:
                logger.warning(f"Error processing CSV file {csv_file}: {e}")
                continue
        
        files_info.sort(key=lambda x: x['modified_at'], reverse=True)
        
        dbn_count = sum(1 for f in files_info if f['file_type'] == 'dbn')
        csv_count = sum(1 for f in files_info if f['file_type'] == 'csv')
        
        return ApiResponse(
            success=True,
            data=files_info,
            message=f"Found {len(files_info)} import files ({dbn_count} DBN, {csv_count} CSV) - basic info only"
        )
    except Exception as e:
        logger.error(f"Error listing import files: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/files/detailed", response_model=ApiResponse)
async def list_import_files_detailed():
    """List available import files with full metadata information - slower but complete."""
    try:
        files_info = await import_manager.list_available_files()
        
        files_data = []
        for file_info in files_info:
            file_dict = file_info.model_dump() if hasattr(file_info, 'model_dump') else file_info.dict()
            files_data.append(file_dict)
        
        return ApiResponse(
            success=True,
            data=files_data,
            message=f"Found {len(files_data)} import files"
        )
    except Exception as e:
        logger.error(f"Error listing import files with metadata: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/imported-data", response_model=ApiResponse)
async def get_imported_data(expand: bool = False):
    """Get list of imported datasets by symbol."""
    try:
        if expand:
            symbol_datasets = import_manager.get_symbol_level_data()
            message = f"Retrieved {len(symbol_datasets)} imported symbol datasets with detailed information"
        else:
            symbol_datasets = import_manager.get_symbol_level_data_basic()
            message = f"Retrieved {len(symbol_datasets)} imported symbol datasets (basic info only)"
        
        return ApiResponse(
            success=True,
            data=symbol_datasets,
            message=message
        )
    except Exception as e:
        logger.error(f"Error getting imported data: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/imported-data/{symbol}", response_model=ApiResponse)
async def get_imported_data_by_symbol(symbol: str, asset_type: Optional[str] = None):
    """Get imported data for a specific symbol."""
    try:
        symbol_datasets = import_manager.get_symbol_level_data()
        
        for dataset in symbol_datasets:
            if dataset.get('symbol', '').upper() == symbol.upper():
                if asset_type:
                    if dataset.get('asset_type', '').lower() == asset_type.lower():
                        return ApiResponse(success=True, data=dataset)
                else:
                    return ApiResponse(success=True, data=dataset)
        
        return ApiResponse(
            success=False,
            data=None,
            message=f"No imported data found for symbol {symbol}"
        )
    except Exception as e:
        logger.error(f"Error getting imported data for {symbol}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/metadata/{filename}", response_model=ApiResponse)
async def get_file_metadata(filename: str, symbol: Optional[str] = None, force_refresh: bool = False):
    """Get detailed metadata for a specific import file."""
    try:
        file_type = import_manager._detect_file_type(filename)
        
        if file_type.value == 'csv' and not symbol:
            from src.data.data_import.csv_metadata_extractor import csv_metadata_extractor
            file_path = import_manager.dbn_files_dir / filename
            if not file_path.exists():
                raise HTTPException(status_code=404, detail=f"File not found: {filename}")
            
            csv_analysis = csv_metadata_extractor.analyze_csv_structure(file_path)
            
            basic_metadata = {
                "filename": filename,
                "file_path": str(file_path),
                "file_size": file_path.stat().st_size,
                "file_type": "csv",
                "csv_format": csv_analysis.get('csv_format', 'generic'),
                "csv_headers": csv_analysis.get('headers', []),
                "record_count": csv_analysis.get('record_count', 0),
                "start_date": csv_analysis.get('start_date'),
                "end_date": csv_analysis.get('end_date'),
                "needs_symbol_input": True,
                "message": "CSV file detected. Symbol input will be required for import."
            }
            
            return ApiResponse(
                success=True,
                data=basic_metadata,
                message=f"Retrieved basic structure for CSV file {filename}"
            )
        else:
            metadata = await import_manager.get_file_metadata(filename, symbol, force_refresh)
            
            return ApiResponse(
                success=True,
                data=metadata.model_dump() if hasattr(metadata, 'model_dump') else metadata.dict(),
                message=f"Retrieved metadata for {filename}"
            )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting file metadata for {filename}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/jobs", response_model=ApiResponse)
async def start_import_job(request: ImportRequest):
    """Start a new data import job."""
    try:
        job_id = await import_manager.start_import_job(request)
        
        return ApiResponse(
            success=True,
            data={
                "job_id": job_id,
                "filename": request.filename,
                "status": "pending"
            },
            message=f"Import job started for {request.filename}"
        )
    except FileNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error starting import job: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/jobs/{job_id}/status", response_model=ApiResponse)
async def get_import_job_status(job_id: str):
    """Get status and progress of an import job."""
    try:
        job_info = await import_manager.get_job_status(job_id)
        
        return ApiResponse(
            success=True,
            data=job_info.model_dump() if hasattr(job_info, 'model_dump') else job_info.dict(),
            message=f"Retrieved status for job {job_id}"
        )
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        logger.error(f"Error getting job status for {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/jobs", response_model=ApiResponse)
async def list_import_jobs(status: Optional[str] = None, limit: int = 50):
    """List import jobs with optional status filtering."""
    try:
        status_filter = None
        if status:
            try:
                status_filter = ImportJobStatus(status.lower())
            except ValueError:
                raise HTTPException(status_code=400, detail=f"Invalid status: {status}")
        
        jobs = await import_manager.list_import_jobs(status_filter, limit)
        
        jobs_data = [job.model_dump() if hasattr(job, 'model_dump') else job.dict() for job in jobs]
        
        return ApiResponse(
            success=True,
            data={
                "jobs": jobs_data,
                "total_jobs": len(jobs_data),
                "status_filter": status,
                "limit": limit
            },
            message=f"Retrieved {len(jobs_data)} import jobs"
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error listing import jobs: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/jobs/{job_id}", response_model=ApiResponse)
async def cancel_import_job(job_id: str):
    """Cancel a running import job."""
    try:
        cancelled = await import_manager.cancel_import_job(job_id)
        
        if cancelled:
            return ApiResponse(
                success=True,
                data={"job_id": job_id, "cancelled": True},
                message=f"Import job {job_id} cancelled successfully"
            )
        else:
            return ApiResponse(
                success=False,
                data={"job_id": job_id, "cancelled": False},
                message=f"Could not cancel job {job_id} (may not be active)"
            )
    except Exception as e:
        logger.error(f"Error cancelling import job {job_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/queue/status", response_model=ApiResponse)
async def get_import_queue_status():
    """Get current status of the import queue."""
    try:
        queue_status = import_manager.import_queue.get_queue_status()
        
        return ApiResponse(
            success=True,
            data=queue_status.model_dump() if hasattr(queue_status, 'model_dump') else dict(queue_status),
            message=f"Retrieved queue status"
        )
    except Exception as e:
        logger.error(f"Error getting import queue status: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/queue", response_model=ApiResponse)
async def add_files_to_import_queue(request: MultiFileImportRequest):
    """Add multiple files to the import queue for sequential processing."""
    try:
        queue_ids = import_manager.import_queue.add_files_to_queue(request)
        
        return ApiResponse(
            success=True,
            data={
                "queue_ids": queue_ids,
                "batch_name": request.batch_name,
                "total_files": len(request.files)
            },
            message=f"Added {len(request.files)} files to import queue"
        )
    except Exception as e:
        logger.error(f"Error adding files to import queue: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/queue/batch", response_model=ApiResponse)
async def add_batch_to_import_queue(request: dict):
    """Add multiple files to the import queue as a batch."""
    try:
        jobs = request.get('jobs', [])
        if not jobs:
            raise HTTPException(status_code=400, detail="No jobs provided in batch")
        
        job_ids = []
        for job_data in jobs:
            import_request = ImportRequest(
                filename=job_data['filename'],
                job_name=job_data.get('job_name', f"Import {job_data['filename']}"),
                overwrite_existing=job_data.get('overwrite_existing', False),
                file_type=job_data.get('file_type'),
                csv_symbol=job_data.get('csv_symbol'),
                csv_format=job_data.get('csv_format'),
                timestamp_convention=job_data.get('timestamp_convention')
            )
            
            job_id = await import_manager.start_import_job(import_request)
            job_ids.append(job_id)
        
        return ApiResponse(
            success=True,
            data={
                "job_ids": job_ids,
                "total_jobs": len(job_ids),
                "batch_size": len(jobs)
            },
            message=f"Successfully added {len(jobs)} files to import queue"
        )
    except Exception as e:
        logger.error(f"Error adding batch to import queue: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/queue/{queue_id}", response_model=ApiResponse)
async def remove_from_import_queue(queue_id: str):
    """Remove an item from the import queue."""
    try:
        removed = import_manager.import_queue.remove_from_queue(queue_id)
        
        if removed:
            return ApiResponse(
                success=True,
                data={"queue_id": queue_id, "removed": True},
                message=f"Queue item {queue_id} removed successfully"
            )
        else:
            return ApiResponse(
                success=False,
                data={"queue_id": queue_id, "removed": False},
                message=f"Could not remove queue item {queue_id} (not found or currently processing)"
            )
    except Exception as e:
        logger.error(f"Error removing from import queue {queue_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/queue/clear", response_model=ApiResponse)
async def clear_import_queue():
    """Clear all items from the import queue."""
    try:
        queue_status = import_manager.import_queue.get_queue_status()
        
        removed_count = 0
        for item in queue_status.queue_items:
            if import_manager.import_queue.remove_from_queue(item.queue_id):
                removed_count += 1
        
        return ApiResponse(
            success=True,
            data={"cleared_items": removed_count},
            message=f"Cleared {removed_count} items from import queue"
        )
    except Exception as e:
        logger.error(f"Error clearing import queue: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/summary", response_model=ApiResponse)
async def get_import_summary():
    """Get summary statistics for import operations."""
    try:
        summary = await import_manager.get_import_summary()
        
        return ApiResponse(
            success=True,
            data=summary.model_dump() if hasattr(summary, 'model_dump') else dict(summary),
            message="Retrieved import summary statistics"
        )
    except Exception as e:
        logger.error(f"Error getting import summary: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/storage/stats", response_model=ApiResponse)
async def get_storage_stats():
    """Get comprehensive storage statistics for data import system."""
    try:
        stats = import_manager.get_storage_stats()
        
        return ApiResponse(
            success=True,
            data=stats,
            message="Retrieved storage statistics"
        )
    except Exception as e:
        logger.error(f"Error getting storage stats: {e}")
        raise HTTPException(status_code=500, detail=str(e))
