package stream

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type streamStreamKey struct {
	runtimeID domain.RuntimeInstanceID
	serviceID domain.ServiceID
	channelID domain.ChannelID
}

func (k streamStreamKey) String() string {
	return string(k.runtimeID) + "/" + string(k.serviceID) + "/" + string(k.channelID)
}

func newStreamGeneration() StreamGeneration {
	return StreamGeneration("gen_" + uuid.New().String())
}

type streamState struct {
	key           streamStreamKey
	policy        StreamPolicy
	generation    StreamGeneration
	replay        *ReplayBuffer
	sequence      Sequence
	overflow      *OverflowHandler
	rateLimiter   *rateLimiter
	overflowQueue *BoundedQueue
	mu            sync.RWMutex
	closed        bool
	stats         StreamStats
}

type StreamManager struct {
	mu        sync.RWMutex
	streams   map[string]*streamState
	sequences *sequenceManager
	policy    PolicyResolver
	shutdown  bool
	clock     func() time.Time
}

func NewStreamManager(resolver PolicyResolver) *StreamManager {
	return &StreamManager{
		streams:   make(map[string]*streamState),
		sequences: newSequenceManager(),
		policy:    resolver,
		clock:     time.Now,
	}
}

func NewStreamManagerWithClock(resolver PolicyResolver, clock func() time.Time) *StreamManager {
	sm := NewStreamManager(resolver)
	sm.clock = clock
	return sm
}

func (sm *StreamManager) streamLookup(keyStr string) (*streamState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	ss, ok := sm.streams[keyStr]
	return ss, ok
}

