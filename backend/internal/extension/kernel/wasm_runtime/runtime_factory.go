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

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type WASMRuntimeFactory struct {
	mu         sync.RWMutex
	engine     Engine
	moduleMgr  *ModuleManager
	hostGateway *HostGateway
	runtime    *Runtime
	defs       map[string]*WASMRuntimeDefinition
	modules    map[string]*LoadedModule
	instances  map[string]*WASMManagedRuntime
	logger     func(level, msg string, fields map[string]any)
}

func NewWASMRuntimeFactory(engine Engine, moduleMgr *ModuleManager) *WASMRuntimeFactory {
	if engine == nil {
		engine = NewWazeroEngine()
	}
	if moduleMgr == nil {
		moduleMgr = NewModuleManager("")
	}
	f := &WASMRuntimeFactory{
		engine:    engine,
		moduleMgr: moduleMgr,
		runtime:   NewRuntime(engine),
		defs:      make(map[string]*WASMRuntimeDefinition),
		modules:   make(map[string]*LoadedModule),
		instances: make(map[string]*WASMManagedRuntime),
		logger:    func(level, msg string, fields map[string]any) {},
	}
	return f
}

func (f *WASMRuntimeFactory) SetLogger(l func(level, msg string, fields map[string]any)) {
	f.logger = l
	f.runtime.SetLogger(l)
}

func (f *WASMRuntimeFactory) SetHostGateway(gw *HostGateway) {
	f.mu.Lock()
	f.hostGateway = gw
	f.mu.Unlock()
}

func (f *WASMRuntimeFactory) RegisterDefinition(def *WASMRuntimeDefinition) error {
	if err := ValidateDefinition(def); err != nil {
		return err
	}
	f.mu.Lock()
	f.defs[def.ModuleID] = def
	f.mu.Unlock()
	return nil
}

func (f *WASMRuntimeFactory) GetDefinition(moduleID string) (*WASMRuntimeDefinition, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	def, ok := f.defs[moduleID]
	return def, ok
}

func (f *WASMRuntimeFactory) LoadModule(moduleID string, data []byte) error {
	mod, err := f.moduleMgr.LoadFromBytes(moduleID, data)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.modules[moduleID] = mod
	f.mu.Unlock()
	return nil
}

func (f *WASMRuntimeFactory) LoadModuleFromPath(moduleID, path string) error {
	mod, err := f.moduleMgr.LoadFromPath(moduleID, path)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.modules[moduleID] = mod
	f.mu.Unlock()
	return nil
}

func (f *WASMRuntimeFactory) Type() domain.RuntimeType {
	return domain.RuntimeTypeWASM
}

func (f *WASMRuntimeFactory) Validate(spec runtime_supervisor.InstanceSpec) error {
	if spec.RuntimeType != domain.RuntimeTypeWASM {
		return errors.New("wasm_runtime: runtime type must be wasm")
	}
	if spec.ExtensionID == "" {
		return errors.New("wasm_runtime: extension id required")
	}
	if spec.ModuleID == "" {
		return errors.New("wasm_runtime: module id required")
	}
	if spec.EntryPoint == "" {
		return errors.New("wasm_runtime: entry point required")
	}
	f.mu.RLock()
	_, defOK := f.defs[string(spec.ModuleID)]
	_, modOK := f.modules[string(spec.ModuleID)]
	f.mu.RUnlock()
	if !defOK {
		return errors.New("wasm_runtime: definition not registered for module: " + string(spec.ModuleID))
	}
	if !modOK {
		return errors.New("wasm_runtime: module not loaded: " + string(spec.ModuleID))
	}
	return nil
}

