package companion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

var errProactiveUnifiedEntryMissing = errors.New("proactive unified entry is not configured")

type proactiveDeliveryScope struct {
	channel string
	peerID  string
	userID  string
}

func (s *service) submitProactiveMessage(ctx context.Context, characterID, conversationID, channelSetting, prompt, requestID string) (*interaction.OrchestrationResult, error) {
	if s.unifiedEntry == nil {
		return nil, errProactiveUnifiedEntryMissing
	}
	scope := s.resolveProactiveDeliveryScope(conversationID, channelSetting)
	req := &interaction.UnifiedEntryRequest{
		Channel:        scope.channel,
		Message:        prompt,
		PeerID:         scope.peerID,
		UserID:         scope.userID,
		Source:         scope.channel,
		CharacterID:    characterID,
		ConversationID: conversationID,
		RequestID:      requestID,
		SessionID:      "proactive:" + characterID,
	}
	return s.unifiedEntry.Handle(ctx, req)
}

func (s *service) resolveProactiveDeliveryScope(conversationID, channelSetting string) proactiveDeliveryScope {
	scope := proactiveDeliveryScope{channel: normalizeProactiveChannel(channelSetting)}
	var channel, peerID string
	s.db.Table("conversations").Select("channel, peer_id").Where("id = ?", conversationID).Limit(1).Row().Scan(&channel, &peerID)
	if strings.TrimSpace(channel) != "" {
		scope.channel = normalizeProactiveChannel(channel)
	}
	scope.peerID = strings.TrimSpace(peerID)
	if scope.channel == "" {
		scope.channel = "web"
	}
	return scope
}

func normalizeProactiveChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" || channel == "all" {
		return "web"
	}
	if strings.Contains(channel, "wechat") {
		return "wechat"
	}
	if strings.Contains(channel, "qq") {
		return "qq"
	}
	if strings.Contains(channel, "voice") {
		return "voice"
	}
	return channel
}

func proactiveRequestID(prefix string, id interface{}, now time.Time) string {
	return fmt.Sprintf("%s-%v-%d", prefix, id, now.UnixNano())
}
