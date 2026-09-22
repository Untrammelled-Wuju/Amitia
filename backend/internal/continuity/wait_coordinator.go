package continuity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type WakeDispatcher interface {
	DispatchContinuityWake(context.Context, WakeRequest) error
}

type WaitCoordinatorConfig struct {
	PollInterval            time.Duration
	BatchSize               int
	MaxBackoff              time.Duration
	MaxConcurrentDispatches int
}

func DefaultWaitCoordinatorConfig() WaitCoordinatorConfig {
	return WaitCoordinatorConfig{PollInterval: 5 * time.Second, BatchSize: 100, MaxBackoff: 30 * time.Minute, MaxConcurrentDispatches: 4}
}

type WaitCoordinator struct {
	repo       *Repository
	dispatcher WakeDispatcher
	cfg        WaitCoordinatorConfig

	startOnce sync.Once
}

func NewWaitCoordinator(repo *Repository, dispatcher WakeDispatcher, cfg WaitCoordinatorConfig) *WaitCoordinator {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Minute
	}
	if cfg.MaxConcurrentDispatches <= 0 {
		cfg.MaxConcurrentDispatches = 4
	}
	return &WaitCoordinator{repo: repo, dispatcher: dispatcher, cfg: cfg}
}

func (c *WaitCoordinator) SetDispatcher(dispatcher WakeDispatcher) { c.dispatcher = dispatcher }

func (c *WaitCoordinator) Start(ctx context.Context) {
	if c == nil || c.repo == nil {
		return
	}
	c.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(c.cfg.PollInterval)
			defer ticker.Stop()
			_ = c.Tick(ctx)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = c.Tick(ctx)
				}
			}
		}()
	})
}

func (c *WaitCoordinator) Tick(ctx context.Context) error {
	if c == nil || c.repo == nil {
		return nil
	}
	now := time.Now().UTC()
	due, err := c.repo.ListDueTimeWaits(now, c.cfg.BatchSize)
	if err != nil {
		return err
	}
	for i := range due {
		_, _ = c.ResolveWait(ctx, due[i].ID, "time", map[string]any{"dueAt": due[i].DueAt}, true)
	}
	pending, err := c.repo.ListPendingWakes(time.Now().UTC(), c.cfg.BatchSize)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, c.cfg.MaxConcurrentDispatches)
	var wg sync.WaitGroup
	for i := range pending {
		wg.Add(1)
		sem <- struct{}{}
		go func(wait Wait) {
			defer wg.Done()
			defer func() { <-sem }()
			_ = c.dispatchWake(ctx, &wait)
		}(pending[i])
	}
	wg.Wait()
	return nil
}

func (c *WaitCoordinator) ResolveInline(ctx context.Context, waitID, resolvedBy string, resolution map[string]any) (*Wait, error) {
	return c.ResolveWait(ctx, waitID, resolvedBy, resolution, false)
}

func (c *WaitCoordinator) ResolveWait(ctx context.Context, waitID, resolvedBy string, resolution map[string]any, scheduleWake bool) (*Wait, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("continuity: wait coordinator is unavailable")
	}
	var result *Wait
	err := c.repo.WithTransaction(func(repo *Repository) error {
		var innerErr error
		result, innerErr = c.resolveWait(ctx, repo, waitID, resolvedBy, resolution, scheduleWake)
		return innerErr
	})
	return result, err
}

func (c *WaitCoordinator) resolveWait(ctx context.Context, repo *Repository, waitID, resolvedBy string, resolution map[string]any, scheduleWake bool) (*Wait, error) {
	if c == nil || repo == nil {
		return nil, errors.New("continuity: wait coordinator is unavailable")
	}
	waitID = strings.TrimSpace(waitID)
	if waitID == "" {
		return nil, errors.New("continuity: waitId is required")
	}
	var result *Wait
	wait, err := repo.GetWait(waitID)
	if err != nil || wait == nil {
		return nil, err
	}
	if wait.Status != WaitStatusWaiting {
		return wait, nil
	}
	thread, err := repo.GetThread(wait.ThreadID, "")
	if err != nil || thread == nil {
		return nil, err
	}
	now := time.Now().UTC()
	resolutionJSON, _ := json.Marshal(resolution)
	if err := repo.UpdateWait(wait.ID, map[string]interface{}{
		"status": WaitStatusResolved, "resolved_at": now, "resolved_by": strings.TrimSpace(resolvedBy),
		"resolution_json": string(resolutionJSON), "wake_state": WakeStateSkipped, "wake_request_id": "",
		"next_wake_at": nil, "last_wake_error": "",
	}); err != nil {
		return nil, err
	}

	open, err := repo.ListOpenWaits(thread.ID, 1)
	if err != nil {
		return nil, err
	}
	if len(open) == 0 {
		for attempt := 0; attempt < 3 && thread != nil && (thread.Status == ThreadStatusWaiting || thread.Status == ThreadStatusBlocked); attempt++ {
			updated, updateErr := repo.UpdateThreadCAS(thread.ID, thread.Revision, map[string]interface{}{"status": ThreadStatusActive, "last_active_at": now})
			if updateErr == nil {
				thread = updated
				break
			}
			if !errors.Is(updateErr, gorm.ErrRecordNotFound) {
				return nil, updateErr
			}
			thread, updateErr = repo.GetThread(thread.ID, "")
			if updateErr != nil {
				return nil, updateErr
			}
		}
		if scheduleWake && wait.AutoResume && thread != nil && !thread.Status.IsTerminal() && thread.Status != ThreadStatusPaused {
			wakeRequestID := "continuity-wake-" + wait.ID
			next := now
			if err := repo.UpdateWait(wait.ID, map[string]interface{}{
				"wake_state": WakeStatePending, "wake_request_id": wakeRequestID, "next_wake_at": &next,
			}); err != nil {
				return nil, err
			}
		}
	}

	wait, err = repo.GetWait(wait.ID)
	if err != nil {
		return nil, err
	}
	result = wait
	payload, _ := json.Marshal(map[string]any{"waitId": wait.ID, "waitType": wait.WaitType, "resolvedBy": resolvedBy, "resolution": resolution})
	_, err = repo.AppendEvent(&ThreadEvent{ThreadID: thread.ID, EventType: "wait.resolved", SourceType: strings.TrimSpace(resolvedBy), SourceID: wait.ID, PayloadJSON: string(payload), IdempotencyKey: "wait-resolved|" + wait.ID, OccurredAt: now})
	return result, err
}

