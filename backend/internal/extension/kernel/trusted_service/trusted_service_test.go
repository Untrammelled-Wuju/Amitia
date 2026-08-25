package trusted_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func makeTestExecutable(t *testing.T, dir string) (path, hash string) {
	t.Helper()
	name := "test.exe"
	if runtime.GOOS != "windows" {
		name = "test"
	}
	path = filepath.Join(dir, name)
	content := []byte("#!/bin/sh\nexit 0\n")
	if runtime.GOOS == "windows" {
		content = []byte("dummy exe")
	}
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	h := sha256.Sum256(content)
	return path, hex.EncodeToString(h[:])
}

func TestPlatformSelectorCurrent(t *testing.T) {
	s := NewPlatformSelector()
	def := &ServiceRuntimeDefinition{
		Executables: []PlatformExecutable{
			{Platform: Platform("unknown/arch"), Path: "a"},
			{Platform: s.current, Path: "b"},
		},
	}
	exe, err := s.Select(def)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if exe.Path != "b" {
		t.Fatalf("expected b, got %s", exe.Path)
	}
}

func TestPlatformSelectorNotSupported(t *testing.T) {
	s := NewPlatformSelector()
	def := &ServiceRuntimeDefinition{
		Executables: []PlatformExecutable{
			{Platform: Platform("unknown/arch"), Path: "a"},
		},
	}
	_, err := s.Select(def)
	if !errors.Is(err, ErrPlatformNotSupported) {
		t.Fatalf("expected platform not supported, got %v", err)
	}
}

func TestBinaryVerifierHashMismatch(t *testing.T) {
	dir := t.TempDir()
	path, _ := makeTestExecutable(t, dir)
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      path,
		Sha256:    "0000000000000000000000000000000000000000000000000000000000000000",
		Signature: BinarySignature{Trusted: true, Value: "sig", Algorithm: "ed25519"},
	}
	err := v.Verify(context.Background(), exe, "")
	if !errors.Is(err, ErrBinaryHashMismatch) {
		t.Fatalf("expected hash mismatch, got %v", err)
	}
}

