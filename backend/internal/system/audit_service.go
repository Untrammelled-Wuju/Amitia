// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/securityaudit"
	"github.com/u-ai/backend/internal/spaceidentity"
	"gorm.io/gorm"
)

type auditLogRecord struct {
	ID     string `json:"id"`
	Time   string `json:"time"`
	RuleID string `json:"ruleId"`
	Action string `json:"action"`
}

func (s *service) GetAuditActions() []string {
	return []string{"login", "logout", "password_change", "character_update", "model_update", "rule_update", "memory_update"}
}

func (s *service) GetAuditLogs(limit int) []auditLogRecord {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	logs, _ := s.canonicalAuditLogs(s.db, spaceidentity.DefaultSpaceID(), limit)
	return logs
}

func (s *service) ClearAuditLogs() int64 {
	result := s.db.Where("space_id = ?", spaceidentity.DefaultSpaceID()).Delete(&securityaudit.AuditEvent{})
	if result.Error != nil {
		return 0
	}
	return result.RowsAffected
}

func (s *service) GetAuditSettings() map[string]interface{} {
	enabled := s.getAppSetting("audit_enabled") != "false"

	retentionDays := 90
	if raw := strings.TrimSpace(s.getAppSetting("audit_retention_days")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 1 && value <= 3650 {
			retentionDays = value
		}
	}

	logActions := true
	if raw := strings.TrimSpace(s.getAppSetting("audit_log_actions")); raw != "" {
		logActions = raw != "false"
	}

	return map[string]interface{}{
		"enabled":       enabled,
		"retentionDays": retentionDays,
		"logActions":    logActions,
	}
}

func (s *service) UpdateAuditSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		s.setAppSetting("audit_enabled", strconv.FormatBool(v))
	}
	if v, ok := body["retentionDays"].(float64); ok {
		days := int(v)
		if days < 1 {
			days = 1
		} else if days > 3650 {
			days = 3650
		}
		s.setAppSetting("audit_retention_days", strconv.Itoa(days))
	}
	if v, ok := body["logActions"].(bool); ok {
		s.setAppSetting("audit_log_actions", strconv.FormatBool(v))
	}
	return s.GetAuditSettings()
}

func (s *service) GetAuditStats() map[string]interface{} {
	var total int64
	s.db.Model(&securityaudit.AuditEvent{}).Where("space_id = ?", spaceidentity.DefaultSpaceID()).Count(&total)
	return map[string]interface{}{"total": total}
}

func auditSpace(ctx context.Context) (string, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || strings.TrimSpace(string(actor.SpaceID)) == "" {
		return "", errors.New("审计查询缺少可信空间身份")
	}
	space := string(actor.SpaceID)
	if _, scoped := coordination.FromContext(ctx); actor.PrincipalType == auth.PrincipalTrustedDevice && !scoped {
		return "", errors.New("绑定设备审计查询缺少当前 Core 授权")
	}
	if scope, ok := coordination.FromContext(ctx); ok && scope.SpaceID != space {
		return "", errors.New("审计空间与当前授权不一致")
	}
	return space, nil
}

func (s *service) canonicalAuditLogs(db *gorm.DB, space string, limit int) ([]auditLogRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var events []securityaudit.AuditEvent
	if err := db.Where("space_id = ?", space).Order("occurred_at DESC, event_id DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, err
	}
	logs := make([]auditLogRecord, 0, len(events))
	for _, event := range events {
		logs = append(logs, auditLogRecord{ID: event.EventID, Time: event.OccurredAt, RuleID: event.ReasonCode, Action: event.EventType})
	}
	return logs, nil
}

func (s *service) AuditDataContext(ctx context.Context, operation string, limit int) (interface{}, error) {
	space, err := auditSpace(ctx)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = configwrite.Transaction(s.db.WithContext(ctx), func(tx *gorm.DB) error {
		if operation == "logs" {
			logs, err := s.canonicalAuditLogs(tx, space, limit)
			result = logs
			return err
		}
		if operation != "stats" {
			return errors.New("未知审计查询")
		}
		var total int64
		if err := tx.Model(&securityaudit.AuditEvent{}).Where("space_id = ?", space).Count(&total).Error; err != nil {
			return err
		}
		result = map[string]interface{}{"total": total}
		return nil
	})
	return result, err
}
