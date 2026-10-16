package workbenchterminal

import (
	"context"
	"errors"
	"time"
)

const bindingWatchInterval = 10 * time.Second
const bindingWatchTimeout = 3 * time.Second

type taskSessionRef struct {
	owner          string
	id             string
	workspaceURI   string
	taskID         string
	conversationID string
}

func (m *Manager) taskSessions() []taskSessionRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]taskSessionRef, 0)
	for _, session := range m.sessions {
		if session.taskID == "" {
			continue
		}
		result = append(result, taskSessionRef{
			owner: session.owner, id: session.id,
			workspaceURI: session.workspaceURI,
			taskID:       session.taskID, conversationID: session.conversationID,
		})
	}
	return result
}

func (h *Handler) checkTaskBindings(ctx context.Context) (int, int) {
	if h == nil || h.manager == nil || h.taskVerifier == nil {
		return 0, 0
	}
	checked, revoked := 0, 0
	for _, session := range h.manager.taskSessions() {
		if ctx.Err() != nil {
			break
		}
		checked++
		if session.conversationID == "" || !conversationPattern.MatchString(session.conversationID) ||
			!taskIDPattern.MatchString(session.taskID) {
			if h.manager.RevokeSession(session.owner, session.id) == nil {
				revoked++
			}
			continue
		}
		err := h.taskVerifier(ctx, session.workspaceURI, session.taskID, session.conversationID)
		if errors.Is(err, ErrTaskBinding) {
			if h.manager.RevokeSession(session.owner, session.id) == nil {
				revoked++
			}
		}
	}
	return checked, revoked
}

func (h *Handler) watchTaskBindings() {
	ticker := time.NewTicker(bindingWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-h.manager.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), bindingWatchTimeout)
			h.checkTaskBindings(ctx)
			cancel()
		}
	}
}
