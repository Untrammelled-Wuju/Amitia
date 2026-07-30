package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

func (r *Runtime) ExecutePackageInstall(ctx context.Context, request PackageInstallRequest) (KernelInstallResult, error) {
	if r.container == nil || r.container.PackageRepository == nil || r.container.PackageArtifactStore == nil {
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
	var required []string
	if err := json.Unmarshal([]byte(session.RequiredConfirmationsJSON), &required); err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: confirmation policy corrupt: %w", err)
	}
	for _, confirmation := range required {
		if !request.Confirmations[confirmation] {
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
	lockValue, _ := r.packageLocks.LoadOrStore(session.ExtensionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	lockedSession, err := r.container.PackageRepository.GetPreview(ctx, request.SessionID, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	if lockedSession.Status == "consumed" {
		return r.completedPackageInstallResult(ctx, request.UserID, request.SessionID)
	}
	if lockedSession.Status != "ready" && lockedSession.Status != "awaiting_confirmation" {
		return KernelInstallResult{}, fmt.Errorf("kernel: preview session status %s", lockedSession.Status)
	}
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	confirmationsJSON, _ := json.Marshal(request.Confirmations)
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID, UserID: request.UserID,
		ScopeType: request.ScopeType, ScopeID: request.ScopeID, ExtensionID: session.ExtensionID,
		TargetVersion: session.Version, OperationType: "install", Status: "created",
		CurrentStep: "create_operation", ArtifactID: artifact.ArtifactID,
		PreviewSessionID: session.SessionID, ConfirmationsJSON: string(confirmationsJSON),
		StartedAt: now, UpdatedAt: now}
	if current, currentErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(session.ExtensionID)); currentErr == nil {
		op.OperationType = "update"
		_ = current
	}
	if err := r.container.PackageRepository.CreateOperation(ctx, op); err != nil {
		return KernelInstallResult{}, err
	}
	step := func(order int, name, status, result, code string) error {
		stamp := time.Now().UTC().Format(time.RFC3339Nano)
		completed := ""
		if status == "completed" || status == "failed" {
			completed = stamp
		}
		if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{StepID: uuid.NewString(), OperationID: operationID, StepName: name, StepOrder: order, Status: status, AttemptCount: 1, ResultJSON: result, ErrorCode: code, StartedAt: stamp, CompletedAt: completed}); err != nil {
			return err
		}
		return r.container.PackageRepository.SetOperation(ctx, operationID, statusForStep(status), name, code, "", false)
	}
	fail := func(name string, cause error, committedPath string) (KernelInstallResult, error) {
		if committedPath != "" {
			if removeErr := r.container.PackageArtifactStore.RemoveInstalled(committedPath); removeErr != nil {
				_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", name, "PACKAGE_RECOVERY_REQUIRED", removeErr.Error(), false)
				return KernelInstallResult{}, fmt.Errorf("%w; recovery required: %v", cause, removeErr)
			}
		}
		_ = step(99, name, "failed", "{}", "PACKAGE_INSTALL_FAILED")
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", name, "PACKAGE_INSTALL_FAILED", cause.Error(), true)
		return KernelInstallResult{}, cause
	}
	if err := step(1, "validate_preview_session", "completed", "{}", ""); err != nil {
		return fail("validate_preview_session", err, "")
	}
	if err := step(2, "reverify_artifact_hash", "completed", "{}", ""); err != nil {
		return fail("reverify_artifact_hash", err, "")
	}
	staging, err := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, operationID)
	if err != nil {
		return fail("extract_to_staging", err, "")
	}
	defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
	if err := step(3, "extract_to_staging", "completed", packageJSON(map[string]string{"stagingId": staging.ID, "path": staging.Path}), ""); err != nil {
		return fail("extract_to_staging", err, "")
	}
	artifactHash := package_security.ComputeDirHash(staging.Path, r.container.PackageSecurity.GetHasher())
	if artifactHash == "" {
		return fail("verify_staging_tree", fmt.Errorf("kernel: staging hash empty"), "")
	}
	definition, err := pkg.Manifest.ToExtensionDefinition()
	if err != nil {
		return fail("build_candidate_definitions", err, "")
	}
	if err := step(4, "build_candidate_definitions", "completed", "{}", ""); err != nil {
		return fail("build_candidate_definitions", err, "")
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
		if err := r.createPackageRollbackPoint(ctx, installed, previousDefinition, previousModules, previousContributions); err != nil {
			return fail("create_rollback_point", err, "")
		}
	}
	targetPath := r.container.PackageArtifactStore.InstalledPath(session.ExtensionID, session.Version, artifactHash)
	commitResult, err := r.container.PackageSecurity.Commit(ctx, staging, targetPath, session.ExtensionID, session.Version)
	if err != nil || commitResult == nil || !commitResult.Success {
		if err == nil {
			err = fmt.Errorf("kernel: installed tree commit failed")
		}
		return fail("commit_installed_tree", err, "")
	}
	if err := step(5, "commit_installed_tree", "completed", packageJSON(map[string]string{"path": targetPath, "artifactHash": artifactHash}), ""); err != nil {
		return fail("commit_installed_tree", err, targetPath)
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
		Metadata: map[string]any{"installedPath": targetPath, "artifactId": artifact.ArtifactID,
			"archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash,
			"contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash,
			"installedTreeHash": artifactHash,
			"ownerUserId":       request.UserID, "scopeType": request.ScopeType, "scopeId": request.ScopeID}}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
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
		return fail("commit_kernel_repositories", err, targetPath)
	}
	artifact.InstalledPath = targetPath
	if err := r.container.PackageRepository.PutArtifact(ctx, artifact); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "persist_artifact_metadata", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false)
		return KernelInstallResult{}, err
	}
	if err := step(6, "commit_kernel_repositories", "completed", "{}", ""); err != nil {
		return KernelInstallResult{}, err
	}
	if err := r.container.PackageRepository.ConsumePreview(ctx, session.SessionID); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "consume_preview_session", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false)
		return KernelInstallResult{}, err
	}
	if err := step(7, "mark_installation_disabled", "completed", "{}", ""); err != nil {
		return KernelInstallResult{}, err
	}
	if err := r.container.PackageRepository.SetOperation(ctx, operationID, "completed", "completed", "", "", true); err != nil {
		return KernelInstallResult{}, err
	}
	return KernelInstallResult{ExtensionID: session.ExtensionID, Version: session.Version,
		InstallationID: installationID, PackageHash: artifact.ArchiveHash,
		ContentTreeHash: artifact.ContentTreeHash, ArtifactPath: artifact.ArchivePath,
		InstallPath: targetPath, DefinitionHash: artifactHash, InstalledAt: time.Now().UTC(),
		OperationID: operationID, TraceID: traceID, Operation: op.OperationType}, nil
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

