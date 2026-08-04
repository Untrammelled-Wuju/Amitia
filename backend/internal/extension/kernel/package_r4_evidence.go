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
	StepConfirmationAuthorityEvidence = "confirmation.authority_evidence"

	confirmationAuthorityEvidenceStepOrder = 0

	packageConfirmationAuthorityInputSchemaVersion = 1

	packageConfirmationAuthorityEvidenceSchemaVersion = 2

	packageConfirmationAuthoritySourcePostLeasePreview = "post_lease_preview_v1"
)

type PackageConfirmationAuthorityInput struct {
	SchemaVersion int `json:"schemaVersion"`

	Source string `json:"source"`

	OperationType string `json:"operationType"`
	ExtensionID   string `json:"extensionId"`
	ArtifactID    string `json:"artifactId"`

	PreviewSessionID string `json:"previewSessionId,omitempty"`
	PreviewHash      string `json:"previewHash"`

	SecurityPolicyHash      string `json:"securityPolicyHash"`
	SnapshotRequirementHash string `json:"snapshotRequirementHash"`

	ArtifactPolicy ArtifactPolicy `json:"artifactPolicy,omitempty"`

	Dependencies []string `json:"dependencies"`

	MigrationPlanHash string `json:"migrationPlanHash,omitempty"`

	RequiredConfirmations []string `json:"requiredConfirmations"`

	CapturedAt string `json:"capturedAt"`
}

