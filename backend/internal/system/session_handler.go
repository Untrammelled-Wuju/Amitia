// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/util"
)

func sessionActor(c *gin.Context) (int64, string) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil {
		return 0, ""
	}
	var userID int64
	fmt.Sscanf(string(actor.UserID), "%d", &userID)
	return userID, actor.SessionID
}
func (h *Handler) CurrentSession(c *gin.Context) {
	userID, sessionID := sessionActor(c)
	util.SuccessResponse(c, h.service.GetCurrentSession(userID, sessionID))
}
func (h *Handler) LoginHistory(c *gin.Context) {
	userID, _ := sessionActor(c)
	util.SuccessResponse(c, h.service.GetLoginHistory(userID))
}
func (h *Handler) RecoveryCodesStatus(c *gin.Context) {
	userID, _ := sessionActor(c)
	util.SuccessResponse(c, h.service.GetRecoveryCodesStatus(userID))
}
func (h *Handler) GenerateRecoveryCodes(c *gin.Context) {
	userID, _ := sessionActor(c)
	util.SuccessResponse(c, h.service.GenerateRecoveryCodes(userID))
}
func (h *Handler) VerifyRecoveryCode(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&body)
	userID, _ := sessionActor(c)
	util.SuccessResponse(c, h.service.VerifyRecoveryCode(userID, body.Code))
}
func (h *Handler) SessionSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSessionSettings())
}
func (h *Handler) UpdateSessionSettings(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateSessionSettings(body))
}
