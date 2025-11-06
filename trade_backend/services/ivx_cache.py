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
            
            # Use stored TTL or fallback to default
            entry_ttl = cached_data.get('ttl', self.ttl_seconds)
            
            if age < entry_ttl:
                remaining_ttl = entry_ttl - age
                ttl_desc = f"{entry_ttl}s"
                if entry_ttl == 60:
                    ttl_desc = "1min(0DTE)"
                elif entry_ttl == self.ttl_seconds:
                    ttl_desc = f"{self.ttl_seconds//60}min"
                
                logger.info(f"📦 Cache HIT for {symbol} (age: {age:.1f}s, TTL: {ttl_desc}, remaining: {remaining_ttl:.1f}s)")
                return cached_data['data']
            else:
                logger.info(f"⏰ Cache EXPIRED for {symbol} (age: {age:.1f}s > TTL: {entry_ttl}s)")
                # Remove expired entry
                del self.cache[symbol]
        else:
            logger.info(f"❌ Cache MISS for {symbol} (not in cache)")
        
        return None

    def set(self, symbol: str, data: List[Dict[str, Any]]):
        """Cache IVx data for a symbol with dynamic TTL based on expiration urgency."""
        # Check if any expiration is 0DTE (same day) and if any are expired
        has_0dte = False
        has_expired_0dte = False
        if data:
            from datetime import datetime
            today = datetime.now().date()
            for expiration in data:
                if 'expiration_date' in expiration:
                    try:
                        exp_date = datetime.strptime(expiration['expiration_date'], "%Y-%m-%d").date()
                        if exp_date == today:
                            has_0dte = True
                            # Check if this 0DTE is marked as expired
                            if expiration.get('is_expired', False):
                                has_expired_0dte = True
                                break
                    except:
                        continue
        
        # Dynamic TTL: 5 minutes for expired 0DTE, 1 minute for active 0DTE, 5 minutes for others
        if has_expired_0dte:
            dynamic_ttl = 300  # 5 minutes for expired 0DTE (stable values)
        elif has_0dte:
            dynamic_ttl = 60   # 1 minute for active 0DTE (changing values)
        else:
            dynamic_ttl = self.ttl_seconds  # 5 minutes for regular expirations
        
        self.cache[symbol] = {
            "data": data,
            "timestamp": time.time(),
            "ttl": dynamic_ttl  # Store the TTL used for this entry
        }
        
        ttl_desc = "5min(expired)" if has_expired_0dte else ("1min(0DTE)" if has_0dte else f"{self.ttl_seconds//60}min")
        logger.info(f"💾 Cached IVx data for {symbol} ({len(data)} expirations, TTL: {ttl_desc})")
    
    def get_cache_info(self) -> Dict[str, Any]:
        """Get information about current cache state."""
        current_time = time.time()
        cache_info = {}
        
        for symbol, cached_data in self.cache.items():
            age = current_time - cached_data['timestamp']
            entry_ttl = cached_data.get('ttl', self.ttl_seconds)
            remaining_ttl = max(0, entry_ttl - age)
            is_expired = age >= entry_ttl
            
            cache_info[symbol] = {
                "age_seconds": age,
                "remaining_ttl": remaining_ttl,
                "is_expired": is_expired,
                "expiration_count": len(cached_data['data']),
                "ttl_seconds": entry_ttl
            }
        
        return {
            "total_cached_symbols": len(self.cache),
            "default_ttl_seconds": self.ttl_seconds,
            "symbols": cache_info
        }

ivx_cache = IVxCache()
