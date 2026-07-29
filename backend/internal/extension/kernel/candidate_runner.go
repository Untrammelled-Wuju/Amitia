package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
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
	extRoot          string

	mu         sync.Mutex
	candidates map[string]*candidateState
}

type candidateState struct {
	candidateID  string
	extensionID  domain.ExtensionID
	instanceIDs  []string
	generationID string
	generation   int64
	contribs     []domain.ContributionDefinition
	artifactPath string
	promoted     bool
}

func NewProductionCandidateRunner(
	supervisor runtime_supervisor.Supervisor,
	contribInstaller *TypedContributionInstaller,
	generationMgr *update.GenerationManager,
	extRoot string,
) *ProductionCandidateRunner {
	return &ProductionCandidateRunner{
		supervisor:       supervisor,
		contribInstaller: contribInstaller,
		generationMgr:    generationMgr,
		extRoot:          extRoot,
		candidates:       make(map[string]*candidateState),
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
				DefinitionID: runtime_supervisor.DefinitionID(string(extID)),
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

	if err := r.contribInstaller.InstallContributions(ctx, allContribs, int64(gen.Generation)); err != nil {
		r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
		return "", fmt.Errorf("candidate_runner: install contributions: %w", err)
	}

	if err := r.generationMgr.Transition(ctx, string(extID), gen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		r.rollbackCandidate(ctx, extID, instanceIDs, gen.GenerationID, "")
		return "", fmt.Errorf("candidate_runner: transition to runtime_ready: %w", err)
	}

	candidateID := "candidate-" + uuid.NewString()
	state := &candidateState{
		candidateID:  candidateID,
		extensionID:  extID,
		instanceIDs:  instanceIDs,
		generationID: gen.GenerationID,
		generation:   int64(gen.Generation),
		contribs:     allContribs,
		artifactPath: rev.ArtifactPath,
		promoted:     false,
	}

	r.mu.Lock()
	r.candidates[candidateID] = state
	r.mu.Unlock()

	return candidateID, nil
}

func (r *ProductionCandidateRunner) HealthCheck(ctx context.Context, instanceID string) error {
	if r == nil {
		return fmt.Errorf("candidate_runner: runner not initialized")
	}

	r.mu.Lock()
	state, ok := r.candidates[instanceID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("candidate_runner: candidate %s not found", instanceID)
	}

	for _, instID := range state.instanceIDs {
		snap, err := r.supervisor.GetInstance(ctx, instID)
		if err != nil {
			return fmt.Errorf("candidate_runner: get instance %s: %w", instID, err)
		}
		if snap.Health != runtime_supervisor.HealthHealthy && snap.Health != runtime_supervisor.HealthDegraded {
			return fmt.Errorf("candidate_runner: instance %s unhealthy (status=%s)", instID, snap.Health)
		}
		if snap.Actual != runtime_supervisor.ActualReady && snap.Actual != runtime_supervisor.ActualDegraded {
			return fmt.Errorf("candidate_runner: instance %s not ready (state=%s)", instID, snap.Actual)
		}
	}

	if !state.promoted {
		if err := r.generationMgr.Transition(ctx, string(state.extensionID), state.generationID, update.GenerationStateActive); err != nil {
			return fmt.Errorf("candidate_runner: promote generation: %w", err)
		}
		r.mu.Lock()
		state.promoted = true
		r.mu.Unlock()
	}

	return nil
}

func (r *ProductionCandidateRunner) DrainInstance(ctx context.Context, instanceID string, timeout time.Duration) error {
	if r == nil {
		return fmt.Errorf("candidate_runner: runner not initialized")
	}

	r.mu.Lock()
	state, ok := r.candidates[instanceID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("candidate_runner: candidate %s not found", instanceID)
	}

	if err := r.generationMgr.Drain(ctx, string(state.extensionID)); err != nil {
		return fmt.Errorf("candidate_runner: drain generation: %w", err)
	}

	for _, instID := range state.instanceIDs {
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

	r.mu.Lock()
	state, ok := r.candidates[instanceID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("candidate_runner: candidate %s not found", instanceID)
	}

	var firstErr error
	for _, instID := range state.instanceIDs {
		if err := r.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("candidate_runner: stop instance %s: %w", instID, err)
		}
	}

	if err := r.contribInstaller.UninstallContributions(ctx, state.extensionID); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("candidate_runner: uninstall contributions: %w", err)
	}

	targetState := update.GenerationStateStopped
	if firstErr != nil {
		targetState = update.GenerationStateFailed
	}
	_ = r.generationMgr.Transition(ctx, string(state.extensionID), state.generationID, targetState)
	_ = r.generationMgr.RemoveGeneration(ctx, string(state.extensionID), state.generationID)

	r.mu.Lock()
	delete(r.candidates, instanceID)
	r.mu.Unlock()

	return firstErr
}

func (r *ProductionCandidateRunner) rollbackCandidate(ctx context.Context, extID domain.ExtensionID, instanceIDs []string, generationID string, _ string) {
	for _, instID := range instanceIDs {
		_ = r.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
	}
	if err := r.contribInstaller.UninstallContributions(ctx, extID); err != nil {
		log.Printf("[candidate_runner] uninstall contributions for rollback %s: %v", extID, err)
	}
	_ = r.generationMgr.Transition(ctx, string(extID), generationID, update.GenerationStateFailed)
	_ = r.generationMgr.RemoveGeneration(ctx, string(extID), generationID)
}

func (r *ProductionCandidateRunner) buildInstanceSpec(extID domain.ExtensionID, modID domain.ModuleID, mod manifest_v2.ModuleMeta, genNum int) runtime_supervisor.InstanceSpec {
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID:    runtime_supervisor.DefinitionID(string(extID)),
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

var _ dev_mode.CandidateRunner = (*ProductionCandidateRunner)(nil)
