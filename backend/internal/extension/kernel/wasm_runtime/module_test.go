package wasm_runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleManagerLoad(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, minimalValidWASM, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr := NewModuleManager(tmpDir)
	loaded, err := mgr.LoadFromPath("mod-1", "test.wasm")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ModuleID != "mod-1" {
		t.Fatalf("expected mod-1, got %s", loaded.ModuleID)
	}
	if loaded.Path != "test.wasm" {
		t.Fatalf("expected test.wasm, got %s", loaded.Path)
	}
	if loaded.Hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if loaded.SHA256 == "" {
		t.Fatalf("expected non-empty sha256")
	}
	if loaded.Hash != loaded.SHA256 {
		t.Fatalf("hash should equal sha256: %s != %s", loaded.Hash, loaded.SHA256)
	}
	if loaded.Size != int64(len(minimalValidWASM)) {
		t.Fatalf("expected size %d, got %d", len(minimalValidWASM), loaded.Size)
	}
	if loaded.Report == nil {
		t.Fatalf("expected non-nil report")
	}
	if !loaded.Report.Valid {
		t.Fatalf("expected valid report")
	}
	if loaded.LoadedAt == 0 {
		t.Fatalf("expected non-zero loaded_at")
	}
}

func TestModuleManagerLoadFromBytes(t *testing.T) {
	mgr := NewModuleManager("")
	loaded, err := mgr.LoadFromBytes("mod-bytes", minimalValidWASM)
	if err != nil {
		t.Fatalf("load from bytes: %v", err)
	}
	if loaded.ModuleID != "mod-bytes" {
		t.Fatalf("expected mod-bytes, got %s", loaded.ModuleID)
	}
	if loaded.Path != "" {
		t.Fatalf("expected empty path, got %s", loaded.Path)
	}
	if loaded.Hash == "" {
		t.Fatalf("expected non-empty hash")
	}
}

func TestModuleManagerLoadInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "invalid.wasm")
	if err := os.WriteFile(wasmPath, []byte("not wasm"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr := NewModuleManager(tmpDir)
	_, err := mgr.LoadFromPath("mod-invalid", "invalid.wasm")
	if err == nil {
		t.Fatalf("expected error for invalid wasm")
	}
}

func TestModuleManagerLoadFromBytesInvalid(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromBytes("mod-invalid", []byte("not wasm"))
	if err == nil {
		t.Fatalf("expected error for invalid wasm")
	}
}

func TestModuleManagerLoadFromBytesEmpty(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromBytes("mod-empty", []byte{})
	if err == nil {
		t.Fatalf("expected error for empty bytes")
	}
}

func TestModuleManagerLoadFromBytesEmptyID(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromBytes("", minimalValidWASM)
	if err == nil {
		t.Fatalf("expected error for empty module id")
	}
}

func TestModuleManagerLoadAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, minimalValidWASM, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromPath("mod-abs", "/absolute/path.wasm")
	if err == nil {
		t.Fatalf("expected error for absolute path")
	}
}

func TestModuleManagerLoadTraversalPath(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromPath("mod-traversal", "../../../etc/passwd")
	if err == nil {
		t.Fatalf("expected error for traversal path")
	}
}

func TestModuleManagerLoadNonExistent(t *testing.T) {
	mgr := NewModuleManager(t.TempDir())
	_, err := mgr.LoadFromPath("mod-missing", "nonexistent.wasm")
	if err == nil {
		t.Fatalf("expected error for non-existent file")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeModuleMissing {
		t.Fatalf("expected %s, got %s", ErrCodeModuleMissing, werr.Code)
	}
}

func TestModuleManagerGet(t *testing.T) {
	mgr := NewModuleManager("")
	loaded, err := mgr.LoadFromBytes("mod-get", minimalValidWASM)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := mgr.Get("mod-get")
	if !ok {
		t.Fatalf("expected to find mod-get")
	}
	if got.ModuleID != loaded.ModuleID {
		t.Fatalf("module id mismatch: %s != %s", got.ModuleID, loaded.ModuleID)
	}
	if got.Hash != loaded.Hash {
		t.Fatalf("hash mismatch: %s != %s", got.Hash, loaded.Hash)
	}
	if len(got.Bytes) != len(loaded.Bytes) {
		t.Fatalf("bytes length mismatch")
	}
	_, ok = mgr.Get("non-existent")
	if ok {
		t.Fatalf("should not find non-existent module")
	}
}

func TestModuleManagerUnload(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromBytes("mod-unload", minimalValidWASM)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, ok := mgr.Get("mod-unload")
	if !ok {
		t.Fatalf("expected to find mod-unload before unload")
	}
	mgr.Unload("mod-unload")
	_, ok = mgr.Get("mod-unload")
	if ok {
		t.Fatalf("should not find mod-unload after unload")
	}
	mgr.Unload("non-existent")
}

func TestModuleManagerList(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.LoadFromBytes("mod-list-1", minimalValidWASM)
	if err != nil {
		t.Fatalf("load 1: %v", err)
	}
	_, err = mgr.LoadFromBytes("mod-list-2", minimalValidWASM)
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(list))
	}
	ids := make(map[string]bool)
	for _, m := range list {
		ids[m.ModuleID] = true
	}
	if !ids["mod-list-1"] || !ids["mod-list-2"] {
		t.Fatalf("missing expected modules in list: %v", ids)
	}
}

