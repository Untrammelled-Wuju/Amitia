package management

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gamehost/control"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

func HTTPStatus(err error) int {
	var topologyErr *ghruntime.TopologyError
	if errors.As(err, &topologyErr) {
		return statusForCode(topologyErr.Code)
	}
	var executionErr *ghruntime.ExecutionError
	if errors.As(err, &executionErr) {
		return statusForCode(executionErr.Code)
	}
	var restartErr *ghruntime.RuntimeRestartError
	if errors.As(err, &restartErr) {
		return statusForCode(restartErr.Code)
	}
	var authorityErr *control.AuthorityError
	if errors.As(err, &authorityErr) {
		return statusForDomainCode(authorityErr.Code)
	}
	var hostErr *ghdomain.HostError
	if errors.As(err, &hostErr) {
		return statusForDomainCode(hostErr.Code)
	}
	return http.StatusInternalServerError
}

func statusForDomainCode(code ghdomain.ErrorCode) int {
	switch code {
	case ghdomain.ErrNotFound:
		return http.StatusNotFound
	case ghdomain.ErrInvalidArgument, ghdomain.ErrProtocolMismatch:
		return http.StatusBadRequest
	case ghdomain.ErrAlreadyExists,
		ghdomain.ErrInvalidState,
		ghdomain.ErrConflict:
		return http.StatusConflict
	case ghdomain.ErrPermissionDenied:
		return http.StatusForbidden
	case ghdomain.ErrUnsupported:
		return http.StatusNotImplemented
	case ghdomain.ErrRuntimeUnavailable:
		return http.StatusServiceUnavailable
	case ghdomain.ErrResourceExhausted:
		return http.StatusTooManyRequests
	case ghdomain.ErrTimeout:
		return http.StatusGatewayTimeout
	case ghdomain.ErrCancelled:
		return 499
	}
	return http.StatusInternalServerError
}

func statusForCode(code ghruntime.ErrorCode) int {
	switch code {
	case ghruntime.ErrNotFound, ghruntime.ErrDependencyNotFound:
		return http.StatusNotFound
	case ghruntime.ErrInvalidArgument:
		return http.StatusBadRequest
	case ghruntime.ErrAlreadyExists,
		ghruntime.ErrDuplicateService,
		ghruntime.ErrDuplicateDependency,
		ghruntime.ErrSelfDependency,
		ghruntime.ErrPluginMismatch,
		ghruntime.ErrInvalidState,
		ghruntime.ErrRevisionChanged,
		ghruntime.ErrLifecycleConflict,
		ghruntime.ErrEmergencyActive,
		ghruntime.ErrRuntimeOperationLocked:
		return http.StatusConflict
	case ghruntime.ErrUnsupported:
		return http.StatusNotImplemented
	case ghruntime.ErrRuntimeUnavailable,
		ghruntime.ErrServiceUnavailable,
		ghruntime.ErrDependencyNotSatisfied:
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func WriteError(c *gin.Context, err error) {
	status := HTTPStatus(err)
	c.JSON(status, gin.H{"code": status, "msg": err.Error()})
}
