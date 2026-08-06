// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnershipStore_CreateAndRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	rec := OwnershipRecord{
		SchemaVersion:        1,
		ComponentID:          ComponentID,
		LaunchID:             "test-launch-id",
		State:                StateAcquiring,
		Owner:                ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "amitia"), StartMarker: "a:1"},
		ExecutablePath:       testAbsPath("bin", "qdrant"),
		ConfigPath:           testAbsPath("etc", "qdrant", "config.yaml"),
		CreatedAtEpochMillis: 1000,
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	read, err := store.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.LaunchID != "test-launch-id" {
		t.Fatalf("launch ID mismatch: %s", read.LaunchID)
	}
	if read.State != StateAcquiring {
		t.Fatalf("state: %s", read.State)
	}
}

func TestOwnershipStore_ReadNotFound(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}

	_, err = store.Read(context.Background())
	if err != ErrOwnershipRecordNotFound {
		t.Fatalf("expected ErrOwnershipRecordNotFound, got: %v", err)
	}
}

func TestOwnershipStore_Update(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	rec := OwnershipRecord{
		SchemaVersion:  1,
		ComponentID:    ComponentID,
		LaunchID:       "launch-1",
		State:          StateAcquiring,
		Owner:          ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "a"), StartMarker: "a:1"},
		ExecutablePath: testAbsPath("bin", "q"),
		ConfigPath:     testAbsPath("etc", "q.yaml"),
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	read, _ := store.Read(context.Background())
	read.State = StateRunning
	if err := store.Update(context.Background(), read); err != nil {
		t.Fatalf("Update: %v", err)
	}

	final, _ := store.Read(context.Background())
	if final.State != StateRunning {
		t.Fatalf("state: %s", final.State)
	}
}

func TestOwnershipStore_Exists(t *testing.T) {
	root := filepath.Join(t.TempDir(), "process", "qdrant")
	fs := NewFileSystem()
	store, err := NewOwnershipStore(root, fs, NewClock())
	if err != nil {
		t.Fatalf("NewOwnershipStore: %v", err)
	}
	if err := os.MkdirAll(activeDirPath(root), 0755); err != nil {
		t.Fatal(err)
	}

	rec := OwnershipRecord{
		SchemaVersion:  1,
		ComponentID:    ComponentID,
		LaunchID:       "launch-1",
		State:          StateAcquiring,
		Owner:          ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "a"), StartMarker: "a:1"},
		ExecutablePath: testAbsPath("bin", "q"),
		ConfigPath:     testAbsPath("etc", "q.yaml"),
	}
	if err := store.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err := store.Exists(context.Background())
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected record to exist")
	}
}
