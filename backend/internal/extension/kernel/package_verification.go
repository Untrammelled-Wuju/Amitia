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
			err = fmt.Errorf("archive security rejected package")
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
	if reason := r.packageTrustBlockReason(pkg, artifact.ArchiveHash); reason != "" {
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
		if trust.IsBlockingSignatureStatus(verification.Status) || artifact.SignatureStatus == string(trust.SignatureStatusValid) && verification.Status != trust.SignatureStatusValid {
			return nil, fmt.Errorf("stored package signature rejected: %s", verification.Status)
		}
	}
	return pkg, nil
}

func (r *Runtime) packageTrustBlockReason(pkg *amitiax.Package, packageHash string) string {
	service := r.container.TrustService
	if blocked := service.Blocklist().Check(packageHash); blocked != nil {
		return blocked.Details
	}
	if revoked := service.RevocationList().CheckPackage(packageHash); revoked != nil {
		return revoked.Reason
	}
	publisherID := pkg.Manifest.Publisher.ID
	keyID := ""
	if len(pkg.V2Signature) > 0 {
		if document, err := trust.ParseSignatureDocument(pkg.V2Signature); err == nil {
			publisherID = document.PublisherID
			keyID = document.KeyID
		}
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
