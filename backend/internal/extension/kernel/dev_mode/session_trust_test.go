package dev_mode

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionManagerRevokeAllInvalidatesPolicySessions(t *testing.T) {
	manager := NewSessionManager(time.Hour)
	session, err := manager.Open(context.Background(), WorkspaceID("workspace-1"), ExtensionID("extension-1"),
		"user-1", "device-1", "test", "policy-1", true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if count := manager.RevokeAll(); count != 1 {
		t.Fatalf("unexpected revoked session count: %d", count)
	}
	if _, err := manager.Validate(session.SessionID); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("expected revoked developer session, got %v", err)
	}
}
