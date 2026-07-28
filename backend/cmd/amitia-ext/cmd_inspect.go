package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

func runInspect(args []string, output *Output) int {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	showFiles := fs.Bool("files", false, "显示文件列表")
	showModules := fs.Bool("modules", false, "显示模块详情")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitiax inspect <package.amitiax> [--files] [--modules]")
	}
	archivePath := fs.Arg(0)

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("打开包失败: %v", err))
	}

	data := map[string]any{
		"archive":     archivePath,
		"extensionId": pkg.Manifest.Extension.ID,
		"name":        pkg.Manifest.Extension.Name.Default,
		"version":     pkg.Manifest.Extension.Version,
		"license":     pkg.Manifest.Extension.License,
		"homepage":    pkg.Manifest.Extension.Homepage,
		"repository":  pkg.Manifest.Extension.Repository,
		"publisher": map[string]any{
			"id":          pkg.Manifest.Publisher.ID,
			"displayName": pkg.Manifest.Publisher.DisplayName,
			"trustLevel":  pkg.Manifest.Publisher.TrustLevel,
		},
		"moduleCount": len(pkg.Manifest.Modules),
		"fileCount":   len(pkg.Integrity.Files),
		"treeHash":    pkg.Tree.TreeHash,
		"algorithm":   pkg.Tree.Algorithm,
		"signed":      pkg.Signatures != nil,
	}

	if len(pkg.Manifest.Dependencies) > 0 {
		var deps []string
		for _, dep := range pkg.Manifest.Dependencies {
			deps = append(deps, fmt.Sprintf("%s:%s@%s", dep.Type, dep.ID, dep.Version))
		}
		data["dependencies"] = deps
	}

	if len(pkg.Manifest.Permissions) > 0 {
		var perms []string
		for _, p := range pkg.Manifest.Permissions {
			perms = append(perms, p.Name)
		}
		data["permissions"] = perms
	}

	if pkg.Signatures != nil {
		data["signature"] = map[string]any{
			"keyId":       pkg.Signatures.KeyID,
			"publisherId": pkg.Signatures.PublisherID,
			"algorithm":   pkg.Signatures.Algorithm,
			"signedAt":    pkg.Signatures.SignedAt,
		}
	}

	if *showModules {
		var modules []map[string]any
		for _, mod := range pkg.Manifest.Modules {
			modData := map[string]any{
				"id":   mod.ID,
				"type": mod.Type,
				"name": mod.Name.Default,
			}
			if mod.Runtime != nil {
				modData["runtime"] = mod.Runtime.Type
				modData["entryPoint"] = mod.Runtime.EntryPoint
			}
			if len(mod.Contributions) > 0 {
				var contribs []string
				for _, c := range mod.Contributions {
					contribs = append(contribs, fmt.Sprintf("%s:%s", c.Kind, c.ID))
				}
				modData["contributions"] = contribs
			}
			modules = append(modules, modData)
		}
		data["modules"] = modules
	}

	if *showFiles {
		var files []string
		sortedFiles := make([]amitiax.FileEntry, len(pkg.Files))
		copy(sortedFiles, pkg.Files)
		sort.Slice(sortedFiles, func(i, j int) bool {
			return sortedFiles[i].Path < sortedFiles[j].Path
		})
		for _, f := range sortedFiles {
			if !f.IsDir {
				files = append(files, fmt.Sprintf("%s  size=%d  sha256=%s", f.Path, f.Size, f.Hash))
			}
		}
		data["files"] = files
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("包信息: %s", archivePath),
		Data:    data,
	})
	return ExitSuccess
}
