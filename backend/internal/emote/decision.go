package emote

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const MinimumSimilarity = 0.35

type DecisionService struct {
	service *Service
	random  func() float64
	search  func(query, characterID string, limit int) ([]DecisionCandidate, error)
}

func NewDecisionService(service *Service) *DecisionService {
	return &DecisionService{service: service, random: rand.Float64, search: service.semantic.Search}
}

func (d *DecisionService) Plan(event *chat.MessagePlanningEvent) *chat.MessagePlanningDecision {
	if event == nil || event.ConversationID == "" || event.CharacterID == "" {
		return nil
	}
	responseID := strings.TrimSpace(event.RequestID)
	if responseID == "" || d.decisionExists(responseID) {
		return nil
	}
	settings, err := d.service.repo.GetSettings(event.CharacterID)
	if err != nil {
		return nil
	}
	var conversation chat.Conversation
	if err = d.service.repo.DB().Where("id = ?", event.ConversationID).First(&conversation).Error; err != nil {
		return nil
	}
	if conversation.CharacterID != event.CharacterID {
		return nil
	}
	channel := conversation.Channel
	if channel == "" {
		channel = event.Channel
	}
	if channel != "web" && channel != "wechat" && channel != "qq" {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "platform_unsupported")
	}
	if channel != "web" && strings.TrimSpace(conversation.PeerID) == "" && strings.TrimSpace(event.PeerID) == "" {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "platform_unavailable")
	}
	reply := strings.TrimSpace(event.Reply)
	emoteOnly := len(event.Lines) == 0 && reply == "" && settings.AllowEmoteOnly == 1
	if settings.Enabled != 1 || (reply == "" && !emoteOnly) || event.ForceVoice {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "disabled_or_empty")
	}
	if suppressedContext(event.Source, reply) {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "context_suppressed")
	}
	if !d.hasEligible(event.CharacterID) {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "no_eligible_emote")
	}
	if d.hourlyLimitReached(event.CharacterID, settings.MaxPerHour) {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "hourly_limit")
	}
	if d.replyGapBlocked(event.ConversationID, event.CharacterID, settings.MinReplyGap) {
		return d.noEmoteDecision(responseID, event, channel, settings, 0, "reply_gap")
	}
	probability := FinalProbability(settings.BaseProbability, settings.MaxProbability, ContextFactor(event.UserMessage, reply, event.Source))
	sample := d.random()
	if sample >= probability {
		log.Info("emote decision", "responseId", responseID, "probability", probability, "sample", sample, "hit", false)
		return d.noEmoteDecision(responseID, event, channel, settings, sample, "random_miss")
	}
	candidates, searchErr := d.search(strings.TrimSpace(event.UserMessage+"\n"+reply), event.CharacterID, 5)
	filtered := d.filterCandidates(candidates, event.CharacterID, channel, settings.SameEmoteCooldownMinutes)
	if len(filtered) == 0 {
		reason := "no_candidate"
		if searchErr != nil {
			reason = "semantic_fallback_empty"
		}
		return d.noEmoteDecision(responseID, event, channel, settings, sample, reason)
	}
	selected, ok := WeightedSelect(filtered, d.random)
	if !ok || selected.Score < MinimumSimilarity {
		return d.noEmoteDecision(responseID, event, channel, settings, sample, "below_similarity")
	}
	insertAfter, sendMode := 0, SendModeEmoteOnly
	if !emoteOnly {
		insertAfter, sendMode = d.selectPlacement(event.Lines, event.Source, reply)
	}
	item := selected.Emote
	alt := "[表情：" + item.Name + "]"
	planned := &chat.PlannedEmote{EmoteID: item.ID, Content: alt, AltText: alt, IsAnimated: item.IsAnimated, Width: item.Width, Height: item.Height, Original: item.FilePath, Fallback: item.FallbackPath, MimeType: item.MimeType}
	peerID := conversation.PeerID
	if peerID == "" {
		peerID = event.PeerID
	}
	log.Info("emote selected", "responseId", responseID, "emoteId", item.ID, "probability", probability, "sample", sample, "candidates", len(filtered), "sendMode", sendMode, "insertAfter", insertAfter)
	return &chat.MessagePlanningDecision{
		Emote:       planned,
		InsertAfter: insertAfter,
		SendMode:    sendMode,
		Persist: func(tx *gorm.DB, message *chat.Message) error {
			if message == nil {
				return errors.New("planned emote message missing")
			}
			now := time.Now().Format("2006-01-02 15:04:05")
			deliveryKey := delivery.GenerateDeliveryID(responseID, channel, peerID, message.ID)
			status := "queued"
			if channel == "web" {
				status = "sent"
			}
			emoteID := item.ID
			record := SendRecord{ID: uuid.New().String(), EmoteID: &emoteID, CharacterID: event.CharacterID, ConversationID: event.ConversationID, MessageID: message.ID, ResponseID: responseID, Platform: channel, TriggerType: TriggerAIRandom, TriggerProbability: probability, RandomSample: sample, TriggerHit: 1, DecisionReason: "selected", SendMode: sendMode, DeliveryKey: deliveryKey, Status: status, CreatedAt: now}
			return tx.Create(&record).Error
		},
	}
}

