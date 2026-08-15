package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type KernelExtensionLifecycle interface {
	ExecuteUpdate(ctx context.Context, extensionID string, targetVersion string, operationID UpgradeOperationID) (*KernelUpdateResult, error)
}

type KernelArchiveUpdater interface {
	UpdateArchive(ctx context.Context, extensionID string, archivePath string) (*KernelUpdateResult, error)
}

type KernelUpdateResult struct {
	Success    bool
	NewVersion string
	Reason     string
}

type PluginRegistryReader interface {
	ListByExtension(ctx context.Context, extensionID string) ([]domain.PluginDescriptor, error)
	Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error)
	Snapshot() []domain.PluginDescriptor
	Count() int
}

type RuntimeManagerReader interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error)
	ListRuntimes() []*runtime.RuntimeInstanceRef
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
}

type RuntimeLifecycleIntentManager interface {
	GetLifecycleIntent(runtimeID domain.RuntimeInstanceID) (string, error)
	SetLifecycleIntent(runtimeID domain.RuntimeInstanceID, intent string) error
}

type DefinitionReconciler interface {
	ReconcileExtension(extensionID string) *service_definition.ReconcileReport
}

type RuntimeGraphReconciler interface {
	ReconcileExtension(ctx context.Context, extensionID string) error
}

type ContributionReconciler interface {
	SyncExtension(ctx context.Context, extensionID string) integration.SyncResult
}

type ConfigValidator interface {
	Resolve(ctx context.Context, pluginID, runtimeID, serviceID string) (*config.ScopedConfig, []config.ValidationError)
}

type UpgradeCoordinator struct {
	mu                    sync.Mutex
	gate                  *UpgradeGate
	pluginReg             PluginRegistryReader
	runtimeManager        RuntimeManagerReader
	lifecycleIntents      RuntimeLifecycleIntentManager
	runtimeExecutor       runtime.RuntimeExecutor
	definitionReconcile   DefinitionReconciler
	runtimeGraphReconcile RuntimeGraphReconciler
	contributionReconcile ContributionReconciler
	configValidator       ConfigValidator
	migrationHooks        *MigrationHookRegistry
	kernelLifecycle       KernelExtensionLifecycle
	archiveUpdater        KernelArchiveUpdater
}

func NewUpgradeCoordinator(
	pluginReg PluginRegistryReader,
	runtimeManager RuntimeManagerReader,
	runtimeExecutor runtime.RuntimeExecutor,
	definitionReconcile DefinitionReconciler,
	runtimeGraphReconcile RuntimeGraphReconciler,
	contributionReconcile ContributionReconciler,
	configValidator ConfigValidator,
	kernelLifecycle KernelExtensionLifecycle,
	archiveUpdater KernelArchiveUpdater,
) (*UpgradeCoordinator, error) {
	if pluginReg == nil {
		return nil, fmt.Errorf("upgrade coordinator: pluginReg is required")
	}
	if runtimeManager == nil {
		return nil, fmt.Errorf("upgrade coordinator: runtimeManager is required")
	}
	if runtimeExecutor == nil {
		return nil, fmt.Errorf("upgrade coordinator: runtimeExecutor is required")
	}
	if definitionReconcile == nil {
		return nil, fmt.Errorf("upgrade coordinator: definitionReconcile is required")
	}
	if runtimeGraphReconcile == nil {
		return nil, fmt.Errorf("upgrade coordinator: runtimeGraphReconcile is required")
	}
	if contributionReconcile == nil {
		return nil, fmt.Errorf("upgrade coordinator: contributionReconcile is required")
	}
	if configValidator == nil {
		return nil, fmt.Errorf("upgrade coordinator: configValidator is required")
	}
	if archiveUpdater == nil {
		return nil, fmt.Errorf("upgrade coordinator: archiveUpdater is required")
	}
	intents, ok := runtimeManager.(RuntimeLifecycleIntentManager)
	if !ok {
		return nil, fmt.Errorf("upgrade coordinator: runtimeManager must manage lifecycle intents")
	}

	return &UpgradeCoordinator{
		gate:                  NewUpgradeGate(),
		pluginReg:             pluginReg,
		runtimeManager:        runtimeManager,
		lifecycleIntents:      intents,
		runtimeExecutor:       runtimeExecutor,
		definitionReconcile:   definitionReconcile,
		runtimeGraphReconcile: runtimeGraphReconcile,
		contributionReconcile: contributionReconcile,
		configValidator:       configValidator,
		migrationHooks:        NewMigrationHookRegistry(),
		kernelLifecycle:       kernelLifecycle,
		archiveUpdater:        archiveUpdater,
	}, nil
}

