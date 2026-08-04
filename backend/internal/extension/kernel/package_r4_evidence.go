package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	StepConfirmationAuthorityEvidence      = "confirmation.authority_evidence"
	confirmationAuthorityEvidenceStepOrder = 0
)

type PackageConfirmationAuthorityEvidence struct {
	SchemaVersion             int      `json:"schemaVersion"`
	OperationID               string   `json:"operationId"`
	OperationType             string   `json:"operationType"`
	ExtensionID               string   `json:"extensionId"`
	ArtifactID                string   `json:"artifactId"`
	PreviewSessionID          string   `json:"previewSessionId,omitempty"`
	PreviewHash               string   `json:"previewHash"`
	SecurityPolicyHash        string   `json:"securityPolicyHash"`
	SnapshotRequirementHash   string   `json:"snapshotRequirementHash"`

	SnapshotRequirementInput *PackageSnapshotRequirementInput `json:"snapshotRequirementInput,omitempty"`
	SnapshotRequirement      *PackageSnapshotRequirement      `json:"snapshotRequirement,omitempty"`

	RequiredConfirmations     []string `json:"requiredConfirmations"`
	RequiredConfirmationsHash string   `json:"requiredConfirmationsHash"`
	DependenciesHash          string   `json:"dependenciesHash"`
	Nonce                     string   `json:"nonce"`
	IssuedAt                  int64    `json:"issuedAt"`
	ExpiresAt                 int64    `json:"expiresAt"`
	SourceVersionID           string   `json:"sourceVersionId,omitempty"`
	SourceGenerationID        string   `json:"sourceGenerationId,omitempty"`
	TargetVersionID           string   `json:"targetVersionId,omitempty"`
	TargetGenerationID        string   `json:"targetGenerationId,omitempty"`
	RollbackPointID           string   `json:"rollbackPointId,omitempty"`
}

func computePackageRequiredConfirmationsHash(required []string) string {
	normalized := normalizeConfirmedItems(required)
	canonical := strings.Join(normalized, ",")
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

func computePackageDependenciesHash(items []string) string {
	normalized := normalizeConfirmedItems(items)
	canonical := strings.Join(normalized, ",")
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

type PackageRollbackPreviewHashInput struct {
	ExtensionID string `json:"extensionId"`

	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion"`

	RollbackPointID string `json:"rollbackPointId"`
	ArtifactID      string `json:"artifactId"`
	SnapshotHash    string `json:"snapshotHash"`

	SnapshotRequirementHash   string `json:"snapshotRequirementHash"`
	RequiredConfirmationsHash string `json:"requiredConfirmationsHash"`
	DependenciesHash          string `json:"dependenciesHash"`
	SecurityPolicyHash        string `json:"securityPolicyHash"`

	InstalledPath     string `json:"installedPath"`
	InstalledTreeHash string `json:"installedTreeHash"`

	SourceGenerationID string `json:"sourceGenerationId"`
	TargetGenerationID string `json:"targetGenerationId"`

	ScopeType string `json:"scopeType"`
	ScopeID   string `json:"scopeId"`
}

func computeRollbackPreviewHash(input PackageRollbackPreviewHashInput) string {
	if input.ExtensionID == "" ||
		input.CurrentVersion == "" ||
		input.TargetVersion == "" ||
		input.RollbackPointID == "" ||
		input.ArtifactID == "" ||
		input.SnapshotHash == "" ||
		input.SnapshotRequirementHash == "" ||
		input.RequiredConfirmationsHash == "" ||
		input.DependenciesHash == "" ||
		input.SecurityPolicyHash == "" ||
		input.SourceGenerationID == "" ||
		input.TargetGenerationID == "" ||
		input.ScopeType == "" {
		return ""
	}

	raw, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}

	hash := sha256.Sum256(raw)

	return "sha256:" + hex.EncodeToString(hash[:])
}

func buildConfirmationAuthorityEvidence(operationID, operationType, previewSessionID string, claims PackageConfirmationClaims) PackageConfirmationAuthorityEvidence {
	required := normalizeConfirmedItems(claims.ConfirmedItems)
	return PackageConfirmationAuthorityEvidence{
		SchemaVersion:             1,
		OperationID:               operationID,
		OperationType:             operationType,
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		PreviewSessionID:          previewSessionID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   claims.SnapshotRequirementHash,
		RequiredConfirmations:     required,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(required),
		DependenciesHash:          claims.DependenciesHash,
		Nonce:                     claims.Nonce,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
		SourceVersionID:           claims.SourceVersionID,
		SourceGenerationID:        claims.SourceGenerationID,
		TargetVersionID:           claims.TargetVersionID,
		TargetGenerationID:        claims.TargetGenerationID,
		RollbackPointID:           claims.RollbackPointID,
	}
}

func standardConfirmationClaimsFromLegacy(operationType string, claims packageConfirmationClaims) PackageConfirmationClaims {
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             operationType,
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   claims.SnapshotRequirementHash,
		RequiredConfirmationsHash: claims.RequiredConfirmationsHash,
		DependenciesHash:          claims.DependenciesHash,
		ConfirmedItems:            confirmedItemsFromMap(claims.Confirmations),
		Confirmations:             claims.Confirmations,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
		Nonce:                     claims.Nonce,
	}
}

