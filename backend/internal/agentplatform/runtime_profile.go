package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type ResolvedAgentRuntime struct {
	AgentID          string   `json:"agentId"`
	CharacterID      string   `json:"characterId"`
	TeamID           string   `json:"teamId"`
	RuntimeID        string   `json:"runtimeId"`
	ModelConfigID    int      `json:"modelConfigId"`
	ReasoningEffort  string   `json:"reasoningEffort"`
	PermissionMode   string   `json:"permissionMode"`
	WorkspaceID      string   `json:"workspaceId"`
	WorkspaceRootURI string   `json:"workspaceRootUri"`
	Skills           []string `json:"skills"`
	Tools            []string `json:"tools"`
	Instructions     string   `json:"instructions"`
}

func (s *Service) ResolveAgentRuntime(ctx context.Context, spaceID, characterID string) (ResolvedAgentRuntime, bool, error) {
	characterID = strings.TrimSpace(characterID)
	if s == nil || s.db == nil || characterID == "" {
		return ResolvedAgentRuntime{}, false, nil
	}
	var agent AgentProfile
	err := s.db.WithContext(ctx).Where(
		"space_id = ? AND character_id = ? AND status = ? AND deleted_at IS NULL",
		spaceID, characterID, "active",
	).Order("updated_at DESC").First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResolvedAgentRuntime{}, false, nil
	}
	if err != nil {
		return ResolvedAgentRuntime{}, false, fmt.Errorf("读取 Agent 运行时配置: %w", err)
	}
	var revision AgentRevision
	if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ?", agent.CurrentRevisionID, spaceID).First(&revision).Error; err != nil {
		return ResolvedAgentRuntime{}, false, fmt.Errorf("读取已发布 Agent 配置: %w", err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(revision.ConfigJSON), &config); err != nil {
		return ResolvedAgentRuntime{}, false, fmt.Errorf("解析已发布 Agent 配置: %w", err)
	}
	profile := ResolvedAgentRuntime{
		AgentID: agent.ID, CharacterID: agent.CharacterID, TeamID: agent.TeamID,
		PermissionMode: "request_approval",
	}
	model := objectValue(config["model"])
	profile.ModelConfigID = integerValue(model["configId"])
	if profile.ModelConfigID == 0 {
		name := firstNonEmpty(stringValueFromMap(model, "name"), stringValueFromMap(model, "model"))
		if name != "" {
			var modelConfig struct {
				ID int
			}
			query := s.db.WithContext(ctx).Table("model_configs").Select("id").Where("is_active = ?", true)
			if err := query.Where("name = ? OR model_name = ?", name, name).Order("id DESC").Scan(&modelConfig).Error; err != nil {
				return ResolvedAgentRuntime{}, false, fmt.Errorf("解析 Agent 模型: %w", err)
			}
			profile.ModelConfigID = modelConfig.ID
		}
	}
	profile.ReasoningEffort = stringValueFromMap(model, "reasoningEffort")
	permissions := objectValue(config["permissions"])
	profile.PermissionMode = firstNonEmpty(stringValueFromMap(permissions, "mode"), profile.PermissionMode)
	profile.Skills = stringSlice(config["skills"])
	profile.Tools = stringSlice(config["tools"])
	profile.Instructions = stringValueFromMap(config, "instructions")
	var runtimeInstance RuntimeInstance
	err = s.db.WithContext(ctx).Where("space_id = ? AND agent_id = ?", spaceID, agent.ID).Order("updated_at DESC").First(&runtimeInstance).Error
	if err == nil {
		profile.RuntimeID = runtimeInstance.ID
		profile.WorkspaceID = runtimeInstance.WorkspaceID
		profile.WorkspaceRootURI = runtimeInstance.WorkspaceRootURI
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ResolvedAgentRuntime{}, false, fmt.Errorf("读取 Agent 电脑绑定: %w", err)
	}
	if agent.TeamID != "" {
		var team Team
		if err := s.db.WithContext(ctx).Where("id = ? AND space_id = ? AND deleted_at IS NULL", agent.TeamID, spaceID).First(&team).Error; err != nil {
			return ResolvedAgentRuntime{}, false, fmt.Errorf("读取 Agent 团队工作区: %w", err)
		}
		if team.SharedWorkspace {
			workspaceID, rootURI, err := s.ensureTeamWorkspace(ctx, team)
			if err != nil {
				return ResolvedAgentRuntime{}, false, err
			}
			profile.WorkspaceID = workspaceID
			profile.WorkspaceRootURI = rootURI
		}
	}
	return profile, true, nil
}

func objectValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		result = map[string]any{}
	}
	return result
}

func stringValueFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func integerValue(value any) int {
	switch current := value.(type) {
	case int:
		return current
	case int64:
		return int(current)
	case float64:
		return int(current)
	case json.Number:
		parsed, _ := strconv.Atoi(current.String())
		return parsed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(current))
		return parsed
	default:
		return 0
	}
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return append([]string(nil), typed...)
		}
		return []string{}
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}
