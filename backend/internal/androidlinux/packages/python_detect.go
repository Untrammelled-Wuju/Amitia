//go:build linux && !android

package packages

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/androidlinux/shell"
)

type PythonDetector struct {
	executor shell.ShellExecutor
}

type PythonVersionInfo struct {
	Major   string
	Minor   string
	Patch   string
	Raw     string
}

func NewPythonDetector(executor shell.ShellExecutor) *PythonDetector {
	return &PythonDetector{executor: executor}
}

func (d *PythonDetector) Detect(ctx context.Context, timeoutMs int64) (PythonStatus, error) {
	status := PythonStatus{}

	if d.executor == nil {
		return status, ErrPythonNotFound()
	}

	result := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "python3",
		Args:       []string{"--version"},
		TimeoutMs:  timeoutMs,
	})

	if result.ExitCode != 0 || result.TimedOut {
		return status, nil
	}

	status.Available = true
	status.Version = parsePythonVersion(result.Stdout)
	status.Implementation = "cpython"

	pipResult := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "python3",
		Args:       []string{"-m", "pip", "--version"},
		TimeoutMs:  timeoutMs,
	})

	if pipResult.ExitCode == 0 && !pipResult.TimedOut {
		status.PipAvailable = true
		status.PipVersion = parsePipVersion(pipResult.Stdout)
	}

	venvResult := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "python3",
		Args:       []string{"-m", "venv", "--help"},
		TimeoutMs:  timeoutMs,
	})

	if venvResult.ExitCode == 0 && !venvResult.TimedOut {
		status.VenvAvailable = true
	}

	return status, nil
}

func (d *PythonDetector) GetSystemPython() (string, error) {
	return "python3", nil
}

func parsePythonVersion(output string) string {
	output = strings.TrimSpace(output)
	re := regexp.MustCompile(`Python\s+(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return output
}

func parsePipVersion(output string) string {
	re := regexp.MustCompile(`pip\s+(\S+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return strings.TrimSpace(output)
}

func ParsePythonVersion(raw string) PythonVersionInfo {
	v := PythonVersionInfo{Raw: raw}
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) >= 4 {
		v.Major = matches[1]
		v.Minor = matches[2]
		v.Patch = matches[3]
	}
	return v
}

func DefaultPythonPath() string {
	return "/usr/bin/python3"
}

func PipExecutable(pythonPath string) string {
	return fmt.Sprintf("%s -m pip", pythonPath)
}
