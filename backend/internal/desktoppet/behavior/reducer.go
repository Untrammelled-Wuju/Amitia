package behavior

import (
	"encoding/json"
	"time"
)

type Reducer struct {
	clock Clock
}

func NewReducer(clock Clock) *Reducer {
	if clock == nil {
		clock = NewRealClock()
	}
	return &Reducer{clock: clock}
}

func (r *Reducer) Reduce(current BehaviorContextSnapshot, event BehaviorEventEnvelope) (BehaviorContextSnapshot, ReduceResult, error) {
	next := current.Copy()
	result := ReduceResult{}

	if event.CharacterID == "" {
		result.IsExpired = true
		result.Reason = "missing characterId"
		return next, result, NewBehaviorError(ErrCodeEventSchemaInvalid, "event missing characterId")
	}

	if event.UserID != "" && next.UserID == "" {
		next.UserID = event.UserID
	}

	now := r.clock.Now()
	if event.ExpiresAt != nil && now.After(*event.ExpiresAt) {
		result.IsExpired = true
		result.Reason = "event expired"
		return next, result, nil
	}

	if r.isDuplicateEvent(next, event) {
		result.IsDuplicate = true
		result.Reason = "duplicate event"
		return next, result, nil
	}

	sequenceKey := behaviorSourceSequenceKey(event)
	if sequenceKey != "" && next.LastSourceRevisions != nil {
		if last := next.LastSourceRevisions[sequenceKey]; last > 0 && event.Sequence <= last {
			result.IsOutOfOrder = true
			result.Reason = "source sequence is stale"
			return next, result, nil
		}
	}

	changed := false
	contextOnlyChanged := false
	layersChanged := []string{}

	switch event.EventType {
	case "chat.message.received", "interaction.received":
		changed = r.reduceChatMessageReceived(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "chat.context.loading", "interaction.context_loading":
		changed = r.reduceContextLoading(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "chat.response.started", "interaction.response_started":
		changed = r.reduceResponseStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "chat.response.ready", "interaction.response_ready":
		changed = r.reduceResponseReady(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "chat.response.completed", "interaction.completed":
		changed = r.reduceResponseCompleted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient", "foreground")
		}

	case "chat.response.failed", "interaction.failed":
		changed = r.reduceResponseFailed(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient", "activeTools", "foreground")
		}

	case "chat.response.cancelled", "interaction.cancelled":
		changed = r.reduceResponseCancelled(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient", "activeTools", "foreground")
		}

	case "delivery.started":
		changed = r.reduceDeliveryStarted(&next, event)

	case "delivery.completed":
		changed = r.reduceDeliveryCompleted(&next, event)

	case "delivery.failed":
		changed = r.reduceDeliveryFailed(&next, event)

	case "agent.tool.started":
		changed = r.reduceToolStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "activeTools")
		}

	case "agent.tool.progress":
		contextOnlyChanged = r.reduceToolProgress(&next, event)
		if contextOnlyChanged {
			layersChanged = append(layersChanged, "activeTools")
		}

	case "agent.tool.completed":
		changed = r.reduceToolCompleted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "activeTools")
		}

	case "agent.tool.failed":
		changed = r.reduceToolFailed(&next, event)
		if changed {
			layersChanged = append(layersChanged, "activeTools")
		}

	case "agent.tool.cancelled":
		changed = r.reduceToolCancelled(&next, event)
		if changed {
			layersChanged = append(layersChanged, "activeTools")
		}

	case "voice.session.started":
		changed = r.reduceVoiceSessionStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.listening.started":
		changed = r.reduceVoiceListeningStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.listening.activity":
		contextOnlyChanged = r.reduceVoiceListeningActivity(&next, event)
		if contextOnlyChanged {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.listening.ended":
		changed = r.reduceVoiceListeningEnded(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.processing.started":
		changed = r.reduceVoiceProcessingStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.speaking.started":
		changed = r.reduceVoiceSpeakingStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.speaking.ended":
		changed = r.reduceVoiceSpeakingEnded(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.turn.interrupted":
		changed = r.reduceVoiceTurnInterrupted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "voice.session.ended":
		changed = r.reduceVoiceSessionEnded(&next, event)
		if changed {
			layersChanged = append(layersChanged, "voice")
		}

	case "character.affect.changed":
		changed = r.reduceAffectChanged(&next, event)
		if changed {
			layersChanged = append(layersChanged, "stable")
		}

	case "character.activity.changed":
		changed = r.reduceActivityChanged(&next, event)
		if changed {
			layersChanged = append(layersChanged, "stable")
		}

	case "character.time_period.changed":
		changed = r.reduceTimePeriodChanged(&next, event)
		if changed {
			layersChanged = append(layersChanged, "stable")
		}

	case "proactive.message.started":
		changed = r.reduceProactiveStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "proactive.message.completed":
		changed = r.reduceProactiveCompleted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "proactive.message.suppressed":
		changed = r.reduceProactiveSuppressed(&next, event)
		if changed {
			layersChanged = append(layersChanged, "transient")
		}

	case "runtime.pointer.clicked":
		changed = r.reduceDesktopClicked(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.pointer.double_clicked":
		changed = r.reduceDesktopDoubleClicked(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.pointer.hovered":
		hoverWasActive := next.DesktopGesture.CurrentGesture == "hovered" &&
			!next.DesktopGesture.ExpiresAt.IsZero() &&
			now.Before(next.DesktopGesture.ExpiresAt)
		hoverUpdated := r.reduceDesktopHovered(&next, event)
		if hoverUpdated {
			if hoverWasActive {
				// Pointer movement while already hovered is a lease refresh. Re-running
				// behavior arbitration for every mousemove creates a decision/write storm.
				contextOnlyChanged = true
			} else {
				changed = true
			}
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.started":
		changed = r.reduceDesktopDragStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.moved":
		contextOnlyChanged = r.reduceDesktopDragMoved(&next, event)
		if contextOnlyChanged {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.completed":
		changed = r.reduceDesktopDragEnded(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.cancelled":
		changed = r.reduceDesktopDragCancelled(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.pet.fall.started":
		changed = r.reduceDesktopFallStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.pet.edge.reached":
		changed = r.reduceDesktopEdgeReached(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.pet.interacted":
		changed = r.reduceDesktopInteracted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.playback.action_started":
		previousGesture := next.DesktopGesture.CurrentGesture
		contextOnlyChanged = r.reducePlaybackStarted(&next, event)
		if contextOnlyChanged {
			layersChanged = append(layersChanged, "foreground")
			if previousGesture != next.DesktopGesture.CurrentGesture {
				layersChanged = append(layersChanged, "desktopGesture")
			}
		}

	case "runtime.playback.action_completed":
		changed = r.reducePlaybackCompleted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "foreground")
		}

	case "runtime.playback.action_interrupted":
		changed = r.reducePlaybackInterrupted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "foreground")
		}

	case "runtime.playback.action_failed":
		changed = r.reducePlaybackFailed(&next, event)
		if changed {
			layersChanged = append(layersChanged, "foreground")
		}

	case "runtime.connected":
		result.NeedsSnapshotSync = true
		changed = true

	case "runtime.disconnected":
		next.Desired = DesiredBehaviorState{
			Semantic:        next.Foreground.Semantic,
			PreferredAction: next.Foreground.ActionKey,
			SourceLayer:     "foreground",
		}
		changed = true
		layersChanged = append(layersChanged, "desired")

	case "installation.active.changed":
		next.Foreground = ForegroundActionState{}
		changed = true
		layersChanged = append(layersChanged, "foreground")

	case "manual.action.requested":
		changed = true
		layersChanged = append(layersChanged, "transient")

	default:
		result.Reason = "unknown event type"
		return next, result, nil
	}

	leaseLayers := r.checkLeases(&next, now)
	if len(leaseLayers) > 0 {
		changed = true
		for _, layer := range leaseLayers {
			layersChanged = appendLayerOnce(layersChanged, layer)
		}
	}

	sequenceAdvanced := false
	if sequenceKey != "" {
		if next.LastSourceRevisions == nil {
			next.LastSourceRevisions = make(map[string]int64)
		}
		if event.Sequence > next.LastSourceRevisions[sequenceKey] {
			next.LastSourceRevisions[sequenceKey] = event.Sequence
			sequenceAdvanced = true
		}
	}

	contextChanged := changed || contextOnlyChanged || sequenceAdvanced
	result.ContextChanged = contextChanged
	result.LayersChanged = layersChanged
	// Advancing only the source sequence is persistence metadata, not a reason
	// to produce another animation decision.
	result.NeedsDecision = changed && !result.IsDuplicate && !result.IsExpired
	next.UpdatedAt = now
	if contextChanged {
		next.Revision++
	}

	return next, result, nil
}

func behaviorSourceSequenceKey(event BehaviorEventEnvelope) string {
	if event.Sequence <= 0 {
		return ""
	}
	switch event.Origin {
	case OriginDesktop, OriginPlayback, OriginRuntime:
	default:
		return ""
	}

	// Runtime V2 sequence numbers belong to a runtime session. Prefer the
	// session identity so a reconnect that legitimately restarts a sequence
	// cannot be rejected by a cursor persisted for the previous session.
	if event.SessionID != "" {
		return "runtime:" + event.SessionID
	}

	// Adapter-originated playback feedback may use a per-command sequence
	// instead of Runtime V2's session-global sequence. Scope that cursor to the
	// command/decision so the next playback is allowed to restart at sequence 1.
	if event.Origin == OriginPlayback {
		payload := parsePayload(event.Payload)
		if commandID, _ := payload["commandId"].(string); commandID != "" {
			return "playback-command:" + commandID
		}
		if decisionID, _ := payload["decisionId"].(string); decisionID != "" {
			return "playback-decision:" + decisionID
		}
	}

	streamID := event.PetInstanceID
	if streamID == "" {
		streamID = event.InstallationID
	}
	if streamID == "" {
		return ""
	}
	return "runtime:" + streamID
}

func (r *Reducer) isDuplicateEvent(ctx BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	for _, rec := range ctx.RecentSemantics {
		_ = rec
	}
	return false
}

func appendLayerOnce(layers []string, layer string) []string {
	for _, existing := range layers {
		if existing == layer {
			return layers
		}
	}
	return append(layers, layer)
}

func (r *Reducer) checkLeases(ctx *BehaviorContextSnapshot, now time.Time) []string {
	layersChanged := []string{}

	for opID, tool := range ctx.ActiveTools {
		if !tool.LeaseExpiresAt.IsZero() && !now.Before(tool.LeaseExpiresAt) {
			delete(ctx.ActiveTools, opID)
			layersChanged = appendLayerOnce(layersChanged, "activeTools")
		}
	}

	if ctx.Voice.State != "" && !ctx.Voice.LeaseExpiresAt.IsZero() && !now.Before(ctx.Voice.LeaseExpiresAt) {
		ctx.Voice = VoiceBehaviorState{}
		layersChanged = appendLayerOnce(layersChanged, "voice")
	}

	if ctx.DesktopGesture.CurrentGesture != "" && !ctx.DesktopGesture.ExpiresAt.IsZero() && !now.Before(ctx.DesktopGesture.ExpiresAt) {
		ctx.DesktopGesture = DesktopGestureState{}
		layersChanged = appendLayerOnce(layersChanged, "desktopGesture")
	}

	return layersChanged
}

func phaseRank(phase string) int {
	ranks := map[string]int{
		"received":         1,
		"context_loading":  2,
		"response_started": 3,
		"response_ready":   4,
		"completed":        5,
		"failed":           5,
		"cancelled":        5,
	}
	rank, ok := ranks[phase]
	if !ok {
		return 0
	}
	return rank
}

func (r *Reducer) reduceChatMessageReceived(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	statusVersion, _ := payload["interactionStatusVersion"].(float64)
	sv := int64(statusVersion)

	if ctx.Transient.InteractionID != "" && ctx.Transient.InteractionID != event.InteractionID {
		if phaseRank(ctx.Transient.InteractionPhase) >= phaseRank("completed") {
			ctx.Transient = TransientBehaviorState{}
		}
	}

	if phaseRank(ctx.Transient.InteractionPhase) > phaseRank("received") {
		return false
	}

	ctx.Transient.InteractionID = event.InteractionID
	ctx.Transient.InteractionPhase = "received"
	if sv > 0 {
		ctx.Transient.StatusVersion = sv
	}
	if event.ConversationID != "" {
	}
	return true
}

func (r *Reducer) reduceContextLoading(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	statusVersion, _ := payload["statusVersion"].(float64)
	sv := int64(statusVersion)

	if ctx.Transient.InteractionID == event.InteractionID && phaseRank(ctx.Transient.InteractionPhase) > phaseRank("context_loading") {
		if sv <= ctx.Transient.StatusVersion {
			return false
		}
	}

	ctx.Transient.InteractionID = event.InteractionID
	ctx.Transient.InteractionPhase = "context_loading"
	if sv > 0 {
		ctx.Transient.StatusVersion = sv
	}
	return true
}

func (r *Reducer) reduceResponseStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	statusVersion, _ := payload["statusVersion"].(float64)
	sv := int64(statusVersion)

	if ctx.Transient.InteractionID == event.InteractionID && phaseRank(ctx.Transient.InteractionPhase) > phaseRank("response_started") {
		if sv <= ctx.Transient.StatusVersion {
			return false
		}
	}

	ctx.Transient.InteractionID = event.InteractionID
	ctx.Transient.InteractionPhase = "response_started"
	if sv > 0 {
		ctx.Transient.StatusVersion = sv
	}
	return true
}

func (r *Reducer) reduceResponseReady(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	statusVersion, _ := payload["statusVersion"].(float64)
	sv := int64(statusVersion)

	if ctx.Transient.InteractionID == event.InteractionID && phaseRank(ctx.Transient.InteractionPhase) > phaseRank("response_ready") {
		if sv <= ctx.Transient.StatusVersion {
			return false
		}
	}

	ctx.Transient.InteractionID = event.InteractionID
	ctx.Transient.InteractionPhase = "response_ready"
	if sv > 0 {
		ctx.Transient.StatusVersion = sv
	}
	return true
}

func (r *Reducer) reduceResponseCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Transient.InteractionID != event.InteractionID {
		return false
	}
	ctx.Transient.InteractionPhase = "completed"
	ctx.Transient.InteractionID = ""
	return true
}

func (r *Reducer) reduceResponseFailed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Transient.InteractionID != event.InteractionID && ctx.Transient.InteractionID != "" {
		return false
	}
	ctx.Transient = TransientBehaviorState{}
	for opID := range ctx.ActiveTools {
		if opID != "" {
			delete(ctx.ActiveTools, opID)
		}
	}
	return true
}

func (r *Reducer) reduceResponseCancelled(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Transient.InteractionID != event.InteractionID && ctx.Transient.InteractionID != "" {
		return false
	}
	ctx.Transient = TransientBehaviorState{}
	for opID := range ctx.ActiveTools {
		delete(ctx.ActiveTools, opID)
	}
	ctx.Transient.ProactiveID = ""
	ctx.Transient.ProactiveIntent = ""
	return true
}

func (r *Reducer) reduceDeliveryStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return true
}

