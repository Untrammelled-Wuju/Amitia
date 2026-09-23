package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

const (
	assistantTurnStatusQueued      = "queued"
	assistantTurnStatusStarting    = "starting"
	assistantTurnStatusRunning     = "running"
	assistantTurnStatusWaitingTool = "waiting_tool"
	assistantTurnStatusCompleted   = "completed"
	assistantTurnStatusFailed      = "failed"
	assistantTurnStatusInterrupted = "interrupted"

	assistantTurnItemReasoning  = "reasoning"
	assistantTurnItemToolCall   = "tool_call"
	assistantTurnItemToolResult = "tool_result"
	assistantTurnItemText       = "text"
	assistantTurnItemError      = "error"
)

type AssistantTurn struct {
	ID             string              `gorm:"column:id;primaryKey" json:"id"`
	ConversationID string              `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	CharacterID    string              `gorm:"column:character_id;not null;default:''" json:"characterId"`
	UserMessageID  string              `gorm:"column:user_message_id;not null;default:''" json:"userMessageId"`
	RequestID      string              `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	ExecutionID    string              `gorm:"column:execution_id;not null;default:'';index" json:"executionId"`
	ParentTurnID   string              `gorm:"column:parent_turn_id;not null;default:'';index" json:"parentTurnId,omitempty"`
	ParentBlockID  string              `gorm:"column:parent_block_id;not null;default:''" json:"parentBlockId,omitempty"`
	AgentID        string              `gorm:"column:agent_id;not null;default:'';index" json:"agentId,omitempty"`
	Sequence       int64               `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Status         string              `gorm:"column:status;not null;default:running" json:"status"`
	CreatedAt      string              `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string              `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
	CompletedAt    string              `gorm:"column:completed_at;not null;default:''" json:"completedAt"`
	Items          []AssistantTurnItem `gorm:"-" json:"items"`
}

func (AssistantTurn) TableName() string { return "assistant_turns" }

type AssistantTurnItem struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	TurnID         string `gorm:"column:turn_id;not null;default:'';index" json:"turnId"`
	ConversationID string `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	ItemType       string `gorm:"column:item_type;not null;default:''" json:"type"`
	Status         string `gorm:"column:status;not null;default:pending" json:"status"`
	Revision       int64  `gorm:"column:revision;not null;default:0" json:"revision"`
	CallID         string `gorm:"column:call_id;not null;default:''" json:"callId,omitempty"`
	ToolName       string `gorm:"column:tool_name;not null;default:''" json:"toolName,omitempty"`
	Content        string `gorm:"column:content;not null;default:''" json:"content,omitempty"`
	ArgumentsJSON  string `gorm:"column:arguments_json;not null;default:''" json:"argumentsJson,omitempty"`
	ResultJSON     string `gorm:"column:result_json;not null;default:''" json:"resultJson,omitempty"`
	ErrorCode      string `gorm:"column:error_code;not null;default:''" json:"errorCode,omitempty"`
	DurationMS     int64  `gorm:"column:duration_ms;not null;default:0" json:"durationMs,omitempty"`
	IsFinal        int    `gorm:"column:is_final;not null;default:0" json:"isFinal,omitempty"`
	MessageID      string `gorm:"column:message_id;not null;default:''" json:"messageId,omitempty"`
	CreatedAt      string `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
}

func (AssistantTurnItem) TableName() string { return "assistant_turn_items" }

type assistantTurnRecorder struct {
	db               *gorm.DB
	TurnID           string
	ConversationID   string
	CharacterID      string
	UserMessageID    string
	RequestID        string
	ExecutionID      string
	TurnSequence     int64
	Provider         string
	enabled          bool
	progressMu       sync.Mutex
	citationMu       sync.RWMutex
	citationIDs      map[int]struct{}
	citationEvidence map[int]citationEvidence
	webResearchUsed  bool
}

func newAssistantTurnRecorder(db *gorm.DB, conversationID, characterID, userMessageID, requestID string, ids ...string) *assistantTurnRecorder {
	turnID := ""
	executionID := ""
	if len(ids) > 0 {
		turnID = ids[0]
	}
	if len(ids) > 1 {
		executionID = ids[1]
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		turnID = uuid.NewString()
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		executionID = uuid.NewString()
	}
	return &assistantTurnRecorder{
		db:               db,
		TurnID:           turnID,
		ConversationID:   strings.TrimSpace(conversationID),
		CharacterID:      strings.TrimSpace(characterID),
		UserMessageID:    strings.TrimSpace(userMessageID),
		RequestID:        strings.TrimSpace(requestID),
		ExecutionID:      executionID,
		citationIDs:      make(map[int]struct{}),
		citationEvidence: make(map[int]citationEvidence),
	}
}

func (r *assistantTurnRecorder) Start(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	if !r.db.Migrator().HasTable(&AssistantTurn{}) ||
		!r.db.Migrator().HasTable(&AssistantTurnItem{}) {
		return nil
	}
	r.enabled = true
	now := nowString()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing AssistantTurn
		err := tx.Where("id = ?", r.TurnID).First(&existing).Error
		if err == nil {
			r.TurnSequence = existing.Sequence
			if r.ExecutionID == "" {
				r.ExecutionID = existing.ExecutionID
			}
			updates := map[string]any{"status": assistantTurnStatusRunning, "updated_at": now}
			if r.ExecutionID != "" {
				updates["execution_id"] = r.ExecutionID
			}
			if err := tx.Model(&AssistantTurn{}).Where("id = ?", r.TurnID).Updates(updates).Error; err != nil {
				return err
			}
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		var sequence int64
		if err := tx.Model(&AssistantTurn{}).Where("conversation_id = ?", r.ConversationID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
			return err
		}
		r.TurnSequence = sequence
		turn := &AssistantTurn{
			ID: r.TurnID, ConversationID: r.ConversationID, CharacterID: r.CharacterID, UserMessageID: r.UserMessageID,
			RequestID: r.RequestID, ExecutionID: r.ExecutionID, Sequence: sequence, Status: assistantTurnStatusRunning,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(turn).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID,
		TurnSequence: r.TurnSequence, Type: "turn.started", Status: assistantTurnStatusRunning,
	}, true); err != nil {
		return err
	}
	return nil
}

func (r *assistantTurnRecorder) AddToolCall(ctx context.Context, callID, toolName, arguments string, status string) error {
	if r == nil {
		return nil
	}
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	if strings.TrimSpace(status) == "" {
		status = assistantTurnStatusRunning
	}
	if r.db != nil && r.enabled && callID != "" {
		var existing AssistantTurnItem
		err := r.db.WithContext(ctx).Where("turn_id = ? AND item_type = ? AND call_id = ?", r.TurnID, assistantTurnItemToolCall, callID).Order("sequence ASC").First(&existing).Error
		if err == nil {
			revision := existing.Revision + 1
			if revision < 2 {
				revision = 2
			}
			argumentsJSON := normalizeTurnJSON(arguments)
			updates := map[string]any{"status": status, "tool_name": toolName, "arguments_json": argumentsJSON, "revision": revision, "updated_at": nowString()}
			if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return err
			}
			_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
				ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
				BlockID: existing.ID, BlockSequence: existing.Sequence, CallID: callID, Revision: revision, Type: "tool.running", Status: status,
				Payload: map[string]any{"blockType": "tool_call", "toolName": toolName, "arguments": argumentsJSON, "recoveryCheckpoint": true},
			}, true)
			return err
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return r.addItem(ctx, AssistantTurnItem{ItemType: assistantTurnItemToolCall, Status: status, CallID: callID, ToolName: toolName, ArgumentsJSON: normalizeTurnJSON(arguments)})
}

func (r *assistantTurnRecorder) AddToolProgress(ctx context.Context, callID, toolName string, progress ToolProgressEvent) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	message := strings.TrimSpace(progress.Message)
	if callID == "" || message == "" {
		return nil
	}
	var item AssistantTurnItem
	if err := r.db.WithContext(ctx).Where("turn_id = ? AND item_type = ? AND call_id = ?", r.TurnID, assistantTurnItemToolCall, callID).Order("sequence ASC").First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	revision := item.Revision + 1
	if revision < 2 {
		revision = 2
	}
	if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ? AND revision = ?", item.ID, item.Revision).Updates(map[string]any{
		"content": message, "status": assistantTurnStatusRunning, "revision": revision, "updated_at": nowString(),
	}).Error; err != nil {
		return err
	}
	payload := map[string]any{
		"blockType":     "tool_call",
		"toolName":      toolName,
		"content":       message,
		"fraction":      progress.Fraction,
		"indeterminate": progress.Indeterminate,
	}
	if len(progress.Metadata) > 0 {
		payload["metadata"] = progress.Metadata
	}
	_, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
		BlockID: item.ID, BlockSequence: item.Sequence, CallID: callID, Revision: revision, Type: "tool.progress", Status: assistantTurnStatusRunning, Payload: payload,
	}, false)
	return err
}

func (r *assistantTurnRecorder) AddToolResult(ctx context.Context, callID, toolName, result, status, errorCode string, durationMS int64) error {
	if r == nil {
		return nil
	}
	r.captureCitationIDs(toolName, result)
	if r.db == nil || !r.enabled {
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	if strings.TrimSpace(status) == "" {
		status = assistantTurnStatusCompleted
	}
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	errorCode = strings.TrimSpace(errorCode)
	now := nowString()
	var toolCallItem AssistantTurnItem
	resultItem := AssistantTurnItem{
		ID:             uuid.NewString(),
		TurnID:         r.TurnID,
		ConversationID: r.ConversationID,
		ItemType:       assistantTurnItemToolResult,
		Status:         status,
		CallID:         callID,
		ToolName:       toolName,
		ResultJSON:     normalizeTurnJSON(result),
		ErrorCode:      errorCode,
		DurationMS:     durationMS,
		Revision:       2,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	var toolRevision int64
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("turn_id = ? AND item_type = ? AND call_id = ?", r.TurnID, assistantTurnItemToolCall, callID).Order("sequence ASC").First(&toolCallItem).Error; err != nil {
			return err
		}
		toolRevision = toolCallItem.Revision + 1
		if toolRevision < 2 {
			toolRevision = 2
		}
		if err := tx.Model(&AssistantTurnItem{}).Where("id = ?", toolCallItem.ID).Updates(map[string]any{
			"status": status, "duration_ms": durationMS, "revision": toolRevision, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return appendAssistantTurnItemTx(tx, &resultItem)
	}); err != nil {
		return err
	}
	eventType := "tool.completed"
	if status == assistantTurnStatusFailed {
		eventType = "tool.failed"
	} else if status == assistantTurnStatusInterrupted {
		eventType = "tool.interrupted"
	}
	payload := map[string]any{"blockType": "tool_call", "toolName": toolName, "durationMs": durationMS, "recoveryCheckpoint": true}
	if errorCode != "" {
		payload["errorCode"] = errorCode
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
		BlockID: toolCallItem.ID, BlockSequence: toolCallItem.Sequence, CallID: callID, Revision: toolRevision, Type: eventType, Status: status, Payload: payload,
	}, true); err != nil {
		return err
	}
	return r.publishItemEvents(ctx, resultItem)
}

type citationAudit struct {
	Available       []int
	Used            []int
	Unknown         []int
	Claims          []claimCitationAudit
	MissingCitation []string
}

type citationEvidence struct {
	EvidenceID string
	RefID      string
	Title      string
	URL        string
	Text       string
}

type claimCitationAudit struct {
	Text         string
	Citations    []int
	Status       string
	SupportScore float64
}

var numericCitationPattern = regexp.MustCompile(`\[(\d{1,6})\]`)

func (r *assistantTurnRecorder) captureCitationIDs(toolName, result string) {
	if r == nil || (toolName != "web_run" && toolName != "web.run") {
		return
	}
	r.citationMu.Lock()
	r.webResearchUsed = true
	r.citationMu.Unlock()
	if strings.TrimSpace(result) == "" {
		return
	}
	var payload struct {
		Citations []struct {
			Index      int    `json:"index"`
			EvidenceID string `json:"evidence_id"`
			RefID      string `json:"ref_id"`
			Title      string `json:"title"`
			URL        string `json:"url"`
			Text       string `json:"text"`
		} `json:"citations"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return
	}
	r.citationMu.Lock()
	defer r.citationMu.Unlock()
	if r.citationIDs == nil {
		r.citationIDs = make(map[int]struct{})
	}
	if r.citationEvidence == nil {
		r.citationEvidence = make(map[int]citationEvidence)
	}
	for _, citation := range payload.Citations {
		if citation.Index > 0 {
			r.citationIDs[citation.Index] = struct{}{}
			r.citationEvidence[citation.Index] = citationEvidence{
				EvidenceID: strings.TrimSpace(citation.EvidenceID),
				RefID:      strings.TrimSpace(citation.RefID),
				Title:      strings.TrimSpace(citation.Title),
				URL:        strings.TrimSpace(citation.URL),
				Text:       strings.TrimSpace(citation.Text),
			}
		}
	}
}

