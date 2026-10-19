package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/temporal"
)

func (p *meshLocalDataPort) ownedRuntimeContext(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (map[string]json.RawMessage, error) {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	legacyScope := interaction.InteractionScope{SpaceID: p.legacySpaceID, CharacterID: scope.RoleID, ConversationID: query.ConversationID, Channel: "web", RequestID: scope.RequestID}
	if query.ChatRuntime != nil {
		legacyScope.Channel, legacyScope.Source, legacyScope.PeerID = query.ChatRuntime.Channel, query.ChatRuntime.Source, query.ChatRuntime.PeerID
		ctx = temporal.ContextWithDeviceTimezone(ctx, query.ChatRuntime.DeviceTimezone)
	}
	loaders := []interaction.ContextLoader{interaction.NewPsycheContextLoader(p.services.DB), interaction.NewRelationshipContextLoader(p.services.DB), interaction.NewNeedContextLoader(p.services.DB), interaction.NewUnresolvedThreadContextLoader(p.services.DB)}
	if p.services.Temporal != nil {
		loaders = append(loaders, interaction.NewTemporalContextLoader(p.services.Temporal))
	}
	if p.services.KernelContainer.ToolFacade != nil {
		loaders = append(loaders, interaction.NewLifeContextLoader(newKernelExtensionContextProvider(p.services.KernelContainer.ToolFacade)))
	}
	result := map[string]json.RawMessage{}
	for _, loader := range loaders {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		loadContext, cancel := context.WithTimeout(ctx, loader.Timeout())
		value, err := loader.Load(loadContext, legacyScope, "owned-role-"+strconv.FormatInt(scope.RoleRevision, 10))
		cancel()
		if err != nil {
			value = interaction.FieldUnavailable[any](loader.Name())
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result[loader.Name()] = raw
	}
	if query.ChatRuntime != nil && !query.ChatRuntime.IsInternal && scope.TurnID != "" && p.services.RelTimeCoordinator != nil {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		prepared, err := p.services.RelTimeCoordinator.PrepareInbound(ctx, temporal.PrepareInboundInput{SpaceID: p.legacySpaceID, CharacterID: scope.RoleID, ConversationID: query.ConversationID, Channel: legacyScope.Channel, PeerID: legacyScope.PeerID, Source: legacyScope.Source, RequestID: scope.RequestID, InteractionID: scope.TurnID, ObservedAt: time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		var field interaction.SnapshotField[temporal.Snapshot]
		if raw := result["temporal"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &field); err != nil {
				return nil, err
			}
		}
		field.Value.RelationshipTime = &prepared
		field.Source, field.Status = "owned-temporal", interaction.LoadStatusReady
		result["temporal"], err = json.Marshal(field)
		if err != nil {
			return nil, err
		}
	}
	return result, coordination.ValidateCurrent(ctx)
}
