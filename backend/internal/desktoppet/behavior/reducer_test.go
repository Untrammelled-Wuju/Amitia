package behavior

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func reducerTestEvent(eventType string, at time.Time, payload map[string]interface{}) BehaviorEventEnvelope {
	raw, _ := json.Marshal(payload)
	return BehaviorEventEnvelope{
		EventID:       "event-1",
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    at,
		ReceivedAt:    at,
		UserID:        "user-1",
		CharacterID:   "character-1",
		PetInstanceID: "runtime-1",
		Origin:        OriginRuntime,
		Payload:       raw,
	}
}

func TestReducerDesktopGestureExpiresAndTriggersRecoveryDecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	reducer := NewReducer(clock)
	ctx := NewDefaultContext("user-1", "character-1")

	click := reducerTestEvent("runtime.pointer.clicked", now, map[string]interface{}{
		"gestureId": "gesture-1",
	})
	click.Sequence = 1
	next, result, err := reducer.Reduce(ctx, click)
	if err != nil {
		t.Fatalf("reduce click: %v", err)
	}
	if !result.NeedsDecision || next.DesktopGesture.CurrentGesture != "clicked" {
		t.Fatalf("click gesture not recorded: result=%+v gesture=%+v", result, next.DesktopGesture)
	}
	if next.DesktopGesture.ExpiresAt.IsZero() {
		t.Fatal("click gesture must carry an expiry")
	}

	clock.Advance(3 * time.Second)
	refresh := reducerTestEvent("voice.listening.activity", clock.Now(), map[string]interface{}{})
	refresh.SessionID = "other-session"
	cleared, result, err := reducer.Reduce(next, refresh)
	if err != nil {
		t.Fatalf("reduce lease check: %v", err)
	}
	if cleared.DesktopGesture.CurrentGesture != "" {
		t.Fatalf("expired gesture survived: %+v", cleared.DesktopGesture)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("gesture expiry must persist and trigger stable recovery: %+v", result)
	}
}

func TestReducerProgressRefreshesPersistWithoutNewDecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	reducer := NewReducer(clock)
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.ActiveTools["tool-1"] = ToolOperationState{
		OperationID:    "tool-1",
		LastActivityAt: now.Add(-time.Minute),
		LeaseExpiresAt: now.Add(time.Minute),
	}

	event := reducerTestEvent("agent.tool.progress", now, map[string]interface{}{
		"toolOperationId": "tool-1",
	})
	event.Origin = OriginTool
	next, result, err := reducer.Reduce(ctx, event)
	if err != nil {
		t.Fatalf("reduce tool progress: %v", err)
	}
	tool := next.ActiveTools["tool-1"]
	if !tool.LastActivityAt.Equal(now) || !tool.LeaseExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("tool lease was not refreshed: %+v", tool)
	}
	if !result.ContextChanged || result.NeedsDecision {
		t.Fatalf("progress must persist without redundant action decision: %+v", result)
	}

	ctx = next
	ctx.Voice = VoiceBehaviorState{
		SessionID:      "voice-1",
		State:          "listening",
		LeaseExpiresAt: now.Add(time.Second),
	}
	voiceEvent := reducerTestEvent("voice.listening.activity", now, map[string]interface{}{})
	voiceEvent.Origin = OriginVoice
	voiceEvent.SessionID = "voice-1"
	voiceNext, voiceResult, err := reducer.Reduce(ctx, voiceEvent)
	if err != nil {
		t.Fatalf("reduce voice activity: %v", err)
	}
	if !voiceNext.Voice.LeaseExpiresAt.Equal(now.Add(15 * time.Second)) {
		t.Fatalf("voice lease was not refreshed: %+v", voiceNext.Voice)
	}
	if !voiceResult.ContextChanged || voiceResult.NeedsDecision {
		t.Fatalf("voice activity must persist without redundant action decision: %+v", voiceResult)
	}
}

