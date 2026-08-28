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

	changed := false
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
		changed = r.reduceToolProgress(&next, event)

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
		changed = r.reduceVoiceListeningActivity(&next, event)

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
		changed = r.reduceDesktopHovered(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.started":
		changed = r.reduceDesktopDragStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "desktopGesture")
		}

	case "runtime.drag.moved":
		changed = r.reduceDesktopDragMoved(&next, event)

	case "runtime.drag.completed", "runtime.drag.cancelled":
		changed = r.reduceDesktopDragEnded(&next, event)
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
		changed = r.reducePlaybackStarted(&next, event)
		if changed {
			layersChanged = append(layersChanged, "foreground")
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

	r.checkLeases(&next, now)

	result.ContextChanged = changed
	result.LayersChanged = layersChanged
	result.NeedsDecision = changed && !result.IsDuplicate && !result.IsExpired
	next.UpdatedAt = now
	if changed {
		next.Revision++
	}

	return next, result, nil
}

func (r *Reducer) isDuplicateEvent(ctx BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	for _, rec := range ctx.RecentSemantics {
		_ = rec
	}
	return false
}

func (r *Reducer) checkLeases(ctx *BehaviorContextSnapshot, now time.Time) {
	if ctx.Transient.InteractionPhase != "" {
		hasActiveLease := false
		for _, tool := range ctx.ActiveTools {
			if now.Before(tool.LeaseExpiresAt) {
				hasActiveLease = true
				break
			}
		}
		_ = hasActiveLease
	}

	expired := []string{}
	for opID, tool := range ctx.ActiveTools {
		if now.After(tool.LeaseExpiresAt) {
			expired = append(expired, opID)
		}
	}
	for _, opID := range expired {
		delete(ctx.ActiveTools, opID)
	}

	if ctx.Voice.State != "" && ctx.Voice.LeaseExpiresAt.After(time.Time{}) {
		if now.After(ctx.Voice.LeaseExpiresAt) {
			ctx.Voice = VoiceBehaviorState{}
		}
	}

	if ctx.DesktopGesture.CurrentGesture == "drag" {
	}
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
		return false
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
	if ctx.Voice.SessionID == event.SessionID {
		return false
	}
	ctx.Voice = VoiceBehaviorState{
		SessionID: event.SessionID,
		State:     "listening",
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
	return false
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

	if ctx.Stable.ActivityKey == activityKey && ctx.Stable.ActivitySource == source {
		return false
	}

	ctx.Stable.ActivityKey = activityKey
	ctx.Stable.ActivitySource = source
	ctx.Stable.ActivityConfidence = confidence
	if version != "" {
	}
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

func (r *Reducer) reduceDesktopClicked(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	seq := int64(getInt(payload, "sequence"))
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	ctx.DesktopGesture.CurrentGesture = "clicked"
	ctx.DesktopGesture.Sequence = seq
	ctx.DesktopGesture.PendingClickWin = false
	return true
}

func (r *Reducer) reduceDesktopDoubleClicked(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	seq := int64(getInt(payload, "sequence"))
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	ctx.DesktopGesture.CurrentGesture = "double_clicked"
	ctx.DesktopGesture.Sequence = seq
	ctx.DesktopGesture.PendingClickWin = false
	return true
}

func (r *Reducer) reduceDesktopHovered(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	ctx.DesktopGesture.CurrentGesture = "hovered"
	return true
}

func (r *Reducer) reduceDesktopDragStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	seq := int64(getInt(payload, "sequence"))
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	ctx.DesktopGesture.CurrentGesture = "drag"
	ctx.DesktopGesture.Sequence = seq
	ctx.DesktopGesture.PendingClickWin = false
	return true
}

func (r *Reducer) reduceDesktopDragMoved(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	return false
}

func (r *Reducer) reduceDesktopDragEnded(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	seq := int64(getInt(payload, "sequence"))
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	ctx.DesktopGesture.CurrentGesture = "dropped"
	ctx.DesktopGesture.Sequence = seq
	return true
}

func (r *Reducer) reduceDesktopFallStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	ctx.DesktopGesture.CurrentGesture = "fall"
	return true
}

func (r *Reducer) reduceDesktopEdgeReached(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	ctx.DesktopGesture.CurrentGesture = "edge"
	return true
}

func (r *Reducer) reduceDesktopInteracted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	seq := int64(getInt(payload, "sequence"))
	if seq > 0 && seq <= ctx.DesktopGesture.Sequence {
		return false
	}
	interactionType, _ := payload["interactionType"].(string)
	if interactionType == "" {
		interactionType = "interacted"
	}
	ctx.DesktopGesture.CurrentGesture = interactionType
	ctx.DesktopGesture.Sequence = seq
	ctx.DesktopGesture.PendingClickWin = false
	return true
}

func (r *Reducer) reducePlaybackStarted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	commandID, _ := payload["commandId"].(string)
	actionKey, _ := payload["actionKey"].(string)

	if ctx.Foreground.DecisionID != "" && decisionID != "" && ctx.Foreground.DecisionID != decisionID {
		return false
	}

	now := r.clock.Now()
	ctx.Foreground.DecisionID = decisionID
	ctx.Foreground.CommandID = commandID
	ctx.Foreground.ActionKey = actionKey
	ctx.Foreground.StartedAt = &now
	return true
}

func (r *Reducer) reducePlaybackCompleted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	if decisionID != "" && ctx.Foreground.DecisionID != decisionID {
		return false
	}
	ctx.Foreground = ForegroundActionState{}
	return true
}

func (r *Reducer) reducePlaybackInterrupted(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	if decisionID != "" && ctx.Foreground.DecisionID != decisionID {
		return false
	}
	ctx.Foreground = ForegroundActionState{}
	return true
}

func (r *Reducer) reducePlaybackFailed(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope) bool {
	payload := parsePayload(event.Payload)
	decisionID, _ := payload["decisionId"].(string)
	if decisionID != "" && ctx.Foreground.DecisionID != decisionID {
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
	return c
}

func NewDefaultContext(userID, characterID string) BehaviorContextSnapshot {
	return BehaviorContextSnapshot{
		UserID:      userID,
		CharacterID: characterID,
		Revision:    1,
		ActiveTools: make(map[string]ToolOperationState),
		Cooldowns:   make(map[string]time.Time),
		Desired:     DesiredBehaviorState{Semantic: "fallback_idle", SourceLayer: "stable"},
	}
}
