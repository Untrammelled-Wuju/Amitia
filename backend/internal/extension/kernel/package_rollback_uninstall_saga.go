package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type PackageUninstallPreviewResult struct {
	ExtensionID             string         `json:"extensionId"`
	CurrentVersion          string         `json:"currentVersion"`
	CurrentVersionID        string         `json:"currentVersionId,omitempty"`
	CurrentGenerationID     string         `json:"currentGenerationId,omitempty"`
	Generation              int64          `json:"generation"`
	Enabled                 bool           `json:"enabled"`
	Dependents              []string       `json:"dependents"`
	InstalledPath           string         `json:"installedPath"`
	InstalledHash           string         `json:"installedTreeHash"`
	ArtifactID              string         `json:"artifactId"`
	Installable             bool           `json:"uninstallable"`
	GenerationID            string         `json:"generationId"`
	OperationID             string         `json:"operationId"`
	ArtifactPolicy          ArtifactPolicy `json:"artifactPolicy,omitempty"`
	PolicyReason            string         `json:"policyReason,omitempty"`
	PreviewHash             string         `json:"previewHash,omitempty"`
	SecurityPolicyHash      string         `json:"securityPolicyHash,omitempty"`
	SnapshotRequirementHash string         `json:"snapshotRequirementHash,omitempty"`
	UserID                  string         `json:"userId,omitempty"`
	ScopeType               string         `json:"scopeType,omitempty"`
	ScopeID                 string         `json:"scopeId,omitempty"`
}

