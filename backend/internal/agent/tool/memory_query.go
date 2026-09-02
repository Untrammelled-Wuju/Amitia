package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	memorysvc "github.com/u-ai/backend/internal/memory"
)

type memoryQueryItem struct {
	ID                 string  `json:"id"`
	Key                string  `json:"key"`
	Value              string  `json:"value"`
	MemoryType         string  `json:"memoryType"`
	Layer              string  `json:"layer"`
	Importance         int     `json:"importance"`
	Confidence         int     `json:"confidence"`
	Score              float64 `json:"score,omitempty"`
	MatchType          string  `json:"matchType,omitempty"`
	CreatedAt          string  `json:"createdAt,omitempty"`
	UpdatedAt          string  `json:"updatedAt,omitempty"`
	SourceConversation string  `json:"sourceConversationId,omitempty"`
}

type memoryQueryEnvelope struct {
	Query string            `json:"query"`
	Mode  string            `json:"mode"`
	Count int               `json:"count"`
	Items []memoryQueryItem `json:"items"`
}

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

func queryMemory(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scoped, scopeErr := requireCharacterReadScope(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scoped
	if toolMemoryService == nil {
		return ErrorResult("service_not_initialized", "ERROR: memory service not initialized")
	}

	query := strings.TrimSpace(stringArg(args, "query"))
	if query == "" {
		return ErrorResult("invalid_args", "ERROR: query is required")
	}
	mode := strings.ToLower(strings.TrimSpace(stringArg(args, "mode")))
	if mode == "" {
		mode = "hybrid"
	}
	if mode != "hybrid" && mode != "keyword" && mode != "vector" {
		return ErrorResult("invalid_args", "ERROR: mode must be hybrid, keyword or vector")
	}
	limit := boundedIntArg(args, "limit", 10, 1, 50)
	minImportance := boundedIntArg(args, "min_importance", 0, 0, 10)
	types := stringSliceArg(args, "types", 32)
	layers := stringSliceArg(args, "layers", 6)
	for _, layer := range layers {
		if !memorysvc.IsValidLayer(layer) {
			return ErrorResult("invalid_args", "ERROR: invalid memory layer: "+layer)
		}
	}
	timeFilter := memoryTimeFilterFromArgs(args)
	allowedTimeIDs, timeErr := timeScopedMemoryIDs(toolMemoryService, execCtx, timeFilter)
	if timeErr != nil {
		return ErrorResult("memory_time_filter_failed", "ERROR: "+timeErr.Error())
	}

	fetchLimit := limit * 5
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	if fetchLimit > 250 {
		fetchLimit = 250
	}
	var items []memoryQueryItem
	var searchErr error
	switch mode {
	case "keyword":
		var memories []memorysvc.Memory
		memories, searchErr = toolMemoryService.Search(&memorysvc.SearchMemoryRequest{
			Keyword: query, CharacterID: execCtx.CharacterID, UserID: execCtx.User, Limit: fetchLimit,
		})
		for _, memory := range memories {
			items = append(items, queryItemFromMemory(memory, 0, "keyword"))
		}
	case "vector":
		var results []memorysvc.VectorSearchResult
		results, searchErr = toolMemoryService.VectorSearch(&memorysvc.VectorSearchRequest{
			Query: query, Keyword: query, CharacterID: execCtx.CharacterID, UserID: execCtx.User,
			Limit: fetchLimit, ConversationID: execCtx.ConversationID, RequestID: execCtx.RequestID, Channel: execCtx.Channel,
		})
		for _, result := range results {
			item := queryItemFromMemory(result.Memory, float64(result.Score), result.MatchType)
			if result.MemoryLayer != "" {
				item.Layer = canonicalLayerForMemory(result.Memory.MemoryType)
			}
			items = append(items, item)
		}
	default:
		var results []memorysvc.HybridSearchResult
		results, searchErr = toolMemoryService.HybridSearch(&memorysvc.VectorSearchRequest{
			Query: query, Keyword: query, CharacterID: execCtx.CharacterID, UserID: execCtx.User,
			Limit: fetchLimit, ConversationID: execCtx.ConversationID, RequestID: execCtx.RequestID, Channel: execCtx.Channel,
		})
		for _, result := range results {
			items = append(items, queryItemFromMemory(result.Memory, result.Score, result.MatchType))
		}
	}
	if searchErr != nil {
		return ErrorResult("memory_query_failed", "ERROR: "+searchErr.Error())
	}

	typeSet := normalizedSet(types)
	layerSet := normalizedSet(layers)
	filtered := make([]memoryQueryItem, 0, min(limit, len(items)))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ID]; exists && item.ID != "" {
			continue
		}
		if item.Importance < minImportance {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(item.MemoryType))]; !ok {
				continue
			}
		}
		if len(layerSet) > 0 {
			if _, ok := layerSet[strings.ToLower(item.Layer)]; !ok {
				continue
			}
		}
		if allowedTimeIDs != nil {
			if _, ok := allowedTimeIDs[item.ID]; !ok {
				continue
			}
		}
		if item.ID != "" {
			seen[item.ID] = struct{}{}
		}
		filtered = append(filtered, item)
		if len(filtered) >= limit {
			break
		}
	}
	payload, err := json.Marshal(memoryQueryEnvelope{Query: query, Mode: mode, Count: len(filtered), Items: filtered})
	if err != nil {
		return ErrorResult("memory_query_encode_failed", "ERROR: "+err.Error())
	}
	return TextResult(string(payload))
}

