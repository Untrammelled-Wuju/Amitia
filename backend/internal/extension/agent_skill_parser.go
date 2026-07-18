package extension

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"golang.org/x/text/unicode/norm"
)

var agentSkillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var windowsDrivePattern = regexp.MustCompile(`^[A-Za-z]:`)
var markdownLinkPattern = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
var textPathPattern = regexp.MustCompile("(?:^|[\\s'\\\"])(references|assets|scripts)/[A-Za-z0-9._/ -]+")
var brandColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type parsedAgentSkill struct {
	Definition AgentSkillDefinition
	Report     AgentSkillCompatibilityReport
	Files      map[string][]byte
}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

func parseAgentSkillFiles(files map[string][]byte, rootName string, source AgentSkillSource, limits AgentSkillLimits) (parsedAgentSkill, error) {
	for name, content := range files {
		if secretPattern.Match(content) {
			return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillArtifactInvalid, "Agent Skill contains a suspected plaintext secret", name, false, nil)
		}
	}
	raw, ok := files["SKILL.md"]
	if !ok {
		return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillMissingSkillMD, "SKILL.md is missing", "", false, nil)
	}
	if int64(len(raw)) > limits.MaxSkillMDBytes {
		return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillArchiveLimit, "SKILL.md exceeds size limit", "", false, nil)
	}
	frontmatter, extra, body, rawFrontmatter, warnings, err := parseSkillMarkdown(raw, limits)
	if err != nil {
		return parsedAgentSkill{}, err
	}
	if !agentSkillNamePattern.MatchString(frontmatter.Name) || len(frontmatter.Name) > 64 || reservedAgentSkillName(frontmatter.Name) {
		return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillNameInvalid, "Agent Skill name is invalid", frontmatter.Name, false, nil)
	}
	if rootName != "" && rootName != frontmatter.Name {
		return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillNameMismatch, "Agent Skill name does not match root directory", rootName, false, nil)
	}
	if err := validateAgentSkillDescription(frontmatter.Description); err != nil {
		return parsedAgentSkill{}, err
	}
	if len(frontmatter.Compatibility) > 500 {
		return parsedAgentSkill{}, NewExtensionError(ErrAgentSkillFrontmatter, "compatibility exceeds size limit", "", false, nil)
	}
	if !descriptionHasTrigger(frontmatter.Description) {
		warnings = append(warnings, AgentSkillWarning{Code: "DESCRIPTION_TRIGGER_MISSING", Message: "description should describe when to use the skill", Path: "SKILL.md"})
	}
	resources, scanWarnings, err := scanAgentSkillResources(files, limits)
	if err != nil {
		return parsedAgentSkill{}, err
	}
	warnings = append(warnings, scanWarnings...)
	openai, openaiWarnings, err := parseAgentSkillOpenAI(files, limits)
	if err != nil {
		return parsedAgentSkill{}, err
	}
	warnings = append(warnings, openaiWarnings...)
	amitiaDependencies, amitiaWarnings, err := parseAgentSkillAmitia(files, limits)
	if err != nil {
		return parsedAgentSkill{}, err
	}
	warnings = append(warnings, amitiaWarnings...)
	dependencies := []AgentSkillMCPDependency{}
	if len(amitiaDependencies) > 0 {
		dependencies = amitiaDependencies
	} else if openai != nil {
		dependencies = openai.Dependencies
	}
	mappings := mapAgentSkillTools(frontmatter.AllowedTools)
	report := analyzeAgentSkillCompatibility(body, files, resources, mappings, warnings)
	metadataRaw, _ := json.Marshal(rawFrontmatter)
	extraRaw, _ := json.Marshal(extra)
	hash := hashAgentSkillFiles(files)
	definition := AgentSkillDefinition{Name: frontmatter.Name, Description: frontmatter.Description, License: frontmatter.License, Compatibility: frontmatter.Compatibility, Metadata: frontmatter.Metadata, AllowedTools: frontmatter.AllowedTools, Source: source, ContentHash: hash, Body: body, RawSkillMD: string(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})), RawFrontmatter: metadataRaw, ExtraFrontmatter: extraRaw, Resources: resources, ToolMappings: mappings, CompatibilityStatus: report.Status, Warnings: report.Warnings, MCPDependencies: dependencies, Enabled: false}
	if openai != nil {
		definition.DisplayName = openai.DisplayName
		definition.ShortDescription = openai.ShortDescription
		definition.DefaultPrompt = openai.DefaultPrompt
		definition.IconSmall = openai.IconSmall
		definition.IconLarge = openai.IconLarge
		definition.BrandColor = openai.BrandColor
		definition.OpenAIMetadata, _ = json.Marshal(openai.Raw)
	}
	return parsedAgentSkill{Definition: definition, Report: report, Files: files}, nil
}

