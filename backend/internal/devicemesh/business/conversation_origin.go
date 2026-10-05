package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ConversationOrigin struct {
	OwnerID string `json:"ownerId"`
	ID      string `json:"id"`
}

type conversationRoute struct {
	CurrentID    string
	HistoricalID string
	Origin       *ConversationOrigin
}

func resolveConversationRoute(scope coordination.ExecutionScope, id string, origin *ConversationOrigin) (conversationRoute, error) {
	if len(id) > 512 || strings.ContainsRune(id, '\x00') {
		return conversationRoute{}, coordination.ErrWrongOwner
	}
	if origin == nil {
		return conversationRoute{CurrentID: id, HistoricalID: id}, nil
	}
	if id == "" || origin.ID != id || origin.OwnerID == "" || len(origin.OwnerID) > 512 || strings.ContainsRune(origin.OwnerID, '\x00') {
		return conversationRoute{}, coordination.ErrWrongOwner
	}
	copy := *origin
	if origin.OwnerID == scope.ResourceOwnerID {
		return conversationRoute{CurrentID: id, Origin: &copy}, nil
	}
	if !scope.Coordinated || origin.OwnerID != scope.TargetDeviceID || scope.ResourceOwnerID != scope.CoreID {
		return conversationRoute{}, coordination.ErrWrongOwner
	}
	return continuationRoute(scope, id, &copy, true)
}

func continuationRoute(scope coordination.ExecutionScope, id string, origin *ConversationOrigin, history bool) (conversationRoute, error) {
	encoded, err := json.Marshal([]string{scope.CoreID, scope.ResourceOwnerID, scope.RoleID, origin.OwnerID, origin.ID})
	if err != nil {
		return conversationRoute{}, err
	}
	digest := sha256.Sum256(encoded)
	copy := *origin
	route := conversationRoute{CurrentID: "continuation/" + hex.EncodeToString(digest[:]), Origin: &copy}
	if history {
		route.HistoricalID = id
	}
	return route, nil
}

func (e *Engine) resolveStoredConversationRoute(ctx context.Context, scope coordination.ExecutionScope, id string, origin *ConversationOrigin) (conversationRoute, error) {
	route, err := resolveConversationRoute(scope, id, origin)
	if err == nil || !errors.Is(err, coordination.ErrWrongOwner) || origin == nil || !scope.Coordinated || origin.ID != id || origin.OwnerID == "" || len(origin.OwnerID) > 512 || len(id) > 512 || strings.ContainsRune(id+origin.OwnerID, '\x00') {
		return route, err
	}
	route, err = continuationRoute(scope, id, origin, false)
	if err != nil {
		return conversationRoute{}, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return conversationRoute{}, coordination.ErrWrongOwner
	}
	resource, err := port.Resource(ctx, scope, "conversation", route.CurrentID)
	if err != nil {
		return conversationRoute{}, err
	}
	var document struct {
		Origin *ConversationOrigin `json:"conversationOrigin"`
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || json.Unmarshal(resource.Body, &document) != nil || document.Origin == nil || *document.Origin != *origin {
		return conversationRoute{}, coordination.ErrWrongOwner
	}
	return route, nil
}

func (e *Engine) resolveRunConversationRoute(ctx context.Context, scope coordination.ExecutionScope, request Request) (conversationRoute, error) {
	route, err := e.resolveStoredConversationRoute(ctx, scope, request.ConversationID, request.ConversationOrigin)
	origin := request.ConversationOrigin
	if err == nil || !errors.Is(err, coordination.ErrWrongOwner) || origin == nil || !scope.Coordinated || request.Context == nil || request.Context.PreviousCoreID != origin.OwnerID || request.Context.ConversationID != origin.ID || origin.ID != request.ConversationID || origin.ID == "" || len(origin.ID) > 512 || len(origin.OwnerID) > 512 || strings.ContainsRune(origin.OwnerID+origin.ID, '\x00') {
		return route, err
	}
	return continuationRoute(scope, origin.ID, origin, false)
}

func rejectAmbiguousConversation(route conversationRoute, current coordination.DataSnapshot, historical *coordination.DataSnapshot) error {
	if route.Origin != nil || historical == nil || route.CurrentID == "" || current.OwnerID == historical.OwnerID {
		return nil
	}
	hasConversation := func(snapshot coordination.DataSnapshot) bool {
		for _, resource := range snapshot.Resources {
			if resource.Kind == "conversation" && resource.ID == route.CurrentID {
				return true
			}
		}
		for _, raw := range snapshot.LegacyConversations {
			var document struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &document) == nil && document.ID == route.CurrentID {
				return true
			}
		}
		return len(snapshot.LegacyMessages) > 0
	}
	if hasConversation(current) && hasConversation(*historical) {
		return coordination.ErrRequestConflict
	}
	return nil
}
