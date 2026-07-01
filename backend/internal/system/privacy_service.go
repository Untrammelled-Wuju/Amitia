// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"
)

func (s *service) PrivacyScan() map[string]interface{} {
	scanId := fmt.Sprintf("scan_%d", time.Now().Unix())
	var msgs []map[string]interface{}
	s.db.Table("messages").Select("id, content").Limit(100).Find(&msgs)
	findings := []interface{}{}
	patterns := []string{"password", "token", "api_key", "secret", "key"}
	for _, msg := range msgs {
		if content, ok := msg["content"].(string); ok {
			for _, p := range patterns {
				if strings.Contains(strings.ToLower(content), p) {
					findings = append(findings, map[string]interface{}{"messageId": msg["id"], "pattern": p, "severity": "high"})
					break
				}
			}
		}
	}
	return map[string]interface{}{"scanId": scanId, "status": "completed", "findings": findings, "totalScanned": len(msgs)}
}

func (s *service) PrivacyScanResults() map[string]interface{} {
	scanId := fmt.Sprintf("scan_%d", time.Now().Unix())
	return s.GetPrivacyScanResult(scanId)
}

func (s *service) PrivacyMask() map[string]interface{} {
	var count int64
	s.db.Table("messages").Where("safety_level = ?", "unsafe").Count(&count)
	if count > 0 {
		s.db.Table("messages").Where("safety_level = ?", "unsafe").Update("safety_level", "masked")
	}
	return map[string]interface{}{"masked": true, "maskedCount": count}
}

func (s *service) GetPrivacyScanResult(id string) map[string]interface{} {
	_ = id
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("safety_level IN ?", []string{"unsafe", "masked"}).Find(&msgs)
	return map[string]interface{}{"result": map[string]interface{}{"scanId": id, "findings": msgs, "totalFindings": len(msgs)}}
}
