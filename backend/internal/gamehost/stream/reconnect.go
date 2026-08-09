package stream

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ResumeResult struct {
	Replayed int
	Entries  []QueueEntry
	Latest   Sequence
	Generation StreamGeneration
}

type ReconnectManager struct {
	manager *StreamManager
}

func NewReconnectManager(sm *StreamManager) *ReconnectManager {
	return &ReconnectManager{manager: sm}
}

func (rm *ReconnectManager) Resume(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	channelID domain.ChannelID,
	cursor Cursor,
) (ResumeResult, error) {
	if ctx.Err() != nil {
		return ResumeResult{}, ctx.Err()
	}

	if err := cursor.Validate(); err != nil {
		return ResumeResult{}, err
	}

	ss, ok := rm.manager.GetStreamByKeys(runtimeID, serviceID, channelID)
	if !ok {
		return ResumeResult{}, ErrStreamClosed
	}

	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if ss.closed {
		return ResumeResult{}, ErrStreamClosed
	}

	if cursor.Generation != ss.generation {
		return ResumeResult{}, ErrGenerationMismatch
	}

	if cursor.Sequence > ss.sequence {
		return ResumeResult{}, ErrCursorAhead
	}

	policy := ss.policy
	switch policy.Resume {
	case ResumeNone:
		return ResumeResult{Latest: ss.sequence, Generation: ss.generation}, nil
	case ResumeLatest:
		return ResumeResult{Latest: ss.sequence, Generation: ss.generation}, nil
	case ResumeBoundedReplay:
		return rm.replayFromCursor(ss, cursor, policy)
	default:
		return rm.replayFromCursor(ss, cursor, policy)
	}
}

func (rm *ReconnectManager) replayFromCursor(ss *streamState, cursor Cursor, policy StreamPolicy) (ResumeResult, error) {
	latestSeq := ss.sequence
	if cursor.Sequence >= latestSeq {
		return ResumeResult{Latest: latestSeq, Generation: ss.generation}, nil
	}

	if policy.ReplayCapacity == 0 {
		return ResumeResult{}, ErrCursorStale
	}

	entries, err := ss.replay.Replay(cursor.Sequence)
	if err != nil {
		if err == ErrCursorStale {
			return ResumeResult{}, ErrCursorStale
		}
		return ResumeResult{}, err
	}

	return ResumeResult{
		Replayed:   len(entries),
		Entries:    entries,
		Latest:     ss.sequence,
		Generation: ss.generation,
	}, nil
}

func (rm *ReconnectManager) ValidateCursorForStream(
	ss *streamState,
	cursor Cursor,
) error {
	if ss.closed {
		return ErrStreamClosed
	}
	if cursor.Generation != ss.generation {
		return ErrGenerationMismatch
	}
	if cursor.Sequence > ss.sequence {
		return ErrCursorAhead
	}
	return nil
}

func (rm *ReconnectManager) LatestSequence(
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	channelID domain.ChannelID,
) Sequence {
	return rm.manager.GetSequence(runtimeID, serviceID, channelID)
}

func (rm *ReconnectManager) StreamGeneration(
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	channelID domain.ChannelID,
) StreamGeneration {
	return rm.manager.GetGeneration(runtimeID, serviceID, channelID)
}
