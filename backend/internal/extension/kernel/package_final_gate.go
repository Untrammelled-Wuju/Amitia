package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageFinalGateResult struct {
	Passed                       bool                               `json:"passed"`
	OperationID                  string                             `json:"operationId"`
	OperationType                string                             `json:"operationType"`
	ExtensionID                  string                             `json:"extensionId"`
	Version                      string                             `json:"version"`
	Mode                         string                             `json:"mode,omitempty"`
	ClaimsVerified               bool                               `json:"claimsVerified"`
	PolicyVersionVerified        bool                               `json:"policyVersionVerified"`
	ConfirmedItemsVerified       bool                               `json:"confirmedItemsVerified"`
	PreviewIdentityVerified      bool                               `json:"previewIdentityVerified"`
	ArtifactPolicyVerified       bool                               `json:"artifactPolicyVerified"`
	SnapshotRequirementVerified  bool                               `json:"snapshotRequirementVerified"`
	StepIntegrityVerified        bool                               `json:"stepIntegrityVerified"`
	SnapshotDecision             *PackageSnapshotFinalGateDecision  `json:"snapshotDecision,omitempty"`
	RestoredIdentityEvidence     *UninstallRestoredIdentityEvidence `json:"restoredIdentityEvidence,omitempty"`
	RestoredIdentityEvidenceHash string                             `json:"restoredIdentityEvidenceHash,omitempty"`
	Checks                       []PackageFinalGateCheck            `json:"checks"`
	Findings                     []PackageFinalGateFinding          `json:"findings,omitempty"`
	VerifiedAt                   string                             `json:"verifiedAt"`
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

type PackageSnapshotFinalGateDecision struct {
	Required         bool     `json:"required"`
	SnapshotPresent  bool     `json:"snapshotPresent"`
	SnapshotVerified bool     `json:"snapshotVerified"`
	ExemptVerified   bool     `json:"exemptVerified"`
	RequirementHash  string   `json:"requirementHash,omitempty"`
	SnapshotHash     string   `json:"snapshotHash,omitempty"`
	Reasons          []string `json:"reasons,omitempty"`
}

func (r *PackageRepository) ListOperationSteps(ctx context.Context, operationID string) ([]PackageOperationStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, operation_id, step_name, step_order, status,
		attempt_count, result_json, error_code, started_at, completed_at, stable_generation,
		target_generation, current_pointer_json, input_hash, result_hash, updated_at, cas_version
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
			&step.StableGeneration, &step.TargetGeneration, &step.CurrentPointerJSON,
			&step.InputHash, &step.ResultHash, &step.UpdatedAt, &step.CASVersion); err != nil {
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

	checkClaims := PackageFinalGateCheck{Name: "claims_verified"}
	{
		if operation.ConfirmationClaimsJSON == "" && operation.ConfirmationsJSON == "" {
			checkClaims.Detail = "confirmation claims not persisted"
		} else {
			claims, parseErr := parseAndValidateOperationConfirmationClaims(operation, packagePolicyVersion)
			if parseErr != nil {
				checkClaims.Detail = fmt.Sprintf("claims validation failed: %v", parseErr)
			} else {
				claimsRecomputeErr := recomputeAndVerifyRequiredConfirmations(claims, operation)
				if claimsRecomputeErr != nil {
					checkClaims.Detail = fmt.Sprintf("required confirmations incomplete: %v", claimsRecomputeErr)
				} else {
					checkClaims.Passed = true
				}
			}
		}
	}
	result.Checks = append(result.Checks, checkClaims)

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
		if operation.ArtifactID == "" {
			checkArtifact.Passed = true
		} else {
			claims, claimsErr := parseOperationConfirmationClaims(operation)
			if claimsErr != nil {
				checkArtifact.Detail = fmt.Sprintf("confirmation claims unavailable for artifact verification: %v", claimsErr)
			} else {
				expectedPolicy := claims.ArtifactPolicy
				artifact, artifactErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
				switch expectedPolicy {
				case ArtifactPolicyDeleteArtifact:
					if artifactErr == nil {
						if artifact.RetentionState != "deleted" {
							checkArtifact.Detail = fmt.Sprintf("delete policy: retention state is %s, expected deleted", artifact.RetentionState)
						} else if artifact.DeletedAt == "" {
							checkArtifact.Detail = "delete policy: deleted_at is empty"
						} else if artifact.InstalledPath != "" {
							if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
								checkArtifact.Detail = "delete policy: installed path still exists"
							} else if !validateArtifactPolicyStepResult(ctx, r.container.PackageRepository, operationID, operation.ArtifactID, expectedPolicy) {
								checkArtifact.Detail = "delete policy: remove_artifact step evidence incomplete"
							} else {
								refCount, refErr := r.container.PackageRepository.CountActiveArtifactReferences(ctx, operation.ArtifactID)
								if refErr != nil {
									checkArtifact.Detail = fmt.Sprintf("delete policy: reference check failed: %v", refErr)
								} else if refCount > 0 {
									checkArtifact.Detail = fmt.Sprintf("delete policy: artifact still has %d active references", refCount)
								} else {
									checkArtifact.Passed = true
								}
							}
						} else if !validateArtifactPolicyStepResult(ctx, r.container.PackageRepository, operationID, operation.ArtifactID, expectedPolicy) {
							checkArtifact.Detail = "delete policy: remove_artifact step evidence incomplete"
						} else {
							refCount, refErr := r.container.PackageRepository.CountActiveArtifactReferences(ctx, operation.ArtifactID)
							if refErr != nil {
								checkArtifact.Detail = fmt.Sprintf("delete policy: reference check failed: %v", refErr)
							} else if refCount > 0 {
								checkArtifact.Detail = fmt.Sprintf("delete policy: artifact still has %d active references", refCount)
							} else {
								checkArtifact.Passed = true
							}
						}
					} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorNotFound) {
						if !validateArtifactPolicyStepResult(ctx, r.container.PackageRepository, operationID, operation.ArtifactID, expectedPolicy) {
							checkArtifact.Detail = "delete policy: artifact not found and remove_artifact step evidence incomplete"
						} else {
							checkArtifact.Passed = true
						}
					} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorUnavailable) {
						checkArtifact.Detail = fmt.Sprintf("delete policy: artifact repository unavailable, fail closed: %v", artifactErr)
					} else if IsRepositoryErrorKind(artifactErr, RepositoryErrorCorrupt) {
						checkArtifact.Detail = fmt.Sprintf("delete policy: artifact repository corrupt, fail closed: %v", artifactErr)
					} else {
						checkArtifact.Detail = fmt.Sprintf("delete policy: artifact query failed, fail closed: %v", artifactErr)
					}
				case ArtifactPolicyRetainArtifact, ArtifactPolicyRetainForRollback, ArtifactPolicyRetainForExport:
					if artifactErr != nil {
						checkArtifact.Detail = fmt.Sprintf("retain policy: artifact unavailable: %v", artifactErr)
					} else if artifact.RetentionState == "deleted" || artifact.DeletedAt != "" {
						checkArtifact.Detail = "retain policy: artifact is in deleted state"
					} else if !validateArtifactPolicyStepResult(ctx, r.container.PackageRepository, operationID, operation.ArtifactID, expectedPolicy) {
						checkArtifact.Detail = "retain policy: remove_artifact step evidence incomplete"
					} else {
						var requiredRefType string
						switch expectedPolicy {
						case ArtifactPolicyRetainForRollback:
							requiredRefType = ArtifactReferenceRollbackPoint
						case ArtifactPolicyRetainForExport:
							requiredRefType = ArtifactReferenceExportLease
						}
						if requiredRefType != "" {
							var refOwnerID string
							var refDetail string
							if requiredRefType == ArtifactReferenceRollbackPoint {
								rp, rpErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.TargetVersion)
								if rpErr != nil {
									checkArtifact.Detail = fmt.Sprintf("retain policy: cannot resolve rollback point for reference: %v", rpErr)
									break
								}
								if rp.ArtifactID != operation.ArtifactID {
									checkArtifact.Detail = fmt.Sprintf("retain policy: rollback point artifact_id mismatch: point=%s operation=%s", rp.ArtifactID, operation.ArtifactID)
									break
								}
								refOwnerID = rp.RollbackPointID
								refDetail = fmt.Sprintf("rollbackPoint=%s", refOwnerID)
							} else if requiredRefType == ArtifactReferenceExportLease {
								checkArtifact.Detail = "PACKAGE_EXPORT_RETENTION_UNSUPPORTED: export retention not supported without export model"
								break
							} else {
								refOwnerID = operation.ExtensionID
								refDetail = fmt.Sprintf("owner=%s", refOwnerID)
							}
							refs, listErr := r.container.PackageRepository.ListActiveArtifactReferences(ctx, operation.ArtifactID, requiredRefType, refOwnerID)
							if listErr != nil {
								checkArtifact.Detail = fmt.Sprintf("retain policy: list active references failed: %v", listErr)
							} else if len(refs) == 0 {
								checkArtifact.Detail = fmt.Sprintf("retain policy: no active %s reference with %s (reference proof required)", requiredRefType, refDetail)
							} else if len(refs) > 1 {
								checkArtifact.Detail = fmt.Sprintf("retain policy: expected exactly 1 active %s reference, found %d (integrity error)", requiredRefType, len(refs))
							} else {
								ref := refs[0]
								if ref.ArtifactID != operation.ArtifactID || ref.ReferenceType != requiredRefType || ref.ReferenceOwnerID != refOwnerID {
									checkArtifact.Detail = "retain policy: reference identity mismatch"
								} else if ref.ReleasedAt != "" {
									checkArtifact.Detail = "retain policy: reference has been released"
								} else if ref.ExpiresAt != "" && ref.ExpiresAt < time.Now().UTC().Format(time.RFC3339Nano) {
									checkArtifact.Detail = "retain policy: reference expired"
								} else {
									checkArtifact.Detail = fmt.Sprintf("retain policy: verified exactly 1 active %s reference %s", requiredRefType, refDetail)
									checkArtifact.Passed = true
								}
							}
						} else {
							checkArtifact.Passed = true
						}
					}
				default:
					checkArtifact.Detail = fmt.Sprintf("unknown artifact policy in claims: %s", expectedPolicy)
				}
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

	checkStepIntegrity := PackageFinalGateCheck{Name: "step_result_integrity"}
	if stepErr != nil {
		checkStepIntegrity.Detail = fmt.Sprintf("operation steps unavailable: %v", stepErr)
	} else {
		tamperedSteps := []string{}
		for _, step := range steps {
			if step.StepOrder == 999 || step.ResultJSON == "" {
				continue
			}
			if step.ResultHash == "" {
				tamperedSteps = append(tamperedSteps, step.StepName+" (missing hash)")
				continue
			}
			recomputed := fmt.Sprintf("%x", sha256.Sum256([]byte(step.ResultJSON)))
			if recomputed != step.ResultHash {
				tamperedSteps = append(tamperedSteps, step.StepName)
			}
		}
		if len(tamperedSteps) > 0 {
			checkStepIntegrity.Detail = fmt.Sprintf("step result hash mismatch (tampered): %s", strings.Join(tamperedSteps, ", "))
		} else {
			checkStepIntegrity.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkStepIntegrity)

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
		decision := &PackageSnapshotFinalGateDecision{}
		if r.container.PackageRepository == nil {
			checkSnapshot.Detail = "package repository unavailable for snapshot check"
		} else {
			claims, claimsErr := parseOperationConfirmationClaims(operation)
			var snapshotPresent bool
			if claimsErr != nil {
				snapshotPresent = false
				decision.SnapshotPresent = snapshotPresent
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("claims unavailable: %v", claimsErr))
				rollbackPoint, rpErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion)
				if rpErr == nil {
					decision.SnapshotPresent = true
					decision.SnapshotHash = rollbackPoint.SnapshotHash
				}
				decision.Required = true
				if decision.SnapshotPresent {
					if validateErr := validatePackageSnapshot(rollbackPoint); validateErr != nil {
						decision.Reasons = append(decision.Reasons, fmt.Sprintf("snapshot validation failed: %v", validateErr))
						checkSnapshot.Detail = fmt.Sprintf("snapshot present but validation failed: %v", validateErr)
					} else {
						decision.SnapshotVerified = true
						checkSnapshot.Passed = true
						checkSnapshot.Detail = fmt.Sprintf("snapshot present and verified (claims unavailable, fail-closed): hash=%s", rollbackPoint.SnapshotHash)
					}
				} else {
					checkSnapshot.Detail = "claims unavailable and no rollback point found (fail-closed)"
				}
			} else {
				requirement := r.computeRollBackSnapshotRequirementFromClaims(ctx, operation, claims)
				decision.RequirementHash = requirement.RequirementHash
				rollbackPoint, rpErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion)
				snapshotPresent = rpErr == nil
				decision.SnapshotPresent = snapshotPresent

				switch {
				case requirement.Required:
					decision.Required = true
					if rpErr != nil {
						decision.Reasons = append(decision.Reasons, "snapshot required but rollback point not found")
						checkSnapshot.Detail = "snapshot required but rollback point not found (fail-closed)"
					} else if snapshotPresent {
						decision.SnapshotHash = rollbackPoint.SnapshotHash
						if validateErr := validatePackageSnapshot(rollbackPoint); validateErr != nil {
							decision.Reasons = append(decision.Reasons, fmt.Sprintf("snapshot validation failed: %v", validateErr))
							checkSnapshot.Detail = fmt.Sprintf("snapshot present but validation failed: %v", validateErr)
						} else {
							decision.SnapshotVerified = true
							checkSnapshot.Passed = true
							checkSnapshot.Detail = fmt.Sprintf("snapshot present and verified: hash=%s", rollbackPoint.SnapshotHash)
						}
					}
				case !requirement.Required && snapshotPresent:
					decision.Required = false
					decision.SnapshotHash = rollbackPoint.SnapshotHash
					if validateErr := validatePackageSnapshot(rollbackPoint); validateErr != nil {
						decision.Reasons = append(decision.Reasons, fmt.Sprintf("optional snapshot validation failed: %v", validateErr))
						checkSnapshot.Detail = fmt.Sprintf("optional snapshot present but validation failed: %v", validateErr)
					} else {
						decision.SnapshotVerified = true
						checkSnapshot.Passed = true
						checkSnapshot.Detail = fmt.Sprintf("optional snapshot present and verified: hash=%s", rollbackPoint.SnapshotHash)
					}
				case !requirement.Required && !snapshotPresent:
					decision.Required = false
					if isRollbackSnapshotExempt(requirement, claims) {
						decision.ExemptVerified = true
						checkSnapshot.Passed = true
						checkSnapshot.Detail = "snapshot exemption verified: no data change, requirement hash and policy hash consistent"
					} else {
						decision.Reasons = append(decision.Reasons, "snapshot exemption verification failed: requirement/policy hash mismatch or missing claims")
						checkSnapshot.Detail = "snapshot not present and exemption verification failed"
					}
				}
			}
		}
		result.SnapshotDecision = decision
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

	checkResourceRestore := PackageFinalGateCheck{Name: "resource_restore_integrity"}
	if r.container.ResourceSnapshotStore == nil {
		checkResourceRestore.Detail = "resource snapshot store unavailable"
	} else if r.container.PackageRepository == nil {
		checkResourceRestore.Detail = "package repository unavailable"
	} else {
		rp, rpErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion)
		if rpErr != nil {
			checkResourceRestore.Passed = true
		} else if rp.ResourceSnapshotJSON == "" {
			checkResourceRestore.Passed = true
		} else if verifyErr := r.container.ResourceSnapshotStore.VerifyResourceSnapshotEntries(ctx, rp.ResourceSnapshotJSON); verifyErr != nil {
			checkResourceRestore.Detail = fmt.Sprintf("resource snapshot verification failed: %v", verifyErr)
		} else if qErr := r.container.ResourceSnapshotStore.VerifyNoActiveQuarantine(ctx, operation.OperationID); qErr != nil {
			checkResourceRestore.Detail = fmt.Sprintf("active quarantine detected: %v", qErr)
		} else {
			checkResourceRestore.Passed = true
		}
	}
	result.Checks = append(result.Checks, checkResourceRestore)

	checkUserDataRestore := PackageFinalGateCheck{Name: "user_data_restore_integrity"}
	if r.container.UserDataSnapshotStore == nil {
		checkUserDataRestore.Detail = "user data snapshot store unavailable"
	} else if r.container.PackageRepository == nil {
		checkUserDataRestore.Detail = "package repository unavailable"
	} else {
		rpUd, rpUdErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion)
		if rpUdErr != nil {
			checkUserDataRestore.Passed = true
		} else if rpUd.UserDataMigrationStateJSON == "" {
			checkUserDataRestore.Passed = true
		} else {
			restoreOperationID := rpUd.SourceOperationID
			if restoreOperationID == "" {
				restoreOperationID = "restore-" + rpUd.RollbackPointID
			}
			if verifyUdErr := r.container.UserDataSnapshotStore.VerifyUserDataRestore(ctx, restoreOperationID, rpUd.UserDataMigrationStateJSON); verifyUdErr != nil {
				checkUserDataRestore.Detail = fmt.Sprintf("user data restore verification failed: %v", verifyUdErr)
			} else {
				checkUserDataRestore.Passed = true
			}
		}
	}
	result.Checks = append(result.Checks, checkUserDataRestore)

	result.ClaimsVerified = findCheckPassed(result.Checks, "claims_verified")
	result.PolicyVersionVerified = findCheckPassed(result.Checks, "claims_verified")
	result.ConfirmedItemsVerified = findCheckPassed(result.Checks, "claims_verified")
	result.PreviewIdentityVerified = findCheckPassed(result.Checks, "claims_verified")
	result.ArtifactPolicyVerified = findCheckPassed(result.Checks, "artifact_path_absent") || findCheckPassed(result.Checks, "artifact_path_and_hash")
	result.SnapshotRequirementVerified = findCheckPassed(result.Checks, "snapshot_integrity")
	result.StepIntegrityVerified = findCheckPassed(result.Checks, "step_result_integrity")

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

