// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"time"
)

func (s *service) GetCurrentSession(token string) map[string]interface{} {
	_ = token
	return map[string]interface{}{
		"deviceName": "Desktop", "ipAddress": "127.0.0.1",
		"lastActiveAt": time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (s *service) GetLoginHistory() []map[string]interface{} {
	var sessions []map[string]interface{}
	s.db.Table("auth_sessions").Order("created_at DESC").Limit(20).Find(&sessions)
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	return sessions
}

func (s *service) GetRecoveryCodesStatus() map[string]interface{} {
	var total, used int64
	s.db.Table("recovery_codes").Count(&total)
	s.db.Table("recovery_codes").Where("used = 1").Count(&used)
	return map[string]interface{}{"totalCodes": total, "usedCodes": used, "enabled": total > 0}
}

func (s *service) GenerateRecoveryCodes() map[string]interface{} {
	codes := []interface{}{}
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("%04d-%04d-%04d", time.Now().UnixNano()%10000, (time.Now().UnixNano()/10000)%10000, (time.Now().UnixNano()/100000000)%10000)
		codes = append(codes, code)
	}
	return map[string]interface{}{"codes": codes, "generatedAt": time.Now().Format(time.DateTime)}
}

func (s *service) VerifyRecoveryCode(code string) map[string]interface{} {
	var count int64
	s.db.Table("recovery_codes").Where("code = ? AND used = 0", code).Count(&count)
	return map[string]interface{}{"valid": count > 0}
}

func (s *service) GetSessionSettings() map[string]interface{} {
	timeout := s.getAppSetting("session_timeout")
	if timeout == "" {
		timeout = "1440"
	}
	maxSess := s.getAppSetting("max_sessions")
	if maxSess == "" {
		maxSess = "10"
	}
	tracking := s.getAppSetting("device_tracking") != "false"
	return map[string]interface{}{"sessionTimeoutMinutes": toInt(timeout), "maxSessionsPerUser": toInt(maxSess), "enableDeviceTracking": tracking}
}

func (s *service) UpdateSessionSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["sessionTimeoutMinutes"]; ok {
		s.setAppSetting("session_timeout", fmt.Sprintf("%d", toInt(v)))
	}
	if v, ok := body["maxSessionsPerUser"]; ok {
		s.setAppSetting("max_sessions", fmt.Sprintf("%d", toInt(v)))
	}
	if v, ok := body["enableDeviceTracking"].(bool); ok {
		if v {
			s.setAppSetting("device_tracking", "true")
		} else {
			s.setAppSetting("device_tracking", "false")
		}
	}
	return s.GetSessionSettings()
}
