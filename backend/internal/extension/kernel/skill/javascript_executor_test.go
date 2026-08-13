package skill

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockJSBackend struct {
	output   string
	exitCode int
	err      error
}

func (m *mockJSBackend) ExecuteScript(ctx context.Context, content string, args []string, timeout time.Duration, workingDir string) (string, int, error) {
	return m.output, m.exitCode, m.err
}

func TestNodeJavaScriptExecutorSupports(t *testing.T) {
	exec := NewNodeJavaScriptExecutor(&mockJSBackend{})
	supported := []string{".js", ".mjs", ".cjs", ".ts"}
	for _, format := range supported {
		if !exec.Supports(format) {
			t.Fatalf("expected to support %s", format)
		}
	}
	unsupported := []string{".py", ".sh", ".exe"}
	for _, format := range unsupported {
		if exec.Supports(format) {
			t.Fatalf("expected not to support %s", format)
		}
	}
}

func TestNodeJavaScriptExecutorExecuteSuccess(t *testing.T) {
	backend := &mockJSBackend{output: "hello world", exitCode: 0}
	exec := NewNodeJavaScriptExecutor(backend)
	result, err := exec.Execute(context.Background(), JavaScriptExecutionContext{
		Content:      []byte("console.log('hi')"),
		Timeout:      5 * time.Second,
		WorkingDir:   "/tmp/test",
		ModuleFormat: ".js",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Output != "hello world" {
		t.Fatalf("expected 'hello world', got %s", result.Output)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestNodeJavaScriptExecutorExecuteError(t *testing.T) {
	backend := &mockJSBackend{output: "", exitCode: 1, err: errors.New("runtime error")}
	exec := NewNodeJavaScriptExecutor(backend)
	result, err := exec.Execute(context.Background(), JavaScriptExecutionContext{
		Content:    []byte("bad code"),
		Timeout:    5 * time.Second,
		WorkingDir: "/tmp/test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error != "runtime error" {
		t.Fatalf("expected 'runtime error', got %s", result.Error)
	}
}

func TestNodeJavaScriptExecutorNilBackend(t *testing.T) {
	exec := NewNodeJavaScriptExecutor(nil)
	result, err := exec.Execute(context.Background(), JavaScriptExecutionContext{
		Content:    []byte("test"),
		WorkingDir: "/tmp",
	})
	if err == nil {
		t.Fatal("expected error for nil backend")
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestNodeJavaScriptExecutorDefaultTimeout(t *testing.T) {
	backend := &mockJSBackend{output: "ok", exitCode: 0}
	exec := NewNodeJavaScriptExecutor(backend)
	result, err := exec.Execute(context.Background(), JavaScriptExecutionContext{
		Content:    []byte("test"),
		WorkingDir: "/tmp",
		Timeout:    0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Fatalf("expected success with default timeout, got %s", result.Status)
	}
}
