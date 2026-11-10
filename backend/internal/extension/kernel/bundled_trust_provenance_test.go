package kernel

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const bundledTrustTestExtensionID = "com.amitia/plugin-market"

var bundledTrustTestVersion = domain.SemanticVersion{Major: 1, Minor: 4, Patch: 4}

type stubBundledArtifactReader struct {
	artifacts map[string]PackageArtifact
}

func (s stubBundledArtifactReader) GetArtifact(_ context.Context, artifactID string) (PackageArtifact, error) {
	artifact, ok := s.artifacts[artifactID]
	if !ok {
		return PackageArtifact{}, errors.New("stub: artifact not found")
	}
	return artifact, nil
}

type stubBundledProvenance struct {
	trusted bool
	calls   int
}

func (s *stubBundledProvenance) IsVerifiedBundledInstallation(context.Context, domain.ExtensionID, domain.SemanticVersion, map[string]any) bool {
	s.calls++
	return s.trusted
}

func bundledTrustFixture(t *testing.T) (domain.InstallationRepository, domain.DefinitionRepository, PackageArtifact) {
	t.Helper()
	artifact := PackageArtifact{
		ArtifactID:      "artifact-sha256-" + bundledTrustTestExtensionID,
		ExtensionID:     bundledTrustTestExtensionID,
		Version:         "1.4.4",
		ArchiveHash:     "sha256:archive",
		ManifestHash:    "sha256:manifest",
		ContentTreeHash: "tree",
		ArtifactHash:    "sha256:artifact",
		SignatureStatus: packageProvenanceBundled,
		TrustDecision:   packageProvenanceBundled,
	}
	definitions := domain.NewInMemoryDefinitionRepository()
	if err := definitions.Put(context.Background(), domain.ExtensionDefinition{
		ID:      domain.ExtensionID(bundledTrustTestExtensionID),
		Version: bundledTrustTestVersion,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			TrustLevel:  packageProvenanceBundled,
		},
	}); err != nil {
		t.Fatalf("put definition: %v", err)
	}
	installations := domain.NewInMemoryInstallationRepository()
	return installations, definitions, artifact
}

func putBundledTrustInstallation(t *testing.T, installations domain.InstallationRepository, metadata map[string]any) {
	t.Helper()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		ExtensionID:       domain.ExtensionID(bundledTrustTestExtensionID),
		InstalledVersion:  bundledTrustTestVersion,
		InstallationState: domain.InstallationStateInstalled,
		Metadata:          metadata,
	}); err != nil {
		t.Fatalf("put installation: %v", err)
	}
}

func verifiedBundledMetadata(artifact PackageArtifact) map[string]any {
	return map[string]any{
		"artifactId":                 artifact.ArtifactID,
		"archiveHash":                artifact.ArchiveHash,
		"manifestHash":               artifact.ManifestHash,
		"contentTreeHash":            artifact.ContentTreeHash,
		packageProvenanceMetadataKey: packageProvenanceBundled,
	}
}

func bundledTrustSubject() permission.PermissionSubject {
	return permission.PermissionSubject{
		Type:        permission.SubjectRuntime,
		ID:          "rt-plugin-market",
		ExtensionID: bundledTrustTestExtensionID,
	}
}

func TestPermissionTrustCheckerAllowsVerifiedBundledInstallation(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	putBundledTrustInstallation(t, installations, verifiedBundledMetadata(artifact))
	checker := newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		}))

	if !checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("bundled package with matching kernel provenance and artifact record must be trusted")
	}
}

func TestPermissionTrustCheckerRejectsBundledTrustWithoutProvenanceMarker(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	metadata := verifiedBundledMetadata(artifact)
	delete(metadata, packageProvenanceMetadataKey)
	putBundledTrustInstallation(t, installations, metadata)
	checker := newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		}))

	if checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("a bundled trust label without a kernel provenance marker must stay untrusted")
	}
}

