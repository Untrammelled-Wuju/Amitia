package scope

import (
	"context"
	"fmt"
	"time"
)

type ScopeResolveRequest struct {
	Expression     ScopeExpression `json:"expression"`
	CharacterID    string          `json:"characterId,omitempty"`
	ConversationID string          `json:"conversationId,omitempty"`
	ExtensionID    string          `json:"extensionId,omitempty"`
	ModuleID       string          `json:"moduleId,omitempty"`
	InvocationID   string          `json:"invocationId,omitempty"`
}

func ResolveScope(ctx context.Context, req ScopeResolveRequest) ([]ScopeRef, error) {
	if err := req.Expression.Validate(); err != nil {
		return nil, fmt.Errorf("invalid expression: %w", err)
	}
	return resolveExpression(ctx, req.Expression, req)
}

func resolveExpression(ctx context.Context, expr ScopeExpression, req ScopeResolveRequest) ([]ScopeRef, error) {
	resolved := make([]ScopeRef, 0, len(expr.Scopes))

	for _, s := range expr.Scopes {
		r, err := resolveScopeRef(ctx, s, req)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, r)
	}

	for _, placeholder := range expr.Placeholders {
		r, err := resolvePlaceholder(ctx, placeholder, req)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, r)
	}

	for _, child := range expr.Children {
		childResolved, err := resolveExpression(ctx, child, req)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, childResolved...)
	}

	return resolved, nil
}

func resolveScopeRef(ctx context.Context, ref ScopeRef, req ScopeResolveRequest) (ScopeRef, error) {
	switch ref.Type {
	case ScopeGlobal:
		return ref, nil
	case ScopeCharacter:
		if ref.CharacterID == "" || ref.CharacterID == string(PHCurrentCharacter) {
			if req.CharacterID == "" {
				return ref, fmt.Errorf("cannot resolve current character: no character ID provided")
			}
			return NewCharacterScope(req.CharacterID), nil
		}
		return ref, nil
	case ScopeConversation:
		if ref.ConversationID == "" || ref.ConversationID == string(PHCurrentConversation) {
			if req.ConversationID == "" {
				return ref, fmt.Errorf("cannot resolve current conversation: no conversation ID provided")
			}
			return NewConversationScope(req.ConversationID), nil
		}
		return ref, nil
	case ScopeExtension:
		if ref.ExtensionID == "" || ref.ExtensionID == string(PHOwnerExtension) {
			if req.ExtensionID == "" {
				return ref, fmt.Errorf("cannot resolve owner extension: no extension ID provided")
			}
			return NewExtensionScope(req.ExtensionID), nil
		}
		return ref, nil
	case ScopeModule:
		extID := ref.ExtensionID
		modID := ref.ModuleID
		if extID == "" || extID == string(PHOwnerExtension) {
			if req.ExtensionID == "" {
				return ref, fmt.Errorf("cannot resolve owner extension: no extension ID provided")
			}
			extID = req.ExtensionID
		}
		if modID == "" || modID == string(PHOwnerModule) {
			if req.ModuleID == "" {
				return ref, fmt.Errorf("cannot resolve owner module: no module ID provided")
			}
			modID = req.ModuleID
		}
		return NewModuleScope(extID, modID), nil
	case ScopeResource, ScopeInvocation, ScopeSession:
		return ref, nil
	default:
		return ref, fmt.Errorf("unknown scope type: %s", ref.Type)
	}
}

func resolvePlaceholder(ctx context.Context, ph ScopePlaceholder, req ScopeResolveRequest) (ScopeRef, error) {
	switch ph {
	case PHCurrentCharacter:
		if req.CharacterID == "" {
			return ScopeRef{}, fmt.Errorf("cannot resolve placeholder CURRENT_CHARACTER: no character ID provided")
		}
		return NewCharacterScope(req.CharacterID), nil
	case PHCurrentConversation:
		if req.ConversationID == "" {
			return ScopeRef{}, fmt.Errorf("cannot resolve placeholder CURRENT_CONVERSATION: no conversation ID provided")
		}
		return NewConversationScope(req.ConversationID), nil
	case PHOwnerExtension:
		if req.ExtensionID == "" {
			return ScopeRef{}, fmt.Errorf("cannot resolve placeholder OWNER_EXTENSION: no extension ID provided")
		}
		return NewExtensionScope(req.ExtensionID), nil
	case PHOwnerModule:
		if req.ModuleID == "" {
			return ScopeRef{}, fmt.Errorf("cannot resolve placeholder OWNER_MODULE: no module ID provided")
		}
		return NewModuleScope(req.ExtensionID, req.ModuleID), nil
	default:
		return ScopeRef{}, fmt.Errorf("unknown placeholder: %s", ph)
	}
}

func CreateSnapshot(invocationID string, scopes []ScopeRef, characterID, conversationID, extensionID, moduleID string) ScopeSnapshot {
	return ScopeSnapshot{
		SnapshotID:     fmt.Sprintf("snap-%s-%d", invocationID, time.Now().UnixNano()),
		InvocationID:   invocationID,
		ResolvedScopes: scopes,
		CharacterID:    characterID,
		ConversationID: conversationID,
		ExtensionID:    extensionID,
		ModuleID:       moduleID,
		CreatedAt:      time.Now(),
	}
}

func (s ScopeSnapshot) Contains(other ScopeRef) bool {
	for _, scope := range s.ResolvedScopes {
		if scope.Contains(other) {
			return true
		}
	}
	return false
}