func (r *Runtime) ExecutePackageRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID, confirmationToken string) (KernelInstallResult, error) {
	rollbackClaims, err := verifyPackageRollbackConfirmation(confirmationToken)
	if err != nil {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, err)
	}
	if rollbackClaims.OperationType != string(PackageOperationTypeRollback) {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, ErrPackageRollbackTokenInvalid)
	}
	if rollbackClaims.PolicyVersion != packagePolicyVersion || rollbackClaims.SecurityPolicyHash != computeSecurityPolicyHash() {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeConfirmationPolicyVersionStale, 403, ErrPackageConfirmationPolicyVersionStale)
	}
	if rollbackClaims.ExtensionID != extensionID {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, ErrPackageRollbackTokenInvalid)
	}
	if rollbackClaims.TargetVersionID != version {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, ErrPackageRollbackTokenInvalid)
	}
	if rollbackClaims.UserID != userID {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, ErrPackageRollbackTokenInvalid)
	}
	if rollbackClaims.ScopeType != scopeType || rollbackClaims.ScopeID != scopeID {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 403, ErrPackageRollbackTokenInvalid)
	}
	claimsJSON, _ := json.Marshal(rollbackClaims)
	point, err := r.container.PackageRepository.GetRollbackPoint(ctx, extensionID, version)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback point unavailable: %w", err)
	}
	if err := validatePackageSnapshot(point); err != nil {
		return KernelInstallResult{}, err
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback artifact unavailable: %w", err)
	}
	pkg, err := r.VerifyStoredPackage(ctx, artifact)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback artifact verification failed: %w", err)
	}
	dependencyPreview := InstallPreview{}
	r.evaluatePackageCompatibilityAndDependencies(ctx, pkg, &dependencyPreview)
	if len(dependencyPreview.Issues) > 0 {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback dependency or compatibility check failed")
	}
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return KernelInstallResult{}, err
	}
	if err := validatePackageOwner(current, userID, scopeType, scopeID); err != nil {
		return KernelInstallResult{}, err
	}
	currentDefinition, err := r.container.DefinitionRepository.GetExtension(ctx, current.ExtensionID, current.InstalledVersion)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentModules, err := r.container.ModuleRepository.ListModules(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentContributions, err := r.container.ContributionRepository.ListContributions(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	idempotencyKey := computeSimplePackageIdempotencyKey("rollback", extensionID, version, userID, scopeType, scopeID)
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	confirmedItemsJSON, _ := json.Marshal(confirmedItemsToMap(rollbackClaims.ConfirmedItems))
	rollbackOp := PackageOperationRecord{OperationID: operationID, TraceID: traceID,
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: version, FromVersion: current.InstalledVersion.String(),
		OperationType: "rollback", Status: "created", CurrentStep: "created",
		ArtifactID: artifact.ArtifactID, ConfirmationsJSON: string(confirmedItemsJSON),
		ConfirmationClaimsJSON: string(claimsJSON), SnapshotRequirementHash: rollbackClaims.SnapshotRequirementHash,
		PreviewSessionID: rollbackClaims.PreviewSessionID,
		IdempotencyKey:   idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "rollback", ExtensionID: extensionID, TargetVersion: version,
			ArtifactID: artifact.ArtifactID, ScopeType: scopeType, ScopeID: scopeID,
			PreviewSessionID: rollbackClaims.PreviewSessionID,
		}), StartedAt: now, UpdatedAt: now}
	existing, created, createErr := r.container.PackageRepository.CreateOrGetOperationWithConfirmationNonce(ctx, rollbackOp, PackageConfirmationNonceBinding{
		Nonce: rollbackClaims.Nonce, OperationType: rollbackOp.OperationType, ExtensionID: rollbackOp.ExtensionID, UserID: rollbackOp.UserID,
		IssuedAt: confirmationTimestamp(rollbackClaims.IssuedAt), ExpiresAt: confirmationTimestamp(rollbackClaims.ExpiresAt),
	})
	if createErr != nil {
		return KernelInstallResult{}, createErr
	}
	if !created {
		if result, handled, handleErr := r.handleExistingPackageOperation(ctx, existing); handled {
			return result, handleErr
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, extensionID, operationID)
	if leaseErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: extension %s has an active operation: %w", extensionID, leaseErr)
	}
	leaseGuard := r.newPackageLeaseGuard(extensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "start_lease_guard", "PACKAGE_OPERATION_LEASE_CONFLICT", startErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: lease guard start failed: %w", startErr)
	}
	defer func() {
		if stopErr := leaseGuard.Stop(context.Background()); stopErr != nil {
			if putErr := r.container.PackageRepository.PutConsistencyFinding(context.Background(), PackageConsistencyFinding{
				FindingID:         "stale-lease-" + operationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   fmt.Sprintf(`["%s"]`, operationID),
				ErrorDetail:       stopErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			}); putErr != nil {
				fmt.Printf("kernel: failed to persist stale lease finding for %s: %v\n", operationID, errors.Join(stopErr, putErr))
			}
		}
	}()
	ctx = sagaCtx
	guard := packageWriteGuard(lease)
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID}
	postLeasePreview, previewErr := r.PreviewPackageRollback(ctx, extensionID, version, userID, scopeType, scopeID)
	if previewErr != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "post_lease_preview", "PACKAGE_CONFIRMATION_STALE", fmt.Sprintf("post-lease preview failed: %v", previewErr), true, guard)
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		return KernelInstallResult{}, errors.Join(NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("post-lease preview unavailable: %w", previewErr)), failErr)
	}
	if driftErr := samePackageRollbackPreview(rollbackClaims, postLeasePreview); driftErr != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "post_lease_preview_drift", "PACKAGE_CONFIRMATION_STALE", fmt.Sprintf("post-lease drift detected: %v", driftErr), true, guard)
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		return KernelInstallResult{}, errors.Join(NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("post-lease drift: %w", driftErr)), failErr)
	}
	postLeaseCurrent, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: post-lease installation unavailable: %w", err)
	}
	postLeaseTargetVersion, err := r.container.PackageRepository.GetPackageVersion(ctx, extensionID, version)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: post-lease target version unavailable: %w", err)
	}
	postLeaseRequirementInput, postLeaseRequirement, err := r.buildRollbackPackageSnapshotRequirement(ctx, postLeaseCurrent, point, postLeaseTargetVersion)
	if err != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "post_lease_snapshot_requirement", PackageErrCodeConfirmationStale, err.Error(), true, guard)
		return KernelInstallResult{}, errors.Join(NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("post-lease snapshot requirement unavailable: %w", err)), failErr)
	}
	if postLeaseRequirement.Hash != postLeasePreview.SnapshotRequirementHash ||
		postLeaseRequirement.Hash != rollbackClaims.SnapshotRequirementHash ||
		postLeaseRequirement.Hash != rollbackOp.SnapshotRequirementHash {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "post_lease_snapshot_requirement_drift", PackageErrCodeConfirmationStale, "snapshot requirement hash changed after lease", true, guard)
		return KernelInstallResult{}, errors.Join(NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("snapshot requirement hash changed after lease")), failErr)
	}
	authorityInput, err := buildRollbackAuthorityInput(postLeasePreview)
	if err != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "recompute_confirmation_authority", PackageErrCodeConfirmationStale, err.Error(), true, guard)
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		return KernelInstallResult{}, errors.Join(err, failErr)
	}

	standardClaims := standardConfirmationClaimsFromRollback(rollbackClaims)

	rollbackEvidence, err := buildPackageConfirmationAuthorityEvidence(op.OperationID, standardClaims, authorityInput)
	if err != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "validate_confirmation_authority", PackageErrCodeConfirmationStale, err.Error(), true, guard)
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		return KernelInstallResult{}, errors.Join(err, failErr)
	}

	rollbackEvidence.SnapshotRequirementInput = &postLeaseRequirementInput
	rollbackEvidence.SnapshotRequirement = &postLeaseRequirement
	rollbackEvidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(rollbackEvidence)

	if evidenceErr := r.persistPackageConfirmationAuthorityEvidence(ctx, rollbackEvidence, guard); evidenceErr != nil {
		failErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "persist_authority_evidence", "PACKAGE_EVIDENCE_PERSIST_FAILED", fmt.Sprintf("authority evidence persistence failed: %v", evidenceErr), true, guard)
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		return KernelInstallResult{}, errors.Join(fmt.Errorf("kernel: persist rollback authority evidence: %w", evidenceErr), failErr)
	}
	revalidatedArtifact, revalArtErr := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if revalArtErr != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback re-preview artifact unavailable: %w", revalArtErr)
	}
	if revalidatedArtifact.ArtifactID != artifact.ArtifactID {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeRollbackTokenInvalid, 409, fmt.Errorf("rollback re-preview: artifact identity changed"))
	}
	reverifiedPkg, reVerifyErr := r.VerifyStoredPackage(ctx, revalidatedArtifact)
	if reVerifyErr != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback re-preview artifact verification failed: %w", reVerifyErr)
	}
	reverifiedDepPreview := InstallPreview{}
	r.evaluatePackageCompatibilityAndDependencies(ctx, reverifiedPkg, &reverifiedDepPreview)
	if len(reverifiedDepPreview.Issues) > 0 {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback re-preview dependency or compatibility check failed")
	}
	forwardPoint, err := r.createPackageRollbackPoint(ctx, op.OperationID, "forward_recovery", current, &currentDefinition, currentModules, currentContributions)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "create_forward_recovery_point", "PACKAGE_FORWARD_RECOVERY_POINT_FAILED", err.Error(), true, guard)
		return KernelInstallResult{}, errors.Join(err, persistErr)
	}
	if manualReason := packageSnapshotManualRecoveryReason(forwardPoint, point); manualReason != "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "requires_manual_recovery", "PACKAGE_MANUAL_RECOVERY_REQUIRED", manualReason, false, guard)
		return KernelInstallResult{}, errors.Join(fmt.Errorf("kernel: requires_manual_recovery: %s", manualReason), persistErr)
	}
	var definition domain.ExtensionDefinition
	var modules []domain.ModuleDefinition
	var contributions []domain.ContributionDefinition
	var permissionSnapshot struct {
		Requirements []sqlite.PermissionRequirement `json:"requirements"`
		Grants       []sqlite.PermissionGrant       `json:"grants"`
	}
	var scopeSnapshot []sqlite.ScopeBinding
	if json.Unmarshal([]byte(point.DefinitionSnapshotJSON), &definition) != nil || json.Unmarshal([]byte(point.ModuleSnapshotJSON), &modules) != nil || json.Unmarshal([]byte(point.ContributionSnapshotJSON), &contributions) != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback point corrupt")
	}
	if json.Unmarshal([]byte(point.PermissionSnapshotJSON), &permissionSnapshot) != nil || json.Unmarshal([]byte(point.ScopeSnapshotJSON), &scopeSnapshot) != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback policy snapshot corrupt")
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "validate_rollback_point", 1, guard); err != nil {
		return KernelInstallResult{}, err
	}
	sourcePath := point.InstalledPath
	var rollbackStagingID string
	if info, statErr := os.Stat(sourcePath); statErr != nil || !info.IsDir() {
		staging, extractErr := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, op.OperationID)
		if extractErr != nil {
			return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "rebuild_rollback_generation", extractErr, guard)
		}
		sourcePath = staging.Path
		rollbackStagingID = staging.ID
		defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), rollbackStagingID)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	targetGeneration, stableGeneration, err := r.preparePackageGeneration(ctx, op.OperationID, artifact, sourcePath, guard.FencingToken)
	if err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "commit_rollback_generation", err, guard)
	}
	stableFromDB := packageGenerationFromInstallation(current)
	if stableFromDB.GenerationID != "" && stableFromDB.GenerationID != stableGeneration.GenerationID {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "validate_current_pointer", ErrPackageGenerationCAS, guard)
	}
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, op.OperationID, stableGeneration.GenerationID, targetGeneration.Current.GenerationID, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "persist_generation_evidence", err, guard)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "commit_rollback_generation", 2, stableGeneration, targetGeneration.Current, packageJSON(map[string]string{"path": targetGeneration.GenerationPath, "treeHash": targetGeneration.Current.TreeHash}), guard); err != nil {
		if compErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, false); compErr != nil {
			return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "commit_rollback_generation", errors.Join(err, fmt.Errorf("generation compensation failed: %w", compErr)), guard)
		}
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "commit_rollback_generation", err, guard)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	if err := r.switchPackageGeneration(ctx, stableGeneration, targetGeneration); err != nil {
		if compErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, false); compErr != nil {
			return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", errors.Join(err, fmt.Errorf("generation compensation failed: %w", compErr)), guard)
		}
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", err, guard)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "switch_current_pointer", 3, stableGeneration, targetGeneration.Current, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		if compErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, true); compErr != nil {
			return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", errors.Join(err, fmt.Errorf("generation compensation failed: %w", compErr)), guard)
		}
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", err, guard)
	}
	compensateRollback := func(step string, cause error) error {
		if r.container.ResourceSnapshotStore != nil {
			restoreErr := r.container.ResourceSnapshotStore.RestoreQuarantinedResources(context.Background(), op.OperationID)
			if restoreErr != nil {
				cause = errors.Join(cause, fmt.Errorf("restore quarantined resources: %w", restoreErr))
			}
		}
		generationErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, true)
		forwardErr := r.restoreForwardPackagePoint(context.Background(), forwardPoint)
		if generationErr == nil && forwardErr == nil {
			forwardErr = r.rebindPackageInstallationGeneration(context.Background(), stableGeneration, forwardPoint.InstalledPath)
		}
		if generationErr != nil || forwardErr != nil {
			detail := errors.Join(cause, generationErr, forwardErr)
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "forward_recovery_failed", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
			return errors.Join(detail, persistErr)
		}
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", step, "PACKAGE_ROLLBACK_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	replacedVersion := current.InstalledVersion.String()
	current.InstalledVersion = definition.Version
	current.PackageID = artifact.ArtifactID
	current.Generation++
	current.EnablementState = domain.EnablementDisabled
	current.UpdatedAt = time.Now().UTC()
	current.Metadata = packageInstallationMetadata(map[string]any{"installedPath": targetGeneration.GenerationPath, "artifactId": artifact.ArtifactID,
		"archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash,
		"contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash,
		"installedTreeHash": targetGeneration.Current.TreeHash,
		"ownerUserId":       userID, "scopeType": scopeType, "scopeId": scopeID}, targetGeneration.Current, targetGeneration.GenerationPath, op.OperationID)
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if r.container.ResourceSnapshotStore != nil {
		var resourceSnapshot packageResourceSnapshot
		if json.Unmarshal([]byte(point.ResourceSnapshotJSON), &resourceSnapshot) == nil && len(resourceSnapshot.Entries) > 0 {
			currentResources, listErr := r.container.ResourceRepository.ListResources(ctx, current.ExtensionID)
			if listErr != nil {
				return KernelInstallResult{}, compensateRollback("list_current_resources", listErr)
			}
			if _, qErr := r.container.ResourceSnapshotStore.QuarantineNewResources(ctx, extensionID, op.OperationID, resourceSnapshot.Entries, currentResources); qErr != nil {
				return KernelInstallResult{}, compensateRollback("quarantine_new_resources", qErr)
			}
		}
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, guard); err != nil {
			return err
		}
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, current.ExtensionID); err != nil {
			return err
		}
		currentGrants, err := r.container.PermissionRepository.ListGrants(txCtx, current.ExtensionID)
		if err != nil {
			return err
		}
		for _, grant := range currentGrants {
			if err := r.container.PermissionRepository.DeleteGrant(txCtx, current.ExtensionID, grant.PermissionName); err != nil {
				return err
			}
		}
		if err := r.container.ScopeRepository.DeleteBindings(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.PutExtension(txCtx, definition); err != nil {
			return err
		}
		for _, module := range modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, module); err != nil {
				return err
			}
		}
		for _, contribution := range contributions {
			if err := r.container.ContributionRepository.PutContribution(txCtx, contribution); err != nil {
				return err
			}
		}
		for _, requirement := range permissionSnapshot.Requirements {
			if err := r.container.PermissionRepository.PutRequirement(txCtx, requirement); err != nil {
				return err
			}
		}
		for _, grant := range permissionSnapshot.Grants {
			if err := r.container.PermissionRepository.PutGrant(txCtx, grant); err != nil {
				return err
			}
		}
		for _, binding := range scopeSnapshot {
			if err := r.container.ScopeRepository.PutBinding(txCtx, binding); err != nil {
				return err
			}
		}
		if err := r.restorePackageRepositorySnapshots(txCtx, current.ExtensionID, point, &current); err != nil {
			return err
		}
		current.Metadata = packageInstallationMetadata(current.Metadata, targetGeneration.Current, targetGeneration.GenerationPath, op.OperationID)
		return r.container.InstallationRepository.PutInstallation(txCtx, current)
	})
	if err != nil {
		return KernelInstallResult{}, compensateRollback("restore_repositories", err)
	}
	if err := r.restorePackageMigrationState(ctx, point); err != nil {
		return KernelInstallResult{}, compensateRollback("restore_migration_state", err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, artifact.ArtifactID, targetGeneration.GenerationPath, guard); err != nil {
		return KernelInstallResult{}, compensateRollback("persist_artifact_metadata", err)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "restore_repositories", 4, stableGeneration, targetGeneration.Current, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, compensateRollback("record_restore_completion", err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.recordPackageVersionAfterOperation(ctx, op.OperationID, "rollback", extensionID, version, artifact.ArtifactID, targetGeneration.GenerationPath, targetGeneration.Current.TreeHash, artifact.ArchiveHash, artifact.ManifestHash, artifact.ContentTreeHash, targetGeneration.Current.GenerationID, guard); err != nil {
		return KernelInstallResult{}, compensateRollback("record_version", err)
	}
	if replacedVersion != "" && replacedVersion != version {
		if err := r.container.PackageRepository.MarkPackageVersionRollbackAvailable(ctx, extensionID, replacedVersion); err != nil {
			return KernelInstallResult{}, compensateRollback("mark_rollback_available", err)
		}
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.FinalizePackageOperation(ctx, op.OperationID, extensionID, leaseGuard, guard); err != nil {
		compErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, true)
		restoreErr := r.restoreForwardPackagePoint(context.Background(), forwardPoint)
		rebindErr := r.rebindPackageInstallationGeneration(context.Background(), stableGeneration, forwardPoint.InstalledPath)
		compensationErr := errors.Join(compErr, restoreErr, rebindErr)
		if compensationErr != nil {
			return KernelInstallResult{}, errors.Join(err, fmt.Errorf("rollback compensation failed: %w", compensationErr))
		}
		return KernelInstallResult{}, err
	}
	if r.container.ResourceSnapshotStore != nil {
		if err := r.container.ResourceSnapshotStore.PurgeQuarantinedResources(ctx, op.OperationID); err != nil {
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "purge_resource_quarantine", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, guard)
			return KernelInstallResult{}, errors.Join(err, persistErr)
		}
	}
	return KernelInstallResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: "rollback",
		ExtensionID: extensionID, Version: version, InstallationID: current.InstallationID,
		PackageHash: artifact.ArchiveHash, ContentTreeHash: artifact.ContentTreeHash,
		ArtifactPath: artifact.ArchivePath, InstallPath: targetGeneration.GenerationPath,
		DefinitionHash: artifact.ArtifactHash, InstalledAt: time.Now().UTC()}, nil
}

