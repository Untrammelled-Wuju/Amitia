package startup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HostIdentityProvider interface {
	GetHostInstanceID() string
	GetHostSessionID() string
}

type ProcessCleanupProvider interface {
	CleanupOwnedProcess(ctx context.Context, instanceID string, pid int) error
	ListOrphanCandidates(ctx context.Context) ([]ProcessCandidate, error)
}

type ProcessCandidate struct {
	PID              int
	ProcessInstanceID string
	RuntimeID        domain.RuntimeInstanceID
	PluginID         domain.PluginID
	ExtensionID      string
	ServiceID        string
	ModuleID         string
	Generation       uint64
	HostInstanceID   string
	HostSessionID    string
	Output           string
}

type TempCleanupProvider interface {
	ListStaleTempCandidates(ctx context.Context) ([]TempCandidate, error)
	RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type TempCandidate struct {
	RuntimeID   domain.RuntimeInstanceID
	Path        string
	PluginID    domain.PluginID
	ExtensionID string
}

type BinaryCleanupProvider interface {
	ListOrphanBinaries(ctx context.Context) ([]BinaryCandidate, error)
	RemoveOrphanBinary(ctx context.Context, binaryID string) error
}

type BinaryCandidate struct {
	BinaryID  string
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID
	Removed   bool
}

type EndpointCleanupProvider interface {
	ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error)
	RemoveStaleEndpoint(ctx context.Context, endpointID string) error
}

type EndpointCandidate struct {
	EndpointID string
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  string
}

type SharedMemoryCleanupProvider interface {
	ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error)
	ReleaseSharedMemory(ctx context.Context, shmID string) error
}

type SharedMemoryCandidate struct {
	ID         string
	RuntimeID  domain.RuntimeInstanceID
	Generation uint64
}

type KernelReconciliationProvider interface {
	CurrentRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error)
	IsValidExtension(ctx context.Context, extensionID string) (bool, error)
	IsExtensionEnabled(ctx context.Context, extensionID string) (bool, error)
	IsValidPlugin(ctx context.Context, pluginID domain.PluginID) (bool, error)
}

type AuditSink interface {
	RecordStartupRecovery(event StartupRecoveryAuditEvent)
}

type StartupRecoveryAuditEvent struct {
	OperationID   StartupRecoveryOperationID
	Stage         StartupRecoveryStage
	ResourceType  ResourceType
	ResourceID    string
	RuntimeID     domain.RuntimeInstanceID
	ExtensionID   string
	Ownership     OwnershipResult
	CleanupResult CleanupResult
	Error         string
	Timestamp     time.Time
}

type RuntimeGraphReconcileProvider interface {
	Reconcile(ctx context.Context) error
}

type StartupRecoveryDeps struct {
	HostIdentity      HostIdentityProvider
	ProcessCleanup    ProcessCleanupProvider
	TempCleanup       TempCleanupProvider
	BinaryCleanup     BinaryCleanupProvider
	EndpointCleanup   EndpointCleanupProvider
	ShmCleanup        SharedMemoryCleanupProvider
	KernelRecon       KernelReconciliationProvider
	RuntimeGraphRecon RuntimeGraphReconcileProvider
	AuditSink         AuditSink
	Gate              *StartupGate
}

type StartupRecoveryCoordinator struct {
	deps    StartupRecoveryDeps
	mu      sync.Mutex
	running bool
}

func NewStartupRecoveryCoordinator(deps StartupRecoveryDeps) (*StartupRecoveryCoordinator, error) {
	if deps.Gate == nil {
		return nil, fmt.Errorf("startup recovery: Gate is required")
	}
	if deps.ProcessCleanup == nil {
		return nil, fmt.Errorf("startup recovery: ProcessCleanup is required")
	}
	if deps.TempCleanup == nil {
		return nil, fmt.Errorf("startup recovery: TempCleanup is required")
	}
	if deps.BinaryCleanup == nil {
		return nil, fmt.Errorf("startup recovery: BinaryCleanup is required")
	}
	if deps.EndpointCleanup == nil {
		return nil, fmt.Errorf("startup recovery: EndpointCleanup is required")
	}
	if deps.ShmCleanup == nil {
		return nil, fmt.Errorf("startup recovery: ShmCleanup is required")
	}
	if deps.KernelRecon == nil {
		return nil, fmt.Errorf("startup recovery: KernelRecon is required")
	}
	if deps.HostIdentity == nil {
		return nil, fmt.Errorf("startup recovery: HostIdentity is required")
	}
	if deps.AuditSink == nil {
		return nil, fmt.Errorf("startup recovery: AuditSink is required")
	}
	return &StartupRecoveryCoordinator{deps: deps}, nil
}