func buildRollbackConfirmationAuthorityEvidence(claims PackageRollbackConfirmationClaims, required []string, dependenciesHash string, previewSessionID string, requirementInput PackageSnapshotRequirementInput, requirement PackageSnapshotRequirement) PackageConfirmationAuthorityEvidence {
	normalizedRequired := normalizeConfirmedItems(required)
	inputCopy := requirementInput
	requirementCopy := requirement
	return PackageConfirmationAuthorityEvidence{
		SchemaVersion:             1,
		OperationType:             string(PackageOperationTypeRollback),
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		PreviewSessionID:          previewSessionID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   requirement.Hash,
		SnapshotRequirementInput:  &inputCopy,
		SnapshotRequirement:       &requirementCopy,
		RequiredConfirmations:     normalizedRequired,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(normalizedRequired),
		DependenciesHash:          dependenciesHash,
		Nonce:                     claims.Nonce,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
		SourceVersionID:           claims.SourceVersionID,
		SourceGenerationID:        claims.SourceGenerationID,
		TargetVersionID:           claims.TargetVersionID,
		TargetGenerationID:        claims.TargetGenerationID,
		RollbackPointID:           claims.RollbackPointID,
	}
}

func (r *Runtime) persistPackageConfirmationAuthorityEvidence(ctx context.Context, evidence PackageConfirmationAuthorityEvidence, guard PackageWriteGuard) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return fmt.Errorf("kernel: package repository unavailable for authority evidence")
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("kernel: marshal authority evidence: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID:       "package-step-" + StepConfirmationAuthorityEvidence + "-" + evidence.OperationID,
		OperationID:  evidence.OperationID,
		StepName:     StepConfirmationAuthorityEvidence,
		StepOrder:    confirmationAuthorityEvidenceStepOrder,
		Status:       "completed",
		AttemptCount: 1,
		ResultJSON:   string(raw),
		StartedAt:    now,
		CompletedAt:  now,
	}, guard)
}

func (r *Runtime) loadPackageConfirmationAuthorityEvidence(ctx context.Context, operationID string) (PackageConfirmationAuthorityEvidence, error) {
	var evidence PackageConfirmationAuthorityEvidence
	if r.container == nil || r.container.PackageRepository == nil {
		return evidence, fmt.Errorf("kernel: package repository unavailable for authority evidence")
	}
	steps, err := r.container.PackageRepository.ListOperationSteps(ctx, operationID)
	if err != nil {
		return evidence, fmt.Errorf("kernel: authority evidence steps unavailable: %w", err)
	}
	for _, step := range steps {
		if step.StepName != StepConfirmationAuthorityEvidence {
			continue
		}
		if step.Status != "completed" || step.ResultJSON == "" {
			return evidence, fmt.Errorf("%w: authority evidence step incomplete", ErrPackageFinalGateEvidenceMissing)
		}
		if err := json.Unmarshal([]byte(step.ResultJSON), &evidence); err != nil {
			return evidence, fmt.Errorf("%w: authority evidence corrupt: %v", ErrPackageFinalGateEvidenceInvalid, err)
		}
		if evidence.OperationID != operationID || evidence.OperationID == "" {
			return evidence, fmt.Errorf("%w: authority evidence operation identity mismatch", ErrPackageFinalGateEvidenceInvalid)
		}
		return evidence, nil
	}
	return evidence, fmt.Errorf("%w: authority evidence step missing", ErrPackageFinalGateEvidenceMissing)
}

