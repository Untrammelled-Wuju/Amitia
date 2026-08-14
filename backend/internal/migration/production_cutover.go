package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	Phase       CutoverPhase   `json:"phase"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Completed   []CutoverPhase `json:"completed"`
	Err         string         `json:"error,omitempty"`
}

var (
	ErrCutoverPreflightFailure = errors.New("cutover preflight failed")
	ErrCutoverPhaseOrder       = errors.New("cutover phase order violation")
)

type CanonicalAuthorityProvider interface {
	ToolFacade() interface{}
	PermissionBroker() interface{}
	EventService() interface{}
	ScheduleService() interface{}
	TaskRuntimeService() interface{}
	HookService() interface{}
}

type CutoverPlan struct {
	db        *gorm.DB
	container CanonicalAuthorityProvider
	now       func() time.Time
}

func NewCutoverPlan(db *gorm.DB, container CanonicalAuthorityProvider) *CutoverPlan {
	return &CutoverPlan{
		db:        db,
		container: container,
		now:       time.Now,
	}
}

func (p *CutoverPlan) Preflight(ctx context.Context) error {
	if p.container == nil {
		return fmt.Errorf("%w: kernel container absent", ErrCutoverPreflightFailure)
	}
	if p.container.ToolFacade() == nil {
		return fmt.Errorf("%w: ToolFacade not initialized", ErrCutoverPreflightFailure)
	}
	if p.container.PermissionBroker() == nil {
		return fmt.Errorf("%w: PermissionBroker not initialized", ErrCutoverPreflightFailure)
	}
	if p.container.EventService() == nil {
		return fmt.Errorf("%w: EventService not initialized", ErrCutoverPreflightFailure)
	}
	if p.container.ScheduleService() == nil {
		return fmt.Errorf("%w: ScheduleService not initialized", ErrCutoverPreflightFailure)
	}
	if p.container.TaskRuntimeService() == nil {
		return fmt.Errorf("%w: TaskRuntimeService not initialized", ErrCutoverPreflightFailure)
	}
	if p.container.HookService() == nil {
		return fmt.Errorf("%w: HookService not initialized", ErrCutoverPreflightFailure)
	}
	return nil
}

func (p *CutoverPlan) VerifyCanonicalAuthorities() []string {
	failures := []string{}
	if p.container.ToolFacade() == nil {
		failures = append(failures, "ToolFacade: missing")
	}
	if p.container.PermissionBroker() == nil {
		failures = append(failures, "PermissionBroker: missing")
	}
	if p.container.EventService() == nil {
		failures = append(failures, "EventService: missing")
	}
	if p.container.ScheduleService() == nil {
		failures = append(failures, "ScheduleService: missing")
	}
	if p.container.TaskRuntimeService() == nil {
		failures = append(failures, "TaskRuntimeService: missing")
	}
	if p.container.HookService() == nil {
		failures = append(failures, "HookService: missing")
	}
	return failures
}

func ProductionCutoverMigration() Migration {
	return Migration{
		Version: "20260101001",
		Name:    "production_cutover_legacy_mcp_removal",
		Up: func(s *Step) error {
			return nil
		},
	}
}
