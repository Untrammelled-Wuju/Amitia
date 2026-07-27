package wasm_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	WazeroEngineName    = "wazero"
	WazeroEngineVersion = "1.12.0"
)

type invocationCtxKey struct{}

type InvocationContext struct {
	InvocationID         string
	OperationID          string
	ExtensionID          string
	ModuleID             string
	ContributionID       string
	RuntimeInstanceID    string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	Deadline             time.Time
	HostCallBudget       int
	HostCallsMade        int
	TraceID              string
	AllowedImports       []HostImportName
	DeterminismPolicy    WasmDeterminismPolicy
	Registry             *HostImportRegistry
	MaxHostCalls         int
	Depth                int
}

func WithInvocationContext(ctx context.Context, ic *InvocationContext) context.Context {
	return context.WithValue(ctx, invocationCtxKey{}, ic)
}

func GetInvocationContext(ctx context.Context) *InvocationContext {
	ic, _ := ctx.Value(invocationCtxKey{}).(*InvocationContext)
	return ic
}

type WazeroEngine struct {
	mu          sync.RWMutex
	compileRT   wazero.Runtime
	compileOnce sync.Once
	initErr     error
}

func NewWazeroEngine() *WazeroEngine {
	return &WazeroEngine{}
}

func (e *WazeroEngine) Name() string    { return WazeroEngineName }
func (e *WazeroEngine) Version() string { return WazeroEngineVersion }

func (e *WazeroEngine) ensureCompileRT(ctx context.Context) error {
	e.compileOnce.Do(func() {
		e.compileRT = wazero.NewRuntime(ctx)
	})
	return e.initErr
}

func (e *WazeroEngine) Compile(ctx context.Context, moduleBytes []byte) (CompiledModule, error) {
	if err := e.ensureCompileRT(ctx); err != nil {
		return nil, err
	}
	if len(moduleBytes) == 0 {
		return nil, NewWASMError(ErrCodeModuleMissing, "empty module bytes", nil)
	}
	if len(moduleBytes) < 8 {
		return nil, NewWASMError(ErrCodeModuleInvalid, "module too small", nil)
	}
	h := sha256.Sum256(moduleBytes)
	hash := hex.EncodeToString(h[:])
	e.mu.RLock()
	rt := e.compileRT
	e.mu.RUnlock()
	if rt == nil {
		return nil, NewWASMError(ErrCodeCompileFailed, "compile runtime not initialized", nil)
	}
	compiled, err := rt.CompileModule(ctx, moduleBytes)
	if err != nil {
		return nil, NewWASMError(ErrCodeCompileFailed, fmt.Sprintf("compile failed: %v", err), err)
	}
	exports := make([]string, 0, len(compiled.ExportedFunctions()))
	for name := range compiled.ExportedFunctions() {
		exports = append(exports, name)
	}
	imports := make([]string, 0, len(compiled.ImportedFunctions()))
	for _, def := range compiled.ImportedFunctions() {
		imports = append(imports, fmt.Sprintf("%s.%s", def.ModuleName(), def.Name()))
	}
	return &wazeroCompiledModule{
		hash:      hash,
		exports:   exports,
		imports:   imports,
		compiled:  compiled,
		bytes:     moduleBytes,
	}, nil
}

func (e *WazeroEngine) Instantiate(ctx context.Context, compiled CompiledModule, opts InstantiateOptions) (Instance, error) {
	wcm, ok := compiled.(*wazeroCompiledModule)
	if !ok {
		return nil, NewWASMError(ErrCodeInstantiateFailed, "invalid compiled module type", nil)
	}
	memPages := MemoryPagesFromBytes(opts.MemoryLimit)
	if memPages == 0 {
		memPages = 1024
	}
	rtCfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(memPages)
	rt := wazero.NewRuntimeWithConfig(ctx, rtCfg)
	hostModuleBuilder := rt.NewHostModuleBuilder(HostModuleAmitia)
	registerHostFunctions(hostModuleBuilder, opts.HostImports, opts.AllowedImports)
	if _, err := hostModuleBuilder.Instantiate(ctx); err != nil {
		rt.Close(ctx)
		return nil, NewWASMError(ErrCodeInstantiateFailed, fmt.Sprintf("instantiate host module: %v", err), err)
	}
	modCfg := wazero.NewModuleConfig().WithStartFunctions()
	mod, err := rt.InstantiateModule(ctx, wcm.compiled, modCfg)
	if err != nil {
		rt.Close(ctx)
		return nil, NewWASMError(ErrCodeInstantiateFailed, fmt.Sprintf("instantiate module: %v", err), err)
	}
	inst := &wazeroInstance{
		id:       fmt.Sprintf("winst-%d", time.Now().UnixNano()),
		module:   mod,
		rt:       rt,
		state:    InstanceStateReady,
		registry: opts.HostImports,
		memPages: memPages,
	}
	inst.stats = InstanceStats{
		InstanceID: inst.id,
		ModuleID:   "",
	}
	return inst, nil
}

