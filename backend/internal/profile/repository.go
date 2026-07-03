// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
	"log"
)

type Repository interface {
	List(q ProfileListQuery) ([]UserProfile, int64, error)
	FindByID(id string) (*UserProfile, error)
	UpsertConfidence(profile *UserProfile) (*UserProfile, error)
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
	GetByUserID(userID string) ([]UserProfile, error)
	GetScopedByUserID(userID, characterID string) ([]UserProfile, error)
	GetUserFactSummary(userID string, characterID ...string) ([]UserProfile, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q ProfileListQuery) ([]UserProfile, int64, error) {
	query := r.db.Model(&UserProfile{})
	if q.UserID != "" {
		query = query.Where("user_id = ?", q.UserID)
	}
	if q.CharacterID != "" {
		query = query.Where("character_id = ?", q.CharacterID)
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}
	var items []UserProfile
	err := query.Order("confidence DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	return items, total, err
}

func (r *repository) FindByID(id string) (*UserProfile, error) {
	var p UserProfile
	err := r.db.Where("id = ?", id).First(&p).Error
	return &p, err
}

func (r *repository) UpsertConfidence(profile *UserProfile) (*UserProfile, error) {
	profile.Confidence = clampProfileConfidence(profile.Confidence)
	clampedConfidence := profile.Confidence
	var existing UserProfile
	err := r.db.Where("user_id = ? AND character_id = ? AND category = ? AND attribute_name = ?",
		profile.UserID, profile.CharacterID, profile.Category, profile.AttributeName).First(&existing).Error
	if err != nil {
		if profile.ID == "" {
			profile.ID = uuid.New().String()
		}
		createErr := r.db.Select("*").Create(profile).Error
		if createErr == nil && profile.Confidence != clampedConfidence {
			profile.Confidence = clampedConfidence
			createErr = r.db.Model(&UserProfile{}).Where("id = ?", profile.ID).Update("confidence", clampedConfidence).Error
		}
		return profile, createErr
	}
	var newConfidence int
	if existing.AttributeValue != profile.AttributeValue {
		newConfidence = existing.Confidence / 2
		if newConfidence < 5 {
			newConfidence = 5
		}
	} else if hasIndependentProfileEvidence(existing, *profile) {
		increment := 10 - existing.Confidence/10
		if increment < 2 {
			increment = 2
		}
		newConfidence = existing.Confidence + increment
		if newConfidence > 100 {
			newConfidence = 100
		}
	} else {
		newConfidence = existing.Confidence
	}
	newConfidence = clampProfileConfidence(newConfidence)
	updates := map[string]interface{}{
		"attribute_value": profile.AttributeValue,
		"confidence":      newConfidence,
		"source":          profile.Source,
		"source_conv_id":  profile.SourceConvID,
	}
	if updateErr := r.db.Model(&existing).Updates(updates).Error; updateErr != nil {
		log.Printf("[Profile] UpsertConfidence update error: %v", updateErr)
	}
	existing.AttributeValue = profile.AttributeValue
	existing.Confidence = newConfidence
	existing.Source = profile.Source
	existing.SourceConvID = profile.SourceConvID
	return &existing, nil
}

func hasIndependentProfileEvidence(existing, incoming UserProfile) bool {
	if incoming.SourceConvID != "" {
		return incoming.SourceConvID != existing.SourceConvID
	}
	if existing.SourceConvID != "" {
		return false
	}
	return incoming.Source != "" && existing.Source != "" && incoming.Source != existing.Source
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	if confidence, ok := updates["confidence"]; ok {
		updates["confidence"] = clampProfileConfidenceValue(confidence)
	}
	return r.db.Model(&UserProfile{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&UserProfile{}).Error
}

func (r *repository) GetByUserID(userID string) ([]UserProfile, error) {
	var items []UserProfile
	err := r.db.Where("user_id = ? AND character_id = ''", userID).Order("confidence DESC").Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	return items, err
}

func (r *repository) GetScopedByUserID(userID, characterID string) ([]UserProfile, error) {
	return r.getScopedProfiles(userID, characterID, "confidence DESC", 0, false)
}

func (r *repository) GetUserFactSummary(userID string, characterID ...string) ([]UserProfile, error) {
	scope := ""
	if len(characterID) > 0 {
		scope = characterID[0]
	}
	return r.getScopedProfiles(userID, scope, "confidence DESC", 20, true)
}

func (r *repository) getScopedProfiles(userID, characterID, order string, limit int, onlyFacts bool) ([]UserProfile, error) {
	var items []UserProfile
	query := r.db.Where("user_id = ?", userID)
	if characterID != "" {
		query = query.Where("character_id IN ?", []string{characterID, ""})
	} else {
		query = query.Where("character_id = ?", "")
	}
	if onlyFacts {
		query = query.Where("confidence >= 50")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order(order).Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	if err != nil || characterID == "" {
		return items, err
	}
	return preferCharacterProfiles(items, characterID), nil
}

func preferCharacterProfiles(items []UserProfile, characterID string) []UserProfile {
	selected := make(map[string]UserProfile, len(items))
	for _, item := range items {
		key := item.Category + "\x00" + item.AttributeName
		existing, ok := selected[key]
		if !ok || existing.CharacterID == "" && item.CharacterID == characterID {
			selected[key] = item
		}
	}
	result := make([]UserProfile, 0, len(selected))
	for _, item := range items {
		key := item.Category + "\x00" + item.AttributeName
		selectedItem, ok := selected[key]
		if ok && selectedItem.ID == item.ID {
			result = append(result, item)
			delete(selected, key)
		}
	}
	return result
}
