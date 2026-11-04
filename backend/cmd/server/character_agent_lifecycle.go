package main

import (
	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/character"
	"gorm.io/gorm"
)

type characterAgentLifecycle struct {
	platform *agentplatform.Service
}

func (l characterAgentLifecycle) EnsureRoleAgent(tx *gorm.DB, role *character.Character) error {
	if l.platform == nil || role == nil {
		return nil
	}
	instructions := role.BasePrompt
	if instructions == "" {
		instructions = role.CharacterBase
	}
	if instructions == "" {
		instructions = role.Description
	}
	_, err := l.platform.SyncRoleAgent(tx, agentplatform.RoleAgentSyncInput{
		SpaceID: role.SpaceID, CharacterID: role.ID, Name: role.Name,
		Description: role.Description, Instructions: instructions, Status: role.Status,
		ModelConfigID: role.ModelConfigID, ReasoningEffort: role.ReasoningEffort,
		ReasoningEnabled: role.ReasoningEnabled,
	})
	return err
}

func (l characterAgentLifecycle) ArchiveRoleAgent(tx *gorm.DB, role *character.Character) error {
	if l.platform == nil || role == nil {
		return nil
	}
	return l.platform.ArchiveRoleAgent(tx, role.SpaceID, role.ID)
}
