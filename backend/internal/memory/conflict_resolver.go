package memory

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/prompt/textlib"
)

type ContradictionJudgment struct {
	Judgment string `json:"judgment"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

func (s *service) checkContradictionWithLLM(newFact, existingFact struct {
	Subcategory string
	Subject     string
	Summary     string
}) (*ContradictionJudgment, error) {
	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("no active model")
	}

	userMsg := fmt.Sprintf(
		textlib.MemoryContradictionUserMsgTemplate,
		existingFact.Subcategory, existingFact.Subject, existingFact.Summary,
		newFact.Subcategory, newFact.Subject, newFact.Summary,
	)

	messages := []map[string]interface{}{
		{"role": "system", "content": textlib.MemoryContradictionSystemPrompt},
		{"role": "user", "content": userMsg},
	}

	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		return nil, err
	}

	content = extractJSONObject(content)
	var judgment ContradictionJudgment
	if err := json.Unmarshal([]byte(content), &judgment); err != nil {
		return nil, fmt.Errorf("parse contradiction judgment: %w", err)
	}
	return &judgment, nil
}

func (s *service) resolveSemanticConflict(newMem, existingMem Memory) (string, error) {
	judgment, err := s.checkContradictionWithLLM(
		struct {
			Subcategory string
			Subject     string
			Summary     string
		}{
			Subcategory: newMem.MemoryType,
			Subject:     newMem.Key,
			Summary:     newMem.Value,
		},
		struct {
			Subcategory string
			Subject     string
			Summary     string
		}{
			Subcategory: existingMem.MemoryType,
			Subject:     existingMem.Key,
			Summary:     existingMem.Value,
		},
	)
	if err != nil {
		return "", err
	}

	switch judgment.Judgment {
	case "unrelated":
		return "keep_both", nil
	case "reinforce", "complement":
		return "merge", nil
	case "strong_conflict":
		switch judgment.Action {
		case "keep_new":
			return "replace", nil
		case "keep_old":
			return "keep_old", nil
		case "merge":
			return "merge", nil
		case "flag":
			return "flag", nil
		default:
			return "keep_new", nil
		}
	case "weak_conflict":
		switch judgment.Action {
		case "keep_new":
			return "replace", nil
		case "merge":
			return "merge", nil
		default:
			return "keep_both", nil
		}
	default:
		return "keep_both", nil
	}
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	i := strings.Index(raw, "{")
	j := strings.LastIndex(raw, "}")
	if i >= 0 && j > i {
		return raw[i : j+1]
	}
	return raw
}