func (c *WaitCoordinator) CancelWait(ctx context.Context, waitID, source string) (*Wait, error) {
	_ = ctx
	if c == nil || c.repo == nil {
		return nil, errors.New("continuity: wait coordinator is unavailable")
	}
	var result *Wait
	err := c.repo.WithTransaction(func(repo *Repository) error {
		var innerErr error
		result, innerErr = c.cancelWait(repo, waitID, source)
		return innerErr
	})
	return result, err
}

func (c *WaitCoordinator) cancelWait(repo *Repository, waitID, source string) (*Wait, error) {
	if c == nil || repo == nil {
		return nil, errors.New("continuity: wait coordinator is unavailable")
	}
	wait, err := repo.GetWait(waitID)
	if err != nil || wait == nil {
		return nil, err
	}
	if wait.Status != WaitStatusWaiting {
		return wait, nil
	}
	now := time.Now().UTC()
	if err := repo.UpdateWait(wait.ID, map[string]interface{}{"status": WaitStatusCancelled, "resolved_at": now, "resolved_by": source, "wake_state": WakeStateSkipped, "next_wake_at": nil}); err != nil {
		return nil, err
	}
	thread, err := repo.GetThread(wait.ThreadID, "")
	if err != nil {
		return nil, err
	}
	if thread != nil && !thread.Status.IsTerminal() && thread.Status != ThreadStatusPaused {
		open, listErr := repo.ListOpenWaits(thread.ID, 1)
		if listErr != nil {
			return nil, listErr
		}
		if len(open) == 0 && (thread.Status == ThreadStatusWaiting || thread.Status == ThreadStatusBlocked) {
			for attempt := 0; attempt < 3; attempt++ {
				updated, updateErr := repo.UpdateThreadCAS(thread.ID, thread.Revision, map[string]interface{}{"status": ThreadStatusActive, "last_active_at": now})
				if updateErr == nil {
					thread = updated
					break
				}
				if !errors.Is(updateErr, gorm.ErrRecordNotFound) {
					return nil, updateErr
				}
				thread, updateErr = repo.GetThread(thread.ID, "")
				if updateErr != nil || thread == nil || thread.Status != ThreadStatusWaiting && thread.Status != ThreadStatusBlocked {
					break
				}
			}
		}
	}
	payload, _ := json.Marshal(map[string]any{"waitId": wait.ID, "waitType": wait.WaitType, "cancelledBy": source})
	_, _ = repo.AppendEvent(&ThreadEvent{ThreadID: wait.ThreadID, EventType: "wait.cancelled", SourceType: strings.TrimSpace(source), SourceID: wait.ID, PayloadJSON: string(payload), IdempotencyKey: "wait-cancelled|" + wait.ID, OccurredAt: now})
	return repo.GetWait(wait.ID)
}

