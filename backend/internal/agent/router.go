// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"github.com/u-ai/backend/internal/androiduiagent"
	"github.com/u-ai/backend/internal/browseragent"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAgentRuntimeRunners(ctx *app.AppContext, unifiedEntry *interaction.UnifiedEntry, toolFacade *extensionkernel.ToolFacade) {
	svc := NewService(ctx, unifiedEntry, toolFacade)
	if runner, ok := svc.(androiduiagent.Runner); ok {
		androiduiagent.SetRunner(runner)
	}
	if runner, ok := svc.(browseragent.Runner); ok {
		browseragent.SetRunner(runner)
	}

}
