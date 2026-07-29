package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/update"
)

type ProductionCandidateRunner struct {
	supervisor       runtime_supervisor.Supervisor
	contribInstaller *TypedContributionInstaller
	generationMgr    *update.GenerationManager
	candidateMgr     *CandidateContributionManager
	extRoot          string
	cleanupRepo      *RuntimeCleanupRepository
}

func NewProductionCandidateRunner(
	supervisor runtime_supervisor.Supervisor,
	contribInstaller *TypedContributionInstaller,
	generationMgr *update.GenerationManager,
	candidateMgr *CandidateContributionManager,
	extRoot string,
) *ProductionCandidateRunner {
	return &ProductionCandidateRunner{
		supervisor:       supervisor,
		contribInstaller: contribInstaller,
		generationMgr:    generationMgr,
		candidateMgr:     candidateMgr,
		extRoot:          extRoot,
	}
}

func (r *ProductionCandidateRunner) WithCleanupRepo(repo *RuntimeCleanupRepository) *ProductionCandidateRunner {
	if r != nil {
		r.cleanupRepo = repo
	}
	return r
}

func (r *ProductionCandidateRunner) StartCandidate(ctx context.Context, id dev_mode.WorkspaceID, rev *dev_mode.Revision) (string, error) {
	if r == nil {
		return "", fmt.Errorf("candidate_runner: runner not initialized")
	}
	if rev == nil || rev.ArtifactPath == "" {
		return "", fmt.Errorf("candidate_runner: missing build artifact")
	}

	manifestPath := filepath.Join(rev.ArtifactPath, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		manifestData, err = os.ReadFile(filepath.Join(filepath.Dir(rev.ArtifactPath), "manifest.json"))
		if err != nil {
			return "", fmt.Errorf("candidate_runner: read manifest: %w", err)
		}
	}

	var manifest manifest_v2.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return "", fmt.Errorf("candidate_runner: parse manifest: %w", err)
	}

	extID := domain.ExtensionID(manifest.Extension.ID)
	if extID == "" {
		return "", fmt.Errorf("candidate_runner: manifest missing extension.id")
	}

	defHash := rev.ManifestHash
	if defHash == "" {
		defHash = rev.SourceHash
	}
	gen := r.generationMgr.Prepare(ctx, string(extID), manifest.Extension.Version, defHash)
	if err := r.generationMgr.Transition(ctx, string(extID), gen.GenerationID, update.GenerationStateValidated); err != nil {
		return "", fmt.Errorf("candidate_runner: transition to validated: %w", err)
	}

	var instanceIDs []string
	var allContribs []domain.ContributionDefinition

	for _, mod := range manifest.Modules {
		modID := domain.ModuleID(mod.ID)

		if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != "static" {
			spec := r.buildInstanceSpec(extID, modID, mod, gen.Generation)
			result := r.supervisor.Reconcile(ctx, runtime_supervisor.ReconcileRequest{
				DefinitionID: runtime_supervisor.BuildRuntimeDefinitionID(string(extID), string(modID), domain.RuntimeType(mod.Runtime.Type)),
				Desired:      runtime_supervisor.DesiredRunning,
				Spec:         spec,
			})
			if result.Error != nil {
				r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
				return "", fmt.Errorf("candidate_runner: start runtime for module %s: %w", modID, result.Error)
			}
			if result.InstanceID == "" {
				r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
				return "", fmt.Errorf("candidate_runner: no instance ID returned for module %s", modID)
			}
			instanceIDs = append(instanceIDs, result.InstanceID)
		}

		for _, contribMeta := range mod.Contributions {
			cd, err := contribMeta.ToDomain(extID, modID)
			if err != nil {
				r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
				return "", fmt.Errorf("candidate_runner: convert contribution %s: %w", contribMeta.ID, err)
			}
			allContribs = append(allContribs, cd)
		}
	}

	if err := r.generationMgr.Transition(ctx, string(extID), gen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
		return "", fmt.Errorf("candidate_runner: transition to runtime_ready: %w", err)
	}

	candidateID := "candidate-" + uuid.NewString()
	var expectedStableGen int64
	if activeGen := r.generationMgr.Active(ctx, string(extID)); activeGen != nil {
		expectedStableGen = int64(activeGen.Generation)
	}
	record := &CandidateRecord{
		CandidateID:              candidateID,
		ExtensionID:              extID,
		InstanceIDs:              instanceIDs,
		GenerationID:             gen.GenerationID,
		CandidateGeneration:      int64(gen.Generation),
		ExpectedStableGeneration: expectedStableGen,
		Contribs:                 allContribs,
		ArtifactPath:             rev.ArtifactPath,
		DefinitionHash:           defHash,
	}

	if err := r.candidateMgr.RegisterCandidate(ctx, record); err != nil {
		r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
		return "", fmt.Errorf("candidate_runner: register candidate: %w", err)
	}

	return candidateID, nil
}

