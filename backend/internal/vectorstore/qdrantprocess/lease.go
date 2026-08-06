// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"fmt"
	"os"
	"sync"
)

type TerminationResult string

const (
	TerminationAlreadyExited TerminationResult = "AlreadyExited"
	TerminationGraceful      TerminationResult = "Graceful"
	TerminationForced        TerminationResult = "Forced"
)

type Lease interface {
	LaunchID() string
	Record() OwnershipRecord

	AttachChild(ctx context.Context, pid int) error
	MarkStopping(ctx context.Context) error
	MarkExited(ctx context.Context) error
	MarkFailed(ctx context.Context) error
	Release(ctx context.Context) error
}

type ownershipLease struct {
	store    OwnershipStore
	launchID string
	owner    ProcessIdentity
	record   OwnRecord
	mu       sync.Mutex
	childSet bool
}

func newLease(store OwnershipStore, launchID string, owner ProcessIdentity, initial OwnRecord) *ownershipLease {
	return &ownershipLease{
		store:    store,
		launchID: launchID,
		owner:    owner,
		record:   initial,
	}
}

func (l *ownershipLease) LaunchID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.launchID
}

func (l *ownershipLease) Record() OwnRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.record
}

func (l *ownershipLease) AttachChild(ctx context.Context, pid int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.record.State != StateAcquiring {
		return fmt.Errorf("%w: state must be acquiring, got %s", ErrChildAttachFailed, l.record.State)
	}
	if l.childSet {
		return fmt.Errorf("%w: child already attached", ErrChildAttachFailed)
	}
	if pid <= 0 {
		return fmt.Errorf("%w: invalid PID %d", ErrChildAttachFailed, pid)
	}

	l.record.Child = &ProcessIdentity{
		PID:            pid,
		ExecutablePath: l.record.ExecutablePath,
	}
	l.record.State = StateRunning
	l.record.UpdatedAtEpochMillis = l.record.CreatedAtEpochMillis
	l.childSet = true

	if err := l.store.Update(ctx, l.record); err != nil {
		return err
	}
	return nil
}

func (l *ownershipLease) MarkStopping(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.record.State != StateRunning {
		return fmt.Errorf("qdrantprocess: cannot mark stopping from state %s", l.record.State)
	}
	l.record.State = StateStopping
	l.record.UpdatedAtEpochMillis = l.record.CreatedAtEpochMillis
	return l.store.Update(ctx, l.record)
}

func (l *ownershipLease) MarkExited(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.record.State != StateRunning && l.record.State != StateStopping {
		return fmt.Errorf("qdrantprocess: cannot mark exited from state %s", l.record.State)
	}
	l.record.State = StateExited
	l.record.Child = nil
	l.record.UpdatedAtEpochMillis = l.record.CreatedAtEpochMillis
	return l.store.Update(ctx, l.record)
}

func (l *ownershipLease) MarkFailed(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.record.State = StateFailed
	l.record.UpdatedAtEpochMillis = l.record.CreatedAtEpochMillis
	return l.store.Update(ctx, l.record)
}

func (l *ownershipLease) Release(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, err := l.store.Read(ctx)
	if err != nil {
		if err == ErrOwnershipRecordNotFound {
			return nil
		}
		return err
	}
	if rec.LaunchID != l.launchID {
		return ErrLeaseOwnershipLost
	}
	if !SameProcessIdentity(rec.Owner, l.owner) {
		return fmt.Errorf("%w: owner mismatch", ErrLeaseOwnershipLost)
	}
	if rec.Child != nil && !l.childSet {
		if isAliveSimple(rec.Child.PID) {
			return ErrProcessStillRunning
		}
	}

	return l.store.Delete(ctx)
}

func isAliveSimple(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return true
}
