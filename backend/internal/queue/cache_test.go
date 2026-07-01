package queue

import (
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})
	if c == nil {
		t.Fatal("expected non-nil Cache")
	}
	if c.Size() != 0 {
		t.Fatalf("expected size 0, got %d", c.Size())
	}
}

func TestCacheSetGet(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("scope1", "v1", "d1", "value1", 5, true)

	val, ok := c.Get("scope1", "v1", "d1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %v", val)
	}
}

func TestCacheMiss(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	val, ok := c.Get("nonexistent", "v1", "d1")
	if ok {
		t.Fatal("expected cache miss")
	}
	if val != nil {
		t.Fatalf("expected nil value, got %v", val)
	}
}

func TestCacheDifferentVersions(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("scope1", "v1", "d1", "value1", 5, false)
	c.Set("scope1", "v2", "d1", "value2", 5, false)
	c.Set("scope1", "v1", "d2", "value3", 5, false)

	val, ok := c.Get("scope1", "v1", "d1")
	if !ok || val != "value1" {
		t.Fatalf("expected value1, got %v, ok=%v", val, ok)
	}

	val, ok = c.Get("scope1", "v2", "d1")
	if !ok || val != "value2" {
		t.Fatalf("expected value2, got %v, ok=%v", val, ok)
	}

	val, ok = c.Get("scope1", "v1", "d2")
	if !ok || val != "value3" {
		t.Fatalf("expected value3, got %v, ok=%v", val, ok)
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("scope1", "v1", "d1", "value1", 5, false)
	count := c.Invalidate("scope1", "v1", "d1", "test_reason")
	if count != 1 {
		t.Fatalf("expected 1 invalidation, got %d", count)
	}

	val, ok := c.Get("scope1", "v1", "d1")
	if ok {
		t.Fatalf("expected cache miss after invalidation, got %v", val)
	}
}

func TestCacheInvalidateByScopePrefix(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("user:1:profile", "v1", "d1", "val1", 3, false)
	c.Set("user:1:settings", "v1", "d1", "val2", 3, false)
	c.Set("user:2:profile", "v1", "d1", "val3", 3, false)

	count := c.InvalidateByScopePrefix("user:1:", "user_change")
	if count != 2 {
		t.Fatalf("expected 2 invalidations, got %d", count)
	}

	_, ok := c.Get("user:1:profile", "v1", "d1")
	if ok {
		t.Fatal("expected miss for user:1:profile")
	}

	_, ok = c.Get("user:2:profile", "v1", "d1")
	if !ok {
		t.Fatal("expected hit for user:2:profile")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cfg := CacheConfig{
		MaxEntries: 100,
		DefaultTTL: 50 * time.Millisecond,
	}
	c := NewCache(cfg)

	c.Set("scope1", "v1", "d1", "value1", 1, false)

	time.Sleep(100 * time.Millisecond)

	val, ok := c.Get("scope1", "v1", "d1")
	if ok {
		t.Fatalf("expected cache miss due to TTL, got %v", val)
	}
}

func TestCacheUserStateTTL(t *testing.T) {
	cfg := CacheConfig{
		MaxEntries:   100,
		DefaultTTL:   1 * time.Hour,
		UserStateTTL: 50 * time.Millisecond,
	}
	c := NewCache(cfg)

	c.Set("user:state", "v1", "d1", "state1", 1, false)

	time.Sleep(100 * time.Millisecond)

	val, ok := c.Get("user:state", "v1", "d1")
	if ok {
		t.Fatalf("expected cache miss for user state, got %v", val)
	}
}

func TestCacheHighAuthorityTTL(t *testing.T) {
	cfg := CacheConfig{
		MaxEntries:       100,
		DefaultTTL:       50 * time.Millisecond,
		HighAuthorityTTL: 500 * time.Millisecond,
	}
	c := NewCache(cfg)

	c.Set("stable:context", "v1", "d1", "ctx1", 5, false)

	time.Sleep(100 * time.Millisecond)

	val, ok := c.Get("stable:context", "v1", "d1")
	if !ok {
		t.Fatal("expected cache hit for high authority entry")
	}
	_ = val
}

func TestCacheStableTTL(t *testing.T) {
	cfg := CacheConfig{
		MaxEntries: 100,
		DefaultTTL: 50 * time.Millisecond,
		StableTTL:  500 * time.Millisecond,
	}
	c := NewCache(cfg)

	c.Set("stable:ctx", "v1", "d1", "ctx1", 5, true)

	time.Sleep(100 * time.Millisecond)

	val, ok := c.Get("stable:ctx", "v1", "d1")
	if !ok {
		t.Fatal("expected cache hit for stable entry")
	}
	_ = val
}

func TestCacheEviction(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 3})

	c.Set("s1", "v1", "d1", "val1", 1, false)
	c.Set("s2", "v1", "d1", "val2", 1, false)
	c.Set("s3", "v1", "d1", "val3", 1, false)
	c.Set("s4", "v1", "d1", "val4", 1, false)

	if c.Size() != 3 {
		t.Fatalf("expected max 3 entries, got %d", c.Size())
	}
}

func TestCacheHitRate(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("s1", "v1", "d1", "val1", 5, false)

	c.Get("s1", "v1", "d1")
	c.Get("s1", "v1", "d1")
	c.Get("missing", "v1", "d1")

	rate := c.HitRate()
	if rate != 2.0/3.0 {
		t.Fatalf("expected hit rate 2/3, got %f", rate)
	}
}

func TestCacheMetrics(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100})

	c.Set("s1", "v1", "d1", "val1", 5, false)
	c.Get("s1", "v1", "d1")
	c.Invalidate("s1", "v1", "d1", "manual")

	m := c.Metrics()
	if m.TotalHits != 1 {
		t.Fatalf("expected 1 hit, got %d", m.TotalHits)
	}
	if m.TotalInvalidations != 1 {
		t.Fatalf("expected 1 invalidation, got %d", m.TotalInvalidations)
	}
}