func parseSkillMarkdown(raw []byte, limits AgentSkillLimits) (skillFrontmatter, map[string]interface{}, string, map[string]interface{}, []AgentSkillWarning, error) {
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "SKILL.md must be valid UTF-8", "", false, nil)
	}
	if !bytes.HasPrefix(raw, []byte("---\n")) && !bytes.HasPrefix(raw, []byte("---\r\n")) {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter must be at the start of SKILL.md", "", false, nil)
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	closing := strings.Index(normalized[4:], "\n---\n")
	if closing < 0 {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter closing delimiter is missing", "", false, nil)
	}
	closing += 4
	frontRaw := normalized[4:closing]
	if int64(len(frontRaw)) > limits.MaxFrontmatterBytes {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter exceeds size limit", "", false, nil)
	}
	body := normalized[closing+5:]
	if strings.TrimSpace(body) == "" {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "SKILL.md body is required", "", false, nil)
	}
	var node yaml.Node
	if err := decodeSafeYAML([]byte(frontRaw), &node, limits.MaxYAMLDepth); err != nil {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter is invalid", err.Error(), false, err)
	}
	var values map[string]interface{}
	if err := node.Decode(&values); err != nil {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter is invalid", err.Error(), false, err)
	}
	var fm skillFrontmatter
	if err := node.Decode(&fm); err != nil {
		return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "frontmatter fields are invalid", err.Error(), false, err)
	}
	if fm.Metadata == nil {
		fm.Metadata = map[string]string{}
	}
	extra := map[string]interface{}{}
	warnings := []AgentSkillWarning{}
	known := map[string]bool{"name": true, "description": true, "license": true, "compatibility": true, "metadata": true, "allowed-tools": true}
	for key, value := range values {
		if !known[key] {
			extra[key] = value
			warnings = append(warnings, AgentSkillWarning{Code: "UNKNOWN_FRONTMATTER_FIELD", Message: "unknown frontmatter field is preserved but not executed", Path: key})
		}
	}
	for key, value := range fm.Metadata {
		if len(key) == 0 || len(key) > 128 || len(value) > 1024 {
			return skillFrontmatter{}, nil, "", nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "metadata key or value exceeds limit", key, false, nil)
		}
	}
	if version := fm.Metadata["version"]; version != "" && !semverPattern.MatchString(version) {
		warnings = append(warnings, AgentSkillWarning{Code: "SOURCE_VERSION_INVALID", Message: "metadata.version is not valid SemVer", Path: "metadata.version"})
	}
	return fm, extra, body, values, warnings, nil
}

func decodeSafeYAML(raw []byte, node *yaml.Node, maxDepth int) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(node); err != nil {
		return err
	}
	var second yaml.Node
	if err := decoder.Decode(&second); err != io.EOF {
		return fmt.Errorf("multiple YAML documents are not allowed")
	}
	return validateYAMLNode(node, 0, maxDepth)
}

func validateYAMLNode(node *yaml.Node, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("YAML nesting exceeds limit")
	}
	if node.Kind == yaml.AliasNode || node.Alias != nil || node.Anchor != "" {
		return fmt.Errorf("YAML anchors and aliases are not allowed")
	}
	if node.Tag != "" && !strings.HasPrefix(node.Tag, "!!") {
		return fmt.Errorf("custom YAML tags are not allowed")
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if key == "<<" {
				return fmt.Errorf("YAML merge keys are not allowed")
			}
			if seen[key] {
				return fmt.Errorf("duplicate YAML key: %s", key)
			}
			seen[key] = true
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLNode(child, depth+1, maxDepth); err != nil {
			return err
		}
	}
	return nil
}

