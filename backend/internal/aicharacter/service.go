// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package aicharacter

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	GetDefault() map[string]interface{}
	GetCharacter(id string) map[string]interface{}
	GetPresets() []map[string]interface{}
	GetPreset(id string) map[string]interface{}
	SetDefault(id string) map[string]interface{}
	ResetDefault() map[string]interface{}
	PreviewPrompt(body map[string]interface{}) map[string]interface{}
	SaveCharacter(body map[string]interface{}) map[string]interface{}
}

type service struct {
	db *gorm.DB
}

func NewService(ctx *app.AppContext) Service { return &service{db: ctx.DB} }

func (s *service) GetDefault() map[string]interface{} {
	var c struct {
		ID, Name, Identity, PersonalityConfig string
		IsDefault                             int
	}
	err := s.db.Table("characters").Select("id, name, identity, personality_config, is_default").Where("is_default = 1").Limit(1).Row().Scan(&c.ID, &c.Name, &c.Identity, &c.PersonalityConfig, &c.IsDefault)
	if err != nil {
		return map[string]interface{}{"id": "", "name": "", "identity": ""}
	}
	result := map[string]interface{}{"id": c.ID, "name": c.Name, "identity": c.Identity, "isDefault": c.IsDefault == 1}
	if c.PersonalityConfig != "" {
		var pc map[string]interface{}
		if json.Unmarshal([]byte(c.PersonalityConfig), &pc) == nil {
			result["personalityConfig"] = pc
		}
	}
	return result
}

func (s *service) GetCharacter(id string) map[string]interface{} {
	var c map[string]interface{}
	s.db.Table("characters").Where("id = ?", id).Take(&c)
	return c
}

func (s *service) GetPresets() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "preset-friendly", "name": "友好伙伴", "category": "日常", "description": "温暖友善的聊天伙伴"},
		{"id": "preset-mentor", "name": "人生导师", "category": "成长", "description": "睿智的人生指导者"},
		{"id": "preset-companion", "name": "贴心伴侣", "category": "情感", "description": "温柔体贴的伴侣"},
		{"id": "preset-funny", "name": "开心果", "category": "娱乐", "description": "幽默风趣的开心果"},
	}
}

func (s *service) GetPreset(id string) map[string]interface{} {
	presets := s.GetPresets()
	for _, p := range presets {
		if p["id"] == id {
			return p
		}
	}
	return map[string]interface{}{}
}

func (s *service) SetDefault(id string) map[string]interface{} {
	s.db.Table("characters").Where("is_default = 1").Update("is_default", 0)
	s.db.Table("characters").Where("id = ?", id).Update("is_default", 1)
	s.db.Exec("UPDATE conversations SET character_id = ?, updated_at = datetime('now', 'localtime') WHERE channel = 'wechat'", id)
	return map[string]interface{}{"success": true}
}

func (s *service) ResetDefault() map[string]interface{} {
	s.db.Table("characters").Where("is_default = 1").Update("is_default", 0)
	return map[string]interface{}{"success": true}
}

func (s *service) PreviewPrompt(body map[string]interface{}) map[string]interface{} {
	prompt := ""
	if v, ok := body["systemPrompt"].(string); ok {
		prompt = v
	}
	return map[string]interface{}{"preview": prompt, "estimatedTokens": len(prompt) / 2}
}

func (s *service) SaveCharacter(body map[string]interface{}) map[string]interface{} {
	id, _ := body["id"].(string)
	name, _ := body["name"].(string)
	desc, _ := body["description"].(string)

	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}
	if desc != "" {
		updates["description"] = desc
	}

	// Handle personalityConfig
	if pc, ok := body["personalityConfig"]; ok {
		if pcBytes, err := json.Marshal(pc); err == nil {
			updates["personality_config"] = string(pcBytes)
		}
	}

	// Handle lifeIdentity
	if li, ok := body["lifeIdentity"].(string); ok && li != "" {
		updates["life_identity"] = li
	}

	// Handle isDefault - only update if explicitly provided (accepts bool or number)
	if isDef, ok := body["isDefault"]; ok {
		var isDefaultVal bool
		switch v := isDef.(type) {
		case bool:
			isDefaultVal = v
		case float64:
			isDefaultVal = v != 0
		}
		if isDefaultVal {
			s.db.Table("characters").Where("is_default = 1").Update("is_default", 0)
			updates["is_default"] = 1
		} else {
			updates["is_default"] = 0
		}
	}

	if id != "" {
		if len(updates) > 0 {
			s.db.Table("characters").Where("id = ?", id).Updates(updates)
		}
		return map[string]interface{}{"id": id, "saved": true}
	}

	// Create new character
	newId := uuid.New().String()
	c := map[string]interface{}{
		"id": newId, "name": name, "description": desc,
		"status": "enabled", "personality_config": "{}", "chat_style_config": "{}", "scene_rules": "{}",
	}
	for k, v := range updates {
		c[k] = v
	}
	s.db.Table("characters").Create(c)
	return map[string]interface{}{"id": newId, "saved": true}
}
