package workspace

import (
	"errors"
	"fmt"
)

var (
	ErrWorkspaceUnavailable          = errors.New("workspace unavailable")
	ErrWorkspaceNotFound             = errors.New("workspace not found")
	ErrMountNotFound                 = errors.New("workspace mount not found")
	ErrMountUnavailable              = errors.New("workspace mount unavailable")
	ErrReadOnly                      = errors.New("workspace is read-only")
	ErrPermissionDenied              = errors.New("workspace permission denied")
	ErrInvalidURI                    = errors.New("invalid workspace URI")
	ErrInvalidPath                   = errors.New("invalid workspace path")
	ErrPathTraversal                 = errors.New("workspace path traversal")
	ErrPathAmbiguous                 = errors.New("workspace path ambiguous")
	ErrSymlinkNotAllowed             = errors.New("workspace symlink not allowed")
	ErrRootMutationDenied            = errors.New("workspace root mutation denied")
	ErrFileNotFound                  = errors.New("workspace file not found")
	ErrDirectoryNotFound             = errors.New("workspace directory not found")
	ErrAlreadyExists                 = errors.New("workspace entry already exists")
	ErrNotDirectory                  = errors.New("not a directory")
	ErrNotFile                       = errors.New("not a file")
	ErrDirectoryNotEmpty             = errors.New("directory not empty")
	ErrReadFailed                    = errors.New("workspace read failed")
	ErrWriteFailed                   = errors.New("workspace write failed")
	ErrListFailed                    = errors.New("workspace list failed")
	ErrCreateFailed                  = errors.New("workspace create failed")
	ErrRenameFailed                  = errors.New("workspace rename failed")
	ErrMoveFailed                    = errors.New("workspace move failed")
	ErrMovePartial                   = errors.New("workspace move partial")
	ErrCopyFailed                    = errors.New("workspace copy failed")
	ErrDeleteFailed                  = errors.New("workspace delete failed")
	ErrTooManyEntries                = errors.New("too many entries")
	ErrResourceTooLarge              = errors.New("resource too large")
	ErrDepthExceeded                 = errors.New("max depth exceeded")
	ErrCrossMountMoveUnsupported     = errors.New("cross-mount move unsupported")
	ErrCrossMountCopyUnsupported     = errors.New("cross-mount copy unsupported")
	ErrOperationUnsupported          = errors.New("operation unsupported")
	ErrCursorStale                   = errors.New("workspace cursor stale")
	ErrSAFUnavailable                = errors.New("SAF unavailable")
	ErrSAFPermissionRevoked          = errors.New("SAF permission revoked")
	ErrSAFProviderUnavailable        = errors.New("SAF provider unavailable")
	ErrVirtualDocumentUnsupported    = errors.New("virtual document unsupported")
	ErrOperationCancelled            = errors.New("workspace operation cancelled")
	ErrOperationTimeout              = errors.New("workspace operation timeout")
)

type WorkspaceError struct {
	Code    string
	Message string
	Cause   error
}

func (e *WorkspaceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("workspace error [%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("workspace error [%s]: %s", e.Code, e.Message)
}

func (e *WorkspaceError) Unwrap() error {
	return e.Cause
}

func NewWorkspaceError(code string, message string, cause error) *WorkspaceError {
	return &WorkspaceError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
