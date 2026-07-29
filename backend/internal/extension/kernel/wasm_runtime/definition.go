package wasm_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type InstancePolicy string

const (
	InstancePolicyPerInvocation InstancePolicy = "per_invocation"
	InstancePolicyPooled        InstancePolicy = "pooled"
	InstancePolicySingleton     InstancePolicy = "singleton_per_module"
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
	ImportLog            HostImportName = "amitia.log"
	ImportTime           HostImportName = "amitia.time"
	ImportRandom         HostImportName = "amitia.random"
	ImportStorageGet     HostImportName = "amitia.storage_get"
	ImportStorageCAS     HostImportName = "amitia.storage_cas"
	ImportResourceRead   HostImportName = "amitia.resource_read"
	ImportArtifactWrite  HostImportName = "amitia.artifact_write"
	ImportToolInvoke     HostImportName = "amitia.tool_invoke"
	ImportResultSetError HostImportName = "amitia.result_set_error"
)

var DefaultAllowedImports = []HostImportName{
	ImportLog,
	ImportTime,
}

var FullAllowedImports = []HostImportName{
	ImportLog,
	ImportTime,
	ImportRandom,
	ImportStorageGet,
	ImportStorageCAS,
	ImportResourceRead,
	ImportArtifactWrite,
	ImportToolInvoke,
	ImportResultSetError,
}

var ForbiddenHostFunctions = map[string]bool{
	"filesystem_raw":   true,
	"network_raw":      true,
	"socket_open":      true,
	"process_spawn":    true,
	"shell_execute":    true,
	"electron_ipc":     true,
	"database_query":   true,
	"secret_read_raw":  true,
	"memory_raw":       true,
	"message_send_raw": true,
	"desktop_control":  true,
	"clipboard_raw":    true,
}

