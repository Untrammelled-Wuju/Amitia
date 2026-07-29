package repair_baseline

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
)

func TestBaseline_FinalGate_AllMetricsZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	targetDir := filepath.Join(extRoot, "tool-basic")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != amitiax.InstallSucceeded {
		t.Fatalf("install must succeed: %v", result.Errors)
	}
	if err := container.DefinitionRepository.PutExtension(ctx, result.Definition); err != nil {
		t.Fatalf("PutExtension must succeed: %v", err)
	}

	if container.EnablementService != nil {
		subject := enablement.StateSubject{
			Kind: enablement.SubjectExtension,
			ID:   string(result.Definition.ID),
		}
		_ = container.EnablementService.Enable(ctx, subject)
		_ = container.EnablementService.Disable(ctx, subject)
	}

	if container.ToolFacade != nil {
		_, _ = container.ToolFacade.ModelTools(ctx, kernel.LegacyScope{
			UserID:    "final-gate",
			Channel:   "test",
			SessionID: "final-gate-verify",
		})
	}

	counter := kernel.GlobalLegacyCallCounter()
	gateMetrics := counter.FinalGateMetrics()
	requiredZeroMetrics := []string{
		"legacy_tool_execute_calls",
		"legacy_mcp_execute_calls",
		"legacy_package_write_calls",
		"legacy_package_read_calls",
		"duplicate_contribution_registrations",
		"orphan_runtime_instances",
		"orphan_ui_sessions",
		"failed_cleanup_resources",
		"duplicate_mcp_tool_registrations",
	}

	if len(gateMetrics) != len(requiredZeroMetrics) {
		t.Fatalf("expected %d gate metrics, got %d", len(requiredZeroMetrics), len(gateMetrics))
	}

	for _, metric := range requiredZeroMetrics {
		val, ok := gateMetrics[metric]
		if !ok {
			t.Fatalf("gate metric %s must be present in FinalGateMetrics()", metric)
		}
		if val != 0 {
			t.Fatalf("final gate metric %s must be 0, got %d", metric, val)
		}
	}

	if !counter.FinalGatePassed() {
		t.Fatalf("FinalGatePassed() must return true when all metrics are 0")
	}

	t.Logf("FinalGate: all %d gate metrics are 0, FinalGatePassed=true", len(requiredZeroMetrics))
}

func TestBaseline_FinalGate_LegacyZeroCallAfterLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate legacy test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	counter := kernel.GlobalLegacyCallCounter()
	beforeTotal := counter.Total()
	beforeMetrics := counter.FinalGateMetrics()

	tempDir := t.TempDir()
	for i := 0; i < 5; i++ {
		container, err := kernel.NewContainerBuilder().
			WithDBPath(filepath.Join(tempDir, fmt.Sprintf("gate-%d.db", i))).
			WithExtensionRoot(filepath.Join(tempDir, fmt.Sprintf("ext-%d", i))).
			Build(ctx)
		if err != nil {
			t.Fatalf("iteration %d Build must succeed: %v", i, err)
		}
		facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
		scope := kernel.LegacyScope{
			UserID:    fmt.Sprintf("gate-%d", i),
			Channel:   "test",
			SessionID: fmt.Sprintf("gate-session-%d", i),
		}
		_, _ = facade.ModelTools(ctx, scope)
		container.Close()
	}

	afterTotal := counter.Total()
	afterMetrics := counter.FinalGateMetrics()

	if afterTotal != beforeTotal {
		t.Fatalf("LegacyCallCounter.Total() must not grow: before=%d after=%d", beforeTotal, afterTotal)
	}

	for metric, before := range beforeMetrics {
		after := afterMetrics[metric]
		if before != after {
			t.Fatalf("gate metric %s must not change: before=%d after=%d", metric, before, after)
		}
	}

	if !counter.FinalGatePassed() {
		t.Fatalf("FinalGatePassed() must return true after lifecycle")
	}

	t.Logf("FinalGate: LegacyCallCounter stayed at %d, all gate metrics unchanged", afterTotal)
	runtime.KeepAlive(counter)
}

