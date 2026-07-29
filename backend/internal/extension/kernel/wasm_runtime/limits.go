package wasm_runtime

import "time"

type WasmResourceLimits struct {
	MaxModuleBytes       int64
	MaxMemoryPages       uint32
	MaxTableElements     uint32
	MaxFunctions         uint32
	MaxGlobals           uint32
	MaxImports           uint32
	MaxExports           uint32
	MaxInputBytes        int64
	MaxOutputBytes       int64
	MaxHostCalls         int
	MaxExecutionDuration time.Duration
	Fuel                 uint64
}

func DefaultWasmResourceLimits() WasmResourceLimits {
	return WasmResourceLimits{
		MaxModuleBytes:       16 * 1024 * 1024,
		MaxMemoryPages:       1024,
		MaxTableElements:     1024,
		MaxFunctions:         4096,
		MaxGlobals:           256,
		MaxImports:           64,
		MaxExports:           64,
		MaxInputBytes:        1 * 1024 * 1024,
		MaxOutputBytes:       1 * 1024 * 1024,
		MaxHostCalls:         128,
		MaxExecutionDuration: 5 * time.Second,
		Fuel:                 0,
	}
}

func (l WasmResourceLimits) OverrideWith(manifest WasmResourceLimits) WasmResourceLimits {
	merged := l
	if manifest.MaxMemoryPages > 0 && manifest.MaxMemoryPages < l.MaxMemoryPages {
		merged.MaxMemoryPages = manifest.MaxMemoryPages
	}
	if manifest.MaxTableElements > 0 && manifest.MaxTableElements < l.MaxTableElements {
		merged.MaxTableElements = manifest.MaxTableElements
	}
	if manifest.MaxFunctions > 0 && manifest.MaxFunctions < l.MaxFunctions {
		merged.MaxFunctions = manifest.MaxFunctions
	}
	if manifest.MaxGlobals > 0 && manifest.MaxGlobals < l.MaxGlobals {
		merged.MaxGlobals = manifest.MaxGlobals
	}
	if manifest.MaxImports > 0 && manifest.MaxImports < l.MaxImports {
		merged.MaxImports = manifest.MaxImports
	}
	if manifest.MaxExports > 0 && manifest.MaxExports < l.MaxExports {
		merged.MaxExports = manifest.MaxExports
	}
	if manifest.MaxInputBytes > 0 && manifest.MaxInputBytes < l.MaxInputBytes {
		merged.MaxInputBytes = manifest.MaxInputBytes
	}
	if manifest.MaxOutputBytes > 0 && manifest.MaxOutputBytes < l.MaxOutputBytes {
		merged.MaxOutputBytes = manifest.MaxOutputBytes
	}
	if manifest.MaxHostCalls > 0 && manifest.MaxHostCalls < l.MaxHostCalls {
		merged.MaxHostCalls = manifest.MaxHostCalls
	}
	if manifest.MaxExecutionDuration > 0 && manifest.MaxExecutionDuration < l.MaxExecutionDuration {
		merged.MaxExecutionDuration = manifest.MaxExecutionDuration
	}
	if manifest.Fuel > 0 {
		merged.Fuel = manifest.Fuel
	}
	return merged
}

const (
	WasmPageSize = 65536
	MaxWasmPages = 65536
)

func MemoryPagesFromBytes(bytes int64) uint32 {
	if bytes <= 0 {
		return 0
	}
	pages := uint32(bytes / WasmPageSize)
	if bytes%WasmPageSize != 0 {
		pages++
	}
	if pages > MaxWasmPages {
		pages = MaxWasmPages
	}
	return pages
}

func IsSupportedInstancePolicy(p InstancePolicy) bool {
	return p == InstancePolicyPerInvocation
}

func ValidateInstancePolicy(p InstancePolicy) error {
	switch p {
	case InstancePolicyPerInvocation:
		return nil
	case InstancePolicyPooled, InstancePolicySingleton:
		return NewWASMError(ErrCodeModuleInvalid, "unsupported_wasm_instance_policy: "+string(p), nil)
	default:
		return NewWASMError(ErrCodeModuleInvalid, "invalid instance policy: "+string(p), nil)
	}
}

type WasmDeterminismPolicy struct {
	AllowWallClock bool
	AllowRandom    bool
	AllowHostCalls bool
}

func DeterministicPolicy() WasmDeterminismPolicy {
	return WasmDeterminismPolicy{
		AllowWallClock: false,
		AllowRandom:    false,
		AllowHostCalls: false,
	}
}

func NonDeterministicPolicy() WasmDeterminismPolicy {
	return WasmDeterminismPolicy{
		AllowWallClock: true,
		AllowRandom:    true,
		AllowHostCalls: true,
	}
}

func (p WasmDeterminismPolicy) AllowsImport(name HostImportName) bool {
	switch name {
	case ImportTime:
		return p.AllowWallClock
	case ImportRandom:
		return p.AllowRandom
	case ImportStorageGet, ImportStorageCAS, ImportResourceRead, ImportToolInvoke, ImportArtifactWrite, ImportResultSetError:
		return p.AllowHostCalls
	default:
		return true
	}
}

type EngineCapabilityReport struct {
	InstructionFuel   bool
	DeadlineCancel    bool
	MemoryLimit       bool
	TrapCapture       bool
	HostFunction      bool
	WasiDisabled      bool
	InstanceIsolation bool
}
