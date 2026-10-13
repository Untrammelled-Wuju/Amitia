package business

import "errors"

type MemoryPrivacySnapshot struct {
	Findings []interface{}
	Count    int
}

func ScanMemoryPrivacy(entries []MemoryConsumerEntry, match func(string) (string, string, bool)) (MemoryPrivacySnapshot, error) {
	result := MemoryPrivacySnapshot{Findings: []interface{}{}}
	if match == nil {
		return result, errors.New("隐私匹配器尚未就绪")
	}
	for _, entry := range entries {
		if entry.Kind != "memory" || entry.Deleted {
			continue
		}
		document, content, err := MemoryContent(entry)
		if err != nil {
			return result, err
		}
		result.Count++
		key, value := MemoryText(document, "key"), MemoryValue(content)
		pattern, severity, found := match(key + " " + value + " " + string(MemoryJSON(content)))
		if !found {
			continue
		}
		preview := []rune(value)
		if len(preview) > 120 {
			preview = append(preview[:120], []rune("…")...)
		}
		result.Findings = append(result.Findings, map[string]interface{}{
			"id": entry.ID, "recordId": entry.ID, "characterId": entry.Scope.RoleID,
			"memoryType": MemoryText(content, "memoryType"), "createdAt": MemoryText(document, "createdAt"),
			"pattern": pattern, "severity": severity, "risk_level": severity, "risk_type": pattern,
			"source_table": "memories", "sourceTable": "memories", "snippet": string(preview), "preview": string(preview),
			"masked": value == "[已脱敏]", "revision": entry.Revision, "ownerId": entry.OwnerID,
			"executionScope": entry.Scope,
		})
	}
	return result, nil
}
