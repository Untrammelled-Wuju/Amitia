// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package user

type AuthUser struct {
	ID           int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username     string  `gorm:"column:username;uniqueIndex;size:100;not null" json:"username"`
	PasswordHash string  `gorm:"column:password_hash;size:255;not null" json:"-"`
	Role         string  `gorm:"column:role;size:20;default:admin" json:"role"`
	IsActive     int     `gorm:"column:is_active;default:1" json:"isActive"`
	CreatedAt    string  `gorm:"column:created_at" json:"createdTime"`
	LastLoginAt  *string `gorm:"column:last_login_at" json:"lastLoginTime"`
}

func (AuthUser) TableName() string { return "auth_users" }

type AuthSession struct {
	ID           int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int     `gorm:"column:user_id;not null;index" json:"userId"`
	TokenHash    string  `gorm:"column:token_hash;size:255;not null" json:"-"`
	DeviceName   string  `gorm:"column:device_name;size:50" json:"deviceName"`
	IPAddress    string  `gorm:"column:ip_address;size:50" json:"ipAddress"`
	UserAgent    string  `gorm:"column:user_agent;size:255" json:"userAgent"`
	LastActiveAt string  `gorm:"column:last_active_at" json:"lastActiveTime"`
	ExpiresAt    *string `gorm:"column:expires_at" json:"expiresTime"`
	CreatedAt    string  `gorm:"column:created_at" json:"createdTime"`
}

func (AuthSession) TableName() string { return "auth_sessions" }

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SetupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UserInfoResponse struct {
	ID             int    `json:"id"`
	Username       string `json:"username"`
	Role           string `json:"role"`
	CreatedAt      string `json:"createdTime"`
	LastLoginAt    string `json:"lastLoginTime"`
	JwtExpiryDays  int    `json:"jwtExpiryDays"`
	SessionTimeout int    `json:"sessionTimeoutMinutes"`
	RequireAuth    bool   `json:"requireAuth"`
}

type SessionResponse struct {
	ID           int    `json:"id"`
	DeviceName   string `json:"deviceName"`
	IPAddress    string `json:"ipAddress"`
	UserAgent    string `json:"userAgent"`
	LastActiveAt string `json:"lastActiveTime"`
	ExpiresAt    string `json:"expiresTime"`
	CreatedAt    string `json:"createdTime"`
}

type StatusResponse struct {
	HasAdmin bool `json:"hasAdmin"`
}
