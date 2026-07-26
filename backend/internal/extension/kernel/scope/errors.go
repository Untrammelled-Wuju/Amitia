package scope

import "errors"

var (
	ErrBindingNotFound  = errors.New("scope binding not found")
	ErrSnapshotNotFound = errors.New("scope snapshot not found")
)
