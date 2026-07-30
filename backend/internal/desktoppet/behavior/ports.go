package behavior

import (
	"context"
)

type ActivePetPort interface {
	ResolveActivePet(ctx context.Context, userID, characterID string) (*ActivePetSnapshot, error)
}

type RuntimeActionPort interface {
	SubmitBehaviorCommand(ctx context.Context, command BehaviorRuntimeCommand) (*CommandReceipt, error)
	QueryPlayback(ctx context.Context, petInstanceID string) (*PlaybackSnapshot, error)
}

type BehaviorStateRepository interface {
	LoadContext(ctx context.Context, userID, characterID string) (*BehaviorContextSnapshot, error)
	SaveContextCAS(ctx context.Context, currentRevision int64, next BehaviorContextSnapshot) (bool, error)
	InsertInboxIfAbsent(ctx context.Context, event BehaviorEventEnvelope) (bool, error)
	LeaseInbox(ctx context.Context, limit int, leaseToken string) ([]InboxRecord, error)
	MarkInboxStatus(ctx context.Context, eventID string, status InboxStatus, errorCode string) error
	AppendDecision(ctx context.Context, decision BehaviorDecisionAudit) error
	UpdateDecisionStatus(ctx context.Context, decisionID string, status DecisionStatus, at interface{}) error
	LoadCooldowns(ctx context.Context, userID, characterID string) ([]CooldownRecord, error)
	SaveCooldown(ctx context.Context, record CooldownRecord) error
	CleanupExpiredCooldowns(ctx context.Context, before interface{}) error
	CleanupOldRecords(ctx context.Context, before interface{}) error
	DeleteCharacterData(ctx context.Context, userID, characterID string) error
}

type CharacterAffectPort interface {
	GetAffectSnapshot(ctx context.Context, userID, characterID string) (*AffectBehaviorSnapshot, error)
}

type CharacterActivityPort interface {
	GetActivitySnapshot(ctx context.Context, userID, characterID string) (*ActivityBehaviorSnapshot, error)
}

type InteractionLifecycleObserver interface {
	OnInteractionLifecycle(ctx context.Context, event InteractionLifecycleEvent)
}

type ToolLifecycleObserver interface {
	OnToolLifecycle(ctx context.Context, event ToolLifecycleEvent)
}

type VoiceLifecycleObserver interface {
	OnVoiceLifecycle(ctx context.Context, event VoiceLifecycleEvent)
}

type AffectChangeObserver interface {
	OnAffectChanged(ctx context.Context, characterID string, old, new AffectBehaviorSnapshot)
}

type DesktopGestureObserver interface {
	OnDesktopGesture(ctx context.Context, event DesktopGestureEvent)
}

type PlaybackFeedbackObserver interface {
	OnPlaybackFeedback(ctx context.Context, feedback PlaybackFeedback)
}

type BehaviorEventPublisher interface {
	PublishBehaviorEvent(ctx context.Context, event BehaviorEventEnvelope) error
}

type BehaviorEventConsumer interface {
	ConsumeBehaviorEvent(ctx context.Context, event BehaviorEventEnvelope) error
}

type NoopInteractionObserver struct{}

func (n *NoopInteractionObserver) OnInteractionLifecycle(_ context.Context, _ InteractionLifecycleEvent) {}

type NoopToolObserver struct{}

func (n *NoopToolObserver) OnToolLifecycle(_ context.Context, _ ToolLifecycleEvent) {}

type NoopVoiceObserver struct{}

func (n *NoopVoiceObserver) OnVoiceLifecycle(_ context.Context, _ VoiceLifecycleEvent) {}

type NoopAffectObserver struct{}

func (n *NoopAffectObserver) OnAffectChanged(_ context.Context, _ string, _, _ AffectBehaviorSnapshot) {}

type NoopDesktopObserver struct{}

func (n *NoopDesktopObserver) OnDesktopGesture(_ context.Context, _ DesktopGestureEvent) {}

type NoopPlaybackObserver struct{}

func (n *NoopPlaybackObserver) OnPlaybackFeedback(_ context.Context, _ PlaybackFeedback) {}
