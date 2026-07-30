// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet"
)

type LegacyFrameExecutor struct {
	w *Worker
}

func NewLegacyFrameExecutor(w *Worker) *LegacyFrameExecutor {
	return &LegacyFrameExecutor{w: w}
}

func (e *LegacyFrameExecutor) CanHandle(task *desktoppet.GenerationTask) bool {
	return task.GenerationPlanVersion == 0
}

func (e *LegacyFrameExecutor) Execute(ctx context.Context, task *desktoppet.GenerationTask, action *desktoppet.GenerationTaskAction) string {
	return e.w.runAction(ctx, task, action)
}