func recomputeAndVerifyRequiredConfirmations(claims PackageConfirmationClaims, operation PackageOperationRecord) error {
	seen := make(map[string]bool, len(claims.ConfirmedItems))
	for _, item := range claims.ConfirmedItems {
		if item == "" {
			continue
		}
		seen[item] = true
	}
	for key, val := range claims.Confirmations {
		if val && !seen[key] {
			return fmt.Errorf("%w: confirmations key %s is not in confirmedItems", ErrPackageConfirmationItemsMismatch, key)
		}
		if !val && seen[key] {
			return fmt.Errorf("%w: confirmedItems contains %s but confirmations[%s] is false", ErrPackageConfirmationItemsMismatch, key, key)
		}
	}
	required := computeRequiredConfirmationsForOperation(operation.OperationType, claims)
	for _, req := range required {
		if !seen[req] && !claims.Confirmations[req] {
			return fmt.Errorf("%w: required confirmation %q not present", ErrPackageConfirmationItemsMissing, req)
		}
	}
	return nil
}

func computeRequiredConfirmationsForOperation(operationType string, claims PackageConfirmationClaims) []string {
	switch operationType {
	case OpInstall:
		required := []string{}
		if claims.DeveloperSessionID != "" {
			required = append(required, "confirm.unsigned_dev")
		}
		sort.Strings(required)
		return required
	case OpUpdate:
		required := []string{"confirm.update"}
		if claims.MigrationPlanHash != "" {
			required = append(required, "confirm.config_migration")
		}
		sort.Strings(required)
		return required
	case OpRollback:
		required := []string{"confirm.rollback"}
		if claims.MigrationPlanHash != "" {
			required = append(required, "confirm.rollback.migration_reverse")
		}
		sort.Strings(required)
		return required
	case OpUninstall:
		return []string{"confirm.uninstall"}
	default:
		return []string{}
	}
}

