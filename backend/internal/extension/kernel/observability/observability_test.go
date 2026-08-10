package observability

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status   ExecutionStatus
		terminal bool
	}{
		{StatusSucceeded, true},
		{StatusFailed, true},
		{StatusCancelled, true},
		{StatusTimedOut, true},
		{StatusDenied, true},
		{StatusRateLimited, true},
		{StatusCircuitOpen, true},
		{StatusInvalid, true},
		{StatusPartiallySucceeded, true},
		{StatusInterrupted, true},
		{StatusCreated, false},
		{StatusQueued, false},
		{StatusRunning, false},
		{StatusRetrying, false},
		{StatusAwaitingApproval, false},
	}

	for _, tt := range tests {
		if tt.status.IsTerminal() != tt.terminal {
			t.Errorf("status %q IsTerminal() = %v, want %v", tt.status, tt.status.IsTerminal(), tt.terminal)
		}
	}
}

func TestIsTransitionValid(t *testing.T) {
	validTransitions := []struct {
		from ExecutionStatus
		to   ExecutionStatus
	}{
		{StatusCreated, StatusQueued},
		{StatusCreated, StatusDenied},
		{StatusCreated, StatusInvalid},
		{StatusQueued, StatusAwaitingApproval},
		{StatusQueued, StatusRunning},
		{StatusQueued, StatusCancelled},
		{StatusQueued, StatusInvalid},
		{StatusAwaitingApproval, StatusRunning},
		{StatusAwaitingApproval, StatusDenied},
		{StatusAwaitingApproval, StatusCancelled},
		{StatusAwaitingApproval, StatusTimedOut},
		{StatusRunning, StatusSucceeded},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusPartiallySucceeded},
		{StatusRunning, StatusCancelled},
		{StatusRunning, StatusTimedOut},
		{StatusRunning, StatusInterrupted},
		{StatusRunning, StatusRetrying},
		{StatusRunning, StatusRateLimited},
		{StatusRunning, StatusCircuitOpen},
		{StatusRetrying, StatusRunning},
		{StatusRetrying, StatusFailed},
		{StatusRetrying, StatusCancelled},
		{StatusRetrying, StatusTimedOut},
	}

	for _, tt := range validTransitions {
		if err := IsTransitionValid(tt.from, tt.to); err != nil {
			t.Errorf("expected valid transition %q -> %q, got error: %v", tt.from, tt.to, err)
		}
	}

	invalidTransitions := []struct {
		from ExecutionStatus
		to   ExecutionStatus
	}{
		{StatusSucceeded, StatusRunning},
		{StatusFailed, StatusSucceeded},
		{StatusCancelled, StatusRetrying},
		{StatusDenied, StatusRunning},
		{StatusSucceeded, StatusCreated},
	}

	for _, tt := range invalidTransitions {
		if err := IsTransitionValid(tt.from, tt.to); err == nil {
			t.Errorf("expected invalid transition %q -> %q, got nil error", tt.from, tt.to)
		}
	}
}

func TestMemoryStoreSaveAndGetInvocation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	inv := InvocationRecord{
		InvocationID: "inv-1",
		TraceID:      "trace-1",
		OperationID:  "op-1",
		CapabilityID: "tool-1",
		Status:       StatusCreated,
		CreatedAt:    time.Now(),
	}

	err := store.SaveInvocation(ctx, inv)
	if err != nil {
		t.Fatalf("SaveInvocation failed: %v", err)
	}

	got, err := store.GetInvocation(ctx, "inv-1")
	if err != nil {
		t.Fatalf("GetInvocation failed: %v", err)
	}

	if got.InvocationID != "inv-1" {
		t.Errorf("expected InvocationID 'inv-1', got %q", got.InvocationID)
	}
	if got.TraceID != "trace-1" {
		t.Errorf("expected TraceID 'trace-1', got %q", got.TraceID)
	}

	_, err = store.GetInvocation(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent invocation")
	}
}