func (r *assistantTurnRecorder) AuditCitationMarkers(markdown string) citationAudit {
	if r == nil {
		return citationAudit{}
	}
	r.citationMu.RLock()
	webResearchUsed := r.webResearchUsed
	availableSet := make(map[int]struct{}, len(r.citationIDs))
	for id := range r.citationIDs {
		availableSet[id] = struct{}{}
	}
	evidenceByCitation := make(map[int]citationEvidence, len(r.citationEvidence))
	for id, evidence := range r.citationEvidence {
		evidenceByCitation[id] = evidence
	}
	r.citationMu.RUnlock()
	usedSet := make(map[int]struct{})
	unknownSet := make(map[int]struct{})
	for _, match := range citationMarkersOutsideCode(markdown) {
		id, err := strconv.Atoi(match)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := availableSet[id]; ok {
			usedSet[id] = struct{}{}
		} else {
			unknownSet[id] = struct{}{}
		}
	}
	missing := []string(nil)
	if webResearchUsed {
		missing = auditMissingCitationClaims(markdown)
	}
	return citationAudit{
		Available:       sortedCitationIDs(availableSet),
		Used:            sortedCitationIDs(usedSet),
		Unknown:         sortedCitationIDs(unknownSet),
		Claims:          auditCitationClaims(markdown, availableSet, evidenceByCitation),
		MissingCitation: missing,
	}
}

