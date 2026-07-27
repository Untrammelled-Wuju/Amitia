package wasm_runtime

import (
	"testing"
	"time"
)

func TestDefaultWasmResourceLimits(t *testing.T) {
	limits := DefaultWasmResourceLimits()
	if limits.MaxModuleBytes != 16*1024*1024 {
		t.Fatalf("MaxModuleBytes: expected %d, got %d", 16*1024*1024, limits.MaxModuleBytes)
	}
	if limits.MaxMemoryPages != 1024 {
		t.Fatalf("MaxMemoryPages: expected 1024, got %d", limits.MaxMemoryPages)
	}
	if limits.MaxTableElements != 1024 {
		t.Fatalf("MaxTableElements: expected 1024, got %d", limits.MaxTableElements)
	}
	if limits.MaxFunctions != 4096 {
		t.Fatalf("MaxFunctions: expected 4096, got %d", limits.MaxFunctions)
	}
	if limits.MaxGlobals != 256 {
		t.Fatalf("MaxGlobals: expected 256, got %d", limits.MaxGlobals)
	}
	if limits.MaxImports != 64 {
		t.Fatalf("MaxImports: expected 64, got %d", limits.MaxImports)
	}
	if limits.MaxExports != 64 {
		t.Fatalf("MaxExports: expected 64, got %d", limits.MaxExports)
	}
	if limits.MaxInputBytes != 1*1024*1024 {
		t.Fatalf("MaxInputBytes: expected %d, got %d", 1*1024*1024, limits.MaxInputBytes)
	}
	if limits.MaxOutputBytes != 1*1024*1024 {
		t.Fatalf("MaxOutputBytes: expected %d, got %d", 1*1024*1024, limits.MaxOutputBytes)
	}
	if limits.MaxHostCalls != 128 {
		t.Fatalf("MaxHostCalls: expected 128, got %d", limits.MaxHostCalls)
	}
	if limits.MaxExecutionDuration != 5*time.Second {
		t.Fatalf("MaxExecutionDuration: expected 5s, got %v", limits.MaxExecutionDuration)
	}
	if limits.Fuel != 0 {
		t.Fatalf("Fuel: expected 0, got %d", limits.Fuel)
	}
}

