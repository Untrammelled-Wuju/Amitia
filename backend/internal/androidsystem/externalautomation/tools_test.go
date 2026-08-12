package externalautomation

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildExternalAutomationTools(t *testing.T) {
	tools := BuildExternalAutomationTools()
	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}
}

func TestBuildExternalAutomationToolsAllEnabled(t *testing.T) {
	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		if !tool.Enabled {
			t.Errorf("tool %s is not enabled", tool.ID)
		}
	}
}

func TestBuildExternalAutomationToolsRuntimeBinding(t *testing.T) {
	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Errorf("tool %s has wrong runtime type: %s", tool.ID, tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != RuntimeIDExternalAutomation {
			t.Errorf("tool %s has wrong runtime ID: %s", tool.ID, tool.Runtime.RuntimeID)
		}
		if tool.Runtime.HandlerName == "" {
			t.Errorf("tool %s has empty handler name", tool.ID)
		}
	}
}

func TestBuildExternalAutomationToolsIDs(t *testing.T) {
	expectedIDs := []string{
		ToolIDStatus,
		ToolIDResolveApp,
		ToolIDOpenApp,
		ToolIDResolveURI,
		ToolIDOpenURI,
		ToolIDOpenSettings,
		ToolIDInvokeIntent,
		ToolIDForeground,
		ToolIDWaitForeground,
	}

	tools := BuildExternalAutomationTools()
	for i, tool := range tools {
		if tool.ID != expectedIDs[i] {
			t.Errorf("expected tool ID %s, got %s", expectedIDs[i], tool.ID)
		}
	}
}

func TestBuildExternalAutomationToolsOperations(t *testing.T) {
	expectedOps := map[string]string{
		ToolIDStatus:        OperationStatus,
		ToolIDResolveApp:    OperationResolveApp,
		ToolIDOpenApp:       OperationOpenApp,
		ToolIDResolveURI:    OperationResolveURI,
		ToolIDOpenURI:       OperationOpenURI,
		ToolIDOpenSettings:  OperationOpenSettings,
		ToolIDInvokeIntent:  OperationInvokeIntent,
		ToolIDForeground:    OperationForeground,
		ToolIDWaitForeground: OperationWaitForeground,
	}

	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		expected := expectedOps[tool.ID]
		if tool.Runtime.HandlerName != expected {
			t.Errorf("tool %s has wrong handler name: expected %s, got %s", tool.ID, expected, tool.Runtime.HandlerName)
		}
	}
}

func TestStatusToolReadOnly(t *testing.T) {
	tools := BuildExternalAutomationTools()
	status := tools[0]

	if status.ID != ToolIDStatus {
		t.Errorf("first tool should be status, got %s", status.ID)
	}
	if status.HasSideEffects != false {
		t.Error("status tool should not have side effects")
	}
	if status.SideEffect != capability.SideEffectReadOnly {
		t.Error("status tool should be read-only")
	}
	if !status.Idempotent {
		t.Error("status tool should be idempotent")
	}
	if status.ExecutionPolicy.ApprovalRequired {
		t.Error("status tool should not require approval")
	}
}

func TestOpenAppToolSideEffects(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var openApp capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDOpenApp {
			openApp = tool
			break
		}
	}

	if openApp.ID == "" {
		t.Fatal("open_app tool not found")
	}
	if !openApp.HasSideEffects {
		t.Error("open_app tool should have side effects")
	}
	if openApp.ExecutionPolicy.ApprovalRequired {
		t.Error("open_app tool should not require approval by default")
	}
	if openApp.Idempotent {
		t.Error("open_app tool should not be idempotent")
	}
	if openApp.RiskLevel != capability.RiskMedium {
		t.Error("open_app tool should be medium risk")
	}
	if openApp.Retryable {
		t.Error("open_app tool should not be retryable")
	}
}

func TestInvokeIntentToolHighRisk(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var intentTool capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDInvokeIntent {
			intentTool = tool
			break
		}
	}

	if intentTool.ID == "" {
		t.Fatal("invoke_intent tool not found")
	}
	if !intentTool.HasSideEffects {
		t.Error("invoke_intent tool should have side effects")
	}
	if !intentTool.ExecutionPolicy.ApprovalRequired {
		t.Error("invoke_intent tool should require approval")
	}
	if intentTool.Idempotent {
		t.Error("invoke_intent tool should not be idempotent")
	}
	if intentTool.RiskLevel != capability.RiskHigh {
		t.Error("invoke_intent tool should be high risk")
	}
	if intentTool.Retryable {
		t.Error("invoke_intent tool should not be retryable")
	}
}

func TestForegroundToolReadOnly(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var foreground capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDForeground {
			foreground = tool
			break
		}
	}

	if foreground.ID == "" {
		t.Fatal("foreground tool not found")
	}
	if foreground.HasSideEffects {
		t.Error("foreground tool should not have side effects")
	}
	if foreground.SideEffect != capability.SideEffectReadOnly {
		t.Error("foreground tool should be read-only")
	}
	if !foreground.Idempotent {
		t.Error("foreground tool should be idempotent")
	}
}

func TestWaitForegroundToolNotIdempotent(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var waitTool capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDWaitForeground {
			waitTool = tool
			break
		}
	}

	if waitTool.ID == "" {
		t.Fatal("wait_foreground tool not found")
	}
	if waitTool.Idempotent {
		t.Error("wait_foreground tool should not be idempotent")
	}
	if waitTool.ExecutionPolicy.ApprovalRequired {
		t.Error("wait_foreground tool should not require approval")
	}
}

