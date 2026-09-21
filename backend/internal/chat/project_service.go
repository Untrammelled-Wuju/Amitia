package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *service) ListConversationSidebarForSpace(spaceID string, recentLimit, projectConversationLimit int) (*ConversationSidebarResponse, error) {
	if recentLimit <= 0 {
		recentLimit = 20
	}
	if projectConversationLimit <= 0 {
		projectConversationLimit = 20
	}
	owner := normalizeConversationOwner(spaceID)
	pinned := make([]Conversation, 0)
	pinnedQuery := s.db.Where("deleted_at IS NULL AND COALESCE(archived_at, '') = '' AND COALESCE(pinned_at, '') <> '' AND LOWER(channel) = 'web'")
	pinnedQuery = applyConversationOwnerScope(pinnedQuery, spaceID)
	if err := pinnedQuery.Order("pinned_at DESC, updated_at DESC").Limit(recentLimit).Find(&pinned).Error; err != nil {
		return nil, err
	}
	for i := range pinned {
		pinned[i].MessageCount = int(s.repo.CountMessagesByConv(pinned[i].ID))
	}
	recent := make([]Conversation, 0)
	recentQuery := s.db.Where("deleted_at IS NULL AND COALESCE(archived_at, '') = '' AND COALESCE(pinned_at, '') = '' AND COALESCE(project_id, '') = '' AND LOWER(channel) = 'web'")
	recentQuery = applyConversationOwnerScope(recentQuery, spaceID)
	if err := recentQuery.Order("updated_at DESC").Limit(recentLimit).Find(&recent).Error; err != nil {
		return nil, err
	}
	for i := range recent {
		recent[i].MessageCount = int(s.repo.CountMessagesByConv(recent[i].ID))
	}

	var projects []Project
	projectQuery := s.db.Where("space_id = ?", owner).Order("updated_at DESC")
	if err := projectQuery.Find(&projects).Error; err != nil {
		return nil, err
	}
	result := &ConversationSidebarResponse{
		Pinned:   pinned,
		Recent:   recent,
		Projects: make([]ProjectConversationSummary, 0, len(projects)),
	}
	for _, project := range projects {
		summary := ProjectConversationSummary{
			Project:       project,
			Available:     true,
			Status:        "ready",
			Conversations: make([]Conversation, 0),
		}
		if s.workspaceResolver != nil {
			available, status, reason := s.workspaceResolver.ResolveWorkspaceAvailability(project.WorkspaceID)
			summary.Available = available
			summary.Status = status
			summary.StatusReason = reason
		} else {
			var mount struct {
				Enabled  int
				ReadOnly int
			}
			if err := s.db.Table("workspace_mounts").Select("enabled", "read_only").Where("id = ?", project.WorkspaceID).Take(&mount).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					summary.Available = false
					summary.Status = "missing"
					summary.StatusReason = "项目根目录不存在"
				} else {
					return nil, err
				}
			} else if mount.Enabled == 0 {
				summary.Available = false
				summary.Status = "disabled"
				summary.StatusReason = "项目根目录已停用"
			} else if mount.ReadOnly != 0 {
				summary.Status = "read_only"
			}
		}
		if err := s.db.Model(&Conversation{}).
			Where("space_id = ? AND project_id = ? AND deleted_at IS NULL AND COALESCE(archived_at, '') = '' AND LOWER(channel) = 'web'", owner, project.ID).
			Count(&summary.ConversationCount).Error; err != nil {
			return nil, err
		}
		if err := s.db.Where("space_id = ? AND project_id = ? AND deleted_at IS NULL AND COALESCE(archived_at, '') = '' AND COALESCE(pinned_at, '') = '' AND LOWER(channel) = 'web'", owner, project.ID).
			Order("updated_at DESC").
			Limit(projectConversationLimit).
			Find(&summary.Conversations).Error; err != nil {
			return nil, err
		}
		for i := range summary.Conversations {
			summary.Conversations[i].MessageCount = int(s.repo.CountMessagesByConv(summary.Conversations[i].ID))
		}
		result.Projects = append(result.Projects, summary)
	}
	return result, nil
}

