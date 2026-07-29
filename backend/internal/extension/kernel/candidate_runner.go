package kernel

import (
	"context"
	"encoding/json"
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
	record := &CandidateRecord{
		CandidateID:         candidateID,
		ExtensionID:         extID,
		InstanceIDs:         instanceIDs,
		GenerationID:        gen.GenerationID,
		CandidateGeneration: int64(gen.Generation),
		Contribs:            allContribs,
		ArtifactPath:        rev.ArtifactPath,
		DefinitionHash:      defHash,
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

func (r *ProductionCandidateRunner) rollbackCandidate(ctx context.Context, extID domain.ExtensionID, instanceIDs []string, generationID string, _ string) {
	for _, instID := range instanceIDs {
		_ = r.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
	}
	_ = r.generationMgr.Transition(ctx, string(extID), generationID, update.GenerationStateFailed)
	_ = r.generationMgr.RemoveGeneration(ctx, string(extID), generationID)
}

func (r *ProductionCandidateRunner) buildInstanceSpec(extID domain.ExtensionID, modID domain.ModuleID, mod manifest_v2.ModuleMeta, genNum int) runtime_supervisor.InstanceSpec {
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID:    runtime_supervisor.BuildRuntimeDefinitionID(string(extID), string(modID), domain.RuntimeType(mod.Runtime.Type)),
		ExtensionID:     extID,
		ModuleID:        modID,
		RuntimeType:     domain.RuntimeType(mod.Runtime.Type),
		Generation:      int64(genNum),
		Strategy:        runtime_supervisor.StrategySingletonPerModule,
		EntryPoint:      mod.Runtime.EntryPoint,
		WorkerCount:     mod.Runtime.WorkerCount,
		Env:             mod.Runtime.Env,
		Permissions:     mod.Runtime.Permissions,
		Capabilities:    mod.Runtime.Capabilities,
		Restart:         runtime_supervisor.RestartOnCrash,
		MaxRestarts:     3,
		RestartWindow:   60 * time.Second,
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

var _ dev_mode.CandidateRunner = (*ProductionCandidateRunner)(nil)