func (d *DecisionService) selectPlacement(lines []string, source, reply string) (int, string) {
	if len(lines) < 2 || !safeForBetween(lines, source, reply) {
		return len(lines), SendModeAfterAllText
	}
	betweenWeight := 0.25
	if source == "proactive" {
		betweenWeight = 0.30
	}
	if d.random() >= betweenWeight {
		return len(lines), SendModeAfterAllText
	}
	gap := int(d.random()*float64(len(lines)-1)) + 1
	if gap >= len(lines) {
		gap = len(lines) - 1
	}
	return gap, SendModeBetweenTextMessages
}

func safeForBetween(lines []string, source, reply string) bool {
	if suppressedContext(source, reply) || len([]rune(reply)) > 240 {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || len([]rune(trimmed)) > 120 {
			return false
		}
		if structuredLine(trimmed) {
			return false
		}
	}
	return true
}

func structuredLine(line string) bool {
	if strings.Contains(line, "```") || strings.Contains(line, "~~~") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "]") {
		return true
	}
	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	return index > 0 && index < len(line) && (line[index] == '.' || line[index] == ')' || line[index] == ':')
}

func suppressedContext(source, reply string) bool {
	if source == "system" || source == "tool" || source == "error" {
		return true
	}
	lower := strings.ToLower(reply)
	for _, marker := range []string{"报错", "错误", "失败", "安全提示", "风险", "正式通知", "系统通知", "紧急", "报警", "急救", "自杀", "自残", "伤害自己", "暴力", "违法", "医疗建议", "法律建议", "财务建议", "验证码", "密码", "error:", "warning:", "traceback", "stack trace"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (d *DecisionService) noEmoteDecision(responseID string, event *chat.MessagePlanningEvent, channel string, settings CharacterSettings, sample float64, reason string) *chat.MessagePlanningDecision {
	probability := FinalProbability(settings.BaseProbability, settings.MaxProbability, ContextFactor(event.UserMessage, event.Reply, event.Source))
	return &chat.MessagePlanningDecision{SendMode: SendModeAfterAllText, Persist: func(tx *gorm.DB, _ *chat.Message) error {
		now := time.Now().Format("2006-01-02 15:04:05")
		record := SendRecord{ID: uuid.New().String(), CharacterID: event.CharacterID, ConversationID: event.ConversationID, ResponseID: responseID, Platform: channel, TriggerType: TriggerAIRandom, TriggerProbability: probability, RandomSample: sample, TriggerHit: 0, DecisionReason: reason, SendMode: SendModeAfterAllText, DeliveryKey: "decision:" + responseID, Status: "skipped", CreatedAt: now}
		return tx.Create(&record).Error
	}}
}

func FinalProbability(base, maxValue, factor float64) float64 {
	value := base * factor
	if value < 0 || maxValue < 0 {
		return 0
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func ContextFactor(userMessage, reply, source string) float64 {
	if source == "system" || source == "tool" || source == "error" {
		return 0
	}
	length := len([]rune(reply))
	if length > 500 {
		return 0.2
	}
	if length > 240 {
		return 0.3
	}
	if strings.Contains(userMessage, "[表情") || strings.Contains(userMessage, "表情包") {
		return 1.3
	}
	if length <= 60 {
		return 1.2
	}
	return 1
}

func WeightedSelect(candidates []DecisionCandidate, random func() float64) (DecisionCandidate, bool) {
	eligible := []DecisionCandidate{}
	for _, candidate := range candidates {
		if candidate.Score >= MinimumSimilarity {
			eligible = append(eligible, candidate)
		}
	}
	if len(eligible) > 5 {
		eligible = eligible[:5]
	}
	if len(eligible) == 0 {
		return DecisionCandidate{}, false
	}
	total := 0.0
	for _, candidate := range eligible {
		total += candidate.Score
	}
	if total <= 0 {
		return DecisionCandidate{}, false
	}
	sample := random() * total
	running := 0.0
	for _, candidate := range eligible {
		running += candidate.Score
		if sample < running {
			return candidate, true
		}
	}
	return eligible[len(eligible)-1], true
}

func (d *DecisionService) decisionExists(responseID string) bool {
	var count int64
	d.service.repo.DB().Model(&SendRecord{}).Where("response_id = ?", responseID).Count(&count)
	return count > 0
}

func (d *DecisionService) hasEligible(characterID string) bool {
	var count int64
	d.service.repo.DB().Table("emotes e").Where("e.deleted_at IS NULL AND e.enabled = 1 AND e.ai_enabled = 1 AND e.meaning <> '' AND (e.role_scope = ? OR EXISTS (SELECT 1 FROM emote_character_bindings b WHERE b.emote_id = e.id AND b.character_id = ?))", RoleScopeAll, characterID).Count(&count)
	return count > 0
}

func (d *DecisionService) hourlyLimitReached(characterID string, limit int) bool {
	if limit <= 0 {
		return true
	}
	since := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	var count int64
	d.service.repo.DB().Model(&SendRecord{}).Where("character_id = ? AND trigger_type = ? AND trigger_hit = 1 AND status IN ? AND created_at >= ?", characterID, TriggerAIRandom, []string{"queued", "sent", "delivered"}, since).Count(&count)
	return count >= int64(limit)
}

func (d *DecisionService) replyGapBlocked(conversationID, characterID string, gap int) bool {
	if gap <= 0 {
		return false
	}
	var last SendRecord
	err := d.service.repo.DB().Where("conversation_id = ? AND character_id = ? AND trigger_hit = 1 AND status IN ?", conversationID, characterID, []string{"queued", "sent", "delivered"}).Order("created_at DESC").First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if err != nil {
		return true
	}
	var count int64
	d.service.repo.DB().Table("messages").Where("conversation_id = ? AND role = 'assistant' AND msg_type = 'text' AND created_at > ?", conversationID, last.CreatedAt).Count(&count)
	return count < int64(gap)
}

func (d *DecisionService) filterCandidates(candidates []DecisionCandidate, characterID, channel string, cooldown int) []DecisionCandidate {
	out := []DecisionCandidate{}
	cutoff := time.Now().Add(-time.Duration(cooldown) * time.Minute).Format("2006-01-02 15:04:05")
	for _, candidate := range candidates {
		item := candidate.Emote
		if !d.service.repo.CanCharacterUse(&item, characterID) || candidate.Score < MinimumSimilarity || !assetAvailable(&item, channel) {
			continue
		}
		if cooldown > 0 {
			var count int64
			d.service.repo.DB().Model(&SendRecord{}).Where("emote_id = ? AND character_id = ? AND trigger_hit = 1 AND status IN ? AND created_at >= ?", item.ID, characterID, []string{"queued", "sent", "delivered"}, cutoff).Count(&count)
			if count > 0 {
				continue
			}
		}
		out = append(out, candidate)
	}
	return out
}

func assetAvailable(item *Emote, channel string) bool {
	relative := item.FilePath
	if channel != "web" && item.IsAnimated == 1 {
		relative = item.FallbackPath
	}
	relative = strings.TrimPrefix(relative, "/emote-assets/")
	if relative == "" || strings.Contains(relative, "..") {
		return false
	}
	path := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", filepath.FromSlash(relative))
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
