package kernel

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PackageRollbackRetentionBinding struct {
	RollbackPoint PackageRollbackPoint
	VersionRecord PackageVersionRecord
	Artifact      PackageArtifact
	Reference     PackageArtifactReference
}

func (r *Runtime) verifyRollbackRetentionBinding(
	ctx context.Context,
	extensionID string,
	sourceVersion string,
	artifactID string,
) (PackageRollbackRetentionBinding, error) {
	var empty PackageRollbackRetentionBinding

	if r == nil ||
		r.container == nil ||
		r.container.PackageRepository == nil {
		return empty,
			NewPackageError(
				PackageErrCodeUninstallArtifactPolicyUnproven,
				503,
				fmt.Errorf(
					"kernel: package repository unavailable for rollback retention verification",
				),
			)
	}

	extensionID = strings.TrimSpace(extensionID)
	sourceVersion = strings.TrimSpace(sourceVersion)
	artifactID = strings.TrimSpace(artifactID)

	if extensionID == "" ||
		sourceVersion == "" ||
		artifactID == "" {
		return empty,
			NewPackageError(
				PackageErrCodeUninstallArtifactPolicyUnproven,
				422,
				fmt.Errorf(
					"kernel: rollback retention identity incomplete",
				),
			)
	}

	repository := r.container.PackageRepository

	point, err := repository.GetRollbackPoint(
		ctx,
		extensionID,
		sourceVersion,
	)

	if err != nil {
		return empty,
			NewPackageError(
				PackageErrCodeUninstallArtifactPolicyUnproven,
				422,
				fmt.Errorf(
					"kernel: rollback point unavailable for %s@%s: %w",
					extensionID,
					sourceVersion,
					err,
				),
			)
	}

	if err := validatePackageSnapshot(
		point,
	); err != nil {
		return empty,
			NewPackageError(
				PackageErrCodeRollbackSnapshotCorrupted,
				409,
				fmt.Errorf(
					"kernel: rollback point snapshot invalid: %w",
					err,
				),
			)
	}

	switch {
	case point.RollbackPointID == "":
		return empty,
			fmt.Errorf(
				"kernel: rollback point id missing",
			)
	case point.ExtensionID != extensionID:
		return empty,
			fmt.Errorf(
				"kernel: rollback point extension mismatch: %s != %s",
				point.ExtensionID,
				extensionID,
			)
	case point.SourceVersion != sourceVersion:
		return empty,
			fmt.Errorf(
				"kernel: rollback point source version mismatch: %s != %s",
				point.SourceVersion,
				sourceVersion,
			)
	case point.SourceVersionID == "":
		return empty,
			fmt.Errorf(
				"kernel: rollback point source version id missing",
			)
	case point.SourceGenerationID == "":
		return empty,
			fmt.Errorf(
				"kernel: rollback point source generation id missing",
			)
	case point.SnapshotID == "":
		return empty,
			fmt.Errorf(
				"kernel: rollback point snapshot id missing",
			)
	case point.ArtifactID != artifactID:
		return empty,
			fmt.Errorf(
				"kernel: rollback point artifact mismatch: %s != %s",
				point.ArtifactID,
				artifactID,
			)
	}

	versionRecord, err := repository.GetPackageVersionByID(
		ctx,
		extensionID,
		point.SourceVersionID,
	)

	if err != nil {
		return empty,
			NewPackageError(
				PackageErrCodeVersionHistoryCorrupted,
				409,
				fmt.Errorf(
					"kernel: rollback source version record unavailable: %w",
					err,
				),
			)
	}

	switch {
	case versionRecord.VersionID != point.SourceVersionID:
		return empty,
			fmt.Errorf(
				"kernel: rollback version id mismatch",
			)
	case versionRecord.ExtensionID != point.ExtensionID:
		return empty,
			fmt.Errorf(
				"kernel: rollback version extension mismatch",
			)
	case versionRecord.Version != point.SourceVersion:
		return empty,
			fmt.Errorf(
				"kernel: rollback version label mismatch: record=%s point=%s",
				versionRecord.Version,
				point.SourceVersion,
			)
	case versionRecord.ArtifactID != point.ArtifactID:
		return empty,
			fmt.Errorf(
				"kernel: rollback version artifact mismatch: record=%s point=%s",
				versionRecord.ArtifactID,
				point.ArtifactID,
			)
	case versionRecord.GenerationID != point.SourceGenerationID:
		return empty,
			fmt.Errorf(
				"kernel: rollback version generation mismatch: record=%s point=%s",
				versionRecord.GenerationID,
				point.SourceGenerationID,
			)
	}

	if point.InstalledPath != "" &&
		versionRecord.InstalledPath != "" &&
		versionRecord.InstalledPath != point.InstalledPath {
		return empty,
			fmt.Errorf(
				"kernel: rollback version installed path mismatch",
			)
	}

	artifact, err := repository.GetArtifact(
		ctx,
		artifactID,
	)

	if err != nil {
		return empty,
			NewPackageError(
				PackageErrCodeUninstallArtifactMissing,
				409,
				fmt.Errorf(
					"kernel: retained artifact unavailable: %w",
					err,
				),
			)
	}

	switch {
	case artifact.ArtifactID != point.ArtifactID:
		return empty,
			fmt.Errorf(
				"kernel: retained artifact id mismatch",
			)
	case artifact.ExtensionID != point.ExtensionID:
		return empty,
			fmt.Errorf(
				"kernel: retained artifact extension mismatch",
			)
	case artifact.Version != point.SourceVersion:
		return empty,
			fmt.Errorf(
				"kernel: retained artifact version mismatch: artifact=%s point=%s",
				artifact.Version,
				point.SourceVersion,
			)
	case artifact.RetentionState == "deleted":
		return empty,
			fmt.Errorf(
				"kernel: retained artifact is deleted",
			)
	case artifact.DeletedAt != "":
		return empty,
			fmt.Errorf(
				"kernel: retained artifact has deletedAt",
			)
	}

	references, err := repository.ListActiveArtifactReferences(
		ctx,
		artifactID,
		ArtifactReferenceRollbackPoint,
		point.RollbackPointID,
	)

	if err != nil {
		return empty,
			fmt.Errorf(
				"kernel: list rollback point references: %w",
				err,
			)
	}

	if len(references) != 1 {
		return empty,
			NewPackageError(
				PackageErrCodeArtifactReferenceMismatch,
				409,
				fmt.Errorf(
					"kernel: expected exactly one active rollback reference for %s, found %d",
					point.RollbackPointID,
					len(references),
				),
			)
	}

	reference := references[0]

	switch {
	case reference.ArtifactID != artifactID:
		return empty,
			fmt.Errorf(
				"kernel: rollback reference artifact mismatch",
			)
	case reference.ReferenceType != ArtifactReferenceRollbackPoint:
		return empty,
			fmt.Errorf(
				"kernel: rollback reference type mismatch",
			)
	case reference.ReferenceOwnerID != point.RollbackPointID:
		return empty,
			fmt.Errorf(
				"kernel: rollback reference owner mismatch",
			)
	case reference.ReleasedAt != "":
		return empty,
			fmt.Errorf(
				"kernel: rollback reference already released",
			)
	}

	if reference.ExpiresAt != "" {
		referenceExpiresAt, parseErr := time.Parse(
			time.RFC3339Nano,
			reference.ExpiresAt,
		)

		if parseErr != nil {
			return empty,
				fmt.Errorf(
					"kernel: rollback reference expiration invalid: %w",
					parseErr,
				)
		}

		if !time.Now().UTC().Before(
			referenceExpiresAt,
		) {
			return empty,
				fmt.Errorf(
					"kernel: rollback reference expired",
				)
		}
	}

	expectedReferenceExpiry := point.RetentionUntil
	if expectedReferenceExpiry == "" {
		expectedReferenceExpiry = point.ExpiresAt
	}

	if expectedReferenceExpiry != "" &&
		reference.ExpiresAt != expectedReferenceExpiry {
		return empty,
			fmt.Errorf(
				"kernel: rollback reference expiration does not match rollback point retention",
			)
	}

	return PackageRollbackRetentionBinding{
		RollbackPoint: point,
		VersionRecord: versionRecord,
		Artifact:      artifact,
		Reference:     reference,
	}, nil
}