func TestMemoryStoreUpdateInvocationStatus(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	inv := InvocationRecord{
		InvocationID: "inv-status-1",
		Status:       StatusCreated,
		CreatedAt:    time.Now(),
	}
	_ = store.SaveInvocation(ctx, inv)

	err := store.UpdateInvocationStatus(ctx, "inv-status-1", StatusQueued)
	if err != nil {
		t.Fatalf("expected valid transition, got error: %v", err)
	}

	err = store.UpdateInvocationStatus(ctx, "inv-status-1", StatusCreated)
	if err == nil {
		t.Error("expected error for invalid backward transition")
	}

	_ = store.UpdateInvocationStatus(ctx, "inv-status-1", StatusRunning)
	_ = store.UpdateInvocationStatus(ctx, "inv-status-1", StatusSucceeded)

	err = store.UpdateInvocationStatus(ctx, "inv-status-1", StatusRunning)
	if err == nil {
		t.Error("expected error for transition from terminal status")
	}
}

func TestMemoryStoreListInvocations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		inv := InvocationRecord{
			InvocationID: "inv-list-ext1-" + string(rune('0'+i)),
			ExtensionID:  "ext-1",
			Status:       StatusSucceeded,
			CreatedAt:    time.Now(),
		}
		_ = store.SaveInvocation(ctx, inv)
	}

	for i := 6; i <= 8; i++ {
		inv := InvocationRecord{
			InvocationID: "inv-list-ext2-" + string(rune('0'+i)),
			ExtensionID:  "ext-2",
			Status:       StatusFailed,
			CreatedAt:    time.Now(),
		}
		_ = store.SaveInvocation(ctx, inv)
	}

	inv := InvocationRecord{
		InvocationID: "inv-list-pagination-1",
		ExtensionID:  "ext-1",
		Status:       StatusSucceeded,
		CreatedAt:    time.Now(),
	}
	_ = store.SaveInvocation(ctx, inv)

	results, _, err := store.ListInvocations(ctx, InvocationFilter{
		ExtensionID: "ext-1",
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListInvocations failed: %v", err)
	}
	if len(results) != 6 {
		t.Errorf("expected 6 results for ext-1, got %d", len(results))
	}

	results, _, err = store.ListInvocations(ctx, InvocationFilter{
		Status:      StatusFailed,
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListInvocations failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results for failed status, got %d", len(results))
	}
}

func TestMemoryStoreGetInvocationChildren(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "parent-1",
		Status:       StatusRunning,
		CreatedAt:    time.Now(),
	})
	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "child-1",
		ParentID:     "parent-1",
		Status:       StatusSucceeded,
		CreatedAt:    time.Now(),
	})
	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "child-2",
		ParentID:     "parent-1",
		Status:       StatusFailed,
		CreatedAt:    time.Now(),
	})

	children, err := store.GetInvocationChildren(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetInvocationChildren failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestMemoryStoreAttempts(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	att1 := ExecutionAttempt{
		AttemptID:     "att-1",
		InvocationID:  "inv-att-1",
		AttemptNumber: 1,
		Status:        StatusFailed,
		StartedAt:     time.Now(),
	}
	att2 := ExecutionAttempt{
		AttemptID:     "att-2",
		InvocationID:  "inv-att-1",
		AttemptNumber: 2,
		Status:        StatusSucceeded,
		StartedAt:     time.Now(),
	}

	_ = store.SaveAttempt(ctx, att1)
	_ = store.SaveAttempt(ctx, att2)

	got, err := store.GetAttempt(ctx, "att-1")
	if err != nil {
		t.Fatalf("GetAttempt failed: %v", err)
	}
	if got.AttemptID != "att-1" {
		t.Errorf("expected AttemptID 'att-1', got %q", got.AttemptID)
	}

	attempts, err := store.ListAttemptsByInvocation(ctx, "inv-att-1")
	if err != nil {
		t.Fatalf("ListAttemptsByInvocation failed: %v", err)
	}
	if len(attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", len(attempts))
	}
}

func TestMemoryStoreAuditEvents(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	evt := AuditEvent{
		AuditID:   "audit-1",
		TraceID:   "trace-1",
		Action:    "permission.evaluate",
		Decision:  "denied",
		Result:    "permission_denied",
		RiskLevel: "high",
		CreatedAt: time.Now(),
	}

	_ = store.SaveAuditEvent(ctx, evt)

	events, _, err := store.ListAuditEvents(ctx, AuditFilter{
		Action:      "permission.evaluate",
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListAuditEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 audit event, got %d", len(events))
	}

	got, err := store.GetAuditEvent(ctx, "audit-1")
	if err != nil {
		t.Fatalf("GetAuditEvent failed: %v", err)
	}
	if got.Decision != "denied" {
		t.Errorf("expected decision 'denied', got %q", got.Decision)
	}
}

func TestMemoryStoreSaveAndGetOperation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	op := OperationRecord{
		OperationID: "op-1",
		TraceID:     "trace-1",
		Type:        OpToolExecute,
		ActorType:   ActorUser,
		ActorID:     "user-1",
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Status:      StatusSucceeded,
		StartedAt:   time.Now(),
		CreatedAt:   time.Now(),
	}

	_ = store.SaveOperation(ctx, op)

	got, err := store.GetOperation(ctx, "op-1")
	if err != nil {
		t.Fatalf("GetOperation failed: %v", err)
	}
	if got.Type != OpToolExecute {
		t.Errorf("expected OpToolExecute, got %q", got.Type)
	}
}

func TestMemoryStoreRuntimeEvents(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.SaveRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      "evt-1",
		InvocationID: "inv-1",
		EventType:    "invocation.started",
		Severity:     "info",
		Timestamp:    time.Now(),
	})
	_ = store.SaveRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      "evt-2",
		InvocationID: "inv-1",
		EventType:    "invocation.finished",
		Severity:     "error",
		Timestamp:    time.Now(),
	})

	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{
		InvocationID: "inv-1",
		ListOptions:  ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListRuntimeEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestSanitizerRedactsSecrets(t *testing.T) {
	s := NewRecordSanitizer()

	inv := &InvocationRecord{InvocationID: "inv-1"}
	s.SanitizeInvocationInput(inv, "API key: sk-abc123def456ghi789jkl012mno345pqr678stu")
	if inv.InputHash == "" {
		t.Error("expected InputHash to be set")
	}
	if strings.Contains(inv.InputSummary, "sk-abc123") {
		t.Error("expected secret to be redacted from input summary")
	}
}

func TestSanitizeMetadataStripsSecretKeys(t *testing.T) {
	s := NewRecordSanitizer()

	meta := map[string]any{
		"token":    "secret-token",
		"name":     "public-name",
		"api_key":  "sk-secret",
		"password": "p@ssw0rd",
		"public":   "visible",
	}

	cleaned := s.SanitizeMetadata(meta)

	if _, ok := cleaned["token"]; ok {
		t.Error("expected 'token' to be removed as secret")
	}
	if _, ok := cleaned["api_key"]; ok {
		t.Error("expected 'api_key' to be removed as secret")
	}
	if _, ok := cleaned["password"]; ok {
		t.Error("expected 'password' to be removed as secret")
	}
	if cleaned["name"] != "public-name" {
		t.Errorf("expected 'name' to be preserved, got %v", cleaned["name"])
	}
}

func TestPagination(t *testing.T) {
	results := make([]InvocationRecord, 0)
	for i := 1; i <= 10; i++ {
		results = append(results, InvocationRecord{
			InvocationID: string(rune('0'+i)) + "-inv-id",
		})
	}

	page, cursor, err := paginateInvs(results, 3, "")
	if err != nil {
		t.Fatalf("paginateInvs failed: %v", err)
	}
	if len(page) != 3 {
		t.Errorf("expected 3 items in first page, got %d", len(page))
	}
	if cursor == "" {
		t.Error("expected non-empty cursor after first page")
	}

	page2, cursor2, err := paginateInvs(results, 3, cursor)
	if err != nil {
		t.Fatalf("paginateInvs page 2 failed: %v", err)
	}
	if len(page2) != 3 {
		t.Errorf("expected 3 items in second page, got %d", len(page2))
	}

	page3, cursor3, err := paginateInvs(results, 3, cursor2)
	if err != nil {
		t.Fatalf("paginateInvs page 3 failed: %v", err)
	}
	if len(page3) != 3 {
		t.Errorf("expected 3 items in third page, got %d", len(page3))
	}

	page4, cursor4, err := paginateInvs(results, 3, cursor3)
	if err != nil {
		t.Fatalf("paginateInvs page 4 failed: %v", err)
	}
	if len(page4) != 1 {
		t.Errorf("expected 1 item in fourth page, got %d", len(page4))
	}
	if cursor4 != "" {
		t.Errorf("expected empty cursor for last page, got %q", cursor4)
	}
}

func TestIDGeneration(t *testing.T) {
	id1 := NewTraceID()
	id2 := NewTraceID()
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty IDs")
	}

	if NewOperationID() == "" {
		t.Error("expected non-empty OperationID")
	}
	if NewInvocationID() == "" {
		t.Error("expected non-empty InvocationID")
	}
	if NewAttemptID() == "" {
		t.Error("expected non-empty AttemptID")
	}
}

