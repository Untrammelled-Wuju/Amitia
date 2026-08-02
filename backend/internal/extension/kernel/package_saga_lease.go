package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const packageLeaseTTL = 10 * time.Minute

func (r *Runtime) acquirePackageInProcessLock(extensionID string) func() {
	actual, _ := r.packageLocks.LoadOrStore(extensionID, &sync.Mutex{})
	lock := actual.(*sync.Mutex)
	lock.Lock()
	return func() { lock.Unlock() }
}

func computePackageIdempotencyKey(operationType, extensionID, version, userID, scopeType, scopeID, previewSessionID string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s", operationType, extensionID, version, userID, scopeType, scopeID, previewSessionID)
	hash := sha256.Sum256([]byte(raw))
	return operationType + ":" + hex.EncodeToString(hash[:16])
}

func computeSimplePackageIdempotencyKey(operationType, extensionID, version, userID, scopeType, scopeID string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%s:%s", operationType, extensionID, version, userID, scopeType, scopeID)
	hash := sha256.Sum256([]byte(raw))
	return operationType + ":" + hex.EncodeToString(hash[:16])
}

func computePackageRequestHash(op PackageOperationRecord) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s", op.OperationType, op.ExtensionID, op.TargetVersion, op.ArtifactID, op.PreviewSessionID, op.ScopeType, op.ScopeID)
	hash := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (r *Runtime) acquirePackageExtensionLease(ctx context.Context, extensionID, operationID string) (PackageExtensionLease, error) {
	if r.container == nil || r.container.PackageRepository == nil {
		return PackageExtensionLease{}, fmt.Errorf("kernel: package repository unavailable")
	}
	return r.container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, operationID, packageLeaseTTL)
}

func (r *Runtime) renewPackageExtensionLease(ctx context.Context, extensionID, operationID string) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return fmt.Errorf("kernel: package repository unavailable")
	}
	_, err := r.container.PackageRepository.RenewExtensionLease(ctx, extensionID, operationID, operationID, packageLeaseTTL)
	return err
}

func (r *Runtime) releasePackageExtensionLease(ctx context.Context, extensionID, operationID string) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return fmt.Errorf("kernel: package repository unavailable")
	}
	return r.container.PackageRepository.ReleaseExtensionLease(ctx, extensionID, operationID, operationID)
}

func packageWriteGuard(lease PackageExtensionLease) PackageWriteGuard {
	return PackageWriteGuard{ExtensionID: lease.ExtensionID, FencingToken: lease.FencingToken}
}

type PackageLeaseGuard struct {
	runtime       *Runtime
	extensionID   string
	operationID   string
	cancel        context.CancelFunc
	lastErr       error
	mu            sync.Mutex
	stopped       bool
	leaseReleased bool
	done          chan struct{}
}

func (r *Runtime) newPackageLeaseGuard(extensionID, operationID string) *PackageLeaseGuard {
	return &PackageLeaseGuard{
		runtime:     r,
		extensionID: extensionID,
		operationID: operationID,
		done:        make(chan struct{}),
	}
}

func (g *PackageLeaseGuard) Start(ctx context.Context) (context.Context, error) {
	if g.runtime == nil || g.runtime.container == nil || g.runtime.container.PackageRepository == nil {
		return nil, fmt.Errorf("kernel: package repository unavailable for lease guard")
	}
	sagaCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	interval := packageLeaseTTL / 3
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	go func() {
		defer close(g.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-sagaCtx.Done():
				return
			case <-ticker.C:
				if err := g.runtime.renewPackageExtensionLease(sagaCtx, g.extensionID, g.operationID); err != nil {
					g.mu.Lock()
					g.lastErr = err
					g.mu.Unlock()
					cancel()
					bgCtx := context.Background()
					if setErr := g.runtime.container.PackageRepository.SetOperation(bgCtx, g.operationID, "requires_recovery", "lease_lost", "PACKAGE_LEASE_LOST", err.Error(), false, PackageWriteGuard{}); setErr != nil {
						fmt.Printf("kernel: persist lease lost for %s: %v\n", g.operationID, setErr)
					}
					return
				}
			}
		}
	}()
	return sagaCtx, nil
}

