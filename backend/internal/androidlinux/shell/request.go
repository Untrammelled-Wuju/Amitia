//go:build linux && !android

package shell

import (
	"time"
)

type ShellMode string

const (
	ShellModeArgv  ShellMode = "argv"
	ShellModeShell ShellMode = "shell"
)

type ShellExecuteRequest struct {
	Mode           ShellMode         `json:"mode"`
	Executable     string            `json:"executable,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Command        string            `json:"command,omitempty"`
	WorkingDir     string            `json:"workingDir,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutMs      int64             `json:"timeoutMs,omitempty"`
	MaxOutputBytes int64             `json:"maxOutputBytes,omitempty"`
}

type ShellPolicy struct {
	Enabled                bool          `json:"enabled"`
	DefaultTimeout         time.Duration `json:"defaultTimeout"`
	MaxTimeout             time.Duration `json:"maxTimeout"`
	MaxStdinBytes          int64         `json:"maxStdinBytes"`
	MaxStdoutBytes         int64         `json:"maxStdoutBytes"`
	MaxStderrBytes         int64         `json:"maxStderrBytes"`
	MaxEnvironmentEntries  int           `json:"maxEnvironmentEntries"`
	MaxEnvironmentBytes    int64         `json:"maxEnvironmentBytes"`
	AllowedEnvironmentKeys  []string      `json:"allowedEnvironmentKeys"`
}

func DefaultShellPolicy() ShellPolicy {
	return ShellPolicy{
		Enabled:                true,
		DefaultTimeout:         30 * time.Second,
		MaxTimeout:             5 * time.Minute,
		MaxStdinBytes:          1 * 1024 * 1024,
		MaxStdoutBytes:         1 * 1024 * 1024,
		MaxStderrBytes:         512 * 1024,
		MaxEnvironmentEntries:  64,
		MaxEnvironmentBytes:    64 * 1024,
		AllowedEnvironmentKeys:  []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "TERM"},
	}
}
