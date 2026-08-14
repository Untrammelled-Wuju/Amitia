// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build ios && cgo
// +build ios,cgo

package sandbox

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../third_party/ish/amitia
#cgo LDFLAGS: -L${SRCDIR}/../../../../third_party/ish/amitia -lamitia_ish

#include "amitia_ish_embed.h"
#include <stdlib.h>
#include <string.h>

static char **alloc_string_array(int n) {
    return (char **)malloc(sizeof(char *) * n);
}

static void free_string_array(char **arr, int n) {
    if (arr == NULL) return;
    for (int i = 0; i < n; i++) {
        free(arr[i]);
    }
    free(arr);
}

static const char **to_const_array(char **arr) {
    return (const char **)arr;
}

static amitia_ish_command_t make_command(
    int argc,
    const char **argv,
    const void *stdin_data,
    size_t stdin_size,
    const char *workdir,
    uint32_t timeout_ms
) {
    amitia_ish_command_t cmd = {0};
    cmd.argc = argc;
    cmd.argv = argv;
    cmd.stdin_data = stdin_data;
    cmd.stdin_size = stdin_size;
    cmd.workdir = workdir;
    cmd.timeout_ms = timeout_ms;
    return cmd;
}
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"
)

type iosNativeBridgeClient struct {
	mu           sync.RWMutex
	state        BackendAvailability
	lifecycle    SandboxLifecycleState
	generation   uint64
	desiredRun   bool
	restartReq   bool
	recoveryPend bool
	rootVer      string
	rootDigest   string
	lastErr      string
	startedAt    time.Time
	activeExecID string
	runtimeID    string

	lifecycleMu sync.Mutex
	startCancel context.CancelFunc
	startDone   chan struct{}
}

func newIOSNativeBridgeClient() *iosNativeBridgeClient {
	return &iosNativeBridgeClient{
		state:     BackendUnavailable,
		lifecycle: SandboxStateIdle,
	}
}

func (b *iosNativeBridgeClient) Availability(_ context.Context) BackendAvailability {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *iosNativeBridgeClient) Start(ctx context.Context, cfg SandboxConfig) error {
	if cfg.RuntimeID == "" {
		return &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: SandboxStateIdle,
			Cause: fmt.Errorf("runtime ID is required"),
		}
	}

	if cfg.RootfsURI == "" {
		return &SandboxLifecycleError{
			Code:  SandboxErrRootfsInvalid,
			State: SandboxStateIdle,
			Cause: fmt.Errorf("rootfs URI is required"),
		}
	}

	b.lifecycleMu.Lock()

	b.mu.RLock()
	currentState := b.lifecycle
	b.mu.RUnlock()

	if currentState == SandboxStateRunning && b.runtimeID == cfg.RuntimeID && !b.restartReq {
		b.lifecycleMu.Unlock()
		return nil
	}

	if currentState == SandboxStateStarting {
		prevDone := b.startDone
		b.lifecycleMu.Unlock()
		if prevDone != nil {
			select {
			case <-prevDone:
			case <-ctx.Done():
				return &SandboxLifecycleError{
					Code:  SandboxErrStartCancelled,
					State: SandboxStateStarting,
					Cause: ctx.Err(),
				}
			}
			b.mu.RLock()
			nowRunning := b.lifecycle == SandboxStateRunning
			b.mu.RUnlock()
			if nowRunning {
				return nil
			}
		}
		b.lifecycleMu.Lock()
	}

	if !b.tryTransitionLocked(SandboxStateIdle, SandboxStateStarting) && !b.tryTransitionLocked(SandboxStateFailed, SandboxStateStarting) {
		b.lifecycleMu.Unlock()
		return &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: b.lifecycle,
			Cause: fmt.Errorf("cannot start from state %s", b.lifecycle),
		}
	}

	startCtx, cancel := context.WithCancel(ctx)
	b.startCancel = cancel
	b.startDone = make(chan struct{})
	b.runtimeID = cfg.runtimeID()
	b.desiredRun = true
	b.mu.Unlock()

	b.mu.Lock()
	b.lastErr = ""
	b.recoveryPend = false
	b.mu.Unlock()

	err := b.doStartNative(startCtx, cfg)

	close(b.startDone)
	b.startCancel = nil

	if err != nil {
		b.mu.Lock()
		b.state = BackendError
		b.lifecycle = SandboxStateFailed
		b.lastErr = b.extractErrorCodeLocked(err)
		b.activeExecID = ""
		if b.desiredRun {
			b.recoveryPend = true
		}
		b.mu.Unlock()
		b.lifecycleMu.Unlock()
		return &SandboxLifecycleError{
			Code:  SandboxErrStartCancelled,
			State: SandboxStateFailed,
			Cause: err,
		}
	}

	b.mu.Lock()
	b.state = BackendRunning
	b.lifecycle = SandboxStateRunning
	b.generation++
	b.startedAt = time.Now()
	b.recoveryPend = false
	b.restartReq = false
	b.lastErr = ""
	b.mu.Unlock()

	b.lifecycleMu.Unlock()
	return nil
}

