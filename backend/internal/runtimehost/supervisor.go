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
	spec               ProcessSpec
	mu                 sync.Mutex
	state              ProcessState
	pid                int
	procHandle         process.ProcessTreeHandle
	executable         string
	startedAt          time.Time
	readyAt            time.Time
	stoppedAt          time.Time
	restartCount       int
	lastExitCode       int
	lastError          string
	healthFailures     int
	stopRequested      bool
	forceRestart       bool
	forceRestartReason string
	generation         uint64
	cancelMonitor      context.CancelFunc
	cancelHealth       context.CancelFunc
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

	if state == StateRunning || state == StateReady || state == StateStarting || state == StateStopping || state == StateRestartBackoff {
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
	if mp.state == StateRunning || mp.state == StateReady || mp.state == StateStarting {
		mp.mu.Unlock()
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessAlreadyRunning, id)
	}
	if mp.state == StateStopping {
		mp.mu.Unlock()
		s.mu.Unlock()
		return fmt.Errorf("%w: process %s is stopping", ErrProcessNotRunning, id)
	}
	if mp.cancelMonitor != nil {
		mp.cancelMonitor()
		mp.cancelMonitor = nil
	}
	if mp.cancelHealth != nil {
		mp.cancelHealth()
		mp.cancelHealth = nil
	}
	mp.stopRequested = false
	mp.healthFailures = 0
	mp.state = StateStarting
	mp.pid = 0
	mp.procHandle = 0
	mp.stoppedAt = time.Time{}
	mp.readyAt = time.Time{}
	mp.generation++
	generation := mp.generation
	restartCount := mp.restartCount
	spec := mp.spec.Clone()
	mp.mu.Unlock()
	s.mu.Unlock()

	s.emit(EventStarting, id, 0, restartCount, "")

	if err := s.claimPorts(spec.Ports); err != nil {
		s.failStart(id, generation, err)
		return err
	}

	env, err := s.buildEnvironment(spec)
	if err != nil {
		s.failStart(id, generation, err)
		return err
	}

	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	var managed *process.ManagedProcess
	if spec.ExecutableProcess != nil {
		pid, handle, execErr := spec.ExecutableProcess.Start()
		if execErr != nil {
			s.failStart(id, generation, execErr)
			return execErr
		}
		managed = process.NewExternalManagedProcess(pid, handle)
	} else {
		if s.host == nil || s.host.processManager == nil {
			err := ErrHostProcessUnsupported
			s.failStart(id, generation, err)
			return err
		}
		var startErr error
		managed, startErr = s.host.processManager.Start(ctx, process.ProcessConfig{
			Executable:     spec.Executable,
			Args:           spec.Args,
			WorkingDir:     spec.WorkingDir,
			Env:            envSlice,
			OnStdout:       spec.OnStdout,
			OnStderr:       spec.OnStderr,
			OnScannerError: spec.OnStreamError,
		})
		if startErr != nil {
			s.failStart(id, generation, startErr)
			return startErr
		}
	}

	mp.mu.Lock()
	if mp.generation != generation || mp.stopRequested {
		mp.mu.Unlock()
		_ = s.stopManagedProcess(spec, managed.PID, managed.Handle)
		return fmt.Errorf("%w: process %s start superseded", ErrProcessNotRunning, id)
	}
	mp.pid = managed.PID
	mp.procHandle = managed.Handle
	mp.startedAt = time.Now().UTC()
	mp.lastError = ""
	if spec.HealthProbe == nil {
		mp.state = StateReady
		mp.readyAt = time.Now().UTC()
	} else {
		mp.state = StateRunning
	}
	mp.mu.Unlock()

	go s.waitForExit(id, managed, generation)

	s.emit(EventStarted, id, managed.PID, restartCount, "")
	if spec.HealthProbe == nil {
		s.emit(EventReady, id, managed.PID, restartCount, "")
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
	spec := mp.spec.Clone()
	state := mp.state
	generation := mp.generation
	pid := mp.pid
	restartCount := mp.restartCount
	mp.mu.Unlock()

	if state == StateReady {
		if spec.HealthProbe != nil {
			s.startHealthMonitor(id, generation)
		}
		return nil
	}
	if spec.HealthProbe == nil {
		s.setReadyIfGeneration(id, generation)
		s.emit(EventReady, id, pid, restartCount, "")
		return nil
	}

	timeout := spec.StartupTimeout
	if timeout <= 0 {
		timeout = DefaultStartupTimeout
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
		curPID := mp.pid
		curGeneration := mp.generation
		mp.mu.Unlock()

		if curGeneration != generation {
			return fmt.Errorf("process %s launch generation changed while waiting for readiness", id)
		}
		if stopReq {
			return fmt.Errorf("process %s stop requested", id)
		}
		if curState == StateStopped || curState == StateRestartBackoff {
			return fmt.Errorf("process %s is %s", id, curState)
		}
		if curState == StateFailed {
			return fmt.Errorf("process %s has failed", id)
		}
		if curPID > 0 && !s.isAlive(curPID) {
			return fmt.Errorf("process %s exited prematurely", id)
		}

		hErr := spec.HealthProbe.Check(readyCtx)
		if hErr == nil {
			if s.setReadyIfGeneration(id, generation) {
				s.emit(EventReady, id, curPID, restartCount, "")
			}
			s.startHealthMonitor(id, generation)
			return nil
		}

		select {
		case <-readyCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: process %s not ready: %v", ErrStartTimeout, id, hErr)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *defaultProcessSupervisor) Restart(ctx context.Context, id ProcessID) error {
	if err := s.Stop(ctx, id); err != nil {
		return err
	}
	if err := s.Start(ctx, id); err != nil {
		return err
	}
	if err := s.WaitReady(ctx, id); err != nil {
		return err
	}
	snap, _ := s.Snapshot(id)
	s.emit(EventRestarted, id, snap.PID, snap.RestartCount, "manual")
	return nil
}

func (s *defaultProcessSupervisor) Stop(ctx context.Context, id ProcessID) error {
	s.mu.Lock()
	mp, ok := s.processes[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	mp.mu.Lock()
	if mp.state == StateStopped {
		mp.mu.Unlock()
		s.mu.Unlock()
		return nil
	}
	if mp.state == StateStopping {
		mp.mu.Unlock()
		s.mu.Unlock()
		return nil
	}
	mp.stopRequested = true
	if mp.cancelMonitor != nil {
		mp.cancelMonitor()
		mp.cancelMonitor = nil
	}
	if mp.cancelHealth != nil {
		mp.cancelHealth()
		mp.cancelHealth = nil
	}
	spec := mp.spec.Clone()
	state := mp.state
	pid := mp.pid
	handle := mp.procHandle
	restartCount := mp.restartCount
	mp.state = StateStopping
	mp.mu.Unlock()
	s.mu.Unlock()

	s.emit(EventStopping, id, pid, restartCount, "")

	if state == StateRegistered || state == StateRestartBackoff || pid <= 0 {
		s.setState(id, StateStopped)
		s.updateStoppedAt(id, time.Now().UTC())
		s.emit(EventStopped, id, 0, restartCount, "")
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
	} else if s.host != nil && s.host.processManager != nil {
		if err := s.host.processManager.StopContext(ctx, pid, handle, grace); err != nil {
			s.setLastError(id, err.Error())
		}
	}

	s.setState(id, StateStopped)
	s.updateStoppedAt(id, time.Now().UTC())
	s.emit(EventStopped, id, pid, restartCount, "")
	return nil
}

func (s *defaultProcessSupervisor) failStart(id ProcessID, generation uint64, err error) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	if mp.generation != generation {
		mp.mu.Unlock()
		return
	}
	mp.state = StateFailed
	mp.lastError = err.Error()
	restartCount := mp.restartCount
	mp.mu.Unlock()
	s.emit(EventFailed, id, 0, restartCount, err.Error())
}

func (s *defaultProcessSupervisor) setReadyIfGeneration(id ProcessID, generation uint64) bool {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.generation != generation || mp.stopRequested || mp.state == StateStopped || mp.state == StateStopping || mp.state == StateFailed {
		return false
	}
	if mp.state == StateReady {
		return false
	}
	mp.state = StateReady
	mp.readyAt = time.Now().UTC()
	mp.healthFailures = 0
	return true
}

func (s *defaultProcessSupervisor) waitForExit(id ProcessID, managed *process.ManagedProcess, generation uint64) {
	code, waitErr := managed.Wait()
	s.mu.RLock()
	mp, ok := s.processes[id]
	stopped := s.stopped
	s.mu.RUnlock()
	if !ok {
		return
	}

	mp.mu.Lock()
	if mp.generation != generation {
		mp.mu.Unlock()
		return
	}
	mp.lastExitCode = code
	if waitErr != nil {
		mp.lastError = waitErr.Error()
	}
	if mp.cancelHealth != nil {
		mp.cancelHealth()
		mp.cancelHealth = nil
	}
	mp.pid = 0
	mp.procHandle = 0
	forcedRestart := mp.forceRestart && !stopped
	forcedReason := mp.forceRestartReason
	manualStop := (mp.stopRequested || stopped) && !forcedRestart
	restartCount := mp.restartCount
	spec := mp.spec.Clone()
	readyAt := mp.readyAt
	if forcedRestart {
		mp.stopRequested = false
		mp.forceRestart = false
		mp.forceRestartReason = ""
		mp.state = StateFailed
	} else if manualStop {
		mp.state = StateStopped
		mp.stoppedAt = time.Now().UTC()
	} else if code == 0 {
		mp.state = StateStopped
		mp.stoppedAt = time.Now().UTC()
	} else {
		mp.state = StateFailed
	}
	mp.mu.Unlock()

	errText := ""
	if waitErr != nil {
		errText = waitErr.Error()
	}
	s.emit(EventExited, id, managed.PID, restartCount, errText)
	if manualStop {
		return
	}

	if spec.RestartPolicy.ResetAfter > 0 && !readyAt.IsZero() && time.Since(readyAt) >= spec.RestartPolicy.ResetAfter {
		mp.mu.Lock()
		if mp.generation == generation {
			mp.restartCount = 0
			restartCount = 0
		}
		mp.mu.Unlock()
	}

	shouldRestart := forcedRestart || spec.RestartPolicy.Mode == RestartAlways || (spec.RestartPolicy.Mode == RestartOnFailure && code != 0)
	restartReason := errText
	if forcedRestart && forcedReason != "" {
		restartReason = forcedReason
	}
	if shouldRestart {
		// scheduleRestart owns the terminal EventFailed emission when the
		// restart budget is exhausted. Returning here avoids duplicate failed
		// events (and avoids reporting a RestartAlways clean exit as stopped
		// when no restart budget remains).
		s.scheduleRestart(id, generation, restartReason)
		return
	}
	if code != 0 {
		s.emit(EventFailed, id, managed.PID, restartCount, errText)
	} else {
		s.emit(EventStopped, id, managed.PID, restartCount, "")
	}
}

func (s *defaultProcessSupervisor) scheduleRestart(id ProcessID, generation uint64, reason string) bool {
	s.mu.RLock()
	mp, ok := s.processes[id]
	stopped := s.stopped
	s.mu.RUnlock()
	if !ok || stopped {
		return false
	}

	mp.mu.Lock()
	if mp.generation != generation || mp.stopRequested {
		mp.mu.Unlock()
		return false
	}
	policy := mp.spec.RestartPolicy
	if policy.Mode == RestartNever || mp.restartCount >= policy.MaxRestarts {
		mp.state = StateFailed
		count := mp.restartCount
		mp.mu.Unlock()
		s.emit(EventFailed, id, 0, count, reason)
		return false
	}
	mp.restartCount++
	count := mp.restartCount
	delay := restartDelay(policy, count)
	if mp.cancelMonitor != nil {
		mp.cancelMonitor()
	}
	monitorCtx, cancel := context.WithCancel(context.Background())
	mp.cancelMonitor = cancel
	mp.state = StateRestartBackoff
	mp.mu.Unlock()

	s.emit(EventRestartScheduled, id, 0, count, reason)
	go func(expectedGeneration uint64, restartCount int) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-monitorCtx.Done():
			return
		case <-timer.C:
		}
		if err := s.Start(context.Background(), id); err != nil {
			// Start records/emits the launch failure and increments the generation.
			// Continue the bounded restart policy from that new generation without
			// emitting the same failure twice.
			s.mu.RLock()
			current := s.processes[id]
			s.mu.RUnlock()
			if current != nil {
				current.mu.Lock()
				newGeneration := current.generation
				current.mu.Unlock()
				s.scheduleRestart(id, newGeneration, err.Error())
			}
			return
		}
		if err := s.WaitReady(context.Background(), id); err != nil {
			s.setLastError(id, err.Error())
			_ = s.stopForHealthRestart(id, err.Error())
			return
		}
		snap, _ := s.Snapshot(id)
		s.emit(EventRestarted, id, snap.PID, restartCount, "")
	}(generation, count)
	return true
}

