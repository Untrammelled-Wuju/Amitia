// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"context"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

type unavailableNodeResolver struct{}

func (unavailableNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{}, ErrNodeEnvironmentUnavailable
}

func UnavailableNodeResolver() NodeEnvironmentResolver {
	return unavailableNodeResolver{}
}
