package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RoleAgentSyncInput struct {
	SpaceID          string
	CharacterID      string
	Name             string
	Description      string
	Instructions     string
	Status           string
	ModelConfigID    int
	ReasoningEffort  string
	ReasoningEnabled int
}

func (s *Service) SyncRoleAgent(tx *gorm.DB, input RoleAgentSyncInput) (AgentProfile, error) {
	if s == nil || s.db == nil {
		return AgentProfile{}, errors.New("Agent 服务不可用")
	}
	if tx == nil {
		tx = s.db
	}
	spaceID := strings.TrimSpace(input.SpaceID)
	characterID := strings.TrimSpace(input.CharacterID)
	if spaceID == "" || characterID == "" {
		return AgentProfile{}, errors.New("角色与空间不能为空")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "新角色"
	}
	desiredStatus := "active"
	if strings.EqualFold(strings.TrimSpace(input.Status), "disabled") {
		desiredStatus = "disabled"
	}
	profiles, err := s.roleAgentProfiles(tx, spaceID, characterID)
	if err != nil {
		return AgentProfile{}, err
	}
	now := time.Now().UTC()
	var canonical AgentProfile
	if len(profiles) == 0 {
		canonical = AgentProfile{
			ID: newID("agent"), SpaceID: spaceID, CharacterID: characterID,
			Name: name, Description: strings.TrimSpace(input.Description),
			Archetype: "companion", Status: desiredStatus, Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&canonical).Error; err != nil {
			return AgentProfile{}, fmt.Errorf("创建角色 Agent 档案: %w", err)
		}
	} else {
		canonical = profiles[0]
	}
	revision, err := s.roleAgentCurrentRevision(tx, spaceID, canonical)
	if err != nil {
		return AgentProfile{}, err
	}
	configJSON, configHash, err := buildRoleAgentConfig(revision.ConfigJSON, input)
	if err != nil {
		return AgentProfile{}, err
	}
	profileChanged := canonical.Name != name ||
		canonical.Description != strings.TrimSpace(input.Description) ||
		canonical.Status != desiredStatus
	revisionChanged := revision.ID == "" || revision.ConfigHash != configHash
	if revisionChanged {
		nextRevision := canonical.Revision + 1
		if revision.ID == "" {
			nextRevision = maxInt64(canonical.Revision, 1)
		}
		revision = AgentRevision{
			ID: newID("revision"), SpaceID: spaceID, AgentID: canonical.ID,
			Revision: nextRevision, ConfigJSON: configJSON, ConfigHash: configHash,
			Status: "draft", ChangeNote: "角色自动同步", CreatedAt: now,
			TestStatus: "auto_passed",
		}
		if err := tx.Create(&revision).Error; err != nil {
			return AgentProfile{}, fmt.Errorf("创建角色 Agent 版本: %w", err)
		}
		canonical.CurrentRevisionID = revision.ID
		canonical.Revision = nextRevision
		profileChanged = true
	}
	if desiredStatus == "active" && (canonical.CurrentReleaseID == "" || revision.ID != canonical.CurrentRevisionID || canonical.Status != "active") {
		release, releaseErr := publishRoleAgentRevision(tx, &canonical, &revision, now)
		if releaseErr != nil {
			return AgentProfile{}, releaseErr
		}
		canonical.CurrentReleaseID = release.ID
		canonical.Status = "active"
		profileChanged = true
	}
	updates := map[string]any{
		"name": name, "description": strings.TrimSpace(input.Description),
		"status": desiredStatus, "updated_at": now,
	}
	if profileChanged {
		updates["current_revision_id"] = canonical.CurrentRevisionID
		updates["current_release_id"] = canonical.CurrentReleaseID
		updates["revision"] = canonical.Revision
	}
	if err := tx.Model(&AgentProfile{}).Where("id = ? AND space_id = ?", canonical.ID, spaceID).Updates(updates).Error; err != nil {
		return AgentProfile{}, fmt.Errorf("更新角色 Agent 档案: %w", err)
	}
	canonical.Name = name
	canonical.Description = strings.TrimSpace(input.Description)
	canonical.Status = desiredStatus
	canonical.UpdatedAt = now
	if len(profiles) > 1 {
		duplicateIDs := make([]string, 0, len(profiles)-1)
		for _, profile := range profiles[1:] {
			duplicateIDs = append(duplicateIDs, profile.ID)
		}
		if err := mergeRoleAgentDuplicates(tx, spaceID, canonical, duplicateIDs, now); err != nil {
			return AgentProfile{}, err
		}
	}
	return canonical, nil
}