func (b *iosNativeBridgeClient) doStartNative(ctx context.Context, cfg SandboxConfig) error {
	guestRootfsPath, err := b.mapWorkspacePath(cfg.RootfsURI)
	if err != nil {
		cancel()
		return err
	}

	guestWorkdir := "/root"
	if cfg.WorkspaceURI != "" {
		mapped, mapErr := b.mapWorkspacePath(cfg.WorkspaceURI)
		if mapErr != nil {
			cancel()
			return mapErr
		}
		guestWorkdir = mapped
	}

	envKeys := []string{"HOME", "PATH", "TMPDIR", "LANG", "TERM", "NO_COLOR"}
	envVals := mapEnvForGuest(cfg.Environment)
	cEnv := buildCEnvArray(envKeys, envVals)
	if cEnv == nil {
		cancel()
		return &NativeBridgeError{Code: NativeErrNativeFailure, Message: "failed to allocate env array"}
	}
	defer C.free_string_array(cEnv, C.int(len(envKeys)))

	cRootfs := C.CString(guestRootfsPath)
	defer C.free(unsafe.Pointer(cRootfs))

	cWorkdir := C.CString(guestWorkdir)
	defer C.free(unsafe.Pointer(cWorkdir))

	cEnvPtr := C.to_const_array(cEnv)

	done := make(chan C.int, 1)
	go func() {
		rc := C.amitia_ish_start(cRootfs, cWorkdir, cEnvPtr, C.size_t(len(envKeys)))
		done <- rc
	}()

	select {
	case <-ctx.Done():
		C.amitia_ish_stop()
		return ctx.Err()
	case rc := <-done:
		if rc != C.AMITIA_ISH_OK {
			return b.mapNativeError(rc, "iSH runtime start failed")
		}
	}

	b.mu.Lock()
	b.rootVer = cfg.RootfsURI
	b.mu.Unlock()

	return nil
}

func (b *iosNativeBridgeClient) Stop(ctx context.Context, reason SandboxStopReason) error {
	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()

	b.mu.Lock()
	if b.lifecycle == SandboxStateIdle {
		b.mu.Unlock()
		return nil
	}
	if b.startCancel != nil {
		b.startCancel()
	}
	b.mu.Unlock()

	if reason == StopReasonUser || reason == StopReasonDisable {
		b.mu.Lock()
		b.desiredRun = false
		b.mu.Unlock()
	}

	b.drainAndStop(ctx, true)

	b.mu.Lock()
	b.state = BackendUnavailable
	b.lifecycle = SandboxStateIdle
	b.activeExecID = ""
	b.recoveryPend = false
	b.mu.Unlock()

	return nil
}

