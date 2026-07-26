package wasm_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func makeValidDefinition(t *testing.T) *WASMRuntimeDefinition {
	t.Helper()
	return makeValidDefinitionHelper()
}

func TestValidateDefinitionValid(t *testing.T) {
	def := makeValidDefinition(t)
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateDefinitionMissingModuleID(t *testing.T) {
	def := makeValidDefinition(t)
	def.ModuleID = ""
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing module_id")
	}
}

func TestValidateDefinitionMissingMemoryLimit(t *testing.T) {
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 0
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing memory limit")
	}
}

func TestValidateDefinitionTooLargeMemory(t *testing.T) {
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 2 << 30
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for too large memory")
	}
}

func TestValidateDefinitionMissingFuel(t *testing.T) {
	def := makeValidDefinition(t)
	def.FuelLimit = 0
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for missing fuel")
	}
}

func TestValidateDefinitionInvalidInstancePolicy(t *testing.T) {
	def := makeValidDefinition(t)
	def.InstancePolicy = "invalid"
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for invalid policy")
	}
}

func TestValidateDefinitionDeterministicForbidsRandom(t *testing.T) {
	def := makeValidDefinition(t)
	def.Deterministic = true
	def.AllowedImports = []HostImportName{ImportRandom}
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for deterministic + random")
	}
}

func TestValidateDefinitionDeterministicForbidsTime(t *testing.T) {
	def := makeValidDefinition(t)
	def.Deterministic = true
	def.AllowedImports = []HostImportName{ImportTime}
	if err := ValidateDefinition(def); err == nil {
		t.Fatalf("expected error for deterministic + time")
	}
}

func TestModuleValidatorValid(t *testing.T) {
	v := NewModuleValidator()
	report, err := v.ValidateBytes(makeValidWASMBytes())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected valid, got errors: %v", report.Errors)
	}
	if report.Version != WASMVersion1 {
		t.Fatalf("version mismatch: %d", report.Version)
	}
}

func TestModuleValidatorInvalidMagic(t *testing.T) {
	v := NewModuleValidator()
	report, err := v.ValidateBytes(makeInvalidWASMBytes())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Valid {
		t.Fatalf("expected invalid")
	}
	if len(report.Errors) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestModuleValidatorTooSmall(t *testing.T) {
	v := NewModuleValidator()
	report, err := v.ValidateBytes([]byte{0x00, 0x01})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Valid {
		t.Fatalf("expected invalid")
	}
}

func TestModuleCachePutGet(t *testing.T) {
	cache := NewModuleCache()
	key := ModuleCacheKey{ModuleHash: "h1", EngineName: "mock", EngineVersion: "1.0"}
	mod := &MockModule{hash: "h1"}
	cache.Put(key, mod)
	got, ok := cache.Get(key)
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if got.Hash() != "h1" {
		t.Fatalf("hash mismatch: %s", got.Hash())
	}
}

func TestModuleCacheVersionMismatch(t *testing.T) {
	cache := NewModuleCache()
	cache.Put(ModuleCacheKey{ModuleHash: "h1", EngineName: "mock", EngineVersion: "1.0"}, &MockModule{hash: "h1"})
	_, ok := cache.Get(ModuleCacheKey{ModuleHash: "h1", EngineName: "mock", EngineVersion: "2.0"})
	if ok {
		t.Fatalf("expected cache miss for version mismatch")
	}
}

func TestModuleCacheInvalidate(t *testing.T) {
	cache := NewModuleCache()
	cache.Put(ModuleCacheKey{ModuleHash: "h1", EngineName: "mock", EngineVersion: "1.0"}, &MockModule{hash: "h1"})
	cache.Invalidate("h1")
	if cache.Size() != 0 {
		t.Fatalf("expected 0 entries, got %d", cache.Size())
	}
}

func TestHostImportRegistry(t *testing.T) {
	r := NewHostImportRegistry()
	called := false
	r.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{}`), nil
	})
	h, ok := r.Lookup(ImportLog)
	if !ok {
		t.Fatalf("lookup failed")
	}
	_, _ = h(context.Background(), HostCallContext{}, nil)
	if !called {
		t.Fatalf("handler not called")
	}
}

func TestHostImportRegistryAllowed(t *testing.T) {
	r := NewHostImportRegistry()
	allowed := []HostImportName{ImportLog, ImportTime}
	if !r.Allowed(allowed, ImportLog) {
		t.Fatalf("log should be allowed")
	}
	if r.Allowed(allowed, ImportStorageGet) {
		t.Fatalf("storage should not be allowed")
	}
}

func TestHostCallCounterLimit(t *testing.T) {
	c := NewHostCallCounter(3)
	if err := c.Increment(ImportLog); err != nil {
		t.Fatalf("incr 1: %v", err)
	}
	if err := c.Increment(ImportLog); err != nil {
		t.Fatalf("incr 2: %v", err)
	}
	if err := c.Increment(ImportTime); err != nil {
		t.Fatalf("incr 3: %v", err)
	}
	if err := c.Increment(ImportLog); !errors.Is(err, ErrHostCallFailed) {
		t.Fatalf("expected host call limit, got %v", err)
	}
	if c.Total() != 3 {
		t.Fatalf("expected total 3, got %d", c.Total())
	}
}

func TestRuntimeInvokeSuccess(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	data := makeValidWASMBytes()
	res, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{"x":1}`),
		ModuleBytes: data,
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res.Output == nil {
		t.Fatalf("missing output")
	}
	if res.Cached {
		t.Fatalf("first invoke should not be cached")
	}
	res2, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{"x":2}`),
		ModuleBytes: data,
	})
	if err != nil {
		t.Fatalf("invoke 2: %v", err)
	}
	if !res2.Cached {
		t.Fatalf("second invoke should use cached module")
	}
}

func TestRuntimeInvokeHashMismatch(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	def.ModuleHash = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{}`),
		ModuleBytes: makeValidWASMBytes(),
	})
	if err == nil {
		t.Fatalf("expected hash mismatch error")
	}
	werr, ok := err.(*WASMError)
	if !ok || werr.Code != ErrCodeModuleInvalid {
		t.Fatalf("expected module_invalid, got %v", err)
	}
}

