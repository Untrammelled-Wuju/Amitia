package sandbox_webui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	htmlContent := `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`
	if err := os.WriteFile(entryFile, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	host := NewHost()
	result, err := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if result.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if result.Nonce == "" {
		t.Error("Nonce should not be empty")
	}
	if result.Token == "" {
		t.Error("Token should not be empty")
	}
	if result.Origin == "" {
		t.Error("Origin should not be empty")
	}
	if result.CSP == "" {
		t.Error("CSP should not be empty")
	}
}

func TestCreateSessionInvalidSandbox(t *testing.T) {
	host := NewHost()
	_, err := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     "invalid",
		EntryPath:   "index.html",
	})
	if err != ErrInvalidSandboxType {
		t.Errorf("expected ErrInvalidSandboxType, got %v", err)
	}
}

func TestCreateSessionMissingEntry(t *testing.T) {
	host := NewHost()
	_, err := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
	})
	if err != ErrEntryMissing {
		t.Errorf("expected ErrEntryMissing, got %v", err)
	}
}

func TestCreateSessionPathTraversal(t *testing.T) {
	host := NewHost()
	_, err := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "../../../etc/passwd",
	})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestGetSession(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	result, _ := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})

	session, err := host.GetSession(result.SessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if session.SessionID != result.SessionID {
		t.Error("SessionID mismatch")
	}
}

func TestCloseSession(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	result, _ := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})

	if err := host.CloseSession(result.SessionID, "test"); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	_, err := host.GetSession(result.SessionID)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestQuarantineSession(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	result, _ := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})

	if err := host.QuarantineSession(result.SessionID, "security_violation"); err != nil {
		t.Fatalf("QuarantineSession failed: %v", err)
	}

	session, _ := host.GetSession(result.SessionID)
	if session.State != SessionStateQuarantined {
		t.Errorf("expected SessionStateQuarantined, got %s", session.State)
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})

	stats := host.GetStats()
	if stats.Total != 1 {
		t.Errorf("expected total=1, got %d", stats.Total)
	}
}

func TestValidateCSP(t *testing.T) {
	err := ValidateCSP(DefaultCSP)
	if err != nil {
		t.Errorf("DefaultCSP should be valid: %v", err)
	}

	err = ValidateCSP("")
	if err != ErrCSPEmpty {
		t.Errorf("expected ErrCSPEmpty, got %v", err)
	}

	err = ValidateCSP("default-src *; script-src 'unsafe-inline'")
	if err == nil {
		t.Error("CSP with unsafe-inline should be invalid")
	}
}

func TestIsMIMEAllowed(t *testing.T) {
	if !IsMIMEAllowed("text/html") {
		t.Error("text/html should be allowed")
	}
	if !IsMIMEAllowed("text/css") {
		t.Error("text/css should be allowed")
	}
	if IsMIMEAllowed("text/xml") {
		t.Error("text/xml should not be allowed")
	}
	if IsMIMEAllowed("application/x-httpd-php") {
		t.Error("PHP mime should not be allowed")
	}
}

func TestSanitizePath(t *testing.T) {
	p := NewProtocolHandler()

	clean, err := p.SanitizePath("/base", "index.html")
	if err != nil || clean != "index.html" {
		t.Errorf("expected index.html, got %s, err=%v", clean, err)
	}

	_, err = p.SanitizePath("/base", "../../../etc/passwd")
	if err == nil {
		t.Error("path traversal should be rejected")
	}

	_, err = p.SanitizePath("/base", "..\\..\\windows\\system32")
	if err == nil {
		t.Error("windows path traversal should be rejected")
	}
}

func TestBundleVerifier(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html><body>Hello</body></html>"), 0644)

	v := NewBundleVerifier()
	err := v.Verify(tmpDir, "index.html")
	if err != nil {
		t.Errorf("valid bundle should pass: %v", err)
	}

	scriptFile := filepath.Join(tmpDir, "script.html")
	os.WriteFile(scriptFile, []byte("<html><script>alert(1)</script></html>"), 0644)
	err = v.Verify(tmpDir, "script.html")
	if err != ErrBundleScriptForbidden {
		t.Errorf("script should be forbidden, got %v", err)
	}

	iframeFile := filepath.Join(tmpDir, "iframe.html")
	os.WriteFile(iframeFile, []byte("<html><iframe src='evil.com'></iframe></html>"), 0644)
	err = v.Verify(tmpDir, "iframe.html")
	if err != ErrBundleIframeForbidden {
		t.Errorf("iframe should be forbidden, got %v", err)
	}
}