func (r *ProductionCandidateRunner) HealthCheck(ctx context.Context, instanceID string) error {
	if r == nil {
		return fmt.Errorf("candidate_runner: runner not initialized")
	}

	if err := r.candidateMgr.HealthCandidate(ctx, instanceID); err != nil {
		return err
	}

	if r.cleanupRepo != nil {
		if err := r.createCleanupTasksForCandidate(ctx, instanceID); err != nil {
			return fmt.Errorf("candidate_runner: create cleanup tasks: %w", err)
		}
	}

	if err := r.candidateMgr.PromoteCandidate(ctx, instanceID); err != nil {
		return fmt.Errorf("candidate_runner: promote candidate: %w", err)
	}

	return nil
}

func (r *ProductionCandidateRunner) DrainInstance(ctx context.Context, instanceID string, timeout time.Duration) error {
	if r == nil {
		return fmt.Errorf("candidate_runner: runner not initialized")
	}

	record, ok := r.candidateMgr.GetCandidate(instanceID)
	if !ok {
		return fmt.Errorf("candidate_runner: candidate %s not found", instanceID)
	}

	if err := r.generationMgr.DrainGeneration(ctx, string(record.ExtensionID), record.GenerationID); err != nil {
		return fmt.Errorf("candidate_runner: drain generation: %w", err)
	}

	for _, instID := range record.InstanceIDs {
		if err := r.supervisor.Drain(ctx, instID, timeout); err != nil {
			return fmt.Errorf("candidate_runner: drain instance %s: %w", instID, err)
		}
	}

	return nil
}

func (r *ProductionCandidateRunner) StopInstance(ctx context.Context, instanceID string) error {
	if r == nil {
		return fmt.Errorf("candidate_runner: runner not initialized")
	}

	if r.cleanupRepo != nil {
		return r.stopInstanceWithCleanupRepo(ctx, instanceID)
	}

	return r.stopInstanceLegacy(ctx, instanceID)
}

func (r *ProductionCandidateRunner) stopInstanceLegacy(ctx context.Context, instanceID string) error {
	record, ok := r.candidateMgr.GetCandidate(instanceID)
	if !ok {
		return nil
	}

	var firstErr error

	if record.Status == CandidateStatusPromoted {
		for _, instID := range record.InstanceIDs {
			if err := r.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("candidate_runner: stop instance %s: %w", instID, err)
			}
		}
		extID := string(record.ExtensionID)
		genID := record.GenerationID
		if gen, _ := r.generationMgr.Get(ctx, extID, genID); gen != nil {
			if gen.State == update.GenerationStateActive {
				_ = r.generationMgr.DrainGeneration(ctx, extID, genID)
			}
			if firstErr != nil {
				_ = r.generationMgr.Transition(ctx, extID, genID, update.GenerationStateFailed)
			} else {
				_ = r.generationMgr.StopGeneration(ctx, extID, genID)
			}
		}
		_ = r.generationMgr.RemoveGeneration(ctx, extID, genID)
		r.candidateMgr.RemovePromotedRecord(instanceID)
		return firstErr
	}

	if err := r.candidateMgr.DiscardCandidate(ctx, instanceID); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

