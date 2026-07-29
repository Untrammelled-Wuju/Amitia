package sandbox_webui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateSessionStoresGrantedPermsAndScopes(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	grantedPerms := []string{"tool.invoke", "data.read"}
	grantedScopes := []string{"character.profile", "conversation.messages"}

	result, err := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-auth-1",
		ModuleID:       "mod-auth-1",
		Generation:     3,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		AllowedActions: []string{"action-a", "action-b"},
		GrantedPerms:   grantedPerms,
		GrantedScopes:  grantedScopes,
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	session, err := host.GetSession(result.SessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(session.GrantedPerms) != 2 {
		t.Fatalf("expected 2 granted perms, got %d", len(session.GrantedPerms))
	}
	for i, expected := range grantedPerms {
		if session.GrantedPerms[i] != expected {
			t.Errorf("granted perm[%d]: expected %s, got %s", i, expected, session.GrantedPerms[i])
		}
	}

	if len(session.GrantedScopes) != 2 {
		t.Fatalf("expected 2 granted scopes, got %d", len(session.GrantedScopes))
	}
	for i, expected := range grantedScopes {
		if session.GrantedScopes[i] != expected {
			t.Errorf("granted scope[%d]: expected %s, got %s", i, expected, session.GrantedScopes[i])
		}
	}
}

func TestGetSessionInfoReturnsGrantedPermsAndScopes(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	grantedPerms := []string{"perm-x"}
	grantedScopes := []string{"scope-y"}

	result, _ := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		GrantedPerms:   grantedPerms,
		GrantedScopes:  grantedScopes,
	})

	info, err := host.GetSessionInfo(result.SessionID)
	if err != nil {
		t.Fatalf("GetSessionInfo failed: %v", err)
	}

	if len(info.GrantedPerms) != 1 || info.GrantedPerms[0] != "perm-x" {
		t.Fatalf("expected GrantedPerms=[perm-x], got %v", info.GrantedPerms)
	}
	if len(info.GrantedScopes) != 1 || info.GrantedScopes[0] != "scope-y" {
		t.Fatalf("expected GrantedScopes=[scope-y], got %v", info.GrantedScopes)
	}
}

func TestPermissionSnapshotFactoryReceivesGrantedPerms(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()

	var capturedPerms []string
	host.SetPermissionSnapshotFactory(func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error) {
		capturedPerms = grantedPerms
		return "perm-snap-1", nil
	})

	grantedPerms := []string{"tool.invoke", "data.write"}
	allowedActions := []string{"action-a", "action-b", "action-c"}

	_, err := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		AllowedActions: allowedActions,
		GrantedPerms:   grantedPerms,
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if len(capturedPerms) != len(grantedPerms) {
		t.Fatalf("expected %d captured perms, got %d", len(grantedPerms), len(capturedPerms))
	}
	for i, expected := range grantedPerms {
		if capturedPerms[i] != expected {
			t.Errorf("captured perm[%d]: expected %s, got %s", i, expected, capturedPerms[i])
		}
	}

	for _, a := range allowedActions {
		for _, c := range capturedPerms {
			if a == c {
				t.Errorf("AllowedAction %s should not appear in captured perms (must use GrantedPerms not AllowedActions)", a)
			}
		}
	}
}

func TestCreateSessionWithoutGrantedPermsUsesEmptyForSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()

	var capturedPerms []string
	host.SetPermissionSnapshotFactory(func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error) {
		capturedPerms = grantedPerms
		return "perm-snap-empty", nil
	})

	_, err := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		AllowedActions: []string{"action-a"},
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if len(capturedPerms) != 0 {
		t.Fatalf("expected 0 captured perms when GrantedPerms not set, got %d (%v)", len(capturedPerms), capturedPerms)
	}
}

func TestRevokeSessionsByContextInvalidatesOldSessions(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()

	req := CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		CharacterID:    "char-revoke-1",
		ConversationID: "conv-revoke-1",
		GrantedPerms:   []string{"tool.invoke"},
		GrantedScopes:  []string{"character.profile"},
	}

	result, err := host.CreateSession(req)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	session, err := host.GetSession(result.SessionID)
	if err != nil {
		t.Fatalf("GetSession before revoke failed: %v", err)
	}
	if len(session.GrantedPerms) != 1 {
		t.Fatalf("expected 1 granted perm before revoke, got %d", len(session.GrantedPerms))
	}

	var releasedScope, releasedPerm string
	host.SetSnapshotReleaser(func(scopeSnapshotID, permissionSnapshotID string) error {
		releasedScope = scopeSnapshotID
		releasedPerm = permissionSnapshotID
		return nil
	})

	count := host.RevokeSessionsByContext("char-revoke-1", "")
	if count != 1 {
		t.Fatalf("expected 1 session revoked, got %d", count)
	}

	_, err = host.GetSession(result.SessionID)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after revoke, got %v", err)
	}

	_ = releasedScope
	_ = releasedPerm
}

func TestRevokeSessionsByContextReleasesSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()

	releaseCalls := 0
	host.SetSnapshotReleaser(func(scopeSnapshotID, permissionSnapshotID string) error {
		releaseCalls++
		return nil
	})
	host.SetScopeSnapshotFactory(func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error) {
		return "scope-snap-revoke", nil
	})
	host.SetPermissionSnapshotFactory(func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error) {
		return "perm-snap-revoke", nil
	})

	req := CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		CharacterID:    "char-snap-1",
		GrantedPerms:   []string{"tool.invoke"},
	}

	_, err := host.CreateSession(req)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	count := host.RevokeSessionsByContext("char-snap-1", "")
	if count != 1 {
		t.Fatalf("expected 1 session revoked, got %d", count)
	}

	if releaseCalls != 1 {
		t.Fatalf("expected 1 snapshot release call, got %d", releaseCalls)
	}
}

func TestGrantedPermsAndScopesAreIndependentFromAllowedActions(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()

	grantedPerms := []string{"perm-granted-1"}
	grantedScopes := []string{"scope-granted-1"}
	allowedActions := []string{"action-declared-1"}

	result, err := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		AllowedActions: allowedActions,
		GrantedPerms:   grantedPerms,
		GrantedScopes:  grantedScopes,
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	session, _ := host.GetSession(result.SessionID)

	if len(session.AllowedActions) != 1 || session.AllowedActions[0] != "action-declared-1" {
		t.Errorf("expected AllowedActions=[action-declared-1], got %v", session.AllowedActions)
	}
	if len(session.GrantedPerms) != 1 || session.GrantedPerms[0] != "perm-granted-1" {
		t.Errorf("expected GrantedPerms=[perm-granted-1], got %v", session.GrantedPerms)
	}
	if len(session.GrantedScopes) != 1 || session.GrantedScopes[0] != "scope-granted-1" {
		t.Errorf("expected GrantedScopes=[scope-granted-1], got %v", session.GrantedScopes)
	}

	if !session.IsActionAllowed("action-declared-1") {
		t.Error("expected action-declared-1 to be allowed")
	}
	if session.IsActionAllowed("perm-granted-1") {
		t.Error("perm-granted-1 should not be an allowed action (it's a permission, not an action)")
	}
}
