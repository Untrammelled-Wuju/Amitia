package execution

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		sensitiveFields: make(map[string]bool),
	}
}

type Sanitizer struct {
	sensitiveFields map[string]bool
}

func (s *Sanitizer) RegisterSensitiveField(name string) {
	s.sensitiveFields[strings.ToLower(name)] = true
}

func (s *Sanitizer) Sanitize(ctx context.Context, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	result = result.Clone()

	if result.Error != nil {
		result.Error = s.sanitizeError(result.Error)
	}

	if result.Structured != nil {
		result.Structured = s.sanitizeJSON(result.Structured)
	}

	sanitized := make([]capability.ToolContent, 0, len(result.Content))
	for _, c := range result.Content {
		if c.Type == capability.ToolContentStructured && c.Data != nil {
			c.Data = s.sanitizeJSON(c.Data)
		}
		sanitized = append(sanitized, c)
	}
	result.Content = sanitized

	return result
}

func (s *Sanitizer) sanitizeError(err *capability.ToolError) *capability.ToolError {
	if err == nil {
		return nil
	}
	if err.Details != nil {
		sanitized := make(map[string]any)
		for k, v := range err.Details {
			if s.isSensitive(k) {
				sanitized[k] = "[redacted]"
			} else {
				sanitized[k] = v
			}
		}
		err.Details = sanitized
	}
	return err
}

func (s *Sanitizer) sanitizeJSON(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}

	s.redactSensitiveKeys(obj)

	cleaned, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return cleaned
}

func (s *Sanitizer) redactSensitiveKeys(m map[string]any) {
	for k, v := range m {
		if s.isSensitive(k) {
			m[k] = "[redacted]"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			s.redactSensitiveKeys(nested)
		}
	}
}

func (s *Sanitizer) isSensitive(key string) bool {
	lower := strings.ToLower(key)
	return s.sensitiveFields[lower]
}

func (s *Sanitizer) sanitizeStreamContent(content *capability.ToolContent) *capability.ToolContent {
	if content == nil {
		return nil
	}
	if content.Type == capability.ToolContentStructured && len(content.Data) > 0 {
		content.Data = s.sanitizeJSON(content.Data)
	}
	return content
}
