// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct{ service Service }
type scopedEpisodicService interface {
	ListForSpace(EpisodicListQuery, string) (*EpisodicListResponse, error)
	CreateForSpace(*CreateEpisodicRequest, string) (*EpisodicMemory, error)
	DeleteForSpace(string, string) error
	UpdateRetentionForSpace(string, string, int) (*EpisodicMemory, error)
	RestoreForSpace(string, string) (*EpisodicMemory, error)
	GetForSpace(string, string) ([]EpisodicMemory, error)
	GetDetailForSpace(string, string) (*EpisodicMemory, []map[string]interface{}, error)
	ExtractForSpace(string, string, []map[string]string, string) error
	SystemPromptForSpace(string, string) string
}

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }
func (h *Handler) scoped(c *gin.Context) (scopedEpisodicService, string, bool) {
	s, ok := h.service.(scopedEpisodicService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "episodic service does not provide user-scoped operations", nil)
		return nil, "", false
	}
	return s, requestidentity.ResolveGin(c), true
}
func (h *Handler) List(c *gin.Context) {
	var q EpisodicListQuery
	_ = c.ShouldBindQuery(&q)
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	r, e := s.ListForSpace(q, u)
	if e != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, r)
}
func (h *Handler) Create(c *gin.Context) {
	var q CreateEpisodicRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少必要参数", nil)
		return
	}
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	m, e := s.CreateForSpace(&q, u)
	if e != nil {
		util.ErrorResponse(c, response.InternalError, e.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "创建成功", m)
}
func (h *Handler) Delete(c *gin.Context) {
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	if e := s.DeleteForSpace(c.Param("id"), u); e != nil {
		util.ErrorResponse(c, response.OperationFailed, e.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "删除成功", nil)
}
func (h *Handler) UpdateRetention(c *gin.Context) {
	var b struct {
		RetentionLevel int `json:"retentionLevel"`
	}
	if e := c.ShouldBindJSON(&b); e != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	m, e := s.UpdateRetentionForSpace(c.Param("id"), u, b.RetentionLevel)
	if e != nil {
		util.ErrorResponse(c, response.OperationFailed, e.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆层级已更新", m)
}
func (h *Handler) Restore(c *gin.Context) {
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	m, e := s.RestoreForSpace(c.Param("id"), u)
	if e != nil {
		util.ErrorResponse(c, response.OperationFailed, e.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "情景记忆已恢复", m)
}
func (h *Handler) GetBySpaceID(c *gin.Context) {
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	m, e := s.GetForSpace(u, c.Query("characterId"))
	if e != nil {
		util.ErrorResponse(c, response.InternalError, e.Error(), nil)
		return
	}
	util.SuccessResponse(c, m)
}
func (h *Handler) GetDetail(c *gin.Context) {
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	m, msg, e := s.GetDetailForSpace(c.Param("id"), u)
	if e != nil {
		util.ErrorResponse(c, response.OperationFailed, e.Error(), nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"memory": m, "messages": msg})
}
func (h *Handler) Extract(c *gin.Context) {
	var b struct {
		CharacterID    string              `json:"characterId"`
		ConversationID string              `json:"conversationId"`
		Messages       []map[string]string `json:"messages"`
	}
	if e := c.ShouldBindJSON(&b); e != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	if e := s.ExtractForSpace(u, b.ConversationID, b.Messages, b.CharacterID); e != nil {
		util.ErrorResponse(c, response.InternalError, e.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "情景检测完成", nil)
}
func (h *Handler) SystemPrompt(c *gin.Context) {
	s, u, ok := h.scoped(c)
	if !ok {
		return
	}
	util.SuccessResponse(c, map[string]string{"prompt": s.SystemPromptForSpace(u, c.Query("characterId"))})
}
