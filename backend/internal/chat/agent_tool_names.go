package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
)

func prepareAgentModelTools(definitions []tool.Tool) ([]tool.Tool, map[string]string) {
	modelTools := make([]tool.Tool, 0, len(definitions))
	aliases := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Function.Name)
		if name == "" {
			continue
		}
		alias := name
		if !validAgentModelToolName(alias) {
			var cleaned strings.Builder
			for _, char := range name {
				if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
					cleaned.WriteRune(char)
				} else {
					cleaned.WriteByte('_')
				}
			}
			prefix := cleaned.String()
			if len(prefix) > 48 {
				prefix = prefix[:48]
			}
			if prefix == "" {
				prefix = "tool"
			}
			digest := sha256.Sum256([]byte(name))
			alias = prefix + "_" + hex.EncodeToString(digest[:6])
		}
		if original, exists := aliases[alias]; exists {
			if original == name {
				continue
			}
			continue
		}
		definition.Function.Name = alias
		modelTools = append(modelTools, definition)
		aliases[alias] = name
	}
	return modelTools, aliases
}

func validAgentModelToolName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			return false
		}
	}
	return true
}
