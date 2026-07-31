package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageFinalGateResult struct {
	Passed        bool                    `json:"passed"`
	OperationID   string                  `json:"operationId"`
	OperationType string                  `json:"operationType"`
	ExtensionID   string                  `json:"extensionId"`
	Version       string                  `json:"version"`
	Checks        []PackageFinalGateCheck `json:"checks"`
	Findings      []PackageFinalGateFinding `json:"findings,omitempty"`
	VerifiedAt    string                  `json:"verifiedAt"`
}

type PackageFinalGateCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

type PackageFinalGateFinding struct {
	FindingID    string `json:"findingId"`
	OperationID  string `json:"operationId"`
	ExtensionID  string `json:"extensionId"`
	FindingType  string `json:"findingType"`
	ResourceID   string `json:"resourceId,omitempty"`
	Severity     string `json:"severity"`
	Expected     string `json:"expected,omitempty"`
	Actual       string `json:"actual,omitempty"`
	DetectedAt   string `json:"detectedAt"`
	ResolvedAt   string `json:"resolvedAt,omitempty"`
}

func (r *PackageRepository) ListOperationSteps(ctx context.Context, operationID string) ([]PackageOperationStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at, stable_generation,
		target_generation, current_pointer_json
		FROM extension_package_operation_steps WHERE operation_id = ? ORDER BY step_order`, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []PackageOperationStep
	for rows.Next() {
		var step PackageOperationStep
		if err := rows.Scan(&step.StepID, &step.OperationID, &step.StepName, &step.StepOrder, &step.Status,
			&step.AttemptCount, &step.ResultJSON, &step.ErrorCode, &step.StartedAt, &step.CompletedAt,
			&step.StableGeneration, &step.TargetGeneration, &step.CurrentPointerJSON); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *Runtime) VerifyPackageFinalGate(ctx context.Context, operationID string) (PackageFinalGateResult, error) {
	result := PackageFinalGateResult{
		OperationID: operationID,
		Checks:      make([]PackageFinalGateCheck, 0, 10),
		VerifiedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}

	if r.container == nil || r.container.PackageRepository == nil {
		return result, fmt.Errorf("kernel: package repository unavailable")
	}

	operation, err := r.container.PackageRepository.getAuthoritativeOperationByID(ctx, operationID)
	if err != nil {
		return result, fmt.Errorf("kernel: operation %s unavailable: %w", operationID, err)
	}

	result.OperationType = operation.OperationType
	result.ExtensionID = operation.ExtensionID
	result.Version = operation.TargetVersion

	isUninstall := operation.OperationType == "uninstall"

	checkStatus := PackageFinalGateCheck{Name: "operation_status_completed"}
	if operation.Status == string(PackageOperationCompleted) || operation.Status == string(PackageOperationInProgress) {
		checkStatus.Passed = true
	} else {
		checkStatus.Detail = fmt.Sprintf("operation status is %s, expected completed or in_progress", operation.Status)
	}
	result.Checks = append(result.Checks, checkStatus)

	var installation domain.ExtensionInstallation
	var installErr error
	if !isUninstall {
		installation, installErr = r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	} else {
		_, installErr = r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	}

	if isUninstall {
		checkInstall := PackageFinalGateCheck{Name: "installation_record_absent"}
		if errors.Is(installErr, domain.ErrInvalidExtensionID) {
			checkInstall.Passed = true
		} else if installErr == nil {
			checkInstall.Detail = "installation record still exists after uninstall"
		} else {
			checkInstall.Detail = fmt.Sprintf("installation state unavailable: %v", installErr)
		}
		result.Checks = append(result.Checks, checkInstall)
	} else {
		checkInstall := PackageFinalGateCheck{Name: "installation_record_consistent"}
		if installErr != nil {
			checkInstall.Detail = fmt.Sprintf("installation unavailable: %v", installErr)
		} else if installation.PackageID != operation.ArtifactID {
			checkInstall.Detail = fmt.Sprintf("artifact id mismatch: %s != %s", installation.PackageID, operation.ArtifactID)
		} else if installation.InstalledVersion.String() != operation.TargetVersion {
			checkInstall.Detail = fmt.Sprintf("version mismatch: %s != %s", installation.InstalledVersion.String(), operation.TargetVersion)
		} else if installation.InstallationState != domain.InstallationStateInstalled {
			checkInstall.Detail = fmt.Sprintf("installation state is %s, expected installed", installation.InstallationState)
		} else {
			checkInstall.Passed = true
		}
		result.Checks = append(result.Checks, checkInstall)
	}

	if isUninstall {
		checkArtifact := PackageFinalGateCheck{Name: "artifact_path_absent", Passed: true}
		artifact, artifactErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if artifactErr == nil && artifact.InstalledPath != "" {
			if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
				checkArtifact.Passed = false
				checkArtifact.Detail = "installed path still exists after uninstall"
			}
		}
		result.Checks = append(result.Checks, checkArtifact)
	} else {
		checkArtifact := PackageFinalGateCheck{Name: "artifact_path_and_hash"}
		if installErr != nil {
			checkArtifact.Detail = "installation unavailable, cannot verify path"
		} else {
			installedPath, _ := installation.Metadata["installedPath"].(string)
			expectedHash, _ := installation.Metadata["installedTreeHash"].(string)
			if installedPath == "" {
				checkArtifact.Detail = "installed path missing from installation metadata"
			} else {
				info, statErr := os.Stat(installedPath)
				if statErr != nil || !info.IsDir() {
					checkArtifact.Detail = fmt.Sprintf("installed path unavailable: %v", statErr)
				} else {
					actualHash, hashErr := computeGenerationTreeHash(ctx, installedPath)
					if hashErr != nil || actualHash != expectedHash {
						checkArtifact.Detail = "installed tree hash mismatch"
					} else {
						checkArtifact.Passed = true
					}
				}
			}
		}
		result.Checks = append(result.Checks, checkArtifact)
	}

	checkTrust := PackageFinalGateCheck{Name: "trust_revocation_blocklist"}
	if isUninstall {
		checkTrust.Passed = true
	} else if r.container.TrustService == nil {
		checkTrust.Passed = false
		checkTrust.Detail = "trust service unavailable, fail closed"
	} else if r.container.PackageTrustRepository == nil {
		checkTrust.Passed = false
		checkTrust.Detail = "trust policy repository unavailable, fail closed"
	} else {
		artifactForTrust, artErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if artErr != nil {
			checkTrust.Detail = fmt.Sprintf("artifact unavailable for trust verification: %v", artErr)
		} else {
			packageHash := artifactForTrust.ArchiveHash
			publisherID := artifactForTrust.PublisherID
			signerKeyID := artifactForTrust.SignerKeyID
			if blocked := r.container.TrustService.Blocklist().Check(packageHash); blocked != nil {
				checkTrust.Detail = fmt.Sprintf("package hash %s is blocklisted: %s", packageHash, blocked.Reason)
			} else if revokedKey := r.container.TrustService.RevocationList().CheckKey(publisherID, signerKeyID); revokedKey != nil {
				checkTrust.Detail = fmt.Sprintf("signing key %s is revoked: %s", signerKeyID, revokedKey.Reason)
			} else if revokedPkg := r.container.TrustService.RevocationList().CheckPackage(packageHash); revokedPkg != nil {
				checkTrust.Detail = fmt.Sprintf("package hash %s is revoked: %s", packageHash, revokedPkg.Reason)
			} else if revokedPub := r.container.TrustService.RevocationList().CheckPublisher(publisherID); revokedPub != nil {
				checkTrust.Detail = fmt.Sprintf("publisher %s is revoked: %s", publisherID, revokedPub.Reason)
			} else {
				checkTrust.Passed = true
			}
		}
	}
	result.Checks = append(result.Checks, checkTrust)

	checkGen := PackageFinalGateCheck{Name: "generation_pointer_consistent"}
	if r.container.PackageGenerationStore == nil {
		checkGen.Passed = false
		checkGen.Detail = "generation store unavailable, fail closed"
	} else if isUninstall {
		_, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
		if errors.Is(currentErr, ErrPackageGenerationNotFound) {
			checkGen.Passed = true
		} else if currentErr == nil {
			checkGen.Detail = "generation current pointer still exists after uninstall"
		} else {
			checkGen.Detail = fmt.Sprintf("generation current read failed: %v", currentErr)
		}
	} else if installErr != nil {
		checkGen.Detail = "installation unavailable, cannot verify generation"
	} else {
		generationID, _ := installation.Metadata["generationId"].(string)
		if generationID == "" {
			checkGen.Passed = true
		} else {
			current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
			if currentErr != nil {
				checkGen.Detail = fmt.Sprintf("generation current read failed: %v", currentErr)
			} else if current.GenerationID != generationID {
				checkGen.Detail = fmt.Sprintf("generation id mismatch: %s != %s", current.GenerationID, generationID)
			} else {
				if verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, current); verifyErr != nil {
					checkGen.Detail = fmt.Sprintf("generation verification failed: %v", verifyErr)
				} else {
					checkGen.Passed = true
				}
			}
		}
	}
	result.Checks = append(result.Checks, checkGen)

	checkPerm := PackageFinalGateCheck{Name: "permission_and_scope_consistent"}
	if isUninstall {
		permClean := true
		if r.container.PermissionRepository != nil {
			requirements, reqErr := r.container.PermissionRepository.ListRequirements(ctx, domain.ExtensionID(operation.ExtensionID))
			if reqErr == nil && len(requirements) > 0 {
				permClean = false
				checkPerm.Detail = "permission requirements still exist after uninstall"
			}
			grants, grantErr := r.container.PermissionRepository.ListGrants(ctx, domain.ExtensionID(operation.ExtensionID))
			if grantErr == nil && len(grants) > 0 {
				permClean = false
				checkPerm.Detail = "permission grants still exist after uninstall"
			}
		}
		if r.container.ScopeRepository != nil && permClean {
			bindings, bindErr := r.container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(operation.ExtensionID))
			if bindErr == nil && len(bindings) > 0 {
				permClean = false
				checkPerm.Detail = "scope bindings still exist after uninstall"
			}
		}
		if permClean {
			checkPerm.Passed = true
		}
	} else {
		if installErr != nil {
			checkPerm.Detail = "installation unavailable, cannot verify permissions"
		} else if r.container.PermissionRepository != nil && r.container.ScopeRepository != nil {
			_, reqErr := r.container.PermissionRepository.ListRequirements(ctx, domain.ExtensionID(operation.ExtensionID))
			_, grantErr := r.container.PermissionRepository.ListGrants(ctx, domain.ExtensionID(operation.ExtensionID))
			_, bindErr := r.container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(operation.ExtensionID))
			if reqErr != nil || grantErr != nil || bindErr != nil {
				checkPerm.Detail = "permission or scope repository query failed"
			} else {
				checkPerm.Passed = true
			}
		} else {
			checkPerm.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkPerm)

	checkLease := PackageFinalGateCheck{Name: "no_active_lease"}
	lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
	if IsPackageOperationError(leaseErr, OperationErrNotFound) {
		checkLease.Passed = true
	} else if leaseErr == nil {
		if lease.OperationID == operationID {
			checkLease.Passed = true
		} else {
			checkLease.Detail = fmt.Sprintf("active lease held by operation %s", lease.OperationID)
		}
	} else {
		checkLease.Detail = fmt.Sprintf("lease state unavailable: %v", leaseErr)
	}
	result.Checks = append(result.Checks, checkLease)

	steps, stepErr := r.container.PackageRepository.ListOperationSteps(ctx, operationID)

	checkNoFailed := PackageFinalGateCheck{Name: "no_failed_steps"}
	if stepErr != nil {
		checkNoFailed.Detail = fmt.Sprintf("operation steps unavailable: %v", stepErr)
	} else {
		var failedStepNames string
		failedCount := 0
		for _, step := range steps {
			if step.StepOrder == 999 {
				continue
			}
			if step.Status == "failed" {
				if failedCount > 0 {
					failedStepNames += ", "
				}
				failedStepNames += step.StepName
				failedCount++
			}
		}
		if failedCount > 0 {
			checkNoFailed.Detail = fmt.Sprintf("failed steps: %s", failedStepNames)
		} else {
			checkNoFailed.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkNoFailed)

	checkSteps := PackageFinalGateCheck{Name: "operation_steps_complete"}
	if stepErr != nil {
		checkSteps.Detail = fmt.Sprintf("operation steps unavailable: %v", stepErr)
	} else if len(steps) == 0 {
		checkSteps.Detail = "no operation steps recorded"
	} else {
		hasFailed := false
		hasIncomplete := false
		nonFinalGateCount := 0
		for _, step := range steps {
			if step.StepOrder == 999 {
				continue
			}
			nonFinalGateCount++
			if step.Status == "failed" {
				hasFailed = true
			} else if step.Status != "completed" {
				hasIncomplete = true
			}
		}
		if nonFinalGateCount == 0 {
			checkSteps.Detail = "no non-final-gate steps recorded"
		} else if hasFailed {
			checkSteps.Detail = "one or more steps failed"
		} else if hasIncomplete {
			checkSteps.Detail = "one or more steps not completed"
		} else {
			checkSteps.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkSteps)

	checkConsistency := PackageFinalGateCheck{Name: "consistency_probe"}
	if isUninstall {
		if installErr == nil {
			checkConsistency.Detail = "installation record still exists for uninstalled extension"
		} else if errors.Is(installErr, domain.ErrInvalidExtensionID) {
			checkConsistency.Passed = true
		} else {
			checkConsistency.Detail = fmt.Sprintf("installation state unavailable: %v", installErr)
		}
	} else {
		if installErr != nil {
			checkConsistency.Detail = fmt.Sprintf("installation unavailable: %v", installErr)
		} else if installation.InstalledVersion.String() != operation.TargetVersion {
			checkConsistency.Detail = fmt.Sprintf("version mismatch: %s != %s", installation.InstalledVersion.String(), operation.TargetVersion)
		} else {
			checkConsistency.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkConsistency)

	if !isUninstall && installErr == nil {
		checkStoredPkg := PackageFinalGateCheck{Name: "stored_package_verification"}
		artifactForVerify, artVErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if artVErr != nil {
			checkStoredPkg.Detail = fmt.Sprintf("artifact unavailable for stored package verification: %v", artVErr)
		} else if _, verifyErr := r.VerifyStoredPackage(ctx, artifactForVerify); verifyErr != nil {
			checkStoredPkg.Detail = fmt.Sprintf("stored package verification failed: %v", verifyErr)
		} else {
			checkStoredPkg.Passed = true
		}
		result.Checks = append(result.Checks, checkStoredPkg)
	}

	checkVersion := PackageFinalGateCheck{Name: "version_record_consistent"}
	if isUninstall {
		checkVersion.Passed = true
	} else if r.container.PackageRepository == nil {
		checkVersion.Passed = false
		checkVersion.Detail = "package repository unavailable for version record check"
	} else {
		versionRecord, vErr := r.container.PackageRepository.GetCurrentPackageVersion(ctx, operation.ExtensionID)
		if vErr != nil {
			checkVersion.Passed = false
			checkVersion.Detail = fmt.Sprintf("current version record missing: %v", vErr)
		} else if versionRecord.ArtifactID != operation.ArtifactID {
			checkVersion.Passed = false
			checkVersion.Detail = fmt.Sprintf("version record artifact mismatch: %s != %s", versionRecord.ArtifactID, operation.ArtifactID)
		} else if versionRecord.Version != operation.TargetVersion {
			checkVersion.Passed = false
			checkVersion.Detail = fmt.Sprintf("version record version mismatch: %s != %s", versionRecord.Version, operation.TargetVersion)
		} else {
			checkVersion.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkVersion)

	allPassed := true
	var findings []PackageFinalGateFinding
	for _, check := range result.Checks {
		if !check.Passed {
			allPassed = false
			findings = append(findings, PackageFinalGateFinding{
				FindingID:    "gate-finding-" + uuid.NewString(),
				OperationID:  operationID,
				ExtensionID:  operation.ExtensionID,
				FindingType:  check.Name,
				Severity:     "error",
				Expected:     "passed",
				Actual:       check.Detail,
				DetectedAt:   result.VerifiedAt,
			})
		}
	}
	result.Passed = allPassed
	result.Findings = findings

	if persistErr := r.recordFinalGateResult(ctx, operationID, result); persistErr != nil {
		if allPassed {
			return result, fmt.Errorf("kernel: final gate finding persistence failed: %w", persistErr)
		}
	}

	if !allPassed {
		var failedDetails string
		for _, f := range findings {
			if failedDetails != "" {
				failedDetails += "; "
			}
			failedDetails += f.FindingType + ": " + f.Actual
		}
		return result, &PackageError{
			Code:              PackageErrCodeFinalGateFailed,
			HTTPStatus:        409,
			Retryable:         false,
			RecoveryRequired:  true,
			RecommendedAction: "requires_recovery",
			Cause:             fmt.Errorf("%s: %s", ErrPackageFinalGateFailed, failedDetails),
		}
	}

	return result, nil
}

func (r *Runtime) recordFinalGateResult(ctx context.Context, operationID string, result PackageFinalGateResult) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return fmt.Errorf("kernel: package repository unavailable")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("kernel: marshal final gate result: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := "failed"
	if result.Passed {
		status = "completed"
	}
	operation, _ := r.container.PackageRepository.getAuthoritativeOperationByID(ctx, operationID)
	guard := PackageWriteGuard{}
	if operation.OperationID != "" {
		lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
		if leaseErr == nil && lease.OperationID == operationID {
			guard = PackageWriteGuard{ExtensionID: operation.ExtensionID, FencingToken: lease.FencingToken}
		}
	}
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID:       "package-step-final-gate-" + operationID,
		OperationID:  operationID,
		StepName:     "final_gate_verification",
		StepOrder:    999,
		Status:       status,
		AttemptCount: 1,
		ResultJSON:   string(raw),
		StartedAt:    now,
		CompletedAt:  now,
	}, guard); err != nil {
		return fmt.Errorf("kernel: persist final gate step: %w", err)
	}
	for _, finding := range result.Findings {
		if err := r.container.PackageRepository.PutConsistencyFinding(ctx, PackageConsistencyFinding{
			FindingID:         finding.FindingID,
			Metric:            "final_gate_" + finding.FindingType,
			Count:             1,
			ResourceIDsJSON:   fmt.Sprintf(`["%s","%s"]`, finding.OperationID, finding.ExtensionID),
			ErrorDetail:       finding.Actual,
			RecommendedAction: "requires_recovery",
		}); err != nil {
			return fmt.Errorf("kernel: persist final gate finding %s: %w", finding.FindingID, err)
		}
	}
	return nil
}
