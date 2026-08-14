// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

type ProcessSupervisor interface {
	Register(spec ProcessSpec) error
	Unregister(id ProcessID) error
	Start(ctx context.Context, id ProcessID) error
	WaitReady(ctx context.Context, id ProcessID) error
	Restart(ctx context.Context, id ProcessID) error
	Stop(ctx context.Context, id ProcessID) error
	StopAll(ctx context.Context) error
	Snapshot(id ProcessID) (ProcessSnapshot, bool)
	List() []ProcessSnapshot
	Subscribe(fn func(ProcessEvent)) func()
}

const (
	healthFailureThreshold = 3
)

type managedProcess struct {
	spec            ProcessSpec
	mu              sync.Mutex
	state           ProcessState
	pid             int
	procHandle      process.ProcessTreeHandle
	executable      string
	startedAt       time.Time
	readyAt         time.Time
	stoppedAt       time.Time
	restartCount    int
	lastExitCode    int
	lastError       string
	healthFailures  int
	stopRequested   bool
	cancelMonitor   context.CancelFunc
	cancelHealth    context.CancelFunc
}

type ProcessStopper interface {
	Stop(handle process.ProcessTreeHandle, pid int, gracePeriod time.Duration) error
}

type defaultProcessSupervisor struct {
	mu          sync.RWMutex
	processes   map[ProcessID]*managedProcess
	startOrder  []ProcessID
	subscribers []func(ProcessEvent)
	host        *nativeProcessHost
	stopOnce    sync.Once
	stopped     bool
}

func newProcessSupervisor(host *nativeProcessHost) *defaultProcessSupervisor {
	return &defaultProcessSupervisor{
		processes: make(map[ProcessID]*managedProcess),
		host:      host,
	}
}

func (s *defaultProcessSupervisor) SetHost(h *nativeProcessHost) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.host = h
}

func (s *defaultProcessSupervisor) Register(spec ProcessSpec) error {
	if err := s.applyDefaults(&spec); err != nil {
		return err
	}
	if err := spec.validate(); err != nil {
		return err
	}

	s.mu.Lock()

	if _, exists := s.processes[spec.ID]; exists {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrDuplicateProcessID, spec.ID)
	}

	mp := &managedProcess{
		spec:       spec.Clone(),
		state:      StateRegistered,
		executable: filepath.Base(spec.Executable),
	}
	s.processes[spec.ID] = mp
	s.startOrder = append(s.startOrder, spec.ID)
	s.mu.Unlock()

	s.emit(EventRegistered, spec.ID, 0, 0, "")
	return nil
}

