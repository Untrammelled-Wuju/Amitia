package developer_console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewDiagnosticRepository(t *testing.T) {
	repo := NewDiagnosticRepository(0)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.maxItems != 1000 {
		t.Errorf("expected maxItems=1000, got %d", repo.maxItems)
	}

	repo = NewDiagnosticRepository(500)
	if repo.maxItems != 500 {
		t.Errorf("expected maxItems=500, got %d", repo.maxItems)
	}
}

func TestDiagnosticRepository_RecordAndListInvocations(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	now := time.Now().UTC()

	repo.RecordInvocation(InvocationRecord{
		ID:          "inv-1",
		ExtensionID: "ext-a",
		StartedAt:   now,
		Status:      "completed",
	})
	repo.RecordInvocation(InvocationRecord{
		ID:          "inv-2",
		ExtensionID: "ext-b",
		StartedAt:   now,
		Status:      "running",
	})

	ctx := context.Background()
	all, err := repo.ListInvocations(ctx, ConsoleFilters{})
	if err != nil {
		t.Fatalf("ListInvocations error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 invocations, got %d", len(all))
	}

	filtered, _ := repo.ListInvocations(ctx, ConsoleFilters{ExtensionID: "ext-a"})
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered invocation, got %d", len(filtered))
	}
	if filtered[0].ID != "inv-1" {
		t.Errorf("expected inv-1, got %s", filtered[0].ID)
	}
}

func TestDiagnosticRepository_RecordAndListLogs(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	now := time.Now().UTC()

	repo.RecordLog(LogEntry{Extension: "ext-a", Level: "error", Message: "boom", At: now})
	repo.RecordLog(LogEntry{Extension: "ext-a", Level: "info", Message: "ok", At: now})
	repo.RecordLog(LogEntry{Extension: "ext-b", Level: "warn", Message: "careful", At: now})

	ctx := context.Background()
	all, _ := repo.ListLogs(ctx, ConsoleFilters{})
	if len(all) != 3 {
		t.Errorf("expected 3 logs, got %d", len(all))
	}

	extFilter, _ := repo.ListLogs(ctx, ConsoleFilters{ExtensionID: "ext-a"})
	if len(extFilter) != 2 {
		t.Errorf("expected 2 logs for ext-a, got %d", len(extFilter))
	}

	sevFilter, _ := repo.ListLogs(ctx, ConsoleFilters{Severity: "error"})
	if len(sevFilter) != 1 {
		t.Errorf("expected 1 error log, got %d", len(sevFilter))
	}
	if sevFilter[0].Message != "boom" {
		t.Errorf("expected 'boom', got '%s'", sevFilter[0].Message)
	}
}

func TestDiagnosticRepository_MaxItems(t *testing.T) {
	repo := NewDiagnosticRepository(3)
	for i := 0; i < 5; i++ {
		repo.RecordLog(LogEntry{
			Extension: "ext",
			Level:     "info",
			Message:   "msg",
			At:        time.Now().UTC(),
		})
	}
	ctx := context.Background()
	logs, _ := repo.ListLogs(ctx, ConsoleFilters{})
	if len(logs) != 3 {
		t.Errorf("expected 3 logs (maxItems), got %d", len(logs))
	}
}

func TestDiagnosticRepository_ExportDiagnostics_Empty(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	ctx := context.Background()

	export, err := repo.ExportDiagnostics(ctx, ConsoleFilters{})
	if err != nil {
		t.Fatalf("ExportDiagnostics error: %v", err)
	}
	if export == nil {
		t.Fatal("expected non-nil export")
	}
	if export.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
	if len(export.Invocations) != 0 {
		t.Errorf("expected 0 invocations, got %d", len(export.Invocations))
	}
	if len(export.Logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(export.Logs))
	}
}

