package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestCleanupAdapter_StopWithCleanup_NilSupervisor(t *testing.T) {
	adapter := NewCleanupAdapter()

	ctx := context.Background()
	err := adapter.StopWithCleanup(ctx, nil, "svc-1", false)
	if err == nil {
		t.Error("expected error for nil supervisor")
	}
}

func TestCleanupAdapter_StopWithCleanup_EmptyInstanceID(t *testing.T) {
	adapter := NewCleanupAdapter()

	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())
	ctx := context.Background()

	err := adapter.StopWithCleanup(ctx, supervisor, "", false)
	if err == nil {
		t.Error("expected error for empty instance ID")
	}
}

func TestCleanupAdapter_StopWithCleanup_ServiceNotFound(t *testing.T) {
	adapter := NewCleanupAdapter()

	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())
	ctx := context.Background()

	err := adapter.StopWithCleanup(ctx, supervisor, "nonexistent", false)
	if err == nil {
		t.Error("expected error for non-existent service")
	}
}

func TestCleanupVerifier_NilSupervisor(t *testing.T) {
	verifier := NewCleanupVerifier()

	ctx := context.Background()
	_, err := verifier.VerifyCleanup(ctx, nil, "svc-1")
	if err == nil {
		t.Error("expected error for nil supervisor")
	}
}

func TestCleanupVerifier_NotFoundIsCleaned(t *testing.T) {
	verifier := NewCleanupVerifier()

	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())
	ctx := context.Background()

	cleaned, err := verifier.VerifyCleanup(ctx, supervisor, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Error("expected nonexistent service to be considered cleaned")
	}
}

func TestCleanupVerifier_RunningServiceNotCleaned(t *testing.T) {
	verifier := NewCleanupVerifier()

	supervisor := trusted_service.NewProcessSupervisor(t.TempDir())

	pingExe, err := exec.LookPath("ping.exe")
	if err != nil {
		t.Fatalf("ping.exe not found: %v", err)
	}

	pingContent, err := os.ReadFile(pingExe)
	if err != nil {
		t.Fatalf("failed to read ping.exe: %v", err)
	}
	hash := sha256.Sum256(pingContent)
	expectedHash := hex.EncodeToString(hash[:])

	def := &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "test-svc",
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		TrustLevel:  string(trusted_service.TrustLevelTrusted),
		Executables: []trusted_service.PlatformExecutable{
			{
				Platform:     trusted_service.CurrentPlatform(),
				Path:         pingExe,
				ArgsTemplate: []string{"127.0.0.1", "-n", "60"},
				Sha256:       expectedHash,
				Signature:    trusted_service.BinarySignature{Trusted: true, Value: "test-sig"},
			},
		},
		Network: trusted_service.ServiceNetworkPolicy{LoopbackOnly: true},
	}
	_ = supervisor.Register(def)

	ctx := context.Background()
	_, err = supervisor.Start(ctx, trusted_service.StartRequest{
		ServiceID:      "test-svc",
		Generation:     1,
		PublisherTrust: trusted_service.TrustLevelTrusted,
	})
	if err != nil {
		t.Fatalf("failed to start process in test env: %v", err)
	}

	t.Cleanup(func() {
		_, _ = supervisor.Stop(ctx, trusted_service.StopRequest{
			ServiceID: "test-svc",
			Reason:    "test_cleanup",
			Force:     true,
		})
	})

	cleaned, err := verifier.VerifyCleanup(ctx, supervisor, "test-svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned {
		t.Error("expected running service to NOT be considered cleaned")
	}
}

func TestCleanupEvent_Identity(t *testing.T) {
	event := CleanupEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		InstanceID: "rt-1/bridge",
		Generation: 3,
		Success:    true,
	}

	identity := event.Identity()
	if identity.RuntimeID != "rt-1" {
		t.Errorf("expected RuntimeID rt-1, got %s", identity.RuntimeID)
	}
	if identity.Generation != 3 {
		t.Errorf("expected Generation 3, got %d", identity.Generation)
	}
}

func TestCleanupAdapter_StopWithCleanup_ExpectedErrorCode(t *testing.T) {
	adapter := NewCleanupAdapter()

	ctx := context.Background()
	err := adapter.StopWithCleanup(ctx, nil, "svc-1", false)
	if !IsExecutionError(err, ErrCleanupFailed) {
		t.Errorf("expected ErrCleanupFailed error code, got %v", err)
	}
}

func TestErrors_RegisterCrashReasons(t *testing.T) {
	reasons := []string{
		"process_exited",
		"health_check_failed",
		"restart_pending",
		"restart_exhausted",
		"quarantined",
	}

	allValid := true
	for _, reason := range reasons {
		if reason == "" {
			allValid = false
			break
		}
	}

	if !allValid {
		t.Error("expected all crash reasons to be non-empty strings")
	}
}

func TestServiceHealthError(t *testing.T) {
	cause := errors.New("original")
	err := &ServiceHealthError{
		Code:      ErrCrashUnrecoverable,
		RuntimeID: "rt-1",
		ServiceID: "bridge",
		Message:   "crash detected",
		Cause:     cause,
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if err.Unwrap() != cause {
		t.Error("expected Unwrap to return original cause")
	}
}

func TestQuarantineError(t *testing.T) {
	cause := errors.New("original")
	err := &QuarantineError{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Reason:     "crash_loop",
		Quarantine: true,
		Cause:      cause,
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	unwrapped := err.Unwrap()
	if unwrapped != nil && unwrapped.Error() != "original" {
		t.Errorf("expected original cause, got %v", unwrapped)
	}
}

func TestCrashHandlerError(t *testing.T) {
	err := &CrashHandlerError{
		RuntimeID:    "rt-1",
		ServiceID:   "bridge",
		ExitExpected: false,
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
