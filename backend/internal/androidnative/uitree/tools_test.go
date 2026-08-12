package uitree

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildUITreeTools(t *testing.T) {
	tools := BuildUITreeTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	expectedIDs := []string{
		"android.ui_tree.status",
		"android.ui_tree.snapshot",
		"android.ui_tree.find",
		"android.ui_tree.get",
	}

	for i, tool := range tools {
		if tool.ID != expectedIDs[i] {
			t.Fatalf("expected tool ID %s, got %s", expectedIDs[i], tool.ID)
		}
		if !tool.Enabled {
			t.Fatalf("tool %s should be enabled", tool.ID)
		}
	}
}

func TestBuildUITreeTools_RuntimeBinding(t *testing.T) {
	tools := BuildUITreeTools()

	for _, tool := range tools {
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Fatalf("expected RuntimeTypeAndroid_Native, got %s", tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != "android_native_ui_tree" {
			t.Fatalf("expected android_native_ui_tree, got %s", tool.Runtime.RuntimeID)
		}
	}
}

func TestBuildUITreeTools_StatusTool(t *testing.T) {
	tools := BuildUITreeTools()
	statusTool := tools[0]

	if statusTool.ID != "android.ui_tree.status" {
		t.Fatalf("expected android.ui_tree.status, got %s", statusTool.ID)
	}
	if statusTool.RiskLevel != capability.RiskLow {
		t.Fatalf("expected RiskLow, got %s", statusTool.RiskLevel)
	}
	if statusTool.HasSideEffects {
		t.Fatal("status tool should not have side effects")
	}
	if !statusTool.Idempotent {
		t.Fatal("status tool should be idempotent")
	}
	if !statusTool.Retryable {
		t.Fatal("status tool should be retryable")
	}
	if statusTool.ExecutionPolicy.ApprovalRequired {
		t.Fatal("status tool should not require approval")
	}
	if !statusTool.ExecutionPolicy.AllowBackground {
		t.Fatal("status tool should allow background")
	}
}

func TestBuildUITreeTools_SnapshotTool(t *testing.T) {
	tools := BuildUITreeTools()
	snapshotTool := tools[1]

	if snapshotTool.ID != "android.ui_tree.snapshot" {
		t.Fatalf("expected android.ui_tree.snapshot, got %s", snapshotTool.ID)
	}
	if snapshotTool.RiskLevel != capability.RiskMedium {
		t.Fatalf("expected RiskMedium, got %s", snapshotTool.RiskLevel)
	}
	if snapshotTool.HasSideEffects {
		t.Fatal("snapshot tool should not have side effects")
	}
	if snapshotTool.Idempotent {
		t.Fatal("snapshot tool should not be idempotent")
	}
	if !snapshotTool.Retryable {
		t.Fatal("snapshot tool should be retryable")
	}
	if snapshotTool.ExecutionPolicy.MaxConcurrency != 2 {
		t.Fatalf("expected MaxConcurrency 2, got %d", snapshotTool.ExecutionPolicy.MaxConcurrency)
	}
}

func TestBuildUITreeTools_FindTool(t *testing.T) {
	tools := BuildUITreeTools()
	findTool := tools[2]

	if findTool.ID != "android.ui_tree.find" {
		t.Fatalf("expected android.ui_tree.find, got %s", findTool.ID)
	}
	if findTool.RiskLevel != capability.RiskLow {
		t.Fatalf("expected RiskLow, got %s", findTool.RiskLevel)
	}
	if findTool.HasSideEffects {
		t.Fatal("find tool should not have side effects")
	}
	if !findTool.Idempotent {
		t.Fatal("find tool should be idempotent")
	}
	if !findTool.Retryable {
		t.Fatal("find tool should be retryable")
	}
}

func TestBuildUITreeTools_GetTool(t *testing.T) {
	tools := BuildUITreeTools()
	getTool := tools[3]

	if getTool.ID != "android.ui_tree.get" {
		t.Fatalf("expected android.ui_tree.get, got %s", getTool.ID)
	}
	if getTool.RiskLevel != capability.RiskLow {
		t.Fatalf("expected RiskLow, got %s", getTool.RiskLevel)
	}
	if getTool.HasSideEffects {
		t.Fatal("get tool should not have side effects")
	}
	if !getTool.Idempotent {
		t.Fatal("get tool should be idempotent")
	}
	if !getTool.Retryable {
		t.Fatal("get tool should be retryable")
	}
}

func TestBuildUITreeTools_UniqueIDs(t *testing.T) {
	tools := BuildUITreeTools()
	ids := make(map[string]bool)
	for _, tool := range tools {
		if ids[tool.ID] {
			t.Fatalf("duplicate tool ID: %s", tool.ID)
		}
		ids[tool.ID] = true
	}
}

func TestBuildUITreeTools_HandlerNames(t *testing.T) {
	tools := BuildUITreeTools()
	expectedHandlers := map[string]string{
		"android.ui_tree.status":   OperationStatus,
		"android.ui_tree.snapshot": OperationSnapshot,
		"android.ui_tree.find":     OperationFind,
		"android.ui_tree.get":      OperationGet,
	}

	for _, tool := range tools {
		expected, ok := expectedHandlers[tool.ID]
		if !ok {
			t.Fatalf("unexpected tool ID: %s", tool.ID)
		}
		if tool.Runtime.HandlerName != expected {
			t.Fatalf("expected handler %s for %s, got %s", expected, tool.ID, tool.Runtime.HandlerName)
		}
	}
}

func TestBuildUITreeTools_ResultPolicy(t *testing.T) {
	tools := BuildUITreeTools()

	for _, tool := range tools {
		if !tool.ResultPolicy.SanitizeError {
			t.Fatalf("tool %s should sanitize errors", tool.ID)
		}
		if tool.ResultPolicy.MaxOutputBytes <= 0 {
			t.Fatalf("tool %s should have positive MaxOutputBytes", tool.ID)
		}
		if tool.ResultPolicy.Streaming.Enabled {
			t.Fatalf("tool %s should not have streaming enabled", tool.ID)
		}
	}
}

func TestMapUITreeErrorToCanonical(t *testing.T) {
	tests := []struct {
		domainCode string
		expected   string
	}{
		{UI_TREE_UNSUPPORTED, "PLATFORM_NOT_SUPPORTED"},
		{UI_TREE_UNAVAILABLE, "PROVIDER_UNAVAILABLE"},
		{UI_TREE_ACCESSIBILITY_DISABLED, "PROVIDER_UNAVAILABLE"},
		{UI_TREE_SNAPSHOT_FAILED, "PROVIDER_UNAVAILABLE"},
		{UI_TREE_INVALID_REQUEST, "AUTHORIZATION_DENIED"},
		{UI_NODE_STALE, "BRIDGE_INVALID_RESPONSE"},
		{UI_TREE_TIMEOUT, "BRIDGE_TIMEOUT"},
		{UI_TREE_CANCELLED, "CANCELLED"},
		{"UNKNOWN_CODE", "PROVIDER_UNAVAILABLE"},
	}

	for _, tt := range tests {
		result := MapUITreeErrorToCanonical(tt.domainCode)
		if result != tt.expected {
			t.Fatalf("MapUITreeErrorToCanonical(%q) = %q, expected %q", tt.domainCode, result, tt.expected)
		}
	}
}
