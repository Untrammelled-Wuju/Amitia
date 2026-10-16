package workbenchterminal

import (
	"sort"
	"time"
)

type SessionSummary struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"taskId,omitempty"`
	WorkspaceURI string    `json:"workspaceUri,omitempty"`
	Mode         string    `json:"mode"`
	Running      bool      `json:"running"`
	ExitCode     int       `json:"exitCode"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (m *Manager) List(owner string) []SessionSummary {
	if owner == "" {
		return []SessionSummary{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]SessionSummary, 0)
	for _, s := range m.sessions {
		if s.owner != owner {
			continue
		}
		s.mu.Lock()
		result = append(result, SessionSummary{
			ID: s.id, TaskID: s.taskID, WorkspaceURI: s.workspaceURI, Mode: s.proc.Mode(), Running: s.running,
			ExitCode: s.exitCode, CreatedAt: s.created,
		})
		s.mu.Unlock()
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}
