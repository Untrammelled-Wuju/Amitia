// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import "github.com/u-ai/backend/internal/spaceidentity"

func (s *service) GetSecurityAccessConfig() map[string]interface{} {
	auth := s.getAppSetting("require_auth") != "false"
	origins := s.getAppSetting("allowed_origins")
	if origins == "" {
		origins = "*"
	}
	rateLimit := s.getAppSetting("rate_limit") != "false"
	return map[string]interface{}{"requireAuth": auth, "allowedOrigins": origins, "rateLimit": rateLimit}
}

func (s *service) UpdateSecurityAccessConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["requireAuth"].(bool); ok {
		if v {
			s.setAppSetting("require_auth", "true")
		} else {
			s.setAppSetting("require_auth", "false")
		}
	}
	if v, ok := body["allowedOrigins"].(string); ok {
		s.setAppSetting("allowed_origins", v)
	}
	if v, ok := body["rateLimit"].(bool); ok {
		if v {
			s.setAppSetting("rate_limit", "true")
		} else {
			s.setAppSetting("rate_limit", "false")
		}
	}
	return s.GetSecurityAccessConfig()
}

func (s *service) GetSecurityAccessStatus() map[string]interface{} {
	cfg := s.GetSecurityAccessConfig()
	return map[string]interface{}{"status": "secure", "config": cfg}
}

func (s *service) GetSecurityStatus() map[string]interface{} {
	identity := s.SecurityIdentityCheck()
	exp := s.SecurityExposureCheck()
	status := "secure"
	if !identity["secure"].(bool) || exp["exposed"].(bool) {
		status = "warning"
	}
	return map[string]interface{}{"status": status, "identity": identity, "exposure": exp}
}

func (s *service) SecurityIdentityCheck() map[string]interface{} {
	spaceID := spaceidentity.DefaultSpaceID()
	var trustedDevices int64
	if spaceID != "" {
		s.db.Table("kernel_devices").Where("space_id = ? AND trust_state = ?", spaceID, "trusted").Count(&trustedDevices)
	}
	return map[string]interface{}{"secure": spaceID != "", "spaceId": spaceID, "trustedDevices": trustedDevices}
}

func (s *service) SecurityExposureCheck() map[string]interface{} {
	var apiKey string
	s.db.Table("app_settings").Select("value").Where("key = ?", "api_key").Row().Scan(&apiKey)
	hasKey := apiKey != ""
	var msgCount int64
	s.db.Table("messages").Where("safety_level = ?", "unsafe").Count(&msgCount)
	exposed := hasKey || msgCount > 0
	return map[string]interface{}{"exposed": exposed, "hasApiKey": hasKey, "unsafeMessages": msgCount}
}
