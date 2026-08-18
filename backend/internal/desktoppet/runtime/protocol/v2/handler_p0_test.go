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
	if err := db.AutoMigrate(&RuntimeSession{}, &RuntimeCommand{}, &EventRecord{}); err != nil {
		t.Fatal(err)
	}
	services := NewServices(db)
	return db, services, NewHandler(services)
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
	env := &Envelope{Sequence: 100, Payload: payload, PayloadHash: ComputePayloadHash(payload)}
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
