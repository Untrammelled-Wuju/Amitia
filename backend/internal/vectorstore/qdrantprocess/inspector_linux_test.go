// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux

package qdrantprocess

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestLinuxInspector_Self(t *testing.T) {
	inspector := newPlatformInspector()
	id, err := inspector.Inspect(context.Background(), os.Getpid())
	if err != nil {
		t.Fatalf("Inspect self: %v", err)
	}
	if id.PID != os.Getpid() {
		t.Fatalf("PID: %d", id.PID)
	}
	if id.ExecutablePath == "" {
		t.Fatal("empty exec path")
	}
	if id.StartMarker == "" {
		t.Fatal("empty start marker")
	}

	alive, err := inspector.IsAlive(context.Background(), id)
	if err != nil {
		t.Fatalf("IsAlive: %v", err)
	}
	if !alive {
		t.Fatal("expected self to be alive")
	}
}

func TestLinuxInspector_ProcessNotExist(t *testing.T) {
	inspector := newPlatformInspector()
	_, err := inspector.Inspect(context.Background(), 99999999)
	if err == nil {
		t.Fatal("expected error for non-existent PID")
	}
}

func TestLinuxInspector_InvalidPID(t *testing.T) {
	inspector := newPlatformInspector()
	_, err := inspector.Inspect(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for PID 0")
	}
	_, err = inspector.Inspect(context.Background(), -1)
	if err == nil {
		t.Fatal("expected error for negative PID")
	}
}

func TestLinuxInspector_StatParse(t *testing.T) {
	line := "12345 (my process) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 1234567890 24"
	marker, err := parseStatStartMarker(line, 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(marker, "1234567890") {
		t.Fatalf("marker should contain starttime: %s", marker)
	}
}

func TestLinuxInspector_StatParse_WithSpaces(t *testing.T) {
	line := "999 (my app (with spaces)) R 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 9999 24"
	marker, err := parseStatStartMarker(line, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(marker, "9999") {
		t.Fatalf("marker: %s", marker)
	}
}

func TestLinuxInspector_Stat_Truncated(t *testing.T) {
	line := "1 (short) S 1 2 3"
	_, err := parseStatStartMarker(line, 1)
	if err == nil {
		t.Fatal("expected error for truncated stat")
	}
}

func TestLinuxInspector_Stat_Empty(t *testing.T) {
	_, err := parseStatStartMarker("", 1)
	if err == nil {
		t.Fatal("expected error for empty stat")
	}
}

func TestCleanExePath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/usr/bin/qdrant", "/usr/bin/qdrant"},
		{"/usr/bin/qdrant (deleted)", "/usr/bin/qdrant"},
	}
	for _, tt := range tests {
		got := cleanExePath(tt.in)
		if got != tt.want {
			t.Errorf("cleanExePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