func TestDiagnosticRepository_ExportDiagnostics_WithFilter(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	now := time.Now().UTC()

	repo.RecordInvocation(InvocationRecord{ID: "inv-1", ExtensionID: "ext-a", StartedAt: now, Status: "completed"})
	repo.RecordInvocation(InvocationRecord{ID: "inv-2", ExtensionID: "ext-b", StartedAt: now, Status: "running"})
	repo.RecordLog(LogEntry{Extension: "ext-a", Level: "error", Message: "err", At: now})
	repo.RecordLog(LogEntry{Extension: "ext-b", Level: "info", Message: "ok", At: now})
	repo.RecordEvent(EventRecord{ID: "evt-1", Type: "test", EmittedAt: now})
	repo.RecordHook(HookRecord{ID: "hk-1", Extension: "ext-a", InvokedAt: now})
	repo.RecordLifecycle(LifecycleEventRecord{Extension: "ext-a", Stage: "install", At: now})
	repo.RecordPerformance(PerformanceRecord{Extension: "ext-a", Metric: "cpu", Value: 42, At: now})

	ctx := context.Background()
	export, err := repo.ExportDiagnostics(ctx, ConsoleFilters{ExtensionID: "ext-a"})
	if err != nil {
		t.Fatalf("ExportDiagnostics error: %v", err)
	}

	if len(export.Invocations) != 1 {
		t.Errorf("expected 1 invocation for ext-a, got %d", len(export.Invocations))
	}
	if export.Invocations[0].ID != "inv-1" {
		t.Errorf("expected inv-1, got %s", export.Invocations[0].ID)
	}
	if len(export.Logs) != 1 {
		t.Errorf("expected 1 log for ext-a, got %d", len(export.Logs))
	}
	if export.Logs[0].Message != "err" {
		t.Errorf("expected 'err', got '%s'", export.Logs[0].Message)
	}
	if len(export.Hooks) != 1 {
		t.Errorf("expected 1 hook for ext-a, got %d", len(export.Hooks))
	}
	if len(export.Lifecycle) != 1 {
		t.Errorf("expected 1 lifecycle event for ext-a, got %d", len(export.Lifecycle))
	}
	if len(export.Performance) != 1 {
		t.Errorf("expected 1 performance record for ext-a, got %d", len(export.Performance))
	}
	if len(export.Events) != 1 {
		t.Errorf("expected 1 event (no extension filter), got %d", len(export.Events))
	}
}

func TestDiagnosticRepository_ExportDiagnostics_TimeRange(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	old := time.Now().Add(-2 * time.Hour).UTC()
	recent := time.Now().UTC()

	repo.RecordInvocation(InvocationRecord{ID: "inv-old", ExtensionID: "ext", StartedAt: old})
	repo.RecordInvocation(InvocationRecord{ID: "inv-recent", ExtensionID: "ext", StartedAt: recent})

	ctx := context.Background()
	since := time.Now().Add(-1 * time.Hour)
	export, _ := repo.ExportDiagnostics(ctx, ConsoleFilters{StartTime: &since})

	if len(export.Invocations) != 1 {
		t.Errorf("expected 1 recent invocation, got %d", len(export.Invocations))
	}
	if export.Invocations[0].ID != "inv-recent" {
		t.Errorf("expected inv-recent, got %s", export.Invocations[0].ID)
	}
}

func TestHTTPHandler_ExportDiagnostics(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	svc := NewConsoleService(nil)
	handler := NewHTTPHandler(svc, repo)

	mux := http.NewServeMux()
	handler.Register(mux)

	now := time.Now().UTC()
	repo.RecordLog(LogEntry{Extension: "ext-test", Level: "info", Message: "hello", At: now})

	req := httptest.NewRequest("GET", "/api/dev-console/export-diagnostics", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := result["overview"]; !ok {
		t.Error("expected 'overview' in response")
	}
	if _, ok := result["diagnostics"]; !ok {
		t.Error("expected 'diagnostics' in response")
	}
}

func TestHTTPHandler_ExportDiagnostics_WithFilter(t *testing.T) {
	repo := NewDiagnosticRepository(100)
	svc := NewConsoleService(nil)
	handler := NewHTTPHandler(svc, repo)

	mux := http.NewServeMux()
	handler.Register(mux)

	now := time.Now().UTC()
	repo.RecordLog(LogEntry{Extension: "ext-a", Level: "error", Message: "err", At: now})
	repo.RecordLog(LogEntry{Extension: "ext-b", Level: "info", Message: "ok", At: now})

	req := httptest.NewRequest("GET", "/api/dev-console/export-diagnostics?extension=ext-a", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	diagnostics, ok := result["diagnostics"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'diagnostics' to be a map")
	}
	logs, ok := diagnostics["logs"].([]interface{})
	if !ok {
		t.Fatal("expected 'logs' to be an array")
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log for ext-a, got %d", len(logs))
	}
}

func TestConsoleService_BuildOverview_NoAggregators(t *testing.T) {
	svc := NewConsoleService(nil)
	ctx := context.Background()
	overview, err := svc.BuildOverview(ctx)
	if err != nil {
		t.Fatalf("BuildOverview error: %v", err)
	}
	if overview == nil {
		t.Fatal("expected non-nil overview")
	}
	if overview.Extensions != 0 {
		t.Errorf("expected 0 extensions, got %d", overview.Extensions)
	}
	if !overview.GeneratedAt.IsZero() {
	}
}

func TestConsoleService_SessionLifecycle(t *testing.T) {
	svc := NewConsoleService(nil)
	ctx := context.Background()

	sess, err := svc.OpenSession(ctx, "workspace-1", time.Hour)
	if err != nil {
		t.Fatalf("OpenSession error: %v", err)
	}
	if sess.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	err = svc.CloseSession(sess.SessionID)
	if err != nil {
		t.Fatalf("CloseSession error: %v", err)
	}
}