func TestHashFunctions(t *testing.T) {
	h1 := HashInput("hello world")
	h2 := HashInput("hello world")
	h3 := HashInput("different")

	if h1 != h2 {
		t.Error("expected same hash for same input")
	}
	if h1 == h3 {
		t.Error("expected different hash for different input")
	}
	if HashInput("") != "" {
		t.Error("expected empty hash for empty input")
	}
}

func TestDefaultRecordWriter(t *testing.T) {
	store := NewMemoryStore()
	config := DefaultWriterConfig()
	config.FlushInterval = 100
	config.BatchSize = 5
	writer := NewRecordWriter(store, config)
	defer writer.Close()

	ctx := context.Background()

	err := writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: "wr-inv-1",
		Status:       StatusCreated,
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("WriteInvocation failed: %v", err)
	}

	_ = writer.Flush(ctx)

	inv, err := store.GetInvocation(ctx, "wr-inv-1")
	if err != nil {
		t.Fatalf("GetInvocation after write failed: %v", err)
	}
	if inv.InvocationID != "wr-inv-1" {
		t.Errorf("expected InvocationID 'wr-inv-1', got %q", inv.InvocationID)
	}
}

func TestDefaultQueryServiceInvocationTree(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "root-1",
		Status:       StatusSucceeded,
		CreatedAt:    time.Now(),
	})
	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "child-1",
		ParentID:     "root-1",
		Status:       StatusSucceeded,
		CreatedAt:    time.Now(),
	})
	_ = store.SaveInvocation(ctx, InvocationRecord{
		InvocationID: "child-2",
		ParentID:     "root-1",
		Status:       StatusFailed,
		CreatedAt:    time.Now(),
	})

	qs := NewQueryService(store)
	tree, err := qs.GetInvocationTree(ctx, "root-1")
	if err != nil {
		t.Fatalf("GetInvocationTree failed: %v", err)
	}

	if tree.Invocation.InvocationID != "root-1" {
		t.Errorf("expected root InvocationID 'root-1'")
	}
	if len(tree.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(tree.Children))
	}
}

