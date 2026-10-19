package business

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) finishOwnedEffects(ctx context.Context, inference Inference, generation Generation) error {
	if generation.Partial || generation.Suppressed || inference.Preview {
		return nil
	}
	if inference.Ingress != nil && inference.Ingress.SuppressReplyPersistence {
		return nil
	}
	port, ok := e.data.(coordination.OwnerPostCommitPort)
	if !ok {
		return nil
	}
	request := coordination.OwnedChatEffectsRequest{ConversationID: inference.ConversationID, RequestID: inference.Scope.RequestID, MessageID: inference.Scope.RequestID + "/assistant", Source: "manual"}
	if inference.Ingress != nil {
		request.ThreadID, request.Source = inference.Ingress.ThreadID, inference.Ingress.Source
		request.Channel, request.PeerID, request.IsInternal = inference.Ingress.Channel, inference.Ingress.PeerID, inference.Ingress.IsInternal
	}
	return port.CommitOwnedChatEffects(ctx, inference.Scope, request)
}
