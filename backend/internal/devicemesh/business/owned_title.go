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

func NeedsOwnedConversationTitle(title, message string) bool {
	title = strings.TrimSpace(title)
	return title == "" || title == strings.TrimSpace(message) || title == "新对话" || title == "新群聊"
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
	if conversation == nil || conversation.Deleted || conversation.RoleID != coordination.StorageRole(inference.Scope, "conversation") || conversation.OwnerID != inference.Scope.ResourceOwnerID {
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
	if !NeedsOwnedConversationTitle(original, inference.Message) {
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
	return e.submit(ctx, inference.Scope, "conversation-title", []coordination.Mutation{{Kind: "conversation", ID: conversation.ID, RoleID: coordination.StorageRole(inference.Scope, "conversation"), ExpectedRevision: conversation.Revision, Body: body(current)}})
}

func (e *Engine) finishOwnedGroupTitle(ctx context.Context, scope coordination.ExecutionScope, state coordination.GroupState, message string, deliveries []Response) error {
	if len(deliveries) == 0 || strings.TrimSpace(message) == "" {
		return nil
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.ErrWrongOwner
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return err
	}
	conversation, err := port.Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return err
	}
	if conversation == nil || conversation.Deleted || conversation.RoleID != coordination.StorageRole(scope, "conversation") || conversation.OwnerID != scope.ResourceOwnerID {
		return coordination.ErrWrongOwner
	}
	var current struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(conversation.Body, &current); err != nil {
		return err
	}
	if !NeedsOwnedConversationTitle(current.Title, message) {
		return nil
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil {
		return err
	}
	generation := Generation{Text: groupTitleGenerationText(deliveries)}
	if strings.TrimSpace(generation.Text) == "" {
		return nil
	}
	inference := Inference{
		Scope:          scope,
		ConversationID: state.ConversationID,
		Snapshot:       coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: role, Group: &state, Resources: []coordination.Resource{*conversation}},
		Message:        message,
	}
	return e.finishOwnedTitle(ctx, inference, generation)
}

func groupTitleGenerationText(deliveries []Response) string {
	var builder strings.Builder
	for _, delivery := range deliveries {
		text := strings.TrimSpace(delivery.Generation.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		if name := strings.TrimSpace(delivery.CharacterName); name != "" {
			builder.WriteString(name)
			builder.WriteString("：")
		}
		builder.WriteString(text)
		if len([]rune(builder.String())) >= 1200 {
			break
		}
	}
	runes := []rune(strings.TrimSpace(builder.String()))
	if len(runes) > 1200 {
		runes = runes[:1200]
	}
	return string(runes)
}