func (b *iosNativeBridgeClient) Execute(ctx context.Context, req SandboxExecuteRequest) (SandboxExecuteResult, error) {
	result := SandboxExecuteResult{
		ExecutionID: req.ExecutionID,
		StartedAt:   time.Now(),
	}
	result.FinishedAt = result.StartedAt

	b.mu.RLock()
	if !CanExecuteInState(b.lifecycle) {
		currentState := b.lifecycle
		b.mu.RUnlock()
		return result, &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: currentState,
			Cause: fmt.Errorf("execute not allowed in state %s", currentState),
		}
	}
	currentGen := b.generation
	b.mu.RUnlock()

	if len(req.Argv) == 0 {
		return result, &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "empty argv"}
	}

	cArgv := C.alloc_string_array(C.int(len(req.Argv)))
	if cArgv == nil {
		return result, &NativeBridgeError{Code: NativeErrNativeFailure, Message: "failed to allocate argv"}
	}
	defer C.free_string_array(cArgv, C.int(len(req.Argv)))

	for i, arg := range req.Argv {
		cStr := C.CString(arg)
		ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cArgv)) + uintptr(i)*unsafe.Sizeof(cStr)))
		*ptr = cStr
	}

	var cStdin unsafe.Pointer
	if len(req.Stdin) > 0 {
		cStdin = C.CBytes(req.Stdin)
		defer C.free(cStdin)
	}

	var cWorkdir *C.char
	if req.WorkingDirectoryURI != "" {
		guestCwd := b.mapURIToGuestPath(req.WorkingDirectoryURI)
		cWorkdir = C.CString(guestCwd)
		defer C.free(unsafe.Pointer(cWorkdir))
	}

	cmd := C.make_command(
		C.int(len(req.Argv)),
		C.to_const_array(cArgv),
		cStdin,
		C.size_t(len(req.Stdin)),
		cWorkdir,
		C.uint32_t(req.TimeoutSeconds*1000),
	)

	result.Generation = currentGen
	b.mu.Lock()
	b.activeExecID = req.ExecutionID
	b.mu.Unlock()

	var nativeResult C.amitia_ish_result_t
	rc := C.amitia_ish_execute(&cmd, &nativeResult)

	b.mu.Lock()
	b.activeExecID = ""
	if nativeResult.fatal {
		b.state = BackendError
		b.lifecycle = SandboxStateFailed
		b.lastErr = NativeErrKernelFailure
		if b.desiredRun {
			b.recoveryPend = true
		}
		b.handleCrashLocked()
	}
	b.mu.Unlock()

	if rc != C.AMITIA_ISH_OK {
		errMsg := ""
		if nativeResult.error_message != nil {
			errMsg = C.GoString(nativeResult.error_message)
		}
		C.amitia_ish_result_free(&nativeResult)
		return result, b.mapNativeError(rc, errMsg)
	}

	result.ExitCode = int(nativeResult.exit_code)
	result.Fatal = bool(nativeResult.fatal)

	if nativeResult.stdout_data != nil && nativeResult.stdout_size > 0 {
		result.Stdout = C.GoBytes(unsafe.Pointer(nativeResult.stdout_data), C.int(nativeResult.stdout_size))
	}
	if nativeResult.stderr_data != nil && nativeResult.stderr_size > 0 {
		result.Stderr = C.GoBytes(unsafe.Pointer(nativeResult.stderr_data), C.int(nativeResult.stderr_size))
	}

	C.amitia_ish_result_free(&nativeResult)

	result.FinishedAt = time.Now()

	b.mu.RLock()
	defer b.mu.RUnlock()
	if result.Generation != b.generation {
		return result, &SandboxLifecycleError{
			Code:  SandboxErrStaleGeneration,
			State: b.lifecycle,
			Cause: fmt.Errorf("execution started gen=%d, current gen=%d", result.Generation, b.generation),
		}
	}

	return result, nil
}

func (b *iosNativeBridgeClient) Cancel(ctx context.Context, executionID string) error {
	if executionID == "" {
		b.mu.RLock()
		currentExec := b.activeExecID
		b.mu.RUnlock()
		if currentExec == "" {
			return nil
		}
		executionID = currentExec
	}

	goExecID, err := parseExecutionID(executionID)
	if err != nil {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: fmt.Sprintf("invalid execution ID: %s", executionID)}
	}

	rc := C.amitia_ish_cancel(C.uint64_t(goExecID))
	if rc != C.AMITIA_ISH_OK {
		return b.mapNativeError(rc, "cancel failed")
	}
	return nil
}

func (b *iosNativeBridgeClient) Health(_ context.Context) SandboxHealth {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := C.amitia_ish_state()
	isRunning := state == C.AMITIA_ISH_RUNNING

	return SandboxHealth{
		Healthy:              isRunning && b.desiredRun && b.lifecycle == SandboxStateRunning,
		Message:              string(b.lifecycle),
		ISHInitialized:       isRunning,
		RootfsInstalled:      b.rootVer != "",
		LifecycleState:       string(b.lifecycle),
		Generation:           b.generation,
		DesiredRunning:       b.desiredRun,
		RestartRequired:      b.restartReq,
		RecoveryPending:      b.recoveryPend,
		ActiveExecutionID:    b.activeExecID,
		RunningRootfsVersion: b.rootVer,
		RunningRootfsDigest:  b.rootDigest,
		LastErrorCode:        b.lastErr,
	}
}

func (b *iosNativeBridgeClient) RootfsStatus(_ context.Context) (RootfsStatus, error) {
	return RootfsStatus{}, &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "rootfs provisioning not supported without native iOS host",
	}
}