func TestMemoryPagesFromBytes(t *testing.T) {
	if pages := MemoryPagesFromBytes(0); pages != 0 {
		t.Fatalf("0 bytes: expected 0 pages, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(-1); pages != 0 {
		t.Fatalf("-1 bytes: expected 0 pages, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(WasmPageSize); pages != 1 {
		t.Fatalf("1 page: expected 1, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(WasmPageSize + 1); pages != 2 {
		t.Fatalf("1 page + 1 byte: expected 2, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(WasmPageSize * 10); pages != 10 {
		t.Fatalf("10 pages: expected 10, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(WasmPageSize*10 - 1); pages != 10 {
		t.Fatalf("10 pages - 1 byte: expected 10, got %d", pages)
	}
	if pages := MemoryPagesFromBytes(int64(MaxWasmPages) * WasmPageSize); pages != MaxWasmPages {
		t.Fatalf("max pages: expected %d, got %d", MaxWasmPages, pages)
	}
	if pages := MemoryPagesFromBytes(int64(MaxWasmPages+1) * WasmPageSize); pages != MaxWasmPages {
		t.Fatalf("over max: expected %d, got %d", MaxWasmPages, pages)
	}
}

func TestOverrideWith(t *testing.T) {
	base := DefaultWasmResourceLimits()
	manifest := WasmResourceLimits{
		MaxMemoryPages:       512,
		MaxTableElements:     256,
		MaxFunctions:         2048,
		MaxGlobals:           128,
		MaxImports:           32,
		MaxExports:           32,
		MaxInputBytes:        512 * 1024,
		MaxOutputBytes:       512 * 1024,
		MaxHostCalls:         64,
		MaxExecutionDuration: 2 * time.Second,
		Fuel:                 500000,
	}
	merged := base.OverrideWith(manifest)
	if merged.MaxMemoryPages != 512 {
		t.Fatalf("MaxMemoryPages: expected 512, got %d", merged.MaxMemoryPages)
	}
	if merged.MaxTableElements != 256 {
		t.Fatalf("MaxTableElements: expected 256, got %d", merged.MaxTableElements)
	}
	if merged.MaxFunctions != 2048 {
		t.Fatalf("MaxFunctions: expected 2048, got %d", merged.MaxFunctions)
	}
	if merged.MaxGlobals != 128 {
		t.Fatalf("MaxGlobals: expected 128, got %d", merged.MaxGlobals)
	}
	if merged.MaxImports != 32 {
		t.Fatalf("MaxImports: expected 32, got %d", merged.MaxImports)
	}
	if merged.MaxExports != 32 {
		t.Fatalf("MaxExports: expected 32, got %d", merged.MaxExports)
	}
	if merged.MaxInputBytes != 512*1024 {
		t.Fatalf("MaxInputBytes: expected %d, got %d", 512*1024, merged.MaxInputBytes)
	}
	if merged.MaxOutputBytes != 512*1024 {
		t.Fatalf("MaxOutputBytes: expected %d, got %d", 512*1024, merged.MaxOutputBytes)
	}
	if merged.MaxHostCalls != 64 {
		t.Fatalf("MaxHostCalls: expected 64, got %d", merged.MaxHostCalls)
	}
	if merged.MaxExecutionDuration != 2*time.Second {
		t.Fatalf("MaxExecutionDuration: expected 2s, got %v", merged.MaxExecutionDuration)
	}
	if merged.Fuel != 500000 {
		t.Fatalf("Fuel: expected 500000, got %d", merged.Fuel)
	}
}

func TestOverrideWith_LargerValuesIgnored(t *testing.T) {
	base := DefaultWasmResourceLimits()
	manifest := WasmResourceLimits{
		MaxMemoryPages:       2048,
		MaxTableElements:     2048,
		MaxFunctions:         8192,
		MaxHostCalls:         256,
		MaxExecutionDuration: 10 * time.Second,
	}
	merged := base.OverrideWith(manifest)
	if merged.MaxMemoryPages != base.MaxMemoryPages {
		t.Fatalf("MaxMemoryPages should not increase: expected %d, got %d", base.MaxMemoryPages, merged.MaxMemoryPages)
	}
	if merged.MaxTableElements != base.MaxTableElements {
		t.Fatalf("MaxTableElements should not increase: expected %d, got %d", base.MaxTableElements, merged.MaxTableElements)
	}
	if merged.MaxFunctions != base.MaxFunctions {
		t.Fatalf("MaxFunctions should not increase: expected %d, got %d", base.MaxFunctions, merged.MaxFunctions)
	}
	if merged.MaxHostCalls != base.MaxHostCalls {
		t.Fatalf("MaxHostCalls should not increase: expected %d, got %d", base.MaxHostCalls, merged.MaxHostCalls)
	}
	if merged.MaxExecutionDuration != base.MaxExecutionDuration {
		t.Fatalf("MaxExecutionDuration should not increase: expected %v, got %v", base.MaxExecutionDuration, merged.MaxExecutionDuration)
	}
}

func TestOverrideWith_ZeroValuesIgnored(t *testing.T) {
	base := DefaultWasmResourceLimits()
	manifest := WasmResourceLimits{}
	merged := base.OverrideWith(manifest)
	if merged.MaxMemoryPages != base.MaxMemoryPages {
		t.Fatalf("MaxMemoryPages should not change for zero manifest")
	}
	if merged.MaxHostCalls != base.MaxHostCalls {
		t.Fatalf("MaxHostCalls should not change for zero manifest")
	}
	if merged.Fuel != 0 {
		t.Fatalf("Fuel should remain 0 for zero manifest")
	}
}

func TestOverrideWith_FuelAlwaysOverridden(t *testing.T) {
	base := DefaultWasmResourceLimits()
	base.Fuel = 1000000
	manifest := WasmResourceLimits{
		Fuel: 500000,
	}
	merged := base.OverrideWith(manifest)
	if merged.Fuel != 500000 {
		t.Fatalf("Fuel: expected 500000, got %d", merged.Fuel)
	}
}

func TestDeterministicPolicy(t *testing.T) {
	policy := DeterministicPolicy()
	if policy.AllowWallClock {
		t.Fatalf("AllowWallClock should be false")
	}
	if policy.AllowRandom {
		t.Fatalf("AllowRandom should be false")
	}
	if policy.AllowHostCalls {
		t.Fatalf("AllowHostCalls should be false")
	}
	if policy.AllowsImport(ImportTime) {
		t.Fatalf("time should not be allowed in deterministic mode")
	}
	if policy.AllowsImport(ImportRandom) {
		t.Fatalf("random should not be allowed in deterministic mode")
	}
	if policy.AllowsImport(ImportStorageGet) {
		t.Fatalf("storage_get should not be allowed in deterministic mode")
	}
	if !policy.AllowsImport(ImportLog) {
		t.Fatalf("log should be allowed in deterministic mode")
	}
}

func TestNonDeterministicPolicy(t *testing.T) {
	policy := NonDeterministicPolicy()
	if !policy.AllowWallClock {
		t.Fatalf("AllowWallClock should be true")
	}
	if !policy.AllowRandom {
		t.Fatalf("AllowRandom should be true")
	}
	if !policy.AllowHostCalls {
		t.Fatalf("AllowHostCalls should be true")
	}
	if !policy.AllowsImport(ImportTime) {
		t.Fatalf("time should be allowed in non-deterministic mode")
	}
	if !policy.AllowsImport(ImportRandom) {
		t.Fatalf("random should be allowed in non-deterministic mode")
	}
	if !policy.AllowsImport(ImportStorageGet) {
		t.Fatalf("storage_get should be allowed in non-deterministic mode")
	}
}
