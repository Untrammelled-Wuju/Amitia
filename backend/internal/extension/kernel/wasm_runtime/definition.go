package wasm_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type InstancePolicy string

const (
	InstancePolicyPerInvocation InstancePolicy = "per_invocation"
	InstancePolicyPooled        InstancePolicy = "pooled"
	InstancePolicySingleton     InstancePolicy = "singleton"
)

type WASIVersion string

const (
	WASINone WASIVersion = ""
	WASIV1   WASIVersion = "wasi-v1"
)

type ABIKind string

const (
	ABIRaw    ABIKind = "raw"
	ABIWIT    ABIKind = "wit"
	ABIAmitia ABIKind = "amitia"
)

type HostImportName string

const (
	ImportLog         HostImportName = "amitia.log"
	ImportTime        HostImportName = "amitia.time"
	ImportRandom      HostImportName = "amitia.random"
	ImportResourceRead HostImportName = "amitia.resource.read"
	ImportStorageGet  HostImportName = "amitia.storage.get"
	ImportStorageCAS  HostImportName = "amitia.storage.cas"
	ImportToolInvoke  HostImportName = "amitia.tool.invoke"
)

var DefaultAllowedImports = []HostImportName{
	ImportLog,
	ImportTime,
}

type WASMRuntimeDefinition struct {
	ModuleID         string                  `json:"module_id"`
	ExtensionID      string                  `json:"extension_id"`
	ModulePath       string                  `json:"module_path"`
	ModuleHash       string                  `json:"module_hash"`
	InterfacePath    string                  `json:"interface_path,omitempty"`
	InterfaceHash    string                  `json:"interface_hash,omitempty"`
	ABI              ABIKind                 `json:"abi"`
	WASIVersion      WASIVersion             `json:"wasi_version"`
	MemoryLimitBytes int64                   `json:"memory_limit_bytes"`
	FuelLimit        uint64                  `json:"fuel_limit"`
	InstancePolicy   InstancePolicy          `json:"instance_policy"`
	AllowedImports   []HostImportName        `json:"allowed_imports"`
	Deterministic    bool                    `json:"deterministic"`
	EntryExport      string                  `json:"entry_export"`
	InputSchema      json.RawMessage         `json:"input_schema,omitempty"`
	OutputSchema     json.RawMessage         `json:"output_schema,omitempty"`
	MaxOutputBytes   int64                   `json:"max_output_bytes"`
	MaxHostCalls     int                     `json:"max_host_calls"`
	CallTimeout      time.Duration           `json:"call_timeout"`
	DefinitionVersion int                    `json:"definition_version"`
}

type InvocationResult struct {
	Output      json.RawMessage
	Duration    time.Duration
	FuelUsed    uint64
	HostCalls   int
	MemoryUsed  int64
	TrapMessage string
	Cached      bool
}

type InstanceStats struct {
	InstanceID    string
	ModuleID      string
	State         string
	Invocations   int64
	Traps         int64
	Timeouts      int64
	LastError     string
	LastUsedAt    *time.Time
	MemoryUsed    int64
	FuelUsed      uint64
}

type InstanceState string

const (
	InstanceStateCreated   InstanceState = "created"
	InstanceStateReady     InstanceState = "ready"
	InstanceStateRunning   InstanceState = "running"
	InstanceStateTrapped   InstanceState = "trapped"
	InstanceStateDisposed  InstanceState = "disposed"
)

type WASMError struct {
	Code    WASMErrorCode
	Message string
	Cause   error
}

