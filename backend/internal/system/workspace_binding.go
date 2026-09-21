package system

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	workspacepkg "github.com/u-ai/backend/internal/workspace"
	"gorm.io/gorm"
)

type conversationWorkspaceBinding struct {
	ConversationID string    `json:"conversationId"`
	ProjectID      string    `json:"projectId,omitempty"`
	WorkspaceID    string    `json:"workspaceId"`
	DeviceID       string    `json:"deviceId,omitempty"`
	WorkspaceName  string    `json:"workspaceName,omitempty"`
	WorkspaceKind  string    `json:"workspaceKind"`
	RootURI        string    `json:"rootUri"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (h *Handler) workspaceBindingForConversation(conversation chat.Conversation, spaceID string) *conversationWorkspaceBinding {
	if projectID := strings.TrimSpace(conversation.ProjectID); projectID != "" {
		return h.workspaceBindingForProject(projectID, conversation.ID, spaceID)
	}
	if workspaceID := strings.TrimSpace(conversation.WorkspaceID); workspaceID != "" {
		return h.workspaceBindingForMount(workspaceID, strings.TrimSpace(conversation.WorkspaceDeviceID), conversation.ID)
	}
	return nil
}

func (h *Handler) workspaceBindingForRequest(conversationID string, body webChatSendRequest, spaceID string) *conversationWorkspaceBinding {
	conversationID = strings.TrimSpace(conversationID)
	projectID := strings.TrimSpace(body.ProjectID)
	workspaceID := strings.TrimSpace(body.WorkspaceID)

	if conversationID != "" {
		var conversation chat.Conversation
		if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", conversationID).First(&conversation).Error; err == nil {
			if binding := h.workspaceBindingForConversation(conversation, spaceID); binding != nil {
				return binding
			}
		}
	}

	if projectID != "" {
		return h.workspaceBindingForProject(projectID, conversationID, spaceID)
	}
	if workspaceID != "" {
		return h.workspaceBindingForMount(workspaceID, strings.TrimSpace(body.WorkspaceDeviceID), conversationID)
	}
	return nil
}

func (h *Handler) workspaceBindingForProject(projectID, conversationID, spaceID string) *conversationWorkspaceBinding {
	var project chat.Project
	query := webChatOwnerQuery(h.db.Model(&chat.Project{}).Where("id = ?", strings.TrimSpace(projectID)), spaceID)
	if err := query.First(&project).Error; err != nil {
		return nil
	}
	var mount struct {
		Name    string
		Kind    string
		Enabled int
	}
	if err := h.db.Table("workspace_mounts").Select("name", "kind", "enabled").Where("id = ?", project.WorkspaceID).Take(&mount).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return nil
	}
	if mount.Enabled == 0 {
		return nil
	}
	return &conversationWorkspaceBinding{
		ConversationID: conversationID,
		ProjectID:      strings.TrimSpace(projectID),
		WorkspaceID:    project.WorkspaceID,
		DeviceID:       project.DeviceID,
		WorkspaceName:  mount.Name,
		WorkspaceKind:  mount.Kind,
		RootURI:        project.RootURI,
		UpdatedAt:      time.Now().UTC(),
	}
}

func (h *Handler) workspaceBindingForMount(workspaceID, deviceID, conversationID string) *conversationWorkspaceBinding {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	var mount struct {
		ID      string
		Name    string
		Kind    string
		Enabled int
	}
	if err := h.db.Table("workspace_mounts").Select("id", "name", "kind", "enabled").Where("id = ?", workspaceID).Take(&mount).Error; err != nil {
		return nil
	}
	if mount.Enabled == 0 {
		return nil
	}
	return &conversationWorkspaceBinding{
		ConversationID: conversationID,
		WorkspaceID:    mount.ID,
		DeviceID:       strings.TrimSpace(deviceID),
		WorkspaceName:  mount.Name,
		WorkspaceKind:  mount.Kind,
		RootURI:        workspacepkg.MountURI(workspacepkg.WorkspaceID(mount.ID)),
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
