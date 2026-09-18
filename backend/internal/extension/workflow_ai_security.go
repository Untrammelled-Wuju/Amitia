package extension

import (
	"strings"
)

type generationSecretIssue struct {
	Level   string
	Message string
}

func ScanWorkflowSecrets(raw []byte) []generationSecretIssue {
	if len(raw) == 0 || !secretPattern.Match(raw) {
		return nil
	}
	return []generationSecretIssue{{Level: "error", Message: "possible plaintext secret detected"}}
}

func hasErrorIssues(items []generationSecretIssue) bool {
	for _, item := range items {
		if strings.EqualFold(item.Level, "error") {
			return true
		}
	}
	return false
}

func sanitizeGenerationError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if len(text) > 500 {
		text = text[:500]
	}
	return text
}