func (c *UpgradeCoordinator) RegisterMigrationHook(extensionID string, hook MigrationHook) {
	c.migrationHooks.Register(extensionID, hook)
}

func (c *UpgradeCoordinator) IsExtensionUpgrading(extensionID string) bool {
	return c.gate.IsUpgrading(extensionID)
}

func (c *UpgradeCoordinator) ExecuteUpgrade(ctx context.Context, req UpgradeRequest) (*UpgradeResult, error) {
	operationID := generateUpgradeOperationID(req.ExtensionID)
	result := &UpgradeResult{
		OperationID: operationID,
		ExtensionID: req.ExtensionID,
		Stage:       UpgradeStatePreparing,
	}
	c.logStage(operationID, req.ExtensionID, UpgradeStatePreparing, "start")

	if err := c.gate.Acquire(req.ExtensionID, operationID); err != nil {
		result.Error = err
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, err)
		return result, err
	}
	defer c.gate.Release(req.ExtensionID, operationID)

	affectedPlugins, err := c.findAffectedPlugins(ctx, req.ExtensionID)
	if err != nil {
		result.Error = fmt.Errorf("preflight: find affected plugins: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}
	result.AffectedPlugins = affectedPlugins

	runtimeSnapshots, err := c.discoverRuntimes(affectedPlugins)
	if err != nil {
		result.Error = fmt.Errorf("preflight: discover runtimes: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	result.Stage = UpgradeStateQuiescing
	c.logStage(operationID, req.ExtensionID, UpgradeStateQuiescing, fmt.Sprintf("runtimes=%d", len(runtimeSnapshots)))
	if err := c.quiesceRuntimes(ctx, runtimeSnapshots); err != nil {
		result.Error = fmt.Errorf("quiesce failed: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}
	for _, snap := range runtimeSnapshots {
		result.QuiescedRuntimes = append(result.QuiescedRuntimes, snap.RuntimeID)
	}

	result.Stage = UpgradeStateUpdating
	c.logStage(operationID, req.ExtensionID, UpgradeStateUpdating, "")
	if c.kernelLifecycle != nil {
		kernelResult, kerr := c.kernelLifecycle.ExecuteUpdate(ctx, req.ExtensionID, req.TargetVersion, operationID)
		if kerr != nil || (kernelResult != nil && !kernelResult.Success) {
			err := kerr
			if err == nil && kernelResult != nil {
				err = fmt.Errorf("kernel update failed: %s", kernelResult.Reason)
			}
			if err == nil {
				err = fmt.Errorf("kernel update failed: unknown reason")
			}
			c.clearUpgradeIntent(runtimeSnapshots)
			result.Error = fmt.Errorf("kernel package update failed: %w", err)
			result.Stage = UpgradeStateFailed
			c.recordAudit(operationID, req, result, result.Error)
			return result, result.Error
		}
	}

	result.Stage = UpgradeStateMigrating
	c.logStage(operationID, req.ExtensionID, UpgradeStateMigrating, "")
	if err := c.executeMigrationHooks(ctx, req, operationID, runtimeSnapshots); err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("migration failed: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	result.Stage = UpgradeStateReconciling
	c.logStage(operationID, req.ExtensionID, UpgradeStateReconciling, "")

	descSync := c.contributionReconcile.SyncExtension(ctx, req.ExtensionID)
	if descSync.HasError() {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("contribution sync failed: %v", descSync.Errors)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	if err := c.runtimeGraphReconcile.ReconcileExtension(ctx, req.ExtensionID); err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("runtime graph reconcile failed: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	defReport := c.definitionReconcile.ReconcileExtension(req.ExtensionID)
	if len(defReport.Errors) > 0 {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("definition reconcile failed: %v", defReport.Errors)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	currentSnapshots, err := c.rediscoverCurrentRuntimes(ctx, req.ExtensionID, runtimeSnapshots)
	if err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("current runtime rediscovery failed: %w", err)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	configErrors := c.reconcileConfigs(ctx, currentSnapshots)
	if len(configErrors) > 0 {
		c.clearUpgradeIntent(runtimeSnapshots)
		result.Error = fmt.Errorf("config validation failed: %v", configErrors)
		result.Stage = UpgradeStateFailed
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	result.Stage = UpgradeStateResuming
	c.logStage(operationID, req.ExtensionID, UpgradeStateResuming, fmt.Sprintf("runtimes=%d", len(runtimeSnapshots)))
	resumeFailures := c.resumeRuntimes(ctx, currentSnapshots)
	result.ResumedRuntimes = make([]domain.RuntimeInstanceID, 0)
	result.FailedRuntimes = make([]domain.RuntimeInstanceID, 0)
	for _, snap := range currentSnapshots {
		if snap.WasRunning || snap.WasSuspended {
			if _, failed := resumeFailures[snap.RuntimeID]; failed {
				result.FailedRuntimes = append(result.FailedRuntimes, snap.RuntimeID)
			} else {
				result.ResumedRuntimes = append(result.ResumedRuntimes, snap.RuntimeID)
			}
		}
	}

	if len(result.FailedRuntimes) > 0 {
		result.Error = fmt.Errorf("partial resume failure: failed=%d", len(result.FailedRuntimes))
		result.Stage = UpgradeStateFailed
		result.Success = false
		c.recordAudit(operationID, req, result, result.Error)
		return result, result.Error
	}

	result.Stage = UpgradeStateCompleted
	result.Success = true
	c.clearUpgradeIntent(runtimeSnapshots)
	c.logStage(operationID, req.ExtensionID, UpgradeStateCompleted, fmt.Sprintf("resumed=%d", len(result.ResumedRuntimes)))
	c.recordAudit(operationID, req, result, nil)
	return result, nil
}

func (c *UpgradeCoordinator) ExecuteUpgradeByArchive(ctx context.Context, extensionID, archivePath string) error {
	operationID := generateUpgradeOperationID(extensionID)
	result := &UpgradeResult{
		OperationID: operationID,
		ExtensionID: extensionID,
		Stage:       UpgradeStatePreparing,
	}
	c.logStage(operationID, extensionID, UpgradeStatePreparing, "start")

	if err := c.gate.Acquire(extensionID, result.OperationID); err != nil {
		return err
	}
	defer c.gate.Release(extensionID, result.OperationID)

	affectedPlugins, err := c.findAffectedPlugins(ctx, extensionID)
	if err != nil {
		return fmt.Errorf("preflight: find affected plugins: %w", err)
	}
	result.AffectedPlugins = affectedPlugins

	runtimeSnapshots, err := c.discoverRuntimes(affectedPlugins)
	if err != nil {
		return fmt.Errorf("preflight: discover runtimes: %w", err)
	}

	result.Stage = UpgradeStateQuiescing
	c.logStage(result.OperationID, extensionID, UpgradeStateQuiescing, fmt.Sprintf("runtimes=%d", len(runtimeSnapshots)))
	if err := c.quiesceRuntimes(ctx, runtimeSnapshots); err != nil {
		return fmt.Errorf("quiesce failed: %w", err)
	}
	for _, snap := range runtimeSnapshots {
		result.QuiescedRuntimes = append(result.QuiescedRuntimes, snap.RuntimeID)
	}

	result.Stage = UpgradeStateUpdating
	c.logStage(result.OperationID, extensionID, UpgradeStateUpdating, "")
	kernelResult, kerr := c.archiveUpdater.UpdateArchive(ctx, extensionID, archivePath)
	if kerr != nil || (kernelResult != nil && !kernelResult.Success) {
		err := kerr
		if err == nil && kernelResult != nil {
			err = fmt.Errorf("kernel update failed: %s", kernelResult.Reason)
		}
		if err == nil {
			err = fmt.Errorf("kernel archive update failed: unknown reason")
		}
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("kernel package update failed: %w", err)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	result.Stage = UpgradeStateMigrating
	c.logStage(result.OperationID, extensionID, UpgradeStateMigrating, "")
	if err := c.executeMigrationHooks(ctx, UpgradeRequest{ExtensionID: extensionID}, result.OperationID, runtimeSnapshots); err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("migration failed: %w", err)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	result.Stage = UpgradeStateReconciling
	c.logStage(result.OperationID, extensionID, UpgradeStateReconciling, "")

	descSync := c.contributionReconcile.SyncExtension(ctx, extensionID)
	if descSync.HasError() {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("contribution sync failed: %v", descSync.Errors)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	if err := c.runtimeGraphReconcile.ReconcileExtension(ctx, extensionID); err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("runtime graph reconcile failed: %w", err)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	defReport := c.definitionReconcile.ReconcileExtension(extensionID)
	if len(defReport.Errors) > 0 {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("definition reconcile failed: %v", defReport.Errors)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	currentSnapshots, err := c.rediscoverCurrentRuntimes(ctx, extensionID, runtimeSnapshots)
	if err != nil {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("current runtime rediscovery failed: %w", err)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	configErrors := c.reconcileConfigs(ctx, currentSnapshots)
	if len(configErrors) > 0 {
		c.clearUpgradeIntent(runtimeSnapshots)
		archiveErr := fmt.Errorf("config validation failed: %v", configErrors)
		c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
		return archiveErr
	}

	result.Stage = UpgradeStateResuming
	c.logStage(result.OperationID, extensionID, UpgradeStateResuming, fmt.Sprintf("runtimes=%d", len(runtimeSnapshots)))
	resumeFailures := c.resumeRuntimes(ctx, currentSnapshots)
	for _, snap := range currentSnapshots {
		if snap.WasRunning || snap.WasSuspended {
			if _, failed := resumeFailures[snap.RuntimeID]; failed {
				c.clearUpgradeIntent(runtimeSnapshots)
				archiveErr := fmt.Errorf("partial resume failure: runtime=%s", snap.RuntimeID)
				c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, archiveErr)
				return archiveErr
			}
		}
	}

	result.Stage = UpgradeStateCompleted
	result.Success = true
	c.clearUpgradeIntent(runtimeSnapshots)
	c.logStage(result.OperationID, extensionID, UpgradeStateCompleted, "")
	c.recordAudit(result.OperationID, UpgradeRequest{ExtensionID: extensionID}, result, nil)
	return nil
}

func (c *UpgradeCoordinator) findAffectedPlugins(ctx context.Context, extensionID string) ([]domain.PluginID, error) {
	descriptors, err := c.pluginReg.ListByExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	pluginIDs := make([]domain.PluginID, 0, len(descriptors))
	for _, desc := range descriptors {
		pluginIDs = append(pluginIDs, desc.ID)
	}
	sort.Slice(pluginIDs, func(i, j int) bool {
		return pluginIDs[i] < pluginIDs[j]
	})
	return pluginIDs, nil
}

func (c *UpgradeCoordinator) discoverRuntimes(pluginIDs []domain.PluginID) ([]RuntimeUpgradeSnapshot, error) {
	affectedSet := make(map[domain.PluginID]struct{}, len(pluginIDs))
	for _, id := range pluginIDs {
		affectedSet[id] = struct{}{}
	}

	allRuntimes := c.runtimeManager.ListRuntimes()
	snapshots := make([]RuntimeUpgradeSnapshot, 0, len(allRuntimes))
	for _, rt := range allRuntimes {
		if _, affected := affectedSet[rt.PluginID]; !affected {
			continue
		}

		snap := RuntimeUpgradeSnapshot{
			RuntimeID:    rt.ID,
			PluginID:     rt.PluginID,
			RuntimeState: rt.State,
			WasRunning:   domain.IsActiveRuntimeState(rt.State),
			WasSuspended: rt.State == domain.RuntimeStateSuspended,
		}

		if gen, err := c.runtimeManager.GetCurrentGeneration(rt.ID); err == nil {
			snap.PreUpgradeGeneration = gen
		}

		snapshots = append(snapshots, snap)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].RuntimeID < snapshots[j].RuntimeID
	})
	return snapshots, nil
}

func (c *UpgradeCoordinator) quiesceRuntimes(ctx context.Context, snapshots []RuntimeUpgradeSnapshot) error {
	committed := make([]domain.RuntimeInstanceID, 0, len(snapshots))
	var quiesceErr error

	for i := range snapshots {
		snap := &snapshots[i]
		rtInfo, err := c.runtimeManager.GetRuntime(snap.RuntimeID)
		if err != nil {
			quiesceErr = fmt.Errorf("get runtime %s: %w", snap.RuntimeID, err)
			break
		}
		if domain.IsTerminalRuntimeState(rtInfo.State) {
			continue
		}
		if err := c.lifecycleIntents.SetLifecycleIntent(snap.RuntimeID, "upgrade"); err != nil {
			quiesceErr = fmt.Errorf("set upgrade intent for runtime %s: %w", snap.RuntimeID, err)
			break
		}
		committed = append(committed, snap.RuntimeID)

		if rtInfo.State == domain.RuntimeStateRunning || rtInfo.State == domain.RuntimeStateDegraded || rtInfo.State == domain.RuntimeStateSuspended {
			if err := c.runtimeExecutor.StopRuntime(ctx, snap.RuntimeID); err != nil {
				quiesceErr = fmt.Errorf("stop runtime %s: %w", snap.RuntimeID, err)
				break
			}
			stopped, err := c.runtimeManager.GetRuntime(snap.RuntimeID)
			if err != nil {
				quiesceErr = fmt.Errorf("verify quiesced runtime %s: %w", snap.RuntimeID, err)
				break
			}
			if stopped.State != domain.RuntimeStateStopped {
				quiesceErr = fmt.Errorf("verify quiesced runtime %s: state=%s", snap.RuntimeID, stopped.State)
				break
			}
		}
	}

	if quiesceErr != nil {
		for _, rid := range committed {
			intent, err := c.lifecycleIntents.GetLifecycleIntent(rid)
			if err == nil && intent == "upgrade" {
				_ = c.lifecycleIntents.SetLifecycleIntent(rid, "")
			}
		}
		return quiesceErr
	}

	return nil
}

func (c *UpgradeCoordinator) executeMigrationHooks(ctx context.Context, req UpgradeRequest, operationID UpgradeOperationID, snapshots []RuntimeUpgradeSnapshot) error {
	hook, exists := c.migrationHooks.Get(req.ExtensionID)
	if !exists || hook == nil {
		return nil
	}

	pluginIDs := make(map[domain.PluginID]struct{})
	for _, snap := range snapshots {
		pluginIDs[snap.PluginID] = struct{}{}
	}

	for pluginID := range pluginIDs {
		mc := MigrationContext{
			OperationID: operationID,
			ExtensionID: req.ExtensionID,
			PluginID:    pluginID,
			FromVersion: "",
			ToVersion:   req.TargetVersion,
		}
		mr, err := hook.ExecuteMigration(ctx, mc)
		if err != nil {
			return fmt.Errorf("migration hook for extension %s plugin %s failed: %w", req.ExtensionID, pluginID, err)
		}
		if mr == MigrationResultFailed {
			return fmt.Errorf("migration hook for extension %s plugin %s returned failed", req.ExtensionID, pluginID)
		}
	}
	return nil
}

func (c *UpgradeCoordinator) reconcileConfigs(ctx context.Context, snapshots []RuntimeUpgradeSnapshot) []config.ValidationError {
	var allErrors []config.ValidationError
	for _, snap := range snapshots {
		_, errs := c.configValidator.Resolve(ctx, string(snap.PluginID), string(snap.RuntimeID), "")
		if len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		}
	}
	return allErrors
}

func (c *UpgradeCoordinator) resumeRuntimes(ctx context.Context, snapshots []RuntimeUpgradeSnapshot) map[domain.RuntimeInstanceID]struct{} {
	failures := make(map[domain.RuntimeInstanceID]struct{})
	for _, snap := range snapshots {
		if !snap.WasRunning && !snap.WasSuspended {
			continue
		}
		intent, err := c.lifecycleIntents.GetLifecycleIntent(snap.RuntimeID)
		if err != nil || intent != "upgrade" {
			log.Printf("[upgrade-coordinator] runtime %s resume suppressed by lifecycle intent=%q err=%v", snap.RuntimeID, intent, err)
			failures[snap.RuntimeID] = struct{}{}
			continue
		}
		if err := c.runtimeExecutor.StartRuntime(ctx, snap.RuntimeID); err != nil {
			log.Printf("[upgrade-coordinator] failed to resume runtime %s: %v", snap.RuntimeID, err)
			failures[snap.RuntimeID] = struct{}{}
		}
	}
	return failures
}

func (c *UpgradeCoordinator) rediscoverCurrentRuntimes(ctx context.Context, extensionID string, previous []RuntimeUpgradeSnapshot) ([]RuntimeUpgradeSnapshot, error) {
	plugins, err := c.findAffectedPlugins(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	current, err := c.discoverRuntimes(plugins)
	if err != nil {
		return nil, err
	}
	if len(current) == 0 {
		return current, nil
	}
	previousByPluginID := make(map[domain.PluginID][]RuntimeUpgradeSnapshot, len(previous))
	for _, snap := range previous {
		previousByPluginID[snap.PluginID] = append(previousByPluginID[snap.PluginID], snap)
	}
	usedRuntimeIDs := make(map[string]bool, len(previous))
	eligible := make([]RuntimeUpgradeSnapshot, 0, len(previous))
	for _, currentSnap := range current {
		candidates := previousByPluginID[currentSnap.PluginID]
		var best RuntimeUpgradeSnapshot
		hasBest := false
		for _, prev := range candidates {
			if usedRuntimeIDs[string(prev.RuntimeID)] {
				continue
			}
			if prev.RuntimeID == currentSnap.RuntimeID {
				best = prev
				hasBest = true
				usedRuntimeIDs[string(prev.RuntimeID)] = true
				break
			}
			if !hasBest {
				best = prev
				hasBest = true
			}
		}
		if hasBest {
			currentSnap.WasRunning = best.WasRunning
			currentSnap.WasSuspended = best.WasSuspended
			currentSnap.PreUpgradeGeneration = best.PreUpgradeGeneration
			usedRuntimeIDs[string(best.RuntimeID)] = true
			if (currentSnap.WasRunning || currentSnap.WasSuspended) && currentSnap.RuntimeID != best.RuntimeID {
				currentIntent, err := c.lifecycleIntents.GetLifecycleIntent(currentSnap.RuntimeID)
				if err != nil || (currentIntent == "" || currentIntent == "upgrade") {
					_ = c.lifecycleIntents.SetLifecycleIntent(currentSnap.RuntimeID, "upgrade")
				}
			}
		}
		eligible = append(eligible, currentSnap)
	}
	return eligible, nil
}

func (c *UpgradeCoordinator) clearUpgradeIntent(snapshots []RuntimeUpgradeSnapshot) {
	for _, snap := range snapshots {
		intent, err := c.lifecycleIntents.GetLifecycleIntent(snap.RuntimeID)
		if err == nil && intent == "upgrade" {
			_ = c.lifecycleIntents.SetLifecycleIntent(snap.RuntimeID, "")
		}
	}
}

func (c *UpgradeCoordinator) logStage(operationID UpgradeOperationID, extensionID string, stage UpgradeOperationState, extra string) {
	log.Printf("[upgrade-coordinator] operationID=%s extensionID=%s stage=%s %s",
		operationID, extensionID, stage, extra)
}

func (c *UpgradeCoordinator) recordAudit(operationID UpgradeOperationID, req UpgradeRequest, result *UpgradeResult, err error) {
	auditResult := "success"
	errMsg := ""
	if err != nil {
		auditResult = "failed"
		errMsg = err.Error()
	}
	log.Printf("[upgrade-audit] operationID=%s extensionID=%s targetVersion=%s stage=%s result=%s plugins=%d quiesced=%d resumed=%d failed=%d error=%s",
		operationID, req.ExtensionID, req.TargetVersion, result.Stage, auditResult,
		len(result.AffectedPlugins), len(result.QuiescedRuntimes), len(result.ResumedRuntimes), len(result.FailedRuntimes), errMsg)
}

func generateUpgradeOperationID(extensionID string) UpgradeOperationID {
	return UpgradeOperationID(fmt.Sprintf("upgrade-%s-%d", extensionID, time.Now().UnixNano()))
}

func runtimeDescriptorRevision(descriptor domain.PluginDescriptor) string {
	raw, _ := json.Marshal(struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}{
		ID:      string(descriptor.ID),
		Version: descriptor.Version,
	})
	sum := sha256.Sum256(raw)
	return "drev-" + hex.EncodeToString(sum[:16])
}
