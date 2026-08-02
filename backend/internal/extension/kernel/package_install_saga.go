package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

func (r *Runtime) ExecutePackageInstall(ctx context.Context, request PackageInstallRequest) (KernelInstallResult, error) {
	if r.container == nil || r.container.PackageRepository == nil || r.container.PackageArtifactStore == nil || r.container.PackageGenerationStore == nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: package services unavailable")
	}
	session, err := r.container.PackageRepository.GetPreview(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session unavailable: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session expired")
	}
	if session.Status == "consumed" {
		return r.completedPackageInstallResult(ctx, request.UserID, request.SessionID)
	}
	if session.Status != "ready" && session.Status != "awaiting_confirmation" {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session status %s", session.Status)
	}
	if session.PolicyVersion != packagePolicyVersion || session.SecurityPolicyHash != computeSecurityPolicyHash() {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview security policy changed")
	}
	if request.ExpectedExtensionID != "" && request.ExpectedExtensionID != session.ExtensionID {
		return KernelInstallResult{}, fmt.Errorf("kernel: package id mismatch: expected %s, got %s", request.ExpectedExtensionID, session.ExtensionID)
	}
	var preview InstallPreview
	if err := json.Unmarshal([]byte(session.PreviewResultJSON), &preview); err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session corrupt: %w", err)
	}
	if !preview.Installable {
		return KernelInstallResult{}, fmt.Errorf("kernel: package is not installable")
	}
	if preview.DevOnly {
		if err := r.validateUnsignedDeveloperSession(preview.DeveloperSessionID, request.UserID, preview.ExtensionID); err != nil {
			return KernelInstallResult{}, fmt.Errorf("kernel: developer session no longer valid: %w", err)
		}
	}
	claims, err := verifyPackageConfirmation(request.ConfirmationToken)
	if err != nil {
		return KernelInstallResult{}, err
	}
	if claims.SessionID != session.SessionID || claims.ArtifactID != session.ArtifactID || claims.ArchiveHash != session.ArchiveHash ||
		claims.ManifestHash != session.ManifestHash || claims.ContentTreeHash != session.ContentTreeHash || claims.UserID != request.UserID ||
		claims.ScopeType != request.ScopeType || claims.ScopeID != request.ScopeID || claims.PolicyVersion != session.PolicyVersion ||
		claims.SecurityPolicyHash != computeSecurityPolicyHash() || claims.DeveloperSessionID != preview.DeveloperSessionID || claims.MigrationPlanHash != preview.MigrationPlanHash {
		return KernelInstallResult{}, fmt.Errorf("kernel: confirmation token binding mismatch")
	}
	var required []string
	if err := json.Unmarshal([]byte(session.RequiredConfirmationsJSON), &required); err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: confirmation policy corrupt: %w", err)
	}
	for _, confirmation := range required {
		if !claims.Confirmations[confirmation] {
			return KernelInstallResult{}, fmt.Errorf("kernel: confirmation required: %s", confirmation)
		}
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, session.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: artifact unavailable: %w", err)
	}
	if artifact.ArchiveHash != session.ArchiveHash || artifact.ManifestHash != session.ManifestHash || artifact.ContentTreeHash != session.ContentTreeHash {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview artifact identity mismatch")
	}
	pkg, err := r.VerifyStoredPackage(ctx, artifact)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: artifact verification failed: %w", err)
	}
	dependencyPreview := InstallPreview{}
	r.evaluatePackageCompatibilityAndDependencies(ctx, pkg, &dependencyPreview)
	if len(dependencyPreview.Issues) > 0 {
		return KernelInstallResult{}, fmt.Errorf("kernel: dependency or compatibility state changed after preview")
	}
	if request.IdempotencyKey == "" {
		return KernelInstallResult{}, NewPackageError(PackageErrCodeIdempotencyKeyRequired, 400, ErrPackageIdempotencyKeyRequired)
	}
	idempotencyKey := request.IdempotencyKey
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	confirmationsJSON, _ := json.Marshal(claims.Confirmations)
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID, UserID: request.UserID,
		ScopeType: request.ScopeType, ScopeID: request.ScopeID, ExtensionID: session.ExtensionID,
		TargetVersion: session.Version, OperationType: "install", Status: "created",
		CurrentStep: "create_operation", ArtifactID: artifact.ArtifactID,
		PreviewSessionID: session.SessionID, ConfirmationsJSON: string(confirmationsJSON),
		IdempotencyKey: idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "install", ExtensionID: session.ExtensionID, TargetVersion: session.Version,
			ArtifactID: artifact.ArtifactID, PreviewSessionID: session.SessionID,
			ScopeType: request.ScopeType, ScopeID: request.ScopeID,
		}), StartedAt: now, UpdatedAt: now}
	existing, created, err := r.container.PackageRepository.CreateOrGetOperation(ctx, op)
	if err != nil {
		return KernelInstallResult{}, err
	}
	if !created {
		if result, handled, handleErr := r.handleExistingPackageOperation(ctx, existing); handled {
			return result, handleErr
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, session.ExtensionID, operationID)
	if leaseErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: extension %s has an active operation: %w", session.ExtensionID, leaseErr)
	}
	leaseGuard := r.newPackageLeaseGuard(session.ExtensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		_ = r.releasePackageExtensionLease(context.Background(), session.ExtensionID, operationID)
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
	if _, currentErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(session.ExtensionID)); currentErr == nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "check_already_installed", "PACKAGE_ALREADY_INSTALLED", fmt.Sprintf("extension %s is already installed", session.ExtensionID), true, guard)
		return KernelInstallResult{}, fmt.Errorf("PACKAGE_ALREADY_INSTALLED: extension %s is already installed", session.ExtensionID)
	}
	stableGeneration := PackageGenerationCurrent{}
	targetGeneration := PackagePreparedGeneration{}
	currentSwitched := false
	step := func(order int, name, status, result, code string) error {
		if renewErr := leaseGuard.AssertAlive(ctx); renewErr != nil {
			return renewErr
		}
		stamp := time.Now().UTC().Format(time.RFC3339Nano)
		completed := ""
		if status == "completed" || status == "failed" {
			completed = stamp
		}
		if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{StepID: uuid.NewString(), OperationID: operationID, StepName: name, StepOrder: order, Status: status, AttemptCount: 1, ResultJSON: result, ErrorCode: code, StartedAt: stamp, CompletedAt: completed, StableGeneration: stableGeneration.GenerationID, TargetGeneration: targetGeneration.Current.GenerationID, CurrentPointerJSON: packageGenerationJSON(targetGeneration.Current)}, guard); err != nil {
			return err
		}
		return r.container.PackageRepository.SetOperation(ctx, operationID, statusForStep(status), name, code, "", false, guard)
	}
	fail := func(name string, cause error, committedPath string) (KernelInstallResult, error) {
		_ = committedPath
		if targetGeneration.Current.GenerationID != "" {
			if compensationErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, currentSwitched); compensationErr != nil {
				_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", name, "PACKAGE_RECOVERY_REQUIRED", compensationErr.Error(), false, guard)
				return KernelInstallResult{}, errors.Join(cause, compensationErr)
			}
		}
		_ = step(99, name, "failed", "{}", "PACKAGE_INSTALL_FAILED")
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", name, "PACKAGE_INSTALL_FAILED", cause.Error(), true, guard)
		return KernelInstallResult{}, cause
	}
	if err := step(1, StepValidatePreviewSession, "completed", "{}", ""); err != nil {
		return fail(StepValidatePreviewSession, err, "")
	}
	if err := step(2, StepReverifyArtifactHash, "completed", "{}", ""); err != nil {
		return fail(StepReverifyArtifactHash, err, "")
	}
	staging, err := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, operationID)
	if err != nil {
		return fail(StepExtractToStaging, err, "")
	}
	defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
	if err := step(3, StepExtractToStaging, "completed", packageJSON(map[string]string{"stagingId": staging.ID, "path": staging.Path}), ""); err != nil {
		return fail(StepExtractToStaging, err, "")
	}
	stagingHash := package_security.ComputeDirHash(staging.Path, r.container.PackageSecurity.GetHasher())
	if stagingHash == "" {
		return fail("install.verify_staging_tree", fmt.Errorf("kernel: staging hash empty"), "")
	}
	definition, err := pkg.Manifest.ToExtensionDefinition()
	if err != nil {
		return fail(StepBuildCandidateDefinitions, err, "")
	}
	if err := step(4, StepBuildCandidateDefinitions, "completed", "{}", ""); err != nil {
		return fail(StepBuildCandidateDefinitions, err, "")
	}
	var previous *domain.ExtensionInstallation
	var previousDefinition *domain.ExtensionDefinition
	var previousModules []domain.ModuleDefinition
	var previousContributions []domain.ContributionDefinition
	if installed, currentErr := r.container.InstallationRepository.GetInstallation(ctx, definition.ID); currentErr == nil {
		previous = &installed
		if oldDef, oldErr := r.container.DefinitionRepository.GetExtension(ctx, definition.ID, installed.InstalledVersion); oldErr == nil {
			previousDefinition = &oldDef
		}
		previousModules, _ = r.container.ModuleRepository.ListModules(ctx, definition.ID)
		previousContributions, _ = r.container.ContributionRepository.ListContributions(ctx, definition.ID)
		if _, err := r.createPackageRollbackPoint(ctx, operationID, "active", installed, previousDefinition, previousModules, previousContributions); err != nil {
			return fail("install.create_rollback_point", err, "")
		}
	}
	targetGeneration, stableGeneration, err = r.preparePackageGeneration(ctx, operationID, artifact, staging.Path, guard.FencingToken)
	if err != nil {
		return fail(StepCommitInstalledTree, err, "")
	}
	if previous == nil && stableGeneration.GenerationID != "" {
		return fail("install.validate_current_pointer", fmt.Errorf("kernel: current pointer exists without installation read model"), targetGeneration.GenerationPath)
	}
	if previous != nil {
		stableFromDB := packageGenerationFromInstallation(*previous)
		if stableFromDB.GenerationID != "" && stableFromDB.GenerationID != stableGeneration.GenerationID {
			return fail("install.validate_current_pointer", fmt.Errorf("kernel: current pointer and installation read model differ"), targetGeneration.GenerationPath)
		}
	}
	targetPath := targetGeneration.GenerationPath
	generationTreeHash := targetGeneration.Current.TreeHash
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, operationID, stableGeneration.GenerationID, targetGeneration.Current.GenerationID, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return fail("install.persist_generation_evidence", err, targetPath)
	}
	commitGenResult := CommitGenerationStepResult{Path: targetPath, TreeHash: generationTreeHash, StableGeneration: stableGeneration.GenerationID, TargetGeneration: targetGeneration.Current.GenerationID, ArtifactHash: artifact.ArtifactHash}
	if err := step(5, StepCommitInstalledTree, "completed", packageJSON(commitGenResult), ""); err != nil {
		return fail(StepCommitInstalledTree, err, targetPath)
	}
	if err := r.switchPackageGeneration(ctx, stableGeneration, targetGeneration); err != nil {
		return fail(StepInstallSwitchCurrentPointer, err, targetPath)
	}
	currentSwitched = true
	if err := step(6, StepInstallSwitchCurrentPointer, "completed", packageGenerationJSON(targetGeneration.Current), ""); err != nil {
		return fail(StepInstallSwitchCurrentPointer, err, targetPath)
	}
	generation := int64(1)
	installedAt := time.Now().UTC()
	installationID := "installation-" + uuid.NewString()
	if previous != nil {
		generation = previous.Generation + 1
		installedAt = previous.InstalledAt
		installationID = previous.InstallationID
	}
	installation := domain.ExtensionInstallation{InstallationID: installationID, ExtensionID: definition.ID,
		InstalledVersion: definition.Version, PackageID: artifact.ArtifactID,
		InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementDisabled,
		InstalledAt: installedAt, UpdatedAt: time.Now().UTC(), Generation: generation,
		Metadata: packageInstallationMetadata(map[string]any{"installedPath": targetPath, "artifactId": artifact.ArtifactID,
			"archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash,
			"contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash,
			"installedTreeHash": generationTreeHash,
			"devOnly":           preview.DevOnly,
			"ownerUserId":       request.UserID, "scopeType": request.ScopeType, "scopeId": request.ScopeID}, targetGeneration.Current, targetPath, operationID)}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, guard); err != nil {
			return err
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, definition.ID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, definition.ID); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.PutExtension(txCtx, definition); err != nil {
			return err
		}
		for _, module := range definition.Modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, module); err != nil {
				return err
			}
			for _, contribution := range module.Contributions {
				if err := r.container.ContributionRepository.PutContribution(txCtx, contribution); err != nil {
					return err
				}
			}
		}
		return r.container.InstallationRepository.PutInstallation(txCtx, installation)
	})
	if err != nil {
		return fail(StepCommitKernelRepositories, err, targetPath)
	}
	artifact.InstalledPath = targetPath
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, artifact.ArtifactID, targetPath, guard); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "persist_artifact_metadata", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, guard)
		return KernelInstallResult{}, err
	}
	commitRepoResult := CommitRepositoryResult{InstallationID: installationID, VersionID: session.Version, ArtifactID: artifact.ArtifactID, GenerationID: targetGeneration.Current.GenerationID}
	if err := step(7, StepCommitKernelRepositories, "completed", packageJSON(commitRepoResult), ""); err != nil {
		return KernelInstallResult{}, err
	}
	if err := r.container.PackageRepository.ConsumePreview(ctx, session.SessionID); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "consume_preview_session", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, guard)
		return KernelInstallResult{}, err
	}
	if err := step(8, StepMarkInstallationDisabled, "completed", "{}", ""); err != nil {
		return KernelInstallResult{}, err
	}
	if err := r.recordPackageVersionAfterOperation(ctx, operationID, "install", session.ExtensionID, session.Version, artifact.ArtifactID, targetPath, generationTreeHash, artifact.ArchiveHash, artifact.ManifestHash, artifact.ContentTreeHash, targetGeneration.Current.GenerationID, guard); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "record_version", "PACKAGE_VERSION_HISTORY_CORRUPTED", err.Error(), false, guard)
		return KernelInstallResult{}, err
	}
	if err := r.FinalizePackageOperation(ctx, operationID, session.ExtensionID, leaseGuard, guard); err != nil {
		return KernelInstallResult{}, err
	}
	return KernelInstallResult{ExtensionID: session.ExtensionID, Version: session.Version,
		InstallationID: installationID, PackageHash: artifact.ArchiveHash,
		ContentTreeHash: artifact.ContentTreeHash, ArtifactPath: artifact.ArchivePath,
		InstallPath: targetPath, DefinitionHash: generationTreeHash, InstalledAt: time.Now().UTC(),
		OperationID: operationID, TraceID: traceID, Operation: op.OperationType}, nil
}