func (f *WASMRuntimeFactory) Create(ctx context.Context, spec runtime_supervisor.InstanceSpec) (runtime_supervisor.ManagedRuntime, error) {
	if err := f.Validate(spec); err != nil {
		return nil, err
	}

	f.mu.RLock()
	def := f.defs[string(spec.ModuleID)]
	mod := f.modules[string(spec.ModuleID)]
	f.mu.RUnlock()

	if def == nil {
		return nil, errors.New("wasm_runtime: definition not found")
	}
	if mod == nil {
		return nil, errors.New("wasm_runtime: module not found")
	}

	instanceID := fmt.Sprintf("wasm-%s-%s", spec.ModuleID, uuid.NewString()[:8])

	managed := &WASMManagedRuntime{
		mu:         sync.Mutex{},
		instanceID: instanceID,
		def:        def,
		module:     mod,
		runtime:    f.runtime,
		engine:     f.engine,
		logger:     f.logger,
		identity: runtime_supervisor.RuntimeIdentity{
			InstanceID:         instanceID,
			RuntimeDefinitionID: runtime_supervisor.DefinitionID(spec.DefinitionID),
			ExtensionID:        spec.ExtensionID,
			ModuleID:           spec.ModuleID,
			RuntimeType:        spec.RuntimeType,
			Generation:         spec.Generation,
			SessionNonce:       uuid.NewString(),
		},
		state: runtime_supervisor.ActualCreated,
	}

	f.mu.Lock()
	f.instances[instanceID] = managed
	f.mu.Unlock()

	return managed, nil
}

type WASMManagedRuntime struct {
	mu         sync.Mutex
	instanceID string
	def        *WASMRuntimeDefinition
	module     *LoadedModule
	runtime    *Runtime
	engine     Engine
	logger     func(level, msg string, fields map[string]any)
	identity   runtime_supervisor.RuntimeIdentity
	state      runtime_supervisor.ActualState
	invocations int64
	traps       int64
	timeouts    int64
	lastError   string
	lastUsedAt  *time.Time
}

func (w *WASMManagedRuntime) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.def == nil || w.module == nil {
		w.state = runtime_supervisor.ActualFailed
		w.lastError = "missing definition or module"
		return errors.New("wasm_runtime: missing definition or module")
	}
	if err := ValidateDefinition(w.def); err != nil {
		w.state = runtime_supervisor.ActualFailed
		w.lastError = err.Error()
		return err
	}
	if w.module != nil && w.def.ModuleHash != "" {
		h := sha256.Sum256(w.module.Bytes)
		actualHash := hex.EncodeToString(h[:])
		if actualHash != w.def.ModuleHash {
			w.state = runtime_supervisor.ActualFailed
			w.lastError = "module hash mismatch"
			return NewWASMError(ErrCodeIntegrityFailed, "module hash mismatch on start", nil)
		}
	}
	w.state = runtime_supervisor.ActualReady
	w.logger("info", "wasm runtime started", map[string]any{
		"instance": w.instanceID,
		"module":   w.def.ModuleID,
	})
	return nil
}

func (w *WASMManagedRuntime) Invoke(ctx context.Context, request runtime_supervisor.InvocationRequest) runtime_supervisor.InvocationResult {
	w.mu.Lock()
	if w.state != runtime_supervisor.ActualReady && w.state != runtime_supervisor.ActualDegraded {
		w.mu.Unlock()
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("wasm_runtime: instance not ready (state=%s)", w.state),
		}
	}
	w.state = runtime_supervisor.ActualReady
	w.invocations++
	now := time.Now().UTC()
	w.lastUsedAt = &now
	w.mu.Unlock()

	input := request.Input
	if len(input) == 0 {
		input = []byte(`{}`)
	}

	invCtx := ctx
	if !request.Deadline.IsZero() {
		var cancel context.CancelFunc
		invCtx, cancel = context.WithDeadline(ctx, request.Deadline)
		defer cancel()
	}

	invCtx = WithInvocationContext(invCtx, &InvocationContext{
		InvocationID:  request.InvocationID,
		OperationID:   request.Operation,
		ExtensionID:   string(w.identity.ExtensionID),
		ModuleID:      string(w.identity.ModuleID),
		Deadline:      request.Deadline,
		MaxHostCalls:  w.def.MaxHostCalls,
		AllowedImports: w.def.AllowedImports,
		DeterminismPolicy: NonDeterministicPolicy(),
		Registry:      w.runtime.Registry(),
	})

	result, err := w.runtime.Invoke(invCtx, InvokeRequest{
		Definition:   w.def,
		Input:        input,
		ModuleBytes:  w.module.Bytes,
	})

	w.mu.Lock()
	if err != nil {
		w.lastError = err.Error()
		var werr *WASMError
		if errors.As(err, &werr) {
			switch werr.Code {
			case ErrCodeTimeout:
				w.timeouts++
			case ErrCodeTrap:
				w.traps++
			}
		}
		w.state = runtime_supervisor.ActualDegraded
	} else {
		w.state = runtime_supervisor.ActualReady
	}
	w.mu.Unlock()

	if err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        err,
		}
	}

	output := result.Output
	if len(output) == 0 {
		output = []byte(`{}`)
	}

	return runtime_supervisor.InvocationResult{
		InvocationID: request.InvocationID,
		Status:       "success",
		Output:       output,
		Duration:     result.Duration,
	}
}