func (r *ProductionCandidateRunner) stopInstanceWithCleanupRepo(ctx context.Context, candidateID string) error {
	record, ok := r.candidateMgr.GetCandidate(candidateID)
	if ok {
		if record.Status != CandidateStatusPromoted {
			return r.candidateMgr.DiscardCandidate(ctx, candidateID)
		}
		return r.stopPromotedCandidateWithVerification(ctx, candidateID, record)
	}

	tasks, err := r.cleanupRepo.ListByCleanupIDPrefix(ctx, candidateID+":")
	if err != nil {
		return fmt.Errorf("candidate_runner: list cleanup tasks for %s: %w", candidateID, err)
	}

	if len(tasks) == 0 {
		task, lookupErr := r.cleanupRepo.ListByRuntimeInstanceID(ctx, candidateID)
		if lookupErr != nil {
			return fmt.Errorf("candidate_runner: lookup cleanup task by instance %s: %w", candidateID, lookupErr)
		}
		if task != nil {
			tasks = []*RuntimeCleanupTask{task}
		}
	}

	if len(tasks) == 0 {
		stopErr := r.stopAndVerify(ctx, candidateID)
		if stopErr != nil {
			_ = r.cleanupRepo.SaveTask(ctx, &RuntimeCleanupTask{
				CleanupID:         candidateID + ":" + candidateID,
				RuntimeInstanceID: candidateID,
				CleanupState:      CleanupStateStopFailed,
				LastErrorCode:     "STOP_FAILED",
				LastErrorMessage:  stopErr.Error(),
				NextRetryAt:       time.Now().UTC().Add(30 * time.Second),
			})
			return stopErr
		}
		return nil
	}

	var firstErr error
	for _, task := range tasks {
		if task.CleanupState == CleanupStateCompleted || task.CleanupState == CleanupStateVerified {
			continue
		}
		if stopErr := r.stopAndVerify(ctx, task.RuntimeInstanceID); stopErr != nil {
			if firstErr == nil {
				firstErr = stopErr
			}
			_ = r.cleanupRepo.UpdateState(ctx, task.CleanupID, CleanupStateStopFailed, "STOP_FAILED", stopErr.Error())
			retryAt := time.Now().UTC().Add(30 * time.Second)
			_ = r.cleanupRepo.UpdateRetry(ctx, task.CleanupID, task.AttemptCount+1, retryAt, CleanupStateStopFailed)
		} else {
			_ = r.cleanupRepo.UpdateState(ctx, task.CleanupID, CleanupStateCompleted, "", "")
			_ = r.cleanupRepo.DeleteTask(ctx, task.CleanupID)
		}
	}
	return firstErr
}

func (r *ProductionCandidateRunner) stopPromotedCandidateWithVerification(ctx context.Context, candidateID string, record *CandidateRecord) error {
	var firstErr error

	for _, instID := range record.InstanceIDs {
		if stopErr := r.stopAndVerify(ctx, instID); stopErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("candidate_runner: stop instance %s: %w", instID, stopErr)
			}
		}
	}

	extID := string(record.ExtensionID)
	genID := record.GenerationID

	if r.cleanupRepo != nil {
		prefix := candidateID + ":"
		existingTasks, _ := r.cleanupRepo.ListByCleanupIDPrefix(ctx, prefix)
		existingMap := make(map[string]*RuntimeCleanupTask, len(existingTasks))
		for _, t := range existingTasks {
			existingMap[t.RuntimeInstanceID] = t
		}

		for _, instID := range record.InstanceIDs {
			cleanupID := candidateID + ":" + instID
			if _, exists := existingMap[instID]; exists {
				if firstErr != nil {
					_ = r.cleanupRepo.UpdateState(ctx, cleanupID, CleanupStateStopFailed, "STOP_FAILED", firstErr.Error())
					retryAt := time.Now().UTC().Add(30 * time.Second)
					_ = r.cleanupRepo.UpdateRetry(ctx, cleanupID, existingMap[instID].AttemptCount+1, retryAt, CleanupStateStopFailed)
				} else {
					_ = r.cleanupRepo.UpdateState(ctx, cleanupID, CleanupStateCompleted, "", "")
					_ = r.cleanupRepo.DeleteTask(ctx, cleanupID)
				}
			} else {
				if firstErr != nil {
					_ = r.cleanupRepo.SaveTask(ctx, &RuntimeCleanupTask{
						CleanupID:         cleanupID,
						ExtensionID:       extID,
						RuntimeInstanceID: instID,
						CleanupState:      CleanupStateStopFailed,
						LastErrorCode:     "STOP_FAILED",
						LastErrorMessage:  firstErr.Error(),
						NextRetryAt:       time.Now().UTC().Add(30 * time.Second),
					})
				}
			}
		}
	}

	if gen, _ := r.generationMgr.Get(ctx, extID, genID); gen != nil {
		if gen.State == update.GenerationStateActive {
			_ = r.generationMgr.DrainGeneration(ctx, extID, genID)
		}
		if firstErr != nil {
			_ = r.generationMgr.Transition(ctx, extID, genID, update.GenerationStateFailed)
		} else {
			_ = r.generationMgr.StopGeneration(ctx, extID, genID)
		}
	}
	_ = r.generationMgr.RemoveGeneration(ctx, extID, genID)
	r.candidateMgr.RemovePromotedRecord(candidateID)
	return firstErr
}

