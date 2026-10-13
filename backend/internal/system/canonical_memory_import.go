package system

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"strings"
)

func WithCanonicalMemory(engine *business.Engine, coreID string) SystemRouterOption {
	return func(handler *Handler) { handler.canonicalMemory, handler.memoryCoreID = engine, coreID }
}

func (h *Handler) importMemoryRequest(c *gin.Context) (business.Request, error) {
	actor := security.GetActor(c)
	if h.canonicalMemory == nil || actor == nil || actor.DeviceID == "" {
		return business.Request{}, errors.New("当前记忆服务或设备身份尚未就绪")
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := h.requireWebChatImportConversation(id, actor.SpaceID.String()); err != nil {
		return business.Request{}, errors.New("导入批次不存在")
	}
	var roles []string
	if err := h.db.Table("messages").Where("conversation_id=? AND character_id<>''", id).Distinct("character_id").Pluck("character_id", &roles).Error; err != nil {
		return business.Request{}, err
	}
	if len(roles) != 1 {
		return business.Request{}, errors.New("导入记录必须明确属于一个角色")
	}
	return business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: h.memoryCoreID, RoleID: roles[0], ConversationID: id, RequestID: uuid.NewString()}, nil
}

func candidateRows(result business.MemoryManagementResponse, conversation string) []map[string]any {
	rows := []map[string]any{}
	for _, resource := range result.Resources {
		var candidate business.OwnedMemoryCandidate
		if json.Unmarshal(resource.Body, &candidate) != nil || candidate.ConversationID != conversation || candidate.State != "pending" {
			continue
		}
		rows = append(rows, map[string]any{"id": resource.ID, "key": candidate.Key, "value": candidate.Value, "memoryType": candidate.MemoryType, "importance": candidate.Importance, "revision": resource.Revision})
	}
	return rows
}

func (h *Handler) canonicalImportCandidates(c *gin.Context, generate bool) {
	request, err := h.importMemoryRequest(c)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "读取导入记忆失败", err.Error())
		return
	}
	result, err := h.canonicalMemory.ListOwnedMemoryCandidates(c.Request.Context(), request, business.MemoryManagementQuery{Limit: 128})
	if err == nil && generate {
		request.ExpectedScope = &result.Scope
		result, err = h.canonicalMemory.ManageOwnedMemoryCandidates(c.Request.Context(), request, business.MemoryCandidateInput{Action: "generate", ConversationID: request.ConversationID})
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "提取导入记忆失败", err.Error())
		return
	}
	util.SuccessResponse(c, map[string]any{"candidates": candidateRows(result, request.ConversationID), "conversationId": request.ConversationID, "requestId": request.RequestID})
}

func (h *Handler) canonicalConfirmImportMemories(c *gin.Context) {
	request, err := h.importMemoryRequest(c)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "确认导入记忆失败", err.Error())
		return
	}
	var input struct {
		SelectedIDs []string `json:"selectedIds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "确认参数无效", err.Error())
		return
	}
	result, err := h.canonicalMemory.ListOwnedMemoryCandidates(c.Request.Context(), request, business.MemoryManagementQuery{Limit: 128})
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "读取候选记忆失败", err.Error())
		return
	}
	selected := map[string]bool{}
	for _, id := range input.SelectedIDs {
		selected[id] = true
	}
	eligible := map[string]bool{}
	for _, row := range candidateRows(result, request.ConversationID) {
		eligible[row["id"].(string)] = true
	}
	for id := range selected {
		if !eligible[id] {
			util.ErrorResponse(c, response.OperationFailed, "所选候选记忆不属于当前导入批次", nil)
			return
		}
	}
	if len(eligible) == 0 {
		util.ErrorResponse(c, response.OperationFailed, "没有待审核候选，请先提取记忆", nil)
		return
	}
	confirmed := 0
	for _, resource := range result.Resources {
		var candidate business.OwnedMemoryCandidate
		if json.Unmarshal(resource.Body, &candidate) != nil || candidate.ConversationID != request.ConversationID || len(selected) > 0 && !selected[resource.ID] {
			continue
		}
		request.ExpectedScope, request.RequestID = &result.Scope, uuid.NewString()
		if _, err := h.canonicalMemory.ManageOwnedMemoryCandidates(c.Request.Context(), request, business.MemoryCandidateInput{Action: "accept", ID: resource.ID, ExpectedRevision: resource.Revision}); err != nil {
			util.ErrorResponse(c, response.OperationFailed, "候选记忆尚未全部确认", err.Error())
			return
		}
		confirmed++
	}
	util.SuccessResponse(c, map[string]any{"confirmed": true, "memoriesCreated": confirmed})
}
