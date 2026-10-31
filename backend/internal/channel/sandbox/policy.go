package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	amitialog "log"
)

type Policy struct {
	SpaceID      string `gorm:"column:space_id;primaryKey" json:"spaceId"`
	ChannelID    string `gorm:"column:channel_id;primaryKey" json:"channelId"`
	Enabled      bool   `gorm:"column:enabled" json:"enabled"`
	AllowNetwork bool   `gorm:"column:allow_network" json:"allowNetwork"`
	WorkspaceID  string `gorm:"column:workspace_id" json:"workspaceId"`
	SandboxRoot  string `gorm:"column:sandbox_root" json:"sandboxRoot"`
	Revision     int64  `gorm:"column:revision" json:"revision"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at" json:"updatedAt"`
}

func (Policy) TableName() string { return "channel_sandbox_policies" }

type Service struct {
	db      *gorm.DB
	dataDir string
}

func NewService(db *gorm.DB, dataDir string) *Service {
	return &Service{db: db, dataDir: strings.TrimSpace(dataDir)}
}

func (s *Service) Get(spaceID, channelID string) (Policy, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return Policy{}, errors.New("渠道标识不能为空")
	}
	policy := Policy{SpaceID: strings.TrimSpace(spaceID), ChannelID: channelID, AllowNetwork: true, Revision: 1}
	if s == nil || s.db == nil {
		return policy, nil
	}
	var stored Policy
	query := s.db.Where("channel_id = ?", channelID)
	if policy.SpaceID != "" {
		query = query.Where("space_id = ?", policy.SpaceID)
	}
	result := query.Order("updated_at DESC").Limit(1).Find(&stored)
	if result.Error != nil {
		return Policy{}, result.Error
	}
	if result.RowsAffected == 0 {
		return policy, nil
	}
	return stored, nil
}

func (s *Service) Set(spaceID, channelID string, enabled, allowNetwork bool) (Policy, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return Policy{}, errors.New("渠道标识不能为空")
	}
	if s == nil || s.db == nil {
		return Policy{}, errors.New("沙箱策略存储不可用")
	}
	spaceID = strings.TrimSpace(spaceID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	root := ""
	workspaceID := ""
	if enabled {
		root, workspaceID = s.ensureSandboxRoot(channelID)
	}
	return s.upsert(Policy{
		SpaceID:      spaceID,
		ChannelID:    channelID,
		Enabled:      enabled,
		AllowNetwork: allowNetwork,
		WorkspaceID:  workspaceID,
		SandboxRoot:  root,
		UpdatedAt:    now,
	})
}

func (s *Service) upsert(policy Policy) (Policy, error) {
	var existing Policy
	result := s.db.Where("space_id = ? AND channel_id = ?", policy.SpaceID, policy.ChannelID).Limit(1).Find(&existing)
	if result.Error != nil {
		return Policy{}, result.Error
	}
	if result.RowsAffected == 0 {
		policy.Revision = 1
		policy.CreatedAt = policy.UpdatedAt
		if err := s.db.Create(&policy).Error; err != nil {
			return Policy{}, err
		}
		return policy, nil
	}
	policy.Revision = existing.Revision + 1
	policy.CreatedAt = existing.CreatedAt
	if err := s.db.Model(&Policy{}).
		Where("space_id = ? AND channel_id = ?", policy.SpaceID, policy.ChannelID).
		Updates(map[string]any{
			"enabled":       policy.Enabled,
			"allow_network": policy.AllowNetwork,
			"workspace_id":  policy.WorkspaceID,
			"sandbox_root":  policy.SandboxRoot,
			"revision":      policy.Revision,
			"updated_at":    policy.UpdatedAt,
		}).Error; err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (s *Service) ensureSandboxRoot(channelID string) (root, workspaceID string) {
	base := s.dataDir
	if base == "" {
		base = os.TempDir()
	}
	safe := SanitizeChannel(channelID)
	root = filepath.Join(base, "sandboxes", "channels", safe)
	if err := os.MkdirAll(root, 0o700); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	workspaceID = "sandbox-" + safe
	return root, workspaceID
}

func SanitizeChannel(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_", " ", "_")
	safe := strings.Trim(replacer.Replace(strings.TrimSpace(value)), "_")
	if safe == "" {
		return "channel"
	}
	return safe
}

func (p Policy) RootOrDefault() (string, error) {
	root := strings.TrimSpace(p.SandboxRoot)
	if root == "" {
		return "", fmt.Errorf("沙箱目录未初始化")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("沙箱目录不可用: %s", root)
	}
	return root, nil
}
