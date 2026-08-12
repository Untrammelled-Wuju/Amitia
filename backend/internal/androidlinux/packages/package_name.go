package packages

import (
	"regexp"
	"strings"
)

var (
	debianPackageNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9+.-]*$`)
	debianVersionPattern     = regexp.MustCompile(`^[a-zA-Z0-9.+~:-]+$`)
	npmPackagePattern        = regexp.MustCompile(`^(@[a-z0-9-][a-z0-9-.]*/)?[a-z0-9-][a-z0-9-.]*$`)
	npmVersionPattern        = regexp.MustCompile(`^[@~>=]?[0-9]+(\.[0-9]+)*(-[a-zA-Z0-9.]+)?$`)
	pythonPackagePattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
)

func ValidateDebianPackageName(input string) error {
	name := strings.TrimSpace(input)
	if name == "" {
		return ErrInvalidPackageName("<empty>")
	}

	for _, ch := range name {
		if isShellMetachar(ch) {
			return ErrInvalidPackageName(input)
		}
	}

	parts := strings.SplitN(name, "=", 2)
	pkgName := parts[0]
	if strings.HasPrefix(pkgName, "-") {
		return ErrInvalidPackageName(input)
	}

	if !debianPackageNamePattern.MatchString(pkgName) {
		return ErrInvalidPackageName(input)
	}

	if len(parts) == 2 {
		if !debianVersionPattern.MatchString(parts[1]) {
			return ErrInvalidPackageName(input)
		}
	}

	return nil
}

func ValidateNpmPackageSpec(spec string) error {
	s := strings.TrimSpace(spec)
	if s == "" {
		return ErrInvalidNpmPackageSpec("<empty>")
	}

	if strings.ContainsAny(s, " \t\n\r;|&$`") {
		return ErrInvalidNpmPackageSpec(spec)
	}

	if strings.HasPrefix(s, "git+") || strings.HasPrefix(s, "github:") {
		return ErrInvalidNpmPackageSpec(spec)
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return ErrInvalidNpmPackageSpec(spec)
	}

	if strings.HasPrefix(s, "file:") {
		return ErrInvalidNpmPackageSpec(spec)
	}

	if strings.HasPrefix(s, "../") || strings.HasPrefix(s, "./") {
		return ErrInvalidNpmPackageSpec(spec)
	}

	return nil
}

func ValidatePythonPackageSpec(spec string) error {
	s := strings.TrimSpace(spec)
	if s == "" {
		return ErrInvalidPythonPackageSpec("<empty>")
	}

	if strings.ContainsAny(s, " \t\n\r;|&$`") {
		return ErrInvalidPythonPackageSpec(spec)
	}

	if strings.HasPrefix(s, "git+") || strings.HasPrefix(s, "git://") {
		return ErrInvalidPythonPackageSpec(spec)
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return ErrInvalidPythonPackageSpec(spec)
	}

	if strings.HasPrefix(s, "file://") || strings.HasPrefix(s, "file:") {
		return ErrInvalidPythonPackageSpec(spec)
	}

	if strings.HasPrefix(s, "../") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") {
		return ErrInvalidPythonPackageSpec(spec)
	}

	return nil
}

func ParseDebianPackageSpec(spec string) PackageNameVersion {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) == 2 {
		return PackageNameVersion{Name: parts[0], Version: parts[1]}
	}
	return PackageNameVersion{Name: spec}
}

func ParseNpmPackageSpec(spec string) PackageNameVersion {
	if strings.HasPrefix(spec, "@") {
		rest := spec[1:]
		if idx := indexOfAny(rest, "@"); idx > 0 {
			return PackageNameVersion{Name: "@" + rest[:idx], Version: rest[idx+1:]}
		}
		return PackageNameVersion{Name: spec}
	}
	if idx := indexOfAny(spec, "@"); idx > 0 {
		return PackageNameVersion{Name: spec[:idx], Version: spec[idx+1:]}
	}
	return PackageNameVersion{Name: spec}
}

func indexOfAny(s string, chars string) int {
	for i, c := range s {
		for _, ch := range chars {
			if c == ch {
				return i
			}
		}
	}
	return -1
}

func isShellMetachar(ch rune) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', ';', '|', '&', '>', '<', '`', '$', '(', ')', '\'', '"', '\\':
		return true
	default:
		return false
	}
}
