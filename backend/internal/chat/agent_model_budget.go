package chat

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *service) compactAgentMessages(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, baseMessageCount int) []map[string]interface{} {
	return s.compactAgentMessagesWithToolBudget(ctx, cfg, messages, baseMessageCount, nil)
}

func estimateModelMessagesTokens(messages []map[string]interface{}) int {
	total := 0
	for _, message := range messages {
		total += 8
		if content, ok := message["content"].(string); ok {
			total += estimateTextTokens(content)
		}
		if parts, ok := message["parts"].([]ModelContentPart); ok {
			for _, part := range parts {
				total += estimateTextTokens(part.Text) + estimateTextTokens(part.Filename) + 12
				switch part.Type {
				case ContentTypeImage:
					switch strings.ToLower(part.Detail) {
					case "low":
						total += 100
					case "high":
						total += 2048
					default:
						total += 1536
					}
				case ContentTypeAudio:
					total += 4096
				case ContentTypeVideo:
					total += 8192
				case ContentTypeFile:
					total += 2048
				}
			}
		}
		if toolCalls, ok := message["tool_calls"]; ok {
			encoded, err := json.Marshal(toolCalls)
			if err == nil {
				total += len(encoded)/4 + 1
			}
		}
	}
	return total
}

func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	ascii := 0
	for _, value := range text {
		if value < 128 {
			ascii++
		}
	}
	nonASCII := runes - ascii
	return ascii/4 + nonASCII + 1
}
