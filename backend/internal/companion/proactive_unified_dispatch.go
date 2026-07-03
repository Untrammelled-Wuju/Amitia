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
	scope := s.resolveProactiveDeliveryScope(conversationID, channelSetting, characterID)
	req := &interaction.UnifiedEntryRequest{
		Channel:        scope.channel,
		Message:        prompt,
		PeerID:         scope.peerID,
		UserID:         scope.userID,
		Source:         "proactive",
		CharacterID:    characterID,
		ConversationID: conversationID,
		RequestID:      requestID,
		SessionID:      "proactive:" + characterID,
		IsInternal:     true,
	}
	return s.unifiedEntry.Handle(ctx, req)
}

func (s *service) resolveProactiveDeliveryScope(conversationID, channelSetting, characterID string) proactiveDeliveryScope {
	scope := proactiveDeliveryScope{channel: normalizeProactiveChannel(channelSetting)}
	var channel, peerID, userID string
	s.db.Table("conversations").Select("channel, peer_id, user_id").Where("id = ?", conversationID).Limit(1).Row().Scan(&channel, &peerID, &userID)
	if strings.TrimSpace(channel) != "" {
		scope.channel = normalizeProactiveChannel(channel)
	}
	scope.peerID = strings.TrimSpace(peerID)
	if userIDFromDB := strings.TrimSpace(userID); userIDFromDB != "" {
		scope.userID = userIDFromDB
	} else {
		if trimmedPeerID := strings.TrimSpace(peerID); trimmedPeerID != "" {
			scope.userID = trimmedPeerID
		} else {
			scope.userID = "character:" + strings.TrimSpace(characterID)
		}
	}
	if scope.channel == "" {
		scope.channel = "web"
	}
	return scope
}

func normalizeProactiveChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		return "web"
	}
	if channel == "all" {
		return "all"
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
	return fmt.Sprintf("%s-%v-%d", prefix, id, now.Unix())
}

func (s *service) DispatchProactiveMessage(ctx context.Context, characterID, conversationID, channel, prompt, requestID string) (string, error) {
	result, err := s.submitProactiveMessage(ctx, characterID, conversationID, channel, prompt, requestID)
	if err != nil {
		return "", err
	}
	if result.Response != nil {
		return result.Response.Reply, nil
	}
	return "", nil
}
