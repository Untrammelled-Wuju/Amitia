package sandbox_webui

import (
	"sync"
	"time"
)

type rateBucket struct {
	timestamps []time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rateBucket),
	}
}

func methodLimit(method BridgeMethod) (int, time.Duration) {
	switch method {
	case MethodReady, MethodSessionPing, MethodContextGet:
		return MaxBridgeCallsPerSec, time.Second
	case MethodDataRequest, MethodDataSubscribe:
		return MaxDataQueriesPerMin, time.Minute
	case MethodActionInvoke:
		return MaxActionsPerMin, time.Minute
	case MethodResize:
		return MaxResizePerMinute, time.Minute
	case MethodLog:
		return MaxLogPerSec, time.Second
	case MethodResourceOpen, MethodResourceRead:
		return MaxResourcePerMin, time.Minute
	case MethodArtifactCreate:
		return MaxActionsPerMin, time.Minute
	default:
		return MaxBridgeCallsPerSec, time.Second
	}
}

func (rl *RateLimiter) Allow(sessionID string, method BridgeMethod) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limit, window := methodLimit(method)
	key := sessionID + ":" + string(method)

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &rateBucket{timestamps: make([]time.Time, 0, limit+1)}
		rl.buckets[key] = bucket
	}

	now := time.Now().UTC()
	cutoff := now.Add(-window)
	pruned := bucket.timestamps[:0]
	for _, ts := range bucket.timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	bucket.timestamps = pruned

	if len(bucket.timestamps) >= limit {
		return false
	}

	bucket.timestamps = append(bucket.timestamps, now)
	return true
}

func (rl *RateLimiter) Reset(sessionID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key := range rl.buckets {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			delete(rl.buckets, key)
		}
	}
}
