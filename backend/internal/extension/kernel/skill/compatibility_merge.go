package skill

import (
	"sort"
	"time"
)

type CompatibilityMerger struct{}

func NewCompatibilityMerger() *CompatibilityMerger {
	return &CompatibilityMerger{}
}

func (m *CompatibilityMerger) Merge(overlays []SkillCompatibilityOverlay, basePolicy SkillInvocationPolicy, baseReport *SkillCompatibilityReport) (*CanonicalSkillCompatibility, *SkillCompatibilityReport) {
	canonical := &CanonicalSkillCompatibility{
		InvocationPolicy:    basePolicy,
		ActivationHints:     []string{},
		ToolAllowHints:      []string{},
		ToolDenyRules:       []string{},
		MCPDependencies:     []SkillMCPDependency{},
		UnsupportedFeatures: []string{},
	}

	var allFieldMappings []SkillFieldMapping
	var allToolMappings []SkillToolMapping
	var allDepMappings []SkillDependencyMapping
	var allFeatures []SkillFeatureResult
	var allWarnings []SkillWarning
	var allErrors []SkillError
	var allUnsupported []string

	adapterVersions := make(map[string]string)

	for _, overlay := range overlays {
		adapterVersions[overlay.Profile] = overlay.AdapterVersion

		if overlay.InvocationPolicy != nil {
			canonical.InvocationPolicy = MergeInvocationPolicy(canonical.InvocationPolicy, *overlay.InvocationPolicy)
		}

		if overlay.ArgumentSchema != nil && canonical.ArgumentSchema == nil {
			canonical.ArgumentSchema = overlay.ArgumentSchema
		}

		canonical.ActivationHints = append(canonical.ActivationHints, overlay.ActivationHints...)

		if overlay.PreferredModelHint != "" && canonical.PreferredModelHint == "" {
			canonical.PreferredModelHint = overlay.PreferredModelHint
		}

		if overlay.PreferredEffort != "" && canonical.PreferredEffort == "" {
			canonical.PreferredEffort = overlay.PreferredEffort
		}

		canonical.WorkspacePathPatterns = append(canonical.WorkspacePathPatterns, overlay.WorkspacePathPatterns...)

		if overlay.ExecutionMode != "" && canonical.ExecutionMode == "" {
			canonical.ExecutionMode = overlay.ExecutionMode
		}

		canonical.ToolAllowHints = mergeUniqueStrings(canonical.ToolAllowHints, overlay.ToolAllowHints...)
		canonical.ToolDenyRules = mergeUniqueStrings(canonical.ToolDenyRules, overlay.ToolDenyRules...)

		if len(overlay.MCPDependencies) > 0 {
			canonical.MCPDependencies = append(canonical.MCPDependencies, overlay.MCPDependencies...)
		}

		if overlay.UI != nil {
			canonical.UI = mergeUIHints(canonical.UI, *overlay.UI)
		}

		allFieldMappings = append(allFieldMappings, overlay.FieldMappings...)
		allFeatures = append(allFeatures, overlay.Features...)
		allDepMappings = append(allDepMappings, overlay.DependencyMappings...)
		canonical.UnsupportedFeatures = mergeUniqueStrings(canonical.UnsupportedFeatures, overlay.UnsupportedFeatures...)
		allWarnings = append(allWarnings, overlay.Warnings...)
		allErrors = append(allErrors, overlay.Errors...)
		allUnsupported = append(allUnsupported, overlay.UnsupportedFeatures...)
	}

	canonical.UnsupportedFeatures = uniqueStrings(canonical.UnsupportedFeatures)

	if baseReport != nil {
		allFieldMappings = append(allFieldMappings, baseReport.FieldMappings...)
		allToolMappings = append(allToolMappings, baseReport.ToolMappings...)
		allDepMappings = append(allDepMappings, baseReport.DependencyMappings...)
		allFeatures = append(allFeatures, baseReport.MappedFeatures...)
		allWarnings = append(allWarnings, baseReport.Warnings...)
		allErrors = append(allErrors, baseReport.Errors...)
		allUnsupported = append(allUnsupported, baseReport.Unsupported...)
	}

	sort.Strings(canonical.ActivationHints)
	sort.Strings(canonical.ToolAllowHints)
	sort.Strings(canonical.ToolDenyRules)

	report := &SkillCompatibilityReport{
		Status:             computeOverallStatus(allFeatures, allErrors, allWarnings, canonical),
		FieldMappings:      allFieldMappings,
		ToolMappings:       allToolMappings,
		DependencyMappings: allDepMappings,
		MappedFeatures:     allFeatures,
		Warnings:           allWarnings,
		Errors:             allErrors,
		Unsupported:        uniqueStrings(allUnsupported),
		EvaluatedAt:        time.Now().UTC(),
	}

	if baseReport != nil {
		report.RequiredScripts = baseReport.RequiredScripts
		report.MissingFiles = baseReport.MissingFiles
	}

	return canonical, report
}

func computeOverallStatus(features []SkillFeatureResult, errors []SkillError, warnings []SkillWarning, canonical *CanonicalSkillCompatibility) string {
	for _, e := range errors {
		if e.Code == "ADAPTER_ANALYZE_ERROR" || e.Code == "OPENAI_YAML_INVALID" {
			continue
		}
		return SkillCompatStatusBlocked
	}

	hasPartial := false
	for _, f := range features {
		if f.State == FeatureStateBlocked {
			return SkillCompatStatusBlocked
		}
		if f.State == FeatureStateUnsupported || f.State == FeatureStateDegraded {
			hasPartial = true
		}
	}

	if len(canonical.UnsupportedFeatures) > 0 {
		return SkillCompatStatusDegraded
	}

	if hasPartial {
		return SkillCompatStatusDegraded
	}

	if len(warnings) > 0 {
		return SkillCompatStatusDegraded
	}

	return SkillCompatStatusCompatible
}

func mergeUIHints(base, overlay SkillUIHints) SkillUIHints {
	result := base
	if overlay.ArgumentHint != "" && result.ArgumentHint == "" {
		result.ArgumentHint = overlay.ArgumentHint
	}
	if overlay.IconSmall != "" && result.IconSmall == "" {
		result.IconSmall = overlay.IconSmall
	}
	if overlay.IconLarge != "" && result.IconLarge == "" {
		result.IconLarge = overlay.IconLarge
	}
	if overlay.BrandColor != "" && result.BrandColor == "" {
		result.BrandColor = overlay.BrandColor
	}
	if overlay.DisplayName != "" && result.DisplayName == "" {
		result.DisplayName = overlay.DisplayName
	}
	if overlay.ShortDescription != "" && result.ShortDescription == "" {
		result.ShortDescription = overlay.ShortDescription
	}
	if overlay.DefaultPrompt != "" && result.DefaultPrompt == "" {
		result.DefaultPrompt = overlay.DefaultPrompt
	}
	return result
}

func mergeUniqueStrings(base []string, items ...string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	result := append([]string{}, base...)
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result
}
