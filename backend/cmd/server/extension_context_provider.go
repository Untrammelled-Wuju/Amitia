package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extensioncontext"
)

type kernelExtensionContextProvider struct {
	facade extensionContextFacade
}

type extensionContextFacade interface {
	ListContextProviders(ctx context.Context, slot string) []capability.ToolDefinition
	ExecuteTool(ctx context.Context, toolID capability.CapabilityID, input json.RawMessage, scope kernel.InvocationScope, externalCallID string, idempotencyKey string) (kernel.ToolDispatchResult, bool)
}

func newKernelExtensionContextProvider(facade extensionContextFacade) *kernelExtensionContextProvider {
	return &kernelExtensionContextProvider{facade: facade}
}

func (p *kernelExtensionContextProvider) Resolve(ctx context.Context, slot string, request extensioncontext.Request) (json.RawMessage, error) {
	if p == nil || p.facade == nil {
		return nil, fmt.Errorf("extension context provider is unavailable")
	}
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return nil, fmt.Errorf("extension context slot is required")
	}
	input, err := json.Marshal(map[string]any{
		"spaceId":        request.SpaceID,
		"characterId":    request.CharacterID,
		"conversationId": request.ConversationID,
		"at":             request.At.Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, err
	}
	providers := p.facade.ListContextProviders(ctx, slot)
	snapshot := extensioncontext.Snapshot{
		Slot:          slot,
		Contributions: make([]extensioncontext.Contribution, 0, len(providers)),
	}
	for _, provider := range providers {
		contribution := extensioncontext.Contribution{
			Source:      kernel.ContextProviderSource(provider),
			ExtensionID: provider.ExtensionID,
			ModuleID:    provider.ModuleID,
			ToolID:      provider.ID,
			Priority:    kernel.ContextProviderPriority(provider),
		}
		result, found := p.facade.ExecuteTool(
			ctx,
			capability.CapabilityID(provider.ID),
			input,
			kernel.InvocationScope{
				SpaceID:        request.SpaceID,
				CharacterID:    request.CharacterID,
				ConversationID: request.ConversationID,
				Trigger:        "extension_context",
			},
			"",
			fmt.Sprintf("extension-context:%s:%s:%s:%d", slot, provider.ID, request.CharacterID, request.At.UnixNano()),
		)
		switch {
		case !found:
			contribution.Error = "context provider unavailable"
		case result.Error != nil:
			contribution.Error = fmt.Sprintf("%s: %s", result.Error.Code, result.Error.Message)
		case !strings.EqualFold(result.Status, "SUCCESS"):
			contribution.Error = fmt.Sprintf("context provider failed: %s", result.Status)
		case len(result.Output) == 0:
			contribution.Error = "context provider returned empty output"
		default:
			contribution.Data = result.Output
		}
		snapshot.Contributions = append(snapshot.Contributions, contribution)
	}
	return json.Marshal(snapshot)
}
