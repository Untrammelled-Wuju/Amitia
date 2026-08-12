//go:build linux && !android

package fileops

import "fmt"

type Error struct {
	code    string
	message string
}

func (e *Error) Error() string {
	return e.code + ": " + e.message
}

func (e *Error) Code() string {
	return e.code
}

const (
	ErrCodeFileopsNotAvailable = "fileops.not_available"
	ErrCodePathDenied          = "fileops.path_denied"
	ErrCodeFileNotFound        = "fileops.file_not_found"
	ErrCodeFileExists          = "fileops.file_exists"
	ErrCodeReadLimitExceeded   = "fileops.read_limit_exceeded"
	ErrCodeWriteLimitExceeded  = "fileops.write_limit_exceeded"
	ErrCodeListLimitExceeded   = "fileops.list_limit_exceeded"
	ErrCodeSearchLimitExceeded = "fileops.search_limit_exceeded"
	ErrCodeInvalidMode         = "fileops.invalid_mode"
	ErrCodeSymlinkDenied       = "fileops.symlink_denied"
	ErrCodeChmodDenied         = "fileops.chmod_denied"
	ErrCodeMutationRootDenied  = "fileops.mutation_root_denied"
	ErrCodeRecursiveLimit      = "fileops.recursive_limit"
	ErrCodeCancelled           = "fileops.cancelled"
	ErrCodeIOFailed            = "fileops.io_failed"
)

func ErrNotAvailable(reason string) *Error {
	return &Error{code: ErrCodeFileopsNotAvailable, message: "fileops not available: " + reason}
}

func ErrPathDenied(path string, reason string) *Error {
	return &Error{code: ErrCodePathDenied, message: fmt.Sprintf("path denied: %s (%s)", path, reason)}
}

func ErrFileNotFound(path string) *Error {
	return &Error{code: ErrCodeFileNotFound, message: "file not found: " + path}
}

func ErrFileExists(path string) *Error {
	return &Error{code: ErrCodeFileExists, message: "file already exists: " + path}
}

func ErrReadLimitExceeded(limit int64) *Error {
	return &Error{code: ErrCodeReadLimitExceeded, message: fmt.Sprintf("read limit exceeded: %d bytes", limit)}
}

func ErrWriteLimitExceeded(limit int64) *Error {
	return &Error{code: ErrCodeWriteLimitExceeded, message: fmt.Sprintf("write limit exceeded: %d bytes", limit)}
}

func ErrListLimitExceeded(limit int) *Error {
	return &Error{code: ErrCodeListLimitExceeded, message: fmt.Sprintf("list entries exceeded: %d", limit)}
}

func ErrSearchLimitExceeded(limit int) *Error {
	return &Error{code: ErrCodeSearchLimitExceeded, message: fmt.Sprintf("search results exceeded: %d", limit)}
}

func ErrInvalidMode(mode uint32) *Error {
	return &Error{code: ErrCodeInvalidMode, message: fmt.Sprintf("invalid file mode: %o", mode)}
}

func ErrSymlinkDenied(path string) *Error {
	return &Error{code: ErrCodeSymlinkDenied, message: "symlink creation denied: " + path}
}

func ErrChmodDenied(path string) *Error {
	return &Error{code: ErrCodeChmodDenied, message: "chmod denied: " + path}
}

func ErrMutationRootDenied(path string) *Error {
	return &Error{code: ErrCodeMutationRootDenied, message: "mutation denied for protected filesystem: " + path}
}

func ErrRecursiveLimit(reason string) *Error {
	return &Error{code: ErrCodeRecursiveLimit, message: "recursive limit reached: " + reason}
}

func ErrCancelled() *Error {
	return &Error{code: ErrCodeCancelled, message: "operation cancelled"}
}

func ErrIOFailed(reason string) *Error {
	return &Error{code: ErrCodeIOFailed, message: "io failed: " + reason}
}