func (r *ProductionCandidateRunner) stopAndVerify(ctx context.Context, runtimeInstanceID string) error {
	if runtimeInstanceID == "" {
		return nil
	}

	stopErr := r.supervisor.Stop(ctx, runtimeInstanceID, runtime_supervisor.StopReasonRollback)
	if stopErr != nil {
		if errors.Is(stopErr, runtime_supervisor.ErrInstanceNotFound) {
			return nil
		}
		return stopErr
	}

	verifyCtx, verifyCancel := context.WithTimeout(ctx, 5*time.Second)
	defer verifyCancel()

	snap, verifyErr := r.supervisor.GetInstance(verifyCtx, runtimeInstanceID)
	if verifyErr != nil {
		if errors.Is(verifyErr, runtime_supervisor.ErrInstanceNotFound) {
			return nil
		}
		return nil
	}

	if snap.Actual == runtime_supervisor.ActualStopped || snap.Actual == runtime_supervisor.ActualFailed {
		return nil
	}

	return fmt.Errorf("candidate_runner: instance %s not stopped after verification (state=%s)", runtimeInstanceID, snap.Actual)
}

func (r *ProductionCandidateRunner) rollbackCandidate(ctx context.Context, extID domain.ExtensionID, instanceIDs []string, generationID string, _ string) {
	for _, instID := range instanceIDs {
		_ = r.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
	}
	_ = r.generationMgr.Transition(ctx, string(extID), generationID, update.GenerationStateFailed)
	_ = r.generationMgr.RemoveGeneration(ctx, string(extID), generationID)
}

func (r *ProductionCandidateRunner) buildInstanceSpec(extID domain.ExtensionID, modID domain.ModuleID, mod manifest_v2.ModuleMeta, genNum int) runtime_supervisor.InstanceSpec {
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID:  runtime_supervisor.BuildRuntimeDefinitionID(string(extID), string(modID), domain.RuntimeType(mod.Runtime.Type)),
		ExtensionID:   extID,
		ModuleID:      modID,
		RuntimeType:   domain.RuntimeType(mod.Runtime.Type),
		Generation:    int64(genNum),
		Strategy:      runtime_supervisor.StrategySingletonPerModule,
		EntryPoint:    mod.Runtime.EntryPoint,
		WorkerCount:   mod.Runtime.WorkerCount,
		Env:           mod.Runtime.Env,
		Permissions:   mod.Runtime.Permissions,
		Capabilities:  mod.Runtime.Capabilities,
		Restart:       runtime_supervisor.RestartOnCrash,
		MaxRestarts:   3,
		RestartWindow: 60 * time.Second,
		Limits: runtime_supervisor.ResourceLimits{
			MaxMemoryBytes:     mod.Runtime.Memory,
			MaxExecutionTime:   0,
			MaxConcurrentCalls: 10,
			MaxQueueDepth:      64,
		},
	}
	if spec.WorkerCount <= 0 {
		spec.WorkerCount = 1
	}
	return spec
}

func (r *ProductionCandidateRunner) CleanupDrainingGenerations(ctx context.Context) int {
	if r == nil {
		return 0
	}
	all := r.generationMgr.ListAll(ctx)
	cleaned := 0
	for _, gen := range all {
		if gen.State == update.GenerationStateDraining {
			_ = r.generationMgr.Transition(ctx, gen.ExtensionID, gen.GenerationID, update.GenerationStateStopped)
			_ = r.generationMgr.RemoveGeneration(ctx, gen.ExtensionID, gen.GenerationID)
			cleaned++
		}
	}
	return cleaned
}

