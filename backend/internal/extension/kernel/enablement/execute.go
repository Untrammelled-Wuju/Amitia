//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrExecuteInvalid      = errors.New("plugin execute: invalid request")
	ErrExecuteDenied       = errors.New("plugin execute: plugin is not enabled")
	ErrActivationTimeout   = errors.New("plugin execute: activation timed out")
	ErrExecuteFailed       = errors.New("plugin execute: invocation failed")
	ErrRequestInFlight     = errors.New("plugin execute: duplicate request in flight")
	ErrRuntimeNotInvocable = errors.New("plugin execute: runtime cannot be invoked")
)

type RuntimeInvoker interface {
	Invoke(ctx context.Context, payload []byte) ([]byte, error)
}

type ExecuteRequest struct {
	ID      string
	Payload []byte
}

type ExecuteResult struct {
	ID       string
	Output   []byte
	Reused   bool
	Duration time.Duration
}

func (i *Installer) requestTables() (map[string]bool, map[string]ExecuteResult) {
	if i.inflight == nil {
		i.inflight = map[string]bool{}
	}
	if i.results == nil {
		i.results = map[string]ExecuteResult{}
	}
	return i.inflight, i.results
}

func (i *Installer) Execute(ctx context.Context, pluginID string, request ExecuteRequest) (ExecuteResult, error) {
	if i == nil {
		return ExecuteResult{}, fmt.Errorf("%w: installer is nil", ErrExecuteInvalid)
	}
	if strings.TrimSpace(request.ID) == "" {
		return ExecuteResult{}, fmt.Errorf("%w: request id is required", ErrExecuteInvalid)
	}
	if err := ctx.Err(); err != nil {
		return ExecuteResult{}, err
	}
	i.reqMu.Lock()
	inflight, results := i.requestTables()
	if cached, ok := results[request.ID]; ok {
		i.reqMu.Unlock()
		cached.Reused = true
		return cached, nil
	}
	if inflight[request.ID] {
		i.reqMu.Unlock()
		return ExecuteResult{}, fmt.Errorf("%w: %s", ErrRequestInFlight, request.ID)
	}
	inflight[request.ID] = true
	i.reqMu.Unlock()
	defer func() {
		i.reqMu.Lock()
		delete(i.inflight, request.ID)
		i.reqMu.Unlock()
	}()

	started := time.Now()
	cold, handle, err := i.ensureRuntime(ctx, pluginID)
	if err != nil {
		return ExecuteResult{}, err
	}
	invoker, ok := handle.(RuntimeInvoker)
	if !ok {
		i.rollbackColdStart(pluginID, handle, cold)
		return ExecuteResult{}, fmt.Errorf("%w: %s", ErrRuntimeNotInvocable, pluginID)
	}
	i.beginCall(pluginID)
	output, err := invoker.Invoke(ctx, request.Payload)
	i.endCall(pluginID)
	if err != nil {
		i.rollbackColdStart(pluginID, handle, cold)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ExecuteResult{}, fmt.Errorf("%w: %w", ErrActivationTimeout, err)
		}
		if errors.Is(err, context.Canceled) {
			return ExecuteResult{}, fmt.Errorf("%w: %w", ErrExecuteFailed, err)
		}
		return ExecuteResult{}, fmt.Errorf("%w: %w", ErrExecuteFailed, err)
	}
	if i.activity != nil {
		i.activity(pluginID)
	}
	result := ExecuteResult{ID: request.ID, Output: output, Duration: time.Since(started)}
	i.reqMu.Lock()
	i.requestTables()
	i.results[request.ID] = result
	i.reqMu.Unlock()
	return result, nil
}

func (i *Installer) ensureRuntime(ctx context.Context, pluginID string) (bool, RuntimeHandle, error) {
	i.mu.Lock()
	machine, err := i.machineFor(pluginID)
	if err != nil {
		i.mu.Unlock()
		return false, nil, err
	}
	state := machine.State()
	switch state {
	case LifecycleActivated, LifecycleWarm, LifecycleRunning:
		handle, ok := i.handles[pluginID]
		i.mu.Unlock()
		if !ok {
			return false, nil, fmt.Errorf("%w: %s is %s without a runtime", ErrExecuteDenied, pluginID, state)
		}
		return false, handle, nil
	case LifecycleEnabled:
	default:
		i.mu.Unlock()
		return false, nil, fmt.Errorf("%w: %s is %s", ErrExecuteDenied, pluginID, state)
	}
	metadata, ok := i.index.entry(pluginID)
	if !ok {
		i.mu.Unlock()
		return false, nil, fmt.Errorf("%w: %s metadata missing", ErrInstallerState, pluginID)
	}
	if i.spawner == nil {
		i.mu.Unlock()
		return false, nil, fmt.Errorf("%w: no runtime spawner", ErrInstallerState)
	}
	if _, err := machine.Apply(ctx, EventActivate, machine.Epoch()+1); err != nil {
		i.mu.Unlock()
		return false, nil, err
	}
	i.mu.Unlock()

	handle, spawnErr := i.spawner.Spawn(ctx, metadata)
	if spawnErr != nil {
		i.rollbackActivated(pluginID, machine)
		if errors.Is(spawnErr, context.DeadlineExceeded) {
			return false, nil, fmt.Errorf("%w: %w", ErrActivationTimeout, spawnErr)
		}
		return false, nil, fmt.Errorf("%w: %w", ErrExecuteFailed, spawnErr)
	}
	i.mu.Lock()
	i.handles[pluginID] = handle
	i.spawns++
	i.mu.Unlock()
	return true, handle, nil
}

func (i *Installer) rollbackActivated(pluginID string, machine *Machine) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if machine == nil {
		return
	}
	if machine.State() == LifecycleActivated {
		_, _ = machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1)
	}
}

func (i *Installer) rollbackColdStart(pluginID string, handle RuntimeHandle, cold bool) {
	if !cold {
		return
	}
	i.mu.Lock()
	current := i.handles[pluginID]
	if current != nil && handle != nil && current.ID() == handle.ID() {
		delete(i.handles, pluginID)
	}
	i.mu.Unlock()
	if closer, ok := handle.(io.Closer); ok {
		_ = closer.Close()
	}
	i.mu.Lock()
	machine := i.machines[pluginID]
	i.mu.Unlock()
	i.rollbackActivated(pluginID, machine)
}
