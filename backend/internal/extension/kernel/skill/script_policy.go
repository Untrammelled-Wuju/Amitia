// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ScriptFileInspector interface {
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	Abs(path string) (string, error)
	ReadLink(path string) (string, error)
}

type defaultScriptFileInspector struct{}

func (defaultScriptFileInspector) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (defaultScriptFileInspector) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (defaultScriptFileInspector) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (defaultScriptFileInspector) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (defaultScriptFileInspector) ReadLink(path string) (string, error) {
	return os.Readlink(path)
}

type ScriptPolicyContext struct {
	Inspector     ScriptFileInspector
	MaxFileSize   int64
	AllowSymlink  bool
	AllowHardlink bool
}

func DefaultScriptPolicyContext() ScriptPolicyContext {
	return ScriptPolicyContext{
		Inspector:     defaultScriptFileInspector{},
		MaxFileSize:   1 << 20,
		AllowSymlink:  false,
		AllowHardlink: false,
	}
}

func ValidateScriptPath(skillRoot, relPath string, policy ScriptPolicyContext) (string, error) {
	if relPath == "" {
		return "", ErrScriptPathEscape
	}
	if filepath.IsAbs(relPath) {
		return "", ErrScriptPathEscape
	}
	if strings.HasPrefix(relPath, "/") {
		return "", ErrScriptPathEscape
	}
	if len(relPath) > 0 && relPath[0] == '\\' {
		return "", ErrScriptPathEscape
	}
	if strings.Contains(relPath, "..") {
		return "", ErrScriptPathEscape
	}
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") {
		return "", ErrScriptPathEscape
	}

	inspector := policy.Inspector
	if inspector == nil {
		inspector = defaultScriptFileInspector{}
	}

	full := filepath.Join(skillRoot, clean)
	absPath, err := inspector.Abs(full)
	if err != nil {
		return "", ErrScriptPathEscape
	}
	rootClean, err := inspector.Abs(skillRoot)
	if err != nil {
		return "", ErrScriptPathEscape
	}
	if !strings.HasPrefix(absPath, rootClean+string(os.PathSeparator)) && absPath != rootClean {
		return "", ErrScriptPathEscape
	}

	lstat, err := inspector.Lstat(absPath)
	if err != nil {
		return "", ErrScriptPathEscape
	}

	if lstat.Mode()&fs.ModeSymlink != 0 && !policy.AllowSymlink {
		return "", ErrScriptSymlinkForbidden
	}

	if !lstat.IsDir() && !lstat.Mode().IsRegular() {
		return "", ErrScriptHardlinkForbidden
	}

	if lstat.Mode().IsRegular() {
		info, err := inspector.Stat(absPath)
		if err != nil {
			return "", ErrScriptPathEscape
		}
		if info.Size() > policy.MaxFileSize {
			return "", ErrScriptPathEscape
		}
		if info.Size() == 0 {
			return "", ErrScriptInvalidDescriptor
		}
	}

	return absPath, nil
}

func VerifyScriptHash(ctx context.Context, absPath string, expectedHash string, inspector ScriptFileInspector) error {
	if inspector == nil {
		inspector = defaultScriptFileInspector{}
	}
	data, err := inspector.ReadFile(absPath)
	if err != nil {
		return ErrScriptInternalError
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, expectedHash) {
		return ErrScriptHashMismatch
	}
	return nil
}

func ComputeFileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ValidateScriptRuntime(runtime string) error {
	switch runtime {
	case ScriptRuntimeNode, ScriptRuntimeNative:
		return nil
	default:
		return ErrScriptInterpreterUnavailable
	}
}

func ValidateScriptKind(kind string) error {
	switch kind {
	case ScriptKindExec, ScriptKindQuery, ScriptKindRender:
		return nil
	default:
		return ErrScriptInvalidDescriptor
	}
}

func ValidateTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = DefaultScriptTimeout
	}
	if timeout > MaxScriptTimeout {
		return ErrScriptInvalidTimeout
	}
	return nil
}

func ValidateWorkingDirPolicy(policy string) error {
	switch policy {
	case WorkingDirPolicySkillRoot, WorkingDirPolicyTemp, WorkingDirPolicyExplicit, "":
		return nil
	default:
		return ErrScriptInvalidWorkingDir
	}
}

