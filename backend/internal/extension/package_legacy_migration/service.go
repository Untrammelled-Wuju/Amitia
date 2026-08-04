//go:build legacy_migration

package package_legacy_migration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type MigrationService struct {
	source *SourceRepository
	target KernelMigrationTarget

	ownerID string
}

func NewMigrationService(
	source *SourceRepository,
	target KernelMigrationTarget,
) (*MigrationService, error) {
	if source == nil ||
		target == nil {
		return nil,
			fmt.Errorf(
				"legacy migration: service dependencies unavailable",
			)
	}

	return &MigrationService{
		source:  source,
		target:  target,
		ownerID: "legacy-migration-" +
			uuid.NewString(),
	}, nil
}

func hashBytes(
	value []byte,
) string {
	sum := sha256.Sum256(value)

	return "sha256:" +
		hex.EncodeToString(sum[:])
}

func hashPreview(
	preview kernel.InstallPreview,
) string {
	required :=
		append(
			[]string(nil),
			preview.RequiredConfirmations...,
		)

	sort.Strings(required)

	evidence := struct {
		SessionID string `json:"sessionId"`

		ArtifactID  string `json:"artifactId"`
		ExtensionID string `json:"extensionId"`
		Version     string `json:"version"`

		ArchiveHash     string `json:"archiveHash"`
		ManifestHash    string `json:"manifestHash"`
		ArtifactHash    string `json:"artifactHash"`
		ContentTreeHash string `json:"contentTreeHash"`

		Installable bool `json:"installable"`

		RequiredConfirmations []string `json:"requiredConfirmations"`
	}{
		SessionID:
			preview.SessionID,
		ArtifactID:
			preview.ArtifactID,
		ExtensionID:
			preview.ExtensionID,
		Version:
			preview.Version,
		ArchiveHash:
			preview.ArchiveHash,
		ManifestHash:
			preview.ManifestHash,
		ArtifactHash:
			preview.ArtifactHash,
		ContentTreeHash:
			preview.ContentTreeHash,
		Installable:
			preview.Installable,
		RequiredConfirmations:
			required,
	}

	raw, err := json.Marshal(evidence)
	if err != nil {
		panic(err)
	}

	return hashBytes(raw)
}

func normalizeCandidate(
	candidate LegacyPackageCandidate,
) LegacyPackageCandidate {
	candidate.ExtensionID =
		strings.TrimSpace(
			candidate.ExtensionID,
		)

	candidate.Version =
		strings.TrimSpace(
			candidate.Version,
		)

	candidate.UserID =
		strings.TrimSpace(
			candidate.UserID,
		)

	candidate.ScopeType =
		strings.TrimSpace(
			candidate.ScopeType,
		)

	candidate.ScopeID =
		strings.TrimSpace(
			candidate.ScopeID,
		)

	if candidate.ScopeType == "" {
		candidate.ScopeType = "global"
	}

	return candidate
}

func (s *MigrationService) Detect(
	ctx context.Context,
) (MigrationReport, error) {
	var report MigrationReport

	candidates, err :=
		s.source.ListCandidates(ctx)
	if err != nil {
		return report, err
	}

	report.Total = len(candidates)

	for _, candidate :=
		range candidates {
		candidate =
			normalizeCandidate(candidate)

		checkpoint,
			found,
			err :=
			s.target.Status(
				ctx,
				candidate.ExtensionID,
			)
		if err != nil {
			return report, err
		}

		if found {
			switch checkpoint.State {
			case kernel.LegacyMigrationStateCompleted:
				report.Completed++
				continue

			case kernel.LegacyMigrationStateManualRequired:
				report.PendingManual++
				report.PendingExtensions =
					append(
						report.PendingExtensions,
						candidate.ExtensionID,
					)
				continue

			case kernel.LegacyMigrationStateBlocked:
				report.Blocked++
				report.PendingExtensions =
					append(
						report.PendingExtensions,
						candidate.ExtensionID,
					)
				continue
			}
		}

		exists, err :=
			s.target.ExtensionExists(
				ctx,
				candidate.ExtensionID,
			)
		if err != nil {
			return report, err
		}

		if exists {
			report.Completed++
			continue
		}

		report.PendingManual++
		report.PendingExtensions =
			append(
				report.PendingExtensions,
				candidate.ExtensionID,
			)
	}

	sort.Strings(
		report.PendingExtensions,
	)

	return report, nil
}

