package notificationruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/nativebridge"
	"gorm.io/gorm"
)

type Runtime struct {
	db                 *gorm.DB
	repo               *Repository
	gateway            *PushGateway
	queue              chan conversationstream.AgentUIEvent
	deliveryQueue      chan deliveryJob
	mu                 sync.Mutex
	active             map[string]*activeRun
	liveActivityStarts map[string]time.Time
	catchUpAttempts    map[string]time.Time
	cancelObserver     func()
	ctx                context.Context
	cancel             context.CancelFunc
}

type activeRun struct {
	state    ExecutionState
	spaceID  string
	notified bool
	timer    *time.Timer
}

type deliveryJob struct {
	outboxID string
	endpoint DeviceEndpoint
	envelope PushEnvelope
}

func NewRuntime(db *gorm.DB, bridge nativebridge.Bridge, preferNative bool) *Runtime {
	repo := NewRepository(db)
	nativeProvider := NewNativeBridgeProvider(bridge)
	return &Runtime{
		db:                 db,
		repo:               repo,
		gateway:            NewPushGateway(repo, nativeProvider, preferNative),
		queue:              make(chan conversationstream.AgentUIEvent, 4096),
		deliveryQueue:      make(chan deliveryJob, 2048),
		active:             map[string]*activeRun{},
		liveActivityStarts: map[string]time.Time{},
		catchUpAttempts:    map[string]time.Time{},
	}
}

func (r *Runtime) Repository() *Repository {
	return r.repo
}

func (r *Runtime) Capabilities() map[string]any {
	nativeDelivery := r.gateway != nil && r.gateway.PreferNative()
	deliveryMode := "remote"
	if nativeDelivery {
		deliveryMode = "native"
	}
	return map[string]any{
		"pushProviders":           r.gateway.Capabilities(),
		"nativeDataPushProviders": r.gateway.NativeDataCapabilities(),
		"messagePush":             true,
		"executionActivity":       true,
		"liveActivity":            true,
		"androidProgress":         true,
		"cloudOwned":              !nativeDelivery,
		"deliveryMode":            deliveryMode,
		"protocolVersion":         1,
	}
}

func (r *Runtime) Start(parent context.Context) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.ctx != nil {
		r.mu.Unlock()
		return
	}
	r.ctx, r.cancel = context.WithCancel(parent)
	r.cancelObserver = conversationstream.DefaultManager().RegisterObserver("notification-runtime", r)
	ctx := r.ctx
	r.mu.Unlock()
	go r.worker(ctx)
	go r.restoreActiveRuns(ctx)
	go r.liveActivityHeartbeatLoop(ctx)
	for i := 0; i < 4; i++ {
		go r.deliveryWorker(ctx)
	}
	go r.outboxRecoveryLoop(ctx)
}

func (r *Runtime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	cancelObserver := r.cancelObserver
	r.ctx = nil
	r.cancel = nil
	r.cancelObserver = nil
	for _, run := range r.active {
		if run.timer != nil {
			run.timer.Stop()
		}
	}
	r.active = map[string]*activeRun{}
	r.liveActivityStarts = map[string]time.Time{}
	r.catchUpAttempts = map[string]time.Time{}
	r.mu.Unlock()
	if cancelObserver != nil {
		cancelObserver()
	}
	if cancel != nil {
		cancel()
	}
}

func (r *Runtime) ObserveAgentUIEvent(_ context.Context, event conversationstream.AgentUIEvent) {
	if r == nil || !interestingEvent(event.Type) {
		return
	}
	select {
	case r.queue <- event:
	default:
		if terminalEvent(event.Type) {
			go func() {
				select {
				case r.queue <- event:
				case <-time.After(2 * time.Second):
				}
			}()
		}
	}
}

func (r *Runtime) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-r.queue:
			r.handleEvent(ctx, event)
		}
	}
}

func (r *Runtime) restoreActiveRuns(ctx context.Context) {
	if r == nil || r.db == nil {
		return
	}
	type persistedTurn struct {
		TurnID            string
		ConversationID    string
		CharacterID       string
		ExecutionID       string
		AgentID           string
		Status            string
		CreatedAt         string
		UpdatedAt         string
		SpaceID           string
		ConversationTitle string
	}
	var rows []persistedTurn
	// assistant_turns stores wall-clock timestamps as "2006-01-02 15:04:05".
	// Keep the comparison in that exact lexical format; RFC3339's 'T' would
	// incorrectly exclude same-day rows in SQLite string comparisons.
	cutoff := time.Now().Add(-6 * time.Hour).Format("2006-01-02 15:04:05")
	if err := r.db.WithContext(ctx).
		Table("assistant_turns AS t").
		Select(
			"t.id AS turn_id, t.conversation_id, t.character_id, t.execution_id, "+
				"t.agent_id, t.status, t.created_at, t.updated_at, "+
				"c.space_id, c.title AS conversation_title",
		).
		Joins("JOIN conversations AS c ON c.id = t.conversation_id").
		Where(
			"t.status IN ? AND t.updated_at >= ?",
			[]string{"queued", "starting", "running", "waiting_tool"},
			cutoff,
		).
		Order("t.updated_at DESC").
		Limit(256).
		Scan(&rows).Error; err != nil {
		return
	}

	type repair struct {
		spaceID string
		state   ExecutionState
	}
	repairs := make([]repair, 0, len(rows))
	now := time.Now().UTC()
	manager := conversationstream.DefaultManager()
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		runID := strings.TrimSpace(row.ExecutionID)
		if runID == "" {
			runID = strings.TrimSpace(row.TurnID)
		}
		if runID == "" || strings.TrimSpace(row.SpaceID) == "" {
			continue
		}
		state := ExecutionState{
			RunID:          runID,
			ConversationID: strings.TrimSpace(row.ConversationID),
			CharacterID:    strings.TrimSpace(row.CharacterID),
			AgentID:        fallback(strings.TrimSpace(row.AgentID), "Amitia"),
			Title:          fallback(strings.TrimSpace(row.ConversationTitle), "Amitia 正在执行"),
			Phase:          "running",
			Summary:        "正在运行",
			StartedAt:      parsePersistedNotificationTime(row.CreatedAt),
			UpdatedAt:      parsePersistedNotificationTime(row.UpdatedAt),
		}
		needsSurface := now.Sub(state.StartedAt) >= 12*time.Second
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case "queued":
			state.Phase = "queued"
			state.Summary = "等待执行"
		case "starting":
			state.Phase = "starting"
			state.Summary = "正在启动"
		case "waiting_tool":
			state.Phase = "waiting_tool"
			state.Summary = "等待工具执行"
			needsSurface = true
		}

		latest, latestErr := manager.LatestSequence(ctx, state.ConversationID)
		if latestErr == nil && latest > 0 {
			after := int64(0)
			if latest > 512 {
				after = latest - 512
			}
			if events, err := manager.ListDurableAfter(ctx, state.ConversationID, after, 1024); err == nil {
				for _, event := range events {
					eventRunID := strings.TrimSpace(event.ExecutionID)
					if eventRunID == "" {
						eventRunID = strings.TrimSpace(event.TurnID)
					}
					if eventRunID != runID || terminalEvent(event.Type) {
						continue
					}
					if event.EventSequence > state.Revision {
						state.Revision = event.EventSequence
					}
					if strings.TrimSpace(event.AgentID) != "" {
						state.AgentID = event.AgentID
					}
					if parsed := parsePersistedNotificationTime(event.CreatedAt); !parsed.IsZero() {
						state.UpdatedAt = parsed
					}
					updateExecutionState(&state, event)
					if executionEventNeedsSurface(event.Type) {
						needsSurface = true
					}
				}
			}
		}

		r.mu.Lock()
		if _, exists := r.active[runID]; exists {
			r.mu.Unlock()
			continue
		}
		run := &activeRun{
			state:    state,
			spaceID:  strings.TrimSpace(row.SpaceID),
			notified: needsSurface,
		}
		r.active[runID] = run
		if !needsSurface {
			remaining := 12*time.Second - now.Sub(state.StartedAt)
			if remaining < time.Second {
				remaining = time.Second
			}
			run.timer = time.AfterFunc(remaining, func() {
				r.promoteRun(runID)
			})
		}
		r.mu.Unlock()

		if needsSurface {
			repairs = append(repairs, repair{spaceID: row.SpaceID, state: state})
		}
	}
	for _, item := range repairs {
		select {
		case <-ctx.Done():
			return
		default:
			r.sendExecution(ctx, item.spaceID, item.state, "update")
		}
	}
}

func parsePersistedNotificationTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999Z07:00"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func interestingEvent(eventType string) bool {
	switch eventType {
	case "turn.queued", "turn.started", "turn.cancelling", "turn.completed", "turn.failed", "turn.interrupted",
		"tool_call.started", "tool_call.completed", "tool_call.failed", "tool_call.cancelled",
		"approval.requested", "approval.approved", "approval.denied", "approval.expired",
		"agent.tool.started", "agent.tool.progress", "agent.tool.completed", "agent.tool.failed", "agent.tool.cancelled":
		return true
	default:
		return false
	}
}

func terminalEvent(eventType string) bool {
	return eventType == "turn.completed" || eventType == "turn.failed" || eventType == "turn.interrupted"
}

func (r *Runtime) handleEvent(ctx context.Context, event conversationstream.AgentUIEvent) {
	runID := strings.TrimSpace(event.ExecutionID)
	if runID == "" {
		runID = strings.TrimSpace(event.TurnID)
	}
	if runID == "" {
		return
	}
	meta := r.resolveMetadata(ctx, event)
	now := time.Now().UTC()

	r.mu.Lock()
	run := r.active[runID]
	if run == nil {
		run = &activeRun{state: ExecutionState{
			RunID:          runID,
			ConversationID: event.ConversationID,
			CharacterID:    meta.characterID,
			AgentID:        fallback(event.AgentID, "Amitia"),
			Title:          fallback(meta.conversationTitle, "Amitia 正在执行"),
			Phase:          "starting",
			Revision:       event.EventSequence,
			StartedAt:      now,
			UpdatedAt:      now,
		}, spaceID: meta.spaceID}
		r.active[runID] = run
	}
	if run.spaceID == "" {
		run.spaceID = meta.spaceID
	}
	if run.state.CharacterID == "" {
		run.state.CharacterID = meta.characterID
	}
	run.state.Revision = event.EventSequence
	run.state.UpdatedAt = now
	updateExecutionState(&run.state, event)

	if terminalEvent(event.Type) {
		if run.timer != nil {
			run.timer.Stop()
			run.timer = nil
		}
		notified := run.notified
		state := run.state
		spaceID := run.spaceID
		delete(r.active, runID)
		r.mu.Unlock()
		if notified {
			r.sendExecution(ctx, spaceID, state, "end")
		}
		if event.Type == "turn.completed" && strings.TrimSpace(event.MessageID) != "" {
			r.sendFinalMessage(ctx, event, meta)
		}
		return
	}

	if (event.Type == "turn.started" || event.Type == "turn.queued") && run.timer == nil && !run.notified {
		run.timer = time.AfterFunc(12*time.Second, func() {
			r.promoteRun(runID)
		})
	}
	immediate := executionEventNeedsSurface(event.Type)
	if immediate && !run.notified {
		run.notified = true
		if run.timer != nil {
			run.timer.Stop()
			run.timer = nil
		}
		state := run.state
		spaceID := run.spaceID
		r.mu.Unlock()
		r.sendExecution(ctx, spaceID, state, "start")
		return
	}
	if run.notified && event.Type != "turn.queued" && event.Type != "turn.started" {
		state := run.state
		spaceID := run.spaceID
		r.mu.Unlock()
		r.sendExecution(ctx, spaceID, state, "update")
		return
	}
	r.mu.Unlock()
}

func (r *Runtime) promoteRun(runID string) {
	r.mu.Lock()
	run := r.active[runID]
	if run == nil || run.notified {
		r.mu.Unlock()
		return
	}
	run.notified = true
	run.timer = nil
	state := run.state
	spaceID := run.spaceID
	ctx := r.ctx
	r.mu.Unlock()
	if ctx != nil {
		r.sendExecution(ctx, spaceID, state, "start")
	}
}

func executionEventNeedsSurface(eventType string) bool {
	switch eventType {
	case "tool_call.started", "agent.tool.started", "approval.requested":
		return true
	default:
		return false
	}
}

func updateExecutionState(state *ExecutionState, event conversationstream.AgentUIEvent) {
	switch event.Type {
	case "turn.queued":
		state.Phase = "queued"
		state.Summary = "等待执行"
	case "turn.started":
		state.Phase = "running"
		state.Summary = "正在运行"
	case "turn.cancelling":
		state.Phase = "cancelling"
		state.Summary = "正在取消"
	case "tool_call.started", "agent.tool.started":
		state.Phase = "running"
		state.CurrentStep++
		name := payloadString(event.Payload, "toolName")
		if name == "" {
			name = payloadString(event.Payload, "name")
		}
		state.Summary = fallback(name, "正在调用工具")
	case "tool_call.completed", "agent.tool.completed":
		state.Phase = "running"
		state.Summary = "工具执行完成"
	case "tool_call.failed", "agent.tool.failed":
		state.Phase = "running"
		state.Summary = "工具执行失败，正在处理"
	case "approval.requested":
		state.Phase = "waiting_approval"
		state.Summary = "等待你的确认"
	case "approval.approved":
		state.Phase = "running"
		state.Summary = "已确认，继续执行"
	case "approval.denied":
		state.Phase = "running"
		state.Summary = "已拒绝，正在调整"
	case "turn.completed":
		state.Phase = "completed"
		state.Summary = "已完成"
		state.Progress = 1
	case "turn.failed":
		state.Phase = "failed"
		state.Summary = "执行失败"
	case "turn.interrupted":
		state.Phase = "interrupted"
		state.Summary = "已中断"
	}
	if total := payloadInt(event.Payload, "totalSteps"); total > 0 {
		state.TotalSteps = total
	}
	if current := payloadInt(event.Payload, "currentStep"); current > 0 {
		state.CurrentStep = current
	}
	if tokens := payloadInt(event.Payload, "totalTokens"); tokens > 0 {
		state.TotalTokens = tokens
	}
	if state.TotalSteps > 0 {
		state.Progress = float64(state.CurrentStep) / float64(state.TotalSteps)
		if state.Progress > 1 {
			state.Progress = 1
		}
	}
}

type eventMetadata struct {
	spaceID           string
	characterID       string
	characterName     string
	conversationTitle string
	messageContent    string
}

func (r *Runtime) resolveMetadata(ctx context.Context, event conversationstream.AgentUIEvent) eventMetadata {
	var meta eventMetadata
	if r.db == nil {
		return meta
	}
	_ = r.db.WithContext(ctx).Table("conversations").Select("space_id", "title").Where("id = ?", event.ConversationID).Row().Scan(&meta.spaceID, &meta.conversationTitle)
	if event.MessageID != "" {
		_ = r.db.WithContext(ctx).Table("messages").Select("character_id", "content").Where("id = ?", event.MessageID).Row().Scan(&meta.characterID, &meta.messageContent)
	}
	if meta.characterID == "" && event.TurnID != "" {
		_ = r.db.WithContext(ctx).Table("assistant_turns").Select("character_id").Where("id = ?", event.TurnID).Row().Scan(&meta.characterID)
	}
	if meta.characterID != "" {
		_ = r.db.WithContext(ctx).Table("characters").Select("name").Where("id = ? AND space_id = ?", meta.characterID, meta.spaceID).Row().Scan(&meta.characterName)
	}
	return meta
}

