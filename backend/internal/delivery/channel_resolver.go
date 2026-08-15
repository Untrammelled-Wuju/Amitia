package delivery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ChannelProvider interface {
	Channel() string
	Deliver(ctx context.Context, intent DeliveryIntent) error
}

type ChannelResolver interface {
	Resolve(ctx context.Context, channel string) (ChannelProvider, error)
}

type CapabilityChannelResolver struct {
	capabilityService     *capability.CapabilityService
	invocationService     *capability.ProviderInvocationService
}

func NewCapabilityChannelResolver(
	capabilityService *capability.CapabilityService,
	invocationService *capability.ProviderInvocationService,
) *CapabilityChannelResolver {
	return &CapabilityChannelResolver{
		capabilityService: capabilityService,
		invocationService: invocationService,
	}
}

func (r *CapabilityChannelResolver) Resolve(ctx context.Context, channel string) (ChannelProvider, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel resolver: channel name is empty")
	}

	capID := capability.CapabilityID("channel.deliver." + channel)

	if r.capabilityService != nil && !r.capabilityService.HasExecutableProvider(capID) {
		return nil, fmt.Errorf("channel resolver: no executable provider for capability %s", capID)
	}

	return &capabilityChannelProvider{
		channel:          channel,
		capabilityID:     capID,
		invocationService: r.invocationService,
	}, nil
}

type capabilityChannelProvider struct {
	channel          string
	capabilityID     capability.CapabilityID
	invocationService *capability.ProviderInvocationService
}

func (p *capabilityChannelProvider) Channel() string {
	return p.channel
}

func (p *capabilityChannelProvider) Deliver(ctx context.Context, intent DeliveryIntent) error {
	if p.invocationService == nil {
		return fmt.Errorf("channel provider %q: invocation service not configured", p.channel)
	}

	input := channelDeliverInput{
		InteractionID:   intent.InteractionID,
		ResponseGroupID: intent.ResponseGroupID,
		PeerID:          intent.PeerID,
		ContentType:     intent.ContentType,
		Payload:         intent.Payload,
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("channel provider %q: marshal deliver input: %w", p.channel, err)
	}

	req := capability.ProviderInvocationRequest{
		CapabilityID:       p.capabilityID,
		Input:              payload,
		PreferredPlacement: capability.ProviderPlacementCore,
		RequiredPlacement:  capability.ProviderPlacementCore,
		AllowCore:          true,
		AllowDevice:        false,
	}

	_, err = p.invocationService.Invoke(ctx, req)
	if err != nil {
		return fmt.Errorf("channel provider %q: invoke capability %s: %w", p.channel, p.capabilityID, err)
	}

	return nil
}

type channelDeliverInput struct {
	InteractionID   string          `json:"interactionId"`
	ResponseGroupID string          `json:"responseGroupId"`
	PeerID          string          `json:"peerId"`
	ContentType     string          `json:"contentType"`
	Payload         json.RawMessage `json:"payload"`
}
