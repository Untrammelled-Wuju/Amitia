package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type LegacyEntityType string

const (
	LegacyEntityPackage     LegacyEntityType = "package"
	LegacyEntitySkill       LegacyEntityType = "skill"
	LegacyEntityAgentSkill  LegacyEntityType = "agent_skill"
	LegacyEntityPlugin      LegacyEntityType = "plugin"
	LegacyEntityMCPServer   LegacyEntityType = "mcp_server"
	LegacyEntityMCPTool     LegacyEntityType = "mcp_tool"
	LegacyEntityWorkflow    LegacyEntityType = "workflow"
	LegacyEntityWorkflowRun LegacyEntityType = "workflow_run"
	LegacyEntitySchedule    LegacyEntityType = "schedule"
	LegacyEntityPermission  LegacyEntityType = "permission"
	LegacyEntityScope       LegacyEntityType = "scope"
	LegacyEntityRun         LegacyEntityType = "run"
	LegacyEntityAudit       LegacyEntityType = "audit"
	LegacyEntityResource    LegacyEntityType = "resource"
	LegacyEntityArtifact    LegacyEntityType = "artifact"
)

type LegacySnapshotRequest struct {
	IncludeEntities []LegacyEntityType
	IncludeArtifacts bool
	IncludeHistory   bool
	Labels           map[string]string
}

type LegacySnapshot struct {
	SnapshotID         string
	SchemaVersion      string
	ApplicationVersion string
	CreatedAt          time.Time
	SourceDatabaseID   string
	DataRevision       string
	ArtifactRevision   string
	EntityCounts       map[string]int64
	IntegrityHash      string
	Status             string
	Labels             map[string]string
	ExpiresAt          *time.Time
}

type LegacyEntityQuery struct {
	EntityType  LegacyEntityType
	Limit       int
	Offset      int
	Filter      map[string]string
	OrderBy     string
	OrderDesc   bool
}

type LegacyEntityPage struct {
	Items     []LegacyEntityRecord
	Total     int64
	Limit     int
	Offset    int
	HasMore   bool
}

type LegacyEntityRecord struct {
	EntityType   LegacyEntityType
	LegacyID     string
	CanonicalID  string
	RawData      json.RawMessage
	Hash         string
	Source       string
	UpdatedAt    time.Time
	Metadata     map[string]any
}

type LegacyArtifactRecord struct {
	ArtifactRef string
	Size        int64
	Hash        string
	ModTime     time.Time
	Readable    bool
	Missing     bool
	Corrupted   bool
	Metadata    map[string]any
}

type LegacyMigrationStatistics struct {
	SnapshotID    string
	EntityCounts  map[LegacyEntityType]int64
	ArtifactCount int64
	MissingArtifacts int64
	CorruptedArtifacts int64
	GeneratedAt   time.Time
}

type LegacyReadOnlyGateway interface {
	CreateSnapshot(ctx context.Context, request LegacySnapshotRequest) (LegacySnapshot, error)
	ListEntities(ctx context.Context, snapshotID string, query LegacyEntityQuery) (LegacyEntityPage, error)
	GetEntity(ctx context.Context, snapshotID string, entityType LegacyEntityType, legacyID string) (LegacyEntityRecord, error)
	ReadArtifact(ctx context.Context, snapshotID string, artifactRef string) (LegacyArtifactRecord, error)
	GetStatistics(ctx context.Context, snapshotID string) (LegacyMigrationStatistics, error)
	ListSnapshots(ctx context.Context) ([]LegacySnapshot, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error
}

var (
	ErrSnapshotNotFound   = errors.New("migration: snapshot not found")
	ErrSnapshotExpired    = errors.New("migration: snapshot expired")
	ErrEntityNotFound     = errors.New("migration: entity not found")
	ErrArtifactMissing    = errors.New("migration: artifact missing")
	ErrWriteForbidden     = errors.New("migration: write forbidden on read-only gateway")
	ErrSnapshotAlreadyExists = errors.New("migration: snapshot already exists")
)

type SnapshotStore interface {
	Put(ctx context.Context, snapshot LegacySnapshot) error
	Get(ctx context.Context, snapshotID string) (LegacySnapshot, error)
	List(ctx context.Context) ([]LegacySnapshot, error)
	Delete(ctx context.Context, snapshotID string) error
}

type EntityStore interface {
	Put(ctx context.Context, snapshotID string, record LegacyEntityRecord) error
	List(ctx context.Context, snapshotID string, query LegacyEntityQuery) (LegacyEntityPage, error)
	Get(ctx context.Context, snapshotID string, entityType LegacyEntityType, legacyID string) (LegacyEntityRecord, error)
	Count(ctx context.Context, snapshotID string, entityType LegacyEntityType) (int64, error)
}

type ArtifactStore interface {
	Register(ctx context.Context, snapshotID string, record LegacyArtifactRecord) error
	Get(ctx context.Context, snapshotID string, artifactRef string) (LegacyArtifactRecord, error)
}

type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]LegacySnapshot
}

