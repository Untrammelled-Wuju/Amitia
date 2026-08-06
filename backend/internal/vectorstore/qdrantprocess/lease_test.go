// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLease_AttachChild(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	owner := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "amitia"), StartMarker: "a:1"}
	rec := OwnershipRecord{
		SchemaVersion:  1,
		ComponentID:    ComponentID,
		LaunchID:       "launch-1",
		State:          StateAcquiring,
		Owner:          owner,
		ExecutablePath: testAbsPath("bin", "qdrant"),
		ConfigPath:     testAbsPath("etc", "q.yaml"),
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	lease := newLease(store, rec.LaunchID, owner, rec)
	if err := lease.AttachChild(context.Background(), 999); err != nil {
		t.Fatalf("AttachChild: %v", err)
	}

	updated := lease.Record()
	if updated.State != StateRunning {
		t.Fatalf("state: %s", updated.State)
	}
	if updated.Child == nil || updated.Child.PID != 999 {
		t.Fatal("child not attached")
	}
}

func TestLease_MarkStoppingAndExited(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	owner := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "amitia"), StartMarker: "a:1"}
	rec := OwnershipRecord{
		SchemaVersion:  1,
		ComponentID:    ComponentID,
		LaunchID:       "launch-1",
		State:          StateAcquiring,
		Owner:          owner,
		ExecutablePath: testAbsPath("bin", "qdrant"),
		ConfigPath:     testAbsPath("etc", "q.yaml"),
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	lease := newLease(store, rec.LaunchID, owner, rec)
	_ = lease.AttachChild(context.Background(), 100)

	if err := lease.MarkStopping(context.Background()); err != nil {
		t.Fatalf("MarkStopping: %v", err)
	}
	if lease.Record().State != StateStopping {
		t.Fatalf("state: %s", lease.Record().State)
	}

	if err := lease.MarkExited(context.Background()); err != nil {
		t.Fatalf("MarkExited: %v", err)
	}
	if lease.Record().State != StateExited {
		t.Fatalf("state: %s", lease.Record().State)
	}
}

func TestLease_InvalidStateTransitions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	owner := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "amitia"), StartMarker: "a:1"}
	rec := OwnershipRecord{
		SchemaVersion:  1,
		ComponentID:    ComponentID,
		LaunchID:       "launch-1",
		State:          StateAcquiring,
		Owner:          owner,
		ExecutablePath: testAbsPath("bin", "qdrant"),
		ConfigPath:     testAbsPath("etc", "q.yaml"),
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	lease := newLease(store, rec.LaunchID, owner, rec)
	err = lease.MarkStopping(context.Background())
	if err == nil {
		t.Fatal("expected error stopping from acquiring")
	}
}
