package imageprovider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ExecutionGate struct {
	mu          sync.Mutex
	concurrency map[string]chan struct{}
	rateLimit   map[string]*rateLimiter
	maxPerKey   int
	ratePerSec  int
}

type rateLimiter struct {
	tokens chan struct{}
	refill chan struct{}
	stopCh chan struct{}
}

func NewExecutionGate(maxPerKey, ratePerSec int) *ExecutionGate {
	return &ExecutionGate{
		concurrency: make(map[string]chan struct{}),
		rateLimit:   make(map[string]*rateLimiter),
		maxPerKey:   maxPerKey,
		ratePerSec:  ratePerSec,
	}
}

func (g *ExecutionGate) Acquire(ctx context.Context, key string) error {
	g.mu.Lock()
	sem, exists := g.concurrency[key]
	if !exists {
		limit := g.maxPerKey
		if limit <= 0 {
			limit = 2
		}
		sem = make(chan struct{}, limit)
		g.concurrency[key] = sem

		if g.ratePerSec > 0 {
			rl := &rateLimiter{
				tokens: make(chan struct{}, g.ratePerSec),
				refill: make(chan struct{}, 1),
				stopCh: make(chan struct{}),
			}
			g.rateLimit[key] = rl
			for i := 0; i < g.ratePerSec; i++ {
				rl.tokens <- struct{}{}
			}
			go g.refillLoop(rl)
		}
	}
	g.mu.Unlock()

	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	if rl, ok := g.rateLimit[key]; ok {
		select {
		case <-rl.tokens:
		case <-ctx.Done():
			<-sem
			return ctx.Err()
		}
	}

	return nil
}

func (g *ExecutionGate) Release(key string) {
	g.mu.Lock()
	sem, exists := g.concurrency[key]
	g.mu.Unlock()
	if !exists {
		return
	}
	select {
	case <-sem:
	default:
	}
}

func (g *ExecutionGate) refillLoop(rl *rateLimiter) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			for i := 0; i < cap(rl.tokens); i++ {
				select {
				case rl.tokens <- struct{}{}:
				default:
					break
				}
			}
		}
	}
}

func (g *ExecutionGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, rl := range g.rateLimit {
		close(rl.stopCh)
		delete(g.rateLimit, key)
	}
	for key := range g.concurrency {
		delete(g.concurrency, key)
	}
}

func (g *ExecutionGate) Stats() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()
	stats := make(map[string]int)
	for key, sem := range g.concurrency {
		stats[fmt.Sprintf("concurrent_%s", key)] = len(sem)
	}
	return stats
}
