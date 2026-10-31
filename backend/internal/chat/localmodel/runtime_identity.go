package localmodel

import "strings"

func RuntimeProcessID(provider string, modelID string, fingerprint string) string {
	prefix := runtimeIdentityPart(provider, 24)
	if prefix == "" {
		prefix = "local-model"
	}
	suffix := runtimeIdentityPart(fingerprint, 24)
	id := prefix + "-"
	if suffix != "" {
		id += suffix
	}
	modelPart := runtimeIdentityPart(modelID, 128-len(id)-1)
	if modelPart != "" {
		id += "-" + modelPart
	}
	return strings.Trim(id, "-._")
}

func runtimeIdentityPart(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastSeparator := false
	for _, char := range value {
		allowed := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-'
		if allowed {
			builder.WriteRune(char)
			lastSeparator = char == '-' || char == '.' || char == '_'
			continue
		}
		if !lastSeparator && builder.Len() > 0 {
			builder.WriteByte('-')
			lastSeparator = true
		}
	}
	result := strings.Trim(builder.String(), "-._")
	if len(result) > limit {
		result = strings.Trim(result[:limit], "-._")
	}
	return result
}
