package sandbox_webui

import (
	"strings"
	"testing"
)

func TestRestrictedBridgeRequiresTokenAndExactGeneration(t *testing.T) {
	_, _, session := setupTestBridge(t)
	message := &BridgeMessage{
		Method:     string(MethodReady),
		Session:    session.SessionID,
		Origin:     session.Origin,
		Nonce:      session.Nonce,
		Token:      "wrong",
		Generation: session.Generation,
	}
	if err := ValidateBridgeMessage(message, session); err != ErrTokenMismatch {
		t.Fatalf("expected token mismatch, got %v", err)
	}
	message.Token = session.Token
	message.Generation = 0
	if err := ValidateBridgeMessage(message, session); err != ErrGenerationStale {
		t.Fatalf("expected stale generation, got %v", err)
	}
}

func TestRestrictedBridgeScriptUsesTransferredPort(t *testing.T) {
	_, _, session := setupTestBridge(t)
	script, err := NewPreloadBuilder().Build(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"event.source !== window.parent", "event.ports[0]", "port.postMessage", "window.parent.postMessage({type:\"amitia.extension.ready\""} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script missing %q", expected)
		}
	}
	if strings.Contains(script, "host.welcome") || strings.Contains(script, "unsafe-inline") {
		t.Fatal("legacy window bridge remained in restricted bridge script")
	}
}

func TestRestrictedCSPMatchesContract(t *testing.T) {
	expected := "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'none'; object-src 'none'; frame-src 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'"
	if RestrictedCSP != expected {
		t.Fatalf("unexpected restricted CSP: %s", RestrictedCSP)
	}
}
