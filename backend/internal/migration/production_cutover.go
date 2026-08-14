package migration

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CutoverPhase string

const (
	CutoverPhasePreflight    CutoverPhase = "preflight"
	CutoverPhaseQuiesce      CutoverPhase = "quiesce"
	CutoverPhaseSnapshot     CutoverPhase = "snapshot"
	CutoverPhaseMigrate      CutoverPhase = "migrate"
	CutoverPhaseBootstrap    CutoverPhase = "bootstrap"
	CutoverPhaseReadSwitch   CutoverPhase = "read_switch"
	CutoverPhaseWriteLockout CutoverPhase = "write_lockout"
	CutoverPhaseWorkerCutoff CutoverPhase = "worker_cutoff"
	CutoverPhaseSmoke        CutoverPhase = "smoke"
	CutoverPhaseCommit       CutoverPhase = "commit"
)

func ValidCutoverPhases() []CutoverPhase {
	return []CutoverPhase{
		CutoverPhasePreflight,
		CutoverPhaseQuiesce,
		CutoverPhaseSnapshot,
		CutoverPhaseMigrate,
		CutoverPhaseBootstrap,
		CutoverPhaseReadSwitch,
		CutoverPhaseWriteLockout,
		CutoverPhaseWorkerCutoff,
		CutoverPhaseSmoke,
		CutoverPhaseCommit,
	}
}

