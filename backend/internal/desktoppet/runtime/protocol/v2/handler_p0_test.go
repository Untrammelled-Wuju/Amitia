package v2

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newHandlerP0DB(t *testing.T) (*gorm.DB, *Services, *Handler) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&RuntimeSession{}, &RuntimeCommand{}, &EventRecord{}, &RuntimeActualState{}); err != nil {
		t.Fatal(err)
	}
	services := NewServices(db)
	return db, services, NewHandler(services)
}

func TestHandleHelloRejectsUnsupportedRuntimeContractBeforeSessionAcquire(t *testing.T) {
	db, _, handler := newHandlerP0DB(t)
	conn := &Connection{
		ID:        "conn-version-gate",
		UserID:    "user-version-gate",
		DeviceID:  "device-version-gate",
		RuntimeID: "runtime-version-gate",
		State:     ConnStateHandshake,
	}
	payload := &HelloPayload{
		RuntimeVersion:         "2.0.0",
		RuntimeContractVersion: "3.0.0",
		DeviceID:               conn.DeviceID,
		RuntimeID:              conn.RuntimeID,
	}

	if _, err := handler.HandleHello(conn, payload); err == nil {
		t.Fatal("expected unsupported runtime contract to be rejected")
	}

	var sessions int64
	if err := db.Model(&RuntimeSession{}).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("unsupported hello must not create a runtime session, got %d", sessions)
	}
	if conn.SessionIDValue() != "" || conn.GetState() != ConnStateHandshake {
		t.Fatalf("unsupported hello mutated connection state: session=%q state=%s", conn.SessionIDValue(), conn.GetState())
	}
}