func (r *Reducer) reduceDeliveryCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return true
}

func (r *Reducer) reduceDeliveryFailed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return true
}

func (r *Reducer) reduceToolStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	opID, _ := payload["toolOperationId"].(string)
	if opID == "" {
		return false
	}

	now := r.clock.Now()
	leaseDuration := 5 * time.Minute
	leaseExpires := now.Add(leaseDuration)

	if existing, ok := ctx.ActiveTools[opID]; ok {
		existing.LastActivityAt = now
		existing.LeaseExpiresAt = leaseExpires
		ctx.ActiveTools[opID] = existing
		return true
	}

	ctx.ActiveTools[opID] = ToolOperationState{
		OperationID:    opID,
		ToolCategory:   getString(payload, "toolCategory"),
		DisplayClass:   getString(payload, "displayClass"),
		Depth:          getInt(payload, "depth"),
		StartedAt:      event.OccurredAt,
		LastActivityAt: now,
		LeaseExpiresAt: leaseExpires,
		LongRunning:    getBool(payload, "expectedLongRunning"),
	}
	return true
}

func (r *Reducer) reduceToolProgress(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	opID, _ := payload["toolOperationId"].(string)
	if opID == "" {
		return false
	}
	if existing, ok := ctx.ActiveTools[opID]; ok {
		existing.LastActivityAt = r.clock.Now()
		existing.LeaseExpiresAt = r.clock.Now().Add(5 * time.Minute)
		ctx.ActiveTools[opID] = existing
		return true
	}
	return false
}

