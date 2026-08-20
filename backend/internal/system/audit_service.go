// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strconv"
	"strings"
)

type auditLogRecord struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
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
	logs := make([]auditLogRecord, 0)
	s.db.Table("audit_logs").Order("time DESC").Limit(limit).Find(&logs)
	return logs
}

func (s *service) ClearAuditLogs() int64 {
	result := s.db.Exec("DELETE FROM audit_logs")
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
	s.db.Table("audit_logs").Count(&total)
	return map[string]interface{}{"total": total}
}