func (r *Runtime) failPackageRollbackWithForwardRecovery(operationID string, forwardPoint PackageRollbackPoint, step string, cause error, guard PackageWriteGuard) error {
	compensationErr := r.restoreForwardPackagePoint(context.Background(), forwardPoint)
	if compensationErr != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "forward_recovery_failed", "PACKAGE_RECOVERY_REQUIRED", errors.Join(cause, compensationErr).Error(), false, guard)
		return errors.Join(cause, compensationErr, persistErr)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", step, "PACKAGE_ROLLBACK_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) PreviewPackageUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreviewResult, error) {
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return PackageUninstallPreviewResult{}, err
	}
	if err := validatePackageOwner(installation, userID, scopeType, scopeID); err != nil {
		return PackageUninstallPreviewResult{}, err
	}
	result := PackageUninstallPreviewResult{ExtensionID: extensionID,
		CurrentVersion: installation.InstalledVersion.String(),
		Generation:     installation.Generation,
		Enabled:        installation.EnablementState == domain.EnablementEnabled, Installable: true}
	result.InstalledPath, _ = installation.Metadata["installedPath"].(string)
	result.InstalledHash, _ = installation.Metadata["installedTreeHash"].(string)
	result.ArtifactID, _ = installation.Metadata["artifactId"].(string)
	result.GenerationID, _ = installation.Metadata["generationId"].(string)
	result.OperationID, _ = installation.Metadata["lastOperationId"].(string)
	if result.InstalledPath == "" || result.InstalledHash == "" {
		return result, fmt.Errorf("kernel: uninstall installation evidence incomplete")
	}
	if result.GenerationID != "" && r.container.PackageGenerationStore != nil {
		current, currentErr := r.container.PackageGenerationStore.ReadCurrent(extensionID)
		if currentErr != nil || current.GenerationID != result.GenerationID || current.ArtifactID != result.ArtifactID || current.TreeHash != result.InstalledHash {
			return result, fmt.Errorf("kernel: uninstall current pointer mismatch")
		}
		if verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, current); verifyErr != nil {
			return result, fmt.Errorf("kernel: uninstall generation verification failed: %w", verifyErr)
		}
	} else {
		actualHash := package_security.ComputeDirHash(result.InstalledPath, r.container.PackageSecurity.GetHasher())
		if actualHash == "" || actualHash != result.InstalledHash {
			return result, fmt.Errorf("kernel: uninstall installed tree verification failed")
		}
	}
	if r.container.PackageRepository != nil {
		currentVer, verErr := r.container.PackageRepository.GetCurrentPackageVersion(ctx, extensionID)
		if verErr != nil {
			return result, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: uninstall version identity unavailable: %w", verErr))
		}
		if currentVer.VersionID == "" {
			return result, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: uninstall version identity missing"))
		}
		if currentVer.GenerationID != "" && result.GenerationID != "" && currentVer.GenerationID != result.GenerationID {
			return result, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: uninstall generation identity drift between version repository and installation"))
		}
		result.CurrentVersionID = currentVer.VersionID
		if currentVer.GenerationID != "" {
			result.CurrentGenerationID = currentVer.GenerationID
		} else {
			result.CurrentGenerationID = result.GenerationID
		}
	}
	result.SecurityPolicyHash = computeSecurityPolicyHash()
	uninstallReq := ComputeUninstallSnapshotRequirement(computeUninstallSnapshotRequirementInput(
		result.InstalledPath, result.InstalledHash, result.ArtifactID, result.ExtensionID, result.CurrentVersionID))
	result.SnapshotRequirementHash = uninstallReq.RequirementHash
	result.UserID = userID
	result.ScopeType = scopeType
	result.ScopeID = scopeID
	if result.CurrentVersionID == "" || result.CurrentGenerationID == "" || result.SecurityPolicyHash == "" || result.SnapshotRequirementHash == "" {
		return result, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: uninstall preview identity incomplete"))
	}
	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return result, err
	}
	for _, definition := range definitions {
		if definition.ID == installation.ExtensionID {
			continue
		}
		dependentInstallation, dependentErr := r.container.InstallationRepository.GetInstallation(ctx, definition.ID)
		if dependentErr != nil || dependentInstallation.InstalledVersion.Compare(definition.Version) != 0 {
			continue
		}
		for _, dependency := range definition.Dependencies {
			if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
				result.Dependents = append(result.Dependents, string(definition.ID))
			}
		}
		for _, module := range definition.Modules {
			for _, dependency := range module.Dependencies {
				if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
					result.Dependents = append(result.Dependents, string(definition.ID))
				}
			}
			for _, contribution := range module.Contributions {
				for _, dependency := range contribution.Dependencies {
					if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
						result.Dependents = append(result.Dependents, string(definition.ID))
					}
				}
			}
		}
	}
	result.Dependents = uniquePackageStrings(result.Dependents)
	result.Installable = !result.Enabled && len(result.Dependents) == 0
	policy, policyReason, policyErr := r.computeUninstallArtifactPolicy(ctx, result.ArtifactID, extensionID, result.CurrentVersion)
	if policyErr != nil {
		switch {
		case IsPackageErrorCode(policyErr, PackageErrCodeExportRetentionUnsupported):
			return result, policyErr
		case IsPackageErrorCode(policyErr, PackageErrCodeUninstallArtifactPolicyUnproven):
			return result, policyErr
		case result.ArtifactID == "":
			return result, policyErr
		default:
			result.ArtifactPolicy = ArtifactPolicyRetainArtifact
			result.PolicyReason = policyReason
		}
	} else {
		result.ArtifactPolicy = policy
		result.PolicyReason = policyReason
	}
	result.PreviewHash = computeUninstallPreviewHash(result)
	return result, nil
}

func (r *Runtime) computeUninstallArtifactPolicy(
	ctx context.Context,
	artifactID string,
	extensionID string,
	currentVersion string,
) (ArtifactPolicy, string, error) {
	if artifactID == "" {
		return ArtifactPolicyRetainArtifact,
			"artifact id missing, fail closed to retain",
			NewPackageError(
				PackageErrCodeUninstallArtifactMissing,
				422,
				fmt.Errorf("artifact id missing for uninstall"),
			)
	}

	if r.container == nil ||
		r.container.PackageRepository == nil {
		return ArtifactPolicyRetainArtifact,
			"repository unavailable, fail closed to retain",
			NewPackageError(
				PackageErrCodeQuarantineMetadataUnavailable,
				503,
				fmt.Errorf("package repository unavailable"),
			)
	}

	repository := r.container.PackageRepository

	point, pointErr := repository.GetRollbackPoint(
		ctx,
		extensionID,
		currentVersion,
	)

	if pointErr == nil {
		_, bindingErr := r.verifyRollbackRetentionBinding(
			ctx,
			extensionID,
			currentVersion,
			artifactID,
		)

		if bindingErr != nil {
			return ArtifactPolicyRetainArtifact,
				"rollback point exists but its version/generation/snapshot/reference binding is invalid",
				NewPackageError(
					PackageErrCodeUninstallArtifactPolicyUnproven,
					409,
					bindingErr,
				)
		}

		return ArtifactPolicyRetainForRollback,
			fmt.Sprintf(
				"artifact retained by verified rollback point %s",
				point.RollbackPointID,
			),
			nil
	}

	if !IsRepositoryErrorKind(
		pointErr,
		RepositoryErrorNotFound,
	) {
		return ArtifactPolicyRetainArtifact,
			fmt.Sprintf(
				"rollback point lookup failed: %v",
				pointErr,
			),
			NewPackageError(
				PackageErrCodeUninstallArtifactPolicyUnproven,
				503,
				pointErr,
			)
	}

	exportReferenceCount, exportErr := repository.CountActiveArtifactReferencesByType(
		ctx,
		artifactID,
		ArtifactReferenceExportLease,
	)

	if exportErr != nil {
		return ArtifactPolicyRetainArtifact,
			fmt.Sprintf(
				"export reference check failed: %v",
				exportErr,
			),
			NewPackageError(
				PackageErrCodeQuarantineMetadataUnavailable,
				503,
				exportErr,
			)
	}

	if exportReferenceCount > 0 {
		return ArtifactPolicyRetainForExport,
			"export retention requires an authoritative export lifecycle model",
			NewPackageError(
				PackageErrCodeExportRetentionUnsupported,
				422,
				ErrPackageExportRetentionUnsupported,
			)
	}

	referenceCount, referenceErr := repository.CountActiveArtifactReferences(
		ctx,
		artifactID,
	)

	if referenceErr != nil {
		return ArtifactPolicyRetainArtifact,
			fmt.Sprintf(
				"reference count check failed: %v",
				referenceErr,
			),
			NewPackageError(
				PackageErrCodeQuarantineMetadataUnavailable,
				503,
				referenceErr,
			)
	}

	if referenceCount > 0 {
		return ArtifactPolicyRetainArtifact,
			fmt.Sprintf(
				"artifact has %d remaining references",
				referenceCount,
			),
			nil
	}

	return ArtifactPolicyDeleteArtifact,
		"artifact has no rollback, export, or manual references",
		nil
}

