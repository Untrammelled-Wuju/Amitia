package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type PackageUpdateDiff struct {
	DefinitionChanged    bool     `json:"definitionChanged"`
	ModulesAdded         []string `json:"modulesAdded"`
	ModulesRemoved       []string `json:"modulesRemoved"`
	ModulesChanged       []string `json:"modulesChanged"`
	ContributionsAdded   []string `json:"contributionsAdded"`
	ContributionsRemoved []string `json:"contributionsRemoved"`
	ContributionsChanged []string `json:"contributionsChanged"`
	PermissionsAdded     []string `json:"permissionsAdded"`
	PermissionsRemoved   []string `json:"permissionsRemoved"`
	ScopesAdded          []string `json:"scopesAdded"`
	ScopesRemoved        []string `json:"scopesRemoved"`
	ResourcesAdded       []string `json:"resourcesAdded"`
	ResourcesRemoved     []string `json:"resourcesRemoved"`
	ResourcesChanged     []string `json:"resourcesChanged"`
	MigrationsAdded      []string `json:"migrationsAdded"`
	MigrationsRemoved    []string `json:"migrationsRemoved"`
	FilesAdded           []string `json:"filesAdded"`
	FilesRemoved         []string `json:"filesRemoved"`
	FilesChanged         []string `json:"filesChanged"`
}

type confirmedPackageUpdate struct {
	session  PackagePreviewSession
	preview  InstallPreview
	claims   packageConfirmationClaims
	artifact PackageArtifact
	pkg      *amitiax.Package
}

