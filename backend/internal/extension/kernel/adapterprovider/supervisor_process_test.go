package adapterprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestAdapterProviderProcessLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX adapter provider fixture")
	}
	root := t.TempDir()
	executable := writeAdapterProviderExecutable(t, root)
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust:      plugincontract.TrustUserTrusted,
		AllowExternal: true,
	})
	router := plugincontract.NewContributionRouter(registry)
	handler := NewSupervisorHandler(registry, router)
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "external.process",
		ContractVersion:    1,
		SupportedKinds:     []string{"process_tool"},
		Operations:         []string{"test"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustUserTrusted,
	}
	if err := handler.RegisterProvider(ProviderRegistration{ServiceID: "process-adapter", Descriptor: descriptor}); err != nil {
		t.Fatal(err)
	}
	supervisor := trusted_service.NewProcessSupervisor(filepath.Join(root, "supervisor"))
	if err := supervisor.RegisterStdioProtocolHandler("adapter-provider", handler); err != nil {
		t.Fatal(err)
	}
	definition := adapterProviderDefinition("process-adapter", executable)
	if err := supervisor.Register(definition); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = supervisor.StopAll(ctx, "test_cleanup")
	}()
	start := func(instance string) *trusted_service.StartResult {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := supervisor.Start(ctx, trusted_service.StartRequest{
			ServiceID:      definition.ServiceID,
			InstanceID:     instance,
			RuntimeID:      "adapter-runtime",
			PublisherTrust: trusted_service.TrustLevelTrusted,
			WorkingDir:     root,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := start("process-adapter-1")
	if !router.Has(descriptor.ID, descriptor.ContractVersion) {
		t.Fatal("adapter route was not registered after process startup")
	}
	if result := callProcessBuild(t, router, descriptor); result != "process-provider" {
		t.Fatalf("process provider result = %s", result)
	}
	if result := callProcessInvoke(t, router, descriptor); result != "process" {
		t.Fatalf("process provider invocation = %s", result)
	}
	process, err := os.FindProcess(first.PID)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitForAdapterRoute(t, router, descriptor, false)
	if result := callProcessBuildExpectError(t, router, descriptor); result == nil {
		t.Fatal("adapter call succeeded after provider process exit")
	}
	start("process-adapter-2")
	waitForAdapterRoute(t, router, descriptor, true)
	if result := callProcessBuild(t, router, descriptor); result != "process-provider" {
		t.Fatalf("restarted process provider result = %s", result)
	}
}

func writeAdapterProviderExecutable(t *testing.T, root string) trusted_service.PlatformExecutable {
	t.Helper()
	path := filepath.Join(root, "adapter-provider.sh")
	content := []byte(`#!/bin/sh
set -eu
while IFS= read -r line; do
  id=$(printf '%s\n' "$line" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
  case "$line" in
    *'"method":"adapter.health"'*) result='{"healthy":true}' ;;
    *'"method":"adapter.build"'*) result='{"id":"process-provider","handleId":"process-handle"}' ;;
    *'"method":"adapter.invoke"'*) result='{"invocationId":"process-invoke","output":{"provider":"process"}}' ;;
    *) result='{}' ;;
  esac
  printf '{"jsonrpc":"2.0","id":%s,"result":%s}\n' "$id" "$result"
done
`)
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	return trusted_service.PlatformExecutable{
		Platform: trusted_service.CurrentPlatform(),
		Path:     path,
		Sha256:   hex.EncodeToString(hash[:]),
		Signature: trusted_service.BinarySignature{
			Algorithm: "test",
			Value:     "adapter-provider-process-test",
			Trusted:   true,
		},
	}
}

func adapterProviderDefinition(serviceID string, executable trusted_service.PlatformExecutable) *trusted_service.ServiceRuntimeDefinition {
	return &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   serviceID,
		ExtensionID: "com.example/adapter-provider",
		ModuleID:    "main",
		TrustLevel:  string(trusted_service.TrustLevelTrusted),
		Executables: []trusted_service.PlatformExecutable{executable},
		Protocol:    "adapter-provider",
		HealthCheck: trusted_service.ServiceHealthCheck{
			Type: "none",
		},
		Recovery: trusted_service.ServiceRecoveryPolicy{
			RecoveryDecisionMode: trusted_service.RecoveryDecisionExternal,
		},
		Shutdown: trusted_service.ServiceShutdownPolicy{
			GracePeriod: 20 * time.Millisecond,
			KillTimeout: 2 * time.Second,
		},
		Network: trusted_service.ServiceNetworkPolicy{Enforce: false},
	}
}

func waitForAdapterRoute(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor, exists bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for router.Has(descriptor.ID, descriptor.ContractVersion) != exists {
		if time.Now().After(deadline) {
			t.Fatalf("adapter route existence = %t, want %t", router.Has(descriptor.ID, descriptor.ContractVersion), exists)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func callProcessBuild(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor) string {
	t.Helper()
	registration, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/process", ModuleID: "main"},
		remoteContribution(descriptor),
	)
	if err != nil {
		t.Fatal(err)
	}
	return registration.ID()
}

func callProcessBuildExpectError(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor) error {
	t.Helper()
	_, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/process", ModuleID: "main"},
		remoteContribution(descriptor),
	)
	return err
}

func callProcessInvoke(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor) string {
	t.Helper()
	result, err := router.InvokeAdapter(
		context.Background(),
		descriptor.ID,
		descriptor.ContractVersion,
		plugincontract.AdapterInvocationRequest{
			InvocationID:   "process-invoke",
			Module:         plugincontract.ModuleRef{PluginID: "com.example/process", ModuleID: "main"},
			ContributionID: "process",
			Operation:      "test",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	return output.Provider
}
