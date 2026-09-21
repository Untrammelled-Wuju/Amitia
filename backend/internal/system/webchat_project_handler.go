package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type webChatProjectService interface {
	ListConversationSidebarForSpace(spaceID string, recentLimit, projectConversationLimit int) (*chat.ConversationSidebarResponse, error)
	ListChannelConversationsForSpace(spaceID string, limit int) ([]chat.Conversation, error)
	CreateProjectForSpace(req *chat.CreateProjectRequest, spaceID string) (*chat.Project, error)
	UpdateProjectForSpace(projectID string, req *chat.UpdateProjectRequest, spaceID string) (*chat.Project, error)
	DeleteProjectForSpace(projectID, spaceID string) error
	MoveConversationToProjectForSpace(conversationID, projectID, spaceID string) (*chat.Conversation, error)
	UpdateConversationSidebarStateForSpace(conversationID string, pinned, archived *bool, spaceID string) (*chat.Conversation, error)
	GetProjectOpenTargetForSpace(projectID, spaceID string) (*chat.ProjectOpenTarget, error)
}

func (h *Handler) WebChatListChannelConversations(c *gin.Context) {
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide channel operations", nil)
		return
	}
	items, err := scoped.ListChannelConversationsForSpace(webChatSpaceID(c), 100)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取渠道消息失败", nil)
		return
	}
	if h.channelAccess != nil {
		filtered := make([]chat.Conversation, 0, len(items))
		for _, item := range items {
			if h.channelAccess.Has(item.Channel) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	util.SuccessResponse(c, gin.H{"items": items})
}

func (h *Handler) WebChatConversationSidebar(c *gin.Context) {
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
		return
	}
	sidebar, err := scoped.ListConversationSidebarForSpace(webChatSpaceID(c), 100, 100)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取侧边栏失败", nil)
		return
	}
	util.SuccessResponse(c, sidebar)
}

func (h *Handler) WebChatCreateProject(c *gin.Context) {
	var body chat.CreateProjectRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
		return
	}
	project, err := scoped.CreateProjectForSpace(&body, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, project)
}

func (h *Handler) WebChatUpdateProject(c *gin.Context) {
	var body chat.UpdateProjectRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
		return
	}
	project, err := scoped.UpdateProjectForSpace(strings.TrimSpace(c.Param("id")), &body, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, project)
}

func (h *Handler) WebChatDeleteProject(c *gin.Context) {
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
		return
	}
	if err := scoped.DeleteProjectForSpace(strings.TrimSpace(c.Param("id")), webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatProjectLocation(c *gin.Context) {
	scoped, ok := h.chatSvc.(webChatProjectService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
		return
	}
	target, err := scoped.GetProjectOpenTargetForSpace(strings.TrimSpace(c.Param("id")), webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, target)
}
