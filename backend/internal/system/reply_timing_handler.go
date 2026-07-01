// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) ReplyTimingOverview(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReplyTimingOverview())
}

func (h *Handler) ReplyTimingBuffers(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReplyTimingBuffers())
}

func (h *Handler) ReplyTimingForce(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingForce())
}

func (h *Handler) ReplyTimingCancelBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingCancelBuffer(c.Param("id")))
}

func (h *Handler) ReplyTimingForceBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingForceBuffer(c.Param("id")))
}

func (h *Handler) ReplyTimingResumeBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingResumeBuffer(c.Param("id")))
}