func (r *Reducer) reduceToolCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	opID, _ := payload["toolOperationId"].(string)
	if opID == "" {
		return false
	}
	if _, ok := ctx.ActiveTools[opID]; !ok {
		return false
	}
	delete(ctx.ActiveTools, opID)
	return true
}

func (r *Reducer) reduceToolFailed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.reduceToolCompleted(ctx, event)
}

func (r *Reducer) reduceToolCancelled(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.reduceToolCompleted(ctx, event)
}

func (r *Reducer) reduceVoiceSessionStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	now := r.clock.Now()
	if ctx.Voice.SessionID == event.SessionID &&
		ctx.Voice.State != "" &&
		!ctx.Voice.LeaseExpiresAt.IsZero() &&
		now.Before(ctx.Voice.LeaseExpiresAt) {
		return false
	}
	ctx.Voice = VoiceBehaviorState{
		SessionID:      event.SessionID,
		State:          "listening",
		LeaseExpiresAt: now.Add(15 * time.Second),
	}
	return true
}

func (r *Reducer) reduceVoiceListeningStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	turnID, _ := payload["turnId"].(string)

	if ctx.Voice.SessionID != event.SessionID && ctx.Voice.SessionID != "" {
		if ctx.Voice.StateVersion > 0 {
			return false
		}
	}

	ctx.Voice.SessionID = event.SessionID
	ctx.Voice.State = "listening"
	ctx.Voice.TurnID = turnID
	ctx.Voice.StateVersion++
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(15 * time.Second)
	return true
}

