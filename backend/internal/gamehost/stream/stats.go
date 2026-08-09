package stream

import (
	"sync/atomic"
)

type StreamStats struct {
	Published  atomic.Uint64
	Delivered  atomic.Uint64
	Dropped    atomic.Uint64
	Rejected   atomic.Uint64
	Coalesced  atomic.Uint64
	Replayed   atomic.Uint64
	QueueDepth atomic.Int32
}

func (s *StreamStats) Snapshot() StreamStatsSnapshot {
	return StreamStatsSnapshot{
		Published:  s.Published.Load(),
		Delivered:  s.Delivered.Load(),
		Dropped:    s.Dropped.Load(),
		Rejected:   s.Rejected.Load(),
		Coalesced:  s.Coalesced.Load(),
		Replayed:   s.Replayed.Load(),
		QueueDepth: s.QueueDepth.Load(),
	}
}

func (s *StreamStats) Reset() {
	s.Published.Store(0)
	s.Delivered.Store(0)
	s.Dropped.Store(0)
	s.Rejected.Store(0)
	s.Coalesced.Store(0)
	s.Replayed.Store(0)
	s.QueueDepth.Store(0)
}

type StreamStatsSnapshot struct {
	Published  uint64
	Delivered  uint64
	Dropped    uint64
	Rejected   uint64
	Coalesced  uint64
	Replayed   uint64
	QueueDepth int32
}

func (s StreamStatsSnapshot) Total() uint64 {
	return s.Published + s.Dropped + s.Rejected + s.Coalesced
}
