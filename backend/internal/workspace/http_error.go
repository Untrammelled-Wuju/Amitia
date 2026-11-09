package workspace

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ErrorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrMountNotFound),
		errors.Is(err, ErrWorkspaceNotFound),
		errors.Is(err, ErrFileNotFound),
		errors.Is(err, ErrDirectoryNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidURI),
		errors.Is(err, ErrInvalidPath),
		errors.Is(err, ErrPathTraversal),
		errors.Is(err, ErrPathAmbiguous),
		errors.Is(err, ErrSymlinkNotAllowed),
		errors.Is(err, ErrRootMutationDenied),
		errors.Is(err, ErrInternalMetadataDenied),
		errors.Is(err, ErrNotDirectory),
		errors.Is(err, ErrNotFile),
		errors.Is(err, ErrDirectoryNotEmpty),
		errors.Is(err, ErrResourceTooLarge),
		errors.Is(err, ErrDepthExceeded),
		errors.Is(err, ErrTooManyEntries),
		errors.Is(err, ErrCursorStale),
		errors.Is(err, ErrVirtualDocumentUnsupported):
		return http.StatusBadRequest
	case errors.Is(err, ErrReadOnly),
		errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrSAFPermissionRevoked):
		return http.StatusForbidden
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrOperationUnsupported),
		errors.Is(err, ErrCrossMountMoveUnsupported),
		errors.Is(err, ErrCrossMountCopyUnsupported):
		return http.StatusNotImplemented
	case errors.Is(err, ErrWorkspaceUnavailable),
		errors.Is(err, ErrMountUnavailable),
		errors.Is(err, ErrSAFProviderUnavailable):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func WriteError(c *gin.Context, err error) {
	c.JSON(ErrorStatus(err), gin.H{"error": err.Error()})
}
