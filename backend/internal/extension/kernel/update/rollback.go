package update

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type RollbackPoint struct {
	PointID           string
	ExtensionID       string
	Version           string
	GenerationID      string
	DefinitionHash    string
	PackageHash       string
	ArtifactPath      string
	StorageSnapshotID string
	ResourceGraph     ResourceGraph
	Permissions       []PermissionSnapshot
	ScopeReferences   []ScopeReference
	SignatureKeyID    string
	PublisherID       string
	CreatedAt         time.Time
	ExpiresAt         *time.Time
	UserPinned        bool
	ProtectionLevel   RollbackLevel
}

type ResourceGraph struct {
	Nodes []ResourceGraphNode
	Edges []ResourceGraphEdge
}

type ResourceGraphNode struct {
	ID    string
	Type  string
	Owner string
	Hash  string
}

type ResourceGraphEdge struct {
	From string
	To   string
	Type string
}

type ScopeReference struct {
	ScopeType string
	ScopeID   string
	Owner     string
}

type RollbackPointStore struct {
	mu     sync.RWMutex
	points map[string]RollbackPoint
	byExt  map[string][]string
}

func NewRollbackPointStore() *RollbackPointStore {
	return &RollbackPointStore{
		points: make(map[string]RollbackPoint),
		byExt:  make(map[string][]string),
	}
}

