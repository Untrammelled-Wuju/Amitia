// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"
)

const (
	privacyScopeMessages    = "messages"
	privacyScopeMemories    = "memories"
	privacyScopeImportItems = "import_items"
)

type privacyMessageRow struct {
	ID                 string `gorm:"column:id"`
	ConversationID     string `gorm:"column:conversation_id"`
	Role               string `gorm:"column:role"`
	Content            string `gorm:"column:content"`
	CreatedAt          string `gorm:"column:created_at"`
	SafetyLevel        string `gorm:"column:safety_level"`
	MessageSource      string `gorm:"column:message_source"`
	ConversationSource string `gorm:"column:conversation_source"`
}

type privacyMemoryRow struct {
	ID               string `gorm:"column:id"`
	CharacterID      string `gorm:"column:character_id"`
	MemoryType       string `gorm:"column:memory_type"`
	Key              string `gorm:"column:key"`
	Value            string `gorm:"column:value"`
	CreatedAt        string `gorm:"column:created_at"`
	SensitivityLevel string `gorm:"column:sensitivity_level"`
}

type privacyMaskTarget struct {
	ID          string
	SourceTable string
}

var privacySensitivePatterns = []struct {
	Pattern  string
	Severity string
}{
	{Pattern: "private key", Severity: "critical"},
	{Pattern: "password", Severity: "high"},
	{Pattern: "passwd", Severity: "high"},
	{Pattern: "token", Severity: "high"},
	{Pattern: "api_key", Severity: "high"},
	{Pattern: "apikey", Severity: "high"},
	{Pattern: "secret", Severity: "high"},
}

