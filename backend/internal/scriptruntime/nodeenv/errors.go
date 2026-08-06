// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/pkg/platform"
)

var (
	ErrScriptRuntimeDisabled     = errors.New("nodeenv: script runtime provider disabled")
	ErrNodeProviderNotSelected   = errors.New("nodeenv: node process provider not selected")
	ErrHostCapabilityUnsupported = errors.New("nodeenv: host capability unsupported")
	ErrUnsupportedGuestPlatform  = errors.New("nodeenv: unsupported guest platform for node")
	ErrRuntimeRootUnavailable    = errors.New("nodeenv: runtime root unavailable")
	ErrNodeNotFound              = errors.New("nodeenv: node binary not found")
	ErrInvalidNodeBinary         = errors.New("nodeenv: invalid node binary")
	ErrNodeNotExecutable         = errors.New("nodeenv: node binary not executable")
	ErrInvalidPackageManagerCLI  = errors.New("nodeenv: invalid package manager CLI")
	ErrShellWrapperUnsupported   = errors.New("nodeenv: shell wrapper not supported as package manager entry")
	ErrInvalidWorkDir            = errors.New("nodeenv: invalid work directory")
	ErrNativeResourceNotAllowed  = errors.New("nodeenv: native resource cannot be used as node path")
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

type nodeNotFoundError struct {
	source Source
}

func (e *nodeNotFoundError) Error() string {
	return fmt.Sprintf("%s: source=%s", ErrNodeNotFound.Error(), string(e.source))
}

func (e *nodeNotFoundError) Is(target error) bool {
	return target == ErrNodeNotFound
}

type invalidNodeBinaryError struct {
	path   string
	reason string
}

func (e *invalidNodeBinaryError) Error() string {
	return fmt.Sprintf("%s: path=%s reason=%s", ErrInvalidNodeBinary.Error(), e.path, e.reason)
}

func (e *invalidNodeBinaryError) Is(target error) bool {
	return target == ErrInvalidNodeBinary
}

type nodeNotExecutableError struct {
	path string
}

func (e *nodeNotExecutableError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrNodeNotExecutable.Error(), e.path)
}

func (e *nodeNotExecutableError) Is(target error) bool {
	return target == ErrNodeNotExecutable
}

type runtimePathError struct {
	reason string
}

func (e *runtimePathError) Error() string {
	return fmt.Sprintf("%s: %s", ErrRuntimeRootUnavailable.Error(), strings.ToLower(e.reason))
}

func (e *runtimePathError) Is(target error) bool {
	return target == ErrRuntimeRootUnavailable
}

type shellWrapperError struct {
	path string
}

func (e *shellWrapperError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrShellWrapperUnsupported.Error(), e.path)
}

func (e *shellWrapperError) Is(target error) bool {
	return target == ErrShellWrapperUnsupported
}

type nativeResourceError struct {
	path string
}

func (e *nativeResourceError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrNativeResourceNotAllowed.Error(), e.path)
}

func (e *nativeResourceError) Is(target error) bool {
	return target == ErrNativeResourceNotAllowed
}

func newUnsupportedGuest(guest platform.GuestPlatform) error {
	return &unsupportedGuestError{guest: guest}
}

func newHostCapabilityError(capability string) error {
	return &hostCapabilityError{capability: capability}
}

func newNodeNotFound(source Source) error {
	return &nodeNotFoundError{source: source}
}

func newInvalidNodeBinary(path, reason string) error {
	return &invalidNodeBinaryError{path: path, reason: reason}
}

func newNodeNotExecutable(path string) error {
	return &nodeNotExecutableError{path: path}
}

func newRuntimePathError(reason string) error {
	return &runtimePathError{reason: reason}
}

func newShellWrapper(path string) error {
	return &shellWrapperError{path: path}
}

func newNativeResource(path string) error {
	return &nativeResourceError{path: path}
}