func getMemoryByTitle(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scoped, scopeErr := requireCharacterReadScope(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scoped
	title := strings.TrimSpace(stringArg(args, "title"))
	if title == "" {
		return ErrorResult("invalid_args", "ERROR: title is required")
	}
	query := strings.TrimSpace(stringArg(args, "query"))
	offset := boundedIntArg(args, "offset", 0, 0, 10_000_000)
	maxChars := boundedIntArg(args, "max_chars", 20_000, 1, 50_000)

	type candidate struct {
		ID, Title, Content, Source, CreatedAt string
	}
	var matches []candidate
	seenIDs := make(map[string]struct{})
	if toolDB != nil {
		rows, err := toolDB.QueryContext(callCtx, `SELECT id, key, value, created_at FROM memories WHERE character_id = ? AND lower(trim(key)) = lower(trim(?)) ORDER BY updated_at DESC, created_at DESC LIMIT 20`, execCtx.CharacterID, title)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, rowTitle, content string
				var created sql.NullString
				if scanErr := rows.Scan(&id, &rowTitle, &content, &created); scanErr == nil {
					matches = append(matches, candidate{ID: id, Title: rowTitle, Content: content, Source: "memory", CreatedAt: created.String})
					seenIDs[id] = struct{}{}
				}
			}
		}
	}
	if toolMemoryService != nil {
		memories, err := toolMemoryService.Search(&memorysvc.SearchMemoryRequest{Keyword: title, CharacterID: execCtx.CharacterID, UserID: execCtx.User, Limit: 100})
		if err == nil {
			for _, memory := range memories {
				if !strings.EqualFold(strings.TrimSpace(memory.Key), title) {
					continue
				}
				if _, exists := seenIDs[memory.ID]; exists && memory.ID != "" {
					continue
				}
				matches = append(matches, candidate{ID: memory.ID, Title: memory.Key, Content: memory.Value, Source: "memory", CreatedAt: memory.CreatedAt})
				if memory.ID != "" {
					seenIDs[memory.ID] = struct{}{}
				}
			}
		}
	}
	if toolDB != nil {
		rows, err := toolDB.QueryContext(callCtx, `SELECT id, title, content, created_at FROM episodic_memories WHERE user_id = ? AND lower(trim(title)) = lower(trim(?)) ORDER BY created_at DESC LIMIT 20`, execCtx.CharacterID, title)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, rowTitle, content string
				var created sql.NullString
				if scanErr := rows.Scan(&id, &rowTitle, &content, &created); scanErr == nil {
					matches = append(matches, candidate{ID: id, Title: rowTitle, Content: content, Source: "episodic_memory", CreatedAt: created.String})
				}
			}
		}
	}
	if len(matches) == 0 {
		return ErrorResult("memory_not_found", fmt.Sprintf("ERROR: no memory found with exact title %q", title))
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].CreatedAt > matches[j].CreatedAt })
	selected := matches[0]
	content := selected.Content
	contentRunes := []rune(content)
	matchOffset := -1
	if query != "" {
		byteOffset := strings.Index(strings.ToLower(content), strings.ToLower(query))
		if byteOffset < 0 {
			return ErrorResult("memory_query_not_found", fmt.Sprintf("ERROR: title found but query %q was not found in its content", query))
		}
		matchOffset = len([]rune(content[:byteOffset]))
		offset = matchOffset
	}
	if offset > len(contentRunes) {
		offset = len(contentRunes)
	}
	end := offset + maxChars
	if end > len(contentRunes) {
		end = len(contentRunes)
	}
	result := map[string]interface{}{
		"id": selected.ID, "title": selected.Title, "source": selected.Source, "content": string(contentRunes[offset:end]),
		"offset": offset, "nextOffset": end, "totalChars": len(contentRunes), "truncated": end < len(contentRunes), "createdAt": selected.CreatedAt,
	}
	if query != "" {
		result["query"] = query
		result["matchOffset"] = matchOffset
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return ErrorResult("memory_encode_failed", "ERROR: "+err.Error())
	}
	return TextResult(string(encoded))
}