func (s *MigrationService) migrateSigners(
	ctx context.Context,
) error {
	signers, err :=
		s.source.ListSigners(ctx)
	if err != nil {
		return err
	}

	now :=
		time.Now().UTC().
			Format(time.RFC3339Nano)

	for _, signer :=
		range signers {
		fingerprint :=
			strings.TrimSpace(
				signer.Fingerprint,
			)

		if fingerprint == "" {
			continue
		}

		keyID :=
			"legacy-" +
				strings.TrimPrefix(
					strings.ReplaceAll(
						fingerprint,
						":",
						"-",
					),
					"sha256-",
				)

		if len(keyID) > 96 {
			keyID = keyID[:96]
		}

		createdAt :=
			signer.CreatedAt

		if createdAt == "" {
			createdAt = now
		}

		err :=
			s.target.PutSigner(
				ctx,
				kernel.PackagePublisherKeyRecord{
					KeyID:
						keyID,
					Fingerprint:
						fingerprint,
					PublicKey:
						[]byte{},
					PublisherID:
						"legacy:" +
							fingerprint,
					TrustSource:
						"legacy_fingerprint_only",
					TrustLevel:
						"unknown",
					KeyState:
						"unknown",
					CreatedAt:
						createdAt,
					UpdatedAt:
						now,
				},
			)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *MigrationService) MigrateAll(
	ctx context.Context,
) error {
	if err :=
		s.source.BackfillOwnership(
			ctx,
		); err != nil {
		return err
	}

	if err :=
		s.migrateSigners(
			ctx,
		); err != nil {
		return err
	}

	candidates, err :=
		s.source.ListCandidates(ctx)
	if err != nil {
		return err
	}

	for _, candidate :=
		range candidates {
		if err :=
			s.MigrateOne(
				ctx,
				normalizeCandidate(
					candidate,
				),
			); err != nil {
			return err
		}
	}

	return nil
}

func (s *MigrationService) MigrateOne(
	ctx context.Context,
	candidate LegacyPackageCandidate,
) error {
	if candidate.ExtensionID == "" {
		return fmt.Errorf(
			"legacy migration: extension id missing",
		)
	}

	sourceHash :=
		hashBytes(
			candidate.PackageBlob,
		)

	checkpoint, err :=
		s.target.Acquire(
			ctx,
			candidate.ExtensionID,
			sourceHash,
			s.ownerID,
			5*time.Minute,
		)
	if errors.Is(
		err,
		kernel.ErrLegacyMigrationAlreadyCompleted,
	) {
		return nil
	}
	if err != nil {
		return err
	}

	releaseRequired := true

	defer func() {
		if releaseRequired {
			_ = s.target.Release(
				context.Background(),
				checkpoint,
			)
		}
	}()

	exists, err :=
		s.target.ExtensionExists(
			ctx,
			candidate.ExtensionID,
		)
	if err != nil {
		return err
	}

	if exists {
		artifactID, err :=
			s.target.InstallationArtifactID(
				ctx,
				candidate.ExtensionID,
			)
		if err != nil {
			return err
		}

		if err :=
			s.target.Update(
				ctx,
				checkpoint,
				kernel.LegacyMigrationCheckpointUpdate{
					State:
						kernel.LegacyMigrationStateVerifying,
					CurrentStep:
						"verify_existing_installation",
					ArtifactID:
						artifactID,
				},
			); err != nil {
			return err
		}

		verification, err :=
			s.target.Verify(
				ctx,
				candidate.ExtensionID,
				artifactID,
				"",
				sourceHash,
			)
		if err != nil {
			return err
		}

		if err :=
			s.target.Complete(
				ctx,
				checkpoint,
				verification,
			); err != nil {
			return err
		}

		releaseRequired = false
		return nil
	}

	if len(candidate.PackageBlob) == 0 ||
		candidate.UserID == "" {
		reason :=
			"legacy package blob is unavailable"

		if candidate.UserID == "" {
			reason =
				"legacy package owner is unavailable"
		}

		return s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateManualRequired,
				CurrentStep:
					"manual_required",
				LastError:
					reason,
			},
		)
	}

	preview, err :=
		s.target.Preview(
			ctx,
			kernel.PackagePreviewRequest{
				UserID:
					candidate.UserID,
				ScopeType:
					candidate.ScopeType,
				ScopeID:
					candidate.ScopeID,
				FileName:
					candidate.ExtensionID +
						"-" +
						candidate.Version +
						".amitiax",
			},
			bytes.NewReader(
				candidate.PackageBlob,
			),
		)
	if err != nil {
		_ = s.target.Update(
			context.Background(),
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateBlocked,
				CurrentStep:
					"preview_failed",
				LastError:
					err.Error(),
			},
		)

		return err
	}

	previewHash :=
		hashPreview(preview)

	if err :=
		s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStatePreviewed,
				CurrentStep:
					"previewed",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
			},
		); err != nil {
		return err
	}

	if !preview.Installable ||
		len(
			preview.RequiredConfirmations,
		) > 0 {
		return s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateManualRequired,
				CurrentStep:
					"manual_confirmation_required",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
				LastError:
					"legacy package requires explicit user confirmation",
			},
		)
	}

	if preview.ExtensionID !=
		candidate.ExtensionID {
		return s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateBlocked,
				CurrentStep:
					"preview_identity_mismatch",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
				LastError:
					"preview extension id does not match legacy candidate",
			},
		)
	}

	if err :=
		s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateMigrating,
				CurrentStep:
					"kernel_install",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
			},
		); err != nil {
		return err
	}

	installResult, err :=
		s.target.Install(
			ctx,
			kernel.PackageInstallRequest{
				SessionID:
					preview.SessionID,
				UserID:
					candidate.UserID,
				ScopeType:
					candidate.ScopeType,
				ScopeID:
					candidate.ScopeID,
				ExpectedExtensionID:
					candidate.ExtensionID,
				IdempotencyKey:
					"legacy-migration:" +
						sourceHash,
			},
		)
	if err != nil {
		_ = s.target.Update(
			context.Background(),
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateBlocked,
				CurrentStep:
					"kernel_install_failed",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
				LastError:
					err.Error(),
			},
		)

		return err
	}

	if installResult.ExtensionID !=
		candidate.ExtensionID {
		return fmt.Errorf(
			"legacy migration: installed extension mismatch",
		)
	}

	if err :=
		s.target.Update(
			ctx,
			checkpoint,
			kernel.LegacyMigrationCheckpointUpdate{
				State:
					kernel.LegacyMigrationStateVerifying,
				CurrentStep:
					"verify_kernel_install",
				PreviewHash:
					previewHash,
				ArtifactID:
					preview.ArtifactID,
				OperationID:
					installResult.OperationID,
			},
		); err != nil {
		return err
	}

	verification, err :=
		s.target.Verify(
			ctx,
			candidate.ExtensionID,
			preview.ArtifactID,
			installResult.OperationID,
			sourceHash,
		)
	if err != nil {
		return err
	}

	if err :=
		s.target.Complete(
			ctx,
			checkpoint,
			verification,
		); err != nil {
		return err
	}

	releaseRequired = false
	return nil
}

func (s *MigrationService) Checkpoints(
	ctx context.Context,
) ([]kernel.LegacyMigrationCheckpoint, error) {
	return s.target.List(ctx)
}
