package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type BuiltAgentSkillPackage struct {
	Archive    []byte
	Definition agent_skill.AgentSkillDefinition
	FileName   string
}

func BuildLocalAgentSkillPackage(ctx context.Context, spaceID, rootName string, files map[string][]byte) (BuiltAgentSkillPackage, error) {
	normalized, err := normalizeAgentSkillImportFiles(files)
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	rawSkill, ok := normalized["SKILL.md"]
	if !ok {
		return BuiltAgentSkillPackage{}, fmt.Errorf("SKILL.md is required")
	}
	parsed, err := agent_skill.NewSkillParser().Parse(rawSkill)
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	name := strings.TrimSpace(fmt.Sprint(parsed.Fields["name"]))
	if !validAgentSkillName(name) {
		return BuiltAgentSkillPackage{}, fmt.Errorf("agent skill name is invalid")
	}
	if strings.TrimSpace(rootName) != "" && strings.TrimSpace(rootName) != name {
		return BuiltAgentSkillPackage{}, fmt.Errorf("agent skill name does not match import root")
	}
	sum := sha256.Sum256(rawSkill)
	spaceHash := sha256.Sum256([]byte(spaceID))
	extensionID := "local.agentskill/" + hex.EncodeToString(spaceHash[:6]) + "-" + name
	version := "1.0.0+" + hex.EncodeToString(sum[:6])
	if metadata, ok := parsed.Fields["metadata"].(map[string]any); ok {
		if candidate := strings.TrimSpace(fmt.Sprint(metadata["version"])); candidate != "" {
			if _, versionErr := parsePackageVersion(candidate); versionErr == nil {
				version = candidate
			}
		}
	}
	description := strings.TrimSpace(fmt.Sprint(parsed.Fields["description"]))
	if description == "" {
		description = name
	}
	entryPoint := "resources/" + name + "/SKILL.md"
	manifest := map[string]any{
		"manifestVersion": 1,
		"extension": map[string]any{
			"id": extensionID, "name": map[string]any{"default": name},
			"description": map[string]any{"default": description}, "version": version,
		},
		"publisher":     map[string]any{"id": "local.agentskill", "displayName": "Local Import"},
		"compatibility": map[string]any{"minHostVersion": "1.0.0"},
		"modules": []any{map[string]any{
			"id": "agent-skill", "name": map[string]any{"default": name},
			"type": "data_only", "version": version,
			"contributions": []any{map[string]any{
				"id": name, "kind": "agent_skill", "name": map[string]any{"default": name},
				"description": map[string]any{"default": description}, "version": version,
				"spec": map[string]any{"entryPoint": entryPoint},
			}},
		}},
		"integrity": map[string]any{"algorithm": "sha256", "contentTreeHash": ""},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	packageFiles := map[string][]byte{"manifest.json": manifestRaw, "modules/agent-skill/data.json": []byte("{}")}
	for relative, content := range normalized {
		packageFiles[path.Join("resources", name, filepath.ToSlash(relative))] = content
	}
	if err := addIntegrityFiles(packageFiles); err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	archive, err := zipFiles(packageFiles)
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	tempDir, err := os.MkdirTemp("", "amitia-skill-import-*")
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	defer os.RemoveAll(tempDir)
	packageDir := filepath.Join(tempDir, "package")
	for relative, content := range packageFiles {
		target := filepath.Join(packageDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return BuiltAgentSkillPackage{}, err
		}
		if err := os.WriteFile(target, content, 0o640); err != nil {
			return BuiltAgentSkillPackage{}, err
		}
	}
	base := agent_skill.AgentSkillDefinition{
		ID: name, ContributionID: name, ExtensionID: extensionID, ModuleID: "agent-skill",
		OwnerSpaceID: spaceID, Version: version, Source: "local-import",
	}
	definition, err := agent_skill.LoadDefinitionFromPackage(ctx, packageDir, entryPoint, base)
	if err != nil {
		return BuiltAgentSkillPackage{}, err
	}
	return BuiltAgentSkillPackage{Archive: archive, Definition: definition, FileName: name + ".amitiax"}, nil
}

func normalizeAgentSkillImportFiles(files map[string][]byte) (map[string][]byte, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("agent skill files are required")
	}
	normalized := map[string][]byte{}
	canonical := map[string]string{}
	var skillRoot string
	foundSkill := false
	for rawPath, content := range files {
		clean := path.Clean(strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
			return nil, fmt.Errorf("agent skill path is invalid: %s", rawPath)
		}
		if strings.Contains(strings.ToLower(clean), ".git/") || strings.HasPrefix(strings.ToLower(clean), ".env") {
			return nil, fmt.Errorf("agent skill path is forbidden: %s", rawPath)
		}
		key := strings.ToLower(clean)
		if previous, exists := canonical[key]; exists {
			return nil, fmt.Errorf("agent skill contains colliding paths: %s and %s", previous, clean)
		}
		canonical[key] = clean
		normalized[clean] = content
		if path.Base(clean) == "SKILL.md" {
			dir := path.Dir(clean)
			if dir == "." {
				dir = ""
			}
			if !foundSkill || len(dir) < len(skillRoot) {
				skillRoot = dir
				foundSkill = true
			}
		}
	}
	if !foundSkill {
		return nil, fmt.Errorf("SKILL.md is required")
	}
	result := map[string][]byte{}
	prefix := ""
	if skillRoot != "" {
		prefix = skillRoot + "/"
	}
	for filePath, content := range normalized {
		if prefix != "" {
			if !strings.HasPrefix(filePath, prefix) {
				continue
			}
			filePath = strings.TrimPrefix(filePath, prefix)
		}
		result[filePath] = content
	}
	if _, ok := result["SKILL.md"]; !ok {
		return nil, fmt.Errorf("SKILL.md is required at the skill root")
	}
	return result, nil
}

func addIntegrityFiles(files map[string][]byte) error {
	entries := make(map[string]amitiax.FileEntry, len(files))
	list := make([]amitiax.FileEntry, 0, len(files))
	for name, content := range files {
		sum := sha256.Sum256(content)
		entry := amitiax.FileEntry{Path: name, Size: int64(len(content)), Hash: hex.EncodeToString(sum[:])}
		entries[name] = entry
		list = append(list, entry)
	}
	filesRaw, err := json.Marshal(amitiax.IntegrityFilesDoc{Algorithm: "sha256", Files: entries, GeneratedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	treeRaw, err := json.Marshal(amitiax.IntegrityTreeDoc{Algorithm: "sha256", TreeHash: amitiax.ComputeTreeHash(list), GeneratedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	files[amitiax.IntegrityFiles] = filesRaw
	files[amitiax.IntegrityTree] = treeRaw
	return nil
}

func zipFiles(files map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o440)
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func validAgentSkillName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for index, char := range name {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			continue
		}
		if char == '-' && index > 0 && index < len(name)-1 {
			continue
		}
		return false
	}
	return true
}

func parsePackageVersion(value string) (string, error) {
	if _, err := domain.ParseVersion(value); err != nil {
		return "", err
	}
	return value, nil
}
