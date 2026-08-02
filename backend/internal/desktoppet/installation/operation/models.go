// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package operation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	TypeInstall       = "install"
	TypeUpgrade       = "upgrade"
	TypeDowngrade     = "downgrade"
	TypeSwitch        = "switch"
	TypeEnable        = "enable"
	TypeDisable       = "disable"
	TypeSettings      = "settings"
	TypeDefaultAction = "default_action"
	TypeRecenter      = "recenter"
	TypeRepair        = "repair"
	TypeUninstall     = "uninstall"
)

const (
	OpStatusCreated           = "created"
	OpStatusQueued            = "queued"
	OpStatusRunning           = "running"
	OpStatusWaitingRuntimeACK = "waiting_runtime_ack"
	OpStatusCompleted         = "completed"
	OpStatusFailedRetryable   = "failed_retryable"
	OpStatusFailedTerminal    = "failed_terminal"
	OpStatusCancelRequested   = "cancel_requested"
	OpStatusCancelled         = "cancelled"
)

const (
	OpStageRequestValidated       = "request_validated"
	OpStageReleaseVerified        = "release_verified"
	OpStageStagingPrepared        = "staging_prepared"
	OpStageStagingVerified        = "staging_verified"
	OpStageOldInstallParked       = "old_install_parked"
	OpStageFilesPublished         = "files_published"
	OpStageDatabaseCommitted      = "database_committed"
	OpStageDesiredStateCommitted  = "desired_state_committed"
	OpStageRuntimeCommandEnqueued = "runtime_command_enqueued"
	OpStageWaitingRuntimeACK      = "waiting_runtime_ack"
	OpStageRuntimeApplied         = "runtime_applied"
	OpStageCleanupCompleted       = "cleanup_completed"
	OpStageCompleted              = "completed"
)

const (
	ErrCodeIDEMPOTENCYConflict   = "INSTALLATION_IDEMPOTENCY_CONFLICT"
	ErrCodeJournalCONFLICT       = "INSTALLATION_JOURNAL_CONFLICT"
	ErrCodeLeaseLOST             = "INSTALLATION_LEASE_LOST"
	ErrCodeInvalidTRANSITION     = "INSTALLATION_INVALID_TRANSITION"
	ErrCodeOWNERSHIPMismatch     = "INSTALLATION_OWNERSHIP_MISMATCH"
	ErrCodeReleaseNOTINSTALLABLE = "INSTALLATION_RELEASE_NOT_INSTALLABLE"
)

var (
	ErrInvalidTransition   = errors.New("operation: invalid state transition")
	ErrLeaseLost           = errors.New("operation: lease lost before commit")
	ErrIdempotencyConflict = errors.New("operation: idempotency key conflict")
	ErrJournalConflict     = errors.New("operation: journal state conflict")
	ErrOwnershipMismatch   = errors.New("operation: ownership mismatch")
)

var validTransitions = map[string]map[string]bool{
	OpStatusCreated: {
		OpStatusQueued: true, OpStatusRunning: true,
		OpStatusCancelRequested: true, OpStatusCancelled: true,
	},
	OpStatusQueued: {
		OpStatusRunning: true, OpStatusCancelRequested: true, OpStatusCancelled: true,
	},
	OpStatusRunning: {
		OpStatusWaitingRuntimeACK: true, OpStatusCompleted: true,
		OpStatusFailedRetryable: true, OpStatusFailedTerminal: true,
		OpStatusCancelRequested: true, OpStatusCancelled: true,
	},
	OpStatusWaitingRuntimeACK: {
		OpStatusCompleted: true, OpStatusFailedRetryable: true,
		OpStatusFailedTerminal: true, OpStatusCancelRequested: true, OpStatusCancelled: true,
	},
	OpStatusFailedRetryable: {
		OpStatusQueued: true, OpStatusRunning: true, OpStatusCancelRequested: true, OpStatusCancelled: true,
	},
	OpStatusCancelRequested: {
		OpStatusCancelled: true, OpStatusFailedTerminal: true,
	},
}

type InstallationOperation struct {
	ID string

	OperationType string

	UserID    string
	DeviceID  string
	RuntimeID string

	InstallationID string
	PetID          string

	SourceReleaseID string
	TargetReleaseID string

	IdempotencyKey string
	RequestHash    string

	Status string
	Stage  string

	AttemptNumber int

	ExecutionID    string
	LeaseOwner     string
	LeaseExpiresAt string
	HeartbeatAt    string

	DesiredRevision         int64
	ExpectedAppliedRevision int64

	ErrorCode    string
	ErrorMessage string

	CreatedAt   string
	StartedAt   string
	UpdatedAt   string
	CompletedAt string
}

func (o InstallationOperation) TableName() string {
	return "desktop_pet_installation_operations"
}

func (o InstallationOperation) IsTerminal() bool {
	return o.Status == OpStatusCompleted ||
		o.Status == OpStatusFailedTerminal ||
		o.Status == OpStatusCancelled
}

func (o InstallationOperation) IsActive() bool {
	return o.Status == OpStatusCreated ||
		o.Status == OpStatusQueued ||
		o.Status == OpStatusRunning ||
		o.Status == OpStatusWaitingRuntimeACK ||
		o.Status == OpStatusFailedRetryable ||
		o.Status == OpStatusCancelRequested
}

func (o InstallationOperation) CanTransitionTo(newStatus string) bool {
	allowed, ok := validTransitions[o.Status]
	return ok && allowed[newStatus]
}

type Lease struct {
	OperationID string
	Owner       string
	ExpiresAt   time.Time
	HeartbeatAt time.Time
}

func (l Lease) IsExpired(now time.Time) bool {
	return now.After(l.ExpiresAt)
}

type OperationResult struct {
	OperationID      string
	InstallationID   string
	Status           string
	Stage            string
	DesiredRevision  int64
	RuntimeSyncState string
	Retryable        bool
}

type ClaimLeaseRequest struct {
	OperationID      string
	LeaseOwner       string
	Timeout          time.Duration
	ExpectedStatuses []string
}

func ComputeIdempotencyKey(parts ...string) string {
	joined := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:16])
}

func ComputeRequestHash(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteByte('=')
		builder.WriteString(fields[k])
		builder.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:16])
}