func (c *StartupRecoveryCoordinator) RunStartupRecovery(ctx context.Context) StartupRecoveryReport {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return StartupRecoveryReport{
			Success: false,
			Errors:  []string{"startup recovery already running"},
		}
	}
	c.running = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	operationID := generateStartupOperationID()
	report := StartupRecoveryReport{
		OperationID: operationID,
		StartedAt:   time.Now(),
		Stage:       StageLoadOwnership,
	}

	if c.deps.Gate != nil {
		c.deps.Gate.Close()
	}

	hostID := ""
	if c.deps.HostIdentity != nil {
		hostID = c.deps.HostIdentity.GetHostInstanceID()
	}

	c.runStage(ctx, &report, StageReconcileKernel, func() error {
		return c.reconcileKernel(ctx, &report, hostID)
	})

	c.runStage(ctx, &report, StageClassifyOrphans, func() error {
		return c.classifyOrphans(ctx, &report, hostID)
	})

	c.runStage(ctx, &report, StageCleanup, func() error {
		return c.cleanupOrphans(ctx, &report)
	})

	c.runStage(ctx, &report, StageReconstruct, func() error {
		return c.reconstructRuntimes(ctx, &report)
	})

	if !report.Degraded {
		report.Stage = StageCompleted
		report.Success = true
		if c.deps.Gate != nil {
			c.deps.Gate.Open()
		}
	} else {
		report.Stage = StageFailed
	}

	completedAt := time.Now()
	report.CompletedAt = &completedAt

	if c.deps.Gate != nil {
		c.deps.Gate.SetReport(report)
	}

	log.Printf("[startup-recovery] operationID=%s success=%v degraded=%v cleaned=%d skipped=%d errors=%d",
		report.OperationID, report.Success, report.Degraded, len(report.Cleaned), len(report.Skipped), len(report.Errors))

	return report
}

func (c *StartupRecoveryCoordinator) runStage(ctx context.Context, report *StartupRecoveryReport, stage StartupRecoveryStage, fn func() error) {
	if report.Degraded && stage != StageCompleted {
		return
	}
	report.Stage = stage
	err := fn()
	if err != nil {
		report.Degraded = true
		report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", stage, err))
		log.Printf("[startup-recovery] stage error: %s: %v", stage, err)
	}
}

func (c *StartupRecoveryCoordinator) reconcileKernel(ctx context.Context, report *StartupRecoveryReport, _ string) error {
	if c.deps.KernelRecon == nil {
		return nil
	}
	_, err := c.deps.KernelRecon.CurrentRuntimeIDs(ctx)
	return err
}

