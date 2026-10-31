//go:build !linux || android

package process

import (
	"fmt"
	amitialog "log"
	"os/exec"
)

func startProcessTreeWithLimits(cmd *exec.Cmd, limits ResourceLimits) (ProcessTreeHandle, error) {
	if cmd == nil {
		return 0, fmt.Errorf("process: nil command")
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	handle, err := attachProcessTreeWithLimits(cmd, limits)
	if err != nil {
		if err := cmd.Process.Kill(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := cmd.Wait(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return 0, fmt.Errorf("process: attach process tree: %w", err)
	}
	return handle, nil
}
