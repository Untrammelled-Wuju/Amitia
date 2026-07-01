// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) Version(c *gin.Context) { util.SuccessResponse(c, h.service.GetVersion()) }

func (h *Handler) About(c *gin.Context) { util.SuccessResponse(c, h.service.GetAbout()) }
