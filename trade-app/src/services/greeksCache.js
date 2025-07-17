/**
 * Greeks Cache System for Options Trading
 *
 * Provides intelligent caching for options Greeks data with:
 * - TTL-based expiration
 * - Batch loading optimization
 * - Memory-efficient storage
 * - Performance tracking
 */

class OptionsGreeksCache {
  constructor(options = {}) {
    this.cache = new Map();
    this.ttl = options.ttl || 5 * 60 * 1000; // 5 minutes default
    this.maxSize = options.maxSize || 1000; // Max cached items
    this.hitCount = 0;
    this.missCount = 0;
    this.loadCount = 0;

    // Cleanup expired entries every minute
    this.cleanupInterval = setInterval(() => {
      this.cleanup();
    }, 60 * 1000);
  }

  /**
   * Set Greeks data for an option symbol
   */
  set(symbol, greeks) {
    // Enforce size limit using LRU eviction
    if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value;
      this.cache.delete(firstKey);
    }

    this.cache.set(symbol, {
      data: greeks,
      timestamp: Date.now(),
    });
  }

  /**
   * Get Greeks data for an option symbol
   */
  get(symbol) {
    const entry = this.cache.get(symbol);

    if (!entry) {
      this.missCount++;
      return null;
    }

    // Check if expired
    if (Date.now() - entry.timestamp > this.ttl) {
      this.cache.delete(symbol);
      this.missCount++;
      return null;
    }

    // Move to end (LRU)
    this.cache.delete(symbol);
    this.cache.set(symbol, entry);
    this.hitCount++;

    return entry.data;
  }

  /**
   * Get Greeks for multiple symbols, returning cached and missing separately
   */
  getBatch(symbols) {
    const cached = {};
    const missing = [];

    symbols.forEach((symbol) => {
      const greeks = this.get(symbol);
      if (greeks) {
        cached[symbol] = greeks;
      } else {
        missing.push(symbol);
      }
    });

    return { cached, missing };
  }

  /**
   * Set Greeks for multiple symbols
   */
  setBatch(greeksData) {
    Object.entries(greeksData).forEach(([symbol, greeks]) => {
      this.set(symbol, greeks);
    });
    this.loadCount += Object.keys(greeksData).length;
  }

  /**
   * Check if symbol exists in cache and is not expired
   */
  has(symbol) {
    return this.get(symbol) !== null;
  }

  /**
   * Remove expired entries
   */
  cleanup() {
    const now = Date.now();
    const expired = [];

    for (const [symbol, entry] of this.cache.entries()) {
      if (now - entry.timestamp > this.ttl) {
        expired.push(symbol);
      }
    }

    expired.forEach((symbol) => {
      this.cache.delete(symbol);
    });

    if (expired.length > 0) {
      console.log(
        `🧹 Greeks cache: Cleaned up ${expired.length} expired entries`
      );
    }
  }

  /**
   * Clear all cached data
   */
  clear() {
    this.cache.clear();
    this.hitCount = 0;
    this.missCount = 0;
    this.loadCount = 0;
  }

  /**
   * Get cache statistics
   */
  getStats() {
    const totalRequests = this.hitCount + this.missCount;
    const hitRate =
      totalRequests > 0
        ? ((this.hitCount / totalRequests) * 100).toFixed(1)
        : 0;

    return {
      size: this.cache.size,
      maxSize: this.maxSize,
      hitCount: this.hitCount,
      missCount: this.missCount,
      loadCount: this.loadCount,
      hitRate: `${hitRate}%`,
      memoryUsage: this.getMemoryUsage(),
    };
  }

  /**
   * Estimate memory usage (rough calculation)
   */
  getMemoryUsage() {
    let totalSize = 0;

    for (const [symbol, entry] of this.cache.entries()) {
      // Rough estimate: symbol string + data object + timestamp
      totalSize += symbol.length * 2; // UTF-16 characters
      totalSize += JSON.stringify(entry.data).length * 2;
      totalSize += 8; // timestamp number
    }

    // Convert to KB
    return `${(totalSize / 1024).toFixed(1)} KB`;
  }

  /**
   * Get cache entries for debugging
   */
  getEntries() {
    const entries = {};
    for (const [symbol, entry] of this.cache.entries()) {
      entries[symbol] = {
        ...entry.data,
        cached_at: new Date(entry.timestamp).toISOString(),
        age_ms: Date.now() - entry.timestamp,
      };
    }
    return entries;
  }

  /**
   * Destroy the cache and cleanup resources
   */
  destroy() {
    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval);
      this.cleanupInterval = null;
    }
    this.clear();
  }
}

// Create singleton instance
export const greeksCache = new OptionsGreeksCache({
  ttl: 5 * 60 * 1000, // 5 minutes
  maxSize: 1000, // Max 1000 cached options
});

// Performance tracking utilities
export class PerformanceTracker {
  constructor() {
    this.metrics = {
      basicLoadTime: [],
      greeksLoadTime: [],
      totalLoadTime: [],
      cacheHitRate: 0,
    };
  }

  startTimer(operation) {
    return performance.now();
  }

  endTimer(operation, startTime) {
    const duration = performance.now() - startTime;

    if (!this.metrics[`${operation}LoadTime`]) {
      this.metrics[`${operation}LoadTime`] = [];
    }

    this.metrics[`${operation}LoadTime`].push(duration);

    // Keep only last 10 measurements
    if (this.metrics[`${operation}LoadTime`].length > 10) {
      this.metrics[`${operation}LoadTime`].shift();
    }

    console.log(`⏱️ ${operation} took ${duration.toFixed(2)}ms`);
    return duration;
  }

  getAverageLoadTime(operation) {
    const times = this.metrics[`${operation}LoadTime`] || [];
    if (times.length === 0) return 0;
    return times.reduce((a, b) => a + b, 0) / times.length;
  }

  getMetrics() {
    return {
      basicLoadAvg: `${this.getAverageLoadTime("basic").toFixed(1)}ms`,
      greeksLoadAvg: `${this.getAverageLoadTime("greeks").toFixed(1)}ms`,
      totalLoadAvg: `${this.getAverageLoadTime("total").toFixed(1)}ms`,
      cacheStats: greeksCache.getStats(),
    };
  }

  logPerformanceReport() {
    const metrics = this.getMetrics();
    console.log("📊 Performance Report:", metrics);
  }
}

// Create singleton performance tracker
export const performanceTracker = new PerformanceTracker();

// Utility function to log cache stats periodically
let statsInterval = null;

export function startPerformanceMonitoring(intervalMs = 30000) {
  if (statsInterval) {
    clearInterval(statsInterval);
  }

  statsInterval = setInterval(() => {
    performanceTracker.logPerformanceReport();
  }, intervalMs);
}

export function stopPerformanceMonitoring() {
  if (statsInterval) {
    clearInterval(statsInterval);
    statsInterval = null;
  }
}

export default greeksCache;
