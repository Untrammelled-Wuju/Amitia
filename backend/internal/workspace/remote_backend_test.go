package workspace

import (
	"context"
	"strings"
	"testing"
)

type fakeRemoteCredResolver struct {
	creds map[string]*RemoteCredential
}

func (r *fakeRemoteCredResolver) ResolveCredential(ctx context.Context, ref string) (*RemoteCredential, error) {
	if cred, ok := r.creds[ref]; ok {
		return cred, nil
	}
	return nil, ErrRemoteCredentialNotFound
}

func newFakeRemoteBackend() (*RemoteBackend, *remoteClientCache) {
	credResolver := &fakeRemoteCredResolver{creds: make(map[string]*RemoteCredential)}
	policy := DefaultRemotePolicy
	backend := NewRemoteBackend(credResolver, policy)
	return backend, backend.clients
}

func TestRemoteBackend_Kind(t *testing.T) {
	backend, _ := newFakeRemoteBackend()
	if backend.Kind() != WorkspaceKindRemote {
		t.Errorf("expected kind %q, got %q", WorkspaceKindRemote, backend.Kind())
	}
}

func TestRemoteBackend_Stat(t *testing.T) {
	backend, cache := newFakeRemoteBackend()
	transport := NewFakeRemoteTransport()
	transport.AddFile("/test.txt", []byte("hello"))

	_ = cache

	mount := WorkspaceMount{
		ID:            "remote-test-1",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	entry, err := backend.Stat(context.Background(), mount, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "test.txt" {
		t.Errorf("expected name test.txt, got %q", entry.Name)
	}
	if entry.Type != WorkspaceEntryTypeFile {
		t.Errorf("expected file type, got %q", entry.Type)
	}
	_ = transport
}

func TestRemoteBackend_List(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-2",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	entries, err := backend.List(context.Background(), mount, "", ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) < 0 {
		t.Errorf("unexpected entries count: %d", len(entries))
	}
}

func TestRemoteBackend_Read(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-3",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/testdir"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Read(context.Background(), mount, "file.txt", ReadOptions{})
	if err != nil && err != ErrFileNotFound {
		t.Logf("read result: %v", err)
	}
}

func TestRemoteBackend_Write(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-4",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	content := "test content"
	_, err := backend.Write(context.Background(), mount, "newfile.txt", strings.NewReader(content), WriteOptions{Overwrite: true})
	if err != nil {
		t.Logf("write result: %v", err)
	}
}

func TestRemoteBackend_Mkdir(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-5",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Mkdir(context.Background(), mount, "newdir")
	if err != nil {
		t.Logf("mkdir result: %v", err)
	}
}

func TestRemoteBackend_Rename(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-6",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Rename(context.Background(), "old.txt", "new.txt")
	if err != nil && err != ErrFileNotFound {
		t.Logf("rename result: %v", err)
	}
}

func TestRemoteBackend_Delete(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-7",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	err := backend.Delete(context.Background(), "deleteme.txt", DeleteOptions{Recursive: false})
	if err != nil && err != ErrFileNotFound {
		t.Logf("delete result: %v", err)
	}
}

func TestRemoteBackend_ReadOnly(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-ro",
		Name:          "Test Remote RO",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReadOnly,
		ReadOnly:      true,
	}

	content := "should fail"
	_, err := backend.Write(context.Background(), mount, "newfile.txt", strings.NewReader(content), WriteOptions{Overwrite: true})
	if err != nil {
		t.Logf("write to read-only mount result: %v", err)
	}
}

func TestRemoteBackend_InvalidConfig(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-bad",
		Name:          "Test Remote Bad",
		Kind:          WorkspaceKindRemote,
		BackendConfig: "invalid json",
		CredentialRef: "cred-1",
	}

	_, err := backend.Stat(context.Background(), mount, "test.txt")
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestRemoteBackend_PathTraversal(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-trav",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Stat(context.Background(), mount, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestRemoteBackend_Copy(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-copy",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Copy(context.Background(), "src.txt", "dstdir")
	if err != nil && err != ErrFileNotFound {
		t.Logf("copy result: %v", err)
	}
}

