package wasm_runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func TestWASMRuntimeFactoryInvoke(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	factory := NewWASMRuntimeFactory(engine, nil)

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	if err := factory.LoadModule("mod-1", makeValidWASMBytes()); err != nil {
		t.Fatalf("load module: %v", err)
	}

	result, err := factory.Invoke(context.Background(), "mod-1", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("factory invoke: %v", err)
	}
	if result.Output == nil {
		t.Fatalf("expected non-nil output")
	}
	if len(result.Output) == 0 {
		t.Fatalf("expected non-empty output")
	}

	result2, err := factory.Invoke(context.Background(), "mod-1", json.RawMessage(`{"x":2}`))
	if err != nil {
		t.Fatalf("factory invoke 2: %v", err)
	}
	if result2.Output == nil {
		t.Fatalf("expected non-nil output on second invoke")
	}
	if !result2.Cached {
		t.Fatalf("second invoke should use cached module")
	}

	spec := runtime_supervisor.InstanceSpec{
		DefinitionID: runtime_supervisor.DefinitionID("def-1"),
		ExtensionID:  domain.ExtensionID("ext-1"),
		ModuleID:     domain.ModuleID("mod-1"),
		RuntimeType:  domain.RuntimeTypeWASM,
		Generation:   1,
		EntryPoint:   "invoke",
	}

	managed, err := factory.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("create managed runtime: %v", err)
	}

	wmr, ok := managed.(*WASMManagedRuntime)
	if !ok {
		t.Fatalf("expected *WASMManagedRuntime, got %T", managed)
	}
	if wmr.instanceID == "" {
		t.Fatalf("expected non-empty instance id")
	}
	if wmr.State() != runtime_supervisor.ActualCreated {
		t.Fatalf("expected created state, got %s", wmr.State())
	}

	if err := managed.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if wmr.State() != runtime_supervisor.ActualReady {
		t.Fatalf("expected ready state, got %s", wmr.State())
	}

	invReq := runtime_supervisor.InvocationRequest{
		InstanceID:   wmr.instanceID,
		InvocationID: "inv-1",
		Operation:    "test_op",
		Input:        []byte(`{"hello":"world"}`),
		Deadline:     time.Now().Add(10 * time.Second),
		Generation:   1,
	}

	invResult := managed.Invoke(context.Background(), invReq)
	if invResult.Status != "success" {
		t.Fatalf("expected success, got %s: %v", invResult.Status, invResult.Error)
	}
	if invResult.InvocationID != "inv-1" {
		t.Fatalf("expected inv-1, got %s", invResult.InvocationID)
	}
	if len(invResult.Output) == 0 {
		t.Fatalf("expected non-empty output")
	}

	health := managed.Health(context.Background())
	if health.Status != runtime_supervisor.HealthHealthy {
		t.Fatalf("expected healthy, got %s: %s", health.Status, health.Reason)
	}
	if health.Metrics["invocations"].(int64) < 1 {
		t.Fatalf("expected at least 1 invocation in metrics")
	}

	invReq2 := runtime_supervisor.InvocationRequest{
		InstanceID:   wmr.instanceID,
		InvocationID: "inv-2",
		Operation:    "test_op_2",
		Input:        []byte(`{"name":"amitia"}`),
		Deadline:     time.Now().Add(10 * time.Second),
		Generation:   1,
	}
	invResult2 := managed.Invoke(context.Background(), invReq2)
	if invResult2.Status != "success" {
		t.Fatalf("expected success on second invoke, got %s: %v", invResult2.Status, invResult2.Error)
	}

	if err := managed.Stop(context.Background(), runtime_supervisor.StopReasonManual); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if wmr.State() != runtime_supervisor.ActualStopped {
		t.Fatalf("expected stopped state, got %s", wmr.State())
	}

	invResult3 := managed.Invoke(context.Background(), invReq)
	if invResult3.Status != "failed" {
		t.Fatalf("expected failed after stop, got %s", invResult3.Status)
	}

	_, err = factory.Invoke(context.Background(), "non-existent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for non-existent module")
	}
}

