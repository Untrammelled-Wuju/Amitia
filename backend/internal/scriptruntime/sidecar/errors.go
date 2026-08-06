// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownSidecarKind      = errors.New("sidecar: unknown sidecar kind")
	ErrSidecarArtifactNotFound = errors.New("sidecar: artifact not found")
	ErrSidecarArtifactInvalid  = errors.New("sidecar: artifact invalid")
	ErrSidecarBundleIncomplete = errors.New("sidecar: runtime bundle incomplete")
	ErrSidecarSourceIncomplete = errors.New("sidecar: workspace source incomplete")
	ErrWorkspaceUnavailable    = errors.New("sidecar: workspace unavailable")
)

type sidecarResolveError struct {
	kind Kind
	source Source
	inner error
}

func (e *sidecarResolveError) Error() string {
	return fmt.Sprintf("sidecar: kind=%s source=%s: %v", string(e.kind), string(e.source), e.inner)
}

func (e *sidecarResolveError) Unwrap() error {
	return e.inner
}

func (e *sidecarResolveError) Is(target error) bool {
	return errors.Is(e.inner, target)
}

func newSidecarError(kind Kind, source Source, inner error) error {
	return &sidecarResolveError{kind: kind, source: source, inner: inner}
}
