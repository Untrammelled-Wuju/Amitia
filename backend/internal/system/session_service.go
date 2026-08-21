// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/accountsession"
)

func (s *service) GetCurrentSession(userID int64, sessionID string) map[string]interface{} {
	if userID <= 0 || strings.TrimSpace(sessionID) == "" {
		return map[string]interface{}{}
	}
	var session accountsession.Session
	if err := s.db.Where("user_id = ? AND public_id = ?", userID, sessionID).First(&session).Error; err != nil {
		return map[string]interface{}{}
	}
	result := map[string]interface{}{
		"sessionId":  session.PublicID,
		"username":   session.Username,
		"role":       session.Role,
		"status":     session.Status,
		"deviceName": session.DeviceName,
		"ipAddress":  session.IPAddress,
		"userAgent":  session.UserAgent,
		"createdAt":  session.CreatedAt,
	}
	if session.LastActiveAt != nil {
		result["lastActiveAt"] = *session.LastActiveAt
	}
	if session.LastRefreshedAt != nil {
		result["lastRefreshedAt"] = *session.LastRefreshedAt
	}
	if session.ExpiresAt != nil {
		result["expiresAt"] = *session.ExpiresAt
	}
	return result
}

func (s *service) GetLoginHistory(userID int64) []map[string]interface{} {
	if userID <= 0 {
		return []map[string]interface{}{}
	}
	events := make([]accountsession.AuditEvent, 0)
	s.db.Where("user_id = ? AND event_type IN ?", fmt.Sprintf("%d", userID), []string{"auth.login_succeeded", "auth.login_failed", "auth.login_rate_limited"}).Order("occurred_at DESC").Limit(100).Find(&events)
	result := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		result = append(result, map[string]interface{}{
			"eventId": event.EventID, "eventType": event.EventType, "outcome": event.Outcome,
			"severity": event.Severity, "sessionId": event.SessionID, "ipAddress": event.IPAddress,
			"userAgent": event.UserAgent, "reasonCode": event.ReasonCode, "occurredAt": event.OccurredAt,
		})
	}
	return result
}

func (s *service) GetRecoveryCodesStatus(userID int64) map[string]interface{} {
	if userID <= 0 {
		return map[string]interface{}{"totalCodes": 0, "usedCodes": 0, "activeCodes": 0, "enabled": false}
	}
	repo := accountsession.NewRecoveryRepository(s.db)
	codes, err := repo.ListUserCodes(userID)
	if err != nil {
		return map[string]interface{}{"totalCodes": 0, "usedCodes": 0, "activeCodes": 0, "enabled": false}
	}
	active, used := 0, 0
	for _, code := range codes {
		if code.Status == accountsession.RecoveryStatusActive {
			active++
		}
		if code.Status == accountsession.RecoveryStatusUsed {
			used++
		}
	}
	return map[string]interface{}{"totalCodes": len(codes), "usedCodes": used, "activeCodes": active, "enabled": active > 0}
}

func (s *service) GenerateRecoveryCodes(userID int64) map[string]interface{} {
	if userID <= 0 {
		return map[string]interface{}{"codes": []string{}, "generated": false}
	}
	audit := accountsession.NewAuditLogger(accountsession.NewAuditRepository(s.db))
	recovery := accountsession.NewRecoveryService(accountsession.NewRecoveryRepository(s.db), accountsession.NewGrantRepository(s.db), audit)
	codes, _, err := recovery.GenerateCodes(userID)
	if err != nil {
		return map[string]interface{}{"codes": []string{}, "generated": false, "error": err.Error()}
	}
	raw := make([]string, 0, len(codes))
	for _, code := range codes {
		raw = append(raw, code.Raw)
	}
	return map[string]interface{}{"codes": raw, "generated": true}
}

func (s *service) VerifyRecoveryCode(userID int64, code string) map[string]interface{} {
	if userID <= 0 || strings.TrimSpace(code) == "" {
		return map[string]interface{}{"valid": false}
	}
	audit := accountsession.NewAuditLogger(accountsession.NewAuditRepository(s.db))
	recovery := accountsession.NewRecoveryService(accountsession.NewRecoveryRepository(s.db), accountsession.NewGrantRepository(s.db), audit)
	result := recovery.ConsumeCodeForUser(userID, code)
	return map[string]interface{}{"valid": result.Success, "codeId": result.CodeID}
}

func (s *service) GetSessionSettings() map[string]interface{} {
	timeout := config.AppCfg.JWT.AccessTokenMinutes
	if raw := strings.TrimSpace(s.getAppSetting("session_timeout_minutes")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 1 {
			timeout = value
		}
	}
	maxSessions := 10
	if raw := strings.TrimSpace(s.getAppSetting("session_max_per_user")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 1 {
			maxSessions = value
		}
	}
	tracking := s.getAppSetting("session_device_tracking") != "false"
	return map[string]interface{}{"sessionTimeoutMinutes": timeout, "maxSessionsPerUser": maxSessions, "enableDeviceTracking": tracking, "requiresRestart": true}
}

func (s *service) UpdateSessionSettings(body map[string]interface{}) map[string]interface{} {
	if value, ok := body["sessionTimeoutMinutes"].(float64); ok {
		minutes := int(value)
		if minutes < 1 {
			minutes = 1
		}
		if minutes > 10080 {
			minutes = 10080
		}
		s.setAppSetting("session_timeout_minutes", strconv.Itoa(minutes))
	}
	if value, ok := body["maxSessionsPerUser"].(float64); ok {
		count := int(value)
		if count < 1 {
			count = 1
		}
		if count > 100 {
			count = 100
		}
		s.setAppSetting("session_max_per_user", strconv.Itoa(count))
	}
	if value, ok := body["enableDeviceTracking"].(bool); ok {
		s.setAppSetting("session_device_tracking", strconv.FormatBool(value))
	}
	result := s.GetSessionSettings()
	result["requiresRestart"] = true
	return result
}
