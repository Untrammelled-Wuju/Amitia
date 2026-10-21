package agent_skill

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/skill"
)

var packageCompatibilityPipeline = skill.DefaultCompatibilityPipeline()
var packageSecretPattern = regexp.MustCompile(`(?i)(?:api[_ -]?key|access[_ -]?token|secret|password)\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|sk-[A-Za-z0-9]{20,}`)

func LoadDefinitionFromPackage(ctx context.Context, packageRoot, entryPoint string, base AgentSkillDefinition) (AgentSkillDefinition, error) {
	entryPoint = filepath.ToSlash(filepath.Clean(strings.TrimSpace(entryPoint)))
	if entryPoint == "." || entryPoint == "" || filepath.IsAbs(entryPoint) || strings.HasPrefix(entryPoint, "../") {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill entrypoint is invalid")
	}
	fullEntry := filepath.Join(packageRoot, filepath.FromSlash(entryPoint))
	rootClean, err := filepath.Abs(packageRoot)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	entryClean, err := filepath.Abs(fullEntry)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	if entryClean != rootClean && !strings.HasPrefix(entryClean, rootClean+string(os.PathSeparator)) {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill entrypoint escapes package root")
	}
	info, err := os.Lstat(entryClean)
	if err != nil {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill entrypoint unavailable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill entrypoint must be a regular file")
	}
	if info.Size() > 1<<20 {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill entrypoint exceeds size limit")
	}
	raw, err := os.ReadFile(entryClean)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	raw = bytes.ReplaceAll(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), []byte("\r\n"), []byte("\n"))
	skillRoot := filepath.Dir(entryClean)
	parsed, err := skill.NewParser().ParseBytes(ctx, skill.SkillRoot{RootURI: skillRoot, Source: "package"}, skill.DefaultParsePolicy, raw)
	if err != nil {
		return AgentSkillDefinition{}, fmt.Errorf("agent skill parse failed: %w", err)
	}
	files, err := readSkillDirectory(skillRoot)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	for fileName, content := range files {
		if packageSecretPattern.Match(content) {
			return AgentSkillDefinition{}, fmt.Errorf("agent skill contains a suspected plaintext secret: %s", fileName)
		}
	}
	canonical, report, err := packageCompatibilityPipeline.Evaluate(ctx, skill.SkillPackageView{RootURI: skillRoot, SourceFile: entryPoint, Files: files, Parsed: parsed, ContentHash: parsed.ContentHash, Source: "package"}, nil)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	resources, err := scanPackageResources(packageRoot, skillRoot)
	if err != nil {
		return AgentSkillDefinition{}, err
	}
	definition := base
	definition.EntryPoint = entryPoint
	definition.Name = parsed.Name
	definition.Description = parsed.Description
	definition.DisplayName = firstNonEmpty(definition.DisplayName, parsed.Name)
	definition.License = parsed.License
	definition.Author = firstNonEmpty(definition.Author, stringValue(parsed.Metadata["author"]))
	definition.Version = firstNonEmpty(definition.Version, stringValue(parsed.Metadata["version"]), "0.0.0")
	definition.Instructions = SkillInstructionRef{
		Text:        SanitizeInstructionText(parsed.Body),
		TokenCount:  EstimateTokens(parsed.Body),
		ContentHash: parsed.ContentHash,
	}
	definition.ContentHash = parsed.ContentHash
	definition.RawSkillMD = string(raw)
	definition.RawFrontmatter = parsed.RawFrontmatter
	definition.ExtraFrontmatter = parsed.ExtraFrontmatter
	definition.Metadata = mergeMetadata(definition.Metadata, parsed.Metadata)
	definition.AllowedTools = strings.Join(parsed.AllowedTools, " ")
	definition.Resources = resources
	definition.RequiredTools = requiredTools(parsed.RawFrontmatter)
	definition.Activation = activationRule(parsed.RawFrontmatter)
	definition.TokenPolicy = tokenPolicy(parsed.RawFrontmatter, definition.Instructions.TokenCount)
	definition.Compatibility = compatibilityFromReport(report)
	definition.CompatibilityStatus = definition.Compatibility.Status
	definition.Compatible = report == nil || report.Status == skill.SkillCompatStatusCompatible
	definition.CompatibilityReport = convertCompatibilityReport(report)
	definition.Warnings = convertWarnings(report)
	definition.MCPDependencies = convertMCPDependencies(canonical)
	definition.ToolMappings = convertToolMappings(report)
	if canonical != nil {
		definition.DisplayName = firstNonEmpty(canonical.UI.DisplayName, definition.DisplayName)
		definition.ShortDescription = canonical.UI.ShortDescription
		definition.DefaultPrompt = canonical.UI.DefaultPrompt
		definition.IconSmall = canonical.UI.IconSmall
		definition.IconLarge = canonical.UI.IconLarge
		definition.BrandColor = canonical.UI.BrandColor
		if len(definition.Activation.Keywords) == 0 {
			definition.Activation.Keywords = append(definition.Activation.Keywords, canonical.ActivationHints...)
		}
	}
	definition.Scope = AgentSkillScopeGlobal
	definition.ScopeID = ""
	definition.Source = "agent_skill"
	definition.Integrity = SkillIntegrity{
		Algorithm:       "sha256",
		ContentHash:     parsed.ContentHash,
		FrontmatterHash: ComputeFrontmatterHash(parsed.Body),
		VerifiedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	definition.CreatedAt = nonZeroTime(definition.CreatedAt)
	definition.UpdatedAt = time.Now().UTC()
	definition.ID = PublicSkillID(definition.ExtensionID)
	if definition.ContributionID == "" {
		definition.ContributionID = definition.ID
	}
	return definition, nil
}

