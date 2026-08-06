// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/vectorstore/qdrantprofile"
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

func TestRenderer_MobileBalanced_FullStructure(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.BalancedSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render mobile: %v", err)
	}
	result := string(out)

	if !strings.HasPrefix(result, "log_level: ") {
		t.Errorf("mobile output should start with log_level:\n%s", result)
	}
	if !strings.Contains(result, "telemetry_disabled: true") {
		t.Error("telemetry should be disabled")
	}
	if !strings.Contains(result, "host: \"127.0.0.1\"") {
		t.Error("host should be loopback")
	}
	if strings.Contains(result, "0.0.0.0") {
		t.Error("must not contain 0.0.0.0")
	}
	if !strings.Contains(result, "enable_cors: false") {
		t.Error("CORS should be disabled")
	}
	if !strings.Contains(result, "enable_tls: false") {
		t.Error("TLS should be disabled")
	}
	if !strings.Contains(result, "cluster:\n  enabled: false") {
		t.Error("cluster should be disabled")
	}
	if !strings.Contains(result, "on_disk_payload: true") {
		t.Error("on_disk_payload should be true")
	}
	if !strings.Contains(result, "max_search_threads: 2") {
		t.Error("max_search_threads should be 2 for balanced")
	}
	if !strings.Contains(result, "optimizer_cpu_budget: 1") {
		t.Error("optimizer_cpu_budget should be 1 for balanced")
	}
	if !strings.Contains(result, "max_optimization_threads: 1") {
		t.Error("max_optimization_threads should be 1")
	}
	if !strings.Contains(result, "memory: \"cached\"") {
		t.Errorf("HNSW memory should be cached for balanced, got:\n%s", result)
	}
	if strings.Contains(result, "on_disk:") {
		t.Error("must not use deprecated on_disk field")
	}
	if strings.Contains(result, "optimizers_overwrite") {
		t.Error("must not include optimizers_overwrite")
	}
	if strings.Contains(result, "low_memory_mode") {
		t.Error("must not include experimental field low_memory_mode")
	}
	if strings.Contains(result, "async_scorer") {
		t.Error("must not include experimental field async_scorer")
	}
}

func TestRenderer_MobileCompact_HNSWCold(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.CompactSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render mobile compact: %v", err)
	}
	result := string(out)

	if !strings.Contains(result, "memory: \"cold\"") {
		t.Errorf("compact should use cold HNSW memory:\n%s", result)
	}
	if !strings.Contains(result, "max_search_threads: 1") {
		t.Error("compact should have max_search_threads=1")
	}
	if !strings.Contains(result, "max_workers: 1") {
		t.Error("compact should have max_workers=1")
	}
	if !strings.Contains(result, "wal_capacity_mb: 8") {
		t.Error("compact should have wal_capacity_mb=8")
	}
}

func TestRenderer_MobilePerformance_MaxIndexingThreads(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.PerformanceSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render mobile performance: %v", err)
	}
	result := string(out)

	if !strings.Contains(result, "max_indexing_threads: 4") {
		t.Errorf("performance should have max_indexing_threads=4:\n%s", result)
	}
	if !strings.Contains(result, "optimizer_cpu_budget: 2") {
		t.Error("performance should have optimizer_cpu_budget=2")
	}
	if !strings.Contains(result, "wal_capacity_mb: 32") {
		t.Error("performance should have wal_capacity_mb=32")
	}
}

func TestRenderer_MobileFieldOrder(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.BalancedSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	expectedOrder := []string{
		"log_level: ",
		"telemetry_disabled: ",
		"service:",
		"  max_request_size_mb: ",
		"  max_workers: ",
		"  host: ",
		"  http_port: ",
		"  grpc_port: ",
		"  enable_cors: ",
		"  enable_tls: ",
		"  enable_snapshot_url_recovery: ",
		"storage:",
		"  storage_path: ",
		"  snapshots_path: ",
		"  on_disk_payload: ",
		"  update_concurrency: ",
		"  wal:",
		"    wal_capacity_mb: ",
		"    wal_segments_ahead: ",
		"  performance:",
		"    max_search_threads: ",
		"    optimizer_cpu_budget: ",
		"  optimizers:",
		"    default_segment_number: ",
		"    indexing_threshold_kb: ",
		"    flush_interval_sec: ",
		"    max_optimization_threads: ",
		"  hnsw_index:",
		"    max_indexing_threads: ",
		"    memory: ",
		"cluster:",
		"  enabled: ",
	}

	if len(lines) != len(expectedOrder) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(expectedOrder), len(lines), string(out))
	}
	for i, prefix := range expectedOrder {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d: expected prefix %q, got %q", i, prefix, lines[i])
		}
	}
}

