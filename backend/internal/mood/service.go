// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	List() []map[string]interface{}
	GetByConversation(id string) []map[string]interface{}
	Delete(id int) bool
	DeleteByConversation(id string) bool
}

type service struct {
	db *gorm.DB
}

func NewService(ctx *app.AppContext) Service { return &service{db: ctx.DB} }

func (s *service) List() []map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Table("moods").Order("created_at DESC").Limit(50).Find(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return moods
}

func (s *service) GetByConversation(id string) []map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Table("moods").Where("conversation_id = ?", id).Order("created_at DESC").Find(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return moods
}

func (s *service) Delete(id int) bool {
	return s.db.Exec("DELETE FROM moods WHERE id = ?", id).RowsAffected > 0
}

func (s *service) DeleteByConversation(id string) bool {
	return s.db.Exec("DELETE FROM moods WHERE conversation_id = ?", id).RowsAffected > 0
}
