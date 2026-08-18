// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	List(includeDisabled bool) ([]Character, error)
	FindByID(id string) (*Character, error)
	Create(c *Character) error
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
	SetActive(id string) error
	ListTemplates() ([]CharacterTemplate, error)
	FindTemplateByID(id string) (*CharacterTemplate, error)
	GetActive() (*Character, error)
	GetRuntimeProfile(id string) (*RoleRuntimeProfile, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(includeDisabled bool) ([]Character, error) {
	var chars []Character
	q := r.db.Where("deleted_at IS NULL").Order("sort_order, created_at")
	if !includeDisabled {
		q = q.Where("status = ?", "enabled")
	}
	err := q.Find(&chars).Error
	if chars == nil {
		chars = []Character{}
	}
	return chars, err
}

func (r *repository) FindByID(id string) (*Character, error) {
	var c Character
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&c).Error
	return &c, err
}

func (r *repository) Create(c *Character) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return r.db.Create(c).Error
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	return r.db.Model(&Character{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&Character{}).Error
}

func (r *repository) SetActive(id string) error {
	r.db.Model(&Character{}).Where("is_active = 1 AND deleted_at IS NULL").Update("is_active", 0)
	return r.db.Model(&Character{}).Where("id = ? AND deleted_at IS NULL", id).Update("is_active", 1).Error
}

func (r *repository) FindTemplateByID(id string) (*CharacterTemplate, error) {
	var t CharacterTemplate
	err := r.db.Where("id = ?", id).First(&t).Error
	return &t, err
}

func (r *repository) ListTemplates() ([]CharacterTemplate, error) {
	var templates []CharacterTemplate
	err := r.db.Find(&templates).Error
	if templates == nil {
		templates = []CharacterTemplate{}
	}
	return templates, err
}

func (r *repository) GetActive() (*Character, error) {
	var c Character
	err := r.db.Where("is_active = 1 AND deleted_at IS NULL").First(&c).Error
	return &c, err
}

func (r *repository) GetRuntimeProfile(id string) (*RoleRuntimeProfile, error) {
	var c Character
	var err error
	if strings.TrimSpace(id) != "" {
		err = r.db.Where("id = ? AND status = ? AND deleted_at IS NULL", id, "enabled").First(&c).Error
	} else {
		err = r.db.Where("is_default = 1 AND status = ? AND deleted_at IS NULL", "enabled").Limit(1).First(&c).Error
		if err != nil {
			err = r.db.Where("status = ? AND deleted_at IS NULL", "enabled").Order("sort_order, created_at").Limit(1).First(&c).Error
		}
	}
	if err != nil {
		if strings.TrimSpace(id) != "" {
			log.Printf("[RoleRuntimeProfile] requested character %s not found or disabled, trying fallback", id)
			err = r.db.Where("is_default = 1 AND status = ? AND deleted_at IS NULL", "enabled").Limit(1).First(&c).Error
			if err != nil {
				err = r.db.Where("status = ? AND deleted_at IS NULL", "enabled").Order("sort_order, created_at").Limit(1).First(&c).Error
			}
		}
		if err != nil {
			return nil, err
		}
	}
	diagnostics := []string{}
	personalityConfig := parseRuntimeJSON(c.ID, "personality_config", c.PersonalityConfig, &diagnostics)
	chatStyleConfig := parseRuntimeJSON(c.ID, "chat_style_config", c.ChatStyleConfig, &diagnostics)
	sceneRules := parseRuntimeJSON(c.ID, "scene_rules", c.SceneRules, &diagnostics)
	for _, item := range diagnostics {
		log.Printf("[RoleRuntimeProfile] %s", item)
	}
	return &RoleRuntimeProfile{
		CharacterID:         c.ID,
		Name:                c.Name,
		Identity:            c.Identity,
		Personality:         c.Personality,
		SpeakingStyle:       c.SpeakingStyle,
		RelationshipStyle:   c.RelationshipStyle,
		CharacterBase:       c.CharacterBase,
		BoundaryRules:       c.BoundaryRules,
		PersonalitySliders:  c.PersonalitySliders,
		BasePrompt:          c.BasePrompt,
		GeneratedPrompt:     c.GeneratedPrompt,
		PersonalityConfig:   personalityConfig,
		ChatStyleConfig:     chatStyleConfig,
		SceneRules:          sceneRules,
		Gender:              c.Gender,
		GenderLabel:         c.GenderLabel,
		Pronoun:             c.Pronoun,
		SelfReference:       c.SelfReference,
		UserAddressingStyle: c.UserAddressingStyle,
		GenderExpression:    c.GenderExpression,
		LifeIdentity:        c.LifeIdentity,
		Diagnostics:         diagnostics,
	}, nil
}

func parseRuntimeJSON(characterID, field, raw string, diagnostics *[]string) map[string]interface{} {
	value := strings.TrimSpace(raw)
	if value == "" {
		*diagnostics = append(*diagnostics, characterID+" "+field+" missing, using runtime-profile-v1 default")
		return defaultRuntimeJSON()
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		*diagnostics = append(*diagnostics, characterID+" "+field+" invalid, using runtime-profile-v1 default: "+err.Error())
		return defaultRuntimeJSON()
	}
	if decoded == nil {
		*diagnostics = append(*diagnostics, characterID+" "+field+" empty, using runtime-profile-v1 default")
		return defaultRuntimeJSON()
	}
	if _, ok := decoded["version"]; !ok {
		decoded["version"] = "runtime-profile-v1"
	}
	return decoded
}

func defaultRuntimeJSON() map[string]interface{} {
	return map[string]interface{}{"version": "runtime-profile-v1"}
}