func (g *PackageLeaseGuard) AssertAlive(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastErr != nil {
		return fmt.Errorf("kernel: lease lost: %w", g.lastErr)
	}
	if g.stopped {
		return fmt.Errorf("kernel: lease guard already stopped")
	}
	return g.runtime.renewPackageExtensionLease(ctx, g.extensionID, g.operationID)
}

func (g *PackageLeaseGuard) MarkLeaseReleased() {
	g.mu.Lock()
	g.leaseReleased = true
	g.mu.Unlock()
}

func (g *PackageLeaseGuard) Stop(ctx context.Context) error {
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		return nil
	}
	g.stopped = true
	alreadyReleased := g.leaseReleased
	g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
	}
	select {
	case <-g.done:
	case <-time.After(5 * time.Second):
	}
	g.mu.Lock()
	lastErr := g.lastErr
	g.mu.Unlock()
	if lastErr != nil {
		return fmt.Errorf("kernel: lease lost before stop: %w", lastErr)
	}
	if alreadyReleased {
		return nil
	}
	releaseErr := g.runtime.releasePackageExtensionLease(ctx, g.extensionID, g.operationID)
	if releaseErr != nil {
		if IsPackageOperationError(releaseErr, OperationErrLeaseConflict) {
			return nil
		}
		if putErr := g.runtime.container.PackageRepository.PutConsistencyFinding(context.Background(),
			PackageConsistencyFinding{
				FindingID:         "stale-lease-" + g.operationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   `["` + g.operationID + `"]`,
				ErrorDetail:       releaseErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			}); putErr != nil {
			fmt.Printf("kernel: persist stale lease finding for %s: %v\n", g.operationID, putErr)
		}
		return releaseErr
	}
	return nil
}

func (r *Runtime) handleExistingPackageOperation(ctx context.Context, existing PackageOperationRecord) (KernelInstallResult, bool, error) {
	switch PackageOperationStatus(existing.Status) {
	case PackageOperationCompleted:
		installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(existing.ExtensionID))
		if err != nil {
			return KernelInstallResult{}, true, fmt.Errorf("kernel: idempotent operation completed but installation missing: %w", err)
		}
		artifact, err := r.container.PackageRepository.GetArtifact(ctx, existing.ArtifactID)
		if err != nil {
			return KernelInstallResult{}, true, fmt.Errorf("kernel: idempotent operation completed but artifact missing: %w", err)
		}
		installPath, _ := installation.Metadata["installedPath"].(string)
		return KernelInstallResult{
			OperationID:     existing.OperationID,
			TraceID:         existing.TraceID,
			Operation:       existing.OperationType,
			ExtensionID:     existing.ExtensionID,
			Version:         existing.TargetVersion,
			InstallationID:  installation.InstallationID,
			PackageHash:     artifact.ArchiveHash,
			ContentTreeHash: artifact.ContentTreeHash,
			ArtifactPath:    artifact.ArchivePath,
			InstallPath:     installPath,
			DefinitionHash:  artifact.ArtifactHash,
			InstalledAt:     installation.UpdatedAt,
		}, true, nil
	case PackageOperationFailed:
		return KernelInstallResult{}, true, fmt.Errorf("kernel: idempotent operation previously failed: %s", existing.ErrorDetail)
	case PackageOperationRequiresRecovery:
		return KernelInstallResult{}, true, fmt.Errorf("kernel: idempotent operation requires recovery: %s", existing.ErrorDetail)
	default:
		return KernelInstallResult{}, true, fmt.Errorf("kernel: package operation already in progress: %s (status=%s)", existing.OperationID, existing.Status)
	}
}
