package scope

import (
	"sync"
	"time"
)

type ScopeCache struct {
	mu    sync.RWMutex
	items map[string]cachedItem
	ttl   time.Duration
}

type cachedItem struct {
	decision  ScopeDecision
	expiresAt time.Time
}

func NewScopeCache() *ScopeCache {
	return &ScopeCache{
		items: make(map[string]cachedItem),
		ttl:   30 * time.Second,
	}
}

func NewScopeCacheWithTTL(ttl time.Duration) *ScopeCache {
	return &ScopeCache{
		items: make(map[string]cachedItem),
		ttl:   ttl,
	}
}

func (c *ScopeCache) Get(key string) (ScopeDecision, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return ScopeDecision{}, false
	}
	if time.Now().After(item.expiresAt) {
		return ScopeDecision{}, false
	}
	return item.decision, true
}

func (c *ScopeCache) Set(key string, decision ScopeDecision) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cachedItem{
		decision:  decision,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *ScopeCache) InvalidateSubject(subjectType ScopeSubjectType, subjectID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := string(subjectType) + "/" + subjectID + "/"
	for key := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.items, key)
		}
	}
}

func (c *ScopeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cachedItem)
}
