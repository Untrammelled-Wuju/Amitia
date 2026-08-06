// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"strings"
)

type EnvironmentSanitizer interface {
	Sanitize([]string) []string
}

type defaultEnvironmentSanitizer struct{}

func NewEnvironmentSanitizer() EnvironmentSanitizer {
	return &defaultEnvironmentSanitizer{}
}

func (s *defaultEnvironmentSanitizer) Sanitize(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "\x00") {
			continue
		}
		idx := strings.IndexByte(entry, '=')
		if idx < 0 {
			continue
		}
		key := entry[:idx]
		if s.shouldRemove(key) {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func (s *defaultEnvironmentSanitizer) shouldRemove(key string) bool {
	upper := strings.ToUpper(key)
	if strings.HasPrefix(upper, "QDRANT__") {
		return true
	}
	if upper == "QDRANT_CONFIG_PATH" || upper == "QDRANT__CONFIG_PATH" {
		return true
	}
	if upper == "QDRANT_RESOURCE_PROFILE" {
		return true
	}
	return false
}
