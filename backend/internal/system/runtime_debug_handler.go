// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeobservability"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) RuntimeDebugSnapshot(c *gin.Context) {
	util.SuccessResponse(c, runtimeobservability.Snapshot(c.Request.Context(), h.db, h.reconciliation))
}