func (b *iosNativeBridgeClient) EnsureRootfs(_ context.Context, _ RootfsInstallSpec) (RootfsInstallResult, error) {
	return RootfsInstallResult{}, &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "rootfs provisioning not supported without native iOS host",
	}
}

func (b *iosNativeBridgeClient) ActivateRootfs(_ context.Context, _ string) error {
	return &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "rootfs provisioning not supported without native iOS host",
	}
}

func (b *iosNativeBridgeClient) RemoveRootfs(_ context.Context, _ string) error {
	return &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "rootfs provisioning not supported without native iOS host",
	}
}

func (b *iosNativeBridgeClient) Quiesce(ctx context.Context) error {
	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()

	if !b.tryTransitionLocked(SandboxStateRunning, SandboxStateQuiescing) {
		return &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: b.lifecycle,
			Cause: fmt.Errorf("quiesce requires running state"),
		}
	}

	b.drainAndStop(ctx, false)

	if !b.tryTransitionLocked(SandboxStateQuiescing, SandboxStateQuiesced) {
		return &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: b.lifecycle,
			Cause: fmt.Errorf("quiesce transition failed"),
		}
	}

	return nil
}

func (b *iosNativeBridgeClient) Resume(ctx context.Context) error {
	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()

	if !b.tryTransitionLocked(SandboxStateQuiesced, SandboxStateRunning) {
		return &SandboxLifecycleError{
			Code:  SandboxErrInvalidState,
			State: b.lifecycle,
			Cause: fmt.Errorf("resume requires quiesced state"),
		}
	}

	return nil
}

func (b *iosNativeBridgeClient) Restart(ctx context.Context, reason SandboxRestartReason) error {
	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()

	previousLifecycle := b.lifecycle

	if !b.tryTransitionLocked(SandboxStateRunning, SandboxStateStopping) &&
		!b.tryTransitionLocked(SandboxStateFailed, SandboxStateStopping) {
		b.tryTransitionLocked(SandboxStateRecoveryPending, SandboxStateStopping)
	}

	b.drainAndStop(ctx, true)
	b.mu.Lock()
	b.state = BackendUnavailable
	b.mu.Unlock()

	if !b.tryTransitionLocked(SandboxStateStopping, SandboxStateStarting) {
		return &SandboxLifecycleError{
			Code:  SandboxErrRestartFailed,
			State: b.lifecycle,
			Cause: fmt.Errorf("restart transition failed"),
		}
	}

	b.mu.Lock()
	b.runtimeID = b.runtimeID
	b.lastErr = ""
	b.mu.Unlock()

	err := b.doStartNative(ctx, SandboxConfig{
		RuntimeID:    b.runtimeID,
		RootfsURI:    b.rootVer,
		WorkspaceURI: "",
	})

	if err != nil {
		b.mu.Lock()
		b.lifecycle = SandboxStateFailed
		b.lastErr = b.extractErrorCodeLocked(err)
		if b.desiredRun {
			b.recoveryPend = true
		}
		b.restartReq = previousLifecycle == SandboxStateRunning
		b.mu.Unlock()
		return &SandboxLifecycleError{
			Code:  SandboxErrRestartFailed,
			State: SandboxStateFailed,
			Cause: err,
		}
	}

	b.mu.Lock()
	b.state = BackendRunning
	b.lifecycle = SandboxStateRunning
	b.generation++
	b.recoveryPend = false
	b.restartReq = false
	b.startedAt = time.Now()
	b.mu.Unlock()

	return nil
}

func (b *iosNativeBridgeClient) Recover(ctx context.Context) error {
	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()

	if !b.tryTransitionLocked(SandboxStateRecoveryPending, SandboxStateRecovering) &&
		!b.tryTransitionLocked(SandboxStateFailed, SandboxStateRecovering) {
		return &SandboxLifecycleError{
			Code:  SandboxErrRecoveryFailed,
			State: b.lifecycle,
			Cause: fmt.Errorf("recover requires recovery_pending or failed state"),
		}
	}

	b.drainAndStop(ctx, false)
	b.mu.Lock()
	b.state = BackendUnavailable
	b.mu.Unlock()

	b.mu.Lock()
	b.runtimeID = b.runtimeID
	b.lastErr = ""
	b.mu.Unlock()

	err := b.doStartNative(ctx, SandboxConfig{
		RuntimeID: b.runtimeID,
		RootfsURI: b.rootVer,
	})

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.lifecycle = SandboxStateFailed
		b.lastErr = b.extractErrorCodeLocked(err)
		if b.desiredRun {
			b.recoveryPend = true
		}
		return &SandboxLifecycleError{
			Code:  SandboxErrRecoveryFailed,
			State: SandboxStateFailed,
			Cause: err,
		}
	}

	b.state = BackendRunning
	b.lifecycle = SandboxStateRunning
	b.generation++
	b.recoveryPend = false
	b.startedAt = time.Now()
	return nil
}