func verifyExactRequiredConfirmations(confirmed []string, required []string) error {
	confirmedSet := normalizeConfirmedItems(confirmed)
	requiredSet := normalizeConfirmedItems(required)
	if len(confirmedSet) != len(requiredSet) {
		return fmt.Errorf("%w: confirmedItems count %d != required %d", ErrPackageConfirmationItemsMismatch, len(confirmedSet), len(requiredSet))
	}
	for i := range requiredSet {
		if confirmedSet[i] != requiredSet[i] {
			return fmt.Errorf("%w: confirmedItems %q != required %q", ErrPackageConfirmationItemsMismatch, confirmedSet[i], requiredSet[i])
		}
	}
	return nil
}

func verifyConfirmationAuthorityEvidenceClaims(evidence PackageConfirmationAuthorityEvidence, claims PackageConfirmationClaims) error {
	if evidence.OperationType != claims.OperationType {
		return fmt.Errorf("%w: evidence operationType %s != claims %s", ErrPackageConfirmationOperationMismatch, evidence.OperationType, claims.OperationType)
	}
	if evidence.ExtensionID != claims.ExtensionID {
		return fmt.Errorf("%w: evidence extensionId mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.ArtifactID != claims.ArtifactID {
		return fmt.Errorf("%w: evidence artifactId mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.PreviewHash != claims.PreviewHash {
		return fmt.Errorf("%w: evidence previewHash mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.SecurityPolicyHash != claims.SecurityPolicyHash {
		return fmt.Errorf("%w: evidence securityPolicyHash mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.DependenciesHash == "" ||
		claims.DependenciesHash == "" {
		return fmt.Errorf(
			"%w: dependenciesHash missing",
			ErrPackageConfirmationClaimsInvalid,
		)
	}
	if evidence.DependenciesHash != claims.DependenciesHash {
		return fmt.Errorf("%w: evidence dependenciesHash mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.SnapshotRequirementHash != claims.SnapshotRequirementHash {
		return fmt.Errorf("%w: evidence snapshotRequirementHash mismatch", ErrPackageSnapshotRequirementHashMismatch)
	}
	if evidence.RequiredConfirmationsHash != computePackageRequiredConfirmationsHash(claims.ConfirmedItems) {
		return fmt.Errorf("%w: evidence requiredConfirmationsHash mismatch", ErrPackageConfirmationItemsMismatch)
	}
	if evidence.RequiredConfirmationsHash != claims.RequiredConfirmationsHash {
		return fmt.Errorf("%w: claims requiredConfirmationsHash mismatch", ErrPackageConfirmationItemsMismatch)
	}
	if evidence.Nonce != claims.Nonce || evidence.Nonce == "" {
		return fmt.Errorf("%w: evidence nonce mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.IssuedAt != claims.IssuedAt || evidence.IssuedAt == 0 {
		return fmt.Errorf("%w: evidence issuedAt mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.ExpiresAt != claims.ExpiresAt || evidence.ExpiresAt == 0 {
		return fmt.Errorf("%w: evidence expiresAt mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if err := verifyExactRequiredConfirmations(claims.ConfirmedItems, evidence.RequiredConfirmations); err != nil {
		return err
	}
	if evidence.OperationType == string(PackageOperationTypeUpdate) || evidence.OperationType == string(PackageOperationTypeRollback) {
		if evidence.SourceVersionID != claims.SourceVersionID || evidence.SourceGenerationID != claims.SourceGenerationID {
			return fmt.Errorf("%w: evidence source version identity mismatch", ErrPackageConfirmationClaimsInvalid)
		}
	}
	if evidence.TargetVersionID != claims.TargetVersionID || evidence.TargetGenerationID != claims.TargetGenerationID {
		return fmt.Errorf("%w: evidence target version identity mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	if evidence.OperationType == string(PackageOperationTypeRollback) && evidence.RollbackPointID != claims.RollbackPointID {
		return fmt.Errorf("%w: evidence rollbackPointId mismatch", ErrPackageConfirmationClaimsInvalid)
	}
	return nil
}

func evidenceRequiredConfirmationsForRollback(required []string) []string {
	return normalizeConfirmedItems(required)
}

var errConfirmationEvidenceSignature = errors.New("kernel: confirmation authority evidence validation failed")

func validateConfirmationAuthorityEvidenceSignature(evidence PackageConfirmationAuthorityEvidence) error {
	if evidence.SchemaVersion != 1 {
		return fmt.Errorf("%w: unsupported evidence schemaVersion %d", errConfirmationEvidenceSignature, evidence.SchemaVersion)
	}
	if evidence.OperationID == "" || evidence.OperationType == "" || evidence.ExtensionID == "" {
		return fmt.Errorf("%w: evidence identity incomplete", errConfirmationEvidenceSignature)
	}
	if evidence.ArtifactID == "" || evidence.PreviewHash == "" || evidence.SecurityPolicyHash == "" || evidence.DependenciesHash == "" {
		return fmt.Errorf("%w: evidence preview identity incomplete", errConfirmationEvidenceSignature)
	}
	if len(evidence.RequiredConfirmations) == 0 || evidence.RequiredConfirmationsHash == "" {
		return fmt.Errorf("%w: evidence required confirmations incomplete", errConfirmationEvidenceSignature)
	}
	if evidence.Nonce == "" || evidence.IssuedAt == 0 || evidence.ExpiresAt == 0 {
		return fmt.Errorf("%w: evidence binding incomplete", errConfirmationEvidenceSignature)
	}
	if evidence.RequiredConfirmationsHash != computePackageRequiredConfirmationsHash(evidence.RequiredConfirmations) {
		return fmt.Errorf("%w: evidence requiredConfirmationsHash inconsistent", errConfirmationEvidenceSignature)
	}

	if evidence.OperationType == string(PackageOperationTypeRollback) {
		if evidence.SnapshotRequirementInput == nil {
			return fmt.Errorf("%w: rollback snapshot requirement input missing", errConfirmationEvidenceSignature)
		}
		if evidence.SnapshotRequirement == nil {
			return fmt.Errorf("%w: rollback snapshot requirement result missing", errConfirmationEvidenceSignature)
		}
		recomputed, err := ComputePackageSnapshotRequirement(*evidence.SnapshotRequirementInput)
		if err != nil {
			return fmt.Errorf("%w: rollback snapshot requirement recompute failed: %v", errConfirmationEvidenceSignature, err)
		}
		if evidence.SnapshotRequirement.Hash == "" || evidence.SnapshotRequirement.Hash != recomputed.Hash {
			return fmt.Errorf("%w: rollback snapshot requirement result hash mismatch", errConfirmationEvidenceSignature)
		}
		if evidence.SnapshotRequirementHash != recomputed.Hash {
			return fmt.Errorf("%w: rollback snapshotRequirementHash mismatch", errConfirmationEvidenceSignature)
		}
		if evidence.SnapshotRequirement.Required != recomputed.Required ||
			evidence.SnapshotRequirement.NoDataChange != recomputed.NoDataChange ||
			evidence.SnapshotRequirement.Reason != recomputed.Reason {
			return fmt.Errorf("%w: rollback snapshot requirement decision mismatch", errConfirmationEvidenceSignature)
		}
	}

	return nil
}

func evidenceRequiresConfirmation(
	evidence PackageConfirmationAuthorityEvidence,
	required string,
) bool {
	required = strings.TrimSpace(required)

	if required == "" {
		return false
	}

	normalized := normalizeConfirmedItems(
		evidence.RequiredConfirmations,
	)

	index := sort.SearchStrings(
		normalized,
		required,
	)

	return index < len(normalized) &&
		normalized[index] == required
}

func verifySnapshotExemptionAuthority(
	requirement PackageSnapshotRequirement,
	claims PackageConfirmationClaims,
	evidence PackageConfirmationAuthorityEvidence,
	claimsVerified bool,
) error {
	if !claimsVerified {
		return fmt.Errorf(
			"kernel: snapshot exemption requires verified claims",
		)
	}

	if requirement.Required {
		return fmt.Errorf(
			"kernel: required snapshot cannot be exempted",
		)
	}

	if !requirement.NoDataChange {
		return fmt.Errorf(
			"kernel: snapshot exemption requires proven no-data-change",
		)
	}

	if strings.TrimSpace(
		requirement.Hash,
	) == "" {
		return fmt.Errorf(
			"kernel: snapshot exemption requirement hash missing",
		)
	}

	if claims.SnapshotRequirementHash !=
		requirement.Hash {
		return fmt.Errorf(
			"kernel: snapshot exemption claims requirement hash mismatch",
		)
	}

	if evidence.SnapshotRequirementHash !=
		requirement.Hash {
		return fmt.Errorf(
			"kernel: snapshot exemption authority requirement hash mismatch",
		)
	}

	if claims.SecurityPolicyHash == "" ||
		claims.SecurityPolicyHash !=
			computeSecurityPolicyHash() {
		return fmt.Errorf(
			"kernel: snapshot exemption security policy mismatch",
		)
	}

	if evidence.SecurityPolicyHash !=
		claims.SecurityPolicyHash {
		return fmt.Errorf(
			"kernel: snapshot exemption authority security policy mismatch",
		)
	}

	if !packageConfirmationContains(
		claims.ConfirmedItems,
		claims.Confirmations,
		PackageConfirmationSnapshotExempt,
	) {
		return fmt.Errorf(
			"kernel: explicit snapshot exemption confirmation missing",
		)
	}

	if !evidenceRequiresConfirmation(
		evidence,
		PackageConfirmationSnapshotExempt,
	) {
		return fmt.Errorf(
			"kernel: snapshot exemption is not present in authority requirements",
		)
	}

	if evidence.RequiredConfirmationsHash == "" ||
		evidence.RequiredConfirmationsHash !=
			computePackageRequiredConfirmationsHash(
				evidence.RequiredConfirmations,
			) {
		return fmt.Errorf(
			"kernel: snapshot exemption authority confirmation hash invalid",
		)
	}

	if claims.RequiredConfirmationsHash == "" ||
		claims.RequiredConfirmationsHash !=
			evidence.RequiredConfirmationsHash {
		return fmt.Errorf(
			"kernel: snapshot exemption required confirmation hash mismatch",
		)
	}

	if err := verifyExactRequiredConfirmations(
		claims.ConfirmedItems,
		evidence.RequiredConfirmations,
	); err != nil {
		return fmt.Errorf(
			"kernel: snapshot exemption confirmation set mismatch: %w",
			err,
		)
	}

	return nil
}

func sortedConfirmationStrings(values []string) []string {
	sorted := make([]string, len(values))
	copy(sorted, values)
	sort.Strings(sorted)
	return sorted
}

func packageDependencyIdentities(preview InstallPreview) []string {
	identities := make([]string, 0, len(preview.MissingDependencies)+len(preview.Issues))
	for _, dependency := range preview.MissingDependencies {
		identities = append(identities, string(dependency.Type)+":"+dependency.ID+"@"+dependency.Version)
	}
	for _, issue := range preview.Issues {
		if issue.Category == PreviewMissingDependency || issue.Category == PreviewNeedsPermission || issue.Category == PreviewNeedsScope {
			identities = append(identities, issue.Category.String()+":"+issue.Code)
		}
	}
	return normalizeConfirmedItems(identities)
}

func confirmationTimestamp(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339Nano)
}

func (c PreviewCategory) String() string {
	return string(c)
}
