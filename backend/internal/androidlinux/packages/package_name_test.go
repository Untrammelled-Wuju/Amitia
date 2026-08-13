//go:build linux && !android

package packages

import (
	"testing"
)

func TestValidateDebianPackageName_Valid(t *testing.T) {
	validNames := []string{
		"python3",
		"git",
		"curl",
		"build-essential",
		"python3-pip",
		"nodejs",
		"python3=3.10.0",
		"git=1:2.34.1-1ubuntu1",
	}
	for _, name := range validNames {
		if err := ValidateDebianPackageName(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}
}

func TestValidateDebianPackageName_Invalid(t *testing.T) {
	invalidNames := []string{
		"",
		"git; rm -rf /",
		"python3 && echo hacked",
		"--allow-unauthenticated",
		"-oProxyCommand=evil",
		"package|cat /etc/passwd",
		"pkg$(cmd)",
		"pkg`cmd`",
		"pkg\nnewline",
	}
	for _, name := range invalidNames {
		if err := ValidateDebianPackageName(name); err == nil {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestValidateNpmPackageSpec_Valid(t *testing.T) {
	validSpecs := []string{
		"express",
		"@scope/package",
		"express@4.18.0",
		"@scope/package@1.0.0",
		"lodash",
	}
	for _, spec := range validSpecs {
		if err := ValidateNpmPackageSpec(spec); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", spec, err)
		}
	}
}

func TestValidateNpmPackageSpec_Invalid(t *testing.T) {
	invalidSpecs := []string{
		"",
		"git+https://github.com/user/repo.git",
		"git+ssh://git@github.com:user/repo.git",
		"https://example.com/package.tgz",
		"file:../local-package",
		"../local",
		"./local",
		"express; rm -rf /",
	}
	for _, spec := range invalidSpecs {
		if err := ValidateNpmPackageSpec(spec); err == nil {
			t.Errorf("expected %q to be invalid", spec)
		}
	}
}

func TestValidatePythonPackageSpec_Valid(t *testing.T) {
	validSpecs := []string{
		"requests",
		"requests==2.32.3",
		"numpy>=2.0",
		"pandas",
		"beautifulsoup4",
	}
	for _, spec := range validSpecs {
		if err := ValidatePythonPackageSpec(spec); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", spec, err)
		}
	}
}

func TestValidatePythonPackageSpec_Invalid(t *testing.T) {
	invalidSpecs := []string{
		"",
		"git+https://github.com/user/repo.git",
		"https://example.com/package.whl",
		"file:///tmp/package",
		"../local-package",
		"/absolute/path",
		"requests; import os",
	}
	for _, spec := range invalidSpecs {
		if err := ValidatePythonPackageSpec(spec); err == nil {
			t.Errorf("expected %q to be invalid", spec)
		}
	}
}

func TestParseDebianPackageSpec(t *testing.T) {
	tests := []struct {
		input    string
		expected PackageNameVersion
	}{
		{"python3", PackageNameVersion{Name: "python3"}},
		{"git=1:2.34.1", PackageNameVersion{Name: "git", Version: "1:2.34.1"}},
	}
	for _, tt := range tests {
		result := ParseDebianPackageSpec(tt.input)
		if result.Name != tt.expected.Name || result.Version != tt.expected.Version {
			t.Errorf("ParseDebianPackageSpec(%q) = %+v, want %+v", tt.input, result, tt.expected)
		}
	}
}

func TestParseNpmPackageSpec(t *testing.T) {
	tests := []struct {
		input    string
		expected PackageNameVersion
	}{
		{"express", PackageNameVersion{Name: "express"}},
		{"express@4.18.0", PackageNameVersion{Name: "express", Version: "4.18.0"}},
		{"@scope/pkg", PackageNameVersion{Name: "@scope/pkg"}},
		{"@scope/pkg@1.0.0", PackageNameVersion{Name: "@scope/pkg", Version: "1.0.0"}},
	}
	for _, tt := range tests {
		result := ParseNpmPackageSpec(tt.input)
		if result.Name != tt.expected.Name || result.Version != tt.expected.Version {
			t.Errorf("ParseNpmPackageSpec(%q) = %+v, want %+v", tt.input, result, tt.expected)
		}
	}
}