func (s *service) ListChannelConversationsForSpace(spaceID string, limit int) ([]Conversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var conversations []Conversation
	query := s.db.Where("deleted_at IS NULL AND COALESCE(project_id, '') = '' AND channel <> '' AND channel <> 'web' AND channel <> 'long_running'")
	query = applyConversationOwnerScope(query, spaceID)
	if err := query.Order("updated_at DESC").Limit(limit).Find(&conversations).Error; err != nil {
		return nil, err
	}
	if conversations == nil {
		conversations = []Conversation{}
	}
	for i := range conversations {
		conversations[i].MessageCount = int(s.repo.CountMessagesByConv(conversations[i].ID))
	}
	return conversations, nil
}

func (s *service) CreateProjectForSpace(req *CreateProjectRequest, spaceID string) (*Project, error) {
	if req == nil {
		return nil, fmt.Errorf("project request is required")
	}
	name := strings.TrimSpace(req.Name)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("项目根目录不能为空")
	}
	if name == "" {
		name = "新项目"
	}
	owner := normalizeConversationOwner(spaceID)
	project := &Project{
		ID:          uuid.New().String(),
		SpaceID:     owner,
		Name:        name,
		WorkspaceID: workspaceID,
		DeviceID:    strings.TrimSpace(req.DeviceID),
		RootURI:     strings.TrimSpace(req.RootURI),
	}
	if project.RootURI == "" {
		project.RootURI = "amitia://workspace/@" + workspaceID + "/"
	}
	if err := s.db.Create(project).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("该文件夹已经属于一个项目")
		}
		return nil, err
	}
	return project, nil
}

func (s *service) UpdateProjectForSpace(projectID string, req *UpdateProjectRequest, spaceID string) (*Project, error) {
	if req == nil {
		return nil, fmt.Errorf("project request is required")
	}
	project, err := s.requireProjectForSpace(projectID, spaceID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": time.Now().Format("2006-01-02 15:04:05")}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("项目名称不能为空")
		}
		updates["name"] = name
	}
	if req.WorkspaceID != nil {
		workspaceID := strings.TrimSpace(*req.WorkspaceID)
		if workspaceID == "" {
			return nil, fmt.Errorf("项目根目录不能为空")
		}
		updates["workspace_id"] = workspaceID
		if req.RootURI == nil {
			updates["root_uri"] = "amitia://workspace/@" + workspaceID + "/"
		}
	}
	if req.DeviceID != nil {
		updates["device_id"] = strings.TrimSpace(*req.DeviceID)
	}
	if req.RootURI != nil {
		updates["root_uri"] = strings.TrimSpace(*req.RootURI)
	}
	if req.Pinned != nil {
		if *req.Pinned {
			updates["pinned_at"] = time.Now().Format("2006-01-02 15:04:05")
		} else {
			updates["pinned_at"] = ""
		}
	}
	if err := s.db.Model(&Project{}).Where("id = ?", project.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.requireProjectForSpace(project.ID, spaceID)
}

func (s *service) DeleteProjectForSpace(projectID, spaceID string) error {
	project, err := s.requireProjectForSpace(projectID, spaceID)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Conversation{}).Where("project_id = ?", project.ID).Update("project_id", "").Error; err != nil {
			return err
		}
		return tx.Where("id = ?", project.ID).Delete(&Project{}).Error
	})
}

