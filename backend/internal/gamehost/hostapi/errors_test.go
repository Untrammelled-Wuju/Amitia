package hostapi

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

func TestMapGatewayError_PermissionDenied(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "denied"})
	if err.Code != CodePermissionDenied {
		t.Fatalf("expected CodePermissionDenied, got %s", err.Code)
	}
	if err.Message != "denied" {
		t.Fatalf("expected preserved message, got %s", err.Message)
	}
}

func TestMapGatewayError_ScopeDenied(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: "out of scope"})
	if err.Code != CodePermissionDenied {
		t.Fatalf("expected scope_denied to map to CodePermissionDenied, got %s", err.Code)
	}
}

func TestMapGatewayError_MethodNotFound(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeMethodNotFound, Message: "not found"})
	if err.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %s", err.Code)
	}
}

func TestMapGatewayError_Timeout(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeTimeout, Message: "deadline exceeded"})
	if err.Code != CodeTimeout {
		t.Fatalf("expected CodeTimeout, got %s", err.Code)
	}
}

func TestMapGatewayError_Cancelled(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeCancelled, Message: "canceled"})
	if err.Code != CodeCancelled {
		t.Fatalf("expected CodeCancelled, got %s", err.Code)
	}
}

func TestMapGatewayError_RateLimited(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeRateLimited, Message: "slow down"})
	if err.Code != CodeResourceExhausted {
		t.Fatalf("expected CodeResourceExhausted, got %s", err.Code)
	}
}

func TestMapGatewayError_NilError(t *testing.T) {
	err := mapGatewayError(nil)
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal for nil, got %s", err.Code)
	}
}

func TestMapGatewayError_UnknownFallsBackToInternal(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: "nonexistent_code", Message: "?"})
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal, got %s", err.Code)
	}
}

func TestMapGatewayError_GenerationStale(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeGenerationStale})
	if err.Code != CodeInvalidRequest {
		t.Fatalf("expected CodeInvalidRequest for generation stale, got %s", err.Code)
	}
}

func TestMapGatewayError_ResourceNotFound(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeResourceNotFound, Message: "missing"})
	if err.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %s", err.Code)
	}
}

func TestMapGatewayError_StateConflict(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeStateConflict, Message: "conflict"})
	if err.Code != CodeInvalidRequest {
		t.Fatalf("expected CodeInvalidRequest, got %s", err.Code)
	}
}

func TestMapGatewayError_HostUnavailable(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeHostUnavailable, Message: "down"})
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal, got %s", err.Code)
	}
}

func TestMapGatewayError_UIHostUnavailable(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeUIHostUnavailable})
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal for UI host unavailable, got %s", err.Code)
	}
}

func TestMapGatewayError_DialogHostUnavailable(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeDialogHostUnavailable})
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal for dialog host unavailable, got %s", err.Code)
	}
}

func TestMapGatewayError_NavigationHostUnavailable(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeNavigationHostUnavailable})
	if err.Code != CodeInternal {
		t.Fatalf("expected CodeInternal for navigation host unavailable, got %s", err.Code)
	}
}

func TestMapGatewayError_IdentityInvalid(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeIdentityInvalid, Message: "bad peer"})
	if err.Code != CodeInvalidRequest {
		t.Fatalf("expected CodeInvalidRequest, got %s", err.Code)
	}
}

func TestMapGatewayError_ApprovalRequired(t *testing.T) {
	err := mapGatewayError(&host_api.Error{Code: host_api.ErrorCodeApprovalRequired, Message: "approval needed"})
	if err.Code != CodePermissionDenied {
		t.Fatalf("expected CodePermissionDenied for approval required, got %s", err.Code)
	}
}

func TestError_ErrorString(t *testing.T) {
	err := &Error{Code: CodePermissionDenied, Message: "no access"}
	if err.Error() != "permission_denied: no access" {
		t.Fatalf("unexpected Error string: %s", err.Error())
	}
	err2 := &Error{Code: CodeInternal}
	if err2.Error() != "internal" {
		t.Fatalf("unexpected Error string: %s", err2.Error())
	}
}
