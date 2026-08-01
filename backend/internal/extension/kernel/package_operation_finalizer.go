package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageRecoveryFinalizer struct {
	r *Runtime
}

func (r *Runtime) newPackageRecoveryFinalizer() *PackageRecoveryFinalizer {
	return &PackageRecoveryFinalizer{r: r}
}

func (f *PackageRecoveryFinalizer) repo() *PackageRepository {
	if f.r == nil || f.r.container == nil {
		return nil
	}
	return f.r.container.PackageRepository
}

func (f *PackageRecoveryFinalizer) completeRecoveryStep(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, stepName string, order int, resultJSON string, guard PackageWriteGuard) error {
	if _, ok := completed[stepName]; ok {
		return nil
	}
	repo := f.repo()
	if repo == nil {
		return errors.New("kernel: package repository unavailable for recovery finalizer")
	}
	step := PackageOperationStep{
		OperationID:        operation.OperationID,
		StepName:           stepName,
		StepOrder:          order,
		Status:             StatusCompleted,
		ResultJSON:         resultJSON,
		StableGeneration:   operation.StableGeneration,
		TargetGeneration:   operation.TargetGeneration,
		CurrentPointerJSON: operation.CurrentPointerJSON,
	}
	if err := repo.PutStep(ctx, step, guard); err != nil {
		if IsRepositoryErrorKind(err, RepositoryErrorConflict) {
			return nil
		}
		return fmt.Errorf("kernel: persist recovery step %s failed: %w", stepName, err)
	}
	completed[stepName] = step
	return nil
}

func (f *PackageRecoveryFinalizer) completeRecoveryStepWithFunc(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, stepName string, order int, action func() error, guard PackageWriteGuard) error {
	if _, ok := completed[stepName]; ok {
		return nil
	}
	if err := action(); err != nil {
		return err
	}
	return f.completeRecoveryStep(ctx, operation, completed, stepName, order, `{"applied":true}`, guard)
}

func (f *PackageRecoveryFinalizer) requireManualRecovery(ctx context.Context, operation PackageOperationRecord, detail string, cause error, guard PackageWriteGuard) error {
	if cause != nil {
		detail = detail + ": " + cause.Error()
	}
	return f.r.requirePackageRecovery(ctx, operation, "requires_manual_recovery: "+detail, cause, guard)
}

func (f *PackageRecoveryFinalizer) gatherIdentityEvidence(ctx context.Context, operation PackageOperationRecord) (RecoveryIdentityEvidence, error) {
	repo := f.repo()
	if repo == nil {
		return RecoveryIdentityEvidence{}, errors.New("kernel: package repository unavailable")
	}
	evidence := RecoveryIdentityEvidence{
		ExtensionID: operation.ExtensionID,
		Version:     operation.TargetVersion,
		ArtifactID:  operation.ArtifactID,
		OperationID: operation.OperationID,
	}
	if operation.CurrentPointerJSON != "" {
		var current PackageGenerationCurrent
		if err := json.Unmarshal([]byte(operation.CurrentPointerJSON), &current); err == nil {
			evidence.GenerationID = current.GenerationID
			evidence.InstalledTreeHash = current.TreeHash
		}
	}
	artifact, err := repo.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return evidence, fmt.Errorf("artifact %s missing", operation.ArtifactID)
		}
		return evidence, fmt.Errorf("artifact unavailable: %w", err)
	}
	evidence.ManifestHash = artifact.ManifestHash
	evidence.ContentTreeHash = artifact.ContentTreeHash
	evidence.ArchiveHash = artifact.ArchiveHash
	if artifact.InstalledPath != "" {
		evidence.InstalledPath = artifact.InstalledPath
	}
	installation, err := f.r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err == nil {
		if path, ok := installation.Metadata["installedPath"].(string); ok && path != "" {
			evidence.InstalledPath = path
		}
		if hash, ok := installation.Metadata["installedTreeHash"].(string); ok && hash != "" {
			evidence.InstalledTreeHash = hash
		}
		if gen, ok := installation.Metadata["generationId"].(string); ok && gen != "" {
			evidence.GenerationID = gen
		}
	}
	return evidence, nil
}