func (e *WazeroEngine) Capabilities() EngineCapabilityReport {
	return EngineCapabilityReport{
		InstructionFuel:   false,
		DeadlineCancel:    true,
		MemoryLimit:       true,
		TrapCapture:       true,
		HostFunction:      true,
		WasiDisabled:      true,
		InstanceIsolation: true,
	}
}

func (e *WazeroEngine) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.compileRT != nil {
		return e.compileRT.Close(ctx)
	}
	return nil
}

type wazeroCompiledModule struct {
	hash     string
	exports  []string
	imports  []string
	compiled wazero.CompiledModule
	bytes    []byte
}

func (m *wazeroCompiledModule) Hash() string      { return m.hash }
func (m *wazeroCompiledModule) Exports() []string { return m.exports }
func (m *wazeroCompiledModule) Imports() []string { return m.imports }
func (m *wazeroCompiledModule) Dispose() {
	if m.compiled != nil {
		context.Background()
		m.compiled.Close(context.Background())
	}
}

type wazeroInstance struct {
	mu       sync.Mutex
	id       string
	module   api.Module
	rt       wazero.Runtime
	state    InstanceState
	stats    InstanceStats
	registry *HostImportRegistry
	memPages uint32
	traps    []*TrapInfo
}

func (i *wazeroInstance) Invoke(ctx context.Context, export string, input []byte, opts InvokeOptions) (result *RawResult, err error) {
	i.mu.Lock()
	if i.state == InstanceStateDisposed {
		i.mu.Unlock()
		return nil, ErrInstanceDisposed
	}
	i.state = InstanceStateRunning
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		if r := recover(); r != nil {
			trap := &TrapInfo{
				Kind:    ClassifyPanic(r),
				Message: fmt.Sprintf("%v", r),
			}
			i.traps = append(i.traps, trap)
			i.state = InstanceStateTrapped
			err = NewWASMError(ErrCodeTrap, trap.Message, nil)
			result = nil
		} else if err != nil {
			kind := ClassifyError(err)
			if kind != "" {
				i.traps = append(i.traps, &TrapInfo{
					Kind:    kind,
					Message: err.Error(),
				})
			}
			i.state = InstanceStateTrapped
		} else {
			i.state = InstanceStateReady
		}
		i.mu.Unlock()
	}()

	mem := i.module.Memory()
	if mem == nil {
		return nil, NewWASMError(ErrCodeInstantiateFailed, "no memory exported", nil)
	}

	allocFn := i.module.ExportedFunction(ExportAmitiaAlloc)
	invokeFn := i.module.ExportedFunction(export)
	if invokeFn == nil {
		if export == ExportAmitiaInvoke {
			return nil, NewWASMError(ErrCodeExportMissing, fmt.Sprintf("export not found: %s", export), nil)
		}
		invokeFn = i.module.ExportedFunction(ExportAmitiaInvoke)
		if invokeFn == nil {
			invokeFn = i.module.ExportedFunction("invoke")
			if invokeFn == nil {
				return nil, NewWASMError(ErrCodeExportMissing, fmt.Sprintf("export not found: %s or %s", export, ExportAmitiaInvoke), nil)
			}
		}
	}

	var inputPtr uint32
	if allocFn != nil && len(input) > 0 {
		allocResults, allocErr := allocFn.Call(ctx, uint64(len(input)))
		if allocErr != nil {
			return nil, NewWASMError(ErrCodeInstantiateFailed, fmt.Sprintf("alloc failed: %v", allocErr), allocErr)
		}
		inputPtr = uint32(allocResults[0])
		if inputPtr == 0 {
			return nil, NewWASMError(ErrCodeMemoryLimit, "alloc returned null pointer", nil)
		}
		if !mem.Write(inputPtr, input) {
			return nil, NewWASMError(ErrCodeMemoryAccess, "failed to write input to memory", nil)
		}
	}

	callCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	start := time.Now()
	results, callErr := invokeFn.Call(callCtx, uint64(inputPtr), uint64(len(input)))
	duration := time.Since(start)

	i.mu.Lock()
	i.stats.Invocations++
	i.mu.Unlock()

	if callErr != nil {
		if errors.Is(callErr, context.DeadlineExceeded) {
			return nil, NewWASMError(ErrCodeTimeout, "execution timeout", callErr)
		}
		if errors.Is(callErr, context.Canceled) {
			return nil, NewWASMError(ErrCodeCancelled, "execution cancelled", callErr)
		}
		return nil, NewWASMError(ErrCodeTrap, callErr.Error(), callErr)
	}

	var output []byte
	if len(results) > 0 && allocFn != nil {
		resultPtr, resultLen, decErr := DecodeResultDescriptor(results[0])
		if decErr != nil {
			return nil, NewWASMError(ErrCodeOutputInvalid, fmt.Sprintf("decode result: %v", decErr), decErr)
		}
		if resultPtr > 0 && resultLen > 0 {
			out, ok := mem.Read(resultPtr, resultLen)
			if !ok {
				return nil, NewWASMError(ErrCodeMemoryAccess, "failed to read result from memory", nil)
			}
			output = make([]byte, len(out))
			copy(output, out)
			deallocFn := i.module.ExportedFunction(ExportAmitiaDealloc)
			if deallocFn != nil {
				deallocFn.Call(ctx, uint64(resultPtr), uint64(resultLen))
			}
		}
	} else if len(results) > 0 {
		output = []byte(fmt.Sprintf(`{"result":%d}`, results[0]))
	} else {
		output = []byte(`{}`)
	}

	memSize := int64(mem.Size()) * WasmPageSize
	i.mu.Lock()
	i.stats.MemoryUsed = memSize
	i.mu.Unlock()

	return &RawResult{
		Output:     output,
		FuelUsed:   0,
		HostCalls:  0,
		MemoryUsed: memSize,
		Duration:   duration,
	}, nil
}