func TestReducerPlaybackStartedRestoresForegroundArbitrationMetadata(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	backendNow := now.Add(2 * time.Second)
	reducer := NewReducer(NewFakeClock(backendNow))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.DesktopGesture = DesktopGestureState{
		CurrentGesture: "clicked",
		GestureID:      "gesture-1",
		Sequence:       7,
		ExpiresAt:      now.Add(2 * time.Second),
	}

	event := reducerTestEvent("runtime.playback.action_started", now, map[string]interface{}{
		"decisionId":    "decision-1",
		"commandId":     "command-1",
		"actionKey":     "clicked",
		"semantic":      "gesture_click",
		"interruptible": true,
		"minimumPlayMs": 500,
		"maximumPlayMs": 2500,
	})
	next, result, err := reducer.Reduce(ctx, event)
	if err != nil {
		t.Fatalf("reduce playback start: %v", err)
	}
	if !result.ContextChanged || result.NeedsDecision {
		t.Fatalf("playback start must persist physical state without immediately replacing it: %+v", result)
	}
	if next.Foreground.Semantic != "gesture_click" || !next.Foreground.Interruptible {
		t.Fatalf("foreground metadata missing: %+v", next.Foreground)
	}
	if next.Foreground.StartedAt == nil || !next.Foreground.StartedAt.Equal(backendNow) {
		t.Fatalf("foreground arbitration start must use backend clock: %+v", next.Foreground.StartedAt)
	}
	if next.Foreground.MinPlayUntil == nil || !next.Foreground.MinPlayUntil.Equal(backendNow.Add(500*time.Millisecond)) {
		t.Fatalf("minimum play window missing: %+v", next.Foreground.MinPlayUntil)
	}
	if next.Foreground.MaxPlayUntil == nil || !next.Foreground.MaxPlayUntil.Equal(backendNow.Add(2500*time.Millisecond)) {
		t.Fatalf("maximum play window missing: %+v", next.Foreground.MaxPlayUntil)
	}
	if next.DesktopGesture.CurrentGesture != "" {
		t.Fatalf("one-shot click gesture must be consumed on physical start: %+v", next.DesktopGesture)
	}
}

func TestReducerReplacementStartWinsAndLateTerminalCannotClearIt(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Foreground = ForegroundActionState{
		DecisionID:    "old-decision",
		CommandID:     "old-command",
		Semantic:      "fallback_idle",
		ActionKey:     "idle_normal",
		Interruptible: true,
	}

	started := reducerTestEvent("runtime.playback.action_started", now, map[string]interface{}{
		"decisionId":    "new-decision",
		"commandId":     "new-command",
		"actionKey":     "speaking",
		"semantic":      "dialogue_speaking",
		"interruptible": true,
	})
	next, _, err := reducer.Reduce(ctx, started)
	if err != nil {
		t.Fatalf("replacement start: %v", err)
	}
	if next.Foreground.DecisionID != "new-decision" {
		t.Fatalf("replacement did not become foreground: %+v", next.Foreground)
	}

	late := reducerTestEvent("runtime.playback.action_interrupted", now.Add(time.Millisecond), map[string]interface{}{
		"decisionId": "old-decision",
		"commandId":  "old-command",
	})
	afterLate, result, err := reducer.Reduce(next, late)
	if err != nil {
		t.Fatalf("late terminal: %v", err)
	}
	if afterLate.Foreground.DecisionID != "new-decision" {
		t.Fatalf("late old terminal cleared replacement: %+v", afterLate.Foreground)
	}
	if result.NeedsDecision {
		t.Fatalf("stale terminal must not create a duplicate behavior decision: %+v", result)
	}
}

func TestReducerDragMoveRefreshesLeaseWithoutRestartingGestureDecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	reducer := NewReducer(clock)
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.DesktopGesture = DesktopGestureState{
		CurrentGesture: "drag",
		GestureID:      "drag-1",
		Sequence:       1,
		ExpiresAt:      now.Add(time.Second),
	}
	ctx.LastSourceRevisions["runtime:runtime-1"] = 1
	clock.Advance(750 * time.Millisecond)

	move := reducerTestEvent("runtime.drag.moved", clock.Now(), map[string]interface{}{
		"gestureId": "drag-1",
	})
	move.Sequence = 2
	next, result, err := reducer.Reduce(ctx, move)
	if err != nil {
		t.Fatalf("drag move: %v", err)
	}
	if next.DesktopGesture.CurrentGesture != "drag" || next.DesktopGesture.Sequence != 2 {
		t.Fatalf("drag lease state not refreshed: %+v", next.DesktopGesture)
	}
	if !next.DesktopGesture.ExpiresAt.After(now.Add(2 * time.Second)) {
		t.Fatalf("drag lease was not extended: %+v", next.DesktopGesture.ExpiresAt)
	}
	if !result.ContextChanged || result.NeedsDecision {
		t.Fatalf("drag move must persist lease without replaying drag action: %+v", result)
	}
}

