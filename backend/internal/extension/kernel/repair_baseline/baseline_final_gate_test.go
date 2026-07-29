package repair_baseline

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
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

type mockMCPDuplicateProvider struct {
	count    int64
	countErr error
	details  []kernel.MCPDuplicateDetail
	listErr  error
}

func (m *mockMCPDuplicateProvider) CountUnresolved(ctx context.Context) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.count, nil
}

func (m *mockMCPDuplicateProvider) ListUnresolved(ctx context.Context) ([]kernel.MCPDuplicateDetail, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.details, nil
}

func TestBaseline_FinalGateProbe_MetricNames(t *testing.T) {
	names := kernel.FinalGateMetricNames()
	required := []string{
		"duplicate_mcp_tool_registrations",
		"orphan_candidate_contributions",
		"orphan_runtime_instances",
		"orphan_ui_sessions",
		"duplicate_schedule_runs",
		"failed_cleanup_resources",
		"legacy_package_read_calls",
		"legacy_package_write_calls",
		"legacy_tool_execute_calls",
		"legacy_mcp_execute_calls",
		"duplicate_contribution_registrations",
		"orphan_sandbox_sessions",
		"audit_incomplete_operations",
		"lifecycle_requires_recovery",
	}
	if len(names) != len(required) {
		t.Fatalf("expected %d metric names, got %d", len(required), len(names))
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, r := range required {
		if !nameSet[r] {
			t.Fatalf("required metric %s not in FinalGateMetricNames()", r)
		}
	}
	t.Logf("FinalGateProbe: %d metric names verified", len(required))
}

func TestBaseline_FinalGateProbe_AllMetricsZeroOnFreshContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate probe test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "gate-probe.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	container.MCPDuplicateProvider = &mockMCPDuplicateProvider{count: 0, details: nil}

	report := container.EvaluateFinalGate(ctx)
	if !report.Passed {
		t.Fatalf("fresh container must pass final gate, errors=%v metrics=%v", report.Errors, report.Metrics)
	}
	for _, name := range kernel.FinalGateMetricNames() {
		val, ok := report.Metrics[name]
		if !ok {
			t.Fatalf("metric %s must be present in report", name)
		}
		if val != 0 {
			t.Fatalf("metric %s must be 0 on fresh container, got %d", name, val)
		}
	}
	t.Logf("FinalGateProbe: fresh container passed, all %d metrics are 0", len(report.Metrics))
}

func TestBaseline_FinalGateProbe_DetectsMCPDuplicateWithoutReadiness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate probe test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "gate-mcp.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	container.MCPDuplicateProvider = &mockMCPDuplicateProvider{
		count: 2,
		details: []kernel.MCPDuplicateDetail{
			{ToolID: "tool-1", ServerID: "srv-1", Owner: "ext-1", Generation: 1, DetectedAt: "2026-01-01T00:00:00Z"},
			{ToolID: "tool-2", ServerID: "srv-2", Owner: "ext-2", Generation: 1, DetectedAt: "2026-01-01T00:00:00Z"},
		},
	}

	report := container.EvaluateFinalGate(ctx)
	if report.Passed {
		t.Fatalf("gate must NOT pass when MCP duplicates exist")
	}
	if report.Metrics["duplicate_mcp_tool_registrations"] != 2 {
		t.Fatalf("expected duplicate_mcp_tool_registrations=2, got %d", report.Metrics["duplicate_mcp_tool_registrations"])
	}
	if len(report.Details) < 2 {
		t.Fatalf("expected at least 2 detail entries, got %d", len(report.Details))
	}

	container.MCPDuplicateProvider = &mockMCPDuplicateProvider{count: 0, details: nil}
	report2 := container.EvaluateFinalGate(ctx)
	if report2.Metrics["duplicate_mcp_tool_registrations"] != 0 {
		t.Fatalf("expected duplicate_mcp_tool_registrations=0 after cleanup, got %d", report2.Metrics["duplicate_mcp_tool_registrations"])
	}

	t.Logf("FinalGateProbe: detected %d MCP duplicates without Readiness, passed after cleanup", 2)
}

func TestBaseline_FinalGateProbe_FailClosedOnQueryError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate probe test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "gate-fail.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	container.MCPDuplicateProvider = &mockMCPDuplicateProvider{
		countErr: fmt.Errorf("database connection lost"),
	}

	report := container.EvaluateFinalGate(ctx)
	if report.Passed {
		t.Fatalf("gate must Fail Closed when query errors occur")
	}
	if len(report.Errors) == 0 {
		t.Fatalf("report must contain query errors")
	}
	foundMCPError := false
	for _, e := range report.Errors {
		if strings.Contains(e, "duplicate_mcp_tool_registrations") {
			foundMCPError = true
			break
		}
	}
	if !foundMCPError {
		t.Fatalf("report must contain MCP duplicate query error, got: %v", report.Errors)
	}

	t.Logf("FinalGateProbe: Fail Closed on query error, %d errors reported", len(report.Errors))
}

func TestBaseline_FinalGateProbe_DetectsOrphanCandidate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping final gate probe test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "gate-cand.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	orphanRecord := &kernel.CandidateRecord{
		CandidateID:         "orphan-cand-1",
		ExtensionID:         "ext-orphan",
		GenerationID:        "gen-1",
		CandidateGeneration: 1,
		Status:              kernel.CandidateStatusRegistered,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	if err := container.CandidateRepository.Save(ctx, orphanRecord); err != nil {
		t.Fatalf("save orphan candidate must succeed: %v", err)
	}

	report := container.EvaluateFinalGate(ctx)
	if report.Passed {
		t.Fatalf("gate must NOT pass when orphan candidates exist")
	}
	if report.Metrics["orphan_candidate_contributions"] != 1 {
		t.Fatalf("expected orphan_candidate_contributions=1, got %d", report.Metrics["orphan_candidate_contributions"])
	}

	if err := container.CandidateRepository.Delete(ctx, "orphan-cand-1"); err != nil {
		t.Fatalf("delete orphan candidate must succeed: %v", err)
	}
	report2 := container.EvaluateFinalGate(ctx)
	if report2.Metrics["orphan_candidate_contributions"] != 0 {
		t.Fatalf("expected orphan_candidate_contributions=0 after cleanup, got %d", report2.Metrics["orphan_candidate_contributions"])
	}

	t.Logf("FinalGateProbe: detected orphan candidate, passed after cleanup")
}