func auditCitationClaims(markdown string, available map[int]struct{}, evidence map[int]citationEvidence) []claimCitationAudit {
	segments := citationClaimSegments(markdown)
	claims := make([]claimCitationAudit, 0, len(segments))
	for _, segment := range segments {
		matches := numericCitationPattern.FindAllStringSubmatch(segment, -1)
		if len(matches) == 0 {
			continue
		}
		ids := make([]int, 0, len(matches))
		seen := map[int]struct{}{}
		knownEvidence := make([]string, 0, len(matches))
		hasUnknown := false
		for _, match := range matches {
			id, err := strconv.Atoi(match[1])
			if err != nil || id <= 0 {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			if _, ok := available[id]; !ok {
				hasUnknown = true
				continue
			}
			if item := evidence[id]; strings.TrimSpace(item.Text) != "" {
				knownEvidence = append(knownEvidence, item.Text)
			}
		}
		if len(ids) == 0 {
			continue
		}
		sort.Ints(ids)
		claimText := strings.TrimSpace(numericCitationPattern.ReplaceAllString(segment, ""))
		status := "supported"
		score := 0.0
		switch {
		case hasUnknown:
			status = "invalid_citation"
		case len(knownEvidence) == 0:
			status = "insufficient"
		default:
			score = citationLexicalSupport(claimText, strings.Join(knownEvidence, "\n"))
			if score == 0 {
				status = "unsupported"
			} else if score < 0.15 {
				status = "partially_supported"
			}
		}
		claims = append(claims, claimCitationAudit{Text: claimText, Citations: ids, Status: status, SupportScore: score})
	}
	return claims
}

var citationClaimSentencePattern = regexp.MustCompile(`(?:(?:\d+\.\d+)|[^.!?。！？\n])+(?:[.!?。！？]+|$)`)

var factualCitationSignalPattern = regexp.MustCompile(`(?i)(?:\b(?:19|20)\d{2}\b|\b\d+(?:\.\d+)+\b|\b(?:released|launch(?:ed)?|supports?|supported|available|version|published|updated|announced|according to|percent|million|billion)\b|发布|上线|支持|版本|更新|宣布|截至|目前|根据|\d+(?:\.\d+)?%|\d+年|\d+月|\d+日)`)

func auditMissingCitationClaims(markdown string) []string {
	sentences := citationSentencesOutsideCode(markdown)
	out := make([]string, 0)
	for _, sentence := range sentences {
		if numericCitationPattern.MatchString(sentence) {
			continue
		}
		plain := strings.TrimSpace(sentence)
		if plain == "" || !factualCitationSignalPattern.MatchString(plain) {
			continue
		}
		lower := strings.ToLower(plain)
		if strings.Contains(lower, "i think") || strings.Contains(lower, "in my view") || strings.Contains(plain, "我认为") || strings.Contains(plain, "我觉得") {
			continue
		}
		out = append(out, truncateCitationAuditText(plain, 320))
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func citationSentencesOutsideCode(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	clean := make([]string, 0, len(lines))
	inFence := false
	fence := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			current := trimmed[:3]
			if !inFence {
				inFence = true
				fence = current
			} else if current == fence {
				inFence = false
				fence = ""
			}
			continue
		}
		if inFence {
			continue
		}
		clean = append(clean, inlineCodePattern.ReplaceAllString(line, ""))
	}
	return citationClaimSentencePattern.FindAllString(strings.Join(clean, "\n"), -1)
}

func truncateCitationAuditText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "…"
}