func (b *iosNativeBridgeClient) LifecycleState(_ context.Context) SandboxLifecycleState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lifecycle
}

func (b *iosNativeBridgeClient) RecoverySnapshot(_ context.Context) SandboxRecoverySnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return SandboxRecoverySnapshot{
		RuntimeID:            b.runtimeID,
		LifecycleState:       b.lifecycle,
		Generation:           b.generation,
		DesiredRunning:       b.desiredRun,
		RecoveryPending:      b.recoveryPend,
		RestartRequired:      b.restartReq,
		ActiveExecutionID:    b.activeExecID,
		ActiveRootfsVersion:  b.rootVer,
		ActiveRootfsDigest:   b.rootDigest,
		RunningRootfsVersion: b.rootVer,
		RunningRootfsDigest:  b.rootDigest,
		RootfsInstalled:      b.rootVer != "",
		LastErrorCode:        b.lastErr,
	}
}

func (b *iosNativeBridgeClient) tryTransitionLocked(from, to SandboxLifecycleState) bool {
	if b.lifecycle != from {
		return false
	}
	if !CanTransitionSandboxState(from, to) {
		return false
	}
	b.lifecycle = to
	return true
}

func (b *iosNativeBridgeClient) drainAndStop(ctx context.Context, fullStop bool) {
	b.mu.Lock()
	execID := b.activeExecID
	b.mu.Unlock()

	if execID != "" {
		drainTimer := time.NewTimer(CommandDrainTimeout)
		defer drainTimer.Stop()

		b.mu.RLock()
		execFinished := b.activeExecID == ""
		b.mu.RUnlock()

		if !execFinished {
			select {
			case <-drainTimer.C:
			case <-ctx.Done():
			}
		}

		b.mu.RLock()
		stillActive := b.activeExecID != ""
		b.mu.RUnlock()

		if stillActive {
			_ = b.Cancel(ctx, execID)
			cancelTimer := time.NewTimer(CommandCancelTimeout)
			defer cancelTimer.Stop()
			select {
			case <-cancelTimer.C:
			case <-ctx.Done():
			}
		}
	}

	if fullStop {
		C.amitia_ish_stop()
	}
}

func (b *iosNativeBridgeClient) extractErrorCodeLocked(err error) string {
	if err == nil {
		return ""
	}
	if le, ok := err.(*SandboxLifecycleError); ok {
		return le.Code
	}
	if ne, ok := err.(*NativeBridgeError); ok {
		return ne.Code
	}
	if re, ok := err.(*RootfsError); ok {
		return re.Code
	}
	return NativeErrNativeFailure
}

func (b *iosNativeBridgeClient) handleCrashLocked() {
	if b.desiredRun {
		b.lifecycle = SandboxStateRecoveryPending
		b.recoveryPend = true
	}
	b.activeExecID = ""
}

func (b *iosNativeBridgeClient) mapWorkspacePath(uri string) (string, error) {
	if uri == "" {
		return "", nil
	}
	if len(uri) > 8 && uri[:8] == "amitia://" {
		guestPath := "/" + uri[8:]
		return guestPath, nil
	}
	return uri, nil
}

func (b *iosNativeBridgeClient) mapURIToGuestPath(uri string) string {
	if uri == "" {
		return ""
	}
	if len(uri) > 8 && uri[:8] == "amitia://" {
		return "/" + uri[8:]
	}
	return uri
}

func (b *iosNativeBridgeClient) runtimeID() string {
	return b.runtimeID
}

