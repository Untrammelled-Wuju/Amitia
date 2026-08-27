package chat

import (
	"context"
	"strings"
	"time"
)

type DesktopPetChatLifecycle struct {
	InteractionID  string
	MessageID      string
	CharacterID    string
	UserID         string
	ConversationID string
	Phase          string
	StatusVersion  int64
	Origin         string
	CorrelationID  string
	OccurredAt     time.Time
}

type DesktopPetToolLifecycle struct {
	InteractionID string
	CharacterID   string
	UserID        string
	OperationID   string
	ToolCallID    string
	ToolName      string
	ToolCategory  string
	DisplayClass  string
	Phase         string
	Depth         int
	ErrorClass    string
	OccurredAt    time.Time
}

type DesktopPetLifecycleObserver interface {
	OnDesktopPetChatLifecycle(context.Context, DesktopPetChatLifecycle)
	OnDesktopPetToolLifecycle(context.Context, DesktopPetToolLifecycle)
}

func (s *service) SetDesktopPetLifecycleObserver(observer DesktopPetLifecycleObserver) {
	s.desktopPetLifecycle = observer
}

func (s *service) emitDesktopPetChat(ctx context.Context, req *ProcessMessageRequest, charID, convID, messageID, phase string, version int64) {
	if s == nil || s.desktopPetLifecycle == nil || strings.TrimSpace(charID) == "" {
		return
	}
	interactionID := strings.TrimSpace(req.InteractionID)
	if interactionID == "" {
		interactionID = strings.TrimSpace(req.RequestID)
	}
	if interactionID == "" {
		interactionID = strings.TrimSpace(messageID)
	}
	s.desktopPetLifecycle.OnDesktopPetChatLifecycle(ctx, DesktopPetChatLifecycle{
		InteractionID: interactionID, MessageID: messageID, CharacterID: charID,
		UserID: req.UserID, ConversationID: convID, Phase: phase, StatusVersion: version,
		Origin: req.Source, CorrelationID: req.RequestID, OccurredAt: time.Now().UTC(),
	})
}

func (s *service) emitDesktopPetTool(ctx context.Context, scope SkillScope, toolCallID, toolName, phase, errorClass string, depth int) {
	if s == nil || s.desktopPetLifecycle == nil || strings.TrimSpace(scope.CharacterID) == "" {
		return
	}
	operationID := strings.TrimSpace(toolCallID)
	if operationID == "" {
		operationID = strings.TrimSpace(scope.RequestID) + ":" + strings.TrimSpace(toolName)
	}
	s.desktopPetLifecycle.OnDesktopPetToolLifecycle(ctx, DesktopPetToolLifecycle{
		InteractionID: scope.RequestID, CharacterID: scope.CharacterID, UserID: scope.UserID,
		OperationID: operationID, ToolCallID: toolCallID, ToolName: toolName,
		ToolCategory: inferDesktopPetToolCategory(toolName), DisplayClass: inferDesktopPetToolDisplayClass(toolName),
		Phase: phase, Depth: depth, ErrorClass: errorClass, OccurredAt: time.Now().UTC(),
	})
}

func inferDesktopPetToolCategory(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(n, "search"), strings.Contains(n, "web"), strings.Contains(n, "browse"):
		return "research"
	case strings.Contains(n, "code"), strings.Contains(n, "shell"), strings.Contains(n, "exec"), strings.Contains(n, "file"):
		return "coding"
	case strings.Contains(n, "schedule"), strings.Contains(n, "calendar"), strings.Contains(n, "remind"):
		return "organizing"
	default:
		return "generic_work"
	}
}

func inferDesktopPetToolDisplayClass(name string) string {
	switch inferDesktopPetToolCategory(name) {
	case "research":
		return "researching"
	case "coding":
		return "working"
	case "organizing":
		return "organizing"
	default:
		return "working"
	}
}