var inlineCodePattern = regexp.MustCompile("`[^`]*`")
var citationTokenPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

func citationClaimSegments(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	clean := make([]string, 0, len(lines))
	inFence := false
	fence := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			current := trimmed[:3]
			if !inFence {
				inFence = true
				fence = current
			} else if current == fence {
				inFence = false
				fence = ""
			}
			continue
		}
		if inFence {
			continue
		}
		clean = append(clean, inlineCodePattern.ReplaceAllString(line, ""))
	}
	text := strings.Join(clean, "\n")
	matches := citationClaimSentencePattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if numericCitationPattern.MatchString(match) {
			out = append(out, strings.TrimSpace(match))
		}
	}
	return out
}

func citationLexicalSupport(claim, evidence string) float64 {
	claimTokens := citationTokens(claim)
	if len(claimTokens) == 0 {
		return 1
	}
	evidenceTokens := citationTokens(evidence)
	matched := 0
	for token := range claimTokens {
		if _, ok := evidenceTokens[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimTokens))
}

func citationTokens(value string) map[string]struct{} {
	value = strings.ToLower(value)
	matches := citationTokenPattern.FindAllString(value, -1)
	out := make(map[string]struct{}, len(matches))
	for _, token := range matches {
		token = strings.TrimSpace(token)
		if token == "" || isCitationStopToken(token) {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func isCitationStopToken(token string) bool {
	switch token {
	case "the", "a", "an", "and", "or", "of", "to", "in", "on", "for", "is", "are", "was", "were", "be", "with", "as", "by", "from", "that", "this", "it", "its", "at":
		return true
	default:
		return false
	}
}

func citationMarkersOutsideCode(markdown string) []string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	out := make([]string, 0)
	inFence := false
	fence := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			current := trimmed[:3]
			if !inFence {
				inFence = true
				fence = current
			} else if current == fence {
				inFence = false
				fence = ""
			}
			continue
		}
		if inFence {
			continue
		}
		out = append(out, citationMarkersOutsideInlineCode(line)...)
	}
	return out
}