func TestWASMRuntimeFactoryInvoke_EmptyInput(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	factory := NewWASMRuntimeFactory(engine, nil)

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if err := factory.LoadModule("mod-1", makeValidWASMBytes()); err != nil {
		t.Fatalf("load module: %v", err)
	}

	result, err := factory.Invoke(context.Background(), "mod-1", nil)
	if err != nil {
		t.Fatalf("invoke with nil input: %v", err)
	}
	if result.Output == nil {
		t.Fatalf("expected non-nil output")
	}

	result2, err := factory.Invoke(context.Background(), "mod-1", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("invoke with empty object: %v", err)
	}
	if result2.Output == nil {
		t.Fatalf("expected non-nil output")
	}
}

func TestWASMRuntimeFactoryRegisterDefinition(t *testing.T) {
	factory := NewWASMRuntimeFactory(NewMockEngine("mock", "1.0"), nil)

	if factory.Type() != domain.RuntimeTypeWASM {
		t.Fatalf("expected %s, got %s", domain.RuntimeTypeWASM, factory.Type())
	}

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	got, ok := factory.GetDefinition("mod-1")
	if !ok {
		t.Fatalf("expected to find definition for mod-1")
	}
	if got.ModuleID != "mod-1" {
		t.Fatalf("expected mod-1, got %s", got.ModuleID)
	}
	if got.ExtensionID != "ext-1" {
		t.Fatalf("expected ext-1, got %s", got.ExtensionID)
	}
	if got.ABI != ABIAmitia {
		t.Fatalf("expected %s, got %s", ABIAmitia, got.ABI)
	}

	_, ok = factory.GetDefinition("non-existent")
	if ok {
		t.Fatalf("should not find non-existent definition")
	}

	invalidDef := makeValidDefinition(t)
	invalidDef.ModuleID = ""
	if err := factory.RegisterDefinition(invalidDef); err == nil {
		t.Fatalf("expected error for missing module id")
	}

	missingPathDef := makeValidDefinition(t)
	missingPathDef.ModulePath = ""
	if err := factory.RegisterDefinition(missingPathDef); err == nil {
		t.Fatalf("expected error for missing module path")
	}

	badMemDef := makeValidDefinition(t)
	badMemDef.ModuleID = "mod-bad-mem"
	badMemDef.MemoryLimitBytes = 0
	badMemDef.Limits.MaxMemoryPages = 0
	if err := factory.RegisterDefinition(badMemDef); err == nil {
		t.Fatalf("expected error for bad memory limit")
	}

	forbiddenDef := makeValidDefinition(t)
	forbiddenDef.ModuleID = "mod-forbidden"
	forbiddenDef.AllowedImports = []HostImportName{HostImportName("filesystem_raw")}
	if err := factory.RegisterDefinition(forbiddenDef); err == nil {
		t.Fatalf("expected error for forbidden host function")
	}

	deterministicDef := makeValidDefinition(t)
	deterministicDef.ModuleID = "mod-det"
	deterministicDef.Deterministic = true
	deterministicDef.AllowedImports = []HostImportName{ImportTime}
	if err := factory.RegisterDefinition(deterministicDef); err == nil {
		t.Fatalf("expected error for deterministic + time")
	}

	validDetDef := makeValidDefinition(t)
	validDetDef.ModuleID = "mod-det-ok"
	validDetDef.Deterministic = true
	validDetDef.AllowedImports = []HostImportName{ImportLog}
	if err := factory.RegisterDefinition(validDetDef); err != nil {
		t.Fatalf("expected valid for deterministic + log: %v", err)
	}

	overwrittenDef := makeValidDefinition(t)
	overwrittenDef.MaxHostCalls = 999
	if err := factory.RegisterDefinition(overwrittenDef); err != nil {
		t.Fatalf("overwrite definition: %v", err)
	}
	got2, ok := factory.GetDefinition("mod-1")
	if !ok {
		t.Fatalf("expected to find overwritten definition")
	}
	if got2.MaxHostCalls != 999 {
		t.Fatalf("expected 999, got %d", got2.MaxHostCalls)
	}
}

