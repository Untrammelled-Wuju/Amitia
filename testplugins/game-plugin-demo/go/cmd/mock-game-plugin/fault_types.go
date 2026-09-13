package main

import (
	"sync"
	"time"
)

type FaultState struct {
	HandshakeDelay    time.Duration
	HandshakeDropCnt  int
	HeartbeatDropCnt  int
	HeartbeatPaused   bool
	HeartbeatPauseEnd time.Time
	HeartbeatUnpause  chan struct{}
	RPCDropCnt        int
	QueueSlowCnt      int
	QueueSlowDelay    time.Duration
	UpgradePending    bool
	UpgradeDeadline   time.Time
	IgnoreShutdown    bool
	IgnoreShutdownEnd time.Time
	LastEmergencyOpID string
	ActiveConns       int
	OpenChannels      int
}

type FaultBarrier interface {
	Wait(version int64)
	Notify(version int64)
	Reset()
}

type faultBarrier struct {
	mu       sync.Mutex
	cond     *sync.Cond
	versions map[int64]struct{}
}

func NewFaultBarrier() FaultBarrier {
	fb := &faultBarrier{
		versions: make(map[int64]struct{}),
	}
	fb.cond = sync.NewCond(&fb.mu)
	return fb
}

func (b *faultBarrier) Wait(version int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for {
		if _, exists := b.versions[version]; exists {
			return
		}
		b.cond.Wait()
	}
}

func (b *faultBarrier) Notify(version int64) {
	b.mu.Lock()
	b.versions[version] = struct{}{}
	b.mu.Unlock()
	b.cond.Broadcast()
}

func (b *faultBarrier) Reset() {
	b.mu.Lock()
	b.versions = make(map[int64]struct{})
	b.mu.Unlock()
}

func NewFaultState() *FaultState {
	return &FaultState{
		HeartbeatUnpause: make(chan struct{}, 1),
	}
}

func (s *FaultState) Reset() {
	s.HandshakeDelay = 0
	s.HandshakeDropCnt = 0
	s.HeartbeatDropCnt = 0
	s.HeartbeatPaused = false
	s.HeartbeatPauseEnd = time.Time{}
	{
		select {
		case <-s.HeartbeatUnpause:
		default:
		}
	}
	s.HeartbeatUnpause = make(chan struct{}, 1)
	s.RPCDropCnt = 0
	s.QueueSlowCnt = 0
	s.QueueSlowDelay = 0
	s.UpgradePending = false
	s.UpgradeDeadline = time.Time{}
	s.IgnoreShutdown = false
	s.IgnoreShutdownEnd = time.Time{}
	s.LastEmergencyOpID = ""
	s.ActiveConns = 0
	s.OpenChannels = 0
}

func (s *FaultState) IsHeartbeatPaused(now time.Time) bool {
	if s.HeartbeatPaused {
		return true
	}
	if !s.HeartbeatPauseEnd.IsZero() && now.Before(s.HeartbeatPauseEnd) {
		return true
	}
	return false
}

func (s *FaultState) IsIgnoringShutdown(now time.Time) bool {
	if s.IgnoreShutdown {
		return true
	}
	if !s.IgnoreShutdownEnd.IsZero() && now.Before(s.IgnoreShutdownEnd) {
		return true
	}
	return false
}
