package javascript_main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ModuleFormat string

const (
	FormatESM     ModuleFormat = "esm"
	FormatCJS     ModuleFormat = "cjs"
	FormatUnknown ModuleFormat = "unknown"
)

const (
	ModuleTypeModule  = "module"
	ModuleTypeCommonJS = "commonjs"
)

type packageJSON struct {
	Type string `json:"type"`
}

type ModuleFormatDetector struct {
}

func NewModuleFormatDetector() *ModuleFormatDetector {
	return &ModuleFormatDetector{}
}

func (d *ModuleFormatDetector) Detect(entryPath string) ModuleFormat {
	if entryPath == "" {
		return FormatUnknown
	}
	ext := strings.ToLower(filepath.Ext(entryPath))
	switch ext {
	case ".mjs":
		return FormatESM
	case ".cjs":
		return FormatCJS
	case ".js", ".ts", ".mts", ".cts":
		return d.detectFromPackageJSON(entryPath)
	default:
		return FormatUnknown
	}
}

func (d *ModuleFormatDetector) detectFromPackageJSON(entryPath string) ModuleFormat {
	dir := filepath.Dir(entryPath)
	for dir != "" && dir != "." && dir != "/" {
		pkgPath := filepath.Join(dir, "package.json")
		if info, err := os.Stat(pkgPath); err == nil && !info.IsDir() {
			return d.parsePackageJSON(pkgPath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return FormatCJS
}

func (d *ModuleFormatDetector) parsePackageJSON(pkgPath string) ModuleFormat {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return FormatCJS
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return FormatCJS
	}
	switch strings.TrimSpace(pkg.Type) {
	case ModuleTypeModule:
		return FormatESM
	case ModuleTypeCommonJS:
		return FormatCJS
	default:
		return FormatCJS
	}
}

func (d *ModuleFormatDetector) IsTypeScript(entryPath string) bool {
	ext := strings.ToLower(filepath.Ext(entryPath))
	return ext == ".ts" || ext == ".tsx" || ext == ".mts" || ext == ".cts"
}

func (d *ModuleFormatDetector) NormalizedExtension(format ModuleFormat) string {
	switch format {
	case FormatESM:
		return ".mjs"
	case FormatCJS:
		return ".cjs"
	default:
		return ".js"
	}
}
