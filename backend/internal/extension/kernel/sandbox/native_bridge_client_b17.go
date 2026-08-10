// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build ios && cgo
// +build ios,cgo

package sandbox

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../third_party/ish/amitia
#cgo LDFLAGS: -L${SRCDIR}/../../../../../third_party/ish/amitia -lamitia_ish

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
	lifecycle    string
	generation   uint64
	desiredRun   bool
	restartReq   bool
	recoveryPend bool
	rootVer      string
	rootDigest   string
	lastErr      string
	startedAt    time.Time
}

func newIOSNativeBridgeClient() *iosNativeBridgeClient {
	return &iosNativeBridgeClient{state: BackendUnavailable}
}

func (b *iosNativeBridgeClient) Availability(_ context.Context) BackendAvailability {
	state := C.amitia_ish_state()
	switch state {
	case C.AMITIA_ISH_RUNNING:
		return BackendRunning
	case C.AMITIA_ISH_STARTING:
		return BackendStarting
	case C.AMITIA_ISH_UNAVAILABLE:
		return BackendUnavailable
	case C.AMITIA_ISH_ERROR:
		return BackendError
	default:
		return BackendUnavailable
	}
}

func (b *iosNativeBridgeClient) Start(ctx context.Context, cfg SandboxConfig) error {
	if cfg.RootfsURI == "" {
		return &NativeBridgeError{Code: NativeErrRootfsUnavailable, Message: "rootfs URI is required"}
	}

	guestRootfsPath, err := b.mapWorkspacePath(cfg.RootfsURI)
	if err != nil {
		return err
	}

	guestWorkdir := "/root"
	if cfg.WorkspaceURI != "" {
		mapped, mapErr := b.mapWorkspacePath(cfg.WorkspaceURI)
		if mapErr != nil {
			return mapErr
		}
		guestWorkdir = mapped
	}

	envKeys := []string{"HOME", "PATH", "TMPDIR", "LANG", "TERM", "NO_COLOR"}
	envVals := mapEnvForGuest(cfg.Environment)
	cEnv := buildCEnvArray(envKeys, envVals)
	defer C.free_string_array(cEnv, C.int(len(envKeys)))

	cRootfs := C.CString(guestRootfsPath)
	defer C.free(unsafe.Pointer(cRootfs))

	cWorkdir := C.CString(guestWorkdir)
	defer C.free(unsafe.Pointer(cWorkdir))

	cEnvPtr := C.to_const_array(cEnv)
	rc := C.amitia_ish_start(cRootfs, cWorkdir, cEnvPtr, C.size_t(len(envKeys)))

	if rc != C.AMITIA_ISH_OK {
		return b.mapNativeError(rc, "iSH runtime start failed")
	}

	b.mu.Lock()
	b.state = BackendRunning
	b.desiredRun = true
	b.generation++
	b.startedAt = time.Now()
	b.lifecycle = "running"
	b.restartReq = false
	b.recoveryPend = false
	b.mu.Unlock()

	return nil
}

func (b *iosNativeBridgeClient) Stop(_ context.Context) error {
	C.amitia_ish_stop()

	b.mu.Lock()
	b.state = BackendUnavailable
	b.lifecycle = "idle"
	b.desiredRun = false
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

	var nativeResult C.amitia_ish_result_t
	rc := C.amitia_ish_execute(&cmd, &nativeResult)

	if rc != C.AMITIA_ISH_OK {
		errMsg := ""
		if nativeResult.error_message != nil {
			errMsg = C.GoString(nativeResult.error_message)
		}
		C.amitia_ish_result_free(&nativeResult)
		return result, b.mapNativeError(rc, errMsg)
	}

	result.ExitCode = int(nativeResult.exit_code)
	result.Generation = uint64(nativeResult.generation)
	result.Fatal = bool(nativeResult.fatal)

	if nativeResult.stdout_data != nil && nativeResult.stdout_size > 0 {
		result.Stdout = C.GoBytes(unsafe.Pointer(nativeResult.stdout_data), C.int(nativeResult.stdout_size))
	}
	if nativeResult.stderr_data != nil && nativeResult.stderr_size > 0 {
		result.Stderr = C.GoBytes(unsafe.Pointer(nativeResult.stderr_data), C.int(nativeResult.stderr_size))
	}

	C.amitia_ish_result_free(&nativeResult)

	b.mu.Lock()
	if nativeResult.fatal {
		b.state = BackendError
		b.lastErr = NativeErrKernelFailure
	}
	b.mu.Unlock()

	result.FinishedAt = time.Now()

	if ctxErr := ctx.Err(); ctxErr != nil {
		b.mu.Lock()
		b.lifecycle = "cancelled"
		b.mu.Unlock()
		return result, ctxErr
	}

	return result, nil
}

func (b *iosNativeBridgeClient) Cancel(_ context.Context, executionID string) error {
	if executionID == "" {
		b.mu.RLock()
		currentExec := b.activeExecutionID()
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
		Healthy:              isRunning && b.desiredRun,
		Message:              b.lifecycle,
		ISHInitialized:       isRunning,
		RootfsInstalled:      b.rootVer != "",
		LifecycleState:       b.lifecycle,
		Generation:           b.generation,
		DesiredRunning:       b.desiredRun,
		RestartRequired:      b.restartReq,
		RecoveryPending:      b.recoveryPend,
		RunningRootfsVersion: b.rootVer,
		RunningRootfsDigest:  b.rootDigest,
		LastErrorCode:        b.lastErr,
	}
}

func (b *iosNativeBridgeClient) activeExecutionID() string {
	return ""
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
		"HOME":    "/root",
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TMPDIR":  "/tmp",
		"LANG":    "C.UTF-8",
		"TERM":    "xterm-256color",
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

func (b *unavailableNativeBridge) Stop(_ context.Context) error {
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
		LifecycleState: "idle",
	}
}
