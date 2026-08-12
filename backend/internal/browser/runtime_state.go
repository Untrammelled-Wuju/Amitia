package browser

import (
	"sync"
	"sync/atomic"
)

type runtimeState struct {
	mu                sync.RWMutex
	state             BrowserRuntimeState
	generation        uint64
	startupInProgress sync.Once
	startupResult     *startupResult
}

type startupResult struct {
	info *BrowserRuntimeInfo
	err  *BrowserError
}

func newRuntimeState() *runtimeState {
	return &runtimeState{
		state:      BrowserRuntimeStopped,
		generation: 0,
	}
}

func (s *runtimeState) current() (BrowserRuntimeState, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.generation
}

func (s *runtimeState) setState(newState BrowserRuntimeState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isValidTransitionLocked(newState) {
		return false
	}
	s.state = newState
	return true
}

func (s *runtimeState) incrementGeneration() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	return s.generation
}

func (s *runtimeState) setStarting() bool {
	return s.setState(BrowserRuntimeStarting)
}

func (s *runtimeState) setReady() bool {
	return s.setState(BrowserRuntimeReady)
}

func (s *runtimeState) setStopping() bool {
	return s.setState(BrowserRuntimeStopping)
}

func (s *runtimeState) setStopped() bool {
	return s.setState(BrowserRuntimeStopped)
}

func (s *runtimeState) setFailed() bool {
	return s.setState(BrowserRuntimeFailed)
}

func (s *runtimeState) isValidTransitionLocked(target BrowserRuntimeState) bool {
	switch s.state {
	case BrowserRuntimeStopped:
		return target == BrowserRuntimeStarting
	case BrowserRuntimeStarting:
		return target == BrowserRuntimeReady || target == BrowserRuntimeFailed
	case BrowserRuntimeReady:
		return target == BrowserRuntimeStopping || target == BrowserRuntimeFailed
	case BrowserRuntimeStopping:
		return target == BrowserRuntimeStopped || target == BrowserRuntimeFailed
	case BrowserRuntimeFailed:
		return target == BrowserRuntimeStarting
	default:
		return false
	}
}

func (s *runtimeState) isReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == BrowserRuntimeReady
}

func (s *runtimeState) isStopped() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == BrowserRuntimeStopped
}

func (s *runtimeState) isFailed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == BrowserRuntimeFailed
}

func (s *runtimeState) tryStartupOnce() *sync.Once {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startupResult != nil {
		return nil
	}
	var once sync.Once
	s.startupInProgress = once
	return &s.startupInProgress
}

func (s *runtimeState) startupCompleted(info *BrowserRuntimeInfo, err *BrowserError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startupResult = &startupResult{info: info, err: err}
}

func (s *runtimeState) consumeStartupResult() (*BrowserRuntimeInfo, *BrowserError, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startupResult == nil {
		return nil, nil, false
	}
	result := s.startupResult
	s.startupResult = nil
	s.startupInProgress = sync.Once{}
	return result.info, result.err, true
}

type atomicCounter struct {
	value uint64
}

func (c *atomicCounter) next() uint64 {
	return atomic.AddUint64(&c.value, 1)
}

func (c *atomicCounter) get() uint64 {
	return atomic.LoadUint64(&c.value)
}
