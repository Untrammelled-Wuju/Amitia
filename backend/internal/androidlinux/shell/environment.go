//go:build linux && !android

package shell

import (
	"fmt"
	"regexp"
	"strings"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	defaultPATH = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	defaultHOME = "/root"
	defaultTERM = "xterm-256color"
)

type EnvironmentBuilder struct {
	policy ShellPolicy
}

func NewEnvironmentBuilder(policy ShellPolicy) *EnvironmentBuilder {
	return &EnvironmentBuilder{policy: policy}
}

func (b *EnvironmentBuilder) Build(env map[string]string) (map[string]string, []string, error) {
	result := make(map[string]string)

	result["PATH"] = defaultPATH
	result["HOME"] = defaultHOME
	result["TMPDIR"] = "/tmp"
	result["TERM"] = defaultTERM

	allowedKeys := b.buildAllowedKeySet()

	for _, key := range b.policy.AllowedEnvironmentKeys {
		allowedKeys[key] = true
	}

	if env != nil {
		if len(env) > b.policy.MaxEnvironmentEntries {
			return nil, nil, ErrTooManyEnvEntries(len(env), b.policy.MaxEnvironmentEntries)
		}

		var totalSize int64
		for key, value := range env {
			totalSize += int64(len(key) + len(value))
			if totalSize > b.policy.MaxEnvironmentBytes {
				return nil, nil, EnvDataTooLarge(totalSize, b.policy.MaxEnvironmentBytes)
			}

			if !envKeyPattern.MatchString(key) {
				return nil, nil, ErrEnvironmentDenied(key)
			}

			if !allowedKeys[key] {
				return nil, nil, ErrEnvironmentDenied(key)
			}

			result[key] = value
		}
	}

	return result, b.toEnvSlice(result), nil
}

func (b *EnvironmentBuilder) buildAllowedKeySet() map[string]bool {
	set := make(map[string]bool)
	for _, key := range b.policy.AllowedEnvironmentKeys {
		set[key] = true
	}
	return set
}

func (b *EnvironmentBuilder) toEnvSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func SanitizeEnvValue(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	return value
}
