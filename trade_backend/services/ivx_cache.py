import time
from typing import Dict, Any, List, Optional

class IVxCache:
    def __init__(self, ttl_seconds: int = 300):
        self.ttl_seconds = ttl_seconds
        self.cache: Dict[str, Dict[str, Any]] = {}

    def get(self, symbol: str) -> Optional[List[Dict[str, Any]]]:
        """Get cached IVx data for a symbol if available and not expired."""
        if symbol in self.cache:
            cached_data = self.cache[symbol]
            if time.time() - cached_data['timestamp'] < self.ttl_seconds:
                return cached_data['data']
        return None

    def set(self, symbol: str, data: List[Dict[str, Any]]):
        """Cache IVx data for a symbol."""
        self.cache[symbol] = {
            "data": data,
            "timestamp": time.time()
        }

ivx_cache = IVxCache()
