// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package delivery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/config"
)

const (
	defaultQQSidecarURL     = "http://127.0.0.1:19877"
	defaultWechatSidecarURL = "http://127.0.0.1:19876"
)

// BuildChannelResolverFromConfig constructs a ChannelResolver based on the
// application configuration. The web channel is always registered. QQ and
// Wechat channels are added only when the corresponding sidecar is enabled
// in config. Sidecar URLs default to the standard local ports unless the
// port value in config is non-zero.
//
// Note: This resolver is used as a Builtin Channel Provider internal implementation.
// The formal Channel discovery mechanism is through CapabilityService → ProviderInvocation
// (channel.deliver.web/qq/wechat capabilities).
func BuildChannelResolverFromConfig() ChannelResolver {
	adapters := []ChannelAdapter{
		NewWebChannelAdapter(),
	}

	cfg := config.AppCfg
	if cfg == nil {
		return NewMapChannelResolverWith(adapters)
	}

	if cfg.Runtime.Sidecars.QQ.Enabled {
		adapters = append(adapters, NewQQChannelAdapter(sidecarURL(cfg.Runtime.Sidecars.QQ.Port, defaultQQSidecarURL)))
	}
	if cfg.Runtime.Sidecars.Wechat.Enabled {
		adapters = append(adapters, NewWechatChannelAdapter(sidecarURL(cfg.Runtime.Sidecars.Wechat.Port, defaultWechatSidecarURL)))
	}

	return NewMapChannelResolverWith(adapters)
}

func sidecarURL(port int, defaultURL string) string {
	if port <= 0 {
		return defaultURL
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// BuildCapabilityChannelResolver constructs a ChannelResolver that delegates
// to the CapabilityService for channel delivery. This is the formal Channel
// discovery mechanism: channel.deliver.web/qq/wechat → CapabilityService → ProviderInvocation.
type CapabilityChannelResolver struct {
	providerInvoker CapabilityProviderInvoker
	builtinResolver ChannelResolver
}

type CapabilityProviderInvoker interface {
	InvokeCapability(ctx context.Context, capabilityID string, input []byte) ([]byte, error)
}

func NewCapabilityChannelResolver(invoker CapabilityProviderInvoker, builtin ChannelResolver) *CapabilityChannelResolver {
	return &CapabilityChannelResolver{
		providerInvoker: invoker,
		builtinResolver: builtin,
	}
}

func (r *CapabilityChannelResolver) Resolve(channelName string) ChannelAdapter {
	if r.providerInvoker != nil {
		capID := "channel.deliver." + channelName
		return &capabilityChannelAdapter{
			invoker: r.providerInvoker,
			capID:   capID,
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
	if r.builtinResolver != nil {
		return r.builtinResolver.Channels()
	}
	return []string{}
}

func (r *CapabilityChannelResolver) Unregister(channelName string) {
	if r.builtinResolver != nil {
		r.builtinResolver.Unregister(channelName)
	}
}

func (r *CapabilityChannelResolver) Has(channelName string) bool {
	if r.builtinResolver != nil {
		return r.builtinResolver.Has(channelName)
	}
	return false
}

type capabilityChannelAdapter struct {
	invoker CapabilityProviderInvoker
	capID   string
}

func (a *capabilityChannelAdapter) Name() string {
	return a.capID
}

func (a *capabilityChannelAdapter) ProviderInstanceID() string {
	return "capability." + a.capID
}

func (a *capabilityChannelAdapter) Deliver(intent DeliveryIntent) error {
	input := map[string]interface{}{
		"channel":     intent.Channel,
		"peerId":      intent.PeerID,
		"contentType": intent.ContentType,
		"payload":     intent.Payload,
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal channel input: %w", err)
	}
	_, err = a.invoker.InvokeCapability(nil, a.capID, inputBytes)
	return err
}
