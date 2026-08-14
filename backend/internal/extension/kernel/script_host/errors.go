// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import "errors"

var (
	ErrNodeResolverUnavailable     = errors.New("script_host: node environment resolver unavailable")
	ErrArtifactResolverUnavailable = errors.New("script_host: artifact resolver unavailable")
	ErrUnknownHostKind             = errors.New("script_host: unknown host kind")
	ErrHostArtifactNotFound        = errors.New("script_host: host artifact not found")
	ErrInvalidHostArtifact         = errors.New("script_host: invalid host artifact")
	ErrUnsupportedHostEntry        = errors.New("script_host: unsupported host entry type")
	ErrRuntimeResourceUnavailable  = errors.New("script_host: runtime resource unavailable")
	ErrWorkspaceUnavailable        = errors.New("script_host: workspace unavailable")
)

type unknownHostKindError struct {
	kind Kind
}

func (e *unknownHostKindError) Error() string {
	return "script_host: unknown host kind: " + string(e.kind)
}

func (e *unknownHostKindError) Is(target error) bool {
	return target == ErrUnknownHostKind
}

type hostArtifactNotFoundError struct {
	kind   Kind
	source Source
	path   string
}

func newHostArtifactNotFound(kind Kind, source Source, path string) *hostArtifactNotFoundError {
	return &hostArtifactNotFoundError{kind: kind, source: source, path: path}
}

func (e *hostArtifactNotFoundError) Error() string {
	return "script_host: host artifact not found: kind=" + string(e.kind) + " source=" + string(e.source) + " path=" + e.path
}

func (e *hostArtifactNotFoundError) Is(target error) bool {
	return target == ErrHostArtifactNotFound
}

type invalidHostArtifactError struct {
	kind   Kind
	path   string
	reason string
}

func newInvalidHostArtifact(kind Kind, path, reason string) *invalidHostArtifactError {
	return &invalidHostArtifactError{kind: kind, path: path, reason: reason}
}

func (e *invalidHostArtifactError) Error() string {
	return "script_host: invalid host artifact: kind=" + string(e.kind) + " path=" + e.path + " reason=" + e.reason
}

func (e *invalidHostArtifactError) Is(target error) bool {
	return target == ErrInvalidHostArtifact
}

type unsupportedHostEntryError struct {
	kind      Kind
	path      string
	extension string
}

func newUnsupportedHostEntry(kind Kind, path, ext string) *unsupportedHostEntryError {
	return &unsupportedHostEntryError{kind: kind, path: path, extension: ext}
}

func (e *unsupportedHostEntryError) Error() string {
	return "script_host: unsupported host entry: kind=" + string(e.kind) + " path=" + e.path + " extension=" + e.extension
}

func (e *unsupportedHostEntryError) Is(target error) bool {
	return target == ErrUnsupportedHostEntry
}

type runtimeResourceUnavailableError struct {
	kind Kind
	uri  string
}

func newRuntimeResourceUnavailable(kind Kind, uri string) *runtimeResourceUnavailableError {
	return &runtimeResourceUnavailableError{kind: kind, uri: uri}
}

func (e *runtimeResourceUnavailableError) Error() string {
	return "script_host: runtime resource unavailable: kind=" + string(e.kind) + " uri=" + e.uri
}

func (e *runtimeResourceUnavailableError) Is(target error) bool {
	return target == ErrRuntimeResourceUnavailable
}

type workspaceUnavailableError struct {
	kind Kind
}

func newWorkspaceUnavailable(kind Kind) *workspaceUnavailableError {
	return &workspaceUnavailableError{kind: kind}
}

func (e *workspaceUnavailableError) Error() string {
	return "script_host: workspace directory unavailable: kind=" + string(e.kind)
}

func (e *workspaceUnavailableError) Is(target error) bool {
	return target == ErrWorkspaceUnavailable
}
