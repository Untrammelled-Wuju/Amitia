// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxNPXPackageNameLength = 214
	maxNPXBinaryNameLength  = 255
)

var npxReservedEnvVars = map[string]bool{
	"NPM_CONFIG_CACHE":          true,
	"NPM_CONFIG_USERCONFIG":     true,
	"NPM_CONFIG_REGISTRY":       true,
	"NPM_CONFIG_IGNORE_SCRIPTS": true,
	"NPM_CONFIG_YES":            true,
	"NODE_OPTIONS":              true,
	"PATH":                      true,
	"HOME":                      true,
}

func ValidateNPXPackageName(packageName string) error {
	if !utf8.ValidString(packageName) {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: package name is not valid UTF-8")
	}

	if packageName == "" {
		return fmt.Errorf("MCP_NPX_PACKAGE_REQUIRED: package name is empty")
	}

	if len(packageName) > maxNPXPackageNameLength {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: package name exceeds maximum length")
	}

	if strings.ContainsRune(packageName, 0) {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: package name contains NUL character")
	}

	if strings.ContainsAny(packageName, "\t\r\n ") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: package name contains whitespace")
	}

	for _, ch := range packageName {
		if ch < 0x20 || ch == 0x7f {
			return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: package name contains control character")
		}
	}

	if strings.Contains(packageName, "://") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: URL package spec is forbidden")
	}

	if strings.HasPrefix(packageName, "git+") || strings.HasPrefix(packageName, "github:") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: git package spec is forbidden")
	}

	if strings.HasPrefix(packageName, "file:") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: file package spec is forbidden")
	}

	if strings.HasPrefix(packageName, "../") || strings.HasPrefix(packageName, "./") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: local path package spec is forbidden")
	}

	if packageName[0] == '/' || strings.Contains(packageName, ":\\") {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: absolute path package spec is forbidden")
	}

	atIndex := strings.LastIndexByte(packageName, '@')
	if atIndex > 0 {
		namePart := packageName[:atIndex]
		versionPart := packageName[atIndex+1:]

		if strings.HasPrefix(namePart, "@") {
			slashIndex := strings.Index(namePart, "/")
			if slashIndex > 0 {
				namePart = namePart[slashIndex+1:]
			}
		}

		if versionPart == "" {
			return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: empty version in package spec")
		}

		if versionPart == "latest" || versionPart == "next" || versionPart == "*" {
			return fmt.Errorf("MCP_NPX_VERSION_REQUIRED: floating version is not allowed by default")
		}

		if strings.HasPrefix(versionPart, "^") || strings.HasPrefix(versionPart, "~") {
			return fmt.Errorf("MCP_NPX_VERSION_REQUIRED: semver range is not allowed by default")
		}
	}

	if !isValidNPXPackageFormat(packageName) {
		return fmt.Errorf("MCP_NPX_PACKAGE_INVALID: invalid npm package format")
	}

	return nil
}

func ValidateNPXBinaryName(binary string) error {
	if binary == "" {
		return nil
	}

	if len(binary) > maxNPXBinaryNameLength {
		return fmt.Errorf("MCP_NPX_BINARY_INVALID: binary name exceeds maximum length")
	}

	if strings.ContainsRune(binary, 0) {
		return fmt.Errorf("MCP_NPX_BINARY_INVALID: binary name contains NUL character")
	}

	if strings.ContainsAny(binary, "\t\r\n /\\") {
		return fmt.Errorf("MCP_NPX_BINARY_INVALID: binary name contains invalid character")
	}

	shellCommands := []string{"sh", "bash", "zsh", "fish", "cmd", "cmd.exe", "powershell", "pwsh"}
	for _, cmd := range shellCommands {
		if strings.EqualFold(binary, cmd) {
			return fmt.Errorf("MCP_NPX_BINARY_INVALID: shell command is not allowed as binary")
		}
	}

	if strings.Contains(binary, "..") {
		return fmt.Errorf("MCP_NPX_BINARY_INVALID: path traversal is not allowed")
	}

	return nil
}

func IsNPXReservedEnvVar(name string) bool {
	return npxReservedEnvVars[name]
}

func isValidNPXPackageFormat(name string) bool {
	if name == "" {
		return false
	}

	if strings.HasPrefix(name, "@") {
		slashIndex := strings.IndexByte(name, '/')
		if slashIndex <= 1 {
			return false
		}
		scope := name[1:slashIndex]
		rest := name[slashIndex+1:]

		atIndex := strings.LastIndexByte(rest, '@')
		if atIndex > 0 {
			rest = rest[:atIndex]
		}

		return isValidNPXPackageScope(scope) && isValidNPXPackageBaseName(rest)
	}

	atIndex := strings.LastIndexByte(name, '@')
	if atIndex > 0 {
		name = name[:atIndex]
	}

	return isValidNPXPackageBaseName(name)
}

func isValidNPXPackageScope(scope string) bool {
	if scope == "" {
		return false
	}
	for _, ch := range scope {
		if !isValidNPXPackageChar(ch) {
			return false
		}
	}
	return true
}

func isValidNPXPackageBaseName(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if !isValidNPXPackageChar(ch) {
			return false
		}
	}
	return true
}

func isValidNPXPackageChar(ch rune) bool {
	if ch >= 'a' && ch <= 'z' {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	if ch == '-' || ch == '_' || ch == '.' {
		return true
	}
	return false
}
