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

func (l ConversationScopeBindingLookup) FindScopeBindings(ctx context.Context, spaceID, channel, peerID string) ([]ScopeBinding, error) {
	if l.db == nil {
		return nil, nil
	}
	spaceID = normalizeScopeValue(spaceID)
	channel = strings.ToLower(normalizeScopeValue(channel))
	peerID = normalizeScopeValue(peerID)
	if spaceID == "" || channel == "" || peerID == "" {
		return nil, nil
	}
	type conversationBinding struct {
		ID      string
		SpaceID string
		Channel string
		PeerID  string
		Source  string
	}
	var rows []conversationBinding
	err := l.db.WithContext(ctx).Table("conversations").
		Select("id, space_id, channel, peer_id, source").
		Where("space_id = ? AND LOWER(channel) = ? AND peer_id = ?", spaceID, channel, peerID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	bindings := make([]ScopeBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, ScopeBinding{
			ID:             row.ID,
			SpaceID:        row.SpaceID,
			ConversationID: row.ID,
			Channel:        row.Channel,
			PeerID:         row.PeerID,
			Source:         row.Source,
			State:          ScopeBindingStateActive,
		})
	}
	return bindings, nil
}
