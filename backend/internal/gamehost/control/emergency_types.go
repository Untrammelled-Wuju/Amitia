package control

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type EmergencyStopActor string

const (
	EmergencyActorUser   EmergencyStopActor = "user"
	EmergencyActorHost   EmergencyStopActor = "host"
	EmergencyActorSystem EmergencyStopActor = "system"
)

type EmergencyStopReason string

const (
	EmergencyReasonUserRequested      EmergencyStopReason = "user_requested"
	EmergencyReasonSafetyPolicy       EmergencyStopReason = "safety_policy"
	EmergencyReasonResourceViolation  EmergencyStopReason = "resource_violation"
	EmergencyReasonRuntimeUnresponsive EmergencyStopReason = "runtime_unresponsive"
	EmergencyReasonManualAdminAction  EmergencyStopReason = "manual_admin_action"
)

type EmergencyStopState string

const (
	EmergencyStateRequested          EmergencyStopState = "requested"
	EmergencyStateCommittingIntent   EmergencyStopState = "committing_intent"
	EmergencyStateClosingGate        EmergencyStopState = "closing_gate"
	EmergencyStateRevokingAuthority  EmergencyStopState = "revoking_authority"
	EmergencyStateCancelling         EmergencyStopState = "cancelling"
	EmergencyStateStoppingRuntime    EmergencyStopState = "stopping_runtime"
	EmergencyStateClosingConnections EmergencyStopState = "closing_connections"
	EmergencyStateRevokingLeases     EmergencyStopState = "revoking_leases"
	EmergencyStateCleaningResources  EmergencyStopState = "cleaning_resources"
	EmergencyStateVerifying          EmergencyStopState = "verifying"
	EmergencyStateCompleted          EmergencyStopState = "completed"
	EmergencyStateFailed             EmergencyStopState = "failed"
)

func (s EmergencyStopState) IsTerminal() bool {
	return s == EmergencyStateCompleted || s == EmergencyStateFailed
}

func (s EmergencyStopState) Before(other EmergencyStopState) bool {
	order := map[EmergencyStopState]int{
		EmergencyStateRequested:          0,
		EmergencyStateCommittingIntent:   1,
		EmergencyStateClosingGate:        2,
		EmergencyStateRevokingAuthority:  3,
		EmergencyStateCancelling:         4,
		EmergencyStateStoppingRuntime:    5,
		EmergencyStateClosingConnections: 6,
		EmergencyStateRevokingLeases:     7,
		EmergencyStateCleaningResources:  8,
		EmergencyStateVerifying:          9,
		EmergencyStateCompleted:          10,
		EmergencyStateFailed:            -1,
	}
	return order[s] < order[other]
}

type EmergencyStopRequest struct {
	RuntimeID      domain.RuntimeInstanceID
	Actor          EmergencyStopActor
	Reason         EmergencyStopReason
	IdempotencyKey string
	Deadline       time.Time
	OperationID    string
}

type VerificationResult struct {
	OutputGateClosed        bool
	AuthoritySuspended      bool
	RuntimeStopped          bool
	ConnectionsClosed       bool
	LeasesRevoked           bool
	PendingCleared          bool
	ReadyCleared            bool
	ChannelsCleared         bool
	StreamsCleared          bool
	TransientBinaryReleased bool
	RecoverySuppressed      bool
	EmergencyLatched        bool
	ResidueErrors           []string
}

type EmergencyStopResult struct {
	OperationID     string
	RuntimeID       domain.RuntimeInstanceID
	State           EmergencyStopState
	Actor           EmergencyStopActor
	Reason          EmergencyStopReason
	PreviousMode    domain.ControlMode
	PreviousEpoch   uint64
	NewEpoch        uint64
	StartedAt       time.Time
	FinishedAt      time.Time
	Duration        time.Duration
	Verification    VerificationResult
	CleanupErrors   []error
	CriticalFailure bool
	Residue         []string
}

func (r EmergencyStopResult) Success() bool {
	return r.State == EmergencyStateCompleted && !r.CriticalFailure
}

type StageError struct {
	Stage   EmergencyStopState
	Message string
}

func (e StageError) Error() string {
	return string(e.Stage) + ": " + e.Message
}

type EmergencyIntentStore interface {
	CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error
	IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool
	GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool)
	ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error
}

type PendingVerifier interface {
	CountRuntimePending(runtimeID domain.RuntimeInstanceID) int
}

type ConnectionVerifier interface {
	CountRuntimeConnections(runtimeID domain.RuntimeInstanceID) int
}

type SecretLeaseVerifier interface {
	CountRuntimeLeases(runtimeID domain.RuntimeInstanceID) int
}

type ChannelVerifier interface {
	CountRuntimeChannels(runtimeID domain.RuntimeInstanceID) int
}

type StreamVerifier interface {
	CountRuntimeStreams(runtimeID domain.RuntimeInstanceID) int
}

type BinaryVerifier interface {
	CountRuntimeBinary(runtimeID domain.RuntimeInstanceID) int
}
