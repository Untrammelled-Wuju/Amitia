package business

import (
	"context"
	"encoding/json"
)

type Ingress struct {
	Source                   string
	Channel                  string
	PeerID                   string
	PeerKind                 string
	ConversationTitle        string
	SenderName               string
	SenderID                 string
	MentionNames             []string
	SandboxWorkspaceID       string
	SandboxRoot              string
	SandboxAllowNetwork      bool
	ReplyExcerpt             string
	ReplyRole                string
	ThreadID                 string
	SessionID                string
	DeviceTimezone           string
	ProactiveTaskInstruction string
	ProactiveTimeContext     string
	ProactiveRecentContext   string
	ProactiveRelationship    string
	ProactiveEmotion         string
	ProactiveMemory          string
	MessageStyle             string
	ExpressionPlan           json.RawMessage
	IsInternal               bool
	SuppressReplyPersistence bool
}

type ingressContextKey struct{}

func WithIngress(ctx context.Context, ingress Ingress) context.Context {
	ingress.ExpressionPlan = append(json.RawMessage(nil), ingress.ExpressionPlan...)
	ingress.MentionNames = append([]string(nil), ingress.MentionNames...)
	return context.WithValue(ctx, ingressContextKey{}, ingress)
}

func IngressFromContext(ctx context.Context) (Ingress, bool) {
	ingress, ok := ctx.Value(ingressContextKey{}).(Ingress)
	ingress.ExpressionPlan = append(json.RawMessage(nil), ingress.ExpressionPlan...)
	return ingress, ok
}
