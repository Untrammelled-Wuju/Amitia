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
	GetUserFactSummary(userID string) ([]UserProfile, error)
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
	var existing UserProfile
	err := r.db.Where("user_id = ? AND category = ? AND attribute_name = ?",
		profile.UserID, profile.Category, profile.AttributeName).First(&existing).Error
	if err != nil {
		if profile.ID == "" {
			profile.ID = uuid.New().String()
		}
		createErr := r.db.Create(profile).Error
		return profile, createErr
	}
	newConfidence := existing.Confidence + 10
	if newConfidence > 100 {
		newConfidence = 100
	}
	updates := map[string]interface{}{
		"attribute_value": profile.AttributeValue,
		"confidence":      newConfidence,
		"source_conv_id":  profile.SourceConvID,
	}
	if updateErr := r.db.Model(&existing).Updates(updates).Error; updateErr != nil {
		log.Printf("[Profile] UpsertConfidence update error: %v", updateErr)
	}
	existing.AttributeValue = profile.AttributeValue
	existing.Confidence = newConfidence
	existing.SourceConvID = profile.SourceConvID
	return &existing, nil
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	return r.db.Model(&UserProfile{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&UserProfile{}).Error
}

func (r *repository) GetByUserID(userID string) ([]UserProfile, error) {
	var items []UserProfile
	err := r.db.Where("user_id = ?", userID).Order("confidence DESC").Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	return items, err
}

func (r *repository) GetUserFactSummary(userID string) ([]UserProfile, error) {
	var items []UserProfile
	err := r.db.Where("user_id = ? AND confidence >= 50", userID).Order("confidence DESC").Limit(20).Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	return items, err
}
