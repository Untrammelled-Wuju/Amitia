package business

import (
	"context"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) PreviewReply(ctx context.Context, request Request) (Generation, error) {
	if strings.TrimSpace(request.Message) == "" || len(request.Message) > 128<<10 {
		return Generation{}, errors.New("预览消息无效")
	}
	if err := validateConversationSettings(request); err != nil {
		return Generation{}, err
	}
	view, err := e.Query(ctx, request, coordination.DataQuery{ResourceKind: "conversation", Limit: 1})
	if err != nil {
		return Generation{}, err
	}
	view.Snapshot.Resources = nil
	var result Generation
	err = e.WithExecutionScope(ctx, view.Scope, func(ctx context.Context) error {
		snapshot, err := e.data.Snapshot(ctx, view.Scope, coordination.DataQuery{Limit: 128})
		if err != nil {
			return err
		}
		if err := coordination.ValidateSnapshot(view.Scope, snapshot); err != nil {
			return err
		}
		result, err = e.model.GenerateOwnedReply(ctx, Inference{Preview: true, Scope: view.Scope, Snapshot: snapshot, Message: request.Message, ModelConfigID: request.ModelConfigID, ReasoningEffort: request.ReasoningEffort, ReasoningEnabled: request.ReasoningEnabled})
		return err
	})
	return result, err
}