func readAgentSkillZIP(raw []byte, limits AgentSkillLimits) (map[string][]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "invalid ZIP archive", err.Error(), false, err)
	}
	if len(reader.File) > limits.MaxFiles {
		return nil, "", NewExtensionError(ErrAgentSkillArchiveLimit, "archive file count exceeds limit", "", false, nil)
	}
	files := map[string][]byte{}
	canonical := map[string]string{}
	root := ""
	var total int64
	for _, file := range reader.File {
		name, err := validateAgentSkillPath(file.Name, limits)
		if err != nil {
			return nil, "", err
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "symbolic links are not allowed", name, false, nil)
		}
		if mode&os.ModeType != 0 && !mode.IsDir() {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "archive contains unsupported file type", name, false, nil)
		}
		parts := strings.Split(name, "/")
		if root == "" {
			root = parts[0]
		} else if root != parts[0] {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "archive must contain a single root directory", "", false, nil)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		rel := strings.Join(parts[1:], "/")
		if rel == "" {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "files must be inside the skill root directory", name, false, nil)
		}
		key := strings.ToLower(norm.NFC.String(rel))
		if existing, ok := canonical[key]; ok {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "archive contains colliding paths", existing+" and "+rel, false, nil)
		}
		canonical[key] = rel
		size := int64(file.UncompressedSize64)
		if size > limits.MaxResourceBytes || (rel == "SKILL.md" && size > limits.MaxSkillMDBytes) {
			return nil, "", NewExtensionError(ErrAgentSkillArchiveLimit, "archive file exceeds size limit", rel, false, nil)
		}
		if file.CompressedSize64 > 0 && float64(file.UncompressedSize64)/float64(file.CompressedSize64) > limits.MaxCompressionRatio {
			return nil, "", NewExtensionError(ErrAgentSkillArchiveLimit, "archive compression ratio exceeds limit", rel, false, nil)
		}
		total += size
		if total > limits.MaxExpandedBytes {
			return nil, "", NewExtensionError(ErrAgentSkillArchiveLimit, "archive expanded size exceeds limit", "", false, nil)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, "", err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, size+1))
		rc.Close()
		if readErr != nil || int64(len(content)) != size {
			return nil, "", NewExtensionError(ErrAgentSkillInvalidArchive, "archive entry could not be read", rel, false, readErr)
		}
		files[rel] = content
	}
	if _, ok := files["SKILL.md"]; !ok {
		return nil, "", NewExtensionError(ErrAgentSkillMissingSkillMD, "SKILL.md is missing", "", false, nil)
	}
	return files, root, nil
}

func validateAgentSkillPath(input string, limits AgentSkillLimits) (string, error) {
	if !utf8.ValidString(input) || strings.ContainsRune(input, 0) || strings.Contains(input, "\\") || strings.HasPrefix(input, "/") || strings.HasPrefix(input, "//") || windowsDrivePattern.MatchString(input) {
		return "", NewExtensionError(ErrAgentSkillPathTraversal, "unsafe archive path", input, false, nil)
	}
	for _, component := range strings.Split(input, "/") {
		if component == ".." || component == "." {
			return "", NewExtensionError(ErrAgentSkillPathTraversal, "archive path contains traversal component", input, false, nil)
		}
	}
	cleaned := path.Clean(input)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", NewExtensionError(ErrAgentSkillPathTraversal, "archive path escapes skill root", input, false, nil)
	}
	parts := strings.Split(cleaned, "/")
	if len(parts) > limits.MaxDepth {
		return "", NewExtensionError(ErrAgentSkillArchiveLimit, "archive path depth exceeds limit", input, false, nil)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, " ") || strings.HasSuffix(part, ".") || windowsReservedName(part) {
			return "", NewExtensionError(ErrAgentSkillPathTraversal, "unsafe archive path component", part, false, nil)
		}
	}
	return norm.NFC.String(cleaned), nil
}

