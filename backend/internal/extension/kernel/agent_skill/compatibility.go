package agent_skill

import (
	"fmt"
	"regexp"
	"strings"
)

type CompatibilityValidator struct{}

func NewCompatibilityValidator() *CompatibilityValidator {
	return &CompatibilityValidator{}
}

func (v *CompatibilityValidator) Validate(fields map[string]any, hostVersion string, platform string) SkillCompatibility {
	status := "compatible"
	var messages []string

	schemaVersion := getIntField(fields, "schemaVersion", 2)

	if schemaVersion < 2 {
		status = "legacy"
		messages = append(messages, "schema version is below minimum")
	}

	minVersion := getStringField(fields, "compatibility.minHostVersion", "")
	maxVersion := getStringField(fields, "compatibility.maxHostVersion", "")

	if minVersion != "" && !isVersionCompatible(hostVersion, minVersion, ">=") {
		status = "incompatible"
		messages = append(messages, "host version "+hostVersion+" below min "+minVersion)
	}
	if maxVersion != "" && !isVersionCompatible(hostVersion, maxVersion, "<=") {
		status = "incompatible"
		messages = append(messages, "host version "+hostVersion+" above max "+maxVersion)
	}

	platforms := getStringArrayField(fields, "compatibility.platforms")
	if len(platforms) > 0 && platform != "" {
		if !containsString(platforms, platform) {
			status = "incompatible"
			messages = append(messages, "platform "+platform+" not supported")
		}
	}

	return SkillCompatibility{
		MinHostVersion: minVersion,
		MaxHostVersion: maxVersion,
		Platforms:      platforms,
		FeatureFlags:   getStringArrayField(fields, "compatibility.featureFlags"),
		SchemaVersion:  schemaVersion,
		Status:         status,
		Messages:       messages,
	}
}

func (v *CompatibilityValidator) GetField(fields map[string]any, key string) (any, bool) {
	parts := strings.Split(key, ".")
	current := any(fields)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = m[part]
	}
	return current, current != nil
}

func getStringField(fields map[string]any, key, defaultVal string) string {
	parts := strings.Split(key, ".")
	current := any(fields)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return defaultVal
		}
		current = m[part]
	}
	if s, ok := current.(string); ok {
		return s
	}
	return defaultVal
}

func getIntField(fields map[string]any, key string, defaultVal int) int {
	parts := strings.Split(key, ".")
	current := any(fields)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return defaultVal
		}
		current = m[part]
	}
	if n, ok := current.(int); ok {
		return n
	}
	return defaultVal
}