func computeUninstallPreviewHash(preview PackageUninstallPreviewResult) string {
	sort.Strings(preview.Dependents)
	dependentsJSON, _ := json.Marshal(preview.Dependents)
	canonical := fmt.Sprintf(`{"extensionId":%q,"currentVersion":%q,"currentVersionId":%q,"currentGenerationId":%q,"generation":%d,"artifactId":%q,"generationId":%q,"installedPath":%q,"installedTreeHash":%q,"artifactPolicy":%q,"securityPolicyHash":%q,"snapshotRequirementHash":%q,"dependents":%q,"userId":%q,"scopeType":%q,"scopeId":%q}`,
		preview.ExtensionID, preview.CurrentVersion, preview.CurrentVersionID, preview.CurrentGenerationID,
		preview.Generation, preview.ArtifactID, preview.GenerationID, preview.InstalledPath,
		preview.InstalledHash, string(preview.ArtifactPolicy),
		preview.SecurityPolicyHash, preview.SnapshotRequirementHash, string(dependentsJSON),
		preview.UserID, preview.ScopeType, preview.ScopeID)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

func requiredUninstallConfirmations(preview PackageUninstallPreviewResult) []string {
	items := []string{"confirm.uninstall"}

	switch preview.ArtifactPolicy {
	case ArtifactPolicyDeleteArtifact:
		items = append(items, "confirm.uninstall.delete")
	case ArtifactPolicyRetainArtifact:
		items = append(items, "confirm.uninstall.retain")
	case ArtifactPolicyRetainForRollback:
		items = append(items, "confirm.uninstall.retain_for_rollback")
	case ArtifactPolicyRetainForExport:
		items = append(items, "confirm.uninstall.retain_for_export")
	default:
		items = append(items, "confirm.uninstall.delete")
	}

	if len(preview.Dependents) > 0 {
		items = append(items, "confirm.uninstall.dependents_affected")
	}

	if preview.SnapshotRequirementHash != "" {
		items = append(items, "confirm.uninstall.data_change")
	}

	return items
}

func buildUninstallPreviewIdentity(preview PackageUninstallPreviewResult, policyVersion string) PackageUninstallPreviewIdentity {
	sort.Strings(preview.Dependents)
	dependentsJSON, _ := json.Marshal(preview.Dependents)
	dependentsHash := sha256.Sum256(dependentsJSON)
	return PackageUninstallPreviewIdentity{
		ExtensionID:             preview.ExtensionID,
		ArtifactID:              preview.ArtifactID,
		ArtifactPolicy:          preview.ArtifactPolicy,
		PolicyReason:            preview.PolicyReason,
		CurrentVersionID:        preview.CurrentVersionID,
		CurrentGenerationID:     preview.CurrentGenerationID,
		CurrentVersion:          preview.CurrentVersion,
		InstalledPath:           preview.InstalledPath,
		InstalledTreeHash:       preview.InstalledHash,
		DependentsHash:          "sha256:" + hex.EncodeToString(dependentsHash[:]),
		SecurityPolicyHash:      preview.SecurityPolicyHash,
		SnapshotRequirementHash: preview.SnapshotRequirementHash,
		UserID:                  preview.UserID,
		ScopeType:               preview.ScopeType,
		ScopeID:                 preview.ScopeID,
		PolicyVersion:           policyVersion,
		PreviewHash:             preview.PreviewHash,
	}
}

func compareUninstallPreviewIdentity(left, right PackageUninstallPreviewIdentity) (bool, string) {
	if left.ExtensionID != right.ExtensionID {
		return false, "extension_changed"
	}
	if left.ArtifactID != right.ArtifactID {
		return false, "artifact_changed"
	}
	if left.ArtifactPolicy != right.ArtifactPolicy {
		return false, "artifact_policy_changed"
	}
	if left.CurrentVersionID != right.CurrentVersionID {
		return false, "version_changed"
	}
	if left.CurrentGenerationID != right.CurrentGenerationID {
		return false, "generation_changed"
	}
	if left.CurrentVersion != right.CurrentVersion {
		return false, "version_label_changed"
	}
	if left.InstalledPath != right.InstalledPath {
		return false, "installed_path_changed"
	}
	if left.InstalledTreeHash != right.InstalledTreeHash {
		return false, "installed_tree_hash_changed"
	}
	if left.DependentsHash != right.DependentsHash {
		return false, "dependents_changed"
	}
	if left.SecurityPolicyHash != right.SecurityPolicyHash {
		return false, "security_policy_changed"
	}
	if left.SnapshotRequirementHash != right.SnapshotRequirementHash {
		return false, "snapshot_requirement_changed"
	}
	if left.UserID != right.UserID {
		return false, "user_changed"
	}
	if left.ScopeType != right.ScopeType {
		return false, "scope_type_changed"
	}
	if left.ScopeID != right.ScopeID {
		return false, "scope_id_changed"
	}
	if left.PolicyVersion != right.PolicyVersion {
		return false, "policy_version_changed"
	}
	if left.PreviewHash != right.PreviewHash {
		return false, "preview_hash_changed"
	}
	return true, ""
}

func (r *Runtime) ExecutePackageUninstall(ctx context.Context, req ExecutePackageUninstallRequest) (PackageOperationRecord, error) {
	extensionID, userID, scopeType, scopeID := req.ExtensionID, req.UserID, req.ScopeType, req.ScopeID

	if req.ConfirmationToken == "" {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationTokenInvalid, 400, ErrPackageConfirmationTokenInvalid)
	}
	claims, err := r.VerifyUninstallConfirmation(req.ConfirmationToken)
	if err != nil {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationTokenInvalid, 400, err)
	}
	if claims.ExtensionID != extensionID || claims.UserID != userID || claims.ScopeType != scopeType || claims.ScopeID != scopeID {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationBindingMismatch, 400, ErrPackageConfirmationBindingMismatch)
	}

	if claims.PolicyVersion == "" {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationPolicyVersionStale, 403, ErrPackageConfirmationPolicyVersionStale)
	}
	if claims.PolicyVersion != r.PolicyVersion() {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationPolicyVersionStale, 409, ErrPackageConfirmationPolicyVersionStale)
	}

	initialPreview, err := r.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationRecord{}, err
	}
	if !initialPreview.Installable {
		return PackageOperationRecord{}, fmt.Errorf("kernel: uninstall preflight failed")
	}

	if initialPreview.PreviewHash != claims.PreviewHash || string(initialPreview.ArtifactPolicy) != claims.ArtifactPolicy ||
		initialPreview.SecurityPolicyHash != claims.SecurityPolicyHash || initialPreview.SnapshotRequirementHash != claims.SnapshotRequirementHash ||
		claims.ArtifactID != initialPreview.ArtifactID || claims.CurrentVersionID != initialPreview.CurrentVersionID || claims.CurrentGenerationID != initialPreview.CurrentGenerationID {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationStale, 409, ErrPackageConfirmationStale)
	}

	required := requiredUninstallConfirmations(initialPreview)
	if err := validateRequiredConfirmations(claims.ConfirmedItems, required); err != nil {
		return PackageOperationRecord{}, err
	}
	if !validateConfirmedItemsConsistency(claims.ConfirmedItems, claims.Confirmations) {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, ErrPackageConfirmationItemsMismatch)
	}
	if claims.RequiredConfirmationsHash == "" || claims.RequiredConfirmationsHash != computePackageRequiredConfirmationsHash(required) {
		return PackageOperationRecord{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, fmt.Errorf("%w: requiredConfirmationsHash mismatch", ErrPackageConfirmationItemsMismatch))
	}

	releaseInProcessLock := r.acquirePackageInProcessLock(extensionID)
	defer releaseInProcessLock()

	idempotencyKey := computeSimplePackageIdempotencyKey("uninstall", extensionID, initialPreview.CurrentVersion, userID, scopeType, scopeID)
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	standardClaims := PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             string(PackageOperationTypeUninstall),
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		ArtifactPolicy:            ArtifactPolicy(claims.ArtifactPolicy),
		CurrentVersionID:          claims.CurrentVersionID,
		CurrentGenerationID:       claims.CurrentGenerationID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   claims.SnapshotRequirementHash,
		RequiredConfirmationsHash: claims.RequiredConfirmationsHash,
		DependenciesHash:          claims.DependenciesHash,
		PolicyVersion:             claims.PolicyVersion,
		UserID:                    claims.UserID,
		ScopeType:                 claims.ScopeType,
		ScopeID:                   claims.ScopeID,
		ConfirmedItems:            claims.ConfirmedItems,
		Confirmations:             claims.Confirmations,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
		Nonce:                     claims.Nonce,
		InstalledPath:             claims.InstalledPath,
		InstalledTreeHash:         claims.InstalledTreeHash,
		CurrentVersion:            claims.CurrentVersion,
	}
	claimsJSON, err := json.Marshal(standardClaims)
	if err != nil {
		return PackageOperationRecord{}, fmt.Errorf("marshal confirmation claims: %w", err)
	}
	confirmationsJSON, err := json.Marshal(claims.Confirmations)
	if err != nil {
		return PackageOperationRecord{}, fmt.Errorf("marshal confirmations: %w", err)
	}

	uninstallOp := PackageOperationRecord{OperationID: operationID, TraceID: traceID,
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: initialPreview.CurrentVersion, OperationType: "uninstall", Status: "created", CurrentStep: "created",
		ArtifactID: initialPreview.ArtifactID, ConfirmationsJSON: string(confirmationsJSON),
		ConfirmationClaimsJSON:  string(claimsJSON),
		SnapshotRequirementHash: claims.SnapshotRequirementHash,
		IdempotencyKey:          idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "uninstall", ExtensionID: extensionID, TargetVersion: initialPreview.CurrentVersion,
			ArtifactID: initialPreview.ArtifactID, ScopeType: scopeType, ScopeID: scopeID,
		}), StartedAt: now, UpdatedAt: now,
	}
	existing, created, createErr := r.container.PackageRepository.CreateOrGetOperationWithConfirmationNonce(ctx, uninstallOp, PackageConfirmationNonceBinding{
		Nonce: claims.Nonce, OperationType: uninstallOp.OperationType, ExtensionID: uninstallOp.ExtensionID, UserID: uninstallOp.UserID,
		IssuedAt: confirmationTimestamp(claims.IssuedAt), ExpiresAt: confirmationTimestamp(claims.ExpiresAt),
	})
	if createErr != nil {
		return PackageOperationRecord{}, createErr
	}
	if !created {
		switch PackageOperationStatus(existing.Status) {
		case PackageOperationCompleted:
			return existing, nil
		case PackageOperationFailed:
			return existing, fmt.Errorf("kernel: idempotent uninstall previously failed: %s", existing.ErrorDetail)
		case PackageOperationRequiresRecovery:
			return existing, fmt.Errorf("kernel: idempotent uninstall requires recovery: %s", existing.ErrorDetail)
		default:
			return existing, fmt.Errorf("kernel: uninstall operation already in progress: %s (status=%s)", existing.OperationID, existing.Status)
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, extensionID, operationID)
	if leaseErr != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return PackageOperationRecord{}, errors.Join(fmt.Errorf("kernel: extension %s has an active operation: %w", extensionID, leaseErr), persistErr)
	}
	leaseGuard := r.newPackageLeaseGuard(extensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		releaseErr := r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "start_lease_guard", "PACKAGE_OPERATION_LEASE_CONFLICT", startErr.Error(), true, PackageWriteGuard{})
		return PackageOperationRecord{}, errors.Join(fmt.Errorf("kernel: lease guard start failed: %w", startErr), releaseErr, persistErr)
	}
	defer func() {
		if stopErr := leaseGuard.Stop(context.Background()); stopErr != nil {
			if putErr := r.container.PackageRepository.PutConsistencyFinding(context.Background(), PackageConsistencyFinding{
				FindingID:         "stale-lease-" + operationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   fmt.Sprintf(`["%s"]`, operationID),
				ErrorDetail:       stopErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			}); putErr != nil {
				fmt.Printf("kernel: failed to persist stale lease finding for %s: %v\n", operationID, errors.Join(stopErr, putErr))
			}
		}
	}()
	ctx = sagaCtx
	uninstallGuard := packageWriteGuard(lease)
	preview, err := r.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recheck_preflight", "PACKAGE_UNINSTALL_PREFLIGHT_FAILED", err.Error(), true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(err, persistErr)
	}
	initialIdentity := buildUninstallPreviewIdentity(initialPreview, claims.PolicyVersion)
	leaseIdentity := buildUninstallPreviewIdentity(preview, r.PolicyVersion())
	same, driftCategory := compareUninstallPreviewIdentity(initialIdentity, leaseIdentity)
	if !preview.Installable || !same {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recheck_preflight", "PACKAGE_UNINSTALL_PREFLIGHT_CHANGED", "uninstall preflight changed after acquiring lease: "+driftCategory, true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(fmt.Errorf("kernel: uninstall preflight changed: %s", driftCategory), persistErr)
	}
	if string(preview.ArtifactPolicy) != claims.ArtifactPolicy {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recheck_preflight", PackageErrCodeConfirmationStale, "artifact policy changed after confirmation", true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(NewPackageError(PackageErrCodeConfirmationStale, 409, ErrPackageConfirmationStale), persistErr)
	}
	authorityInput, err := buildUninstallAuthorityInput(preview)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recompute_confirmation_authority", PackageErrCodeConfirmationStale, err.Error(), true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(err, persistErr)
	}

	uninstallEvidence, err := buildPackageConfirmationAuthorityEvidence(operationID, standardClaims, authorityInput)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "validate_confirmation_authority", PackageErrCodeConfirmationStale, err.Error(), true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(err, persistErr)
	}

	if evidenceErr := r.persistPackageConfirmationAuthorityEvidence(ctx, uninstallEvidence, uninstallGuard); evidenceErr != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "persist_authority_evidence", "PACKAGE_EVIDENCE_PERSIST_FAILED", evidenceErr.Error(), true, uninstallGuard)
		return PackageOperationRecord{}, errors.Join(fmt.Errorf("kernel: persist uninstall authority evidence: %w", evidenceErr), persistErr)
	}
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID, ExtensionID: extensionID, ArtifactID: initialPreview.ArtifactID}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "validate_uninstall_preflight", 1, uninstallGuard); err != nil {
		return op, err
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return op, err
	}
	currentPointer, err := r.container.PackageGenerationStore.ReadCurrent(extensionID)
	if err != nil || currentPointer.GenerationID != preview.GenerationID || currentPointer.TreeHash != preview.InstalledHash {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "bind_current_pointer", "PACKAGE_RECOVERY_REQUIRED", fmt.Sprint(err), false, uninstallGuard)
		return op, errors.Join(fmt.Errorf("kernel: uninstall current pointer binding failed"), err, persistErr)
	}
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, op.OperationID, currentPointer.GenerationID, "", packageGenerationJSON(currentPointer), uninstallGuard); err != nil {
		return op, err
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "renew_lease", "PACKAGE_LEASE_LOST", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	quarantinedCurrent, err := r.container.PackageGenerationStore.QuarantineCurrent(extensionID, currentPointer.GenerationID, op.OperationID)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "quarantine_current_pointer", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	quarantinePath := ""
	quarantinePath, err = r.container.PackageGenerationStore.QuarantineGeneration(ctx, currentPointer)
	if err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	snapshotJSON, snapshotHash, expectedGenerationID, snErr := captureInstallationSnapshot(installation, preview)
	if snErr != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, fmt.Errorf("capture installation snapshot: %w", snErr))
	}
	qm := PackageQuarantineMetadata{
		QuarantineID:             "quarantine-" + op.OperationID,
		OperationID:              op.OperationID,
		ExtensionID:              extensionID,
		GenerationQuarantinePath: quarantinePath,
		CurrentQuarantinePath:    quarantinedCurrent.Path,
		OriginalGenerationPath:   preview.InstalledPath,
		OriginalCurrentPath:      filepath.Join(r.container.ExtRoot, "installations", safeDirectoryName(extensionID), "current.json"),
		TreeHash:                 preview.InstalledHash,
		ArtifactID:               preview.ArtifactID,
		State:                    "active",
		FencingToken:             uninstallGuard.FencingToken,
		SnapshotJSON:             snapshotJSON,
		SnapshotHash:             snapshotHash,
		ExpectedGenerationID:     expectedGenerationID,
		ExpectedVersionID:        preview.CurrentVersionID,
	}
	if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, qm, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "persist_quarantine_metadata", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "move_to_quarantine", 2, currentPointer, PackageGenerationCurrent{}, packageJSON(map[string]string{"originalPath": preview.InstalledPath, "quarantinePath": quarantinePath, "currentQuarantinePath": quarantinedCurrent.Path, "treeHash": preview.InstalledHash}), uninstallGuard); err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, "", uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "clear_artifact_installation_path", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, uninstallGuard); err != nil {
			return err
		}
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		grants, err := r.container.PermissionRepository.ListGrants(txCtx, installation.ExtensionID)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			if err := r.container.PermissionRepository.DeleteGrant(txCtx, installation.ExtensionID, grant.PermissionName); err != nil {
				return err
			}
		}
		if err := r.container.ScopeRepository.DeleteBindings(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		resources, err := r.container.ResourceRepository.ListResources(txCtx, installation.ExtensionID)
		if err != nil {
			return err
		}
		for _, resource := range resources {
			if err := r.container.ResourceRepository.DeleteResource(txCtx, resource.ResourceID); err != nil {
				return err
			}
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.InstallationRepository.DeleteInstallation(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		for _, definition := range definitions {
			if definition.ID == installation.ExtensionID {
				if err := r.container.DefinitionRepository.DeleteExtension(txCtx, definition.ID, definition.Version); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "cleanup_kernel_repositories", 3, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "cleanup_kernel_repositories", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.executeRemoveArtifactStep(ctx, op, preview, ArtifactPolicy(claims.ArtifactPolicy), uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", StepRemoveArtifact, "PACKAGE_UNINSTALL_ARTIFACT_REMOVAL_FAILED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if quarantinePath != "" {
		if err := os.RemoveAll(quarantinePath); err != nil {
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "remove_files", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
			return op, errors.Join(err, persistErr)
		}
	}
	if err := os.RemoveAll(quarantinedCurrent.Path); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "remove_current_quarantine", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "renew_lease", "PACKAGE_LEASE_LOST", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if initQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		initQM.State = "finalizing"
		if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, initQM, uninstallGuard); err != nil {
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "finalize_quarantine", PackageErrCodeQuarantineStatePersistFailed, err.Error(), false, uninstallGuard)
			return op, errors.Join(err, persistErr)
		}
	} else {
		kind := RepositoryErrorKindOf(qmErr)
		code := PackageErrCodeQuarantineMetadataUnavailable
		if kind == RepositoryErrorNotFound {
			code = PackageErrCodeQuarantineMetadataMissing
		}
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "finalize_quarantine", code, qmErr.Error(), false, uninstallGuard)
		return op, errors.Join(qmErr, persistErr)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "finalize_quarantine", 4, uninstallGuard); err != nil {
		return op, err
	}
	finalizeQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID)
	if qmErr != nil {
		kind := RepositoryErrorKindOf(qmErr)
		if kind == RepositoryErrorNotFound {
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "finalize_quarantine", PackageErrCodeQuarantineMetadataMissing, qmErr.Error(), false, uninstallGuard)
			return op, errors.Join(qmErr, persistErr)
		}
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "finalize_quarantine", PackageErrCodeQuarantineMetadataUnavailable, qmErr.Error(), false, uninstallGuard)
		return op, errors.Join(qmErr, persistErr)
	}
	finalizeQM.State = "finalized"
	if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, finalizeQM, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "finalize_quarantine", PackageErrCodeQuarantineStatePersistFailed, err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.deactivatePackageVersionAfterUninstall(ctx, extensionID, initialPreview.CurrentVersion, op.OperationID, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "deactivate_version", "PACKAGE_VERSION_HISTORY_CORRUPTED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.container.PackageRepository.ReleaseQuarantineMetadata(ctx, finalizeQM.QuarantineID, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "release_quarantine", PackageErrCodeQuarantineReleaseFailed, err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.FinalizePackageOperation(ctx, op.OperationID, extensionID, leaseGuard, uninstallGuard); err != nil {
		return op, err
	}
	op.Status = "completed"
	op.CurrentStep = "completed"
	return op, nil
}

