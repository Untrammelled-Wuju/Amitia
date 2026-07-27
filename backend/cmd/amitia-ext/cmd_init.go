package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
)

func runInit(args []string, output *Output) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "扩展 ID（如 com.example/my-ext）")
	dir := fs.String("dir", "", "目标目录（默认使用扩展名称）")
	displayName := fs.String("display-name", "", "扩展显示名称")
	publisher := fs.String("publisher", "", "发布者 ID")
	publisherName := fs.String("publisher-name", "", "发布者显示名称")
	version := fs.String("version", "0.1.0", "扩展版本")
	fs.Parse(args)

	if *name == "" {
		output.fail(ExitConfig, "缺少 --name 参数，例如: --name com.example/my-ext")
	}

	extID := *name
	if err := validateExtID(extID); err != nil {
		output.fail(ExitConfig, err.Error())
	}

	if *displayName == "" {
		parts := strings.SplitN(extID, "/", 2)
		if len(parts) == 2 {
			*displayName = parts[1]
		} else {
			*displayName = extID
		}
	}
	if *publisher == "" {
		parts := strings.SplitN(extID, "/", 2)
		if len(parts) == 2 {
			*publisher = parts[0]
		} else {
			*publisher = "unknown"
		}
	}
	if *publisherName == "" {
		*publisherName = *publisher
	}

	targetDir := *dir
	if targetDir == "" {
		parts := strings.SplitN(extID, "/", 2)
		if len(parts) == 2 {
			targetDir = parts[1]
		} else {
			targetDir = strings.ReplaceAll(extID, "/", "-")
		}
	}

	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析路径失败: %v", err))
	}

	if info, err := os.Stat(absDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(absDir)
		if len(entries) > 0 {
			output.fail(ExitConfig, fmt.Sprintf("目标目录不为空: %s", absDir))
		}
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		output.fail(ExitEnv, fmt.Sprintf("创建目录失败: %v", err))
	}

	manifest := manifest_v2.Manifest{
		ManifestVersion: manifest_v2.ManifestVersion,
		Extension: manifest_v2.ExtensionMeta{
			ID:      extID,
			Name:    manifest_v2.LocalizedText{Default: *displayName},
			Version: *version,
			License: "MIT",
		},
		Publisher: manifest_v2.PublisherMeta{
			ID:          *publisher,
			DisplayName: *publisherName,
		},
		Compatibility: manifest_v2.Compatibility{},
		Modules: []manifest_v2.ModuleMeta{
			{
				ID:   "main",
				Name: manifest_v2.LocalizedText{Default: "Main Module"},
				Type: "javascript",
				Runtime: &manifest_v2.RuntimeMeta{
					Type:       "javascript",
					EntryPoint: "index.js",
				},
			},
		},
		Integrity: manifest_v2.IntegrityMeta{
			Algorithm: "sha256",
		},
	}

	files := map[string][]byte{
		"manifest.json":            mustJSON(manifest),
		"modules/main/index.js":    []byte(indexJSTemplate),
		"modules/main/module.json": []byte(moduleJSONTemplate),
		"package.json":             []byte(buildPackageJSON(extID, *version)),
	}
	dirs := []string{"resources", "licenses"}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(absDir, d), 0o755); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("创建目录失败: %v", err))
		}
	}
	for path, content := range files {
		full := filepath.Join(absDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("创建目录失败: %v", err))
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			output.fail(ExitEnv, fmt.Sprintf("写入文件失败: %v", err))
		}
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("扩展项目已创建: %s", absDir),
		Data: map[string]any{
			"dir":          absDir,
			"extensionId":  extID,
			"version":      *version,
			"publisher":    *publisher,
			"displayName":  *displayName,
			"filesCreated": len(files) + len(dirs),
		},
	})
	return ExitSuccess
}

func validateExtID(id string) error {
	if id == "" {
		return fmt.Errorf("扩展 ID 不能为空")
	}
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return fmt.Errorf("扩展 ID 格式错误，应为 <reverse-domain>/<name>，例如 com.example/my-ext")
	}
	domainParts := strings.Split(parts[0], ".")
	if len(domainParts) < 2 {
		return fmt.Errorf("扩展 ID 域名部分至少包含两个段，例如 com.example")
	}
	for _, p := range domainParts {
		if p == "" {
			return fmt.Errorf("扩展 ID 域名部分不能有空段")
		}
	}
	return nil
}

func mustJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}

func buildPackageJSON(extID, version string) string {
	parts := strings.SplitN(extID, "/", 2)
	pkgName := extID
	if len(parts) == 2 {
		pkgName = parts[1]
	}
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "%s",
  "description": "Amitia extension",
  "main": "modules/main/index.js",
  "scripts": {
    "build": "echo build",
    "test": "echo test"
  }
}`, pkgName, version)
}

const indexJSTemplate = `'use strict';

module.exports = {
  activate(context) {
    console.log('Extension activated');
  },
  deactivate() {
    console.log('Extension deactivated');
  }
};
`

const moduleJSONTemplate = `{
  "id": "main",
  "type": "javascript",
  "entryPoint": "index.js"
}`
