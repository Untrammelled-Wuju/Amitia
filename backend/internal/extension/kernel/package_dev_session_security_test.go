package kernel

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
)

func newBoundDeveloperSession(t *testing.T, container *Container, owner string) (*dev_mode.DeveloperSession, dev_mode.WorkspaceID) {
	t.Helper()
	workspaceID := dev_mode.WorkspaceID("unsigned-security")
	_, err := container.DevModeRegistry.Register(context.Background(), dev_mode.RegisterWorkspaceInput{
		WorkspaceID: workspaceID, ExtensionID: dev_mode.ExtensionID("com.example/pipeline"), OwnerUserID: owner,
		PathReference: t.TempDir(), ManifestPath: "manifest.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	workspace, err := container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := container.DevModeSessions.Open(context.Background(), workspaceID, dev_mode.ExtensionID("com.example/pipeline"), owner, "device", "test", packagePolicyVersion, true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	return session, workspaceID
}

func TestUnsignedDeveloperSessionBindings(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	runtime, container := newPackagePipelineRuntime(t)
	session, workspaceID := newBoundDeveloperSession(t, container, "user-1")
	if err := runtime.validateUnsignedDeveloperSession(session.SessionID, "user-1", "com.example/pipeline"); err != nil {
		t.Fatalf("valid developer session rejected: %v", err)
	}
	if len(session.Scopes) != 1 || session.Scopes[0] != "extensions.install.unsigned" {
		t.Fatalf("developer session did not receive the fixed minimum scope: %v", session.Scopes)
	}
	if err := runtime.validateUnsignedDeveloperSession(session.SessionID, "user-2", "com.example/pipeline"); err == nil {
		t.Fatal("cross-user developer session must be rejected")
	}
	if err := runtime.validateUnsignedDeveloperSession(session.SessionID, "user-1", "com.example/other"); err == nil {
		t.Fatal("cross-extension developer session must be rejected")
	}
	if err := container.DevModeRegistry.RevokeDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.validateUnsignedDeveloperSession(session.SessionID, "user-1", "com.example/pipeline"); err == nil {
		t.Fatal("revoked developer trust must invalidate the session")
	}
}

func TestUnsignedDeveloperSessionRejectsEnvironmentWorkspacePolicyAndExpiry(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	session, _ := newBoundDeveloperSession(t, container, "user-1")
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "false")
	if err := runtime.validateUnsignedDeveloperSession(session.SessionID, "user-1", "com.example/pipeline"); err == nil {
		t.Fatal("production environment must reject unsigned developer sessions")
	}
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	fake, err := container.DevModeSessions.Open(context.Background(), dev_mode.WorkspaceID("missing"), dev_mode.ExtensionID("com.example/pipeline"), "user-1", "device", "test", packagePolicyVersion, true, session.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.validateUnsignedDeveloperSession(fake.SessionID, "user-1", "com.example/pipeline"); err == nil {
		t.Fatal("missing workspace must be rejected")
	}
	stale, err := container.DevModeSessions.Open(context.Background(), session.WorkspaceID, session.ExtensionID, "user-1", "device", "test", "stale-policy", true, session.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.validateUnsignedDeveloperSession(stale.SessionID, "user-1", "com.example/pipeline"); err == nil {
		t.Fatal("stale policy session must be rejected")
	}
	container.DevModeSessions = dev_mode.NewSessionManager(time.Nanosecond)
	expired, err := container.DevModeSessions.Open(context.Background(), session.WorkspaceID, session.ExtensionID, "user-1", "device", "test", packagePolicyVersion, true, session.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := runtime.validateUnsignedDeveloperSession(expired.SessionID, "user-1", "com.example/pipeline"); err == nil {
		t.Fatal("expired developer session must be rejected")
	}
}
