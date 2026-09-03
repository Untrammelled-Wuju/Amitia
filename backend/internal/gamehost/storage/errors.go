package storage

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DirectoryError struct {
	Op      string
	Kind    string
	Path    string
	Cause   error
}

func (e *DirectoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("storage: %s: %s [%s]: %v", e.Op, e.Kind, e.Path, e.Cause)
	}
	return fmt.Sprintf("storage: %s: %s [%s]", e.Op, e.Kind, e.Path)
}

func (e *DirectoryError) Unwrap() error {
	return e.Cause
}

func newInvalidPathError(op, path string, cause error) error {
	return &DirectoryError{Op: op, Kind: "invalid_path", Path: path, Cause: cause}
}

func newPathEscapeError(op, path string) error {
	return &DirectoryError{Op: op, Kind: "path_escape", Path: path}
}

func newCreateFailedError(op, path string, cause error) error {
	return domain.NewHostErrorWithCause(
		domain.ErrInternal,
		fmt.Sprintf("create directory failed: %s", path),
		cause,
	)
}

func newRemoveFailedError(op, path string, cause error) error {
	return domain.NewHostErrorWithCause(
		domain.ErrInternal,
		fmt.Sprintf("remove directory failed: %s", path),
		cause,
	)
}

func newNotFoundError(op, path string) error {
	return domain.NewHostError(domain.ErrNotFound, fmt.Sprintf("directory not found: %s", path))
}
