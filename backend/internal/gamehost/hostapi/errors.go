package hostapi

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

const (
	CodeInvalidRequest    = "invalid_request"
	CodeNotFound          = "not_found"
	CodeUnsupported       = "unsupported"
	CodePermissionDenied  = "permission_denied"
	CodeResourceExhausted = "resource_exhausted"
	CodeTimeout           = "timeout"
	CodeCancelled         = "cancelled"
	CodeInternal          = "internal"
	CodeNotReady          = "not_ready"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
	ErrNotReady = &Error{Code: CodeNotReady, Message: "connection not ready"}
)

func mapGatewayError(gwErr *host_api.Error) *Error {
	if gwErr == nil {
		return &Error{Code: CodeInternal, Message: "gateway returned no error detail"}
	}
	switch gwErr.Code {
	case host_api.ErrorCodePermissionDenied:
		return &Error{Code: CodePermissionDenied, Message: gwErr.Message}
	case host_api.ErrorCodeScopeDenied:
		return &Error{Code: CodePermissionDenied, Message: gwErr.Message}
	case host_api.ErrorCodeMethodNotFound:
		return &Error{Code: CodeNotFound, Message: gwErr.Message}
	case host_api.ErrorCodeVersionUnsupported:
		return &Error{Code: CodeUnsupported, Message: gwErr.Message}
	case host_api.ErrorCodeInputInvalid:
		return &Error{Code: CodeInvalidRequest, Message: gwErr.Message}
	case host_api.ErrorCodeTimeout:
		return &Error{Code: CodeTimeout, Message: gwErr.Message}
	case host_api.ErrorCodeCancelled:
		return &Error{Code: CodeCancelled, Message: gwErr.Message}
	case host_api.ErrorCodeRateLimited:
		return &Error{Code: CodeResourceExhausted, Message: gwErr.Message}
	case host_api.ErrorCodeResourceNotFound:
		return &Error{Code: CodeNotFound, Message: gwErr.Message}
	case host_api.ErrorCodeIdentityInvalid:
		return &Error{Code: CodeInvalidRequest, Message: gwErr.Message}
	case host_api.ErrorCodeGenerationStale:
		return &Error{Code: CodeInvalidRequest, Message: "connection generation stale"}
	case host_api.ErrorCodeApprovalRequired:
		return &Error{Code: CodePermissionDenied, Message: "approval required"}
	case host_api.ErrorCodeStateConflict:
		return &Error{Code: CodeInvalidRequest, Message: gwErr.Message}
	case host_api.ErrorCodeOutputInvalid,
		host_api.ErrorCodeUIHostUnavailable,
		host_api.ErrorCodeDialogHostUnavailable,
		host_api.ErrorCodeNavigationHostUnavailable:
		return &Error{Code: CodeInternal, Message: gwErr.Message}
	case host_api.ErrorCodeHostUnavailable:
		return &Error{Code: CodeInternal, Message: gwErr.Message}
	default:
		return &Error{Code: CodeInternal, Message: gwErr.Message}
	}
}