func (c *StartupRecoveryCoordinator) classifyOrphans(ctx context.Context, report *StartupRecoveryReport, hostID string) error {
	candidates := make([]*OrphanResource, 0)

	if c.deps.ProcessCleanup != nil {
		procs, err := c.deps.ProcessCleanup.ListOrphanCandidates(ctx)
		if err != nil {
			return fmt.Errorf("list process candidates: %w", err)
		}
		for _, pc := range procs {
			proof := OwnershipProof{
				HostInstanceID: pc.HostInstanceID,
				RuntimeID:      pc.RuntimeID,
				PluginID:       pc.PluginID,
				ExtensionID:    pc.ExtensionID,
				Generation:     pc.Generation,
			}
			result := c.verifyProcessOwnership(ctx, pc, proof)
			if result == OwnershipUnknown || result == OwnershipBelongsToForeign {
				log.Printf("[startup-recovery] skip process candidate pid=%d runtime=%s extension=%s ownership=%s",
					pc.PID, pc.RuntimeID, pc.ExtensionID, result)
				continue
			}
		candidates = append(candidates, &OrphanResource{
			Type:        ResourceOrphanProcess,
			ResourceID:  fmt.Sprintf("pid-%d", pc.PID),
			ExtensionID: pc.ExtensionID,
			PluginID:    pc.PluginID,
			RuntimeID:   pc.RuntimeID,
			ServiceName: pc.ProcessInstanceID,
			Generation:  pc.Generation,
			Ownership:   proof,
			Path:        pc.Output,
		})
		}
	}

	if c.deps.TempCleanup != nil {
		temps, err := c.deps.TempCleanup.ListStaleTempCandidates(ctx)
		if err != nil {
			return fmt.Errorf("list temp candidates: %w", err)
		}
		for _, tc := range temps {
			candidates = append(candidates, &OrphanResource{
				Type:        ResourceStaleTemp,
				ResourceID:  tc.Path,
				ExtensionID: tc.ExtensionID,
				PluginID:    tc.PluginID,
				RuntimeID:   tc.RuntimeID,
				Path:        tc.Path,
			})
		}
	}

	if c.deps.BinaryCleanup != nil {
		binaries, err := c.deps.BinaryCleanup.ListOrphanBinaries(ctx)
		if err != nil {
			return fmt.Errorf("list binary candidates: %w", err)
		}
		for _, bc := range binaries {
			candidates = append(candidates, &OrphanResource{
				Type:       ResourceStaleBinary,
				ResourceID: bc.BinaryID,
				PluginID:   bc.PluginID,
				RuntimeID:  bc.RuntimeID,
			})
		}
	}

	if c.deps.EndpointCleanup != nil {
		endpoints, err := c.deps.EndpointCleanup.ListStaleEndpoints(ctx)
		if err != nil {
			if isNotApplicable(err) {
				log.Printf("[startup-recovery] endpoint cleanup NOT_APPLICABLE: %v", err)
			} else {
				return fmt.Errorf("list endpoint candidates: %w", err)
			}
		} else {
			for _, ec := range endpoints {
				candidates = append(candidates, &OrphanResource{
					Type:        ResourceStaleEndpoint,
					ResourceID:  ec.EndpointID,
					RuntimeID:   ec.RuntimeID,
					ServiceName: ec.ServiceID,
				})
			}
		}
	}

	if c.deps.ShmCleanup != nil {
		shms, err := c.deps.ShmCleanup.ListStaleSharedMemory(ctx)
		if err != nil {
			if isNotApplicable(err) {
				log.Printf("[startup-recovery] shared memory cleanup NOT_APPLICABLE: %v", err)
			} else {
				return fmt.Errorf("list shared memory candidates: %w", err)
			}
		} else {
			for _, sc := range shms {
				candidates = append(candidates, &OrphanResource{
					Type:       ResourceStaleSharedMem,
					ResourceID: sc.ID,
					RuntimeID:  sc.RuntimeID,
					Generation: sc.Generation,
				})
			}
		}
	}

	report.Candidates = candidates
	return nil
}

func isNotApplicable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "NOT_APPLICABLE")
}

func (c *StartupRecoveryCoordinator) cleanupOrphans(ctx context.Context, report *StartupRecoveryReport) error {
	if len(report.Candidates) == 0 {
		return nil
	}

	for _, candidate := range report.Candidates {
		result := c.cleanupOneCandidate(ctx, candidate)
		switch result {
		case CleanupSuccess, CleanupNoOp:
			report.Cleaned = append(report.Cleaned, candidate)
			c.recordAudit(report, candidate, "", OwnershipVerified, CleanupSuccess, nil)
		case CleanupSkipped:
			report.Skipped = append(report.Skipped, candidate)
			c.recordAudit(report, candidate, "", OwnershipUnknown, CleanupSkipped, nil)
		case CleanupFailed:
			report.Degraded = true
			report.Errors = append(report.Errors, fmt.Sprintf("cleanup failed for %s/%s", candidate.Type, candidate.ResourceID))
			c.recordAudit(report, candidate, "cleanup failed", OwnershipUnknown, CleanupFailed, nil)
		}
	}

	return nil
}

func (c *StartupRecoveryCoordinator) cleanupOneCandidate(ctx context.Context, candidate *OrphanResource) CleanupResult {
	switch candidate.Type {
	case ResourceOrphanProcess:
		if c.deps.ProcessCleanup == nil {
			return CleanupSkipped
		}
		return c.cleanupProcessCandidate(ctx, candidate)

	case ResourceStaleTemp:
		if c.deps.TempCleanup == nil {
			return CleanupSkipped
		}
		if err := c.deps.TempCleanup.RemoveStaleTemp(ctx, candidate.RuntimeID); err != nil {
			log.Printf("[startup-recovery] failed to remove stale temp %s: %v", candidate.Path, err)
			return CleanupFailed
		}
		return CleanupSuccess

	case ResourceStaleBinary:
		if c.deps.BinaryCleanup == nil {
			return CleanupSkipped
		}
		if err := c.deps.BinaryCleanup.RemoveOrphanBinary(ctx, candidate.ResourceID); err != nil {
			log.Printf("[startup-recovery] failed to remove binary %s: %v", candidate.ResourceID, err)
			return CleanupFailed
		}
		return CleanupSuccess

	case ResourceStaleEndpoint:
		if c.deps.EndpointCleanup == nil {
			return CleanupSkipped
		}
		if err := c.deps.EndpointCleanup.RemoveStaleEndpoint(ctx, candidate.ResourceID); err != nil {
			log.Printf("[startup-recovery] failed to remove endpoint %s: %v", candidate.ResourceID, err)
			return CleanupFailed
		}
		return CleanupSuccess

	case ResourceStaleSharedMem:
		if c.deps.ShmCleanup == nil {
			return CleanupSkipped
		}
		if err := c.deps.ShmCleanup.ReleaseSharedMemory(ctx, candidate.ResourceID); err != nil {
			log.Printf("[startup-recovery] failed to release shared memory %s: %v", candidate.ResourceID, err)
			return CleanupFailed
		}
		return CleanupSuccess

	default:
		return CleanupSkipped
	}
}

