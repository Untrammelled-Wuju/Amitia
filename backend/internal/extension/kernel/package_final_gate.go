package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

type PackageFinalGateResult struct {
	Passed        bool                    `json:"passed"`
	OperationID   string                  `json:"operationId"`
	OperationType string                  `json:"operationType"`
	ExtensionID   string                  `json:"extensionId"`
	Version       string                  `json:"version"`
	Checks        []PackageFinalGateCheck `json:"checks"`
	VerifiedAt    string                  `json:"verifiedAt"`
}

type PackageFinalGateCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
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
					actualHash := package_security.ComputeDirHash(installedPath, r.container.PackageSecurity.GetHasher())
					if actualHash == "" || actualHash != expectedHash {
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
		checkTrust.Passed = true
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
	if r.container.PackageGenerationStore != nil {
		if isUninstall {
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
	} else {
		checkGen.Passed = true
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

	allPassed := true
	for _, check := range result.Checks {
		if !check.Passed {
			allPassed = false
			break
		}
	}
	result.Passed = allPassed

	r.recordFinalGateResult(ctx, operationID, result)

	return result, nil
}

func (r *Runtime) recordFinalGateResult(ctx context.Context, operationID string, result PackageFinalGateResult) {
	if r.container == nil || r.container.PackageRepository == nil {
		return
	}
	raw, _ := json.Marshal(result)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := "failed"
	if result.Passed {
		status = "completed"
	}
	_ = r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID:       "package-step-final-gate-" + operationID,
		OperationID:  operationID,
		StepName:     "final_gate_verification",
		StepOrder:    999,
		Status:       status,
		AttemptCount: 1,
		ResultJSON:   string(raw),
		StartedAt:    now,
		CompletedAt:  now,
	}, PackageWriteGuard{})
}
