package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
)

func runValidate(args []string, output *Output) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	useSchema := fs.Bool("schema", false, "启用 schema 验证")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitiax validate <manifest.json|package.amitiax|目录> [--schema]")
	}
	target := fs.Arg(0)

	info, err := os.Stat(target)
	if err != nil {
		output.fail(ExitEnv, fmt.Sprintf("无法访问 %s: %v", target, err))
	}

	if info.IsDir() {
		manifestPath := filepath.Join(target, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			output.fail(ExitConfig, fmt.Sprintf("目录中未找到 manifest.json: %s", target))
		}
		target = manifestPath
	}

	if strings.HasSuffix(target, ".amitiax") {
		return validatePackage(target, output)
	}
	return validateManifest(target, output, *useSchema)
}

func validateManifest(path string, output *Output, useSchema bool) int {
	m, report := manifest_v2.ValidateFile(path)

	result := Result{
		OK:      true,
		Message: fmt.Sprintf("验证 manifest: %s", path),
	}

	for _, e := range report.Errors {
		if isContentTreeHashMissing(e) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s（打包后将自动生成）", e.Path, e.Message))
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s [%s]", e.Path, e.Message, e.Code))
		}
	}
	for _, w := range report.Warnings {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", w.Path, w.Message))
	}

	if useSchema && m != nil {
		schemaReport := m.ValidateWithSchema()
		for _, e := range schemaReport.Errors {
			if isContentTreeHashMissing(e) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("schema: %s: %s（打包后将自动生成）", e.Path, e.Message))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("schema: %s: %s", e.Path, e.Message))
			}
		}
	}

	result.OK = len(result.Errors) == 0

	if m != nil && result.OK {
		result.Data = map[string]any{
			"extensionId": m.Extension.ID,
			"version":     m.Extension.Version,
			"publisher":   m.Publisher.ID,
			"modules":     len(m.Modules),
		}
	}

	output.emit(result)
	if result.OK {
		return ExitSuccess
	}
	return ExitFailure
}

func validatePackage(path string, output *Output) int {
	pkg, err := amitiax.OpenArchive(path)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("打开包失败: %v", err))
	}

	report := pkg.Manifest.Validate()

	result := Result{
		OK:      true,
		Message: fmt.Sprintf("验证包: %s", path),
		Data: map[string]any{
			"extensionId": pkg.Manifest.Extension.ID,
			"version":     pkg.Manifest.Extension.Version,
			"publisher":   pkg.Manifest.Publisher.ID,
			"modules":     len(pkg.Manifest.Modules),
			"treeHash":    pkg.Tree.TreeHash,
			"fileCount":   len(pkg.Integrity.Files),
			"signed":      pkg.Signatures != nil,
		},
	}

	for _, e := range report.Errors {
		result.Errors = append(result.Errors, fmt.Sprintf("%s: %s [%s]", e.Path, e.Message, e.Code))
	}
	for _, w := range report.Warnings {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", w.Path, w.Message))
	}

	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("完整性验证: %v", err))
	}

	result.OK = len(result.Errors) == 0

	output.emit(result)
	if result.OK {
		return ExitSuccess
	}
	return ExitFailure
}

func isContentTreeHashMissing(e manifest_v2.ValidationError) bool {
	return e.Code == "missing" && strings.Contains(e.Path, "contentTreeHash")
}
