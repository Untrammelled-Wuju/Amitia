package trusted_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeLifecycleExecutable(t *testing.T, dir, name, body string) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("trusted_service lifecycle process fixture requires a POSIX executable")
	}
	path := filepath.Join(dir, name)
	content := []byte("#!/bin/sh\nset -eu\n" + body + "\n")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	hash := sha256.Sum256(content)
	return path, hex.EncodeToString(hash[:])
}

func lifecycleDefinition(serviceID, executablePath, hash string) *ServiceRuntimeDefinition {
	return &ServiceRuntimeDefinition{
		ServiceID:   serviceID,
		ExtensionID: "test.extension",
		ModuleID:    "runtime",
		TrustLevel:  string(TrustLevelTrusted),
		Executables: []PlatformExecutable{{
			Platform: CurrentPlatform(),
			Path:     executablePath,
			Sha256:   hash,
			Signature: BinarySignature{
				Algorithm: "test",
				Value:     "trusted-test-signature",
				Trusted:   true,
			},
		}},
		Protocol: "plain",
		HealthCheck: ServiceHealthCheck{
			Type: "none",
		},
		Recovery: ServiceRecoveryPolicy{
			MaxRestarts:          2,
			RestartDelay:         20 * time.Millisecond,
			BackoffMultiplier:    1,
			MaxRestartDelay:      20 * time.Millisecond,
			RecoveryDecisionMode: RecoveryDecisionAutoRestart,
		},
		Shutdown: ServiceShutdownPolicy{
			GracePeriod: 20 * time.Millisecond,
			KillTimeout: 2 * time.Second,
		},
		Network: ServiceNetworkPolicy{Enforce: false},
	}
}

