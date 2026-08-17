// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type pluginResourceAttachment struct {
	Handle         string `gorm:"primaryKey"`
	ExtensionID    string `gorm:"index"`
	ContributionID string `gorm:"index"`
	Revision       int
	Definition     string
	CreatedAt      int64
}

func (pluginResourceAttachment) TableName() string { return "plugin_resource_attachments" }

type pluginActionAttachment struct {
	Handle         string `gorm:"primaryKey"`
	ExtensionID    string `gorm:"index"`
	ContributionID string `gorm:"index"`
	Revision       int
	TargetJSON     string
	Definition     string
	CreatedAt      int64
}

func (pluginActionAttachment) TableName() string { return "plugin_action_attachments" }

type pluginRuntimeAttachment struct {
	Handle         string `gorm:"primaryKey"`
	ExtensionID    string `gorm:"index"`
	ContributionID string `gorm:"index"`
	Revision       int
	Definition     string
	CreatedAt      int64
}

func (pluginRuntimeAttachment) TableName() string { return "plugin_runtime_attachments" }

type pluginWindowAttachment struct {
	ExtensionID    string `gorm:"primaryKey"`
	ContributionID string `gorm:"primaryKey"`
	Definition     string
	CreatedAt      int64
}

func (pluginWindowAttachment) TableName() string { return "plugin_window_attachments" }

type sqliteResourcePort struct {
	mu sync.RWMutex
	db *gorm.DB
}

func NewSQLiteResourcePort(db *gorm.DB) ExistingPetResourcePort {
	return &sqliteResourcePort{db: db}
}

func (p *sqliteResourcePort) AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	handle := uuid.New().String()
	defJSON := encodeDefinition(definition)
	now := timestampMs()
	rec := pluginResourceAttachment{Handle: handle, ExtensionID: extensionID, ContributionID: contributionID, Revision: revision, Definition: defJSON, CreatedAt: now}
	if err := p.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return "", fmt.Errorf("create resource attachment: %w", err)
	}
	return handle, nil
}

func (p *sqliteResourcePort) DetachPluginResource(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	return p.db.WithContext(ctx).Delete(&pluginResourceAttachment{}, "handle = ?", handle).Error
}

func (p *sqliteResourcePort) ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error) {
	var rows []pluginResourceAttachment
	if err := p.db.WithContext(ctx).Where("extension_id = ?", extensionID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list resource attachments: %w", err)
	}
	result := make([]ExistingPetResourceBinding, 0, len(rows))
	for _, r := range rows {
		result = append(result, ExistingPetResourceBinding{Handle: r.Handle, ExtensionID: r.ExtensionID, ContributionID: r.ContributionID, Revision: r.Revision, Definition: decodeDefinition(r.Definition)})
	}
	return result, nil
}

func (p *sqliteResourcePort) RebuildFromExisting() error {
	return nil
}

type sqliteActionPort struct {
	mu sync.RWMutex
	db *gorm.DB
}

func NewSQLiteActionPort(db *gorm.DB) ExistingPetActionPort {
	return &sqliteActionPort{db: db}
}

func (p *sqliteActionPort) AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	handle := uuid.New().String()
	defJSON := encodeDefinition(definition)
	targetJSON := encodeDefinition(map[string]any{"installationID": target.InstallationID, "deviceID": target.DeviceID, "userID": target.UserID})
	now := timestampMs()
	rec := pluginActionAttachment{Handle: handle, ExtensionID: extensionID, ContributionID: contributionID, Revision: revision, TargetJSON: targetJSON, Definition: defJSON, CreatedAt: now}
	if err := p.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return "", fmt.Errorf("create action attachment: %w", err)
	}
	return handle, nil
}

func (p *sqliteActionPort) DetachPluginAction(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	return p.db.WithContext(ctx).Delete(&pluginActionAttachment{}, "handle = ?", handle).Error
}

