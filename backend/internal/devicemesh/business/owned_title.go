package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type TitleModel interface {
	PlanOwnedTitle(context.Context, Inference, Generation) (string, error)
}

func (e *Engine) finishOwnedTitle(ctx context.Context, inference Inference, generation Generation) error {
	planner, ok := e.model.(TitleModel)
	if !ok || inference.Preview || inference.InternalWorker || generation.Partial || generation.Suppressed {
		return nil
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.ErrWrongOwner
	}
	if err := e.coordination.Validate(ctx, inference.Scope); err != nil {
		return err
	}
	conversation, err := port.Resource(ctx, inference.Scope, "conversation", inference.ConversationID)
	if err != nil {
		return err
	}
	if conversation == nil || conversation.Deleted || conversation.RoleID != inference.Scope.RoleID || conversation.OwnerID != inference.Scope.ResourceOwnerID {
		return coordination.ErrWrongOwner
	}
	var current map[string]json.RawMessage
	if err := json.Unmarshal(conversation.Body, &current); err != nil {
		return err
	}
	var original string
	if value, exists := current["title"]; exists {
		if err := json.Unmarshal(value, &original); err != nil {
			return err
		}
	}
	if title := strings.TrimSpace(original); title != "" && title != strings.TrimSpace(inference.Message) && title != "新对话" {
		return nil
	}
	title, err := planner.PlanOwnedTitle(ctx, inference, generation)
	if err != nil {
		return err
	}
	if title == "" {
		return nil
	}
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) < 2 || utf8.RuneCountInString(title) > 16 || strings.ContainsAny(title, "\x00\r\n") {
		return errors.New("自动会话标题格式无效")
	}
	if err := e.coordination.Validate(ctx, inference.Scope); err != nil {
		return err
	}
	latest, err := port.Resource(ctx, inference.Scope, "conversation", inference.ConversationID)
	if err != nil {
		return err
	}
	if latest == nil || latest.Deleted || latest.Revision != conversation.Revision {
		return coordination.ErrResourceVersion
	}
	current["title"] = body(title)
	return e.submit(ctx, inference.Scope, "conversation-title", []coordination.Mutation{{Kind: "conversation", ID: conversation.ID, RoleID: inference.Scope.RoleID, ExpectedRevision: conversation.Revision, Body: body(current)}})
}
