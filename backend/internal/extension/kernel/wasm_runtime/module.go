package wasm_runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ModuleManager struct {
	mu        sync.RWMutex
	basePath  string
	validator *ModuleValidator
	cache     *ModuleCache
	loaded    map[string]*LoadedModule
}

type LoadedModule struct {
	ModuleID    string
	Path        string
	Hash        string
	SHA256      string
	Bytes       []byte
	Report      *ValidationReport
	Size        int64
	LoadedAt    int64
}

func NewModuleManager(basePath string) *ModuleManager {
	return &ModuleManager{
		basePath:  basePath,
		validator: NewModuleValidator(),
		cache:     NewModuleCache(),
		loaded:    make(map[string]*LoadedModule),
	}
}

func (m *ModuleManager) LoadFromPath(moduleID, relPath string) (*LoadedModule, error) {
	if moduleID == "" {
		return nil, NewWASMError(ErrCodeModuleInvalid, "module_id required", nil)
	}
	if err := ValidateModulePath(relPath); err != nil {
		return nil, err
	}

	absPath := relPath
	if m.basePath != "" && !filepath.IsAbs(relPath) {
		absPath = filepath.Join(m.basePath, relPath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, NewWASMError(ErrCodeModuleMissing, fmt.Sprintf("read module file: %v", err), err)
	}

	report, err := m.validator.ValidateBytes(data)
	if err != nil {
		return nil, NewWASMError(ErrCodeModuleInvalid, fmt.Sprintf("validate module: %v", err), err)
	}
	if !report.Valid {
		return nil, NewWASMError(ErrCodeModuleInvalid, fmt.Sprintf("module validation failed: %v", report.Errors), nil)
	}

	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	loaded := &LoadedModule{
		ModuleID: moduleID,
		Path:     relPath,
		Hash:     hash,
		SHA256:   hash,
		Bytes:    data,
		Report:   report,
		Size:     int64(len(data)),
		LoadedAt: timeNowUnix(),
	}

	m.mu.Lock()
	m.loaded[moduleID] = loaded
	m.mu.Unlock()

	return loaded, nil
}

func (m *ModuleManager) LoadFromBytes(moduleID string, data []byte) (*LoadedModule, error) {
	if moduleID == "" {
		return nil, NewWASMError(ErrCodeModuleInvalid, "module_id required", nil)
	}
	if len(data) == 0 {
		return nil, NewWASMError(ErrCodeModuleMissing, "empty module bytes", nil)
	}

	report, err := m.validator.ValidateBytes(data)
	if err != nil {
		return nil, NewWASMError(ErrCodeModuleInvalid, fmt.Sprintf("validate module: %v", err), err)
	}
	if !report.Valid {
		return nil, NewWASMError(ErrCodeModuleInvalid, fmt.Sprintf("module validation failed: %v", report.Errors), nil)
	}

	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	loaded := &LoadedModule{
		ModuleID: moduleID,
		Path:     "",
		Hash:     hash,
		SHA256:   hash,
		Bytes:    data,
		Report:   report,
		Size:     int64(len(data)),
		LoadedAt: timeNowUnix(),
	}

	m.mu.Lock()
	m.loaded[moduleID] = loaded
	m.mu.Unlock()

	return loaded, nil
}

func (m *ModuleManager) Get(moduleID string) (*LoadedModule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mod, ok := m.loaded[moduleID]
	return mod, ok
}

func (m *ModuleManager) VerifyHash(moduleID, expectedHash string) error {
	m.mu.RLock()
	mod, ok := m.loaded[moduleID]
	m.mu.RUnlock()
	if !ok {
		return NewWASMError(ErrCodeModuleMissing, fmt.Sprintf("module not loaded: %s", moduleID), nil)
	}
	if mod.Hash != expectedHash {
		return NewWASMError(ErrCodeIntegrityFailed, fmt.Sprintf("hash mismatch: expected=%s actual=%s", expectedHash, mod.Hash), nil)
	}
	return nil
}

func (m *ModuleManager) Unload(moduleID string) {
	m.mu.Lock()
	delete(m.loaded, moduleID)
	m.mu.Unlock()
}

func (m *ModuleManager) List() []*LoadedModule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*LoadedModule, 0, len(m.loaded))
	for _, mod := range m.loaded {
		out = append(out, mod)
	}
	return out
}