type CutoverState struct {
	OperationID       string       `gorm:"column:operation_id;primaryKey" json:"operation_id"`
	Phase             CutoverPhase `gorm:"column:phase" json:"phase"`
	Status            string       `gorm:"column:status" json:"status"`
	SnapshotID        string       `gorm:"column:snapshot_id" json:"snapshot_id"`
	ErrorMessage      string       `gorm:"column:error_message" json:"error_message"`
	StartedAt         time.Time    `gorm:"column:started_at" json:"started_at"`
	UpdatedAt         time.Time    `gorm:"column:updated_at" json:"updated_at"`
	CompletedAt       *time.Time   `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CanonicalGeneration int64      `gorm:"column:canonical_generation" json:"canonical_generation"`
	PlanVersion       int          `gorm:"column:plan_version" json:"plan_version"`
}

func (CutoverState) TableName() string {
	return "production_cutover_state"
}

var (
	ErrCutoverPreflightFailure = errors.New("cutover preflight failed")
	ErrCutoverPhaseOrder       = errors.New("cutover phase order violation")
	ErrCutoverPhaseFailed      = errors.New("cutover phase failed")
	ErrCutoverIncomplete       = errors.New("cutover incomplete")
	ErrCutoverNotCommitted     = errors.New("cutover not committed")
)

type CanonicalAuthorityProvider interface {
	ToolFacade() interface{}
	PermissionBroker() interface{}
	EventService() interface{}
	ScheduleService() interface{}
	TaskRuntimeService() interface{}
	HookService() interface{}
}

type CutoverMaintenanceGate interface {
	BeginQuiesce(ctx context.Context) error
	EndQuiesce(ctx context.Context) error
	IsQuiesced() bool
}

type CutoverSnapshotPort interface {
	CreatePortableSnapshot(ctx context.Context, operationID string) (snapshotID string, err error)
	VerifyPortableSnapshot(ctx context.Context, snapshotID string) error
}

type CutoverMigrationPort interface {
	ExecuteLegacyToCanonical(ctx context.Context, operationID string) error
}

type CutoverBootstrapPort interface {
	VerifyCanonicalWiring(ctx context.Context) error
}

type CutoverReadSwitchPort interface {
	VerifyReadCanonical(ctx context.Context) error
}

type CutoverWriteLockoutPort interface {
	LockoutLegacyWrites(ctx context.Context) error
	VerifyLegacyWriteLockout(ctx context.Context) error
}

type CutoverWorkerCutoffPort interface {
	GetLegacyWorkerStatus() LegacyWorkerStatus
	StopLegacyWorkers(ctx context.Context) error
}

type CutoverSmokePort interface {
	RunSmokeChecks(ctx context.Context) error
}

type CutoverLegacyVerifier interface {
	LegacyMCPManagerPresent() bool
	LegacyPluginWorkersPresent() bool
	MemoryRawWriterPresent() bool
	LegacyRuntimeActive() int
	LegacyWriteEnabled() int
}

type CutoverStateStore interface {
	LoadState(ctx context.Context) (*CutoverState, error)
	SaveState(ctx context.Context, state *CutoverState) error
	LoadLatestState(ctx context.Context) (*CutoverState, error)
}

type LegacyWorkerStatus struct {
	PluginManagerStarted  bool
	LegacyHookWorkerStarted bool
	LegacyEventWorkerStarted bool
	LegacyScheduleWorkerStarted bool
	LegacyRuntimeStarted  bool
	LegacyMCPWorkerActive bool
}

type AuthoritySnapshot struct {
	ToolFacadeCount        int
	PermissionBrokerCount  int
	TaskRuntimeCount       int
	EventServiceCount      int
	ScheduleServiceCount   int
	HookServiceCount       int
	AdapterTypes           []string
	MCPDuplicateUnresolved int64
}

type CutoverDependencies struct {
	DB            *gorm.DB
	Container     CanonicalAuthorityProvider
	Maintenance   CutoverMaintenanceGate
	Snapshot      CutoverSnapshotPort
	Migration     CutoverMigrationPort
	Bootstrap     CutoverBootstrapPort
	ReadSwitch    CutoverReadSwitchPort
	WriteLockout  CutoverWriteLockoutPort
	WorkerCutoff  CutoverWorkerCutoffPort
	Smoke         CutoverSmokePort
	LegacyVerifier CutoverLegacyVerifier
	StateStore    CutoverStateStore
	Now           func() time.Time
}

type CutoverPlan struct {
	deps CutoverDependencies
}

func NewCutoverPlan(deps CutoverDependencies) *CutoverPlan {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &CutoverPlan{deps: deps}
}

func NewCutoverPlanLegacy(db *gorm.DB, container CanonicalAuthorityProvider) *CutoverPlan {
	return &CutoverPlan{
		deps: CutoverDependencies{
			DB:        db,
			Container: container,
			Now:       time.Now,
		},
	}
}

func (p *CutoverPlan) getDB() *gorm.DB {
	return p.deps.DB
}

func (p *CutoverPlan) getStateStore() CutoverStateStore {
	if p.deps.StateStore != nil {
		return p.deps.StateStore
	}
	return &dbStateStore{db: p.deps.DB, now: p.deps.Now}
}

func (p *CutoverPlan) getMaintenance() CutoverMaintenanceGate {
	return p.deps.Maintenance
}

func (p *CutoverPlan) getSnapshot() CutoverSnapshotPort {
	return p.deps.Snapshot
}

func (p *CutoverPlan) getMigration() CutoverMigrationPort {
	return p.deps.Migration
}

func (p *CutoverPlan) getBootstrap() CutoverBootstrapPort {
	return p.deps.Bootstrap
}

func (p *CutoverPlan) getReadSwitch() CutoverReadSwitchPort {
	return p.deps.ReadSwitch
}

func (p *CutoverPlan) getWriteLockout() CutoverWriteLockoutPort {
	return p.deps.WriteLockout
}

func (p *CutoverPlan) getWorkerCutoff() CutoverWorkerCutoffPort {
	return p.deps.WorkerCutoff
}

func (p *CutoverPlan) getSmoke() CutoverSmokePort {
	return p.deps.Smoke
}

func (p *CutoverPlan) getLegacyVerifier() CutoverLegacyVerifier {
	return p.deps.LegacyVerifier
}

type dbStateStore struct {
	db  *gorm.DB
	now func() time.Time
}

func (s *dbStateStore) LoadState(ctx context.Context) (*CutoverState, error) {
	var state CutoverState
	if err := s.db.WithContext(ctx).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *dbStateStore) LoadLatestState(ctx context.Context) (*CutoverState, error) {
	var state CutoverState
	if err := s.db.WithContext(ctx).Order("started_at DESC").First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *dbStateStore) SaveState(ctx context.Context, state *CutoverState) error {
	state.UpdatedAt = s.now()
	return s.db.WithContext(ctx).Save(state).Error
}

func (p *CutoverPlan) Run(ctx context.Context) error {
	store := p.getStateStore()

	existing, err := store.LoadLatestState(ctx)
	if err != nil {
		return fmt.Errorf("load cutover state: %w", err)
	}

	var state *CutoverState
	if existing != nil && existing.Status != "committed" && existing.Status != "failed" && existing.Status != "rolled_back" {
		state = existing
	} else {
		state = &CutoverState{
			OperationID:         uuid.NewString(),
			Phase:               "",
			Status:              "running",
			StartedAt:           p.deps.Now(),
			UpdatedAt:           p.deps.Now(),
			PlanVersion:         1,
			CanonicalGeneration: 1,
		}
		if err := store.SaveState(ctx, state); err != nil {
			return fmt.Errorf("save initial cutover state: %w", err)
		}
	}

	phases := []struct {
		name CutoverPhase
		fn   func(context.Context, *CutoverState) error
	}{
		{CutoverPhasePreflight, p.runPreflight},
		{CutoverPhaseQuiesce, p.runQuiesce},
		{CutoverPhaseSnapshot, p.runSnapshot},
		{CutoverPhaseMigrate, p.runMigrate},
		{CutoverPhaseBootstrap, p.runBootstrap},
		{CutoverPhaseReadSwitch, p.runReadSwitch},
		{CutoverPhaseWriteLockout, p.runWriteLockout},
		{CutoverPhaseWorkerCutoff, p.runWorkerCutoff},
		{CutoverPhaseSmoke, p.runSmoke},
		{CutoverPhaseCommit, p.runCommit},
	}

	started := false
	for _, phase := range phases {
		if !started && state.Phase != "" && state.Phase != phase.name {
			continue
		}
		if !started {
			started = true
		}

		state.Phase = phase.name
		state.Status = "running"
		state.UpdatedAt = p.deps.Now()
		if err := store.SaveState(ctx, state); err != nil {
			return fmt.Errorf("persist phase %s running: %w", phase.name, err)
		}

		if err := phase.fn(ctx, state); err != nil {
			state.Status = "failed"
			state.ErrorMessage = err.Error()
			state.UpdatedAt = p.deps.Now()
			_ = store.SaveState(ctx, state)
			return fmt.Errorf("phase %s failed: %w", phase.name, err)
		}
	}

	state.Status = "completed"
	now := p.deps.Now()
	state.CompletedAt = &now
	state.UpdatedAt = now
	if err := store.SaveState(ctx, state); err != nil {
		return fmt.Errorf("persist final cutover state: %w", err)
	}
	return nil
}

func (p *CutoverPlan) runPreflight(ctx context.Context, state *CutoverState) error {
	if err := p.Preflight(ctx); err != nil {
		return err
	}
	if p.getLegacyVerifier() != nil {
		v := p.getLegacyVerifier()
		if v.LegacyMCPManagerPresent() {
			return fmt.Errorf("%w: legacy MCP manager present", ErrCutoverPreflightFailure)
		}
		if v.LegacyPluginWorkersPresent() {
			return fmt.Errorf("%w: legacy plugin workers present", ErrCutoverPreflightFailure)
		}
		if v.MemoryRawWriterPresent() {
			return fmt.Errorf("%w: memory raw writer present", ErrCutoverPreflightFailure)
		}
	}
	return nil
}

func (p *CutoverPlan) runQuiesce(ctx context.Context, state *CutoverState) error {
	if p.getMaintenance() == nil {
		return nil
	}
	return p.getMaintenance().BeginQuiesce(ctx)
}

func (p *CutoverPlan) runSnapshot(ctx context.Context, state *CutoverState) error {
	if p.getSnapshot() == nil {
		return nil
	}
	snapshotID, err := p.getSnapshot().CreatePortableSnapshot(ctx, state.OperationID)
	if err != nil {
		return fmt.Errorf("create portable snapshot: %w", err)
	}
	if err := p.getSnapshot().VerifyPortableSnapshot(ctx, snapshotID); err != nil {
		return fmt.Errorf("verify portable snapshot: %w", err)
	}
	state.SnapshotID = snapshotID
	return nil
}

func (p *CutoverPlan) runMigrate(ctx context.Context, state *CutoverState) error {
	if p.getMigration() == nil {
		return nil
	}
	return p.getMigration().ExecuteLegacyToCanonical(ctx, state.OperationID)
}

func (p *CutoverPlan) runBootstrap(ctx context.Context, state *CutoverState) error {
	if p.getBootstrap() == nil {
		return errors.New("bootstrap port not configured")
	}
	return p.getBootstrap().VerifyCanonicalWiring(ctx)
}

func (p *CutoverPlan) runReadSwitch(ctx context.Context, state *CutoverState) error {
	if p.getReadSwitch() == nil {
		return nil
	}
	return p.getReadSwitch().VerifyReadCanonical(ctx)
}

func (p *CutoverPlan) runWriteLockout(ctx context.Context, state *CutoverState) error {
	if p.getWriteLockout() == nil {
		return errors.New("write lockout port not configured")
	}
	if err := p.getWriteLockout().LockoutLegacyWrites(ctx); err != nil {
		return err
	}
	return p.getWriteLockout().VerifyLegacyWriteLockout(ctx)
}

func (p *CutoverPlan) runWorkerCutoff(ctx context.Context, state *CutoverState) error {
	if p.getWorkerCutoff() == nil {
		return errors.New("worker cutoff port not configured")
	}
	if err := p.getWorkerCutoff().StopLegacyWorkers(ctx); err != nil {
		return err
	}
	status := p.getWorkerCutoff().GetLegacyWorkerStatus()
	if status.PluginManagerStarted || status.LegacyHookWorkerStarted || status.LegacyEventWorkerStarted ||
		status.LegacyScheduleWorkerStarted || status.LegacyRuntimeStarted || status.LegacyMCPWorkerActive {
		return fmt.Errorf("legacy workers still active: %+v", status)
	}
	return nil
}

func (p *CutoverPlan) runSmoke(ctx context.Context, state *CutoverState) error {
	if p.getSmoke() == nil {
		return nil
	}
	return p.getSmoke().RunSmokeChecks(ctx)
}

func (p *CutoverPlan) runCommit(ctx context.Context, state *CutoverState) error {
	state.Status = "committed"
	state.SnapshotID = state.SnapshotID

	var calcErr error
	state.CanonicalGeneration, calcErr = p.computeCanonicalGeneration()
	if calcErr != nil {
		state.CanonicalGeneration = p.deps.Now().Unix()
	}

	if p.getMaintenance() != nil {
		if err := p.getMaintenance().EndQuiesce(ctx); err != nil {
			return fmt.Errorf("end quiesce during commit: %w", err)
		}
	}

	commitTime := p.deps.Now()
	state.CompletedAt = &commitTime
	state.Status = "committed"
	state.UpdatedAt = commitTime
	if err := p.getStateStore().SaveState(ctx, state); err != nil {
		return fmt.Errorf("persist committed state: %w", err)
	}
	return nil
}

func (p *CutoverPlan) computeCanonicalGeneration() (int64, error) {
	if p.deps.Container == nil {
		return 0, errors.New("container not available")
	}
	h := sha256.New()
	if p.deps.Container.ToolFacade() != nil {
		h.Write([]byte("tool_facade"))
	}
	if p.deps.Container.PermissionBroker() != nil {
		h.Write([]byte("permission_broker"))
	}
	if p.deps.Container.EventService() != nil {
		h.Write([]byte("event_service"))
	}
	if p.deps.Container.ScheduleService() != nil {
		h.Write([]byte("schedule_service"))
	}
	if p.deps.Container.TaskRuntimeService() != nil {
		h.Write([]byte("task_runtime"))
	}
	if p.deps.Container.HookService() != nil {
		h.Write([]byte("hook_service"))
	}
	var sum int64
	for _, b := range h.Sum(nil) {
		sum += int64(b)
	}
	return sum, nil
}

func (p *CutoverPlan) Preflight(ctx context.Context) error {
	if p.deps.Container == nil {
		return fmt.Errorf("%w: kernel container absent", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.ToolFacade() == nil {
		return fmt.Errorf("%w: ToolFacade not initialized", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.PermissionBroker() == nil {
		return fmt.Errorf("%w: PermissionBroker not initialized", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.EventService() == nil {
		return fmt.Errorf("%w: EventService not initialized", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.ScheduleService() == nil {
		return fmt.Errorf("%w: ScheduleService not initialized", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.TaskRuntimeService() == nil {
		return fmt.Errorf("%w: TaskRuntimeService not initialized", ErrCutoverPreflightFailure)
	}
	if p.deps.Container.HookService() == nil {
		return fmt.Errorf("%w: HookService not initialized", ErrCutoverPreflightFailure)
	}
	return nil
}

func (p *CutoverPlan) VerifyCanonicalAuthorities() []string {
	failures := []string{}
	if p.deps.Container == nil {
		failures = append(failures, "KernelContainer: missing")
		return failures
	}
	if p.deps.Container.ToolFacade() == nil {
		failures = append(failures, "ToolFacade: missing")
	}
	if p.deps.Container.PermissionBroker() == nil {
		failures = append(failures, "PermissionBroker: missing")
	}
	if p.deps.Container.EventService() == nil {
		failures = append(failures, "EventService: missing")
	}
	if p.deps.Container.ScheduleService() == nil {
		failures = append(failures, "ScheduleService: missing")
	}
	if p.deps.Container.TaskRuntimeService() == nil {
		failures = append(failures, "TaskRuntimeService: missing")
	}
	if p.deps.Container.HookService() == nil {
		failures = append(failures, "HookService: missing")
	}
	return failures
}

func (p *CutoverPlan) CheckCutoverState(ctx context.Context) (committed bool, incomplete bool, err error) {
	store := p.getStateStore()
	state, err := store.LoadLatestState(ctx)
	if err != nil {
		return false, false, err
	}
	if state == nil {
		return false, false, nil
	}
	if state.Status == "committed" {
		return true, false, nil
	}
	if state.Status == "running" {
		return false, true, nil
	}
	return false, false, nil
}

func ProductionCutoverMigration() Migration {
	return Migration{
		Version: "20260101001",
		Name:    "production_cutover_state",
		Up: func(s *Step) error {
			return s.CreateTable(`CREATE TABLE IF NOT EXISTS production_cutover_state (
				operation_id TEXT PRIMARY KEY,
				phase TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT '',
				snapshot_id TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				started_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				completed_at TEXT NOT NULL DEFAULT '',
				canonical_generation INTEGER NOT NULL DEFAULT 0,
				plan_version INTEGER NOT NULL DEFAULT 1
			)`)
		},
	}
}