func (r *Runtime) ExecutePackageUpdate(ctx context.Context, request PackageInstallRequest) (KernelInstallResult, error) {
	confirmed, err := r.validateConfirmedPackageUpdate(ctx, request)
	if err != nil {
		return KernelInstallResult{}, err
	}
	if request.IdempotencyKey == "" {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeIdempotencyKeyRequired, 400, ErrPackageIdempotencyKeyRequired)
	}
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(confirmed.session.ExtensionID))
	if err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), "package-operation-"+uuid.NewString(), "failed", "check_installed", "PACKAGE_NOT_INSTALLED", fmt.Sprintf("extension %s is not installed", confirmed.session.ExtensionID), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("PACKAGE_NOT_INSTALLED: extension %s is not installed", confirmed.session.ExtensionID)
	}
	if err := validatePackageOwner(current, request.UserID, request.ScopeType, request.ScopeID); err != nil {
		return KernelInstallResult{}, err
	}
	idempotencyKey := request.IdempotencyKey
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	confirmationsJSON, _ := json.Marshal(confirmed.claims.Confirmations)
	claimsJSON, _ := json.Marshal(PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             string(PackageOperationTypeUpdate),
		ExtensionID:               confirmed.session.ExtensionID,
		ArtifactID:                confirmed.artifact.ArtifactID,
		PolicyVersion:             confirmed.session.PolicyVersion,
		SecurityPolicyHash:        computeSecurityPolicyHash(),
		DeveloperSessionID:        confirmed.preview.DeveloperSessionID,
		MigrationPlanHash:         confirmed.preview.MigrationPlanHash,
		PreviewSessionID:          confirmed.session.SessionID,
		PreviewHash:               confirmed.claims.PreviewHash,
		SnapshotRequirementHash:   confirmed.claims.SnapshotRequirementHash,
		RequiredConfirmationsHash: confirmed.claims.RequiredConfirmationsHash,
		DependenciesHash:          confirmed.claims.DependenciesHash,
		UserID:                    request.UserID,
		ScopeType:                 request.ScopeType,
		ScopeID:                   request.ScopeID,
		ConfirmedItems:            confirmedItemsFromMap(confirmed.claims.Confirmations),
		Confirmations:             confirmed.claims.Confirmations,
		IssuedAt:                  confirmed.claims.IssuedAt,
		ExpiresAt:                 confirmed.claims.ExpiresAt,
		Nonce:                     confirmed.claims.Nonce,
		ArchiveHash:               confirmed.session.ArchiveHash,
		ManifestHash:              confirmed.session.ManifestHash,
		ContentTreeHash:           confirmed.session.ContentTreeHash,
		SourceVersionID:           current.InstalledVersion.String(),
		TargetVersion:             confirmed.session.Version,
		TargetVersionID:           confirmed.session.Version,
	})
	updateOp := PackageOperationRecord{OperationID: operationID, TraceID: traceID, UserID: request.UserID,
		ScopeType: request.ScopeType, ScopeID: request.ScopeID, ExtensionID: confirmed.session.ExtensionID,
		TargetVersion: confirmed.session.Version, FromVersion: current.InstalledVersion.String(),
		OperationType: "update", Status: "created",
		CurrentStep: "create_operation", ArtifactID: confirmed.artifact.ArtifactID,
		PreviewSessionID: confirmed.session.SessionID, ConfirmationsJSON: string(confirmationsJSON),
		ConfirmationClaimsJSON: string(claimsJSON), SnapshotRequirementHash: confirmed.claims.SnapshotRequirementHash,
		IdempotencyKey: idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "update", ExtensionID: confirmed.session.ExtensionID, TargetVersion: confirmed.session.Version,
			ArtifactID: confirmed.artifact.ArtifactID, PreviewSessionID: confirmed.session.SessionID,
			ScopeType: request.ScopeType, ScopeID: request.ScopeID,
		}), StartedAt: now, UpdatedAt: now}
	existing, created, createErr := r.container.PackageRepository.CreateOrGetOperationWithConfirmationNonce(ctx, updateOp, PackageConfirmationNonceBinding{
		Nonce: confirmed.claims.Nonce, OperationType: updateOp.OperationType, ExtensionID: updateOp.ExtensionID, UserID: updateOp.UserID,
		IssuedAt: confirmationTimestamp(confirmed.claims.IssuedAt), ExpiresAt: confirmationTimestamp(confirmed.claims.ExpiresAt),
	})
	if createErr != nil {
		return KernelInstallResult{}, createErr
	}
	if !created {
		if result, handled, handleErr := r.handleExistingPackageOperation(ctx, existing); handled {
			return result, handleErr
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, confirmed.session.ExtensionID, operationID)
	if leaseErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: extension %s has an active operation: %w", confirmed.session.ExtensionID, leaseErr)
	}
	leaseGuard := r.newPackageLeaseGuard(confirmed.session.ExtensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		_ = r.releasePackageExtensionLease(context.Background(), confirmed.session.ExtensionID, operationID)
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
	lockedSession, err := r.container.PackageRepository.GetPreview(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "lock_preview_session", "PACKAGE_PREVIEW_SESSION_LOCK_FAILED", err.Error(), true, guard)
		return KernelInstallResult{}, err
	}
	if lockedSession.Status == "consumed" {
		return r.completedPackageInstallResult(ctx, request.UserID, request.SessionID)
	}
	if lockedSession.Status != "ready" && lockedSession.Status != "awaiting_confirmation" {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "lock_preview_session", "PACKAGE_PREVIEW_SESSION_STATUS", fmt.Sprintf("status %s", lockedSession.Status), true, guard)
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session status %s", lockedSession.Status)
	}
	if request.ExpectedExtensionID == "" || request.ExpectedExtensionID != string(current.ExtensionID) {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "validate_extension_id", "PACKAGE_UPDATE_ID_MISMATCH", "expected extension id must match installed extension", true, guard)
		return KernelInstallResult{}, fmt.Errorf("PACKAGE_UPDATE_ID_MISMATCH: expected extension id must match installed extension")
	}
	if confirmed.artifact.ArtifactID == current.PackageID || confirmed.session.Version == current.InstalledVersion.String() {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "validate_target_version", "PACKAGE_UPDATE_TARGET_UNCHANGED", "update target must have a different version and artifact", true, guard)
		return KernelInstallResult{}, fmt.Errorf("PACKAGE_UPDATE_TARGET_UNCHANGED: update target must have a different version and artifact")
	}

	var lockedPreview InstallPreview
	if err := json.Unmarshal([]byte(lockedSession.PreviewResultJSON), &lockedPreview); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, "recompute_confirmation_authority", err, nil, guard)
	}

	authorityInput, err := r.buildInstallUpdateAuthorityInput(ctx, string(PackageOperationTypeUpdate), lockedSession, lockedPreview)
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, "recompute_confirmation_authority", err, nil, guard)
	}

	standardClaims := standardConfirmationClaimsFromLegacy(string(PackageOperationTypeUpdate), confirmed.claims)

	updateEvidence, err := buildPackageConfirmationAuthorityEvidence(operationID, standardClaims, authorityInput)
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, "validate_confirmation_authority", err, nil, guard)
	}

	if err := r.persistPackageConfirmationAuthorityEvidence(ctx, updateEvidence, guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, "persist_authority_evidence", err, nil, guard)
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
	currentRequirements, err := r.container.PermissionRepository.ListRequirements(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentGrants, err := r.container.PermissionRepository.ListGrants(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentScopes, err := r.container.ScopeRepository.ListBindings(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentResources, err := r.container.ResourceRepository.ListResources(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	targetDefinition, err := confirmed.pkg.Manifest.ToExtensionDefinition()
	if err != nil {
		return KernelInstallResult{}, err
	}
	bindAuthoritativePublisherTrust(&targetDefinition, confirmed.preview.TrustDecision)
	migrationPreflight, err := r.revalidatePackageMigrationPreflight(ctx, current, confirmed)
	if err != nil {
		return KernelInstallResult{}, err
	}
	diff := computePackageUpdateDiff(currentDefinition, currentModules, currentContributions, currentRequirements, currentResources, confirmed.preview.Manifest, confirmed.artifact)
	diffJSON, _ := json.Marshal(diff)
	if err := r.completePackageUpdateStep(ctx, operationID, 1, StepValidateAndDiff, string(diffJSON), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, StepValidateAndDiff, err, nil, guard)
	}
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID}
	if renewErr := leaseGuard.AssertAlive(ctx); renewErr != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(operationID, "renew_lease", renewErr, nil, guard)
	}
	rollbackPoint, err := r.createPackageRollbackPoint(ctx, op.OperationID, "active", current, &currentDefinition, currentModules, currentContributions)
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCreateRollbackPoint, err, nil, guard)
	}
	if err := r.completePackageUpdateStep(ctx, op.OperationID, 2, StepCreateRollbackPoint, packageJSON(map[string]string{"rollbackPointId": rollbackPoint.RollbackPointID}), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCreateRollbackPoint, err, nil, guard)
	}
	staging, err := r.container.PackageSecurity.ExtractFileToStaging(ctx, confirmed.artifact.ArchivePath, op.OperationID)
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "update.extract_to_staging", err, nil, guard)
	}
	defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
	if package_security.ComputeDirHash(staging.Path, r.container.PackageSecurity.GetHasher()) == "" {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "update.verify_staging_tree", fmt.Errorf("kernel: staging hash empty"), nil, guard)
	}
	targetGeneration, stableGeneration, err := r.preparePackageGeneration(ctx, op.OperationID, confirmed.artifact, staging.Path, guard.FencingToken)
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCommitTargetGeneration, err, nil, guard)
	}
	compensation := &packageUpdateCompensation{runtime: r, operationID: op.OperationID, rollbackPoint: rollbackPoint, stable: stableGeneration, target: targetGeneration}
	stableFromDB := packageGenerationFromInstallation(current)
	if stableFromDB.GenerationID == "" || stableFromDB.GenerationID != stableGeneration.GenerationID {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "update.validate_current_pointer", ErrPackageGenerationCAS, compensation, guard)
	}
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, op.OperationID, stableGeneration.GenerationID, targetGeneration.Current.GenerationID, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "update.persist_generation_evidence", err, compensation, guard)
	}
	commitGenResult := CommitGenerationStepResult{Path: targetGeneration.GenerationPath, TreeHash: targetGeneration.Current.TreeHash, StableGeneration: stableGeneration.GenerationID, TargetGeneration: targetGeneration.Current.GenerationID, ArtifactHash: confirmed.artifact.ArtifactHash}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, StepCommitTargetGeneration, 3, stableGeneration, targetGeneration.Current, packageJSON(commitGenResult), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCommitTargetGeneration, err, compensation, guard)
	}
	migrationExecution, err := r.executePackageUpdateMigrations(ctx, op.OperationID, migrationPreflight, rollbackPoint, confirmed.pkg, staging.Path)
	compensation.migration = migrationExecution
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepExecuteMigrations, err, compensation, guard)
	}
	migrationOperation := ""
	if migrationExecution != nil {
		migrationOperation = migrationExecution.request.OperationID
	}
	if err := r.completePackageUpdateStep(ctx, op.OperationID, 4, StepExecuteMigrations, packageJSON(map[string]string{"migrationOperationId": migrationOperation, "migrationPlanHash": confirmed.preview.MigrationPlanHash}), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepExecuteMigrations, err, compensation, guard)
	}
	if err := r.switchPackageGeneration(ctx, stableGeneration, targetGeneration); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepUpdateSwitchCurrentPointer, err, compensation, guard)
	}
	compensation.switched = true
	if err := r.completePackageGenerationStep(ctx, op.OperationID, StepUpdateSwitchCurrentPointer, 5, stableGeneration, targetGeneration.Current, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepUpdateSwitchCurrentPointer, err, compensation, guard)
	}
	targetRequirements := packageManifestRequirements(current.ExtensionID, confirmed.preview.Manifest.Permissions)
	targetResources := packageManifestResources(current.ExtensionID, confirmed.preview.Manifest.Resources, targetGeneration.GenerationPath)
	retainedGrants := retainPackagePermissionGrants(currentGrants, targetRequirements)
	current.InstalledVersion = targetDefinition.Version
	current.PackageID = confirmed.artifact.ArtifactID
	current.Generation++
	current.EnablementState = domain.EnablementDisabled
	current.UpdatedAt = time.Now().UTC()
	metadata := clonePackageMetadata(current.Metadata)
	metadata["artifactId"] = confirmed.artifact.ArtifactID
	metadata["archiveHash"] = confirmed.artifact.ArchiveHash
	metadata["manifestHash"] = confirmed.artifact.ManifestHash
	metadata["contentTreeHash"] = confirmed.artifact.ContentTreeHash
	metadata["artifactHash"] = confirmed.artifact.ArtifactHash
	metadata["devOnly"] = confirmed.preview.DevOnly
	current.Metadata = packageInstallationMetadata(metadata, targetGeneration.Current, targetGeneration.GenerationPath, op.OperationID)
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, guard); err != nil {
			return err
		}
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, current.ExtensionID); err != nil {
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
		for _, resource := range currentResources {
			if err := r.container.ResourceRepository.DeleteResource(txCtx, resource.ResourceID); err != nil {
				return err
			}
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.DeleteExtension(txCtx, current.ExtensionID, currentDefinition.Version); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.PutExtension(txCtx, targetDefinition); err != nil {
			return err
		}
		for _, module := range targetDefinition.Modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, module); err != nil {
				return err
			}
			for _, contribution := range module.Contributions {
				if err := r.container.ContributionRepository.PutContribution(txCtx, contribution); err != nil {
					return err
				}
			}
		}
		for _, requirement := range targetRequirements {
			if err := r.container.PermissionRepository.PutRequirement(txCtx, requirement); err != nil {
				return err
			}
		}
		for _, grant := range retainedGrants {
			if err := r.container.PermissionRepository.PutGrant(txCtx, grant); err != nil {
				return err
			}
		}
		for _, binding := range currentScopes {
			if err := r.container.ScopeRepository.PutBinding(txCtx, binding); err != nil {
				return err
			}
		}
		for _, resource := range targetResources {
			if err := r.container.ResourceRepository.PutResource(txCtx, resource); err != nil {
				return err
			}
		}
		return r.container.InstallationRepository.PutInstallation(txCtx, current)
	})
	if err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCommitUpdateState, err, compensation, guard)
	}
	compensation.repositoriesCommitted = true
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, confirmed.artifact.ArtifactID, targetGeneration.GenerationPath, guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "persist_artifact_metadata", err, compensation, guard)
	}
	commitRepoResult := CommitRepositoryResult{InstallationID: current.InstallationID, VersionID: confirmed.session.Version, ArtifactID: confirmed.artifact.ArtifactID, GenerationID: targetGeneration.Current.GenerationID}
	if err := r.completePackageUpdateStep(ctx, op.OperationID, 6, StepCommitUpdateState, packageJSON(commitRepoResult), guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, StepCommitUpdateState, err, compensation, guard)
	}
	if err := r.container.PackageRepository.ConsumePreview(ctx, confirmed.session.SessionID); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "consume_preview_session", err, compensation, guard)
	}
	if err := r.recordPackageVersionAfterOperation(ctx, op.OperationID, "update", confirmed.session.ExtensionID, confirmed.session.Version, confirmed.artifact.ArtifactID, targetGeneration.GenerationPath, targetGeneration.Current.TreeHash, confirmed.artifact.ArchiveHash, confirmed.artifact.ManifestHash, confirmed.artifact.ContentTreeHash, targetGeneration.Current.GenerationID, guard); err != nil {
		return KernelInstallResult{}, r.failPackageUpdateOperation(op.OperationID, "record_version", err, compensation, guard)
	}
	if err := r.FinalizePackageOperation(ctx, op.OperationID, confirmed.session.ExtensionID, leaseGuard, guard); err != nil {
		if compensation != nil {
			_ = compensation.run()
		}
		return KernelInstallResult{}, err
	}
	if r.container.UIHostNotifier != nil {
		r.container.UIHostNotifier.BroadcastExtensionChange("extension_updated", confirmed.session.ExtensionID, nil)
		r.container.UIHostNotifier.BroadcastExtensionChange("extension_generation_changed", confirmed.session.ExtensionID, map[string]interface{}{"generation": current.Generation})
		r.container.UIHostNotifier.BroadcastExtensionChange("extension_contributions_changed", confirmed.session.ExtensionID, nil)
	}
	return KernelInstallResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: "update", ExtensionID: confirmed.session.ExtensionID, Version: confirmed.session.Version, InstallationID: current.InstallationID, PackageHash: confirmed.artifact.ArchiveHash, ContentTreeHash: confirmed.artifact.ContentTreeHash, ArtifactPath: confirmed.artifact.ArchivePath, InstallPath: targetGeneration.GenerationPath, DefinitionHash: targetGeneration.Current.TreeHash, InstalledAt: current.UpdatedAt}, nil
}

