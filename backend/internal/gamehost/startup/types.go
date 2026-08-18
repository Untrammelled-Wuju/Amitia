package startup

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ResourceType string

const (
	ResourceOrphanProcess   ResourceType = "orphan_process"
	ResourceStaleTemp       ResourceType = "stale_temp"
	ResourceStaleBinary     ResourceType = "stale_binary"
	ResourceStaleSharedMem  ResourceType = "stale_shared_memory"
	ResourceStaleEndpoint   ResourceType = "stale_endpoint"
	ResourceStaleCheckpoint ResourceType = "stale_checkpoint"
)

type OwnershipResult string

const (
	OwnershipVerified         OwnershipResult = "verified"
	OwnershipUnknown          OwnershipResult = "unknown"
	OwnershipBelongsToForeign OwnershipResult = "foreign"
)

type CleanupResult string

const (
	CleanupSuccess CleanupResult = "success"
	CleanupSkipped CleanupResult = "skipped"
	CleanupFailed  CleanupResult = "failed"
	CleanupNoOp    CleanupResult = "noop"
)

type OrphanResource struct {
	Type        ResourceType
	ResourceID  string
	ExtensionID string
	PluginID    domain.PluginID
	RuntimeID   domain.RuntimeInstanceID
	ServiceName string
	Generation  uint64
	Path        string
	Ownership   OwnershipProof
}

type OwnershipProof struct {
	HostInstanceID string
	RuntimeID      domain.RuntimeInstanceID
	PluginID       domain.PluginID
	ExtensionID    string
	ServiceID      string
	Generation     uint64
	ResourceToken  string
	Executable     string
	ProcessStartID string
}

type StartupRecoveryStage string

const (
	StageLoadOwnership   StartupRecoveryStage = "load_ownership"
	StageReconcileKernel StartupRecoveryStage = "reconcile_kernel"
	StageClassifyOrphans StartupRecoveryStage = "classify_orphans"
	StageCleanup         StartupRecoveryStage = "cleanup"
	StageReconstruct     StartupRecoveryStage = "reconstruct"
	StageOpenGate        StartupRecoveryStage = "open_gate"
	StageCompleted       StartupRecoveryStage = "completed"
	StageFailed          StartupRecoveryStage = "failed"
)

type StartupRecoveryOperationID string

type StartupRecoveryReport struct {
	OperationID           StartupRecoveryOperationID
	StartedAt             time.Time
	CompletedAt           *time.Time
	Stage                 StartupRecoveryStage
	Candidates            []*OrphanResource
	Cleaned               []*OrphanResource
	Skipped               []*OrphanResource
	ReconstructedRuntimes []domain.RuntimeInstanceID
	Errors                []string
	Success               bool
	Degraded              bool
}

type StartupGate struct {
	mu       sync.RWMutex
	ready    bool
	recovery StartupRecoveryReport
}

func NewStartupGate() *StartupGate {
	return &StartupGate{ready: false}
}

func (g *StartupGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ready = false
}

func (g *StartupGate) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ready = true
}

func (g *StartupGate) IsReady() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.ready
}

func (g *StartupGate) SetReport(report StartupRecoveryReport) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recovery = report
}

func (g *StartupGate) GetReport() StartupRecoveryReport {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.recovery
}