func TestPermissionTrustCheckerRejectsBundledProvenanceWithoutBundledArtifactRecord(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	artifact.SignatureStatus = "unsigned"
	artifact.TrustDecision = "user_trusted"
	putBundledTrustInstallation(t, installations, verifiedBundledMetadata(artifact))
	checker := newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		}))

	if checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("an installation whose stored artifact record is not bundled must stay untrusted")
	}
}

func TestPermissionTrustCheckerRejectsBundledProvenanceWithForeignArtifact(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	metadata := verifiedBundledMetadata(artifact)
	metadata["archiveHash"] = "sha256:other-archive"
	putBundledTrustInstallation(t, installations, metadata)
	checker := newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		}))

	if checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("provenance metadata that does not match the stored artifact must stay untrusted")
	}
}

func TestPermissionTrustCheckerRejectsBundledProvenanceWithMismatchedVersion(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	artifact.Version = "1.4.3"
	putBundledTrustInstallation(t, installations, verifiedBundledMetadata(artifact))
	checker := newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		}))

	if checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("provenance metadata bound to another package version must stay untrusted")
	}
}

func TestPermissionTrustCheckerRejectsBundledTrustWithoutProvenanceVerifier(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	putBundledTrustInstallation(t, installations, verifiedBundledMetadata(artifact))
	checker := newRepositoryPermissionTrustChecker(installations, definitions)

	if checker.IsTrusted(bundledTrustSubject()) {
		t.Fatal("bundled trust must fail closed when no provenance verifier is attached")
	}
}

func TestBundledInstallationProvenanceRequiresStoredArtifact(t *testing.T) {
	_, _, artifact := bundledTrustFixture(t)
	provenance := newBundledInstallationProvenance(stubBundledArtifactReader{artifacts: map[string]PackageArtifact{}})

	if provenance.IsVerifiedBundledInstallation(context.Background(), domain.ExtensionID(bundledTrustTestExtensionID), bundledTrustTestVersion, verifiedBundledMetadata(artifact)) {
		t.Fatal("missing artifact record must not be treated as verified bundled provenance")
	}
}

func TestBundledInstallationProvenanceRejectsQuarantinedArtifact(t *testing.T) {
	_, _, artifact := bundledTrustFixture(t)
	artifact.QuarantinedAt = "2026-11-10T00:00:00Z"
	provenance := newBundledInstallationProvenance(stubBundledArtifactReader{
		artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
	})

	if provenance.IsVerifiedBundledInstallation(context.Background(), domain.ExtensionID(bundledTrustTestExtensionID), bundledTrustTestVersion, verifiedBundledMetadata(artifact)) {
		t.Fatal("a quarantined artifact must not be treated as verified bundled provenance")
	}
}

func TestPermissionBrokerRejectsTrustedOnlyPermissionForSelfClaimedBundledPackage(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	metadata := verifiedBundledMetadata(artifact)
	delete(metadata, packageProvenanceMetadataKey)
	putBundledTrustInstallation(t, installations, metadata)
	broker := permission.NewDefaultPermissionBroker(permission.NewPermissionDefinitionRegistry(), permission.NewMemoryPermissionStorage())
	broker.SetTrustLevelChecker(newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		})))

	result := broker.Evaluate(context.Background(), permission.PermissionEvaluationRequest{
		Subject: bundledTrustSubject(),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: permission.PermissionServiceNetworkRequest,
			Scope:        permission.ScopeForExtension(bundledTrustTestExtensionID),
		}},
	})

	if result.Decision != permission.DecisionRequireApproval {
		t.Fatalf("self-claimed bundled package must stay on the trusted_only path, got decision=%s reasons=%+v", result.Decision, result.Reasons)
	}
	if !hasPermissionReason(result.Reasons, "trusted_only") {
		t.Fatalf("expected trusted_only rejection reason, got %+v", result.Reasons)
	}
	if hasPermissionReason(result.Reasons, "missing_grant") {
		t.Fatalf("self-claimed bundled package must not reach grant evaluation, reasons=%+v", result.Reasons)
	}
}