func TestRenderer_GoldenDesktop(t *testing.T) {
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

	goldenBytes, err := os.ReadFile("testdata/desktop-default.golden.yaml")
	if err != nil {
		t.Fatalf("Read golden: %v", err)
	}
	golden := strings.ReplaceAll(string(goldenBytes), "REPLACE_STORAGE_PATH", mustMarshalPath(storagePath))
	golden = strings.ReplaceAll(golden, "REPLACE_SNAPSHOT_PATH", mustMarshalPath(snapshotsPath))

	if string(out) != golden {
		t.Errorf("desktop output mismatch:\nexpected:\n%s\ngot:\n%s", golden, string(out))
	}
}

func TestRenderer_GoldenMobileBalanced(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.BalancedSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	goldenBytes, err := os.ReadFile("testdata/mobile-balanced.golden.yaml")
	if err != nil {
		t.Fatalf("Read golden: %v", err)
	}
	golden := strings.ReplaceAll(string(goldenBytes), "REPLACE_STORAGE_PATH", mustMarshalPath(storagePath))
	golden = strings.ReplaceAll(golden, "REPLACE_SNAPSHOT_PATH", mustMarshalPath(snapshotsPath))

	if string(out) != golden {
		t.Errorf("mobile-balanced output mismatch:\nexpected:\n%s\ngot:\n%s", golden, string(out))
	}
}

func TestRenderer_GoldenMobileCompact(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.CompactSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	goldenBytes, err := os.ReadFile("testdata/mobile-compact.golden.yaml")
	if err != nil {
		t.Fatalf("Read golden: %v", err)
	}
	golden := strings.ReplaceAll(string(goldenBytes), "REPLACE_STORAGE_PATH", mustMarshalPath(storagePath))
	golden = strings.ReplaceAll(golden, "REPLACE_SNAPSHOT_PATH", mustMarshalPath(snapshotsPath))

	if string(out) != golden {
		t.Errorf("mobile-compact output mismatch:\nexpected:\n%s\ngot:\n%s", golden, string(out))
	}
}

func TestRenderer_GoldenMobilePerformance(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.PerformanceSettings()
	doc := Document{
		HTTPPort:        19178,
		GRPCPort:        19179,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	goldenBytes, err := os.ReadFile("testdata/mobile-performance.golden.yaml")
	if err != nil {
		t.Fatalf("Read golden: %v", err)
	}
	golden := strings.ReplaceAll(string(goldenBytes), "REPLACE_STORAGE_PATH", mustMarshalPath(storagePath))
	golden = strings.ReplaceAll(golden, "REPLACE_SNAPSHOT_PATH", mustMarshalPath(snapshotsPath))

	if string(out) != golden {
		t.Errorf("mobile-performance output mismatch:\nexpected:\n%s\ngot:\n%s", golden, string(out))
	}
}

func TestRenderer_DocumentFieldIntegrity(t *testing.T) {
	r := NewRenderer()
	storagePath := filepath.Join(t.TempDir(), "storage")
	snapshotsPath := filepath.Join(t.TempDir(), "snapshots")

	s := qdrantprofile.BalancedSettings()
	doc := Document{
		HTTPPort:        19999,
		GRPCPort:        20000,
		StoragePath:     storagePath,
		SnapshotPath:    snapshotsPath,
		ResourceProfile: &s,
	}
	out, err := r.Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	result := string(out)

	if !strings.Contains(result, "http_port: 19999") {
		t.Error("HTTP port should come from document")
	}
	if !strings.Contains(result, "grpc_port: 20000") {
		t.Error("GRPC port should come from document")
	}
	storageJSON := mustMarshalPath(storagePath)
	if !strings.Contains(result, "storage_path: "+storageJSON) {
		t.Error("storage path should come from layout")
	}
}
