package screenframe

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type StartHandler struct {
	capabilityID capability.CapabilityID
	store        ScreenFrameSessionStore
	policy       ScreenFramePolicy
}

func NewStartHandler(store ScreenFrameSessionStore, policy ScreenFramePolicy) *StartHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media_screen_frame",
			"start",
		),
	)
	return &StartHandler{capabilityID: id, store: store, policy: policy}
}

func (h *StartHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *StartHandler) BuildPayload(request StartRequest) (map[string]any, error) {
	if err := request.Validate(h.policy); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"displayId": request.ResolveDisplayID(),
		"targetFps": request.ResolveFPS(h.policy),
		"maxWidth":  request.ResolveMaxWidth(h.policy),
		"maxHeight": request.ResolveMaxHeight(h.policy),
	}
	return payload, nil
}

func (h *StartHandler) Handle(ctx context.Context, owner SessionOwner, rawRequest json.RawMessage) (StartResult, error) {
	var req StartRequest
	if err := json.Unmarshal(rawRequest, &req); err != nil {
		return StartResult{}, NewFrameError(ErrInvalidFPS, "invalid start request: "+err.Error())
	}
	if err := req.Validate(h.policy); err != nil {
		return StartResult{}, err
	}

	session := ScreenFrameSession{
		ID:        ScreenFrameSessionID("blocked"),
		Owner:     owner,
		DisplayID: req.ResolveDisplayID(),
		Width:     req.ResolveMaxWidth(h.policy),
		Height:    req.ResolveMaxHeight(h.policy),
		TargetFPS: req.ResolveFPS(h.policy),
		State:     SessionStateStarting,
	}

	_, err := h.store.Create(ctx, session)
	if err != nil {
		return StartResult{}, err
	}

	return StartResult{}, NewFrameError(ErrBlockedNativeHost, "android native host source not available; screen frame capture blocked")
}

type LatestHandler struct {
	capabilityID capability.CapabilityID
	store        ScreenFrameSessionStore
	policy       ScreenFramePolicy
}

func NewLatestHandler(store ScreenFrameSessionStore, policy ScreenFramePolicy) *LatestHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media_screen_frame",
			"latest",
		),
	)
	return &LatestHandler{capabilityID: id, store: store, policy: policy}
}

func (h *LatestHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *LatestHandler) BuildPayload(request LatestRequest) (map[string]any, error) {
	session, err := h.store.Get(context.Background(), request.SessionID)
	if err != nil {
		return nil, err
	}
	if !session.Active() {
		return nil, NewFrameError(ErrSessionNotRunning, "frame capture session not running")
	}

	payload := map[string]any{
		"sessionId": string(request.SessionID),
		"format":    string(request.ResolveFormat()),
	}

	if request.AfterSequence != nil {
		payload["afterSequence"] = *request.AfterSequence
	}
	if request.Quality != nil {
		payload["quality"] = *request.Quality
	}
	if request.MaxWidth != nil {
		payload["maxWidth"] = *request.MaxWidth
	}
	if request.MaxHeight != nil {
		payload["maxHeight"] = *request.MaxHeight
	}
	payload["waitMs"] = int(request.WaitDuration().Milliseconds())

	return payload, nil
}

func (h *LatestHandler) Handle(ctx context.Context, owner SessionOwner, rawRequest json.RawMessage) (LatestResult, error) {
	var req LatestRequest
	if err := json.Unmarshal(rawRequest, &req); err != nil {
		return LatestResult{}, NewFrameError(ErrInvalidFPS, "invalid latest request: "+err.Error())
	}

	session, err := h.store.Get(ctx, req.SessionID)
	if err != nil {
		return LatestResult{}, err
	}
	if !session.Active() {
		return LatestResult{}, NewFrameError(ErrSessionNotRunning, "frame capture session not running")
	}
	if session.Owner.UserID != owner.UserID {
		return LatestResult{}, NewFrameError(ErrSessionNotFound, "frame capture session not found for this user")
	}
	if session.Owner.ConversationID != owner.ConversationID {
		return LatestResult{}, NewFrameError(ErrSessionNotFound, "frame capture session not scoped to this conversation")
	}

	return LatestResult{}, NewFrameError(ErrBlockedNativeHost, "android native host source not available; screen frame latest blocked")
}

type StopHandler struct {
	capabilityID capability.CapabilityID
	store        ScreenFrameSessionStore
}

func NewStopHandler(store ScreenFrameSessionStore) *StopHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media_screen_frame",
			"stop",
		),
	)
	return &StopHandler{capabilityID: id, store: store}
}

func (h *StopHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *StopHandler) Handle(ctx context.Context, owner SessionOwner, sessionID ScreenFrameSessionID) (StopResult, error) {
	session, err := h.store.Get(ctx, sessionID)
	if err != nil {
		return StopResult{}, err
	}
	if session.Owner.UserID != owner.UserID {
		return StopResult{}, NewFrameError(ErrSessionNotFound, "frame capture session not found for this user")
	}
	if session.Owner.ConversationID != owner.ConversationID {
		return StopResult{}, NewFrameError(ErrSessionNotFound, "frame capture session not scoped to this conversation")
	}

	if !session.Active() {
		return StopResult{SessionID: sessionID, State: session.State}, nil
	}

	if err := h.store.UpdateState(ctx, sessionID, SessionStateStopped, 0); err != nil {
		return StopResult{}, err
	}
	if session.cancel != nil {
		session.cancel()
	}

	return StopResult{SessionID: sessionID, State: SessionStateStopped}, nil
}

type StatusHandler struct {
	capabilityID capability.CapabilityID
	store        ScreenFrameSessionStore
}

func NewStatusHandler(store ScreenFrameSessionStore) *StatusHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media_screen_frame",
			"status",
		),
	)
	return &StatusHandler{capabilityID: id, store: store}
}

func (h *StatusHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *StatusHandler) Handle(ctx context.Context, owner SessionOwner) (StatusResult, error) {
	sessions, err := h.store.ListByUser(ctx, owner.UserID)
	if err != nil {
		return StatusResult{}, err
	}

	var active *ScreenFrameSession
	for _, sess := range sessions {
		if sess.Active() {
			active = sess
			break
		}
	}

	result := StatusResult{
		Supported:          false,
		PermissionState:    "native_host_unavailable",
		ActiveSession:      false,
		UserActionRequired: true,
		State:              "native_host_missing",
	}

	if active != nil {
		result.ActiveSession = true
		result.SessionID = string(active.ID)
		result.DisplayID = active.DisplayID
		result.Width = active.Width
		result.Height = active.Height
		result.TargetFPS = active.TargetFPS
		result.LastFrameSequence = active.LastFrameSequence
		result.State = string(active.State)
		if !active.LastFrameAt.IsZero() {
			result.LastFrameAt = active.LastFrameAt.UnixMilli()
		}
	}

	return result, nil
}