func TestHandlerCommandAckUsesRuntimeEnvelopeSequenceAndTerminalStatus(t *testing.T) {
	db, services, handler := newHandlerP0DB(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	session := &RuntimeSession{ID: "sess-1", UserID: "user-1", DeviceID: "device-1", RuntimeID: "runtime-1", ConnectionGeneration: 1, Status: string(SessionStatusReady), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	cmd := &RuntimeCommand{ID: "cmd-1", UserID: "user-1", DeviceID: "device-1", RuntimeID: "runtime-1", RuntimeSessionID: session.ID, CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusRendererAccepted), PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)), PayloadSchemaVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	conn := &Connection{ID: "conn-1", UserID: "user-1", DeviceID: "device-1", RuntimeID: "runtime-1", State: ConnStateConnected}
	conn.ActivateSession(session.ID, 1, 3)
	payload, _ := json.Marshal(CommandAckPayload{CommandID: cmd.ID, CommandSequence: 7, Status: string(CommandStatusCompleted), RuntimeSessionID: session.ID, ReceivedAt: time.Now().UTC()})
	env := &Envelope{
		UserID:               conn.UserID,
		DeviceID:             conn.DeviceID,
		RuntimeID:            conn.RuntimeID,
		RuntimeSessionID:     session.ID,
		ConnectionGeneration: 1,
		Sequence:             100,
		Payload:              payload,
		PayloadHash:          ComputePayloadHash(payload),
	}
	if err := handler.HandleCommandAck(conn, env, &CommandAckPayload{CommandID: cmd.ID, CommandSequence: 7, Status: string(CommandStatusCompleted), RuntimeSessionID: session.ID, ReceivedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	got, err := services.Commands.GetCommand(cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != string(CommandStatusCompleted) {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if conn.LastInboundSequence() != 100 {
		t.Fatalf("expected inbound sequence 100, got %d", conn.LastInboundSequence())
	}
	storedSession, err := services.Sessions.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedSession.LastEventSequence != 100 {
		t.Fatalf("expected persisted event sequence 100, got %d", storedSession.LastEventSequence)
	}
	var event EventRecord
	if err := db.Where("runtime_session_id = ? AND sequence = ?", session.ID, 100).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.CommandID != cmd.ID {
		t.Fatalf("expected command id %s, got %s", cmd.ID, event.CommandID)
	}
}

func TestHandlerInboundAndOutboundSequencesAreIndependent(t *testing.T) {
	_, _, _ = newHandlerP0DB(t)
	conn := &Connection{LastSeq: 9}
	if got := conn.NextOutboundSequence(); got != 1 {
		t.Fatalf("expected first outbound sequence 1, got %d", got)
	}
	if got := conn.NextOutboundSequence(); got != 2 {
		t.Fatalf("expected second outbound sequence 2, got %d", got)
	}
	if got := conn.LastInboundSequence(); got != 9 {
		t.Fatalf("outbound sequence must not advance inbound replay cursor: %d", got)
	}
}

func TestEventServiceAppendIsIdempotentBySessionSequence(t *testing.T) {
	_, services, _ := newHandlerP0DB(t)
	payload := []byte(`{"commandId":"cmd-1"}`)
	commandID := "cmd-1"
	first, err := services.Events.Append(EventCommandAcknowledged, payload, "sess-idem", 5, TriggerSourceRuntimeCommand, &commandID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := services.Events.Append(EventCommandAcknowledged, payload, "sess-idem", 5, TriggerSourceRuntimeCommand, &commandID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("retry must return existing event, got %s and %s", first.ID, second.ID)
	}
	if _, err := services.Events.Append(EventHealthChanged, []byte(`{"x":1}`), "sess-idem", 5, TriggerSourceRuntimeCommand, nil); err == nil {
		t.Fatal("conflicting reuse of runtime event sequence must fail")
	}
}

func TestPersistActualStateSnapshotProjectsAuthoritativeRuntimeState(t *testing.T) {
	_, services, handler := newHandlerP0DB(t)
	conn := &Connection{ID: "conn-state", UserID: "user-state", DeviceID: "device-state", RuntimeID: "runtime-state", State: ConnStateConnected}
	conn.ActivateSession("sess-state", 3, 10)

	payload, err := json.Marshal(StateSnapshotPayload{
		ConnectionGeneration:         3,
		EventSequence:                77,
		ActualStateHash:              "sha256:state",
		InstanceStatus:               InstanceStatusReady,
		WindowStatus:                 WindowStatusVisible,
		RendererStatus:               RendererStatusRuntimeReady,
		PlaybackStatus:               PlaybackStatusPlaying,
		AppliedDesiredRevision:       12,
		AppliedDesiredHash:           "sha256:desired",
		AppliedSettingsRevision:      8,
		InstallationID:               "installation-1",
		PetID:                        "pet-1",
		ReleaseID:                    "release-1",
		StableActionKey:              "idle",
		CurrentActionKey:             "wave",
		PlaybackInstanceID:           "playback-1",
		CurrentCommandID:             "command-1",
		LastProcessedCommandSequence: 44,
	})
	if err != nil {
		t.Fatal(err)
	}
	env := &Envelope{MessageName: EventStateSnapshot, Sequence: 77, Payload: payload}
	if err := handler.persistActualStateSnapshot(conn, env); err != nil {
		t.Fatal(err)
	}

	state, err := services.ActualStates.Get("runtime-state", "installation-1")
	if err != nil {
		t.Fatal(err)
	}
	if state.ConnectionGeneration != 3 || state.LastEventSequence != 77 {
		t.Fatalf("expected authoritative generation/sequence 3/77, got %d/%d", state.ConnectionGeneration, state.LastEventSequence)
	}
	if state.RendererStatus != RendererStatusRuntimeReady || state.HealthStatus != HealthStatusHealthy {
		t.Fatalf("expected runtime_ready/healthy, got %s/%s", state.RendererStatus, state.HealthStatus)
	}
	if state.AppliedDesiredRevision != 12 || state.AppliedSettingsRevision != 8 {
		t.Fatalf("unexpected applied revisions: desired=%d settings=%d", state.AppliedDesiredRevision, state.AppliedSettingsRevision)
	}
	if state.ActualStateHash != "sha256:state" || state.CurrentActionKey != "wave" || !state.Visible {
		t.Fatalf("actual state projection incomplete: %#v", state)
	}
}

func TestPersistActualStateSnapshotRejectsStaleProjection(t *testing.T) {
	_, services, handler := newHandlerP0DB(t)
	conn := &Connection{ID: "conn-stale", UserID: "user-stale", DeviceID: "device-stale", RuntimeID: "runtime-stale", State: ConnStateConnected}
	conn.ActivateSession("sess-stale", 2, 0)

	makeEnv := func(sequence int64, action string) *Envelope {
		payload, err := json.Marshal(StateSnapshotPayload{
			ConnectionGeneration:    2,
			EventSequence:           sequence,
			InstanceStatus:          InstanceStatusReady,
			WindowStatus:            WindowStatusHidden,
			RendererStatus:          RendererStatusRuntimeReady,
			PlaybackStatus:          PlaybackStatusIdle,
			InstallationID:          "installation-stale",
			CurrentActionKey:        action,
			ActualStateHash:         "sha256:" + action,
			AppliedDesiredRevision:  1,
			AppliedSettingsRevision: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return &Envelope{MessageName: EventStateSnapshot, Sequence: sequence, Payload: payload}
	}

	if err := handler.persistActualStateSnapshot(conn, makeEnv(20, "new")); err != nil {
		t.Fatal(err)
	}
	if err := handler.persistActualStateSnapshot(conn, makeEnv(19, "old")); err != nil {
		t.Fatal(err)
	}
	state, err := services.ActualStates.Get("runtime-stale", "installation-stale")
	if err != nil {
		t.Fatal(err)
	}
	if state.LastEventSequence != 20 || state.CurrentActionKey != "new" {
		t.Fatalf("stale projection overwrote authoritative state: seq=%d action=%s", state.LastEventSequence, state.CurrentActionKey)
	}
}

func TestPersistActualStateSnapshotRejectsCursorMismatch(t *testing.T) {
	_, _, handler := newHandlerP0DB(t)
	conn := &Connection{ID: "conn-mismatch", UserID: "user-mismatch", DeviceID: "device-mismatch", RuntimeID: "runtime-mismatch", State: ConnStateConnected}
	conn.ActivateSession("sess-mismatch", 4, 0)
	payload, err := json.Marshal(StateSnapshotPayload{
		ConnectionGeneration:    3,
		EventSequence:           10,
		ActualStateHash:         "sha256:mismatch",
		InstanceStatus:          InstanceStatusReady,
		WindowStatus:            WindowStatusVisible,
		RendererStatus:          RendererStatusRuntimeReady,
		PlaybackStatus:          PlaybackStatusIdle,
		AppliedDesiredRevision:  1,
		AppliedSettingsRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.persistActualStateSnapshot(conn, &Envelope{MessageName: EventStateSnapshot, Sequence: 10, Payload: payload}); err == nil {
		t.Fatal("expected generation mismatch to be rejected")
	}
}

func TestHandleEventRejectsInvalidStateSnapshotBeforeConsumingSequence(t *testing.T) {
	db, _, handler := newHandlerP0DB(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	session := &RuntimeSession{
		ID:                   "sess-invalid-state",
		UserID:               "user-invalid-state",
		DeviceID:             "device-invalid-state",
		RuntimeID:            "runtime-invalid-state",
		ConnectionGeneration: 5,
		Status:               string(SessionStatusReady),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	conn := &Connection{
		ID:        "conn-invalid-state",
		UserID:    "user-invalid-state",
		DeviceID:  "device-invalid-state",
		RuntimeID: "runtime-invalid-state",
		State:     ConnStateConnected,
	}
	conn.ActivateSession(session.ID, 5, 0)

	payload, err := json.Marshal(StateSnapshotPayload{
		ConnectionGeneration:    4, // Deliberately stale: active connection is generation 5.
		EventSequence:           9,
		ActualStateHash:         "sha256:invalid",
		InstanceStatus:          InstanceStatusReady,
		WindowStatus:            WindowStatusVisible,
		RendererStatus:          RendererStatusRuntimeReady,
		PlaybackStatus:          PlaybackStatusIdle,
		AppliedDesiredRevision:  1,
		AppliedSettingsRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	env := &Envelope{
		UserID:               conn.UserID,
		DeviceID:             conn.DeviceID,
		RuntimeID:            conn.RuntimeID,
		RuntimeSessionID:     session.ID,
		ConnectionGeneration: 5,
		MessageName:          EventStateSnapshot,
		Sequence:             9,
		Payload:              payload,
		PayloadHash:          ComputePayloadHash(payload),
	}
	if _, err := handler.HandleEvent(conn, env); err == nil {
		t.Fatal("expected invalid state snapshot to be rejected")
	}

	var count int64
	if err := db.Model(&EventRecord{}).Where("runtime_session_id = ? AND sequence = ?", session.ID, 9).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid snapshot consumed event sequence: count=%d", count)
	}
	if conn.LastInboundSequence() != 0 {
		t.Fatalf("invalid snapshot advanced inbound cursor: %d", conn.LastInboundSequence())
	}
}

func TestReconnectFencesSupersededConnectionBeforeCommandAckMutation(t *testing.T) {
	db, services, handler := newHandlerP0DB(t)
	oldConn, err := handler.HandleConnect("user-reconnect", "device-reconnect", "runtime-reconnect")
	if err != nil {
		t.Fatal(err)
	}
	hello := &HelloPayload{
		RuntimeVersion:         "2.0.0",
		RuntimeContractVersion: CurrentSchemaVersion,
		DeviceID:               oldConn.DeviceID,
		RuntimeID:              oldConn.RuntimeID,
	}
	ack, err := handler.HandleHello(oldConn, hello)
	if err != nil {
		t.Fatal(err)
	}
	if ack == nil || ack.SessionID == "" {
		t.Fatal("expected established old runtime session")
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	cmd := &RuntimeCommand{
		ID:                   "cmd-reconnect-fence",
		UserID:               string(oldConn.UserID),
		DeviceID:             string(oldConn.DeviceID),
		RuntimeID:            string(oldConn.RuntimeID),
		RuntimeSessionID:     string(ack.SessionID),
		CommandType:          string(CommandTypePlayAction),
		Durability:           "ephemeral",
		Status:               string(CommandStatusRendererAccepted),
		PayloadJSON:          `{}`,
		PayloadHash:          ComputePayloadHash([]byte(`{}`)),
		PayloadSchemaVersion: 1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}

	newConn, err := handler.HandleConnect(oldConn.UserID, oldConn.DeviceID, oldConn.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	if newConn == oldConn || oldConn.GetState() != ConnStateClosing {
		t.Fatalf("old connection was not fenced: old=%s new=%s", oldConn.GetState(), newConn.GetState())
	}

	payload, err := json.Marshal(CommandAckPayload{
		CommandID:        cmd.ID,
		Status:           string(CommandStatusCompleted),
		RuntimeSessionID: string(ack.SessionID),
		ReceivedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, generation := oldConn.SessionSnapshot()
	env := &Envelope{
		UserID:               oldConn.UserID,
		DeviceID:             oldConn.DeviceID,
		RuntimeID:            oldConn.RuntimeID,
		RuntimeSessionID:     ack.SessionID,
		ConnectionGeneration: generation,
		Sequence:             10,
		Payload:              payload,
		PayloadHash:          ComputePayloadHash(payload),
	}
	if err := handler.HandleCommandAck(oldConn, env, &CommandAckPayload{
		CommandID:        cmd.ID,
		Status:           string(CommandStatusCompleted),
		RuntimeSessionID: string(ack.SessionID),
		ReceivedAt:       time.Now().UTC(),
	}); err == nil {
		t.Fatal("superseded connection command ack must be rejected")
	}

	stored, err := services.Commands.GetCommand(cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(CommandStatusRendererAccepted) {
		t.Fatalf("stale ack mutated command status: %s", stored.Status)
	}
	var eventCount int64
	if err := db.Model(&EventRecord{}).Where("runtime_session_id = ? AND sequence = ?", ack.SessionID, 10).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 {
		t.Fatalf("stale ack appended an event: %d", eventCount)
	}
}

func TestReconnectFencesSupersededHandshakeBeforeSessionAcquire(t *testing.T) {
	db, _, handler := newHandlerP0DB(t)
	oldConn, err := handler.HandleConnect("user-handshake", "device-handshake", "runtime-handshake")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handler.HandleConnect(oldConn.UserID, oldConn.DeviceID, oldConn.RuntimeID); err != nil {
		t.Fatal(err)
	}
	if oldConn.GetState() != ConnStateClosing {
		t.Fatalf("old handshake was not fenced: %s", oldConn.GetState())
	}
	if _, err := handler.HandleHello(oldConn, &HelloPayload{
		RuntimeVersion:         "2.0.0",
		RuntimeContractVersion: CurrentSchemaVersion,
		DeviceID:               oldConn.DeviceID,
		RuntimeID:              oldConn.RuntimeID,
	}); err == nil {
		t.Fatal("superseded handshake must not acquire a runtime session")
	}
	var sessionCount int64
	if err := db.Model(&RuntimeSession{}).Count(&sessionCount).Error; err != nil {
		t.Fatal(err)
	}
	if sessionCount != 0 {
		t.Fatalf("superseded handshake created runtime sessions: %d", sessionCount)
	}
}

func TestHandleCommandAckRejectsWrongGenerationBeforeMutation(t *testing.T) {
	db, services, handler := newHandlerP0DB(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	session := &RuntimeSession{ID: "sess-generation", UserID: "user-generation", DeviceID: "device-generation", RuntimeID: "runtime-generation", ConnectionGeneration: 3, Status: string(SessionStatusReady), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	cmd := &RuntimeCommand{ID: "cmd-generation", UserID: session.UserID, DeviceID: session.DeviceID, RuntimeID: session.RuntimeID, RuntimeSessionID: session.ID, CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusRendererAccepted), PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)), PayloadSchemaVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	conn := &Connection{ID: "conn-generation", UserID: "user-generation", DeviceID: "device-generation", RuntimeID: "runtime-generation", State: ConnStateConnected}
	conn.ActivateSession(session.ID, 3, 0)
	ack := &CommandAckPayload{CommandID: cmd.ID, Status: string(CommandStatusCompleted), RuntimeSessionID: session.ID, ReceivedAt: time.Now().UTC()}
	payload, err := json.Marshal(ack)
	if err != nil {
		t.Fatal(err)
	}
	env := &Envelope{UserID: conn.UserID, DeviceID: conn.DeviceID, RuntimeID: conn.RuntimeID, RuntimeSessionID: session.ID, ConnectionGeneration: 2, Sequence: 1, Payload: payload, PayloadHash: ComputePayloadHash(payload)}
	if err := handler.HandleCommandAck(conn, env, ack); err == nil {
		t.Fatal("wrong generation must be rejected")
	}
	stored, err := services.Commands.GetCommand(cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(CommandStatusRendererAccepted) {
		t.Fatalf("wrong-generation ack mutated command: %s", stored.Status)
	}
}
