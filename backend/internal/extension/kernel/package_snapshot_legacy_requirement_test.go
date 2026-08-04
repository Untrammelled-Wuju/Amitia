package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Deprecated: kept for tests only, use ComputePackageSnapshotRequirement in production.
func ComputeRollbackSnapshotRequirement(input RollbackSnapshotRequirementInput) RollbackSnapshotRequirement {
	req := RollbackSnapshotRequirement{
		ManifestNoDataChange: input.ManifestNoDataChange,
	}

	req.ConfigChanged = input.ConfigBeforeHash != "" && input.ConfigAfterHash != "" && input.ConfigBeforeHash != input.ConfigAfterHash
	req.ResourcesChanged = (input.ResourceBeforeTreeHash != "" && input.ResourceAfterTreeHash != "" && input.ResourceBeforeTreeHash != input.ResourceAfterTreeHash) ||
		len(input.ResourceSetDiff.Added) > 0 || len(input.ResourceSetDiff.Removed) > 0 || len(input.ResourceSetDiff.Changed) > 0
	req.UserDataChanged = input.UserDataBeforeHash != "" && input.UserDataAfterHash != "" && input.UserDataBeforeHash != input.UserDataAfterHash
	req.MigrationPlanPresent = input.MigrationPlan != nil
	req.MigrationDefinitionPresent = len(input.MigrationDefinitions) > 0
	req.MigrationOperationPresent = len(input.MigrationOperations) > 0

	configSourceMissing := (input.ConfigBeforeHash != "") != (input.ConfigAfterHash != "")
	resourceSourceMissing := (input.ResourceBeforeTreeHash != "") != (input.ResourceAfterTreeHash != "")
	userDataSourceMissing := (input.UserDataBeforeHash != "") != (input.UserDataAfterHash != "")
	missingSource := configSourceMissing || resourceSourceMissing || userDataSourceMissing

	anyChange := req.ConfigChanged || req.ResourcesChanged || req.UserDataChanged ||
		req.MigrationPlanPresent || req.MigrationDefinitionPresent || req.MigrationOperationPresent

	req.Required = anyChange || missingSource || !input.ManifestNoDataChange
	req.NoDataChange = !req.Required

	if req.Required {
		switch {
		case missingSource:
			req.Reason = "before/after evidence count mismatch, fail-closed"
		case !input.ManifestNoDataChange:
			req.Reason = "manifest does not declare no-data-change, fail-closed"
		case req.MigrationPlanPresent:
			req.Reason = "migration plan present"
		case req.MigrationDefinitionPresent:
			req.Reason = "migration definitions present"
		case req.MigrationOperationPresent:
			req.Reason = "migration operations present"
		case req.ConfigChanged:
			req.Reason = "config changed"
		case req.ResourcesChanged:
			req.Reason = "resources changed"
		case req.UserDataChanged:
			req.Reason = "user data changed"
		default:
			req.Reason = "changes detected"
		}
	} else {
		req.Reason = "no data change detected"
	}

	req.RequirementHash = computeSnapshotRequirementHash(req)
	return req
}

// Deprecated: kept for tests only, build PackageSnapshotRequirementInput from the point and use ComputePackageSnapshotRequirement in production.
func computeRollbackSnapshotRequirementFromPoint(point PackageRollbackPoint) RollbackSnapshotRequirement {
	input := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	corrupt := false
	if point.ConfigSnapshotJSON != "" {
		input.ConfigBeforeHash = packageSnapshotDigest([]byte(point.ConfigSnapshotJSON))
	}
	if point.ResourceSnapshotJSON != "" {
		input.ResourceBeforeTreeHash = packageSnapshotDigest([]byte(point.ResourceSnapshotJSON))
	}
	if point.UserDataMigrationStateJSON != "" {
		input.UserDataBeforeHash = packageSnapshotDigest([]byte(point.UserDataMigrationStateJSON))
	}
	if point.MigrationStateSnapshotJSON != "" {
		var migrationState packageMigrationStateSnapshot
		if json.Unmarshal([]byte(point.MigrationStateSnapshotJSON), &migrationState) != nil {
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
	req := ComputeRollbackSnapshotRequirement(input)
	if corrupt && req.NoDataChange {
		req.Required = true
		req.NoDataChange = false
		req.Reason = "migration state snapshot corrupt, fail-closed"
		req.RequirementHash = computeSnapshotRequirementHash(req)
	}
	return req
}

func computeSnapshotRequirementHash(req RollbackSnapshotRequirement) string {
	canonical := fmt.Sprintf(`{"required":%v,"configChanged":%v,"resourcesChanged":%v,"userDataChanged":%v,"migrationPlanPresent":%v,"migrationDefinitionPresent":%v,"migrationOperationPresent":%v,"migrationStateUnverified":%v,"manifestNoDataChange":%v,"noDataChange":%v}`,
		req.Required, req.ConfigChanged, req.ResourcesChanged, req.UserDataChanged,
		req.MigrationPlanPresent, req.MigrationDefinitionPresent, req.MigrationOperationPresent,
		req.MigrationStateUnverified, req.ManifestNoDataChange, req.NoDataChange)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

// Deprecated: kept for tests only, use ComputePackageSnapshotRequirement in production.
func computeRollbackSnapshotRequirement(point PackageRollbackPoint) RollbackSnapshotRequirement {
	return computeRollbackSnapshotRequirementFromPoint(point)
}

// Deprecated: kept for tests only, final gate must not auto-exempt on NoDataChange alone.
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
