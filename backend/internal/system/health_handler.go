// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) Health(c *gin.Context) { util.SuccessResponse(c, h.service.Health()) }

func (h *Handler) Diagnostics(c *gin.Context) { util.SuccessResponse(c, h.service.Diagnostics()) }

func (h *Handler) RunDiagnostics(c *gin.Context) { util.SuccessResponse(c, h.service.RunDiagnostics()) }
