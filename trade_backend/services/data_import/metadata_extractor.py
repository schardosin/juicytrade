"""
Metadata Extractor

Handles efficient extraction and caching of DBN file metadata.
Provides caching mechanisms to avoid re-reading large files repeatedly.
"""

import json
import logging
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Dict, List, Optional, Any
import hashlib

from ...path_manager import path_manager
from .import_models import DBNFileInfo, DBNMetadata
from .dbn_reader import DBNReader

logger = logging.getLogger(__name__)


class MetadataExtractor:
    """
    Handles extraction and caching of DBN file metadata.
    """
    
    def __init__(self, cache_ttl_hours: int = 24):
        """
        Initialize metadata extractor.
        
        Args:
            cache_ttl_hours: Time-to-live for cached metadata in hours
        """
        self.cache_ttl_hours = cache_ttl_hours
        self.dbn_reader = DBNReader()
        
        # Set up cache directory
        self.cache_dir = path_manager.data_dir / "metadata_cache"
        self.cache_dir.mkdir(exist_ok=True)
        
        # Cache file for metadata
        self.cache_file = self.cache_dir / "dbn_metadata_cache.json"
        
        # Load existing cache
        self._cache = self._load_cache()
    
    def get_file_metadata(self, file_path: Path, force_refresh: bool = False) -> DBNMetadata:
        """
        Get metadata for a DBN file, using cache when possible.
        
        Args:
            file_path: Path to the DBN file
            force_refresh: Force refresh of cached metadata
            
        Returns:
            DBNMetadata object
        """
        cache_key = self._get_cache_key(file_path)
        
        # Check cache first (unless force refresh)
        if not force_refresh and cache_key in self._cache:
            cached_entry = self._cache[cache_key]
            
            # Check if cache is still valid
            if self._is_cache_valid(cached_entry, file_path):
                logger.debug(f"Using cached metadata for {file_path.name}")
                return DBNMetadata(**cached_entry['metadata'])
        
        # Extract fresh metadata
        logger.info(f"Extracting fresh metadata for {file_path.name}")
        metadata = self.dbn_reader.extract_metadata(file_path)
        
        # Cache the metadata
        self._cache_metadata(cache_key, metadata, file_path)
        
        return metadata
    
    def get_file_info(self, file_path: Path) -> DBNFileInfo:
        """
        Get basic file info with cached metadata if available.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            DBNFileInfo object
        """
        from .import_models import ImportFileType
        
        # Get basic file info
        file_info_dict = self.dbn_reader.get_file_info(file_path)
        
        file_info = DBNFileInfo(
            filename=file_info_dict['filename'],
            file_path=file_info_dict['file_path'],
            file_size=file_info_dict['file_size'],
            modified_at=file_info_dict['modified_at'],
            file_type=ImportFileType.DBN
        )
        
        # Check if we have cached metadata
        cache_key = self._get_cache_key(file_path)
        if cache_key in self._cache:
            cached_entry = self._cache[cache_key]
            
            if self._is_cache_valid(cached_entry, file_path):
                file_info.metadata = DBNMetadata(**cached_entry['metadata'])
                file_info.metadata_cached = True
                file_info.metadata_cache_time = datetime.fromisoformat(cached_entry['cached_at'])
        
        return file_info
    
    def list_files_with_metadata(self, dbn_files_dir: Path) -> List[DBNFileInfo]:
        """
        List all DBN files in directory with their metadata info.
        
        Args:
            dbn_files_dir: Directory containing DBN files
            
        Returns:
            List of DBNFileInfo objects
        """
        files_info = []
        
        if not dbn_files_dir.exists():
            logger.warning(f"DBN files directory does not exist: {dbn_files_dir}")
            return files_info
        
        # Find all .dbn files
        dbn_files = list(dbn_files_dir.glob("*.dbn"))
        logger.info(f"Found {len(dbn_files)} DBN files in {dbn_files_dir}")
        
        for file_path in dbn_files:
            try:
                file_info = self.get_file_info(file_path)
                files_info.append(file_info)
            except Exception as e:
                logger.error(f"Error getting info for {file_path}: {e}")
                # Create basic file info even if metadata extraction fails
                try:
                    from .import_models import ImportFileType
                    stat = file_path.stat()
                    file_info = DBNFileInfo(
                        filename=file_path.name,
                        file_path=str(file_path),
                        file_size=stat.st_size,
                        modified_at=datetime.fromtimestamp(stat.st_mtime),
                        file_type=ImportFileType.DBN
                    )
                    files_info.append(file_info)
                except Exception as e2:
                    logger.error(f"Error getting basic file info for {file_path}: {e2}")
        
        # Sort by filename
        files_info.sort(key=lambda x: x.filename)
        
        return files_info
    
    def extract_metadata_batch(self, file_paths: List[Path], 
                              progress_callback: Optional[callable] = None) -> Dict[str, DBNMetadata]:
        """
        Extract metadata for multiple files in batch.
        
        Args:
            file_paths: List of file paths to process
            progress_callback: Optional callback for progress updates
            
        Returns:
            Dictionary mapping file paths to metadata
        """
        results = {}
        total_files = len(file_paths)
        
        logger.info(f"Starting batch metadata extraction for {total_files} files")
        
        for i, file_path in enumerate(file_paths):
            try:
                metadata = self.get_file_metadata(file_path)
                results[str(file_path)] = metadata
                
                if progress_callback:
                    progress_callback(i + 1, total_files, file_path.name)
                    
            except Exception as e:
                logger.error(f"Error extracting metadata for {file_path}: {e}")
                # Continue with other files
        
        logger.info(f"Completed batch metadata extraction: {len(results)}/{total_files} successful")
        return results
    
    def clear_cache(self, file_path: Optional[Path] = None):
        """
        Clear metadata cache.
        
        Args:
            file_path: Optional specific file to clear from cache. If None, clears all cache.
        """
        if file_path:
            cache_key = self._get_cache_key(file_path)
            if cache_key in self._cache:
                del self._cache[cache_key]
                logger.info(f"Cleared cache for {file_path.name}")
        else:
            self._cache.clear()
            logger.info("Cleared all metadata cache")
        
        self._save_cache()
    
    def get_cache_stats(self) -> Dict[str, Any]:
        """
        Get cache statistics.
        
        Returns:
            Dictionary with cache statistics
        """
        total_entries = len(self._cache)
        valid_entries = 0
        expired_entries = 0
        
        for cache_key, cached_entry in self._cache.items():
            cached_at = datetime.fromisoformat(cached_entry['cached_at'])
            # Ensure cached_at is timezone-aware
            if cached_at.tzinfo is None:
                cached_at = cached_at.replace(tzinfo=timezone.utc)
            if datetime.now(timezone.utc) - cached_at < timedelta(hours=self.cache_ttl_hours):
                valid_entries += 1
            else:
                expired_entries += 1
        
        return {
            'total_entries': total_entries,
            'valid_entries': valid_entries,
            'expired_entries': expired_entries,
            'cache_file': str(self.cache_file),
            'cache_ttl_hours': self.cache_ttl_hours
        }
    
    def cleanup_expired_cache(self):
        """
        Remove expired entries from cache.
        """
        expired_keys = []
        cutoff_time = datetime.now(timezone.utc) - timedelta(hours=self.cache_ttl_hours)
        
        for cache_key, cached_entry in self._cache.items():
            cached_at = datetime.fromisoformat(cached_entry['cached_at'])
            if cached_at < cutoff_time:
                expired_keys.append(cache_key)
        
        for key in expired_keys:
            del self._cache[key]
        
        if expired_keys:
            logger.info(f"Cleaned up {len(expired_keys)} expired cache entries")
            self._save_cache()
    
    def _get_cache_key(self, file_path: Path) -> str:
        """
        Generate cache key for a file.
        
        Args:
            file_path: Path to the file
            
        Returns:
            Cache key string
        """
        # Use file path and modification time for cache key
        try:
            stat = file_path.stat()
            key_data = f"{file_path}:{stat.st_mtime}:{stat.st_size}"
            return hashlib.md5(key_data.encode()).hexdigest()
        except Exception:
            # Fallback to just file path
            return hashlib.md5(str(file_path).encode()).hexdigest()
    
    def _is_cache_valid(self, cached_entry: Dict[str, Any], file_path: Path) -> bool:
        """
        Check if cached entry is still valid.
        
        Args:
            cached_entry: Cached entry data
            file_path: Path to the file
            
        Returns:
            True if cache is valid, False otherwise
        """
        try:
            # Check TTL
            cached_at = datetime.fromisoformat(cached_entry['cached_at'])
            # Ensure cached_at is timezone-aware
            if cached_at.tzinfo is None:
                cached_at = cached_at.replace(tzinfo=timezone.utc)
            if datetime.now(timezone.utc) - cached_at > timedelta(hours=self.cache_ttl_hours):
                return False
            
            # Check if file has been modified
            stat = file_path.stat()
            cached_mtime = cached_entry.get('file_mtime', 0)
            cached_size = cached_entry.get('file_size', 0)
            
            if stat.st_mtime != cached_mtime or stat.st_size != cached_size:
                return False
            
            return True
            
        except Exception as e:
            logger.warning(f"Error validating cache for {file_path}: {e}")
            return False
    
    def _cache_metadata(self, cache_key: str, metadata: DBNMetadata, file_path: Path):
        """
        Cache metadata for a file.
        
        Args:
            cache_key: Cache key
            metadata: Metadata to cache
            file_path: Path to the file
        """
        try:
            stat = file_path.stat()
            
            cache_entry = {
                'cached_at': datetime.now(timezone.utc).isoformat(),
                'file_mtime': stat.st_mtime,
                'file_size': stat.st_size,
                'metadata': metadata.model_dump()
            }
            
            self._cache[cache_key] = cache_entry
            self._save_cache()
            
        except Exception as e:
            logger.error(f"Error caching metadata for {file_path}: {e}")
    
    def _load_cache(self) -> Dict[str, Any]:
        """
        Load cache from file.
        
        Returns:
            Cache dictionary
        """
        try:
            if self.cache_file.exists():
                with open(self.cache_file, 'r') as f:
                    cache_data = json.load(f)
                logger.info(f"Loaded metadata cache with {len(cache_data)} entries")
                return cache_data
        except Exception as e:
            logger.warning(f"Error loading metadata cache: {e}")
        
        return {}
    
    def _save_cache(self):
        """
        Save cache to file.
        """
        try:
            with open(self.cache_file, 'w') as f:
                json.dump(self._cache, f, indent=2, default=str)
        except Exception as e:
            logger.error(f"Error saving metadata cache: {e}")


# Global instance
metadata_extractor = MetadataExtractor()