func (r *Runtime) sendFinalMessage(ctx context.Context, event conversationstream.AgentUIEvent, meta eventMetadata) {
	if meta.spaceID == "" || meta.messageContent == "" {
		meta = r.resolveMetadata(ctx, event)
	}
	if meta.spaceID == "" || meta.messageContent == "" {
		return
	}
	devices, err := r.repo.ListPushDevices(ctx, meta.spaceID)
	if err != nil {
		return
	}
	source := ""
	if event.Payload != nil {
		if rawSource, ok := event.Payload["source"].(string); ok {
			source = strings.ToLower(strings.TrimSpace(rawSource))
		}
	}
	isReminder := strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.RequestID)), "reminder:")
	isProactive := source == "proactive"
	notificationType := "message.completed"
	if isReminder {
		notificationType = "reminder.triggered"
	} else if isProactive {
		notificationType = "proactive.message"
	}
	for _, endpoint := range devices {
		if !endpoint.SystemNotificationsEnabled {
			continue
		}
		if suppressMessageForPresence(endpoint, event.ConversationID) {
			continue
		}
		if isReminder || isProactive {
			if !endpoint.ReminderPushEnabled {
				continue
			}
		} else if !endpoint.MessagePushEnabled {
			continue
		}
		title, body := PreviewForMode(endpoint.PreviewMode, fallback(meta.characterName, meta.conversationTitle), meta.messageContent)
		envelope := PushEnvelope{
			NotificationID: uuid.NewString(),
			Type:           notificationType,
			SpaceID:        meta.spaceID,
			ConversationID: event.ConversationID,
			CharacterID:    meta.characterID,
			MessageID:      event.MessageID,
			RunID:          fallback(event.ExecutionID, event.TurnID),
			Revision:       event.EventSequence,
			Title:          title,
			Body:           body,
			DeepLink:       fmt.Sprintf("amitia://chat/%s?message=%s", event.ConversationID, event.MessageID),
			Priority:       "high",
			TTL:            24 * time.Hour,
			Sound:          endpoint.SoundEnabled,
			Data: map[string]string{
				"senderName":  fallback(meta.characterName, meta.conversationTitle),
				"previewMode": endpoint.PreviewMode,
				"source":      source,
			},
		}
		r.deliver(ctx, endpoint, envelope)
	}
}

func liveActivityDeviceKey(endpoint DeviceEndpoint) string {
	return strings.TrimSpace(endpoint.SpaceID) + "\x00" + strings.TrimSpace(endpoint.DeviceID)
}

func (r *Runtime) recordLiveActivityStart(endpoint DeviceEndpoint) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.liveActivityStarts == nil {
		r.liveActivityStarts = map[string]time.Time{}
	}
	r.liveActivityStarts[liveActivityDeviceKey(endpoint)] = time.Now().UTC()
}

func (r *Runtime) liveActivityStartedRecently(
	ctx context.Context,
	endpoint DeviceEndpoint,
	within time.Duration,
) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	started := r.liveActivityStarts[liveActivityDeviceKey(endpoint)]
	r.mu.Unlock()
	if !started.IsZero() && time.Since(started) < within {
		return true
	}
	if r.repo != nil {
		recent, err := r.repo.HasRecentLiveActivityStart(
			ctx,
			endpoint.SpaceID,
			endpoint.DeviceID,
			within,
		)
		return err == nil && recent
	}
	return false
}

func (r *Runtime) liveActivityHeartbeatLoop(ctx context.Context) {
	if r == nil || r.gateway == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			type heartbeatRun struct {
				spaceID string
				state   ExecutionState
			}
			r.mu.Lock()
			runs := make([]heartbeatRun, 0, len(r.active))
			for _, run := range r.active {
				if run == nil || !run.notified || strings.TrimSpace(run.spaceID) == "" {
					continue
				}
				runs = append(runs, heartbeatRun{spaceID: run.spaceID, state: run.state})
			}
			r.mu.Unlock()
			for _, run := range runs {
				if r.gateway.PreferNative() {
					// Local Core: renew ActivityKit/Android progress through the
					// existing native bridge instead of inventing a second path.
					r.sendExecution(ctx, run.spaceID, run.state, "update")
				} else {
					r.sendLiveActivityHeartbeat(ctx, run.spaceID, run.state)
				}
			}
		}
	}
}

func (r *Runtime) sendLiveActivityHeartbeat(ctx context.Context, spaceID string, state ExecutionState) {
	if r == nil || r.repo == nil || r.gateway == nil || r.gateway.PreferNative() {
		return
	}
	devices, err := r.repo.ListPushDevices(ctx, spaceID)
	if err != nil {
		return
	}
	for _, endpoint := range devices {
		if endpoint.Platform != "ios" || !endpoint.ExecutionActivityEnabled || !endpoint.LiveActivitySupported {
			continue
		}
		live, err := r.repo.GetLiveActivityToken(ctx, endpoint.SpaceID, endpoint.DeviceID, state.RunID)
		if err != nil || strings.TrimSpace(live.UpdateToken) == "" {
			continue
		}
		presentation := localizeExecutionState(
			executionStateForPreview(state, endpoint.PreviewMode),
			endpoint.Locale,
		)
		result := r.gateway.SendLiveActivity(ctx, endpoint, live.UpdateToken, presentation, "update")
		r.recordProviderResult(ctx, endpoint, PushEnvelope{
			NotificationID: fmt.Sprintf("liveactivity:heartbeat:%s:%d", state.RunID, time.Now().Unix()/120),
			Type:           "liveactivity.update",
			SpaceID:        endpoint.SpaceID,
			RunID:          state.RunID,
			Revision:       state.Revision,
			TTL:            5 * time.Minute,
		}, result, 0)
		if result.InvalidToken {
			_ = r.repo.EndLiveActivity(ctx, endpoint.SpaceID, endpoint.DeviceID, state.RunID, state.Revision)
		}
	}
}

// CatchUpLiveActivity repairs the Live Activity presentation for one newly
// registered iOS device. It never broadcasts catch-up to other devices, never
// revives a terminal run, and throttles registration-triggered starts.
func (r *Runtime) CatchUpLiveActivity(endpoint DeviceEndpoint) {
	if !r.liveActivityCatchUpEligible(endpoint) {
		return
	}
	key := liveActivityDeviceKey(endpoint)
	now := time.Now().UTC()
	r.mu.Lock()
	if last := r.catchUpAttempts[key]; !last.IsZero() && now.Sub(last) < 2*time.Minute {
		r.mu.Unlock()
		return
	}
	if r.catchUpAttempts == nil {
		r.catchUpAttempts = map[string]time.Time{}
	}
	r.catchUpAttempts[key] = now
	var selected *ExecutionState
	for _, run := range r.active {
		if run == nil || !run.notified || run.spaceID != endpoint.SpaceID {
			continue
		}
		if selected == nil || run.state.UpdatedAt.After(selected.UpdatedAt) {
			copyState := run.state
			selected = &copyState
		}
	}
	r.mu.Unlock()
	if selected != nil {
		r.catchUpLiveActivityState(endpoint, *selected)
	}
}

// CatchUpLiveActivityRun is used after ActivityKit publishes an update token.
// It targets that exact run instead of the newest run for the device, avoiding
// cross-run token races when more than one Live Activity briefly overlaps.
func (r *Runtime) CatchUpLiveActivityRun(endpoint DeviceEndpoint, runID string) {
	if !r.liveActivityCatchUpEligible(endpoint) {
		return
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return
	}
	r.mu.Lock()
	run := r.active[runID]
	if run == nil || !run.notified || run.spaceID != endpoint.SpaceID {
		r.mu.Unlock()
		return
	}
	state := run.state
	r.mu.Unlock()
	r.catchUpLiveActivityState(endpoint, state)
}

func (r *Runtime) liveActivityCatchUpEligible(endpoint DeviceEndpoint) bool {
	return r != nil && r.repo != nil && r.gateway != nil &&
		!r.gateway.PreferNative() && endpoint.Platform == "ios" &&
		endpoint.PushEnabled && endpoint.ExecutionActivityEnabled &&
		endpoint.LiveActivitySupported
}

