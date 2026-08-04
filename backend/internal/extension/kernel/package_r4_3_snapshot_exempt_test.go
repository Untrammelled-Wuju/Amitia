package kernel

import (
	"testing"
)

func r43SnapshotExemptRequirement() PackageSnapshotRequirement {
	return PackageSnapshotRequirement{
		Required:     false,
		NoDataChange: true,
		Reason:       "no data change detected",
		Hash:         "sha256:r43-requirement",
	}
}

func r43SnapshotExemptClaims() PackageConfirmationClaims {
	required := []string{
		"confirm.update",
		PackageConfirmationSnapshotExempt,
	}

	return PackageConfirmationClaims{
		SchemaVersion:   PackageConfirmationClaimsSchemaVersion,
		OperationType:   string(PackageOperationTypeUpdate),
		ExtensionID:     "com.example/r43",
		ArtifactID:      "artifact-r43",
		PreviewHash:     "sha256:r43-preview",
		SecurityPolicyHash: computeSecurityPolicyHash(),
		SnapshotRequirementHash: "sha256:r43-requirement",
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(
			required,
		),
		ConfirmedItems: required,
		Confirmations: map[string]bool{
			"confirm.update":            true,
			PackageConfirmationSnapshotExempt: true,
		},
		Nonce:     "nonce-r43",
		IssuedAt:  100,
		ExpiresAt: 200,
	}
}

func r43SnapshotExemptEvidence() PackageConfirmationAuthorityEvidence {
	claims := r43SnapshotExemptClaims()

	return PackageConfirmationAuthorityEvidence{
		SchemaVersion:             1,
		OperationID:               "operation-r43",
		OperationType:             claims.OperationType,
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   claims.SnapshotRequirementHash,
		RequiredConfirmations:     append([]string(nil), claims.ConfirmedItems...),
		RequiredConfirmationsHash: claims.RequiredConfirmationsHash,
		DependenciesHash:          computePackageDependenciesHash(nil),
		Nonce:                     claims.Nonce,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
	}
}

func TestR43SnapshotExemptionRequiresExplicitConfirmation(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()
	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err != nil {
		t.Fatalf(
			"valid explicit exemption rejected: %v",
			err,
		)
	}
}

func TestR43SnapshotExemptionRejectsMissingConfirmation(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()

	delete(
		claims.Confirmations,
		PackageConfirmationSnapshotExempt,
	)

	claims.ConfirmedItems = []string{
		"confirm.update",
	}

	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err == nil {
		t.Fatal(
			"missing snapshot exemption confirmation must fail",
		)
	}
}

func TestR43SnapshotExemptionRejectsFalseConfirmation(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()

	claims.Confirmations[PackageConfirmationSnapshotExempt] = false

	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err == nil {
		t.Fatal(
			"false snapshot exemption confirmation must fail",
		)
	}
}

func TestR43SnapshotExemptionRejectsAuthorityWithoutConfirmation(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()
	evidence := r43SnapshotExemptEvidence()

	evidence.RequiredConfirmations = []string{
		"confirm.update",
	}

	evidence.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(
		evidence.RequiredConfirmations,
	)

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err == nil {
		t.Fatal(
			"authority without snapshot exemption requirement must fail",
		)
	}
}

func TestR43RequiredSnapshotCannotBeExempted(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	requirement.Required = true
	requirement.NoDataChange = false
	requirement.Reason = "config changed"

	claims := r43SnapshotExemptClaims()
	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err == nil {
		t.Fatal(
			"required snapshot must not be exempted",
		)
	}
}

func TestR43SnapshotExemptionRejectsUnverifiedClaims(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()
	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		false,
	); err == nil {
		t.Fatal(
			"snapshot exemption with unverified claims must fail",
		)
	}
}

func TestR43SnapshotExemptionRejectsRequirementHashDrift(
	t *testing.T,
) {
	requirement := r43SnapshotExemptRequirement()
	claims := r43SnapshotExemptClaims()

	claims.SnapshotRequirementHash = "sha256:stale-requirement"

	evidence := r43SnapshotExemptEvidence()

	if err := verifySnapshotExemptionAuthority(
		requirement,
		claims,
		evidence,
		true,
	); err == nil {
		t.Fatal(
			"snapshot exemption requirement hash drift must fail",
		)
	}
}
