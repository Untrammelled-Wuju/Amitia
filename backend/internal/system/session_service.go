// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/u-ai/backend/config"
)

func (s *service) GetCurrentSession(token string) map[string]interface{} {
	_ = token
	return map[string]interface{}{
		"deviceName":   "Desktop",
		"ipAddress":    "127.0.0.1",
		"lastActiveAt": "",
	}
}

func (s *service) GetLoginHistory() []map[string]interface{} {
	return []map[string]interface{}{}
}

func (s *service) GetRecoveryCodesStatus() map[string]interface{} {
	return map[string]interface{}{"totalCodes": 0, "usedCodes": 0, "enabled": false}
}

func (s *service) GenerateRecoveryCodes() map[string]interface{} {
	return map[string]interface{}{"codes": []interface{}{}, "generatedAt": ""}
}

func (s *service) VerifyRecoveryCode(code string) map[string]interface{} {
	_ = code
	return map[string]interface{}{"valid": false}
}

func (s *service) GetSessionSettings() map[string]interface{} {
	return map[string]interface{}{
		"sessionTimeoutMinutes": config.AppCfg.JWT.AccessTokenMinutes,
		"maxSessionsPerUser":    10,
		"enableDeviceTracking":  true,
	}
}

func (s *service) UpdateSessionSettings(body map[string]interface{}) map[string]interface{} {
	return s.GetSessionSettings()
}
