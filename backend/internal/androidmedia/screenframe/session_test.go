package screenframe

import (
	"context"
	"testing"
	"time"
)

func TestStartRequest_Validate_Default(t *testing.T) {
	p := DefaultScreenFramePolicy()
	req := StartRequest{}
	if err := req.Validate(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartRequest_Validate_InvalidFPS(t *testing.T) {
	p := DefaultScreenFramePolicy()

	negative := -1.0
	req := StartRequest{TargetFPS: &negative}
	if err := req.Validate(p); err == nil {
		t.Fatal("expected error for negative fps")
	}

	zero := 0.0
	req2 := StartRequest{TargetFPS: &zero}
	if err := req2.Validate(p); err == nil {
		t.Fatal("expected error for zero fps")
	}

	tooHigh := 15.0
	req3 := StartRequest{TargetFPS: &tooHigh}
	if err := req3.Validate(p); err == nil {
		t.Fatal("expected error for fps > max")
	}
}

func TestStartRequest_Validate_InvalidSize(t *testing.T) {
	p := DefaultScreenFramePolicy()

	w := 0
	req := StartRequest{MaxWidth: &w}
	if err := req.Validate(p); err == nil {
		t.Fatal("expected error for zero maxWidth")
	}

	h := -5
	req2 := StartRequest{MaxHeight: &h}
	if err := req2.Validate(p); err == nil {
		t.Fatal("expected error for negative maxHeight")
	}
}

func TestStartRequest_Validate_InvalidDisplay(t *testing.T) {
	p := DefaultScreenFramePolicy()

	d := -1
	req := StartRequest{DisplayID: &d}
	if err := req.Validate(p); err == nil {
		t.Fatal("expected error for negative displayId")
	}
}

func TestStartRequest_ResolveFPS(t *testing.T) {
	p := DefaultScreenFramePolicy()

	req := StartRequest{}
	if got := req.ResolveFPS(p); got != p.DefaultFPS {
		t.Errorf("expected default fps %v, got %v", p.DefaultFPS, got)
	}

	custom := 5.0
	req2 := StartRequest{TargetFPS: &custom}
	if got := req2.ResolveFPS(p); got != 5.0 {
		t.Errorf("expected custom fps 5.0, got %v", got)
	}
}

func TestStartRequest_ResolveSizes(t *testing.T) {
	p := DefaultScreenFramePolicy()

	w := 800
	h := 600
	req := StartRequest{MaxWidth: &w, MaxHeight: &h}
	if got := req.ResolveMaxWidth(p); got != 800 {
		t.Errorf("expected maxWidth 800, got %v", got)
	}
	if got := req.ResolveMaxHeight(p); got != 600 {
		t.Errorf("expected maxHeight 600, got %v", got)
	}
}

func TestStartRequest_ResolveDisplayID(t *testing.T) {
	req := StartRequest{}
	if got := req.ResolveDisplayID(); got != 0 {
		t.Errorf("expected default display 0, got %v", got)
	}

	d := 2
	req2 := StartRequest{DisplayID: &d}
	if got := req2.ResolveDisplayID(); got != 2 {
		t.Errorf("expected display 2, got %v", got)
	}
}

func TestLatestRequest_WaitDuration(t *testing.T) {
	req := LatestRequest{}
	if got := req.WaitDuration(); got != 0 {
		t.Errorf("expected 0, got %v", got)
	}

	req.WaitMs = 2000
	if got := req.WaitDuration(); got != 2*time.Second {
		t.Errorf("expected 2s, got %v", got)
	}

	req.WaitMs = 10000
	if got := req.WaitDuration(); got != 5*time.Second {
		t.Errorf("expected 5s (capped), got %v", got)
	}
}

func TestLatestRequest_ResolveFormat(t *testing.T) {
	req := LatestRequest{}
	if got := req.ResolveFormat(); got != FormatJPEG {
		t.Errorf("expected jpeg default, got %v", got)
	}

	f := FormatPNG
	req2 := LatestRequest{Format: &f}
	if got := req2.ResolveFormat(); got != FormatPNG {
		t.Errorf("expected png, got %v", got)
	}
}

func TestBlockedSessionStore_Create(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(1)
	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("session-1"),
		Owner: owner,
	}

	_, err := store.Create(ctx, session)
	if err == nil {
		t.Fatal("expected error for blocked session store")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrBlockedNativeHost {
		t.Errorf("expected ErrBlockedNativeHost, got %v", serr.Code)
	}
}

func TestBlockedSessionStore_SessionLimit(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)

	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	for i := 1; i <= 3; i++ {
		session := ScreenFrameSession{
			ID:    ScreenFrameSessionID("session-" + string(rune('0'+i))),
			Owner: owner,
			State: SessionStateRunning,
			Width: 1080, Height: 2400, TargetFPS: 2,
			StartedAt: time.Now(),
		}
		store.sessions[session.ID] = &session
	}

	sessions, _ := store.ListActive(ctx)
	if len(sessions) > 2 {
		t.Errorf("expected at most 2 active sessions, got %v", len(sessions))
	}
}

func TestBlockedSessionStore_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(1)

	_, err := store.Get(ctx, "nonexistent")
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

func TestBlockedSessionStore_ListByUser(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(3).(*blockedSessionStore)

	owner := SessionOwner{UserID: "u1", CharacterID: "c1", ConversationID: "v1"}

	for i := 1; i <= 2; i++ {
		session := ScreenFrameSession{
			ID:    ScreenFrameSessionID("owned-" + string(rune('0'+i))),
			Owner: owner,
			State: SessionStateRunning,
			Width: 1080, Height: 2400, TargetFPS: 2,
			StartedAt: time.Now(),
		}
		store.sessions[session.ID] = &session
	}

	other := ScreenFrameSession{
		ID:    ScreenFrameSessionID("other"),
		Owner: SessionOwner{UserID: "u2", CharacterID: "c2", ConversationID: "v2"},
		State: SessionStateRunning,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[other.ID] = &other

	sessions, _ := store.ListByUser(ctx, "u1")
	if len(sessions) != 2 {
		t.Errorf("expected 2 owned sessions, got %v", len(sessions))
	}
}

func TestBlockedSessionStore_UpdateState(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("s-1"),
		Owner: SessionOwner{UserID: "u1"},
		State: SessionStateRunning,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[session.ID] = &session

	if err := store.UpdateState(ctx, "s-1", SessionStateStopped, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, _ := store.Get(ctx, "s-1")
	if s.State != SessionStateStopped {
		t.Errorf("expected state stopped, got %v", s.State)
	}
	if s.CaptureGeneration != 3 {
		t.Errorf("expected generation 3, got %v", s.CaptureGeneration)
	}
}

func TestBlockedSessionStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(2).(*blockedSessionStore)

	session := ScreenFrameSession{
		ID:    ScreenFrameSessionID("s-1"),
		Owner: SessionOwner{UserID: "u1"},
		State: SessionStateRunning,
		Width: 1080, Height: 2400, TargetFPS: 2,
		StartedAt: time.Now(),
	}
	store.sessions[session.ID] = &session

	if err := store.Delete(ctx, "s-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := store.Get(ctx, "s-1")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestBlockedSessionStore_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(1)

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestBlockedSessionStore_StopAll(t *testing.T) {
	ctx := context.Background()
	store := NewBlockedSessionStore(3).(*blockedSessionStore)

	for i := 1; i <= 2; i++ {
		session := ScreenFrameSession{
			ID:    ScreenFrameSessionID("s-" + string(rune('0'+i))),
			Owner: SessionOwner{UserID: "u1"},
			State: SessionStateRunning,
			Width: 1080, Height: 2400, TargetFPS: 2,
			StartedAt: time.Now(),
		}
		store.sessions[session.ID] = &session
	}

	if err := store.StopAll(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessions, _ := store.ListActive(ctx)
	for _, s := range sessions {
		if s.Active() {
			t.Errorf("session %s should not be active after stop-all", s.ID)
		}
	}
}

func TestSession_Active(t *testing.T) {
	tests := []struct {
		state  SessionState
		active bool
	}{
		{SessionStateStarting, true},
		{SessionStateAwaitingPermission, true},
		{SessionStateRunning, true},
		{SessionStateStopping, false},
		{SessionStateStopped, false},
		{SessionStateProjectionRevoked, false},
		{SessionStateFailed, false},
	}

	for _, tt := range tests {
		s := ScreenFrameSession{State: tt.state}
		if got := s.Active(); got != tt.active {
			t.Errorf("state %s: expected active=%v, got %v", tt.state, tt.active, got)
		}
	}
}

func TestFrameSnapshot_IsValid(t *testing.T) {
	valid := FrameSnapshot{
		Sequence:  1,
		Width:     1080,
		Height:    2400,
		Timestamp: time.Now(),
		Buffer:    []byte{1, 2, 3},
	}
	if !valid.IsValid() {
		t.Error("expected valid snapshot")
	}

	invalid := FrameSnapshot{Sequence: 0}
	if invalid.IsValid() {
		t.Error("expected invalid snapshot with zero sequence")
	}

	emptyBuffer := FrameSnapshot{Sequence: 1, Width: 1080, Height: 2400, Timestamp: time.Now()}
	if emptyBuffer.IsValid() {
		t.Error("expected invalid snapshot with empty buffer")
	}

	noTimestamp := FrameSnapshot{Sequence: 1, Width: 1080, Height: 2400, Buffer: []byte{1}}
	if noTimestamp.IsValid() {
		t.Error("expected invalid snapshot with zero timestamp")
	}
}