func citationMarkersOutsideInlineCode(line string) []string {
	out := make([]string, 0)
	start := 0
	inCode := false
	for i := 0; i <= len(line); i++ {
		if i < len(line) && line[i] != '`' {
			continue
		}
		if !inCode && i > start {
			segment := line[start:i]
			for _, match := range numericCitationPattern.FindAllStringSubmatch(segment, -1) {
				if len(match) > 1 {
					out = append(out, match[1])
				}
			}
		}
		if i == len(line) {
			break
		}
		inCode = !inCode
		start = i + 1
	}
	return out
}

func sortedCitationIDs(values map[int]struct{}) []int {
	out := make([]int, 0, len(values))
	for id := range values {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func finalizeAssistantTurnFailureByID(db *gorm.DB, turnID string, cause error) error {
	if db == nil {
		return nil
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	var turn AssistantTurn
	if err := db.Where("id = ?", turnID).First(&turn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	recorder := &assistantTurnRecorder{
		db:             db,
		TurnID:         turn.ID,
		ConversationID: turn.ConversationID,
		CharacterID:    turn.CharacterID,
		UserMessageID:  turn.UserMessageID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnSequence:   turn.Sequence,
		enabled:        true,
	}
	return recorder.FinalizeFailure(context.Background(), assistantTurnStatusFailed, cause)
}

func (r *assistantTurnRecorder) Finalize(ctx context.Context, status string) error {
	return r.finalize(ctx, status, nil)
}

func (r *assistantTurnRecorder) FinalizeFailure(ctx context.Context, status string, cause error) error {
	payload := map[string]any{
		"errorCode":          "runtime_error",
		"errorType":          "runtime",
		"retryable":          true,
		"userMessage":        "Agent 执行失败，可重试",
		"internalMessage":    "",
		"provider":           strings.TrimSpace(r.Provider),
		"recoveryCheckpoint": true,
	}
	if cause != nil {
		payload["internalMessage"] = cause.Error()
		var modelErr *TextModelCallError
		if errors.As(cause, &modelErr) {
			payload["errorCode"] = "provider_error"
			payload["errorType"] = "provider"
			payload["internalMessage"] = modelErr.RawError
			raw := strings.ToLower(modelErr.RawError)
			payload["retryable"] = strings.Contains(raw, "429") || strings.Contains(raw, "500") || strings.Contains(raw, "502") || strings.Contains(raw, "503") || strings.Contains(raw, "504") || strings.Contains(raw, "timeout") || strings.Contains(raw, "busy") || strings.Contains(raw, "unavailable")
		}
	}
	if status == assistantTurnStatusInterrupted {
		payload["errorCode"] = "interrupted"
		payload["errorType"] = "interrupt"
		payload["retryable"] = false
		payload["userMessage"] = "已停止生成"
	}
	return r.finalize(ctx, status, payload)
}

func (r *assistantTurnRecorder) finalize(ctx context.Context, status string, payload map[string]any) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = assistantTurnStatusCompleted
	}
	now := nowString()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AssistantTurn{}).Where("id = ?", r.TurnID).Updates(map[string]any{
			"status":       status,
			"updated_at":   now,
			"completed_at": now,
		}).Error; err != nil {
			return err
		}
		if status == assistantTurnStatusFailed || status == assistantTurnStatusInterrupted {
			return tx.Model(&AssistantTurnItem{}).Where("turn_id = ? AND status NOT IN ?", r.TurnID, []string{assistantTurnStatusCompleted, assistantTurnStatusFailed, assistantTurnStatusInterrupted}).Updates(map[string]any{
				"status": status, "revision": gorm.Expr("revision + 1"), "updated_at": now,
			}).Error
		}
		return nil
	}); err != nil {
		return err
	}
	if status != assistantTurnStatusFailed && status != assistantTurnStatusInterrupted {
		return nil
	}
	if status == assistantTurnStatusFailed && payload != nil {
		if err := r.persistErrorItem(context.WithoutCancel(ctx), payload); err != nil {
			return err
		}
	}
	eventType := "turn.failed"
	if status == assistantTurnStatusInterrupted {
		eventType = "turn.interrupted"
	}
	if payload == nil {
		payload = map[string]any{"recoveryCheckpoint": true}
	}
	_, err := conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence, Type: eventType, Status: status, Payload: payload}, true)
	return err
}