func restartDelay(policy RestartPolicy, restartCount int) time.Duration {
	base := policy.BaseDelay
	if base <= 0 {
		base = time.Second
	}
	maxDelay := policy.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	delay := base
	for i := 1; i < restartCount && delay < maxDelay; i++ {
		if delay > maxDelay/2 {
			delay = maxDelay
			break
		}
		delay *= 2
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func (s *defaultProcessSupervisor) startHealthMonitor(id ProcessID, generation uint64) {
	s.mu.RLock()
	mp, ok := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		return
	}
	mp.mu.Lock()
	if mp.generation != generation || mp.stopRequested || mp.spec.HealthProbe == nil || mp.state != StateReady {
		mp.mu.Unlock()
		return
	}
	if mp.cancelHealth != nil {
		mp.cancelHealth()
	}
	healthCtx, cancel := context.WithCancel(context.Background())
	mp.cancelHealth = cancel
	interval := mp.spec.HealthInterval
	probe := mp.spec.HealthProbe
	resetAfter := mp.spec.RestartPolicy.ResetAfter
	mp.mu.Unlock()
	if interval <= 0 {
		interval = DefaultHealthInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-healthCtx.Done():
				return
			case <-ticker.C:
			}
			checkCtx, cancelCheck := context.WithTimeout(healthCtx, interval)
			err := probe.Check(checkCtx)
			cancelCheck()

			mp.mu.Lock()
			if mp.generation != generation || mp.stopRequested || mp.state != StateReady {
				mp.mu.Unlock()
				return
			}
			if err == nil {
				mp.healthFailures = 0
				if resetAfter > 0 && mp.restartCount > 0 && !mp.readyAt.IsZero() && time.Since(mp.readyAt) >= resetAfter {
					mp.restartCount = 0
				}
				mp.mu.Unlock()
				continue
			}
			mp.healthFailures++
			failures := mp.healthFailures
			pid := mp.pid
			restartCount := mp.restartCount
			mp.lastError = err.Error()
			mp.mu.Unlock()
			s.emit(EventUnhealthy, id, pid, restartCount, err.Error())
			if failures >= healthFailureThreshold {
				_ = s.stopForHealthRestart(id, err.Error())
				return
			}
		}
	}()
}

