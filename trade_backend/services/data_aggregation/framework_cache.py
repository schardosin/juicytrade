"""
Framework-level caching system for data services.

This provides generic caching capabilities that any strategy or service can use,
with TTL (Time To Live) support and pattern-based cache invalidation.
"""

import logging
import time
from datetime import datetime, timedelta
from typing import Any, Optional, Dict, List
import threading
from dataclasses import dataclass

logger = logging.getLogger(__name__)


@dataclass
class CacheEntry:
    """Represents a single cache entry with TTL support."""
    data: Any
    created_at: float  # Unix timestamp
    ttl_hours: int
    
    def is_expired(self) -> bool:
        """Check if this cache entry has expired."""
        if self.ttl_hours <= 0:  # Permanent cache
            return False
        
        expiry_time = self.created_at + (self.ttl_hours * 3600)
        return time.time() > expiry_time
    
    def time_to_expiry(self) -> float:
        """Get seconds until expiry (negative if already expired)."""
        if self.ttl_hours <= 0:
            return float('inf')
        
        expiry_time = self.created_at + (self.ttl_hours * 3600)
        return expiry_time - time.time()


class FrameworkCache:
    """
    Generic framework-level caching system.
    
    Provides TTL-based caching for any data type, with thread-safe operations
    and pattern-based cache invalidation. Any strategy or service can use this.
    
    Features:
    - TTL (Time To Live) support with configurable expiration
    - Thread-safe operations for multi-strategy usage
    - Pattern-based cache invalidation
    - Automatic cleanup of expired entries
    - Memory usage tracking and limits
    """
    
    def __init__(self, max_entries: int = 10000, cleanup_interval: int = 300):
        """
        Initialize the framework cache.
        
        Args:
            max_entries: Maximum number of cache entries before cleanup
            cleanup_interval: Seconds between automatic cleanup runs
        """
        self._cache: Dict[str, CacheEntry] = {}
        self._lock = threading.RLock()
        self._max_entries = max_entries
        self._cleanup_interval = cleanup_interval
        self._last_cleanup = time.time()
        
        logger.info(f"FrameworkCache initialized with max_entries={max_entries}")
    
    def cache_data(self, key: str, data: Any, ttl_hours: int = 24) -> None:
        """
        Cache data with TTL support.
        
        Args:
            key: Cache key (should be descriptive, e.g., "expirations_SPXW_2025-08-12")
            data: Data to cache (any serializable type)
            ttl_hours: Time to live in hours (0 = permanent, -1 = one-time use)
        """
        with self._lock:
            entry = CacheEntry(
                data=data,
                created_at=time.time(),
                ttl_hours=ttl_hours
            )
            
            self._cache[key] = entry
            
            # Automatic cleanup if needed
            if len(self._cache) > self._max_entries or \
               time.time() - self._last_cleanup > self._cleanup_interval:
                self._cleanup_expired()
            
            logger.debug(f"Cached data: {key} (TTL: {ttl_hours}h, size: {len(self._cache)})")
    
    def get_cached_data(self, key: str) -> Optional[Any]:
        """
        Get cached data if it exists and hasn't expired.
        
        Args:
            key: Cache key to retrieve
            
        Returns:
            Cached data or None if not found/expired
        """
        with self._lock:
            entry = self._cache.get(key)
            
            if entry is None:
                logger.debug(f"Cache miss: {key}")
                return None
            
            if entry.is_expired():
                logger.debug(f"Cache expired: {key}")
                del self._cache[key]
                return None
            
            # Handle one-time use entries
            if entry.ttl_hours == -1:
                logger.debug(f"Cache one-time use: {key}")
                del self._cache[key]
            else:
                logger.debug(f"Cache hit: {key} (expires in {entry.time_to_expiry():.0f}s)")
            
            return entry.data
    
    def invalidate_cache(self, pattern: Optional[str] = None) -> int:
        """
        Invalidate cache entries, optionally by pattern matching.
        
        Args:
            pattern: Optional pattern to match keys (None = clear all)
                    Examples: "expirations_*", "*_2025-08-12", "SPXW_*"
            
        Returns:
            Number of entries invalidated
        """
        with self._lock:
            if pattern is None:
                # Clear all
                count = len(self._cache)
                self._cache.clear()
                logger.info(f"Invalidated entire cache: {count} entries")
                return count
            
            # Pattern-based invalidation
            import fnmatch
            keys_to_remove = []
            
            for key in self._cache.keys():
                if fnmatch.fnmatch(key, pattern):
                    keys_to_remove.append(key)
            
            for key in keys_to_remove:
                del self._cache[key]
            
            logger.info(f"Invalidated cache pattern '{pattern}': {len(keys_to_remove)} entries")
            return len(keys_to_remove)
    
    def _cleanup_expired(self) -> int:
        """
        Clean up expired cache entries.
        
        Returns:
            Number of entries cleaned up
        """
        with self._lock:
            expired_keys = []
            
            for key, entry in self._cache.items():
                if entry.is_expired():
                    expired_keys.append(key)
            
            for key in expired_keys:
                del self._cache[key]
            
            self._last_cleanup = time.time()
            
            if expired_keys:
                logger.debug(f"Cleaned up {len(expired_keys)} expired cache entries")
            
            return len(expired_keys)
    
    def get_cache_stats(self) -> Dict[str, Any]:
        """
        Get cache statistics for monitoring.
        
        Returns:
            Dictionary with cache statistics
        """
        with self._lock:
            total_entries = len(self._cache)
            expired_count = 0
            permanent_count = 0
            
            for entry in self._cache.values():
                if entry.is_expired():
                    expired_count += 1
                elif entry.ttl_hours <= 0:
                    permanent_count += 1
            
            return {
                "total_entries": total_entries,
                "expired_entries": expired_count,
                "permanent_entries": permanent_count,
                "active_entries": total_entries - expired_count,
                "max_entries": self._max_entries,
                "last_cleanup": datetime.fromtimestamp(self._last_cleanup).isoformat()
            }
    
    def has_cached_data(self, key: str) -> bool:
        """
        Check if data is cached and not expired (without retrieving it).
        
        Args:
            key: Cache key to check
            
        Returns:
            True if data is cached and valid
        """
        with self._lock:
            entry = self._cache.get(key)
            return entry is not None and not entry.is_expired()
    
    def get_cache_keys(self, pattern: Optional[str] = None) -> List[str]:
        """
        Get all cache keys, optionally filtered by pattern.
        
        Args:
            pattern: Optional pattern to match keys
            
        Returns:
            List of matching cache keys
        """
        with self._lock:
            if pattern is None:
                return list(self._cache.keys())
            
            import fnmatch
            return [key for key in self._cache.keys() if fnmatch.fnmatch(key, pattern)]


# Global framework cache instance
_framework_cache: Optional[FrameworkCache] = None


def get_framework_cache() -> FrameworkCache:
    """
    Get the global framework cache instance.
    
    Returns:
        The singleton FrameworkCache instance
    """
    global _framework_cache
    
    if _framework_cache is None:
        _framework_cache = FrameworkCache()
    
    return _framework_cache


def clear_framework_cache():
    """Clear the global framework cache."""
    cache = get_framework_cache()
    cache.invalidate_cache()


def get_framework_cache_stats() -> Dict[str, Any]:
    """Get framework cache statistics."""
    cache = get_framework_cache()
    return cache.get_cache_stats()
