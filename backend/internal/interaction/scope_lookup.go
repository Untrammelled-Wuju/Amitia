package interaction

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

type ConversationScopeBindingLookup struct {
	db *gorm.DB
}

func NewConversationScopeBindingLookup(db *gorm.DB) ConversationScopeBindingLookup {
	return ConversationScopeBindingLookup{db: db}
}

func (l ConversationScopeBindingLookup) FindScopeBindings(ctx context.Context, channel, peerID string) ([]ScopeBinding, error) {
	if l.db == nil {
		return nil, nil
	}
	channel = strings.ToLower(normalizeScopeValue(channel))
	peerID = normalizeScopeValue(peerID)
	if channel == "" || peerID == "" {
		return nil, nil
	}
	type conversationBinding struct {
		ID          string
		CharacterID string
		Channel     string
		PeerID      string
		Source      string
	}
	var rows []conversationBinding
	err := l.db.WithContext(ctx).Table("conversations").
		Select("id, character_id, channel, peer_id, source").
		Where("LOWER(channel) = ? AND peer_id = ?", channel, peerID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	bindings := make([]ScopeBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, ScopeBinding{
			ID:             row.ID,
			CharacterID:    row.CharacterID,
			ConversationID: row.ID,
			Channel:        row.Channel,
			PeerID:         row.PeerID,
			Source:         row.Source,
			State:          ScopeBindingStateActive,
		})
	}
	return bindings, nil
}