func (s *defaultProcessSupervisor) Unregister(id ProcessID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mp, ok := s.processes[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	mp.mu.Lock()
	state := mp.state
	mp.mu.Unlock()

	if state == StateRunning || state == StateReady || state == StateStarting {
		return fmt.Errorf("%w: process %s is %s", ErrProcessNotRunning, id, state)
	}

	delete(s.processes, id)
	newOrder := make([]ProcessID, 0, len(s.startOrder))
	for _, pid := range s.startOrder {
		if pid != id {
			newOrder = append(newOrder, pid)
		}
	}
	s.startOrder = newOrder
	return nil
}

func (s *defaultProcessSupervisor) Start(ctx context.Context, id ProcessID) error {
	s.mu.Lock()
	mp, ok := s.processes[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	if s.stopped {
		s.mu.Unlock()
		return ErrHostStopped
	}
	mp.mu.Lock()
	if mp.state == StateRunning || mp.state == StateReady {
		mp.mu.Unlock()
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessAlreadyRunning, id)
	}
	if mp.state == StateStopping {
		mp.mu.Unlock()
		s.mu.Unlock()
		return fmt.Errorf("%w: process %s is stopping", ErrProcessNotRunning, id)
	}
	mp.state = StateStarting
	spec := mp.spec.Clone()
	mp.mu.Unlock()
	s.mu.Unlock()

	s.emit(EventStarting, id, 0, mp.restartCount, "")

	if err := s.claimPorts(spec.Ports); err != nil {
		s.setState(id, StateFailed)
		s.setLastError(id, err.Error())
		return err
	}

	env, err := s.buildEnvironment(spec)
	if err != nil {
		s.setState(id, StateFailed)
		s.setLastError(id, err.Error())
		return err
	}

	mp.mu.Lock()
	mp.state = StateStarting
	mp.mu.Unlock()

	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	var managed *process.ManagedProcess
	if spec.ExecutableProcess != nil {
		pid, handle, execErr := spec.ExecutableProcess.Start()
		if execErr != nil {
			s.setState(id, StateFailed)
			s.setLastError(id, execErr.Error())
			return execErr
		}
		managed = process.NewExternalManagedProcess(pid, handle)
		mp.mu.Lock()
		mp.pid = pid
		mp.procHandle = handle
		mp.mu.Unlock()
	} else {
		var startErr error
		managed, startErr = s.host.processManager.Start(ctx, process.ProcessConfig{
			Executable: spec.Executable,
			Args:       spec.Args,
			WorkingDir: spec.WorkingDir,
			Env:        envSlice,
		})
		if startErr != nil {
			s.setState(id, StateFailed)
			s.setLastError(id, startErr.Error())
			return startErr
		}
		mp.mu.Lock()
		mp.pid = managed.PID
		mp.mu.Unlock()
	}

	go func() {
		code, _ := managed.Wait()
		mp.mu.Lock()
		mp.lastExitCode = code
		mp.stopRequested = true
		mp.mu.Unlock()
		s.emit(EventExited, id, managed.PID, mp.restartCount, "")
	}()

	s.emit(EventStarted, id, managed.PID, mp.restartCount, "")
	if spec.HealthProbe == nil {
		s.setState(id, StateReady)
		s.markReady(id)
	} else {
		s.setState(id, StateRunning)
	}
	return nil
}

func (s *defaultProcessSupervisor) WaitReady(ctx context.Context, id ProcessID) error {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	mp.mu.Lock()
	spec := mp.spec
	state := mp.state
	mp.mu.Unlock()

	if state == StateReady {
		return nil
	}
	if spec.HealthProbe == nil {
		s.setState(id, StateReady)
		s.markReady(id)
		return nil
	}

	timeout := spec.StartupTimeout
	if timeout <= 0 {
		timeout = DefaultStartupTimeout
	}
	deadline := time.Now().Add(timeout)

	for {
		s.mu.RLock()
		stopped := s.stopped
		s.mu.RUnlock()
		if stopped {
			return ErrHostStopped
		}

		mp.mu.Lock()
		curState := mp.state
		stopReq := mp.stopRequested
		pid := mp.pid
		mp.mu.Unlock()

		if stopReq {
			return fmt.Errorf("process %s stop requested", id)
		}
		if curState == StateStopped {
			return fmt.Errorf("process %s has stopped", id)
		}
		if curState == StateFailed {
			return fmt.Errorf("process %s has failed", id)
		}

		if pid > 0 && !s.isAlive(pid) {
			return fmt.Errorf("process %s exited prematurely", id)
		}

		hErr := spec.HealthProbe.Check(ctx)
		if hErr == nil {
			s.setState(id, StateReady)
			s.markReady(id)
			s.resetHealthFailures(id)
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w: process %s not ready: %v", ErrStartTimeout, id, hErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *defaultProcessSupervisor) Restart(ctx context.Context, id ProcessID) error {
	if err := s.Stop(ctx, id); err != nil {
		return err
	}
	return s.Start(ctx, id)
}

func (s *defaultProcessSupervisor) Stop(ctx context.Context, id ProcessID) error {
	s.mu.Lock()
	mp, ok := s.processes[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	mp.mu.Lock()
	if mp.state == StateStopped || mp.state == StateStopping {
		state := mp.state
		mp.mu.Unlock()
		s.mu.Unlock()
		if state == StateStopped {
			return nil
		}
	}
	mp.stopRequested = true
	if mp.cancelHealth != nil {
		mp.cancelHealth()
	}
	spec := mp.spec
	state := mp.state
	pid := mp.pid
	handle := mp.procHandle
	mp.state = StateStopping
	mp.mu.Unlock()
	s.mu.Unlock()

	s.emit(EventStopping, id, pid, mp.restartCount, "")

	if state == StateRegistered || pid <= 0 {
		s.setState(id, StateStopped)
		s.updateStoppedAt(id, time.Now())
		s.emit(EventStopped, id, 0, mp.restartCount, "")
		return nil
	}

	grace := spec.StopGracePeriod
	if grace <= 0 {
		grace = DefaultStopGracePeriod
	}
	if stopper, ok := spec.ExecutableProcess.(ProcessStopper); ok {
		if err := stopper.Stop(handle, pid, grace); err != nil {
			s.setLastError(id, err.Error())
		}
	} else {
		s.host.processManager.Stop(pid, handle, grace)
	}

	s.setState(id, StateStopped)
	s.updateStoppedAt(id, time.Now())
	s.emit(EventStopped, id, pid, mp.restartCount, "")
	return nil
}

func (s *defaultProcessSupervisor) StopAll(ctx context.Context) error {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		ids := make([]ProcessID, len(s.startOrder))
		copy(ids, s.startOrder)
		s.mu.Unlock()

		for i := len(ids) - 1; i >= 0; i-- {
			_ = s.Stop(ctx, ids[i])
		}
	})
	return nil
}

func (s *defaultProcessSupervisor) Snapshot(id ProcessID) (ProcessSnapshot, bool) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return ProcessSnapshot{}, false
	}
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return ProcessSnapshot{
		ID:             mp.spec.ID,
		State:          mp.state,
		PID:            mp.pid,
		Executable:     mp.executable,
		StartedAt:      mp.startedAt,
		ReadyAt:        mp.readyAt,
		StoppedAt:      mp.stoppedAt,
		RestartCount:   mp.restartCount,
		LastExitCode:   mp.lastExitCode,
		LastError:      mp.lastError,
		HealthFailures: mp.healthFailures,
	}, true
}

