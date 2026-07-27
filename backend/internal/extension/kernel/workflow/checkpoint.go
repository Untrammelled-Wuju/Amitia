package workflow

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Checkpoint struct {
	WorkflowID  string          `json:"workflowId"`
	ExecutionID string          `json:"executionId"`
	NodeID      string          `json:"nodeId"`
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output"`
	CompletedAt time.Time       `json:"completedAt"`
}

type CheckpointStore interface {
	Save(ctx context.Context, cp Checkpoint) error
	Load(ctx context.Context, executionID, nodeID string) (*Checkpoint, error)
	List(ctx context.Context, executionID string) ([]Checkpoint, error)
	Delete(ctx context.Context, executionID string) error
}

type MemoryCheckpointStore struct {
	mu   sync.RWMutex
	data map[string]map[string]Checkpoint
}

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{
		data: make(map[string]map[string]Checkpoint),
	}
}

func (m *MemoryCheckpointStore) Save(_ context.Context, cp Checkpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[cp.ExecutionID] == nil {
		m.data[cp.ExecutionID] = make(map[string]Checkpoint)
	}
	m.data[cp.ExecutionID][cp.NodeID] = cp
	return nil
}

func (m *MemoryCheckpointStore) Load(_ context.Context, executionID, nodeID string) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes, ok := m.data[executionID]
	if !ok {
		return nil, ErrCheckpointNotFound
	}
	cp, ok := nodes[nodeID]
	if !ok {
		return nil, ErrCheckpointNotFound
	}
	return &cp, nil
}

func (m *MemoryCheckpointStore) List(_ context.Context, executionID string) ([]Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes, ok := m.data[executionID]
	if !ok {
		return nil, nil
	}
	result := make([]Checkpoint, 0, len(nodes))
	for _, cp := range nodes {
		result = append(result, cp)
	}
	return result, nil
}

func (m *MemoryCheckpointStore) Delete(_ context.Context, executionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, executionID)
	return nil
}
