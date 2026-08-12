package screenframe

import (
	"context"
	"testing"
	"time"
)

func TestStartHandler_CapabilityID(t *testing.T) {
	h := NewStartHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("capability id must not be empty")
	}
}

func TestStartHandler_BuildPayload(t *testing.T) {
	p := DefaultScreenFramePolicy()
	h := NewStartHandler(NewBlockedSessionStore(1), p)
	req := StartRequest{}

	payload, err := h.BuildPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload["displayId"] != 0 {
		t.Errorf("expected displayId 0, got %v", payload["displayId"])
	}
	if payload["targetFps"] != p.DefaultFPS {
		t.Errorf("expected fps %v, got %v", p.DefaultFPS, payload["targetFps"])
	}
	if payload["maxWidth"] != p.MaxWidth {
		t.Errorf("expected maxWidth %v, got %v", p.MaxWidth, payload["maxWidth"])
	}
	if payload["maxHeight"] != p.MaxHeight {
		t.Errorf("expected maxHeight %v, got %v", p.MaxHeight, payload["maxHeight"])
	}
}

func TestStartHandler_BuildPayload_Custom(t *testing.T) {
	p := DefaultScreenFramePolicy()
	h := NewStartHandler(NewBlockedSessionStore(1), p)

	fps := 5.0
	w := 800
	hReq := 600
	req := StartRequest{TargetFPS: &fps, MaxWidth: &w, MaxHeight: &hReq}

	payload, err := h.BuildPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload["targetFps"] != 5.0 {
		t.Errorf("expected fps 5.0, got %v", payload["targetFps"])
	}
	if payload["maxWidth"] != 800 {
		t.Errorf("expected maxWidth 800, got %v", payload["maxWidth"])
	}
	if payload["maxHeight"] != 600 {
		t.Errorf("expected maxHeight 600, got %v", payload["maxHeight"])
	}
}

func TestStartHandler_Handle_Blocked(t *testing.T) {
	ctx := context.Background()
	h := NewStartHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	req := StartRequest{}
	result, err := h.Handle(ctx, owner, marshalRequest(req))
	if err == nil {
		t.Fatal("expected error when native host unavailable")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrBlockedNativeHost {
		t.Errorf("expected ErrBlockedNativeHost, got %v", serr.Code)
	}
	var zero StartResult
	if result != zero {
		t.Errorf("expected zero result, got %+v", result)
	}
}

func TestLatestHandler_CapabilityID(t *testing.T) {
	h := NewLatestHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("capability id must not be empty")
	}
}

func TestLatestHandler_Handle_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	h := NewLatestHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	req := LatestRequest{SessionID: "nonexistent"}
	_, err := h.Handle(ctx, owner, marshalRequest(req))
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", serr.Code)
	}
}

func TestLatestHandler_Handle_WrongOwner(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)
	h := NewLatestHandler(store, DefaultScreenFramePolicy())
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("s-1"),
		Owner: SessionOwner{UserID: "other-user"},
		State: SessionStateRunning,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[session.ID] = &session

	req := LatestRequest{SessionID: "s-1"}
	_, err := h.Handle(ctx, owner, marshalRequest(req))
	if err == nil {
		t.Fatal("expected error for wrong owner")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", serr.Code)
	}
}

func TestStopHandler_CapabilityID(t *testing.T) {
	h := NewStopHandler(NewBlockedSessionStore(1))
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("capability id must not be empty")
	}
}

func TestStopHandler_Handle_NotFound(t *testing.T) {
	ctx := context.Background()
	h := NewStopHandler(NewBlockedSessionStore(1))
	owner := SessionOwner{UserID: "u1"}

	_, err := h.Handle(ctx, owner, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestStopHandler_Handle_WrongConversation(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)
	h := NewStopHandler(store)
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("s-1"),
		Owner: SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v2"},
		State: SessionStateRunning,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[session.ID] = &session

	_, err := h.Handle(ctx, owner, "s-1")
	if err == nil {
		t.Fatal("expected error for wrong conversation scope")
	}
}

func TestStopHandler_Handle_AlreadyStopped(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)
	h := NewStopHandler(store)
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("s-1"),
		Owner: owner,
		State: SessionStateStopped,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[session.ID] = &session

	result, err := h.Handle(ctx, owner, "s-1")
	if err != nil {
		t.Fatalf("unexpected error for already-stopped session: %v", err)
	}
	if result.SessionID != "s-1" {
		t.Errorf("expected session id s-1, got %v", result.SessionID)
	}
	if result.State != SessionStateStopped {
		t.Errorf("expected state stopped, got %v", result.State)
	}
}

func TestStatusHandler_CapabilityID(t *testing.T) {
	h := NewStatusHandler(NewBlockedSessionStore(1))
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("capability id must not be empty")
	}
}

func TestStatusHandler_Handle_Blocked(t *testing.T) {
	ctx := context.Background()
	h := NewStatusHandler(NewBlockedSessionStore(1))
	owner := SessionOwner{UserID: "u1"}

	result, err := h.Handle(ctx, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Supported {
		t.Error("expected supported=false when native host missing")
	}
	if !result.UserActionRequired {
		t.Error("expected user action required")
	}
	if result.State != "native_host_missing" {
		t.Errorf("expected state native_host_missing, got %v", result.State)
	}
}

func TestStatusHandler_Handle_ActiveSession(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionstore)
	h := NewStatusHandler(store)
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	session := ScreenFrameSession{
		ID:              ScreenFrameSessionID("s-1"),
		Owner:           owner,
		State:           SessionStateRunning,
		DisplayID:       0,
		Width:           1080,
		Height:          2400,
		TargetFPS:       2,
		StartedAt:       time.Now(),
	}
	store.sessions[session.ID] = &session

	result, err := h.Handle(ctx, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ActiveSession {
		t.Error("expected active session in status")
	}
	if result.SessionID != "s-1" {
		t.Errorf("expected session id s-1, got %v", result.SessionID)
	}
	if result.Width != 1080 {
		t.Errorf("expected width 1080, got %v", result.Width)
	}
	if result.Height != 2400 {
		t.Errorf("expected height 2400, got %v", result.Height)
	}
}

func marshalRequest(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
