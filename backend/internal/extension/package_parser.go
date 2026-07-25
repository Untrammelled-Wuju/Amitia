// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

func parsePackageInput(request PreviewPackageImportRequest, validator *SchemaValidator, limits PackageLimits) (parsedExtensionPackage, error) {
	if len(request.Directory) > 0 {
		if len(request.Directory) > limits.MaxFiles {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "目录文件数量超过限制", "", false, nil)
		}
		files := make(map[string][]byte, len(request.Directory))
		canonical := map[string]string{}
		var total int64
		for name, content := range request.Directory {
			clean, err := validatePackagePath(name, limits)
			if err != nil {
				return parsedExtensionPackage{}, err
			}
			if err := validatePackageFile(clean, content); err != nil {
				return parsedExtensionPackage{}, err
			}
			key := strings.ToLower(norm.NFC.String(clean))
			if previous, exists := canonical[key]; exists {
				return parsedExtensionPackage{}, NewExtensionError(ErrPackageInvalidArchive, "目录包含冲突路径", previous+" / "+clean, false, nil)
			}
			canonical[key] = clean
			if int64(len(content)) > limits.MaxFileBytes {
				return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "目录单文件超过限制", clean, false, nil)
			}
			total += int64(len(content))
			if total > limits.MaxExpandedBytes {
				return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "目录总大小超过限制", "", false, nil)
			}
			files[clean] = append([]byte(nil), content...)
		}
		raw, err := stablePackageZIP(files)
		if err != nil {
			return parsedExtensionPackage{}, err
		}
		parsed, err := parseNativeAgentSkills(files, request.RootName, AgentSkillSourceDirectory, PackageFormatAgentSkillsDir, raw)
		if err != nil {
			return parsedExtensionPackage{}, err
		}
		parsed.Source = "local-agentskills-directory"
		return parsed, nil
	}
	files, views, err := readPackageZIP(request.Raw, limits)
	if err != nil {
		return parsedExtensionPackage{}, err
	}
	if _, hasManifest := files["manifest.json"]; hasManifest || strings.EqualFold(path.Ext(request.FileName), ".amitiax") {
		if !hasManifest {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestMissing, ".amitiax 缺少 manifest.json", "", false, nil)
		}
		return parseAmitiax(request.Raw, files, views, validator, limits)
	}
	agentFiles, root, agentErr := readAgentSkillZIP(request.Raw, DefaultAgentSkillLimits())
	if agentErr != nil {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageFormatUnsupported, "无法识别本地扩展包格式", "", false, agentErr)
	}
	parsed, err := parseNativeAgentSkills(agentFiles, root, AgentSkillSourceZIP, PackageFormatAgentSkillsZIP, request.Raw)
	if err != nil {
		return parsedExtensionPackage{}, err
	}
	parsed.Source = "local-agentskills-zip"
	return parsed, nil
}

func parseNativeAgentSkills(files map[string][]byte, root string, source AgentSkillSource, format PackageFormat, raw []byte) (parsedExtensionPackage, error) {
	agent, err := parseAgentSkillFiles(files, root, source, DefaultAgentSkillLimits())
	if err != nil {
		return parsedExtensionPackage{}, err
	}
	views := make([]PackageFileView, 0, len(files))
	for name, content := range files {
		views = append(views, PackageFileView{Path: name, Size: int64(len(content)), Kind: packageFileKind(name)})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Path < views[j].Path })
	return parsedExtensionPackage{Format: format, Files: files, Raw: append([]byte(nil), raw...), PackageHash: packageHash(raw), AgentSkill: &agent, Signature: PackageSignatureView{Status: PackageSignatureUnsigned}, FileViews: views}, nil
}

