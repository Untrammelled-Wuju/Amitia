package repair_baseline

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBaseline_Cutover_NoDirectSkillRuntimeInProduction(t *testing.T) {
	src := readServicesSource(t)
	if strings.Contains(src, "chatSvc.SetSkillRuntime(") {
		t.Fatalf("services.go must not call chatSvc.SetSkillRuntime in production; Phase 9 requires chat to use SetToolRuntime (Kernel ToolFacade) as the single main chain, not the legacy extension.Runtime")
	}
	if !strings.Contains(src, "chatSvc.SetToolRuntime(") {
		t.Fatalf("services.go must call chatSvc.SetToolRuntime to wire the Kernel ToolFacade as the production tool runtime")
	}
}

func TestBaseline_Cutover_ProductionSkipsPluginManagerStart(t *testing.T) {
	src := readServicesSource(t)
	if !strings.Contains(src, "SkipPluginManagerStart: true") {
		t.Fatalf("services.go must create the legacy runtime with SkipPluginManagerStart: true; Phase 9 forbids starting the old PluginManager in production")
	}
}

func TestBaseline_Cutover_LegacyCallCounterHasAllMetrics(t *testing.T) {
	src := readLegacyCallCounterSource(t)
	requiredMetrics := []string{
		"legacy_plugin_start",
		"legacy_plugin_dispatch",
		"legacy_tool_execute",
		"legacy_package_install",
		"legacy_skill_execute",
		"legacy_mcp_tool_register",
		"legacy_schedule_tick",
	}
	missing := []string{}
	for _, metric := range requiredMetrics {
		if !strings.Contains(src, metric) {
			missing = append(missing, metric)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("LegacyCallCounter must track all 7 legacy metrics required by Phase 9 section 13.5; missing=%v", missing)
	}
	if !strings.Contains(src, "func (c *LegacyCallCounter) Total()") {
		t.Fatalf("LegacyCallCounter must expose Total() for aggregate legacy call verification")
	}
	if !strings.Contains(src, "func (c *LegacyCallCounter) Snapshot()") {
		t.Fatalf("LegacyCallCounter must expose Snapshot() for dev console and readiness reporting")
	}
}

func TestBaseline_Cutover_PluginStartCounterWired(t *testing.T) {
	src := readLegacyRuntimeSource(t)
	if !strings.Contains(src, "GlobalLegacyCallCounter().IncPluginStart()") {
		t.Fatalf("runtime.go must call GlobalLegacyCallCounter().IncPluginStart() before pluginManager.Start; Phase 9 requires legacy_plugin_start monitoring")
	}
}

func TestBaseline_Cutover_LegacyDispatcherCountersWired(t *testing.T) {
	wiringSrc := readToolFacadeWiringSource(t)
	if strings.Contains(wiringSrc, "legacyDispatcherAdapter") {
		t.Fatalf("tool_facade_wiring.go must not contain legacyDispatcherAdapter; legacyDispatcherAdapter has been removed")
	}

	servicesSrc := readServicesSource(t)
	if strings.Contains(servicesSrc, "legacyDispatcher") {
		t.Fatalf("services.go must not reference legacyDispatcher; NewToolFacade must use 3 parameters without legacyDispatcher")
	}
}

func readServicesSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "cmd", "server", "services.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read services.go: %v", err)
	}
	return string(data)
}

func readLegacyCallCounterSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "legacy_call_counter.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read legacy_call_counter.go: %v", err)
	}
	return string(data)
}

func readLegacyRuntimeSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "runtime.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read runtime.go: %v", err)
	}
	return string(data)
}

func readToolFacadeWiringSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "cmd", "server", "tool_facade_wiring.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tool_facade_wiring.go: %v", err)
	}
	return string(data)
}
