package kernel

import (
	"context"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var _ capability.ChannelStore = (*deliveryChannelStore)(nil)

type deliveryChannelStore struct {
	store    *delivery.SQLiteDeliveryStore
	resolver delivery.ChannelResolver
}

func NewDeliveryChannelStore(store *delivery.SQLiteDeliveryStore, resolver delivery.ChannelResolver) *deliveryChannelStore {
	return &deliveryChannelStore{store: store, resolver: resolver}
}

func (s *deliveryChannelStore) CreateIntent(ctx context.Context, channel, peerID, contentType string, payload []byte) (string, error) {
	intent := delivery.DeliveryIntent{
		ID:            delivery.GenerateDeliveryID(channel, peerID, contentType, uuid.New().String()),
		InteractionID: channel,
		Channel:       channel,
		PeerID:        peerID,
		ContentType:   contentType,
		Payload:       payload,
		Status:        delivery.DeliveryStatusPending,
		MaxRetries:    5,
	}
	if err := s.store.CreateIntent(intent); err != nil {
		return "", err
	}
	return intent.ID, nil
}

func (s *deliveryChannelStore) IsAvailable(channel string) bool {
	if s.resolver == nil {
		return channel == "web"
	}
	return s.resolver.Has(channel)
}
