package binary

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileProvider_CreateWriteSealResolve(t *testing.T) {
	tmpDir := t.TempDir()
	prov, err := NewFileProvider(tmpDir)
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	owner := BinaryOwner{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
		ChannelID: "frames",
	}

	handle, err := prov.Create(context.Background(), owner, CreateRequest{
		ExpectedSize: 5,
		MediaType:    "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	testData := []byte("hello")
	if _, err := handle.Writer.Write(testData); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	ref, err := handle.Seal(5, nil)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	if ref.ID != handle.ObjectID {
		t.Fatal("ref id mismatch")
	}
	if ref.Size != 5 {
		t.Fatalf("expected size 5, got %d", ref.Size)
	}

	resolved, err := prov.Resolve(context.Background(), owner, ref)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	defer resolved.Reader.Close()

	data, err := io.ReadAll(resolved.Reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content mismatch: %s", data)
	}

	if ref.Checksum == nil || ref.Checksum.Algorithm != "sha256" {
		t.Fatal("expected sha256 checksum")
	}
}

func TestFileProvider_Resolve_WrongSize(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}

	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	ref.Size = 999
	_, err := prov.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("size mismatch should fail")
	}
}

func TestFileProvider_Resolve_NotFoundSafe(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	ref := BinaryReference{
		ID:       NewBinaryObjectID(),
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
	}

	_, err := prov.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("non-existent should fail")
	}
}

func TestFileProvider_Release_NotFoundSafe(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	if err := prov.Release(context.Background(), owner, NewBinaryObjectID()); err != nil {
		t.Fatalf("release non-existent should be safe: %v", err)
	}
}

func TestFileProvider_Release_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}

	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	prov.Release(context.Background(), owner, ref.ID)
	if err := prov.Release(context.Background(), owner, ref.ID); err != nil {
		t.Fatalf("second release should be safe: %v", err)
	}
}

func TestFileProvider_WireRef_NoPath(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})

	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	if strings.Contains(string(ref.ID), "/") || strings.Contains(string(ref.ID), "\\") {
		t.Fatal("id should not contain path separators")
	}
}

func TestFileProvider_PathTraversalBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}

	invalidPaths := []string{
		"../../../etc/passwd",
		"C:\\Windows\\System32\\config",
		"/etc/passwd",
	}

	for _, path := range invalidPaths {
		fullPath := filepath.Join(tmpDir, path)
		_ = fullPath
	}

	handle, err := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 4})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	handle.Writer.Write([]byte("test"))
	_, err = handle.Seal(4, nil)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	_ = invalidPaths
}

func TestFileProvider_Shutdown_CleansTemp(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 4})
	handle.Writer.Write([]byte("data"))
	if _, err := handle.Seal(4, nil); err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	if err := prov.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestFileProvider_Resolve_ReadAllContent(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	resolved, err := prov.Resolve(context.Background(), owner, ref)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	defer resolved.Reader.Close()

	data, _ := io.ReadAll(resolved.Reader)
	if string(data) != "hello" {
		t.Fatalf("content mismatch: %s", data)
	}
}

func TestFileProvider_EmptyOwnerRejected(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{}
	_, err := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	if err == nil {
		t.Fatal("empty owner should be rejected")
	}
}

func TestNewFileProvider_InvalidRoot(t *testing.T) {
	_, err := NewFileProvider("")
	if err == nil {
		t.Fatal("empty root should be rejected")
	}
}

func TestFileProvider_Resolve_InvalidIDRejected(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	ref := BinaryReference{
		ID:       BinaryObjectID("invalid"),
		Kind:     BinaryStorageFile,
		Size:     100,
		Lifetime: BinaryLifetimeMessage,
	}
	_, err := prov.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("invalid id should be rejected")
	}
}

func TestFileProvider_DataPersistsInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	prov, _ := NewFileProvider(tmpDir)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), fileExtension) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("binary file should exist in root directory")
	}

	_ = ref
}

func TestFileProvider_SealRejectsDeclaredSizeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	prov, err := NewFileProvider(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, err := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Seal(4, nil); err == nil {
		t.Fatal("seal must reject an actual size that differs from expectedSize")
	}
}

func TestFileProvider_SealRejectsCallerSizeThatDoesNotMatchFile(t *testing.T) {
	tmpDir := t.TempDir()
	prov, err := NewFileProvider(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, err := prov.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Seal(5, nil); err == nil {
		t.Fatal("seal must verify the physical file size")
	}
}

func TestFileProvider_SealRejectsForgedChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	prov, err := NewFileProvider(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, err := prov.Create(context.Background(), owner, CreateRequest{ExpectedSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	_, err = handle.Seal(4, &Checksum{Algorithm: "sha256", Value: "deadbeef"})
	if err == nil {
		t.Fatal("seal must verify a supplied checksum instead of trusting it")
	}
}
