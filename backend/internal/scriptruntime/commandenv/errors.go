// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCommandRequired          = errors.New("commandenv: command is required")
	ErrCommandInvalid           = errors.New("commandenv: command contains invalid characters")
	ErrCommandNotFound          = errors.New("commandenv: command not found")
	ErrShellCommandForbidden    = errors.New("commandenv: shell command is forbidden")
	ErrNodeEnvironmentUnavailable = errors.New("commandenv: managed node environment unavailable")
	ErrNodeCLIUnavailable       = errors.New("commandenv: node cli unavailable")
	ErrUnmanagedNodeCommand     = errors.New("commandenv: unmanaged node command rejected")
	ErrExecutableInvalid        = errors.New("commandenv: executable path invalid")
)

type commandError struct {
	kind    Kind
	command string
	reason  string
}

func (e *commandError) Error() string {
	var b strings.Builder
	b.WriteString("commandenv: ")
	b.WriteString(e.reason)
	if e.kind != "" {
		b.WriteString(" kind=")
		b.WriteString(string(e.kind))
	}
	if e.command != "" {
		b.WriteString(" command=")
		b.WriteString(e.command)
	}
	return b.String()
}

func (e *commandError) Is(target error) bool {
	switch e.reason {
	case ErrCommandRequired.Error():
		return target == ErrCommandRequired
	case ErrCommandInvalid.Error():
		return target == ErrCommandInvalid
	case ErrCommandNotFound.Error():
		return target == ErrCommandNotFound
	case ErrShellCommandForbidden.Error():
		return target == ErrShellCommandForbidden
	case ErrNodeEnvironmentUnavailable.Error():
		return target == ErrNodeEnvironmentUnavailable
	case ErrNodeCLIUnavailable.Error():
		return target == ErrNodeCLIUnavailable
	case ErrUnmanagedNodeCommand.Error():
		return target == ErrUnmanagedNodeCommand
	case ErrExecutableInvalid.Error():
		return target == ErrExecutableInvalid
	}
	return false
}

func newCommandError(reason error, kind Kind, command string) error {
	return &commandError{kind: kind, command: command, reason: reason.Error()}
}

type wrappedCommandError struct {
	inner  error
	kind   Kind
	command string
}

func (e *wrappedCommandError) Error() string {
	return fmt.Sprintf("%s: kind=%s command=%s cause=%v", ErrNodeEnvironmentUnavailable.Error(), string(e.kind), e.command, e.inner)
}

func (e *wrappedCommandError) Is(target error) bool {
	return target == ErrNodeEnvironmentUnavailable
}

func (e *wrappedCommandError) Unwrap() error {
	return e.inner
}

func wrapNodeError(err error, kind Kind, command string) error {
	return &wrappedCommandError{inner: err, kind: kind, command: command}
}