func samePackageUninstallPreview(left, right PackageUninstallPreviewResult) bool {
	return left.ExtensionID == right.ExtensionID && left.CurrentVersion == right.CurrentVersion &&
		left.Generation == right.Generation && left.Enabled == right.Enabled &&
		left.InstalledPath == right.InstalledPath && left.InstalledHash == right.InstalledHash &&
		left.ArtifactID == right.ArtifactID && left.GenerationID == right.GenerationID && left.OperationID == right.OperationID && strings.Join(left.Dependents, "\x00") == strings.Join(right.Dependents, "\x00") &&
		left.ArtifactPolicy == right.ArtifactPolicy && left.PreviewHash == right.PreviewHash &&
		left.CurrentVersionID == right.CurrentVersionID && left.CurrentGenerationID == right.CurrentGenerationID
}

func (r *Runtime) failPackageUninstallAfterGenerationQuarantine(ctx context.Context, op PackageOperationRecord, quarantinedCurrent PackageQuarantinedCurrent, current PackageGenerationCurrent, preview PackageUninstallPreviewResult, guard PackageWriteGuard, cause error) error {
	if quarantinedCurrent.Path == "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	var qmPersistFailed bool
	if existingQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		existingQM.State = "restoring"
		if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, existingQM, guard); err != nil {
			qmPersistFailed = true
		}
	} else {
		kind := RepositoryErrorKindOf(qmErr)
		if kind != RepositoryErrorNotFound {
			qmPersistFailed = true
		}
	}
	restoreErr := r.container.PackageGenerationStore.RestoreQuarantinedGeneration(ctx, current)
	if restoreErr == nil {
		restoreErr = r.container.PackageGenerationStore.RestoreQuarantinedCurrent(quarantinedCurrent)
	}
	if restoreErr == nil {
		restoreErr = r.container.PackageGenerationStore.VerifyGeneration(ctx, current)
	}
	if restoreErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore quarantined installation: %w", restoreErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_quarantine", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if pathErr := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, preview.InstalledPath, guard); pathErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installed path: %w", pathErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_path", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if _, refErr := r.container.PackageRepository.AcquireArtifactReference(ctx, preview.ArtifactID, ArtifactReferenceInstallation, op.ExtensionID, time.Time{}); refErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installation reference: %w", refErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_reference", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if restoredQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		restoredQM.State = "restored"
		if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, restoredQM, guard); err != nil {
			qmPersistFailed = true
		}
	} else {
		kind := RepositoryErrorKindOf(qmErr)
		if kind != RepositoryErrorNotFound {
			qmPersistFailed = true
		}
	}
	if qmPersistFailed {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "persist_quarantine_state", PackageErrCodeQuarantineStatePersistFailed, "quarantine metadata state persistence failed during compensation", false, guard)
		return errors.Join(cause, persistErr)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) failPackageUninstallAfterQuarantine(ctx context.Context, op PackageOperationRecord, quarantinePath string, preview PackageUninstallPreviewResult, guard PackageWriteGuard, cause error) error {
	if quarantinePath == "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	var qmPersistFailed bool
	if existingQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		existingQM.State = "restoring"
		if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, existingQM, guard); err != nil {
			qmPersistFailed = true
		}
	} else {
		kind := RepositoryErrorKindOf(qmErr)
		if kind != RepositoryErrorNotFound {
			qmPersistFailed = true
		}
	}
	restoreErr := os.Rename(quarantinePath, preview.InstalledPath)
	if restoreErr == nil {
		actualHash := package_security.ComputeDirHash(preview.InstalledPath, r.container.PackageSecurity.GetHasher())
		if actualHash == "" || actualHash != preview.InstalledHash {
			restoreErr = errors.New("kernel: restored installation tree hash mismatch")
		}
	}
	if restoreErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore quarantined installation: %w", restoreErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_quarantine", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if pathErr := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, preview.InstalledPath, guard); pathErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installed path: %w", pathErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_path", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if _, refErr := r.container.PackageRepository.AcquireArtifactReference(ctx, preview.ArtifactID, ArtifactReferenceInstallation, op.ExtensionID, time.Time{}); refErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installation reference: %w", refErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_reference", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if restoredQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		restoredQM.State = "restored"
		if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, restoredQM, guard); err != nil {
			qmPersistFailed = true
		}
	} else {
		kind := RepositoryErrorKindOf(qmErr)
		if kind != RepositoryErrorNotFound {
			qmPersistFailed = true
		}
	}
	if qmPersistFailed {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "persist_quarantine_state", PackageErrCodeQuarantineStatePersistFailed, "quarantine metadata state persistence failed during compensation", false, guard)
		return errors.Join(cause, persistErr)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) completePackageStepWithResult(ctx context.Context, operationID, name string, order int, result string, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name,
		StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: result, StartedAt: now, CompletedAt: now,
	}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
}

