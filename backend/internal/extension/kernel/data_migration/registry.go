package data_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DataMigrationTable struct {
	TableName      string `json:"tableName"`
	LegacySchema   string `json:"legacySchema"`
	NewSchema      string `json:"newSchema"`
	Transformation string `json:"transformation,omitempty"`
	RequiresBackup bool   `json:"requiresBackup"`
	Destructive    bool   `json:"destructive"`
	BatchSize      int    `json:"batchSize"`
}

type DataMigrationSpec struct {
	MigrationID  string               `json:"migrationId"`
	SourceSystem string               `json:"sourceSystem"`
	TargetSystem string               `json:"targetSystem"`
	Version      string               `json:"version"`
	Tables       []DataMigrationTable `json:"tables"`
	PreChecks    []string             `json:"preChecks,omitempty"`
	PostChecks   []string             `json:"postChecks,omitempty"`
	Rollbackable bool                 `json:"rollbackable"`
	DryRunOnly   bool                 `json:"dryRunOnly"`
}

type DataMigrationRegistry struct {
	mu       sync.RWMutex
	specs    map[string]*DataMigrationSpec
	executed map[string]*MigrationExecution
}

type MigrationExecution struct {
	MigrationID     string    `json:"migrationId"`
	StartedAt       time.Time `json:"startedAt"`
	CompletedAt     time.Time `json:"completedAt,omitempty"`
	Status          string    `json:"status"`
	RecordsMigrated int       `json:"recordsMigrated"`
	Errors          []string  `json:"errors,omitempty"`
	IsDryRun        bool      `json:"isDryRun"`
}

func NewDataMigrationRegistry() *DataMigrationRegistry {
	return &DataMigrationRegistry{
		specs:    make(map[string]*DataMigrationSpec),
		executed: make(map[string]*MigrationExecution),
	}
}

func (r *DataMigrationRegistry) Register(spec *DataMigrationSpec) error {
	if spec == nil || spec.MigrationID == "" {
		return ErrInvalidSpec
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.MigrationID]; exists {
		return fmt.Errorf("%w: %s", ErrMigrationExists, spec.MigrationID)
	}
	for i := range spec.Tables {
		if spec.Tables[i].BatchSize <= 0 {
			spec.Tables[i].BatchSize = 1000
		}
	}
	r.specs[spec.MigrationID] = spec
	return nil
}

func (r *DataMigrationRegistry) Get(migrationID string) (*DataMigrationSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[migrationID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrMigrationNotFound, migrationID)
	}
	return spec, nil
}

func (r *DataMigrationRegistry) List() []*DataMigrationSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*DataMigrationSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

func (r *DataMigrationRegistry) MarkExecuted(exec *MigrationExecution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executed[exec.MigrationID] = exec
}

func (r *DataMigrationRegistry) GetExecution(migrationID string) (*MigrationExecution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, exists := r.executed[migrationID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrExecutionNotFound, migrationID)
	}
	return exec, nil
}

type DataMigrationReport struct {
	StartTime        time.Time             `json:"startTime"`
	EndTime          time.Time             `json:"endTime"`
	TotalMigrations  int                   `json:"totalMigrations"`
	CompletedCount   int                   `json:"completedCount"`
	FailedCount      int                   `json:"failedCount"`
	SkippedCount     int                   `json:"skippedCount"`
	RecordsMigrated  int                   `json:"recordsMigrated"`
	FailedMigrations []FailedDataMigration `json:"failedMigrations,omitempty"`
	Status           string                `json:"status"`
}

type FailedDataMigration struct {
	MigrationID string `json:"migrationId"`
	Reason      string `json:"reason"`
}

func RunDataMigration(ctx context.Context, registry *DataMigrationRegistry, migrationIDs []string, dryRun bool) (*DataMigrationReport, error) {
	report := &DataMigrationReport{
		StartTime: time.Now().UTC(),
		Status:    "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()
	report.TotalMigrations = len(migrationIDs)
	for _, mid := range migrationIDs {
		spec, err := registry.Get(mid)
		if err != nil {
			report.FailedCount++
			report.FailedMigrations = append(report.FailedMigrations, FailedDataMigration{
				MigrationID: mid,
				Reason:      err.Error(),
			})
			continue
		}
		if spec.DryRunOnly && !dryRun {
			report.SkippedCount++
			continue
		}
		exec := &MigrationExecution{
			MigrationID: mid,
			StartedAt:   time.Now().UTC(),
			Status:      "running",
			IsDryRun:    dryRun,
		}
		if dryRun {
			exec.Status = "dry_run_completed"
			exec.CompletedAt = time.Now().UTC()
		} else {
			exec.Status = "completed"
			exec.CompletedAt = time.Now().UTC()
			exec.RecordsMigrated = 0
			report.RecordsMigrated += exec.RecordsMigrated
		}
		registry.MarkExecuted(exec)
		report.CompletedCount++
	}
	return report, nil
}

type CutoverPlan struct {
	PlanID            string        `json:"planId"`
	StartTime         time.Time     `json:"startTime"`
	Steps             []CutoverStep `json:"steps"`
	RequiresDowntime  bool          `json:"requiresDowntime"`
	EstimatedDuration time.Duration `json:"estimatedDuration"`
}

type CutoverStep struct {
	StepID    string `json:"stepId"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Required  bool   `json:"required"`
	Completed bool   `json:"completed"`
}

func NewCutoverPlan() *CutoverPlan {
	return &CutoverPlan{
		PlanID:    fmt.Sprintf("cutover_%d", time.Now().UnixNano()),
		StartTime: time.Now().UTC(),
		Steps: []CutoverStep{
			{StepID: "backup", Name: "备份数据", Type: "backup", Required: true},
			{StepID: "stop_legacy", Name: "停止旧系统", Type: "stop", Required: true},
			{StepID: "migrate_data", Name: "迁移数据", Type: "migrate", Required: true},
			{StepID: "verify_data", Name: "验证数据", Type: "verify", Required: true},
			{StepID: "enable_kernel", Name: "启用 Extension Kernel", Type: "enable", Required: true},
			{StepID: "verify_runtime", Name: "验证运行时", Type: "verify", Required: true},
			{StepID: "cleanup_legacy", Name: "清理旧系统", Type: "cleanup", Required: false},
		},
		RequiresDowntime:  true,
		EstimatedDuration: 30 * time.Minute,
	}
}

var (
	ErrInvalidSpec       = errors.New("data_migration: invalid spec")
	ErrMigrationExists   = errors.New("data_migration: migration exists")
	ErrMigrationNotFound = errors.New("data_migration: migration not found")
	ErrExecutionNotFound = errors.New("data_migration: execution not found")
)

var _ = json.Marshal
