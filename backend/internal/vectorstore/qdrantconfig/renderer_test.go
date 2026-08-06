// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func mustMarshalPath(path string) string {
	b, _ := json.Marshal(path)
	return string(b)
}

func TestRenderer_FixedFieldOrderAndIndent(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	doc := Document{
		HTTPPort:     19178,
		GRPCPort:     19179,
		StoragePath:  storagePath,
		SnapshotPath: snapshotsPath,
	}

	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	expected := "service:\n" +
		"  http_port: 19178\n" +
		"  grpc_port: 19179\n" +
		"storage:\n" +
		"  storage_path: " + mustMarshalPath(storagePath) + "\n" +
		"  snapshots_path: " + mustMarshalPath(snapshotsPath) + "\n"

	if string(out) != expected {
		t.Errorf("output mismatch:\nexpected:\n%s\ngot:\n%s", expected, string(out))
	}
}

func TestRenderer_StructureHasTrailingNewline(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")
	doc := Document{
		HTTPPort:     2000,
		GRPCPort:     2001,
		StoragePath:  storagePath,
		SnapshotPath: snapshotsPath,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasSuffix(string(out), "\n") {
		t.Errorf("output should end with newline, got %q", string(out))
	}
	if strings.HasSuffix(string(out), "\n\n") {
		t.Errorf("output should have exactly one trailing newline")
	}
}

func TestRenderer_Deterministic(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")
	doc := Document{
		HTTPPort:     2000,
		GRPCPort:     2001,
		StoragePath:  storagePath,
		SnapshotPath: snapshotsPath,
	}
	out1, _ := r.Render(doc)
	out2, _ := r.Render(doc)
	if string(out1) != string(out2) {
		t.Errorf("render not deterministic")
	}
}

func TestRenderer_NoTimestampNoComment(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")
	doc := Document{
		HTTPPort:     4000,
		GRPCPort:     4001,
		StoragePath:  storagePath,
		SnapshotPath: snapshotsPath,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "#") {
		t.Errorf("output should not contain comments: %q", s)
	}
	if strings.Contains(s, "time:") {
		t.Errorf("output should not contain time: %q", s)
	}
}

func TestRenderer_InvalidPorts(t *testing.T) {
	r := NewRenderer()
	validPath := filepath.Join(t.TempDir(), "storage")
	validSnap := filepath.Join(t.TempDir(), "snapshots")

	invalidDocs := []Document{
		{HTTPPort: 0, GRPCPort: 1000, StoragePath: validPath, SnapshotPath: validSnap},
		{HTTPPort: 1000, GRPCPort: 0, StoragePath: validPath, SnapshotPath: validSnap},
		{HTTPPort: 70000, GRPCPort: 1000, StoragePath: validPath, SnapshotPath: validSnap},
		{HTTPPort: 1000, GRPCPort: 1000, StoragePath: validPath, SnapshotPath: validSnap},
		{HTTPPort: 1000, GRPCPort: 1001, StoragePath: "", SnapshotPath: validSnap},
		{HTTPPort: 1000, GRPCPort: 1001, StoragePath: validPath, SnapshotPath: ""},
	}

	for i, doc := range invalidDocs {
		_, err := r.Render(doc)
		if err == nil {
			t.Errorf("case %d: expected error for invalid input", i)
		}
	}
}
