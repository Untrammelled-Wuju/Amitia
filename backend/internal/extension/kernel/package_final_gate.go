package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageFinalGateResult struct {
	Passed        bool                      `json:"passed"`
	OperationID   string                    `json:"operationId"`
	OperationType string                    `json:"operationType"`
	ExtensionID   string                    `json:"extensionId"`
	Version       string                    `json:"version"`
	Checks        []PackageFinalGateCheck   `json:"checks"`
	Findings      []PackageFinalGateFinding `json:"findings,omitempty"`
	VerifiedAt    string                    `json:"verifiedAt"`
}

type PackageFinalGateCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

type PackageFinalGateFinding struct {
	FindingID   string `json:"findingId"`
	OperationID string `json:"operationId"`
	ExtensionID string `json:"extensionId"`
	FindingType string `json:"findingType"`
	ResourceID  string `json:"resourceId,omitempty"`
	Severity    string `json:"severity"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
	DetectedAt  string `json:"detectedAt"`
	ResolvedAt  string `json:"resolvedAt,omitempty"`
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
	return r.verifyPackageFinalGateWithGuard(ctx, operationID, PackageWriteGuard{})
}

func (r *Runtime) verifyPackageFinalGateWithGuard(ctx context.Context, operationID string, guard PackageWriteGuard) (PackageFinalGateResult, error) {
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

	checkIdentity := PackageFinalGateCheck{Name: "authoritative_identity"}
	{
		var missing []string
		if operation.OperationID == "" {
			missing = append(missing, "OperationID")
		}
		if operation.ExtensionID == "" {
			missing = append(missing, "ExtensionID")
		}
		if operation.FencingToken == 0 {
			missing = append(missing, "FencingToken")
		}
		if !isUninstall {
			if operation.ArtifactID == "" {
				missing = append(missing, "ArtifactID")
			}
			if operation.TargetGeneration == "" {
				missing = append(missing, "GenerationID")
			}
			if operation.TargetVersion == "" {
				missing = append(missing, "VersionID")
			}
		}
		if len(missing) > 0 {
			checkIdentity.Detail = "missing: " + strings.Join(missing, ", ")
		} else {
			checkIdentity.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkIdentity)

	checkStatus := PackageFinalGateCheck{Name: "operation_status_completed"}
	if operation.Status == string(PackageOperationCompleted) || operation.Status == string(PackageOperationInProgress) || operation.Status == string(PackageOperationFinalizing) {
		checkStatus.Passed = true
	} else {
		checkStatus.Detail = fmt.Sprintf("operation status is %s, expected completed, in_progress or finalizing", operation.Status)
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
		checkArtifact := PackageFinalGateCheck{Name: "artifact_path_absent"}
		artifact, artifactErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if artifactErr == nil {
			if artifact.InstalledPath == "" {
				checkArtifact.Passed = true
			} else {
				if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
					checkArtifact.Detail = "installed path still exists after uninstall"
				} else {
					checkArtifact.Passed = true
				}
			}
		} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorNotFound) {
			claims, claimsErr := parseOperationConfirmationClaims(operation)
			if claimsErr != nil {
				checkArtifact.Detail = fmt.Sprintf("artifact not found and confirmation claims unavailable: %v", claimsErr)
			} else if claims.ArtifactPolicy != ArtifactPolicyDeleteArtifact {
				checkArtifact.Detail = fmt.Sprintf("artifact not found but claims policy is %s, fail closed", claims.ArtifactPolicy)
			} else if !validateUninstallArtifactDeletion(ctx, r.container.PackageRepository, operationID, operation.ArtifactID) {
				checkArtifact.Detail = "artifact not found but remove_artifact step evidence incomplete"
			} else {
				refCount, refErr := r.container.PackageRepository.CountActiveArtifactReferences(ctx, operation.ArtifactID)
				if refErr != nil {
					checkArtifact.Detail = fmt.Sprintf("artifact reference check failed (repository unavailable): %v", refErr)
				} else if refCount > 0 {
					checkArtifact.Detail = fmt.Sprintf("artifact not found but still has %d active references", refCount)
				} else {
					checkArtifact.Passed = true
				}
			}
		} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorUnavailable) {
			checkArtifact.Detail = fmt.Sprintf("artifact repository unavailable, fail closed: %v", artifactErr)
		} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorCorrupt) {
			checkArtifact.Detail = fmt.Sprintf("artifact repository corrupt, fail closed: %v", artifactErr)
		} else {
			checkArtifact.Detail = fmt.Sprintf("artifact query failed (unknown error), fail closed: %v", artifactErr)
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
			blockVersion := func(reason string) {
				if operation.TargetVersion != "" {
					_ = r.container.PackageRepository.BlockPackageVersion(ctx, operation.ExtensionID, operation.TargetVersion, reason)
				}
			}
			if blocked := r.container.TrustService.Blocklist().Check(packageHash); blocked != nil {
				checkTrust.Detail = fmt.Sprintf("package hash %s is blocklisted: %s", packageHash, blocked.Reason)
				blockVersion("blocklisted: " + string(blocked.Reason))
			} else if revokedKey := r.container.TrustService.RevocationList().CheckKey(publisherID, signerKeyID); revokedKey != nil {
				checkTrust.Detail = fmt.Sprintf("signing key %s is revoked: %s", signerKeyID, revokedKey.Reason)
				blockVersion("signing_key_revoked: " + string(revokedKey.Reason))
			} else if revokedPkg := r.container.TrustService.RevocationList().CheckPackage(packageHash); revokedPkg != nil {
				checkTrust.Detail = fmt.Sprintf("package hash %s is revoked: %s", packageHash, revokedPkg.Reason)
				blockVersion("package_revoked: " + string(revokedPkg.Reason))
			} else if revokedPub := r.container.TrustService.RevocationList().CheckPublisher(publisherID); revokedPub != nil {
				checkTrust.Detail = fmt.Sprintf("publisher %s is revoked: %s", publisherID, revokedPub.Reason)
				blockVersion("publisher_revoked: " + string(revokedPub.Reason))
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
			checkGen.Passed = false
			checkGen.Detail = "generation id missing from installation metadata, fail closed"
		} else {
			current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
			if currentErr != nil {
				checkGen.Detail = fmt.Sprintf("generation current read failed: %v", currentErr)
			} else if current.GenerationID != generationID {
				checkGen.Detail = fmt.Sprintf("generation id mismatch: %s != %s", current.GenerationID, generationID)
			} else if current.ExtensionID != operation.ExtensionID {
				checkGen.Detail = fmt.Sprintf("current.json extension mismatch: %s != %s", current.ExtensionID, operation.ExtensionID)
			} else if current.ArtifactID != operation.ArtifactID {
				checkGen.Detail = fmt.Sprintf("current.json artifact mismatch: %s != %s", current.ArtifactID, operation.ArtifactID)
			} else if current.Version != operation.TargetVersion {
				checkGen.Detail = fmt.Sprintf("current.json version mismatch: %s != %s", current.Version, operation.TargetVersion)
			} else if current.OperationID != operation.OperationID {
				checkGen.Detail = fmt.Sprintf("current.json operation mismatch: %s != %s", current.OperationID, operation.OperationID)
			} else if current.FencingToken != operation.FencingToken {
				checkGen.Detail = fmt.Sprintf("current.json fencing token mismatch: %d != %d", current.FencingToken, operation.FencingToken)
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
		if r.container.PermissionRepository == nil || r.container.ScopeRepository == nil {
			checkPerm.Passed = false
			checkPerm.Detail = "permission or scope repository unavailable for uninstall verification, fail closed"
		} else {
			permClean := true
			requirements, reqErr := r.container.PermissionRepository.ListRequirements(ctx, domain.ExtensionID(operation.ExtensionID))
			if reqErr != nil {
				permClean = false
				checkPerm.Detail = fmt.Sprintf("permission requirements query failed: %v", reqErr)
			} else if len(requirements) > 0 {
				permClean = false
				checkPerm.Detail = "permission requirements still exist after uninstall"
			}
			grants, grantErr := r.container.PermissionRepository.ListGrants(ctx, domain.ExtensionID(operation.ExtensionID))
			if grantErr != nil {
				permClean = false
				checkPerm.Detail = fmt.Sprintf("permission grants query failed: %v", grantErr)
			} else if len(grants) > 0 {
				permClean = false
				checkPerm.Detail = "permission grants still exist after uninstall"
			}
			bindings, bindErr := r.container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(operation.ExtensionID))
			if bindErr != nil {
				permClean = false
				checkPerm.Detail = fmt.Sprintf("scope bindings query failed: %v", bindErr)
			} else if len(bindings) > 0 {
				permClean = false
				checkPerm.Detail = "scope bindings still exist after uninstall"
			}
			if permClean {
				checkPerm.Passed = true
			}
		}
	} else {
		if installErr != nil {
			checkPerm.Detail = "installation unavailable, cannot verify permissions"
		} else if r.container.PermissionRepository == nil || r.container.ScopeRepository == nil {
			checkPerm.Passed = false
			checkPerm.Detail = "permission or scope repository unavailable, fail closed"
		} else {
			_, reqErr := r.container.PermissionRepository.ListRequirements(ctx, domain.ExtensionID(operation.ExtensionID))
			_, grantErr := r.container.PermissionRepository.ListGrants(ctx, domain.ExtensionID(operation.ExtensionID))
			_, bindErr := r.container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(operation.ExtensionID))
			if reqErr != nil || grantErr != nil || bindErr != nil {
				checkPerm.Detail = "permission or scope repository query failed"
			} else {
				checkPerm.Passed = true
			}
		}
	}
	result.Checks = append(result.Checks, checkPerm)

	checkLease := PackageFinalGateCheck{Name: "valid_lease"}
	lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
	if leaseErr != nil {
		checkLease.Detail = fmt.Sprintf("no active lease: %v", leaseErr)
	} else if lease.OperationID != operationID {
		checkLease.Detail = fmt.Sprintf("lease held by different operation: %s", lease.OperationID)
	} else if lease.FencingToken != operation.FencingToken {
		checkLease.Detail = fmt.Sprintf("fencing token mismatch: lease=%d operation=%d", lease.FencingToken, operation.FencingToken)
	} else {
		leaseExpired := false
		if lease.LeaseExpiresAt != "" {
			expiresAt, parseErr := time.Parse(time.RFC3339Nano, lease.LeaseExpiresAt)
			if parseErr == nil && expiresAt.Before(time.Now().UTC()) {
				leaseExpired = true
			}
		}
		if leaseExpired {
			checkLease.Detail = "lease has expired"
		} else {
			checkLease.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkLease)

	checkQuarantine := PackageFinalGateCheck{Name: "quarantine_metadata_released"}
	blockingQm, qmErr := r.container.PackageRepository.GetBlockingQuarantineMetadata(ctx, operation.ExtensionID)
	if qmErr != nil {
		if IsPackageOperationError(qmErr, OperationErrNotFound) {
			checkQuarantine.Passed = true
		} else {
			checkQuarantine.Detail = fmt.Sprintf("quarantine metadata query failed: %v", qmErr)
		}
	} else {
		checkQuarantine.Detail = fmt.Sprintf("blocking quarantine in state %s for operation %s", blockingQm.State, blockingQm.OperationID)
	}
	result.Checks = append(result.Checks, checkQuarantine)

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

	if !isUninstall && (operation.OperationType == "update" || operation.OperationType == "rollback") {
		checkSnapshot := PackageFinalGateCheck{Name: "snapshot_integrity"}
		if r.container.PackageRepository == nil {
			checkSnapshot.Detail = "package repository unavailable for snapshot check"
		} else {
			rollbackPoint, rpErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion)
			if rpErr != nil {
				if !IsRepositoryErrorKind(rpErr, RepositoryErrorNotFound) {
					checkSnapshot.Detail = fmt.Sprintf("rollback point query failed (repository unavailable): %v", rpErr)
		} else {
				claims, claimsErr := parseOperationConfirmationClaims(operation)
				if claimsErr != nil {
					checkSnapshot.Detail = fmt.Sprintf("rollback point not found and no valid snapshot exemption claims (fail-closed): %v", claimsErr)
				} else {
					emptyPoint := PackageRollbackPoint{ExtensionID: operation.ExtensionID, SourceVersion: operation.FromVersion}
					currentReq := computeRollbackSnapshotRequirementFromPoint(emptyPoint)
					if isRollbackSnapshotExempt(currentReq, claims) {
						checkSnapshot.Passed = true
						checkSnapshot.Detail = fmt.Sprintf("exempt: rollback point absent, claims hash=%s current hash=%s no-data-change confirmed", claims.SnapshotRequirementHash, currentReq.RequirementHash)
					} else {
						checkSnapshot.Detail = fmt.Sprintf("rollback point not found and snapshot exemption claims invalid (fail-closed): claims hash=%s current hash=%s", claims.SnapshotRequirementHash, currentReq.RequirementHash)
					}
				}
			}
		} else {
			if validateErr := validatePackageSnapshot(rollbackPoint); validateErr != nil {
				checkSnapshot.Detail = fmt.Sprintf("snapshot validation failed: %v", validateErr)
			} else {
				claims, claimsErr := parseOperationConfirmationClaims(operation)
				if claimsErr != nil {
					checkSnapshot.Detail = fmt.Sprintf("snapshot present but no valid snapshot exemption claims (fail-closed): %v", claimsErr)
				} else {
					currentReq := computeRollbackSnapshotRequirementFromPoint(rollbackPoint)
					if isRollbackSnapshotExempt(currentReq, claims) {
						checkSnapshot.Passed = true
						checkSnapshot.Detail = fmt.Sprintf("exempt: snapshot integrity confirmed via claims, claims hash=%s current hash=%s", claims.SnapshotRequirementHash, currentReq.RequirementHash)
					} else {
						checkSnapshot.Detail = fmt.Sprintf("snapshot exemption claims mismatch (fail-closed): claims hash=%s current hash=%s", claims.SnapshotRequirementHash, currentReq.RequirementHash)
					}
				}
			}
		}
		}
		result.Checks = append(result.Checks, checkSnapshot)
	}

	checkVersion := PackageFinalGateCheck{Name: "version_record_consistent"}
	if isUninstall {
		checkVersion.Passed = true
	} else if r.container.PackageRepository == nil {
		checkVersion.Passed = false
		checkVersion.Detail = "package repository unavailable for version record check, fail closed"
	} else if installErr != nil {
		checkVersion.Passed = false
		checkVersion.Detail = "installation unavailable, cannot verify version record"
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
		} else if versionRecord.VersionState != "current" {
			checkVersion.Passed = false
			checkVersion.Detail = fmt.Sprintf("version record state is %s, expected current", versionRecord.VersionState)
		} else if versionRecord.GenerationID != operation.TargetGeneration {
			checkVersion.Passed = false
			checkVersion.Detail = fmt.Sprintf("version record generation mismatch: %s != %s", versionRecord.GenerationID, operation.TargetGeneration)
		} else {
			allVersions, listErr := r.container.PackageRepository.ListPackageVersions(ctx, operation.ExtensionID)
			if listErr != nil {
				checkVersion.Passed = false
				checkVersion.Detail = fmt.Sprintf("version list query failed: %v", listErr)
			} else {
				currentCount := 0
				for _, v := range allVersions {
					if v.VersionState == "current" {
						currentCount++
					}
				}
				if currentCount != 1 {
					checkVersion.Passed = false
					checkVersion.Detail = fmt.Sprintf("expected exactly 1 current version, found %d", currentCount)
				} else {
					checkVersion.Passed = true
				}
			}
		}
	}
	result.Checks = append(result.Checks, checkVersion)

	allPassed := true
	var findings []PackageFinalGateFinding
	for _, check := range result.Checks {
		if !check.Passed {
			allPassed = false
			findings = append(findings, PackageFinalGateFinding{
				FindingID:   "gate-finding-" + uuid.NewString(),
				OperationID: operationID,
				ExtensionID: operation.ExtensionID,
				FindingType: check.Name,
				Severity:    "error",
				Expected:    "passed",
				Actual:      check.Detail,
				DetectedAt:  result.VerifiedAt,
			})
		}
	}
	result.Passed = allPassed
	result.Findings = findings

	if persistErr := r.recordFinalGateResult(ctx, operationID, result, guard); persistErr != nil {
		return result, fmt.Errorf("kernel: final gate finding persistence failed: %w", persistErr)
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

func (r *Runtime) recordFinalGateResult(ctx context.Context, operationID string, result PackageFinalGateResult, guard PackageWriteGuard) error {
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
	if guard.IsZero() {
		operation, _ := r.container.PackageRepository.getAuthoritativeOperationByID(ctx, operationID)
		if operation.OperationID != "" {
			lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
			if leaseErr == nil && lease.OperationID == operationID {
				guard = PackageWriteGuard{ExtensionID: operation.ExtensionID, FencingToken: lease.FencingToken}
			}
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

func computeRollbackSnapshotRequirement(point PackageRollbackPoint) RollbackSnapshotRequirement {
	return computeRollbackSnapshotRequirementFromPoint(point)
}

func isRollbackSnapshotExempt(req RollbackSnapshotRequirement, claims packageConfirmationClaims) bool {
	if !req.NoDataChange {
		return false
	}
	if claims.SnapshotRequirementHash == "" || claims.SnapshotRequirementHash != req.RequirementHash {
		return false
	}
	return true
}

func parseOperationConfirmationClaims(operation PackageOperationRecord) (packageConfirmationClaims, error) {
	if operation.ConfirmationClaimsJSON == "" || operation.ConfirmationClaimsJSON == "{}" {
		return packageConfirmationClaims{}, fmt.Errorf("kernel: confirmation claims missing from operation")
	}
	var claims packageConfirmationClaims
	if err := json.Unmarshal([]byte(operation.ConfirmationClaimsJSON), &claims); err != nil {
		return packageConfirmationClaims{}, fmt.Errorf("kernel: confirmation claims corrupt: %w", err)
	}
	if claims.ArtifactID == "" || claims.PreviewHash == "" {
		return packageConfirmationClaims{}, fmt.Errorf("kernel: confirmation claims incomplete")
	}
	return claims, nil
}

func validateUninstallArtifactDeletion(ctx context.Context, repo *PackageRepository, operationID, artifactID string) bool {
	if artifactID == "" {
		return false
	}
	steps, err := repo.ListOperationSteps(ctx, operationID)
	if err != nil {
		return false
	}
	for _, step := range steps {
		if step.Status != "completed" || step.StepName != "remove_artifact" {
			continue
		}
		if step.ResultJSON == "" || step.ResultJSON == "{}" {
			return false
		}
		var stepResult RemoveArtifactStepResult
		if json.Unmarshal([]byte(step.ResultJSON), &stepResult) != nil {
			return false
		}
		if stepResult.ArtifactID != artifactID {
			return false
		}
		if stepResult.ArtifactPolicy != ArtifactPolicyDeleteArtifact {
			return false
		}
		if !stepResult.Deleted {
			return false
		}
		if stepResult.RemainingRefs != 0 {
			return false
		}
		return true
	}
	return false
}
