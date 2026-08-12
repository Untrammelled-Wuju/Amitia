package overlay

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildOverlayTools(t *testing.T) {
	tools := BuildOverlayTools()
	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}
}

func TestBuildOverlayToolsAllEnabled(t *testing.T) {
	tools := BuildOverlayTools()
	for _, tool := range tools {
		if !tool.Enabled {
			t.Errorf("tool %s is not enabled", tool.ID)
		}
	}
}

func TestBuildOverlayToolsRuntimeBinding(t *testing.T) {
	tools := BuildOverlayTools()
	for _, tool := range tools {
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Errorf("tool %s has wrong runtime type: %s", tool.ID, tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != RuntimeIDOverlay {
			t.Errorf("tool %s has wrong runtime ID: %s", tool.ID, tool.Runtime.RuntimeID)
		}
		if tool.Runtime.HandlerName == "" {
			t.Errorf("tool %s has empty handler name", tool.ID)
		}
	}
}

func TestBuildOverlayToolsIDs(t *testing.T) {
	expectedIDs := []string{
		ToolIDStatus,
		ToolIDPermissionRequest,
		ToolIDCreate,
		ToolIDUpdate,
		ToolIDShow,
		ToolIDHide,
		ToolIDClose,
		ToolIDList,
		ToolIDCloseAll,
	}

	tools := BuildOverlayTools()
	for i, tool := range tools {
		if tool.ID != expectedIDs[i] {
			t.Errorf("expected tool ID %s, got %s", expectedIDs[i], tool.ID)
		}
	}
}

func TestBuildOverlayToolsOperations(t *testing.T) {
	expectedOps := map[string]string{
		ToolIDStatus:            OperationStatus,
		ToolIDPermissionRequest: OperationPermissionRequest,
		ToolIDCreate:            OperationCreate,
		ToolIDUpdate:            OperationUpdate,
		ToolIDShow:              OperationShow,
		ToolIDHide:              OperationHide,
		ToolIDClose:             OperationClose,
		ToolIDList:              OperationList,
		ToolIDCloseAll:          OperationCloseAll,
	}

	tools := BuildOverlayTools()
	for _, tool := range tools {
		expected := expectedOps[tool.ID]
		if tool.Runtime.HandlerName != expected {
			t.Errorf("tool %s has wrong handler name: expected %s, got %s", tool.ID, expected, tool.Runtime.HandlerName)
		}
	}
}

func TestStatusToolReadOnly(t *testing.T) {
	tools := BuildOverlayTools()
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
	if status.Idempotent != true {
		t.Error("status tool should be idempotent")
	}
if status.ExecutionPolicy.ApprovalRequired {
			t.Error("status tool should not require approval")
	}
}

func TestCreateToolSideEffects(t *testing.T) {
	tools := BuildOverlayTools()
	var create capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDCreate {
			create = tool
			break
		}
	}

	if create.ID == "" {
		t.Fatal("create tool not found")
	}
	if create.HasSideEffects != true {
		t.Error("create tool should have side effects")
	}
if !create.ExecutionPolicy.ApprovalRequired {
			t.Error("create tool should require approval")
	}
	if create.Idempotent != false {
		t.Error("create tool should not be idempotent")
	}
	if create.RiskLevel != capability.RiskHigh {
		t.Error("create tool should be high risk")
	}
}

func TestPermissionRequestTool(t *testing.T) {
	tools := BuildOverlayTools()
	var permTool capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDPermissionRequest {
			permTool = tool
			break
		}
	}

	if permTool.ID == "" {
		t.Fatal("permission request tool not found")
	}
	if permTool.HasSideEffects != true {
		t.Error("permission request tool should have side effects")
	}
if !permTool.ExecutionPolicy.ApprovalRequired {
			t.Error("permission request tool should require approval")
	}
	if permTool.Idempotent != false {
		t.Error("permission request tool should not be idempotent")
	}
}

func TestUpdateToolIdempotent(t *testing.T) {
	tools := BuildOverlayTools()
	var update capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDUpdate {
			update = tool
			break
		}
	}

	if update.ID == "" {
		t.Fatal("update tool not found")
	}
	if update.Idempotent != true {
		t.Error("update tool should be idempotent")
	}
}

func TestShowHideToolsIdempotent(t *testing.T) {
	tools := BuildOverlayTools()
	for _, tool := range tools {
		if tool.ID == ToolIDShow || tool.ID == ToolIDHide {
			if !tool.Idempotent {
				t.Errorf("tool %s should be idempotent", tool.ID)
			}
		}
	}
}

func TestCloseToolIdempotent(t *testing.T) {
	tools := BuildOverlayTools()
	var closeTool capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDClose {
			closeTool = tool
			break
		}
	}

	if closeTool.ID == "" {
		t.Fatal("close tool not found")
	}
	if closeTool.Idempotent != true {
		t.Error("close tool should be idempotent")
	}
	if closeTool.HasSideEffects != true {
		t.Error("close tool should have side effects")
	}
}

