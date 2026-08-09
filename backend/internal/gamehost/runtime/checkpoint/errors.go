package checkpoint

import (
	"fmt"
)

type CheckpointError struct {
	Op   string
	Kind string
	ID   string
	Cause error
}

func (e *CheckpointError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("checkpoint: %s: %s [%s]: %v", e.Op, e.Kind, e.ID, e.Cause)
	}
	return fmt.Sprintf("checkpoint: %s: %s [%s]", e.Op, e.Kind, e.ID)
}

func (e *CheckpointError) Unwrap() error {
	return e.Cause
}

const (
	ErrInvalidSchema     = "invalid_schema"
	ErrRuntimeIDMismatch = "runtime_id_mismatch"
	ErrPluginIDMismatch  = "plugin_id_mismatch"
	ErrCorrupt           = "corrupt_checkpoint"
	ErrCorruptMetadata   = "corrupt_metadata"
	ErrTooLarge          = "too_large"
	ErrInvalidState      = "invalid_state"
	ErrStaleRevision     = "stale_revision"
	ErrOrphaned          = "orphaned_runtime"
	ErrNotFound          = "not_found"
	ErrUnsupportedSchema = "unsupported_schema"
	ErrInvalidService    = "invalid_service"
)

func newError(op, kind, id string, cause error) error {
	return &CheckpointError{Op: op, Kind: kind, ID: id, Cause: cause}
}

func errCorrupt(op, id string, cause error) error {
	return newError(op, ErrCorrupt, id, cause)
}

func errCorruptMetadata(op, id string, cause error) error {
	return newError(op, ErrCorruptMetadata, id, cause)
}

func errTooLarge(op, id string) error {
	return newError(op, ErrTooLarge, id, nil)
}

func errInvalidSchema(op, id string) error {
	return newError(op, ErrInvalidSchema, id, nil)
}

func errUnsupportedSchema(op, id string) error {
	return newError(op, ErrUnsupportedSchema, id, nil)
}

func errRuntimeIDMismatch(op, id string, cause error) error {
	return newError(op, ErrRuntimeIDMismatch, id, cause)
}

func errPluginIDMismatch(op, id string, cause error) error {
	return newError(op, ErrPluginIDMismatch, id, cause)
}

func errInvalidState(op, id string, cause error) error {
	return newError(op, ErrInvalidState, id, cause)
}

func errNotFound(op, id string) error {
	return newError(op, ErrNotFound, id, nil)
}
