package adb

import (
	"testing"
)

func TestBuildADBTools(t *testing.T) {
	tools := BuildADBTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 ADB tools, got %d", len(tools))
	}

	statusTool := tools[0]
	if statusTool.ID != "android.adb.status" {
		t.Fatalf("expected tool ID android.adb.status, got %s", statusTool.ID)
	}
	if !statusTool.Idempotent {
		t.Fatalf("expected status tool to be idempotent")
	}
	if statusTool.HasSideEffects {
		t.Fatalf("expected status tool to have no side effects")
	}
	if statusTool.RiskLevel != "low" {
		t.Fatalf("expected status tool risk low, got %s", statusTool.RiskLevel)
	}
	if !statusTool.Enabled {
		t.Fatalf("expected status tool to be enabled")
	}
	if statusTool.Runtime.RuntimeType != "android_native" {
		t.Fatalf("expected runtime type android_native, got %s", statusTool.Runtime.RuntimeType)
	}
	if statusTool.Runtime.HandlerName != "adb.status" {
		t.Fatalf("expected handler name adb.status, got %s", statusTool.Runtime.HandlerName)
	}

	devicesTool := tools[1]
	if devicesTool.ID != "android.adb.devices" {
		t.Fatalf("expected tool ID android.adb.devices, got %s", devicesTool.ID)
	}
	if !devicesTool.Idempotent {
		t.Fatalf("expected devices tool to be idempotent")
	}
	if devicesTool.HasSideEffects {
		t.Fatalf("expected devices tool to have no side effects")
	}
	if devicesTool.Runtime.HandlerName != "adb.devices" {
		t.Fatalf("expected handler name adb.devices, got %s", devicesTool.Runtime.HandlerName)
	}

	executeTool := tools[2]
	if executeTool.ID != "android.adb.execute" {
		t.Fatalf("expected tool ID android.adb.execute, got %s", executeTool.ID)
	}
	if executeTool.Idempotent {
		t.Fatalf("expected execute tool to NOT be idempotent")
	}
	if !executeTool.HasSideEffects {
		t.Fatalf("expected execute tool to have side effects")
	}
	if executeTool.RiskLevel != "medium" {
		t.Fatalf("expected execute tool risk medium, got %s", executeTool.RiskLevel)
	}
	if executeTool.Retryable {
		t.Fatalf("expected execute tool to NOT be retryable")
	}
	if !executeTool.ExecutionPolicy.ApprovalRequired {
		t.Fatalf("expected execute tool to require approval")
	}
	if executeTool.Runtime.HandlerName != "adb.execute" {
		t.Fatalf("expected handler name adb.execute, got %s", executeTool.Runtime.HandlerName)
	}
}

func TestBuildADBPermissionDefinitions(t *testing.T) {
	defs := BuildADBPermissionDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 permission definitions, got %d", len(defs))
	}

	if defs[0].ID != PermissionADBInspect {
		t.Fatalf("expected first permission ID %s, got %s", PermissionADBInspect, defs[0].ID)
	}
	if defs[1].ID != PermissionADBExecute {
		t.Fatalf("expected second permission ID %s, got %s", PermissionADBExecute, defs[1].ID)
	}
	if defs[0].Risk != "low" {
		t.Fatalf("expected inspect risk low, got %s", defs[0].Risk)
	}
	if defs[1].Risk != "medium" {
		t.Fatalf("expected execute risk medium, got %s", defs[1].Risk)
	}
}

func TestConstants(t *testing.T) {
	if OperationStatus != "adb.status" {
		t.Fatalf("expected OperationStatus=adb.status, got %s", OperationStatus)
	}
	if OperationDevices != "adb.devices" {
		t.Fatalf("expected OperationDevices=adb.devices, got %s", OperationDevices)
	}
	if OperationExecute != "adb.execute" {
		t.Fatalf("expected OperationExecute=adb.execute, got %s", OperationExecute)
	}
	if PermissionADBInspect != "android.adb.inspect" {
		t.Fatalf("expected PermissionADBInspect=android.adb.inspect, got %s", PermissionADBInspect)
	}
	if PermissionADBExecute != "android.adb.execute" {
		t.Fatalf("expected PermissionADBExecute=android.adb.execute, got %s", PermissionADBExecute)
	}
}

func TestToolVersion(t *testing.T) {
	tools := BuildADBTools()
	for _, tool := range tools {
		if tool.ToolVersion.Revision != "b28-adb-v1" {
			t.Errorf("tool %s has wrong revision %s", tool.ID, tool.ToolVersion.Revision)
		}
	}
}
