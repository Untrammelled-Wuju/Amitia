package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type hostCommandOutput struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func (o *hostCommandOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	available := o.limit - len(o.data)
	if available > 0 {
		if available > n {
			available = n
		}
		o.data = append(o.data, p[:available]...)
	}
	if available < n {
		o.truncated = true
	}
	return n, nil
}

func (o *hostCommandOutput) Snapshot() (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return string(o.data), o.truncated
}

func resolveHostCommandDirectory(root, requested string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	root = canonicalRoot
	directory := root
	if strings.TrimSpace(requested) != "" {
		directory = requested
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(root, directory)
		}
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("resolve host command directory: %w", err)
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", errors.New("working directory is outside the bound workspace")
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return "", errors.New("working directory does not exist")
	}
	return directory, nil
}

func (s *BuiltinUtilityService) executeHostCommand(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	if s.workspace == nil || invocation.ExecContext == nil || strings.TrimSpace(invocation.ExecContext.WorkspaceID) == "" {
		return nil, errors.New("WORKSPACE_NOT_BOUND: host commands require a local conversation workspace")
	}
	root, err := s.workspace.LocalWorkspaceRoot(invocation.ExecContext.WorkspaceID)
	if err != nil {
		return nil, err
	}
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	command := stringValue(obj, "command")
	if command == "" {
		return nil, errors.New("command is required")
	}
	directory, err := resolveHostCommandDirectory(root, stringValue(obj, "cwd", "workingDir"))
	if err != nil {
		return nil, err
	}
	timeoutMS := intValue(obj, 30000, "timeoutMs")
	if timeoutMS < 100 || timeoutMS > 300000 {
		return nil, errors.New("timeoutMs must be between 100 and 300000")
	}
	maxOutput := intValue(obj, 262144, "maxOutputBytes")
	if maxOutput < 1 || maxOutput > 1048576 {
		return nil, errors.New("maxOutputBytes must be between 1 and 1048576")
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	var process *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		shell, lookupErr := exec.LookPath("pwsh.exe")
		if lookupErr != nil {
			shell, lookupErr = exec.LookPath("powershell.exe")
			if lookupErr != nil {
				return nil, fmt.Errorf("PowerShell executable unavailable: %w", lookupErr)
			}
		}
		process = exec.CommandContext(execCtx, shell, "-NoProfile", "-NonInteractive", "-Command", command)
	case "linux", "darwin":
		process = exec.CommandContext(execCtx, "/bin/sh", "-lc", command)
	default:
		return nil, fmt.Errorf("host command execution is not supported on %s", runtime.GOOS)
	}
	process.Dir = directory
	stdout := &hostCommandOutput{limit: maxOutput}
	stderr := &hostCommandOutput{limit: maxOutput}
	process.Stdout = stdout
	process.Stderr = stderr
	startedAt := time.Now()
	runErr := process.Run()
	outText, outTruncated := stdout.Snapshot()
	errText, errTruncated := stderr.Snapshot()
	exitCode := 0
	if process.ProcessState != nil {
		exitCode = process.ProcessState.ExitCode()
	}
	timedOut := errors.Is(execCtx.Err(), context.DeadlineExceeded)
	result := map[string]any{
		"exitCode": exitCode, "stdout": outText, "stderr": errText,
		"truncated": outTruncated || errTruncated, "timedOut": timedOut,
		"cwd": directory, "durationMs": time.Since(startedAt).Milliseconds(),
	}
	if runErr != nil {
		if timedOut {
			return nil, fmt.Errorf("host command timed out after %dms: %s", timeoutMS, errText)
		}
		return nil, fmt.Errorf("host command failed with exit code %d: %v; stderr: %s; stdout: %s", exitCode, runErr, errText, outText)
	}
	return marshalResult(result)
}
