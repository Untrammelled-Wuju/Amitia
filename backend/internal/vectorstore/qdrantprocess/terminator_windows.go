// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package qdrantprocess

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

type windowsTerminator struct{}

func newPlatformTerminator() ProcessTerminator {
	return windowsTerminator{}
}

func (windowsTerminator) Terminate(ctx context.Context, id ProcessIdentity, gracePeriod time.Duration) (TerminationResult, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(id.PID))
	if err != nil {
		return "", fmt.Errorf("%w: OpenProcess: %v", ErrOrphanTerminationFailed, err)
	}
	defer windows.CloseHandle(handle)

	var exePath [windows.MAX_PATH]uint16
	pathLen := uint32(len(exePath))
	if err := windows.QueryFullProcessImageName(handle, 0, &exePath[0], &pathLen); err != nil {
		return "", fmt.Errorf("%w: QueryFullProcessImageName: %v", ErrOrphanTerminationFailed, err)
	}
	exe := windows.UTF16ToString(exePath[:pathLen])
	exe = normalizeWindowsPath(exe)
	if !SameExecutablePath(exe, id.ExecutablePath) {
		return "", ErrProcessIdentityMismatch
	}

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return "", fmt.Errorf("%w: TerminateProcess: %v", ErrOrphanTerminationFailed, err)
	}

	waitDeadline := time.Now().Add(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(waitDeadline) {
		var exitCode uint32
		if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
			return "", err
		}
		if exitCode != 259 {
			return TerminationForced, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
	return TerminationForced, fmt.Errorf("%w: process still running after TerminateProcess", ErrOrphanTerminationFailed)
}