func (i *wazeroInstance) State() InstanceState {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.state
}

func (i *wazeroInstance) Stats() InstanceStats {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stats
}

func (i *wazeroInstance) Dispose() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.state == InstanceStateDisposed {
		return nil
	}
	i.state = InstanceStateDisposed
	if i.module != nil {
		i.module.Close(context.Background())
	}
	if i.rt != nil {
		i.rt.Close(context.Background())
	}
	return nil
}

func registerHostFunctions(builder wazero.HostModuleBuilder, registry *HostImportRegistry, allowed []HostImportName) {
	allowedSet := make(map[HostImportName]bool)
	for _, a := range allowed {
		allowedSet[a] = true
	}

	registerOne := func(name HostImportName) {
		if registry == nil {
			return
		}
		if !allowedSet[name] {
			return
		}
		h, ok := registry.Lookup(name)
		if !ok {
			return
		}
		builder.NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod api.Module, inputPtr, inputLen uint32) uint64 {
				ic := GetInvocationContext(ctx)
				if ic != nil {
					ic.HostCallsMade++
					if ic.MaxHostCalls > 0 && ic.HostCallsMade > ic.MaxHostCalls {
						return EncodeResultDescriptor(0, 0)
					}
					if !ic.DeterminismPolicy.AllowsImport(name) {
						return EncodeResultDescriptor(0, 0)
					}
				}
				var input []byte
				if inputLen > 0 && mod.Memory() != nil {
					data, ok := mod.Memory().Read(inputPtr, inputLen)
					if !ok {
						return EncodeResultDescriptor(0, 0)
					}
					input = make([]byte, len(data))
					copy(input, data)
				}
				hctx := HostCallContext{
					InvocationID: "",
					ModuleID:     "",
					ExtensionID:  "",
					Remaining:    0,
					CancelSignal: ctx,
				}
				if ic != nil {
					hctx.InvocationID = ic.InvocationID
					hctx.ModuleID = ic.ModuleID
					hctx.ExtensionID = ic.ExtensionID
					if ic.MaxHostCalls > 0 {
						hctx.Remaining = ic.MaxHostCalls - ic.HostCallsMade
					}
				}
				result, err := h(ctx, hctx, input)
				if err != nil {
					errJSON := []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
					return writeResultToMemory(mod, errJSON)
				}
				if len(result) == 0 {
					result = []byte(`{}`)
				}
				return writeResultToMemory(mod, result)
			}).
			Export(string(name))
	}

	for _, name := range FullAllowedImports {
		registerOne(name)
	}
}

func writeResultToMemory(mod api.Module, data []byte) uint64 {
	if mod == nil || mod.Memory() == nil {
		return EncodeResultDescriptor(0, 0)
	}
	mem := mod.Memory()
	currentPages := mem.Size()
	neededBytes := uint32(len(data))
	if neededBytes == 0 {
		return EncodeResultDescriptor(0, 0)
	}
	available := currentPages * WasmPageSize
	used := uint32(0)
	for used < neededBytes {
		if available < neededBytes {
			_, ok := mem.Grow(1)
			if !ok {
				return EncodeResultDescriptor(0, 0)
			}
			available += WasmPageSize
		}
		break
	}
	ptr := currentPages * WasmPageSize
	if !mem.Write(ptr, data) {
		newPages := (neededBytes + WasmPageSize - 1) / WasmPageSize
		if newPages > 0 {
			mem.Grow(newPages)
		}
		ptr = mem.Size()*WasmPageSize - neededBytes
		if !mem.Write(ptr, data) {
			return EncodeResultDescriptor(0, 0)
		}
	}
	return EncodeResultDescriptor(ptr, neededBytes)
}