func (r *assistantTurnRecorder) persistErrorItem(ctx context.Context, payload map[string]any) error {
	if r == nil || r.db == nil || !r.enabled || payload == nil {
		return nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	userMessage := strings.TrimSpace(fmt.Sprint(payload["userMessage"]))
	errorCode := strings.TrimSpace(fmt.Sprint(payload["errorCode"]))
	now := nowString()
	var existing AssistantTurnItem
	err = r.db.WithContext(ctx).
		Where("turn_id = ? AND item_type = ?", r.TurnID, assistantTurnItemError).
		Order("sequence ASC").
		First(&existing).Error
	if err == nil {
		revision := existing.Revision + 1
		if revision < 2 {
			revision = 2
		}
		if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"status":      assistantTurnStatusFailed,
			"content":     userMessage,
			"result_json": string(encoded),
			"error_code":  errorCode,
			"revision":    revision,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		existing.Status = assistantTurnStatusFailed
		existing.Content = userMessage
		existing.ResultJSON = string(encoded)
		existing.ErrorCode = errorCode
		existing.Revision = revision
		existing.UpdatedAt = now
		return r.publishItemEvents(ctx, existing)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType:   assistantTurnItemError,
		Status:     assistantTurnStatusFailed,
		ErrorCode:  errorCode,
		Content:    userMessage,
		ResultJSON: string(encoded),
		Revision:   2,
	})
}

