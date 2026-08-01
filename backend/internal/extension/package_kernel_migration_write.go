//go:build legacy_migration

package extension

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (r *Runtime) MigrateLegacyPackages(ctx context.Context) error {
	svc := NewPackageService(r.Repository, r.Registry, r.Validator, NewWorkflowCompiler(r.Registry), r.Workshop.installer, r.AgentSkills)
	if err := svc.AttachKernelProxy(NewKernelLifecycleProxy(r.Kernel)); err != nil {
		return err
	}
	return svc.MigrateLegacyPackages(ctx)
}

func (r *Runtime) LegacyPackageMetrics() map[string]uint64 {
	svc := NewPackageService(r.Repository, r.Registry, r.Validator, NewWorkflowCompiler(r.Registry), r.Workshop.installer, r.AgentSkills)
	svc.AttachKernelProxy(NewKernelLifecycleProxy(r.Kernel))
	return svc.Metrics()
}

func (s *PackageService) MigrateLegacyPackages(ctx context.Context) error {
	if s == nil || s.repository == nil || s.repository.db == nil {
		return fmt.Errorf("legacy package migration repository unavailable")
	}
	if s.kernelProxy == nil {
		return fmt.Errorf("extension kernel migration unavailable")
	}
	if err := s.migrateLegacyPackageSigners(ctx); err != nil {
		return err
	}
	var candidates []legacyPackageMigrationCandidate
	err := s.repository.db.WithContext(ctx).Raw(`
		SELECT e.extension_id, e.current_version AS version, COALESCE(v.package_blob, X'') AS package_blob,
		COALESCE(e.owner_user_id, '') AS user_id, COALESCE(e.scope_type, 'global') AS scope_type,
		COALESCE(e.scope_id, '') AS scope_id
		FROM extensions e
		LEFT JOIN extension_versions v ON v.extension_id = e.extension_id AND v.version = e.current_version
		WHERE COALESCE(e.archived_at, '') = ''
	`).Scan(&candidates).Error
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		candidate.UserID = strings.TrimSpace(candidate.UserID)
		candidate.ScopeType = strings.TrimSpace(candidate.ScopeType)
		candidate.ScopeID = strings.TrimSpace(candidate.ScopeID)
		if candidate.ScopeType == "" {
			candidate.ScopeType = "global"
		}
		source := "legacy-db:extensions/" + candidate.ExtensionID + "@" + candidate.Version
		state, found, stateErr := s.kernelProxy.legacyMigrationStatus(ctx, candidate.ExtensionID)
		if stateErr != nil {
			return stateErr
		}
		if found && (state == LegacyMigrationCompleted || state == LegacyMigrationManualRequired) {
			continue
		}
		if !found {
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationNotStarted, Source: source}); err != nil {
				return err
			}
		}
		if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationAnalyzing, Source: source}); err != nil {
			return err
		}
		exists, existsErr := s.kernelProxy.extensionExists(ctx, candidate.ExtensionID)
		if existsErr != nil {
			_ = s.kernelProxy.recordLegacyMigrationState(context.Background(), legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationBlocked, Failure: truncateLegacyMigrationFailure(existsErr.Error()), Source: source})
			return existsErr
		}
		if exists {
			container := s.kernelProxy.ReadContainer()
			if container == nil || container.InstallationRepository == nil {
				return fmt.Errorf("extension kernel installation repository unavailable")
			}
			installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(candidate.ExtensionID))
			if err != nil {
				return err
			}
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationCompleted, Source: source, ArtifactID: installation.PackageID}); err != nil {
				return err
			}
			continue
		}
		if len(candidate.PackageBlob) == 0 || candidate.UserID == "" {
			reason := "legacy package blob is unavailable"
			if candidate.UserID == "" {
				reason = "legacy package owner is unavailable"
			}
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationManualRequired, Failure: reason, Source: source}); err != nil {
				return err
			}
			continue
		}
		if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationReady, Source: source}); err != nil {
			return err
		}
		preview, previewErr := s.kernelProxy.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{UserID: candidate.UserID,
			ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID,
			FileName: candidate.ExtensionID + "-" + candidate.Version + ".amitiax"}, bytes.NewReader(candidate.PackageBlob))
		if previewErr != nil {
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationBlocked, Failure: truncateLegacyMigrationFailure(previewErr.Error()), Source: source}); err != nil {
				return err
			}
			continue
		}
		if len(preview.RequiredConfirmations) > 0 || !preview.Installable {
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationManualRequired, Failure: "legacy package requires explicit user confirmation", Source: source, ArtifactID: preview.ArtifactID}); err != nil {
				return err
			}
			continue
		}
		if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationMigrating, Source: source, ArtifactID: preview.ArtifactID}); err != nil {
			return err
		}
		if _, installErr := s.kernelProxy.InstallPreviewedPackage(ctx, kernelruntime.PackageInstallRequest{SessionID: preview.SessionID,
			UserID: candidate.UserID, ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID, ExpectedExtensionID: candidate.ExtensionID}); installErr != nil {
			if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationBlocked, Failure: truncateLegacyMigrationFailure(installErr.Error()), Source: source, ArtifactID: preview.ArtifactID}); err != nil {
				return err
			}
			continue
		}
		if err := s.kernelProxy.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: candidate.ExtensionID, State: LegacyMigrationCompleted, Source: source, ArtifactID: preview.ArtifactID}); err != nil {
			return err
		}
	}
	return nil
}

func truncateLegacyMigrationFailure(value string) string {
	if len(value) > 1000 {
		return value[:1000]
	}
	return value
}

func (s *PackageService) migrateLegacyPackageSigners(ctx context.Context) error {
	if s == nil || s.repository == nil || s.repository.db == nil || s.kernelProxy == nil {
		return fmt.Errorf("legacy signer migration dependencies unavailable")
	}
	if !s.repository.db.Migrator().HasTable("extension_package_signers") {
		return nil
	}
	var records []packageSignerRecord
	if err := s.repository.db.WithContext(ctx).Find(&records).Error; err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, record := range records {
		if record.Fingerprint == "" {
			continue
		}
		keyID := "legacy-" + strings.TrimPrefix(strings.ReplaceAll(record.Fingerprint, ":", "-"), "sha256-")
		if len(keyID) > 96 {
			keyID = keyID[:96]
		}
		item := kernelruntime.PackagePublisherKeyRecord{KeyID: keyID, Fingerprint: record.Fingerprint,
			PublicKey: []byte{}, PublisherID: "legacy:" + record.Fingerprint, TrustSource: "legacy_fingerprint_only",
			TrustLevel: "unknown", KeyState: "unknown", CreatedAt: record.CreatedAt, UpdatedAt: now}
		if item.CreatedAt == "" {
			item.CreatedAt = now
		}
		container := s.kernelProxy.ReadContainer()
		if container == nil || container.PackageTrustRepository == nil {
			return fmt.Errorf("extension kernel trust repository unavailable")
		}
		if err := container.PackageTrustRepository.Put(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
