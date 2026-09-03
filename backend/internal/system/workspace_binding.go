package system

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type conversationWorkspaceBinding struct {
	ConversationID string    `json:"conversationId" gorm:"column:conversation_id"`
	WorkspaceID    string    `json:"workspaceId" gorm:"column:workspace_id"`
	DeviceID       string    `json:"deviceId,omitempty" gorm:"column:device_id"`
	WorkspaceName  string    `json:"workspaceName,omitempty" gorm:"column:workspace_name"`
	WorkspaceKind  string    `json:"workspaceKind" gorm:"column:workspace_kind"`
	RootURI        string    `json:"rootUri" gorm:"column:root_uri"`
	UpdatedAt      time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func normalizeConversationWorkspaceBinding(binding conversationWorkspaceBinding) (conversationWorkspaceBinding, error) {
	binding.ConversationID = strings.TrimSpace(binding.ConversationID)
	binding.WorkspaceID = strings.TrimSpace(binding.WorkspaceID)
	binding.DeviceID = strings.TrimSpace(binding.DeviceID)
	binding.WorkspaceName = strings.TrimSpace(binding.WorkspaceName)
	binding.WorkspaceKind = strings.TrimSpace(binding.WorkspaceKind)
	binding.RootURI = strings.TrimSpace(binding.RootURI)
	if binding.ConversationID == "" {
		return binding, errors.New("conversationId is required")
	}
	if binding.WorkspaceID == "" {
		return binding, errors.New("workspaceId is required")
	}
	if binding.WorkspaceKind == "" {
		binding.WorkspaceKind = "local"
	}
	if binding.RootURI == "" {
		binding.RootURI = "amitia://workspace/@" + binding.WorkspaceID + "/"
	}
	expectedRoot := "amitia://workspace/@" + binding.WorkspaceID + "/"
	if binding.RootURI != expectedRoot {
		return binding, errors.New("rootUri does not match workspaceId")
	}
	binding.UpdatedAt = time.Now().UTC()
	return binding, nil
}

func (h *Handler) loadConversationWorkspaceBinding(conversationID string) (*conversationWorkspaceBinding, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	var binding conversationWorkspaceBinding
	err := h.db.Table("conversation_workspace_bindings").Where("conversation_id = ?", conversationID).Take(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (h *Handler) saveConversationWorkspaceBinding(binding conversationWorkspaceBinding) (*conversationWorkspaceBinding, error) {
	normalized, err := normalizeConversationWorkspaceBinding(binding)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := h.db.Table("conversations").Where("id = ?", normalized.ConversationID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err = h.db.Exec(`INSERT INTO conversation_workspace_bindings
		(conversation_id, workspace_id, device_id, workspace_name, workspace_kind, root_uri, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conversation_id) DO UPDATE SET
		workspace_id = excluded.workspace_id,
		device_id = excluded.device_id,
		workspace_name = excluded.workspace_name,
		workspace_kind = excluded.workspace_kind,
		root_uri = excluded.root_uri,
		updated_at = excluded.updated_at`,
		normalized.ConversationID,
		normalized.WorkspaceID,
		normalized.DeviceID,
		normalized.WorkspaceName,
		normalized.WorkspaceKind,
		normalized.RootURI,
		normalized.UpdatedAt,
	).Error
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (h *Handler) workspaceBindingForRequest(conversationID string, body webChatSendRequest) *conversationWorkspaceBinding {
	if strings.TrimSpace(body.WorkspaceID) != "" {
		binding := conversationWorkspaceBinding{
			ConversationID: conversationID,
			WorkspaceID:    body.WorkspaceID,
			DeviceID:       body.WorkspaceDeviceID,
			WorkspaceName:  body.WorkspaceName,
			WorkspaceKind:  body.WorkspaceKind,
			RootURI:        body.WorkspaceRootURI,
		}
		if saved, err := h.saveConversationWorkspaceBinding(binding); err == nil {
			return saved
		}
		if normalized, err := normalizeConversationWorkspaceBinding(binding); err == nil {
			return &normalized
		}
	}
	binding, err := h.loadConversationWorkspaceBinding(conversationID)
	if err != nil {
		return nil
	}
	return binding
}

func (h *Handler) persistConversationWorkspaceBinding(conversationID string, binding *conversationWorkspaceBinding) {
	if h == nil || binding == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	copyBinding := *binding
	copyBinding.ConversationID = strings.TrimSpace(conversationID)
	_, _ = h.saveConversationWorkspaceBinding(copyBinding)
}

func applyWorkspaceBinding(req *interaction.UnifiedEntryRequest, binding *conversationWorkspaceBinding) {
	if req == nil || binding == nil {
		return
	}
	req.WorkspaceID = binding.WorkspaceID
	req.WorkspaceDeviceID = binding.DeviceID
	req.WorkspaceName = binding.WorkspaceName
	req.WorkspaceKind = binding.WorkspaceKind
	req.WorkspaceRootURI = binding.RootURI
}

func (h *Handler) handleUnifiedEntryWithWorkspace(ctx context.Context, req *interaction.UnifiedEntryRequest, binding *conversationWorkspaceBinding) (*interaction.OrchestrationResult, error) {
	applyWorkspaceBinding(req, binding)
	return h.unifiedEntry.Handle(ctx, req)
}

func (h *Handler) WebChatGetWorkspace(c *gin.Context) {
	binding, err := h.loadConversationWorkspaceBinding(c.Param("id"))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取工作目录失败", nil)
		return
	}
	if binding == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": nil})
		return
	}
	util.SuccessResponse(c, binding)
}

func (h *Handler) WebChatSetWorkspace(c *gin.Context) {
	var body struct {
		WorkspaceID   string `json:"workspaceId" binding:"required"`
		DeviceID      string `json:"deviceId"`
		WorkspaceName string `json:"workspaceName"`
		WorkspaceKind string `json:"workspaceKind"`
		RootURI       string `json:"rootUri"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效工作目录", nil)
		return
	}
	binding, err := h.saveConversationWorkspaceBinding(conversationWorkspaceBinding{
		ConversationID: c.Param("id"),
		WorkspaceID:    body.WorkspaceID,
		DeviceID:       body.DeviceID,
		WorkspaceName:  body.WorkspaceName,
		WorkspaceKind:  body.WorkspaceKind,
		RootURI:        body.RootURI,
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, response.InvalidParams, "对话不存在", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, binding)
}

func (h *Handler) WebChatClearWorkspace(c *gin.Context) {
	if err := h.db.Exec("DELETE FROM conversation_workspace_bindings WHERE conversation_id = ?", strings.TrimSpace(c.Param("id"))).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "清除工作目录失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"cleared": true})
}