func TestResolveAppTool(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var resolveApp capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDResolveApp {
			resolveApp = tool
			break
		}
	}

	if resolveApp.ID == "" {
		t.Fatal("resolve_app tool not found")
	}
	if resolveApp.HasSideEffects {
		t.Error("resolve_app tool should not have side effects")
	}
	if !resolveApp.Idempotent {
		t.Error("resolve_app tool should be idempotent")
	}
	if resolveApp.ExecutionPolicy.ApprovalRequired {
		t.Error("resolve_app tool should not require approval")
	}
}

func TestResolveURITool(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var resolveURI capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDResolveURI {
			resolveURI = tool
			break
		}
	}

	if resolveURI.ID == "" {
		t.Fatal("resolve_uri tool not found")
	}
	if resolveURI.HasSideEffects {
		t.Error("resolve_uri tool should not have side effects")
	}
	if !resolveURI.Idempotent {
		t.Error("resolve_uri tool should be idempotent")
	}
}

func TestOpenURITool(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var openURI capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDOpenURI {
			openURI = tool
			break
		}
	}

	if openURI.ID == "" {
		t.Fatal("open_uri tool not found")
	}
	if !openURI.HasSideEffects {
		t.Error("open_uri tool should have side effects")
	}
	if openURI.Idempotent {
		t.Error("open_uri tool should not be idempotent")
	}
	if openURI.RiskLevel != capability.RiskMedium {
		t.Error("open_uri tool should be medium risk")
	}
}

func TestOpenSettingsTool(t *testing.T) {
	tools := BuildExternalAutomationTools()
	var openSettings capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDOpenSettings {
			openSettings = tool
			break
		}
	}

	if openSettings.ID == "" {
		t.Fatal("open_settings tool not found")
	}
	if !openSettings.HasSideEffects {
		t.Error("open_settings tool should have side effects")
	}
	if openSettings.Idempotent {
		t.Error("open_settings tool should not be idempotent")
	}
	if openSettings.RiskLevel != capability.RiskMedium {
		t.Error("open_settings tool should be medium risk")
	}
}

func TestMapErrorPermissionDenied(t *testing.T) {
	err := MapError(AUTOMATION_PERMISSION_DENIED, "permission denied")
	if err.Code != capability.ErrorCodePermissionDenied {
		t.Errorf("expected PermissionDenied, got %s", err.Code)
	}
	if err.Retryable {
		t.Error("permission error should not be retryable")
	}
}

func TestMapErrorNotAvailable(t *testing.T) {
	err := MapError(AUTOMATION_UNSUPPORTED, "not supported")
	if err.Code != capability.ErrorCodeNotAvailable {
		t.Errorf("expected NotAvailable, got %s", err.Code)
	}
}

func TestMapErrorInvalidInput(t *testing.T) {
	err := MapError(AUTOMATION_INVALID_REQUEST, "invalid request")
	if err.Code != capability.ErrorCodeInvalidInput {
		t.Errorf("expected InvalidInput, got %s", err.Code)
	}
}

func TestMapErrorTimeout(t *testing.T) {
	err := MapError(AUTOMATION_TIMEOUT, "timeout")
	if err.Code != capability.ErrorCodeTimeout {
		t.Errorf("expected Timeout, got %s", err.Code)
	}
	if !err.Retryable {
		t.Error("timeout error should be retryable")
	}
}

func TestMapErrorUnknownCode(t *testing.T) {
	err := MapError("UNKNOWN_CODE", "unknown error")
	if err.Code != capability.ErrorCodeExecutionFailed {
		t.Errorf("expected ExecutionFailed, got %s", err.Code)
	}
}

func TestBuildExternalAutomationToolsUniqueIDs(t *testing.T) {
	tools := BuildExternalAutomationTools()
	ids := make(map[string]bool)
	for _, tool := range tools {
		if ids[tool.ID] {
			t.Errorf("duplicate tool ID: %s", tool.ID)
		}
		ids[tool.ID] = true
	}
}

func TestBuildExternalAutomationToolsUniqueModelNames(t *testing.T) {
	tools := BuildExternalAutomationTools()
	names := make(map[string]bool)
	for _, tool := range tools {
		if names[tool.ModelName] {
			t.Errorf("duplicate model name: %s", tool.ModelName)
		}
		names[tool.ModelName] = true
	}
}

func TestBuildExternalAutomationToolsPermissionRequirements(t *testing.T) {
	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		if len(tool.Permissions) == 0 {
			t.Errorf("tool %s has no permission requirements", tool.ID)
		}
		for _, perm := range tool.Permissions {
			if perm.Capability == "" {
				t.Errorf("tool %s has empty permission capability", tool.ID)
			}
		}
	}
}

func TestBuildExternalAutomationToolsInputSchema(t *testing.T) {
	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		if len(tool.InputSchema) == 0 {
			t.Errorf("tool %s has no input schema", tool.ID)
		}
	}
}

func TestBuildExternalAutomationToolsOutputSchema(t *testing.T) {
	tools := BuildExternalAutomationTools()
	for _, tool := range tools {
		if len(tool.OutputSchema) == 0 {
			t.Errorf("tool %s has no output schema", tool.ID)
		}
	}
}

func TestAutomationErrorImplementsError(t *testing.T) {
	var err error = newAutomationError("TEST_ERROR_CODE", "test")
	if err == nil {
		t.Fatal("automationError should implement error interface")
	}
	if err.Error() != "test" {
		t.Errorf("expected 'test', got '%s'", err.Error())
	}
}
