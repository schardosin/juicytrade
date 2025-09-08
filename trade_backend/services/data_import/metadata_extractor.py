"""
Metadata Extractor

Handles extraction of DBN file metadata.
"""

import logging
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

from .import_models import DBNFileInfo, DBNMetadata
from .dbn_reader import DBNReader

logger = logging.getLogger(__name__)


class MetadataExtractor:
    """
    Handles extraction of DBN file metadata.
    """
    
    def __init__(self):
        """Initialize metadata extractor."""
        self.dbn_reader = DBNReader()
    
    def get_file_metadata(self, file_path: Path, force_refresh: bool = False) -> DBNMetadata:
        """
        Get metadata for a DBN file.
        
        Args:
            file_path: Path to the DBN file
            force_refresh: Ignored (kept for compatibility)
            
        Returns:
            DBNMetadata object
        """
        logger.info(f"Extracting metadata for {file_path.name}")
        return self.dbn_reader.extract_metadata(file_path)
    
    def get_file_info(self, file_path: Path) -> DBNFileInfo:
        """
        Get basic file info.
        
        Args:
            file_path: Path to the DBN file
            
        Returns:
            DBNFileInfo object
        """
        from .import_models import ImportFileType
        
        # Get basic file info
        file_info_dict = self.dbn_reader.get_file_info(file_path)
        
        return DBNFileInfo(
            filename=file_info_dict['filename'],
            file_path=file_info_dict['file_path'],
            file_size=file_info_dict['file_size'],
            modified_at=file_info_dict['modified_at'],
            file_type=ImportFileType.DBN
        )
    
    def list_files_with_metadata(self, dbn_files_dir: Path) -> List[DBNFileInfo]:
        """
        List all DBN files in directory.
        
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


# Global instance
metadata_extractor = MetadataExtractor()
