package shizuku

import (
	"encoding/json"
	"testing"
)

func TestBuildShizukuToolsExposeAllOperations(t *testing.T) {
	tools := BuildShizukuTools()
	if len(tools) != 18 {
		t.Fatalf("tool count = %d, want 18", len(tools))
	}

	expected := map[string]string{
		"shizuku_status":                 OperationStatus,
		"shizuku_info":                   OperationInfo,
		"shizuku_request_permission":     OperationRequestPermission,
		"shizuku_permission_check":       OperationPermissionCheck,
		"shizuku_set_enabled":            OperationSetEnabled,
		"shizuku_open_manager":           OperationOpenManager,
		"shizuku_test":                   OperationTest,
		"shizuku_user_service_status":    OperationUserServiceStatus,
		"shizuku_user_service_bind":      OperationUserServiceBind,
		"shizuku_user_service_unbind":    OperationUserServiceUnbind,
		"shizuku_process_start":          OperationProcessStart,
		"shizuku_process_write":          OperationProcessWrite,
		"shizuku_process_read":           OperationProcessRead,
		"shizuku_process_wait":           OperationProcessWait,
		"shizuku_process_kill":           OperationProcessKill,
		"shizuku_system_service_resolve": OperationSystemService,
		"shizuku_binder_transact":        OperationBinderTransact,
		"shizuku_exec":                   OperationExecute,
	}
	for _, tool := range tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatalf("tool %s has invalid input schema: %v", tool.ModelName, err)
		}
		for _, char := range tool.ModelName {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
				t.Fatalf("model name %q is not OpenAI-compatible", tool.ModelName)
			}
		}
		operation, ok := expected[tool.ModelName]
		if !ok {
			t.Fatalf("unexpected model name %q", tool.ModelName)
		}
		if tool.Runtime.HandlerName != operation {
			t.Fatalf("%s handler = %s, want %s", tool.ModelName, tool.Runtime.HandlerName, operation)
		}
		delete(expected, tool.ModelName)
	}
	if len(expected) != 0 {
		t.Fatalf("missing tools: %#v", expected)
	}
}

func TestShizukuExecExposesCommandAndStructuredExecution(t *testing.T) {
	var execToolInput json.RawMessage
	for _, tool := range BuildShizukuTools() {
		if tool.ModelName == "shizuku_exec" {
			execToolInput = tool.InputSchema
			if len(tool.Permissions) != 1 || tool.Permissions[0].Capability != PermissionExecute {
				t.Fatalf("permissions = %#v", tool.Permissions)
			}
			if tool.Metadata["canonicalModelName"] != "shizuku.exec" {
				t.Fatalf("canonical model name = %v", tool.Metadata["canonicalModelName"])
			}
		}
	}
	if len(execToolInput) == 0 {
		t.Fatal("shizuku_exec not found")
	}

	var schema map[string]any
	if err := json.Unmarshal(execToolInput, &schema); err != nil {
		t.Fatal(err)
	}
	properties, _ := schema["properties"].(map[string]any)
	for _, key := range []string{"command", "executable", "args", "env", "workDir"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("missing shizuku.exec property %q", key)
		}
	}
}