func TestWASMRuntimeFactoryValidateSpec(t *testing.T) {
	factory := NewWASMRuntimeFactory(NewMockEngine("mock", "1.0"), nil)

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	spec := runtime_supervisor.InstanceSpec{
		DefinitionID: runtime_supervisor.DefinitionID("def-1"),
		ExtensionID:  domain.ExtensionID("ext-1"),
		ModuleID:     domain.ModuleID("mod-1"),
		RuntimeType:  domain.RuntimeTypeWASM,
		EntryPoint:   "invoke",
	}
	if err := factory.Validate(spec); err == nil {
		t.Fatalf("expected error for missing module")
	}

	if err := factory.LoadModule("mod-1", makeValidWASMBytes()); err != nil {
		t.Fatalf("load module: %v", err)
	}
	if err := factory.Validate(spec); err != nil {
		t.Fatalf("validate should pass: %v", err)
	}

	wrongTypeSpec := spec
	wrongTypeSpec.RuntimeType = domain.RuntimeType("native")
	if err := factory.Validate(wrongTypeSpec); err == nil {
		t.Fatalf("expected error for wrong runtime type")
	}

	missingExtSpec := spec
	missingExtSpec.ExtensionID = ""
	if err := factory.Validate(missingExtSpec); err == nil {
		t.Fatalf("expected error for missing extension id")
	}

	missingModSpec := spec
	missingModSpec.ModuleID = ""
	if err := factory.Validate(missingModSpec); err == nil {
		t.Fatalf("expected error for missing module id")
	}

	missingEntrySpec := spec
	missingEntrySpec.EntryPoint = ""
	if err := factory.Validate(missingEntrySpec); err == nil {
		t.Fatalf("expected error for missing entry point")
	}

	unknownModSpec := spec
	unknownModSpec.ModuleID = domain.ModuleID("non-existent")
	if err := factory.Validate(unknownModSpec); err == nil {
		t.Fatalf("expected error for non-existent module")
	}
}

