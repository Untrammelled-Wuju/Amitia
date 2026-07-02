package queue

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

type CacheKey struct {
	Scope         string
	ConfigVersion string
	DataVersion   string
}

type CacheEntry struct {
	Key         CacheKey
	Value       interface{}
	Authority   int
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastAccess  time.Time
	HitCount    int64
	Stable      bool
}

type CacheConfig struct {
	MaxEntries     int
	DefaultTTL     time.Duration
	HighAuthorityTTL time.Duration
	StableTTL      time.Duration
	UserStateTTL   time.Duration
}

type CacheMetrics struct {
	mu               sync.Mutex
	TotalHits         int64
	TotalMisses       int64
	TotalInvalidations int64
	InvalidationReasons map[string]int64
}

type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	config  CacheConfig
	metrics *CacheMetrics
}

var DefaultCacheConfig = CacheConfig{
	MaxEntries:      500,
	DefaultTTL:      5 * time.Minute,
	HighAuthorityTTL: 30 * time.Minute,
	StableTTL:       1 * time.Hour,
	UserStateTTL:    30 * time.Second,
}

func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		InvalidationReasons: make(map[string]int64),
	}
}

func NewCache(cfg CacheConfig) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg = DefaultCacheConfig
	}
	return &Cache{
		entries: make(map[string]*CacheEntry),
		config:  cfg,
		metrics: NewCacheMetrics(),
	}
}

func (c *Cache) cacheKeyString(key CacheKey) string {
	raw := fmt.Sprintf("%s|%s|%s", key.Scope, key.ConfigVersion, key.DataVersion)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:32]
}

func (c *Cache) Get(scope, configVersion, dataVersion string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := CacheKey{Scope: scope, ConfigVersion: configVersion, DataVersion: dataVersion}
	cacheStr := c.cacheKeyString(key)

	entry, ok := c.entries[cacheStr]
	if !ok {
		c.metrics.TotalMisses++
		return nil, false
	}

	now := time.Now().UTC()
	if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
		delete(c.entries, cacheStr)
		c.metrics.TotalMisses++
		c.metrics.TotalInvalidations++
		c.metrics.InvalidationReasons["ttl_expired"]++
		return nil, false
	}

	entry.LastAccess = now
	entry.HitCount++
	c.metrics.TotalHits++
	return entry.Value, true
}

func (c *Cache) Set(scope, configVersion, dataVersion string, value interface{}, authority int, stable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := CacheKey{Scope: scope, ConfigVersion: configVersion, DataVersion: dataVersion}
	cacheStr := c.cacheKeyString(key)

	now := time.Now().UTC()
	var ttl time.Duration

	if stable && authority >= 5 {
		ttl = c.config.StableTTL
	} else if authority >= 3 {
		ttl = c.config.HighAuthorityTTL
	} else if c.isUserStateScope(scope) {
		ttl = c.config.UserStateTTL
	} else {
		ttl = c.config.DefaultTTL
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Authority:  authority,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		LastAccess: now,
		Stable:     stable,
	}

	if len(c.entries) >= c.config.MaxEntries {
		c.evictOldestLocked()
	}

	c.entries[cacheStr] = entry
}

func (c *Cache) Invalidate(scope, configVersion, dataVersion, reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := CacheKey{Scope: scope, ConfigVersion: configVersion, DataVersion: dataVersion}
	cacheStr := c.cacheKeyString(key)

	_, ok := c.entries[cacheStr]
	if !ok {
		return 0
	}

	delete(c.entries, cacheStr)
	c.metrics.TotalInvalidations++
	if reason != "" {
		c.metrics.InvalidationReasons[reason]++
	} else {
		c.metrics.InvalidationReasons["manual"]++
	}
	return 1
}

func (c *Cache) InvalidateByScopePrefix(scopePrefix, reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for k, entry := range c.entries {
		if strings.HasPrefix(entry.Key.Scope, scopePrefix) {
			delete(c.entries, k)
			count++
		}
	}

	c.metrics.TotalInvalidations += int64(count)
	if reason != "" {
		c.metrics.InvalidationReasons[reason] += int64(count)
	} else {
		c.metrics.InvalidationReasons["prefix_invalidation"] += int64(count)
	}
	return count
}

func (c *Cache) Metrics() *CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := &CacheMetrics{
		TotalHits:          c.metrics.TotalHits,
		TotalMisses:        c.metrics.TotalMisses,
		TotalInvalidations: c.metrics.TotalInvalidations,
		InvalidationReasons: make(map[string]int64),
	}
	for k, v := range c.metrics.InvalidationReasons {
		snapshot.InvalidationReasons[k] = v
	}
	return snapshot
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time

	for k, entry := range c.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = k
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.metrics.TotalInvalidations++
		c.metrics.InvalidationReasons["eviction"]++
	}
}

func (c *Cache) isUserStateScope(scope string) bool {
	userStateScopes := []string{"user:", "current:", "active:", "session:"}
	for _, prefix := range userStateScopes {
		if strings.HasPrefix(strings.ToLower(scope), prefix) {
			return true
		}
	}
	return false
}

func (c *Cache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.metrics.TotalHits + c.metrics.TotalMisses
	if total == 0 {
		return 0
	}
	return float64(c.metrics.TotalHits) / float64(total)
}