func (r *Runtime) catchUpLiveActivityState(endpoint DeviceEndpoint, state ExecutionState) {
	r.mu.Lock()
	baseCtx := r.ctx
	r.mu.Unlock()
	if baseCtx == nil {
		return
	}
	ctx, cancel := context.WithTimeout(baseCtx, 20*time.Second)
	defer cancel()
	terminal, err := r.repo.IsRunTerminal(ctx, endpoint.SpaceID, endpoint.DeviceID, state.RunID)
	if err != nil || terminal {
		return
	}
	event := "update"
	token := ""
	if live, liveErr := r.repo.GetLiveActivityToken(ctx, endpoint.SpaceID, endpoint.DeviceID, state.RunID); liveErr == nil {
		token = strings.TrimSpace(live.UpdateToken)
	}
	if token == "" {
		event = "start"
		token = strings.TrimSpace(endpoint.LiveActivityPushToStartToken)
		if token == "" || r.liveActivityStartedRecently(ctx, endpoint, 30*time.Minute) {
			return
		}
	}
	presentation := localizeExecutionState(
		executionStateForPreview(state, endpoint.PreviewMode),
		endpoint.Locale,
	)
	if event == "start" {
		r.endSupersededLiveActivities(ctx, endpoint, presentation)
	}
	result := r.gateway.SendLiveActivity(ctx, endpoint, token, presentation, event)
	r.recordProviderResult(ctx, endpoint, PushEnvelope{
		NotificationID: fmt.Sprintf("liveactivity:catchup:%s:%d", state.RunID, state.Revision),
		Type:           "liveactivity." + event,
		SpaceID:        endpoint.SpaceID,
		ConversationID: state.ConversationID,
		RunID:          state.RunID,
		Revision:       state.Revision,
		TTL:            2 * time.Minute,
	}, result, 0)
	if result.Accepted && event == "start" {
		r.recordLiveActivityStart(endpoint)
	}
	if result.InvalidToken {
		if event == "start" {
			_ = r.repo.ClearLiveActivityPushToStartToken(ctx, endpoint.SpaceID, endpoint.DeviceID)
		} else {
			_ = r.repo.EndLiveActivity(ctx, endpoint.SpaceID, endpoint.DeviceID, state.RunID, state.Revision)
		}
	}
}

func suppressMessageForPresence(endpoint DeviceEndpoint, conversationID string) bool {
	return endpoint.Foreground && endpoint.ActiveConversationID == conversationID && time.Since(endpoint.PresenceUpdatedAt) < 90*time.Second
}

func (r *Runtime) endSupersededLiveActivities(
	ctx context.Context,
	endpoint DeviceEndpoint,
	state ExecutionState,
) {
	if r == nil || r.repo == nil || r.gateway == nil ||
		endpoint.Platform != "ios" || r.gateway.PreferNative() ||
		strings.TrimSpace(state.ConversationID) == "" {
		return
	}
	rows, err := r.repo.ListActiveLiveActivitiesForConversation(
		ctx,
		endpoint.SpaceID,
		endpoint.DeviceID,
		state.ConversationID,
	)
	if err != nil {
		return
	}
	for _, row := range rows {
		if row.RunID == state.RunID || strings.TrimSpace(row.UpdateToken) == "" {
			continue
		}
		revision := row.Revision + 1
		if revision <= state.Revision {
			revision = state.Revision
		}
		ended := ExecutionState{
			RunID:          row.RunID,
			ConversationID: state.ConversationID,
			AgentID:        state.AgentID,
			Title:          state.Title,
			Summary:        "已由新任务替换",
			Phase:          "interrupted",
			Revision:       revision,
			StartedAt:      state.StartedAt,
			UpdatedAt:      time.Now().UTC(),
		}
		result := r.gateway.SendLiveActivity(ctx, endpoint, row.UpdateToken, ended, "end")
		r.recordProviderResult(ctx, endpoint, PushEnvelope{
			NotificationID: fmt.Sprintf("liveactivity:supersede:%s:%d", row.RunID, revision),
			Type:           "liveactivity.end",
			SpaceID:        endpoint.SpaceID,
			ConversationID: state.ConversationID,
			RunID:          row.RunID,
			Revision:       revision,
			TTL:            time.Minute,
		}, result, 0)
		if result.Accepted || result.InvalidToken {
			_ = r.repo.EndLiveActivity(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				row.RunID,
				revision,
			)
		}
	}
}

func (r *Runtime) sendExecution(ctx context.Context, spaceID string, state ExecutionState, event string) {
	if spaceID == "" {
		return
	}
	devices, err := r.repo.ListPushDevices(ctx, spaceID)
	if err != nil {
		return
	}
	for _, endpoint := range devices {
		if event == "end" && endpoint.Platform == "ios" && !r.gateway.PreferNative() {
			_ = r.repo.MarkRunTerminal(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				state.RunID,
				state.Revision,
			)
			_ = r.repo.SupersedePendingLiveActivityStart(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				state.RunID,
			)
		}
		if !endpoint.ExecutionActivityEnabled {
			continue
		}
		if endpoint.Platform == "android" && !endpoint.SystemNotificationsEnabled {
			continue
		}
		if endpoint.Platform == "ios" &&
			!endpoint.SystemNotificationsEnabled &&
			!endpoint.LiveActivitySupported {
			continue
		}
		presentationState := localizeExecutionState(
			executionStateForPreview(state, endpoint.PreviewMode),
			endpoint.Locale,
		)
		if event == "start" && endpoint.Platform == "ios" && endpoint.LiveActivitySupported && !r.gateway.PreferNative() {
			r.endSupersededLiveActivities(ctx, endpoint, presentationState)
		}
		if endpoint.Platform == "ios" && endpoint.LiveActivitySupported && !r.gateway.PreferNative() {
			token := ""
			usingPushToStart := false
			if event == "start" {
				token = endpoint.LiveActivityPushToStartToken
				usingPushToStart = token != ""
			} else if live, liveErr := r.repo.GetLiveActivityToken(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				state.RunID,
			); liveErr == nil {
				token = live.UpdateToken
			}
			if token != "" {
				result := r.gateway.SendLiveActivity(ctx, endpoint, token, presentationState, event)
				r.recordProviderResult(ctx, endpoint, PushEnvelope{
					NotificationID: uuid.NewString(),
					Type:           "run." + event,
					SpaceID:        spaceID,
					RunID:          state.RunID,
					Revision:       state.Revision,
					TTL:            5 * time.Minute,
				}, result, 0)
				if result.Accepted && event == "start" {
					r.recordLiveActivityStart(endpoint)
				}
				if result.Accepted && event == "end" {
					_ = r.repo.EndLiveActivity(
						ctx,
						endpoint.SpaceID,
						endpoint.DeviceID,
						state.RunID,
						state.Revision,
					)
				}
				if result.InvalidToken {
					if usingPushToStart {
						_ = r.repo.ClearLiveActivityPushToStartToken(
							ctx,
							endpoint.SpaceID,
							endpoint.DeviceID,
						)
					} else {
						_ = r.repo.EndLiveActivity(
							ctx,
							endpoint.SpaceID,
							endpoint.DeviceID,
							state.RunID,
							state.Revision,
						)
					}
				}
				if result.Accepted {
					continue
				}
				if !result.InvalidToken &&
					retryableProviderResult(result) &&
					(event == "start" || event == "end") {
					r.queueLiveActivityRetry(
						ctx,
						endpoint,
						presentationState,
						event,
					)
				}
				// Live Activity delivery is an enhancement. Fall through to
				// the standard notification path whenever it is unavailable
				// or rejected so lock-screen execution state is never lost.
			} else if event == "start" || event == "end" {
				r.queueLiveActivityRetry(ctx, endpoint, presentationState, event)
			}
		}
		if endpoint.Platform == "ios" &&
			!r.gateway.PreferNative() &&
			!endpoint.SystemNotificationsEnabled {
			continue
		}
		if endpoint.Platform == "ios" && !r.gateway.PreferNative() && event == "update" {
			// A delivered APNs alert cannot be updated in-place. When Live
			// Activity is unavailable, keep start/end visibility without
			// stacking one lock-screen alert for every progress revision.
			continue
		}
		envelope := PushEnvelope{
			NotificationID: uuid.NewString(),
			Type:           "run." + event,
			SpaceID:        spaceID,
			ConversationID: state.ConversationID,
			CharacterID:    state.CharacterID,
			RunID:          state.RunID,
			Revision:       state.Revision,
			Title:          fallback(presentationState.Title, "Amitia 正在执行"),
			Body:           executionNotificationBody(presentationState, endpoint.Locale),
			DeepLink:       executionDeepLink(state),
			Priority:       map[bool]string{true: "high", false: "normal"}[event == "start" || event == "end"],
			TTL:            5 * time.Minute,
			Data: map[string]string{
				"phase":       presentationState.Phase,
				"summary":     presentationState.Summary,
				"agentId":     presentationState.AgentID,
				"currentStep": strconv.Itoa(presentationState.CurrentStep),
				"totalSteps":  strconv.Itoa(presentationState.TotalSteps),
				"progress":    strconv.FormatFloat(presentationState.Progress, 'f', 4, 64),
				"totalTokens": strconv.Itoa(presentationState.TotalTokens),
				"startedAt":   presentationState.StartedAt.Format(time.RFC3339Nano),
				"updatedAt":   presentationState.UpdatedAt.Format(time.RFC3339Nano),
				"locale":      endpoint.Locale,
				"appearance":  endpoint.Appearance,
				"previewMode": endpoint.PreviewMode,
			},
		}
		r.deliver(ctx, endpoint, envelope)
	}
}

