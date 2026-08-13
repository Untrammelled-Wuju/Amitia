package extension

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/skill"
)

var defaultProfilePipeline = skill.DefaultCompatibilityPipeline()

func EnhanceAgentSkillCompatibility(files map[string][]byte, rootName string, source AgentSkillSource, baseReport AgentSkillCompatibilityReport, contentHash string) AgentSkillCompatibilityReport {
	parsed := buildSkillParsedFromFiles(files, rootName)
	pkg := skill.SkillPackageView{
		RootURI:     rootName,
		SourceFile:  "SKILL.md",
		Files:       files,
		Parsed:      parsed,
		ContentHash: contentHash,
		Source:      string(source),
	}

	var kernelBaseReport *skill.SkillCompatibilityReport
	kernelBaseReport = convertExtensionReportToKernel(&baseReport)

	canonical, kernelReport, err := defaultProfilePipeline.Evaluate(context.Background(), pkg, kernelBaseReport)
	if err != nil || kernelReport == nil {
		return baseReport
	}

	merged := convertKernelReportToExtension(kernelReport)
	applyCanonicalToReport(canonical, merged, contentHash)
	return *merged
}

func applyCanonicalToReport(canonical *skill.CanonicalSkillCompatibility, report *AgentSkillCompatibilityReport, contentHash string) {
	if canonical == nil {
		return
	}
	if report.Fingerprint == nil {
		report.Fingerprint = &AgentSkillCompatibilityFingerprint{
			ContentHash:   contentHash,
			CapabilityGen: skill.CapabilityGeneration,
		}
	}
	if report.EvaluatedAt.IsZero() {
		report.EvaluatedAt = time.Now().UTC()
	}
}

func convertExtensionReportToKernel(r *AgentSkillCompatibilityReport) *skill.SkillCompatibilityReport {
	if r == nil {
		return nil
	}
	detected := make([]skill.SkillEcosystemProfile, 0, len(r.DetectedProfiles))
	for _, p := range r.DetectedProfiles {
		detected = append(detected, skill.SkillEcosystemProfile{
			ID:       p.ID,
			Version:  p.Version,
			Evidence: append([]string{}, p.Evidence...),
		})
	}
	return &skill.SkillCompatibilityReport{
		Status:          string(r.Status),
		Detected:        detected,
		RequiredScripts: append([]string{}, r.RequiredScripts...),
		MissingFiles:    append([]string{}, r.MissingFiles...),
		Unsupported:     append([]string{}, r.Unsupported...),
		Warnings:        convertExtensionWarningsToKernel(r.Warnings),
		Errors:          convertExtensionErrorsToKernel(r.Errors),
	}
}

func convertKernelReportToExtension(r *skill.SkillCompatibilityReport) *AgentSkillCompatibilityReport {
	if r == nil {
		return nil
	}
	return &AgentSkillCompatibilityReport{
		Status:             mapKernelStatus(r.Status),
		DetectedProfiles:   convertKernelProfilesToExtension(r.Detected),
		FieldMappings:      convertKernelFieldMappingsToExtension(r.FieldMappings),
		ToolMappings:       convertKernelToolMappingsToExtension(r.ToolMappings),
		MappedFeatures:     convertKernelFeaturesToExtension(r.MappedFeatures),
		DependencyMappings: convertKernelDepMappingsToExtension(r.DependencyMappings),
		RequiredScripts:    append([]string{}, r.RequiredScripts...),
		MissingFiles:       append([]string{}, r.MissingFiles...),
		Unsupported:        append([]string{}, r.Unsupported...),
		Warnings:           convertKernelWarningsToExtension(r.Warnings),
		Errors:             convertKernelErrorsToExtension(r.Errors),
		Fingerprint:        convertKernelFingerprintToExtension(r.Fingerprint),
		EvaluatedAt:        r.EvaluatedAt,
	}
}

func mapKernelStatus(status string) AgentSkillCompatibilityStatus {
	switch status {
	case skill.SkillCompatStatusBlocked:
		return AgentSkillBlocked
	case skill.SkillCompatStatusCompatible:
		return AgentSkillCompatible
	default:
		return AgentSkillPartiallyCompatible
	}
}

func convertKernelProfilesToExtension(p []skill.SkillEcosystemProfile) []AgentSkillEcosystemProfile {
	if len(p) == 0 {
		return nil
	}
	result := make([]AgentSkillEcosystemProfile, 0, len(p))
	for _, item := range p {
		result = append(result, AgentSkillEcosystemProfile{
			ID:       item.ID,
			Version:  item.Version,
			Evidence: append([]string{}, item.Evidence...),
		})
	}
	return result
}

func convertKernelFieldMappingsToExtension(m []skill.SkillFieldMapping) []AgentSkillFieldMapping {
	if len(m) == 0 {
		return nil
	}
	result := make([]AgentSkillFieldMapping, 0, len(m))
	for _, item := range m {
		result = append(result, AgentSkillFieldMapping{
			Profile: item.Profile,
			Source:  item.Source,
			Target:  item.Target,
			State:   item.State,
			Reason:  item.Reason,
		})
	}
	return result
}

