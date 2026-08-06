// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidDocument  = errors.New("qdrantconfig: invalid document")
	ErrRenderFailed     = errors.New("qdrantconfig: render failed")
	ErrWriteFailed      = errors.New("qdrantconfig: write failed")
	ErrUnsafePathInYAML = errors.New("qdrantconfig: unsafe path in YAML output")
)

type invalidDocumentError struct {
	reason string
}

func (e *invalidDocumentError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidDocument.Error(), e.reason)
}

func (e *invalidDocumentError) Is(target error) bool {
	return target == ErrInvalidDocument
}

func newInvalidDocument(reason string) error {
	return &invalidDocumentError{reason: reason}
}

type renderFailedError struct {
	err error
}

func (e *renderFailedError) Error() string {
	return fmt.Sprintf("%s: %v", ErrRenderFailed.Error(), e.err)
}

func (e *renderFailedError) Is(target error) bool {
	return target == ErrRenderFailed
}

func (e *renderFailedError) Unwrap() error {
	return e.err
}

func newRenderFailed(err error) error {
	return &renderFailedError{err: err}
}

type writeFailedError struct {
	path string
	err  error
}

func (e *writeFailedError) Error() string {
	return fmt.Sprintf("%s: path=%s: %v", ErrWriteFailed.Error(), e.path, e.err)
}

func (e *writeFailedError) Is(target error) bool {
	return target == ErrWriteFailed
}

func (e *writeFailedError) Unwrap() error {
	return e.err
}

func newWriteFailed(path string, err error) error {
	return &writeFailedError{path: path, err: err}
}