func queryItemFromMemory(memory memorysvc.Memory, score float64, matchType string) memoryQueryItem {
	return memoryQueryItem{
		ID: memory.ID, Key: memory.Key, Value: memory.Value, MemoryType: memory.MemoryType,
		Layer: canonicalLayerForMemory(memory.MemoryType), Importance: memory.Importance, Confidence: memory.Confidence,
		Score: score, MatchType: matchType, CreatedAt: memory.CreatedAt, UpdatedAt: memory.UpdatedAt, SourceConversation: memory.SourceConvID,
	}
}

func canonicalLayerForMemory(memoryType string) string {
	switch strings.ToLower(strings.TrimSpace(memoryType)) {
	case "profile", "user_profile", "personal_info", "hobby", "preference", "habit", "relationship", "nickname":
		return string(memorysvc.MemoryLayerProfile)
	case "episodic", "episode", "event", "moment", "scene":
		return string(memorysvc.MemoryLayerEpisodic)
	case "working", "working_memory", "summary", "current_summary":
		return string(memorysvc.MemoryLayerWorking)
	case "worldbook", "world_book":
		return string(memorysvc.MemoryLayerWorldbook)
	case "graph", "memory_graph":
		return string(memorysvc.MemoryLayerGraph)
	default:
		return string(memorysvc.MemoryLayerFact)
	}
}

func memoryTimeFilterFromArgs(args map[string]interface{}) *memorysvc.MemoryTimeFilter {
	basis := strings.TrimSpace(stringArg(args, "time_basis"))
	from := strings.TrimSpace(stringArg(args, "from"))
	to := strings.TrimSpace(stringArg(args, "to"))
	at := strings.TrimSpace(stringArg(args, "at"))
	localFrom := strings.TrimSpace(stringArg(args, "local_date_from"))
	localTo := strings.TrimSpace(stringArg(args, "local_date_to"))
	includeUnknown, _ := args["include_unknown_time"].(bool)
	if basis == "" && from == "" && to == "" && at == "" && localFrom == "" && localTo == "" && !includeUnknown {
		return nil
	}
	filter := &memorysvc.MemoryTimeFilter{Basis: memorysvc.MemoryTimeBasis(basis), LocalDateFrom: localFrom, LocalDateTo: localTo, IncludeUnknown: includeUnknown}
	if from != "" {
		filter.FromUTC = &from
	}
	if to != "" {
		filter.ToUTC = &to
	}
	if at != "" {
		filter.AtUTC = &at
	}
	return filter
}

func timeScopedMemoryIDs(service memorysvc.Service, execCtx ToolExecutionContext, filter *memorysvc.MemoryTimeFilter) (map[string]struct{}, error) {
	if filter == nil {
		return nil, nil
	}
	memories, err := service.Search(&memorysvc.SearchMemoryRequest{Keyword: "", CharacterID: execCtx.CharacterID, UserID: execCtx.User, Limit: 500, Time: filter})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(memories))
	for _, memory := range memories {
		ids[memory.ID] = struct{}{}
	}
	return ids, nil
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