func (p *sqliteActionPort) ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error) {
	var rows []pluginActionAttachment
	if err := p.db.WithContext(ctx).Where("extension_id = ?", extensionID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list action attachments: %w", err)
	}
	result := make([]ExistingPetActionBinding, 0, len(rows))
	for _, r := range rows {
		result = append(result, ExistingPetActionBinding{Handle: r.Handle, ExtensionID: r.ExtensionID, ContributionID: r.ContributionID, Revision: r.Revision})
	}
	return result, nil
}

func (p *sqliteActionPort) RebuildFromExisting() error {
	return nil
}

type sqliteRuntimePort struct {
	mu sync.RWMutex
	db *gorm.DB
}

func NewSQLiteRuntimePort(db *gorm.DB) ExistingPetRuntimePort {
	return &sqliteRuntimePort{db: db}
}

func (p *sqliteRuntimePort) AttachPluginRuntime(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	handle := uuid.New().String()
	defJSON := encodeDefinition(definition)
	now := timestampMs()
	rec := pluginRuntimeAttachment{Handle: handle, ExtensionID: extensionID, ContributionID: contributionID, Revision: revision, Definition: defJSON, CreatedAt: now}
	if err := p.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return "", fmt.Errorf("create runtime attachment: %w", err)
	}
	return handle, nil
}

func (p *sqliteRuntimePort) DetachPluginRuntime(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	return p.db.WithContext(ctx).Delete(&pluginRuntimeAttachment{}, "handle = ?", handle).Error
}

func (p *sqliteRuntimePort) ListAttachedRuntimes(ctx context.Context, extensionID string) ([]ExistingPetRuntimeBinding, error) {
	var rows []pluginRuntimeAttachment
	if err := p.db.WithContext(ctx).Where("extension_id = ?", extensionID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list runtime attachments: %w", err)
	}
	result := make([]ExistingPetRuntimeBinding, 0, len(rows))
	for _, r := range rows {
		result = append(result, ExistingPetRuntimeBinding{Handle: r.Handle, ExtensionID: r.ExtensionID, ContributionID: r.ContributionID, Revision: r.Revision})
	}
	return result, nil
}

func (p *sqliteRuntimePort) RebuildFromExisting() error {
	return nil
}

type sqliteWindowPort struct {
	mu sync.RWMutex
	db *gorm.DB
}

func NewSQLiteWindowPort(db *gorm.DB) ExistingPetWindowPort {
	return &sqliteWindowPort{db: db}
}

func (p *sqliteWindowPort) PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error {
	if extensionID == "" || contributionID == "" {
		return fmt.Errorf("extensionID and contributionID required")
	}
	defJSON := encodeDefinition(definition)
	now := timestampMs()
	rec := pluginWindowAttachment{ExtensionID: extensionID, ContributionID: contributionID, Definition: defJSON, CreatedAt: now}
	return p.db.WithContext(ctx).Save(&rec).Error
}

func (p *sqliteWindowPort) RetractFloatingWindowContribution(ctx context.Context, extensionID, contributionID string) error {
	if extensionID == "" || contributionID == "" {
		return fmt.Errorf("extensionID and contributionID required")
	}
	return p.db.WithContext(ctx).Delete(&pluginWindowAttachment{}, "extension_id = ? AND contribution_id = ?", extensionID, contributionID).Error
}

func (p *sqliteWindowPort) ListAttachedWindows(ctx context.Context, extensionID string) ([]ExistingPetWindowBinding, error) {
	var rows []pluginWindowAttachment
	if err := p.db.WithContext(ctx).Where("extension_id = ?", extensionID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list window attachments: %w", err)
	}
	result := make([]ExistingPetWindowBinding, 0, len(rows))
	for _, r := range rows {
		result = append(result, ExistingPetWindowBinding{ExtensionID: r.ExtensionID, ContributionID: r.ContributionID})
	}
	return result, nil
}

func (p *sqliteWindowPort) RebuildFromExisting() error {
	return nil
}

func encodeDefinition(def map[string]any) string {
	if def == nil {
		return "{}"
	}
	buf, err := json.Marshal(def)
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func decodeDefinition(s string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func timestampMs() int64 {
	return time.Now().UnixMilli()
}
