package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestTopologyIntegration_RuntimeWithServices(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.topology")
	rtID := domain.RuntimeInstanceID("runtime-topology")
	svcA := domain.ServiceID("bridge")
	svcB := domain.ServiceID("agent")

	rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svcPathsA, err := dm.EnsureServicePaths(ctx, rtID, svcA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svcPathsB, err := dm.EnsureServicePaths(ctx, rtID, svcB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svcPathsA.Root == svcPathsB.Root {
		t.Fatal("service A and service B must have different roots")
	}
	if !stringsHasPrefix(svcPathsA.Root, rtPaths.Services) {
		t.Fatalf("service A root %s should be under rt services %s", svcPathsA.Root, rtPaths.Services)
	}
	if !stringsHasPrefix(svcPathsB.Root, rtPaths.Services) {
		t.Fatalf("service B root %s should be under rt services %s", svcPathsB.Root, rtPaths.Services)
	}

	pluginPaths, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pluginPaths.Data == rtPaths.Data {
		t.Fatal("plugin data should differ from runtime data")
	}
	if pluginPaths.Cache == rtPaths.Cache {
		t.Fatal("plugin cache should differ from runtime cache")
	}
}

func TestRuntimeTopologySnapshot_Mapping(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.snapshot")
	rtID := domain.RuntimeInstanceID("runtime-snapshot")
	services := []domain.ServiceID{"bridge", "agent", "vision"}

	pluginPaths, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, svcID := range services {
		_, err := dm.EnsureServicePaths(ctx, rtID, svcID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	rtCtx, err := dm.BuildRuntimeContext(pluginID, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rtCtx.Plugin.Root != pluginPaths.Root {
		t.Fatalf("plugin root mismatch: %s vs %s", rtCtx.Plugin.Root, pluginPaths.Root)
	}
	if rtCtx.Runtime.Root != rtPaths.Root {
		t.Fatalf("runtime root mismatch: %s vs %s", rtCtx.Runtime.Root, rtPaths.Root)
	}

	for _, svcID := range services {
		svcCtx, err := dm.BuildServiceContext(pluginID, rtID, svcID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svcCtx.Plugin.Root != pluginPaths.Root {
			t.Fatalf("service context plugin root mismatch")
		}
		if svcCtx.Runtime.Root != rtPaths.Root {
			t.Fatalf("service context runtime root mismatch")
		}
		if !stringsHasPrefix(svcCtx.Service.Root, rtPaths.Services) {
			t.Fatalf("service root not under rt services")
		}
	}
}

func TestServiceData_IsolationFromRuntimeData(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	rtID := domain.RuntimeInstanceID("runtime-isolation")
	svcID := domain.ServiceID("svc-isolation")

	rtPaths, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svcPaths, err := dm.EnsureServicePaths(ctx, rtID, svcID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rtPaths.Data == svcPaths.Data {
		t.Fatal("runtime data and service data must differ")
	}

	rtFilePath := filepath.Join(rtPaths.Data, "runtime_state.json")
	svcFilePath := filepath.Join(svcPaths.Data, "service_state.json")

	if err := writeTestFile(rtFilePath, "runtime"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeTestFile(svcFilePath, "service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rtContent, err := readTestFile(rtFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svcContent, err := readTestFile(svcFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rtContent == svcContent {
		t.Fatal("runtime and service data must be isolated")
	}
}

func TestDirectoryStructure_MatchesSpecification(t *testing.T) {
	dm, dataRoot := newTestDirManager(t)
	ctx := context.Background()

	pluginID := domain.PluginID("com.example.spec")
	rtID := domain.RuntimeInstanceID("runtime-spec")
	svcID := domain.ServiceID("svc-spec")

	_, err := dm.EnsurePluginPaths(ctx, pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = dm.EnsureServicePaths(ctx, rtID, svcID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDirs := []string{
		"data", "cache", "shared",
	}
	for _, dir := range expectedDirs {
		path := filepath.Join(dataRoot, "gamehost", "plugins", "*", dir)
		matches, err := filepath.Glob(path)
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		if len(matches) == 0 {
			t.Logf("warning: no match for %s", path)
		}
	}

	rtDirs := []string{"data", "temp", "cache", "services"}
	for _, dir := range rtDirs {
		path := filepath.Join(dataRoot, "gamehost", "runtimes", "*", dir)
		matches, err := filepath.Glob(path)
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		if len(matches) == 0 {
			t.Logf("warning: no match for %s", path)
		}
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func readTestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