func scanAgentSkillResources(files map[string][]byte, limits AgentSkillLimits) ([]AgentSkillResource, []AgentSkillWarning, error) {
	paths := make([]string, 0, len(files))
	for item := range files {
		paths = append(paths, item)
	}
	sort.Strings(paths)
	resources := make([]AgentSkillResource, 0, len(paths))
	warnings := []AgentSkillWarning{}
	for _, item := range paths {
		clean, err := validateAgentSkillRelativePath(item, limits)
		if err != nil {
			return nil, nil, err
		}
		content := files[item]
		kind := AgentSkillResourceOther
		switch {
		case clean == "SKILL.md":
			kind = AgentSkillResourceSkill
		case strings.HasPrefix(clean, "references/"):
			kind = AgentSkillResourceReference
		case strings.HasPrefix(clean, "assets/"):
			kind = AgentSkillResourceAsset
		case strings.HasPrefix(clean, "scripts/"):
			kind = AgentSkillResourceScript
		case clean == "agents/openai.yaml":
			kind = AgentSkillResourceAgentMetadata
		}
		text := supportedTextResource(clean, content)
		detectedMIME := http.DetectContentType(content)
		extensionMIME := mime.TypeByExtension(strings.ToLower(path.Ext(clean)))
		mimeType := detectedMIME
		if text && extensionMIME != "" {
			mimeType = extensionMIME
		} else if detectedMIME == "application/octet-stream" && extensionMIME != "" {
			mimeType = extensionMIME
		}
		if text && int64(len(content)) > limits.MaxTextResourceBytes {
			return nil, nil, NewExtensionError(ErrAgentSkillArchiveLimit, "text resource exceeds size limit", clean, false, nil)
		}
		hash := sha256.Sum256(content)
		resources = append(resources, AgentSkillResource{Path: clean, Kind: kind, MIMEType: mimeType, Size: int64(len(content)), SHA256: hex.EncodeToString(hash[:]), TextReadable: text, Executable: false, Supported: text || kind == AgentSkillResourceAsset})
		if kind == AgentSkillResourceScript {
			warnings = append(warnings, AgentSkillWarning{Code: "SCRIPT_EXECUTION_DISABLED", Message: "script is imported for inspection only and cannot execute", Path: clean})
		}
	}
	return resources, warnings, nil
}

func validateAgentSkillRelativePath(input string, limits AgentSkillLimits) (string, error) {
	wrapper := "root/" + input
	clean, err := validateAgentSkillPath(wrapper, limits)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(clean, "root/"), nil
}

func validateAgentSkillDescription(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(value) > 1024 || strings.ContainsRune(value, 0) {
		return NewExtensionError(ErrAgentSkillDescription, "Agent Skill description is invalid", "", false, nil)
	}
	for _, r := range value {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return NewExtensionError(ErrAgentSkillDescription, "Agent Skill description contains control characters", "", false, nil)
		}
	}
	return nil
}

