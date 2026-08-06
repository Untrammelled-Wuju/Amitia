// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package qdrantprocess

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

type windowsInspector struct{}

func newPlatformInspector() ProcessInspector {
	return windowsInspector{}
}

func (windowsInspector) Inspect(ctx context.Context, pid int) (ProcessIdentity, error) {
	return inspectProcess(ctx, pid)
}

func (windowsInspector) IsAlive(ctx context.Context, id ProcessIdentity) (bool, error) {
	actual, err := inspectProcess(ctx, id.PID)
	if err != nil {
		return false, err
	}
	if !SameProcessIdentity(id, actual) {
		return false, nil
	}
	return true, nil
}

func inspectProcess(ctx context.Context, pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("%w: invalid PID %d", ErrProcessInspectionFailed, pid)
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("%w: OpenProcess: %v", ErrProcessInspectionFailed, err)
	}
	defer windows.CloseHandle(handle)

	exe, err := queryProcessExe(handle)
	if err != nil {
		return ProcessIdentity{}, err
	}

	startMarker, err := queryStartMarker(handle)
	if err != nil {
		return ProcessIdentity{}, err
	}

	return ProcessIdentity{
		PID:            pid,
		ExecutablePath: exe,
		StartMarker:    startMarker,
	}, nil
}

func queryProcessExe(handle windows.Handle) (string, error) {
	var exePath [windows.MAX_PATH]uint16
	pathLen := uint32(len(exePath))
	if err := windows.QueryFullProcessImageName(handle, 0, &exePath[0], &pathLen); err != nil {
		return "", fmt.Errorf("%w: QueryFullProcessImageName: %v", ErrProcessInspectionFailed, err)
	}
	return normalizeWindowsPath(windows.UTF16ToString(exePath[:pathLen])), nil
}

func queryStartMarker(handle windows.Handle) (string, error) {
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	if err := windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return "", fmt.Errorf("%w: GetProcessTimes: %v", ErrProcessInspectionFailed, err)
	}
	return fmt.Sprintf("%016x", creationTimeToFileTimeValue(creationTime)), nil
}

func creationTimeToFileTimeValue(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}