func (r *Reducer) reduceVoiceListeningActivity(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	if ctx.Voice.State != "listening" {
		return false
	}
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(15 * time.Second)
	return true
}

func (r *Reducer) reduceVoiceListeningEnded(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	if ctx.Voice.State != "listening" {
		return false
	}
	ctx.Voice.State = "thinking"
	ctx.Voice.StateVersion++
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(30 * time.Second)
	return true
}

func (r *Reducer) reduceVoiceProcessingStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	ctx.Voice.State = "processing"
	ctx.Voice.StateVersion++
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(30 * time.Second)
	return true
}

func (r *Reducer) reduceVoiceSpeakingStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	payload := parsePayload(event.Payload)
	turnID, _ := payload["turnId"].(string)

	if ctx.Voice.TurnID != "" && turnID != "" && ctx.Voice.TurnID != turnID {
		return false
	}

	ctx.Voice.State = "speaking"
	ctx.Voice.TurnID = turnID
	ctx.Voice.StateVersion++
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(5 * time.Minute)
	return true
}

func (r *Reducer) reduceVoiceSpeakingEnded(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	if ctx.Voice.State != "speaking" {
		return false
	}
	payload := parsePayload(event.Payload)
	turnID, _ := payload["turnId"].(string)
	if turnID != "" && ctx.Voice.TurnID != "" && turnID != ctx.Voice.TurnID {
		return false
	}

	ctx.Voice.State = "listening"
	ctx.Voice.StateVersion++
	ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(15 * time.Second)
	return true
}

