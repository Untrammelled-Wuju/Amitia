package wasm_runtime

import (
	"context"
	"testing"
)

var minimalValidWASM = []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

func TestWazeroEngineName(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	if engine.Name() != WazeroEngineName {
		t.Fatalf("expected %s, got %s", WazeroEngineName, engine.Name())
	}
}

func TestWazeroEngineVersion(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	if engine.Version() != WazeroEngineVersion {
		t.Fatalf("expected %s, got %s", WazeroEngineVersion, engine.Version())
	}
}

func TestWazeroEngineCompile(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	ctx := context.Background()
	compiled, err := engine.Compile(ctx, minimalValidWASM)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer compiled.Dispose()
	if compiled.Hash() == "" {
		t.Fatalf("expected non-empty hash")
	}
	if len(compiled.Exports()) != 0 {
		t.Fatalf("expected 0 exports, got %d", len(compiled.Exports()))
	}
	if len(compiled.Imports()) != 0 {
		t.Fatalf("expected 0 imports, got %d", len(compiled.Imports()))
	}
}

func TestWazeroEngineCompileEmpty(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	ctx := context.Background()
	_, err := engine.Compile(ctx, []byte{})
	if err == nil {
		t.Fatalf("expected error for empty bytes")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeModuleMissing {
		t.Fatalf("expected %s, got %s", ErrCodeModuleMissing, werr.Code)
	}
}

func TestWazeroEngineCompileTooSmall(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	ctx := context.Background()
	_, err := engine.Compile(ctx, []byte{0x00, 0x01})
	if err == nil {
		t.Fatalf("expected error for too small bytes")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeModuleInvalid {
		t.Fatalf("expected %s, got %s", ErrCodeModuleInvalid, werr.Code)
	}
}

func TestWazeroEngineCompileInvalid(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	ctx := context.Background()
	_, err := engine.Compile(ctx, []byte("not wasm"))
	if err == nil {
		t.Fatalf("expected error for invalid wasm")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeCompileFailed {
		t.Fatalf("expected %s, got %s", ErrCodeCompileFailed, werr.Code)
	}
}

func TestWazeroEngineCapabilities(t *testing.T) {
	engine := NewWazeroEngine()
	defer engine.Close(context.Background())
	caps := engine.Capabilities()
	if caps.InstructionFuel {
		t.Fatalf("InstructionFuel should be false")
	}
	if !caps.DeadlineCancel {
		t.Fatalf("DeadlineCancel should be true")
	}
	if !caps.MemoryLimit {
		t.Fatalf("MemoryLimit should be true")
	}
	if !caps.TrapCapture {
		t.Fatalf("TrapCapture should be true")
	}
	if !caps.HostFunction {
		t.Fatalf("HostFunction should be true")
	}
	if !caps.WasiDisabled {
		t.Fatalf("WasiDisabled should be true")
	}
	if !caps.InstanceIsolation {
		t.Fatalf("InstanceIsolation should be true")
	}
}
