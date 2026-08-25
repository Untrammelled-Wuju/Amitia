package package_security

import (
	"bytes"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"
)

type EntryValidator struct {
	policy              ArchivePolicy
	fileTypeDetector    *FileTypeDetector
	forbiddenExt        map[string]bool
	nestedArchiveExt    map[string]bool
	executableMagics    [][]byte
	declaredExecutables map[string]struct{}
}

func NewEntryValidator(policy ArchivePolicy) *EntryValidator {
	return NewEntryValidatorWithDeclaredExecutables(policy, nil)
}

func NewEntryValidatorWithDeclaredExecutables(policy ArchivePolicy, declared map[string]struct{}) *EntryValidator {
	copied := make(map[string]struct{}, len(declared))
	for name := range declared {
		copied[strings.ToLower(strings.ReplaceAll(name, "\\", "/"))] = struct{}{}
	}
	return &EntryValidator{
		policy:           policy,
		fileTypeDetector: NewFileTypeDetector(),
		forbiddenExt: map[string]bool{
			".exe": true, ".com": true, ".bat": true, ".cmd": true,
			".msi": true, ".dll": true, ".sys": true, ".scr": true,
			".pif": true, ".cpl": true, ".so": true, ".dylib": true,
			".app": true, ".class": true, ".jar": true,
			".apk": true, ".deb": true, ".rpm": true,
		},
		nestedArchiveExt: map[string]bool{
			".zip": true, ".rar": true, ".7z": true, ".tar": true,
			".gz": true, ".tgz": true, ".bz2": true, ".xz": true,
		},
		executableMagics: [][]byte{
			{'M', 'Z'},
			{0x7f, 'E', 'L', 'F'},
			{0xca, 0xfe, 0xba, 0xbe},
			{0xcf, 0xfa, 0xed, 0xfe},
			{0xce, 0xfa, 0xed, 0xfe},
		},
		declaredExecutables: copied,
	}
}

type EntryValidationResult struct {
	Passed   bool
	Warnings []string
	Errors   []string
}

func (v *EntryValidator) Validate(entry ArchiveEntryInfo, content []byte) EntryValidationResult {
	result := EntryValidationResult{Passed: true}

	ext := strings.ToLower(path.Ext(entry.NormalizedPath))
	_, declaredExecutable := v.declaredExecutables[strings.ToLower(strings.ReplaceAll(entry.NormalizedPath, "\\", "/"))]
	executableAllowed := v.policy.AllowExecutable || (v.policy.AllowDeclaredExecutable && declaredExecutable)

	if v.forbiddenExt[ext] && !executableAllowed {
		result.Passed = false
		result.Errors = append(result.Errors, "forbidden file extension: "+ext)
	}

	if v.nestedArchiveExt[ext] && !v.policy.AllowNestedArchive {
		result.Passed = false
		result.Errors = append(result.Errors, "nested archive not allowed: "+ext)
	}

	if ext == ".wasm" && len(content) >= 4 && bytes.Equal(content[:4], []byte{0, 'a', 's', 'm'}) {
		result.Warnings = append(result.Warnings, "WASM binary detected: "+entry.NormalizedPath)
	}

	for _, magic := range v.executableMagics {
		if len(content) >= len(magic) && bytes.Equal(content[:len(magic)], magic) {
			if !executableAllowed {
				result.Passed = false
				result.Errors = append(result.Errors, "executable binary not allowed: "+entry.NormalizedPath)
			} else {
				result.Warnings = append(result.Warnings, "declared executable binary: "+entry.NormalizedPath)
			}
			break
		}
	}

	mime := http.DetectContentType(content)
	if !strings.HasPrefix(mime, "text/") &&
		!strings.Contains(mime, "json") &&
		!strings.Contains(mime, "xml") &&
		!strings.HasPrefix(mime, "image/") &&
		!strings.HasPrefix(mime, "audio/") &&
		!strings.HasPrefix(mime, "video/") &&
		mime != "application/octet-stream" &&
		len(content) > 0 {
		result.Warnings = append(result.Warnings, "unknown binary type: "+mime+" at "+entry.NormalizedPath)
	}

	if utf8.Valid(content) && len(content) > 0 {
		v.checkSecretPatterns(entry.NormalizedPath, content, &result)
	}

	return result
}

func (v *EntryValidator) checkSecretPatterns(path string, content []byte, result *EntryValidationResult) {
	text := string(content)

	secretIndicators := []string{
		"-----BEGIN RSA PRIVATE KEY-----",
		"-----BEGIN EC PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----",
		"-----BEGIN CERTIFICATE-----",
		"ghp_",
		"gho_",
		"ghu_",
		"ghs_",
		"ghr_",
		"sk-",
	}

	for _, indicator := range secretIndicators {
		if strings.Contains(text, indicator) {
			result.Warnings = append(result.Warnings, "potential secret detected in: "+path+" ("+indicator+"...)")
			break
		}
	}

	pathLower := strings.ToLower(path)
	if pathLower == ".env" || strings.HasSuffix(pathLower, ".env") {
		result.Warnings = append(result.Warnings, "environment file detected: "+path)
	}
}
