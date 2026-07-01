// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) WechatBridgeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeStatus())
}

func (h *Handler) WechatBridgeStatusDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeStatusDetail())
}

func (h *Handler) WechatBridgeConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeConfig())
}

func (h *Handler) UpdateWechatBridgeConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateWechatBridgeConfig(body))
}

func (h *Handler) WechatBridgeEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeEvents())
}

func (h *Handler) WechatBridgeQRCode(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeQRCode())
}

func (h *Handler) WechatBridgeRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatBridgeRecover())
}

func (h *Handler) QQBridgeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeStatus())
}

func (h *Handler) QQBridgeStatusDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeStatusDetail())
}

func (h *Handler) QQBridgeConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeConfig())
}

func (h *Handler) QQBridgeEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeEvents())
}

func (h *Handler) QQBridgeRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.QQBridgeRecover())
}

func (h *Handler) WechatCloudCheckRun(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckRun())
}

func (h *Handler) WechatCloudCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheck())
}

func (h *Handler) WechatCloudCheckReport(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckReport())
}

func (h *Handler) WechatCloudCheckRiskSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckRiskSummary())
}

func (h *Handler) WechatLoginReconnect(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginReconnect())
}

func (h *Handler) WechatLoginRescan(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginRescan())
}

func (h *Handler) WechatLoginStart(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginStart())
}

func (h *Handler) WechatLoginWait(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginWait())
}

func (h *Handler) WechatStatus(c *gin.Context) { util.SuccessResponse(c, h.service.GetWechatStatus()) }

func (h *Handler) WechatEvents(c *gin.Context) { util.SuccessResponse(c, h.service.GetWechatEvents()) }

func (h *Handler) WechatReplyTimingRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatReplyTimingRecover())
}

func (h *Handler) WechatReplyTimingStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatReplyTimingStatus())
}

func (h *Handler) MaintenanceRestartQQBridge(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceRestartQQBridge())
}
