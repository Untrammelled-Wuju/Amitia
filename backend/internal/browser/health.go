package browser

import (
	"context"
	"sync"
	"time"
)

type HealthProber interface {
	Probe(ctx context.Context) BrowserRuntimeHealth
}

type engineHealthProber struct {
	engine BrowserEngine
	mu     sync.Mutex
	cache  healthCache
}

type healthCache struct {
	result    BrowserRuntimeHealth
	timestamp time.Time
	ttl       time.Duration
}

func NewBrowserHealthProber(engine BrowserEngine) HealthProber {
	return &engineHealthProber{
		engine: engine,
		cache: healthCache{
			ttl: 1 * time.Second,
		},
	}
}

func (p *engineHealthProber) Probe(ctx context.Context) BrowserRuntimeHealth {
	p.mu.Lock()
	defer p.mu.Unlock()

	if time.Since(p.cache.timestamp) < p.cache.ttl {
		return p.cache.result
	}

	result := p.engine.Health(ctx)
	p.cache.result = result
	p.cache.timestamp = time.Now()
	return result
}

func MapRuntimeHealth(state BrowserRuntimeState, processAlive, cdpConnected, pingOK bool) BrowserRuntimeHealth {
	switch state {
	case BrowserRuntimeStopped:
		return BrowserHealthUnavailable
	case BrowserRuntimeStarting:
		return BrowserHealthStarting
	case BrowserRuntimeStopping:
		return BrowserHealthUnhealthy
	case BrowserRuntimeFailed:
		return BrowserHealthUnhealthy
	case BrowserRuntimeReady:
		if !processAlive || !cdpConnected || !pingOK {
			return BrowserHealthUnhealthy
		}
		return BrowserHealthHealthy
	default:
		return BrowserRuntimeHealth(BrowserHealthUnknown)
	}
}