func (r *Runtime) CleanupExecutionNotifications(
	ctx context.Context,
	spaceID, deviceID string,
) error {
	if r == nil || r.repo == nil || r.gateway == nil {
		return fmt.Errorf("notification runtime unavailable")
	}
	endpoint, err := r.repo.GetDevice(ctx, strings.TrimSpace(spaceID), strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}

	active := make(map[string]ExecutionState)
	r.mu.Lock()
	for _, run := range r.active {
		if run == nil || run.spaceID != strings.TrimSpace(spaceID) || !run.notified {
			continue
		}
		active[run.state.RunID] = run.state
	}
	r.mu.Unlock()

	now := time.Now().UTC()
	if endpoint.Platform == "ios" && !r.gateway.PreferNative() {
		rows, listErr := r.repo.ListActiveLiveActivities(ctx, endpoint.SpaceID, endpoint.DeviceID)
		if listErr != nil {
			return listErr
		}
		for _, row := range rows {
			if _, ok := active[row.RunID]; ok {
				continue
			}
			active[row.RunID] = ExecutionState{
				RunID:     row.RunID,
				AgentID:   "Amitia",
				Title:     "Amitia",
				Summary:   "任务系统通知已关闭",
				Phase:     "interrupted",
				Revision:  row.Revision,
				StartedAt: now,
				UpdatedAt: now,
			}
		}
		for _, state := range active {
			state.Phase = "interrupted"
			state.Summary = "任务系统通知已关闭"
			if state.Revision <= 0 {
				state.Revision = 1
			} else {
				state.Revision++
			}
			state.UpdatedAt = now
			if state.StartedAt.IsZero() {
				state.StartedAt = now
			}
			_ = r.repo.MarkRunTerminal(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				state.RunID,
				state.Revision,
			)
			_ = r.repo.SupersedePendingLiveActivityStart(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				state.RunID,
			)
			r.queueLiveActivityRetry(ctx, *endpoint, state, "end")
		}
		return nil
	}

	canDismissOnAndroid := endpoint.Platform != "android" ||
		r.gateway.PreferNative() ||
		strings.EqualFold(endpoint.PreferredProvider, "fcm") ||
		endpoint.SupportsNativeData(endpoint.PreferredProvider)
	if !canDismissOnAndroid {
		return nil
	}
	for _, state := range active {
		envelope := PushEnvelope{
			NotificationID: fmt.Sprintf(
				"run:dismiss:%s:%s",
				state.RunID,
				endpoint.DeviceID,
			),
			Type:           "run.dismiss",
			SpaceID:        endpoint.SpaceID,
			ConversationID: state.ConversationID,
			CharacterID:    state.CharacterID,
			RunID:          state.RunID,
			Revision:       state.Revision + 1,
			Priority:       "high",
			TTL:            time.Minute,
			Data: map[string]string{
				"phase":   "interrupted",
				"summary": "任务系统通知已关闭",
			},
		}
		r.deliver(ctx, *endpoint, envelope)
	}
	return nil
}

type IncomingCall struct {
	RecipientDeviceID string
	Owned             bool
	CallID            string
	ConversationID    string
	CharacterID       string
	CallerName        string
	CallType          string
}

func (r *Runtime) PushIncomingCall(ctx context.Context, spaceID string, call IncomingCall) (int, error) {
	if r == nil || r.repo == nil {
		return 0, fmt.Errorf("notification runtime unavailable")
	}
	call.CallID = strings.TrimSpace(call.CallID)
	if call.CallID == "" {
		call.CallID = uuid.NewString()
	}
	call.CallType = strings.ToLower(strings.TrimSpace(call.CallType))
	if call.CallType != "video" {
		call.CallType = "audio"
	}
	devices, err := r.repo.ListPushDevices(ctx, strings.TrimSpace(spaceID))
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, endpoint := range devices {
		if call.RecipientDeviceID != "" && endpoint.DeviceID != call.RecipientDeviceID {
			continue
		}
		if !endpoint.CallPushEnabled {
			continue
		}
		callerName := strings.TrimSpace(call.CallerName)
		if callerName == "" {
			callerName = "Amitia"
		}
		displayCallerName := callerName
		callBody := map[bool]string{
			true:  "邀请你进行视频通话",
			false: "邀请你进行语音通话",
		}[call.CallType == "video"]
		switch strings.ToLower(strings.TrimSpace(endpoint.PreviewMode)) {
		case "hidden":
			displayCallerName = "Amitia"
			callBody = "你有一通来电"
		case "sender_only":
			callBody = "发来通话邀请"
		}
		deepLink := fmt.Sprintf("amitia://call/%s?call=%s&type=%s", call.ConversationID, call.CallID, call.CallType)
		if call.Owned {
			deepLink += "&owned=1&characterId=" + url.QueryEscape(call.CharacterID)
		}
		envelope := PushEnvelope{
			NotificationID: fmt.Sprintf(
				"call:incoming:%s:%s",
				call.CallID,
				endpoint.DeviceID,
			),
			Type:           "call.incoming",
			SpaceID:        spaceID,
			ConversationID: strings.TrimSpace(call.ConversationID),
			CharacterID:    strings.TrimSpace(call.CharacterID),
			Title:          displayCallerName,
			Body:           callBody,
			DeepLink:       deepLink,
			Priority:       "high",
			TTL:            45 * time.Second,
			Sound:          true,
			Data: map[string]string{
				"callId":      call.CallID,
				"callerName":  displayCallerName,
				"callType":    call.CallType,
				"previewMode": endpoint.PreviewMode,
			},
		}
		r.deliver(ctx, endpoint, envelope)
		accepted++
	}
	return accepted, nil
}

func (r *Runtime) EndCall(ctx context.Context, spaceID, callID, conversationID, reason string) (int, error) {
	return r.endCall(ctx, spaceID, "", callID, conversationID, reason)
}

func (r *Runtime) AnswerCall(
	ctx context.Context,
	spaceID, answeringDeviceID, callID, conversationID string,
) (int, error) {
	return r.endCall(
		ctx,
		spaceID,
		strings.TrimSpace(answeringDeviceID),
		callID,
		conversationID,
		"answered_elsewhere",
	)
}

func (r *Runtime) endCall(
	ctx context.Context,
	spaceID, excludedDeviceID, callID, conversationID, reason string,
) (int, error) {
	if r == nil || r.repo == nil {
		return 0, fmt.Errorf("notification runtime unavailable")
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return 0, fmt.Errorf("callId is required")
	}
	devices, err := r.repo.ListPushDevices(ctx, strings.TrimSpace(spaceID))
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, endpoint := range devices {
		if !endpoint.CallPushEnabled ||
			(excludedDeviceID != "" && endpoint.DeviceID == excludedDeviceID) {
			continue
		}
		envelope := PushEnvelope{
			NotificationID: fmt.Sprintf(
				"call:end:%s:%s",
				callID,
				endpoint.DeviceID,
			),
			Type:           "call.ended",
			SpaceID:        spaceID,
			ConversationID: strings.TrimSpace(conversationID),
			DeepLink:       fmt.Sprintf("amitia://call/%s?call=%s", conversationID, callID),
			Priority:       "high",
			TTL:            30 * time.Second,
			Data: map[string]string{
				"callId": callID,
				"reason": strings.TrimSpace(reason),
			},
		}
		r.deliver(ctx, endpoint, envelope)
		accepted++
	}
	return accepted, nil
}