func (r *Runtime) createPackageRollbackPoint(ctx context.Context, installed domain.ExtensionInstallation, definition *domain.ExtensionDefinition, modules []domain.ModuleDefinition, contributions []domain.ContributionDefinition) error {
	if definition == nil {
		return fmt.Errorf("kernel: current definition missing for rollback point")
	}
	definitionJSON, _ := json.Marshal(definition)
	moduleJSON, _ := json.Marshal(modules)
	contributionJSON, _ := json.Marshal(contributions)
	requirements, err := r.container.PermissionRepository.ListRequirements(ctx, installed.ExtensionID)
	if err != nil {
		return err
	}
	grants, err := r.container.PermissionRepository.ListGrants(ctx, installed.ExtensionID)
	if err != nil {
		return err
	}
	bindings, err := r.container.ScopeRepository.ListBindings(ctx, installed.ExtensionID)
	if err != nil {
		return err
	}
	permissionJSON, _ := json.Marshal(map[string]any{"requirements": requirements, "grants": grants})
	scopeJSON, _ := json.Marshal(bindings)
	installedPath, _ := installed.Metadata["installedPath"].(string)
	artifactID, _ := installed.Metadata["artifactId"].(string)
	point := PackageRollbackPoint{RollbackPointID: "rollback-point-" + uuid.NewString(),
		ExtensionID: string(installed.ExtensionID), SourceVersion: installed.InstalledVersion.String(),
		SourceGeneration: installed.Generation, ArtifactID: artifactID,
		DefinitionSnapshotJSON: string(definitionJSON), ModuleSnapshotJSON: string(moduleJSON),
		ContributionSnapshotJSON: string(contributionJSON), PermissionSnapshotJSON: string(permissionJSON),
		ScopeSnapshotJSON: string(scopeJSON), InstalledPath: installedPath,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return r.container.PackageRepository.PutRollbackPoint(ctx, point)
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
