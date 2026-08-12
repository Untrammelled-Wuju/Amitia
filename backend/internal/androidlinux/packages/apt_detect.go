//go:build linux && !android

package packages

import (
	"context"
	"os/exec"
	"strings"

	"github.com/u-ai/backend/internal/androidlinux/shell"
)

type AptDetector struct {
	executor shell.ShellExecutor
}

func NewAptDetector(executor shell.ShellExecutor) *AptDetector {
	return &AptDetector{executor: executor}
}

func (d *AptDetector) Detect(ctx context.Context, timeoutMs int64) (AptStatus, error) {
	status := AptStatus{}

	if d.executor == nil {
		return status, ErrManagerNotFound("apt-get")
	}

	result := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "apt-get",
		Args:       []string{"--version"},
		TimeoutMs:  timeoutMs,
	})

	if result.ExitCode != 0 || result.TimedOut {
		return status, nil
	}

	status.Available = true
	status.Executable = "apt-get"
	status.Version = parseAptVersion(result.Stdout)

	archResult := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "dpkg",
		Args:       []string{"--print-architecture"},
		TimeoutMs:  timeoutMs,
	})
	if archResult.ExitCode == 0 {
		status.Architecture = strings.TrimSpace(archResult.Stdout)
	}

	privResult := d.executor.Execute(ctx, shell.ShellExecuteRequest{
		Mode:       shell.ShellModeArgv,
		Executable: "id",
		Args:       []string{"-u"},
		TimeoutMs:  timeoutMs,
	})
	if privResult.ExitCode == 0 {
		uid := strings.TrimSpace(privResult.Stdout)
		if uid == "0" {
			status.PrivilegeState = "guest_root"
		} else {
			status.PrivilegeState = "non_root"
		}
	}

	status.PackageIndexState = "unknown"
	_ = exec.CommandContext

	return status, nil
}

func parseAptVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "apt") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return strings.TrimSpace(output)
}
