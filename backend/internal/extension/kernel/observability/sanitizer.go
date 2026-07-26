package observability

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

type RecordSanitizer struct {
	sensitiveFields map[string]DataSensitivity
	secretPatterns  []*regexp.Regexp
}

func NewRecordSanitizer() *RecordSanitizer {
	return &RecordSanitizer{
		sensitiveFields: map[string]DataSensitivity{
			"token":        SensitivitySecret,
			"apikey":       SensitivitySecret,
			"api_key":      SensitivitySecret,
			"secret":       SensitivitySecret,
			"password":     SensitivitySecret,
			"passwd":       SensitivitySecret,
			"authorization": SensitivitySecret,
			"cookie":       SensitivitySecret,
			"key":          SensitivityRestricted,
			"credential":   SensitivitySecret,
			"private_key":  SensitivitySecret,
			"session_id":   SensitivitySensitive,
			"url":          SensitivitySensitive,
			"path":         SensitivitySensitive,
			"file":         SensitivitySensitive,
			"input":        SensitivityRestricted,
			"prompt":       SensitivityRestricted,
			"content":      SensitivityRestricted,
		},
		secretPatterns: []*regexp.Regexp{
			regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
			regexp.MustCompile(`ghp_[a-zA-Z0-9]{20,}`),
			regexp.MustCompile(`xox[bpras]-[a-zA-Z0-9-]+`),
		},
	}
}

func (s *RecordSanitizer) SanitizeInvocationInput(inv *InvocationRecord, rawInput string) {
	if rawInput == "" {
		return
	}
	inv.InputHash = HashInput(rawInput)
	cleaned := s.redactSecrets(rawInput)
	if len(cleaned) > 500 {
		cleaned = cleaned[:500] + "..."
	}
	inv.InputSummary = cleaned
}

func (s *RecordSanitizer) SanitizeInvocationOutput(inv *InvocationRecord, rawOutput string) {
	if rawOutput == "" {
		return
	}
	inv.OutputHash = HashOutput(rawOutput)
	cleaned := s.redactSecrets(rawOutput)
	if len(cleaned) > 500 {
		cleaned = cleaned[:500] + "..."
	}
	inv.OutputSummary = cleaned
}

func (s *RecordSanitizer) SanitizeMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	cleaned := make(map[string]any)
	for k, v := range meta {
		lower := strings.ToLower(k)
		sens, isSensitive := s.sensitiveFields[lower]
		if isSensitive && sens == SensitivitySecret {
			continue
		}
		if isSensitive && (sens == SensitivityRestricted || sens == SensitivitySensitive) {
			if str, ok := v.(string); ok {
				cleaned[k] = s.redactSecrets(str)
			} else {
				cleaned[k] = "[redacted]"
			}
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

func (s *RecordSanitizer) sanitizeErrorRecord(rec *ErrorRecord, rawError error) {
	if rawError == nil {
		return
	}
	msg := rawError.Error()
	rec.SanitizedMessage = s.redactSecrets(msg)
	if len(rec.SanitizedMessage) > 1000 {
		rec.SanitizedMessage = rec.SanitizedMessage[:1000] + "..."
	}
	h := sha256.Sum256([]byte(msg))
	rec.InternalReference = fmt.Sprintf("ref:%x", h[:8])
}

func (s *RecordSanitizer) SanitizeAuditEvent(ctx context.Context, evt *AuditEvent) {
	if evt.Metadata != nil {
		evt.Metadata = s.SanitizeMetadata(evt.Metadata)
	}
}

func (s *RecordSanitizer) ClassifySensitivity(field string) DataSensitivity {
	lower := strings.ToLower(field)
	if sens, ok := s.sensitiveFields[lower]; ok {
		return sens
	}
	return SensitivityInternal
}

func (s *RecordSanitizer) RegisterSensitiveField(field string, sensitivity DataSensitivity) {
	s.sensitiveFields[strings.ToLower(field)] = sensitivity
}

func (s *RecordSanitizer) redactSecrets(text string) string {
	for _, pattern := range s.secretPatterns {
		text = pattern.ReplaceAllString(text, "[redacted]")
	}
	return text
}
