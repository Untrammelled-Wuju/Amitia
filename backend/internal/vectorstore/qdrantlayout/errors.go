// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"errors"
	"fmt"
)

var (
	ErrQdrantNotInstalled      = errors.New("qdrantlayout: qdrant is not installed")
	ErrResourceRootUnavailable = errors.New("qdrantlayout: resource root is unavailable")
	ErrInvalidLayout           = errors.New("qdrantlayout: invalid layout")
	ErrPathOverlap             = errors.New("qdrantlayout: path overlap detected")
	ErrUnsafeRootPath          = errors.New("qdrantlayout: path must not be filesystem root")
	ErrDirectoryCreationFailed = errors.New("qdrantlayout: directory creation failed")
	ErrConfigRenderFailed      = errors.New("qdrantlayout: config render failed")
	ErrConfigWriteFailed       = errors.New("qdrantlayout: config write failed")
)

type resourceRootUnavailableError struct {
	resource string
	reason   string
}

func (e *resourceRootUnavailableError) Error() string {
	return fmt.Sprintf("%s: resource=%s reason=%s", ErrResourceRootUnavailable.Error(), e.resource, e.reason)
}

func (e *resourceRootUnavailableError) Is(target error) bool {
	return target == ErrResourceRootUnavailable
}

func newResourceRootUnavailable(resource, reason string) error {
	return &resourceRootUnavailableError{resource: resource, reason: reason}
}

type invalidLayoutError struct {
	reason string
}

func (e *invalidLayoutError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidLayout.Error(), e.reason)
}

func (e *invalidLayoutError) Is(target error) bool {
	return target == ErrInvalidLayout
}

func newInvalidLayout(reason string) error {
	return &invalidLayoutError{reason: reason}
}

type pathOverlapError struct {
	pathA string
	pathB string
}

func (e *pathOverlapError) Error() string {
	return fmt.Sprintf("%s: pathA=%s pathB=%s", ErrPathOverlap.Error(), e.pathA, e.pathB)
}

func (e *pathOverlapError) Is(target error) bool {
	return target == ErrPathOverlap
}

func newPathOverlap(pathA, pathB string) error {
	return &pathOverlapError{pathA: pathA, pathB: pathB}
}

type unsafeRootError struct {
	path string
}

func (e *unsafeRootError) Error() string {
	return fmt.Sprintf("%s: path=%s", ErrUnsafeRootPath.Error(), e.path)
}

func (e *unsafeRootError) Is(target error) bool {
	return target == ErrUnsafeRootPath
}

func newUnsafeRoot(path string) error {
	return &unsafeRootError{path: path}
}

type directoryCreationError struct {
	path string
	err  error
}

func (e *directoryCreationError) Error() string {
	return fmt.Sprintf("%s: path=%s: %v", ErrDirectoryCreationFailed.Error(), e.path, e.err)
}

func (e *directoryCreationError) Is(target error) bool {
	return target == ErrDirectoryCreationFailed
}

func (e *directoryCreationError) Unwrap() error {
	return e.err
}

func newDirectoryCreation(path string, err error) error {
	return &directoryCreationError{path: path, err: err}
}
