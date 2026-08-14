package skill

import (
	"context"
	"time"
)

type JavaScriptExecutionContext struct {
	ExecutionID     string
	ExtensionID     string
	SkillName       string
	EntryPoint      string
	Content         []byte
	ModuleFormat    string
	Args            map[string]any
	Inputs          map[string]string
	Permissions     []string
	Timeout         time.Duration
	WorkingDir      string
	Environment     map[string]string
	ContentTreeHash string
}

type JavaScriptExecutorResult struct {
	Status   string
	Output   string
	ExitCode int
	Duration time.Duration
	Error    string
}

type JavaScriptExecutor interface {
	Execute(ctx context.Context, execCtx JavaScriptExecutionContext) (*JavaScriptExecutorResult, error)
	Supports(format string) bool
}

type nodeJavaScriptExecutor struct {
	backend JavaScriptRuntimeBackend
}

type JavaScriptRuntimeBackend interface {
	ExecuteScript(ctx context.Context, content string, args []string, timeout time.Duration, workingDir string) (string, int, error)
}

func NewNodeJavaScriptExecutor(backend JavaScriptRuntimeBackend) JavaScriptExecutor {
	return &nodeJavaScriptExecutor{backend: backend}
}

func (e *nodeJavaScriptExecutor) Supports(format string) bool {
	switch format {
	case ".js", ".mjs", ".cjs", ".ts":
		return true
	default:
		return false
	}
}

func (e *nodeJavaScriptExecutor) Execute(ctx context.Context, execCtx JavaScriptExecutionContext) (*JavaScriptExecutorResult, error) {
	if e.backend == nil {
		return &JavaScriptExecutorResult{Status: StatusFailed, Error: "javascript backend not configured"}, ErrScriptInterpreterUnavailable
	}
	timeout := execCtx.Timeout
	if timeout <= 0 {
		timeout = DefaultScriptTimeout
	}
	output, exitCode, err := e.backend.ExecuteScript(ctx, string(execCtx.Content), nil, timeout, execCtx.WorkingDir)
	result := &JavaScriptExecutorResult{
		Status:   StatusSuccess,
		Output:   output,
		ExitCode: exitCode,
	}
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		return result, err
	}
	return result, nil
}