func (r *Runtime) TestDevicePush(ctx context.Context, spaceID, deviceID string) (map[string]any, error) {
	endpoint, err := r.repo.GetDevice(ctx, spaceID, deviceID)
	if err != nil {
		return nil, err
	}
	body := "Cloud Remote Push 链路已可用。"
	if r.gateway != nil && r.gateway.PreferNative() {
		body = "本地 Core → Native Bridge 通知链路已可用。"
	}
	envelope := PushEnvelope{
		NotificationID: uuid.NewString(),
		Type:           "system.test",
		SpaceID:        spaceID,
		Title:          "Amitia 测试通知",
		Body:           body,
		DeepLink:       "amitia://settings/notifications",
		Priority:       "high",
		TTL:            5 * time.Minute,
		Sound:          endpoint.SoundEnabled,
	}
	result := r.gateway.Send(ctx, *endpoint, envelope)
	r.recordProviderResult(ctx, *endpoint, envelope, result, 0)
	return map[string]any{
		"accepted":          result.Accepted,
		"provider":          result.Provider,
		"providerMessageId": result.ProviderMessageID,
		"errorCode":         result.ErrorCode,
		"errorMessage":      result.ErrorMessage,
	}, nil
}

func (r *Runtime) queueLiveActivityRetry(
	ctx context.Context,
	endpoint DeviceEndpoint,
	state ExecutionState,
	event string,
) {
	if r == nil || r.repo == nil || endpoint.Platform != "ios" {
		return
	}
	event = strings.ToLower(strings.TrimSpace(event))
	if event != "start" && event != "end" {
		return
	}
	ttl := 2 * time.Minute
	if event == "end" {
		ttl = 3 * time.Minute
	}
	envelope := PushEnvelope{
		NotificationID: fmt.Sprintf(
			"liveactivity:%s:%s:%d",
			event,
			state.RunID,
			state.Revision,
		),
		Type:           "liveactivity." + event,
		SpaceID:        endpoint.SpaceID,
		ConversationID: state.ConversationID,
		CharacterID:    state.CharacterID,
		RunID:          state.RunID,
		Revision:       state.Revision,
		Title:          state.Title,
		Body:           state.Summary,
		Priority:       "high",
		TTL:            ttl,
		Data: map[string]string{
			"phase":       state.Phase,
			"agentId":     state.AgentID,
			"currentStep": strconv.Itoa(state.CurrentStep),
			"totalSteps":  strconv.Itoa(state.TotalSteps),
			"progress":    strconv.FormatFloat(state.Progress, 'f', 4, 64),
			"totalTokens": strconv.Itoa(state.TotalTokens),
			"startedAt":   state.StartedAt.Format(time.RFC3339Nano),
			"updatedAt":   state.UpdatedAt.Format(time.RFC3339Nano),
		},
	}
	r.deliver(ctx, endpoint, envelope)
}

func executionStateFromLiveActivityEnvelope(envelope PushEnvelope) ExecutionState {
	state := ExecutionState{
		RunID:          envelope.RunID,
		ConversationID: envelope.ConversationID,
		CharacterID:    envelope.CharacterID,
		AgentID:        envelope.Data["agentId"],
		Title:          envelope.Title,
		Summary:        envelope.Body,
		Phase:          envelope.Data["phase"],
		Revision:       envelope.Revision,
	}
	state.CurrentStep, _ = strconv.Atoi(envelope.Data["currentStep"])
	state.TotalSteps, _ = strconv.Atoi(envelope.Data["totalSteps"])
	state.Progress, _ = strconv.ParseFloat(envelope.Data["progress"], 64)
	state.TotalTokens, _ = strconv.Atoi(envelope.Data["totalTokens"])
	state.StartedAt = parseNotificationTime(envelope.Data["startedAt"])
	state.UpdatedAt = parseNotificationTime(envelope.Data["updatedAt"])
	return state
}

func parseNotificationTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed
		}
	}
	return time.Now().UTC()
}

func (r *Runtime) sendDeliveryAttempt(
	ctx context.Context,
	job deliveryJob,
) ProviderResult {
	if strings.HasPrefix(job.envelope.Type, "liveactivity.") {
		return r.sendLiveActivityRetryAttempt(ctx, job.endpoint, job.envelope)
	}
	return r.gateway.Send(ctx, job.endpoint, job.envelope)
}

func (r *Runtime) sendLiveActivityRetryAttempt(
	ctx context.Context,
	endpoint DeviceEndpoint,
	envelope PushEnvelope,
) ProviderResult {
	event := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(envelope.Type)), "liveactivity.")
	if event != "start" && event != "end" {
		return ProviderResult{
			Provider:     "apns",
			ErrorCode:    "invalid_live_activity_event",
			ErrorMessage: "unsupported live activity retry event",
		}
	}
	state := executionStateFromLiveActivityEnvelope(envelope)
	token := ""
	usingPushToStart := event == "start"
	if usingPushToStart && r.repo != nil {
		terminal, err := r.repo.IsRunTerminal(
			ctx,
			endpoint.SpaceID,
			endpoint.DeviceID,
			envelope.RunID,
		)
		if err != nil {
			return ProviderResult{
				Provider:     "apns",
				ErrorCode:    "repository_error",
				ErrorMessage: err.Error(),
			}
		}
		if terminal {
			return ProviderResult{
				Provider:     "apns",
				ErrorCode:    "live_activity_run_terminal",
				ErrorMessage: "live activity start superseded by terminal run state",
			}
		}
	}
	if usingPushToStart {
		token = strings.TrimSpace(endpoint.LiveActivityPushToStartToken)
	} else if r.repo != nil {
		if live, err := r.repo.GetLiveActivityToken(
			ctx,
			endpoint.SpaceID,
			endpoint.DeviceID,
			envelope.RunID,
		); err == nil {
			token = strings.TrimSpace(live.UpdateToken)
		}
	}
	if token == "" {
		return ProviderResult{
			Provider:     "apns",
			ErrorCode:    "live_activity_token_pending",
			ErrorMessage: "live activity token is not available yet",
		}
	}
	result := r.gateway.SendLiveActivity(ctx, endpoint, token, state, event)
	if result.Accepted && event == "start" {
		r.recordLiveActivityStart(endpoint)
	}
	if result.Accepted && event == "end" && r.repo != nil {
		_ = r.repo.EndLiveActivity(
			ctx,
			endpoint.SpaceID,
			endpoint.DeviceID,
			envelope.RunID,
			envelope.Revision,
		)
	}
	if result.InvalidToken && r.repo != nil {
		if usingPushToStart {
			_ = r.repo.ClearLiveActivityPushToStartToken(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
			)
		} else {
			_ = r.repo.EndLiveActivity(
				ctx,
				endpoint.SpaceID,
				endpoint.DeviceID,
				envelope.RunID,
				envelope.Revision,
			)
		}
	}
	return result
}

func (r *Runtime) deliver(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope) {
	if envelope.TTL <= 0 {
		envelope.TTL = 15 * time.Minute
	}
	if envelope.NotificationID == "" {
		envelope.NotificationID = uuid.NewString()
	}
	if envelope.SpaceID == "" {
		envelope.SpaceID = endpoint.SpaceID
	}
	encoded, err := json.Marshal(envelope)
	if err == nil && r.repo != nil {
		now := time.Now().UTC()
		row := &NotificationOutbox{
			ID:             uuid.NewString(),
			NotificationID: envelope.NotificationID,
			SpaceID:        envelope.SpaceID,
			DeviceID:       endpoint.DeviceID,
			Type:           envelope.Type,
			RunID:          envelope.RunID,
			EnvelopeJSON:   string(encoded),
			Status:         "queued",
			NextAttemptAt:  now,
			ExpiresAt:      now.Add(envelope.TTL),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if createErr := r.repo.CreateOutbox(ctx, row); createErr == nil {
			switch row.Status {
			case "queued":
				r.enqueueDelivery(deliveryJob{outboxID: row.ID})
			case "retry":
				if row.NextAttemptAt.IsZero() || !row.NextAttemptAt.After(now) {
					r.enqueueDelivery(deliveryJob{outboxID: row.ID})
				}
			}
			return
		}
	}
	r.enqueueDelivery(deliveryJob{endpoint: endpoint, envelope: envelope})
}

func (r *Runtime) enqueueDelivery(job deliveryJob) {
	select {
	case r.deliveryQueue <- job:
	default:
		go func() {
			select {
			case r.deliveryQueue <- job:
			case <-time.After(2 * time.Second):
				if job.outboxID != "" {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				result, retryCount := r.sendWithRetry(ctx, job)
				r.recordProviderResult(ctx, job.endpoint, job.envelope, result, retryCount)
			}
		}()
	}
}

func (r *Runtime) deliveryWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-r.deliveryQueue:
			if job.outboxID != "" {
				r.deliverOutbox(ctx, job.outboxID)
				continue
			}
			r.deliverWithRetry(ctx, job)
		}
	}
}

