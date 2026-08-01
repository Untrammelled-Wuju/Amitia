//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"regexp"
)

func LegacyStableJSON(raw json.RawMessage) json.RawMessage {
	return stableJSON(raw)
}

func LegacyArtifactChecksum(artifact extensionArtifactRecord) string {
	return artifactChecksum(artifact)
}

func LegacyIsMigrationToolContext(ctx context.Context) bool {
	return isLegacyMigrationToolContext(ctx)
}

func LegacyArtifactBase(record packageArtifactRecord) extensionArtifactRecord {
	return record.base()
}

func LegacySkillDefinitionFromManifest(manifest Manifest, schemas map[string]json.RawMessage) SkillDefinition {
	return skillDefinitionFromManifest(manifest, schemas)
}

func LegacyDecodeAgentSkillArtifact(raw []byte, limits AgentSkillLimits) (map[string][]byte, error) {
	return decodeAgentSkillArtifact(raw, limits)
}

func LegacyEncodeAgentSkillArtifact(files map[string][]byte) ([]byte, error) {
	return encodeAgentSkillArtifact(files)
}

func LegacyParseAgentSkillFiles(files map[string][]byte, rootName string, source AgentSkillSource, limits AgentSkillLimits) (parsedAgentSkill, error) {
	return parseAgentSkillFiles(files, rootName, source, limits)
}

func LegacyParseNativeAgentSkills(files map[string][]byte, root string, source AgentSkillSource, format PackageFormat, raw []byte) (parsedExtensionPackage, error) {
	return parseNativeAgentSkills(files, root, source, format, raw)
}

func LegacyDefaultAgentSkillLimits() AgentSkillLimits {
	return DefaultAgentSkillLimits()
}

func LegacyBuildAgentSkillManifest(definition AgentSkillDefinition, version string) SkillDefinition {
	return buildAgentSkillManifest(definition, version)
}

func LegacySecretPattern() *regexp.Regexp {
	return secretPattern
}

func LegacySemverPattern() *regexp.Regexp {
	return semverPattern
}

func LegacyParsePackageInput(request PreviewPackageImportRequest, validator *SchemaValidator, limits PackageLimits) (parsedExtensionPackage, error) {
	return parsePackageInput(request, validator, limits)
}

func LegacyTruncateLegacyMigrationFailure(value string) string {
	return truncateLegacyMigrationFailure(value)
}