func (s *service) PrivacyScan(scope []string) map[string]interface{} {
	normalizedScope := normalizePrivacyScopes(scope)
	scanID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	findings := make([]interface{}, 0)
	totalScanned := 0

	for _, item := range normalizedScope {
		switch item {
		case privacyScopeMessages:
			rows := s.scanPrivacyMessages(false)
			totalScanned += len(rows)
			for _, row := range rows {
				if finding := privacyMessageFinding(row, privacyScopeMessages); finding != nil {
					findings = append(findings, finding)
				}
			}
		case privacyScopeImportItems:
			rows := s.scanPrivacyMessages(true)
			totalScanned += len(rows)
			for _, row := range rows {
				if finding := privacyMessageFinding(row, privacyScopeImportItems); finding != nil {
					findings = append(findings, finding)
				}
			}
		case privacyScopeMemories:
			rows := s.scanPrivacyMemories()
			totalScanned += len(rows)
			for _, row := range rows {
				if finding := privacyMemoryFinding(row); finding != nil {
					findings = append(findings, finding)
				}
			}
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
		"scope":         normalizedScope,
		"findings":      findings,
		"totalFindings": len(findings),
		"totalFound":    len(findings),
		"total_found":   len(findings),
		"highRisk":      highRisk,
		"high_risk":     highRisk,
		"mediumRisk":    mediumRisk,
		"totalScanned":  totalScanned,
	}
	s.privacyMu.Lock()
	s.privacyScans = append([]map[string]interface{}{clonePrivacyMap(result)}, s.privacyScans...)
	if len(s.privacyScans) > 50 {
		s.privacyScans = s.privacyScans[:50]
	}
	s.privacyMu.Unlock()
	return result
}

func (s *service) scanPrivacyMessages(imported bool) []privacyMessageRow {
	var rows []privacyMessageRow
	query := s.db.Table("messages AS m").
		Select("m.id, m.conversation_id, m.role, m.content, m.created_at, m.safety_level, COALESCE(m.source, '') AS message_source, COALESCE(c.source, '') AS conversation_source").
		Joins("LEFT JOIN conversations AS c ON c.id = m.conversation_id")
	if imported {
		query = query.Where("COALESCE(m.source, '') = ? OR COALESCE(c.source, '') = ?", "import", "import")
	} else {
		query = query.Where("COALESCE(m.source, '') <> ? AND COALESCE(c.source, '') <> ?", "import", "import")
	}
	query.Order("m.created_at DESC").Limit(1000).Scan(&rows)
	return rows
}

func (s *service) scanPrivacyMemories() []privacyMemoryRow {
	var rows []privacyMemoryRow
	s.db.Table("memories").
		Select("id, character_id, memory_type, key, value, created_at, sensitivity_level").
		Order("created_at DESC").
		Limit(1000).
		Scan(&rows)
	return rows
}

func privacyMessageFinding(row privacyMessageRow, sourceTable string) map[string]interface{} {
	pattern, severity, ok := matchPrivacyPattern(row.Content)
	if !ok {
		return nil
	}
	return map[string]interface{}{
		"id":             row.ID,
		"recordId":       row.ID,
		"messageId":      row.ID,
		"conversationId": row.ConversationID,
		"role":           row.Role,
		"createdAt":      row.CreatedAt,
		"pattern":        pattern,
		"severity":       severity,
		"risk_level":     severity,
		"risk_type":      pattern,
		"source_table":   sourceTable,
		"sourceTable":    sourceTable,
		"snippet":        privacyPreview(row.Content),
		"preview":        privacyPreview(row.Content),
		"masked":         strings.EqualFold(row.SafetyLevel, "masked") || strings.TrimSpace(row.Content) == "[已脱敏]",
	}
}

func privacyMemoryFinding(row privacyMemoryRow) map[string]interface{} {
	searchText := strings.TrimSpace(row.Key + " " + row.Value)
	pattern, severity, ok := matchPrivacyPattern(searchText)
	if !ok {
		return nil
	}
	return map[string]interface{}{
		"id":           row.ID,
		"recordId":     row.ID,
		"characterId":  row.CharacterID,
		"memoryType":   row.MemoryType,
		"createdAt":    row.CreatedAt,
		"pattern":      pattern,
		"severity":     severity,
		"risk_level":   severity,
		"risk_type":    pattern,
		"source_table": privacyScopeMemories,
		"sourceTable":  privacyScopeMemories,
		"snippet":      privacyPreview(row.Value),
		"preview":      privacyPreview(row.Value),
		"masked":       strings.TrimSpace(row.Value) == "[已脱敏]",
	}
}

func matchPrivacyPattern(content string) (string, string, bool) {
	lower := strings.ToLower(content)
	for _, item := range privacySensitivePatterns {
		if strings.Contains(lower, item.Pattern) {
			return item.Pattern, item.Severity, true
		}
	}
	return "", "", false
}

func normalizePrivacyScopes(scope []string) []string {
	if len(scope) == 0 {
		// Preserve the historical API behaviour for callers that do not yet send
		// a scope. Updated Electron/Flutter clients send their selected scope.
		return []string{privacyScopeMessages}
	}
	allowed := map[string]bool{
		privacyScopeMessages:    true,
		privacyScopeMemories:    true,
		privacyScopeImportItems: true,
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(scope))
	for _, raw := range scope {
		value := strings.TrimSpace(strings.ToLower(raw))
		if !allowed[value] || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if len(result) == 0 {
		return []string{privacyScopeMessages}
	}
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
	sourceCounts := map[string]int{}
	for _, raw := range resultItems {
		if finding, ok := raw.(map[string]interface{}); ok {
			riskCounts[fmt.Sprint(finding["risk_type"])]++
			sourceCounts[fmt.Sprint(finding["source_table"])]++
		}
	}
	riskTypes := make([]interface{}, 0, len(riskCounts))
	for riskType, count := range riskCounts {
		riskTypes = append(riskTypes, map[string]interface{}{"risk_type": riskType, "cnt": count})
	}
	sourceTables := make([]interface{}, 0, len(sourceCounts))
	for sourceTable, count := range sourceCounts {
		sourceTables = append(sourceTables, map[string]interface{}{"source_table": sourceTable, "cnt": count})
	}
	return map[string]interface{}{
		"items":        resultItems,
		"total":        len(resultItems),
		"history":      items,
		"historyTotal": len(items),
		"riskTypes":    riskTypes,
		"sourceTables": sourceTables,
	}
}

func (s *service) PrivacyMask() map[string]interface{} {
	var count int64
	s.db.Table("messages").Where("safety_level = ?", "unsafe").Count(&count)
	if count > 0 {
		s.db.Table("messages").Where("safety_level = ?", "unsafe").Updates(map[string]interface{}{
			"content":      "[已脱敏]",
			"safety_level": "masked",
		})
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
			for i, raw := range value {
				if item, ok := raw.(map[string]interface{}); ok {
					copyValue[i] = clonePrivacyMap(item)
				} else {
					copyValue[i] = raw
				}
			}
			dst[k] = copyValue
		case []string:
			copyValue := append([]string(nil), value...)
			dst[k] = copyValue
		default:
			dst[k] = value
		}
	}
	return dst
}

func (s *service) markPrivacyFindingsMasked(targets []privacyMaskTarget) {
	wanted := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		wanted[target.SourceTable+"\x00"+target.ID] = struct{}{}
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
			key := fmt.Sprint(finding["source_table"]) + "\x00" + fmt.Sprint(finding["id"])
			if _, ok := wanted[key]; ok {
				finding["masked"] = true
				finding["snippet"] = "[已脱敏]"
				finding["preview"] = "[已脱敏]"
			}
		}
	}
}