func (r *Runtime) outboxRecoveryLoop(ctx context.Context) {
	if r.repo == nil {
		return
	}
	_ = r.repo.RecoverOutbox(ctx)
	r.enqueueDueOutbox(ctx)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.repo.ExpireOutbox(ctx)
			r.enqueueDueOutbox(ctx)
		}
	}
}

func (r *Runtime) enqueueDueOutbox(ctx context.Context) {
	rows, err := r.repo.ListDueOutbox(ctx, 2000)
	if err != nil {
		return
	}
	for _, row := range rows {
		r.enqueueDelivery(deliveryJob{outboxID: row.ID})
	}
}

func (r *Runtime) deliverOutbox(ctx context.Context, id string) {
	row, err := r.repo.ClaimOutbox(ctx, id)
	if err != nil {
		return
	}
	if !row.ExpiresAt.IsZero() && !row.ExpiresAt.After(time.Now().UTC()) {
		_ = r.repo.UpdateOutbox(ctx, row.ID, map[string]any{
			"status":     "failed",
			"last_error": "notification TTL expired",
		})
		return
	}
	endpoint, err := r.repo.GetDevice(ctx, row.SpaceID, row.DeviceID)
	if err != nil {
		_ = r.repo.UpdateOutbox(ctx, row.ID, map[string]any{
			"status":     "failed",
			"last_error": "notification device is unavailable or revoked",
		})
		return
	}
	var envelope PushEnvelope
	if err := json.Unmarshal([]byte(row.EnvelopeJSON), &envelope); err != nil {
		_ = r.repo.UpdateOutbox(ctx, row.ID, map[string]any{
			"status":     "failed",
			"last_error": "invalid notification envelope",
		})
		return
	}
	job := deliveryJob{endpoint: *endpoint, envelope: envelope}
	result, retryCount := r.sendWithRetry(ctx, job)
	if ctx.Err() != nil {
		_ = r.repo.UpdateOutbox(context.Background(), row.ID, map[string]any{
			"status":          "queued",
			"next_attempt_at": time.Now().UTC(),
		})
		return
	}
	r.recordProviderResult(ctx, *endpoint, envelope, result, retryCount)
	attemptCount := row.AttemptCount + retryCount + 1
	status := "failed"
	nextAttemptAt := time.Now().UTC()
	if result.Accepted {
		status = "accepted"
	} else if retryableProviderResult(result) && attemptCount < 6 {
		nextAttemptAt = time.Now().UTC().Add(time.Duration(1<<minInt(attemptCount, 6)) * time.Second)
		if row.ExpiresAt.IsZero() || nextAttemptAt.Before(row.ExpiresAt) {
			status = "retry"
		}
	}
	_ = r.repo.UpdateOutbox(ctx, row.ID, map[string]any{
		"status":              status,
		"provider":            result.Provider,
		"provider_message_id": result.ProviderMessageID,
		"attempt_count":       attemptCount,
		"last_error":          truncateRunes(result.ErrorMessage, 1024),
		"next_attempt_at":     nextAttemptAt,
	})
}

func (r *Runtime) deliverWithRetry(ctx context.Context, job deliveryJob) {
	result, retryCount := r.sendWithRetry(ctx, job)
	if ctx.Err() != nil {
		return
	}
	r.recordProviderResult(ctx, job.endpoint, job.envelope, result, retryCount)
}

func (r *Runtime) sendWithRetry(ctx context.Context, job deliveryJob) (ProviderResult, int) {
	envelope := job.envelope
	deadline := time.Now().Add(envelope.TTL)
	var result ProviderResult
	retryCount := 0
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			retryCount = attempt
			delay := time.Duration(1<<(attempt-1)) * time.Second
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, retryCount
			case <-timer.C:
			}
		}
		if time.Now().After(deadline) {
			result = ProviderResult{Provider: result.Provider, ErrorCode: "expired", ErrorMessage: "notification TTL expired"}
			break
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		result = r.sendDeliveryAttempt(attemptCtx, job)
		cancel()
		if result.Accepted || result.InvalidToken || !retryableProviderResult(result) {
			break
		}
	}
	return result, retryCount
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func retryableProviderResult(result ProviderResult) bool {
	switch result.ErrorCode {
	case "network_error", "request_failed", "live_activity_token_pending":
		return true
	}
	if strings.HasPrefix(result.ErrorCode, "http_") {
		code, err := strconv.Atoi(strings.TrimPrefix(result.ErrorCode, "http_"))
		return err == nil && (code == 408 || code == 425 || code == 429 || code >= 500)
	}
	return false
}

func (r *Runtime) recordProviderResult(ctx context.Context, endpoint DeviceEndpoint, envelope PushEnvelope, result ProviderResult, retryCount int) {
	now := time.Now().UTC()
	status := "failed"
	if result.Accepted {
		status = "accepted"
	}
	row := &DeliveryLog{
		ID:                uuid.NewString(),
		NotificationID:    envelope.NotificationID,
		SpaceID:           envelope.SpaceID,
		DeviceID:          endpoint.DeviceID,
		Provider:          result.Provider,
		Type:              envelope.Type,
		Status:            status,
		ProviderMessageID: result.ProviderMessageID,
		ErrorCode:         result.ErrorCode,
		ErrorMessage:      truncateRunes(result.ErrorMessage, 1024),
		RetryCount:        retryCount,
		QueuedAt:          now,
		ExpiresAt:         now.Add(envelope.TTL),
	}
	row.SentAt = &now
	if result.Accepted {
		row.AcceptedAt = &now
	}
	_ = r.repo.CreateDelivery(ctx, row)
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func payloadInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func executionStateForPreview(state ExecutionState, mode string) ExecutionState {
	projected := state
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "hidden":
		projected.Title = "Amitia"
		projected.Summary = ""
		projected.AgentID = ""
		projected.TotalTokens = 0
	case "sender_only":
		projected.Summary = ""
	}
	return projected
}

func localizeExecutionState(state ExecutionState, locale string) ExecutionState {
	lang := notificationLanguage(locale)
	if lang == "zh" {
		return state
	}
	if strings.TrimSpace(state.Title) == "Amitia 正在执行" {
		state.Title = executionLocaleText(lang, "title")
	}
	keys := map[string]string{
		"等待执行":        "queued",
		"正在启动":        "starting",
		"正在运行":        "running",
		"正在取消":        "cancelling",
		"等待工具执行":      "waiting_tool",
		"正在调用工具":      "calling_tool",
		"工具执行完成":      "tool_completed",
		"工具执行失败，正在处理": "tool_failed",
		"等待你的确认":      "waiting_approval",
		"已确认，继续执行":    "approved",
		"已拒绝，正在调整":    "denied",
		"已完成":         "completed",
		"执行失败":        "failed",
		"已中断":         "interrupted",
	}
	if key := keys[strings.TrimSpace(state.Summary)]; key != "" {
		state.Summary = executionLocaleText(lang, key)
	}
	return state
}

func notificationLanguage(locale string) string {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	switch {
	case strings.HasPrefix(value, "zh-hant"), strings.HasPrefix(value, "zh-tw"), strings.HasPrefix(value, "zh-hk"):
		return "zh-tw"
	case strings.HasPrefix(value, "zh"):
		return "zh"
	}
	for _, candidate := range []string{"ja", "ko", "fr", "es", "de", "pt", "ru", "ar"} {
		if strings.HasPrefix(value, candidate) {
			return candidate
		}
	}
	return "en"
}

