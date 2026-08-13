//go:build linux && !android

package packages

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidlinux/shell"
)

type mockShellExecutor struct {
	executions []shell.ShellExecuteRequest
}

func (m *mockShellExecutor) Execute(ctx context.Context, req shell.ShellExecuteRequest) shell.ShellExecuteResult {
	m.executions = append(m.executions, req)
	return shell.ShellExecuteResult{ExitCode: 0}
}

func TestHandler_Handle_InvalidOperation(t *testing.T) {
	svc := &mockPackageService{}
	handler := NewPackagesHandler(svc)

	_, err := handler.Handle(context.Background(), "invalid.op", nil)
	if err == nil {
		t.Error("expected error for invalid operation")
	}
}

func TestHandler_Handle_StatusOperation(t *testing.T) {
	svc := &mockPackageService{}
	handler := NewPackagesHandler(svc)

	result, err := handler.Handle(context.Background(), OpStatus, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["supported"] != true {
		t.Error("expected supported=true")
	}
}

type mockPackageService struct{}

func (m *mockPackageService) Status(ctx context.Context) (RuntimePackagesStatus, error) {
	return RuntimePackagesStatus{Supported: true}, nil
}

func (m *mockPackageService) AptUpdate(ctx context.Context, timeoutMs int64) (*PackageInstallResult, error) {
	return nil, nil
}
func (m *mockPackageService) AptInstall(ctx context.Context, req AptInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	return nil, nil
}
func (m *mockPackageService) AptQuery(ctx context.Context, packages []string) (*PackageInstallResult, error) {
	return nil, nil
}
func (m *mockPackageService) PythonStatus(ctx context.Context, timeoutMs int64) (PythonStatus, error) {
	return PythonStatus{}, nil
}
func (m *mockPackageService) PythonInvoke(ctx context.Context, req PythonInvokeRequest) (*InvokeResult, error) {
	return nil, nil
}
func (m *mockPackageService) PythonInstall(ctx context.Context, req PythonPackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	return nil, nil
}
func (m *mockPackageService) NodeStatus(ctx context.Context, timeoutMs int64) (NodeStatus, error) {
	return NodeStatus{}, nil
}
func (m *mockPackageService) NodeInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	return nil, nil
}
func (m *mockPackageService) NodeInstall(ctx context.Context, req NodePackageInstallRequest, timeoutMs int64) (*PackageInstallResult, error) {
	return nil, nil
}
func (m *mockPackageService) NpxInvoke(ctx context.Context, req NodeInvokeRequest) (*InvokeResult, error) {
	return nil, nil
}