func TestModuleManagerVerifyHash(t *testing.T) {
	mgr := NewModuleManager("")
	loaded, err := mgr.LoadFromBytes("mod-verify", minimalValidWASM)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := mgr.VerifyHash("mod-verify", loaded.Hash); err != nil {
		t.Fatalf("verify hash: %v", err)
	}
	err = mgr.VerifyHash("mod-verify", "wrong-hash")
	if err == nil {
		t.Fatalf("expected error for wrong hash")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeIntegrityFailed {
		t.Fatalf("expected %s, got %s", ErrCodeIntegrityFailed, werr.Code)
	}
	if err := mgr.VerifyHash("non-existent", "hash"); err == nil {
		t.Fatalf("expected error for non-existent module")
	}
}

func TestModuleManagerBuildDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "build.wasm")
	if err := os.WriteFile(wasmPath, minimalValidWASM, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr := NewModuleManager(tmpDir)
	_, err := mgr.LoadFromPath("mod-build", "build.wasm")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded, _ := mgr.Get("mod-build")
	def, err := mgr.BuildDefinition(BuildDefinitionParams{
		RuntimeDefinitionID: "def-1",
		ModuleID:            "mod-build",
		ExtensionID:         "ext-1",
	})
	if err != nil {
		t.Fatalf("build definition: %v", err)
	}
	if def.ModuleID != "mod-build" {
		t.Fatalf("expected mod-build, got %s", def.ModuleID)
	}
	if def.ModuleHash != loaded.Hash {
		t.Fatalf("hash mismatch: %s != %s", def.ModuleHash, loaded.Hash)
	}
	if def.EngineType != WazeroEngineName {
		t.Fatalf("expected %s, got %s", WazeroEngineName, def.EngineType)
	}
	if def.ABI != ABIAmitia {
		t.Fatalf("expected %s, got %s", ABIAmitia, def.ABI)
	}
	if def.InstancePolicy != InstancePolicyPerInvocation {
		t.Fatalf("expected per_invocation, got %s", def.InstancePolicy)
	}
	if def.EntryExport != ExportAmitiaInvoke {
		t.Fatalf("expected %s, got %s", ExportAmitiaInvoke, def.EntryExport)
	}
}

func TestModuleManagerBuildDefinition_Deterministic(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "build_det.wasm")
	if err := os.WriteFile(wasmPath, minimalValidWASM, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr := NewModuleManager(tmpDir)
	_, err := mgr.LoadFromPath("mod-build-det", "build_det.wasm")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	def, err := mgr.BuildDefinition(BuildDefinitionParams{
		RuntimeDefinitionID: "def-det",
		ModuleID:            "mod-build-det",
		ExtensionID:         "ext-1",
		Deterministic:       true,
	})
	if err != nil {
		t.Fatalf("build definition: %v", err)
	}
	if !def.Deterministic {
		t.Fatalf("expected deterministic")
	}
	for _, imp := range def.AllowedImports {
		policy := DeterministicPolicy()
		if !policy.AllowsImport(imp) {
			t.Fatalf("deterministic definition should not allow %s", imp)
		}
	}
}

func TestModuleManagerBuildDefinition_MissingModule(t *testing.T) {
	mgr := NewModuleManager("")
	_, err := mgr.BuildDefinition(BuildDefinitionParams{
		ModuleID: "non-existent",
	})
	if err == nil {
		t.Fatalf("expected error for missing module")
	}
}
