// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import "time"

const (
	ProductDesktop = "desktop"
	ProductAndroid = "android"

	ChannelStable = "stable"
	ChannelBeta   = "beta"
	ChannelAlpha  = "alpha"

	StatusDraft     = "draft"
	StatusReady     = "ready"
	StatusPublished = "published"
	StatusPaused    = "paused"
	StatusFailed    = "failed"

	RoleAdmin     = "admin"
	RolePublisher = "publisher"
)

type AdminUser struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;not null;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:512;not null" json:"-"`
	Role         string    `gorm:"size:32;not null;index" json:"role"`
	Active       bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AdminSession struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"not null;index"`
	TokenHash  string    `gorm:"size:64;not null;uniqueIndex"`
	CSRFHash   string    `gorm:"size:64;not null"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	LastSeenAt time.Time `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

type UpdateRelease struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Product           string     `gorm:"size:16;not null;uniqueIndex:idx_update_release_version" json:"product"`
	Channel           string     `gorm:"size:16;not null;uniqueIndex:idx_update_release_version" json:"channel"`
	Version           string     `gorm:"size:64;not null;uniqueIndex:idx_update_release_version" json:"version"`
	VersionCode       int64      `gorm:"not null;default:0" json:"versionCode"`
	Status            string     `gorm:"size:24;not null;index" json:"status"`
	ReleaseName       string     `gorm:"size:160" json:"releaseName"`
	ReleaseNotes      string     `gorm:"type:text" json:"releaseNotes"`
	Mandatory         bool       `gorm:"not null;default:false" json:"mandatory"`
	MinVersionCode    int64      `gorm:"not null;default:0" json:"minSupportedVersionCode"`
	RolloutPercentage int        `gorm:"not null;default:100" json:"rolloutPercentage"`
	ABI               string     `gorm:"size:32" json:"abi"`
	PackageName       string     `gorm:"size:160" json:"packageName"`
	PublishPath       string     `gorm:"size:512" json:"publishPath"`
	CreatedBy         uint       `gorm:"not null;index" json:"createdBy"`
	PublishedAt       *time.Time `json:"publishedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type UpdateReleaseArtifact struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ReleaseID    uint      `gorm:"not null;index;uniqueIndex:idx_release_artifact_kind" json:"releaseId"`
	Kind         string    `gorm:"size:32;not null;uniqueIndex:idx_release_artifact_kind" json:"kind"`
	OriginalName string    `gorm:"size:512;not null" json:"originalName"`
	StoredPath   string    `gorm:"size:1024;not null" json:"-"`
	Size         int64     `gorm:"not null" json:"size"`
	SHA256       string    `gorm:"size:64;not null" json:"sha256"`
	SHA512       string    `gorm:"size:128;not null" json:"sha512"`
	SHA512Base64 string    `gorm:"size:256;not null" json:"sha512Base64"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AdminAuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ActorID   uint      `gorm:"index" json:"actorId"`
	ActorName string    `gorm:"size:64" json:"actorName"`
	Action    string    `gorm:"size:64;not null;index" json:"action"`
	Product   string    `gorm:"size:16;index" json:"product"`
	Channel   string    `gorm:"size:16;index" json:"channel"`
	ReleaseID uint      `gorm:"index" json:"releaseId"`
	Target    string    `gorm:"size:512" json:"target"`
	Result    string    `gorm:"size:24;not null;index" json:"result"`
	Message   string    `gorm:"type:text" json:"message"`
	CreatedAt time.Time `gorm:"not null;index" json:"createdAt"`
}

type releaseResponse struct {
	UpdateRelease
	Artifacts []UpdateReleaseArtifact `json:"artifacts"`
}
