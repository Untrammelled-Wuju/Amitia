// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

func (s *service) GetAuditActions() []string {
	return []string{"login", "logout", "password_change", "character_update", "model_update", "rule_update", "memory_update"}
}

func (s *service) GetAuditSettings() map[string]interface{} {
	enabled := s.getAppSetting("audit_enabled") != "false"
	return map[string]interface{}{"enabled": enabled, "retentionDays": 90, "logActions": true}
}

func (s *service) UpdateAuditSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		if v {
			s.setAppSetting("audit_enabled", "true")
		} else {
			s.setAppSetting("audit_enabled", "false")
		}
	}
	return s.GetAuditSettings()
}

func (s *service) GetAuditStats() map[string]interface{} {
	var total int64
	s.db.Table("audit_logs").Count(&total)
	return map[string]interface{}{"total": total}
}
