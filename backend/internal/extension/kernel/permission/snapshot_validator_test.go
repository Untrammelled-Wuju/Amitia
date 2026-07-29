package permission_test

import (
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func makeTestRegistry() *permission.PermissionDefinitionRegistry {
	registry := permission.NewPermissionDefinitionRegistry()
	host_api.RegisterPermissionDefinitions(registry)
	return registry
}

func TestPermissionIDValidator_ValidPermissionIDs(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	validIDs := []string{
		"clipboard.write",
		"workflow.execute",
		"files.read",
		"files.write",
		"network.request",
		"tool.invoke",
		"storage.state.read",
		"storage.state.write",
		"character.read",
		"conversation.read",
		"memory.read",
	}
	for _, id := range validIDs {
		if !validator.IsValidPermissionID(id) {
			t.Fatalf("expected %q to be a valid permission ID", id)
		}
	}
}

func TestPermissionIDValidator_ActionIDsRejected(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	actionIDs := []string{
		"workflow.run",
		"action.copy",
		"action.paste",
		"action.delete",
	}
	for _, id := range actionIDs {
		if validator.IsValidPermissionID(id) {
			t.Fatalf("expected %q to be rejected as an action ID, not a permission ID", id)
		}
	}
}

func TestPermissionIDValidator_ValidateAll(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	ids := []string{"clipboard.write", "workflow.run", "files.read", "action.copy"}
	invalid := validator.ValidateAll(ids)
	if len(invalid) != 2 {
		t.Fatalf("expected 2 invalid IDs, got %d: %v", len(invalid), invalid)
	}
	joined := strings.Join(invalid, ",")
	if !strings.Contains(joined, "workflow.run") {
		t.Fatalf("expected workflow.run in invalid IDs, got %v", invalid)
	}
	if !strings.Contains(joined, "action.copy") {
		t.Fatalf("expected action.copy in invalid IDs, got %v", invalid)
	}
}

func TestPermissionIDValidator_NilRegistry(t *testing.T) {
	validator := permission.NewPermissionIDValidator(nil)
	if validator.IsValidPermissionID("clipboard.write") {
		t.Fatal("expected false when registry is nil")
	}
}

func TestSnapshot_ValidateGrantedPerms_ValidPermissions(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	snap := permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-valid",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write", "files.read", "workflow.execute"},
		CreatedAt:    time.Now().UTC(),
	}

	if err := snap.ValidateGrantedPerms(validator); err != nil {
		t.Fatalf("expected validation to pass for valid permission IDs, got error: %v", err)
	}
}

func TestSnapshot_ValidateGrantedPerms_ActionIDsRejected(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	snap := permission.PermissionSnapshot{
		SnapshotID:   "psnap-test-action",
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write", "workflow.run", "action.copy"},
		CreatedAt:    time.Now().UTC(),
	}

	err := snap.ValidateGrantedPerms(validator)
	if err == nil {
		t.Fatal("expected validation to fail for action IDs, got nil error")
	}
	if !strings.Contains(err.Error(), "workflow.run") {
		t.Fatalf("expected error to mention workflow.run, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action.copy") {
		t.Fatalf("expected error to mention action.copy, got: %v", err)
	}
}

func TestSnapshot_ValidateGrantedPerms_NilValidator(t *testing.T) {
	snap := permission.PermissionSnapshot{
		GrantedPerms: []string{"clipboard.write"},
	}
	err := snap.ValidateGrantedPerms(nil)
	if err == nil {
		t.Fatal("expected error when validator is nil")
	}
}

func TestSnapshot_ValidateGrantedPerms_EmptyPerms(t *testing.T) {
	validator := permission.NewPermissionIDValidator(permission.NewPermissionDefinitionRegistry())

	snap := permission.PermissionSnapshot{
		GrantedPerms: []string{},
	}
	if err := snap.ValidateGrantedPerms(validator); err != nil {
		t.Fatalf("expected no error for empty GrantedPerms, got: %v", err)
	}
}

func TestSnapshot_HasActionIDs_True(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	snap := permission.PermissionSnapshot{
		GrantedPerms: []string{"clipboard.write", "workflow.run"},
	}
	if !snap.HasActionIDs(validator) {
		t.Fatal("expected HasActionIDs to return true for snapshot with action IDs")
	}
}

func TestSnapshot_HasActionIDs_False(t *testing.T) {
	validator := permission.NewPermissionIDValidator(makeTestRegistry())

	snap := permission.PermissionSnapshot{
		GrantedPerms: []string{"clipboard.write", "files.read"},
	}
	if snap.HasActionIDs(validator) {
		t.Fatal("expected HasActionIDs to return false for snapshot with only valid permission IDs")
	}
}

func TestSnapshot_HasActionIDs_Empty(t *testing.T) {
	validator := permission.NewPermissionIDValidator(permission.NewPermissionDefinitionRegistry())

	snap := permission.PermissionSnapshot{
		GrantedPerms: []string{},
	}
	if snap.HasActionIDs(validator) {
		t.Fatal("expected HasActionIDs to return false for empty GrantedPerms")
	}
}

func TestSnapshot_VerifySession_Match(t *testing.T) {
	snap := permission.PermissionSnapshot{
		SessionID: "sess-1",
	}
	if err := snap.VerifySession("sess-1"); err != nil {
		t.Fatalf("expected session match to pass, got: %v", err)
	}
}

func TestSnapshot_VerifySession_Mismatch(t *testing.T) {
	snap := permission.PermissionSnapshot{
		SessionID: "sess-1",
	}
	err := snap.VerifySession("sess-2")
	if err == nil {
		t.Fatal("expected session mismatch to fail")
	}
}

func TestNewPermissionSnapshot_WithValidPerms(t *testing.T) {
	snap := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
		SessionID:    "sess-1",
		ExtensionID:  "ext-1",
		ModuleID:     "mod-1",
		Generation:   1,
		GrantedPerms: []string{"clipboard.write", "files.read"},
		Lifetime:     time.Hour,
	})
	if snap.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}
	if !strings.HasPrefix(snap.SnapshotID, "psnap-") {
		t.Fatalf("expected snapshot ID to start with psnap-, got: %s", snap.SnapshotID)
	}
	if len(snap.GrantedPerms) != 2 {
		t.Fatalf("expected 2 granted perms, got %d", len(snap.GrantedPerms))
	}
	if snap.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
}
