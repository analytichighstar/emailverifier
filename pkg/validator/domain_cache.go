package validator

import (
	"context"
	"sync"
	"time"

	"emailvalidator/pkg/cache"
)

const domainCacheCleanupThreshold = 500

// domainValidationEntry stores cached DNS validation results for a domain.
type domainValidationEntry struct {
	domainExists bool
	hasMX        bool
	timestamp    time.Time
}

// DomainCacheResult is the structure stored in Redis cache
type DomainCacheResult struct {
	Exists bool `json:"exists"`
	HasMX  bool `json:"has_mx"`
}

// DomainCacheManager handles caching of domain validation results
type DomainCacheManager struct {
	localCache    map[string]domainValidationEntry
	cacheMutex    sync.RWMutex
	cacheDuration time.Duration
	redisCache    cache.Cache
}

// NewDomainCacheManager creates a new instance of DomainCacheManager with local cache only
func NewDomainCacheManager(duration time.Duration) *DomainCacheManager {
	return &DomainCacheManager{
		localCache:    make(map[string]domainValidationEntry, 100),
		cacheDuration: duration,
		redisCache:    nil,
	}
}

// NewDomainCacheManagerWithRedis creates a new instance of DomainCacheManager with Redis cache
func NewDomainCacheManagerWithRedis(duration time.Duration, redisCache cache.Cache) *DomainCacheManager {
	return &DomainCacheManager{
		localCache:    make(map[string]domainValidationEntry, 100),
		cacheDuration: duration,
		redisCache:    redisCache,
	}
}

// GetValidation retrieves cached domain existence and MX results.
func (m *DomainCacheManager) GetValidation(domain string) (exists, hasMX bool, ok bool) {
	m.cacheMutex.RLock()
	cached, found := m.localCache[domain]
	if found && time.Since(cached.timestamp) <= m.cacheDuration {
		m.cacheMutex.RUnlock()
		return cached.domainExists, cached.hasMX, true
	}
	m.cacheMutex.RUnlock()

	if m.redisCache != nil {
		var result DomainCacheResult
		err := m.redisCache.Get(context.Background(), "domain:"+domain, &result)
		if err == nil {
			m.cacheMutex.Lock()
			m.localCache[domain] = domainValidationEntry{
				domainExists: result.Exists,
				hasMX:        result.HasMX,
				timestamp:    time.Now(),
			}
			m.cacheMutex.Unlock()
			return result.Exists, result.HasMX, true
		}
	}

	return false, false, false
}

// SetValidation stores domain existence and MX results in cache.
func (m *DomainCacheManager) SetValidation(domain string, exists, hasMX bool) {
	m.cacheMutex.Lock()
	m.localCache[domain] = domainValidationEntry{
		domainExists: exists,
		hasMX:        hasMX,
		timestamp:    time.Now(),
	}
	shouldCleanup := len(m.localCache) > domainCacheCleanupThreshold
	m.cacheMutex.Unlock()

	if m.redisCache != nil {
		result := DomainCacheResult{Exists: exists, HasMX: hasMX}
		_ = m.redisCache.Set(context.Background(), "domain:"+domain, result, m.cacheDuration)
	}

	if shouldCleanup {
		m.ClearExpired()
	}
}

// Get retrieves a cached domain existence result for backward compatibility.
func (m *DomainCacheManager) Get(domain string) (bool, bool) {
	exists, _, ok := m.GetValidation(domain)
	return exists, ok
}

// Set stores a domain existence result for backward compatibility.
func (m *DomainCacheManager) Set(domain string, exists bool) {
	hasMX, _, ok := m.GetValidation(domain)
	if !ok {
		hasMX = false
	}
	m.SetValidation(domain, exists, hasMX)
}

// ClearExpired removes expired entries from the local cache.
func (m *DomainCacheManager) ClearExpired() {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	now := time.Now()
	for domain, cached := range m.localCache {
		if now.Sub(cached.timestamp) > m.cacheDuration {
			delete(m.localCache, domain)
		}
	}
}

// SetDuration updates the cache duration
func (m *DomainCacheManager) SetDuration(duration time.Duration) {
	m.cacheMutex.Lock()
	m.cacheDuration = duration
	m.cacheMutex.Unlock()
}

// SetRedisCache sets the Redis cache backend
func (m *DomainCacheManager) SetRedisCache(redisCache cache.Cache) {
	m.redisCache = redisCache
}

// HasRedis returns true if Redis cache is configured
func (m *DomainCacheManager) HasRedis() bool {
	return m.redisCache != nil
}

// Close closes the Redis connection if available
func (m *DomainCacheManager) Close() error {
	if m.redisCache != nil {
		return m.redisCache.Close()
	}
	return nil
}
