package workbenchterminal

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

const exportLimit = bufferCapacity

type Transcript struct {
	ID                      string           `json:"id"`
	TaskID                  string           `json:"taskId,omitempty"`
	WorkspaceURI            string           `json:"workspaceUri,omitempty"`
	Mode                    string           `json:"mode"`
	Running                 bool             `json:"running"`
	ExitCode                int              `json:"exitCode"`
	Data                    string           `json:"data"`
	BaseCursor              int64            `json:"baseCursor"`
	EndCursor               int64            `json:"endCursor"`
	Truncated               bool             `json:"truncated"`
	CapturedAt              time.Time        `json:"capturedAt"`
	StartedAt               time.Time        `json:"startedAt"`
	OutputBytes             int              `json:"outputBytes"`
	OutputDigest            string           `json:"outputDigest"`
	DigestScope             string           `json:"digestScope"`
	Lifecycle               []LifecycleEvent `json:"lifecycle"`
	LifecycleLatestSequence uint64           `json:"lifecycleLatestSequence"`
	LifecycleTruncated      bool             `json:"lifecycleTruncated"`
}

func (m *Manager) Export(owner, id string) (Transcript, error) {
	s, err := m.get(owner, id)
	if err != nil {
		return Transcript{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return Transcript{}, ErrNotFound
	}
	s.lastUsed = time.Now()
	data := s.output.data
	if len(data) > exportLimit {
		data = data[len(data)-exportLimit:]
	}
	base := s.output.base + int64(len(s.output.data)-len(data))
	digest := sha256.Sum256(data)
	lifecycle := make([]LifecycleEvent, len(s.lifecycle))
	copy(lifecycle, s.lifecycle)
	lifecycleTruncated := len(lifecycle) > 0 && lifecycle[0].Sequence > 1
	return Transcript{Lifecycle: lifecycle, LifecycleLatestSequence: s.lifecycleSequence, LifecycleTruncated: lifecycleTruncated, CapturedAt: time.Now().UTC(), StartedAt: s.created.UTC(), OutputBytes: len(data), OutputDigest: hex.EncodeToString(digest[:]), DigestScope: "retained-bytes-only", ID: s.id, TaskID: s.taskID, WorkspaceURI: s.workspaceURI, Mode: s.proc.Mode(), Running: s.running, ExitCode: s.exitCode, Data: base64.StdEncoding.EncodeToString(data), BaseCursor: base, EndCursor: base + int64(len(data)), Truncated: base > 0}, nil
}
