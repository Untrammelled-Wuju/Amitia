package root

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildRootTools(t *testing.T) {
	tools := BuildRootTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	expectedIDs := []string{
		"android.root.status",
		"android.root.request",
		"android.root.execute",
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

func TestBuildRootTools_RuntimeBinding(t *testing.T) {
	tools := BuildRootTools()

	for _, tool := range tools {
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Fatalf("expected RuntimeTypeAndroid_Native, got %s", tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != "android_native_root" {
			t.Fatalf("expected android_native_root, got %s", tool.Runtime.RuntimeID)
		}
	}
}

func TestBuildRootTools_StatusTool(t *testing.T) {
	tools := BuildRootTools()
	statusTool := tools[0]

	if statusTool.ID != "android.root.status" {
		t.Fatalf("expected android.root.status, got %s", statusTool.ID)
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
	if len(statusTool.Permissions) != 1 || statusTool.Permissions[0].Capability != PermissionRootInspect {
		t.Fatalf("expected PermissionRootInspect, got %+v", statusTool.Permissions)
	}
}

func TestBuildRootTools_RequestTool(t *testing.T) {
	tools := BuildRootTools()
	requestTool := tools[1]

	if requestTool.ID != "android.root.request" {
		t.Fatalf("expected android.root.request, got %s", requestTool.ID)
	}
	if requestTool.RiskLevel != capability.RiskHigh {
		t.Fatalf("expected RiskHigh, got %s", requestTool.RiskLevel)
	}
	if !requestTool.HasSideEffects {
		t.Fatal("request tool should have side effects")
	}
	if requestTool.Idempotent {
		t.Fatal("request tool should not be idempotent")
	}
	if requestTool.Retryable {
		t.Fatal("request tool should not be retryable")
	}
	if !requestTool.ExecutionPolicy.ApprovalRequired {
		t.Fatal("request tool should require approval")
	}
	if requestTool.ExecutionPolicy.AllowBackground {
		t.Fatal("request tool should not allow background")
	}
	if len(requestTool.Permissions) != 1 || requestTool.Permissions[0].Capability != PermissionRootRequest {
		t.Fatalf("expected PermissionRootRequest, got %+v", requestTool.Permissions)
	}
}

func TestBuildRootTools_ExecuteTool(t *testing.T) {
	tools := BuildRootTools()
	executeTool := tools[2]

	if executeTool.ID != "android.root.execute" {
		t.Fatalf("expected android.root.execute, got %s", executeTool.ID)
	}
	if executeTool.RiskLevel != capability.RiskHigh {
		t.Fatalf("expected RiskHigh, got %s", executeTool.RiskLevel)
	}
	if !executeTool.HasSideEffects {
		t.Fatal("execute tool should have side effects")
	}
	if executeTool.Idempotent {
		t.Fatal("execute tool should not be idempotent")
	}
	if executeTool.Retryable {
		t.Fatal("execute tool should not be retryable")
	}
	if !executeTool.ExecutionPolicy.ApprovalRequired {
		t.Fatal("execute tool should require approval")
	}
	if executeTool.ExecutionPolicy.AllowBackground {
		t.Fatal("execute tool should not allow background")
	}
	if executeTool.ExecutionPolicy.MaxConcurrency != 1 {
		t.Fatalf("expected MaxConcurrency 1, got %d", executeTool.ExecutionPolicy.MaxConcurrency)
	}
	if len(executeTool.Permissions) != 1 || executeTool.Permissions[0].Capability != PermissionRootExecute {
		t.Fatalf("expected PermissionRootExecute, got %+v", executeTool.Permissions)
	}
}

func TestBuildRootTools_UniqueIDs(t *testing.T) {
	tools := BuildRootTools()
	ids := make(map[string]bool)
	for _, tool := range tools {
		if ids[tool.ID] {
			t.Fatalf("duplicate tool ID: %s", tool.ID)
		}
		ids[tool.ID] = true
	}
}

func TestBuildRootTools_HandlerNames(t *testing.T) {
	tools := BuildRootTools()
	expectedHandlers := map[string]string{
		"android.root.status":  OperationStatus,
		"android.root.request": OperationRequest,
		"android.root.execute": OperationExecute,
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

func TestBuildRootPermissionDefinitions(t *testing.T) {
	perms := BuildRootPermissionDefinitions()
	if len(perms) != 4 {
		t.Fatalf("expected 4 permissions, got %d", len(perms))
	}

	expectedIDs := []string{
		PermissionRootInspect,
		PermissionRootRequest,
		PermissionRootExecute,
		PermissionRootShell,
	}

	for i, perm := range perms {
		if perm.ID != expectedIDs[i] {
			t.Fatalf("expected permission ID %s, got %s", expectedIDs[i], perm.ID)
		}
	}
}

func TestBuildRootTools_ResultPolicy(t *testing.T) {
	tools := BuildRootTools()

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
