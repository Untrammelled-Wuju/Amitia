package package_distribution

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestServiceCheckAndDownloadSignedRegistryRelease(t *testing.T) {
	server, publicKey := newSignedRegistryServer(t, "2.0.0", false)
	service := newTestService(t, server, publicKey)

	update, err := service.Check(context.Background(), "com.example/update-test", "1.0.0")
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if update == nil || update.Version != "2.0.0" {
		t.Fatalf("unexpected update: %+v", update)
	}

	state, err := service.StartDownload(context.Background(), "com.example/update-test", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("start download: %v", err)
	}
	completed := waitForTestDownload(t, service, state.DownloadID)
	path, metadata, err := service.CompletedDownload(completed.DownloadID)
	if err != nil {
		t.Fatalf("completed download: %v", err)
	}
	if metadata.Version != "2.0.0" {
		t.Fatalf("unexpected metadata version: %s", metadata.Version)
	}
	if !strings.HasSuffix(path, "package.tmp") {
		t.Fatalf("unexpected download path: %s", path)
	}
	service.CleanupDownload(completed.DownloadID)
}

func TestServiceRejectsDownloadHashMismatch(t *testing.T) {
	server, publicKey := newSignedRegistryServer(t, "2.0.0", true)
	service := newTestService(t, server, publicKey)
	state, err := service.StartDownload(context.Background(), "com.example/update-test", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("start download: %v", err)
	}
	completed := waitForTestDownload(t, service, state.DownloadID)
	if completed.Status != DownloadStatusFailed {
		t.Fatalf("expected failed download, got %s", completed.Status)
	}
	if !strings.Contains(strings.ToLower(completed.Error), "hash mismatch") {
		t.Fatalf("expected hash mismatch, got %q", completed.Error)
	}
}

func TestServiceRequiresRegistryConfiguration(t *testing.T) {
	service := NewService(Config{DataDir: t.TempDir()})
	if service.Configured() {
		t.Fatal("empty registry configuration must not be accepted")
	}
	if _, err := service.Check(context.Background(), "com.example/update-test", "1.0.0"); err == nil {
		t.Fatal("expected registry configuration error")
	}
}

func newTestService(t *testing.T, server *httptest.Server, publicKey ed25519.PublicKey) *Service {
	t.Helper()
	service := NewService(Config{
		RegistryURL:           server.URL,
		RegistryPublicKey:     base64.StdEncoding.EncodeToString(publicKey),
		DataDir:               t.TempDir(),
		HostVersion:           "26.2.0",
		RuntimeProfile:        "local",
		Platform:              runtime.GOOS,
		Architecture:          runtime.GOARCH,
		AllowPrivateDownloads: true,
	})
	service.client.client = server.Client()
	service.downloads.client = server.Client()
	return service
}

func newSignedRegistryServer(t *testing.T, version string, corruptHash bool) (*httptest.Server, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	payload := []byte("signed-extension-package")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/extensions/com.example/update-test/latest":
			index := ReleaseIndex{
				SchemaVersion: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				ExtensionID: "com.example/update-test", PublisherID: "com.example",
				PublisherKeyID: "test-key", Channel: "stable", Version: version,
				DownloadURL: "package.amitiax", SignatureURL: "package.amitiax.sig",
				SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
				Platforms: []string{runtime.GOOS}, Architectures: []string{runtime.GOARCH},
				PackageSize: int64(len(payload)), ManifestVersion: 2,
			}
			if !corruptHash {
				index.SHA256 = testSHA256(payload)
			}
			signed, signErr := SignReleaseIndex(index, privateKey)
			if signErr != nil {
				http.Error(w, signErr.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(signed)
		case "/package.amitiax":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, publicKey
}

func waitForTestDownload(t *testing.T, service *Service, downloadID string) DownloadState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, _, err := service.GetDownload(downloadID)
		if err != nil {
			t.Fatalf("get download: %v", err)
		}
		if state.Status == DownloadStatusCompleted || state.Status == DownloadStatusFailed || state.Status == DownloadStatusCancelled {
			return state
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("download did not reach a terminal state")
	return DownloadState{}
}

func testSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
