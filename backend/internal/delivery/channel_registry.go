// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	basechannel "github.com/u-ai/backend/internal/channel"
)

func BuildBuiltinChannelResolver() ChannelResolver {
	return NewMapChannelResolverWith([]ChannelAdapter{
		NewWebChannelAdapter(),
	})
}

type CapabilityChannelResolver struct {
	providers       *PluginChannelProviderRegistry
	builtinResolver ChannelResolver
}

func NewCapabilityChannelResolver(providers *PluginChannelProviderRegistry, builtin ChannelResolver) *CapabilityChannelResolver {
	return &CapabilityChannelResolver{
		providers:       providers,
		builtinResolver: builtin,
	}
}

func (r *CapabilityChannelResolver) Resolve(channelName string) ChannelAdapter {
	if r.providers != nil {
		if provider, err := r.providers.Provider(channelName); err == nil {
			return &pluginChannelAdapter{
				name:     channelName,
				provider: provider,
			}
		}
	}
	if r.builtinResolver != nil {
		return r.builtinResolver.Resolve(channelName)
	}
	return nil
}

func (r *CapabilityChannelResolver) Register(adapter ChannelAdapter) {
	if r.builtinResolver != nil {
		r.builtinResolver.Register(adapter)
	}
}

func (r *CapabilityChannelResolver) Channels() []string {
	result := make([]string, 0)
	if r.builtinResolver != nil {
		result = append(result, r.builtinResolver.Channels()...)
	}
	if r.providers != nil {
		result = append(result, r.providers.Channels()...)
	}
	sort.Strings(result)
	return result
}

func (r *CapabilityChannelResolver) Unregister(channelName string) {
	if r.builtinResolver != nil {
		r.builtinResolver.Unregister(channelName)
	}
}

func (r *CapabilityChannelResolver) Has(channelName string) bool {
	if r.providers != nil && r.providers.Has(channelName) {
		return true
	}
	if r.builtinResolver != nil {
		return r.builtinResolver.Has(channelName)
	}
	return false
}

type pluginChannelAdapter struct {
	name     string
	provider *basechannel.HTTPProvider
}

func (a *pluginChannelAdapter) Name() string {
	return a.name
}

func (a *pluginChannelAdapter) ProviderInstanceID() string {
	return "channel.provider." + a.name
}

func (a *pluginChannelAdapter) Deliver(intent DeliveryIntent) error {
	if a.provider == nil {
		return fmt.Errorf("channel provider unavailable: %s", a.name)
	}
	request := basechannel.SendRequest{
		Channel:        basechannel.ID(intent.Channel),
		PeerID:         intent.PeerID,
		ConversationID: intent.InteractionID,
		ContentType:    intent.ContentType,
		Payload:        json.RawMessage(intent.Payload),
		IdempotencyKey: intent.ID,
	}
	if intent.ContentType == "text" {
		request.Text = extractContentFromPayload(intent.Payload)
	}
	_, err := a.provider.Send(context.Background(), request)
	return err
}