func (r *assistantTurnRecorder) addItem(ctx context.Context, item AssistantTurnItem) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	now := nowString()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.ID = uuid.NewString()
		if item.Revision <= 0 {
			item.Revision = 1
		}
		if item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending" && item.Revision < 2 {
			item.Revision = 2
		}
		item.TurnID = r.TurnID
		item.ConversationID = r.ConversationID
		item.CreatedAt = now
		item.UpdatedAt = now
		return appendAssistantTurnItemTx(tx, &item)
	})
	if err != nil {
		return err
	}
	return r.publishItemEvents(ctx, item)
}

func (r *assistantTurnRecorder) publishItemEvents(ctx context.Context, item AssistantTurnItem) error {
	blockType := item.ItemType
	startedType := "block.started"
	completedType := "block.completed"
	if blockType == "text" || blockType == "reasoning" {
		startedType = blockType + ".started"
		completedType = blockType + ".completed"
	}
	if blockType == assistantTurnItemToolCall {
		startedType = "tool.started"
		completedType = "tool.running"
	}
	if item.Status == assistantTurnStatusFailed {
		switch blockType {
		case "text", "reasoning":
			completedType = blockType + ".failed"
		case assistantTurnItemToolCall:
			completedType = "tool.failed"
		default:
			completedType = "block.failed"
		}
	} else if item.Status == assistantTurnStatusInterrupted {
		switch blockType {
		case "text", "reasoning":
			completedType = blockType + ".interrupted"
		case assistantTurnItemToolCall:
			completedType = "tool.interrupted"
		default:
			completedType = "block.interrupted"
		}
	}
	payload := map[string]any{"blockType": blockType, "content": item.Content, "toolName": item.ToolName, "arguments": item.ArgumentsJSON, "result": item.ResultJSON, "errorCode": item.ErrorCode}
	if item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending" {
		payload["recoveryCheckpoint"] = true
	}
	terminal := item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending"
	startedRevision := item.Revision
	if terminal && startedRevision > 1 {
		startedRevision--
	}
	startedStatus := item.Status
	if terminal {
		startedStatus = assistantTurnStatusRunning
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
		BlockID: item.ID, BlockSequence: item.Sequence, MessageID: item.MessageID, CallID: item.CallID, Revision: startedRevision, Type: startedType, Status: startedStatus, Payload: payload,
	}, true); err != nil {
		return err
	}
	if terminal {
		_, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
			ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
			BlockID: item.ID, BlockSequence: item.Sequence, MessageID: item.MessageID, CallID: item.CallID, Revision: item.Revision, Type: completedType, Status: item.Status, Payload: payload,
		}, true)
		return err
	}
	return nil
}