func (s *service) CreateProjectConversationForSpace(projectID string, req *CreateConversationRequest, spaceID string) (*Conversation, error) {
	if _, err := s.requireProjectForSpace(projectID, spaceID); err != nil {
		return nil, err
	}
	if req == nil {
		req = &CreateConversationRequest{}
	}
	copyReq := *req
	copyReq.ProjectID = projectID
	if existing, err := s.findReusableEmptyConversationForSpace(projectID, copyReq.Channel, copyReq.Source, spaceID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	return s.CreateConversationForSpace(&copyReq, spaceID)
}

func (s *service) findReusableEmptyConversationForSpace(projectID, channel, source, spaceID string) (*Conversation, error) {
	if strings.TrimSpace(channel) != "" && !strings.EqualFold(strings.TrimSpace(channel), "web") {
		return nil, nil
	}
	if source != "" && !strings.EqualFold(strings.TrimSpace(source), "web") && !strings.EqualFold(strings.TrimSpace(source), "mobile") {
		return nil, nil
	}
	var conversation Conversation
	query := s.db.Where(`deleted_at IS NULL
		AND channel = 'web'
		AND COALESCE(archived_at, '') = ''
		AND COALESCE(pinned_at, '') = ''
		AND COALESCE(project_id, '') = ?
		AND (TRIM(COALESCE(title, '')) = '' OR title = '新对话')
		AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = conversations.id AND m.deleted_at IS NULL)`,
		strings.TrimSpace(projectID))
	query = applyConversationOwnerScope(query, spaceID)
	if err := query.Order("updated_at DESC").First(&conversation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

func (s *service) UpdateConversationSidebarStateForSpace(conversationID string, pinned, archived *bool, spaceID string) (*Conversation, error) {
	conversation, err := s.requireConversationOwner(conversationID, spaceID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"revision": conversation.Revision + 1,
	}
	if pinned != nil {
		if *pinned {
			updates["pinned_at"] = now
			updates["archived_at"] = ""
		} else {
			updates["pinned_at"] = ""
		}
	}
	if archived != nil {
		if *archived {
			updates["archived_at"] = now
			updates["pinned_at"] = ""
		} else {
			updates["archived_at"] = ""
		}
	}
	if err := s.db.Model(&Conversation{}).Where("id = ? AND revision = ?", conversation.ID, conversation.Revision).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.requireConversationOwner(conversation.ID, spaceID)
}

func (s *service) GetProjectOpenTargetForSpace(projectID, spaceID string) (*ProjectOpenTarget, error) {
	project, err := s.requireProjectForSpace(projectID, spaceID)
	if err != nil {
		return nil, err
	}
	var mount struct {
		Kind        string
		LocalRoot   string
		NativeGrant string
	}
	if err := s.db.Table("workspace_mounts").
		Select("kind", "COALESCE(local_root, '') AS local_root", "COALESCE(native_grant_id, '') AS native_grant").
		Where("id = ?", project.WorkspaceID).
		Take(&mount).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("项目根目录不存在")
		}
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(mount.Kind)) {
	case "local":
		if strings.TrimSpace(mount.LocalRoot) == "" {
			return nil, fmt.Errorf("项目根目录不存在")
		}
		return &ProjectOpenTarget{Kind: "local", Path: mount.LocalRoot, URI: project.RootURI}, nil
	case "saf":
		if strings.TrimSpace(mount.NativeGrant) == "" {
			return nil, fmt.Errorf("项目目录授权不可用")
		}
		return &ProjectOpenTarget{Kind: "saf", URI: mount.NativeGrant}, nil
	default:
		return nil, fmt.Errorf("当前项目目录不支持在资源管理器中打开")
	}
}

func (s *service) MoveConversationToProjectForSpace(conversationID, projectID, spaceID string) (*Conversation, error) {
	conversation, err := s.requireConversationOwner(conversationID, spaceID)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID != "" {
		if _, err := s.requireProjectForSpace(projectID, spaceID); err != nil {
			return nil, err
		}
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	revision := conversation.Revision + 1
	if err := s.db.Model(&Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]interface{}{
		"project_id":          projectID,
		"workspace_id":        "",
		"workspace_device_id": "",
		"updated_at":          now,
		"revision":            revision,
	}).Error; err != nil {
		return nil, err
	}
	conversation.ProjectID = projectID
	conversation.WorkspaceID = ""
	conversation.WorkspaceDeviceID = ""
	conversation.UpdatedAt = now
	conversation.Revision = revision
	return conversation, nil
}

func (s *service) requireProjectForSpace(projectID, spaceID string) (*Project, error) {
	var project Project
	query := s.db.Where("id = ?", strings.TrimSpace(projectID))
	owner := normalizeConversationOwner(spaceID)
	if chatLocalSingleUserMode() {
		query = query.Where("(space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?)", owner, "default")
	} else {
		query = query.Where("space_id = ?", owner)
	}
	if err := query.First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("项目不存在")
		}
		return nil, err
	}
	return &project, nil
}