func parseAmitiax(raw []byte, files map[string][]byte, views []PackageFileView, validator *SchemaValidator, limits PackageLimits) (parsedExtensionPackage, error) {
	manifestRaw := files["manifest.json"]
	if int64(len(manifestRaw)) > limits.MaxManifestBytes {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "Manifest 超过大小限制", "", false, nil)
	}
	if err := validator.ValidateManifest(manifestRaw); err != nil {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest 校验失败", err.Error(), false, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest JSON 无效", "", false, err)
	}
	if manifest.Kind != "Skill" || manifest.Entry.Kind != "workflow" && manifest.Entry.Kind != "instructions" {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageEntryUnsupported, "本地包仅支持 workflow 和 instructions Skill", manifest.Entry.Kind, false, nil)
	}
	if !skillIDPattern.MatchString(manifest.Metadata.ID) || !semverPattern.MatchString(manifest.Metadata.Version) || strings.TrimSpace(manifest.Metadata.Name) == "" || strings.TrimSpace(manifest.Metadata.Description) == "" || strings.TrimSpace(manifest.Metadata.License) == "" {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest 元数据不完整或版本无效", "", false, nil)
	}
	if manifest.Enabled {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "本地包不能要求自动启用", "", false, nil)
	}
	for _, capability := range manifest.Capabilities {
		if capability == "*" || strings.Contains(capability, "**") {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest 不能声明无限权限", capability, false, nil)
		}
		if _, ok := Capability(capability); !ok {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageCapabilityMismatch, "Manifest 包含未知 Capability", capability, false, nil)
		}
	}
	if err := validateChecksums(files); err != nil {
		return parsedExtensionPackage{}, err
	}
	signature, digest, signatureErr := verifyPackageSignature(files, false)
	if signatureErr != nil {
		return parsedExtensionPackage{}, signatureErr
	}
	parsed := parsedExtensionPackage{Format: PackageFormatAmitiax, Source: "local-amitiax", Files: files, Raw: append([]byte(nil), raw...), PackageHash: packageHash(raw), Manifest: manifest, ManifestRaw: append(json.RawMessage(nil), manifestRaw...), Signature: signature, FileViews: views, SignedDigest: digest, Schemas: map[string]json.RawMessage{}}
	allowedTop := map[string]bool{"manifest.json": true, "checksums.sha256": true, "signature.json": true, "LICENSE": true, "SBOM.spdx.json": true}
	for name := range files {
		top := strings.Split(name, "/")[0]
		if allowedTop[name] || top == "schemas" || top == "config" || top == "workflows" || top == "instructions" || top == "tests" || top == "docs" {
			continue
		}
		parsed.Warnings = append(parsed.Warnings, "未知顶层文件或目录: "+top)
	}
	if _, ok := files["LICENSE"]; !ok {
		parsed.Warnings = append(parsed.Warnings, "包内缺少 LICENSE")
	}
	if _, ok := files["SBOM.spdx.json"]; !ok {
		parsed.Warnings = append(parsed.Warnings, "包内缺少 SBOM.spdx.json")
	}
	for _, schemaName := range []string{"input.schema.json", "output.schema.json", "config.schema.json"} {
		if content, ok := files["schemas/"+schemaName]; ok {
			if int64(len(content)) > limits.MaxSchemaBytes {
				return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "Schema 超过大小限制", schemaName, false, nil)
			}
			if err := validator.ValidateSchema(schemaName, content); err != nil {
				return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Schema 无效", schemaName+": "+err.Error(), false, err)
			}
			parsed.Schemas[strings.TrimSuffix(schemaName, ".schema.json")] = append(json.RawMessage(nil), content...)
		}
	}
	if tests, ok := files["tests/cases.json"]; ok {
		if !json.Valid(tests) {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "测试用例 JSON 无效", "", false, nil)
		}
		parsed.Tests = append(json.RawMessage(nil), tests...)
	}
	if manifest.Entry.Kind == "workflow" {
		return parseAmitiaxWorkflow(parsed, limits)
	}
	return parseAmitiaxInstructions(parsed)
}

func parseAmitiaxWorkflow(parsed parsedExtensionPackage, limits PackageLimits) (parsedExtensionPackage, error) {
	content, ok := parsed.Files["workflows/main.json"]
	if !ok {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "workflow 包缺少 workflows/main.json", "", false, nil)
	}
	if int64(len(content)) > limits.MaxWorkflowBytes {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageArchiveLimit, "Workflow 超过大小限制", "", false, nil)
	}
	for name := range parsed.Files {
		if strings.HasPrefix(name, "scripts/") || strings.Contains(name, "/scripts/") {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageEntryUnsupported, "workflow 包不允许包含 scripts", name, false, nil)
		}
		if strings.HasPrefix(name, "instructions/") {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "包内不能同时存在 workflow 和 instructions", name, false, nil)
		}
	}
	var workflow WorkflowDefinition
	if err := json.Unmarshal(content, &workflow); err != nil || len(workflow.Steps) == 0 {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Workflow JSON 无效", "", false, err)
	}
	entryPath := parsed.Manifest.Entry.Path
	if entryPath != "" && entryPath != "workflows/main.json" {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest Entry 与 workflow 文件不一致", entryPath, false, nil)
	}
	parsed.Workflow = &workflow
	parsed.WorkflowRaw = append(json.RawMessage(nil), content...)
	return parsed, nil
}

func parseAmitiaxInstructions(parsed parsedExtensionPackage) (parsedExtensionPackage, error) {
	if _, ok := parsed.Files["instructions/SKILL.md"]; !ok {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "instructions 包缺少 instructions/SKILL.md", "", false, nil)
	}
	for name := range parsed.Files {
		if strings.HasPrefix(name, "workflows/") {
			return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "包内不能同时存在 workflow 和 instructions", name, false, nil)
		}
	}
	agentFiles := map[string][]byte{}
	for name, content := range parsed.Files {
		if strings.HasPrefix(name, "instructions/") {
			agentFiles[strings.TrimPrefix(name, "instructions/")] = content
		}
	}
	agent, err := parseAgentSkillFiles(agentFiles, parsed.Manifest.Metadata.Name, AgentSkillSourceZIP, DefaultAgentSkillLimits())
	if err != nil {
		return parsedExtensionPackage{}, err
	}
	if parsed.Manifest.Entry.Path != "" && parsed.Manifest.Entry.Path != "instructions/SKILL.md" {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest Entry 与 instructions 文件不一致", parsed.Manifest.Entry.Path, false, nil)
	}
	if agent.Definition.Name != parsed.Manifest.Metadata.Name {
		return parsedExtensionPackage{}, NewExtensionError(ErrPackageManifestInvalid, "Manifest 名称与 Agent Skill name 不一致", agent.Definition.Name, false, nil)
	}
	parsed.AgentSkill = &agent
	return parsed, nil
}

func packageStepSummary(workflow *WorkflowDefinition) []string {
	if workflow == nil {
		return nil
	}
	result := make([]string, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		result = append(result, fmt.Sprintf("%s:%s", step.ID, step.Type))
	}
	return result
}