func (r *Runtime) ConfirmPackagePreview(ctx context.Context, request PackagePreviewConfirmationRequest) (PackagePreviewConfirmation, error) {
	if r.container == nil || r.container.PackageRepository == nil {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: package services unavailable")
	}
	session, err := r.container.PackageRepository.GetPreview(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: preview session unavailable: %w", err)
	}
	if session.Status != "ready" && session.Status != "awaiting_confirmation" {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: preview session status %s", session.Status)
	}
	if session.PolicyVersion != packagePolicyVersion || session.SecurityPolicyHash != computeSecurityPolicyHash() {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: preview security policy changed")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: preview session expired")
	}
	var required []string
	if err := json.Unmarshal([]byte(session.RequiredConfirmationsJSON), &required); err != nil {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: confirmation policy corrupt: %w", err)
	}
	confirmed := make(map[string]bool, len(required))
	for _, confirmation := range required {
		if !request.Confirmations[confirmation] {
			return PackagePreviewConfirmation{}, fmt.Errorf("kernel: confirmation required: %s", confirmation)
		}
		confirmed[confirmation] = true
	}
	var preview InstallPreview
	if err := json.Unmarshal([]byte(session.PreviewResultJSON), &preview); err != nil {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: preview session corrupt: %w", err)
	}
	if preview.DevOnly {
		if err := r.validateUnsignedDeveloperSession(preview.DeveloperSessionID, request.UserID, preview.ExtensionID); err != nil {
			return PackagePreviewConfirmation{}, fmt.Errorf("kernel: developer session no longer valid: %w", err)
		}
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, session.ArtifactID)
	if err != nil {
		return PackagePreviewConfirmation{}, fmt.Errorf("kernel: artifact unavailable: %w", err)
	}
	tokenExpiry := time.Now().UTC().Add(10 * time.Minute)
	if expiresAt.Before(tokenExpiry) {
		tokenExpiry = expiresAt
	}
	installReq := ComputeInstallSnapshotRequirement(computeInstallSnapshotRequirementInput(preview.InstalledPath, preview.InstalledTreeHash, artifact.ArtifactID, session.ExtensionID))
	token, err := signPackageConfirmation(packageConfirmationClaims{SessionID: session.SessionID, ArtifactID: session.ArtifactID,
		ArchiveHash: session.ArchiveHash, ManifestHash: session.ManifestHash, ContentTreeHash: session.ContentTreeHash,
		UserID: session.UserID, ScopeType: session.ScopeType, ScopeID: session.ScopeID, PolicyVersion: session.PolicyVersion,
		SecurityPolicyHash: computeSecurityPolicyHash(), DeveloperSessionID: preview.DeveloperSessionID, MigrationPlanHash: preview.MigrationPlanHash,
		SnapshotRequirementHash: installReq.RequirementHash, InstalledPath: preview.InstalledPath, InstalledTreeHash: preview.InstalledTreeHash,
		Confirmations: confirmed, ExpiresAt: tokenExpiry.Unix()})
	if err != nil {
		return PackagePreviewConfirmation{}, err
	}
	return PackagePreviewConfirmation{ConfirmationToken: token, ExpiresAt: tokenExpiry}, nil
}