func readSkillDirectory(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if info.Size() > 20<<20 || total > 50<<20 {
			return fmt.Errorf("agent skill resources exceed size limit")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	return files, err
}

func scanPackageResources(packageRoot, skillRoot string) ([]SkillResourceDescriptor, error) {
	resources := make([]SkillResourceDescriptor, 0)
	var total int64
	err := filepath.WalkDir(skillRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == skillRoot {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "SKILL.md" && filepath.Dir(path) == skillRoot {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if info.Size() > 20<<20 || total > 50<<20 || len(resources) >= 500 {
			return fmt.Errorf("agent skill resources exceed limit")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relativeToSkill, err := filepath.Rel(skillRoot, path)
		if err != nil {
			return err
		}
		relativeToPackage, err := filepath.Rel(packageRoot, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		mimeType, _ := skill.DetectMIMEFromExtension(filepath.Ext(path))
		if mimeType == "" {
			if skill.SniffTextBytes(content) {
				mimeType = "text/plain"
			} else {
				mimeType = "application/octet-stream"
			}
		}
		textReadable := skill.IsTextMIME(mimeType) && skill.SniffTextBytes(content)
		tokenEstimate := 0
		if textReadable {
			tokenEstimate = EstimateTokens(string(content))
		}
		resources = append(resources, SkillResourceDescriptor{
			Path:          filepath.ToSlash(relativeToPackage),
			Kind:          packageResourceKind(relativeToSkill),
			MIMEType:      mimeType,
			Size:          info.Size(),
			SHA256:        hex.EncodeToString(sum[:]),
			TextReadable:  textReadable,
			TokenEstimate: tokenEstimate,
			Executable:    packageResourceKind(relativeToSkill) == KindScript,
			Supported:     true,
		})
		return nil
	})
	sort.Slice(resources, func(i, j int) bool { return resources[i].Path < resources[j].Path })
	return resources, err
}

func packageResourceKind(relative string) SkillResourceKind {
	normalized := filepath.ToSlash(strings.ToLower(relative))
	switch {
	case strings.HasPrefix(normalized, "scripts/"):
		return KindScript
	case strings.HasPrefix(normalized, "references/"):
		return KindReference
	case strings.HasPrefix(normalized, "assets/"):
		return KindAsset
	case strings.HasPrefix(normalized, "templates/"):
		return KindTemplate
	case strings.HasPrefix(normalized, "data/"):
		return KindData
	default:
		return KindConfig
	}
}

func compatibilityFromReport(report *skill.SkillCompatibilityReport) SkillCompatibility {
	if report == nil {
		return SkillCompatibility{SchemaVersion: 2, Status: skill.SkillCompatStatusCompatible}
	}
	messages := make([]string, 0, len(report.Warnings)+len(report.Errors))
	for _, warning := range report.Warnings {
		messages = append(messages, warning.Message)
	}
	for _, item := range report.Errors {
		messages = append(messages, item.Message)
	}
	status := report.Status
	if status == skill.SkillCompatStatusDegraded {
		status = "partially_compatible"
	}
	if status == skill.SkillCompatStatusBlocked {
		status = "blocked"
	}
	return SkillCompatibility{SchemaVersion: 2, Status: status, Messages: messages}
}

func convertCompatibilityReport(report *skill.SkillCompatibilityReport) *SkillCompatibilityReport {
	if report == nil {
		return nil
	}
	result := &SkillCompatibilityReport{
		Status:          report.Status,
		ToolMappings:    make([]SkillToolMapping, 0, len(report.ToolMappings)),
		RequiredScripts: append([]string(nil), report.RequiredScripts...),
		MissingFiles:    append([]string(nil), report.MissingFiles...),
		Unsupported:     append([]string(nil), report.Unsupported...),
		Warnings:        convertWarnings(report),
		EvaluatedAt:     report.EvaluatedAt,
	}
	for _, item := range report.Detected {
		result.DetectedProfiles = append(result.DetectedProfiles, map[string]any{"id": item.ID, "version": item.Version, "evidence": append([]string(nil), item.Evidence...)})
	}
	for _, item := range report.FieldMappings {
		result.FieldMappings = append(result.FieldMappings, map[string]any{"profile": item.Profile, "source": item.Source, "target": item.Target, "state": item.State, "reason": item.Reason})
	}
	for _, item := range report.ToolMappings {
		result.ToolMappings = append(result.ToolMappings, SkillToolMapping{SourceTool: item.SourceTool, TargetSkillID: item.TargetSkillID, Status: item.Status, Reason: item.Reason})
	}
	for _, item := range report.MappedFeatures {
		result.MappedFeatures = append(result.MappedFeatures, map[string]any{"profile": item.Profile, "feature": item.Feature, "state": item.State, "reason": item.Reason})
	}
	for _, item := range report.DependencyMappings {
		result.DependencyMappings = append(result.DependencyMappings, map[string]any{"profile": item.Profile, "resolvedAs": item.ResolvedAs, "state": item.State, "reason": item.Reason})
	}
	for _, item := range report.Errors {
		result.Errors = append(result.Errors, SkillError{Code: item.Code, Message: item.Message, Path: item.Path})
	}
	if report.Fingerprint != nil {
		result.Fingerprint = map[string]any{"contentHash": report.Fingerprint.ContentHash, "adapterVersions": report.Fingerprint.AdapterVersions, "capabilityGeneration": report.Fingerprint.CapabilityGen}
	}
	return result
}

func convertWarnings(report *skill.SkillCompatibilityReport) []SkillWarning {
	if report == nil {
		return nil
	}
	result := make([]SkillWarning, 0, len(report.Warnings))
	for _, item := range report.Warnings {
		result = append(result, SkillWarning{Code: item.Code, Message: item.Message, Path: item.Path})
	}
	return result
}

func convertMCPDependencies(canonical *skill.CanonicalSkillCompatibility) []SkillMCPDependency {
	if canonical == nil {
		return nil
	}
	result := make([]SkillMCPDependency, 0, len(canonical.MCPDependencies))
	for _, item := range canonical.MCPDependencies {
		result = append(result, SkillMCPDependency{
			ID: item.ID, Description: item.Description, Required: item.Required, Transport: item.Transport,
			URL: item.URL, Command: item.Command, Args: append([]string(nil), item.Args...), AuthType: item.AuthType,
			ToolAllowlist: append([]string(nil), item.ToolAllowlist...), DefaultScope: item.DefaultScope,
			AutoConfigure: item.AutoConfigure, AutoEnable: item.AutoEnable, RequiresManualConfirmation: item.RequiresManualConfirmation,
		})
	}
	return result
}

func convertToolMappings(report *skill.SkillCompatibilityReport) []map[string]any {
	if report == nil {
		return nil
	}
	result := make([]map[string]any, 0, len(report.ToolMappings))
	for _, item := range report.ToolMappings {
		result = append(result, map[string]any{"sourceTool": item.SourceTool, "targetSkillId": item.TargetSkillID, "status": item.Status, "reason": item.Reason})
	}
	return result
}

func requiredTools(frontmatter map[string]interface{}) []ToolReference {
	raw, ok := frontmatter["requiredTools"]
	if !ok {
		return nil
	}
	items, _ := raw.([]interface{})
	result := make([]ToolReference, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			result = append(result, ToolReference{ToolID: value, Required: true})
		case map[string]interface{}:
			result = append(result, ToolReference{ToolID: stringValue(value["id"]), Required: boolValue(value["required"], true), Constraint: stringValue(value["constraint"])})
		}
	}
	return result
}

func activationRule(frontmatter map[string]interface{}) ActivationRule {
	rule := ActivationRule{Mode: ActivationManual}
	raw, _ := frontmatter["activation"].(map[string]interface{})
	if mode := strings.ToLower(stringValue(raw["mode"])); mode != "" {
		switch mode {
		case "auto":
			rule.Mode = ActivationAuto
		case "explicit":
			rule.Mode = ActivationExplicit
		}
	}
	for _, item := range stringSlice(raw["keywords"]) {
		rule.Keywords = append(rule.Keywords, item)
	}
	rule.Priority = intValue(raw["priority"])
	rule.MaxPerRound = intValue(raw["maxPerRound"])
	return rule
}

func tokenPolicy(frontmatter map[string]interface{}, instructionTokens int) SkillTokenPolicy {
	policy := SkillTokenPolicy{MaxInstructionTokens: max(instructionTokens, 2000)}
	raw, _ := frontmatter["tokenPolicy"].(map[string]interface{})
	if value := intValue(raw["maxInstructionTokens"]); value > 0 {
		policy.MaxInstructionTokens = value
	}
	policy.MaxResourceTokensPerTurn = intValue(raw["maxResourceTokensPerTurn"])
	policy.MaxTotalResources = intValue(raw["maxTotalResources"])
	policy.TruncationStrategy = stringValue(raw["truncationStrategy"])
	return policy
}

func mergeMetadata(base map[string]any, values map[string]string) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		result[key] = value
	}
	for key, value := range values {
		result[key] = value
	}
	return result
}

func nonZeroTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func boolValue(value any, fallback bool) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return fallback
}

func stringSlice(value any) []string {
	items, _ := value.([]interface{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := stringValue(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}
