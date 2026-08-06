// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import "context"

type unavailableArtifactResolver struct{}

func (unavailableArtifactResolver) Resolve(context.Context, Kind) (Artifact, error) {
	return Artifact{}, ErrWorkspaceUnavailable
}

func UnavailableArtifactResolver() ArtifactResolver {
	return unavailableArtifactResolver{}
}