func ValidateOutputMode(mode string) error {
	switch mode {
	case OutputModeStdout, OutputModeFile, OutputModeResource, "":
		return nil
	default:
		return ErrScriptInvalidOutputMode
	}
}

func SanitizeArgs(args map[string]any, spec []SkillScriptArgSpec) (map[string]any, error) {
	if args == nil {
		args = make(map[string]any)
	}
	result := make(map[string]any)
	specMap := make(map[string]SkillScriptArgSpec)
	for _, s := range spec {
		specMap[s.Name] = s
	}

	for name, spec := range specMap {
		val, exists := args[name]
		if !exists {
			if spec.Required && spec.Default == "" {
				return nil, fmt.Errorf("%w: %s", ErrScriptArgMissing, name)
			}
			if spec.Default != "" {
				result[name] = spec.Default
			}
			continue
		}
		if err := validateArgValue(name, val, spec); err != nil {
			return nil, err
		}
		result[name] = val
	}

	for name := range args {
		if _, ok := specMap[name]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrScriptArgInvalid, name)
		}
	}

	return result, nil
}

func validateArgValue(name string, val any, spec SkillScriptArgSpec) error {
	switch spec.Type {
	case ArgTypeString, "":
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %s expected string", ErrScriptArgTypeMismatch, name)
		}
		if spec.MaxLength > 0 && len(s) > spec.MaxLength {
			return fmt.Errorf("%w: %s exceeds max length", ErrScriptArgInvalid, name)
		}
		if len(spec.Enum) > 0 {
			found := false
			for _, e := range spec.Enum {
				if s == e {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: %s not in enum", ErrScriptArgInvalid, name)
			}
		}
	case ArgTypeInt:
		var intVal int64
		switch v := val.(type) {
		case int:
			intVal = int64(v)
		case int64:
			intVal = v
		case float64:
			intVal = int64(v)
		default:
			return fmt.Errorf("%w: %s expected int", ErrScriptArgTypeMismatch, name)
		}
		if err := checkIntBounds(name, intVal, spec); err != nil {
			return err
		}
	case ArgTypeFloat:
		var floatVal float64
		switch v := val.(type) {
		case float64:
			floatVal = v
		case int:
			floatVal = float64(v)
		default:
			return fmt.Errorf("%w: %s expected float", ErrScriptArgTypeMismatch, name)
		}
		if err := checkFloatBounds(name, floatVal, spec); err != nil {
			return err
		}
	case ArgTypeBool:
		_, ok := val.(bool)
		if !ok {
			return fmt.Errorf("%w: %s expected bool", ErrScriptArgTypeMismatch, name)
		}
	case ArgTypePath:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %s expected path string", ErrScriptArgTypeMismatch, name)
		}
		if filepath.IsAbs(s) || strings.Contains(s, "..") {
			return fmt.Errorf("%w: %s path must be relative", ErrScriptArgInvalid, name)
		}
	case ArgTypeEnum:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %s expected string", ErrScriptArgTypeMismatch, name)
		}
		if len(spec.Enum) > 0 {
			found := false
			for _, e := range spec.Enum {
				if s == e {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: %s not in enum", ErrScriptArgInvalid, name)
			}
		}
	}
	return nil
}

func checkIntBounds(name string, val int64, spec SkillScriptArgSpec) error {
	if spec.MinInt != nil && val < *spec.MinInt {
		return fmt.Errorf("%w: %s below minimum", ErrScriptArgInvalid, name)
	}
	if spec.MaxInt != nil && val > *spec.MaxInt {
		return fmt.Errorf("%w: %s above maximum", ErrScriptArgInvalid, name)
	}
	return nil
}

func checkFloatBounds(name string, val float64, spec SkillScriptArgSpec) error {
	if spec.MinFloat != nil && val < *spec.MinFloat {
		return fmt.Errorf("%w: %s below minimum", ErrScriptArgInvalid, name)
	}
	if spec.MaxFloat != nil && val > *spec.MaxFloat {
		return fmt.Errorf("%w: %s above maximum", ErrScriptArgInvalid, name)
	}
	return nil
}

func ValidateDependencies(deps []SkillScriptDependency) error {
	for _, dep := range deps {
		if dep.Kind == "npm" || dep.Kind == "pip" || dep.Kind == "package" {
			return ErrScriptAutoInstallForbidden
		}
	}
	return nil
}
