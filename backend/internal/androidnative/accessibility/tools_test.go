package accessibility

import (
	"encoding/json"
	"testing"
)

func TestBuildPermissionDefinitions(t *testing.T) {
	defs := BuildPermissionDefinitions()
	if len(defs) != 3 {
		t.Fatalf("expected 3 permission definitions, got %d", len(defs))
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
	if defs[2].ID != "android.accessibility.manage" {
		t.Fatalf("expected third permission ID android.accessibility.manage, got %s", defs[2].ID)
	}
}

func TestBuildAccessibilityTools(t *testing.T) {
	tools := BuildAccessibilityTools()
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatalf("tool %s has invalid input schema: %v", tool.ID, err)
		}
	}

	statusTool := tools[0]
	if statusTool.ID != "android.accessibility.status" {
		t.Fatalf("expected first tool ID android.accessibility.status, got %s", statusTool.ID)
	}
	if statusTool.ModelName != "android_accessibility_status" {
		t.Fatalf("expected safe model name android_accessibility_status, got %s", statusTool.ModelName)
	}
	if statusTool.Metadata["canonicalModelName"] != "android.accessibility.status" {
		t.Fatalf("expected canonical model name android.accessibility.status, got %v", statusTool.Metadata["canonicalModelName"])
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
	var outputSchema map[string]any
	if err := json.Unmarshal(statusTool.OutputSchema, &outputSchema); err != nil {
		t.Fatalf("invalid output schema: %v", err)
	}
	properties, ok := outputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("output schema properties missing")
	}
	for _, field := range []string{"includeNotImportantViews", "enhancedWebAccessibility", "canPerformGestures", "canTakeScreenshot", "interactionReady", "visualReady", "ready"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("output schema missing %s", field)
		}
	}

	openSettingsTool := tools[1]
	if openSettingsTool.ID != "android.accessibility.open_settings" {
		t.Fatalf("expected second tool ID android.accessibility.open_settings, got %s", openSettingsTool.ID)
	}
	if openSettingsTool.ModelName != "android_accessibility_open_settings" {
		t.Fatalf("expected safe model name android_accessibility_open_settings, got %s", openSettingsTool.ModelName)
	}
	if openSettingsTool.Metadata["canonicalModelName"] != "android.accessibility.open_settings" {
		t.Fatalf("expected canonical model name android.accessibility.open_settings, got %v", openSettingsTool.Metadata["canonicalModelName"])
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

	providerStatusTool := tools[2]
	if providerStatusTool.ModelName != "android_accessibility_provider_status" {
		t.Fatalf("expected safe provider status model name, got %s", providerStatusTool.ModelName)
	}
	if providerStatusTool.Metadata["canonicalModelName"] != "android.accessibility.provider.status" {
		t.Fatalf("expected provider status canonical name, got %v", providerStatusTool.Metadata["canonicalModelName"])
	}

	providerInstallTool := tools[3]
	if providerInstallTool.ModelName != "android_accessibility_provider_install" {
		t.Fatalf("expected safe provider install model name, got %s", providerInstallTool.ModelName)
	}
	if !providerInstallTool.ExecutionPolicy.ApprovalRequired {
		t.Fatal("expected provider install to require approval")
	}

	providerSettingsTool := tools[4]
	if providerSettingsTool.ModelName != "android_accessibility_provider_open_settings" {
		t.Fatalf("expected safe provider settings model name, got %s", providerSettingsTool.ModelName)
	}
}

func TestConstants(t *testing.T) {
	if OperationStatus != "accessibility.status" {
		t.Fatalf("expected OperationStatus=accessibility.status, got %s", OperationStatus)
	}
	if OperationOpenSettings != "accessibility.open_settings" {
		t.Fatalf("expected OperationOpenSettings=accessibility.open_settings, got %s", OperationOpenSettings)
	}
	if OperationProviderStatus != "accessibility.provider.status" {
		t.Fatalf("expected OperationProviderStatus=accessibility.provider.status, got %s", OperationProviderStatus)
	}
	if OperationProviderInstall != "accessibility.provider.install" {
		t.Fatalf("expected OperationProviderInstall=accessibility.provider.install, got %s", OperationProviderInstall)
	}
	if OperationProviderOpenSettings != "accessibility.provider.open_settings" {
		t.Fatalf("expected OperationProviderOpenSettings=accessibility.provider.open_settings, got %s", OperationProviderOpenSettings)
	}
}
