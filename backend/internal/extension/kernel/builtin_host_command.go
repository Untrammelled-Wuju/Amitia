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
	requested = strings.TrimSpace(requested)
	if strings.HasPrefix(requested, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		requested = filepath.Join(home, strings.TrimPrefix(requested, "~/"))
	}
	directory := requested
	if directory == "" {
		if strings.TrimSpace(root) == "" {
			return "", errors.New("working directory is required when no project folder is selected")
		}
		directory = root
	} else if !filepath.IsAbs(directory) {
		if strings.TrimSpace(root) == "" {
			return "", errors.New("relative working directory requires a project folder")
		}
		directory = filepath.Join(root, directory)
	}
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("resolve host command directory: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", errors.New("working directory does not exist")
	}
	return canonical, nil
}

func (s *BuiltinUtilityService) executeHostCommand(ctx context.Context, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	obj, err := unmarshalObject(input)
	if err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(stringValue(obj, "workspaceId"))
	if workspaceID == "" && invocation.ExecContext != nil {
		workspaceID = strings.TrimSpace(invocation.ExecContext.PrimaryWorkspaceID)
	}
	if workspaceID == "" && invocation.ExecContext != nil {
		workspaceID = strings.TrimSpace(invocation.ExecContext.WorkspaceID)
	}
	root := ""
	if workspaceID != "" {
		if executionWorkspaceRoot(invocation.ExecContext, workspaceID) == "" {
			return nil, errors.New("WORKSPACE_SCOPE_MISMATCH: selected workspace is outside the current project")
		}
		if s.workspace == nil {
			return nil, errors.New("workspace service not configured")
		}
		root, err = s.workspace.LocalWorkspaceRoot(workspaceID)
		if err != nil {
			return nil, err
		}
	}
	command := stringValue(obj, "command")
	if command == "" {
		return nil, errors.New("command is required")
	}
	requestedDirectory := stringValue(obj, "cwd", "workingDir")
	if mountedWorkspaceID, relative, ok := parseMountedWorkspaceDirectory(requestedDirectory); ok {
		if mountedWorkspaceID != workspaceID {
			workspaceID = mountedWorkspaceID
			if s.workspace == nil {
				return nil, errors.New("workspace service not configured")
			}
			root, err = s.workspace.LocalWorkspaceRoot(workspaceID)
			if err != nil {
				return nil, err
			}
		}
		requestedDirectory = relative
	}
	directory, err := resolveHostCommandDirectory(root, requestedDirectory)
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
		"cwd": directory, "workspaceId": workspaceID, "durationMs": time.Since(startedAt).Milliseconds(),
	}
	if runErr != nil {
		if timedOut {
			return nil, fmt.Errorf("host command timed out after %dms: %s", timeoutMS, errText)
		}
		return nil, fmt.Errorf("host command failed with exit code %d: %v; stderr: %s; stdout: %s", exitCode, runErr, errText, outText)
	}
	return marshalResult(result)
}

func parseMountedWorkspaceDirectory(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	const prefix = "amitia://workspace/@"
	if !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(value, prefix)
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", "", false
	}
	relative := ""
	if len(parts) == 2 {
		relative = parts[1]
	}
	return strings.TrimSpace(parts[0]), relative, true
}