func TestProcessSupervisorCallerContextDoesNotOwnReadyProcessLifetime(t *testing.T) {
	dir := t.TempDir()
	path, hash := writeLifecycleExecutable(t, dir, "long-running", "exec /bin/sleep 30")

	s := NewProcessSupervisor(filepath.Join(dir, "supervisor"))
	def := lifecycleDefinition("runtime/service", path, hash)
	if err := s.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}

	startupCtx, cancelStartup := context.WithCancel(context.Background())
	result, err := s.Start(startupCtx, StartRequest{
		ServiceID:      def.ServiceID,
		InstanceID:     "runtime/service",
		RuntimeID:      "runtime",
		PublisherTrust: TrustLevelTrusted,
		WorkingDir:     dir,
	})
	if err != nil {
		cancelStartup()
		t.Fatalf("start: %v", err)
	}
	cancelStartup()

	time.Sleep(150 * time.Millisecond)
	inst, err := s.Get(def.ServiceID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if inst.PID != result.PID || !procIsAlive(inst.PID) {
		t.Fatalf("ready process was killed by caller context cancellation: pid=%d state=%s", inst.PID, inst.State_())
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCancel()
	if _, err := s.Stop(stopCtx, StopRequest{ServiceID: def.ServiceID, Reason: "test_cleanup", Force: true}); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestProcessSupervisorAutoRestartPreservesIdentityAndLaunchSnapshot(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "restart.marker")
	_, hash := writeLifecycleExecutable(t, dir, "restartable", `
marker="$1"
if [ ! -f "$marker" ]; then
  /bin/sleep 0.20
  /usr/bin/touch "$marker"
  exit 17
fi
exec /bin/sleep 30`)

	s := NewProcessSupervisor(filepath.Join(dir, "supervisor"))
	def := lifecycleDefinition("runtime/service", "restartable", hash)
	def.Executables[0].ArgsTemplate = []string{"${MARKER}"}
	if err := s.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}

	const stableInstanceID = "runtime/service"
	_, err := s.Start(context.Background(), StartRequest{
		ServiceID:        def.ServiceID,
		InstanceID:       stableInstanceID,
		RuntimeID:        "runtime",
		PluginID:         "publisher/plugin",
		LogicalServiceID: "service",
		ExtensionID:      "test.extension",
		ContributionID:   "game",
		Generation:       7,
		PublisherTrust:   TrustLevelTrusted,
		BasePath:         dir,
		WorkingDir:       dir,
		SessionToken:     "session-token",
		SecretLease:      "secret-lease",
		Args:             map[string]string{"MARKER": marker},
	})
	if err != nil {
		t.Fatalf("initial start: %v", err)
	}

	deadline := time.Now().Add(4 * time.Second)
	var restarted *ServiceInstance
	for time.Now().Before(deadline) {
		inst, getErr := s.Get(def.ServiceID)
		if getErr == nil && inst.InstanceID == stableInstanceID && inst.RestartCount == 1 && inst.State_() == ServiceStateReady && procIsAlive(inst.PID) {
			restarted = inst
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if restarted == nil {
		inst, _ := s.Get(def.ServiceID)
		if inst == nil {
			t.Fatal("service did not auto-restart")
		}
		t.Fatalf("service did not auto-restart with stable identity: instance=%q restarts=%d state=%s lastError=%q", inst.InstanceID, inst.RestartCount, inst.State_(), inst.lastExitError)
	}
	if restarted.restartRequest.BasePath != dir || restarted.restartRequest.SecretLease != "secret-lease" {
		t.Fatalf("restart launch snapshot was not preserved: base=%q secret=%q", restarted.restartRequest.BasePath, restarted.restartRequest.SecretLease)
	}
	if restarted.restartRequest.Args["MARKER"] != marker {
		t.Fatalf("restart args were not preserved: %#v", restarted.restartRequest.Args)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCancel()
	if _, err := s.Stop(stopCtx, StopRequest{ServiceID: def.ServiceID, Reason: "test_cleanup", Force: true}); err != nil {
		t.Fatalf("stop restarted service: %v", err)
	}
}

func TestServiceInstanceAllowsOnlyOneProcessWaitOwner(t *testing.T) {
	inst := &ServiceInstance{}
	if !inst.claimWaitOwner() {
		t.Fatal("first wait owner must be accepted")
	}
	if inst.claimWaitOwner() {
		t.Fatal("second wait owner must be rejected")
	}
}

func TestRestartStartRequestPreservesStableRoutingAndDeepCopiesArgs(t *testing.T) {
	inst := &ServiceInstance{
		InstanceID:       "runtime-1/service-1",
		ServiceID:        "runtime-1/service-1",
		RuntimeID:        "runtime-1",
		PluginID:         "publisher/plugin",
		LogicalServiceID: "service-1",
		ExtensionID:      "extension-1",
		Generation:       11,
		Definition:       &ServiceRuntimeDefinition{TrustLevel: string(TrustLevelTrusted)},
		restartRequest: StartRequest{
			BasePath:     "/generation/root",
			WorkingDir:   "/runtime/data",
			SessionToken: "session",
			SecretLease:  "lease",
			Args:         map[string]string{"PORT": "25565"},
		},
	}

	req := restartStartRequest(inst, 3)
	if req.InstanceID != inst.InstanceID || req.ServiceID != inst.ServiceID {
		t.Fatalf("routing identity changed across restart: service=%q instance=%q", req.ServiceID, req.InstanceID)
	}
	if req.RuntimeID != inst.RuntimeID || req.LogicalServiceID != inst.LogicalServiceID || req.PluginID != inst.PluginID {
		t.Fatalf("routing metadata changed across restart: %#v", req)
	}
	if req.BasePath != "/generation/root" || req.WorkingDir != "/runtime/data" || req.SecretLease != "lease" || req.RestartCount != 3 {
		t.Fatalf("launch snapshot not preserved: %#v", req)
	}
	req.Args["PORT"] = "25566"
	if inst.restartRequest.Args["PORT"] != "25565" {
		t.Fatal("restart request args alias the stored launch snapshot")
	}
}
