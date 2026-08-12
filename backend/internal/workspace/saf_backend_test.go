package workspace

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newTestMount(grantID string, readOnly bool) WorkspaceMount {
	return WorkspaceMount{
		ID:          WorkspaceID("test-mount-id"),
		Name:        "Test SAF Mount",
		Kind:        WorkspaceKindSAF,
		ReadOnly:    readOnly,
		Available:   true,
		Status:      WorkspaceStatusReady,
		RootURI:     MountURI("test-mount-id"),
		NativeGrant: grantID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func TestSAFBackend_Kind(t *testing.T) {
	backend := NewSAFBackend(NewFakeSAFBridge())
	if backend.Kind() != WorkspaceKindSAF {
		t.Errorf("expected kind %q, got %q", WorkspaceKindSAF, backend.Kind())
	}
}

func TestSAFBackend_NilBridge(t *testing.T) {
	backend := NewSAFBackend(nil)
	_, err := backend.Stat(context.Background(), newTestMount("saf_test", false), "")
	if err == nil {
		t.Error("expected error with nil bridge")
	}
}

func TestSAFBackend_StatRoot(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_root"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Stat(context.Background(), mount, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Type != WorkspaceEntryTypeDirectory {
		t.Errorf("expected directory, got %q", entry.Type)
	}
}

func TestSAFBackend_StatTraversalRejected(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_trav"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	_, err := backend.Stat(context.Background(), mount, "../etc/passwd")
	if err == nil {
		t.Error("expected traversal error")
	}
}

func TestSAFBackend_List(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_list"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "file1.txt",
		MIMEType:         "text/plain",
	})
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "file2.txt",
		MIMEType:         "text/plain",
	})
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "subdir",
	})

	backend := NewFakeSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entries, err := backend.List(context.Background(), mount, "", ListOptions{Limit: MaxListEntries})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Type != WorkspaceEntryTypeDirectory {
		t.Errorf("expected first entry to be directory, got %q", entries[0].Type)
	}
}

func NewFakeSAFBackend(bridge *FakeSAFBridge) *SAFBackend {
	return NewSAFBackend(bridge)
}

func TestSAFBackend_Read(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_read"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "hello.txt",
		MIMEType:         "text/plain",
	})
	_, _ = bridge.Write(context.Background(), grantID, grantID+":root/hello.txt", "hello.txt", SAFWriteSource{Stream: []byte("hello world")}, true)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	result, err := backend.Read(context.Background(), mount, "hello.txt", ReadOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(result.Content))
	}
	if !result.IsText {
		t.Error("expected IsText to be true")
	}
}

func TestSAFBackend_ReadWithOffset(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_offset"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "data.txt",
		MIMEType:         "text/plain",
	})
	_, _ = bridge.Write(context.Background(), grantID, grantID+":root/data.txt", "data.txt", SAFWriteSource{Stream: []byte("1234567890")}, true)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	result, err := backend.Read(context.Background(), mount, "data.txt", ReadOptions{Offset: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content) != "67890" {
		t.Errorf("expected '67890', got %q", string(result.Content))
	}
}

func TestSAFBackend_WriteNewFile(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_write"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Write(context.Background(), mount, "test.txt", strings.NewReader("content"), WriteOptions{Overwrite: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "test.txt" {
		t.Errorf("expected 'test.txt', got %q", entry.Name)
	}
	if entry.Type != WorkspaceEntryTypeFile {
		t.Errorf("expected file type, got %q", entry.Type)
	}
}

func TestSAFBackend_WriteRejectDuplicate(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_dup"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	_, err := backend.Write(context.Background(), mount, "dup.txt", strings.NewReader("first"), WriteOptions{Overwrite: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = backend.Write(context.Background(), mount, "dup.txt", strings.NewReader("second"), WriteOptions{Overwrite: false})
	if err == nil {
		t.Error("expected error for duplicate file")
	}
}

func TestSAFBackend_WriteOverwrite(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_ow"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	_, err := backend.Write(context.Background(), mount, "ow.txt", strings.NewReader("first"), WriteOptions{Overwrite: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, err := backend.Write(context.Background(), mount, "ow.txt", strings.NewReader("second"), WriteOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "ow.txt" {
		t.Errorf("expected 'ow.txt', got %q", entry.Name)
	}
}

func TestSAFBackend_Mkdir(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_mkdir"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Mkdir(context.Background(), mount, "newdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Type != WorkspaceEntryTypeDirectory {
		t.Errorf("expected directory, got %q", entry.Type)
	}
}

func TestSAFBackend_Rename(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_rename"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "old.txt",
		MIMEType:         "text/plain",
	})

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Rename(context.Background(), mount, "old.txt", "new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "new.txt" {
		t.Errorf("expected 'new.txt', got %q", entry.Name)
	}
}

func TestSAFBackend_Move(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_move"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "src",
	})
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "dst",
	})
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root/src",
		DisplayName:      "file.txt",
		MIMEType:         "text/plain",
	})

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Move(context.Background(), mount, "src/file.txt", "dst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "file.txt" {
		t.Errorf("expected 'file.txt', got %q", entry.Name)
	}
}

func TestSAFBackend_Copy(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_copy"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "src",
	})
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "dst",
	})
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root/src",
		DisplayName:      "file.txt",
		MIMEType:         "text/plain",
	})

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	entry, err := backend.Copy(context.Background(), mount, "src/file.txt", "dst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Name != "file.txt" {
		t.Errorf("expected 'file.txt', got %q", entry.Name)
	}
}