func (s *defaultProcessSupervisor) stopForHealthRestart(id ProcessID, reason string) error {
	s.mu.RLock()
	mp, ok := s.processes[id]
	stopped := s.stopped
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	if stopped {
		return ErrHostStopped
	}

	mp.mu.Lock()
	policy := mp.spec.RestartPolicy
	if policy.Mode == RestartNever {
		count := mp.restartCount
		mp.state = StateFailed
		mp.lastError = reason
		mp.mu.Unlock()
		s.emit(EventFailed, id, 0, count, reason)
		return nil
	}
	if mp.state == StateStopping || mp.state == StateStopped || mp.state == StateRestartBackoff {
		mp.mu.Unlock()
		return nil
	}
	if mp.cancelHealth != nil {
		mp.cancelHealth()
		mp.cancelHealth = nil
	}
	if mp.cancelMonitor != nil {
		mp.cancelMonitor()
		mp.cancelMonitor = nil
	}
	mp.stopRequested = false
	mp.forceRestart = true
	mp.forceRestartReason = reason
	mp.lastError = reason
	mp.state = StateStopping
	spec := mp.spec.Clone()
	pid := mp.pid
	handle := mp.procHandle
	restartCount := mp.restartCount
	mp.mu.Unlock()

	s.emit(EventStopping, id, pid, restartCount, reason)
	if pid <= 0 {
		// There is no live process whose wait goroutine can schedule the restart.
		mp.mu.Lock()
		mp.forceRestart = false
		mp.forceRestartReason = ""
		mp.state = StateFailed
		generation := mp.generation
		mp.mu.Unlock()
		if !s.scheduleRestart(id, generation, reason) {
			return fmt.Errorf("restart policy exhausted for %s", id)
		}
		return nil
	}
	if err := s.stopManagedProcess(spec, pid, handle); err != nil {
		s.setLastError(id, err.Error())
		return err
	}
	// waitForExit owns the transition into restart backoff. Keeping that
	// transition in one place prevents a late Wait() from scheduling a second
	// restart after an automated health stop.
	return nil
}

func (s *defaultProcessSupervisor) stopManagedProcess(spec ProcessSpec, pid int, handle process.ProcessTreeHandle) error {
	grace := spec.StopGracePeriod
	if grace <= 0 {
		grace = DefaultStopGracePeriod
	}
	if stopper, ok := spec.ExecutableProcess.(ProcessStopper); ok {
		return stopper.Stop(handle, pid, grace)
	}
	if s.host == nil || s.host.processManager == nil {
		return nil
	}
	return s.host.processManager.Stop(pid, handle, grace)
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
	if host == nil || host.processManager == nil {
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
	if spec.RestartPolicy.BaseDelay <= 0 {
		spec.RestartPolicy.BaseDelay = time.Second
	}
	if spec.RestartPolicy.MaxDelay <= 0 {
		spec.RestartPolicy.MaxDelay = 30 * time.Second
	}
	if spec.RestartPolicy.MaxDelay < spec.RestartPolicy.BaseDelay {
		spec.RestartPolicy.MaxDelay = spec.RestartPolicy.BaseDelay
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
