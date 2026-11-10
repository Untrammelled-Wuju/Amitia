package host_api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

func newClassificationGateway(t *testing.T, route Route) *DefaultGateway {
	t.Helper()
	gateway := NewDefaultGateway()
	gateway.SetPermissionChecker(PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, ScopePolicy) error {
		return nil
	}))
	if err := gateway.RegisterRoute(route); err != nil {
		t.Fatal(err)
	}
	return gateway
}

func callClassificationRoute(gateway *DefaultGateway) CallResult {
	return gateway.Call(context.Background(), CallRequest{
		CallID:          "classification-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1"},
		Method:          MethodClipboardRead,
		Version:         1,
	})
}

func TestGatewayPreservesUpstreamErrorClass(t *testing.T) {
	message := `upstream HTTP 500 Internal Server Error: {"code":500,"msg":"读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused"}`
	gateway := newClassificationGateway(t, Route{
		Method:  MethodClipboardRead,
		Version: 1,
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{}, fmt.Errorf("host.network.request: %w", &Error{Code: ErrorCodeUpstream, Message: message})
		},
	})
	result := callClassificationRoute(gateway)
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrorCodeUpstream {
		t.Fatalf("unexpected code: %s", result.Error.Code)
	}
	if result.Status != StatusFailed {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	if !strings.Contains(result.Error.Message, "读取插件市场分类") || !strings.Contains(result.Error.Message, "connection refused") {
		t.Fatalf("original error text lost: %s", result.Error.Message)
	}
	if !errors.Is(result.Error, ErrUpstream) {
		t.Fatalf("expected ErrUpstream in chain: %v", result.Error)
	}
}

func TestGatewayMapsContextDeadlineToTimeout(t *testing.T) {
	gateway := newClassificationGateway(t, Route{
		Method:  MethodClipboardRead,
		Version: 1,
		Timeout: 10 * time.Millisecond,
		Handler: func(ctx context.Context, _ CallRequest) (CallResult, error) {
			<-ctx.Done()
			return CallResult{}, fmt.Errorf("host.network.request: %w", ctx.Err())
		},
	})
	restore := timeoutpolicy.Configure(timeoutpolicy.Settings{Seconds: 180})
	defer restore()
	result := callClassificationRoute(gateway)
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrorCodeTimeout || result.Status != StatusTimeout {
		t.Fatalf("unexpected timeout classification: %s %s %s", result.Status, result.Error.Code, result.Error.Message)
	}
	if !errors.Is(result.Error, ErrTimeout) {
		t.Fatalf("expected ErrTimeout in chain: %v", result.Error)
	}
}

func TestGatewayMapsContextCancelToCancelled(t *testing.T) {
	gateway := newClassificationGateway(t, Route{
		Method:  MethodClipboardRead,
		Version: 1,
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{}, fmt.Errorf("host.network.request: %w", context.Canceled)
		},
	})
	result := callClassificationRoute(gateway)
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrorCodeCancelled || result.Status != StatusCancelled {
		t.Fatalf("unexpected cancel classification: %s %s %s", result.Status, result.Error.Code, result.Error.Message)
	}
	if !errors.Is(result.Error, ErrCancelled) {
		t.Fatalf("expected ErrCancelled in chain: %v", result.Error)
	}
}

func TestGatewayKeepsPermissionDeniedClass(t *testing.T) {
	gateway := newClassificationGateway(t, Route{
		Method:  MethodClipboardRead,
		Version: 1,
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{}, fmt.Errorf("host.clipboard.read: %w", ErrPermissionDenied)
		},
	})
	result := callClassificationRoute(gateway)
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrorCodePermissionDenied || result.Status != StatusRejected {
		t.Fatalf("unexpected permission classification: %s %s", result.Status, result.Error.Code)
	}
}