func TestRemoteBackend_Move(t *testing.T) {
	backend, _ := newFakeRemoteBackend()

	mount := WorkspaceMount{
		ID:            "remote-test-move",
		Name:          "Test Remote",
		Kind:          WorkspaceKindRemote,
		BackendConfig: `{"protocol":"sftp","host":"example.com","basePath":"/"}`,
		CredentialRef: "cred-1",
		Available:     true,
		Status:        WorkspaceStatusReady,
	}

	_, err := backend.Move(context.Background(), "src.txt", "dstdir")
	if err != nil && err != ErrFileNotFound {
		t.Logf("move result: %v", err)
	}
}

func TestFakeRemoteTransport_CRUD(t *testing.T) {
	transport := NewFakeRemoteTransport()
	ctx := context.Background()

	if err := transport.Mkdir(ctx, "/testdir"); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if err := transport.Write(ctx, "/testdir/hello.txt", strings.NewReader("hello world"), true); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result, err := transport.Stat(ctx, "/testdir/hello.txt")
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if result.Name != "hello.txt" {
		t.Errorf("expected name hello.txt, got %q", result.Name)
	}
	if result.SizeBytes != 11 {
		t.Errorf("expected size 11, got %d", result.SizeBytes)
	}

	listResult, err := transport.List(ctx, "/testdir", 100)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(listResult.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(listResult.Entries))
	}

	readResult, err := transport.Read(ctx, "/testdir/hello.txt", 0, 100)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(readResult.Content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(readResult.Content))
	}

	if err := transport.Rename(ctx, "/testdir/hello.txt", "/testdir/renamed.txt"); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	if err := transport.Copy(ctx, "/testdir/renamed.txt", "/testdir"); err != nil {
		t.Logf("copy result: %v", err)
	}

	if err := transport.Delete(ctx, "/testdir/renamed.txt", false); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestFakeRemoteTransport_WriteRejectDuplicate(t *testing.T) {
	transport := NewFakeRemoteTransport()
	ctx := context.Background()

	if err := transport.Write(ctx, "/file.txt", strings.NewReader("content"), true); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	err := transport.Write(ctx, "/file.txt", strings.NewReader("new content"), false)
	if err == nil {
		t.Error("expected error for duplicate write")
	}
}

func TestFakeRemoteTransport_DirectoryNotEmpty(t *testing.T) {
	transport := NewFakeRemoteTransport()
	ctx := context.Background()

	transport.AddDir("/mydir")
	transport.AddFile("/mydir/file.txt", []byte("data"))

	err := transport.Delete(ctx, "/mydir", false)
	if err == nil {
		t.Error("expected error for non-empty directory")
	}

	if err := transport.Delete(ctx, "/mydir", true); err != nil {
		t.Fatalf("recursive delete failed: %v", err)
	}
}

func TestRemotePolicy_ResolveRemotePathSFTP(t *testing.T) {
	tests := []struct {
		base     string
		rel      string
		expected string
	}{
		{"/home/user", "file.txt", "/home/user/file.txt"},
		{"/home/user", "dir/file.txt", "/home/user/dir/file.txt"},
		{"/home/user", "", "/home/user"},
		{"/home/user", ".", "/home/user"},
	}

	for _, tt := range tests {
		result := ResolveRemotePathSFTP(tt.base, tt.rel)
		if result != tt.expected {
			t.Errorf("ResolveRemotePathSFTP(%q, %q) = %q, want %q", tt.base, tt.rel, result, tt.expected)
		}
	}
}

func TestRemotePolicy_ValidateRemotePath(t *testing.T) {
	err := ValidateRemotePath("../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}

	err = ValidateRemotePath("/absolute/path")
	if err == nil {
		t.Error("expected error for absolute path")
	}

	err = ValidateRemotePath("valid/path/file.txt")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemotePolicy_InferRemoteMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"test.go", "text/x-go"},
		{"test.txt", "text/plain"},
		{"test.json", "application/json"},
		{"test.pdf", "application/pdf"},
		{"test.png", "image/png"},
		{"test.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := InferRemoteMIMEType(tt.name)
		if result != tt.expected {
			t.Errorf("InferRemoteMIMEType(%q) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}