func TestRuntimeInvokeInvalidModule(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	h := sha256.Sum256(makeInvalidWASMBytes())
	def.ModuleHash = hex.EncodeToString(h[:])
	_, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{}`),
		ModuleBytes: makeInvalidWASMBytes(),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRuntimeInvokeCompileErr(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	engine.SetCompileErr(errors.New("compile failed"))
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	_, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{}`),
		ModuleBytes: makeValidWASMBytes(),
	})
	if err == nil {
		t.Fatalf("expected compile error")
	}
}

func TestRuntimeInvokeMemoryLimit(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	def.MemoryLimitBytes = 4
	_, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{"large":"input exceeds limit"}`),
		ModuleBytes: makeValidWASMBytes(),
	})
	if err == nil {
		t.Fatalf("expected memory limit error")
	}
}

func TestRuntimeRegisterMultipleEngines(t *testing.T) {
	rt := NewRuntime(nil)
	e1 := NewMockEngine("engine1", "1.0")
	e2 := NewMockEngine("engine2", "1.0")
	if err := rt.RegisterEngine(e1); err != nil {
		t.Fatalf("register e1: %v", err)
	}
	if err := rt.RegisterEngine(e2); err != nil {
		t.Fatalf("register e2: %v", err)
	}
	if err := rt.SetDefaultEngine("engine2"); err != nil {
		t.Fatalf("set default: %v", err)
	}
	if err := rt.SetDefaultEngine("missing"); err == nil {
		t.Fatalf("expected error for missing engine")
	}
}

func TestRuntimeInvokeWithCustomHandler(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	data := makeValidWASMBytes()
	res, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{"name":"test"}`),
		ModuleBytes: data,
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res.Output == nil {
		t.Fatalf("missing output")
	}
	if res.FuelUsed == 0 {
		t.Fatalf("fuel used should be > 0")
	}
}

func TestRuntimeDefaultHostImports(t *testing.T) {
	rt := NewRuntime(NewMockEngine("mock", "1.0"))
	h, ok := rt.Registry().Lookup(ImportLog)
	if !ok {
		t.Fatalf("log import not registered")
	}
	_, err := h(context.Background(), HostCallContext{}, []byte(`{}`))
	if err != nil {
		t.Fatalf("log call: %v", err)
	}
	h, ok = rt.Registry().Lookup(ImportTime)
	if !ok {
		t.Fatalf("time import not registered")
	}
	_, err = h(context.Background(), HostCallContext{}, []byte(`{}`))
	if err != nil {
		t.Fatalf("time call: %v", err)
	}
}

func TestRuntimeInvokeTimeout(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	def.CallTimeout = 1 * time.Millisecond
	_, err := rt.Invoke(context.Background(), InvokeRequest{
		Definition:  def,
		Input:       []byte(`{}`),
		ModuleBytes: makeValidWASMBytes(),
	})
	if err == nil {
		return
	}
}

func TestWASMErrorFormat(t *testing.T) {
	err := NewWASMError(ErrCodeTrap, "division by zero", nil)
	if err.Error() == "" {
		t.Fatalf("error message empty")
	}
	if err.Code != ErrCodeTrap {
		t.Fatalf("code mismatch")
	}
}

func TestInstanceStateTransitions(t *testing.T) {
	inst := &MockInstance{state: InstanceStateReady}
	if inst.State() != InstanceStateReady {
		t.Fatalf("expected ready")
	}
	inst.state = InstanceStateRunning
	if inst.State() != InstanceStateRunning {
		t.Fatalf("expected running")
	}
	if err := inst.Dispose(); err != nil {
		t.Fatalf("dispose: %v", err)
	}
	if inst.State() != InstanceStateDisposed {
		t.Fatalf("expected disposed")
	}
}

func TestInstanceInvokeAfterDispose(t *testing.T) {
	inst := &MockInstance{state: InstanceStateReady}
	if err := inst.Dispose(); err != nil {
		t.Fatalf("dispose: %v", err)
	}
	_, err := inst.Invoke(context.Background(), "invoke", []byte{}, InvokeOptions{})
	if !errors.Is(err, ErrInstanceDisposed) {
		t.Fatalf("expected disposed error, got %v", err)
	}
}

func TestConcurrentInvoke(t *testing.T) {
	engine := NewMockEngine("mock", "1.0")
	rt := NewRuntime(engine)
	def := makeValidDefinition(t)
	data := makeValidWASMBytes()
	const N = 8
	var wg sync.WaitGroup
	errs := make(chan error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := rt.Invoke(context.Background(), InvokeRequest{
				Definition:  def,
				Input:       []byte(`{}`),
				ModuleBytes: data,
			})
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("invoke failed: %v", err)
	}
}
