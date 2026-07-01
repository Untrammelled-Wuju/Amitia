// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package log

import (
	"regexp"
	"strings"
)

var (
	apiKeyPattern   = regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key|token|password|authorization)[:=]\s*["']?([^\s"',;)]+)`)
	bearerPattern   = regexp.MustCompile(`(?i)bearer\s+([^\s"',;)]+)`)
	skPrefixPattern = regexp.MustCompile(`(sk-[^\s"',;)]{4,})`)
	phonePattern    = regexp.MustCompile(`(1[3-9]\d)(\d{4})(\d{4})`)
	idCardPattern   = regexp.MustCompile(`(\d{6})(\d{8})(\d{4})`)
)

func MaskSensitive(s string) string {
	if s == "" {
		return s
	}
	result := s
	result = apiKeyPattern.ReplaceAllString(result, "$1=***")
	result = bearerPattern.ReplaceAllString(result, "Bearer ***")
	result = skPrefixPattern.ReplaceAllStringFunc(result, func(m string) string {
		if len(m) <= 4 {
			return m
		}
		return m[:4] + strings.Repeat("*", len(m)-4)
	})
	result = phonePattern.ReplaceAllString(result, "$1****$3")
	result = idCardPattern.ReplaceAllString(result, "$1********$3")
	return result
}