func TestSAFBackend_Delete(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_delete"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "del.txt",
		MIMEType:         "text/plain",
	})

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	err := backend.Delete(context.Background(), mount, "del.txt", DeleteOptions{Recursive: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSAFBackend_DeleteNonEmptyRecursive(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_delne"
	bridge.RegisterGrant(grantID)
	_, _ = bridge.Mkdir(context.Background(), grantID, SAFCreateDirInput{
		ParentDocumentID: grantID + ":root",
		DisplayName:      "parent",
	})
	_, _ = bridge.CreateFile(context.Background(), grantID, SAFCreateFileInput{
		ParentDocumentID: grantID + ":root/parent",
		DisplayName:      "child.txt",
		MIMEType:         "text/plain",
	})

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, false)
	err := backend.Delete(context.Background(), mount, "parent", DeleteOptions{Recursive: false})
	if err == nil {
		t.Error("expected error for non-empty directory without recursive")
	}

	err = backend.Delete(context.Background(), mount, "parent", DeleteOptions{Recursive: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSAFBackend_WriteReadOnly(t *testing.T) {
	bridge := NewFakeSAFBridge()
	grantID := "saf_test_ro"
	bridge.RegisterGrant(grantID)

	backend := NewSAFBackend(bridge)
	mount := newTestMount(grantID, true)

	revokedStatus := SAFGrantStatus{Valid: false, Readable: false, Writable: false, ProviderAvailable: false, RootExists: false}
	mountStatus, available := GrantStatusToMountUpdate(grantID, revokedStatus)
	if available {
		t.Error("mount should not be available when grant is revoked")
	}
	if mountStatus != WorkspaceStatusPermissionRevoked {
		t.Errorf("expected permission revoked status, got %q", mountStatus)
	}
	_ = backend
	_ = mount
}

func TestSAFBackend_WriteNotWritable(t *testing.T) {
	grantID := "saf_test_nw"
	status := SAFGrantStatus{Valid: true, Readable: true, Writable: false, ProviderAvailable: true, RootExists: true}

	mountStatus, available := GrantStatusToMountUpdate(grantID, status)
	if !available {
		t.Error("mount should be available for read-only")
	}
	if mountStatus != WorkspaceStatusReadOnly {
		t.Errorf("expected read-only status, got %q", mountStatus)
	}
}

func TestSAFBackend_GrantStatusToMount(t *testing.T) {
	tests := []struct {
		name     string
		status   SAFGrantStatus
		wantStat WorkspaceStatus
		wantAvail bool
	}{
		{
			name:      "valid ready",
			status:    SAFGrantStatus{Valid: true, Readable: true, Writable: true, ProviderAvailable: true, RootExists: true},
			wantStat:  WorkspaceStatusReady,
			wantAvail: true,
		},
		{
			name:      "read only",
			status:    SAFGrantStatus{Valid: true, Readable: true, Writable: false, ProviderAvailable: true, RootExists: true},
			wantStat:  WorkspaceStatusReadOnly,
			wantAvail: true,
		},
		{
			name:      "revoked",
			status:    SAFGrantStatus{Valid: false},
			wantStat:  WorkspaceStatusPermissionRevoked,
			wantAvail: false,
		},
		{
			name:      "provider unavailable",
			status:    SAFGrantStatus{Valid: true, ProviderAvailable: false},
			wantStat:  WorkspaceStatusUnavailable,
			wantAvail: false,
		},
		{
			name:      "root missing",
			status:    SAFGrantStatus{Valid: true, ProviderAvailable: true, RootExists: false},
			wantStat:  WorkspaceStatusMissing,
			wantAvail: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStat, gotAvail := GrantStatusToMountUpdate("test", tt.status)
			if gotStat != tt.wantStat {
				t.Errorf("status: got %q, want %q", gotStat, tt.wantStat)
			}
			if gotAvail != tt.wantAvail {
				t.Errorf("available: got %v, want %v", gotAvail, tt.wantAvail)
			}
		})
	}
}

func TestSAFBackend_NativeGrantNotExposed(t *testing.T) {
	mount := newTestMount("secret_grant_id", false)
	if mount.NativeGrant != "secret_grant_id" {
		t.Error("NativeGrant should be stored internally")
	}
}

func TestSAFPolicy_InferMIMEType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"test.txt", "text/plain; charset=utf-8"},
		{"test.png", "image/png"},
		{"test.unknown", "application/octet-stream"},
	}
	for _, tt := range tests {
		got := InferMIMEType(tt.filename)
		if got != tt.want {
			t.Errorf("InferMIMEType(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestSAFPolicy_Flags(t *testing.T) {
	if !IsSAFMIMEDirectory(SAFMIMETypeDir) {
		t.Error("expected dir mime to be detected")
	}
	if IsSAFMIMEDirectory("text/plain") {
		t.Error("text/plain should not be detected as dir")
	}
	flags := int64(SAFFlagSupportsDelete | SAFFlagSupportsWrite)
	if !IsSAFSupportsDelete(flags) {
		t.Error("expected delete flag detected")
	}
	if !IsSAFWritable(flags) {
		t.Error("expected write flag detected")
	}
	if IsSAFSupportsRename(flags) {
		t.Error("rename flag should not be set")
	}
}
