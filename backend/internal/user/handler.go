// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package user

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func (h *Handler) Status(c *gin.Context) {
	hasAdmin, err := h.service.HasAdmin()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "检查状态失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"hasAdmin": hasAdmin})
}

func (h *Handler) Setup(c *gin.Context) {
	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "用户名和密码不能为空", nil)
		return
	}
	if len(req.Password) < 6 {
		util.ErrorResponse(c, response.InvalidParams, "密码至少 6 位", nil)
		return
	}

	resp, err := h.service.Setup(req.Username, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "管理员注册成功", resp)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "用户名和密码不能为空", nil)
		return
	}

	resp, _, err := h.service.Login(req.Username, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "登录成功", resp)
}

func (h *Handler) Me(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	resp, err := h.service.GetMe(token)
	if err != nil {
		util.ErrorResponse(c, response.InvalidToken, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	if err := h.service.Logout(token); err != nil {
		util.ErrorResponse(c, response.InternalError, "登出失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "已登出", nil)
}

func (h *Handler) ListSessions(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	sessions, err := h.service.ListSessions(token)
	if err != nil {
		util.ErrorResponse(c, response.InvalidToken, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, sessions)
}

func (h *Handler) RevokeSession(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	id := parseID(c.Param("id"))
	if id == 0 {
		util.ErrorResponse(c, response.InvalidParams, "无效的会话 ID", nil)
		return
	}

	if err := h.service.RevokeSession(token, id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, nil)
}

func (h *Handler) RevokeAllSessions(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	if err := h.service.RevokeAllSessions(token); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, nil)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		util.ErrorResponse(c, response.Unauthorized, "请先登录", nil)
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请输入新旧密码", nil)
		return
	}
	if len(req.NewPassword) < 6 {
		util.ErrorResponse(c, response.InvalidParams, "新密码至少 6 位", nil)
		return
	}

	if err := h.service.ChangePassword(token, req.OldPassword, req.NewPassword); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "密码修改成功", nil)
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

func parseID(s string) int {
	id := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		id = id*10 + int(ch-'0')
	}
	return id
}
