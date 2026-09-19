package system

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

type conversationWorkspaceBinding struct {
	ConversationID string    `json:"conversationId"`
	WorkspaceID    string    `json:"workspaceId"`
	DeviceID       string    `json:"deviceId,omitempty"`
	WorkspaceName  string    `json:"workspaceName,omitempty"`
	WorkspaceKind  string    `json:"workspaceKind"`
	RootURI        string    `json:"rootUri"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (h *Handler) workspaceBindingForRequest(conversationID string, body webChatSendRequest, spaceID string) *conversationWorkspaceBinding {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		if strings.TrimSpace(body.ProjectID) == "" {
			return nil
		}
		return h.workspaceBindingForProject(body.ProjectID, conversationID)
	}
	var conversation chat.Conversation
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", conversationID).First(&conversation).Error; err != nil {
		if strings.TrimSpace(body.ProjectID) == "" {
			return nil
		}
		return h.workspaceBindingForProject(body.ProjectID, conversationID)
	}
	if strings.TrimSpace(conversation.ProjectID) == "" {
		return nil
	}
	return h.workspaceBindingForProject(conversation.ProjectID, conversation.ID)
}

func (h *Handler) workspaceBindingForProject(projectID, conversationID string) *conversationWorkspaceBinding {
	var project chat.Project
	if err := h.db.Where("id = ?", strings.TrimSpace(projectID)).First(&project).Error; err != nil {
		return nil
	}
	var mount struct {
		Name string
		Kind string
	}
	if err := h.db.Table("workspace_mounts").Select("name", "kind").Where("id = ?", project.WorkspaceID).Take(&mount).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return nil
	}
	return &conversationWorkspaceBinding{
		ConversationID: conversationID,
		WorkspaceID:    project.WorkspaceID,
		DeviceID:       project.DeviceID,
		WorkspaceName:  mount.Name,
		WorkspaceKind:  mount.Kind,
		RootURI:        project.RootURI,
		UpdatedAt:      time.Now().UTC(),
	}
}

func (h *Handler) handleUnifiedEntryWithWorkspace(ctx context.Context, req *interaction.UnifiedEntryRequest, binding *conversationWorkspaceBinding) (*interaction.OrchestrationResult, error) {
	if req != nil && binding != nil {
		req.WorkspaceID = binding.WorkspaceID
		req.WorkspaceDeviceID = binding.DeviceID
		req.WorkspaceName = binding.WorkspaceName
		req.WorkspaceKind = binding.WorkspaceKind
		req.WorkspaceRootURI = binding.RootURI
	}
	return h.unifiedEntry.Handle(ctx, req)
}
