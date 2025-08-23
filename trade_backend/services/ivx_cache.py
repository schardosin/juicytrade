import time
import logging
from typing import Dict, Any, List, Optional

logger = logging.getLogger(__name__)

class IVxCache:
    def __init__(self, ttl_seconds: int = 300):
        self.ttl_seconds = ttl_seconds
        self.cache: Dict[str, Dict[str, Any]] = {}

    def get(self, symbol: str) -> Optional[List[Dict[str, Any]]]:
        """Get cached IVx data for a symbol if available and not expired."""
        if symbol in self.cache:
            cached_data = self.cache[symbol]
            age = time.time() - cached_data['timestamp']
            
            if age < self.ttl_seconds:
                remaining_ttl = self.ttl_seconds - age
                logger.info(f"📦 Cache HIT for {symbol} (age: {age:.1f}s, TTL remaining: {remaining_ttl:.1f}s)")
                return cached_data['data']
            else:
                logger.info(f"⏰ Cache EXPIRED for {symbol} (age: {age:.1f}s > TTL: {self.ttl_seconds}s)")
                # Remove expired entry
                del self.cache[symbol]
        else:
            logger.info(f"❌ Cache MISS for {symbol} (not in cache)")
        
        return None

    def set(self, symbol: str, data: List[Dict[str, Any]]):
        """Cache IVx data for a symbol."""
        self.cache[symbol] = {
            "data": data,
            "timestamp": time.time()
        }
        logger.info(f"💾 Cached IVx data for {symbol} ({len(data)} expirations, TTL: {self.ttl_seconds}s)")
    
    def get_cache_info(self) -> Dict[str, Any]:
        """Get information about current cache state."""
        current_time = time.time()
        cache_info = {}
        
        for symbol, cached_data in self.cache.items():
            age = current_time - cached_data['timestamp']
            remaining_ttl = max(0, self.ttl_seconds - age)
            is_expired = age >= self.ttl_seconds
            
            cache_info[symbol] = {
                "age_seconds": age,
                "remaining_ttl": remaining_ttl,
                "is_expired": is_expired,
                "expiration_count": len(cached_data['data'])
            }
        
        return {
            "total_cached_symbols": len(self.cache),
            "ttl_seconds": self.ttl_seconds,
            "symbols": cache_info
        }

ivx_cache = IVxCache()
