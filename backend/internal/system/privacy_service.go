// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

const (
	privacyScopeMessages    = "messages"
	privacyScopeMemories    = "memories"
	privacyScopeImportItems = "import_items"
)

type privacyMessageRow struct {
	ID                 string
	ConversationID     string
	Role               string
	Content            string
	CreatedAt          string
	SafetyLevel        string
	MessageSource      string
	ConversationSource string
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

func (s *service) privacyScan(scope []string, memories business.MemoryPrivacySnapshot, messages []privacyMessageRow) map[string]interface{} {
	normalizedScope := normalizePrivacyScopes(scope)
	scanID := fmt.Sprintf("scan_%d", time.Now().UnixNano())
	findings := make([]interface{}, 0)
	totalScanned := 0

	for _, item := range normalizedScope {
		switch item {
		case privacyScopeMessages, privacyScopeImportItems:
			for _, row := range messages {
				imported := row.MessageSource == "import" || row.ConversationSource == "import"
				if (item == privacyScopeImportItems) != imported {
					continue
				}
				totalScanned++
				if finding := privacyMessageFinding(row, item); finding != nil {
					findings = append(findings, finding)
				}
			}
		case privacyScopeMemories:
			totalScanned += memories.Count
			findings = append(findings, memories.Findings...)
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
		"complete":      true,
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

func (s *service) finalizePrivacyScan(result map[string]interface{}) {
	s.privacyMu.Lock()
	defer s.privacyMu.Unlock()
	keep := map[string]bool{}
	for index, scan := range s.privacyScans {
		if scan["scanId"] == result["scanId"] {
			s.privacyScans[index] = clonePrivacyMap(result)
			scan = s.privacyScans[index]
		}
		findings, _ := scan["findings"].([]interface{})
		for _, raw := range findings {
			if finding, ok := raw.(map[string]interface{}); ok {
				id, _ := finding["id"].(string)
				keep[id] = true
			}
		}
	}
	for id := range s.privacyMessages {
		if !keep[id] {
			delete(s.privacyMessages, id)
		}
	}
	for id := range s.privacyMemoryEntries {
		if !keep[id] {
			delete(s.privacyMemoryEntries, id)
		}
	}
}
