// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package user

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	FindByUsername(username string) (*AuthUser, error)
	FindByID(id int) (*AuthUser, error)
	HasAdmin() (bool, error)
	Create(user *AuthUser) error
	UpdateLoginTime(id int) error
	UpdatePassword(id int, hash string) error
	CreateSession(session *AuthSession) error
	ListSessions(userID int) ([]SessionResponse, error)
	DeleteSession(id int) error
	DeleteOtherSessions(userID int, currentHash string) error
	DeleteSessionByHash(hash string) error
	GetSetting(key string) string
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) FindByUsername(username string) (*AuthUser, error) {
	var user AuthUser
	err := r.db.Where("username = ? AND is_active = 1", username).First(&user).Error
	return &user, err
}

func (r *repository) FindByID(id int) (*AuthUser, error) {
	var user AuthUser
	err := r.db.First(&user, id).Error
	return &user, err
}

func (r *repository) HasAdmin() (bool, error) {
	var count int64
	err := r.db.Model(&AuthUser{}).Count(&count).Error
	return count > 0, err
}

func (r *repository) Create(user *AuthUser) error {
	return r.db.Create(user).Error
}

func (r *repository) UpdateLoginTime(id int) error {
	return r.db.Model(&AuthUser{}).Where("id = ?", id).Update("last_login_at", gorm.Expr("NOW()")).Error
}

func (r *repository) UpdatePassword(id int, hash string) error {
	return r.db.Model(&AuthUser{}).Where("id = ?", id).Update("password_hash", hash).Error
}

func (r *repository) CreateSession(session *AuthSession) error {
	return r.db.Create(session).Error
}

func (r *repository) ListSessions(userID int) ([]SessionResponse, error) {
	var sessions []AuthSession
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	res := make([]SessionResponse, len(sessions))
	for i, s := range sessions {
		res[i] = SessionResponse{
			ID: s.ID, DeviceName: s.DeviceName, IPAddress: s.IPAddress,
			UserAgent:    s.UserAgent,
			LastActiveAt: s.LastActiveAt,
			CreatedAt:    s.CreatedAt,
		}
		if s.ExpiresAt != nil {
			res[i].ExpiresAt = *s.ExpiresAt
		}
	}
	return res, nil
}

func (r *repository) DeleteSession(id int) error {
	return r.db.Delete(&AuthSession{}, id).Error
}

func (r *repository) DeleteOtherSessions(userID int, currentHash string) error {
	return r.db.Where("user_id = ? AND token_hash != ?", userID, currentHash).Delete(&AuthSession{}).Error
}

func (r *repository) DeleteSessionByHash(hash string) error {
	return r.db.Where("token_hash = ?", hash).Delete(&AuthSession{}).Error
}

func (r *repository) GetSetting(key string) string {
	var value string
	r.db.Table("app_settings").Select("value").Where("key = ?", key).Scan(&value)
	return value
}
