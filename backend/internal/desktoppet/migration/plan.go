// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type MigrationStage string

const (
	StagePreflight          MigrationStage = "preflight"
	StageBackup             MigrationStage = "backing_up"
	StageSchema             MigrationStage = "schema"
	StageBackfill           MigrationStage = "backfill"
	StageVerifying          MigrationStage = "verifying"
	StageReadCutover        MigrationStage = "read_cutover"
	StageWriteCutover       MigrationStage = "write_cutover"
	StageLegacyWriteBlocked MigrationStage = "legacy_write_blocked"
	StageCompleted          MigrationStage = "completed"
	StageFailedRetryable    MigrationStage = "failed_retryable"
	StageFailedTerminal     MigrationStage = "failed_terminal"
	StageManualReview       MigrationStage = "manual_review"
)

type CheckType string

const (
	CheckTypeDatabase CheckType = "database"
	CheckTypeStorage  CheckType = "storage"
	CheckTypeVersion  CheckType = "version"
	CheckTypeDisk     CheckType = "disk"
	CheckTypeWorker   CheckType = "worker"
)

type CheckResult struct {
	Name      string    `json:"name"`
	Type      CheckType `json:"type"`
	Passed    bool      `json:"passed"`
	Message   string    `json:"message,omitempty"`
	CheckedAt string    `json:"checkedAt"`
}

type CheckFunc func() (bool, string)

type DomainMigrationPlan struct {
	ID                    string            `json:"id"`
	Version               string            `json:"version"`
	Domain                string            `json:"domain"`
	Dependencies          []string          `json:"dependencies"`
	MinAppVersion         string            `json:"minAppVersion"`
	MaxRollbackAppVersion string            `json:"maxRollbackAppVersion"`
	PreflightChecks       []CheckFunc       `json:"-"`
	SchemaSteps           []StepFunc        `json:"-"`
	BackfillSteps         []BatchedStepFunc `json:"-"`
	VerificationChecks    []CheckFunc       `json:"-"`
	CutoverSteps          []StepFunc        `json:"-"`
	LegacyWriteBlockSteps []StepFunc        `json:"-"`
	CleanupSteps          []StepFunc        `json:"-"`
	ForwardFixPolicy      string            `json:"forwardFixPolicy"`
	RollbackPolicy        string            `json:"rollbackPolicy"`
}

type StepFunc func() error

type BatchedStepFunc func(batchOffset, batchSize int) (processed int, conflicts int, err error)

type MigrationOperation struct {
	ID                   string         `json:"id"`
	PlanID               string         `json:"planId"`
	SourceVersion        string         `json:"sourceVersion"`
	TargetVersion        string         `json:"targetVersion"`
	Stage                MigrationStage `json:"stage"`
	Checkpoint           string         `json:"checkpoint"`
	ProcessedCount       int64          `json:"processedCount"`
	ConflictCount        int64          `json:"conflictCount"`
	BackupID             string         `json:"backupId,omitempty"`
	Lease                string         `json:"lease,omitempty"`
	VerifiedReadCutover  bool           `json:"verifiedReadCutover"`
	VerifiedWriteCutover bool           `json:"verifiedWriteCutover"`
	Error                string         `json:"error,omitempty"`
	StartedAt            string         `json:"startedAt"`
	UpdatedAt            string         `json:"updatedAt"`
	CompletedAt          string         `json:"completedAt,omitempty"`
}

type MigrationConflict struct {
	ID                   string `json:"id"`
	MigrationOperationID string `json:"migrationOperationId"`
	Domain               string `json:"domain"`
	LegacyEntityType     string `json:"legacyEntityType"`
	LegacyEntityID       string `json:"legacyEntityId"`
	ReasonCode           string `json:"reasonCode"`
	EvidenceJSON         string `json:"evidenceJson,omitempty"`
	Status               string `json:"status"`
	ResolutionJSON       string `json:"resolutionJson,omitempty"`
}

var (
	ErrMigrationLocked     = errors.New("migration: exclusive lock held by another instance")
	ErrMigrationDependency = errors.New("migration: dependency not satisfied")
	ErrMigrationStage      = errors.New("migration: invalid stage transition")
	ErrPreflightFailed     = errors.New("migration: preflight check failed")
	ErrBackupFailed        = errors.New("migration: backup failed")
	ErrMaintenanceRequired = errors.New("migration: maintenance mode required")
)

type MigrationLock struct {
	mu         sync.Mutex
	locked     bool
	owner      string
	acquiredAt time.Time
	expiresAt  time.Time
}

func NewMigrationLock() *MigrationLock {
	return &MigrationLock{}
}

func (l *MigrationLock) Acquire(owner string, ttl time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked && time.Now().Before(l.expiresAt) {
		return fmt.Errorf("%w: held by %s until %v", ErrMigrationLocked, l.owner, l.expiresAt)
	}
	now := time.Now()
	l.locked = true
	l.owner = owner
	l.acquiredAt = now
	l.expiresAt = now.Add(ttl)
	return nil
}

func (l *MigrationLock) Release(owner string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.locked {
		return nil
	}
	if l.owner != owner {
		return fmt.Errorf("migration: lock held by %s, cannot release by %s", l.owner, owner)
	}
	l.locked = false
	l.owner = ""
	l.acquiredAt = time.Time{}
	l.expiresAt = time.Time{}
	return nil
}

func (l *MigrationLock) Extend(owner string, ttl time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.locked || l.owner != owner {
		return fmt.Errorf("migration: lock not held by %s", owner)
	}
	l.expiresAt = time.Now().Add(ttl)
	return nil
}

func (l *MigrationLock) IsLocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked && time.Now().Before(l.expiresAt) {
		return true
	}
	if l.locked {
		l.locked = false
		l.owner = ""
		l.expiresAt = time.Time{}
	}
	return false
}

func (l *MigrationLock) Info() (locked bool, owner string, acquiredAt, expiresAt time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked && time.Now().Before(l.expiresAt), l.owner, l.acquiredAt, l.expiresAt
}
