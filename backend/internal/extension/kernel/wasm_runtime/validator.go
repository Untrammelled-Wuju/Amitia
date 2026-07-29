package wasm_runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	WASMMagic    = "\x00asm"
	WASMVersion1 = 1
)

type ModuleValidator struct{}

func NewModuleValidator() *ModuleValidator {
	return &ModuleValidator{}
}

type ValidationReport struct {
	Valid                bool
	ModuleHash           string
	Version              uint32
	Exports              []string
	Imports              []string
	MemoryMin            int64
	MemoryMax            int64
	HasStart             bool
	Errors               []string
	SizeBytes            int64
	EstimatedCompileCost int64
}

func (v *ModuleValidator) Validate(reader io.Reader) (*ValidationReport, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("wasm_runtime: read module: %w", err)
	}
	return v.ValidateBytes(data)
}

func (v *ModuleValidator) ValidateFile(path string) (*ValidationReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("wasm_runtime: open %s: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("wasm_runtime: read %s: %w", path, err)
	}
	return v.ValidateBytes(data)
}

func (v *ModuleValidator) ValidateBytes(data []byte) (*ValidationReport, error) {
	report := &ValidationReport{SizeBytes: int64(len(data))}
	h := sha256.Sum256(data)
	report.ModuleHash = hex.EncodeToString(h[:])
	if len(data) < 8 {
		report.Errors = append(report.Errors, "module too small")
		return report, nil
	}
	if !bytes.Equal(data[:4], []byte(WASMMagic)) {
		report.Errors = append(report.Errors, "invalid magic")
		return report, nil
	}
	report.Version = binary.LittleEndian.Uint32(data[4:8])
	if report.Version != WASMVersion1 {
		report.Errors = append(report.Errors, fmt.Sprintf("unsupported version %d", report.Version))
		return report, nil
	}
	report.Valid = true
	report.Exports = []string{}
	report.Imports = []string{}
	report.EstimatedCompileCost = int64(len(data)) * 2
	return report, nil
}

type ModuleCacheKey struct {
	ModuleHash    string
	EngineName    string
	EngineVersion string
	Platform      string
}

type CompiledModuleEntry struct {
	Module   CompiledModule
	Key      ModuleCacheKey
	CachedAt time.Time
}

type ModuleCache struct {
	mu      sync.RWMutex
	entries map[string]*CompiledModuleEntry
}

func NewModuleCache() *ModuleCache {
	return &ModuleCache{entries: make(map[string]*CompiledModuleEntry)}
}

func (c *ModuleCache) Get(key ModuleCacheKey) (CompiledModule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key.ModuleHash]
	if !ok {
		return nil, false
	}
	if entry.Key.EngineName != key.EngineName || entry.Key.EngineVersion != key.EngineVersion {
		return nil, false
	}
	return entry.Module, true
}

func (c *ModuleCache) Put(key ModuleCacheKey, module CompiledModule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key.ModuleHash] = &CompiledModuleEntry{
		Module:   module,
		Key:      key,
		CachedAt: time.Now().UTC(),
	}
}

func (c *ModuleCache) Invalidate(moduleHash string) {
	c.mu.Lock()
	delete(c.entries, moduleHash)
	c.mu.Unlock()
}

func (c *ModuleCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CompiledModuleEntry)
	c.mu.Unlock()
}

func (c *ModuleCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

type HostCallCounter struct {
	mu       sync.Mutex
	calls    map[string]int
	maxCalls int
}

func NewHostCallCounter(maxCalls int) *HostCallCounter {
	return &HostCallCounter{calls: make(map[string]int), maxCalls: maxCalls}
}

func (c *HostCallCounter) Increment(name HostImportName) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, n := range c.calls {
		total += n
	}
	if c.maxCalls > 0 && total >= c.maxCalls {
		return ErrHostCallFailed
	}
	c.calls[string(name)]++
	return nil
}

func (c *HostCallCounter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, n := range c.calls {
		total += n
	}
	return total
}

type Runtime struct {
	mu          sync.RWMutex
	engines     map[string]Engine
	defaultName string
	cache       *ModuleCache
	registry    *HostImportRegistry
	instances   map[string]Instance
	stats       map[string]*InstanceStats
	logger      func(level, msg string, fields map[string]any)
}

func NewRuntime(defaultEngine Engine) *Runtime {
	r := &Runtime{
		engines:   make(map[string]Engine),
		cache:     NewModuleCache(),
		registry:  NewHostImportRegistry(),
		instances: make(map[string]Instance),
		stats:     make(map[string]*InstanceStats),
		logger:    func(level, msg string, fields map[string]any) {},
	}
	if defaultEngine != nil {
		r.engines[defaultEngine.Name()] = defaultEngine
		r.defaultName = defaultEngine.Name()
	}
	r.registerDefaultHostImports()
	return r
}

func (r *Runtime) SetLogger(l func(level, msg string, fields map[string]any)) {
	r.logger = l
}