type PackageConfirmationAuthorityEvidence struct {
	SchemaVersion int `json:"schemaVersion"`

	OperationID   string `json:"operationId"`
	OperationType string `json:"operationType"`
	ExtensionID   string `json:"extensionId"`
	ArtifactID    string `json:"artifactId"`

	AuthorityInput     PackageConfirmationAuthorityInput `json:"authorityInput"`
	AuthorityInputHash string                            `json:"authorityInputHash"`

	PreviewSessionID string `json:"previewSessionId,omitempty"`
	PreviewHash      string `json:"previewHash"`

	SecurityPolicyHash      string `json:"securityPolicyHash"`
	SnapshotRequirementHash string `json:"snapshotRequirementHash"`

	SnapshotRequirementInput *PackageSnapshotRequirementInput `json:"snapshotRequirementInput,omitempty"`
	SnapshotRequirement      *PackageSnapshotRequirement      `json:"snapshotRequirement,omitempty"`

	ArtifactPolicy ArtifactPolicy `json:"artifactPolicy,omitempty"`

	Dependencies     []string `json:"dependencies"`
	DependenciesHash string   `json:"dependenciesHash"`

	RequiredConfirmations     []string `json:"requiredConfirmations"`
	RequiredConfirmationsHash string   `json:"requiredConfirmationsHash"`

	Nonce     string `json:"nonce"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`

	SourceVersionID    string `json:"sourceVersionId,omitempty"`
	SourceGenerationID string `json:"sourceGenerationId,omitempty"`
	TargetVersionID    string `json:"targetVersionId,omitempty"`
	TargetGenerationID string `json:"targetGenerationId,omitempty"`
	RollbackPointID    string `json:"rollbackPointId,omitempty"`

	EvidenceHash string `json:"evidenceHash"`
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

func computePackageConfirmationAuthorityInputHash(
	input PackageConfirmationAuthorityInput,
) string {
	copyValue :=
		input

	copyValue.Dependencies =
		normalizeConfirmedItems(
			copyValue.Dependencies,
		)

	copyValue.RequiredConfirmations =
		normalizeConfirmedItems(
			copyValue.RequiredConfirmations,
		)

	raw, err :=
		json.Marshal(copyValue)
	if err != nil {
		panic(err)
	}

	hash :=
		sha256.Sum256(raw)

	return "sha256:" +
		hex.EncodeToString(
			hash[:],
		)
}

func computePackageConfirmationAuthorityEvidenceHash(
	evidence PackageConfirmationAuthorityEvidence,
) string {
	copyValue :=
		evidence

	copyValue.EvidenceHash = ""

	copyValue.Dependencies =
		normalizeConfirmedItems(
			copyValue.Dependencies,
		)

	copyValue.RequiredConfirmations =
		normalizeConfirmedItems(
			copyValue.RequiredConfirmations,
		)

	copyValue.AuthorityInput.Dependencies =
		normalizeConfirmedItems(
			copyValue.
				AuthorityInput.
				Dependencies,
		)

	copyValue.AuthorityInput.RequiredConfirmations =
		normalizeConfirmedItems(
			copyValue.
				AuthorityInput.
				RequiredConfirmations,
		)

	raw, err :=
		json.Marshal(copyValue)
	if err != nil {
		panic(err)
	}

	hash :=
		sha256.Sum256(raw)

	return "sha256:" +
		hex.EncodeToString(
			hash[:],
		)
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

func buildPackageConfirmationAuthorityEvidence(
	operationID string,
	claims PackageConfirmationClaims,
	input PackageConfirmationAuthorityInput,
) (
	PackageConfirmationAuthorityEvidence,
	error,
) {
	if strings.TrimSpace(
		operationID,
	) == "" {
		return PackageConfirmationAuthorityEvidence{},
			fmt.Errorf(
				"kernel: authority operation id missing",
			)
	}

	input.SchemaVersion =
		packageConfirmationAuthorityInputSchemaVersion

	input.Source =
		packageConfirmationAuthoritySourcePostLeasePreview

	input.OperationType =
		strings.TrimSpace(
			input.OperationType,
		)

	input.ExtensionID =
		strings.TrimSpace(
			input.ExtensionID,
		)

	input.ArtifactID =
		strings.TrimSpace(
			input.ArtifactID,
		)

	input.Dependencies =
		normalizeConfirmedItems(
			input.Dependencies,
		)

	input.RequiredConfirmations =
		normalizeConfirmedItems(
			input.RequiredConfirmations,
		)

	if input.OperationType == "" ||
		input.ExtensionID == "" ||
		input.ArtifactID == "" ||
		input.PreviewHash == "" ||
		input.SecurityPolicyHash == "" ||
		input.SnapshotRequirementHash == "" {
		return PackageConfirmationAuthorityEvidence{},
			fmt.Errorf(
				"kernel: confirmation authority input incomplete",
			)
	}

	if input.CapturedAt == "" {
		input.CapturedAt =
			time.Now().
				UTC().
				Format(
					time.RFC3339Nano,
				)
	}

	if claims.OperationType !=
		input.OperationType ||
		claims.ExtensionID !=
			input.ExtensionID ||
		claims.ArtifactID !=
			input.ArtifactID {
		return PackageConfirmationAuthorityEvidence{},
			fmt.Errorf(
				"kernel: confirmation authority claims identity mismatch",
			)
	}

	if claims.PreviewHash !=
		input.PreviewHash {
		return PackageConfirmationAuthorityEvidence{},
			NewPackageError(
				PackageErrCodeConfirmationStale,
				409,
				fmt.Errorf(
					"kernel: post-lease previewHash changed",
				),
			)
	}

	if claims.SecurityPolicyHash !=
		input.SecurityPolicyHash {
		return PackageConfirmationAuthorityEvidence{},
			NewPackageError(
				PackageErrCodeConfirmationPolicyVersionStale,
				409,
				fmt.Errorf(
					"kernel: post-lease security policy changed",
				),
			)
	}

	if claims.SnapshotRequirementHash !=
		input.SnapshotRequirementHash {
		return PackageConfirmationAuthorityEvidence{},
			NewPackageError(
				PackageErrCodeConfirmationStale,
				409,
				fmt.Errorf(
					"kernel: post-lease snapshot requirement changed",
				),
			)
	}

	if claims.DependenciesHash == "" ||
		claims.DependenciesHash !=
			computePackageDependenciesHash(
				input.Dependencies,
			) {
		return PackageConfirmationAuthorityEvidence{},
			NewPackageError(
				PackageErrCodeConfirmationStale,
				409,
				fmt.Errorf(
					"kernel: post-lease dependency authority changed",
				),
			)
	}

	if claims.RequiredConfirmationsHash == "" ||
		claims.RequiredConfirmationsHash !=
			computePackageRequiredConfirmationsHash(
				input.RequiredConfirmations,
			) {
		return PackageConfirmationAuthorityEvidence{},
			NewPackageError(
				PackageErrCodeConfirmationItemsMismatch,
				409,
				fmt.Errorf(
					"kernel: post-lease required confirmations changed",
				),
			)
	}

	if err :=
		verifyExactRequiredConfirmations(
			claims.ConfirmedItems,
			input.RequiredConfirmations,
		); err != nil {
		return PackageConfirmationAuthorityEvidence{},
			err
	}

	if !validateConfirmedItemsConsistency(
		claims.ConfirmedItems,
		claims.Confirmations,
	) {
		return PackageConfirmationAuthorityEvidence{},
			ErrPackageConfirmationItemsMismatch
	}

	inputHash :=
		computePackageConfirmationAuthorityInputHash(
			input,
		)

	evidence :=
		PackageConfirmationAuthorityEvidence{
			SchemaVersion: packageConfirmationAuthorityEvidenceSchemaVersion,

			OperationID: operationID,

			OperationType: input.OperationType,

			ExtensionID: input.ExtensionID,

			ArtifactID: input.ArtifactID,

			AuthorityInput: input,

			AuthorityInputHash: inputHash,

			PreviewSessionID: input.PreviewSessionID,

			PreviewHash: input.PreviewHash,

			SecurityPolicyHash: input.SecurityPolicyHash,

			SnapshotRequirementHash: input.SnapshotRequirementHash,

			ArtifactPolicy: input.ArtifactPolicy,

			Dependencies: input.Dependencies,

			DependenciesHash: computePackageDependenciesHash(
				input.Dependencies,
			),

			RequiredConfirmations: input.RequiredConfirmations,

			RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(
				input.RequiredConfirmations,
			),

			Nonce: claims.Nonce,

			IssuedAt: claims.IssuedAt,

			ExpiresAt: claims.ExpiresAt,

			SourceVersionID: claims.SourceVersionID,

			SourceGenerationID: claims.SourceGenerationID,

			TargetVersionID: claims.TargetVersionID,

			TargetGenerationID: claims.TargetGenerationID,

			RollbackPointID: claims.RollbackPointID,
		}

	evidence.EvidenceHash =
		computePackageConfirmationAuthorityEvidenceHash(
			evidence,
		)

	return evidence, nil
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

func standardConfirmationClaimsFromRollback(
	claims PackageRollbackConfirmationClaims,
) PackageConfirmationClaims {
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             string(PackageOperationTypeRollback),
		ExtensionID:               claims.ExtensionID,
		ArtifactID:                claims.ArtifactID,
		PreviewSessionID:          claims.PreviewSessionID,
		PreviewHash:               claims.PreviewHash,
		SecurityPolicyHash:        claims.SecurityPolicyHash,
		SnapshotRequirementHash:   claims.SnapshotRequirementHash,
		RequiredConfirmationsHash: claims.RequiredConfirmationsHash,
		DependenciesHash:          claims.DependenciesHash,
		PolicyVersion:             claims.PolicyVersion,
		UserID:                    claims.UserID,
		ScopeType:                 claims.ScopeType,
		ScopeID:                   claims.ScopeID,
		ConfirmedItems:            claims.ConfirmedItems,
		Confirmations:             claims.Confirmations,
		IssuedAt:                  claims.IssuedAt,
		ExpiresAt:                 claims.ExpiresAt,
		Nonce:                     claims.Nonce,
		SourceVersionID:           claims.SourceVersionID,
		SourceGenerationID:        claims.SourceGenerationID,
		TargetVersionID:           claims.TargetVersionID,
		TargetGenerationID:        claims.TargetGenerationID,
		RollbackPointID:           claims.RollbackPointID,
	}
}

func (
	r *Runtime,
) recomputeInstallUpdateAuthorityPreview(
	ctx context.Context,
	operationType string,
	session PackagePreviewSession,
	persisted InstallPreview,
) (
	InstallPreview,
	error,
) {
	if operationType !=
		string(
			PackageOperationTypeInstall,
		) &&
		operationType !=
			string(
				PackageOperationTypeUpdate,
			) {
		return InstallPreview{},
			fmt.Errorf(
				"kernel: unsupported authority preview operation %s",
				operationType,
			)
	}

	if session.PolicyVersion !=
		packagePolicyVersion ||
		session.SecurityPolicyHash !=
			computeSecurityPolicyHash() {
		return InstallPreview{},
			NewPackageError(
				PackageErrCodeConfirmationPolicyVersionStale,
				409,
				fmt.Errorf(
					"kernel: authority preview security policy changed",
				),
			)
	}

	artifact, err :=
		r.container.
			PackageRepository.
			GetArtifact(
				ctx,
				session.ArtifactID,
			)
	if err != nil {
		return InstallPreview{},
			fmt.Errorf(
				"kernel: authority artifact unavailable: %w",
				err,
			)
	}

	if artifact.ArtifactID !=
		session.ArtifactID ||
		artifact.ExtensionID !=
			session.ExtensionID ||
		artifact.Version !=
			session.Version ||
		artifact.ArchiveHash !=
			session.ArchiveHash ||
		artifact.ManifestHash !=
			session.ManifestHash ||
		artifact.ContentTreeHash !=
			session.ContentTreeHash {
		return InstallPreview{},
			NewPackageError(
				PackageErrCodeConfirmationStale,
				409,
				fmt.Errorf(
					"kernel: authority artifact identity changed",
				),
			)
	}

	pkg, err :=
		r.VerifyStoredPackage(
			ctx,
			artifact,
		)
	if err != nil {
		return InstallPreview{},
			fmt.Errorf(
				"kernel: authority package verification failed: %w",
				err,
			)
	}

	current :=
		InstallPreview{
			SessionID: session.SessionID,

			ArtifactID: artifact.ArtifactID,

			ExtensionID: artifact.ExtensionID,

			Version: artifact.Version,

			Publisher: artifact.PublisherID,

			ArchiveHash: artifact.ArchiveHash,

			ManifestHash: artifact.ManifestHash,

			ArtifactHash: persisted.ArtifactHash,

			ContentTreeHash: artifact.ContentTreeHash,

			SignatureStatus: artifact.SignatureStatus,

			TrustDecision: artifact.TrustDecision,

			DevOnly: persisted.DevOnly,

			DeveloperSessionID: persisted.DeveloperSessionID,

			SignerKeyID: artifact.SignerKeyID,

			Manifest: pkg.Manifest,

			InstalledPath: persisted.InstalledPath,

			InstalledTreeHash: persisted.InstalledTreeHash,
		}

	if current.DevOnly {
		if err :=
			r.validateUnsignedDeveloperSession(
				current.DeveloperSessionID,
				session.UserID,
				current.ExtensionID,
			); err != nil {
			return InstallPreview{},
				fmt.Errorf(
					"kernel: authority developer session invalid: %w",
					err,
				)
		}

		current.RequiredConfirmations =
			append(
				current.RequiredConfirmations,
				"confirm.unsigned_dev",
			)
	}

	r.evaluatePackageCompatibilityAndDependencies(
		ctx,
		pkg,
		&current,
	)

	if operationType ==
		string(
			PackageOperationTypeUpdate,
		) {
		r.evaluatePackageUpdateRisks(
			ctx,
			&current,
		)
	}

	r.evaluatePackageMigrationPreflight(
		ctx,
		pkg.Manifest,
		&current,
	)

	for _, file := range pkg.Files {
		lower :=
			strings.ToLower(
				file.Path,
			)

		if strings.Contains(
			lower,
			"/scripts/",
		) ||
			strings.HasPrefix(
				lower,
				"scripts/",
			) {
			current.RequiredConfirmations =
				append(
					current.RequiredConfirmations,
					"confirm.scripts",
				)
		}

		if strings.HasPrefix(
			lower,
			"migrations/",
		) {
			current.RequiredConfirmations =
				append(
					current.RequiredConfirmations,
					"confirm.config_migration",
				)
		}
	}

	if len(
		pkg.Manifest.Permissions,
	) > 0 {
		current.RequiredConfirmations =
			append(
				current.RequiredConfirmations,
				"confirm.permission_escalation",
			)
	}

	for _, module := range pkg.Manifest.Modules {
		for _, contribution := range module.Contributions {
			if len(
				contribution.RequiredScope,
			) > 0 {
				current.RequiredConfirmations =
					append(
						current.RequiredConfirmations,
						"confirm.scope_expansion",
					)
			}
		}
	}

	current.RequiredConfirmations =
		uniquePackageStrings(
			current.RequiredConfirmations,
		)

	current.RiskFlags =
		uniquePackageStrings(
			current.RiskFlags,
		)

	current.Installable =
		len(current.Issues) == 0

	current.Category =
		classifyPreview(
			current.Issues,
			current.Installable,
		)

	if !current.Installable {
		return current,
			NewPackageError(
				PackageErrCodeConfirmationStale,
				409,
				fmt.Errorf(
					"kernel: post-lease package preview no longer installable",
				),
			)
	}

	return current, nil
}

func (
	r *Runtime,
) buildInstallUpdateAuthorityInput(
	ctx context.Context,
	operationType string,
	session PackagePreviewSession,
	persisted InstallPreview,
) (
	PackageConfirmationAuthorityInput,
	error,
) {
	current, err :=
		r.recomputeInstallUpdateAuthorityPreview(
			ctx,
			operationType,
			session,
			persisted,
		)
	if err != nil {
		return PackageConfirmationAuthorityInput{},
			err
	}

	requirement :=
		ComputeInstallSnapshotRequirement(
			computeInstallSnapshotRequirementInput(
				current.InstalledPath,
				current.InstalledTreeHash,
				session.ArtifactID,
				session.ExtensionID,
			),
		)

	if requirement.RequirementHash == "" {
		return PackageConfirmationAuthorityInput{},
			fmt.Errorf(
				"kernel: current snapshot requirement hash missing",
			)
	}

	previewHash :=
		computeInstallPreviewHash(
			session,
			current,
		)

	if previewHash == "" {
		return PackageConfirmationAuthorityInput{},
			fmt.Errorf(
				"kernel: current preview hash missing",
			)
	}

	return PackageConfirmationAuthorityInput{
		SchemaVersion: packageConfirmationAuthorityInputSchemaVersion,

		Source: packageConfirmationAuthoritySourcePostLeasePreview,

		OperationType: operationType,

		ExtensionID: session.ExtensionID,

		ArtifactID: session.ArtifactID,

		PreviewSessionID: session.SessionID,

		PreviewHash: previewHash,

		SecurityPolicyHash: computeSecurityPolicyHash(),

		SnapshotRequirementHash: requirement.RequirementHash,

		Dependencies: packageDependencyIdentities(
			current,
		),

		MigrationPlanHash: current.MigrationPlanHash,

		RequiredConfirmations: current.RequiredConfirmations,

		CapturedAt: time.Now().
			UTC().
			Format(
				time.RFC3339Nano,
			),
	}, nil
}

func buildRollbackAuthorityInput(
	preview PackageRollbackPreviewResult,
) (
	PackageConfirmationAuthorityInput,
	error,
) {
	if preview.ExtensionID == "" ||
		preview.TargetArtifactID == "" ||
		preview.PreviewHash == "" ||
		preview.SecurityPolicyHash == "" ||
		preview.SnapshotRequirementHash == "" {
		return PackageConfirmationAuthorityInput{},
			fmt.Errorf(
				"kernel: rollback post-lease authority preview incomplete",
			)
	}

	required :=
		normalizeConfirmedItems(
			preview.RequiredConfirmations,
		)

	dependencies :=
		normalizeConfirmedItems(
			preview.Dependents,
		)

	return PackageConfirmationAuthorityInput{
		SchemaVersion: packageConfirmationAuthorityInputSchemaVersion,

		Source: packageConfirmationAuthoritySourcePostLeasePreview,

		OperationType: string(
			PackageOperationTypeRollback,
		),

		ExtensionID: preview.ExtensionID,

		ArtifactID: preview.TargetArtifactID,

		PreviewSessionID: preview.PreviewSessionID,

		PreviewHash: preview.PreviewHash,

		SecurityPolicyHash: preview.SecurityPolicyHash,

		SnapshotRequirementHash: preview.SnapshotRequirementHash,

		Dependencies: dependencies,

		RequiredConfirmations: required,

		CapturedAt: time.Now().
			UTC().
			Format(
				time.RFC3339Nano,
			),
	}, nil
}

func buildUninstallAuthorityInput(
	preview PackageUninstallPreviewResult,
) (
	PackageConfirmationAuthorityInput,
	error,
) {
	if preview.ExtensionID == "" ||
		preview.ArtifactID == "" ||
		preview.PreviewHash == "" ||
		preview.SecurityPolicyHash == "" ||
		preview.SnapshotRequirementHash == "" {
		return PackageConfirmationAuthorityInput{},
			fmt.Errorf(
				"kernel: uninstall post-lease authority preview incomplete",
			)
	}

	required :=
		normalizeConfirmedItems(
			requiredUninstallConfirmations(
				preview,
			),
		)

	dependencies :=
		normalizeConfirmedItems(
			preview.Dependents,
		)

	return PackageConfirmationAuthorityInput{
		SchemaVersion: packageConfirmationAuthorityInputSchemaVersion,

		Source: packageConfirmationAuthoritySourcePostLeasePreview,

		OperationType: string(
			PackageOperationTypeUninstall,
		),

		ExtensionID: preview.ExtensionID,

		ArtifactID: preview.ArtifactID,

		PreviewHash: preview.PreviewHash,

		SecurityPolicyHash: preview.SecurityPolicyHash,

		SnapshotRequirementHash: preview.SnapshotRequirementHash,

		ArtifactPolicy: preview.ArtifactPolicy,

		Dependencies: dependencies,

		RequiredConfirmations: required,

		CapturedAt: time.Now().
			UTC().
			Format(
				time.RFC3339Nano,
			),
	}, nil
}

func (
	r *Runtime,
) persistPackageConfirmationAuthorityEvidence(
	ctx context.Context,
	evidence PackageConfirmationAuthorityEvidence,
	guard PackageWriteGuard,
) error {
	if r.container == nil ||
		r.container.PackageRepository == nil {
		return fmt.Errorf(
			"kernel: package repository unavailable for authority evidence",
		)
	}

	if err :=
		validateConfirmationAuthorityEvidenceSignature(
			evidence,
		); err != nil {
		return err
	}

	raw, err :=
		json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf(
			"kernel: marshal authority evidence: %w",
			err,
		)
	}

	resultHash :=
		fmt.Sprintf(
			"%x",
			sha256.Sum256(raw),
		)

	now :=
		time.Now().
			UTC().
			Format(
				time.RFC3339Nano,
			)

	return r.container.
		PackageRepository.
		PutStep(
			ctx,
			PackageOperationStep{
				StepID: "package-step-" +
					StepConfirmationAuthorityEvidence +
					"-" +
					evidence.OperationID,

				OperationID: evidence.OperationID,

				StepName: StepConfirmationAuthorityEvidence,

				StepOrder: confirmationAuthorityEvidenceStepOrder,

				Status: StatusCompleted,

				AttemptCount: 1,

				ResultJSON: string(raw),

				ResultHash: resultHash,

				StartedAt: now,

				CompletedAt: now,
			},
			guard,
		)
}

func (
	r *Runtime,
) loadPackageConfirmationAuthorityEvidence(
	ctx context.Context,
	operationID string,
) (
	PackageConfirmationAuthorityEvidence,
	error,
) {
	var evidence PackageConfirmationAuthorityEvidence

	if r.container == nil ||
		r.container.PackageRepository == nil {
		return evidence,
			fmt.Errorf(
				"kernel: package repository unavailable for authority evidence",
			)
	}

	steps, err :=
		r.container.
			PackageRepository.
			ListOperationSteps(
				ctx,
				operationID,
			)
	if err != nil {
		return evidence,
			fmt.Errorf(
				"kernel: authority evidence steps unavailable: %w",
				err,
			)
	}

	matches :=
		make(
			[]PackageOperationStep,
			0,
			1,
		)

	for _, step := range steps {
		if step.StepName ==
			StepConfirmationAuthorityEvidence {
			matches =
				append(
					matches,
					step,
				)
		}
	}

	if len(matches) != 1 {
		return evidence,
			fmt.Errorf(
				"%w: expected exactly one authority evidence step, found %d",
				ErrPackageFinalGateEvidenceMissing,
				len(matches),
			)
	}

	step :=
		matches[0]

	if step.Status !=
		StatusCompleted ||
		step.ResultJSON == "" ||
		step.ResultHash == "" {
		return evidence,
			fmt.Errorf(
				"%w: authority evidence step incomplete",
				ErrPackageFinalGateEvidenceMissing,
			)
	}

	actualResultHash :=
		fmt.Sprintf(
			"%x",
			sha256.Sum256(
				[]byte(
					step.ResultJSON,
				),
			),
		)

	if actualResultHash !=
		step.ResultHash {
		return evidence,
			fmt.Errorf(
				"%w: authority evidence step result hash mismatch",
				ErrPackageFinalGateEvidenceInvalid,
			)
	}

	if err :=
		json.Unmarshal(
			[]byte(
				step.ResultJSON,
			),
			&evidence,
		); err != nil {
		return evidence,
			fmt.Errorf(
				"%w: authority evidence corrupt: %v",
				ErrPackageFinalGateEvidenceInvalid,
				err,
			)
	}

	if evidence.OperationID !=
		operationID ||
		evidence.OperationID == "" {
		return evidence,
			fmt.Errorf(
				"%w: authority evidence operation identity mismatch",
				ErrPackageFinalGateEvidenceInvalid,
			)
	}

	if err :=
		validateConfirmationAuthorityEvidenceSignature(
			evidence,
		); err != nil {
		return evidence, err
	}

	return evidence, nil
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
	if evidence.OperationType !=
		claims.OperationType ||
		evidence.ExtensionID !=
			claims.ExtensionID ||
		evidence.ArtifactID !=
			claims.ArtifactID {
		return fmt.Errorf(
			"%w: authority evidence operation identity mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	if evidence.PreviewHash !=
		claims.PreviewHash {
		return fmt.Errorf(
			"%w: evidence previewHash mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	if evidence.SecurityPolicyHash !=
		claims.SecurityPolicyHash {
		return fmt.Errorf(
			"%w: evidence securityPolicyHash mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	if evidence.SnapshotRequirementHash !=
		claims.SnapshotRequirementHash {
		return fmt.Errorf(
			"%w: evidence snapshotRequirementHash mismatch",
			ErrPackageSnapshotRequirementHashMismatch,
		)
	}

	if evidence.DependenciesHash == "" ||
		claims.DependenciesHash == "" ||
		evidence.DependenciesHash !=
			claims.DependenciesHash {
		return fmt.Errorf(
			"%w: evidence dependenciesHash mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	if evidence.RequiredConfirmationsHash == "" ||
		claims.RequiredConfirmationsHash == "" ||
		evidence.RequiredConfirmationsHash !=
			claims.RequiredConfirmationsHash {
		return fmt.Errorf(
			"%w: requiredConfirmationsHash mismatch",
			ErrPackageConfirmationItemsMismatch,
		)
	}

	if err :=
		verifyExactRequiredConfirmations(
			claims.ConfirmedItems,
			evidence.RequiredConfirmations,
		); err != nil {
		return err
	}

	if !validateConfirmedItemsConsistency(
		claims.ConfirmedItems,
		claims.Confirmations,
	) {
		return ErrPackageConfirmationItemsMismatch
	}

	for item, value := range claims.Confirmations {
		if !value {
			return fmt.Errorf(
				"%w: confirmation %s is false",
				ErrPackageConfirmationItemsMismatch,
				item,
			)
		}
	}

	if evidence.Nonce !=
		claims.Nonce ||
		evidence.IssuedAt !=
			claims.IssuedAt ||
		evidence.ExpiresAt !=
			claims.ExpiresAt {
		return fmt.Errorf(
			"%w: evidence claims binding mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	if evidence.OperationType ==
		string(
			PackageOperationTypeRollback,
		) {
		if evidence.TargetVersionID !=
			claims.TargetVersionID ||
			evidence.TargetGenerationID !=
				claims.TargetGenerationID {
			return fmt.Errorf(
				"%w: evidence target identity mismatch",
				ErrPackageConfirmationClaimsInvalid,
			)
		}

		if evidence.RollbackPointID !=
			claims.RollbackPointID {
			return fmt.Errorf(
				"%w: evidence rollback point mismatch",
				ErrPackageConfirmationClaimsInvalid,
			)
		}
	}

	if evidence.OperationType ==
		string(
			PackageOperationTypeUninstall,
		) &&
		evidence.ArtifactPolicy !=
			claims.ArtifactPolicy {
		return fmt.Errorf(
			"%w: evidence artifact policy mismatch",
			ErrPackageConfirmationClaimsInvalid,
		)
	}

	return nil
}

func evidenceRequiredConfirmationsForRollback(required []string) []string {
	return normalizeConfirmedItems(required)
}

var errConfirmationEvidenceSignature = errors.New("kernel: confirmation authority evidence validation failed")

func validateConfirmationAuthorityEvidenceSignature(
	evidence PackageConfirmationAuthorityEvidence,
) error {
	if err := validatePackageConfirmationTemporalBinding(
		evidence.IssuedAt,
		evidence.ExpiresAt,
		evidence.Nonce,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("%w: evidence temporal binding invalid: %v", errConfirmationEvidenceSignature, err)
	}
	if evidence.SchemaVersion !=
		packageConfirmationAuthorityEvidenceSchemaVersion {
		return fmt.Errorf(
			"%w: unsupported evidence schemaVersion %d",
			errConfirmationEvidenceSignature,
			evidence.SchemaVersion,
		)
	}

	if evidence.OperationID == "" ||
		evidence.OperationType == "" ||
		evidence.ExtensionID == "" ||
		evidence.ArtifactID == "" {
		return fmt.Errorf(
			"%w: evidence identity incomplete",
			errConfirmationEvidenceSignature,
		)
	}

	input :=
		evidence.AuthorityInput

	if input.SchemaVersion !=
		packageConfirmationAuthorityInputSchemaVersion ||
		input.Source !=
			packageConfirmationAuthoritySourcePostLeasePreview {
		return fmt.Errorf(
			"%w: authority input schema or source invalid",
			errConfirmationEvidenceSignature,
		)
	}

	if input.OperationType !=
		evidence.OperationType ||
		input.ExtensionID !=
			evidence.ExtensionID ||
		input.ArtifactID !=
			evidence.ArtifactID {
		return fmt.Errorf(
			"%w: authority input identity mismatch",
			errConfirmationEvidenceSignature,
		)
	}

	expectedInputHash :=
		computePackageConfirmationAuthorityInputHash(
			input,
		)

	if evidence.AuthorityInputHash == "" ||
		evidence.AuthorityInputHash !=
			expectedInputHash {
		return fmt.Errorf(
			"%w: authority input hash mismatch",
			errConfirmationEvidenceSignature,
		)
	}

	required :=
		normalizeConfirmedItems(
			input.RequiredConfirmations,
		)

	if computePackageRequiredConfirmationsHash(
		required,
	) != evidence.RequiredConfirmationsHash {
		return fmt.Errorf(
			"%w: required confirmations hash inconsistent",
			errConfirmationEvidenceSignature,
		)
	}

	if err :=
		verifyExactRequiredConfirmations(
			evidence.RequiredConfirmations,
			required,
		); err != nil {
		return fmt.Errorf(
			"%w: evidence required confirmations differ from authority input: %v",
			errConfirmationEvidenceSignature,
			err,
		)
	}

	dependencies :=
		normalizeConfirmedItems(
			input.Dependencies,
		)

	if computePackageDependenciesHash(
		dependencies,
	) != evidence.DependenciesHash {
		return fmt.Errorf(
			"%w: dependencies hash inconsistent",
			errConfirmationEvidenceSignature,
		)
	}

	if err :=
		verifyExactRequiredConfirmations(
			evidence.Dependencies,
			dependencies,
		); err != nil {
		return fmt.Errorf(
			"%w: evidence dependencies differ from authority input: %v",
			errConfirmationEvidenceSignature,
			err,
		)
	}

	if evidence.PreviewHash !=
		input.PreviewHash ||
		evidence.SecurityPolicyHash !=
			input.SecurityPolicyHash ||
		evidence.SnapshotRequirementHash !=
			input.SnapshotRequirementHash ||
		evidence.ArtifactPolicy !=
			input.ArtifactPolicy {
		return fmt.Errorf(
			"%w: evidence authority fields mismatch",
			errConfirmationEvidenceSignature,
		)
	}

	if evidence.Nonce == "" ||
		evidence.IssuedAt == 0 ||
		evidence.ExpiresAt == 0 {
		return fmt.Errorf(
			"%w: evidence claims binding incomplete",
			errConfirmationEvidenceSignature,
		)
	}

	expectedEvidenceHash :=
		computePackageConfirmationAuthorityEvidenceHash(
			evidence,
		)

	if evidence.EvidenceHash == "" ||
		evidence.EvidenceHash !=
			expectedEvidenceHash {
		return fmt.Errorf(
			"%w: evidence hash mismatch",
			errConfirmationEvidenceSignature,
		)
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