func validatePackageOwner(installation domain.ExtensionInstallation, userID, scopeType, scopeID string) error {
	owner, _ := installation.Metadata["ownerUserId"].(string)
	storedScopeType, _ := installation.Metadata["scopeType"].(string)
	storedScopeID, _ := installation.Metadata["scopeId"].(string)
	if owner == "" || owner != userID || storedScopeType != scopeType || storedScopeID != scopeID {
		return fmt.Errorf("kernel: package scope mismatch")
	}
	return nil
}

func (r *Runtime) beginSimplePackageOperation(ctx context.Context, userID, scopeType, scopeID, extensionID, version, operationType, artifactID string) (PackageOperationRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{OperationID: "package-operation-" + uuid.NewString(), TraceID: "package-trace-" + uuid.NewString(),
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: version, OperationType: operationType, Status: "created", CurrentStep: "created",
		ArtifactID: artifactID, ConfirmationsJSON: "{}",
		StartedAt: now, UpdatedAt: now}
	return op, r.container.PackageRepository.CreateOperation(ctx, op)
}

func (r *Runtime) restoreQuarantinedGeneration(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	if qm.GenerationQuarantinePath == "" || qm.OriginalGenerationPath == "" {
		return fmt.Errorf("kernel: quarantine metadata missing generation paths")
	}
	if _, err := os.Stat(qm.OriginalGenerationPath); err == nil {
		actualHash, hashErr := computeGenerationTreeHash(ctx, qm.OriginalGenerationPath)
		if hashErr != nil {
			return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: failed to compute existing generation hash: %w", hashErr))
		}
		if equalTreeHash(actualHash, qm.TreeHash) {
			return nil
		}
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: generation already exists at original path with different hash"))
	}
	if _, err := os.Stat(qm.GenerationQuarantinePath); err != nil {
		if os.IsNotExist(err) {
			return NewRepositoryError(RepositoryErrorNotFound, fmt.Errorf("kernel: generation quarantine path does not exist"))
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: cannot stat generation quarantine path: %w", err))
	}
	if err := copyDirContents(qm.GenerationQuarantinePath, qm.OriginalGenerationPath); err != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: failed to restore generation: %w", err))
	}
	restoredHash, hashErr := computeGenerationTreeHash(ctx, qm.OriginalGenerationPath)
	if hashErr != nil {
		os.RemoveAll(qm.OriginalGenerationPath)
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: failed to compute restored generation hash: %w", hashErr))
	}
	if !equalTreeHash(restoredHash, qm.TreeHash) {
		os.RemoveAll(qm.OriginalGenerationPath)
		return NewRepositoryError(RepositoryErrorCorrupt, fmt.Errorf("kernel: restored generation hash mismatch: expected %s got %s", qm.TreeHash, restoredHash))
	}
	return nil
}

