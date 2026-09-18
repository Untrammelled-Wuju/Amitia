package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type channelCommandInput struct {
	ChannelID      string         `json:"channelId"`
	AccountID      string         `json:"accountId,omitempty"`
	ConversationID string         `json:"conversationId,omitempty"`
	Limit          int            `json:"limit,omitempty"`
	Offset         int            `json:"offset,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

func SetupChannelHostCommands(registry *HostCommandRegistry, providers *delivery.PluginChannelProviderRegistry) error {
	if registry == nil {
		return fmt.Errorf("channel host commands: registry is nil")
	}
	if providers == nil {
		return fmt.Errorf("channel host commands: provider registry is nil")
	}
	commands := []HostCommandDefinition{
		{
			CommandID:   "channel.status",
			Description: "Get channel provider status",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"accountId":{"type":"string"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.StatusData(ctx, req.ChannelID)
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.connect",
			Description: "Connect a channel provider",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskMedium,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"config":{"type":"object"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				config := req.Config
				if config == nil {
					config = map[string]any{}
				}
				result, err := providers.Connect(ctx, req.ChannelID, config)
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.disconnect",
			Description: "Disconnect a channel provider",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskMedium,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				if err := providers.Disconnect(ctx, req.ChannelID); err != nil {
					return nil, err
				}
				return json.RawMessage(`{"success":true}`), nil
			},
		},
		{
			CommandID:   "channel.messages",
			Description: "Read channel messages",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"conversationId":{"type":"string"},"limit":{"type":"integer"},"offset":{"type":"integer"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.Messages(ctx, req.ChannelID, req.ConversationID, req.Limit, req.Offset)
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
	}
	for _, cmd := range commands {
		if err := registry.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

func decodeChannelCommandInput(input []byte) (channelCommandInput, error) {
	var req channelCommandInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return req, NewHostCommandError(ErrCodeHostCommandInputInvalid, "invalid channel command input", err)
		}
	}
	if req.ChannelID == "" {
		return req, NewHostCommandError(ErrCodeHostCommandInputInvalid, "channelId is required", nil)
	}
	if req.Limit <= 0 {
		req.Limit = 200
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	return req, nil
}
