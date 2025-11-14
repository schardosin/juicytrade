package ivx

import (
	"log"
	"time"

	"trade-backend-go/internal/models"
)

// Cache handles IVx data caching with dynamic TTL based on expiration urgency
type Cache struct {
	cache map[string]*cacheEntry
}

// cacheEntry represents a cached IVx data entry with metadata
type cacheEntry struct {
	data      []models.IVxExpiration
	timestamp time.Time
	ttl       time.Duration
}

// NewCache creates a new IVx cache instance
func NewCache() *Cache {
	return &Cache{
		cache: make(map[string]*cacheEntry),
	}
}

// Get retrieves cached IVx data for a symbol if available and not expired
func (c *Cache) Get(symbol string) []models.IVxExpiration {
	entry, exists := c.cache[symbol]
	if !exists {
		log.Printf("IVx Cache MISS for %s (not in cache)", symbol)
		return nil
	}

	age := time.Since(entry.timestamp)
	remainingTTL := entry.ttl - age

	// Check if cache is still valid
	if age < entry.ttl {
		ttlDesc := c.getTTLDescription(entry.ttl)
		log.Printf("IVx Cache HIT for %s (age: %.1fs, TTL: %s, remaining: %.1fs)", symbol, age.Seconds(), ttlDesc, remainingTTL.Seconds())
		return entry.data
	}

	// Cache expired
	ttlDesc := c.getTTLDescription(entry.ttl)
	log.Printf("IVx Cache EXPIRED for %s (age: %.1fs > TTL: %s)", symbol, age.Seconds(), ttlDesc)

	// Remove expired entry
	delete(c.cache, symbol)
	return nil
}

// Set caches IVx data for a symbol with dynamic TTL based on expiration urgency
func (c *Cache) Set(symbol string, data []models.IVxExpiration) {
	if len(data) == 0 {
		return
	}

	// Determine dynamic TTL based on expiration types
	dynamicTTL := c.calculateDynamicTTL(data)

	c.cache[symbol] = &cacheEntry{
		data:      data,
		timestamp: time.Now(),
		ttl:       dynamicTTL,
	}

	ttlDesc := c.getTTLDescription(dynamicTTL)
	log.Printf("IVx Cache SET for %s (%d expirations, TTL: %s)", symbol, len(data), ttlDesc)
}

// calculateDynamicTTL determines the appropriate TTL based on expiration data
func (c *Cache) calculateDynamicTTL(data []models.IVxExpiration) time.Duration {
	// Check if any expiration is 0DTE (same day) and if any are expired
	has0DTE := false
	hasExpired0DTE := false

	for _, expiration := range data {
		// Check if this is a 0DTE expiration (days to expiration <= 1 and not marked as expired)
		if expiration.DaysToExpiration <= 1.0 && !expiration.IsExpired {
			has0DTE = true
		}

		// Check if this is an expired 0DTE
		if expiration.IsExpired && expiration.DaysToExpiration <= 0.0001 {
			hasExpired0DTE = true
			break
		}
	}

	// Dynamic TTL: 5 minutes for expired 0DTE, 1 minute for active 0DTE, 5 minutes for others
	if hasExpired0DTE {
		return 5 * time.Minute // 5 minutes for expired 0DTE (stable values)
	} else if has0DTE {
		return 1 * time.Minute // 1 minute for active 0DTE (changing values)
	} else {
		return 5 * time.Minute // 5 minutes for regular expirations
	}
}

// getTTLDescription returns a human-readable description of the TTL
func (c *Cache) getTTLDescription(ttl time.Duration) string {
	switch ttl {
	case 1 * time.Minute:
		return "1min(0DTE)"
	case 5 * time.Minute:
		return "5min"
	default:
		return ttl.String()
	}
}

// GetCacheInfo returns information about current cache state for debugging
func (c *Cache) GetCacheInfo() map[string]interface{} {
	currentTime := time.Now()
	cacheInfo := make(map[string]interface{})

	totalSymbols := 0
	totalExpirations := 0

	for symbol, entry := range c.cache {
		age := currentTime.Sub(entry.timestamp)
		remainingTTL := entry.ttl - age
		isExpired := age >= entry.ttl

		cacheInfo[symbol] = map[string]interface{}{
			"age_seconds":       age.Seconds(),
			"remaining_ttl":     remainingTTL.Seconds(),
			"is_expired":        isExpired,
			"expiration_count":  len(entry.data),
			"ttl_seconds":       entry.ttl.Seconds(),
			"ttl_description":   c.getTTLDescription(entry.ttl),
		}

		totalSymbols++
		totalExpirations += len(entry.data)
	}

	return map[string]interface{}{
		"total_cached_symbols": totalSymbols,
		"total_expirations":    totalExpirations,
		"default_ttl_seconds":  (5 * time.Minute).Seconds(),
		"symbols":             cacheInfo,
	}
}

// Clear removes all cached data
func (c *Cache) Clear() {
	c.cache = make(map[string]*cacheEntry)
	log.Printf("IVx Cache cleared - all data removed")
}

// Size returns the number of cached symbols
func (c *Cache) Size() int {
	return len(c.cache)
}

// IsExpired checks if a symbol's cache entry is expired without retrieving the data
func (c *Cache) IsExpired(symbol string) bool {
	entry, exists := c.cache[symbol]
	if !exists {
		return true
	}

	age := time.Since(entry.timestamp)
	return age >= entry.ttl
}

// Remove deletes a specific symbol from the cache
func (c *Cache) Remove(symbol string) {
	if _, exists := c.cache[symbol]; exists {
		delete(c.cache, symbol)
		log.Printf("IVx Cache removed entry for %s", symbol)
	}
}

// Cleanup removes all expired entries from the cache
func (c *Cache) Cleanup() {
	currentTime := time.Now()
	expiredSymbols := []string{}

	for symbol, entry := range c.cache {
		age := currentTime.Sub(entry.timestamp)
		if age >= entry.ttl {
			expiredSymbols = append(expiredSymbols, symbol)
		}
	}

	for _, symbol := range expiredSymbols {
		delete(c.cache, symbol)
	}

	if len(expiredSymbols) > 0 {
		log.Printf("IVx Cache cleanup removed %d expired entries", len(expiredSymbols))
	}
}