func (r *Runtime) restoreQuarantinedCurrent(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	if qm.CurrentQuarantinePath == "" || qm.OriginalCurrentPath == "" {
		return fmt.Errorf("kernel: quarantine metadata missing current paths")
	}
	if _, err := os.Stat(qm.OriginalCurrentPath); err == nil {
		return nil
	}
	currentQuarantineFilePath := qm.CurrentQuarantinePath
	if fi, statErr := os.Stat(currentQuarantineFilePath); statErr == nil && fi.IsDir() {
		currentQuarantineFilePath = filepath.Join(currentQuarantineFilePath, "current.json")
	}
	if _, err := os.Stat(currentQuarantineFilePath); err != nil {
		if os.IsNotExist(err) {
			return NewRepositoryError(RepositoryErrorNotFound, fmt.Errorf("kernel: current quarantine path does not exist"))
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: cannot stat current quarantine path: %w", err))
	}
	if err := copyFile(currentQuarantineFilePath, qm.OriginalCurrentPath); err != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: failed to restore current.json: %w", err))
	}
	return nil
}

func (r *Runtime) restoreQuarantinedInstallation(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err == nil {
		if installation.InstalledVersion.String() == operation.TargetVersion {
			if genID, ok := installation.Metadata["generationId"]; ok && genID != "" {
				return nil
			}
		}
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation already exists with different state"))
	}
	if !errors.Is(err, domain.ErrInvalidExtensionID) {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: cannot check installation state: %w", err))
	}
	artifact, artErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if artErr != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: artifact unavailable for installation restore: %w", artErr))
	}
	if artifact.InstalledPath == "" || artifact.InstalledPath != qm.OriginalGenerationPath {
		return fmt.Errorf("kernel: artifact installed path does not match quarantine metadata")
	}
	if !qm.snapshotVerified() {
		return NewRepositoryError(RepositoryErrorCorrupt, fmt.Errorf("kernel: installation snapshot hash mismatch"))
	}
	var snapshot packageInstallationSnapshot
	if err := json.Unmarshal([]byte(qm.SnapshotJSON), &snapshot); err != nil {
		return NewRepositoryError(RepositoryErrorCorrupt, fmt.Errorf("kernel: snapshot corrupt: %w", err))
	}
	installedVersion, parseErr := domain.ParseVersion(snapshot.InstalledVersion)
	if parseErr != nil {
		return NewRepositoryError(RepositoryErrorCorrupt, fmt.Errorf("kernel: snapshot installed version parse failed: %w", parseErr))
	}
	restoredInstallation := domain.ExtensionInstallation{
		InstallationID:    operation.ExtensionID,
		ExtensionID:       domain.ExtensionID(operation.ExtensionID),
		InstalledVersion:  installedVersion,
		PackageID:         snapshot.PackageID,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		Generation:        snapshot.Generation,
		Metadata:          snapshot.Metadata,
	}
	if err := r.container.InstallationRepository.PutInstallation(ctx, restoredInstallation); err != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: restore installation failed: %w", err))
	}
	verifiedInst, verifyErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if verifyErr != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: verify restored installation failed: %w", verifyErr))
	}
	if genID, ok := verifiedInst.Metadata["generationId"]; !ok || genID != qm.ExpectedGenerationID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: restored installation generation_id mismatch"))
	}
	return nil
}

func (r *Runtime) restoreVersionStateToCurrent(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	targetVersionID := qm.ExpectedVersionID
	if targetVersionID == "" {
		return nil
	}
	existing, err := r.container.PackageRepository.GetPackageVersionByID(ctx, operation.ExtensionID, targetVersionID)
	if err != nil {
		kind := RepositoryErrorKindOf(err)
		if kind == RepositoryErrorNotFound {
			return nil
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: version lookup by id failed: %w", err))
	}
	expectedState := string(PackageVersionStateCurrent)
	if existing.VersionState == expectedState && existing.VersionID == targetVersionID {
		return nil
	}
	if existing.VersionState == string(PackageVersionStateRemoved) || existing.VersionState == string(PackageVersionStatePending) {
		db := r.container.PackageRepository.DB()
		if db == nil {
			return fmt.Errorf("kernel: package version database unavailable")
		}
		result, err := db.ExecContext(ctx, `UPDATE package_versions SET version_state=? WHERE extension_id=? AND version_id=? AND version_state IN (?, ?)`,
			expectedState, operation.ExtensionID, targetVersionID, string(PackageVersionStateRemoved), string(PackageVersionStatePending))
		if err != nil {
			return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: restore version state update failed: %w", err))
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: restore version state rows affected failed: %w", err))
		}
		if affected != 1 {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: version state restore affected %d rows, expected 1", affected))
		}
		return nil
	}
	if existing.VersionState != expectedState {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: unexpected version state: %s", existing.VersionState))
	}
	return nil
}

func (r *Runtime) restoreArtifactInstalledPath(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		kind := RepositoryErrorKindOf(err)
		if kind == RepositoryErrorNotFound {
			return fmt.Errorf("kernel: artifact not found for path restore")
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: artifact unavailable for path restore: %w", err))
	}
	if artifact.InstalledPath == "" {
		if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, operation.ArtifactID, qm.OriginalGenerationPath, guard); err != nil {
			return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: set artifact installed path failed: %w", err))
		}
		return nil
	}
	if filepath.Clean(artifact.InstalledPath) == filepath.Clean(qm.OriginalGenerationPath) {
		return nil
	}
	return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: artifact installed path conflict: existing %s, expected %s", artifact.InstalledPath, qm.OriginalGenerationPath))
}

func (r *Runtime) restoreArtifactInstallationReference(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	if err := r.container.PackageRepository.EnsureArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID, time.Time{}); err != nil {
		kind := RepositoryErrorKindOf(err)
		if kind == RepositoryErrorConflict {
			return NewPackageErrorWithRecovery(PackageErrCodeArtifactReferenceConflict, 409, false, true, "Inspect artifact reference", fmt.Errorf("kernel: artifact reference ensure conflict: %w", err))
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: artifact reference ensure failed: %w", err))
	}
	return nil
}

func (r *Runtime) verifyUninstallRestoredState(ctx context.Context, operation PackageOperationRecord, qm PackageQuarantineMetadata, guard PackageWriteGuard) error {
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: installation missing after restore: %w", err))
	}
	if installation.InstallationState != domain.InstallationStateInstalled {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation state is %s after restore, expected installed", installation.InstallationState))
	}
	if installation.ExtensionID != domain.ExtensionID(operation.ExtensionID) {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation extension_id mismatch after restore: %s != %s", installation.ExtensionID, operation.ExtensionID))
	}
	if genID, ok := installation.Metadata["generationId"]; !ok || genID == "" {
		return errors.New("kernel: installation generation id missing after restore")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: artifact unavailable after restore: %w", err))
	}
	if artifact.ArtifactID != operation.ArtifactID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: artifact identity mismatch after restore: %s != %s", artifact.ArtifactID, operation.ArtifactID))
	}
	if artifact.ExtensionID != operation.ExtensionID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: artifact extension_id mismatch after restore: %s != %s", artifact.ExtensionID, operation.ExtensionID))
	}
	if filepath.Clean(artifact.InstalledPath) != filepath.Clean(qm.OriginalGenerationPath) {
		return errors.New("kernel: artifact installed path mismatch after restore")
	}
	if artifact.RetentionState == "deleted" || artifact.DeletedAt != "" {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: artifact is in deleted state after restore: retention_state=%s deleted_at=%s", artifact.RetentionState, artifact.DeletedAt))
	}
	version, err := r.container.PackageRepository.GetPackageVersion(ctx, operation.ExtensionID, operation.TargetVersion)
	if err != nil {
		kind := RepositoryErrorKindOf(err)
		if kind == RepositoryErrorNotFound {
			return errors.New("kernel: version state missing after restore")
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: version state unavailable after restore: %w", err))
	}
	if version.VersionState != string(PackageVersionStateCurrent) {
		return errors.New("kernel: version state is not current after restore")
	}
	if qm.ExpectedVersionID != "" && version.VersionID != qm.ExpectedVersionID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: restored version_id mismatch: expected %s, got %s", qm.ExpectedVersionID, version.VersionID))
	}
	if qm.ExpectedGenerationID != "" && version.GenerationID != qm.ExpectedGenerationID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: restored generation_id mismatch: expected %s, got %s", qm.ExpectedGenerationID, version.GenerationID))
	}
	installRef, refErr := r.container.PackageRepository.FindArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID)
	if refErr != nil {
		kind := RepositoryErrorKindOf(refErr)
		if kind == RepositoryErrorNotFound {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation artifact reference missing after restore"))
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: artifact reference verification failed: %w", refErr))
	}
	if installRef.ArtifactID != operation.ArtifactID || installRef.ReferenceType != ArtifactReferenceInstallation || installRef.ReferenceOwnerID != operation.ExtensionID {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation artifact reference identity mismatch after restore"))
	}
	if installRef.ReleasedAt != "" {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: installation artifact reference is released after restore"))
	}
	if r.container.PackageGenerationStore != nil {
		current, curErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
		if curErr != nil {
			return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: generation current pointer unavailable after restore: %w", curErr))
		}
		if qm.ExpectedGenerationID != "" && current.GenerationID != qm.ExpectedGenerationID {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: generation current pointer generation_id mismatch after restore: expected %s, got %s", qm.ExpectedGenerationID, current.GenerationID))
		}
		if current.ExtensionID != operation.ExtensionID {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: generation current pointer extension_id mismatch after restore: expected %s, got %s", operation.ExtensionID, current.ExtensionID))
		}
		if current.ArtifactID != operation.ArtifactID {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: generation current pointer artifact_id mismatch after restore: expected %s, got %s", operation.ArtifactID, current.ArtifactID))
		}
		if current.TreeHash == "" {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: generation current pointer tree_hash is empty after restore"))
		}
		if installErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, current); installErr != nil {
			return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: restored generation verification failed: %w", installErr))
		}
	}
	if !r.filePackageTreeMatches(ctx, qm.OriginalGenerationPath, qm.TreeHash) {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: restored installed tree hash does not match quarantine"))
	}
	qmAfter, err := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if err != nil {
		kind := RepositoryErrorKindOf(err)
		if kind == RepositoryErrorNotFound {
			return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataMissing, 409, false, true, "Inspect persisted quarantine metadata", fmt.Errorf("quarantine metadata missing after restore completion"))
		}
		return NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("kernel: quarantine metadata unavailable after restore: %w", err))
	}
	if qmAfter.State != "active" && qmAfter.State != "restored" {
		return NewRepositoryError(RepositoryErrorConflict, fmt.Errorf("kernel: quarantine metadata in unexpected state after restore: %s", qmAfter.State))
	}
	return nil
}