func (s *RollbackPointStore) Save(point RollbackPoint) error {
	if point.PointID == "" {
		return errors.New("update: point id required")
	}
	if point.ExtensionID == "" {
		return errors.New("update: extension id required")
	}
	if point.CreatedAt.IsZero() {
		point.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.points[point.PointID]; exists {
		return fmt.Errorf("update: rollback point %s already exists", point.PointID)
	}
	s.points[point.PointID] = point
	s.byExt[point.ExtensionID] = append(s.byExt[point.ExtensionID], point.PointID)
	return nil
}

func (s *RollbackPointStore) Get(ctx context.Context, pointID string) (*RollbackPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.points[pointID]
	if !ok {
		return nil, fmt.Errorf("update: rollback point %s not found", pointID)
	}
	result := p
	return &result, nil
}

func (s *RollbackPointStore) List(ctx context.Context, extensionID string) []RollbackPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byExt[extensionID]
	result := make([]RollbackPoint, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.points[id]; ok {
			if p.ExpiresAt == nil || p.ExpiresAt.After(time.Now().UTC()) {
				result = append(result, p)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *RollbackPointStore) Pin(ctx context.Context, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.points[pointID]
	if !ok {
		return fmt.Errorf("update: rollback point %s not found", pointID)
	}
	p.UserPinned = true
	s.points[pointID] = p
	return nil
}

func (s *RollbackPointStore) Unpin(ctx context.Context, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.points[pointID]
	if !ok {
		return fmt.Errorf("update: rollback point %s not found", pointID)
	}
	p.UserPinned = false
	s.points[pointID] = p
	return nil
}

func (s *RollbackPointStore) Delete(ctx context.Context, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.points[pointID]
	if !ok {
		return fmt.Errorf("update: rollback point %s not found", pointID)
	}
	if p.UserPinned {
		return errors.New("update: cannot delete pinned rollback point")
	}
	delete(s.points, pointID)
	ids := s.byExt[p.ExtensionID]
	for i, id := range ids {
		if id == pointID {
			s.byExt[p.ExtensionID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	return nil
}

type RetentionPolicy struct {
	MaxPoints        int
	MaxAge           time.Duration
	MaxDiskBytes     int64
	KeepCurrent      bool
	KeepLastN        int
	KeepIrreversible bool
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MaxPoints:        5,
		MaxAge:           30 * 24 * time.Hour,
		KeepCurrent:      true,
		KeepLastN:        2,
		KeepIrreversible: true,
	}
}

func (s *RollbackPointStore) ApplyRetentionPolicy(ctx context.Context, extensionID string, policy RetentionPolicy) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.byExt[extensionID]
	var all []RollbackPoint
	for _, id := range ids {
		if p, ok := s.points[id]; ok {
			all = append(all, p)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	var deleted []string
	kept := 0
	for i, p := range all {
		if policy.KeepCurrent && i == 0 {
			kept++
			continue
		}
		if policy.KeepLastN > 0 && i < policy.KeepLastN {
			kept++
			continue
		}
		if policy.KeepIrreversible && p.ProtectionLevel == RollbackLevelDataSnapshotRequired {
			kept++
			continue
		}
		if p.UserPinned {
			kept++
			continue
		}
		if policy.MaxAge > 0 && time.Since(p.CreatedAt) > policy.MaxAge {
			delete(s.points, p.PointID)
			deleted = append(deleted, p.PointID)
			continue
		}
		if policy.MaxPoints > 0 && kept >= policy.MaxPoints {
			delete(s.points, p.PointID)
			deleted = append(deleted, p.PointID)
			continue
		}
		kept++
	}

	if len(deleted) > 0 {
		var remaining []string
		for _, id := range ids {
			if _, exists := s.points[id]; exists {
				remaining = append(remaining, id)
			}
		}
		s.byExt[extensionID] = remaining
	}

	return deleted
}

type RollbackExecutor struct {
	points      *RollbackPointStore
	migrations  *MigrationExecutor
	generations *GenerationManager
}

func NewRollbackExecutor(points *RollbackPointStore, migrations *MigrationExecutor, generations *GenerationManager) *RollbackExecutor {
	return &RollbackExecutor{
		points:      points,
		migrations:  migrations,
		generations: generations,
	}
}

type RollbackRequest struct {
	ExtensionID string
	PointID     string
	Reason      string
	UserID      string
}

type RollbackResult struct {
	Success      bool
	PointID      string
	GenerationID string
	Reason       string
	RestoredData map[string][]byte
	Steps        []RollbackStep
}

type RollbackStep struct {
	Name   string
	Status string
	Error  string
}

func (e *RollbackExecutor) Execute(ctx context.Context, req RollbackRequest) RollbackResult {
	result := RollbackResult{PointID: req.PointID}

	point, err := e.points.Get(ctx, req.PointID)
	if err != nil {
		result.Reason = fmt.Sprintf("rollback point not found: %v", err)
		return result
	}

	if point.ExtensionID != req.ExtensionID {
		result.Reason = "extension id mismatch"
		return result
	}

	active := e.generations.Active(ctx, req.ExtensionID)
	if active != nil {
		if err := e.generations.Transition(ctx, req.ExtensionID, active.GenerationID, GenerationStateDraining); err != nil {
			result.Reason = fmt.Sprintf("drain active generation failed: %v", err)
			return result
		}
		result.Steps = append(result.Steps, RollbackStep{Name: "drain_active", Status: "succeeded"})
	}

	if point.StorageSnapshotID != "" {
		data, err := e.migrations.RestoreSnapshot(ctx, point.StorageSnapshotID)
		if err != nil {
			result.Reason = fmt.Sprintf("restore snapshot failed: %v", err)
			result.Steps = append(result.Steps, RollbackStep{Name: "restore_snapshot", Status: "failed", Error: err.Error()})
			return result
		}
		result.RestoredData = data
		result.Steps = append(result.Steps, RollbackStep{Name: "restore_snapshot", Status: "succeeded"})
	}

	rollbackGen, err := e.generations.RollbackTarget(ctx, req.ExtensionID)
	if err != nil {
		result.Reason = fmt.Sprintf("no rollback target: %v", err)
		return result
	}

	if err := e.generations.Reactivate(ctx, req.ExtensionID, rollbackGen.GenerationID); err != nil {
		result.Reason = fmt.Sprintf("activate rollback generation failed: %v", err)
		result.Steps = append(result.Steps, RollbackStep{Name: "activate_rollback", Status: "failed", Error: err.Error()})
		return result
	}

	if active != nil {
		e.generations.Transition(ctx, req.ExtensionID, active.GenerationID, GenerationStateStopped)
	}

	result.Success = true
	result.GenerationID = rollbackGen.GenerationID
	result.Steps = append(result.Steps, RollbackStep{Name: "activate_rollback", Status: "succeeded"})
	return result
}
