package accessibility

import (
	"testing"
)

func TestBuildPermissionDefinitions(t *testing.T) {
	defs := BuildPermissionDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 permission definitions, got %d", len(defs))
	}

	if defs[0].ID != "android.accessibility.read_state" {
		t.Fatalf("expected first permission ID android.accessibility.read_state, got %s", defs[0].ID)
	}
	if defs[1].ID != "android.accessibility.open_settings" {
		t.Fatalf("expected second permission ID android.accessibility.open_settings, got %s", defs[1].ID)
	}
	if defs[0].Risk != "low" {
		t.Fatalf("expected first risk low, got %s", defs[0].Risk)
	}
	if defs[1].Risk != "medium" {
		t.Fatalf("expected second risk medium, got %s", defs[1].Risk)
	}
}

func TestBuildAccessibilityTools(t *testing.T) {
	tools := BuildAccessibilityTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	statusTool := tools[0]
	if statusTool.ID != "android.accessibility.status" {
		t.Fatalf("expected first tool ID android.accessibility.status, got %s", statusTool.ID)
	}
	if !statusTool.Idempotent {
		t.Fatalf("expected status tool to be idempotent")
	}
	if statusTool.HasSideEffects {
		t.Fatalf("expected status tool to have no side effects")
	}
	if !statusTool.Retryable {
		t.Fatalf("expected status tool to be retryable")
	}
	if statusTool.RiskLevel != "low" {
		t.Fatalf("expected status tool risk low, got %s", statusTool.RiskLevel)
	}
	if statusTool.ExecutionPolicy.ApprovalRequired {
		t.Fatalf("expected status tool to not require approval")
	}
	if statusTool.Runtime.RuntimeType != "android_native" {
		t.Fatalf("expected android_native runtime type, got %s", statusTool.Runtime.RuntimeType)
	}
	if statusTool.Runtime.HandlerName != "accessibility.status" {
		t.Fatalf("expected handler name accessibility.status, got %s", statusTool.Runtime.HandlerName)
	}

	openSettingsTool := tools[1]
	if openSettingsTool.ID != "android.accessibility.open_settings" {
		t.Fatalf("expected second tool ID android.accessibility.open_settings, got %s", openSettingsTool.ID)
	}
	if openSettingsTool.Idempotent {
		t.Fatalf("expected open_settings tool to not be idempotent")
	}
	if !openSettingsTool.HasSideEffects {
		t.Fatalf("expected open_settings tool to have side effects")
	}
	if openSettingsTool.Retryable {
		t.Fatalf("expected open_settings tool to not be retryable")
	}
	if openSettingsTool.RiskLevel != "medium" {
		t.Fatalf("expected open_settings tool risk medium, got %s", openSettingsTool.RiskLevel)
	}
	if !openSettingsTool.ExecutionPolicy.ApprovalRequired {
		t.Fatalf("expected open_settings tool to require approval")
	}
	if openSettingsTool.Runtime.HandlerName != "accessibility.open_settings" {
		t.Fatalf("expected handler name accessibility.open_settings, got %s", openSettingsTool.Runtime.HandlerName)
	}
}

func TestConstants(t *testing.T) {
	if OperationStatus != "accessibility.status" {
		t.Fatalf("expected OperationStatus=accessibility.status, got %s", OperationStatus)
	}
	if OperationOpenSettings != "accessibility.open_settings" {
		t.Fatalf("expected OperationOpenSettings=accessibility.open_settings, got %s", OperationOpenSettings)
	}
}