func executionLocaleText(lang, key string) string {
	translations := map[string]map[string]string{
		"zh": {
			"title": "Amitia 正在执行", "queued": "等待执行", "starting": "正在启动",
			"running": "任务正在执行", "cancelling": "正在取消任务", "waiting_tool": "等待工具执行",
			"calling_tool": "正在调用工具", "tool_completed": "工具执行完成",
			"tool_failed": "工具执行失败，正在处理", "waiting_approval": "等待你的确认",
			"approved": "已确认，继续执行", "denied": "已拒绝，正在调整",
			"completed": "任务已完成", "failed": "任务执行失败", "interrupted": "任务已中断",
		},
		"en": {
			"title": "Amitia is working", "queued": "Waiting to run", "starting": "Starting",
			"running": "Running", "cancelling": "Cancelling", "waiting_tool": "Waiting for tool",
			"calling_tool": "Calling tool", "tool_completed": "Tool completed",
			"tool_failed": "Tool failed, recovering", "waiting_approval": "Waiting for your approval",
			"approved": "Approved, continuing", "denied": "Denied, adjusting",
			"completed": "Completed", "failed": "Execution failed", "interrupted": "Interrupted",
		},
		"zh-tw": {
			"title": "Amitia 正在執行", "queued": "等待執行", "starting": "正在啟動",
			"running": "正在執行", "cancelling": "正在取消", "waiting_tool": "等待工具執行",
			"calling_tool": "正在呼叫工具", "tool_completed": "工具執行完成",
			"tool_failed": "工具執行失敗，正在處理", "waiting_approval": "等待你的確認",
			"approved": "已確認，繼續執行", "denied": "已拒絕，正在調整",
			"completed": "已完成", "failed": "執行失敗", "interrupted": "已中斷",
		},
		"ja": {
			"title": "Amitia が実行中", "queued": "実行待ち", "starting": "開始中",
			"running": "実行中", "cancelling": "キャンセル中", "waiting_tool": "ツール待機中",
			"calling_tool": "ツールを呼び出し中", "tool_completed": "ツール実行完了",
			"tool_failed": "ツール失敗、処理中", "waiting_approval": "確認待ち",
			"approved": "確認済み、続行中", "denied": "拒否済み、調整中",
			"completed": "完了", "failed": "実行失敗", "interrupted": "中断",
		},
		"ko": {
			"title": "Amitia 실행 중", "queued": "실행 대기", "starting": "시작 중",
			"running": "실행 중", "cancelling": "취소 중", "waiting_tool": "도구 대기 중",
			"calling_tool": "도구 호출 중", "tool_completed": "도구 실행 완료",
			"tool_failed": "도구 실패, 처리 중", "waiting_approval": "확인 대기",
			"approved": "확인됨, 계속 진행", "denied": "거부됨, 조정 중",
			"completed": "완료", "failed": "실행 실패", "interrupted": "중단",
		},
		"fr": {
			"title": "Amitia travaille", "queued": "En attente", "starting": "Démarrage",
			"running": "En cours", "cancelling": "Annulation", "waiting_tool": "Attente de l’outil",
			"calling_tool": "Appel de l’outil", "tool_completed": "Outil terminé",
			"tool_failed": "Échec de l’outil, reprise", "waiting_approval": "En attente de votre confirmation",
			"approved": "Confirmé, poursuite", "denied": "Refusé, ajustement",
			"completed": "Terminé", "failed": "Échec de l’exécution", "interrupted": "Interrompu",
		},
		"es": {
			"title": "Amitia está trabajando", "queued": "En espera", "starting": "Iniciando",
			"running": "En ejecución", "cancelling": "Cancelando", "waiting_tool": "Esperando herramienta",
			"calling_tool": "Llamando herramienta", "tool_completed": "Herramienta completada",
			"tool_failed": "Falló la herramienta, recuperando", "waiting_approval": "Esperando tu confirmación",
			"approved": "Confirmado, continuando", "denied": "Rechazado, ajustando",
			"completed": "Completado", "failed": "Falló la ejecución", "interrupted": "Interrumpido",
		},
		"de": {
			"title": "Amitia arbeitet", "queued": "Wartet auf Ausführung", "starting": "Startet",
			"running": "Wird ausgeführt", "cancelling": "Wird abgebrochen", "waiting_tool": "Wartet auf Tool",
			"calling_tool": "Tool wird aufgerufen", "tool_completed": "Tool abgeschlossen",
			"tool_failed": "Tool fehlgeschlagen, Wiederherstellung", "waiting_approval": "Wartet auf Bestätigung",
			"approved": "Bestätigt, wird fortgesetzt", "denied": "Abgelehnt, wird angepasst",
			"completed": "Abgeschlossen", "failed": "Ausführung fehlgeschlagen", "interrupted": "Unterbrochen",
		},
		"pt": {
			"title": "Amitia está executando", "queued": "Aguardando execução", "starting": "Iniciando",
			"running": "Em execução", "cancelling": "Cancelando", "waiting_tool": "Aguardando ferramenta",
			"calling_tool": "Chamando ferramenta", "tool_completed": "Ferramenta concluída",
			"tool_failed": "Falha na ferramenta, recuperando", "waiting_approval": "Aguardando sua confirmação",
			"approved": "Confirmado, continuando", "denied": "Recusado, ajustando",
			"completed": "Concluído", "failed": "Falha na execução", "interrupted": "Interrompido",
		},
		"ru": {
			"title": "Amitia выполняет задачу", "queued": "Ожидание запуска", "starting": "Запуск",
			"running": "Выполняется", "cancelling": "Отмена", "waiting_tool": "Ожидание инструмента",
			"calling_tool": "Вызов инструмента", "tool_completed": "Инструмент завершён",
			"tool_failed": "Ошибка инструмента, восстановление", "waiting_approval": "Ожидание подтверждения",
			"approved": "Подтверждено, продолжаем", "denied": "Отклонено, корректировка",
			"completed": "Готово", "failed": "Ошибка выполнения", "interrupted": "Прервано",
		},
		"ar": {
			"title": "Amitia قيد التنفيذ", "queued": "بانتظار التنفيذ", "starting": "جارٍ البدء",
			"running": "قيد التنفيذ", "cancelling": "جارٍ الإلغاء", "waiting_tool": "بانتظار الأداة",
			"calling_tool": "جارٍ استدعاء الأداة", "tool_completed": "اكتمل تنفيذ الأداة",
			"tool_failed": "فشلت الأداة، جارٍ الاسترداد", "waiting_approval": "بانتظار تأكيدك",
			"approved": "تم التأكيد، جارٍ المتابعة", "denied": "تم الرفض، جارٍ التعديل",
			"completed": "مكتمل", "failed": "فشل التنفيذ", "interrupted": "متوقف",
		},
	}
	if values := translations[lang]; values != nil {
		if value := values[key]; value != "" {
			return value
		}
	}
	return translations["en"][key]
}

func executionNotificationBody(state ExecutionState, locale string) string {
	if summary := BuildPreview(state.Summary, 120); summary != "" {
		return summary
	}
	lang := notificationLanguage(locale)
	key := "running"
	switch strings.ToLower(strings.TrimSpace(state.Phase)) {
	case "queued":
		key = "queued"
	case "starting":
		key = "starting"
	case "completed":
		key = "completed"
	case "failed":
		key = "failed"
	case "cancelled", "interrupted":
		key = "interrupted"
	case "waiting_approval":
		key = "waiting_approval"
	case "waiting_tool":
		key = "waiting_tool"
	case "cancelling":
		key = "cancelling"
	}
	value := executionLocaleText(lang, key)
	if state.TotalSteps > 0 && key == "running" {
		return fmt.Sprintf("%s · %d/%d", value, state.CurrentStep, state.TotalSteps)
	}
	return value
}

func executionDeepLink(state ExecutionState) string {
	if strings.TrimSpace(state.ConversationID) != "" {
		return fmt.Sprintf("amitia://chat/%s?run=%s", state.ConversationID, state.RunID)
	}
	return "amitia://run/" + state.RunID
}

func fallback(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