func (r *Runtime) validateConfirmedPackageUpdate(ctx context.Context, request PackageInstallRequest) (confirmedPackageUpdate, error) {
	if r.container == nil || r.container.PackageRepository == nil || r.container.PackageArtifactStore == nil || r.container.PackageGenerationStore == nil {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: package services unavailable")
	}
	session, err := r.container.PackageRepository.GetPreview(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: preview session unavailable: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: preview session expired")
	}
	if session.Status != "ready" && session.Status != "awaiting_confirmation" && session.Status != "consumed" {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: preview session status %s", session.Status)
	}
	if session.PolicyVersion != packagePolicyVersion || session.SecurityPolicyHash != computeSecurityPolicyHash() {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: preview security policy changed")
	}
	if request.ExpectedExtensionID == "" || request.ExpectedExtensionID != session.ExtensionID {
		return confirmedPackageUpdate{}, fmt.Errorf("PACKAGE_UPDATE_ID_MISMATCH: expected extension id must match preview")
	}
	var preview InstallPreview
	if err := json.Unmarshal([]byte(session.PreviewResultJSON), &preview); err != nil || !preview.Installable {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: package preview is unavailable or not installable")
	}
	if preview.DevOnly {
		if err := r.validateUnsignedDeveloperSession(preview.DeveloperSessionID, request.UserID, preview.ExtensionID); err != nil {
			return confirmedPackageUpdate{}, fmt.Errorf("kernel: developer session no longer valid: %w", err)
		}
	}
	claims, err := verifyPackageConfirmation(request.ConfirmationToken)
	if err != nil {
		return confirmedPackageUpdate{}, err
	}
	if claims.SessionID != session.SessionID || claims.ArtifactID != session.ArtifactID || claims.ArchiveHash != session.ArchiveHash || claims.ManifestHash != session.ManifestHash || claims.ContentTreeHash != session.ContentTreeHash || claims.UserID != request.UserID || claims.ScopeType != request.ScopeType || claims.ScopeID != request.ScopeID || claims.PolicyVersion != session.PolicyVersion || claims.SecurityPolicyHash != computeSecurityPolicyHash() || claims.DeveloperSessionID != preview.DeveloperSessionID || claims.MigrationPlanHash != preview.MigrationPlanHash {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: confirmation token binding mismatch")
	}
	var required []string
	if err := json.Unmarshal([]byte(session.RequiredConfirmationsJSON), &required); err != nil {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: confirmation policy corrupt: %w", err)
	}
	for _, confirmation := range required {
		if !claims.Confirmations[confirmation] {
			return confirmedPackageUpdate{}, fmt.Errorf("kernel: confirmation required: %s", confirmation)
		}
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, session.ArtifactID)
	if err != nil {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: artifact unavailable: %w", err)
	}
	if artifact.ArchiveHash != session.ArchiveHash || artifact.ManifestHash != session.ManifestHash || artifact.ContentTreeHash != session.ContentTreeHash {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: preview artifact identity mismatch")
	}
	pkg, err := r.VerifyStoredPackage(ctx, artifact)
	if err != nil {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: artifact verification failed: %w", err)
	}
	dependencyPreview := InstallPreview{}
	r.evaluatePackageCompatibilityAndDependencies(ctx, pkg, &dependencyPreview)
	if len(dependencyPreview.Issues) > 0 {
		return confirmedPackageUpdate{}, fmt.Errorf("kernel: dependency or compatibility state changed after preview")
	}
	if claims.PreviewHash == "" || claims.SnapshotRequirementHash == "" {
		return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, ErrPackageConfirmationClaimsInvalid)
	}
	if claims.Nonce == "" || claims.IssuedAt == 0 {
		return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: nonce and issuedAt required", ErrPackageConfirmationClaimsInvalid))
	}
	if claims.RequiredConfirmationsHash == "" || claims.RequiredConfirmationsHash != computePackageRequiredConfirmationsHash(required) {
		return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, fmt.Errorf("%w: requiredConfirmationsHash mismatch", ErrPackageConfirmationItemsMismatch))
	}
	if claims.DependenciesHash == "" {
		return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: dependenciesHash required", ErrPackageConfirmationClaimsInvalid))
	}
	confirmedItems := confirmedItemsFromMap(claims.Confirmations)
	if len(confirmedItems) == 0 || !validateConfirmedItemsConsistency(confirmedItems, claims.Confirmations) {
		return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, ErrPackageConfirmationItemsMismatch)
	}
	for _, confirmation := range confirmedItems {
		if !claims.Confirmations[confirmation] {
			return confirmedPackageUpdate{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, ErrPackageConfirmationItemsMismatch)
		}
	}
	return confirmedPackageUpdate{session: session, preview: preview, claims: claims, artifact: artifact, pkg: pkg}, nil
}

