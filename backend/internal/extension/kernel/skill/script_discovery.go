// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ScriptDiscoveryContext struct {
	SkillRootResolver func(ctx context.Context, extensionID string) (string, error)
	ResourceResolver  func(ctx context.Context, extensionID, resourcePath string) ([]byte, error)
	FileInspector     ScriptFileInspector
}

type scriptDiscovery struct {
	ctx ScriptDiscoveryContext
}

func NewScriptDiscovery(ctx ScriptDiscoveryContext) *scriptDiscovery {
	if ctx.FileInspector == nil {
		ctx.FileInspector = defaultScriptFileInspector{}
	}
	return &scriptDiscovery{ctx: ctx}
}

func (d *scriptDiscovery) DiscoverFromResource(ctx context.Context, extensionID string, resource interface{}) (*SkillScriptDescriptor, error) {
	res, ok := resource.(struct {
		Path         string
		Kind         string
		SHA256       string
		Metadata     map[string]any
		Size         int64
		TextReadable bool
	})
	if !ok {
		return nil, ErrScriptInvalidDescriptor
	}
	if res.Kind != "script" {
		return nil, ErrScriptNotFound
	}

	skillRoot, err := d.ctx.SkillRootResolver(ctx, extensionID)
	if err != nil {
		return nil, ErrScriptInternalError
	}

	absPath, err := ValidateScriptPath(skillRoot, res.Path, DefaultScriptPolicyContext())
	if err != nil {
		return nil, err
	}

	if res.SHA256 != "" {
		if err := VerifyScriptHash(ctx, absPath, res.SHA256, d.ctx.FileInspector); err != nil {
			return nil, err
		}
	}

	data, err := d.ctx.FileInspector.ReadFile(absPath)
	if err != nil {
		return nil, ErrScriptInternalError
	}

	computedHash := ComputeFileHash(data)
	if res.SHA256 != "" && !strings.EqualFold(computedHash, res.SHA256) {
		return nil, ErrScriptHashMismatch
	}

	interp := ScriptRuntimeNode
	kind := ScriptKindExec
	if res.Metadata != nil {
		if rt, ok := res.Metadata["runtime"].(string); ok && rt != "" {
			interp = rt
		}
		if k, ok := res.Metadata["kind"].(string); ok && k != "" {
			kind = k
		}
	}

	return &SkillScriptDescriptor{
		ExtensionID:  extensionID,
		RelativePath: res.Path,
		FileHash:     computedHash,
		Runtime:      interp,
		Kind:         kind,
		EntryName:    filepath.Base(res.Path),
		Metadata:     res.Metadata,
	}, nil
}

func (d *scriptDiscovery) DiscoverFromInventory(ctx context.Context, extensionID, relPath string) (*SkillScriptDescriptor, error) {
	skillRoot, err := d.ctx.SkillRootResolver(ctx, extensionID)
	if err != nil {
		return nil, ErrScriptInternalError
	}

	absPath, err := ValidateScriptPath(skillRoot, relPath, DefaultScriptPolicyContext())
	if err != nil {
		return nil, err
	}

	data, err := d.ctx.FileInspector.ReadFile(absPath)
	if err != nil {
		return nil, ErrScriptInternalError
	}

	fileHash := ComputeFileHash(data)

	interp := ScriptRuntimeNode
	kind := ScriptKindExec
	entryName := filepath.Base(relPath)

	return &SkillScriptDescriptor{
		ExtensionID:  extensionID,
		RelativePath: relPath,
		FileHash:     fileHash,
		Runtime:      interp,
		Kind:         kind,
		EntryName:    entryName,
	}, nil
}

func (d *scriptDiscovery) ReadScriptContent(ctx context.Context, extensionID, relPath string) ([]byte, string, error) {
	skillRoot, err := d.ctx.SkillRootResolver(ctx, extensionID)
	if err != nil {
		return nil, "", ErrScriptInternalError
	}

	absPath, err := ValidateScriptPath(skillRoot, relPath, DefaultScriptPolicyContext())
	if err != nil {
		return nil, "", err
	}

	data, err := d.ctx.FileInspector.ReadFile(absPath)
	if err != nil {
		return nil, "", ErrScriptInternalError
	}

	return data, ComputeFileHash(data), nil
}

func ComputeContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func DeriveScriptID(extensionID, relPath string) string {
	return fmt.Sprintf("%s/%s", extensionID, strings.TrimPrefix(relPath, "scripts/"))
}

func HasScriptResource(resources []struct {
	Path string
	Kind string
}) bool {
	for _, r := range resources {
		if r.Kind == "script" {
			return true
		}
	}
	return false
}

func FilterScriptResources(resources []struct {
	Path         string
	Kind         string
	SHA256       string
	Metadata     map[string]any
	Size         int64
	TextReadable bool
}) []struct {
	Path         string
	Kind         string
	SHA256       string
	Metadata     map[string]any
	Size         int64
	TextReadable bool
} {
	var scripts []struct {
		Path         string
		Kind         string
		SHA256       string
		Metadata     map[string]any
		Size         int64
		TextReadable bool
	}
	for _, r := range resources {
		if r.Kind == "script" {
			scripts = append(scripts, r)
		}
	}
	return scripts
}

func FileNameWithoutExtension(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func SanitizeScriptRelPath(relPath string) string {
	relPath = strings.TrimSpace(relPath)
	relPath = strings.TrimLeft(relPath, "./\\")
	if !strings.HasPrefix(relPath, "scripts/") {
		relPath = "scripts/" + relPath
	}
	return relPath
}

func BuildAbsScriptPath(skillRoot, relPath string) string {
	cleanRel := strings.TrimSpace(relPath)
	cleanRel = strings.TrimLeft(cleanRel, "./\\")
	return filepath.Join(skillRoot, cleanRel)
}

func NormalizeScriptPath(relPath string) string {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return ""
	}
	if strings.HasPrefix(relPath, "..") {
		return ""
	}
	if filepath.IsAbs(relPath) {
		return ""
	}
	if strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(clean, "..") {
		return ""
	}
	if clean == ".." || strings.Contains(clean, "/../") {
		return ""
	}
	return clean
}

func ValidateScriptNotNull(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return ErrScriptPathEscape
	}
	if info.IsDir() {
		return ErrScriptInvalidDescriptor
	}
	return nil
}