func convertKernelFeaturesToExtension(f []skill.SkillFeatureResult) []AgentSkillFeatureResult {
	if len(f) == 0 {
		return nil
	}
	result := make([]AgentSkillFeatureResult, 0, len(f))
	for _, item := range f {
		result = append(result, AgentSkillFeatureResult{
			Profile: item.Profile,
			Feature: item.Feature,
			State:   item.State,
			Reason:  item.Reason,
		})
	}
	return result
}

func convertKernelDepMappingsToExtension(d []skill.SkillDependencyMapping) []AgentSkillDependencyMapping {
	if len(d) == 0 {
		return nil
	}
	result := make([]AgentSkillDependencyMapping, 0, len(d))
	for _, item := range d {
		result = append(result, AgentSkillDependencyMapping{
			Profile:    item.Profile,
			ResolvedAs: item.ResolvedAs,
			State:      item.State,
			Reason:     item.Reason,
		})
	}
	return result
}

func convertKernelToolMappingsToExtension(m []skill.SkillToolMapping) []AgentSkillToolMapping {
	if len(m) == 0 {
		return nil
	}
	result := make([]AgentSkillToolMapping, 0, len(m))
	for _, item := range m {
		result = append(result, AgentSkillToolMapping{
			SourceTool:    item.SourceTool,
			TargetSkillID: item.TargetSkillID,
			Status:        item.Status,
			Reason:        item.Reason,
		})
	}
	return result
}

func convertKernelFingerprintToExtension(f *skill.CompatibilityFingerprint) *AgentSkillCompatibilityFingerprint {
	if f == nil {
		return nil
	}
	versions := make(map[string]string, len(f.AdapterVersions))
	for k, v := range f.AdapterVersions {
		versions[k] = v
	}
	return &AgentSkillCompatibilityFingerprint{
		ContentHash:     f.ContentHash,
		AdapterVersions: versions,
		CapabilityGen:   f.CapabilityGen,
	}
}

func convertExtensionWarningsToKernel(w []AgentSkillWarning) []skill.SkillWarning {
	if len(w) == 0 {
		return nil
	}
	result := make([]skill.SkillWarning, 0, len(w))
	for _, item := range w {
		result = append(result, skill.SkillWarning{
			Code:    item.Code,
			Message: item.Message,
			Path:    item.Path,
		})
	}
	return result
}

func convertKernelWarningsToExtension(w []skill.SkillWarning) []AgentSkillWarning {
	if len(w) == 0 {
		return nil
	}
	result := make([]AgentSkillWarning, 0, len(w))
	for _, item := range w {
		result = append(result, AgentSkillWarning{
			Code:    item.Code,
			Message: item.Message,
			Path:    item.Path,
		})
	}
	return result
}

func convertExtensionErrorsToKernel(e []AgentSkillError) []skill.SkillError {
	if len(e) == 0 {
		return nil
	}
	result := make([]skill.SkillError, 0, len(e))
	for _, item := range e {
		result = append(result, skill.SkillError{
			Code:    item.Code,
			Message: item.Message,
			Path:    item.Path,
		})
	}
	return result
}

func convertKernelErrorsToExtension(e []skill.SkillError) []AgentSkillError {
	if len(e) == 0 {
		return nil
	}
	result := make([]AgentSkillError, 0, len(e))
	for _, item := range e {
		result = append(result, AgentSkillError{
			Code:    item.Code,
			Message: item.Message,
			Path:    item.Path,
		})
	}
	return result
}

func buildSkillParsedFromFiles(files map[string][]byte, rootName string) skill.ParsedSkill {
	raw, ok := files["SKILL.md"]
	if !ok {
		return skill.ParsedSkill{}
	}
	frontmatter, extra, body, _, warnings, err := parseSkillMarkdown(raw, DefaultAgentSkillLimits())
	if err != nil {
		return skill.ParsedSkill{}
	}
	_ = warnings
	return skill.ParsedSkill{
		Name:             frontmatter.Name,
		Description:      frontmatter.Description,
		License:          frontmatter.License,
		Compatibility:    frontmatter.Compatibility,
		Metadata:         frontmatter.Metadata,
		AllowedTools:     parseAllowedToolsFromString(frontmatter.AllowedTools),
		Body:             body,
		RawFrontmatter:   nil,
		ExtraFrontmatter: extra,
	}
}

func parseAllowedToolsFromString(s string) []string {
	if s == "" {
		return nil
	}
	fields := splitAllowedTools(s)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func splitAllowedTools(s string) []string {
	var result []string
	for _, tok := range splitToolTokens(s) {
		if tok != "" {
			result = append(result, tok)
		}
	}
	return result
}

func splitToolTokens(s string) []string {
	var tokens []string
	current := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == ',' {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
			continue
		}
		current += string(r)
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}