func TestExecutionHookOnInvocationCreated(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	inv := capability.ToolInvocationContext{
		InvocationID: "hook-inv-1",
		UserID:       "user-1",
		Source:       capability.InvocationSourceModel,
	}

	traceID := hook.OnInvocationCreated(ctx, inv, "tool-1")
	if traceID == "" {
		t.Error("expected non-empty trace ID")
	}

	_ = writer.Flush(ctx)

	got, err := store.GetInvocation(ctx, "hook-inv-1")
	if err != nil {
		t.Fatalf("GetInvocation failed: %v", err)
	}
	if got.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got %q", got.UserID)
	}
	if got.CapabilityID != "tool-1" {
		t.Errorf("expected CapabilityID 'tool-1', got %q", got.CapabilityID)
	}
}

func TestExecutionHookEvents(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	hook.OnInvocationQueued(ctx, "inv-evt-1")
	hook.OnInvocationStarted(ctx, "inv-evt-1")
	hook.OnInvocationFinished(ctx, "inv-evt-1", StatusSucceeded, "")

	_ = writer.Flush(ctx)

	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{
		InvocationID: "inv-evt-1",
		ListOptions:  ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListRuntimeEvents failed: %v", err)
	}
	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}
}

func TestExecutionHookAttempts(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	hook.OnAttemptStarted(ctx, "inv-att-1", 1, "plugin", "runtime-1")
	hook.OnAttemptStarted(ctx, "inv-att-1", 2, "plugin", "runtime-1")

	_ = writer.Flush(ctx)

	attempts, err := store.ListAttemptsByInvocation(ctx, "inv-att-1")
	if err != nil {
		t.Fatalf("ListAttemptsByInvocation failed: %v", err)
	}
	if len(attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", len(attempts))
	}
}