func TestReducerDragCancelledClearsDragWithoutCreatingDropGesture(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.DesktopGesture = DesktopGestureState{
		CurrentGesture: "drag",
		GestureID:      "drag-1",
		Sequence:       10,
		ExpiresAt:      now.Add(2 * time.Second),
	}

	cancel := reducerTestEvent("runtime.drag.cancelled", now, map[string]interface{}{
		"gestureId": "drag-1",
	})
	cancel.Sequence = 11
	next, result, err := reducer.Reduce(ctx, cancel)
	if err != nil {
		t.Fatalf("drag cancel: %v", err)
	}
	if next.DesktopGesture.CurrentGesture != "" {
		t.Fatalf("cancelled drag must clear gesture without becoming dropped: %+v", next.DesktopGesture)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("drag cancellation must trigger stable recovery: %+v", result)
	}
}

func TestReducerStaleDragCancellationCannotCancelNewerDrag(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.DesktopGesture = DesktopGestureState{
		CurrentGesture: "drag",
		GestureID:      "drag-new",
		Sequence:       20,
		ExpiresAt:      now.Add(2 * time.Second),
	}

	cancel := reducerTestEvent("runtime.drag.cancelled", now, map[string]interface{}{
		"gestureId": "drag-old",
	})
	cancel.Sequence = 21
	next, result, err := reducer.Reduce(ctx, cancel)
	if err != nil {
		t.Fatalf("stale drag cancel: %v", err)
	}
	if next.DesktopGesture.CurrentGesture != "drag" || next.DesktopGesture.GestureID != "drag-new" {
		t.Fatalf("stale cancellation cleared newer drag: %+v", next.DesktopGesture)
	}
	if result.NeedsDecision {
		t.Fatalf("ignored stale cancellation must not create a behavior decision: %+v", result)
	}
}

func TestReducerVoiceSessionStartedCarriesExpiryLease(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	reducer := NewReducer(clock)
	ctx := NewDefaultContext("user-1", "character-1")

	started := reducerTestEvent("voice.session.started", now, map[string]interface{}{})
	started.Origin = OriginVoice
	started.SessionID = "voice-1"
	next, result, err := reducer.Reduce(ctx, started)
	if err != nil {
		t.Fatalf("voice session start: %v", err)
	}
	if next.Voice.SessionID != "voice-1" || next.Voice.State != "listening" {
		t.Fatalf("voice session state missing: %+v", next.Voice)
	}
	if !next.Voice.LeaseExpiresAt.Equal(now.Add(15 * time.Second)) {
		t.Fatalf("voice session must have fail-safe lease: %+v", next.Voice.LeaseExpiresAt)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("voice session start must persist and decide: %+v", result)
	}

	clock.Advance(16 * time.Second)
	refresh := reducerTestEvent("runtime.connected", clock.Now(), map[string]interface{}{})
	cleared, result, err := reducer.Reduce(next, refresh)
	if err != nil {
		t.Fatalf("voice lease expiry: %v", err)
	}
	if cleared.Voice.State != "" || cleared.Voice.SessionID != "" {
		t.Fatalf("expired voice session survived: %+v", cleared.Voice)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("expired voice session must trigger stable recovery: %+v", result)
	}
}