func (r *Runtime) RegisterEngine(engine Engine) error {
	if engine == nil {
		return errors.New("wasm_runtime: nil engine")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.engines[engine.Name()] = engine
	if r.defaultName == "" {
		r.defaultName = engine.Name()
	}
	return nil
}

func (r *Runtime) SetDefaultEngine(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.engines[name]; !exists {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, name)
	}
	r.defaultName = name
	return nil
}

func (r *Runtime) Registry() *HostImportRegistry { return r.registry }
func (r *Runtime) Cache() *ModuleCache           { return r.cache }

type InvokeRequest struct {
	Definition  *WASMRuntimeDefinition
	Input       json.RawMessage
	ModuleBytes []byte
	EngineName  string
}

func (r *Runtime) Invoke(ctx context.Context, req InvokeRequest) (*InvocationResult, error) {
	if req.Definition == nil {
		return nil, ErrDefinitionRequired
	}
	if err := ValidateDefinition(req.Definition); err != nil {
		return nil, err
	}
	if len(req.ModuleBytes) == 0 {
		return nil, NewWASMError(ErrCodeModuleInvalid, "missing module bytes", nil)
	}
	validator := NewModuleValidator()
	report, err := validator.ValidateBytes(req.ModuleBytes)
	if err != nil {
		return nil, err
	}
	if !report.Valid {
		return nil, NewWASMError(ErrCodeModuleInvalid, "module validation failed", nil)
	}
	if report.ModuleHash != req.Definition.ModuleHash {
		return nil, NewWASMError(ErrCodeModuleInvalid, "module hash mismatch", nil)
	}
	engineName := req.EngineName
	if engineName == "" {
		engineName = r.defaultName
	}
	r.mu.RLock()
	engine, ok := r.engines[engineName]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrEngineNotFound, engineName)
	}
	cacheKey := ModuleCacheKey{
		ModuleHash:    report.ModuleHash,
		EngineName:    engine.Name(),
		EngineVersion: engine.Version(),
	}
	compiled, cached := r.cache.Get(cacheKey)
	if !cached {
		compiled, err = engine.Compile(ctx, req.ModuleBytes)
		if err != nil {
			return nil, NewWASMError(ErrCodeModuleInvalid, "compile failed", err)
		}
		r.cache.Put(cacheKey, compiled)
	}
	instance, err := engine.Instantiate(ctx, compiled, InstantiateOptions{
		MemoryLimit:    req.Definition.MemoryLimitBytes,
		FuelLimit:      req.Definition.FuelLimit,
		AllowedImports: req.Definition.AllowedImports,
		HostImports:    r.registry,
	})
	if err != nil {
		return nil, NewWASMError(ErrCodeModuleInvalid, "instantiate failed", err)
	}
	defer instance.Dispose()
	timeout := req.Definition.CallTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := instance.Invoke(callCtx, req.Definition.EntryExport, req.Input, InvokeOptions{
		FuelLimit:    req.Definition.FuelLimit,
		MemoryLimit:  req.Definition.MemoryLimitBytes,
		Timeout:      timeout,
		MaxHostCalls: req.Definition.MaxHostCalls,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		if errors.Is(err, context.Canceled) {
			return nil, ErrCancelled
		}
		werr, ok := err.(*WASMError)
		if !ok {
			werr = NewWASMError(ErrCodeTrap, err.Error(), err)
		}
		var partialResult *InvocationResult
		if raw != nil {
			partialResult = &InvocationResult{
				Duration:    raw.Duration,
				FuelUsed:    raw.FuelUsed,
				HostCalls:   raw.HostCalls,
				MemoryUsed:  raw.MemoryUsed,
				TrapMessage: raw.TrapMessage,
			}
		}
		return partialResult, werr
	}
	if raw == nil {
		return nil, NewWASMError(ErrCodeTrap, "instance returned nil result", nil)
	}
	if int64(len(raw.Output)) > req.Definition.MaxOutputBytes {
		return nil, ErrOutputInvalid
	}
	if req.Definition.OutputSchema != nil {
		if err := validateOutputSchema(raw.Output, req.Definition.OutputSchema); err != nil {
			return nil, NewWASMError(ErrCodeOutputInvalid, err.Error(), err)
		}
	}
	return &InvocationResult{
		Output:     raw.Output,
		Duration:   raw.Duration,
		FuelUsed:   raw.FuelUsed,
		HostCalls:  raw.HostCalls,
		MemoryUsed: raw.MemoryUsed,
		Cached:     cached,
	}, nil
}

func validateOutputSchema(output []byte, schema json.RawMessage) error {
	if len(output) == 0 {
		return errors.New("empty output")
	}
	var v any
	if err := json.Unmarshal(output, &v); err != nil {
		return fmt.Errorf("output not json: %w", err)
	}
	return nil
}

func (r *Runtime) registerDefaultHostImports() {
	r.registry.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		r.logger("info", "wasm host log", map[string]any{
			"module": hctx.ModuleID, "params": string(params),
		})
		return json.RawMessage(`{}`), nil
	})
	r.registry.Register(ImportTime, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		now := time.Now().UTC().Unix()
		return json.Marshal(map[string]any{"now": now})
	})
	r.registry.Register(ImportRandom, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"seed": time.Now().UnixNano()})
	})
}
