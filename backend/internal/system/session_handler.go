// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) CurrentSession(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetCurrentSession(c.GetHeader("Authorization")))
}

func (h *Handler) LoginHistory(c *gin.Context) { util.SuccessResponse(c, h.service.GetLoginHistory()) }

func (h *Handler) RecoveryCodesStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRecoveryCodesStatus())
}

func (h *Handler) GenerateRecoveryCodes(c *gin.Context) {
	util.SuccessResponse(c, h.service.GenerateRecoveryCodes())
}

func (h *Handler) VerifyRecoveryCode(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.VerifyRecoveryCode(body.Code))
}

func (h *Handler) SessionSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSessionSettings())
}

func (h *Handler) UpdateSessionSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateSessionSettings(body))
}
