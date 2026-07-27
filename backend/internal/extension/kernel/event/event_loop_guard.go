package event

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type LoopGuard struct {
	mu              sync.Mutex
	chains          map[string]*callChain
	maxDepth        int
	maxChainLength  int
	shortWindow     time.Duration
	maxPerWindow    int
}

type callChain struct {
	depth         int
	eventTypes    map[string][]time.Time
	contributions map[string][]time.Time
	originContrib string
	traceID       string
}

func NewLoopGuard(maxDepth, maxChainLength int) *LoopGuard {
	if maxDepth <= 0 {
		maxDepth = 8
	}
	if maxChainLength <= 0 {
		maxChainLength = 32
	}
	return &LoopGuard{
		chains:         make(map[string]*callChain),
		maxDepth:       maxDepth,
		maxChainLength: maxChainLength,
		shortWindow:    10 * time.Second,
		maxPerWindow:   10,
	}
}

func (g *LoopGuard) Enter(chainKey, eventTypeID, contributionID string, depth int, traceID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if depth > g.maxDepth {
		return fmt.Errorf("%w: depth %d exceeds max %d", ErrEventDepthExceeded, depth, g.maxDepth)
	}
	chain, ok := g.chains[chainKey]
	if !ok {
		chain = &callChain{
			eventTypes:    make(map[string][]time.Time),
			contributions: make(map[string][]time.Time),
			traceID:       traceID,
		}
		g.chains[chainKey] = chain
	}
	chain.depth = depth
	if contributionID != "" {
		chain.contributions[contributionID] = append(chain.contributions[contributionID], time.Now().UTC())
		chain.contributions[contributionID] = pruneOld(chain.contributions[contributionID], g.shortWindow)
		if len(chain.contributions[contributionID]) > g.maxPerWindow {
			return fmt.Errorf("%w: contribution %s exceeded %d calls in %v", ErrEventLoopDetected, contributionID, g.maxPerWindow, g.shortWindow)
		}
	}
	if eventTypeID != "" {
		chain.eventTypes[eventTypeID] = append(chain.eventTypes[eventTypeID], time.Now().UTC())
		chain.eventTypes[eventTypeID] = pruneOld(chain.eventTypes[eventTypeID], g.shortWindow)
		if len(chain.eventTypes[eventTypeID]) > g.maxPerWindow {
			return fmt.Errorf("%w: event type %s exceeded %d in %v", ErrEventLoopDetected, eventTypeID, g.maxPerWindow, g.shortWindow)
		}
	}
	return nil
}

func (g *LoopGuard) Exit(chainKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.chains, chainKey)
}

func (g *LoopGuard) CheckDuplicate(chainKey, idempotencyKey string, seen map[string]bool) bool {
	if idempotencyKey == "" {
		return false
	}
	key := chainKey + ":" + idempotencyKey
	if seen[key] {
		return true
	}
	seen[key] = true
	return false
}

func (g *LoopGuard) DetectCycle(chainKey string, eventTypeID, contributionID, aggregateID, idempotencyKey string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	chain, ok := g.chains[chainKey]
	if !ok {
		return nil
	}
	if contributionID != "" && eventTypeID != "" {
		if times, ok := chain.eventTypes[eventTypeID]; ok && len(times) >= 2 {
			if len(chain.contributions[contributionID]) >= 2 {
				return fmt.Errorf("%w: same contribution %s publishing %s repeatedly", ErrEventLoopDetected, contributionID, eventTypeID)
			}
		}
	}
	if aggregateID != "" && idempotencyKey != "" {
		if times, ok := chain.eventTypes[eventTypeID+":"+aggregateID+":"+idempotencyKey]; ok && len(times) >= 1 {
			return fmt.Errorf("%w: duplicate aggregate %s event %s idempotency %s", ErrEventLoopDetected, aggregateID, eventTypeID, idempotencyKey)
		}
	}
	return nil
}

func pruneOld(times []time.Time, window time.Duration) []time.Time {
	now := time.Now().UTC()
	cutoff := now.Add(-window)
	idx := 0
	for ; idx < len(times); idx++ {
		if times[idx].After(cutoff) {
			break
		}
	}
	if idx > 0 {
		times = times[idx:]
	}
	return times
}

func (g *LoopGuard) ChainDepth(chainKey string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	chain, ok := g.chains[chainKey]
	if !ok {
		return 0
	}
	return chain.depth
}

func (g *LoopGuard) Reset(chainKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.chains, chainKey)
}

func (g *LoopGuard) MaxDepth() int {
	return g.maxDepth
}

var _ = errors.New