func PersistAssistantTurnError(ctx context.Context, db *gorm.DB, turn AssistantTurn, errorCode, errorType, userMessage, internalMessage, provider string, retryable bool) error {
	if db == nil || strings.TrimSpace(turn.ID) == "" {
		return nil
	}
	var encodedInternalMessage any = internalMessage
	trimmedInternalMessage := strings.TrimSpace(internalMessage)
	if trimmedInternalMessage != "" && json.Valid([]byte(trimmedInternalMessage)) {
		encodedInternalMessage = json.RawMessage(trimmedInternalMessage)
	}
	payload := map[string]any{
		"errorCode": strings.TrimSpace(errorCode), "errorType": strings.TrimSpace(errorType), "retryable": retryable,
		"userMessage": strings.TrimSpace(userMessage), "internalMessage": encodedInternalMessage,
		"provider": strings.TrimSpace(provider), "recoveryCheckpoint": true,
	}
	recorder := &assistantTurnRecorder{
		db: db, TurnID: turn.ID, ConversationID: turn.ConversationID, CharacterID: turn.CharacterID,
		UserMessageID: turn.UserMessageID, RequestID: turn.RequestID, ExecutionID: turn.ExecutionID,
		TurnSequence: turn.Sequence, enabled: true,
	}
	return recorder.persistErrorItem(ctx, payload)
}

func appendAssistantTurnItemTx(tx *gorm.DB, item *AssistantTurnItem) error {
	var sequence int64
	if err := tx.Model(&AssistantTurnItem{}).Where("turn_id = ?", item.TurnID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
		return err
	}
	item.Sequence = sequence
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	return nil
}

func completeAssistantTurnTx(tx *gorm.DB, turnID, reply, messageID string) error {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	if !tx.Migrator().HasTable(&AssistantTurn{}) ||
		!tx.Migrator().HasTable(&AssistantTurnItem{}) {
		return nil
	}
	now := nowString()
	var turn AssistantTurn
	if err := tx.Where("id = ?", turnID).First(&turn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	messageID = strings.TrimSpace(messageID)
	var existing AssistantTurnItem
	err := tx.Where("turn_id = ? AND item_type = ?", turnID, assistantTurnItemText).Order("sequence DESC").First(&existing).Error
	if err == nil {
		if err := tx.Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"content":    strings.TrimSpace(reply),
			"revision":   existing.Revision + 1,
			"status":     assistantTurnStatusCompleted,
			"is_final":   1,
			"message_id": messageID,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	} else if err != gorm.ErrRecordNotFound {
		return err
	} else if strings.TrimSpace(reply) != "" {
		item := AssistantTurnItem{
			ID: uuid.NewString(), TurnID: turnID, ConversationID: turn.ConversationID,
			ItemType: assistantTurnItemText, Status: assistantTurnStatusCompleted, Revision: 2,
			Content: strings.TrimSpace(reply), IsFinal: 1, MessageID: messageID, CreatedAt: now, UpdatedAt: now,
		}
		if err := appendAssistantTurnItemTx(tx, &item); err != nil {
			return err
		}
	}
	if err := tx.Model(&AssistantTurn{}).Where("id = ?", turnID).Updates(map[string]any{
		"status": assistantTurnStatusCompleted, "updated_at": now, "completed_at": now,
	}).Error; err != nil {
		return err
	}
	return nil
}

func normalizeTurnJSON(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if json.Valid([]byte(value)) {
		return value
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%q", value)
	}
	return string(encoded)
}

func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
