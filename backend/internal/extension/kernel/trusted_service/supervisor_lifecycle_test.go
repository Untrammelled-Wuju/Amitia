package trusted_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestLifecyclePortProcess(t *testing.T) {
	if os.Getenv("AMITIA_LIFECYCLE_PORT") == "" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("AMITIA_LIFECYCLE_PORT"))
	if err != nil {
		os.Exit(3)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}
}

func TestLifecycleStopReleasesPortAndBlocksRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process fixture")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, hash := writeLifecycleExecutable(t, dir, "port-service", fmt.Sprintf("AMITIA_LIFECYCLE_PORT=%s exec %q -test.run=^TestLifecyclePortProcess$", strconv.Itoa(port), executable))
	s := NewProcessSupervisor(filepath.Join(dir, "supervisor"))
	def := lifecycleDefinition("runtime/port-service", path, hash)
	if err := s.Register(def); err != nil {
		t.Fatal(err)
	}
	req := StartRequest{ServiceID: def.ServiceID, InstanceID: def.ServiceID, RuntimeID: "runtime", PublisherTrust: TrustLevelTrusted, WorkingDir: dir}
	if _, err := s.Start(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service did not open port")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := s.Stop(context.Background(), StopRequest{ServiceID: def.ServiceID, Reason: "update"}); err != nil {
		t.Fatal(err)
	}
	listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("port still occupied after update stop: %v", err)
	}
	listener.Close()
	if _, err := s.Start(context.Background(), req); err != nil {
		t.Fatalf("new generation failed to start: %v", err)
	}
	if err := s.StopAll(context.Background(), "host_shutdown"); err != nil {
		t.Fatal(err)
	}
	listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("port still occupied: %v", err)
	}
	listener.Close()
	if _, err := s.Start(context.Background(), req); err == nil {
		t.Fatal("start accepted after shutdown")
	}
}

type lifecycleHostIdentity struct{}

func (lifecycleHostIdentity) GetHostInstanceID() string { return "test-host" }
func (lifecycleHostIdentity) GetHostSessionID() string  { return "test-session" }

type foreignLifecycleHostIdentity struct{}

func (foreignLifecycleHostIdentity) GetHostInstanceID() string { return "other-host" }
func (foreignLifecycleHostIdentity) GetHostSessionID() string  { return "other-session" }

func TestLifecycleRecoversOwnedProcessBeforeStarting(t *testing.T) {
	dir := t.TempDir()
	path, hash := writeLifecycleExecutable(t, dir, "recover-service", "/bin/sleep 30")
	root := filepath.Join(dir, "supervisor")
	def := lifecycleDefinition("runtime/recover-service", path, hash)
	def.Recovery.MaxRestarts = 0
	req := StartRequest{ServiceID: def.ServiceID, InstanceID: def.ServiceID, RuntimeID: "runtime", PublisherTrust: TrustLevelTrusted, WorkingDir: dir}
	old := NewProcessSupervisor(root)
	old.SetHostIdentityProvider(lifecycleHostIdentity{})
	if err := old.Register(def); err != nil {
		t.Fatal(err)
	}
	first, err := old.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = old.Stop(context.Background(), StopRequest{ServiceID: def.ServiceID, Force: true}) })
	next := NewProcessSupervisor(root)
	next.SetHostIdentityProvider(foreignLifecycleHostIdentity{})
	if err := next.Register(def); err != nil {
		t.Fatal(err)
	}
	if _, err := next.Start(context.Background(), req); err == nil || !procIsAlive(first.PID) {
		t.Fatal("foreign process was not protected")
	}
	next.SetHostIdentityProvider(lifecycleHostIdentity{})
	second, err := next.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = next.StopAll(context.Background(), "test") })
	if procIsAlive(first.PID) || !procIsAlive(second.PID) {
		t.Fatalf("old pid %d or new pid %d has unexpected state", first.PID, second.PID)
	}
}

func TestLifecycleStopRemovesChildrenAfterParentExits(t *testing.T) {
	dir := t.TempDir()
	unrelated := exec.Command("/bin/sleep", "30")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() })
	childFile := filepath.Join(dir, "child.pid")
	path, hash := writeLifecycleExecutable(t, dir, "child-service", fmt.Sprintf("/bin/sleep 30 &\nprintf '%%s' \"$!\" > %q\n/bin/sleep 0.4", childFile))
	s := NewProcessSupervisor(filepath.Join(dir, "supervisor"))
	def := lifecycleDefinition("runtime/child-service", path, hash)
	def.Shutdown.GracePeriod = time.Second
	if err := s.Register(def); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(context.Background(), StartRequest{ServiceID: def.ServiceID, InstanceID: def.ServiceID, RuntimeID: "runtime", PublisherTrust: TrustLevelTrusted, WorkingDir: dir}); err != nil {
		t.Fatal(err)
	}
	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(childFile); err == nil {
			childPID, _ = strconv.Atoi(string(b))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("child process did not start")
	}
	t.Cleanup(func() {
		if p, err := os.FindProcess(childPID); err == nil {
			_ = p.Kill()
		}
	})
	if _, err := s.Stop(context.Background(), StopRequest{ServiceID: def.ServiceID, Reason: "update"}); err != nil {
		t.Fatal(err)
	}
	gone := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if !procIsAlive(childPID) {
			gone = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !gone {
		t.Fatal("child survived parent exit")
	}
	if !procIsAlive(unrelated.Process.Pid) {
		t.Fatal("unrelated process was terminated")
	}
}

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