func (r *Runtime) completedPackageInstallResult(ctx context.Context, userID, sessionID string) (KernelInstallResult, error) {
	op, err := r.container.PackageRepository.GetCompletedOperationByPreview(ctx, userID, sessionID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: consumed session has no completed operation: %w", err)
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(op.ExtensionID))
	if err != nil {
		return KernelInstallResult{}, err
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, op.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	installPath, _ := installation.Metadata["installedPath"].(string)
	return KernelInstallResult{ExtensionID: op.ExtensionID, Version: op.TargetVersion,
		InstallationID: installation.InstallationID, PackageHash: artifact.ArchiveHash,
		ContentTreeHash: artifact.ContentTreeHash, ArtifactPath: artifact.ArchivePath,
		InstallPath: installPath, DefinitionHash: artifact.ArtifactHash, InstalledAt: installation.UpdatedAt,
		OperationID: op.OperationID, TraceID: op.TraceID, Operation: op.OperationType}, nil
}

func (r *Runtime) createPackageRollbackPoint(ctx context.Context, sourceOperationID, retentionState string, installed domain.ExtensionInstallation, definition *domain.ExtensionDefinition, modules []domain.ModuleDefinition, contributions []domain.ContributionDefinition) (PackageRollbackPoint, error) {
	if definition == nil {
		return PackageRollbackPoint{}, fmt.Errorf("kernel: current definition missing for rollback point")
	}
	definitionJSON, err := json.Marshal(definition)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	moduleJSON, err := json.Marshal(modules)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	contributionJSON, err := json.Marshal(contributions)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	requirements, err := r.container.PermissionRepository.ListRequirements(ctx, installed.ExtensionID)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	grants, err := r.container.PermissionRepository.ListGrants(ctx, installed.ExtensionID)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	bindings, err := r.container.ScopeRepository.ListBindings(ctx, installed.ExtensionID)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	permissionJSON, err := json.Marshal(map[string]any{"requirements": requirements, "grants": grants})
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	scopeJSON, err := json.Marshal(bindings)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	configJSON, secretRefsJSON, resourceJSON, migrationJSON, userDataMigrationJSON, err := r.capturePackageStateSnapshots(ctx, installed)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	if configJSON == "" || resourceJSON == "" || migrationJSON == "" || userDataMigrationJSON == "" {
		return PackageRollbackPoint{}, fmt.Errorf("kernel: rollback point requires complete snapshots (config/resource/migration/userdata)")
	}
	if string(definitionJSON) == "" || string(moduleJSON) == "" || string(contributionJSON) == "" || string(permissionJSON) == "" || string(scopeJSON) == "" {
		return PackageRollbackPoint{}, fmt.Errorf("kernel: rollback point requires complete package snapshots (definition/module/contribution/permission/scope)")
	}
	installedPath, _ := installed.Metadata["installedPath"].(string)
	artifactID, _ := installed.Metadata["artifactId"].(string)
	retentionUntil := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	point := PackageRollbackPoint{RollbackPointID: "rollback-point-" + uuid.NewString(),
		ExtensionID: string(installed.ExtensionID), SourceVersion: installed.InstalledVersion.String(),
		SourceGeneration: installed.Generation, ArtifactID: artifactID,
		DefinitionSnapshotJSON: string(definitionJSON), ModuleSnapshotJSON: string(moduleJSON),
		ContributionSnapshotJSON: string(contributionJSON), PermissionSnapshotJSON: string(permissionJSON),
		ScopeSnapshotJSON: string(scopeJSON), ConfigSnapshotJSON: configJSON,
		SecretRefsJSON: secretRefsJSON, ResourceSnapshotJSON: resourceJSON,
		MigrationStateSnapshotJSON: migrationJSON, UserDataMigrationStateJSON: userDataMigrationJSON,
		RetentionState: retentionState, RetentionUntil: retentionUntil, ExpiresAt: retentionUntil,
		SourceOperationID: sourceOperationID, InstalledPath: installedPath,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	point.SnapshotHash, err = computePackageSnapshotHash(point)
	if err != nil {
		return PackageRollbackPoint{}, err
	}
	if err := r.container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		return PackageRollbackPoint{}, err
	}
	return point, nil
}

func statusForStep(status string) string {
	if status == "failed" {
		return "compensating"
	}
	return "in_progress"
}

func packageJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
