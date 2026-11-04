package agentplatform

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AcquireResourceLeaseInput struct {
	AgentID      string `json:"agentId"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	TaskID       string `json:"taskId"`
	Mode         string `json:"mode"`
	Fingerprint  string `json:"fingerprint"`
	TTLSeconds   int    `json:"ttlSeconds"`
}

func (s *Service) AcquireResourceLease(ctx context.Context, spaceID string, input AcquireResourceLeaseInput) (ResourceLease, error) {
	if strings.TrimSpace(input.ResourceType) == "" || strings.TrimSpace(input.ResourceID) == "" || strings.TrimSpace(input.AgentID) == "" {
		return ResourceLease{}, errors.New("资源类型、资源 ID 和 Agent ID 不能为空")
	}
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "exclusive"
	}
	if mode != "exclusive" && mode != "shared" {
		return ResourceLease{}, errors.New("资源租约模式必须是 exclusive 或 shared")
	}
	if input.TTLSeconds <= 0 {
		input.TTLSeconds = 900
	}
	if input.TTLSeconds > 86400 {
		input.TTLSeconds = 86400
	}
	now := time.Now().UTC()
	lease := ResourceLease{
		ID: newID("resourcelease"), SpaceID: spaceID, ResourceType: input.ResourceType,
		ResourceID: input.ResourceID, HolderAgentID: input.AgentID, TaskID: input.TaskID,
		Mode: mode, Fingerprint: input.Fingerprint, Status: "active",
		ExpiresAt: now.Add(time.Duration(input.TTLSeconds) * time.Second), CreatedAt: now,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ResourceLease{}).Where("space_id = ? AND expires_at <= ? AND status = ?", spaceID, now, "active").Updates(map[string]any{"status": "expired", "released_at": now}).Error; err != nil {
			return fmt.Errorf("清理过期资源租约: %w", err)
		}
		var conflicts []ResourceLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ? AND resource_type = ? AND resource_id = ? AND status = ? AND expires_at > ? AND holder_agent_id <> ?", spaceID, input.ResourceType, input.ResourceID, "active", now, input.AgentID).Find(&conflicts).Error; err != nil {
			return fmt.Errorf("检查资源冲突: %w", err)
		}
		for _, conflict := range conflicts {
			if mode == "exclusive" || conflict.Mode == "exclusive" {
				return fmt.Errorf("资源 %s/%s 已被 Agent %s 锁定", input.ResourceType, input.ResourceID, conflict.HolderAgentID)
			}
		}
		if err := tx.Create(&lease).Error; err != nil {
			return fmt.Errorf("创建资源租约: %w", err)
		}
		return nil
	})
	if err != nil {
		return ResourceLease{}, err
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, AgentID: input.AgentID, Kind: "resource.lease", State: "acquired", Title: "资源租约已获取", Detail: input.ResourceType + "/" + input.ResourceID}); err != nil {
		return ResourceLease{}, err
	}
	return lease, nil
}

func (s *Service) ReleaseResourceLease(ctx context.Context, spaceID, leaseID, agentID string) (ResourceLease, error) {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&ResourceLease{}).Where("id = ? AND space_id = ? AND holder_agent_id = ? AND status = ?", leaseID, spaceID, agentID, "active").Updates(map[string]any{"status": "released", "released_at": now})
	if result.Error != nil {
		return ResourceLease{}, fmt.Errorf("释放资源租约: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ResourceLease{}, errors.New("资源租约不存在或不属于当前 Agent")
	}
	var lease ResourceLease
	if err := s.db.WithContext(ctx).Where("id = ?", leaseID).First(&lease).Error; err != nil {
		return ResourceLease{}, fmt.Errorf("读取资源租约: %w", err)
	}
	return lease, nil
}

func (s *Service) ListResourceLeases(ctx context.Context, spaceID, resourceType, resourceID string, activeOnly bool) ([]ResourceLease, error) {
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(resourceType) != "" {
		query = query.Where("resource_type = ?", resourceType)
	}
	if strings.TrimSpace(resourceID) != "" {
		query = query.Where("resource_id = ?", resourceID)
	}
	if activeOnly {
		query = query.Where("status = ? AND expires_at > ?", "active", time.Now().UTC())
	}
	leases := make([]ResourceLease, 0)
	if err := query.Order("created_at DESC").Find(&leases).Error; err != nil {
		return nil, fmt.Errorf("读取资源租约: %w", err)
	}
	return leases, nil
}

func numericValue(value any) int64 {
	switch current := value.(type) {
	case int64:
		return current
	case int:
		return int64(current)
	case float64:
		return int64(current)
	case []byte:
		parsed, _ := strconv.ParseInt(string(current), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(current, 10, 64)
		return parsed
	default:
		return 0
	}
}
