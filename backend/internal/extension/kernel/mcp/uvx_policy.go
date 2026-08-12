// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"fmt"
	"strings"
	"unicode"
)

type UvxPolicy struct {
	RequireExactVersion bool
	AllowCustomIndex    bool
	AllowVersionRange   bool
	AllowDirectURL      bool
	AllowGit            bool
	AllowLocalPath      bool
	AllowSourceBuild    bool
	AllowPythonDownload bool
	MaxArgs             int
	MaxArgBytes         int
}

var (
	ErrUvxPackageInvalid        = fmt.Errorf("MCP_UVX_PACKAGE_INVALID: invalid python package requirement")
	ErrUvxPackageDirectURL      = fmt.Errorf("MCP_UVX_PACKAGE_DIRECT_URL: direct URL dependencies are not allowed")
	ErrUvxPackageGit            = fmt.Errorf("MCP_UVX_PACKAGE_GIT: git dependencies are not allowed")
	ErrUvxPackageLocalPath      = fmt.Errorf("MCP_UVX_PACKAGE_LOCAL_PATH: local path dependencies are not allowed")
	ErrUvxPackageVersionUnlucky = fmt.Errorf("MCP_UVX_VERSION_UNPINNED: exact version is required")
	ErrUvxPackageVersionRange   = fmt.Errorf("MCP_UVX_VERSION_RANGE: version range is not allowed by policy")
	ErrUvxCommandInvalid        = fmt.Errorf("MCP_UVX_COMMAND_INVALID: invalid command name")
	ErrUvxArgsInvalid           = fmt.Errorf("MCP_UVX_ARGUMENT_INVALID: argument validation failed")
	ErrUvxPythonInvalid         = fmt.Errorf("MCP_UVX_PYTHON_UNSUPPORTED: invalid python selector")
	ErrUvxWorkDirInvalid        = fmt.Errorf("MCP_UVX_WORKDIR_INVALID: invalid working directory")
)

func ParsePythonToolRequirement(requirement string) (PythonToolRequirement, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return PythonToolRequirement{}, ErrUvxPackageInvalid
	}

	if strings.Contains(requirement, "@") && !strings.Contains(requirement, "==") && !strings.Contains(requirement, ">=") && !strings.Contains(requirement, "<=") && !strings.Contains(requirement, "~=") && !strings.Contains(requirement, "!=") {
		if strings.Contains(requirement, "://") {
			return PythonToolRequirement{}, ErrUvxPackageDirectURL
		}
		if strings.HasPrefix(requirement, "git+") {
			return PythonToolRequirement{}, ErrUvxPackageGit
		}
	}

	requirement = strings.ToLower(requirement)

	var nameEnd int
	var extras []string

	bracketIdx := strings.IndexByte(requirement, '[')
	if bracketIdx >= 0 {
		closeBracket := strings.IndexByte(requirement, ']')
		if closeBracket <= bracketIdx {
			return PythonToolRequirement{}, ErrUvxPackageInvalid
		}
		nameEnd = bracketIdx
		extraStr := requirement[bracketIdx+1 : closeBracket]
		for _, e := range strings.Split(extraStr, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				extras = append(extras, e)
			}
		}
		if !isValidPythonName(requirement[0:nameEnd]) {
			return PythonToolRequirement{}, ErrUvxPackageInvalid
		}
		for _, e := range extras {
			if !isValidPythonName(e) {
				return PythonToolRequirement{}, ErrUvxPackageInvalid
			}
		}
		rest := strings.TrimSpace(requirement[closeBracket+1:])
		versionSpec, err := parseVersionSpec(rest)
		if err != nil {
			return PythonToolRequirement{}, err
		}
		return PythonToolRequirement{Name: requirement[0:nameEnd], Extras: extras, VersionSpec: versionSpec}, nil
	}

	opIdx := strings.IndexAny(requirement, "=<>~!")
	if opIdx >= 0 {
		nameEnd = opIdx
		if !isValidPythonName(requirement[0:nameEnd]) {
			return PythonToolRequirement{}, ErrUvxPackageInvalid
		}
		rest := strings.TrimSpace(requirement[opIdx:])
		versionSpec, err := parseVersionSpec(rest)
		if err != nil {
			return PythonToolRequirement{}, err
		}
		return PythonToolRequirement{Name: requirement[0:nameEnd], VersionSpec: versionSpec}, nil
	}

	if !isValidPythonName(requirement) {
		return PythonToolRequirement{}, ErrUvxPackageInvalid
	}
	return PythonToolRequirement{Name: requirement, VersionSpec: ""}, nil
}

func isValidPythonName(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '-' && ch != '_' && ch != '.' {
			return false
		}
	}
	return true
}

func parseVersionSpec(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil
	}

	if spec == "latest" || spec == "*" {
		return "", ErrUvxPackageVersionUnlucky
	}

	if strings.HasPrefix(spec, "==") {
		ver := strings.TrimSpace(spec[2:])
		if ver == "" {
			return "", ErrUvxPackageInvalid
		}
		if !isValidVersion(ver) {
			return "", ErrUvxPackageInvalid
		}
		return "==" + ver, nil
	}

	if strings.HasPrefix(spec, "~=") || strings.HasPrefix(spec, ">=") || strings.HasPrefix(spec, "<=") || strings.HasPrefix(spec, "!=") || strings.HasPrefix(spec, ">") || strings.HasPrefix(spec, "<") {
		return "", ErrUvxPackageVersionRange
	}

	return "", ErrUvxPackageInvalid
}

func isValidVersion(ver string) bool {
	if ver == "" {
		return false
	}
	for _, ch := range ver {
		if !unicode.IsDigit(ch) && ch != '.' && ch != '-' && ch != '_' && ch != 'a' && ch != 'b' && ch != 'r' && ch != 'c' {
			return false
		}
	}
	return true
}

func ValidateUvxCommand(command string) error {
	if command == "" {
		return ErrUvxCommandInvalid
	}
	if len(command) > 128 {
		return ErrUvxCommandInvalid
	}
	for _, ch := range command {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '-' && ch != '_' && ch != '.' {
			return ErrUvxCommandInvalid
		}
	}
	return nil
}

func ValidateUvxArgs(args []string, maxArgs, maxArgBytes, maxTotalBytes int) error {
	if len(args) > maxArgs {
		return ErrUvxArgsInvalid
	}
	total := 0
	for _, arg := range args {
		if len(arg) > maxArgBytes {
			return ErrUvxArgsInvalid
		}
		total += len(arg)
	}
	if total > maxTotalBytes {
		return ErrUvxArgsInvalid
	}
	return nil
}

func ValidateUvxPythonSelector(python string) error {
	if python == "" {
		return nil
	}
	if len(python) > 32 {
		return ErrUvxPythonInvalid
	}
	if strings.HasPrefix(python, "pypy@") {
		ver := strings.TrimPrefix(python, "pypy@")
		return validatePythonVersion(ver)
	}
	return validatePythonVersion(python)
}

func validatePythonVersion(ver string) error {
	parts := strings.Split(ver, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return ErrUvxPythonInvalid
	}
	for _, p := range parts {
		if p == "" {
			return ErrUvxPythonInvalid
		}
		for _, ch := range p {
			if !unicode.IsDigit(ch) {
				return ErrUvxPythonInvalid
			}
		}
	}
	return nil
}

func ValidateUvxWorkDir(workDir string) error {
	if workDir == "" {
		return nil
	}
	if strings.Contains(workDir, "\x00") {
		return ErrUvxWorkDirInvalid
	}
	if strings.Contains(workDir, "..") {
		return ErrUvxWorkDirInvalid
	}
	return nil
}