func (c *WaitCoordinator) Signal(ctx context.Context, signal Signal) ([]Wait, error) {
	if c == nil || c.repo == nil || !ValidWaitType(signal.WaitType) {
		return nil, nil
	}
	signal.SpaceID = strings.TrimSpace(signal.SpaceID)
	if signal.OccurredAt.IsZero() {
		signal.OccurredAt = time.Now().UTC()
	}
	resolved := make([]Wait, 0)
	matchedWaits := make([]Wait, 0, c.cfg.BatchSize)
	offset := 0
	for len(matchedWaits) < c.cfg.BatchSize {
		page, err := c.repo.ListWaitingWaitsByType(signal.WaitType, signal.SpaceID, offset, c.cfg.BatchSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for i := range page {
			if waitMatchesSignal(page[i], signal) {
				matchedWaits = append(matchedWaits, page[i])
				if len(matchedWaits) == c.cfg.BatchSize {
					break
				}
			}
		}
		if len(page) < c.cfg.BatchSize {
			break
		}
		offset += len(page)
	}
	for i := range matchedWaits {
		result, err := c.ResolveWait(ctx, matchedWaits[i].ID, firstNonEmpty(signal.Source, signal.WaitType), map[string]any{"sourceId": signal.SourceID, "attributes": signal.Attributes, "occurredAt": signal.OccurredAt}, true)
		if err != nil {
			return resolved, err
		}
		if result != nil {
			resolved = append(resolved, *result)
		}
	}
	return resolved, nil
}

func waitMatchesSignal(wait Wait, signal Signal) bool {
	if wait.WaitType != signal.WaitType {
		return false
	}
	condition := map[string]any{}
	if strings.TrimSpace(wait.ConditionJSON) != "" && strings.TrimSpace(wait.ConditionJSON) != "{}" {
		if err := json.Unmarshal([]byte(wait.ConditionJSON), &condition); err != nil {
			return false
		}
	}
	if wait.WaitType == WaitTypeTime {
		return wait.DueAt != nil && !wait.DueAt.After(signal.OccurredAt)
	}
	attrs := signal.Attributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	matchKeys := make([]string, 0)
	switch wait.WaitType {
	case WaitTypeDevice:
		matchKeys = []string{"deviceId"}
	case WaitTypeApproval:
		matchKeys = []string{"approvalId", "requestId", "toolCallId"}
	case WaitTypeDependency:
		matchKeys = []string{"executionId", "workflowRunId", "taskRunId", "operationId", "invocationId"}
	case WaitTypeExternal:
		for key := range condition {
			if key != "kind" && key != "autoResume" && key != "resumeHint" {
				matchKeys = append(matchKeys, key)
			}
		}
		// Unscoped external waits must never be unlocked by arbitrary events.
		if len(matchKeys) == 0 {
			return false
		}
	case WaitTypeUser:
		return false
	}
	matchedSpecific := false
	for _, key := range matchKeys {
		expected, ok := condition[key]
		if !ok || strings.TrimSpace(fmt.Sprint(expected)) == "" {
			continue
		}
		matchedSpecific = true
		actual, ok := attrs[key]
		if !ok && key == "sourceId" {
			actual, ok = signal.SourceID, signal.SourceID != ""
		}
		if !ok || fmt.Sprint(actual) != fmt.Sprint(expected) {
			return false
		}
	}
	return matchedSpecific
}

func (c *WaitCoordinator) dispatchWake(ctx context.Context, wait *Wait) error {
	if wait == nil || c.dispatcher == nil {
		return nil
	}
	thread, err := c.repo.GetThread(wait.ThreadID, "")
	if err != nil || thread == nil {
		return err
	}
	if thread.Status.IsTerminal() || thread.Status == ThreadStatusPaused {
		return c.repo.UpdateWait(wait.ID, map[string]interface{}{"wake_state": WakeStateSkipped, "next_wake_at": nil, "last_wake_error": "thread is terminal or paused"})
	}
	conversationID, _ := c.repo.MostRecentBindingID(thread.ID, "conversation")
	channel := "web"
	peerID := ""
	if route, routeErr := c.repo.GetConversationRoute(conversationID); routeErr == nil && route != nil {
		if strings.TrimSpace(route.Channel) != "" {
			channel = strings.TrimSpace(route.Channel)
		}
		peerID = strings.TrimSpace(route.PeerID)
	}
	requestID := strings.TrimSpace(wait.WakeRequestID)
	if requestID == "" {
		requestID = "continuity-wake-" + wait.ID
	}
	message := "持续事项中的等待条件已经满足：" + strings.TrimSpace(wait.Description) + "。请基于当前事项状态重新评估下一步；可安全继续的步骤直接继续，需要用户决定时再联系用户。"
	request := WakeRequest{WaitID: wait.ID, ThreadID: thread.ID, SpaceID: thread.SpaceID, CharacterID: thread.CharacterID, ConversationID: conversationID, Channel: channel, PeerID: peerID, RequestID: requestID, Message: message, ResumeHint: wait.ResumeHint, ResolvedBy: wait.ResolvedBy}
	if err := c.dispatcher.DispatchContinuityWake(ctx, request); err != nil {
		attempts := wait.WakeAttempts + 1
		backoff := time.Duration(math.Pow(2, math.Min(float64(attempts), 10))) * time.Second
		if backoff > c.cfg.MaxBackoff {
			backoff = c.cfg.MaxBackoff
		}
		next := time.Now().UTC().Add(backoff)
		_ = c.repo.UpdateWait(wait.ID, map[string]interface{}{"wake_state": WakeStateFailed, "wake_attempts": attempts, "next_wake_at": next, "last_wake_error": shortText(err.Error(), 500)})
		return err
	}
	now := time.Now().UTC()
	return c.repo.UpdateWait(wait.ID, map[string]interface{}{"wake_state": WakeStateDelivered, "wake_attempts": wait.WakeAttempts + 1, "next_wake_at": nil, "last_wake_error": "", "wake_delivered_at": now})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "continuity"
}
