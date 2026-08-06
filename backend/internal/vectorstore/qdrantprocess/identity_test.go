// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testAbsPath(elems ...string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(append([]string{"C:\\"}, elems...)...)
	}
	return filepath.Join(append([]string{"/"}, elems...)...)
}

func TestProcessIdentity_Validate_Valid(t *testing.T) {
	id := ProcessIdentity{
		PID:            123,
		ExecutablePath: testAbsPath("usr", "bin", "qdrant"),
		StartMarker:    "boot1:1000",
	}
	if err := id.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessIdentity_Validate_PIDZero(t *testing.T) {
	id := ProcessIdentity{PID: 0, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a"}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for PID 0")
	}
}

func TestProcessIdentity_Validate_NegativePID(t *testing.T) {
	id := ProcessIdentity{PID: -1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a"}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for negative PID")
	}
}

func TestProcessIdentity_Validate_EmptyExecutable(t *testing.T) {
	id := ProcessIdentity{PID: 1, ExecutablePath: "", StartMarker: "a"}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for empty executable path")
	}
}

func TestProcessIdentity_Validate_RelativeExecutable(t *testing.T) {
	id := ProcessIdentity{PID: 1, ExecutablePath: "qdrant", StartMarker: "a"}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for relative executable path")
	}
}

func TestProcessIdentity_Validate_EmptyStartMarker(t *testing.T) {
	id := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: ""}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for empty start marker")
	}
}

func TestProcessIdentity_Validate_NULInCommandLine(t *testing.T) {
	id := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a", CommandLine: []string{"\x00bad"}}
	if err := id.Validate(); err == nil {
		t.Fatal("expected error for NUL in command line")
	}
}

func TestSameProcessIdentity_Equal(t *testing.T) {
	a := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:1"}
	b := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:1"}
	if !SameProcessIdentity(a, b) {
		t.Fatal("expected equal identities")
	}
}

func TestSameProcessIdentity_PIDMismatch(t *testing.T) {
	a := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:1"}
	b := ProcessIdentity{PID: 2, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:1"}
	if SameProcessIdentity(a, b) {
		t.Fatal("expected mismatch for different PID")
	}
}

func TestSameProcessIdentity_StartMarkerMismatch(t *testing.T) {
	a := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:1"}
	b := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "q"), StartMarker: "a:2"}
	if SameProcessIdentity(a, b) {
		t.Fatal("expected mismatch for different start marker")
	}
}

func TestSameProcessIdentity_PathMismatch(t *testing.T) {
	a := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("bin", "qdrant"), StartMarker: "a:1"}
	b := ProcessIdentity{PID: 1, ExecutablePath: testAbsPath("other", "qdrant"), StartMarker: "a:1"}
	if SameProcessIdentity(a, b) {
		t.Fatal("expected mismatch for different path")
	}
}
