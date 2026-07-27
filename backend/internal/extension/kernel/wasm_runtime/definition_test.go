package wasm_runtime

import (
	"testing"
)

func TestValidateDefinition_Valid(t *testing.T) {
	def := makeValidDefinition(t)
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
	if def.EntryExport != "invoke" {
		t.Fatalf("entry export should be invoke, got %s", def.EntryExport)
	}
}

func TestValidateDefinition_MissingModuleID(t *testing.T) {
	def := makeValidDefinition(t)
	def.ModuleID = ""
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing module_id")
	}
	if err := ValidateDefinition(nil); err == nil {
		t.Fatalf("expected error for nil definition")
	}
}

func TestValidateDefinition_MissingModulePath(t *testing.T) {
	def := makeValidDefinition(t)
	def.ModulePath = ""
	err := ValidateDefinition(def)
	if err == nil {
		t.Fatalf("expected error for missing module_path")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeModuleInvalid {
		t.Fatalf("expected %s, got %s", ErrCodeModuleInvalid, werr.Code)
	}
}

func TestValidateDefinition_MissingModuleHash(t *testing.T) {
	def := makeValidDefinition(t)
	def.ModuleHash = ""
	def.ModuleSHA256 = ""
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing module hash")
	}
}

func TestValidateDefinition_InvalidMemoryLimit(t *testing.T) {
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 0
	def.Limits.MaxMemoryPages = 0
	err := ValidateDefinition(def)
	if err == nil {
		t.Fatalf("expected error for invalid memory limit")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeMemoryLimit {
		t.Fatalf("expected %s, got %s", ErrCodeMemoryLimit, werr.Code)
	}
}

func TestValidateDefinition_MemoryLimitFromPages(t *testing.T) {
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 0
	def.Limits.MaxMemoryPages = 256
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid with pages fallback: %v", err)
	}
	expected := int64(256) * WasmPageSize
	if def.MemoryLimitBytes != expected {
		t.Fatalf("expected %d, got %d", expected, def.MemoryLimitBytes)
	}
}

func TestValidateDefinition_MemoryLimitTooLarge(t *testing.T) {
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 2 << 30
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for memory > 1GB")
	}
}

func TestValidateDefinition_MissingFuel(t *testing.T) {
	def := makeValidDefinition(t)
	def.FuelLimit = 0
	def.Limits.Fuel = 0
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing fuel")
	}
}

func TestValidateDefinition_FuelFromLimits(t *testing.T) {
	def := makeValidDefinition(t)
	def.FuelLimit = 0
	def.Limits.Fuel = 500000
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid with fuel fallback: %v", err)
	}
	if def.FuelLimit != 500000 {
		t.Fatalf("expected 500000, got %d", def.FuelLimit)
	}
}

func TestValidateDefinition_MissingABI(t *testing.T) {
	def := makeValidDefinition(t)
	def.ABI = ""
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing abi")
	}
}

func TestValidateDefinition_MissingEntryExport(t *testing.T) {
	def := makeValidDefinition(t)
	def.EntryExport = ""
	def.Entry.ExportName = ""
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing entry export")
	}
}

func TestValidateDefinition_EntryExportFromEntry(t *testing.T) {
	def := makeValidDefinition(t)
	def.EntryExport = ""
	def.Entry.ExportName = "custom_invoke"
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if def.EntryExport != "custom_invoke" {
		t.Fatalf("expected custom_invoke, got %s", def.EntryExport)
	}
}

func TestValidateDefinition_MissingMaxOutput(t *testing.T) {
	def := makeValidDefinition(t)
	def.MaxOutputBytes = 0
	def.Limits.MaxOutputBytes = 0
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing max output")
	}
}

func TestValidateDefinition_MaxOutputFromLimits(t *testing.T) {
	def := makeValidDefinition(t)
	def.MaxOutputBytes = 0
	def.Limits.MaxOutputBytes = 512 * 1024
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if def.MaxOutputBytes != 512*1024 {
		t.Fatalf("expected 512KB, got %d", def.MaxOutputBytes)
	}
}

func TestValidateDefinition_DeterministicWithForbiddenImport(t *testing.T) {
	def := makeValidDefinition(t)
	def.Deterministic = true
	def.AllowedImports = []HostImportName{ImportTime}
	err := ValidateDefinition(def)
	if err == nil {
		t.Fatalf("expected error for deterministic + time")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeImportNotAllowed {
		t.Fatalf("expected %s, got %s", ErrCodeImportNotAllowed, werr.Code)
	}
}

func TestValidateDefinition_DeterministicWithRandom(t *testing.T) {
	def := makeValidDefinition(t)
	def.Deterministic = true
	def.AllowedImports = []HostImportName{ImportRandom}
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for deterministic + random")
	}
}

func TestValidateDefinition_DeterministicWithLog(t *testing.T) {
	def := makeValidDefinition(t)
	def.Deterministic = true
	def.AllowedImports = []HostImportName{ImportLog}
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("expected valid for deterministic + log: %v", err)
	}
}

func TestValidateDefinition_ForbiddenHostFunction(t *testing.T) {
	def := makeValidDefinition(t)
	def.AllowedImports = []HostImportName{HostImportName("filesystem_raw")}
	err := ValidateDefinition(def)
	if err == nil {
		t.Fatalf("expected error for forbidden host function")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeHostFunctionDenied {
		t.Fatalf("expected %s, got %s", ErrCodeHostFunctionDenied, werr.Code)
	}
}

func TestValidateDefinition_UnsupportedWASI(t *testing.T) {
	def := makeValidDefinition(t)
	def.WASIVersion = WASIVersion("wasi-v2")
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for unsupported wasi")
	}
}

func TestValidateDefinition_WASIV1(t *testing.T) {
	def := makeValidDefinition(t)
	def.WASIVersion = WASIV1
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for wasi-v1")
	}
}

func TestValidateModulePath(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
	}{
		{"modules/test.wasm", true},
		{"a/b/c/module.wasm", true},
		{"module.wasm", true},
		{"", false},
		{"/absolute/path.wasm", false},
		{"C:/windows/path.wasm", false},
		{"../traversal.wasm", false},
		{"a/../b/traversal.wasm", false},
	}
	for _, tc := range tests {
		err := ValidateModulePath(tc.path)
		if tc.ok && err != nil {
			t.Fatalf("expected ok for %q, got: %v", tc.path, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("expected error for %q", tc.path)
		}
	}
}

func TestValidateInstancePolicy(t *testing.T) {
	if err := ValidateInstancePolicy(InstancePolicyPerInvocation); err != nil {
		t.Fatalf("per_invocation should be valid: %v", err)
	}
	if err := ValidateInstancePolicy(InstancePolicyPooled); err == nil {
		t.Fatalf("pooled should be invalid")
	}
	if err := ValidateInstancePolicy(InstancePolicySingleton); err == nil {
		t.Fatalf("singleton should be invalid")
	}
	if err := ValidateInstancePolicy(InstancePolicy("invalid")); err == nil {
		t.Fatalf("invalid policy should be rejected")
	}
	if !IsSupportedInstancePolicy(InstancePolicyPerInvocation) {
		t.Fatalf("per_invocation should be supported")
	}
	if IsSupportedInstancePolicy(InstancePolicyPooled) {
		t.Fatalf("pooled should not be supported")
	}
}