func computeRollbackSnapshotRequirement(point PackageRollbackPoint) RollbackSnapshotRequirement {
	return computeRollbackSnapshotRequirementFromPoint(point)
}

func (r *Runtime) computeRollBackSnapshotRequirementFromClaims(ctx context.Context, operation PackageOperationRecord, claims PackageConfirmationClaims) RollbackSnapshotRequirement {
	input := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	corrupt := false

	var rollbackPoint *PackageRollbackPoint
	if r.container.PackageRepository != nil && operation.FromVersion != "" {
		if rp, err := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.FromVersion); err == nil {
			rollbackPoint = &rp
		}
	}

	if rollbackPoint != nil {
		if rollbackPoint.ConfigSnapshotJSON != "" {
			input.ConfigBeforeHash = packageSnapshotDigest([]byte(rollbackPoint.ConfigSnapshotJSON))
		}
		if rollbackPoint.ResourceSnapshotJSON != "" {
			input.ResourceBeforeTreeHash = packageSnapshotDigest([]byte(rollbackPoint.ResourceSnapshotJSON))
		}
		if rollbackPoint.UserDataMigrationStateJSON != "" {
			input.UserDataBeforeHash = packageSnapshotDigest([]byte(rollbackPoint.UserDataMigrationStateJSON))
		}
		if rollbackPoint.MigrationStateSnapshotJSON != "" {
			var migrationState packageMigrationStateSnapshot
			if json.Unmarshal([]byte(rollbackPoint.MigrationStateSnapshotJSON), &migrationState) != nil {
				corrupt = true
			} else {
				if migrationState.Mode != "" && migrationState.Mode != "none" {
					input.MigrationDefinitions = migrationState.Definitions
				}
				for i := range migrationState.Operations {
					input.MigrationOperations = append(input.MigrationOperations, migrationState.Operations[i].Operation)
				}
			}
		}
	}

	if r.container.ContributionRepository != nil && r.container.PermissionRepository != nil && r.container.ScopeRepository != nil {
		contributions, _ := r.container.ContributionRepository.ListContributions(ctx, domain.ExtensionID(operation.ExtensionID))
		requirements, _ := r.container.PermissionRepository.ListRequirements(ctx, domain.ExtensionID(operation.ExtensionID))
		grants, _ := r.container.PermissionRepository.ListGrants(ctx, domain.ExtensionID(operation.ExtensionID))
		scopeBindings, _ := r.container.ScopeRepository.ListBindings(ctx, domain.ExtensionID(operation.ExtensionID))

		sort.Slice(contributions, func(i, j int) bool { return string(contributions[i].ID) < string(contributions[j].ID) })
		sort.Slice(requirements, func(i, j int) bool { return requirements[i].PermissionName < requirements[j].PermissionName })
		sort.Slice(grants, func(i, j int) bool { return grants[i].PermissionName < grants[j].PermissionName })
		sort.Slice(scopeBindings, func(i, j int) bool {
			if scopeBindings[i].ScopeType != scopeBindings[j].ScopeType {
				return scopeBindings[i].ScopeType < scopeBindings[j].ScopeType
			}
			return scopeBindings[i].ScopeID < scopeBindings[j].ScopeID
		})

		afterConfig := packageConfigSnapshot{
			Contributions: contributions,
			Permissions: packageConfigPermissionSnapshot{
				Requirements: requirements,
				Grants:       grants,
			},
			ScopeBindings: scopeBindings,
			SchemaVersion: packageConfigSnapshotSchemaVersion,
		}
		if configJSON, err := json.Marshal(afterConfig); err == nil {
			input.ConfigAfterHash = packageSnapshotDigest(configJSON)
		}
	}

	if r.container.ResourceRepository != nil {
		resources, _ := r.container.ResourceRepository.ListResources(ctx, domain.ExtensionID(operation.ExtensionID))
		sort.Slice(resources, func(i, j int) bool { return resources[i].ResourceID < resources[j].ResourceID })
		afterResource := packageResourceSnapshot{Entries: make([]packageResourceSnapshotEntry, 0, len(resources))}
		for _, resource := range resources {
			resource.Metadata, _ = sanitizePackageSnapshotMap(resource.Metadata)
			originalPath := resource.Reference
			absOriginal, absErr := filepath.Abs(originalPath)
			if absErr != nil {
				continue
			}
			info, statErr := os.Lstat(absOriginal)
			if statErr != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				corrupt = true
				continue
			}
			file, openErr := os.Open(absOriginal)
			if openErr != nil {
				continue
			}
			hasher := sha256.New()
			if _, copyErr := io.Copy(hasher, file); copyErr != nil {
				file.Close()
				continue
			}
			file.Close()
			afterResource.Entries = append(afterResource.Entries, packageResourceSnapshotEntry{
				Resource:    resource,
				ContentHash: "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
			})
		}
		if resourceJSON, err := json.Marshal(afterResource); err == nil {
			input.ResourceAfterTreeHash = packageSnapshotDigest(resourceJSON)
		}
	}

	req := ComputeRollbackSnapshotRequirement(input)
	if corrupt && req.NoDataChange {
		req.Required = true
		req.NoDataChange = false
		req.Reason = "live state validation failure, fail-closed"
		req.RequirementHash = computeSnapshotRequirementHash(req)
	}
	return req
}

