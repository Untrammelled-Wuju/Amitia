package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
	amitialog "log"
)

var ErrMemorySemanticUnavailable = errors.New("记忆语义检索不可用")

type MemoryManagementInput struct {
	RetentionLevel           int    `json:"retentionLevel,omitempty"`
	Pinned                   bool   `json:"pinned,omitempty"`
	Action                   string `json:"action"`
	Key                      string `json:"key"`
	Value                    string `json:"value"`
	MemoryType               string `json:"memoryType"`
	Partition                string `json:"partition,omitempty"`
	Importance               int    `json:"importance"`
	ConflictID               string `json:"conflictId,omitempty"`
	ExpectedConflictRevision int64  `json:"expectedConflictRevision,omitempty"`
	Resolution               string `json:"resolution,omitempty"`
}

type MemoryManagementQuery struct {
	ContextUsable     bool              `json:"contextUsable,omitempty"`
	IncludeDormant    bool              `json:"includeDormant,omitempty"`
	Types             []string          `json:"types,omitempty"`
	Layers            []string          `json:"layers,omitempty"`
	MinimumImportance int               `json:"minimumImportance,omitempty"`
	Time              *MemoryTimeFilter `json:"time,omitempty"`
	ExactTitle        bool              `json:"exactTitle,omitempty"`
	RetentionLevel    int               `json:"retentionLevel,omitempty"`
	Pinned            *bool             `json:"pinned,omitempty"`
	DecayState        string            `json:"decayState,omitempty"`
	Query             string            `json:"query"`
	Mode              string            `json:"mode"`
	MemoryType        string            `json:"memoryType,omitempty"`
	Source            string            `json:"source,omitempty"`
	Sort              string            `json:"sort,omitempty"`
	Cursor            string            `json:"cursor,omitempty"`
	Limit             int               `json:"limit,omitempty"`
}

type MemoryManagementResponse struct {
	Total           int                           `json:"total"`
	Resources       []coordination.Resource       `json:"resources"`
	Scope           coordination.ExecutionScope   `json:"executionScope"`
	Saved           bool                          `json:"saved"`
	Acknowledgement coordination.Acknowledgement  `json:"acknowledgement"`
	NextCursor      string                        `json:"nextCursor,omitempty"`
	Scores          map[string]float64            `json:"scores,omitempty"`
	ScoreDetails    map[string]MemoryScoreDetails `json:"scoreDetails,omitempty"`
}

type MemoryScoreDetails struct {
	FinalScore      float64 `json:"finalScore"`
	SemanticScore   float64 `json:"semanticScore"`
	KeywordMatch    bool    `json:"keywordMatch"`
	Importance      float64 `json:"importance"`
	Confidence      float64 `json:"confidence"`
	Strength        float64 `json:"strength"`
	Recency         float64 `json:"recency"`
	Verification    float64 `json:"verification"`
	Reinforcement   float64 `json:"reinforcement"`
	Utility         float64 `json:"utility"`
	Validity        float64 `json:"validity"`
	TemporalBoost   float64 `json:"temporalBoost"`
	ValidityPenalty float64 `json:"validityPenalty"`
}

type MemoryManagementConflict struct {
	Resources []coordination.Resource `json:"conflicts"`
}

type memoryManagementCursor struct {
	Scope   string `json:"scope"`
	Filter  string `json:"filter"`
	Catalog string `json:"catalog"`
	Offset  int    `json:"offset"`
}

func memoryManagementHash(value any) string {
	digest := sha256.Sum256(body(value))
	return hex.EncodeToString(digest[:])
}

func (e MemoryManagementConflict) Error() string {
	return "同一角色已有同键记忆，请确认原记录版本后选择冲突处理方式"
}

func (e *Engine) memoryManagementScope(ctx context.Context, request Request, write bool) (context.Context, coordination.ExecutionScope, func(), error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ctx, scope, finish, err
	}
	if write && (request.ExpectedScope == nil || summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope)) {
		finish()
		return ctx, scope, func() {}, coordination.ErrScopeExpired
	}
	return ctx, scope, finish, nil
}