func (r *Reducer) reduceVoiceTurnInterrupted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID {
		return false
	}
	if ctx.Voice.State == "speaking" {
		ctx.Voice.State = "listening"
		ctx.Voice.StateVersion++
		ctx.Voice.LeaseExpiresAt = r.clock.Now().Add(15 * time.Second)
		return true
	}
	return false
}

func (r *Reducer) reduceVoiceSessionEnded(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	if ctx.Voice.SessionID != event.SessionID && ctx.Voice.SessionID != "" {
		return false
	}
	ctx.Voice = VoiceBehaviorState{}
	return true
}

func (r *Reducer) reduceAffectChanged(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	version, _ := payload["version"].(string)
	label, _ := payload["label"].(string)

	if ctx.Stable.AffectVersion == version && version != "" {
		return false
	}

	ctx.Stable.AffectLabel = label
	ctx.Stable.AffectVersion = version
	return true
}

func (r *Reducer) reduceActivityChanged(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	activityKey, _ := payload["activityKey"].(string)
	source, _ := payload["source"].(string)
	confidence, _ := payload["confidence"].(float64)
	version, _ := payload["version"].(string)

	if version != "" && ctx.Stable.ActivityVersion == version {
		return false
	}
	if version == "" &&
		ctx.Stable.ActivityKey == activityKey &&
		ctx.Stable.ActivitySource == source &&
		ctx.Stable.ActivityConfidence == confidence {
		return false
	}

	ctx.Stable.ActivityKey = activityKey
	ctx.Stable.ActivitySource = source
	ctx.Stable.ActivityConfidence = confidence
	ctx.Stable.ActivityVersion = version
	return true
}

