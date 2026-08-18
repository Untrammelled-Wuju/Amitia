// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package delivery

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ResolverChannelStore struct {
	resolver ChannelResolver
}

func NewResolverChannelStore(resolver ChannelResolver) capability.ChannelStore {
	return &ResolverChannelStore{resolver: resolver}
}

func (s *ResolverChannelStore) CreateIntent(ctx context.Context, channel, peerID, contentType string, payload []byte) (string, error) {
	if !s.resolver.Has(channel) {
		return "", fmt.Errorf("channel %s not available", channel)
	}
	intent := NewDeliveryIntent(uuid.New().String(), channel, peerID, contentType, payload)
	return intent.ID, nil
}

func (s *ResolverChannelStore) IsAvailable(channel string) bool {
	return s.resolver.Has(channel)
}