func (r *ProductionCandidateRunner) RecoverOrphans(ctx context.Context) ([]string, error) {
	if r == nil || r.candidateMgr == nil {
		return nil, nil
	}
	return r.candidateMgr.RecoverOrphanCandidates(ctx)
}

func (r *ProductionCandidateRunner) createCleanupTasksForCandidate(ctx context.Context, candidateID string) error {
	if r == nil || r.cleanupRepo == nil {
		return nil
	}
	record, ok := r.candidateMgr.GetCandidate(candidateID)
	if !ok {
		return nil
	}

	extID := string(record.ExtensionID)
	oldGen := record.CandidateGeneration
	if activeGen := r.generationMgr.Active(ctx, extID); activeGen != nil {
		oldGen = int64(activeGen.Generation)
	}

	for _, instID := range record.InstanceIDs {
		snap, err := r.supervisor.GetInstance(ctx, instID)
		if err != nil {
			continue
		}

		var processID int
		defID := string(snap.Identity.RuntimeDefinitionID)
		modID := string(snap.Identity.ModuleID)
		rtType := string(snap.Identity.RuntimeType)

		r.CreateCleanupTask(ctx, candidateID, extID, modID, oldGen, defID, instID, rtType, processID)
	}

	return nil
}

func (r *ProductionCandidateRunner) CreateCleanupTask(ctx context.Context, candidateID string, extID string, moduleID string, oldGen int64, defID string, instanceID string, rtType string, processID int) error {
	if r == nil || r.cleanupRepo == nil {
		return nil
	}
	cleanupID := candidateID + ":" + instanceID
	task := &RuntimeCleanupTask{
		CleanupID:           cleanupID,
		ExtensionID:         extID,
		ModuleID:            moduleID,
		OldGeneration:       oldGen,
		RuntimeDefinitionID: defID,
		RuntimeInstanceID:   instanceID,
		RuntimeType:         rtType,
		ProcessID:           processID,
		CleanupState:        CleanupStatePending,
	}
	if err := r.cleanupRepo.SaveTask(ctx, task); err != nil {
		return fmt.Errorf("candidate_runner: save cleanup task %s: %w", cleanupID, err)
	}
	return nil
}

func (r *ProductionCandidateRunner) RecoverCleanupTasks(ctx context.Context) int {
	if r == nil || r.cleanupRepo == nil {
		return 0
	}

	tasks, err := r.cleanupRepo.ListPending(ctx)
	if err != nil {
		return 0
	}

	cleaned := 0
	for _, task := range tasks {
		if !task.NextRetryAt.IsZero() && time.Now().UTC().Before(task.NextRetryAt) {
			continue
		}

		if task.RuntimeInstanceID == "" {
			_ = r.cleanupRepo.DeleteTask(ctx, task.CleanupID)
			cleaned++
			continue
		}

		stopErr := r.stopAndVerify(ctx, task.RuntimeInstanceID)
		if stopErr == nil {
			_ = r.cleanupRepo.UpdateState(ctx, task.CleanupID, CleanupStateCompleted, "", "")
			_ = r.cleanupRepo.DeleteTask(ctx, task.CleanupID)
			cleaned++
		} else {
			newAttempt := task.AttemptCount + 1
			if newAttempt >= 5 {
				_ = r.cleanupRepo.UpdateState(ctx, task.CleanupID, CleanupStateRequiresManualRecovery, "MAX_RETRIES_EXCEEDED", stopErr.Error())
			} else {
				backoff := time.Duration(30*(newAttempt+1)) * time.Second
				_ = r.cleanupRepo.UpdateRetry(ctx, task.CleanupID, newAttempt, time.Now().UTC().Add(backoff), CleanupStateStopFailed)
				_ = r.cleanupRepo.UpdateState(ctx, task.CleanupID, CleanupStateStopFailed, "STOP_FAILED", stopErr.Error())
			}
		}
	}

	return cleaned
}

var _ dev_mode.CandidateRunner = (*ProductionCandidateRunner)(nil)