func TestBaseline_FinalGate_NoFalsePassed(t *testing.T) {
	counter := kernel.NewLegacyCallCounter()

	if !counter.FinalGatePassed() {
		t.Fatalf("fresh counter must pass final gate (all zeros)")
	}

	counter.IncToolExecuteCalls()
	if counter.FinalGatePassed() {
		t.Fatalf("counter with legacy_tool_execute_calls=1 must NOT pass final gate")
	}

	counter2 := kernel.NewLegacyCallCounter()
	counter2.IncOrphanUISession()
	if counter2.FinalGatePassed() {
		t.Fatalf("counter with orphan_ui_sessions=1 must NOT pass final gate")
	}

	counter3 := kernel.NewLegacyCallCounter()
	counter3.IncFailedCleanupResource()
	if counter3.FinalGatePassed() {
		t.Fatalf("counter with failed_cleanup_resources=1 must NOT pass final gate")
	}

	counter4 := kernel.NewLegacyCallCounter()
	counter4.IncPackageWriteCalls()
	if counter4.FinalGatePassed() {
		t.Fatalf("counter with legacy_package_write_calls=1 must NOT pass final gate")
	}

	counter5 := kernel.NewLegacyCallCounter()
	counter5.IncDuplicateContributionRegistration()
	if counter5.FinalGatePassed() {
		t.Fatalf("counter with duplicate_contribution_registrations=1 must NOT pass final gate")
	}

	counter6 := kernel.NewLegacyCallCounter()
	counter6.IncOrphanRuntimeInstance()
	if counter6.FinalGatePassed() {
		t.Fatalf("counter with orphan_runtime_instances=1 must NOT pass final gate")
	}

	counter7 := kernel.NewLegacyCallCounter()
	counter7.IncMCPExecute()
	if counter7.FinalGatePassed() {
		t.Fatalf("counter with legacy_mcp_execute_calls=1 must NOT pass final gate")
	}

	counter8 := kernel.NewLegacyCallCounter()
	counter8.SetDuplicateMCPFromRegistry(1)
	if counter8.FinalGatePassed() {
		t.Fatalf("counter with duplicate_mcp_tool_registrations=1 must NOT pass final gate")
	}

	counter8.SetDuplicateMCPFromRegistry(0)
	if !counter8.FinalGatePassed() {
		t.Fatalf("counter with duplicate_mcp_tool_registrations=0 must pass final gate after cleanup")
	}

	t.Logf("FinalGate: all 8 metrics correctly block final gate when non-zero")
}

func TestBaseline_FinalGate_GateMetricsSnapshot(t *testing.T) {
	counter := kernel.NewLegacyCallCounter()
	metrics := counter.FinalGateMetrics()

	expectedMetrics := []string{
		"legacy_tool_execute_calls",
		"legacy_mcp_execute_calls",
		"legacy_package_write_calls",
		"legacy_package_read_calls",
		"duplicate_contribution_registrations",
		"orphan_runtime_instances",
		"orphan_ui_sessions",
		"failed_cleanup_resources",
		"duplicate_mcp_tool_registrations",
	}

	if len(metrics) != len(expectedMetrics) {
		t.Fatalf("expected %d gate metrics, got %d", len(expectedMetrics), len(metrics))
	}

	for _, m := range expectedMetrics {
		if _, ok := metrics[m]; !ok {
			t.Fatalf("gate metric %s must be present", m)
		}
	}

	snap := counter.Snapshot()
	for _, m := range expectedMetrics {
		if _, ok := snap[m]; !ok {
			t.Fatalf("gate metric %s must also be in Snapshot()", m)
		}
	}

	t.Logf("FinalGate: %d gate metrics all present in FinalGateMetrics() and Snapshot()", len(expectedMetrics))
}
