package runtime_supervisor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type FakeRuntime struct {
	mu          sync.Mutex
	started     bool
	startErr    error
	stopErr     error
	invokeErr   error
	health      HealthReport
	calls       []InvocationRequest
	results     map[string][]byte
	stopCount   int
	startCount  int
	invokeCount int
}

func NewFakeRuntime() *FakeRuntime {
	return &FakeRuntime{
		health:  HealthReport{Status: HealthHealthy},
		results: make(map[string][]byte),
	}
}

func (f *FakeRuntime) Start(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCount++
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	return nil
}

func (f *FakeRuntime) Invoke(_ context.Context, request InvocationRequest) InvocationResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invokeCount++
	f.calls = append(f.calls, request)
	if f.invokeErr != nil {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        f.invokeErr,
			Duration:     time.Millisecond,
		}
	}
	out, ok := f.results[request.Operation]
	if !ok {
		out = []byte(`{"ok":true}`)
	}
	return InvocationResult{
		InvocationID: request.InvocationID,
		Status:       "success",
		Output:       out,
		Duration:     time.Millisecond,
	}
}

func (f *FakeRuntime) Health(_ context.Context) HealthReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.health.Status == "" {
		f.health.Status = HealthHealthy
	}
	return f.health
}

func (f *FakeRuntime) Stop(_ context.Context, _ StopReason) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCount++
	f.started = false
	return f.stopErr
}

func (f *FakeRuntime) SetStartErr(err error)  { f.startErr = err }
func (f *FakeRuntime) SetInvokeErr(err error) { f.invokeErr = err }
func (f *FakeRuntime) SetHealth(h HealthReport) { f.health = h }

type FakeFactory struct {
	mu         sync.Mutex
	runtime    *FakeRuntime
	createErr  error
	validateErr error
	type_      domain.RuntimeType
	created    int
}

func NewFakeFactory(t domain.RuntimeType, runtime *FakeRuntime) *FakeFactory {
	return &FakeFactory{type_: t, runtime: runtime}
}

func (f *FakeFactory) Type() domain.RuntimeType { return f.type_ }

func (f *FakeFactory) Validate(_ InstanceSpec) error {
	return f.validateErr
}

func (f *FakeFactory) Create(_ context.Context, _ InstanceSpec) (ManagedRuntime, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.runtime, nil
}

func (f *FakeFactory) SetCreateErr(err error)   { f.createErr = err }
func (f *FakeFactory) SetValidateErr(err error) { f.validateErr = err }

var (
	ErrFakeStart   = errors.New("fake start error")
	ErrFakeInvoke  = errors.New("fake invoke error")
	ErrFakeCreate  = errors.New("fake create error")
)