func (r *Reducer) reduceTimePeriodChanged(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	period, _ := payload["timePeriod"].(string)
	if ctx.Stable.TimePeriod == period {
		return false
	}
	ctx.Stable.TimePeriod = period
	return true
}

func (r *Reducer) reduceProactiveStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	correlationID, _ := payload["correlationId"].(string)
	intent, _ := payload["intent"].(string)

	if ctx.Transient.ProactiveID == correlationID && correlationID != "" {
		return false
	}

	ctx.Transient.ProactiveID = correlationID
	ctx.Transient.ProactiveIntent = intent
	return true
}

func (r *Reducer) reduceProactiveCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	correlationID, _ := payload["correlationId"].(string)
	if ctx.Transient.ProactiveID != correlationID && ctx.Transient.ProactiveID != "" {
		return false
	}
	ctx.Transient.ProactiveID = ""
	ctx.Transient.ProactiveIntent = ""
	return true
}

func (r *Reducer) reduceProactiveSuppressed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	ctx.Transient.ProactiveID = ""
	ctx.Transient.ProactiveIntent = ""
	return true
}

func desktopEventSequence(event BehaviorEventEnvelope, payload map[string]interface{}) int64 {
	if event.Sequence > 0 {
		return event.Sequence
	}
	return int64(getInt(payload, "sequence"))
}

func (r *Reducer) desktopGestureExpiry(_ BehaviorEventEnvelope, minimumLease time.Duration) time.Time {
	// Lease arithmetic must stay in the backend clock domain. Runtime events can
	// originate on another device whose wall clock is skewed relative to the
	// cloud/core process; using occurredAt here can pin or instantly expire a
	// gesture even though the event itself was accepted now.
	return r.clock.Now().Add(minimumLease)
}