type WasmEntryDefinition struct {
	ExportName   string          `json:"export_name"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}

type WasmABIDefinition struct {
	Version       string `json:"version"`
	AllocExport   string `json:"alloc_export"`
	DeallocExport string `json:"dealloc_export"`
	InvokeExport  string `json:"invoke_export"`
}

type WasmImportRequirement struct {
	ModuleName   string `json:"module_name"`
	FunctionName string `json:"function_name"`
}

type WasmExportDefinition struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type WASMRuntimeDefinition struct {
	RuntimeDefinitionID string `json:"runtime_definition_id"`
	ModuleID            string `json:"module_id"`
	ExtensionID         string `json:"extension_id"`
	ModulePath          string `json:"module_path"`
	ModuleHash          string `json:"module_hash"`
	ModuleSHA256        string `json:"module_sha256"`
	InterfacePath       string `json:"interface_path,omitempty"`
	InterfaceHash       string `json:"interface_hash,omitempty"`

	EngineType string              `json:"engine_type"`
	Entry      WasmEntryDefinition `json:"entry"`

	ABI            ABIKind                 `json:"abi"`
	ABIDef         WasmABIDefinition       `json:"abi_def,omitempty"`
	WASIVersion    WASIVersion             `json:"wasi_version"`
	Imports        []WasmImportRequirement `json:"imports,omitempty"`
	Exports        []WasmExportDefinition  `json:"exports,omitempty"`
	AllowedImports []HostImportName        `json:"allowed_imports"`

	Limits            WasmResourceLimits    `json:"limits"`
	MemoryLimitBytes  int64                 `json:"memory_limit_bytes"`
	FuelLimit         uint64                `json:"fuel_limit"`
	InstancePolicy    InstancePolicy        `json:"instance_policy"`
	DeterminismPolicy WasmDeterminismPolicy `json:"determinism_policy"`
	Deterministic     bool                  `json:"deterministic"`

	EntryExport    string          `json:"entry_export"`
	InputSchema    json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema   json.RawMessage `json:"output_schema,omitempty"`
	MaxOutputBytes int64           `json:"max_output_bytes"`
	MaxHostCalls   int             `json:"max_host_calls"`
	CallTimeout    time.Duration   `json:"call_timeout"`

	PermissionRequirements []string `json:"permission_requirements,omitempty"`
	ScopeRule              string   `json:"scope_rule,omitempty"`

	DefinitionHash    string `json:"definition_hash"`
	DefinitionVersion int    `json:"definition_version"`
	Version           string `json:"version"`
	Generation        int64  `json:"generation"`
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
	InstanceID  string
	ModuleID    string
	State       string
	Invocations int64
	Traps       int64
	Timeouts    int64
	LastError   string
	LastUsedAt  *time.Time
	MemoryUsed  int64
	FuelUsed    uint64
}

type InstanceState string

const (
	InstanceStateCreated  InstanceState = "created"
	InstanceStateReady    InstanceState = "ready"
	InstanceStateRunning  InstanceState = "running"
	InstanceStateTrapped  InstanceState = "trapped"
	InstanceStateDisposed InstanceState = "disposed"
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
	ErrCodeModuleInvalid      WASMErrorCode = "wasm_module_invalid"
	ErrCodeModuleMissing      WASMErrorCode = "wasm_module_missing"
	ErrCodeIntegrityFailed    WASMErrorCode = "wasm_module_integrity_failed"
	ErrCodeFeatureUnsupported WASMErrorCode = "wasm_feature_unsupported"
	ErrCodeImportNotAllowed   WASMErrorCode = "wasm_import_not_allowed"
	ErrCodeExportMissing      WASMErrorCode = "wasm_export_missing"
	ErrCodeABIMismatch        WASMErrorCode = "wasm_abi_incompatible"
	ErrCodeCompileFailed      WASMErrorCode = "wasm_compile_failed"
	ErrCodeInstantiateFailed  WASMErrorCode = "wasm_instantiate_failed"
	ErrCodeMemoryLimit        WASMErrorCode = "wasm_memory_limit_exceeded"
	ErrCodeMemoryAccess       WASMErrorCode = "wasm_memory_access_invalid"
	ErrCodeFuelExhausted      WASMErrorCode = "wasm_fuel_exhausted"
	ErrCodeTimeout            WASMErrorCode = "wasm_execution_timeout"
	ErrCodeCancelled          WASMErrorCode = "wasm_execution_cancelled"
	ErrCodeTrap               WASMErrorCode = "wasm_trap"
	ErrCodeOutputInvalid      WASMErrorCode = "wasm_output_invalid"
	ErrCodeOutputTooLarge     WASMErrorCode = "wasm_output_too_large"
	ErrCodeHostCallLimit      WASMErrorCode = "wasm_host_call_limit_exceeded"
	ErrCodeHostFunctionDenied WASMErrorCode = "wasm_host_function_denied"
	ErrCodePermissionDenied   WASMErrorCode = "wasm_permission_denied"
	ErrCodeScopeDenied        WASMErrorCode = "wasm_scope_denied"
	ErrCodeRecursionDetected  WASMErrorCode = "wasm_recursion_detected"
	ErrCodeDepthExceeded      WASMErrorCode = "wasm_depth_exceeded"
	ErrCodeCircuitOpen        WASMErrorCode = "wasm_runtime_circuit_open"
	ErrCodeQuarantined        WASMErrorCode = "wasm_runtime_quarantined"

	ErrCodeHostCallFailed   WASMErrorCode = "wasm_host_call_failed"
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
	ErrImportDenied     = NewWASMError(ErrCodeImportNotAllowed, "import denied", nil)
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

func NormalizeDefinition(def *WASMRuntimeDefinition) {
	if def == nil {
		return
	}
	if def.ModuleSHA256 == "" {
		def.ModuleSHA256 = def.ModuleHash
	}
	if def.MemoryLimitBytes <= 0 && def.Limits.MaxMemoryPages > 0 {
		def.MemoryLimitBytes = int64(def.Limits.MaxMemoryPages) * WasmPageSize
	}
	if def.FuelLimit == 0 {
		def.FuelLimit = def.Limits.Fuel
	}
	if def.EntryExport == "" {
		def.EntryExport = def.Entry.ExportName
	}
	if def.MaxOutputBytes <= 0 && def.Limits.MaxOutputBytes > 0 {
		def.MaxOutputBytes = def.Limits.MaxOutputBytes
	}
	if def.MaxHostCalls <= 0 {
		if def.Limits.MaxHostCalls > 0 {
			def.MaxHostCalls = def.Limits.MaxHostCalls
		} else {
			def.MaxHostCalls = 128
		}
	}
	if def.CallTimeout <= 0 {
		if def.Limits.MaxExecutionDuration > 0 {
			def.CallTimeout = def.Limits.MaxExecutionDuration
		} else {
			def.CallTimeout = 5 * time.Second
		}
	}
}

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
	if err := ValidateModulePath(def.ModulePath); err != nil {
		return err
	}
	if def.ModuleHash == "" && def.ModuleSHA256 == "" {
		return NewWASMError(ErrCodeModuleInvalid, "missing module hash", nil)
	}
	memLimit := def.MemoryLimitBytes
	if memLimit <= 0 && def.Limits.MaxMemoryPages > 0 {
		memLimit = int64(def.Limits.MaxMemoryPages) * WasmPageSize
	}
	if memLimit <= 0 {
		return NewWASMError(ErrCodeMemoryLimit, "memory limit must be > 0", nil)
	}
	if memLimit > 1<<30 {
		return NewWASMError(ErrCodeMemoryLimit, "memory limit exceeds 1GB", nil)
	}
	fuel := def.FuelLimit
	if fuel == 0 {
		fuel = def.Limits.Fuel
	}
	if fuel == 0 {
		return NewWASMError(ErrCodeFuelExhausted, "fuel limit must be > 0", nil)
	}
	entryExport := def.EntryExport
	if entryExport == "" {
		entryExport = def.Entry.ExportName
	}
	if entryExport == "" {
		return NewWASMError(ErrCodeABIMismatch, "entry export required", nil)
	}
	if def.ABI == "" {
		return NewWASMError(ErrCodeABIMismatch, "abi kind required", nil)
	}
	maxOutput := def.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = def.Limits.MaxOutputBytes
	}
	if maxOutput <= 0 {
		return NewWASMError(ErrCodeOutputTooLarge, "max output bytes must be > 0", nil)
	}
	if def.WASIVersion != WASINone && def.WASIVersion != WASIV1 {
		return NewWASMError(ErrCodeFeatureUnsupported, "unsupported wasi version", nil)
	}
	if def.WASIVersion == WASIV1 {
		return NewWASMError(ErrCodeFeatureUnsupported, "wasi not supported in v1, use host functions instead", nil)
	}
	if err := ValidateInstancePolicy(def.InstancePolicy); err != nil {
		return err
	}
	if def.Deterministic {
		policy := DeterministicPolicy()
		for _, imp := range def.AllowedImports {
			if !policy.AllowsImport(imp) {
				return NewWASMError(ErrCodeImportNotAllowed, "deterministic mode forbids time/random/host imports: "+string(imp), nil)
			}
		}
	}
	for _, imp := range def.AllowedImports {
		if ForbiddenHostFunctions[string(imp)] {
			return NewWASMError(ErrCodeHostFunctionDenied, "forbidden host function: "+string(imp), nil)
		}
	}
	return nil
}

func ValidateModulePath(path string) error {
	if path == "" {
		return NewWASMError(ErrCodeModuleInvalid, "empty module path", nil)
	}
	if len(path) > 0 && (path[0] == '/' || (len(path) > 2 && path[1] == ':')) {
		return NewWASMError(ErrCodeModuleInvalid, "absolute path not allowed: "+path, nil)
	}
	if strings.Contains(path, "..") {
		return NewWASMError(ErrCodeModuleInvalid, "path traversal not allowed: "+path, nil)
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
	MemoryLimit    int64
	FuelLimit      uint64
	AllowedImports []HostImportName
	HostImports    *HostImportRegistry
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