func TestReducerActivityVersionAndConfidenceUpdatesAreNotDropped(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Stable = StableBehaviorState{
		ActivityKey:        "work",
		ActivitySource:     "calendar",
		ActivityConfidence: 0.4,
		ActivityVersion:    "v1",
	}

	updated := reducerTestEvent("character.activity.changed", now, map[string]interface{}{
		"activityKey": "work",
		"source":      "calendar",
		"confidence":  0.9,
		"version":     "v2",
	})
	updated.Origin = OriginActivity
	next, result, err := reducer.Reduce(ctx, updated)
	if err != nil {
		t.Fatalf("activity update: %v", err)
	}
	if next.Stable.ActivityVersion != "v2" || next.Stable.ActivityConfidence != 0.9 {
		t.Fatalf("activity version/confidence update was dropped: %+v", next.Stable)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("new activity version must be persisted and re-evaluated: %+v", result)
	}

	duplicate, duplicateResult, err := reducer.Reduce(next, updated)
	if err != nil {
		t.Fatalf("duplicate activity update: %v", err)
	}
	if duplicate.Stable.ActivityVersion != "v2" || duplicate.Stable.ActivityConfidence != 0.9 {
		t.Fatalf("duplicate changed stable activity: %+v", duplicate.Stable)
	}
	if duplicateResult.ContextChanged || duplicateResult.NeedsDecision {
		t.Fatalf("identical activity version should be ignored: %+v", duplicateResult)
	}
}

func TestReducerVoiceSessionStartedRecoversLegacyLeaseLessState(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Voice = VoiceBehaviorState{SessionID: "voice-1", State: "listening"}

	started := reducerTestEvent("voice.session.started", now, map[string]interface{}{})
	started.Origin = OriginVoice
	started.SessionID = "voice-1"
	next, result, err := reducer.Reduce(ctx, started)
	if err != nil {
		t.Fatalf("voice session recovery: %v", err)
	}
	if next.Voice.LeaseExpiresAt.IsZero() || !next.Voice.LeaseExpiresAt.Equal(now.Add(15*time.Second)) {
		t.Fatalf("legacy lease-less voice state was not repaired: %+v", next.Voice)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("repaired voice state must persist and re-evaluate: %+v", result)
	}
}

func TestReducerHoverRefreshExtendsLeaseWithoutDecisionStorm(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	reducer := NewReducer(clock)
	ctx := NewDefaultContext("user-1", "character-1")

	first := reducerTestEvent("runtime.pointer.hovered", now, map[string]interface{}{
		"gestureId": "hover-1",
	})
	first.Sequence = 1
	next, firstResult, err := reducer.Reduce(ctx, first)
	if err != nil {
		t.Fatalf("first hover: %v", err)
	}
	if !firstResult.NeedsDecision {
		t.Fatalf("first hover must trigger behavior: %+v", firstResult)
	}
	firstExpiry := next.DesktopGesture.ExpiresAt

	clock.Advance(200 * time.Millisecond)
	refresh := reducerTestEvent("runtime.pointer.hovered", clock.Now(), map[string]interface{}{
		"gestureId": "hover-2",
	})
	refresh.Sequence = 2
	refreshed, refreshResult, err := reducer.Reduce(next, refresh)
	if err != nil {
		t.Fatalf("hover refresh: %v", err)
	}
	if !refreshResult.ContextChanged || refreshResult.NeedsDecision {
		t.Fatalf("active hover refresh must persist without another decision: %+v", refreshResult)
	}
	if !refreshed.DesktopGesture.ExpiresAt.After(firstExpiry) {
		t.Fatalf("hover refresh did not extend lease: first=%v next=%v", firstExpiry, refreshed.DesktopGesture.ExpiresAt)
	}
}