func (r *Reducer) setDesktopGesture(
	ctx *BehaviorContextSnapshot,
	event BehaviorEventEnvelope,
	gesture string,
	minimumLease time.Duration,
) bool {
	payload := parsePayload(event.Payload)
	seq := desktopEventSequence(event, payload)
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	gestureID, _ := payload["gestureId"].(string)
	ctx.DesktopGesture = DesktopGestureState{
		CurrentGesture:  gesture,
		GestureID:       gestureID,
		Sequence:        seq,
		ExpiresAt:       r.desktopGestureExpiry(event, minimumLease),
		PendingClickWin: false,
	}
	return true
}

func (r *Reducer) reduceDesktopClicked(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "clicked", 2*time.Second)
}

func (r *Reducer) reduceDesktopDoubleClicked(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "double_clicked", 2*time.Second)
}

func (r *Reducer) reduceDesktopHovered(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "hovered", time.Second)
}

func (r *Reducer) reduceDesktopDragStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "drag", 2*time.Second)
}

func (r *Reducer) reduceDesktopDragMoved(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	// Drag movement is a lease refresh, not a new semantic decision. The manager
	// sends these events at a bounded cadence so a stalled/disconnected drag can
	// still expire instead of pinning the character in gesture_drag forever.
	return r.setDesktopGesture(ctx, event, "drag", 2*time.Second)
}

func (r *Reducer) reduceDesktopDragEnded(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "dropped", 3*time.Second)
}

func (r *Reducer) reduceDesktopDragCancelled(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	// Cancellation means the drag gesture ceased without a successful drop. It
	// must never be converted into `dropped`, otherwise the resolver schedules a
	// gesture_drop/land animation for pointer-capture loss, Escape/cancel paths,
	// or renderer teardown. Clear only an active drag and let normal stable
	// recovery arbitration choose what should play next.
	if ctx.DesktopGesture.CurrentGesture != "drag" {
		return false
	}

	payload := parsePayload(event.Payload)
	seq := desktopEventSequence(event, payload)
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	if gestureID, _ := payload["gestureId"].(string); gestureID != "" && ctx.DesktopGesture.GestureID != "" && gestureID != ctx.DesktopGesture.GestureID {
		return false
	}

	ctx.DesktopGesture = DesktopGestureState{}
	return true
}

func (r *Reducer) reduceDesktopFallStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "fall", 3*time.Second)
}

func (r *Reducer) reduceDesktopEdgeReached(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return r.setDesktopGesture(ctx, event, "edge", 3*time.Second)
}

func (r *Reducer) reduceDesktopInteracted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	interactionType, _ := payload["interactionType"].(string)
	if interactionType == "" {
		interactionType = "interacted"
	}
	return r.setDesktopGesture(ctx, event, interactionType, 5*time.Second)
}

func playbackStartedAt(_ BehaviorEventEnvelope, backendNow time.Time) time.Time {
	// Foreground timing participates in backend arbitration. Keep minimum/maximum
	// play windows in the same clock domain as the arbiter instead of trusting a
	// device wall clock that can be skewed.
	return backendNow
}

func foregroundMatchesPlayback(ctx *BehaviorContextSnapshot, payload map[string]interface{}) bool {
	if ctx.Foreground.DecisionID == "" && ctx.Foreground.CommandID == "" {
		return false
	}
	decisionID, _ := payload["decisionId"].(string)
	if decisionID != "" {
		return ctx.Foreground.DecisionID == decisionID
	}
	commandID, _ := payload["commandId"].(string)
	if commandID != "" {
		return ctx.Foreground.CommandID == commandID
	}
	return false
}

func consumeGestureForSemantic(ctx *BehaviorContextSnapshot, semantic string) {
	matches := false
	switch semantic {
	case "gesture_click":
		matches = ctx.DesktopGesture.CurrentGesture == "clicked"
	case "gesture_double_click":
		matches = ctx.DesktopGesture.CurrentGesture == "double_clicked"
	case "gesture_hover":
		matches = ctx.DesktopGesture.CurrentGesture == "hovered"
	case "gesture_drop":
		matches = ctx.DesktopGesture.CurrentGesture == "dropped" || ctx.DesktopGesture.CurrentGesture == "fall"
	case "physics_edge_sit":
		matches = ctx.DesktopGesture.CurrentGesture == "edge"
	}
	if matches {
		ctx.DesktopGesture = DesktopGestureState{}
	}
}

