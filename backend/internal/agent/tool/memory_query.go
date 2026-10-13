package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	queryParams, err := ParseParametersSchema(json.RawMessage(`{
		"type":"object",
		"required":["query"],
		"additionalProperties":false,
		"properties":{
			"query":{"type":"string","minLength":1,"maxLength":4096},
			"mode":{"type":"string","enum":["hybrid","keyword","vector"]},
			"limit":{"type":"integer","minimum":1,"maximum":50},
			"min_importance":{"type":"integer","minimum":0,"maximum":10},
			"types":{"type":"array","maxItems":32,"items":{"type":"string"}},
			"layers":{"type":"array","maxItems":6,"items":{"type":"string","enum":["fact","profile","episodic","working","worldbook","graph"]}},
			"time_basis":{"type":"string","enum":["occurred","validity","created","updated","last_used"]},
			"from":{"type":"string"},
			"to":{"type":"string"},
			"at":{"type":"string"},
			"local_date_from":{"type":"string"},
			"local_date_to":{"type":"string"},
			"include_unknown_time":{"type":"boolean"}
		}
	}`))
	if err != nil {
		panic(err)
	}
	RegisterMemory(Tool{Type: "function", Function: Function{
		Name:        "query_memory",
		Description: "Search Amitia memory with hybrid semantic/keyword retrieval. Supports type, layer, importance and temporal filtering while enforcing the current character scope.",
		Parameters:  queryParams,
	}}, queryMemory)

	titleParams, err := ParseParametersSchema(json.RawMessage(`{
		"type":"object",
		"required":["title"],
		"additionalProperties":false,
		"properties":{
			"title":{"type":"string","minLength":1,"maxLength":512},
			"query":{"type":"string","maxLength":2048},
			"offset":{"type":"integer","minimum":0,"maximum":10000000},
			"max_chars":{"type":"integer","minimum":1,"maximum":50000}
		}
	}`))
	if err != nil {
		panic(err)
	}
	RegisterMemory(Tool{Type: "function", Function: Function{
		Name:        "get_memory_by_title",
		Description: "Read memory content by exact title/key inside the current character scope. Supports bounded chunking and an optional in-document query.",
		Parameters:  titleParams,
	}}, getMemoryByTitle)
}

func requireCharacterReadScope(execCtx ToolExecutionContext) (ToolExecutionContext, *ToolCallResult) {
	execCtx.CharacterID = strings.TrimSpace(execCtx.CharacterID)
	if execCtx.CharacterID == "" {
		result := ErrorResult("missing_character_scope", "ERROR: character scope is required")
		result.Audit = map[string]interface{}{"conversation_id": execCtx.ConversationID, "channel": execCtx.Channel}
		return execCtx, &result
	}
	return execCtx, nil
}

func normalizedSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func stringSliceArg(args map[string]interface{}, key string, maxItems int) []string {
	raw, ok := args[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, min(len(raw), maxItems))
	for _, item := range raw {
		if len(result) >= maxItems {
			break
		}
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func stringArg(args map[string]interface{}, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func boundedIntArg(args map[string]interface{}, key string, fallback, low, high int) int {
	value := fallback
	switch raw := args[key].(type) {
	case float64:
		value = int(raw)
	case float32:
		value = int(raw)
	case int:
		value = raw
	case int64:
		value = int(raw)
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func queryMemory(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "query_memory", args)
}

func getMemoryByTitle(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "get_memory_by_title", args)
}