func (s *Service) ArchiveRoleAgent(tx *gorm.DB, spaceID, characterID string) error {
	if s == nil || s.db == nil {
		return errors.New("Agent 服务不可用")
	}
	if tx == nil {
		tx = s.db
	}
	spaceID = strings.TrimSpace(spaceID)
	characterID = strings.TrimSpace(characterID)
	if spaceID == "" || characterID == "" {
		return nil
	}
	profiles, err := s.roleAgentProfiles(tx, spaceID, characterID)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return nil
	}
	now := time.Now().UTC()
	agentIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		agentIDs = append(agentIDs, profile.ID)
	}
	if err := tx.Where("member_type = ? AND member_id IN ?", "agent", agentIDs).Delete(&TeamMember{}).Error; err != nil {
		return fmt.Errorf("移除已删除角色的团队成员关系: %w", err)
	}
	if err := tx.Model(&Team{}).Where("space_id = ? AND lead_agent_id IN ?", spaceID, agentIDs).Updates(map[string]any{"lead_agent_id": "", "updated_at": now}).Error; err != nil {
		return fmt.Errorf("清除已删除角色的团队负责人关系: %w", err)
	}
	if err := tx.Model(&AgentProfile{}).Where("space_id = ? AND id IN ?", spaceID, agentIDs).Updates(map[string]any{
		"status": "archived", "deleted_at": now, "updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("归档角色 Agent 档案: %w", err)
	}
	return nil
}

func (s *Service) ReconcileRoleAgents(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("Agent 服务不可用")
	}
	var roles []struct {
		ID               string
		SpaceID          string
		Name             string
		Description      string
		BasePrompt       string
		CharacterBase    string
		Status           string
		ModelConfigID    int
		ReasoningEffort  string
		ReasoningEnabled int
	}
	if err := s.db.WithContext(ctx).Table("characters").
		Select("id, space_id, name, description, base_prompt, character_base, status, model_config_id, reasoning_effort, reasoning_enabled").
		Where("deleted_at IS NULL OR deleted_at = ''").
		Order("created_at ASC").
		Scan(&roles).Error; err != nil {
		return fmt.Errorf("读取待同步角色: %w", err)
	}
	for _, role := range roles {
		input := RoleAgentSyncInput{
			SpaceID: role.SpaceID, CharacterID: role.ID, Name: role.Name,
			Description: role.Description, Instructions: firstNonEmpty(firstNonEmpty(role.BasePrompt, role.CharacterBase), role.Description),
			Status: role.Status, ModelConfigID: role.ModelConfigID,
			ReasoningEffort: role.ReasoningEffort, ReasoningEnabled: role.ReasoningEnabled,
		}
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, err := s.SyncRoleAgent(tx, input)
			return err
		}); err != nil {
			return fmt.Errorf("同步角色 %s 的 Agent 档案: %w", role.ID, err)
		}
	}
	if err := s.db.WithContext(ctx).Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_profiles_character_unique
		ON agent_profiles(space_id, character_id)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("创建角色 Agent 唯一索引: %w", err)
	}
	return nil
}

