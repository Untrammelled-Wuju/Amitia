package kernel

import (
	"strings"
	"testing"
	"time"
)

func testRollbackPreviewHashInput() PackageRollbackPreviewHashInput {
	return PackageRollbackPreviewHashInput{
		ExtensionID: "com.example/r4",

		CurrentVersion: "2.0.0",

		TargetVersion: "1.0.0",

		RollbackPointID: "rollback-point-r4",

		ArtifactID: "artifact-r4",

		SnapshotHash: "sha256:snapshot",

		SnapshotRequirementHash: "sha256:requirement",

		RequiredConfirmationsHash: "sha256:confirmations",

		DependenciesHash: "sha256:dependencies-a",

		SecurityPolicyHash: "sha256:security-a",

		InstalledPath: "/extensions/com.example/r4",

		InstalledTreeHash: "sha256:tree",

		SourceGenerationID: "generation-2",

		TargetGenerationID: "generation-1",

		ScopeType: "global",

		ScopeID: "",
	}
}

func TestR41RollbackPreviewHashBindsDependenciesHash(t *testing.T) {
	left := testRollbackPreviewHashInput()

	right := left

	right.DependenciesHash = "sha256:dependencies-b"

	leftHash := computeRollbackPreviewHash(left)

	rightHash := computeRollbackPreviewHash(right)

	if leftHash == "" ||
		rightHash == "" {
		t.Fatal("preview hash must not be empty")
	}

	if leftHash == rightHash {
		t.Fatal("dependenciesHash drift must change rollback PreviewHash")
	}
}

func TestR41RollbackPreviewHashBindsSecurityPolicyHash(t *testing.T) {
	left := testRollbackPreviewHashInput()

	right := left

	right.SecurityPolicyHash = "sha256:security-b"

	leftHash := computeRollbackPreviewHash(left)

	rightHash := computeRollbackPreviewHash(right)

	if leftHash == rightHash {
		t.Fatal("securityPolicyHash drift must change rollback PreviewHash")
	}
}

func TestR41SameRollbackPreviewRejectsDependenciesHashDrift(t *testing.T) {
	input := testRollbackPreviewHashInput()

	current := PackageRollbackPreviewResult{
		ExtensionID: input.ExtensionID,

		CurrentVersion: input.CurrentVersion,

		TargetVersion: input.TargetVersion,

		TargetArtifactID: input.ArtifactID,

		RollbackPointID: input.RollbackPointID,

		SourceGenerationID: input.SourceGenerationID,

		TargetGenerationID: input.TargetGenerationID,

		SecurityPolicyHash: input.SecurityPolicyHash,

		SnapshotRequirementHash: input.SnapshotRequirementHash,

		RequiredConfirmationsHash: input.RequiredConfirmationsHash,

		DependenciesHash: input.DependenciesHash,
	}

	current.PreviewHash = computeRollbackPreviewHash(input)

	claims := PackageRollbackConfirmationClaims{
		ExtensionID: current.ExtensionID,

		ArtifactID: current.TargetArtifactID,

		SourceVersionID: current.CurrentVersion,

		SourceGenerationID: current.SourceGenerationID,

		TargetVersionID: current.TargetVersion,

		TargetGenerationID: current.TargetGenerationID,

		RollbackPointID: current.RollbackPointID,

		SecurityPolicyHash: current.SecurityPolicyHash,

		SnapshotRequirementHash: current.SnapshotRequirementHash,

		RequiredConfirmationsHash: current.RequiredConfirmationsHash,

		DependenciesHash: "sha256:stale-dependencies",

		PreviewHash: current.PreviewHash,
	}

	err := samePackageRollbackPreview(claims, current)

	if err == nil {
		t.Fatal("dependenciesHash drift must be rejected")
	}

	if !strings.Contains(err.Error(), "dependenciesHash mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR41RollbackConfirmationRejectsMissingDependenciesHash(t *testing.T) {
	now := time.Now().UTC()

	required := []string{"confirm.rollback"}

	claims := PackageRollbackConfirmationClaims{
		SchemaVersion: PackageRollbackConfirmationClaimsSchemaVersion,

		OperationType: string(PackageOperationTypeRollback),

		PolicyVersion: packagePolicyVersion,

		ExtensionID: "com.example/r4",

		ArtifactID: "artifact-r4",

		SourceVersionID: "2.0.0",

		SourceGenerationID: "generation-2",

		TargetVersionID: "1.0.0",

		TargetGenerationID: "generation-1",

		RollbackPointID: "rollback-point-r4",

		PreviewSessionID: "rollback-preview-r4",

		PreviewHash: "sha256:preview",

		SecurityPolicyHash: "sha256:security",

		SnapshotRequirementHash: "sha256:requirement",

		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(required),

		DependenciesHash: "",

		UserID: "user-1",

		ScopeType: "global",

		ConfirmedItems: required,

		Confirmations: map[string]bool{
			"confirm.rollback": true,
		},

		IssuedAt: now.Unix(),

		ExpiresAt: now.Add(10 * time.Minute).Unix(),

		Nonce: "nonce-r4",
	}

	token, err := signPackageRollbackConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifyPackageRollbackConfirmation(token)

	if err == nil {
		t.Fatal("missing dependenciesHash must be rejected")
	}

	if !strings.Contains(err.Error(), "dependenciesHash") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR41AuthorityEvidenceRejectsDependenciesHashMismatch(t *testing.T) {
	required := []string{"confirm.rollback"}

	evidence := PackageConfirmationAuthorityEvidence{
		OperationType: string(PackageOperationTypeRollback),

		ExtensionID: "com.example/r4",

		ArtifactID: "artifact-r4",

		PreviewHash: "sha256:preview",

		SecurityPolicyHash: "sha256:security",

		SnapshotRequirementHash: "sha256:requirement",

		RequiredConfirmations: required,

		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(required),

		DependenciesHash: "sha256:dependencies-current",

		Nonce: "nonce-r4",

		IssuedAt: 100,

		ExpiresAt: 200,

		SourceVersionID: "2.0.0",

		SourceGenerationID: "generation-2",

		TargetVersionID: "1.0.0",

		TargetGenerationID: "generation-1",

		RollbackPointID: "rollback-point-r4",
	}

	claims := PackageConfirmationClaims{
		OperationType: string(PackageOperationTypeRollback),

		ExtensionID: evidence.ExtensionID,

		ArtifactID: evidence.ArtifactID,

		PreviewHash: evidence.PreviewHash,

		SecurityPolicyHash: evidence.SecurityPolicyHash,

		SnapshotRequirementHash: evidence.SnapshotRequirementHash,

		RequiredConfirmationsHash: evidence.RequiredConfirmationsHash,

		DependenciesHash: "sha256:dependencies-stale",

		ConfirmedItems: required,

		Nonce: evidence.Nonce,

		IssuedAt: evidence.IssuedAt,

		ExpiresAt: evidence.ExpiresAt,

		SourceVersionID: evidence.SourceVersionID,

		SourceGenerationID: evidence.SourceGenerationID,

		TargetVersionID: evidence.TargetVersionID,

		TargetGenerationID: evidence.TargetGenerationID,

		RollbackPointID: evidence.RollbackPointID,
	}

	err := verifyConfirmationAuthorityEvidenceClaims(evidence, claims)

	if err == nil {
		t.Fatal("authority evidence dependenciesHash mismatch must fail")
	}
}
