package workbenchterminal

import "time"

const maxLifecycleEvents = 64

type LifecycleEvent struct {
	Sequence uint64    `json:"sequence"`
	Kind     string    `json:"kind"`
	At       time.Time `json:"at"`
	ExitCode *int      `json:"exitCode,omitempty"`
	Rows     uint16    `json:"rows,omitempty"`
	Cols     uint16    `json:"cols,omitempty"`
}

type LifecyclePage struct {
	SessionID      string           `json:"sessionId"`
	TaskID         string           `json:"taskId,omitempty"`
	WorkspaceURI   string           `json:"workspaceUri,omitempty"`
	LatestSequence uint64           `json:"latestSequence"`
	Truncated      bool             `json:"truncated"`
	Events         []LifecycleEvent `json:"events"`
	Scope          string           `json:"scope"`
}

func (s *session) recordLifecycleLocked(event LifecycleEvent) {
	s.lifecycleSequence++
	event.Sequence = s.lifecycleSequence
	event.At = time.Now().UTC()
	if len(s.lifecycle) == maxLifecycleEvents {
		copy(s.lifecycle, s.lifecycle[1:])
		s.lifecycle = s.lifecycle[:maxLifecycleEvents-1]
	}
	s.lifecycle = append(s.lifecycle, event)
}

func (s *session) lifecyclePage(after uint64) (LifecyclePage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return LifecyclePage{}, ErrNotFound
	}
	s.lastUsed = time.Now()
	result := LifecyclePage{SessionID: s.id, TaskID: s.taskID, WorkspaceURI: s.workspaceURI, LatestSequence: s.lifecycleSequence, Events: []LifecycleEvent{}, Scope: "live-session-memory-only"}
	if len(s.lifecycle) > 0 && after < s.lifecycle[0].Sequence-1 {
		result.Truncated = true
	}
	for _, event := range s.lifecycle {
		if event.Sequence > after {
			result.Events = append(result.Events, event)
		}
	}
	return result, nil
}

func (m *Manager) Lifecycle(owner, id string, after uint64) (LifecyclePage, error) {
	s, err := m.get(owner, id)
	if err != nil {
		return LifecyclePage{}, err
	}
	return s.lifecyclePage(after)
}