func (m *ModuleManager) Cache() *ModuleCache {
	return m.cache
}

func (m *ModuleManager) Validator() *ModuleValidator {
	return m.validator
}

func (m *ModuleManager) ValidateDefinition(def *WASMRuntimeDefinition) error {
	NormalizeDefinition(def)
	if err := ValidateDefinition(def); err != nil {
		return err
	}
	if def.ModuleHash != "" {
		mod, ok := m.Get(def.ModuleID)
		if ok {
			if err := m.VerifyHash(def.ModuleID, def.ModuleHash); err != nil {
				return err
			}
			_ = mod
		}
	}
	if len(def.AllowedImports) > 0 {
		for _, imp := range def.AllowedImports {
			if ForbiddenHostFunctions[string(imp)] {
				return NewWASMError(ErrCodeHostFunctionDenied, "forbidden host function: "+string(imp), nil)
			}
		}
	}
	return nil
}

func (m *ModuleManager) BuildDefinition(params BuildDefinitionParams) (*WASMRuntimeDefinition, error) {
	mod, ok := m.Get(params.ModuleID)
	if !ok {
		return nil, NewWASMError(ErrCodeModuleMissing, fmt.Sprintf("module not loaded: %s", params.ModuleID), nil)
	}

	limits := DefaultWasmResourceLimits()
	if params.Limits.MaxMemoryPages > 0 {
		limits.MaxMemoryPages = params.Limits.MaxMemoryPages
	}
	if params.Limits.Fuel > 0 {
		limits.Fuel = params.Limits.Fuel
	}
	if params.Limits.MaxExecutionDuration > 0 {
		limits.MaxExecutionDuration = params.Limits.MaxExecutionDuration
	}
	if params.Limits.MaxHostCalls > 0 {
		limits.MaxHostCalls = params.Limits.MaxHostCalls
	}
	if params.Limits.MaxOutputBytes > 0 {
		limits.MaxOutputBytes = params.Limits.MaxOutputBytes
	}

	allowed := DefaultAllowedImports
	if len(params.AllowedImports) > 0 {
		allowed = params.AllowedImports
	}

	memLimitBytes := int64(limits.MaxMemoryPages) * WasmPageSize
	fuelLimit := limits.Fuel
	if fuelLimit == 0 {
		fuelLimit = 1000000
	}

	def := &WASMRuntimeDefinition{
		RuntimeDefinitionID: params.RuntimeDefinitionID,
		ModuleID:            params.ModuleID,
		ExtensionID:         params.ExtensionID,
		ModulePath:          mod.Path,
		ModuleHash:          mod.Hash,
		ModuleSHA256:        mod.SHA256,
		EngineType:          WazeroEngineName,
		ABI:                 ABIAmitia,
		AllowedImports:      allowed,
		Limits:              limits,
		MemoryLimitBytes:    memLimitBytes,
		FuelLimit:           fuelLimit,
		InstancePolicy:      InstancePolicyPerInvocation,
		EntryExport:         ExportAmitiaInvoke,
		MaxOutputBytes:      limits.MaxOutputBytes,
		MaxHostCalls:        limits.MaxHostCalls,
		CallTimeout:         limits.MaxExecutionDuration,
		DefinitionVersion:   1,
		Generation:          1,
	}

	if params.Deterministic {
		def.Deterministic = true
		def.AllowedImports = filterDeterministicImports(allowed)
	}

	if params.EntryExport != "" {
		def.EntryExport = params.EntryExport
	}

	if err := m.ValidateDefinition(def); err != nil {
		return nil, err
	}

	return def, nil
}

type BuildDefinitionParams struct {
	RuntimeDefinitionID string
	ModuleID            string
	ExtensionID         string
	AllowedImports      []HostImportName
	Limits              WasmResourceLimits
	EntryExport         string
	Deterministic       bool
}

func filterDeterministicImports(imports []HostImportName) []HostImportName {
	policy := DeterministicPolicy()
	out := make([]HostImportName, 0, len(imports))
	for _, imp := range imports {
		if policy.AllowsImport(imp) {
			out = append(out, imp)
		}
	}
	return out
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