func (s *Service) roleAgentProfiles(tx *gorm.DB, spaceID, characterID string) ([]AgentProfile, error) {
	var profiles []AgentProfile
	if err := tx.Where("space_id = ? AND character_id = ? AND deleted_at IS NULL", spaceID, characterID).
		Order("CASE WHEN status = 'active' AND current_release_id <> '' THEN 0 WHEN status = 'active' THEN 1 WHEN status = 'draft' THEN 2 ELSE 3 END, revision DESC, updated_at DESC").
		Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("读取角色 Agent 档案: %w", err)
	}
	return profiles, nil
}

func (s *Service) roleAgentCurrentRevision(tx *gorm.DB, spaceID string, profile AgentProfile) (AgentRevision, error) {
	var revision AgentRevision
	if strings.TrimSpace(profile.CurrentRevisionID) == "" {
		return revision, nil
	}
	if err := tx.Where("id = ? AND space_id = ? AND agent_id = ?", profile.CurrentRevisionID, spaceID, profile.ID).First(&revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentRevision{}, nil
		}
		return AgentRevision{}, fmt.Errorf("读取角色 Agent 当前版本: %w", err)
	}
	return revision, nil
}

func buildRoleAgentConfig(currentJSON string, input RoleAgentSyncInput) (string, string, error) {
	config := map[string]any{}
	if raw := strings.TrimSpace(currentJSON); raw != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return "", "", fmt.Errorf("解析角色 Agent 配置: %w", err)
		}
	}
	identity := objectValue(config["identity"])
	identity["kind"] = "role"
	identity["name"] = strings.TrimSpace(input.Name)
	identity["description"] = strings.TrimSpace(input.Description)
	config["identity"] = identity
	config["instructions"] = strings.TrimSpace(input.Instructions)
	model := objectValue(config["model"])
	delete(model, "configId")
	delete(model, "name")
	delete(model, "model")
	if input.ModelConfigID > 0 {
		model["configId"] = input.ModelConfigID
	}
	if effort := strings.TrimSpace(input.ReasoningEffort); effort != "" {
		model["reasoningEffort"] = effort
	} else {
		delete(model, "reasoningEffort")
	}
	if input.ReasoningEnabled >= 0 {
		model["reasoningEnabled"] = input.ReasoningEnabled == 1
	}
	config["model"] = model
	if _, ok := config["memory"]; !ok {
		config["memory"] = map[string]any{"scope": "agent"}
	}
	if _, ok := config["permissions"]; !ok {
		config["permissions"] = map[string]any{"mode": "request_approval"}
	}
	if _, ok := config["runtime"]; !ok {
		config["runtime"] = map[string]any{"provider": "local"}
	}
	configJSON, configHash, err := hashJSON(config)
	if err != nil {
		return "", "", fmt.Errorf("编码角色 Agent 配置: %w", err)
	}
	return configJSON, configHash, nil
}

func publishRoleAgentRevision(tx *gorm.DB, profile *AgentProfile, revision *AgentRevision, now time.Time) (AgentRelease, error) {
	if err := tx.Model(&AgentRelease{}).Where("agent_id = ? AND status = ?", profile.ID, "published").Updates(map[string]any{
		"status": "superseded", "superseded_at": now,
	}).Error; err != nil {
		return AgentRelease{}, fmt.Errorf("退役旧角色 Agent 版本: %w", err)
	}
	release := AgentRelease{
		ID: newID("release"), SpaceID: profile.SpaceID, AgentID: profile.ID,
		RevisionID: revision.ID, Version: fmt.Sprintf("role-%d", revision.Revision),
		ReleaseNotes: "角色自动同步", Status: "published", CreatedAt: now,
	}
	if err := tx.Create(&release).Error; err != nil {
		return AgentRelease{}, fmt.Errorf("发布角色 Agent 版本: %w", err)
	}
	if err := tx.Model(&AgentRevision{}).Where("id = ?", revision.ID).Updates(map[string]any{
		"status": "published", "test_status": "auto_passed", "published_at": now,
	}).Error; err != nil {
		return AgentRelease{}, fmt.Errorf("更新角色 Agent 发布版本: %w", err)
	}
	revision.Status = "published"
	revision.TestStatus = "auto_passed"
	revision.PublishedAt = now
	return release, nil
}