func TestListToolReadOnly(t *testing.T) {
	tools := BuildOverlayTools()
	var list capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDList {
			list = tool
			break
		}
	}

	if list.ID == "" {
		t.Fatal("list tool not found")
	}
	if list.HasSideEffects != false {
		t.Error("list tool should not have side effects")
	}
	if list.SideEffect != capability.SideEffectReadOnly {
		t.Error("list tool should be read-only")
	}
	if list.Idempotent != true {
		t.Error("list tool should be idempotent")
	}
	if list.RiskLevel != capability.RiskLow {
		t.Error("list tool should be low risk")
	}
}

func TestMapErrorPermissionDenied(t *testing.T) {
	err := MapError(OVERLAY_PERMISSION_REQUIRED, "permission required")
	if err.Code != capability.ErrorCodePermissionDenied {
		t.Errorf("expected PermissionDenied, got %s", err.Code)
	}
	if err.Retryable {
		t.Error("permission error should not be retryable")
	}
}

func TestMapErrorNotAvailable(t *testing.T) {
	err := MapError(OVERLAY_UNSUPPORTED, "not supported")
	if err.Code != capability.ErrorCodeNotAvailable {
		t.Errorf("expected NotAvailable, got %s", err.Code)
	}
}

func TestMapErrorInvalidInput(t *testing.T) {
	err := MapError(OVERLAY_INVALID_INPUT, "invalid input")
	if err.Code != capability.ErrorCodeInvalidInput {
		t.Errorf("expected InvalidInput, got %s", err.Code)
	}
}

func TestMapErrorNotFund(t *testing.T) {
	err := MapError(OVERLAY_NOT_FOUND, "not found")
	if err.Code != capability.ErrorCodeNotAvailable {
		t.Errorf("expected NotAvailable, got %s", err.Code)
	}
}

func TestMapErrorTimeout(t *testing.T) {
	err := MapError(OVERLAY_TIMEOUT, "timeout")
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

func TestMapErrorNilMessage(t *testing.T) {
	err := MapError(OVERLAY_CANCELLED, "")
	if err.Code != capability.ErrorCodeCancelled {
		t.Errorf("expected Cancelled, got %s", err.Code)
	}
}

func TestBuildOverlayToolsUniqueModelNames(t *testing.T) {
	tools := BuildOverlayTools()
	names := make(map[string]bool)
	for _, tool := range tools {
		if names[tool.ModelName] {
			t.Errorf("duplicate model name: %s", tool.ModelName)
		}
		names[tool.ModelName] = true
	}
}

func TestBuildOverlayToolsUniqueIDs(t *testing.T) {
	tools := BuildOverlayTools()
	ids := make(map[string]bool)
	for _, tool := range tools {
		if ids[tool.ID] {
			t.Errorf("duplicate tool ID: %s", tool.ID)
		}
		ids[tool.ID] = true
	}
}

func TestCreateToolInputSchema(t *testing.T) {
	tools := BuildOverlayTools()
	var create capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDCreate {
			create = tool
			break
		}
	}

	if create.ID == "" {
		t.Fatal("create tool not found")
	}
	if len(create.InputSchema) == 0 {
		t.Error("create tool should have input schema")
	}
}

func TestStatusToolInputSchema(t *testing.T) {
	tools := BuildOverlayTools()
	status := tools[0]

	if len(status.InputSchema) == 0 {
		t.Error("status tool should have input schema")
	}
}

func TestOverlayErrorInterface(t *testing.T) {
	err := newOverlayError(TEST_ERROR_CODE, "test message")
	if err.Error() != "test message" {
		t.Errorf("expected 'test message', got '%s'", err.Error())
	}
	if err.Code() != TEST_ERROR_CODE {
		t.Errorf("expected '%s', got '%s'", TEST_ERROR_CODE, err.Code())
	}
}

func TestOverlayErrorImplementsError(t *testing.T) {
	var err error = newOverlayError(TEST_ERROR_CODE, "test")
	if err == nil {
		t.Fatal("overlayError should implement error interface")
	}
	if err.Error() != "test" {
		t.Errorf("expected 'test', got '%s'", err.Error())
	}
}

const TEST_ERROR_CODE = "TEST_ERROR_CODE"

func TestBuildOverlayToolsPermissionRequirements(t *testing.T) {
	tools := BuildOverlayTools()
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

func TestBuildOverlayToolsCloseAllIdempotent(t *testing.T) {
	tools := BuildOverlayTools()
	var closeAll capability.ToolDefinition
	for _, tool := range tools {
		if tool.ID == ToolIDCloseAll {
			closeAll = tool
			break
		}
	}

	if closeAll.ID == "" {
		t.Fatal("close_all tool not found")
	}
	if !closeAll.Idempotent {
		t.Error("close_all tool should be idempotent")
	}
}