func (w *WASMManagedRuntime) Health(ctx context.Context) runtime_supervisor.HealthReport {
	w.mu.Lock()
	defer w.mu.Unlock()

	status := runtime_supervisor.HealthHealthy
	reason := ""
	if w.state == runtime_supervisor.ActualCrashed || w.state == runtime_supervisor.ActualFailed {
		status = runtime_supervisor.HealthUnhealthy
		reason = w.lastError
	} else if w.state == runtime_supervisor.ActualDegraded {
		status = runtime_supervisor.HealthDegraded
		reason = w.lastError
	} else if w.state == runtime_supervisor.ActualStopped {
		status = runtime_supervisor.HealthUnknown
	}

	metrics := map[string]any{
		"invocations": w.invocations,
		"traps":       w.traps,
		"timeouts":    w.timeouts,
		"state":       string(w.state),
	}

	return runtime_supervisor.HealthReport{
		Status:    status,
		Reason:    reason,
		CheckedAt: time.Now().UTC(),
		Metrics:   metrics,
	}
}

func (w *WASMManagedRuntime) Stop(ctx context.Context, reason runtime_supervisor.StopReason) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state = runtime_supervisor.ActualStopped
	w.logger("info", "wasm runtime stopped", map[string]any{
		"instance": w.instanceID,
		"reason":   string(reason),
	})
	return nil
}

func (w *WASMManagedRuntime) State() runtime_supervisor.ActualState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

func (w *WASMManagedRuntime) Identity() runtime_supervisor.RuntimeIdentity {
	return w.identity
}

func (w *WASMManagedRuntime) Stats() map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	return map[string]any{
		"invocations": w.invocations,
		"traps":       w.traps,
		"timeouts":    w.timeouts,
		"state":       string(w.state),
		"last_error":  w.lastError,
		"last_used":   w.lastUsedAt,
	}
}

func (f *WASMRuntimeFactory) GetInstance(instanceID string) (*WASMManagedRuntime, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	inst, ok := f.instances[instanceID]
	return inst, ok
}

func (f *WASMRuntimeFactory) ListInstances() []*WASMManagedRuntime {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*WASMManagedRuntime, 0, len(f.instances))
	for _, inst := range f.instances {
		out = append(out, inst)
	}
	return out
}

func (f *WASMRuntimeFactory) Runtime() *Runtime {
	return f.runtime
}

func (f *WASMRuntimeFactory) ModuleManager() *ModuleManager {
	return f.moduleMgr
}

func (f *WASMRuntimeFactory) Invoke(ctx context.Context, moduleID string, input json.RawMessage) (*InvocationResult, error) {
	f.mu.RLock()
	def, defOK := f.defs[moduleID]
	mod, modOK := f.modules[moduleID]
	f.mu.RUnlock()
	if !defOK || !modOK {
		return nil, NewWASMError(ErrCodeModuleMissing, "module not registered: "+moduleID, nil)
	}
	return f.runtime.Invoke(ctx, InvokeRequest{
		Definition:  def,
		Input:       input,
		ModuleBytes: mod.Bytes,
	})
}

var _ runtime_supervisor.RuntimeFactory = (*WASMRuntimeFactory)(nil)
var _ runtime_supervisor.ManagedRuntime = (*WASMManagedRuntime)(nil)
