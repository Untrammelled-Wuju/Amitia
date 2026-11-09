package git

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/u-ai/backend/internal/workspace"
)

func HTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrGitCancelled):
		return 499
	case errors.Is(err, ErrGitTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, ErrRepositoryNotFound),
		errors.Is(err, ErrBranchNotFound),
		errors.Is(err, ErrGitRemoteNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrRepositoryInvalid),
		errors.Is(err, ErrGitRefInvalid),
		errors.Is(err, ErrGitPathInvalid),
		errors.Is(err, ErrGitPathOutsideRepository),
		errors.Is(err, ErrGitRemoteInvalid),
		errors.Is(err, ErrIsolatedInvalidRequest),
		errors.Is(err, ErrIsolatedTooLarge),
		errors.Is(err, ErrGitSubmoduleUnsupported),
		errors.Is(err, ErrGitLFSUnsupported),
		errors.Is(err, ErrGitSymlinkUnsupported),
		errors.Is(err, ErrGitExternalFilterUnsupported),
		errors.Is(err, ErrGitHookExecutionDisabled),
		errors.Is(err, ErrGitOperationUnsupported):
		return http.StatusBadRequest
	case errors.Is(err, ErrRepositoryUntrusted),
		errors.Is(err, ErrInternalMetadataDenied),
		errors.Is(err, ErrGitRemoteAuthFailed),
		errors.Is(err, ErrGitRemoteHostKeyChanged):
		return http.StatusForbidden
	case errors.Is(err, ErrGitDirty),
		errors.Is(err, ErrGitConflictState),
		errors.Is(err, ErrGitDiverged),
		errors.Is(err, ErrBranchExists),
		errors.Is(err, ErrGitPullNotFastForward),
		errors.Is(err, ErrGitPushRejected),
		errors.Is(err, ErrIsolatedDirty):
		return http.StatusConflict
	case errors.Is(err, ErrGitRemoteNetworkFailed),
		errors.Is(err, ErrGitRemoteTLSFailed):
		return http.StatusBadGateway
	case errors.Is(err, ErrGitUnavailable):
		return http.StatusServiceUnavailable
	}
	return workspace.ErrorStatus(err)
}

func WriteHTTPError(c *gin.Context, err error) {
	c.JSON(HTTPStatus(err), gin.H{"error": err.Error()})
}
