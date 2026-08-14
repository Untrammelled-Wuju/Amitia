package skill

import (
	"context"
	"strings"
)

type ClaudeLegacyCommandAdapter struct{}

func NewClaudeLegacyCommandAdapter() *ClaudeLegacyCommandAdapter {
	return &ClaudeLegacyCommandAdapter{}
}

func (a *ClaudeLegacyCommandAdapter) ID() string      { return ProfileIDClaudeCommandLegacy }
func (a *ClaudeLegacyCommandAdapter) Version() string { return AdapterVersionClaudeLegacy }

func (a *ClaudeLegacyCommandAdapter) Detect(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillProfileDetection, error) {
	isLegacy := false
	var evidence []string

	if strings.HasPrefix(pkg.SourceFile, ".claude/commands/") {
		isLegacy = true
		evidence = append(evidence, "source:.claude/commands/")
	}
	if strings.HasSuffix(pkg.SourceFile, ".md") && pkg.SourceFile != "SKILL.md" {
		if _, isStd := parsed.ExtraFrontmatter["name"]; !isStd {
			isLegacy = true
			evidence = append(evidence, "non-skillmd command file")
		}
	}
	if _, hasLegacyDesc := parsed.ExtraFrontmatter["description"]; !hasLegacyDesc && len(parsed.Body) > 0 {
		if strings.HasPrefix(pkg.SourceFile, ".claude/commands/") {
			isLegacy = true
			evidence = append(evidence, "no frontmatter description")
		}
	}

	if isLegacy {
		return SkillProfileDetection{
			Detected: []SkillEcosystemProfile{
				{ID: ProfileIDClaudeCommandLegacy, Version: AdapterVersionClaudeLegacy, Evidence: evidence},
			},
		}, nil
	}
	return SkillProfileDetection{Detected: []SkillEcosystemProfile{}}, nil
}

func (a *ClaudeLegacyCommandAdapter) Analyze(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillCompatibilityOverlay, error) {
	overlay := SkillCompatibilityOverlay{
		Profile:        ProfileIDClaudeCommandLegacy,
		AdapterVersion: AdapterVersionClaudeLegacy,
		FieldMappings:  []SkillFieldMapping{},
		Features:       []SkillFeatureResult{},
		Warnings:       []SkillWarning{},
		Errors:         []SkillError{},
	}

	if parsed.Name == "" {
		derivedName := deriveNameFromSource(pkg.SourceFile)
		if derivedName != "" {
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCommandLegacy, Source: "sourceFileName", Target: "name",
				State: FeatureStateMapped, Reason: "derived from source file name",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "LEGACY_NAME_DERIVED",
				Message: "name was derived from source directory/file name",
				Path:    pkg.SourceFile,
			})
		} else {
			overlay.Errors = append(overlay.Errors, SkillError{
				Code:    "LEGACY_NAME_REQUIRED",
				Message: "legacy command requires a valid name",
				Path:    pkg.SourceFile,
			})
		}
	}

	if parsed.Description == "" {
		derivedDesc := deriveFirstParagraph(parsed.Body)
		if derivedDesc != "" {
			overlay.FieldMappings = append(overlay.FieldMappings, SkillFieldMapping{
				Profile: ProfileIDClaudeCommandLegacy, Source: "bodyFirstParagraph", Target: "description",
				State: FeatureStateDegraded, Reason: "description derived from first body paragraph as fallback",
			})
			overlay.Warnings = append(overlay.Warnings, SkillWarning{
				Code:    "LEGACY_DESCRIPTION_DERIVED",
				Message: "description was derived from first body paragraph; user should review before install",
				Path:    "SKILL.md",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCommandLegacy, Feature: "description", State: FeatureStateDegraded,
				Reason: "no explicit description; derived from body",
			})
		} else {
			overlay.Errors = append(overlay.Errors, SkillError{
				Code:    "LEGACY_DESCRIPTION_REQUIRED",
				Message: "legacy command requires a description; none could be derived",
				Path:    "SKILL.md",
			})
			overlay.Features = append(overlay.Features, SkillFeatureResult{
				Profile: ProfileIDClaudeCommandLegacy, Feature: "description", State: FeatureStateBlocked,
				Reason: "no description available and none could be derived",
			})
		}
	}

	overlay.Features = append(overlay.Features, SkillFeatureResult{
		Profile: ProfileIDClaudeCommandLegacy, Feature: "legacy_command_format", State: FeatureStateMapped,
		Reason: "Claude custom command converted to canonical skill artifact",
	})

	return overlay, nil
}

func deriveNameFromSource(sourceFile string) string {
	base := sourceFile
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if strings.HasSuffix(base, ".md") {
		base = base[:len(base)-3]
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	lower := strings.ToLower(base)
	lower = strings.ReplaceAll(lower, " ", "-")
	lower = strings.ReplaceAll(lower, "_", "-")
	return lower
}

func deriveFirstParagraph(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	var para []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			break
		}
		if strings.HasPrefix(t, "#") {
			continue
		}
		para = append(para, t)
	}
	result := strings.Join(para, " ")
	result = strings.TrimSpace(result)
	if len(result) > 256 {
		result = result[:256]
	}
	return result
}
