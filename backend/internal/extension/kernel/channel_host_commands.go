package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type channelCommandInput struct {
	ChannelID      string         `json:"channelId"`
	AccountID      string         `json:"accountId,omitempty"`
	ConversationID string         `json:"conversationId,omitempty"`
	PeerID         string         `json:"peerId,omitempty"`
	Action         string         `json:"action,omitempty"`
	Text           string         `json:"text,omitempty"`
	DeliveryKey    string         `json:"deliveryKey,omitempty"`
	Limit          int            `json:"limit,omitempty"`
	Offset         int            `json:"offset,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	Input          map[string]any `json:"input,omitempty"`
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
		{
			CommandID:   "channel.config",
			Description: "Read channel provider configuration",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.Config(ctx, req.ChannelID)
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.invoke",
			Description: "Invoke a channel provider action",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskMedium,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"action":{"type":"string"},"input":{"type":"object"}},"required":["channelId","action"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				action := strings.TrimSpace(req.Action)
				if action == "" {
					return nil, NewHostCommandError(ErrCodeHostCommandInputInvalid, "action is required", nil)
				}
				result, err := providers.Invoke(ctx, req.ChannelID, action, req.Input)
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.logs",
			Description: "Read channel provider runtime logs",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"limit":{"type":"integer"},"offset":{"type":"integer"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.Logs(ctx, req.ChannelID, map[string]any{
					"limit":  req.Limit,
					"offset": req.Offset,
				})
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.peers",
			Description: "Read channel provider peers",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"limit":{"type":"integer"},"offset":{"type":"integer"}},"required":["channelId"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.Peers(ctx, req.ChannelID, map[string]any{
					"limit":  req.Limit,
					"offset": req.Offset,
				})
				if err != nil {
					return nil, err
				}
				return json.Marshal(result)
			},
		},
		{
			CommandID:   "channel.send",
			Description: "Send a text message through a channel provider",
			Permission:  "channel.provider.invoke",
			Scope:       HostCommandScopeExtension,
			Risk:        host_api.RiskHigh,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"channelId":{"type":"string"},"peerId":{"type":"string"},"conversationId":{"type":"string"},"text":{"type":"string"},"deliveryKey":{"type":"string"}},"required":["channelId","peerId","text"],"additionalProperties":false}`),
			Handler: func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
				req, err := decodeChannelCommandInput(input)
				if err != nil {
					return nil, err
				}
				result, err := providers.Send(ctx, req.ChannelID, req.PeerID, req.ConversationID, req.Text, req.DeliveryKey)
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