func TestReducerDeduplicatesEventIdentityAcrossEphemeralDispatch(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Voice = VoiceBehaviorState{
		SessionID:      "voice-1",
		State:          "listening",
		LeaseExpiresAt: now.Add(time.Second),
	}

	event := reducerTestEvent("voice.listening.activity", now, map[string]interface{}{
		"level": 0.8,
	})
	event.Origin = OriginVoice
	event.SessionID = "voice-1"
	event.DedupKey = "voice-1:activity:42"

	next, firstResult, err := reducer.Reduce(ctx, event)
	if err != nil {
		t.Fatalf("first ephemeral event: %v", err)
	}
	if !firstResult.ContextChanged || len(next.RecentEventKeys) != 1 {
		t.Fatalf("first event identity must be persisted: result=%+v keys=%v", firstResult, next.RecentEventKeys)
	}

	duplicate, duplicateResult, err := reducer.Reduce(next, event)
	if err != nil {
		t.Fatalf("duplicate ephemeral event: %v", err)
	}
	if !duplicateResult.IsDuplicate || duplicateResult.ContextChanged || duplicateResult.NeedsDecision {
		t.Fatalf("duplicate event must be ignored before mutation: %+v", duplicateResult)
	}
	if duplicate.Revision != next.Revision || len(duplicate.RecentEventKeys) != 1 {
		t.Fatalf("duplicate event changed persisted context: before=%+v after=%+v", next, duplicate)
	}
}

func TestReducerRecentEventIdentityWindowIsBounded(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")

	for i := 0; i < MaxRecentEventKeys+8; i++ {
		event := reducerTestEvent("manual.action.requested", now, map[string]interface{}{})
		event.EventID = fmt.Sprintf("event-%d", i)
		event.DedupKey = fmt.Sprintf("manual-%d", i)
		next, _, err := reducer.Reduce(ctx, event)
		if err != nil {
			t.Fatalf("reduce event %d: %v", i, err)
		}
		ctx = next
	}

	if len(ctx.RecentEventKeys) != MaxRecentEventKeys {
		t.Fatalf("dedup window must stay bounded: got=%d want=%d", len(ctx.RecentEventKeys), MaxRecentEventKeys)
	}
	if ctx.RecentEventKeys[0] == "manual.action.requested\x00manual-0" {
		t.Fatal("oldest event identity was not evicted")
	}
}

func interactionReducerEvent(eventID, eventType, interactionID string, at time.Time, statusVersion int64) BehaviorEventEnvelope {
	event := reducerTestEvent(eventType, at, map[string]interface{}{
		"interactionStatusVersion": statusVersion,
		"statusVersion":            statusVersion,
	})
	event.EventID = eventID
	event.DedupKey = eventID
	event.Origin = OriginChat
	event.InteractionID = interactionID
	return event
}

func TestReducerCompletedInteractionAllowsNewerReceivedInteraction(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")

	first, _, err := reducer.Reduce(ctx, interactionReducerEvent("recv-a", "chat.message.received", "interaction-a", now, 1))
	if err != nil {
		t.Fatalf("first received: %v", err)
	}
	completed, _, err := reducer.Reduce(first, interactionReducerEvent("done-a", "chat.response.completed", "interaction-a", now.Add(time.Second), 5))
	if err != nil {
		t.Fatalf("complete first interaction: %v", err)
	}
	if completed.Transient.InteractionID != "interaction-a" || completed.Transient.InteractionPhase != "completed" {
		t.Fatalf("terminal interaction state not preserved: %+v", completed.Transient)
	}

	next, result, err := reducer.Reduce(completed, interactionReducerEvent("recv-b", "chat.message.received", "interaction-b", now.Add(2*time.Second), 1))
	if err != nil {
		t.Fatalf("second received: %v", err)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("new interaction must replace terminal foreground state: %+v", result)
	}
	if next.Transient.InteractionID != "interaction-b" || next.Transient.InteractionPhase != "received" {
		t.Fatalf("new interaction was swallowed by terminal phase: %+v", next.Transient)
	}
}