func TestPermissionBrokerAllowsTrustedOnlyPermissionForVerifiedBundledPackage(t *testing.T) {
	installations, definitions, artifact := bundledTrustFixture(t)
	putBundledTrustInstallation(t, installations, verifiedBundledMetadata(artifact))
	broker := permission.NewDefaultPermissionBroker(permission.NewPermissionDefinitionRegistry(), permission.NewMemoryPermissionStorage())
	broker.SetTrustLevelChecker(newRepositoryPermissionTrustChecker(installations, definitions).
		withBundledProvenance(newBundledInstallationProvenance(stubBundledArtifactReader{
			artifacts: map[string]PackageArtifact{artifact.ArtifactID: artifact},
		})))

	result := broker.Evaluate(context.Background(), permission.PermissionEvaluationRequest{
		Subject: bundledTrustSubject(),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: permission.PermissionServiceNetworkRequest,
			Scope:        permission.ScopeForExtension(bundledTrustTestExtensionID),
		}},
	})

	if hasPermissionReason(result.Reasons, "trusted_only") {
		t.Fatalf("verified bundled package must pass the trust check, reasons=%+v", result.Reasons)
	}
	if result.Decision != permission.DecisionRequireApproval || !hasPermissionReason(result.Reasons, "missing_grant") {
		t.Fatalf("verified bundled package must fall through to the normal approval flow, got decision=%s reasons=%+v", result.Decision, result.Reasons)
	}
}

const selfClaimedBundledManifest = `{
  "schemaVersion": 1,
  "plugin": {
    "id": "com.example/self-claimed",
    "name": {"default": "Self Claimed"},
    "description": {"default": "Self Claimed"},
    "version": "1.0.0",
    "publisher": {"id": "com.example", "displayName": "Example", "trustLevel": "bundled"}
  },
  "compatibility": {"minHostVersion": "1.0.0", "platforms": ["linux"]},
  "modules": [{
    "id": "main",
    "name": {"default": "Main"},
    "runtime": {"adapter": "amitia.javascript", "entry": "dist/index.js"},
    "dynamic": {"allowedAdapters": ["tool"], "maxTools": 10},
    "contributions": [{
      "id": "lookup",
      "kind": "tool",
      "adapter": "tool",
      "contractVersion": 1,
      "name": {"default": "Lookup"},
      "spec": {"toolId": "lookup"}
    }]
  }],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "tree"}
}`

func TestManifestTrustLevelCannotProduceBundledTrust(t *testing.T) {
	manifest, parseErr := plugincontract.Parse([]byte(selfClaimedBundledManifest))
	if parseErr != nil {
		t.Fatalf("parse manifest: %v", parseErr)
	}
	if manifest.Plugin.Publisher.TrustLevel != packageProvenanceBundled {
		t.Fatalf("fixture must declare a bundled trust level, got %q", manifest.Plugin.Publisher.TrustLevel)
	}
	definition, err := manifest.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("project manifest: %v", err)
	}
	if definition.Publisher.TrustLevel == packageProvenanceBundled {
		t.Fatal("manifest-declared trust level must not survive manifest projection")
	}

	definitions := domain.NewInMemoryDefinitionRepository()
	if err := definitions.Put(context.Background(), definition); err != nil {
		t.Fatalf("put definition: %v", err)
	}
	installations := domain.NewInMemoryInstallationRepository()
	if err := installations.PutInstallation(context.Background(), domain.ExtensionInstallation{
		ExtensionID:       definition.ID,
		InstalledVersion:  definition.Version,
		InstallationState: domain.InstallationStateInstalled,
		Metadata:          map[string]any{},
	}); err != nil {
		t.Fatalf("put installation: %v", err)
	}
	provenance := &stubBundledProvenance{trusted: true}
	checker := newRepositoryPermissionTrustChecker(installations, definitions).withBundledProvenance(provenance)

	if checker.IsTrusted(permission.PermissionSubject{
		Type:        permission.SubjectRuntime,
		ID:          "rt-self-claimed",
		ExtensionID: string(definition.ID),
	}) {
		t.Fatal("a package that only declares a bundled trust level must stay untrusted")
	}
	if provenance.calls != 0 {
		t.Fatal("provenance verification must not run for a non-bundled definition")
	}
}

func hasPermissionReason(reasons []permission.PermissionReason, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
