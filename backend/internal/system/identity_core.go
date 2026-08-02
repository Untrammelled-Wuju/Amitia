// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

var identityCoreFields = []string{
	"name",
	"identity",
	"character_base",
	"boundary_rules",
	"gender",
	"pronoun",
	"self_reference",
	"life_identity",
}

var identityGrowthAllowedFields = []string{
	"personality",
	"speaking_style",
	"relationship_style",
	"personality_config",
	"chat_style_config",
	"scene_rules",
	"emotion",
	"emotion_scale",
	"silence_duration",
}

var identityCoreJSONFields = map[string]string{
	"systemPrompt":   "character_base",
	"boundaryRules":  "boundary_rules",
	"selfReference":  "self_reference",
	"lifeIdentity":   "life_identity",
	"name":           "name",
	"identity":       "identity",
	"gender":         "gender",
	"pronoun":        "pronoun",
	"character_base": "character_base",
	"system_prompt":  "character_base",
	"boundary_rules": "boundary_rules",
	"self_reference": "self_reference",
	"life_identity":  "life_identity",
}

var identityGrowthJSONFields = map[string]string{
	"personality":        "personality",
	"speakingStyle":      "speaking_style",
	"relationshipStyle":  "relationship_style",
	"personalityConfig":  "personality_config",
	"chatStyleConfig":    "chat_style_config",
	"sceneRules":         "scene_rules",
	"emotion":            "emotion",
	"emotionScale":       "emotion_scale",
	"silenceDuration":    "silence_duration",
	"speaking_style":     "speaking_style",
	"relationship_style": "relationship_style",
	"personality_config": "personality_config",
	"chat_style_config":  "chat_style_config",
	"scene_rules":        "scene_rules",
	"emotion_scale":      "emotion_scale",
	"silence_duration":   "silence_duration",
}

func (s *service) GetIdentityCore(characterID string) map[string]interface{} {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return map[string]interface{}{"found": false, "error": "missing_character_id"}
	}
	var row map[string]interface{}
	s.db.Table("characters").Select(strings.Join(identityCoreFields, ", ")).Where("id = ?", characterID).Limit(1).Scan(&row)
	if row == nil || len(row) == 0 {
		return map[string]interface{}{"found": false, "error": "character_not_found", "characterId": characterID}
	}
	core := map[string]interface{}{}
	for _, field := range identityCoreFields {
		core[field] = normalizeIdentityCoreValue(row[field])
	}
	return map[string]interface{}{
		"found":               true,
		"characterId":         characterID,
		"version":             identityCoreVersion(core),
		"core":                core,
		"frozenFields":        append([]string{}, identityCoreFields...),
		"growthAllowedFields": append([]string{}, identityGrowthAllowedFields...),
		"runtimeWritable":     false,
	}
}

func (s *service) ValidateIdentityCorePatch(characterID string, body map[string]interface{}) map[string]interface{} {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return map[string]interface{}{"valid": false, "error": "missing_character_id"}
	}
	if body == nil {
		body = map[string]interface{}{}
	}
	blocked := []map[string]interface{}{}
	allowed := []string{}
	unknown := []string{}
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		normalized := normalizeIdentityPatchField(key)
		switch {
		case isIdentityCoreField(normalized):
			blocked = append(blocked, map[string]interface{}{"field": key, "normalizedField": normalized, "reason": "identity_core_read_only"})
		case isIdentityGrowthAllowedField(normalized):
			allowed = append(allowed, normalized)
		default:
			unknown = append(unknown, key)
		}
	}
	valid := len(blocked) == 0 && len(unknown) == 0
	return map[string]interface{}{
		"valid":               valid,
		"characterId":         characterID,
		"blockedFields":       blocked,
		"allowedFields":       uniqueSorted(allowed),
		"unknownFields":       unknown,
		"frozenFields":        append([]string{}, identityCoreFields...),
		"growthAllowedFields": append([]string{}, identityGrowthAllowedFields...),
	}
}

func normalizeIdentityPatchField(key string) string {
	key = strings.TrimSpace(key)
	if field, ok := identityCoreJSONFields[key]; ok {
		return field
	}
	if field, ok := identityGrowthJSONFields[key]; ok {
		return field
	}
	return key
}

func isIdentityCoreField(field string) bool {
	for _, item := range identityCoreFields {
		if item == field {
			return true
		}
	}
	return false
}

func isIdentityGrowthAllowedField(field string) bool {
	for _, item := range identityGrowthAllowedFields {
		if item == field {
			return true
		}
	}
	return false
}

func normalizeIdentityCoreValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func identityCoreVersion(core map[string]interface{}) string {
	keys := make([]string, 0, len(core))
	for key := range core {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprint(core[key]))
		builder.WriteString("\n")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "identity-core-v1:" + hex.EncodeToString(sum[:8])
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