func isRollbackSnapshotExempt(req RollbackSnapshotRequirement, claims PackageConfirmationClaims) bool {
	if !req.NoDataChange {
		return false
	}
	if claims.SnapshotRequirementHash == "" || claims.SnapshotRequirementHash != req.RequirementHash {
		return false
	}
	if claims.SecurityPolicyHash == "" || claims.SecurityPolicyHash != computeSecurityPolicyHash() {
		return false
	}
	return true
}

func parseOperationConfirmationClaims(operation PackageOperationRecord) (PackageConfirmationClaims, error) {
	if operation.ConfirmationClaimsJSON != "" && operation.ConfirmationClaimsJSON != "{}" {
		return parseAndValidateOperationConfirmationClaims(operation, packagePolicyVersion)
	}
	if operation.ConfirmationsJSON != "" && operation.ConfirmationsJSON != "{}" {
		var legacy packageConfirmationClaims
		if err := json.Unmarshal([]byte(operation.ConfirmationsJSON), &legacy); err != nil {
			return PackageConfirmationClaims{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, fmt.Errorf("%w: %v", ErrPackageConfirmationClaimsInvalid, err))
		}
		return PackageConfirmationClaims{
			ExtensionID:             legacy.ExtensionID,
			ArtifactID:              legacy.ArtifactID,
			ArtifactPolicy:          legacy.ArtifactPolicy,
			CurrentVersionID:        legacy.CurrentVersionID,
			CurrentGenerationID:     legacy.CurrentGenerationID,
			SnapshotRequirementHash: legacy.SnapshotRequirementHash,
			PolicyVersion:           legacy.PolicyVersion,
			SecurityPolicyHash:      legacy.SecurityPolicyHash,
		}, nil
	}
	return PackageConfirmationClaims{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 403, ErrPackageConfirmationClaimsInvalid)
}

