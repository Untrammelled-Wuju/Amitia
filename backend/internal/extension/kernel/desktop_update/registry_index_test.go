package desktop_update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signedIndex(t *testing.T, privateKey ed25519.PrivateKey) ReleaseIndex {
	t.Helper()
	index := ReleaseIndex{
		SchemaVersion: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ExtensionID: "dev.amitia.test", PublisherID: "dev.amitia",
		PublisherKeyID: "key-1", Channel: "stable", Version: "2.0.0",
		DownloadURL:    "https://downloads.amitia.dev/test.amitiax",
		SHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SignatureURL:   "https://downloads.amitia.dev/test.amitiax.sig",
		MinHostVersion: "1.0.0", Platforms: []string{"win32"},
		PackageSize: 128, ManifestVersion: 1,
	}
	payload, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	index.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return index
}

func TestRegistryClientSignedIndex(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	index := signedIndex(t, privateKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(index)
	}))
	defer server.Close()
	client := NewRegistryClient(server.URL)
	if err := client.SetPublicKeyBase64(base64.StdEncoding.EncodeToString(publicKey)); err != nil {
		t.Fatal(err)
	}
	metadata, err := client.QueryExtension(context.Background(), index.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Version != index.Version || metadata.PackageSHA256 != index.SHA256 {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
}

func TestSignReleaseIndex(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	index := signedIndex(t, privateKey)
	index.Signature = ""
	payload, err := SignReleaseIndex(index, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var signed ReleaseIndex
	if json.Unmarshal(payload, &signed) != nil {
		t.Fatal("signed index is not valid json")
	}
	client := NewRegistryClient("https://registry.invalid")
	if err := client.SetPublicKeyBase64(base64.StdEncoding.EncodeToString(publicKey)); err != nil {
		t.Fatal(err)
	}
	if err := client.verifyReleaseIndex(signed); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryClientRejectsInvalidIndexSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	index := signedIndex(t, privateKey)
	index.Version = "2.0.1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(index)
	}))
	defer server.Close()
	client := NewRegistryClient(server.URL)
	if err := client.SetPublicKeyBase64(base64.StdEncoding.EncodeToString(publicKey)); err != nil {
		t.Fatal(err)
	}
	_, err = client.QueryExtension(context.Background(), index.ExtensionID)
	if ErrorCodeOf(err) != ErrorCodeIndexSignatureInvalid || !errors.Is(err, ErrIndexSignatureInvalid) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateManagerRestoresOperation(t *testing.T) {
	dir := t.TempDir()
	manager := NewUpdateManager(dir, "1.0.0")
	metadata := validMetadata()
	operation, err := manager.CreateUpdateOperation(context.Background(), metadata.ExtensionID, metadata)
	if err != nil {
		t.Fatal(err)
	}
	restored := NewUpdateManager(dir, "1.0.0")
	loaded, ok := restored.GetOperation(operation.OperationID)
	if !ok || loaded.Status != StateCreated || loaded.ExtensionID != metadata.ExtensionID {
		t.Fatalf("operation was not restored: %+v", loaded)
	}
	if len(restored.Journal().GetEntries(operation.OperationID)) == 0 {
		t.Fatal("journal was not restored")
	}
}
