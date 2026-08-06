// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"context"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

func UnavailableNodeResolver() NodeEnvironmentResolver {
	return &unavailableNodeResolver{}
}

func UnavailableArtifactResolver() ArtifactResolver {
	return &unavailableArtifactResolver{}
}

type unavailableNodeResolver struct{}

func (r *unavailableNodeResolver) Resolve(_ context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{}, ErrNodeResolverUnavailable
}

type unavailableArtifactResolver struct{}

func (r *unavailableArtifactResolver) Resolve(_ context.Context, kind Kind) (Artifact, error) {
	return Artifact{Kind: kind}, ErrArtifactResolverUnavailable
}
