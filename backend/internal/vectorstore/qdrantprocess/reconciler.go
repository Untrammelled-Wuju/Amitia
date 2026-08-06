// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"fmt"
	"os"
	"time"
)

const reconcileGracePeriod = 10 * time.Second

type Reconciler interface {
	Acquire(ctx context.Context, expected ExpectedProcess) (Lease, error)
}

type ownershipReconciler struct {
	root       string
	store      OwnershipStore
	inspector  ProcessInspector
	terminator ProcessTerminator
	clock      Clock
	maxWait    time.Duration
}

func NewReconciler(root string, fs FileSystem, inspector ProcessInspector, terminator ProcessTerminator, clock Clock) (Reconciler, error) {
	store, err := NewOwnershipStore(root, fs, clock)
	if err != nil {
		return nil, err
	}
	if inspector == nil {
		inspector = NewProcessInspector()
	}
	if terminator == nil {
		terminator = NewProcessTerminator()
	}
	if clock == nil {
		clock = NewClock()
	}
	return &ownershipReconciler{
		root:       root,
		store:      store,
		inspector:  inspector,
		terminator: terminator,
		clock:      clock,
		maxWait:    500 * time.Millisecond,
	}, nil
}

func (r *ownershipReconciler) Acquire(ctx context.Context, expected ExpectedProcess) (Lease, error) {
	if err := expected.Validate(); err != nil {
		return nil, err
	}
	return r.tryAcquire(ctx, expected)
}

func (r *ownershipReconciler) tryAcquire(ctx context.Context, expected ExpectedProcess) (Lease, error) {
	owner, err := r.currentOwnerIdentity()
	if err != nil {
		return nil, err
	}
	launchID, err := NewLaunchID()
	if err != nil {
		return nil, err
	}

	root := activeDirPath(r.root)
	if err := os.Mkdir(root, 0700); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		return r.reconcileExistingLease(ctx, expected, root, owner)
	}

	rec := OwnershipRecord{
		SchemaVersion:        ownershipSchemaVersion,
		ComponentID:          expected.ComponentID,
		LaunchID:             launchID,
		State:                StateAcquiring,
		Owner:                owner,
		ExecutablePath:       expected.ExecutablePath,
		ConfigPath:           expected.ConfigPath,
		CreatedAtEpochMillis: r.clock.Now().UnixMilli(),
	}
	if err := r.store.Create(ctx, rec); err != nil {
		os.Remove(root)
		return nil, err
	}

	return newLease(r.store, launchID, owner, rec), nil
}

func (r *ownershipReconciler) reconcileExistingLease(ctx context.Context, expected ExpectedProcess, root string, owner ProcessIdentity) (Lease, error) {
	rec, err := ReadWithRetry(r.store, r.maxWait)
	if err != nil {
		if err == ErrOwnershipRecordNotFound {
			return nil, fmt.Errorf("%w: active dir exists but record is missing", ErrOwnershipRecordCorrupted)
		}
		return nil, err
	}

	if err := r.checkOwnerStillAlive(ctx, rec); err != nil {
		return nil, err
	}

	if rec.Child == nil {
		r.cleanupStaleLease(root)
		return r.tryAcquire(ctx, expected)
	}

	childAlive, err := r.checkChildIdentity(ctx, rec, expected)
	if err != nil {
		return nil, err
	}
	if !childAlive {
		r.cleanupStaleLease(root)
		return r.tryAcquire(ctx, expected)
	}

	if rec.Child != nil {
		_, termErr := r.terminator.Terminate(ctx, *rec.Child, reconcileGracePeriod)
		if termErr != nil && termErr != ErrProcessIdentityMismatch {
			return nil, termErr
		}
	}

	r.cleanupStaleLease(root)
	return r.tryAcquire(ctx, expected)
}

func (r *ownershipReconciler) checkOwnerStillAlive(ctx context.Context, rec OwnershipRecord) error {
	alive, err := r.inspector.IsAlive(ctx, rec.Owner)
	if err != nil {
		return err
	}
	if alive {
		return ErrOwnedByLiveRuntime
	}
	return nil
}

func (r *ownershipReconciler) checkChildIdentity(ctx context.Context, rec OwnershipRecord, expected ExpectedProcess) (bool, error) {
	if rec.Child == nil {
		return false, nil
	}
	actual, err := r.inspector.Inspect(ctx, rec.Child.PID)
	if err != nil {
		return false, nil
	}
	if !SameProcessIdentity(*rec.Child, actual) {
		return false, nil
	}
	if !SameExecutablePath(actual.ExecutablePath, expected.ExecutablePath) {
		return false, fmt.Errorf("%w: child executable path mismatch", ErrProcessIdentityConflict)
	}
	if len(actual.CommandLine) > 0 && len(rec.Child.CommandLine) > 0 {
		if !ContainsConfigPath(actual.CommandLine, expected.ConfigPath) {
			return false, fmt.Errorf("%w: child config path mismatch", ErrProcessIdentityConflict)
		}
	}
	return true, nil
}

func (r *ownershipReconciler) cleanupStaleLease(activeDir string) {
	entries, err := os.ReadDir(activeDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		os.Remove(activeDir + string(os.PathSeparator) + entry.Name())
	}
	os.Remove(activeDir)
}

func (r *ownershipReconciler) currentOwnerIdentity() (ProcessIdentity, error) {
	pid := os.Getpid()
	return r.inspector.Inspect(context.Background(), pid)
}
