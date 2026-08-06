// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux

package qdrantprocess

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	defaultGracePeriod = 10 * time.Second
	finalWaitPeriod    = 5 * time.Second
)

type linuxTerminator struct {
	inspector ProcessInspector
}

func newPlatformTerminator() ProcessTerminator {
	return linuxTerminator{}
}

func (t linuxTerminator) Terminate(ctx context.Context, id ProcessIdentity, gracePeriod time.Duration) (TerminationResult, error) {
	if t.inspector == nil {
		t.inspector = NewProcessInspector()
	}

	actual, err := t.inspector.Inspect(ctx, id.PID)
	if err != nil {
		if os.IsNotExist(err) {
			return TerminationAlreadyExited, nil
		}
		return "", err
	}
	if !SameProcessIdentity(id, actual) {
		return "", ErrProcessIdentityMismatch
	}

	if err := syscall.Kill(id.PID, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return TerminationAlreadyExited, nil
		}
		return "", fmt.Errorf("qdrantprocess: SIGTERM: %w", err)
	}

	if gracePeriod <= 0 {
		gracePeriod = defaultGracePeriod
	}
	deadline := time.Now().Add(gracePeriod)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
		_, err := t.inspector.Inspect(ctx, id.PID)
		if err != nil {
			return TerminationGraceful, nil
		}
	}

	actual, err = t.inspector.Inspect(ctx, id.PID)
	if err != nil {
		if os.IsNotExist(err) {
			return TerminationGraceful, nil
		}
		return "", err
	}
	if !SameProcessIdentity(id, actual) {
		return TerminationGraceful, nil
	}

	if err := syscall.Kill(id.PID, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return TerminationGraceful, nil
		}
		return "", fmt.Errorf("qdrantprocess: SIGKILL: %w", err)
	}

	finalDeadline := time.Now().Add(finalWaitPeriod)
	for time.Now().Before(finalDeadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
		_, err := t.inspector.Inspect(ctx, id.PID)
		if err != nil {
			return TerminationForced, nil
		}
	}
	return TerminationForced, fmt.Errorf("%w: process still alive after SIGKILL", ErrOrphanTerminationFailed)
}
