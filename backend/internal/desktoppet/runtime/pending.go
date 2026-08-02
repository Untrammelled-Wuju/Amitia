// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type PendingResult struct {
	CommandID      string
	Status         contracts.ResultStatus
	ErrorCode      string
	ErrorMsg       string
	AppliedRev     int64
	ActualState    *contracts.PetInstanceSummary
	AcceptedAction string
	PlaybackReqID  string
	Err            error
}

type waiter struct {
	commandID string
	sessionID string
	ch        chan *PendingResult
	deadline  time.Time
}

type PendingTracker struct {
	mu      sync.Mutex
	waiters map[string]*waiter
}

func NewPendingTracker() *PendingTracker {
	return &PendingTracker{
		waiters: make(map[string]*waiter),
	}
}

func (t *PendingTracker) Register(commandID, sessionID string, timeout time.Duration) (<-chan *PendingResult, context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan *PendingResult, 1)
	w := &waiter{
		commandID: commandID,
		sessionID: sessionID,
		ch:        ch,
		deadline:  time.Now().Add(timeout),
	}
	t.waiters[commandID] = w

	cancel := func() {
		t.Cancel(commandID, "cancelled by caller")
	}

	return ch, cancel
}

func (t *PendingTracker) Complete(result *PendingResult) bool {
	t.mu.Lock()
	w, ok := t.waiters[result.CommandID]
	if ok {
		delete(t.waiters, result.CommandID)
	}
	t.mu.Unlock()

	if !ok {
		return false
	}

	select {
	case w.ch <- result:
	default:
	}

	close(w.ch)
	return true
}

func (t *PendingTracker) Cancel(commandID, reason string) {
	t.mu.Lock()
	w, ok := t.waiters[commandID]
	if ok {
		delete(t.waiters, commandID)
	}
	t.mu.Unlock()

	if !ok {
		return
	}

	result := &PendingResult{
		CommandID: commandID,
		Status:    contracts.ResultCancelled,
		ErrorMsg:  reason,
		Err:       ErrRuntimeDisconnected,
	}

	select {
	case w.ch <- result:
	default:
	}

	close(w.ch)
}

func (t *PendingTracker) FailForSession(sessionID string, status contracts.ResultStatus, errorCode, errorMsg string, err error) int {
	t.mu.Lock()
	var toRemove []*waiter
	for _, w := range t.waiters {
		if w.sessionID == sessionID {
			toRemove = append(toRemove, w)
			delete(t.waiters, w.commandID)
		}
	}
	t.mu.Unlock()

	for _, w := range toRemove {
		result := &PendingResult{
			CommandID: w.commandID,
			Status:    status,
			ErrorCode: errorCode,
			ErrorMsg:  errorMsg,
			Err:       err,
		}
		select {
		case w.ch <- result:
		default:
		}
		close(w.ch)
	}

	return len(toRemove)
}

func (t *PendingTracker) FailAll(status contracts.ResultStatus, errorCode, errorMsg string, err error) int {
	t.mu.Lock()
	all := make([]*waiter, 0, len(t.waiters))
	for _, w := range t.waiters {
		all = append(all, w)
	}
	t.waiters = make(map[string]*waiter)
	t.mu.Unlock()

	for _, w := range all {
		result := &PendingResult{
			CommandID: w.commandID,
			Status:    status,
			ErrorCode: errorCode,
			ErrorMsg:  errorMsg,
			Err:       err,
		}
		select {
		case w.ch <- result:
		default:
		}
		close(w.ch)
	}

	return len(all)
}

func (t *PendingTracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.waiters)
}

func (t *PendingTracker) CleanupExpired() int {
	t.mu.Lock()
	var expired []*waiter
	now := time.Now()
	for id, w := range t.waiters {
		if now.After(w.deadline) {
			expired = append(expired, w)
			delete(t.waiters, id)
		}
	}
	t.mu.Unlock()

	for _, w := range expired {
		result := &PendingResult{
			CommandID: w.commandID,
			Status:    contracts.ResultExpired,
			ErrorCode: ErrCodeRuntimeCommandTimeout,
			ErrorMsg:  "command deadline exceeded",
			Err:       ErrRuntimeCommandTimeout,
		}
		select {
		case w.ch <- result:
		default:
		}
		close(w.ch)
	}

	return len(expired)
}