func TestBinaryVerifierSuccess(t *testing.T) {
	dir := t.TempDir()
	path, hash := makeTestExecutable(t, dir)
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      path,
		Sha256:    hash,
		Signature: BinarySignature{Trusted: true, Value: "sig", Algorithm: "ed25519"},
	}
	if err := v.Verify(context.Background(), exe, ""); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestBinaryVerifierMissingSignature(t *testing.T) {
	dir := t.TempDir()
	path, hash := makeTestExecutable(t, dir)
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      path,
		Sha256:    hash,
		Signature: BinarySignature{Trusted: true, Value: "", Algorithm: "ed25519"},
	}
	err := v.Verify(context.Background(), exe, "")
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestBinaryVerifierUntrustedSignature(t *testing.T) {
	dir := t.TempDir()
	path, hash := makeTestExecutable(t, dir)
	v := NewBinaryVerifier()
	exe := &PlatformExecutable{
		Path:      path,
		Sha256:    hash,
		Signature: BinarySignature{Trusted: false, Value: "sig", Algorithm: "ed25519"},
	}
	err := v.Verify(context.Background(), exe, "")
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func TestArgsBuilderSubstitute(t *testing.T) {
	b := NewArgsBuilder([]string{"--port", "${PORT}", "--host", "${HOST}"})
	args, err := b.Build(map[string]string{"PORT": "8080", "HOST": "127.0.0.1"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(args) != 4 || args[1] != "8080" || args[3] != "127.0.0.1" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestArgsBuilderMissingParam(t *testing.T) {
	b := NewArgsBuilder([]string{"--port", "${PORT}"})
	_, err := b.Build(map[string]string{})
	if err == nil {
		t.Fatalf("expected missing param error")
	}
}

func TestEnvBuilderAllowedOnly(t *testing.T) {
	b := NewEnvBuilder()
	exe := &PlatformExecutable{
		EnvTemplate: map[string]string{
			"AMITIA_SESSION":  "s1",
			"PATH":            "/usr/bin",
			"AMITIA_HOST_API": "internal-rpc",
		},
	}
	env := b.Build(exe, "sess-1", "inst-1", 1, "/tmp", "info", "lease-1")
	hasSession := false
	hasPath := false
	hasHostAPI := false
	for _, e := range env {
		if e == "AMITIA_SESSION=sess-1" {
			hasSession = true
		}
		if e == "PATH=/usr/bin" {
			hasPath = true
		}
		if e == "AMITIA_HOST_API=internal-rpc" {
			hasHostAPI = true
		}
	}
	if !hasSession {
		t.Fatalf("AMITIA_SESSION missing")
	}
	if hasPath {
		t.Fatalf("PATH should be filtered out")
	}
	if !hasHostAPI {
		t.Fatalf("AMITIA_HOST_API missing")
	}
}

func TestValidateTrustUnknownPublisher(t *testing.T) {
	def := &ServiceRuntimeDefinition{
		TrustLevel:  string(TrustLevelTrusted),
		Executables: []PlatformExecutable{{Path: "x", Sha256: "h", Signature: BinarySignature{Trusted: true, Value: "s"}}},
		Network:     ServiceNetworkPolicy{LoopbackOnly: true},
	}
	err := ValidateTrust(def, TrustLevelUnknown)
	if !errors.Is(err, ErrUnknownPublisher) {
		t.Fatalf("expected unknown publisher, got %v", err)
	}
}

func TestValidateTrustCommunityPublisher(t *testing.T) {
	def := &ServiceRuntimeDefinition{
		TrustLevel:  string(TrustLevelCommunity),
		Executables: []PlatformExecutable{{Path: "x", Sha256: "h", Signature: BinarySignature{Trusted: true, Value: "s"}}},
		Network:     ServiceNetworkPolicy{LoopbackOnly: true},
	}
	if err := ValidateTrust(def, TrustLevelCommunity); !errors.Is(err, ErrTrustLevelInsufficient) {
		t.Fatalf("community executable must require explicit user trust promotion, got: %v", err)
	}
	if !TrustLevelCommunity.RequiresFullSandbox() {
		t.Fatal("community publisher must require full sandbox")
	}
}

func TestValidateTrustTrustedPublisher(t *testing.T) {
	def := &ServiceRuntimeDefinition{
		TrustLevel:  string(TrustLevelTrusted),
		Executables: []PlatformExecutable{{Path: "x", Sha256: "h", Signature: BinarySignature{Trusted: true, Value: "s"}}},
		Network:     ServiceNetworkPolicy{LoopbackOnly: true},
	}
	if err := ValidateTrust(def, TrustLevelTrusted); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateTrustInboundNotLoopback(t *testing.T) {
	def := &ServiceRuntimeDefinition{
		TrustLevel:  string(TrustLevelTrusted),
		Executables: []PlatformExecutable{{Path: "x", Sha256: "h", Signature: BinarySignature{Trusted: true, Value: "s"}}},
		Network:     ServiceNetworkPolicy{AllowInbound: true, LoopbackOnly: false},
	}
	err := ValidateTrust(def, TrustLevelTrusted)
	if err == nil {
		t.Fatalf("expected inbound/loopback error")
	}
}

func TestValidateNoShellRejects(t *testing.T) {
	cases := []string{"cmd.exe", "powershell.exe", "bash", "sh", "zsh"}
	for _, c := range cases {
		if err := ValidateNoShell([]string{c}); !errors.Is(err, ErrShellDisallowed) {
			t.Fatalf("expected shell disallowed for %s, got %v", c, err)
		}
	}
}

func TestValidateNoShellAllows(t *testing.T) {
	if err := ValidateNoShell([]string{"/usr/local/bin/myapp", "--port", "8080"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateNoShellRejectsNewline(t *testing.T) {
	err := ValidateNoShell([]string{"/usr/local/bin/myapp", "arg\nrm -rf /"})
	if !errors.Is(err, ErrShellDisallowed) {
		t.Fatalf("expected shell disallowed for newline")
	}
}

func TestServiceInstanceStates(t *testing.T) {
	inst := &ServiceInstance{State: ServiceStateRegistered}
	inst.MarkStarted()
	if !inst.State_().IsHealthy() {
		t.Fatalf("expected healthy after start")
	}
	inst.MarkStopped()
	if !inst.State_().IsTerminal() {
		t.Fatalf("expected terminal after stop")
	}
}

func TestServiceInstanceHealthFails(t *testing.T) {
	inst := &ServiceInstance{State: ServiceStateReady}
	if c := inst.RecordHealthFail(); c != 1 {
		t.Fatalf("expected 1 fail, got %d", c)
	}
	if c := inst.RecordHealthFail(); c != 2 {
		t.Fatalf("expected 2 fails, got %d", c)
	}
	inst.RecordHealthSuccess()
	if inst.HealthFails != 0 {
		t.Fatalf("expected 0 fails after success, got %d", inst.HealthFails)
	}
}

func TestProcessSupervisorRegisterUnregister(t *testing.T) {
	s := NewProcessSupervisor(t.TempDir())
	def := &ServiceRuntimeDefinition{ServiceID: "svc-1"}
	if err := s.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.Register(def); err == nil {
		t.Fatalf("expected duplicate register error")
	}
	if err := s.Unregister("svc-1"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
}

func TestProcessSupervisorGetNotFound(t *testing.T) {
	s := NewProcessSupervisor(t.TempDir())
	_, err := s.Get("missing")
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestProcessSupervisorStopNotRunning(t *testing.T) {
	s := NewProcessSupervisor(t.TempDir())
	inst := &ServiceInstance{ServiceID: "svc-1", State: ServiceStateStopped}
	s.mu.Lock()
	s.instances["svc-1"] = inst
	s.mu.Unlock()
	res, err := s.Stop(context.Background(), StopRequest{ServiceID: "svc-1"})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if res.State != ServiceStateStopped {
		t.Fatalf("expected stopped, got %s", res.State)
	}
}

func TestHealthMonitorNoCheckType(t *testing.T) {
	m := NewHealthMonitor()
	inst := &ServiceInstance{State: ServiceStateReady, PID: 1}
	def := &ServiceRuntimeDefinition{ServiceID: "svc-1"}
	m.Monitor(inst, def)
	time.Sleep(50 * time.Millisecond)
	m.Stop(inst)
}

func TestTrustLevelAllowedForService(t *testing.T) {
	if !TrustLevelOfficial.AllowedForService() {
		t.Fatalf("official should be allowed")
	}
	if !TrustLevelTrusted.AllowedForService() {
		t.Fatalf("trusted should be allowed")
	}
	if TrustLevelCommunity.AllowedForService() {
		t.Fatalf("community executable services must require explicit user trust promotion")
	}
	if !TrustLevelCommunity.RequiresFullSandbox() {
		t.Fatalf("community should require full sandbox")
	}
	if TrustLevelUnknown.AllowedForService() {
		t.Fatalf("unknown should not be allowed")
	}
}

func TestServiceStateTerminal(t *testing.T) {
	terminal := []ServiceState{ServiceStateStopped, ServiceStateFailed, ServiceStateQuarantined}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Fatalf("expected %s to be terminal", s)
		}
	}
	nonTerminal := []ServiceState{ServiceStateRegistered, ServiceStateStarting, ServiceStateReady, ServiceStateStopping}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Fatalf("expected %s to NOT be terminal", s)
		}
	}
}

func TestTrustedServiceFactoryTypeReturnsService(t *testing.T) {
	f := &TrustedServiceFactory{}
	if f.Type() != domain.RuntimeTypeService {
		t.Fatalf("expected RuntimeTypeService, got %s", f.Type())
	}
}
