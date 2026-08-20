// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"
)

func (s *service) PrivacyScan() map[string]interface{} {
	scanID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	var msgs []map[string]interface{}
	s.db.Table("messages").Select("id, content, conversation_id, role, created_at, safety_level").Limit(1000).Find(&msgs)

	findings := make([]interface{}, 0)
	patterns := map[string]string{
		"password":    "high",
		"passwd":      "high",
		"token":       "high",
		"api_key":     "high",
		"apikey":      "high",
		"secret":      "high",
		"private key": "critical",
	}
	for _, msg := range msgs {
		content, _ := msg["content"].(string)
		lower := strings.ToLower(content)
		for pattern, severity := range patterns {
			if !strings.Contains(lower, pattern) {
				continue
			}
			findings = append(findings, map[string]interface{}{
				"id":             msg["id"],
				"messageId":      msg["id"],
				"conversationId": msg["conversation_id"],
				"role":           msg["role"],
				"createdAt":      msg["created_at"],
				"pattern":        pattern,
				"severity":       severity,
				"risk_level":     severity,
				"risk_type":      pattern,
				"source_table":   "messages",
				"snippet":        privacyPreview(content),
				"preview":        privacyPreview(content),
				"masked":         msg["safety_level"] == "masked",
			})
			break
		}
	}

	highRisk := 0
	mediumRisk := 0
	for _, raw := range findings {
		finding, _ := raw.(map[string]interface{})
		switch finding["severity"] {
		case "critical", "high":
			highRisk++
		case "medium":
			mediumRisk++
		}
	}
	createdAt := time.Now().UTC().Format(time.RFC3339)
	result := map[string]interface{}{
		"scanId":        scanID,
		"id":            scanID,
		"status":        "completed",
		"createdAt":     createdAt,
		"scan_time":     createdAt,
		"scope":         []string{"messages"},
		"findings":      findings,
		"totalFindings": len(findings),
		"totalFound":    len(findings),
		"total_found":   len(findings),
		"highRisk":      highRisk,
		"high_risk":     highRisk,
		"mediumRisk":    mediumRisk,
		"totalScanned":  len(msgs),
	}
	s.privacyMu.Lock()
	s.privacyScans = append([]map[string]interface{}{clonePrivacyMap(result)}, s.privacyScans...)
	if len(s.privacyScans) > 50 {
		s.privacyScans = s.privacyScans[:50]
	}
	s.privacyMu.Unlock()
	return result
}

func (s *service) PrivacyScanResults() map[string]interface{} {
	s.privacyMu.RLock()
	items := make([]map[string]interface{}, len(s.privacyScans))
	for i, item := range s.privacyScans {
		items[i] = clonePrivacyMap(item)
	}
	s.privacyMu.RUnlock()
	resultItems := make([]interface{}, 0)
	if len(items) > 0 {
		if findings, ok := items[0]["findings"].([]interface{}); ok {
			resultItems = append(resultItems, findings...)
		}
	}
	riskCounts := map[string]int{}
	for _, raw := range resultItems {
		if finding, ok := raw.(map[string]interface{}); ok {
			riskCounts[fmt.Sprint(finding["risk_type"])]++
		}
	}
	riskTypes := make([]interface{}, 0, len(riskCounts))
	for riskType, count := range riskCounts {
		riskTypes = append(riskTypes, map[string]interface{}{"risk_type": riskType, "cnt": count})
	}
	return map[string]interface{}{
		"items":        resultItems,
		"total":        len(resultItems),
		"history":      items,
		"historyTotal": len(items),
		"riskTypes":    riskTypes,
		"sourceTables": []interface{}{map[string]interface{}{"source_table": "messages", "cnt": len(resultItems)}},
	}
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
	s.privacyMu.RLock()
	defer s.privacyMu.RUnlock()
	if id == "" {
		if len(s.privacyScans) == 0 {
			return map[string]interface{}{"result": nil}
		}
		return map[string]interface{}{"result": clonePrivacyMap(s.privacyScans[0])}
	}
	for _, item := range s.privacyScans {
		if item["scanId"] == id {
			return map[string]interface{}{"result": clonePrivacyMap(item)}
		}
	}
	return map[string]interface{}{"result": nil, "scanId": id}
}

func privacyPreview(content string) string {
	text := strings.TrimSpace(content)
	const maxRunes = 96
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return text
}

func clonePrivacyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch value := v.(type) {
		case []interface{}:
			copyValue := make([]interface{}, len(value))
			copy(copyValue, value)
			dst[k] = copyValue
		default:
			dst[k] = value
		}
	}
	return dst
}

func (s *service) markPrivacyFindingsMasked(ids []uint) {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[fmt.Sprint(id)] = struct{}{}
	}
	s.privacyMu.Lock()
	defer s.privacyMu.Unlock()
	for _, scan := range s.privacyScans {
		findings, _ := scan["findings"].([]interface{})
		for _, raw := range findings {
			finding, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if _, ok := wanted[fmt.Sprint(finding["id"])]; ok {
				finding["masked"] = true
				finding["snippet"] = "[已脱敏]"
				finding["preview"] = "[已脱敏]"
			}
		}
	}
}
