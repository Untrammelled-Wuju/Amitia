package extension

import (
	"regexp"
	"strings"
)

var workflowSecretPattern = regexp.MustCompile(`(?i)(api[_ -]?key|access[_ -]?token|secret|password|authorization)\s*[:=]\s*["']?[A-Za-z0-9_\-]{12,}`)

type generationSecretIssue struct {
	Level   string
	Message string
}

func ScanWorkflowSecrets(raw []byte) []generationSecretIssue {
	if len(raw) == 0 || !workflowSecretPattern.Match(raw) {
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
	return strings.TrimSpace(err.Error())
}