func (c *StartupRecoveryCoordinator) cleanupProcessCandidate(ctx context.Context, candidate *OrphanResource) CleanupResult {
	if c.deps.KernelRecon != nil && candidate.ExtensionID != "" {
		valid, err := c.deps.KernelRecon.IsValidExtension(ctx, candidate.ExtensionID)
		if err == nil && !valid {
			return CleanupSkipped
		}
	}
	if c.deps.ProcessCleanup != nil {
		pid := extractPID(candidate.ResourceID)
		if pid > 0 {
			instanceID := candidate.ServiceName
			if instanceID == "" {
				instanceID = string(candidate.RuntimeID)
			}
			err := c.deps.ProcessCleanup.CleanupOwnedProcess(ctx, instanceID, pid)
			if err != nil {
				log.Printf("[startup-recovery] process cleanup failed for %s: %v", candidate.ResourceID, err)
				return CleanupFailed
			}
		}
	}
	return CleanupSuccess
}

func (c *StartupRecoveryCoordinator) reconstructRuntimes(ctx context.Context, report *StartupRecoveryReport) error {
	if c.deps.RuntimeGraphRecon != nil {
		if err := c.deps.RuntimeGraphRecon.Reconcile(ctx); err != nil {
			return fmt.Errorf("runtime graph reconcile: %w", err)
		}
	}
	if c.deps.KernelRecon == nil {
		return nil
	}
	runtimeIDs, err := c.deps.KernelRecon.CurrentRuntimeIDs(ctx)
	if err != nil {
		return fmt.Errorf("list current runtime IDs: %w", err)
	}
	report.ReconstructedRuntimes = runtimeIDs
	return nil
}

func (c *StartupRecoveryCoordinator) verifyProcessOwnership(ctx context.Context, candidate ProcessCandidate, proof OwnershipProof) OwnershipResult {
	if c.deps.HostIdentity == nil {
		return OwnershipUnknown
	}
	actualHostID := c.deps.HostIdentity.GetHostInstanceID()
	if actualHostID == "" || proof.HostInstanceID == "" {
		return OwnershipUnknown
	}
	if proof.HostInstanceID != actualHostID {
		return OwnershipBelongsToForeign
	}
	if proof.RuntimeID == "" && proof.PluginID == "" {
		return OwnershipUnknown
	}
	if c.deps.KernelRecon != nil && proof.ExtensionID != "" {
		if valid, err := c.deps.KernelRecon.IsValidExtension(ctx, proof.ExtensionID); err == nil && !valid {
			return OwnershipUnknown
		}
	}
	return OwnershipVerified
}

func (c *StartupRecoveryCoordinator) recordAudit(report *StartupRecoveryReport, resource *OrphanResource, errMsg string, ownership OwnershipResult, result CleanupResult, err error) {
	if c.deps.AuditSink == nil {
		return
	}
	e := errMsg
	if err != nil {
		e = err.Error()
	}
	c.deps.AuditSink.RecordStartupRecovery(StartupRecoveryAuditEvent{
		OperationID:   report.OperationID,
		Stage:         report.Stage,
		ResourceType:  resource.Type,
		ResourceID:    resource.ResourceID,
		RuntimeID:     resource.RuntimeID,
		ExtensionID:   resource.ExtensionID,
		Ownership:     ownership,
		CleanupResult: result,
		Error:         e,
		Timestamp:     time.Now(),
	})
}

func generateStartupOperationID() StartupRecoveryOperationID {
	input := fmt.Sprintf("startup-%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(input))
	return StartupRecoveryOperationID("startup-" + hex.EncodeToString(hash[:8]))
}

func extractPID(resourceID string) int {
	if len(resourceID) < 4 || resourceID[:4] != "pid-" {
		return 0
	}
	var pid int
	_, err := fmt.Sscanf(resourceID[4:], "%d", &pid)
	if err != nil {
		return 0
	}
	return pid
}
