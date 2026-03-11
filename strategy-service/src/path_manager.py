from pathlib import Path
import logging

logger = logging.getLogger(__name__)

class PathManager:
    """
    Centralized path management for configuration and data files.
    
    Follows container best practices:
    - Container mode: Uses /app/data (when directory exists)
    - Development mode: Uses current directory (fallback)
    """
    
    def __init__(self):
        self._data_dir = self._determine_data_directory()
        self._config_dir = self._data_dir / "config"
        self._cache_dir = self._data_dir / "cache"
        
        # Ensure directory structure exists
        self._ensure_directory_structure()
    
    def _determine_data_directory(self) -> Path:
        """Determine the appropriate data directory based on environment."""
        container_data_dir = Path("/app/data")
        
        if container_data_dir.exists():
            logger.info(f"📁 Using container data directory: {container_data_dir}")
            return container_data_dir
        else:
            # Development mode - use project root (parent of strategy-service)
            # This ensures parquet data at /project/parquet is accessible
            project_root = Path(__file__).parent.parent.parent
            logger.info(f"📁 Using development data directory: {project_root}")
            return project_root
    
    def _ensure_directory_structure(self):
        """Ensure required directory structure exists."""
        if self.is_container_mode():
            # Only create subdirectories in container mode
            self._config_dir.mkdir(parents=True, exist_ok=True)
            self._cache_dir.mkdir(parents=True, exist_ok=True)
            logger.info(f"📁 Created directory structure in {self._data_dir}")
    
    @property
    def data_dir(self) -> Path:
        """Get the data directory path."""
        return self._data_dir
    
    @property
    def config_dir(self) -> Path:
        """Get the config directory path."""
        return self._config_dir
    
    @property
    def cache_dir(self) -> Path:
        """Get the cache directory path."""
        return self._cache_dir
    
    def is_container_mode(self) -> bool:
        """Check if running in container mode."""
        return self._data_dir == Path("/app/data")
    
    def get_config_file_path(self, filename: str) -> Path:
        """Get the full path for a configuration file."""
        if self.is_container_mode():
            return self._config_dir / filename
        else:
            # Development mode - use root directory
            return self._data_dir / filename
    
    def get_cache_file_path(self, filename: str) -> Path:
        """Get the full path for a cache file."""
        if self.is_container_mode():
            return self._cache_dir / filename
        else:
            # Development mode - use cache subdirectory
            cache_dir = self._data_dir / "cache"
            cache_dir.mkdir(exist_ok=True)
            return cache_dir / filename
    
    def get_status(self) -> dict:
        """Get current path configuration status."""
        return {
            "mode": "container" if self.is_container_mode() else "development",
            "data_dir": str(self._data_dir),
            "config_dir": str(self._config_dir),
            "cache_dir": str(self._cache_dir),
            "data_dir_exists": self._data_dir.exists(),
            "config_dir_exists": self._config_dir.exists(),
            "cache_dir_exists": self._cache_dir.exists()
        }

# Global instance
path_manager = PathManager()