func (r *Reducer) reducePlaybackStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	commandID, _ := payload["commandId"].(string)
	actionKey, _ := payload["actionKey"].(string)
	semantic, _ := payload["semantic"].(string)

	startedAt := playbackStartedAt(event, r.clock.Now())
	interruptible := true
	if value, ok := payload["interruptible"].(bool); ok {
		interruptible = value
	}

	foreground := ForegroundActionState{
		DecisionID:      decisionID,
		CommandID:       commandID,
		Semantic:        semantic,
		ActionKey:       actionKey,
		StartedAt:       &startedAt,
		Interruptible:   interruptible,
		InstallationRev: int64(getInt(payload, "installationRevision")),
	}
	if minimumPlayMS := getInt(payload, "minimumPlayMs"); minimumPlayMS > 0 {
		until := startedAt.Add(time.Duration(minimumPlayMS) * time.Millisecond)
		foreground.MinPlayUntil = &until
	}
	if maximumPlayMS := getInt(payload, "maximumPlayMs"); maximumPlayMS > 0 {
		until := startedAt.Add(time.Duration(maximumPlayMS) * time.Millisecond)
		foreground.MaxPlayUntil = &until
	}

	// action_started is physical truth. A replacement may legitimately start
	// before the interrupted terminal event for the old playback arrives, so the
	// new foreground must supersede the old one rather than being rejected.
	ctx.Foreground = foreground
	consumeGestureForSemantic(ctx, semantic)
	return true
}

func (r *Reducer) reducePlaybackCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	if !foregroundMatchesPlayback(ctx, payload) {
		return false
	}
	ctx.Foreground = ForegroundActionState{}
	return true
}

func (r *Reducer) reducePlaybackInterrupted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	if !foregroundMatchesPlayback(ctx, payload) {
		return false
	}
	ctx.Foreground = ForegroundActionState{}
	return true
}

func (r *Reducer) reducePlaybackFailed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	if !foregroundMatchesPlayback(ctx, payload) {
		return false
	}
	ctx.Foreground = ForegroundActionState{}
	return true
}

func parsePayload(payload json.RawMessage) map[string]interface{} {
	if len(payload) == 0 {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func (s BehaviorContextSnapshot) Copy() BehaviorContextSnapshot {
	c := s
	if s.ActiveTools != nil {
		c.ActiveTools = make(map[string]ToolOperationState, len(s.ActiveTools))
		for k, v := range s.ActiveTools {
			c.ActiveTools[k] = v
		}
	}
	if s.Cooldowns != nil {
		c.Cooldowns = make(map[string]time.Time, len(s.Cooldowns))
		for k, v := range s.Cooldowns {
			c.Cooldowns[k] = v
		}
	}
	if s.RecentSemantics != nil {
		c.RecentSemantics = make([]RecentSemanticRecord, len(s.RecentSemantics))
		copy(c.RecentSemantics, s.RecentSemantics)
	}
	if s.LastSourceRevisions != nil {
		c.LastSourceRevisions = make(map[string]int64, len(s.LastSourceRevisions))
		for k, v := range s.LastSourceRevisions {
			c.LastSourceRevisions[k] = v
		}
	}
	return c
}

func NewDefaultContext(userID, characterID string) BehaviorContextSnapshot {
	return BehaviorContextSnapshot{
		UserID:              userID,
		CharacterID:         characterID,
		Revision:            1,
		ActiveTools:         make(map[string]ToolOperationState),
		Cooldowns:           make(map[string]time.Time),
		LastSourceRevisions: make(map[string]int64),
		Desired:             DesiredBehaviorState{Semantic: "fallback_idle", SourceLayer: "stable"},
	}
}