func getStringArrayField(fields map[string]any, key string) []string {
	parts := strings.Split(key, ".")
	current := any(fields)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	if arr, ok := current.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func isVersionCompatible(current, target string, op string) bool {
	curMatch := versionPattern.FindStringSubmatch(strings.TrimPrefix(current, "v"))
	tarMatch := versionPattern.FindStringSubmatch(strings.TrimPrefix(target, "v"))
	if curMatch == nil || tarMatch == nil {
		return false
	}

	cmp := compareVersions(curMatch[1], curMatch[2], curMatch[3], tarMatch[1], tarMatch[2], tarMatch[3])

	switch op {
	case ">=":
		return cmp >= 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case "<":
		return cmp < 0
	default:
		return cmp == 0
	}
}

func compareVersions(aMajor, aMinor, aPatch, bMajor, bMinor, bPatch string) int {
	return compareVersionParts(aMajor, bMajor, aMinor, bMinor, aPatch, bPatch)
}

func compareVersionParts(am, bm, ai, bi, ap, bp string) int {
	for _, pair := range [][2]string{{am, bm}, {ai, bi}, {ap, bp}} {
		a, b := pair[0], pair[1]
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	}
	return 0
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

type DependencyValidator struct{}

func NewDependencyValidator() *DependencyValidator {
	return &DependencyValidator{}
}

func (v *DependencyValidator) ValidateTools(refs []ToolReference, available map[string]bool) (missing []string) {
	for _, ref := range refs {
		if !ref.Required {
			continue
		}
		if !available[ref.ToolID] {
			missing = append(missing, ref.ToolID)
		}
	}
	return
}

func (v *DependencyValidator) ValidateMCP(refs []MCPReference, available map[string]bool) (missing []string) {
	for _, ref := range refs {
		if ref.Optional {
			continue
		}
		if !available[ref.ServerID] {
			missing = append(missing, ref.ServerID)
		}
	}
	return
}

type ChangeDetector struct{}

func NewChangeDetector() *ChangeDetector {
	return &ChangeDetector{}
}

type SkillChangeReport struct {
	HasChanges         bool                      `json:"hasChanges"`
	InstructionChanged bool                      `json:"instructionChanged"`
	ResourcesAdded     []SkillResourceDescriptor `json:"resourcesAdded,omitempty"`
	ResourcesRemoved   []SkillResourceDescriptor `json:"resourcesRemoved,omitempty"`
	ResourcesModified  []SkillResourceDescriptor `json:"resourcesModified,omitempty"`
	ToolsAdded         []string                  `json:"toolsAdded,omitempty"`
	ToolsRemoved       []string                  `json:"toolsRemoved,omitempty"`
	MCPAdded           []string                  `json:"mcpAdded,omitempty"`
	MCPRemoved         []string                  `json:"mcpRemoved,omitempty"`
}

func (d *ChangeDetector) Detect(old, new AgentSkillDefinition, indexer *ResourceIndexer) SkillChangeReport {
	report := SkillChangeReport{}

	if old.Instructions.ContentHash != new.Instructions.ContentHash {
		report.InstructionChanged = true
		report.HasChanges = true
	}

	if indexer != nil {
		added, removed, modified := indexer.Diff(old.Resources, new.Resources)
		report.ResourcesAdded = added
		report.ResourcesRemoved = removed
		report.ResourcesModified = modified
		if len(added)+len(removed)+len(modified) > 0 {
			report.HasChanges = true
		}
	}

	oldTools := toolRefMap(old.RequiredTools)
	newTools := toolRefMap(new.RequiredTools)
	for id := range newTools {
		if !oldTools[id] {
			report.ToolsAdded = append(report.ToolsAdded, id)
			report.HasChanges = true
		}
	}
	for id := range oldTools {
		if !newTools[id] {
			report.ToolsRemoved = append(report.ToolsRemoved, id)
			report.HasChanges = true
		}
	}

	oldMCP := mcpRefMap(old.RequiredMCP)
	newMCP := mcpRefMap(new.RequiredMCP)
	for id := range newMCP {
		if !oldMCP[id] {
			report.MCPAdded = append(report.MCPAdded, id)
			report.HasChanges = true
		}
	}
	for id := range oldMCP {
		if !newMCP[id] {
			report.MCPRemoved = append(report.MCPRemoved, id)
			report.HasChanges = true
		}
	}

	return report
}

func toolRefMap(refs []ToolReference) map[string]bool {
	m := map[string]bool{}
	for _, ref := range refs {
		m[ref.ToolID] = true
	}
	return m
}

func mcpRefMap(refs []MCPReference) map[string]bool {
	m := map[string]bool{}
	for _, ref := range refs {
		m[ref.ServerID] = true
	}
	return m
}

func ValidateFrontmatterFields(fields map[string]any) []string {
	knownFields := map[string]bool{
		"id": true, "name": true, "description": true, "version": true,
		"schemaVersion": true, "activation": true, "requiredTools": true,
		"resources": true, "tokenPolicy": true, "compatibility": true,
		"scope": true, "author": true, "license": true,
		"displayName": true, "requiredMCP": true, "metadata": true,
	}

	var warnings []string
	for key := range fields {
		if !knownFields[key] {
			warnings = append(warnings, fmt.Sprintf("unknown field: %s", key))
		}
	}
	return warnings
}