func (sm *StreamManager) getOrCreateStream(input PolicyInput, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) (*streamState, error) {
	key := streamStreamKey{runtimeID: runtimeID, serviceID: serviceID, channelID: channelID}
	keyStr := key.String()

	ss, ok := sm.streamLookup(keyStr)
	if ok {
		return ss, nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if ss, ok := sm.streams[keyStr]; ok {
		return ss, nil
	}

	policy := sm.policy.Resolve(input)
	generation := newStreamGeneration()
	ss = &streamState{
		key:        key,
		policy:     policy,
		generation: generation,
		replay:     NewReplayBuffer(policy.ReplayCapacity),
		overflow:   NewOverflowHandler(policy.Overflow),
	}
	if policy.RateLimit.Enabled() {
		ss.rateLimiter = newRateLimiter(policy.RateLimit)
	}
	sm.streams[keyStr] = ss
	return ss, nil
}

func (sm *StreamManager) GetStream(keyStr string) (*streamState, bool) {
	return sm.streamLookup(keyStr)
}

func (sm *StreamManager) GetStreamByKeys(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) (*streamState, bool) {
	key := streamStreamKey{runtimeID: runtimeID, serviceID: serviceID, channelID: channelID}
	return sm.streamLookup(key.String())
}

func (sm *StreamManager) Publish(ctx context.Context, input PolicyInput, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(payload) == 0 {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream: payload must not be empty")
	}

	ss, err := sm.getOrCreateStream(input, runtimeID, serviceID, channelID)
	if err != nil {
		return err
	}

	if ss.rateLimiter != nil && !ss.rateLimiter.Allow() {
		ss.stats.Rejected.Add(1)
		return ErrRateLimited
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.closed {
		return ErrStreamClosed
	}

	seq := sm.sequences.Next(ss.key.String())
	entry := QueueEntry{
		Sequence:  seq,
		Payload:   bytesClone(payload),
		Size:      int64(len(payload)),
		CreatedAt: sm.clock().UnixNano(),
	}

	if ss.policy.Overflow == OverflowCoalesce {
		coalesceKey := string(runtimeID) + "/" + string(serviceID) + "/" + string(channelID)
		if ss.overflowQueue == nil {
			ss.overflowQueue = NewBoundedQueue(ss.policy.QueueCapacity)
		}
		coalesced := ss.overflowQueue.Coalesce(QueueEntry{Sequence: seq, Payload: entry.Payload, Size: entry.Size, CreatedAt: entry.CreatedAt}, coalesceKey)
		if coalesced {
			ss.stats.Coalesced.Add(1)
		}
		ss.sequence = seq
		ss.replay.Append(entry)
		ss.stats.Published.Add(1)
		return nil
	}

	if ss.overflowQueue == nil {
		ss.overflowQueue = NewBoundedQueue(ss.policy.QueueCapacity)
	}

	result := ss.computeOverflowLocked(entry)
	if result.Action == OverflowActionReject || result.Action == OverflowActionDropNewest {
		ss.stats.Rejected.Add(1)
		return ErrQueueFull
	}
	if result.Action == OverflowActionDropOldest {
		ss.stats.Dropped.Add(1)
	}

	ss.sequence = seq
	ss.replay.Append(entry)
	ss.stats.Published.Add(1)
	return nil
}

func (ss *streamState) computeOverflowLocked(entry QueueEntry) OverflowResult {
	switch ss.policy.Overflow {
	case OverflowReject:
		if ss.overflowQueue.Len() >= ss.policy.QueueCapacity {
			return OverflowResult{Action: OverflowActionReject, Entry: entry}
		}
		ss.overflowQueue.Push(entry)
		return OverflowResult{Action: OverflowActionNone, Entry: entry}
	case OverflowDropOldest:
		if ss.overflowQueue.Len() >= ss.policy.QueueCapacity {
			ss.overflowQueue.PopOldest()
			ss.overflowQueue.Push(entry)
			return OverflowResult{Action: OverflowActionDropOldest, Entry: entry}
		}
		ss.overflowQueue.Push(entry)
		return OverflowResult{Action: OverflowActionNone, Entry: entry}
	case OverflowDropNewest:
		if ss.overflowQueue.Len() >= ss.policy.QueueCapacity {
			return OverflowResult{Action: OverflowActionDropNewest, Entry: entry}
		}
		ss.overflowQueue.Push(entry)
		return OverflowResult{Action: OverflowActionNone, Entry: entry}
	case OverflowCoalesce:
		ss.overflowQueue.Push(entry)
		return OverflowResult{Action: OverflowActionCoalesce, Entry: entry}
	case OverflowBlock:
		ss.overflowQueue.Push(entry)
		return OverflowResult{Action: OverflowActionNone, Entry: entry}
	default:
		return OverflowResult{Action: OverflowActionReject, Entry: entry}
	}
}

func (sm *StreamManager) GetSequence(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) Sequence {
	return sm.sequences.Current(streamStreamKey{runtimeID: runtimeID, serviceID: serviceID, channelID: channelID}.String())
}

func (sm *StreamManager) GetReplayBuffer(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) *ReplayBuffer {
	ss, ok := sm.GetStreamByKeys(runtimeID, serviceID, channelID)
	if !ok {
		return nil
	}
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.replay
}

func (sm *StreamManager) GetGeneration(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) StreamGeneration {
	ss, ok := sm.GetStreamByKeys(runtimeID, serviceID, channelID)
	if !ok {
		return StreamGenerationZero
	}
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.generation
}

func (sm *StreamManager) RemoveStream(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) bool {
	key := streamStreamKey{runtimeID: runtimeID, serviceID: serviceID, channelID: channelID}
	keyStr := key.String()

	sm.mu.Lock()
	defer sm.mu.Unlock()
	ss, ok := sm.streams[keyStr]
	if !ok {
		return false
	}
	ss.mu.Lock()
	ss.closed = true
	ss.mu.Unlock()
	delete(sm.streams, keyStr)
	sm.sequences.Remove(keyStr)
	return true
}

func (sm *StreamManager) RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	for keyStr, ss := range sm.streams {
		if ss.key.runtimeID == runtimeID {
			ss.mu.Lock()
			ss.closed = true
			ss.mu.Unlock()
			delete(sm.streams, keyStr)
			sm.sequences.Remove(keyStr)
			count++
		}
	}
	return count
}

func (sm *StreamManager) RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	for keyStr, ss := range sm.streams {
		if ss.key.runtimeID == runtimeID && ss.key.serviceID == serviceID {
			ss.mu.Lock()
			ss.closed = true
			ss.mu.Unlock()
			delete(sm.streams, keyStr)
			sm.sequences.Remove(keyStr)
			count++
		}
	}
	return count
}

func (sm *StreamManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.shutdown = true
	for _, ss := range sm.streams {
		ss.mu.Lock()
		ss.closed = true
		ss.mu.Unlock()
	}
	return nil
}

func (sm *StreamManager) IsShutdown() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.shutdown
}

func (sm *StreamManager) StatsFor(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) StreamStatsSnapshot {
	ss, ok := sm.GetStreamByKeys(runtimeID, serviceID, channelID)
	if !ok {
		return StreamStatsSnapshot{}
	}
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.stats.Snapshot()
}