func TestReducerLateConcurrentLifecycleCannotStealForegroundInteraction(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")

	first, _, err := reducer.Reduce(ctx, interactionReducerEvent("recv-a", "chat.message.received", "interaction-a", now, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := reducer.Reduce(first, interactionReducerEvent("recv-b", "chat.message.received", "interaction-b", now.Add(time.Second), 1))
	if err != nil {
		t.Fatal(err)
	}
	late, result, err := reducer.Reduce(second, interactionReducerEvent("start-a-late", "chat.response.started", "interaction-a", now.Add(2*time.Second), 3))
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsDecision {
		t.Fatalf("late lifecycle event must not trigger a foreground behavior decision: %+v", result)
	}
	if late.Transient.InteractionID != "interaction-b" || late.Transient.InteractionPhase != "received" {
		t.Fatalf("late interaction stole foreground state: %+v", late.Transient)
	}
}

func TestReducerFailureClearsOnlyMatchingInteractionToolsAndKeepsFailedPhase(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "response_started",
		InteractionStartedAt: now,
		StatusVersion:        3,
	}
	ctx.ActiveTools["tool-a"] = ToolOperationState{OperationID: "tool-a", InteractionID: "interaction-a"}
	ctx.ActiveTools["tool-b"] = ToolOperationState{OperationID: "tool-b", InteractionID: "interaction-b"}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("fail-a", "chat.response.failed", "interaction-a", now.Add(time.Second), 4))
	if err != nil {
		t.Fatal(err)
	}
	if !result.ContextChanged || !result.NeedsDecision {
		t.Fatalf("failure must persist and trigger behavior resolution: %+v", result)
	}
	if next.Transient.InteractionID != "interaction-a" || next.Transient.InteractionPhase != "failed" {
		t.Fatalf("failed phase was cleared before resolver could observe it: %+v", next.Transient)
	}
	if _, ok := next.ActiveTools["tool-a"]; ok {
		t.Fatal("failed interaction retained its own active tool")
	}
	if _, ok := next.ActiveTools["tool-b"]; !ok {
		t.Fatal("failed interaction cleared a concurrent interaction tool")
	}
}

func TestReducerToolLifecycleRejectsMismatchedInteractionOwnership(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.ActiveTools["tool-a"] = ToolOperationState{
		OperationID:    "tool-a",
		InteractionID:  "interaction-a",
		LastActivityAt: now,
		LeaseExpiresAt: now.Add(time.Minute),
	}

	progress := reducerTestEvent("agent.tool.progress", now.Add(time.Second), map[string]interface{}{"toolOperationId": "tool-a"})
	progress.EventID = "progress-wrong-owner"
	progress.DedupKey = progress.EventID
	progress.Origin = OriginTool
	progress.InteractionID = "interaction-b"
	afterProgress, progressResult, err := reducer.Reduce(ctx, progress)
	if err != nil {
		t.Fatal(err)
	}
	if progressResult.NeedsDecision || !afterProgress.ActiveTools["tool-a"].LastActivityAt.Equal(now) {
		t.Fatalf("foreign interaction refreshed tool ownership: result=%+v tool=%+v", progressResult, afterProgress.ActiveTools["tool-a"])
	}

	completed := reducerTestEvent("agent.tool.completed", now.Add(2*time.Second), map[string]interface{}{"toolOperationId": "tool-a"})
	completed.EventID = "complete-wrong-owner"
	completed.DedupKey = completed.EventID
	completed.Origin = OriginTool
	completed.InteractionID = "interaction-b"
	afterComplete, _, err := reducer.Reduce(afterProgress, completed)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := afterComplete.ActiveTools["tool-a"]; !ok {
		t.Fatal("foreign interaction completed another interaction's tool")
	}
}

func TestResolverCanObserveFailedInteractionPhase(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	resolver := NewResolver(NewFakeClock(now), nil)
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{InteractionID: "interaction-a", InteractionPhase: "failed"}
	event := interactionReducerEvent("fail-a", "chat.response.failed", "interaction-a", now, 4)

	candidates := resolver.generateCandidates(&ctx, event, map[string]bool{"confused": true, "thinking": true}, "installation-1")
	found := false
	for _, candidate := range candidates {
		if candidate.Semantic == "emotion_confused" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("failed interaction did not produce emotion_confused candidate: %+v", candidates)
	}
}

func TestReducerTerminalInteractionCannotFlipToAnotherTerminalState(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "failed",
		InteractionStartedAt: now,
		StatusVersion:        8,
	}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("late-complete", "chat.response.completed", "interaction-a", now.Add(time.Second), 9))
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsDecision {
		t.Fatalf("terminal flip triggered a decision: %+v", result)
	}
	if next.Transient.InteractionPhase != "failed" {
		t.Fatalf("late terminal event flipped failed state to %q", next.Transient.InteractionPhase)
	}
}