func mergeRoleAgentDuplicates(tx *gorm.DB, spaceID string, canonical AgentProfile, duplicateIDs []string, now time.Time) error {
	if len(duplicateIDs) == 0 {
		return nil
	}
	var memberships []TeamMember
	if err := tx.Where("member_type = ? AND member_id IN ?", "agent", duplicateIDs).Find(&memberships).Error; err != nil {
		return fmt.Errorf("读取待迁移团队成员: %w", err)
	}
	for _, membership := range memberships {
		var canonicalCount int64
		if err := tx.Model(&TeamMember{}).Where("space_id = ? AND team_id = ? AND member_type = ? AND member_id = ?", spaceID, membership.TeamID, "agent", canonical.ID).Count(&canonicalCount).Error; err != nil {
			return fmt.Errorf("检查团队保留成员: %w", err)
		}
		if canonicalCount > 0 {
			if err := tx.Delete(&TeamMember{}, "id = ?", membership.ID).Error; err != nil {
				return fmt.Errorf("删除重复团队成员: %w", err)
			}
			continue
		}
		if err := tx.Model(&TeamMember{}).Where("id = ?", membership.ID).Updates(map[string]any{
			"member_id": canonical.ID, "updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("迁移团队成员: %w", err)
		}
	}
	if err := tx.Model(&Team{}).Where("space_id = ? AND lead_agent_id IN ?", spaceID, duplicateIDs).Updates(map[string]any{"lead_agent_id": canonical.ID, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("迁移团队负责人: %w", err)
	}
	if err := tx.Model(&TeamHandoff{}).Where("space_id = ? AND from_agent_id IN ?", spaceID, duplicateIDs).Update("from_agent_id", canonical.ID).Error; err != nil {
		return fmt.Errorf("迁移团队交接来源: %w", err)
	}
	if err := tx.Model(&TeamHandoff{}).Where("space_id = ? AND to_agent_id IN ?", spaceID, duplicateIDs).Update("to_agent_id", canonical.ID).Error; err != nil {
		return fmt.Errorf("迁移团队交接目标: %w", err)
	}
	references := []struct {
		table  string
		column string
	}{
		{"runtime_instances", "agent_id"},
		{"runtime_snapshots", "agent_id"},
		{"host_control_sessions", "agent_id"},
		{"approval_records", "agent_id"},
		{"resource_leases", "holder_agent_id"},
		{"idempotent_operations", "agent_id"},
		{"activity_events", "agent_id"},
	}
	for _, reference := range references {
		if err := tx.Table(reference.table).Where(reference.column+" IN ?", duplicateIDs).Update(reference.column, canonical.ID).Error; err != nil {
			return fmt.Errorf("迁移 %s.%s: %w", reference.table, reference.column, err)
		}
	}
	if err := tx.Where("space_id = ? AND agent_id IN ? AND EXISTS (SELECT 1 FROM agent_autonomy_policies p WHERE p.space_id = ? AND p.agent_id = ?)", spaceID, duplicateIDs, spaceID, canonical.ID).
		Delete(&AgentAutonomyPolicy{}).Error; err != nil {
		return fmt.Errorf("清理重复自治策略: %w", err)
	}
	if err := tx.Model(&AgentAutonomyPolicy{}).Where("space_id = ? AND agent_id IN ?", spaceID, duplicateIDs).Update("agent_id", canonical.ID).Error; err != nil {
		return fmt.Errorf("迁移角色自治策略: %w", err)
	}
	if err := tx.Model(&AgentProfile{}).Where("space_id = ? AND id IN ?", spaceID, duplicateIDs).Updates(map[string]any{
		"status": "archived", "deleted_at": now, "updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("归档重复角色 Agent 档案: %w", err)
	}
	return nil
}
