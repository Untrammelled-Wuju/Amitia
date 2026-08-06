// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"path/filepath"
)

func validateArtifact(artifact Artifact) error {
	if !knownKind(artifact.Kind) {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if artifact.EntryPath == "" {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if !filepath.IsAbs(artifact.EntryPath) {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if !isJSFile(artifact.EntryPath) {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if artifact.WorkingDir == "" {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if !filepath.IsAbs(artifact.WorkingDir) {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	if !knownSource(artifact.Source) {
		return newSidecarError(artifact.Kind, artifact.Source, ErrSidecarArtifactInvalid)
	}
	return nil
}
