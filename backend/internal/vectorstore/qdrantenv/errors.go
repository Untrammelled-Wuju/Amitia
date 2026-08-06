// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"errors"
	"fmt"

	"github.com/u-ai/backend/pkg/platform"
)

var (
	ErrVectorStoreDisabled      = errors.New("qdrantenv: vector store provider disabled")
	ErrQdrantProviderNotSelected = errors.New("qdrantenv: qdrant process provider not selected")
	ErrHostCapabilityUnsupported = errors.New("qdrantenv: host capability unsupported")
	ErrUnsupportedGuestPlatform  = errors.New("qdrantenv: unsupported guest platform for qdrant")
	ErrRuntimeRootUnavailable    = errors.New("qdrantenv: runtime root unavailable")
	ErrExplicitBinaryNotFound    = errors.New("qdrantenv: explicit qdrant binary not found")
	ErrQdrantBinaryNotInstalled  = errors.New("qdrantenv: qdrant binary not installed")
	ErrInvalidQdrantBinary       = errors.New("qdrantenv: invalid qdrant binary")
	ErrQdrantBinaryNotExecutable = errors.New("qdrantenv: qdrant binary not executable")
)

type unsupportedGuestError struct {
	guest platform.GuestPlatform
}

func (e *unsupportedGuestError) Error() string {
	return fmt.Sprintf("%s: guest=%s", ErrUnsupportedGuestPlatform.Error(), string(e.guest))
}

func (e *unsupportedGuestError) Is(target error) bool {
	return target == ErrUnsupportedGuestPlatform
}

type hostCapabilityError struct {
	capability string
}

func (e *hostCapabilityError) Error() string {
	return fmt.Sprintf("%s: capability=%s", ErrHostCapabilityUnsupported.Error(), e.capability)
}

func (e *hostCapabilityError) Is(target error) bool {
	return target == ErrHostCapabilityUnsupported
}

type runtimeRootUnavailableError struct {
	reason string
}

func (e *runtimeRootUnavailableError) Error() string {
	return fmt.Sprintf("%s: %s", ErrRuntimeRootUnavailable.Error(), e.reason)
}

func (e *runtimeRootUnavailableError) Is(target error) bool {
	return target == ErrRuntimeRootUnavailable
}

type explicitBinaryNotFoundError struct {
	path string
}

func (e *explicitBinaryNotFoundError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrExplicitBinaryNotFound.Error(), e.path)
}

func (e *explicitBinaryNotFoundError) Is(target error) bool {
	return target == ErrExplicitBinaryNotFound
}

type qdrantBinaryNotInstalledError struct {
	source Source
}

func (e *qdrantBinaryNotInstalledError) Error() string {
	return fmt.Sprintf("%s: source=%s", ErrQdrantBinaryNotInstalled.Error(), string(e.source))
}

func (e *qdrantBinaryNotInstalledError) Is(target error) bool {
	return target == ErrQdrantBinaryNotInstalled
}

type invalidQdrantBinaryError struct {
	path   string
	reason string
}

func (e *invalidQdrantBinaryError) Error() string {
	return fmt.Sprintf("%s: path=%s reason=%s", ErrInvalidQdrantBinary.Error(), e.path, e.reason)
}

func (e *invalidQdrantBinaryError) Is(target error) bool {
	return target == ErrInvalidQdrantBinary
}

type qdrantBinaryNotExecutableError struct {
	path string
}

func (e *qdrantBinaryNotExecutableError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrQdrantBinaryNotExecutable.Error(), e.path)
}

func (e *qdrantBinaryNotExecutableError) Is(target error) bool {
	return target == ErrQdrantBinaryNotExecutable
}

func newUnsupportedGuest(guest platform.GuestPlatform) error {
	return &unsupportedGuestError{guest: guest}
}

func newHostCapabilityError(capability string) error {
	return &hostCapabilityError{capability: capability}
}

func newRuntimeRootUnavailable(reason string) error {
	return &runtimeRootUnavailableError{reason: reason}
}

func newExplicitBinaryNotFound(path string) error {
	return &explicitBinaryNotFoundError{path: path}
}

func newQdrantBinaryNotInstalled(source Source) error {
	return &qdrantBinaryNotInstalledError{source: source}
}

func newInvalidQdrantBinary(path, reason string) error {
	return &invalidQdrantBinaryError{path: path, reason: reason}
}

func newQdrantBinaryNotExecutable(path string) error {
	return &qdrantBinaryNotExecutableError{path: path}
}
