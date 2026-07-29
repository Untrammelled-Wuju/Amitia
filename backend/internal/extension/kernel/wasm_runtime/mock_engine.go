package wasm_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type MockEngine struct {
	mu           sync.Mutex
	name         string
	version      string
	instances    map[string]*MockInstance
	compileErr   error
	allowCompile bool
}

func NewMockEngine(name, version string) *MockEngine {
	return &MockEngine{
		name:         name,
		version:      version,
		instances:    make(map[string]*MockInstance),
		allowCompile: true,
	}
}

func (e *MockEngine) Name() string    { return e.name }
func (e *MockEngine) Version() string { return e.version }

func (e *MockEngine) SetCompileErr(err error) {
	e.mu.Lock()
	e.compileErr = err
	e.mu.Unlock()
}

func (e *MockEngine) SetAllowCompile(allow bool) {
	e.mu.Lock()
	e.allowCompile = allow
	e.mu.Unlock()
}

func (e *MockEngine) Compile(ctx context.Context, moduleBytes []byte) (CompiledModule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.allowCompile {
		return nil, errors.New("mock: compile disabled")
	}
	if e.compileErr != nil {
		return nil, e.compileErr
	}
	h := sha256.Sum256(moduleBytes)
	return &MockModule{
		hash:    hex.EncodeToString(h[:]),
		exports: []string{"invoke"},
		imports: []string{},
	}, nil
}

func (e *MockEngine) Instantiate(ctx context.Context, compiled CompiledModule, opts InstantiateOptions) (Instance, error) {
	inst := &MockInstance{
		id:       fmt.Sprintf("inst-%d", time.Now().UnixNano()),
		module:   compiled,
		opts:     opts,
		state:    InstanceStateReady,
		stats:    InstanceStats{InstanceID: "", ModuleID: ""},
		registry: opts.HostImports,
	}
	return inst, nil
}

type MockModule struct {
	hash    string
	exports []string
	imports []string
}

func (m *MockModule) Hash() string      { return m.hash }
func (m *MockModule) Exports() []string { return m.exports }
func (m *MockModule) Imports() []string { return m.imports }
func (m *MockModule) Dispose()          {}

type MockInstance struct {
	mu       sync.Mutex
	id       string
	module   CompiledModule
	opts     InstantiateOptions
	state    InstanceState
	stats    InstanceStats
	registry *HostImportRegistry
	InvokeFn func(ctx context.Context, export string, input []byte, opts InvokeOptions) (*RawResult, error)
}

func (i *MockInstance) Invoke(ctx context.Context, export string, input []byte, opts InvokeOptions) (*RawResult, error) {
	i.mu.Lock()
	if i.state == InstanceStateDisposed {
		i.mu.Unlock()
		return nil, ErrInstanceDisposed
	}
	i.state = InstanceStateRunning
	i.mu.Unlock()
	defer func() {
		i.mu.Lock()
		i.state = InstanceStateReady
		i.mu.Unlock()
	}()
	if i.InvokeFn != nil {
		return i.InvokeFn(ctx, export, input, opts)
	}
	start := time.Now()
	output, err := json.Marshal(map[string]any{"ok": true, "input_size": len(input)})
	if err != nil {
		return nil, err
	}
	if opts.MemoryLimit > 0 && int64(len(input)) > opts.MemoryLimit {
		return nil, ErrMemoryLimit
	}
	if opts.FuelLimit > 0 && uint64(len(input))*100 > opts.FuelLimit {
		return nil, ErrFuelExhausted
	}
	if opts.MaxHostCalls > 0 && i.registry != nil {
	}
	i.mu.Lock()
	i.stats.Invocations++
	i.stats.MemoryUsed = int64(len(input))
	i.stats.FuelUsed = uint64(len(input)) * 100
	i.mu.Unlock()
	return &RawResult{
		Output:     output,
		FuelUsed:   uint64(len(input)) * 100,
		HostCalls:  0,
		MemoryUsed: int64(len(input)),
		Duration:   time.Since(start),
	}, nil
}

func (i *MockInstance) State() InstanceState {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.state
}

func (i *MockInstance) Stats() InstanceStats {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stats
}

func (i *MockInstance) Dispose() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state = InstanceStateDisposed
	return nil
}

func makeValidWASMBytes() []byte {
	data := []byte(WASMMagic)
	data = append(data, 0x01, 0x00, 0x00, 0x00)
	data = append(data, 0x00)
	return data
}

func makeInvalidWASMBytes() []byte {
	return []byte("not wasm")
}

func makeValidDefinitionHelper() *WASMRuntimeDefinition {
	data := makeValidWASMBytes()
	h := sha256.Sum256(data)
	return &WASMRuntimeDefinition{
		ModuleID:         "mod-1",
		ExtensionID:      "ext-1",
		ModulePath:       "modules/wasm/mod-1/module.wasm",
		ModuleHash:       hex.EncodeToString(h[:]),
		ABI:              ABIAmitia,
		MemoryLimitBytes: 16 * 1024 * 1024,
		FuelLimit:        1000000,
		InstancePolicy:   InstancePolicyPerInvocation,
		AllowedImports:   DefaultAllowedImports,
		EntryExport:      "invoke",
		MaxOutputBytes:   1024 * 1024,
		MaxHostCalls:     10,
		CallTimeout:      5 * time.Second,
	}
}