func (f *PackageRecoveryFinalizer) planInstallRepair(ctx context.Context, operation PackageOperationRecord, evidence RecoveryIdentityEvidence) (RecoveryRepairPlan, error) {
	repo := f.repo()
	plan := RecoveryRepairPlan{NeedFinalGate: true}
	if operation.PreviewSessionID != "" {
		plan.NeedPreviewConsume = true
		plan.NeedPreviewReferenceRelease = true
	}
	plan.NeedOperationReferenceRelease = true
	artifact, err := repo.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return plan, fmt.Errorf("plan install artifact unavailable: %w", err)
	}
	if artifact.InstalledPath == "" || (evidence.InstalledPath != "" && filepath.Clean(artifact.InstalledPath) != filepath.Clean(evidence.InstalledPath)) {
		plan.NeedArtifactInstalledPath = true
	}
	existing, vErr := repo.GetPackageVersion(ctx, operation.ExtensionID, operation.TargetVersion)
	if errors.Is(vErr, sql.ErrNoRows) {
		plan.NeedVersionRecord = true
	} else if vErr == nil {
		if existing.ArtifactID != operation.ArtifactID || (evidence.GenerationID != "" && existing.GenerationID != evidence.GenerationID) {
			return plan, fmt.Errorf("version record conflict: existing artifact %s generation %s, expected artifact %s generation %s", existing.ArtifactID, existing.GenerationID, operation.ArtifactID, evidence.GenerationID)
		}
		plan.NeedVersionRecord = existing.VersionState != string(PackageVersionStateCurrent)
	} else {
		return plan, fmt.Errorf("plan install version lookup failed: %w", vErr)
	}
	refCount, refErr := repo.CountActiveArtifactReferences(ctx, operation.ArtifactID)
	if refErr == nil && !f.hasArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID) {
		plan.NeedInstallationReference = true
	}
	_ = refCount
	return plan, nil
}

func (f *PackageRecoveryFinalizer) planUpdateRepair(ctx context.Context, operation PackageOperationRecord, evidence RecoveryIdentityEvidence, completed map[string]PackageOperationStep) (RecoveryRepairPlan, error) {
	plan, err := f.planInstallRepair(ctx, operation, evidence)
	if err != nil {
		return plan, err
	}
	plan.NeedRollbackPointVerify = true
	plan.NeedMigrationJournalVerify = true
	return plan, nil
}

func (f *PackageRecoveryFinalizer) planRollbackRepair(ctx context.Context, operation PackageOperationRecord, evidence RecoveryIdentityEvidence) (RecoveryRepairPlan, error) {
	return f.planInstallRepair(ctx, operation, evidence)
}

func (f *PackageRecoveryFinalizer) planUninstallRepair(ctx context.Context, operation PackageOperationRecord) RecoveryRepairPlan {
	return RecoveryRepairPlan{
		NeedFinalGate:                 true,
		NeedOperationReferenceRelease: true,
		NeedUninstallVersionRemoved:   operation.TargetVersion != "" && operation.ExtensionID != "",
	}
}

