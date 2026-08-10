package control

import (
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
	EmergencyStateRequested         EmergencyStopState = "requested"
	EmergencyStateClosingGate       EmergencyStopState = "closing_gate"
	EmergencyStateRevokingAuthority EmergencyStopState = "revoking_authority"
	EmergencyStateCancelling        EmergencyStopState = "cancelling"
	EmergencyStateStoppingRuntime   EmergencyStopState = "stopping_runtime"
	EmergencyStateClosingConnections EmergencyStopState = "closing_connections"
	EmergencyStateRevokingLeases    EmergencyStopState = "revoking_leases"
	EmergencyStateCleaningResources EmergencyStopState = "cleaning_resources"
	EmergencyStateVerifying         EmergencyStopState = "verifying"
	EmergencyStateCompleted         EmergencyStopState = "completed"
	EmergencyStateFailed            EmergencyStopState = "failed"
)

func (s EmergencyStopState) IsTerminal() bool {
	return s == EmergencyStateCompleted || s == EmergencyStateFailed
}

func (s EmergencyStopState) Before(other EmergencyStopState) bool {
	order := map[EmergencyStopState]int{
		EmergencyStateRequested:          0,
		EmergencyStateClosingGate:        1,
		EmergencyStateRevokingAuthority:  2,
		EmergencyStateCancelling:         3,
		EmergencyStateStoppingRuntime:    4,
		EmergencyStateClosingConnections: 5,
		EmergencyStateRevokingLeases:    6,
		EmergencyStateCleaningResources:  7,
		EmergencyStateVerifying:          8,
		EmergencyStateCompleted:          9,
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
	Force          bool
	OperationID    string
}

type VerificationResult struct {
	OutputGateClosed     bool
	AuthoritySuspended   bool
	RuntimeStopped       bool
	ConnectionsClosed    bool
	LeasesRevoked        bool
	PendingCleared       bool
	ProcessCleanedUp     bool
	ResidueErrors        []string
}

type EmergencyStopResult struct {
	OperationID       string
	RuntimeID         domain.RuntimeInstanceID
	State             EmergencyStopState
	Actor             EmergencyStopActor
	Reason            EmergencyStopReason
	PreviousMode      domain.ControlMode
	PreviousEpoch     uint64
	NewEpoch          uint64
	StartedAt         time.Time
	FinishedAt        time.Time
	Duration          time.Duration
	Verification      VerificationResult
	CleanupErrors     []error
	CriticalFailure   bool
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