func (e *WASMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("wasm: %s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("wasm: %s: %s", e.Code, e.Message)
}

func (e *WASMError) Unwrap() error { return e.Cause }

type WASMErrorCode string

const (
	ErrCodeModuleInvalid    WASMErrorCode = "module_invalid"
	ErrCodeABIMismatch      WASMErrorCode = "abi_mismatch"
	ErrCodeImportDenied     WASMErrorCode = "import_denied"
	ErrCodeMemoryLimit      WASMErrorCode = "memory_limit"
	ErrCodeFuelExhausted    WASMErrorCode = "fuel_exhausted"
	ErrCodeTimeout          WASMErrorCode = "timeout"
	ErrCodeTrap             WASMErrorCode = "trap"
	ErrCodeOutputInvalid    WASMErrorCode = "output_invalid"
	ErrCodeHostCallFailed   WASMErrorCode = "host_call_failed"
	ErrCodeCancelled        WASMErrorCode = "cancelled"
	ErrCodeInstanceDisposed WASMErrorCode = "instance_disposed"
	ErrCodeImportUnknown    WASMErrorCode = "import_unknown"
	ErrCodeSchemaInvalid    WASMErrorCode = "schema_invalid"
)

func NewWASMError(code WASMErrorCode, msg string, cause error) *WASMError {
	return &WASMError{Code: code, Message: msg, Cause: cause}
}

var (
	ErrModuleInvalid    = NewWASMError(ErrCodeModuleInvalid, "module invalid", nil)
	ErrABIMismatch      = NewWASMError(ErrCodeABIMismatch, "abi mismatch", nil)
	ErrImportDenied     = NewWASMError(ErrCodeImportDenied, "import denied", nil)
	ErrMemoryLimit      = NewWASMError(ErrCodeMemoryLimit, "memory limit exceeded", nil)
	ErrFuelExhausted    = NewWASMError(ErrCodeFuelExhausted, "fuel exhausted", nil)
	ErrTimeout          = NewWASMError(ErrCodeTimeout, "timeout", nil)
	ErrTrap             = NewWASMError(ErrCodeTrap, "trap", nil)
	ErrOutputInvalid    = NewWASMError(ErrCodeOutputInvalid, "output invalid", nil)
	ErrHostCallFailed   = NewWASMError(ErrCodeHostCallFailed, "host call failed", nil)
	ErrCancelled        = NewWASMError(ErrCodeCancelled, "cancelled", nil)
	ErrInstanceDisposed = NewWASMError(ErrCodeInstanceDisposed, "instance disposed", nil)
	ErrImportUnknown    = NewWASMError(ErrCodeImportUnknown, "import unknown", nil)
	ErrSchemaInvalid    = NewWASMError(ErrCodeSchemaInvalid, "schema invalid", nil)
)

var (
	ErrDefinitionRequired = errors.New("wasm_runtime: definition required")
	ErrModuleIDRequired   = errors.New("wasm_runtime: module_id required")
	ErrEngineNotFound     = errors.New("wasm_runtime: engine not found")
)

func ValidateDefinition(def *WASMRuntimeDefinition) error {
	if def == nil {
		return ErrDefinitionRequired
	}
	if def.ModuleID == "" {
		return ErrModuleIDRequired
	}
	if def.ModulePath == "" {
		return NewWASMError(ErrCodeModuleInvalid, "missing module path", nil)
	}
	if def.ModuleHash == "" {
		return NewWASMError(ErrCodeModuleInvalid, "missing module hash", nil)
	}
	if def.MemoryLimitBytes <= 0 {
		return NewWASMError(ErrCodeModuleInvalid, "memory limit must be > 0", nil)
	}
	if def.MemoryLimitBytes > 1<<30 {
		return NewWASMError(ErrCodeModuleInvalid, "memory limit exceeds 1GB", nil)
	}
	if def.FuelLimit == 0 {
		return NewWASMError(ErrCodeModuleInvalid, "fuel limit must be > 0", nil)
	}
	if def.EntryExport == "" {
		return NewWASMError(ErrCodeABIMismatch, "entry export required", nil)
	}
	if def.ABI == "" {
		return NewWASMError(ErrCodeABIMismatch, "abi kind required", nil)
	}
	if def.MaxOutputBytes <= 0 {
		return NewWASMError(ErrCodeModuleInvalid, "max output bytes must be > 0", nil)
	}
	if def.WASIVersion != WASINone && def.WASIVersion != WASIV1 {
		return NewWASMError(ErrCodeModuleInvalid, "unsupported wasi version", nil)
	}
	switch def.InstancePolicy {
	case InstancePolicyPerInvocation, InstancePolicyPooled, InstancePolicySingleton:
	default:
		return NewWASMError(ErrCodeModuleInvalid, "invalid instance policy: "+string(def.InstancePolicy), nil)
	}
	if def.Deterministic {
		for _, imp := range def.AllowedImports {
			if imp == ImportRandom || imp == ImportTime {
				return NewWASMError(ErrCodeImportDenied, "deterministic mode forbids time/random imports", nil)
			}
		}
	}
	return nil
}

type HostCallContext struct {
	InstanceID   string
	InvocationID string
	ModuleID     string
	ExtensionID  string
	Remaining    int
	CancelSignal context.Context
}

type HostCallHandler func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error)

type HostImportRegistry struct {
	mu       sync.RWMutex
	handlers map[HostImportName]HostCallHandler
}

func NewHostImportRegistry() *HostImportRegistry {
	return &HostImportRegistry{handlers: make(map[HostImportName]HostCallHandler)}
}

func (r *HostImportRegistry) Register(name HostImportName, h HostCallHandler) {
	r.mu.Lock()
	r.handlers[name] = h
	r.mu.Unlock()
}

func (r *HostImportRegistry) Lookup(name HostImportName) (HostCallHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

func (r *HostImportRegistry) Allowed(allowed []HostImportName, name HostImportName) bool {
	for _, a := range allowed {
		if a == name {
			return true
		}
	}
	return false
}

type Engine interface {
	Name() string
	Version() string
	Compile(ctx context.Context, moduleBytes []byte) (CompiledModule, error)
	Instantiate(ctx context.Context, compiled CompiledModule, opts InstantiateOptions) (Instance, error)
}

type CompiledModule interface {
	Hash() string
	Exports() []string
	Imports() []string
	Dispose()
}

type InstantiateOptions struct {
	MemoryLimit int64
	FuelLimit   uint64
	AllowedImports []HostImportName
	HostImports *HostImportRegistry
}

type Instance interface {
	Invoke(ctx context.Context, export string, input []byte, opts InvokeOptions) (*RawResult, error)
	State() InstanceState
	Stats() InstanceStats
	Dispose() error
}

type InvokeOptions struct {
	FuelLimit    uint64
	MemoryLimit  int64
	Timeout      time.Duration
	MaxHostCalls int
}

type RawResult struct {
	Output      []byte
	FuelUsed    uint64
	HostCalls   int
	MemoryUsed  int64
	TrapMessage string
	Duration    time.Duration
}