func (r *Runtime) filePackageTreeMatches(ctx context.Context, path, expectedHash string) bool {
	if path == "" || expectedHash == "" {
		return false
	}
	actualHash, hashErr := computeGenerationTreeHash(ctx, path)
	if hashErr != nil {
		return false
	}
	return actualHash != "" && equalTreeHash(actualHash, expectedHash)
}

func (r *Runtime) completeSimplePackageStep(ctx context.Context, operationID, name string, order int, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name,
		StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: "{}", StartedAt: now, CompletedAt: now,
	}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
}

func computeArtifactEvidenceHash(artifactID string, refCount int, exists bool) string {
	canonical := fmt.Sprintf("artifact:%s:refs=%d:exists=%v", artifactID, refCount, exists)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

func (r *Runtime) executeRemoveArtifactStep(ctx context.Context, op PackageOperationRecord, preview PackageUninstallPreviewResult, policy ArtifactPolicy, guard PackageWriteGuard) error {
	artifactID := op.ArtifactID
	if artifactID == "" {
		return nil
	}
	repo := r.container.PackageRepository

	evidenceBefore := computeArtifactEvidenceHash(artifactID, -1, true)

	switch policy {
	case ArtifactPolicyRetainArtifact:
		if err := repo.ReleaseArtifactReference(ctx, artifactID, ArtifactReferenceInstallation, op.ExtensionID); err != nil && !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
			return fmt.Errorf("failed to release installation reference for retain: %w", err)
		}
		art, getErr := repo.GetArtifact(ctx, artifactID)
		if getErr != nil || art.ArtifactID == "" {
			return NewPackageError(PackageErrCodeUninstallArtifactMissing, 404, fmt.Errorf("retain policy: artifact %s not found after releasing installation ref", artifactID))
		}
		refCount, countErr := repo.CountActiveArtifactReferences(ctx, artifactID)
		if countErr != nil {
			return fmt.Errorf("retain policy: failed to count remaining refs: %w", countErr)
		}
		return r.recordRemoveArtifactStep(ctx, op, policy, false, int(refCount), time.Time{}, evidenceBefore, evidenceBefore, guard)

	case ArtifactPolicyRetainForRollback:
		if err := repo.ReleaseArtifactReference(ctx, artifactID, ArtifactReferenceInstallation, op.ExtensionID); err != nil && !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
			return fmt.Errorf("failed to release installation reference for retain_rollback: %w", err)
		}
		refCount, countErr := repo.CountActiveArtifactReferences(ctx, artifactID)
		if countErr != nil {
			return fmt.Errorf("retain_rollback policy: failed to count remaining refs: %w", countErr)
		}
		return r.recordRemoveArtifactStep(ctx, op, policy, false, int(refCount), time.Time{}, evidenceBefore, evidenceBefore, guard)

	case ArtifactPolicyRetainForExport:
		if err := repo.ReleaseArtifactReference(ctx, artifactID, ArtifactReferenceInstallation, op.ExtensionID); err != nil && !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
			return fmt.Errorf("failed to release installation reference for retain_export: %w", err)
		}
		refCount, countErr := repo.CountActiveArtifactReferences(ctx, artifactID)
		if countErr != nil {
			return fmt.Errorf("retain_export policy: failed to count remaining refs: %w", countErr)
		}
		return r.recordRemoveArtifactStep(ctx, op, policy, false, int(refCount), time.Time{}, evidenceBefore, evidenceBefore, guard)

	case ArtifactPolicyDeleteArtifact:
		if err := repo.ReleaseArtifactReference(ctx, artifactID, ArtifactReferenceInstallation, op.ExtensionID); err != nil && !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
			return fmt.Errorf("failed to release installation reference for delete: %w", err)
		}
		refCount, countErr := repo.CountActiveArtifactReferences(ctx, artifactID)
		if countErr != nil {
			kind := RepositoryErrorKindOf(countErr)
			if kind == RepositoryErrorUnavailable {
				return NewPackageError(PackageErrCodeQuarantineMetadataUnavailable, 503, fmt.Errorf("repository unavailable during delete policy execution: %w", countErr))
			}
			return fmt.Errorf("delete policy: failed to count refs: %w", countErr)
		}
		if refCount != 0 {
			return NewPackageError(PackageErrCodeUninstallArtifactRemovalFailed, 409, fmt.Errorf("delete policy: artifact still has %d active references", refCount))
		}
		if _, markErr := repo.MarkArtifactGCPending(ctx, artifactID); markErr != nil {
			return fmt.Errorf("delete policy: failed to mark artifact gc-pending: %w", markErr)
		}
		art, getErr := repo.GetArtifact(ctx, artifactID)
		if getErr != nil {
			return fmt.Errorf("delete policy: failed to get artifact for removal: %w", getErr)
		}
		removeErr := r.container.PackageArtifactStore.RemoveArchive(art)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return NewPackageError(PackageErrCodeUninstallArtifactRemovalFailed, 500, fmt.Errorf("delete policy: failed to remove archive: %w", removeErr))
		}
		if completeErr := repo.CompleteArtifactGC(ctx, artifactID, removeErr); completeErr != nil {
			return fmt.Errorf("delete policy: failed to complete artifact GC: %w", completeErr)
		}
		deletedArt, getDeletedErr := repo.GetArtifact(ctx, artifactID)
		refCountAfter, countAfterErr := repo.CountActiveArtifactReferences(ctx, artifactID)
		if getDeletedErr == nil && deletedArt.ArtifactID != "" && (countAfterErr != nil || refCountAfter != 0) {
			return NewPackageError(PackageErrCodeUninstallArtifactRemovalFailed, 500, fmt.Errorf("delete policy: artifact %s was not successfully deleted", artifactID))
		}
		evidenceAfter := computeArtifactEvidenceHash(artifactID, 0, false)
		deletedAt := time.Now().UTC()
		return r.recordRemoveArtifactStep(ctx, op, policy, true, 0, deletedAt, evidenceBefore, evidenceAfter, guard)

	default:
		return NewPackageError(PackageErrCodeUninstallArtifactRemovalFailed, 500, fmt.Errorf("unknown artifact policy: %s", policy))
	}
}

func (r *Runtime) recordRemoveArtifactStep(ctx context.Context, op PackageOperationRecord, policy ArtifactPolicy, deleted bool, remainingRefs int, deletedAt time.Time, evidenceBefore, evidenceAfter string, guard PackageWriteGuard) error {
	retained := !deleted && policy != ArtifactPolicyDeleteArtifact
	retentionState := "active"
	if deleted {
		retentionState = "deleted"
	} else if policy == ArtifactPolicyRetainForRollback || policy == ArtifactPolicyRetainForExport {
		retentionState = "retained"
	}
	result := RemoveArtifactStepResult{
		ArtifactID:         op.ArtifactID,
		ExtensionID:        op.ExtensionID,
		ArtifactPolicy:     policy,
		Deleted:            deleted,
		Retained:           retained,
		RetentionState:     retentionState,
		RemainingRefs:      remainingRefs,
		DeletedAt:          deletedAt,
		EvidenceHashBefore: evidenceBefore,
		EvidenceHashAfter:  evidenceAfter,
	}
	result.EvidenceHash = computeArtifactStepEvidenceHash(result)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal RemoveArtifactStepResult: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID:      "step-remove-artifact-" + uuid.NewString(),
		OperationID: op.OperationID,
		StepName:    StepRemoveArtifact,
		StepOrder:   31,
		Status:      "completed",
		ResultJSON:  string(resultJSON),
		StartedAt:   now,
		CompletedAt: now,
	}, guard)
}

func computeArtifactStepEvidenceHash(result RemoveArtifactStepResult) string {
	canonical := fmt.Sprintf("artifact:%s:ext:%s:policy:%s:deleted:%v:retained:%v:state:%s:refs:%d:deletedAt:%v:before:%s:after:%s",
		result.ArtifactID, result.ExtensionID, string(result.ArtifactPolicy),
		result.Deleted, result.Retained, result.RetentionState, result.RemainingRefs,
		result.DeletedAt.Unix(), result.EvidenceHashBefore, result.EvidenceHashAfter)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}
