//go:build linux && !android

package shell

import (
	"context"
	"fmt"
	"strings"
)

type ShellHandler interface {
	Handle(ctx context.Context, request ShellExecuteRequest) (ShellExecuteResult, error)
}

type ShellHandlerImpl struct {
	executor ShellExecutor
}

func NewShellHandler(executor ShellExecutor) *ShellHandlerImpl {
	return &ShellHandlerImpl{executor: executor}
}

func (h *ShellHandlerImpl) Handle(ctx context.Context, request ShellExecuteRequest) (ShellExecuteResult, error) {
	result := h.executor.Execute(ctx, request)

	if result.ExitCode != 0 {
		if result.Metadata == nil {
			result.Metadata = make(map[string]string)
		}
		result.Metadata["exit_code"] = fmt.Sprintf("%d", result.ExitCode)
		if result.Signal != "" {
			result.Metadata["signal"] = result.Signal
		}
	}

	return result, nil
}

func ParseShellExecuteInput(input map[string]any) (ShellExecuteRequest, error) {
	req := ShellExecuteRequest{
		Mode: ShellModeArgv,
	}

	if mode, ok := input["mode"].(string); ok && mode != "" {
		req.Mode = ShellMode(mode)
	}

	if cmd, ok := input["command"].(string); ok {
		req.Command = cmd
	}

	if exe, ok := input["executable"].(string); ok {
		req.Executable = exe
	}

	if args, ok := input["args"].([]any); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				req.Args = append(req.Args, s)
			}
		}
	}

	if workDir, ok := input["workingDir"].(string); ok {
		req.WorkingDir = workDir
	}

	if env, ok := input["environment"].(map[string]any); ok {
		req.Environment = make(map[string]string)
		for k, v := range env {
			if s, ok := v.(string); ok {
				req.Environment[k] = s
			}
		}
	}

	if stdin, ok := input["stdin"].(string); ok {
		req.Stdin = stdin
	}

	if timeout, ok := input["timeoutMs"].(float64); ok {
		req.TimeoutMs = int64(timeout)
	}

	if maxOutput, ok := input["maxOutputBytes"].(float64); ok {
		req.MaxOutputBytes = int64(maxOutput)
	}

	return req, nil
}

func MapShellError(err *Error) (string, string) {
	if err == nil {
		return "", ""
	}
	return err.Code(), err.Message()
}

func isExitSignal(exitCode int) bool {
	return exitCode > 128 && exitCode <= 165
}

func extractSignalFromCode(exitCode int) string {
	if !isExitSignal(exitCode) {
		return ""
	}
	sig := exitCode - 128
	switch sig {
	case 9:
		return "SIGKILL"
	case 15:
		return "SIGTERM"
	case 2:
		return "SIGINT"
	case 3:
		return "SIGQUIT"
	case 11:
		return "SIGSEGV"
	case 6:
		return "SIGABRT"
	default:
		return fmt.Sprintf("SIG_%d", sig)
	}
}

func SanitizeCommandLog(cmd string) string {
	if len(cmd) > 200 {
		return cmd[:200] + "..."
	}
	return strings.ReplaceAll(cmd, "\n", "\\n")
}
