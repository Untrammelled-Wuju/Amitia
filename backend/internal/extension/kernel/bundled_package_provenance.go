package kernel

import (
	"context"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	packageProvenanceMetadataKey          = "packageProvenance"
	packageProvenanceCatalogMetadataKey   = "packageProvenanceCatalog"
	packageProvenanceURIMetadataKey       = "packageProvenanceUri"
	packageProvenancePublisherMetadataKey = "packageProvenancePublisher"
	packageProvenanceBundled              = "bundled"
)

var packageProvenanceMetadataKeys = []string{
	packageProvenanceMetadataKey,
	packageProvenanceCatalogMetadataKey,
	packageProvenanceURIMetadataKey,
	packageProvenancePublisherMetadataKey,
}

func applyVerifiedPackageProvenance(metadata map[string]any, preview InstallPreview) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	if !isVerifiedBundledPreview(preview) {
		for _, key := range packageProvenanceMetadataKeys {
			delete(metadata, key)
		}
		return metadata
	}
	metadata[packageProvenanceMetadataKey] = packageProvenanceBundled
	metadata[packageProvenanceCatalogMetadataKey] = strings.TrimSpace(preview.Source.CatalogID)
	metadata[packageProvenanceURIMetadataKey] = strings.TrimSpace(preview.Source.URI)
	metadata[packageProvenancePublisherMetadataKey] = strings.TrimSpace(preview.Source.Publisher)
	return metadata
}

func isVerifiedBundledPreview(preview InstallPreview) bool {
	if !strings.EqualFold(strings.TrimSpace(preview.TrustDecision), packageProvenanceBundled) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(preview.SignatureStatus), packageProvenanceBundled)
}

func applyStoredBundledProvenance(ctx context.Context, repo bundledArtifactReader, metadata map[string]any, artifactID, archiveHash string) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	if repo == nil || strings.TrimSpace(artifactID) == "" || strings.TrimSpace(archiveHash) == "" {
		for _, key := range packageProvenanceMetadataKeys {
			delete(metadata, key)
		}
		return metadata
	}
	artifact, err := repo.GetArtifact(ctx, artifactID)
	if err != nil || artifact.DeletedAt != "" || artifact.QuarantinedAt != "" {
		for _, key := range packageProvenanceMetadataKeys {
			delete(metadata, key)
		}
		return metadata
	}
	if artifact.ArchiveHash != archiveHash || !isBundledTrustRecord(artifact) {
		for _, key := range packageProvenanceMetadataKeys {
			delete(metadata, key)
		}
		return metadata
	}
	metadata[packageProvenanceMetadataKey] = packageProvenanceBundled
	metadata[packageProvenanceCatalogMetadataKey] = packageProvenanceBundled
	metadata[packageProvenanceURIMetadataKey] = "bundled:" + strings.TrimSpace(artifact.Version)
	metadata[packageProvenancePublisherMetadataKey] = strings.TrimSpace(artifact.PublisherID)
	return metadata
}

func isBundledTrustRecord(artifact PackageArtifact) bool {
	if !strings.EqualFold(strings.TrimSpace(artifact.SignatureStatus), packageProvenanceBundled) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(artifact.TrustDecision), packageProvenanceBundled)
}

type bundledArtifactReader interface {
	GetArtifact(ctx context.Context, artifactID string) (PackageArtifact, error)
}

type bundledInstallationProvenance struct {
	artifacts bundledArtifactReader
}

func newBundledInstallationProvenance(repo bundledArtifactReader) *bundledInstallationProvenance {
	return &bundledInstallationProvenance{artifacts: repo}
}

func (p *bundledInstallationProvenance) IsVerifiedBundledInstallation(ctx context.Context, extensionID domain.ExtensionID, version domain.SemanticVersion, metadata map[string]any) bool {
	if p == nil || p.artifacts == nil || len(metadata) == 0 {
		return false
	}
	if !strings.EqualFold(provenanceMetadataString(metadata, packageProvenanceMetadataKey), packageProvenanceBundled) {
		return false
	}
	artifactID := provenanceMetadataString(metadata, "artifactId")
	archiveHash := provenanceMetadataString(metadata, "archiveHash")
	manifestHash := provenanceMetadataString(metadata, "manifestHash")
	contentTreeHash := provenanceMetadataString(metadata, "contentTreeHash")
	if artifactID == "" || archiveHash == "" || manifestHash == "" || contentTreeHash == "" {
		return false
	}
	artifact, err := p.artifacts.GetArtifact(ctx, artifactID)
	if err != nil {
		return false
	}
	if artifact.DeletedAt != "" || artifact.QuarantinedAt != "" {
		return false
	}
	if artifact.ExtensionID != string(extensionID) || artifact.Version != version.String() {
		return false
	}
	if artifact.ArchiveHash != archiveHash || artifact.ManifestHash != manifestHash || artifact.ContentTreeHash != contentTreeHash {
		return false
	}
	return isBundledTrustRecord(artifact)
}

func provenanceMetadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(metadataString(value))
}