func TestWASMRuntimeFactoryCreateAndManage(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	factory := NewWASMRuntimeFactory(engine, nil)

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if err := factory.LoadModule("mod-1", makeValidWASMBytes()); err != nil {
		t.Fatalf("load module: %v", err)
	}

	spec := runtime_supervisor.InstanceSpec{
		DefinitionID: runtime_supervisor.DefinitionID("def-1"),
		ExtensionID:  domain.ExtensionID("ext-1"),
		ModuleID:     domain.ModuleID("mod-1"),
		RuntimeType:  domain.RuntimeTypeWASM,
		Generation:   1,
		EntryPoint:   "invoke",
	}

	managed, err := factory.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	wmr := managed.(*WASMManagedRuntime)
	if wmr.Identity().ExtensionID != domain.ExtensionID("ext-1") {
		t.Fatalf("expected ext-1, got %s", wmr.Identity().ExtensionID)
	}
	if wmr.Identity().ModuleID != domain.ModuleID("mod-1") {
		t.Fatalf("expected mod-1, got %s", wmr.Identity().ModuleID)
	}
	if wmr.Identity().RuntimeType != domain.RuntimeTypeWASM {
		t.Fatalf("expected wasm, got %s", wmr.Identity().RuntimeType)
	}
	if wmr.Identity().Generation != 1 {
		t.Fatalf("expected 1, got %d", wmr.Identity().Generation)
	}
	if wmr.Identity().SessionNonce == "" {
		t.Fatalf("expected non-empty session nonce")
	}

	gotInst, ok := factory.GetInstance(wmr.instanceID)
	if !ok {
		t.Fatalf("expected to find instance %s", wmr.instanceID)
	}
	if gotInst != wmr {
		t.Fatalf("instance mismatch")
	}

	instances := factory.ListInstances()
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	if err := managed.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	stats := wmr.Stats()
	if stats["state"] != string(runtime_supervisor.ActualReady) {
		t.Fatalf("expected ready, got %v", stats["state"])
	}

	invReq := runtime_supervisor.InvocationRequest{
		InstanceID:   wmr.instanceID,
		InvocationID: "inv-stats",
		Operation:    "test",
		Input:        []byte(`{}`),
	}
	result := managed.Invoke(context.Background(), invReq)
	if result.Status != "success" {
		t.Fatalf("invoke: %v", result.Error)
	}

	stats = wmr.Stats()
	if stats["invocations"].(int64) < 1 {
		t.Fatalf("expected at least 1 invocation")
	}

	if err := managed.Stop(context.Background(), runtime_supervisor.StopReasonDrain); err != nil {
		t.Fatalf("stop: %v", err)
	}

	health := managed.Health(context.Background())
	if health.Status != runtime_supervisor.HealthUnknown {
		t.Fatalf("expected unknown for stopped, got %s", health.Status)
	}

	invalidSpec := runtime_supervisor.InstanceSpec{
		DefinitionID: runtime_supervisor.DefinitionID("def-bad"),
		ExtensionID:  domain.ExtensionID("ext-1"),
		ModuleID:     domain.ModuleID("non-existent"),
		RuntimeType:  domain.RuntimeTypeWASM,
		EntryPoint:   "invoke",
	}
	_, err = factory.Create(context.Background(), invalidSpec)
	if err == nil {
		t.Fatalf("expected error for invalid spec")
	}
}

func TestWASMRuntimeFactoryLoadModuleFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test_module.wasm")
	if err := os.WriteFile(wasmPath, makeValidWASMBytes(), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	engine := NewMockEngine("mock", "1.0")
	mgr := NewModuleManager(tmpDir)
	factory := NewWASMRuntimeFactory(engine, mgr)

	def := makeValidDefinition(t)
	def.ModulePath = "test_module.wasm"
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	if err := factory.LoadModuleFromPath("mod-1", "test_module.wasm"); err != nil {
		t.Fatalf("load module from path: %v", err)
	}

	result, err := factory.Invoke(context.Background(), "mod-1", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Output == nil {
		t.Fatalf("expected non-nil output")
	}
}

func TestWASMRuntimeFactoryMultipleInvocations(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	factory := NewWASMRuntimeFactory(engine, nil)

	def := makeValidDefinition(t)
	if err := factory.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if err := factory.LoadModule("mod-1", makeValidWASMBytes()); err != nil {
		t.Fatalf("load module: %v", err)
	}

	inputs := []string{
		`{"a":1}`,
		`{"b":2}`,
		`{"c":3}`,
		`{"d":4}`,
		`{"e":5}`,
	}

	for i, input := range inputs {
		result, err := factory.Invoke(context.Background(), "mod-1", json.RawMessage(input))
		if err != nil {
			t.Fatalf("invoke %d: %v", i, err)
		}
		if result.Output == nil {
			t.Fatalf("expected non-nil output for invoke %d", i)
		}
		if i > 0 && !result.Cached {
			t.Fatalf("invoke %d should use cached module", i)
		}
	}
}

func TestWASMRuntimeFactoryModuleManager(t *testing.T) {
	mgr := NewModuleManager("")
	factory := NewWASMRuntimeFactory(NewMockEngine("mock", "1.0"), mgr)

	if factory.ModuleManager() != mgr {
		t.Fatalf("module manager mismatch")
	}

	if factory.Runtime() == nil {
		t.Fatalf("expected non-nil runtime")
	}
}
