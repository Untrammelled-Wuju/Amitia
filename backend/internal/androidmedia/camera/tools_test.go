package camera

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildStatusToolDefinition(t *testing.T) {
	def, err := BuildStatusToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != toolIDStatus {
		t.Fatalf("expected ID %s, got %s", toolIDStatus, def.ID)
	}
	if def.ModelName != toolIDStatus {
		t.Fatalf("expected ModelName %s, got %s", toolIDStatus, def.ModelName)
	}
	if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("expected Android_Native runtime, got %s", def.Runtime.RuntimeType)
	}
	if def.Runtime.HandlerName != handlerStatus {
		t.Fatalf("expected handler %s, got %s", handlerStatus, def.Runtime.HandlerName)
	}
	if def.RiskLevel != capability.RiskLow {
		t.Fatalf("expected low risk, got %s", def.RiskLevel)
	}
	if def.HasSideEffects {
		t.Fatal("status should have no side effects")
	}
	if !def.Idempotent {
		t.Fatal("status should be idempotent")
	}
	if def.ExecutionPolicy.ApprovalRequired {
		t.Fatal("status should not require approval")
	}
	if def.Metadata["androidNativeOperation"] != OperationCameraStatus {
		t.Fatalf("expected metadata operation %s, got %v", OperationCameraStatus, def.Metadata["androidNativeOperation"])
	}
	if def.Metadata["bridgeProtocol"] != "android_native" {
		t.Fatalf("expected bridgeProtocol android_native, got %v", def.Metadata["bridgeProtocol"])
	}
}

func TestBuildListToolDefinition(t *testing.T) {
	def, err := BuildListToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != toolIDList {
		t.Fatalf("expected ID %s, got %s", toolIDList, def.ID)
	}
	if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("expected Android_Native runtime, got %s", def.Runtime.RuntimeType)
	}
	if def.Runtime.HandlerName != handlerList {
		t.Fatalf("expected handler %s, got %s", handlerList, def.Runtime.HandlerName)
	}
	if def.RiskLevel != capability.RiskLow {
		t.Fatalf("expected low risk, got %s", def.RiskLevel)
	}
	if def.HasSideEffects {
		t.Fatal("list should have no side effects")
	}
}

func TestBuildCaptureToolDefinition(t *testing.T) {
	def, err := BuildCaptureToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != toolIDCapture {
		t.Fatalf("expected ID %s, got %s", toolIDCapture, def.ID)
	}
	if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("expected Android_Native runtime, got %s", def.Runtime.RuntimeType)
	}
	if def.Runtime.HandlerName != handlerCapture {
		t.Fatalf("expected handler %s, got %s", handlerCapture, def.Runtime.HandlerName)
	}
	if def.RiskLevel != capability.RiskHigh {
		t.Fatalf("expected high risk, got %s", def.RiskLevel)
	}
	if !def.HasSideEffects {
		t.Fatal("capture should have side effects")
	}
	if def.Idempotent {
		t.Fatal("capture should not be idempotent")
	}
	if def.Retryable {
		t.Fatal("capture should not be retryable")
	}
	if !def.ExecutionPolicy.ApprovalRequired {
		t.Fatal("capture should require approval")
	}
	if def.ExecutionPolicy.AllowBackground {
		t.Fatal("capture should not allow background")
	}
	if def.ExecutionPolicy.MaxConcurrency != 1 {
		t.Fatalf("expected max concurrency 1, got %d", def.ExecutionPolicy.MaxConcurrency)
	}
	if def.Metadata["androidNativeOperation"] != OperationCameraCapture {
		t.Fatalf("expected metadata operation %s, got %v", OperationCameraCapture, def.Metadata["androidNativeOperation"])
	}
}

func TestBuildPermissionDefinition(t *testing.T) {
	pd := BuildPermissionDefinition()

	if pd.ID != permissionID {
		t.Fatalf("expected ID %s, got %s", permissionID, pd.ID)
	}
	if pd.Risk != "high" {
		t.Fatalf("expected high risk, got %s", pd.Risk)
	}
}

func TestToolDefinitions_UniqueIDs(t *testing.T) {
	status, _ := BuildStatusToolDefinition()
	list, _ := BuildListToolDefinition()
	capture, _ := BuildCaptureToolDefinition()

	ids := map[string]bool{}
	for _, def := range []capability.ToolDefinition{status, list, capture} {
		if ids[def.ID] {
			t.Fatalf("duplicate tool ID: %s", def.ID)
		}
		ids[def.ID] = true
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 unique IDs, got %d", len(ids))
	}
}

func TestToolDefinitions_AndroidNativeRuntime(t *testing.T) {
	status, _ := BuildStatusToolDefinition()
	list, _ := BuildListToolDefinition()
	capture, _ := BuildCaptureToolDefinition()

	for _, def := range []capability.ToolDefinition{status, list, capture} {
		if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Fatalf("tool %s: expected Android_Native runtime, got %s", def.ID, def.Runtime.RuntimeType)
		}
	}
}

func TestToolDefinitions_NoNewRuntimeType(t *testing.T) {
	status, _ := BuildStatusToolDefinition()
	list, _ := BuildListToolDefinition()
	capture, _ := BuildCaptureToolDefinition()

	for _, def := range []capability.ToolDefinition{status, list, capture} {
		if def.Runtime.RuntimeType == "camera" || def.Runtime.RuntimeType == "android_camera" {
			t.Fatalf("tool %s: should not have camera-specific runtime type", def.ID)
		}
	}
}

func TestToolDefinitions_BridgeProtocol(t *testing.T) {
	status, _ := BuildStatusToolDefinition()
	list, _ := BuildListToolDefinition()
	capture, _ := BuildCaptureToolDefinition()

	for _, def := range []capability.ToolDefinition{status, list, capture} {
		if def.Metadata["bridgeProtocol"] != "android_native" {
			t.Fatalf("tool %s: expected bridgeProtocol android_native, got %v", def.ID, def.Metadata["bridgeProtocol"])
		}
	}
}

func TestCaptureTool_InputSchema_Required(t *testing.T) {
	def, err := BuildCaptureToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(def.InputSchema) == 0 {
		t.Fatal("expected input schema")
	}
	if len(def.OutputSchema) == 0 {
		t.Fatal("expected output schema")
	}
}

func TestCaptureTool_Timeout(t *testing.T) {
	def, _ := BuildCaptureToolDefinition()
	if def.TimeoutMS != 30000 {
		t.Fatalf("expected 30000ms timeout, got %d", def.TimeoutMS)
	}
	if def.ExecutionPolicy.Timeout.Seconds() != 30 {
		t.Fatalf("expected 30s execution timeout, got %v", def.ExecutionPolicy.Timeout)
	}
}
