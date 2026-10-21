// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hello\n"), 0755); err != nil {
		t.Fatalf("failed to write test executable: %v", err)
	}
}

func TestValidateStdioCommandRejectsEmpty(t *testing.T) {
	err := validateStdioCommand("")
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("expected 'command is required' error, got %v", err)
	}
}

func TestValidateStdioCommandRejectsWhitespaceOnly(t *testing.T) {
	err := validateStdioCommand("   ")
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("expected 'command is required' error, got %v", err)
	}
}

func TestValidateStdioCommandRejectsNullByte(t *testing.T) {
	err := validateStdioCommand("node\x00")
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("expected 'command is required' error, got %v", err)
	}
}

func TestValidateStdioCommandRejectsShellCommands(t *testing.T) {
	shells := []string{
		"sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh",
		"/bin/bash", "/usr/bin/sh", "C:\\Windows\\System32\\cmd.exe",
	}
	for _, sh := range shells {
		err := validateStdioCommand(sh)
		if err == nil || !strings.Contains(err.Error(), "shell commands are forbidden") {
			t.Fatalf("expected 'shell commands are forbidden' for %q, got %v", sh, err)
		}
	}
}

func TestValidateStdioCommandAcceptsValidCommands(t *testing.T) {
	valid := []string{
		"node",
		"python",
		"/usr/bin/node",
		"my-mcp-server",
		"uvx",
		"npx",
		"@scope/pkg",
	}
	for _, cmd := range valid {
		if err := validateStdioCommand(cmd); err != nil {
			t.Errorf("expected %q to be valid, got %v", cmd, err)
		}
	}
}

func TestValidateStdioExecutableRejectsEmpty(t *testing.T) {
	err := validateStdioExecutable("")
	if err == nil || !strings.Contains(err.Error(), "executable path is required") {
		t.Fatalf("expected 'executable path is required' error, got %v", err)
	}
}

func TestValidateStdioExecutableRejectsWhitespaceOnly(t *testing.T) {
	err := validateStdioExecutable("   ")
	if err == nil || !strings.Contains(err.Error(), "executable path is required") {
		t.Fatalf("expected 'executable path is required' error, got %v", err)
	}
}

func TestValidateStdioExecutableRejectsMissingFile(t *testing.T) {
	err := validateStdioExecutable("/nonexistent/path/to/binary")
	if err == nil || !strings.Contains(err.Error(), "executable not found") {
		t.Fatalf("expected 'executable not found' error, got %v", err)
	}
}

func TestValidateStdioExecutableRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	err := validateStdioExecutable(dir)
	if err == nil || !strings.Contains(err.Error(), "executable is a directory") {
		t.Fatalf("expected 'executable is a directory' error, got %v", err)
	}
}

func TestValidateStdioExecutableAcceptsValidFile(t *testing.T) {
	root := t.TempDir()
	exePath := filepath.Join(root, "my-binary")
	writeTestExecutable(t, exePath)
	if err := validateStdioExecutable(exePath); err != nil {
		t.Errorf("expected valid file to pass, got %v", err)
	}
}

func TestStdioConfigExecutableField(t *testing.T) {
	config := StdioConfig{
		Command:    "original-command",
		Executable: "/resolved/path/to/node",
		Args:       []string{"--version"},
	}
	if config.Command != "original-command" {
		t.Fatal("Command should preserve original")
	}
	if config.Executable != "/resolved/path/to/node" {
		t.Fatal("Executable should be set")
	}
}

func TestStdioNewSetsDefaults(t *testing.T) {
	s := NewStdio(StdioConfig{
		Command:    "node",
		Executable: "/usr/bin/node",
	})
	if s.config.StartTimeout != 10*1000000000 {
		t.Fatal("expected 10s default StartTimeout")
	}
	if s.config.MaxMessageBytes != 4<<20 {
		t.Fatal("expected 4MB default MaxMessageBytes")
	}
}

func TestStdioCommandFallsBackToConfigCommand(t *testing.T) {
	s := NewStdio(StdioConfig{
		Command:    "my-mcp",
		Executable: "/usr/bin/my-mcp",
	})
	if s.config.Command != "my-mcp" {
		t.Fatal("Command should be set correctly")
	}
	if s.config.OriginalCommand != "" {
		t.Fatal("OriginalCommand should be empty")
	}
}