func (s *defaultProcessSupervisor) List() []ProcessSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProcessSnapshot, 0, len(s.processes))
	for _, id := range s.startOrder {
		if mp, ok := s.processes[id]; ok {
			mp.mu.Lock()
			snap := ProcessSnapshot{
				ID:             mp.spec.ID,
				State:          mp.state,
				PID:            mp.pid,
				Executable:     mp.executable,
				StartedAt:      mp.startedAt,
				ReadyAt:        mp.readyAt,
				StoppedAt:      mp.stoppedAt,
				RestartCount:   mp.restartCount,
				LastExitCode:   mp.lastExitCode,
				LastError:      mp.lastError,
				HealthFailures: mp.healthFailures,
			}
			mp.mu.Unlock()
			out = append(out, snap)
		}
	}
	return out
}

func (s *defaultProcessSupervisor) Subscribe(fn func(ProcessEvent)) func() {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, fn)
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		idx := -1
		for i, sub := range s.subscribers {
			if fmt.Sprintf("%p", sub) == fmt.Sprintf("%p", fn) {
				idx = i
				break
			}
		}
		if idx >= 0 {
			s.subscribers = append(s.subscribers[:idx], s.subscribers[idx+1:]...)
		}
	}
}

func (s *defaultProcessSupervisor) isAlive(pid int) bool {
	s.mu.RLock()
	host := s.host
	s.mu.RUnlock()
	if host == nil {
		return false
	}
	return host.processManager.IsProcessAlive(pid)
}

func (s *defaultProcessSupervisor) emit(eventType ProcessEventType, id ProcessID, pid, restartCount int, errStr string) {
	evt := ProcessEvent{
		ProcessID:    id,
		Type:         eventType,
		PID:          pid,
		RestartCount: restartCount,
		Timestamp:    time.Now(),
		Error:        errStr,
	}
	s.mu.RLock()
	subs := make([]func(ProcessEvent), len(s.subscribers))
	copy(subs, s.subscribers)
	s.mu.RUnlock()
	for _, fn := range subs {
		func() {
			defer func() {
				_ = recover()
			}()
			fn(evt)
		}()
	}
}

