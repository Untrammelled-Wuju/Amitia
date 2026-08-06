// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import "fmt"

var (
	ErrOwnershipRootUnavailable     = fmt.Errorf("qdrantprocess: ownership root unavailable")
	ErrOwnershipRecordNotFound      = fmt.Errorf("qdrantprocess: ownership record not found")
	ErrOwnershipRecordCorrupted     = fmt.Errorf("qdrantprocess: ownership record corrupted")
	ErrOwnershipRecordInvalid       = fmt.Errorf("qdrantprocess: ownership record invalid")
	ErrOwnershipLeaseExists         = fmt.Errorf("qdrantprocess: ownership lease already exists")
	ErrOwnedByLiveRuntime           = fmt.Errorf("qdrantprocess: owned by live runtime")
	ErrLeaseOwnershipLost           = fmt.Errorf("qdrantprocess: lease ownership lost")
	ErrQdrantAlreadyRunning         = fmt.Errorf("qdrantprocess: qdrant already running")
	ErrProcessNotFound              = fmt.Errorf("qdrantprocess: process not found")
	ErrProcessIdentityMismatch      = fmt.Errorf("qdrantprocess: process identity mismatch")
	ErrProcessIdentityConflict      = fmt.Errorf("qdrantprocess: process identity conflict")
	ErrProcessInspectionUnsupported = fmt.Errorf("qdrantprocess: process inspection unsupported")
	ErrProcessInspectionFailed      = fmt.Errorf("qdrantprocess: process inspection failed")
	ErrOrphanTerminationUnsupported = fmt.Errorf("qdrantprocess: orphan termination unsupported")
	ErrOrphanTerminationFailed      = fmt.Errorf("qdrantprocess: orphan termination failed")
	ErrChildAttachFailed            = fmt.Errorf("qdrantprocess: child attach failed")
	ErrProcessStillRunning          = fmt.Errorf("qdrantprocess: process still running")
)

type orphError struct {
	err error
	pid int
}

func (e orphError) Error() string {
	if e.pid > 0 {
		return fmt.Sprintf("qdrantprocess: pid %d: %v", e.pid, e.err)
	}
	return fmt.Sprintf("qdrantprocess: %v", e.err)
}

func (e orphError) Unwrap() error { return e.err }
