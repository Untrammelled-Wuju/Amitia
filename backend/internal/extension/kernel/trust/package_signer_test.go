package trust

import (
	"context"
	"testing"
)

func TestPackageSignerPreservesPayloadIdentity(t *testing.T) {
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID := ComputeKeyID(publicKey)
	signer := NewSigner("com.example", keyID, privateKey)
	doc, _, err := NewPackageSigner(signer).SignPackage(PackageSignatureInput{
		ExtensionID:     "com.example/plugin",
		Version:         "1.0.0",
		ManifestVersion: 1,
		ManifestHash:    "sha256:manifest",
		ContentTreeHash: "tree",
		ArtifactHash:    "sha256:artifact",
		Channel:         "beta",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewPublisherStore()
	if err := store.RegisterDevelopment("com.example", PublisherKey{
		KeyID:       keyID,
		PublisherID: "com.example",
		PublicKey:   publicKey,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
	}); err != nil {
		t.Fatal(err)
	}
	result := NewSignatureVerifier(store).VerifyPackage(context.Background(), PackageVerificationInput{
		Document:              doc,
		ActualExtensionID:     "com.example/plugin",
		ActualVersion:         "1.0.0",
		ActualManifestVersion: 1,
		ActualManifestHash:    "sha256:manifest",
		ActualContentTreeHash: "tree",
		ActualArtifactHash:    "sha256:artifact",
	})
	if !IsSignatureValid(result) {
		t.Fatalf("signature verification failed: %s: %s", result.Status, result.Reason)
	}
}