func (s *defaultProcessSupervisor) setState(id ProcessID, state ProcessState) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	mp.state = state
	mp.mu.Unlock()
}

func (s *defaultProcessSupervisor) markReady(id ProcessID) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	mp.readyAt = time.Now()
	mp.mu.Unlock()
}

func (s *defaultProcessSupervisor) updateStoppedAt(id ProcessID, t time.Time) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	mp.stoppedAt = t
	mp.mu.Unlock()
}

func (s *defaultProcessSupervisor) setLastError(id ProcessID, errMsg string) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	mp.lastError = errMsg
	mp.mu.Unlock()
}

func (s *defaultProcessSupervisor) resetHealthFailures(id ProcessID) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	mp.healthFailures = 0
	mp.mu.Unlock()
}

func (s *defaultProcessSupervisor) applyDefaults(spec *ProcessSpec) error {
	if spec.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidProcessSpec)
	}
	if err := ValidateProcessID(spec.ID); err != nil {
		return err
	}
	if spec.StartupTimeout <= 0 {
		spec.StartupTimeout = DefaultStartupTimeout
	}
	if spec.StopGracePeriod <= 0 {
		spec.StopGracePeriod = DefaultStopGracePeriod
	}
	if spec.HealthInterval <= 0 {
		spec.HealthInterval = DefaultHealthInterval
	}
	if spec.Environment.Policy == "" {
		spec.Environment.Policy = EnvPolicyMinimal
	}
	switch spec.RestartPolicy.Mode {
	case "":
		spec.RestartPolicy.Mode = RestartNever
	case RestartNever, RestartOnFailure, RestartAlways:
	default:
		return fmt.Errorf("%w: invalid restart mode %q", ErrInvalidProcessSpec, spec.RestartPolicy.Mode)
	}
	if spec.RestartPolicy.MaxRestarts <= 0 {
		spec.RestartPolicy.MaxRestarts = DefaultMaxRestarts
	}
	return nil
}

func (s *defaultProcessSupervisor) claimPorts(ports []LoopbackPortClaim) error {
	if len(ports) == 0 {
		return nil
	}
	s.mu.RLock()
	host := s.host
	s.mu.RUnlock()
	if host == nil {
		return fmt.Errorf("%w: no host bound", ErrHostProcessUnsupported)
	}
	return host.checkPorts(ports)
}

func (s *defaultProcessSupervisor) buildEnvironment(spec ProcessSpec) (map[string]string, error) {
	if spec.Environment.Policy == EnvPolicyMinimal {
		return s.buildMinimalEnv(spec), nil
	}
	if spec.Environment.Policy == EnvPolicyInherit {
		return s.buildInheritEnv(spec), nil
	}
	return spec.Environment.Values, nil
}

func (s *defaultProcessSupervisor) buildMinimalEnv(spec ProcessSpec) map[string]string {
	env := make(map[string]string)
	s.mu.RLock()
	host := s.host
	s.mu.RUnlock()
	if host != nil {
		env["AMITIA_RUNTIME_INSTANCE_ID"] = host.RuntimeInstanceID()
		env["AMITIA_PROCESS_ID"] = string(spec.ID)
		if host.descriptor.Host != "" {
			env["AMITIA_HOST_PLATFORM"] = string(host.descriptor.Host)
		}
		if host.descriptor.Kind != "" {
			env["AMITIA_RUNTIME_KIND"] = string(host.descriptor.Kind)
		}
		if host.descriptor.Guest != "" {
			env["AMITIA_GUEST_PLATFORM"] = string(host.descriptor.Guest)
		}
	}
	for k, v := range spec.Environment.Values {
		env[k] = v
	}
	return env
}

func (s *defaultProcessSupervisor) buildInheritEnv(spec ProcessSpec) map[string]string {
	env := make(map[string]string)
	for k, v := range spec.Environment.Values {
		env[k] = v
	}
	return env
}

// ErrStartTimeout is returned when WaitReady exceeds startup timeout
var ErrStartTimeout = fmt.Errorf("%w: startup timeout", ErrProcessNotRunning)