func (e *Engine) memoryManagementSnapshot(ctx context.Context, scope coordination.ExecutionScope, kind, conversation string) (coordination.DataSnapshot, error) {
	combined := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID}
	seen := map[string]bool{}
	cursor := ""
	bytes := 0
	for page := 0; page < 256; page++ {
		snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{Management: true, ResourceKind: kind, ConversationID: conversation, Cursor: cursor, Limit: 128})
		if err != nil {
			return combined, err
		}
		if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
			return combined, err
		}
		combined.Role = snapshot.Role
		for _, resource := range snapshot.Resources {
			if resource.Kind != kind || resource.Deleted {
				continue
			}
			key := resource.Kind + "/" + resource.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			bytes += len(resource.Body)
			combined.Resources = append(combined.Resources, resource)
		}
		if len(combined.Resources) > 32768 || bytes > 32<<20 {
			return combined, coordination.ErrPendingLimit
		}
		next := snapshot.NextCursors[kind]
		if next == "" {
			return combined, nil
		}
		if seen["cursor/"+next] || next == cursor {
			return combined, coordination.ErrRequestConflict
		}
		seen["cursor/"+next] = true
		cursor = next
	}
	return combined, coordination.ErrPendingLimit
}

func memoryManagementText(raw json.RawMessage) (string, string, string, string, int) {
	var outer struct {
		Key     string          `json:"key"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &outer) != nil {
		return "", "", "", "", 0
	}
	var content struct {
		Value      string `json:"value"`
		Text       string `json:"text"`
		Type       string `json:"memoryType"`
		Source     string `json:"source"`
		Importance int    `json:"importance"`
	}
	if json.Unmarshal(outer.Content, &content) != nil {
		var text string
		if err := json.Unmarshal(outer.Content, &text); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return outer.Key, text, "", "", 0
	}
	if content.Value == "" {
		content.Value = content.Text
	}
	return outer.Key, content.Value, content.Type, content.Source, content.Importance
}

func (e *Engine) SearchOwnedMemories(ctx context.Context, request Request, input MemoryManagementQuery) (MemoryManagementResponse, error) {
	if err := validateMemoryTimeFilter(input.Time); err != nil {
		return MemoryManagementResponse{}, err
	}
	if len(input.Types) > 32 || len(input.Layers) > 6 || input.MinimumImportance < 0 || input.MinimumImportance > 10 {
		return MemoryManagementResponse{}, errors.New("记忆筛选参数无效")
	}
	for _, layer := range input.Layers {
		if !IsValidLayer(layer) {
			return MemoryManagementResponse{}, errors.New("记忆层筛选参数无效")
		}
	}
	now := time.Now()
	if input.RetentionLevel < 0 || input.RetentionLevel > 5 || input.DecayState != "" && input.DecayState != "active" && input.DecayState != "fading" && input.DecayState != "dormant" && input.DecayState != "archived" {
		return MemoryManagementResponse{}, errors.New("记忆生命周期筛选参数无效")
	}
	if len(input.Query) > 4096 || !utf8.ValidString(input.Query) || strings.ContainsRune(input.Query, 0) || input.Limit < 0 || input.Limit > 128 || len(input.Cursor) > 4096 || input.Mode != "" && input.Mode != "keyword" && input.Mode != "hybrid" && input.Mode != "vector" {
		return MemoryManagementResponse{}, errors.New("记忆检索参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	rankingProfile, _, err := e.memoryRankingProfile(ctx, scope)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	result := MemoryManagementResponse{Scope: scope, Resources: []coordination.Resource{}, Scores: map[string]float64{}, ScoreDetails: map[string]MemoryScoreDetails{}}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	semantic := map[string]float64{}
	availability := func(body json.RawMessage) bool {
		return coordination.ResourceAvailable(body, time.Now(), input.IncludeDormant)
	}
	if input.Mode == "hybrid" || input.Mode == "vector" {
		if query == "" {
			return MemoryManagementResponse{}, errors.New("语义检索内容不能为空")
		}
		model, ok := e.model.(SemanticModel)
		if !ok {
			return MemoryManagementResponse{}, fmt.Errorf("%w: 当前 Core 尚未配置记忆检索模型", ErrMemorySemanticUnavailable)
		}
		values, fingerprint, err := model.OwnedQueryVector(ctx, input.Query)
		if err != nil {
			return MemoryManagementResponse{}, fmt.Errorf("%w: %w", ErrMemorySemanticUnavailable, err)
		}
		if err := coordination.ValidateQueryVector(coordination.DataQuery{Vector: values, VectorModel: fingerprint}); err != nil || len(values) == 0 {
			return MemoryManagementResponse{}, fmt.Errorf("%w: 当前 Core 的检索向量无效", ErrMemorySemanticUnavailable)
		}
		vectors, err := e.memoryManagementSnapshot(ctx, scope, "vector", "")
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		usable := map[string]bool{}
		for _, resource := range snapshot.Resources {
			usable[resource.ID] = availability(resource.Body)
		}
		for _, resource := range vectors.Resources {
			if !usable[resource.SourceID] || !availability(resource.Body) {
				continue
			}
			var vector struct {
				Content struct {
					Values []float32 `json:"values"`
					Model  string    `json:"modelFingerprint"`
				} `json:"content"`
			}
			if json.Unmarshal(resource.Body, &vector) != nil || vector.Content.Model != fingerprint || len(vector.Content.Values) != len(values) {
				continue
			}
			var dot, left, right float64
			valid := true
			for index, value := range vector.Content.Values {
				x, y := float64(value), float64(values[index])
				if math.IsNaN(x) || math.IsInf(x, 0) {
					valid = false
					break
				}
				dot += x * y
				left += x * x
				right += y * y
			}
			if valid && left > 0 && right > 0 {
				score := dot / math.Sqrt(left*right)
				if score > semantic[resource.SourceID] {
					semantic[resource.SourceID] = score
				}
			}
		}
	}
	for _, resource := range snapshot.Resources {
		key, value, kind, source, importance := memoryManagementText(resource.Body)
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if input.ContextUsable && !availability(resource.Body) {
			continue
		}
		var content map[string]json.RawMessage
		if err := json.Unmarshal(document["content"], &content); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		layer := MemoryText(document, "kind")
		if layer == "" {
			layer = "fact"
		}
		if importance < input.MinimumImportance || !memorySetMatches(input.Types, kind) || !memorySetMatches(input.Layers, layer) || !memoryTimeMatches(document, content, input.Time) || input.ExactTitle && key != input.Query {
			continue
		}
		state := memorypolicy.ReadLifecycle(document, now).Effective(now)
		if input.RetentionLevel != 0 && input.RetentionLevel != state.RetentionLevel || input.Pinned != nil && *input.Pinned != state.Pinned || input.DecayState != "" && input.DecayState != state.DecayState {
			continue
		}
		if input.MemoryType != "" && input.MemoryType != kind || input.Source != "" && input.Source != source {
			continue
		}
		text := strings.ToLower(key + " " + value)
		keyword := query == "" || strings.Contains(text, query)
		if input.Mode == "vector" && semantic[resource.ID] <= 0 || input.Mode != "vector" && !keyword && semantic[resource.ID] <= 0 {
			continue
		}
		result.Resources = append(result.Resources, resource)
		if input.Sort == "importance_desc" {
			result.Scores[resource.ID] = float64(importance)
			result.ScoreDetails[resource.ID] = MemoryScoreDetails{FinalScore: float64(importance), SemanticScore: semantic[resource.ID], KeywordMatch: keyword, Importance: float64(importance), Strength: state.MemoryStrength, Validity: memoryValidityFactor(content, now), ValidityPenalty: 1}
		} else {
			details := memoryRetrievalScore(input.Mode, rankingProfile.Weights, semantic[resource.ID], keyword, state, content, document, now)
			result.Scores[resource.ID] = details.FinalScore
			result.ScoreDetails[resource.ID] = details
		}
	}
	temporalRank := false
	if hasTemporalIntent(input.Query) && input.Sort == "" {
		enabled := true
		if policy, ok := e.model.(MemoryTemporalPolicy); ok {
			var err error
			enabled, err = policy.OwnedMemoryTemporalEnabled(ctx, scope.SpaceID)
			if err != nil {
				return MemoryManagementResponse{}, err
			}
		}
		if enabled {
			temporalRank = true
			candidates := []MemoryScoreCandidate{}
			metadata := map[string]MemoryTemporalMetadata{}
			for _, resource := range result.Resources {
				var document map[string]json.RawMessage
				if err := json.Unmarshal(resource.Body, &document); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				var item MemoryTemporalMetadata
				if err := json.Unmarshal(document["content"], &item); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				_, _, kind, _, _ := memoryManagementText(resource.Body)
				metadata[resource.ID] = item
				candidates = append(candidates, MemoryScoreCandidate{MemoryID: resource.ID, BaseScore: result.Scores[resource.ID], MemoryType: kind})
			}
			reranked, err := RerankMemoryScores(ctx, input.Query, candidates, metadata, now)
			if err != nil {
				return MemoryManagementResponse{}, err
			}
			for id, score := range reranked {
				result.Scores[id] = score.FinalScore
				details := result.ScoreDetails[id]
				details.FinalScore = score.FinalScore
				details.TemporalBoost = score.TemporalBoost
				details.ValidityPenalty = score.ValidityPenalty
				result.ScoreDetails[id] = details
			}
		}
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		if (temporalRank || input.Sort == "importance_desc" || input.Mode == "hybrid" || input.Mode == "vector") && result.Scores[result.Resources[i].ID] != result.Scores[result.Resources[j].ID] {
			return result.Scores[result.Resources[i].ID] > result.Scores[result.Resources[j].ID]
		}
		return result.Resources[i].ID < result.Resources[j].ID
	})
	result.Resources = dedupeMemorySearchResults(result.Resources)
	catalog := []coordination.ResourceVersion{}
	for _, resource := range result.Resources {
		catalog = append(catalog, coordination.ResourceVersion{Kind: resource.Kind, ID: resource.ID, Revision: resource.Revision})
	}
	filter := input
	result.Total = len(result.Resources)
	filter.Cursor, filter.Limit = "", 0
	page := memoryManagementCursor{Scope: memoryManagementHash(summaryAuthority(scope)), Filter: memoryManagementHash(filter), Catalog: memoryManagementHash(catalog)}
	if input.Cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		var previous memoryManagementCursor
		if err != nil || json.Unmarshal(encoded, &previous) != nil || previous.Scope != page.Scope || previous.Filter != page.Filter || previous.Catalog != page.Catalog || previous.Offset < 1 || previous.Offset >= len(result.Resources) {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		page.Offset = previous.Offset
		result.Resources = result.Resources[page.Offset:]
	}
	limit := input.Limit
	if limit == 0 {
		limit = 32
	}
	if len(result.Resources) > limit {
		page.Offset += limit
		result.NextCursor = base64.RawURLEncoding.EncodeToString(body(page))
		result.Resources = result.Resources[:limit]
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return MemoryManagementResponse{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return MemoryManagementResponse{}, err
	}
	for index, resource := range result.Resources {
		result.Resources[index].Body = memorypolicy.ProjectLifecycle(resource.Body, time.Now())
	}
	return result, nil
}

func dedupeMemorySearchResults(resources []coordination.Resource) []coordination.Resource {
	selected := make([]coordination.Resource, 0, len(resources))
	seen := map[string]bool{}
	for _, resource := range resources {
		key := memoryTextFromBody(resource.Body, "kind") + "\x00" + strings.ToLower(memoryTextFromBody(resource.Body, "key"))
		if key == "\x00" {
			selected = append(selected, resource)
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		selected = append(selected, resource)
	}
	return selected
}

func memoryTextFromBody(body json.RawMessage, key string) string {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		amitialog.Printf("[original-error] %v", err)
		return ""
	}
	return MemoryText(document, key)
}

func memoryRetrievalScore(mode string, weights memoryRankingWeights, semantic float64, keyword bool, state memorypolicy.Lifecycle, content, document map[string]json.RawMessage, now time.Time) MemoryScoreDetails {
	semantic = math.Max(0, math.Min(1, semantic))
	keywordScore := 0.0
	if keyword {
		keywordScore = 1
	}
	importance := math.Max(0, math.Min(10, memoryNumber(content["importance"], 0))) / 10
	confidence := math.Max(0, math.Min(100, memoryNumber(content["confidence"], 50))) / 100
	strength := math.Max(0, math.Min(1, state.MemoryStrength))
	recency := memoryRecency(document, content, now)
	verification := 0.55
	source := memoryOptionalText(content, "source")
	if source == "" {
		source = memoryOptionalText(document, "source")
	}
	switch strings.ToLower(source) {
	case "manual", "reviewed", "verified", "confirmed", "user_correction":
		verification = 1
	}
	status := memoryOptionalText(content, "verifiedStatus")
	if status == "" {
		status = memoryOptionalText(document, "verifiedStatus")
	}
	switch strings.ToLower(status) {
	case "verified", "confirmed", "reviewed", "user_corrected":
		verification = 1
	case "rejected", "invalidated", "disputed":
		verification = 0
	}
	reinforcement := math.Min(5, math.Max(0, memoryNumber(document["reinforceCount"], 0))) / 5
	utility := math.Max(0, math.Min(1, memoryNumber(document["retrievalUtility"], .5)))
	validity := memoryValidityFactor(content, now)
	details := MemoryScoreDetails{SemanticScore: semantic, KeywordMatch: keyword, Importance: importance, Confidence: confidence, Strength: strength, Recency: recency, Verification: verification, Reinforcement: reinforcement, Utility: utility, Validity: validity, ValidityPenalty: 1}
	if mode == "vector" {
		weights.Semantic += .08
		weights.Keyword -= .08
	} else if mode == "keyword" {
		weights.Keyword += .18
		weights.Semantic -= .10
		weights.Confidence -= .03
		weights.Utility -= .02
		weights.Validity -= .03
	}
	weights = weights.normalize()
	details.FinalScore = semantic*weights.Semantic + keywordScore*weights.Keyword + importance*weights.Importance +
		confidence*weights.Confidence + strength*weights.Strength + recency*weights.Recency + verification*weights.Verification +
		reinforcement*weights.Reinforcement + utility*weights.Utility + validity*weights.Validity
	details.FinalScore *= validity
	return details
}

func memoryValidityFactor(content map[string]json.RawMessage, now time.Time) float64 {
	start := memoryOptionalText(content, "validFromUtc")
	if start == "" {
		start = memoryOptionalText(content, "validFrom")
	}
	if start != "" {
		from, err := parseMemoryPolicyTime(start)
		if err != nil {
			amitialog.Printf("[original-error] %v", err)
			return 0
		}
		if now.Before(from) {
			return .65
		}
	}
	end := memoryOptionalText(content, "validToUtc")
	if end == "" {
		end = memoryOptionalText(content, "validTo")
	}
	if end == "" {
		end = memoryOptionalText(content, "expiresAt")
	}
	if end != "" {
		until, err := parseMemoryPolicyTime(end)
		if err != nil {
			amitialog.Printf("[original-error] %v", err)
			return 0
		}
		if !now.Before(until) {
			return .35
		}
	}
	return 1
}

func parseMemoryPolicyTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		if layout == "2006-01-02" {
			return time.Time{}, err
		}
	}
	return time.Time{}, nil
}

func memoryRecency(document, content map[string]json.RawMessage, now time.Time) float64 {
	raw := ""
	for _, value := range []string{memoryOptionalText(content, "occurredAtUtc"), memoryOptionalText(content, "updatedAtUtc"), memoryOptionalText(content, "createdAtUtc"), memoryOptionalText(document, "updatedAt"), memoryOptionalText(document, "createdAt")} {
		if value != "" {
			raw = value
			break
		}
	}
	if raw == "" {
		return .5
	}
	created, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		amitialog.Printf("[original-error] %v", err)
		return .5
	}
	age := now.Sub(created).Hours() / 24
	if age <= 0 {
		return 1
	}
	return math.Exp(-age / 180)
}

func memoryOptionalText(document map[string]json.RawMessage, key string) string {
	raw, exists := document[key]
	if !exists || len(raw) == 0 {
		return ""
	}
	return MemoryText(document, key)
}

func memoryNumber(raw json.RawMessage, fallback float64) float64 {
	if len(raw) == 0 {
		return fallback
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		amitialog.Printf("[original-error] %v", err)
		return fallback
	}
	return value
}

func (e *Engine) ManageOwnedMemory(ctx context.Context, request Request, input MemoryManagementInput) (MemoryManagementResponse, error) {
	if input.RetentionLevel < 0 || input.RetentionLevel > 5 {
		return MemoryManagementResponse{}, errors.New("记忆等级必须为 L1—L5")
	}
	input.Key, input.Value = strings.TrimSpace(input.Key), strings.TrimSpace(input.Value)
	if input.Action != "create" && input.Action != "resolve" || input.Key == "" || input.Value == "" || len(input.Key) > 512 || len(input.Value) > 128<<10 || !utf8.ValidString(input.Key+input.Value) || strings.ContainsRune(input.Key+input.Value, 0) || input.Importance < 0 || input.Importance > 10 || len(input.MemoryType) > 64 {
		return MemoryManagementResponse{}, errors.New("记忆内容或管理操作参数无效")
	}
	if input.Partition != "" && input.Partition != "memory" && input.Partition != "profile" {
		return MemoryManagementResponse{}, errors.New("记忆分区参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, true)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryManagementResponse{}, errors.New("数据所有者不支持记忆管理")
	}
	if scope.MemoryRoleID != "" && scope.MemoryRoleID != coordination.GlobalMemoryRole {
		roles, err := e.data.Roles(ctx, scope)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		if _, err := coordination.ResolveRole(scope.MemoryRoleID, roles); err != nil {
			return MemoryManagementResponse{}, err
		}
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00memory-management\x00" + scope.RoleID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope coordination.ExecutionScope
		Input MemoryManagementInput
	}{summaryAuthority(scope), input}))
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "memory-management/" + request.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return MemoryManagementResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return MemoryManagementResponse{}, err
		}
	}
	if receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return MemoryManagementResponse{}, err
	} else if receipt != nil {
		var saved struct {
			Hash     string                   `json:"hash"`
			Response MemoryManagementResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint || summaryAuthority(saved.Response.Scope) != summaryAuthority(scope) {
			return MemoryManagementResponse{}, coordination.ErrRequestConflict
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		return saved.Response, nil
	}
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	derivedKind := "fact"
	if input.Partition == "profile" {
		derivedKind = "profile"
	}
	identity := sha256.Sum256([]byte(coordination.StorageRole(scope, "memory") + "\x00memory\x00" + derivedKind + "/" + input.Key))
	memoryID := hex.EncodeToString(identity[:16])
	existing, err := port.Resource(ctx, scope, "memory", memoryID)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	dependencies := []coordination.ResourceVersion{}
	if existing != nil {
		if existing.Deleted {
			return MemoryManagementResponse{}, fmt.Errorf("%w: memory/%s", ErrMemoryDeleted, memoryID)
		}
		if input.Action != "resolve" {
			return MemoryManagementResponse{}, MemoryManagementConflict{Resources: []coordination.Resource{*existing}}
		}
		if input.ConflictID != existing.ID || input.ExpectedConflictRevision != existing.Revision {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		dependencies = append(dependencies, coordination.ResourceVersion{Kind: "memory", ID: existing.ID, Revision: existing.Revision})
		switch input.Resolution {
		case "replace":
		case "merge":
			_, previous, _, _, _ := memoryManagementText(existing.Body)
			input.Value = previous + "\n" + input.Value
			if len(input.Value) > 128<<10 {
				return MemoryManagementResponse{}, coordination.ErrPendingLimit
			}
		case "keep_both":
			input.Key += " · " + request.RequestID
			if len(input.Key) > 512 {
				return MemoryManagementResponse{}, errors.New("同时保留后的记忆标题过长，请缩短标题")
			}
		default:
			return MemoryManagementResponse{}, errors.New("请选择替换、合并或同时保留")
		}
	} else if input.Action == "resolve" {
		return MemoryManagementResponse{}, coordination.ErrResourceVersion
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	derived := []DerivedMemory{{Kind: derivedKind, Key: input.Key, Body: body(map[string]any{"value": input.Value, "memoryType": input.MemoryType, "importance": input.Importance, "source": "manual", "createdAt": now, "retentionLevel": input.RetentionLevel, "pinned": input.Pinned})}}
	mutations, err := e.prepareMemoryMutations(ctx, scope, "", derived, &snapshot)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	response := MemoryManagementResponse{Scope: scope, Saved: true, Resources: []coordination.Resource{}, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{}}}
	for _, mutation := range mutations {
		response.Acknowledgement.Versions[mutation.Kind+"/"+mutation.ID] = mutation.ExpectedRevision + 1
		response.Resources = append(response.Resources, coordination.Resource{OwnerID: scope.ResourceOwnerID, RoleID: coordination.StorageRole(scope, mutation.Kind), Kind: mutation.Kind, ID: mutation.ID, SourceID: mutation.SourceID, Revision: mutation.ExpectedRevision + 1, Body: mutation.Body})
	}
	response.Acknowledgement.Versions["checkpoint/"+receiptID] = 1
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "response": response})})
	commit := coordination.Commit{Scope: scope, Mutations: mutations, Dependencies: dependencies}
	ack, err := e.commit(ctx, commit)
	if err != nil {
		if !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrRequestConflict) && !errors.Is(err, coordination.ErrScopeExpired) {
			if pendingErr := e.coordination.Enqueue(ctx, commit); pendingErr != nil {
				return MemoryManagementResponse{}, errors.Join(err, pendingErr)
			}
		}
		return MemoryManagementResponse{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID {
		return MemoryManagementResponse{}, coordination.ErrWrongOwner
	}
	for key, version := range response.Acknowledgement.Versions {
		if ack.Versions[key] != version {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
	}
	response.Acknowledgement = ack
	return response, nil
}