func TestReducerCancelledInteractionKeepsTerminalTombstoneAndRecoversToIdle(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "response_started",
		InteractionStartedAt: now,
		StatusVersion:        3,
	}

	cancelled, result, err := reducer.Reduce(ctx, interactionReducerEvent("cancel-a", "chat.response.cancelled", "interaction-a", now.Add(time.Second), 4))
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsDecision || cancelled.Transient.InteractionPhase != "cancelled" || cancelled.Transient.InteractionID != "interaction-a" {
		t.Fatalf("cancelled interaction did not retain terminal tombstone: result=%+v transient=%+v", result, cancelled.Transient)
	}

	late, lateResult, err := reducer.Reduce(cancelled, interactionReducerEvent("late-start-a", "chat.response.started", "interaction-a", now.Add(2*time.Second), 5))
	if err != nil {
		t.Fatal(err)
	}
	if lateResult.NeedsDecision || late.Transient.InteractionPhase != "cancelled" {
		t.Fatalf("late lifecycle resurrected cancelled interaction: result=%+v transient=%+v", lateResult, late.Transient)
	}

	resolver := NewResolver(NewFakeClock(now), nil)
	candidates := resolver.generateCandidates(&cancelled, interactionReducerEvent("cancel-resolve", "chat.response.cancelled", "interaction-a", now, 4), map[string]bool{"idle_breathing": true}, "installation-1")
	foundIdle := false
	for _, candidate := range candidates {
		if candidate.Semantic == "calm_idle" {
			foundIdle = true
			break
		}
	}
	if !foundIdle {
		t.Fatalf("cancelled interaction did not recover to calm idle: %+v", candidates)
	}
}

func TestReducerRejectsLowerStatusVersionEvenWhenPhaseWouldAdvance(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "received",
		InteractionStartedAt: now,
		StatusVersion:        5,
	}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("stale-context", "chat.context.loading", "interaction-a", now.Add(time.Second), 4))
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsDecision {
		t.Fatalf("stale status version triggered behavior: %+v", result)
	}
	if next.Transient.InteractionPhase != "received" || next.Transient.StatusVersion != 5 {
		t.Fatalf("stale status version advanced interaction: %+v", next.Transient)
	}
}

func TestReducerCompletedInteractionReportsActiveToolsLayerWhenItClearsOwnedTools(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "response_ready",
		InteractionStartedAt: now,
		StatusVersion:        4,
	}
	ctx.ActiveTools["tool-a"] = ToolOperationState{OperationID: "tool-a", InteractionID: "interaction-a"}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("done-with-tool", "chat.response.completed", "interaction-a", now.Add(time.Second), 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := next.ActiveTools["tool-a"]; ok {
		t.Fatal("completed interaction retained an owned tool")
	}
	foundLayer := false
	for _, layer := range result.LayersChanged {
		if layer == "activeTools" {
			foundLayer = true
			break
		}
	}
	if !foundLayer {
		t.Fatalf("active tool removal was not reported in changed layers: %+v", result.LayersChanged)
	}
}

func TestResolverTerminalTombstoneDoesNotSuppressStableBehavior(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	resolver := NewResolver(NewFakeClock(now), nil)
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:    "interaction-a",
		InteractionPhase: "completed",
	}
	ctx.Stable.ActivityKey = "work"
	event := interactionReducerEvent("after-complete", "character.activity.changed", "interaction-a", now, 5)

	candidates := resolver.generateCandidates(&ctx, event, map[string]bool{
		"idle_breathing": true,
		"work":           true,
		"study":          true,
		"thinking":       true,
	}, "installation-1")
	foundWork := false
	for _, candidate := range candidates {
		if candidate.Semantic == "working" && candidate.SourceLayer == "stable" {
			foundWork = true
			break
		}
	}
	if !foundWork {
		t.Fatalf("terminal tombstone suppressed stable activity candidates: %+v", candidates)
	}
}

