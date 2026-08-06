// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func captureVersionOutput(t *testing.T) (string, int, string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	exitCode := handleVersion()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	io.Copy(&stdoutBuf, rOut)
	rOut.Close()
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, rErr)
	rErr.Close()

	return stdoutBuf.String(), exitCode, stderrBuf.String()
}

func TestHandleVersionExitCode(t *testing.T) {
	_, exitCode, _ := captureVersionOutput(t)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestHandleVersionStderrEmpty(t *testing.T) {
	_, _, stderr := captureVersionOutput(t)
	if stderr != "" {
		t.Fatalf("expected stderr empty, got %q", stderr)
	}
}

func TestHandleVersionSingleLineJSON(t *testing.T) {
	stdout, _, _ := captureVersionOutput(t)
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	lineCount := 0
	var lastLine string
	for scanner.Scan() {
		lineCount++
		lastLine = scanner.Text()
	}
	if lineCount != 1 {
		t.Fatalf("expected exactly 1 line, got %d", lineCount)
	}
	if !strings.HasSuffix(lastLine, "}") {
		t.Fatalf("expected JSON line, got %q", lastLine)
	}
}

func TestHandleVersionJSONFields(t *testing.T) {
	stdout, _, _ := captureVersionOutput(t)
	stdout = strings.TrimSpace(stdout)
	var out versionOutput
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.Name != "amitia-server" {
		t.Fatalf("expected name 'amitia-server', got %q", out.Name)
	}
	if out.Version == "" {
		t.Fatal("version should not be empty")
	}
	if out.Commit == "" {
		t.Fatal("commit should not be empty")
	}
	if out.Target == "" {
		t.Fatal("target should not be empty")
	}
	if out.GoVersion == "" {
		t.Fatal("goVersion should not be empty")
	}
	if out.GOOS == "" {
		t.Fatal("goos should not be empty")
	}
	if out.GOARCH == "" {
		t.Fatal("goarch should not be empty")
	}
}

func TestHandleVersionJSONFieldOrder(t *testing.T) {
	stdout, _, _ := captureVersionOutput(t)
	stdout = strings.TrimSpace(stdout)
	expected := `{"name":"amitia-server","version":"dev","commit":"unknown","target":"development","goVersion":"go`
	if !strings.HasPrefix(stdout, expected) {
		t.Fatalf("unexpected JSON output: %s", stdout)
	}
}