func (f *PackageRecoveryFinalizer) hasArtifactReference(ctx context.Context, artifactID, referenceType, ownerID string) bool {
	repo := f.repo()
	if repo == nil {
		return false
	}
	row := repo.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE artifact_id=? AND reference_type=? AND reference_owner_id=? AND released_at=''`, artifactID, referenceType, ownerID)
	var count int64
	if err := row.Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (f *PackageRecoveryFinalizer) ensureVersionRecord(ctx context.Context, operation PackageOperationRecord, evidence RecoveryIdentityEvidence, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if !evidence.Proven() {
		return f.requireManualRecovery(ctx, operation, "version record identity cannot be proven", nil, guard)
	}
	repo := f.repo()
	existing, err := repo.GetPackageVersion(ctx, evidence.ExtensionID, evidence.Version)
	if err == nil {
		if existing.ArtifactID != evidence.ArtifactID || (evidence.GenerationID != "" && existing.GenerationID != evidence.GenerationID) {
			return f.requireManualRecovery(ctx, operation, fmt.Sprintf("version record consistency conflict: artifact %s generation %s", existing.ArtifactID, existing.GenerationID), nil, guard)
		}
		if existing.VersionState == string(PackageVersionStateCurrent) && existing.InstallOperationID == operation.OperationID {
			return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureVersionRecord, recoveryStepOrderEnsureVersionRecord, `{"versionRecord":"current"}`, guard)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("kernel: ensure version record lookup failed: %w", err)
	}
	if err := f.r.recordPackageVersionAfterOperation(ctx, operation.OperationID, operation.OperationType, evidence.ExtensionID, evidence.Version, evidence.ArtifactID, evidence.InstalledPath, evidence.InstalledTreeHash, evidence.ArchiveHash, evidence.ManifestHash, evidence.ContentTreeHash, evidence.GenerationID, guard); err != nil {
		return f.requireManualRecovery(ctx, operation, "version record rebuild failed", err, guard)
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureVersionRecord, recoveryStepOrderEnsureVersionRecord, `{"versionRecord":"rebuilt"}`, guard)
}

func (f *PackageRecoveryFinalizer) ensureArtifactInstalledPath(ctx context.Context, operation PackageOperationRecord, evidence RecoveryIdentityEvidence, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	repo := f.repo()
	artifact, err := repo.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("kernel: ensure artifact metadata unavailable: %w", err)
	}
	if artifact.InstalledPath != "" {
		if evidence.InstalledPath != "" && filepath.Clean(artifact.InstalledPath) != filepath.Clean(evidence.InstalledPath) {
			return f.requireManualRecovery(ctx, operation, fmt.Sprintf("artifact installed path conflict: %s vs %s", artifact.InstalledPath, evidence.InstalledPath), nil, guard)
		}
		return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureArtifactMetadata, recoveryStepOrderEnsureArtifactMetadata, `{"installedPath":"present"}`, guard)
	}
	if evidence.InstalledPath == "" {
		return f.requireManualRecovery(ctx, operation, "artifact installed path cannot be proven", nil, guard)
	}
	if err := repo.SetArtifactInstalledPath(ctx, operation.ArtifactID, evidence.InstalledPath, guard); err != nil {
		return fmt.Errorf("kernel: persist artifact installed path failed: %w", err)
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureArtifactMetadata, recoveryStepOrderEnsureArtifactMetadata, `{"installedPath":"repaired"}`, guard)
}

func (f *PackageRecoveryFinalizer) ensureInstallationReference(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if f.hasArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID) {
		return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureInstallationRef, recoveryStepOrderEnsureInstallationRef, `{"installationReference":"present"}`, guard)
	}
	if _, err := f.repo().AcquireArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID, time.Time{}); err != nil {
		return fmt.Errorf("kernel: acquire installation reference failed: %w", err)
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepEnsureInstallationRef, recoveryStepOrderEnsureInstallationRef, `{"installationReference":"repaired"}`, guard)
}

func (f *PackageRecoveryFinalizer) consumePreviewForRecovery(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if operation.PreviewSessionID == "" {
		return nil
	}
	repo := f.repo()
	if err := repo.ConsumePreview(ctx, operation.PreviewSessionID); err != nil {
		if !IsRepositoryErrorKind(err, RepositoryErrorConflict) {
			return fmt.Errorf("kernel: recovery consume preview failed: %w", err)
		}
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepConsumePreview, recoveryStepOrderConsumePreview, `{"preview":"consumed"}`, guard)
}

func (f *PackageRecoveryFinalizer) releasePreviewReference(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if operation.PreviewSessionID == "" {
		return nil
	}
	if !f.hasArtifactReference(ctx, operation.ArtifactID, ArtifactReferencePreview, operation.PreviewSessionID) {
		return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepReleasePreviewReference, recoveryStepOrderReleasePreviewReference, `{"previewReference":"absent"}`, guard)
	}
	if err := f.repo().ReleaseArtifactReference(ctx, operation.ArtifactID, ArtifactReferencePreview, operation.PreviewSessionID); err != nil {
		return fmt.Errorf("kernel: release preview reference failed: %w", err)
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepReleasePreviewReference, recoveryStepOrderReleasePreviewReference, `{"previewReference":"released"}`, guard)
}

func (f *PackageRecoveryFinalizer) releaseOperationReference(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if !f.hasArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceOperation, operation.OperationID) {
		return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepReleaseOperationRef, recoveryStepOrderReleaseOperationRef, `{"operationReference":"absent"}`, guard)
	}
	if err := f.repo().ReleaseArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceOperation, operation.OperationID); err != nil {
		return fmt.Errorf("kernel: release operation reference failed: %w", err)
	}
	return f.completeRecoveryStep(ctx, operation, completed, RecoveryStepReleaseOperationRef, recoveryStepOrderReleaseOperationRef, `{"operationReference":"released"}`, guard)
}

func (f *PackageRecoveryFinalizer) runRecoveryFinalGate(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	return f.completeRecoveryStepWithFunc(ctx, operation, completed, RecoveryStepRunFinalGate, recoveryStepOrderRunFinalGate, func() error {
		return f.r.runPackageFinalGate(ctx, operation.OperationID, guard)
	}, guard)
}

func (f *PackageRecoveryFinalizer) finalizeOperation(ctx context.Context, operation PackageOperationRecord, completionNote string, guard PackageWriteGuard, completed map[string]PackageOperationStep) error {
	if err := f.completeRecoveryStep(ctx, operation, completed, RecoveryStepFinalizeOperation, recoveryStepOrderFinalizeOperation, `{"finalized":true}`, guard); err != nil {
		return err
	}
	return f.repo().SetOperation(ctx, operation.OperationID, string(PackageOperationFinalizing), "finalizing", "", "", false, guard)
}

func (f *PackageRecoveryFinalizer) FinalizeInstallRecovery(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	if err := f.completeRecoveryStep(ctx, operation, completed, RecoveryStepVerifySideEffects, recoveryStepOrderVerifySideEffects, `{"sideEffects":"verified"}`, guard); err != nil {
		return err
	}
	evidence, evidenceErr := f.gatherIdentityEvidence(ctx, operation)
	if evidenceErr != nil || !evidence.Proven() {
		return f.requireManualRecovery(ctx, operation, "install identity evidence incomplete", evidenceErr, guard)
	}
	plan, planErr := f.planInstallRepair(ctx, operation, evidence)
	if planErr != nil {
		return f.requireManualRecovery(ctx, operation, "install repair plan failed", planErr, guard)
	}
	if plan.NeedVersionRecord {
		if err := f.ensureVersionRecord(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedArtifactInstalledPath {
		if err := f.ensureArtifactInstalledPath(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedInstallationReference {
		if err := f.ensureInstallationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedPreviewConsume {
		if err := f.consumePreviewForRecovery(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedPreviewReferenceRelease {
		if err := f.releasePreviewReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedOperationReferenceRelease {
		if err := f.releaseOperationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if err := f.runRecoveryFinalGate(ctx, operation, guard, completed); err != nil {
		return f.r.requirePackageRecovery(ctx, operation, "install final gate failed during recovery", err, guard)
	}
	return f.finalizeOperation(ctx, operation, "recovered_completed", guard, completed)
}

func (f *PackageRecoveryFinalizer) FinalizeUpdateRecovery(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	if err := f.completeRecoveryStep(ctx, operation, completed, RecoveryStepVerifySideEffects, recoveryStepOrderVerifySideEffects, `{"sideEffects":"verified"}`, guard); err != nil {
		return err
	}
	evidence, evidenceErr := f.gatherIdentityEvidence(ctx, operation)
	if evidenceErr != nil || !evidence.Proven() {
		return f.requireManualRecovery(ctx, operation, "update identity evidence incomplete", evidenceErr, guard)
	}
	plan, planErr := f.planUpdateRepair(ctx, operation, evidence, completed)
	if planErr != nil {
		return f.requireManualRecovery(ctx, operation, "update repair plan failed", planErr, guard)
	}
	if plan.NeedRollbackPointVerify {
		if _, ok := completed[StepCreateRollbackPoint]; !ok {
			return f.requireManualRecovery(ctx, operation, "update rollback point step missing", nil, guard)
		}
	}
	if plan.NeedMigrationJournalVerify {
		if _, ok := completed[StepExecuteMigrations]; !ok {
			return f.requireManualRecovery(ctx, operation, "update migration journal step missing", nil, guard)
		}
	}
	if plan.NeedVersionRecord {
		if err := f.ensureVersionRecord(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedArtifactInstalledPath {
		if err := f.ensureArtifactInstalledPath(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedInstallationReference {
		if err := f.ensureInstallationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedPreviewConsume {
		if err := f.consumePreviewForRecovery(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedPreviewReferenceRelease {
		if err := f.releasePreviewReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedOperationReferenceRelease {
		if err := f.releaseOperationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if err := f.runRecoveryFinalGate(ctx, operation, guard, completed); err != nil {
		return f.r.requirePackageRecovery(ctx, operation, "update final gate failed during recovery", err, guard)
	}
	return f.finalizeOperation(ctx, operation, "recovered_completed", guard, completed)
}

func (f *PackageRecoveryFinalizer) FinalizeRollbackRecovery(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	if err := f.completeRecoveryStep(ctx, operation, completed, RecoveryStepVerifySideEffects, recoveryStepOrderVerifySideEffects, `{"sideEffects":"verified"}`, guard); err != nil {
		return err
	}
	evidence, evidenceErr := f.gatherIdentityEvidence(ctx, operation)
	if evidenceErr != nil || !evidence.Proven() {
		return f.requireManualRecovery(ctx, operation, "rollback identity evidence incomplete", evidenceErr, guard)
	}
	plan, planErr := f.planRollbackRepair(ctx, operation, evidence)
	if planErr != nil {
		return f.requireManualRecovery(ctx, operation, "rollback repair plan failed", planErr, guard)
	}
	if plan.NeedVersionRecord {
		if err := f.ensureVersionRecord(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedArtifactInstalledPath {
		if err := f.ensureArtifactInstalledPath(ctx, operation, evidence, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedInstallationReference {
		if err := f.ensureInstallationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if plan.NeedOperationReferenceRelease {
		if err := f.releaseOperationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if err := f.runRecoveryFinalGate(ctx, operation, guard, completed); err != nil {
		return f.r.requirePackageRecovery(ctx, operation, "rollback final gate failed during recovery", err, guard)
	}
	return f.finalizeOperation(ctx, operation, "recovered_completed", guard, completed)
}

func (f *PackageRecoveryFinalizer) FinalizeUninstallRecovery(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	if err := f.completeRecoveryStep(ctx, operation, completed, RecoveryStepVerifySideEffects, recoveryStepOrderVerifySideEffects, `{"sideEffects":"verified"}`, guard); err != nil {
		return err
	}
	plan := f.planUninstallRepair(ctx, operation)
	if plan.NeedUninstallVersionRemoved {
		repo := f.repo()
		if existing, vErr := repo.GetPackageVersion(ctx, operation.ExtensionID, operation.TargetVersion); vErr == nil {
			if existing.VersionState != string(PackageVersionStateRemoved) {
				if err := repo.RemovePackageVersion(ctx, operation.ExtensionID, operation.TargetVersion); err != nil {
					if !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
						return f.requireManualRecovery(ctx, operation, "uninstall version remove failed", err, guard)
					}
				}
			}
		}
	}
	if plan.NeedOperationReferenceRelease {
		if err := f.releaseOperationReference(ctx, operation, guard, completed); err != nil {
			return err
		}
	}
	if err := f.runRecoveryFinalGate(ctx, operation, guard, completed); err != nil {
		return f.r.requirePackageRecovery(ctx, operation, "uninstall final gate failed during recovery", err, guard)
	}
	return f.finalizeOperation(ctx, operation, "recovered_completed", guard, completed)
}