func TestComputeHash(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	content := []byte("<html></html>")
	os.WriteFile(entryFile, content, 0644)

	v := NewBundleVerifier()
	hash, size, err := v.ComputeHash(tmpDir, "index.html")
	if err != nil {
		t.Fatalf("ComputeHash failed: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}

	err = v.VerifyIntegrity(tmpDir, "index.html", hash)
	if err != nil {
		t.Errorf("integrity check should pass: %v", err)
	}

	err = v.VerifyIntegrity(tmpDir, "index.html", "sha256-wronghash")
	if err != ErrIntegrityMismatch {
		t.Errorf("expected ErrIntegrityMismatch, got %v", err)
	}
}

func TestRevokeSessionsByContext(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	baseReq := CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	}

	req1 := baseReq
	req1.CharacterID = "char-1"
	req1.ConversationID = "conv-1"
	r1, _ := host.CreateSession(req1)

	req2 := baseReq
	req2.CharacterID = "char-1"
	req2.ConversationID = "conv-2"
	r2, _ := host.CreateSession(req2)

	req3 := baseReq
	req3.CharacterID = "char-2"
	req3.ConversationID = "conv-3"
	r3, _ := host.CreateSession(req3)

	stats := host.GetStats()
	if stats.Total != 3 {
		t.Fatalf("expected 3 sessions, got %d", stats.Total)
	}

	count := host.RevokeSessionsByContext("char-1", "")
	if count != 2 {
		t.Fatalf("expected 2 sessions revoked for char-1, got %d", count)
	}

	stats = host.GetStats()
	if stats.Total != 1 {
		t.Fatalf("expected 1 remaining session, got %d", stats.Total)
	}

	_, err := host.GetSession(r1.SessionID)
	if err != ErrSessionNotFound {
		t.Errorf("expected r1 to be revoked, got err=%v", err)
	}
	_, err = host.GetSession(r2.SessionID)
	if err != ErrSessionNotFound {
		t.Errorf("expected r2 to be revoked, got err=%v", err)
	}
	_, err = host.GetSession(r3.SessionID)
	if err != nil {
		t.Errorf("expected r3 to still exist, got err=%v", err)
	}
}

func TestRevokeSessionsByConversationID(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	baseReq := CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	}

	req1 := baseReq
	req1.CharacterID = "char-1"
	req1.ConversationID = "conv-1"
	host.CreateSession(req1)

	req2 := baseReq
	req2.CharacterID = "char-2"
	req2.ConversationID = "conv-1"
	host.CreateSession(req2)

	req3 := baseReq
	req3.CharacterID = "char-3"
	req3.ConversationID = "conv-2"
	r3, _ := host.CreateSession(req3)

	count := host.RevokeSessionsByContext("", "conv-1")
	if count != 2 {
		t.Fatalf("expected 2 sessions revoked for conv-1, got %d", count)
	}

	stats := host.GetStats()
	if stats.Total != 1 {
		t.Fatalf("expected 1 remaining session, got %d", stats.Total)
	}

	_, err := host.GetSession(r3.SessionID)
	if err != nil {
		t.Errorf("expected r3 to still exist, got err=%v", err)
	}
}

func TestRevokeSessionsByContextNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	req := CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
		CharacterID: "char-1",
		ConversationID: "conv-1",
	}
	host.CreateSession(req)

	count := host.RevokeSessionsByContext("char-999", "conv-999")
	if count != 0 {
		t.Fatalf("expected 0 sessions revoked, got %d", count)
	}

	stats := host.GetStats()
	if stats.Total != 1 {
		t.Fatalf("expected 1 remaining session, got %d", stats.Total)
	}
}
