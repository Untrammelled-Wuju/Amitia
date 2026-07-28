package permission

import (
	"context"
	"sync"
	"time"
)

type cacheEntry struct {
	grants    []PermissionGrant
	expiresAt time.Time
}

type PermissionCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	done    chan struct{}
	closed  bool
}

func NewPermissionCache(ttl time.Duration) *PermissionCache {
	c := &PermissionCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		done:    make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

func (c *PermissionCache) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	close(c.done)
}

func (c *PermissionCache) cacheKey(subject PermissionSubject, permissionID string) string {
	return string(subject.Type) + ":" + subject.ID + ":" + permissionID
}

func (c *PermissionCache) GetOrLoad(ctx context.Context, subject PermissionSubject, permissionID string, loader func() []PermissionGrant) []PermissionGrant {
	key := c.cacheKey(subject, permissionID)

	c.mu.RLock()
	if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.grants
	}
	c.mu.RUnlock()

	grants := loader()

	c.mu.Lock()
	c.entries[key] = &cacheEntry{
		grants:    grants,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return grants
}

func (c *PermissionCache) Invalidate(subject PermissionSubject, permissionID string) {
	key := c.cacheKey(subject, permissionID)
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *PermissionCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

func (c *PermissionCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.entries {
				if now.After(v.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