func TestMigrationMappingsCoverage(t *testing.T) {
	mappings := DefaultMigrationMappings()
	if len(mappings) == 0 {
		t.Error("expected non-empty migration mappings")
	}

	expectedTables := []string{
		"plugin_runs", "mcp_operations", "workflow_runs",
		"extension_runs", "package_operations", "agent_skill_activations",
	}

	for _, expected := range expectedTables {
		found := false
		for _, m := range mappings {
			if m.OldTable == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected migration mapping for table %q", expected)
		}
	}
}

func TestErrorCodeToCategory(t *testing.T) {
	if cat := ErrorCodeToCategory("invalid_input"); cat != "validation" {
		t.Errorf("expected 'validation', got %q", cat)
	}
	if cat := ErrorCodeToCategory("permission_denied"); cat != "security" {
		t.Errorf("expected 'security', got %q", cat)
	}
	if cat := ErrorCodeToCategory("timeout"); cat != "timeout" {
		t.Errorf("expected 'timeout', got %q", cat)
	}
	if cat := ErrorCodeToCategory("rate_limited"); cat != "capacity" {
		t.Errorf("expected 'capacity', got %q", cat)
	}
	if cat := ErrorCodeToCategory("unknown_code"); cat != "unknown" {
		t.Errorf("expected 'unknown', got %q", cat)
	}
}

func TestErrorCodeToRetryable(t *testing.T) {
	if !ErrorCodeToRetryable("timeout") {
		t.Error("expected timeout to be retryable")
	}
	if !ErrorCodeToRetryable("connection_lost") {
		t.Error("expected connection_lost to be retryable")
	}
	if ErrorCodeToRetryable("invalid_input") {
		t.Error("expected invalid_input to be non-retryable")
	}
	if ErrorCodeToRetryable("permission_denied") {
		t.Error("expected permission_denied to be non-retryable")
	}
}

func TestExecutionHookRecordLifecycleEvent(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	hook.RecordLifecycleEvent(ctx, OpExtensionInstall, ActorSystem, "system", SubjectExtension, "ext-1", StatusSucceeded)

	_ = writer.Flush(ctx)

	ops, _, err := store.ListOperations(ctx, OperationFilter{
		Type:        OpExtensionInstall,
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListOperations failed: %v", err)
	}
	if len(ops) == 0 {
		t.Error("expected at least 1 operation record")
	}

	events, _, err := store.ListAuditEvents(ctx, AuditFilter{
		Action:      string(OpExtensionInstall),
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListAuditEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least 1 audit event for lifecycle")
	}
}

func TestExecutionHookAuditForGrant(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	grant := permission.PermissionGrant{
		GrantID:      "grant-1",
		PermissionID: "files.read",
		Decision:     permission.DecisionAllow,
	}

	hook.WriteAuditForGrant(ctx, grant, ActorUser, "user-1")
	_ = writer.Flush(ctx)

	events, _, err := store.ListAuditEvents(ctx, AuditFilter{
		Action:      "permission.grant",
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListAuditEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least 1 audit event for grant")
	}
}

func TestExecutionHookSideEffects(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	effects := []capability.RecordedSideEffect{
		{Type: "write", Target: "file.txt", Description: "created file"},
		{Type: "network", Target: "https://api.example.com", Description: "sent request"},
	}

	hook.OnSideEffectRecorded(ctx, "inv-se-1", effects)
	_ = writer.Flush(ctx)

	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{
		InvocationID: "inv-se-1",
		ListOptions:  ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListRuntimeEvents failed: %v", err)
	}
	if len(events) < 2 {
		t.Errorf("expected at least 2 side effect events, got %d", len(events))
	}
}

func TestExecutionHookPermissionDecision(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	result := permission.PermissionEvaluationResult{
		Decision:      permission.DecisionDeny,
		Missing:       []permission.PermissionRequirement{{PermissionID: "files.write"}},
		MatchedGrants: []permission.PermissionGrant{},
		Reasons:       []permission.PermissionReason{{Code: "no_grant", Permission: "files.write"}},
	}

	hook.OnPermissionDecision(ctx, capability.ToolInvocationContext{
		InvocationID: "inv-perm-1",
		UserID:       "user-1",
	}, "tool-1", result)

	_ = writer.Flush(ctx)

	events, _, err := store.ListAuditEvents(ctx, AuditFilter{
		Action:      "permission.evaluate",
		ListOptions: ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListAuditEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least 1 audit event for permission decision")
	}
}

func TestStatusForUnifiedResult(t *testing.T) {
	tests := []struct {
		result capability.UnifiedToolResult
		want   ExecutionStatus
	}{
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess},
			StatusSucceeded,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed, Error: &capability.ToolError{Code: capability.ErrorCodePermissionDenied}},
			StatusDenied,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed, Error: &capability.ToolError{Code: capability.ErrorCodeCancelled}},
			StatusCancelled,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed, Error: &capability.ToolError{Code: capability.ErrorCodeTimeout}},
			StatusTimedOut,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusCancelled},
			StatusCancelled,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusTimedOut},
			StatusTimedOut,
		},
		{
			capability.UnifiedToolResult{Status: capability.ToolResultStatusFailed, Error: &capability.ToolError{Code: "unknown"}},
			StatusFailed,
		},
	}

	for _, tt := range tests {
		got := StatusForUnifiedResult(tt.result)
		if got != tt.want {
			t.Errorf("StatusForUnifiedResult(status=%q, err=%v) = %q, want %q",
				tt.result.Status, tt.result.Error, got, tt.want)
		}
	}
}