type packageUpdateCompensation struct {
	runtime               *Runtime
	operationID           string
	rollbackPoint         PackageRollbackPoint
	stable                PackageGenerationCurrent
	target                PackagePreparedGeneration
	switched              bool
	repositoriesCommitted bool
	migration             *packageMigrationExecution
}

func (c *packageUpdateCompensation) run() error {
	var failures []error
	if c.migration != nil {
		if err := c.migration.guard.CompensateReverse(context.Background(), c.migration.request, c.migration.handler); err != nil {
			failures = append(failures, err)
		}
	}
	if c.switched || c.target.Current.GenerationID != "" {
		if err := c.runtime.compensatePackageGeneration(context.Background(), c.stable, c.target, c.switched); err != nil {
			failures = append(failures, err)
		}
	}
	if c.repositoriesCommitted {
		if err := c.runtime.restoreForwardPackagePoint(context.Background(), c.rollbackPoint); err != nil {
			failures = append(failures, err)
		} else if err := c.runtime.rebindPackageInstallationGeneration(context.Background(), c.stable, c.rollbackPoint.InstalledPath); err != nil {
			failures = append(failures, err)
		}
	}
	if err := c.runtime.restorePackageMigrationState(context.Background(), c.rollbackPoint); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (r *Runtime) failPackageUpdateOperation(operationID, step string, cause error, compensation *packageUpdateCompensation, guard PackageWriteGuard) error {
	var compensationErr error
	if compensation != nil {
		compensationErr = compensation.run()
	}
	if compensationErr != nil {
		detail := errors.Join(cause, compensationErr)
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "requires_manual_recovery", "PACKAGE_MANUAL_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", step, "PACKAGE_UPDATE_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) completePackageUpdateStep(ctx context.Context, operationID string, order int, name, result string, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name, StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: result, StartedAt: now, CompletedAt: now}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
}

type packageMigrationExecution struct {
	guard   *PackageMigrationGuard
	request migration.ReversibleExecutionRequest
	handler migration.ReversibleStepHandler
}

func (r *Runtime) revalidatePackageMigrationPreflight(ctx context.Context, current domain.ExtensionInstallation, confirmed confirmedPackageUpdate) (*migration.ReversiblePreflight, error) {
	if !packageManifestHasMigrations(confirmed.pkg.Manifest) {
		if confirmed.preview.MigrationPreview != nil || confirmed.preview.MigrationPlanHash != "" || confirmed.claims.MigrationPlanHash != "" {
			return nil, fmt.Errorf("kernel: package migration plan drift")
		}
		return nil, nil
	}
	if r.container.MigrationRepository == nil || confirmed.preview.MigrationPreview == nil || confirmed.preview.MigrationPlanHash == "" {
		return nil, fmt.Errorf("kernel: package migration preflight evidence missing")
	}
	currentPreflight, err := NewPackageMigrationGuard(r.container.MigrationRepository).PreflightManifest(ctx, confirmed.pkg.Manifest, current.InstalledVersion.String())
	if err != nil {
		return nil, fmt.Errorf("kernel: package migration preflight failed: %w", err)
	}
	if currentPreflight.ManualRequired || currentPreflight.Irreversible {
		return nil, fmt.Errorf("kernel: package migration requires controlled manual recovery")
	}
	if confirmed.claims.MigrationPlanHash != confirmed.preview.MigrationPlanHash || currentPreflight.PlanHash != confirmed.preview.MigrationPlanHash || packageCanonicalJSON(currentPreflight) != packageCanonicalJSON(confirmed.preview.MigrationPreview) {
		return nil, fmt.Errorf("kernel: package migration plan drift")
	}
	return currentPreflight, nil
}

func (r *Runtime) executePackageUpdateMigrations(ctx context.Context, packageOperationID string, preflight *migration.ReversiblePreflight, rollbackPoint PackageRollbackPoint, pkg *amitiax.Package, stagingPath string) (*packageMigrationExecution, error) {
	if preflight == nil {
		return nil, nil
	}
	guard := NewPackageMigrationGuard(r.container.MigrationRepository)
	snapshot, err := json.Marshal(rollbackPoint)
	if err != nil {
		return nil, err
	}
	snapshotHash := ""
	if preflight.UserDataSnapshotRequired {
		sum := sha256.Sum256(snapshot)
		snapshotHash = "sha256:" + hex.EncodeToString(sum[:])
	}
	request := migration.ReversibleExecutionRequest{
		OperationID: "migration-" + packageOperationID,
		Preflight:   preflight,
	}
	if preflight.UserDataSnapshotRequired {
		request.Snapshot = snapshot
		request.SnapshotHash = snapshotHash
		request.CurrentSnapshot = func(context.Context) ([]byte, error) {
			return json.Marshal(rollbackPoint)
		}
	}
	migrationRuntime := newPackageMigrationRuntime(r, pkg, stagingPath, preflight.ExtensionID, packageOperationID)
	handler := migrationRuntime.handler()
	execution := &packageMigrationExecution{guard: guard, request: request, handler: handler}
	_, err = guard.ExecuteForward(ctx, request, handler)
	return execution, err
}

func packageManifestRequirements(extensionID domain.ExtensionID, permissions []manifest_v2.PermissionReq) []sqlite.PermissionRequirement {
	result := make([]sqlite.PermissionRequirement, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, sqlite.PermissionRequirement{ExtensionID: extensionID, PermissionName: permission.Name, Reason: permission.Reason, Required: permission.Required, Scope: permission.Scope})
	}
	return result
}

func packageManifestResources(extensionID domain.ExtensionID, resources []manifest_v2.ResourceMeta, generationPath string) []domain.ResourceOwnership {
	now := time.Now().UTC()
	result := make([]domain.ResourceOwnership, 0, len(resources))
	for _, resource := range resources {
		result = append(result, domain.ResourceOwnership{ResourceID: string(extensionID) + "/" + resource.ID, OwnerType: "extension", OwnerID: string(extensionID), ResourceType: resource.Type, Reference: generationPath + "/" + strings.ReplaceAll(resource.Path, "\\", "/"), AcquiredAt: now, Metadata: map[string]any{"manifestResourceId": resource.ID, "hash": resource.Hash, "size": resource.Size}})
	}
	return result
}

func retainPackagePermissionGrants(grants []sqlite.PermissionGrant, requirements []sqlite.PermissionRequirement) []sqlite.PermissionGrant {
	allowed := map[string]bool{}
	for _, requirement := range requirements {
		allowed[requirement.PermissionName] = true
	}
	result := make([]sqlite.PermissionGrant, 0, len(grants))
	for _, grant := range grants {
		if allowed[grant.PermissionName] {
			result = append(result, grant)
		}
	}
	return result
}

func clonePackageMetadata(metadata map[string]any) map[string]any {
	result := make(map[string]any, len(metadata)+8)
	for key, value := range metadata {
		result[key] = value
	}
	return result
}

func computePackageUpdateDiff(oldDefinition domain.ExtensionDefinition, oldModules []domain.ModuleDefinition, oldContributions []domain.ContributionDefinition, oldRequirements []sqlite.PermissionRequirement, oldResources []domain.ResourceOwnership, target manifest_v2.Manifest, artifact PackageArtifact) PackageUpdateDiff {
	targetDefinition, _ := target.ToExtensionDefinition()
	diff := PackageUpdateDiff{DefinitionChanged: packageCanonicalJSON(oldDefinition) != packageCanonicalJSON(targetDefinition)}
	diff.ModulesAdded, diff.ModulesRemoved, diff.ModulesChanged = packageObjectDiff(oldModules, targetDefinition.Modules, func(value domain.ModuleDefinition) string { return string(value.ID) })
	targetContributions := []domain.ContributionDefinition{}
	for _, module := range targetDefinition.Modules {
		targetContributions = append(targetContributions, module.Contributions...)
	}
	diff.ContributionsAdded, diff.ContributionsRemoved, diff.ContributionsChanged = packageObjectDiff(oldContributions, targetContributions, func(value domain.ContributionDefinition) string { return string(value.ID) })
	oldPermissions := map[string]string{}
	for _, value := range oldRequirements {
		oldPermissions[value.PermissionName] = packageCanonicalJSON(value)
	}
	newPermissions := map[string]string{}
	for _, value := range packageManifestRequirements(targetDefinition.ID, target.Permissions) {
		newPermissions[value.PermissionName] = packageCanonicalJSON(value)
	}
	diff.PermissionsAdded, diff.PermissionsRemoved, _ = packageMapDiff(oldPermissions, newPermissions)
	oldScopes := map[string]string{}
	for _, contribution := range oldContributions {
		for _, value := range contribution.RequiredScope {
			oldScopes[string(contribution.ID)+":"+value] = value
		}
	}
	newScopes := map[string]string{}
	for _, contribution := range targetContributions {
		for _, value := range contribution.RequiredScope {
			newScopes[string(contribution.ID)+":"+value] = value
		}
	}
	diff.ScopesAdded, diff.ScopesRemoved, _ = packageMapDiff(oldScopes, newScopes)
	oldResourceMap := map[string]string{}
	for _, value := range oldResources {
		oldResourceMap[value.ResourceID] = packageCanonicalJSON(value)
	}
	newResourceMap := map[string]string{}
	for _, value := range target.Resources {
		newResourceMap[string(targetDefinition.ID)+"/"+value.ID] = packageCanonicalJSON(value)
	}
	diff.ResourcesAdded, diff.ResourcesRemoved, diff.ResourcesChanged = packageMapDiff(oldResourceMap, newResourceMap)
	oldFiles := map[string]string{}
	if oldDefinition.Integrity.FileHashes != nil {
		for path, hash := range oldDefinition.Integrity.FileHashes {
			oldFiles[path] = hash
		}
	}
	newFiles := target.Integrity.FileHashes
	diff.FilesAdded, diff.FilesRemoved, diff.FilesChanged = packageMapDiff(oldFiles, newFiles)
	oldMigrations := packagePrefixedMap(oldFiles, "migrations/")
	newMigrations := packagePrefixedMap(newFiles, "migrations/")
	diff.MigrationsAdded, diff.MigrationsRemoved, _ = packageMapDiff(oldMigrations, newMigrations)
	_ = artifact
	return diff
}

func packageObjectDiff[T any](oldValues, newValues []T, key func(T) string) ([]string, []string, []string) {
	oldMap := map[string]string{}
	newMap := map[string]string{}
	for _, value := range oldValues {
		oldMap[key(value)] = packageCanonicalJSON(value)
	}
	for _, value := range newValues {
		newMap[key(value)] = packageCanonicalJSON(value)
	}
	return packageMapDiff(oldMap, newMap)
}

func packageMapDiff(oldValues, newValues map[string]string) ([]string, []string, []string) {
	added := []string{}
	removed := []string{}
	changed := []string{}
	for key, newValue := range newValues {
		if oldValue, exists := oldValues[key]; !exists {
			added = append(added, key)
		} else if oldValue != newValue {
			changed = append(changed, key)
		}
	}
	for key := range oldValues {
		if _, exists := newValues[key]; !exists {
			removed = append(removed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return added, removed, changed
}

func packagePrefixedMap(values map[string]string, prefix string) map[string]string {
	result := map[string]string{}
	for key, value := range values {
		if strings.HasPrefix(strings.ReplaceAll(key, "\\", "/"), prefix) {
			result[key] = value
		}
	}
	return result
}

func packageCanonicalJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
