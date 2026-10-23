// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"testing"
	"time"
)

func TestScheduledRestartIsCancelledByShutdown(t *testing.T) {
	supervisor, ctx, cancel := newTestSupervisor(t)
	defer cancel()
	concrete := supervisor.(*defaultProcessSupervisor)
	spec := ProcessSpec{
		ID:         "scheduled.restart",
		Executable: testExecutable(t),
		WorkingDir: t.TempDir(),
		RestartPolicy: RestartPolicy{
			Mode:      RestartOnFailure,
			BaseDelay: 200 * time.Millisecond,
		},
	}
	if err := supervisor.Register(spec); err != nil {
		t.Fatal(err)
	}
	concrete.mu.RLock()
	mp := concrete.processes[spec.ID]
	concrete.mu.RUnlock()
	mp.mu.Lock()
	mp.state = StateRestartBackoff
	generation := mp.generation
	mp.mu.Unlock()
	if !concrete.scheduledRestartAllowed(spec.ID, generation) {
		t.Fatal("scheduled restart should be allowed before shutdown")
	}
	if err := supervisor.StopAll(ctx); err != nil {
		t.Fatal(err)
	}
	if concrete.scheduledRestartAllowed(spec.ID, generation) {
		t.Fatal("scheduled restart must be rejected after shutdown")
	}
}
