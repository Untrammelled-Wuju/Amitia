package adapters

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type CharacterOwnerPort interface {
	ResolveUserID(ctx context.Context, characterID string) string
}

type PetInfoPort interface {
	ResolvePetInfo(ctx context.Context, petInstanceID string) (userID, characterID string)
}

type enginePublisher struct {
	engine *behavior.BehaviorEngine
}

func NewEnginePublisher(engine *behavior.BehaviorEngine) behavior.BehaviorEventPublisher {
	return &enginePublisher{engine: engine}
}

func (p *enginePublisher) PublishBehaviorEvent(ctx context.Context, event behavior.BehaviorEventEnvelope) error {
	return p.engine.SubmitEvent(ctx, event)
}

type realClock struct{}

func (realClock) Now() time.Time                    { return time.Now() }
func (realClock) Since(t time.Time) time.Duration   { return time.Since(t) }

type AdapterManagerOptions struct {
	Clock         behavior.Clock
	OwnerResolver CharacterOwnerPort
	PetInfoPort   PetInfoPort
}

type AdapterManager struct {
	clock     behavior.Clock
	publisher behavior.BehaviorEventPublisher

	Interaction *InteractionAdapter
	Tool        *ToolAdapter
	Voice       *VoiceAdapter
	Affect      *AffectAdapter
	Activity    *ActivityAdapter
	Desktop     *DesktopAdapter
	Playback    *PlaybackAdapter
	Proactive   *ProactiveAdapter
}

func NewAdapterManager(publisher behavior.BehaviorEventPublisher, opts AdapterManagerOptions) *AdapterManager {
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}
	am := &AdapterManager{
		clock:     clock,
		publisher: publisher,
	}
	am.Interaction = NewInteractionAdapter(publisher, clock)
	am.Tool = NewToolAdapter(publisher, clock)
	am.Voice = NewVoiceAdapter(publisher, clock)
	am.Affect = NewAffectAdapter(publisher, clock, opts.OwnerResolver)
	am.Activity = NewActivityAdapter(publisher, clock, opts.OwnerResolver)
	am.Desktop = NewDesktopAdapter(publisher, clock)
	am.Playback = NewPlaybackAdapter(publisher, clock, opts.PetInfoPort)
	am.Proactive = NewProactiveAdapter(publisher, clock)
	return am
}

func (am *AdapterManager) OnInteractionLifecycle(ctx context.Context, event behavior.InteractionLifecycleEvent) {
	am.Interaction.OnInteractionLifecycle(ctx, event)
}

func (am *AdapterManager) OnToolLifecycle(ctx context.Context, event behavior.ToolLifecycleEvent) {
	am.Tool.OnToolLifecycle(ctx, event)
}

func (am *AdapterManager) OnVoiceLifecycle(ctx context.Context, event behavior.VoiceLifecycleEvent) {
	am.Voice.OnVoiceLifecycle(ctx, event)
}

func (am *AdapterManager) OnAffectChanged(ctx context.Context, characterID string, old, new behavior.AffectBehaviorSnapshot) {
	am.Affect.OnAffectChanged(ctx, characterID, old, new)
}

func (am *AdapterManager) OnDesktopGesture(ctx context.Context, event behavior.DesktopGestureEvent) {
	am.Desktop.OnDesktopGesture(ctx, event)
}

func (am *AdapterManager) OnPlaybackFeedback(ctx context.Context, feedback behavior.PlaybackFeedback) {
	am.Playback.OnPlaybackFeedback(ctx, feedback)
}

func (am *AdapterManager) OnActivityChanged(ctx context.Context, event ActivityChangeEvent) {
	am.Activity.OnActivityChanged(ctx, event)
}

func (am *AdapterManager) OnProactiveEvent(ctx context.Context, event ProactiveEvent) {
	am.Proactive.OnProactiveEvent(ctx, event)
}