func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{snapshots: make(map[string]LegacySnapshot)}
}

func (s *InMemorySnapshotStore) Put(_ context.Context, snapshot LegacySnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.SnapshotID] = snapshot
	return nil
}

func (s *InMemorySnapshotStore) Get(_ context.Context, snapshotID string) (LegacySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[snapshotID]
	if !ok {
		return LegacySnapshot{}, ErrSnapshotNotFound
	}
	if snap.ExpiresAt != nil && time.Now().After(*snap.ExpiresAt) {
		return LegacySnapshot{}, ErrSnapshotExpired
	}
	return snap, nil
}

func (s *InMemorySnapshotStore) List(_ context.Context) ([]LegacySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LegacySnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *InMemorySnapshotStore) Delete(_ context.Context, snapshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, snapshotID)
	return nil
}

type InMemoryEntityStore struct {
	mu       sync.RWMutex
	entities map[string][]LegacyEntityRecord
}

func NewInMemoryEntityStore() *InMemoryEntityStore {
	return &InMemoryEntityStore{entities: make(map[string][]LegacyEntityRecord)}
}

func entityKey(snapshotID string, entityType LegacyEntityType) string {
	return snapshotID + "|" + string(entityType)
}

func (s *InMemoryEntityStore) Put(_ context.Context, snapshotID string, record LegacyEntityRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := entityKey(snapshotID, record.EntityType)
	s.entities[key] = append(s.entities[key], record)
	return nil
}

func (s *InMemoryEntityStore) List(_ context.Context, snapshotID string, query LegacyEntityQuery) (LegacyEntityPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := entityKey(snapshotID, query.EntityType)
	items := append([]LegacyEntityRecord{}, s.entities[key]...)
	total := int64(len(items))
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if query.Offset > len(items) {
		query.Offset = len(items)
	}
	end := query.Offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := LegacyEntityPage{
		Items:   items[query.Offset:end],
		Total:   total,
		Limit:   limit,
		Offset:  query.Offset,
		HasMore: end < len(items),
	}
	return page, nil
}

func (s *InMemoryEntityStore) Get(_ context.Context, snapshotID string, entityType LegacyEntityType, legacyID string) (LegacyEntityRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := entityKey(snapshotID, entityType)
	for _, rec := range s.entities[key] {
		if rec.LegacyID == legacyID {
			return rec, nil
		}
	}
	return LegacyEntityRecord{}, ErrEntityNotFound
}

func (s *InMemoryEntityStore) Count(_ context.Context, snapshotID string, entityType LegacyEntityType) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := entityKey(snapshotID, entityType)
	return int64(len(s.entities[key])), nil
}

type InMemoryArtifactStore struct {
	mu       sync.RWMutex
	artifacts map[string]LegacyArtifactRecord
}

func NewInMemoryArtifactStore() *InMemoryArtifactStore {
	return &InMemoryArtifactStore{artifacts: make(map[string]LegacyArtifactRecord)}
}

func (s *InMemoryArtifactStore) Register(_ context.Context, snapshotID string, record LegacyArtifactRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts[snapshotID+"|"+record.ArtifactRef] = record
	return nil
}

func (s *InMemoryArtifactStore) Get(_ context.Context, snapshotID string, artifactRef string) (LegacyArtifactRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.artifacts[snapshotID+"|"+artifactRef]
	if !ok {
		return LegacyArtifactRecord{}, ErrArtifactMissing
	}
	return rec, nil
}

type DefaultGateway struct {
	snapshots SnapshotStore
	entities  EntityStore
	artifacts ArtifactStore
	mu        sync.Mutex
	currentID string
}

func NewDefaultGateway(snapshots SnapshotStore, entities EntityStore, artifacts ArtifactStore) *DefaultGateway {
	return &DefaultGateway{
		snapshots: snapshots,
		entities:  entities,
		artifacts: artifacts,
	}
}