func TestReducerRejectsEqualStatusVersionForDifferentPhase(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-a",
		InteractionPhase:     "received",
		InteractionStartedAt: now,
		StatusVersion:        5,
	}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("same-version-context", "chat.context.loading", "interaction-a", now.Add(time.Second), 5))
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsDecision {
		t.Fatalf("equal status version advanced a different phase: %+v", result)
	}
	if next.Transient.InteractionPhase != "received" || next.Transient.StatusVersion != 5 {
		t.Fatalf("equal status version advanced interaction: %+v", next.Transient)
	}
}

func TestReducerBackgroundTerminalCleansOwnedToolsWithoutForegroundDecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-b",
		InteractionPhase:     "response_started",
		InteractionStartedAt: now.Add(time.Second),
		StatusVersion:        3,
	}
	ctx.ActiveTools["tool-a"] = ToolOperationState{OperationID: "tool-a", InteractionID: "interaction-a"}
	ctx.ActiveTools["tool-b"] = ToolOperationState{OperationID: "tool-b", InteractionID: "interaction-b"}

	next, result, err := reducer.Reduce(ctx, interactionReducerEvent("complete-a-background", "chat.response.completed", "interaction-a", now.Add(2*time.Second), 8))
	if err != nil {
		t.Fatal(err)
	}
	if !result.ContextChanged || result.NeedsDecision {
		t.Fatalf("background terminal cleanup must persist without re-arbitrating foreground: %+v", result)
	}
	if next.Transient.InteractionID != "interaction-b" || next.Transient.InteractionPhase != "response_started" {
		t.Fatalf("background terminal changed foreground transient: %+v", next.Transient)
	}
	if _, ok := next.ActiveTools["tool-a"]; ok {
		t.Fatal("background terminal retained its owned tool")
	}
	if _, ok := next.ActiveTools["tool-b"]; !ok {
		t.Fatal("background terminal cleared foreground interaction tool")
	}
	foundToolsLayer := false
	for _, layer := range result.LayersChanged {
		if layer == "activeTools" {
			foundToolsLayer = true
		}
		if layer == "transient" {
			t.Fatalf("background terminal incorrectly reported transient mutation: %+v", result.LayersChanged)
		}
	}
	if !foundToolsLayer {
		t.Fatalf("background tool cleanup omitted activeTools layer: %+v", result.LayersChanged)
	}
}

func TestResolverIgnoresBackgroundInteractionToolsWhileForegroundInteractionIsActive(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	resolver := NewResolver(NewFakeClock(now), nil)
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-b",
		InteractionPhase:     "received",
		InteractionStartedAt: now,
		StatusVersion:        1,
	}
	ctx.ActiveTools["tool-a"] = ToolOperationState{
		OperationID:   "tool-a",
		InteractionID: "interaction-a",
		DisplayClass:  "research",
	}

	event := interactionReducerEvent("recv-b-resolve", "chat.message.received", "interaction-b", now, 1)
	candidates := resolver.generateCandidates(&ctx, event, map[string]bool{
		"listening": true,
		"work":      true,
		"thinking":  true,
	}, "installation-1")

	foundListening := false
	for _, candidate := range candidates {
		if candidate.Semantic == "working" && candidate.SourceLayer == "tool" {
			t.Fatalf("background tool suppressed foreground interaction behavior: %+v", candidates)
		}
		if candidate.Semantic == "dialogue_listening" {
			foundListening = true
		}
	}
	if !foundListening {
		t.Fatalf("foreground received phase did not produce listening candidate: %+v", candidates)
	}
}

func TestReducerForegroundTerminalRequestsAuthoritativeSnapshotSync(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	reducer := NewReducer(NewFakeClock(now))
	ctx := NewDefaultContext("user-1", "character-1")
	ctx.Transient = TransientBehaviorState{
		InteractionID:        "interaction-b",
		InteractionPhase:     "response_ready",
		InteractionStartedAt: now,
		StatusVersion:        4,
	}

	_, result, err := reducer.Reduce(ctx, interactionReducerEvent(
		"complete-b",
		"chat.response.completed",
		"interaction-b",
		now.Add(time.Second),
		5,
	))
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsSnapshotSync {
		t.Fatalf("foreground terminal must request snapshot sync so hidden concurrent interactions can be promoted: %+v", result)
	}
}