func descriptionHasTrigger(v string) bool {
	lower := strings.ToLower(v)
	for _, s := range []string{"use when", "when ", "用于", "适用", "当用户", "触发"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
func reservedAgentSkillName(v string) bool {
	return v == "agent-skill-activate" || strings.HasPrefix(v, "dev-amitia") || v == "system" || v == "admin"
}
func windowsReservedName(v string) bool {
	base := strings.ToUpper(strings.TrimSuffix(v, path.Ext(v)))
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	for i := 1; i <= 9; i++ {
		if base == fmt.Sprintf("COM%d", i) || base == fmt.Sprintf("LPT%d", i) {
			return true
		}
	}
	return false
}
func supportedTextResource(name string, content []byte) bool {
	ext := strings.ToLower(path.Ext(name))
	allowed := map[string]bool{".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true, ".csv": true, ".tsv": true, ".html": true, ".css": true, ".js": true, ".ts": true, ".py": true, ".sh": true, ".ps1": true, ".go": true, ".xml": true, ".svg": true}
	return allowed[ext] && utf8.Valid(content) && !bytes.Contains(content, []byte{0})
}
func hashAgentSkillFiles(files map[string][]byte) string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(files[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func mapAgentSkillTools(value string) []AgentSkillToolMapping {
	if strings.TrimSpace(value) == "" {
		return []AgentSkillToolMapping{}
	}
	tokens := regexp.MustCompile(`[^\s,]+(?:\([^)]*\))?`).FindAllString(value, -1)
	result := make([]AgentSkillToolMapping, 0, len(tokens))
	for _, token := range tokens {
		lower := strings.ToLower(token)
		mapping := AgentSkillToolMapping{SourceTool: token, Status: "unsupported", Reason: "tool is not available in Amitia"}
		switch {
		case lower == "read":
			mapping.TargetSkillID = "dev.amitia.skill.agent-skill-read-resource"
			mapping.Status = "mapped"
			mapping.Reason = "restricted to resources of the active Agent Skill"
		case lower == "websearch" || lower == "web-search":
			mapping.TargetSkillID = "web_search"
			mapping.Status = "partially_mapped"
			mapping.Reason = "available only when the existing Amitia skill and permission policy allow it"
		case strings.HasPrefix(lower, "bash") || lower == "shell" || lower == "powershell" || lower == "python" || lower == "node":
			mapping.Status = "blocked"
			mapping.Reason = "arbitrary code execution is disabled"
		case strings.HasPrefix(lower, "mcp("):
			mapping.Reason = "MCP dependencies are compatibility metadata only and are never connected automatically"
		}
		result = append(result, mapping)
	}
	return result
}

func analyzeAgentSkillCompatibility(body string, files map[string][]byte, resources []AgentSkillResource, mappings []AgentSkillToolMapping, warnings []AgentSkillWarning) AgentSkillCompatibilityReport {
	report := AgentSkillCompatibilityReport{Status: AgentSkillCompatible, ToolMappings: mappings, RequiredScripts: []string{}, MissingFiles: []string{}, Unsupported: []string{}, Warnings: append([]AgentSkillWarning{}, warnings...), Errors: []AgentSkillError{}}
	refs := map[string]bool{}
	referencedScripts := map[string]bool{}
	for _, m := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		if len(m) > 1 {
			refs[strings.TrimSpace(strings.Split(m[1], "#")[0])] = true
		}
	}
	for _, m := range textPathPattern.FindAllString(body, -1) {
		ref := strings.TrimSpace(strings.TrimLeft(m, " \t\r\n'\"`"))
		refs[strings.TrimRight(ref, ".,;:!?)]}")] = true
	}
	for ref := range refs {
		if strings.Contains(ref, "://") {
			if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
				report.Errors = append(report.Errors, AgentSkillError{Code: "DANGEROUS_URI_SCHEME", Message: "dangerous URI scheme is not allowed", Path: ref})
			}
			continue
		}
		clean, err := validateAgentSkillRelativePath(ref, DefaultAgentSkillLimits())
		if err != nil {
			report.Errors = append(report.Errors, AgentSkillError{Code: ErrAgentSkillPathTraversal, Message: "reference escapes skill root", Path: ref})
			continue
		}
		if _, ok := files[clean]; !ok {
			report.MissingFiles = append(report.MissingFiles, clean)
			report.Warnings = append(report.Warnings, AgentSkillWarning{Code: "MISSING_REFERENCE", Message: "referenced file is missing", Path: clean})
		}
		if strings.HasPrefix(clean, "scripts/") {
			report.RequiredScripts = append(report.RequiredScripts, clean)
			referencedScripts[clean] = true
		}
	}
	lower := strings.ToLower(body)
	dangerous := []struct{ needle, code string }{{"ignore previous", "PROMPT_OVERRIDE"}, {"忽略之前", "PROMPT_OVERRIDE"}, {"system prompt", "SYSTEM_PROMPT_LEAK"}, {"系统提示词", "SYSTEM_PROMPT_LEAK"}, {"api key", "SECRET_ACCESS"}, {"environment variable", "ENVIRONMENT_ACCESS"}, {"环境变量", "ENVIRONMENT_ACCESS"}, {"execute shell", "SHELL_REQUIRED"}, {"执行 shell", "SHELL_REQUIRED"}, {"powershell", "POWERSHELL_REQUIRED"}, {"python ", "PYTHON_REQUIRED"}, {"node ", "NODE_REQUIRED"}, {"local filesystem", "LOCAL_FILESYSTEM_REQUIRED"}, {"本地文件", "LOCAL_FILESYSTEM_REQUIRED"}, {"install software", "SYSTEM_SOFTWARE_REQUIRED"}, {"安装软件", "SYSTEM_SOFTWARE_REQUIRED"}, {"download binary", "DOWNLOADED_BINARY_REQUIRED"}, {"下载二进制", "DOWNLOADED_BINARY_REQUIRED"}, {"send message", "CHANNEL_SEND_REQUIRED"}, {"发送消息", "CHANNEL_SEND_REQUIRED"}, {"http://", "NETWORK_REQUIRED"}, {"https://", "NETWORK_REQUIRED"}, {"bypass permission", "PERMISSION_BYPASS"}, {"绕过权限", "PERMISSION_BYPASS"}, {"cross-role", "CROSS_ROLE_ACCESS"}, {"跨角色", "CROSS_ROLE_ACCESS"}, {"database", "DATABASE_ACCESS"}}
	for _, item := range dangerous {
		if strings.Contains(lower, item.needle) {
			report.Warnings = append(report.Warnings, AgentSkillWarning{Code: item.code, Message: "skill content requests a potentially unsafe capability", Path: "SKILL.md"})
		}
	}
	if estimateTokens(body) > DefaultAgentSkillLimits().MaxBodyTokens {
		report.Warnings = append(report.Warnings, AgentSkillWarning{Code: ErrAgentSkillPromptLimit, Message: "skill body exceeds the activation token limit", Path: "SKILL.md"})
	}
	for _, needle := range []string{"<script", "<iframe", "javascript:", "onload=", "onerror="} {
		if strings.Contains(lower, needle) {
			report.Errors = append(report.Errors, AgentSkillError{Code: "DANGEROUS_HTML", Message: "skill Markdown contains active HTML content", Path: "SKILL.md"})
			break
		}
	}
	for _, mapping := range mappings {
		if mapping.Status == "unsupported" || mapping.Status == "blocked" {
			report.Unsupported = append(report.Unsupported, mapping.SourceTool)
		}
	}
	scripts := 0
	for _, r := range resources {
		if r.Kind == AgentSkillResourceScript {
			scripts++
			if !referencedScripts[r.Path] {
				report.Warnings = append(report.Warnings, AgentSkillWarning{Code: "OPTIONAL_SCRIPT", Message: "script is present but is not referenced by the core instructions", Path: r.Path})
			}
		}
		if r.Kind == AgentSkillResourceAsset && strings.Contains(strings.ToLower(r.MIMEType), "svg") && unsafeSVG(files[r.Path]) {
			report.Errors = append(report.Errors, AgentSkillError{Code: "UNSAFE_SVG", Message: "SVG contains active or external content", Path: r.Path})
		}
	}
	if len(report.Errors) > 0 || strings.Contains(lower, "reveal system prompt") || strings.Contains(lower, "leak secret") {
		report.Status = AgentSkillBlocked
	} else if len(report.RequiredScripts) > 0 {
		report.Status = AgentSkillPartiallyCompatible
	} else if scripts > 0 || len(report.Unsupported) > 0 || len(report.Warnings) > 0 {
		report.Status = AgentSkillCompatibleWarnings
	}
	sort.Strings(report.RequiredScripts)
	sort.Strings(report.MissingFiles)
	sort.Strings(report.Unsupported)
	return report
}

type parsedOpenAIYAML struct {
	DisplayName      string
	ShortDescription string
	IconSmall        string
	IconLarge        string
	BrandColor       string
	DefaultPrompt    string
	Raw              map[string]interface{}
	Dependencies     []AgentSkillMCPDependency
}

func parseAgentSkillOpenAI(files map[string][]byte, limits AgentSkillLimits) (*parsedOpenAIYAML, []AgentSkillWarning, error) {
	raw, ok := files["agents/openai.yaml"]
	if !ok {
		return nil, nil, nil
	}
	if int64(len(raw)) > limits.MaxTextResourceBytes {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/openai.yaml exceeds size limit", "", false, nil)
	}
	var node yaml.Node
	if err := decodeSafeYAML(raw, &node, limits.MaxYAMLDepth); err != nil {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/openai.yaml is invalid", err.Error(), false, err)
	}
	var value struct {
		Interface struct {
			DisplayName      string `yaml:"display_name"`
			ShortDescription string `yaml:"short_description"`
			IconSmall        string `yaml:"icon_small"`
			IconLarge        string `yaml:"icon_large"`
			BrandColor       string `yaml:"brand_color"`
			DefaultPrompt    string `yaml:"default_prompt"`
		} `yaml:"interface"`
		Dependencies struct {
			Tools []map[string]interface{} `yaml:"tools"`
		} `yaml:"dependencies"`
	}
	if err := node.Decode(&value); err != nil {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/openai.yaml fields are invalid", err.Error(), false, err)
	}
	var rawMap map[string]interface{}
	_ = node.Decode(&rawMap)
	warnings := []AgentSkillWarning{}
	if value.Interface.BrandColor != "" && !brandColorPattern.MatchString(value.Interface.BrandColor) {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "brand_color is invalid", value.Interface.BrandColor, false, nil)
	}
	for _, icon := range []string{value.Interface.IconSmall, value.Interface.IconLarge} {
		if icon == "" {
			continue
		}
		clean := strings.TrimPrefix(icon, "./")
		if strings.Contains(icon, ":") || !strings.HasPrefix(clean, "assets/") {
			return nil, nil, NewExtensionError(ErrAgentSkillPathTraversal, "icon must be a local assets path", icon, false, nil)
		}
		if _, ok := files[clean]; !ok {
			return nil, nil, NewExtensionError(ErrAgentSkillResourceNotFound, "icon file is missing", clean, false, nil)
		}
	}
	dependencies := []AgentSkillMCPDependency{}
	for _, item := range value.Dependencies.Tools {
		typeName, _ := item["type"].(string)
		if strings.ToLower(strings.TrimSpace(typeName)) != "mcp" {
			return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "only MCP tool dependencies are supported", "agents/openai.yaml", false, nil)
		}
		valueName, _ := item["value"].(string)
		description, _ := item["description"].(string)
		transportName, _ := item["transport"].(string)
		endpoint, _ := item["url"].(string)
		dependency := AgentSkillMCPDependency{ID: valueName, Description: description, Required: true, Transport: transportName, URL: endpoint, AuthType: "none", ToolAllowlist: []string{}, DefaultScope: "character", AutoConfigure: false, AutoEnable: false, RequiresManualConfirmation: true}
		if dependency.Transport == "" && dependency.URL != "" {
			dependency.Transport = "streamable_http"
		}
		if err := validateMCPDependency(dependency); err != nil {
			return nil, nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	if len(dependencies) > 0 {
		warnings = append(warnings, AgentSkillWarning{Code: "MCP_DEPENDENCY_REQUIRES_CONFIRMATION", Message: "MCP dependencies require an explicit installation plan and user confirmation", Path: "agents/openai.yaml"})
	}
	return &parsedOpenAIYAML{DisplayName: value.Interface.DisplayName, ShortDescription: value.Interface.ShortDescription, IconSmall: strings.TrimPrefix(value.Interface.IconSmall, "./"), IconLarge: strings.TrimPrefix(value.Interface.IconLarge, "./"), BrandColor: value.Interface.BrandColor, DefaultPrompt: value.Interface.DefaultPrompt, Raw: rawMap, Dependencies: dependencies}, warnings, nil
}

func parseAgentSkillAmitia(files map[string][]byte, limits AgentSkillLimits) ([]AgentSkillMCPDependency, []AgentSkillWarning, error) {
	raw, ok := files["agents/amitia.yaml"]
	if !ok {
		return nil, nil, nil
	}
	if int64(len(raw)) > limits.MaxTextResourceBytes {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/amitia.yaml exceeds size limit", "", false, nil)
	}
	var node yaml.Node
	if err := decodeSafeYAML(raw, &node, limits.MaxYAMLDepth); err != nil {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/amitia.yaml is invalid", err.Error(), false, err)
	}
	var value struct {
		Version      string `yaml:"version"`
		Dependencies []struct {
			ID          string   `yaml:"id"`
			Description string   `yaml:"description"`
			Required    *bool    `yaml:"required"`
			Transport   string   `yaml:"transport"`
			URL         string   `yaml:"url"`
			Command     string   `yaml:"command"`
			Args        []string `yaml:"args"`
			Auth        struct {
				Type string `yaml:"type"`
			} `yaml:"auth"`
			Tools struct {
				Allow []string `yaml:"allow"`
			} `yaml:"tools"`
			Scope struct {
				Default string `yaml:"default"`
			} `yaml:"scope"`
			Install struct {
				AutoConfigure              bool `yaml:"auto_configure"`
				AutoEnable                 bool `yaml:"auto_enable"`
				RequiresManualConfirmation bool `yaml:"requires_manual_confirmation"`
			} `yaml:"install"`
		} `yaml:"mcp_dependencies"`
	}
	if err := node.Decode(&value); err != nil || value.Version != "1" {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "agents/amitia.yaml version is invalid", "", false, err)
	}
	if len(value.Dependencies) > 20 {
		return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "too many MCP dependencies", "", false, nil)
	}
	dependencies := make([]AgentSkillMCPDependency, 0, len(value.Dependencies))
	seen := map[string]bool{}
	for _, item := range value.Dependencies {
		required := true
		if item.Required != nil {
			required = *item.Required
		}
		dependency := AgentSkillMCPDependency{ID: item.ID, Description: item.Description, Required: required, Transport: strings.ToLower(item.Transport), URL: item.URL, Command: item.Command, Args: append([]string{}, item.Args...), AuthType: strings.ToLower(item.Auth.Type), ToolAllowlist: append([]string{}, item.Tools.Allow...), DefaultScope: strings.ToLower(item.Scope.Default), AutoConfigure: item.Install.AutoConfigure, AutoEnable: item.Install.AutoEnable, RequiresManualConfirmation: item.Install.RequiresManualConfirmation}
		if dependency.AuthType == "" {
			dependency.AuthType = "none"
		}
		if dependency.DefaultScope == "" {
			dependency.DefaultScope = "character"
		}
		if dependency.Transport == "stdio" {
			dependency.RequiresManualConfirmation = true
			dependency.AutoConfigure = false
		}
		if err := validateMCPDependency(dependency); err != nil {
			return nil, nil, err
		}
		key := strings.ToLower(dependency.ID)
		if seen[key] {
			return nil, nil, NewExtensionError(ErrAgentSkillFrontmatter, "duplicate MCP dependency", dependency.ID, false, nil)
		}
		seen[key] = true
		dependencies = append(dependencies, dependency)
	}
	warnings := []AgentSkillWarning{}
	if len(dependencies) > 0 {
		warnings = append(warnings, AgentSkillWarning{Code: "MCP_DEPENDENCY_REQUIRES_CONFIRMATION", Message: "MCP dependencies require explicit confirmation before installation", Path: "agents/amitia.yaml"})
	}
	return dependencies, warnings, nil
}

func validateMCPDependency(dependency AgentSkillMCPDependency) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`).MatchString(dependency.ID) {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency id is invalid", dependency.ID, false, nil)
	}
	if dependency.Transport == "" && dependency.URL == "" && dependency.Command == "" {
		return nil
	}
	if dependency.Transport != "streamable_http" && dependency.Transport != "stdio" {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency transport is invalid", dependency.ID, false, nil)
	}
	if dependency.AuthType != "none" && dependency.AuthType != "oauth" && dependency.AuthType != "bearer_token" && dependency.AuthType != "custom_headers" && dependency.AuthType != "stdio_env" {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency auth type is invalid", dependency.ID, false, nil)
	}
	if dependency.DefaultScope != "global" && dependency.DefaultScope != "character" {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency scope is invalid", dependency.ID, false, nil)
	}
	if len(dependency.Args) > 32 || len(dependency.ToolAllowlist) > 100 {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency exceeds limits", dependency.ID, false, nil)
	}
	if dependency.Transport == "streamable_http" {
		parsed, err := url.Parse(dependency.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || net.ParseIP(parsed.Hostname()) != nil && net.ParseIP(parsed.Hostname()).IsLoopback())) {
			return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency URL is unsafe", dependency.ID, false, nil)
		}
	}
	if dependency.Transport == "stdio" && strings.TrimSpace(dependency.Command) == "" {
		return NewExtensionError(ErrAgentSkillFrontmatter, "MCP dependency command is required", dependency.ID, false, nil)
	}
	return nil
}