func (g *DefaultGateway) CreateSnapshot(ctx context.Context, request LegacySnapshotRequest) (LegacySnapshot, error) {
	g.mu.Lock()
	snapshotID := newMigrationID("snapshot")
	g.currentID = snapshotID
	g.mu.Unlock()

	snapshot := LegacySnapshot{
		SnapshotID:         snapshotID,
		SchemaVersion:      "v1",
		ApplicationVersion: "legacy",
		CreatedAt:          time.Now().UTC(),
		SourceDatabaseID:   "legacy",
		DataRevision:       newMigrationID("data"),
		ArtifactRevision:   newMigrationID("artifact"),
		EntityCounts:       make(map[string]int64),
		Status:             "active",
		Labels:             request.Labels,
	}
	if len(request.IncludeEntities) == 0 {
		snapshot.EntityCounts[string(LegacyEntityPackage)] = 0
		snapshot.EntityCounts[string(LegacyEntitySkill)] = 0
		snapshot.EntityCounts[string(LegacyEntityAgentSkill)] = 0
	} else {
		for _, et := range request.IncludeEntities {
			snapshot.EntityCounts[string(et)] = 0
		}
	}
	snapshot.IntegrityHash = computeSnapshotHash(snapshot)
	if err := g.snapshots.Put(ctx, snapshot); err != nil {
		return LegacySnapshot{}, err
	}
	return snapshot, nil
}

func (g *DefaultGateway) ListEntities(ctx context.Context, snapshotID string, query LegacyEntityQuery) (LegacyEntityPage, error) {
	if _, err := g.snapshots.Get(ctx, snapshotID); err != nil {
		return LegacyEntityPage{}, err
	}
	return g.entities.List(ctx, snapshotID, query)
}

func (g *DefaultGateway) GetEntity(ctx context.Context, snapshotID string, entityType LegacyEntityType, legacyID string) (LegacyEntityRecord, error) {
	if _, err := g.snapshots.Get(ctx, snapshotID); err != nil {
		return LegacyEntityRecord{}, err
	}
	return g.entities.Get(ctx, snapshotID, entityType, legacyID)
}

func (g *DefaultGateway) ReadArtifact(ctx context.Context, snapshotID string, artifactRef string) (LegacyArtifactRecord, error) {
	if _, err := g.snapshots.Get(ctx, snapshotID); err != nil {
		return LegacyArtifactRecord{}, err
	}
	return g.artifacts.Get(ctx, snapshotID, artifactRef)
}

func (g *DefaultGateway) GetStatistics(ctx context.Context, snapshotID string) (LegacyMigrationStatistics, error) {
	snap, err := g.snapshots.Get(ctx, snapshotID)
	if err != nil {
		return LegacyMigrationStatistics{}, err
	}
	stats := LegacyMigrationStatistics{
		SnapshotID:   snapshotID,
		EntityCounts: make(map[LegacyEntityType]int64),
		GeneratedAt:  time.Now().UTC(),
	}
	for ent, count := range snap.EntityCounts {
		stats.EntityCounts[LegacyEntityType(ent)] = count
	}
	return stats, nil
}

func (g *DefaultGateway) ListSnapshots(ctx context.Context) ([]LegacySnapshot, error) {
	return g.snapshots.List(ctx)
}

func (g *DefaultGateway) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	return g.snapshots.Delete(ctx, snapshotID)
}

func (g *DefaultGateway) RegisterEntity(ctx context.Context, snapshotID string, record LegacyEntityRecord) error {
	return g.entities.Put(ctx, snapshotID, record)
}

func (g *DefaultGateway) RegisterArtifact(ctx context.Context, snapshotID string, record LegacyArtifactRecord) error {
	return g.artifacts.Register(ctx, snapshotID, record)
}

var _ LegacyReadOnlyGateway = (*DefaultGateway)(nil)

func newMigrationID(prefix string) string {
	b := make([]byte, 8)
	_, _ = readRand(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func computeSnapshotHash(snapshot LegacySnapshot) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s", snapshot.SnapshotID, snapshot.SchemaVersion, snapshot.ApplicationVersion, snapshot.SourceDatabaseID, snapshot.DataRevision)
	keys := make([]string, 0, len(snapshot.EntityCounts))
	for k := range snapshot.EntityCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "|%s=%d", k, snapshot.EntityCounts[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