func (b *iosNativeBridgeClient) mapNativeError(rc C.int, detail string) error {
	code := NativeErrNativeFailure
	switch rc {
	case C.AMITIA_ISH_ERR_NOT_INITIALIZED:
		code = NativeErrRuntimeNotStarted
	case C.AMITIA_ISH_ERR_ROOTFS_NOT_READY:
		code = NativeErrRootfsUnavailable
	case C.AMITIA_ISH_ERR_INVALID_ARGUMENT:
		code = NativeErrExecutionInvalid
	case C.AMITIA_ISH_ERR_EXEC_BUSY:
		code = NativeErrExecutionBusy
	case C.AMITIA_ISH_ERR_EXEC_CANCELLED:
		code = NativeErrExecutionCancelled
	case C.AMITIA_ISH_ERR_EXEC_TIMEOUT:
		code = NativeErrExecutionTimeout
	case C.AMITIA_ISH_ERR_INTERNAL:
		code = NativeErrKernelFailure
	}
	msg := detail
	if msg == "" {
		msg = fmt.Sprintf("native error %d", int(rc))
	}
	return &NativeBridgeError{Code: code, Message: msg}
}

func buildCEnvArray(keys []string, vals map[string]string) **C.char {
	count := len(keys)
	arr := C.alloc_string_array(C.int(count))
	if arr == nil {
		return nil
	}
	for i, k := range keys {
		v := vals[k]
		pair := k + "=" + v
		cStr := C.CString(pair)
		ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(arr)) + uintptr(i)*unsafe.Sizeof(cStr)))
		*ptr = cStr
	}
	return arr
}

func mapEnvForGuest(env map[string]string) map[string]string {
	result := map[string]string{
		"HOME":     "/root",
		"PATH":     "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TMPDIR":   "/tmp",
		"LANG":     "C.UTF-8",
		"TERM":     "xterm-256color",
		"NO_COLOR": "1",
	}
	for k, v := range env {
		result[k] = v
	}
	return result
}

func parseExecutionID(s string) (uint64, error) {
	var id uint64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

type unavailableNativeBridge struct {
	reason string
}

func (b *unavailableNativeBridge) Availability(_ context.Context) BackendAvailability {
	return BackendUnavailable
}

func (b *unavailableNativeBridge) Start(_ context.Context, _ SandboxConfig) error {
	return &NativeBridgeError{Code: NativeErrRuntimeNotStarted, Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) Stop(_ context.Context, _ SandboxStopReason) error {
	return nil
}

func (b *unavailableNativeBridge) Execute(_ context.Context, _ SandboxExecuteRequest) (SandboxExecuteResult, error) {
	return SandboxExecuteResult{}, &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason),
	}
}

func (b *unavailableNativeBridge) Cancel(_ context.Context, _ string) error {
	return &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason),
	}
}

func (b *unavailableNativeBridge) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy:        false,
		Message:        fmt.Sprintf("unavailable: %s", b.reason),
		LifecycleState: string(SandboxStateIdle),
	}
}

func (b *unavailableNativeBridge) RootfsStatus(_ context.Context) (RootfsStatus, error) {
	return RootfsStatus{}, &RootfsError{Code: RootfsErrNotConfigured, Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) EnsureRootfs(_ context.Context, _ RootfsInstallSpec) (RootfsInstallResult, error) {
	return RootfsInstallResult{}, &RootfsError{Code: RootfsErrNotConfigured, Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) ActivateRootfs(_ context.Context, _ string) error {
	return &RootfsError{Code: RootfsErrNotConfigured, Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) RemoveRootfs(_ context.Context, _ string) error {
	return &RootfsError{Code: RootfsErrNotConfigured, Message: fmt.Sprintf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) Quiesce(_ context.Context) error {
	return &SandboxLifecycleError{Code: SandboxErrInvalidState, State: SandboxStateIdle, Cause: fmt.Errorf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) Resume(_ context.Context) error {
	return &SandboxLifecycleError{Code: SandboxErrInvalidState, State: SandboxStateIdle, Cause: fmt.Errorf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) Restart(_ context.Context, _ SandboxRestartReason) error {
	return &SandboxLifecycleError{Code: SandboxErrRestartFailed, State: SandboxStateIdle, Cause: fmt.Errorf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) Recover(_ context.Context) error {
	return &SandboxLifecycleError{Code: SandboxErrRecoveryFailed, State: SandboxStateIdle, Cause: fmt.Errorf("iSH native bridge unavailable: %s", b.reason)}
}

func (b *unavailableNativeBridge) LifecycleState(_ context.Context) SandboxLifecycleState {
	return SandboxStateIdle
}

func (b *unavailableNativeBridge) RecoverySnapshot(_ context.Context) SandboxRecoverySnapshot {
	return SandboxRecoverySnapshot{
		LifecycleState: SandboxStateIdle,
	}
}
