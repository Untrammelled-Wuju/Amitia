package kernel

import (
	"strings"
	"testing"
)

func r42ValidRollbackRequirementInput(
	evidenceInput PackageConfirmationAuthorityInput,
	claims PackageConfirmationClaims,
) PackageSnapshotRequirementInput {
	return PackageSnapshotRequirementInput{
		SchemaVersion:            1,
		OperationType:            string(PackageOperationTypeRollback),
		ExtensionID:              evidenceInput.ExtensionID,
		SourceVersion:            claims.SourceVersionID,
		SourceGeneration:         claims.SourceGenerationID,
		TargetVersion:            claims.TargetVersionID,
		TargetGeneration:         claims.TargetGenerationID,
		ConfigBeforeHash:         "sha256:r42-config-before",
		ConfigAfterHash:          "sha256:r42-config-after",
		ConfigEvidencePresent:    true,
		ResourceBeforeHash:       "sha256:r42-resource-before",
		ResourceAfterHash:        "sha256:r42-resource-after",
		ResourceEvidencePresent:  true,
		ResourceAdded:            []string{},
		ResourceRemoved:          []string{},
		ResourceChanged:          []string{},
		UserDataBeforeHash:       "sha256:r42-user-data",
		UserDataAfterHash:        "sha256:r42-user-data",
		UserDataEvidencePresent:  true,
		MigrationEvidencePresent: true,
		ManifestNoDataChange:     false,
		ManifestEvidencePresent:  true,
	}
}

func r42ValidRollbackEvidence(
	t *testing.T,
) PackageConfirmationAuthorityEvidence {
	t.Helper()

	input := r44AuthorityInputRollback()
	claims := r44ClaimsRollback()

	requirementInput := r42ValidRollbackRequirementInput(input, claims)

	requirement, err := ComputePackageSnapshotRequirement(requirementInput)
	if err != nil {
		t.Fatal(err)
	}

	input.SnapshotRequirementHash = requirement.Hash
	claims.SnapshotRequirementHash = requirement.Hash

	evidence, err := buildPackageConfirmationAuthorityEvidence(
		"operation-r42-rollback",
		claims,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}

	evidence, err = finalizeRollbackSnapshotRequirementEvidence(
		evidence,
		requirementInput,
		requirement,
	)
	if err != nil {
		t.Fatal(err)
	}

	return evidence
}

func TestR42ValidRollbackRequirementEvidencePasses(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err != nil {
		t.Fatalf("valid requirement evidence rejected: %v", err)
	}
}

func TestR42RejectsMissingRequirementInput(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirementInput = nil
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("missing snapshot requirement input must fail")
	}

	if !strings.Contains(err.Error(), "snapshotRequirementInput missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR42RejectsMissingRequirementResult(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirement = nil
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("missing snapshot requirement result must fail")
	}
}

func TestR42RejectsRequirementInputTampering(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirementInput.ConfigAfterHash = "sha256:r42-tampered-config"
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("tampered requirement input must fail even after evidenceHash recomputation")
	}

	if !strings.Contains(err.Error(), "snapshot requirement result") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR42RejectsRequirementResultHashTampering(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirement.Hash = "sha256:r42-tampered-result"
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("tampered requirement result hash must fail")
	}
}

func TestR42RejectsRequirementDecisionTampering(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirement.Required = !evidence.SnapshotRequirement.Required
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("tampered requirement decision must fail")
	}
}

func TestR42RejectsRequirementNoDataChangeTampering(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirement.NoDataChange = !evidence.SnapshotRequirement.NoDataChange
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("tampered requirement NoDataChange decision must fail")
	}
}

func TestR42RejectsRequirementReasonTampering(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirement.Reason = "tampered reason"
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("tampered requirement reason must fail")
	}
}

func TestR42RejectsTopLevelRequirementHashDrift(t *testing.T) {
	evidence := r42ValidRollbackEvidence(t)

	evidence.SnapshotRequirementHash = "sha256:r42-top-level-drift"
	evidence.AuthorityInput.SnapshotRequirementHash = evidence.SnapshotRequirementHash
	evidence.AuthorityInputHash = computePackageConfirmationAuthorityInputHash(evidence.AuthorityInput)
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	err := validateConfirmationAuthorityEvidenceSignature(evidence)
	if err == nil {
		t.Fatal("top-level requirement hash drift must fail")
	}
}

func TestR42RejectsSnapshotRequirementEvidenceOnInstall(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatal(err)
	}

	requirementInput := PackageSnapshotRequirementInput{
		SchemaVersion:    1,
		OperationType:    string(PackageOperationTypeRollback),
		ExtensionID:      evidence.ExtensionID,
		SourceVersion:    "1.0.0",
		SourceGeneration: "generation-1",
		TargetVersion:    "0.9.0",
		TargetGeneration: "generation-0",
	}

	requirement, err := ComputePackageSnapshotRequirement(requirementInput)
	if err != nil {
		t.Fatal(err)
	}

	evidence.SnapshotRequirementInput = &requirementInput
	evidence.SnapshotRequirement = &requirement
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err == nil {
		t.Fatal("install evidence must not carry rollback requirement objects")
	}
}
