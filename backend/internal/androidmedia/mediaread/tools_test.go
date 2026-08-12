package mediaread

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildInfoToolDefinition(t *testing.T) {
	def, err := BuildInfoToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != ToolIDInfo {
		t.Fatalf("expected ID %s, got %s", ToolIDInfo, def.ID)
	}
	if def.ModelName != ToolIDInfo {
		t.Fatalf("expected ModelName %s, got %s", ToolIDInfo, def.ModelName)
	}
	if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("expected Android_Native runtime, got %s", def.Runtime.RuntimeType)
	}
	if def.Runtime.HandlerName != HandlerInfo {
		t.Fatalf("expected handler %s, got %s", HandlerInfo, def.Runtime.HandlerName)
	}
	if def.RiskLevel != capability.RiskLow {
		t.Fatalf("expected low risk, got %s", def.RiskLevel)
	}
	if def.HasSideEffects {
		t.Fatal("info should have no side effects")
	}
	if !def.Idempotent {
		t.Fatal("info should be idempotent")
	}
	if def.ExecutionPolicy.ApprovalRequired {
		t.Fatal("info should not require approval")
	}
	if def.Metadata["operation"] != OperationInfo {
		t.Fatalf("expected metadata operation %s, got %v", OperationInfo, def.Metadata["operation"])
	}
	if def.Metadata["bridgeProtocol"] != "mediaread" {
		t.Fatalf("expected bridgeProtocol mediaread, got %v", def.Metadata["bridgeProtocol"])
	}
}

func TestBuildImageToolDefinition(t *testing.T) {
	def, err := BuildImageToolDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != ToolIDImage {
		t.Fatalf("expected ID %s, got %s", ToolIDImage, def.ID)
	}
	if def.ModelName != ToolIDImage {
		t.Fatalf("expected ModelName %s, got %s", ToolIDImage, def.ModelName)
	}
	if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("expected Android_Native runtime, got %s", def.Runtime.RuntimeType)
	}
	if def.Runtime.HandlerName != HandlerImage {
		t.Fatalf("expected handler %s, got %s", HandlerImage, def.Runtime.HandlerName)
	}
	if def.RiskLevel != capability.RiskLow {
		t.Fatalf("expected low risk, got %s", def.RiskLevel)
	}
	if !def.HasSideEffects {
		t.Fatal("image should have side effects")
	}
	if !def.Idempotent {
		t.Fatal("image should be idempotent")
	}
}

func TestBuildPermissionDefinition(t *testing.T) {
	pd := BuildPermissionDefinition()

	if pd.ID != permissionRead {
		t.Fatalf("expected ID %s, got %s", permissionRead, pd.ID)
	}
	if pd.Risk != "low" {
		t.Fatalf("expected low risk, got %s", pd.Risk)
	}
}

func TestToolDefinitions_UniqueIDs(t *testing.T) {
	info, _ := BuildInfoToolDefinition()
	image, _ := BuildImageToolDefinition()

	ids := map[string]bool{}
	for _, def := range []capability.ToolDefinition{info, image} {
		if ids[def.ID] {
			t.Fatalf("duplicate tool ID: %s", def.ID)
		}
		ids[def.ID] = true
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 unique IDs, got %d", len(ids))
	}
}

func TestToolDefinitions_AndroidNativeRuntime(t *testing.T) {
	info, _ := BuildInfoToolDefinition()
	image, _ := BuildImageToolDefinition()

	for _, def := range []capability.ToolDefinition{info, image} {
		if def.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Fatalf("tool %s: expected Android_Native runtime, got %s", def.ID, def.Runtime.RuntimeType)
		}
	}
}

func TestToolDefinitions_NotOCR(t *testing.T) {
	info, _ := BuildInfoToolDefinition()
	image, _ := BuildImageToolDefinition()

	for _, def := range []capability.ToolDefinition{info, image} {
		if def.ID == "android.media.read.ocr" || def.ID == "android.media.read.understand" {
			t.Fatalf("tool %s should not exist - 方案 A only exposes info and image", def.ID)
		}
	}
}

func TestToolDefinitions_InputOutputSchema(t *testing.T) {
	info, _ := BuildInfoToolDefinition()
	if len(info.InputSchema) == 0 {
		t.Fatal("expected input schema for info tool")
	}
	if len(info.OutputSchema) == 0 {
		t.Fatal("expected output schema for info tool")
	}

	image, _ := BuildImageToolDefinition()
	if len(image.InputSchema) == 0 {
		t.Fatal("expected input schema for image tool")
	}
	if len(image.OutputSchema) == 0 {
		t.Fatal("expected output schema for image tool")
	}
}

func TestImageTool_Timeout(t *testing.T) {
	def, _ := BuildImageToolDefinition()
	if def.TimeoutMS != 15000 {
		t.Fatalf("expected 15000ms timeout, got %d", def.TimeoutMS)
	}
}
