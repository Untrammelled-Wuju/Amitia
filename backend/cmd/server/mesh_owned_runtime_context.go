package main

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func (p *meshLocalDataPort) ownedRuntimeContext(ctx context.Context, scope coordination.ExecutionScope, conversationID string) (map[string]json.RawMessage, error) {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	legacyScope := interaction.InteractionScope{SpaceID: p.legacySpaceID, CharacterID: scope.RoleID, ConversationID: conversationID, Channel: "web", RequestID: scope.RequestID}
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
	return result, coordination.ValidateCurrent(ctx)
}