func findCheckPassed(checks []PackageFinalGateCheck, name string) bool {
	for _, c := range checks {
		if c.Name == name {
			return c.Passed
		}
	}
	return false
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

func validateArtifactPolicyStepResult(ctx context.Context, repo *PackageRepository, operationID, artifactID string, expectedPolicy ArtifactPolicy) bool {
	if artifactID == "" || operationID == "" {
		fmt.Fprintf(os.Stderr, "DEBUG_FG: empty artifactID or operationID\n")
		return false
	}
	steps, err := repo.ListOperationSteps(ctx, operationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG_FG: list steps failed: %v\n", err)
		return false
	}
	if len(steps) == 0 {
		fmt.Fprintf(os.Stderr, "DEBUG_FG: no steps found for operationID=%s\n", operationID)
	}
	for _, step := range steps {
		fmt.Fprintf(os.Stderr, "DEBUG_FG: checking step name=%s status=%s hasResult=%v\n", step.StepName, step.Status, step.ResultJSON != "")
		if step.Status != "completed" || step.StepName != "remove_artifact" {
			continue
		}
		if step.ResultJSON == "" || step.ResultJSON == "{}" {
			fmt.Fprintf(os.Stderr, "DEBUG_FG: empty result json for remove_artifact step\n")
			return false
		}
		var stepResult RemoveArtifactStepResult
		if json.Unmarshal([]byte(step.ResultJSON), &stepResult) != nil {
			fmt.Printf("DEBUG_FG: unmarshal failed json=%s\n", step.ResultJSON)
			return false
		}
		if stepResult.ArtifactID != artifactID {
			fmt.Fprintf(os.Stderr, "DEBUG_FG: artifactID mismatch step=%s op=%s\n", stepResult.ArtifactID, artifactID)
			return false
		}
		if stepResult.ArtifactPolicy != expectedPolicy {
			fmt.Fprintf(os.Stderr, "DEBUG_FG: policy mismatch step=%s expected=%s\n", stepResult.ArtifactPolicy, expectedPolicy)
			return false
		}
		switch expectedPolicy {
		case ArtifactPolicyDeleteArtifact:
			if !stepResult.Deleted {
				return false
			}
			if stepResult.RetentionState != "deleted" {
				return false
			}
			if stepResult.RemainingRefs != 0 {
				return false
			}
		case ArtifactPolicyRetainArtifact, ArtifactPolicyRetainForRollback, ArtifactPolicyRetainForExport:
			if !stepResult.Retained {
				fmt.Fprintf(os.Stderr, "DEBUG_FG: retain branch but Retained=false json=%s\n", step.ResultJSON)
				return false
			}
			if stepResult.RetentionState == "deleted" {
				fmt.Fprintf(os.Stderr, "DEBUG_FG: retain branch but state=deleted json=%s\n", step.ResultJSON)
				return false
			}
		default:
			fmt.Printf("DEBUG_FG: unknown policy=%s\n", expectedPolicy)
			return false
		}
		if stepResult.EvidenceHash == "" {
			fmt.Fprintf(os.Stderr, "DEBUG_FG: empty evidence hash, json=%s\n", step.ResultJSON)
			return false
		}
		var deletedAt time.Time
		if !stepResult.DeletedAt.IsZero() {
			deletedAt = stepResult.DeletedAt
		}
		recomputedEvidence := computeArtifactStepEvidenceHash(RemoveArtifactStepResult{
			ArtifactID:         stepResult.ArtifactID,
			ExtensionID:        stepResult.ExtensionID,
			ArtifactPolicy:     stepResult.ArtifactPolicy,
			Deleted:            stepResult.Deleted,
			Retained:           stepResult.Retained,
			RetentionState:     stepResult.RetentionState,
			RemainingRefs:      stepResult.RemainingRefs,
			DeletedAt:          deletedAt,
			EvidenceHashBefore: stepResult.EvidenceHashBefore,
			EvidenceHashAfter:  stepResult.EvidenceHashAfter,
		})
		if recomputedEvidence != stepResult.EvidenceHash {
			fmt.Fprintf(os.Stderr, "DEBUG_FG: evidence mismatch stored=%s recomputed=%s json=%s\n", stepResult.EvidenceHash, recomputedEvidence, step.ResultJSON)
			return false
		}
		return true
	}
	return false
}
