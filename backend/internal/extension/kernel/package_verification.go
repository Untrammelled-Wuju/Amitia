package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func (r *Runtime) VerifyStoredPackage(ctx context.Context, artifact PackageArtifact) (*amitiax.Package, error) {
	if err := r.container.PackageArtifactStore.VerifyArchive(artifact); err != nil {
		return nil, err
	}
	report, err := r.container.PackageSecurity.InspectFile(ctx, artifact.ArchivePath, package_security.PackageSource{SourceType: package_security.SourceLocalFile, LocalPath: artifact.ArchivePath})
	if err != nil || report == nil || !report.Passed {
		if err == nil {
			err = fmt.Errorf("archive security rejected package (%s)", securityRejectionDetail(report))
		}
		return nil, err
	}
	pkg, err := amitiax.OpenArchive(artifact.ArchivePath)
	if err != nil {
		return nil, err
	}
	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		return nil, err
	}
	if computeManifestHash(pkg) != artifact.ManifestHash || pkg.Tree.TreeHash != artifact.ContentTreeHash || computeArtifactHashFromPackage(pkg) != artifact.ArtifactHash {
		return nil, fmt.Errorf("stored package identity mismatch")
	}
	if reason := r.packageTrustBlockReason(ctx, pkg, artifact.ArchiveHash); reason != "" {
		return nil, fmt.Errorf("package trust policy rejected artifact: %s", reason)
	}
	if len(pkg.V2Signature) > 0 {
		document, err := trust.ParseSignatureDocument(pkg.V2Signature)
		if err != nil {
			return nil, err
		}
		verification := r.container.TrustService.Verifier().VerifyPackage(ctx, trust.PackageVerificationInput{
			Document: document, ActualExtensionID: artifact.ExtensionID, ActualVersion: artifact.Version,
			ActualManifestVersion: pkg.Manifest.ManifestVersion, ActualManifestHash: artifact.ManifestHash,
			ActualContentTreeHash: artifact.ContentTreeHash, ActualArtifactHash: artifact.ArtifactHash,
		})
		if verification.Status != trust.SignatureStatusValid || artifact.SignatureStatus != string(trust.SignatureStatusValid) {
			return nil, fmt.Errorf("stored package signature rejected: %s", verification.Status)
		}
		identity, identityErr := r.container.TrustService.Store().Get(ctx, document.PublisherID)
		if identityErr != nil || identity.TrustLevel != trust.TrustLevelOfficial && identity.TrustLevel != trust.TrustLevelTrusted && identity.TrustLevel != trust.TrustLevelUserTrusted {
			return nil, fmt.Errorf("stored package publisher is not trusted")
		}
	} else if artifact.TrustDecision != string(trust.TrustLevelDevelopment) || artifact.SignatureStatus != "unsigned" {
		return nil, fmt.Errorf("stored package signature required")
	}
	return pkg, nil
}

func (r *Runtime) packageTrustBlockReason(ctx context.Context, pkg *amitiax.Package, packageHash string) string {
	service := r.container.TrustService
	publisherID := pkg.Manifest.Publisher.ID
	keyID := ""
	if len(pkg.V2Signature) > 0 {
		if document, err := trust.ParseSignatureDocument(pkg.V2Signature); err == nil {
			publisherID = document.PublisherID
			keyID = document.KeyID
		}
	}
	if r.container.PackageTrustRepository == nil {
		return "trust policy repository unavailable"
	}
	pendingReason, err := r.container.PackageTrustRepository.PendingRestrictionReason(ctx, publisherID, "", packageHash)
	if err != nil {
		return "trust policy repository read failed"
	}
	if pendingReason != "" {
		return pendingReason
	}
	if blocked := service.Blocklist().Check(packageHash); blocked != nil {
		return blocked.Details
	}
	if revoked := service.RevocationList().CheckPackage(packageHash); revoked != nil {
		return revoked.Reason
	}
	if revoked := service.RevocationList().CheckKey(publisherID, keyID); revoked != nil {
		return revoked.Reason
	}
	if revoked := service.RevocationList().CheckPublisher(publisherID); revoked != nil {
		return revoked.Reason
	}
	if revoked := service.RevocationList().CheckExtension(pkg.Manifest.Extension.ID, pkg.Manifest.Extension.Version); revoked != nil {
		return revoked.Reason
	}
	return ""
}