func TestStatusForExecutionError(t *testing.T) {
	if s := StatusForExecutionError(nil); s != StatusFailed {
		t.Errorf("expected StatusFailed for nil error, got %q", s)
	}
	if s := StatusForExecutionError(&capability.ToolError{Code: capability.ErrorCodePermissionDenied}); s != StatusDenied {
		t.Errorf("expected StatusDenied, got %q", s)
	}
	if s := StatusForExecutionError(&capability.ToolError{Code: capability.ErrorCodeTimeout}); s != StatusTimedOut {
		t.Errorf("expected StatusTimedOut, got %q", s)
	}
}

func TestExecutionHookTimeoutAndCancel(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	hook.OnTimeoutTriggered(ctx, "inv-tc-1")
	hook.OnCancelled(ctx, "inv-tc-1", "user_request")

	_ = writer.Flush(ctx)

	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{
		InvocationID: "inv-tc-1",
		ListOptions:  ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListRuntimeEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestExecutionHookCircuitEvents(t *testing.T) {
	store := NewMemoryStore()
	writer := NewRecordWriter(store, DefaultWriterConfig())
	defer writer.Close()
	hook := NewExecutionHook(writer, nil)
	ctx := context.Background()

	hook.OnCircuitOpen(ctx, "inv-co-1", "runtime-1")
	hook.OnCircuitClosed(ctx, "inv-co-1", "runtime-1")

	_ = writer.Flush(ctx)

	events, _, err := store.ListRuntimeEvents(ctx, EventFilter{
		InvocationID: "inv-co-1",
		ListOptions:  ListOptions{Limit: 10},
	})
	if err != nil {
		t.Fatalf("ListRuntimeEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 circuit events, got %d", len(events))
	}
}

func TestOperationTypeConstants(t *testing.T) {
	types := []OperationType{
		OpToolExecute, OpWorkflowExecute, OpWorkflowSchedule,
		OpPluginHook, OpPluginEvent, OpPluginSchedule,
		OpMCPConnect, OpMCPDisconnect, OpMCPDiscover, OpMCPToolExecute,
		OpExtensionInstall, OpExtensionEnable, OpExtensionDisable,
		OpExtensionUpdate, OpExtensionRollback, OpExtensionUninstall, OpExtensionRestore,
		OpAgentSkillImport, OpAgentSkillActivate, OpAgentSkillRemove,
		OpPermissionGrant, OpPermissionRevoke,
		OpScopeBind, OpScopeUnbind,
		OpMigrationExecute,
		OpRuntimeStart, OpRuntimeStop, OpRuntimeCrash,
	}
	for _, opType := range types {
		if opType == "" {
			t.Error("expected non-empty operation type")
		}
	}
}

func TestDataSensitivityConstants(t *testing.T) {
	sensitivities := []DataSensitivity{
		SensitivityPublic, SensitivityInternal, SensitivitySensitive,
		SensitivityRestricted, SensitivitySecret,
	}
	for _, s := range sensitivities {
		if s == "" {
			t.Error("expected non-empty sensitivity")
		}
	}
}
