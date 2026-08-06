// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	proc "github.com/u-ai/backend/internal/platform/process"
)

type ProcessID string

var processIDPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{2,127}$`)

func (id ProcessID) String() string { return string(id) }

func ValidateProcessID(id ProcessID) error {
	s := string(id)
	if s == "" {
		return fmt.Errorf("%w: empty process ID", ErrInvalidProcessSpec)
	}
	if !processIDPattern.MatchString(s) {
		return fmt.Errorf("%w: invalid process ID %q (must be lowercase alphanumeric, dots, hyphens, underscores; 3-128 chars)", ErrInvalidProcessSpec, s)
	}
	return nil
}

const (
	EnvPolicyMinimal  EnvironmentPolicy = "minimal"
	EnvPolicyInherit  EnvironmentPolicy = "inherit"
	EnvPolicyExplicit EnvironmentPolicy = "explicit"
)

type EnvironmentPolicy string

type EnvironmentSpec struct {
	Policy       EnvironmentPolicy   `json:"policy"`
	Values       map[string]string   `json:"values"`
	SensitiveKeys []string            `json:"sensitive_keys"`
}

type LoopbackPortClaim struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type RestartPolicy struct {
	Mode        RestartMode       `json:"mode"`
	MaxRestarts int               `json:"max_restarts"`
	BaseDelay   time.Duration     `json:"base_delay"`
	MaxDelay    time.Duration     `json:"max_delay"`
	ResetAfter  time.Duration     `json:"reset_after"`
}

type RestartMode string

const (
	RestartNever    RestartMode = "never"
	RestartOnFailure RestartMode = "on-failure"
	RestartAlways   RestartMode = "always"
)

const (
	DefaultStartupTimeout   = 30 * time.Second
	DefaultStopGracePeriod  = 5 * time.Second
	DefaultHealthInterval   = 10 * time.Second
	DefaultMaxRestarts      = 5
)

type ProcessExec interface {
	Start() (pid int, handle proc.ProcessTreeHandle, err error)
}

type ProcessSpec struct {
	ID                ProcessID         `json:"id"`
	Executable        string            `json:"executable"`
	Args              []string          `json:"args,omitempty"`
	WorkingDir        string            `json:"working_dir"`
	Environment       EnvironmentSpec   `json:"environment"`
	Ports             []LoopbackPortClaim `json:"ports"`
	StartupTimeout    time.Duration     `json:"startup_timeout"`
	StopGracePeriod   time.Duration     `json:"stop_grace_period"`
	HealthProbe       HealthProbe       `json:"health_probe,omitempty"`
	HealthInterval    time.Duration     `json:"health_interval"`
	RestartPolicy     RestartPolicy     `json:"restart_policy"`
	OnStdout          func(line string) `json:"-"`
	OnStderr          func(line string) `json:"-"`
	OnStreamError     func(error)       `json:"-"`
	SensitiveArgIndexes []int           `json:"-"`
}

func cloneSliceInt(in []int) []int {
	if in == nil {
		return nil
	}
	out := make([]int, len(in))
	copy(out, in)
	return out
}

func cloneSliceString(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneMapString(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePorts(in []LoopbackPortClaim) []LoopbackPortClaim {
	if in == nil {
		return nil
	}
	out := make([]LoopbackPortClaim, len(in))
	copy(out, in)
	return out
}

func (s ProcessSpec) Clone() ProcessSpec {
	return ProcessSpec{
		ID:                s.ID,
		Executable:        s.Executable,
		Args:              cloneSliceString(s.Args),
		WorkingDir:        s.WorkingDir,
		Environment: EnvironmentSpec{
			Policy:       s.Environment.Policy,
			Values:       cloneMapString(s.Environment.Values),
			SensitiveKeys: cloneSliceString(s.Environment.SensitiveKeys),
		},
		Ports:             clonePorts(s.Ports),
		StartupTimeout:    s.StartupTimeout,
		StopGracePeriod:   s.StopGracePeriod,
		HealthInterval:    s.HealthInterval,
		RestartPolicy:     s.RestartPolicy,
		OnStdout:          s.OnStdout,
		OnStderr:          s.OnStderr,
		OnStreamError:     s.OnStreamError,
		SensitiveArgIndexes: cloneSliceInt(s.SensitiveArgIndexes),
	}
}

func (s ProcessSpec) validate() error {
	if err := ValidateProcessID(s.ID); err != nil {
		return err
	}
	if s.Executable == "" || !filepath.IsAbs(s.Executable) {
		return fmt.Errorf("%w: executable must be an absolute path", ErrInvalidProcessSpec)
	}
	if s.WorkingDir == "" || !filepath.IsAbs(s.WorkingDir) {
		return fmt.Errorf("%w: working_dir must be an absolute path", ErrInvalidProcessSpec)
	}
	for _, arg := range s.Args {
		if strings.ContainsAny(arg, "\x00") {
			return fmt.Errorf("%w: args contain null byte", ErrInvalidProcessSpec)
		}
	}
	for _, p := range s.Ports {
		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("%w: port %d out of range", ErrInvalidProcessSpec, p.Port)
		}
		if !isLoopback(p.Host) {
			return fmt.Errorf("%w: host %q is not a loopback address", ErrInvalidProcessSpec, p.Host)
		}
		if p.Protocol != "" && p.Protocol != "tcp" {
			return fmt.Errorf("%w: only TCP protocol supported, got %q", ErrInvalidProcessSpec, p.Protocol)
		}
	}
	for _, k := range s.Environment.SensitiveKeys {
		if _, ok := s.Environment.Values[k]; !ok {
			continue
		}
	}
	for _, idx := range s.SensitiveArgIndexes {
		if idx < 0 || idx >= len(s.Args) {
			return fmt.Errorf("%w: sensitive arg index %d out of bounds", ErrInvalidProcessSpec, idx)
		}
	}
	return nil
}

func isLoopback(host string) bool {
	switch strings.ToLower(host) {
	case "127.0.0.1", "::1", "localhost", "":
		return true
	}
	if strings.HasPrefix(host, "127.") {
		return true
	}
	return false
}
