package stream

import (
	"sync"
	"sync/atomic"
)

type Sequence uint64

const SequenceZero Sequence = 0

type sequenceGenerator struct {
	current atomic.Uint64
}

func newSequenceGenerator() *sequenceGenerator {
	return &sequenceGenerator{}
}

func (g *sequenceGenerator) Next() Sequence {
	return Sequence(g.current.Add(1))
}

func (g *sequenceGenerator) Current() Sequence {
	return Sequence(g.current.Load())
}

func (g *sequenceGenerator) Reset() {
	g.current.Store(0)
}

type sequenceManager struct {
	mu        sync.RWMutex
	generators map[string]*sequenceGenerator
}

func newSequenceManager() *sequenceManager {
	return &sequenceManager{
		generators: make(map[string]*sequenceGenerator),
	}
}

func (m *sequenceManager) Next(key string) Sequence {
	m.mu.RLock()
	gen, ok := m.generators[key]
	m.mu.RUnlock()
	if ok {
		return gen.Next()
	}
	m.mu.Lock()
	gen, ok = m.generators[key]
	if !ok {
		gen = newSequenceGenerator()
		m.generators[key] = gen
	}
	m.mu.Unlock()
	return gen.Next()
}

func (m *sequenceManager) Current(key string) Sequence {
	m.mu.RLock()
	gen, ok := m.generators[key]
	m.mu.RUnlock()
	if !ok {
		return SequenceZero
	}
	return gen.Current()
}

func (m *sequenceManager) Reset(key string) {
	m.mu.RLock()
	gen, ok := m.generators[key]
	m.mu.RUnlock()
	if ok {
		gen.Reset()
	}
}

func (m *sequenceManager) Remove(key string) {
	m.mu.Lock()
	delete(m.generators, key)
	m.mu.Unlock()
}

func streamKey(runtimeID string, serviceID string, channelID string) string {
	return runtimeID + "/" + serviceID + "/" + channelID
}
