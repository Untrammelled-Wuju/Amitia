package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func waitForReservedRequest(t *testing.T, client *Client, transport *MockTransport, requestID string, responsePayload string) protocol.Envelope {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for transport.GetSentMessagesLen() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected exactly one reserved request, got %d", len(messages))
	}
	go func() {
		response, err := client.Receive(context.Background())
		if err == nil {
			client.DispatchIncomingResponse(response)
		}
	}()
	transport.QueueMessage(protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        "host-response",
		RequestID: requestID,
		Payload:   json.RawMessage(responsePayload),
	})
	return messages[0]
}

func decodePayloadObject(t *testing.T, envelope protocol.Envelope) map[string]json.RawMessage {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

func assertNoIdentityFields(t *testing.T, payload map[string]json.RawMessage) {
	t.Helper()
	for _, key := range []string{"runtimeId", "pluginId", "serviceId", "generation"} {
		if _, exists := payload[key]; exists {
			t.Fatalf("trusted identity field %q must not be carried in reserved RPC payload", key)
		}
	}
}

func TestAuthoritySDKMatchesCanonicalHostWireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("snapshot", func(t *testing.T) {
		transport := NewMockTransport()
		client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("authority-snapshot")), WithPendingTimeout(time.Second))
		resultC := make(chan AuthoritySnapshot, 1)
		errC := make(chan error, 1)
		go func() {
			result, err := client.GetAuthoritySnapshot(ctx)
			resultC <- result
			errC <- err
		}()
		sent := waitForReservedRequest(t, client, transport, "authority-snapshot", `{"runtimeId":"runtime-1","pluginId":"plugin-1","mode":"observe_only","epoch":7,"updatedAt":"2026-08-25T00:00:00Z","lastTransitionReason":"startup","lastTransitionActor":"host"}`)
		if sent.Method != MethodAuthoritySnapshot {
			t.Fatalf("method=%q want %q", sent.Method, MethodAuthoritySnapshot)
		}
		payload := decodePayloadObject(t, sent)
		assertNoIdentityFields(t, payload)
		if len(payload) != 0 {
			t.Fatalf("snapshot request payload must be empty, got %s", string(sent.Payload))
		}
		if err := <-errC; err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		result := <-resultC
		if result.Mode != ControlModeObserveOnly || result.Epoch != 7 || result.UpdatedAt.IsZero() {
			t.Fatalf("unexpected snapshot: %+v", result)
		}
	})

	t.Run("takeover", func(t *testing.T) {
		transport := NewMockTransport()
		client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("authority-takeover")), WithPendingTimeout(time.Second))
		epoch := uint64(7)
		resultC := make(chan AuthorityTakeoverResult, 1)
		errC := make(chan error, 1)
		go func() {
			result, err := client.TakeoverAuthority(ctx, AuthorityTakeoverInput{ExpectedEpoch: &epoch})
			resultC <- result
			errC <- err
		}()
		sent := waitForReservedRequest(t, client, transport, "authority-takeover", `{"previousMode":"observe_only","newMode":"user_control","previousEpoch":7,"newEpoch":8,"snapshot":{"runtimeId":"runtime-1","pluginId":"plugin-1","mode":"user_control","epoch":8,"updatedAt":"2026-08-25T00:00:01Z"}}`)
		if sent.Method != MethodControlAuthorityTakeover {
			t.Fatalf("method=%q", sent.Method)
		}
		payload := decodePayloadObject(t, sent)
		assertNoIdentityFields(t, payload)
		for _, key := range []string{"actor", "targetMode"} {
			if _, exists := payload[key]; exists {
				t.Fatalf("host-owned takeover field %q must not be sent", key)
			}
		}
		if err := <-errC; err != nil {
			t.Fatalf("takeover: %v", err)
		}
		result := <-resultC
		if result.NewMode != ControlModeUserControl || result.NewEpoch != 8 || result.Snapshot.Mode != ControlModeUserControl {
			t.Fatalf("unexpected takeover result: %+v", result)
		}
	})

	t.Run("release", func(t *testing.T) {
		transport := NewMockTransport()
		client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("authority-release")), WithPendingTimeout(time.Second))
		epoch := uint64(8)
		resultC := make(chan AuthorityReleaseResult, 1)
		errC := make(chan error, 1)
		go func() {
			result, err := client.ReleaseAuthority(ctx, AuthorityReleaseInput{TargetMode: ControlModeObserveOnly, ExpectedEpoch: &epoch})
			resultC <- result
			errC <- err
		}()
		sent := waitForReservedRequest(t, client, transport, "authority-release", `{"previousMode":"user_control","newMode":"observe_only","previousEpoch":8,"newEpoch":9,"snapshot":{"runtimeId":"runtime-1","pluginId":"plugin-1","mode":"observe_only","epoch":9,"updatedAt":"2026-08-25T00:00:02Z"}}`)
		payload := decodePayloadObject(t, sent)
		assertNoIdentityFields(t, payload)
		if _, exists := payload["actor"]; exists {
			t.Fatal("actor must be host-bound, not supplied by plugin")
		}
		if err := <-errC; err != nil {
			t.Fatalf("release: %v", err)
		}
		result := <-resultC
		if result.NewMode != ControlModeObserveOnly || result.NewEpoch != 9 {
			t.Fatalf("unexpected release result: %+v", result)
		}
	})
}

func TestSecretSDKMatchesCanonicalHostWireContract(t *testing.T) {
	ctx := context.Background()

	transport := NewMockTransport()
	client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("secret-acquire")), WithPendingTimeout(time.Second))
	resultC := make(chan SecretAcquireResult, 1)
	errC := make(chan error, 1)
	go func() {
		result, err := client.AcquireSecret(ctx, SecretAcquireInput{Ref: "secret://provider/key", Purpose: SecretPurposeRuntime, Required: true})
		resultC <- result
		errC <- err
	}()
	sent := waitForReservedRequest(t, client, transport, "secret-acquire", `{"leaseId":"lease-1","ref":"secret://provider/key","purpose":"runtime","granted":true,"expiresAt":12345}`)
	payload := decodePayloadObject(t, sent)
	assertNoIdentityFields(t, payload)
	if _, exists := payload["status"]; exists {
		t.Fatal("secret acquire request must not carry status")
	}
	if err := <-errC; err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if result := <-resultC; !result.Granted || result.LeaseID != "lease-1" || result.Purpose != SecretPurposeRuntime {
		t.Fatalf("unexpected acquire result: %+v", result)
	}

	transport = NewMockTransport()
	client = NewClient(transport, WithIDGenerator(NewFixedIDGenerator("secret-query")), WithPendingTimeout(time.Second))
	queryC := make(chan SecretQueryResult, 1)
	errC = make(chan error, 1)
	go func() {
		result, err := client.QuerySecretLease(ctx, SecretQueryInput{LeaseID: "lease-1"})
		queryC <- result
		errC <- err
	}()
	sent = waitForReservedRequest(t, client, transport, "secret-query", `{"leaseId":"lease-1","ref":"secret://provider/key","granted":true,"valid":true,"expiresAt":12345}`)
	payload = decodePayloadObject(t, sent)
	assertNoIdentityFields(t, payload)
	if _, exists := payload["ref"]; exists {
		t.Fatal("secret.query v1 is leaseId-only; ref must not be sent")
	}
	if err := <-errC; err != nil {
		t.Fatalf("query: %v", err)
	}
	if result := <-queryC; !result.Valid || result.LeaseID != "lease-1" {
		t.Fatalf("unexpected query result: %+v", result)
	}
}

func TestHostInvokeSDKPreservesOutputEnvelope(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("host-invoke")), WithPendingTimeout(time.Second))
	resultC := make(chan HostInvokeResult, 1)
	errC := make(chan error, 1)
	go func() {
		result, err := client.InvokeHostMethod(context.Background(), HostInvokeInput{Method: "host.runtime.health", Version: 1, Input: json.RawMessage(`{"probe":true}`), TimeoutMs: 250})
		resultC <- result
		errC <- err
	}()
	sent := waitForReservedRequest(t, client, transport, "host-invoke", `{"status":"success","output":{"moduleId":"module-1","instances":["runtime-1"]},"method":"host.runtime.health","durationMs":4}`)
	if sent.Method != MethodHostInvoke {
		t.Fatalf("method=%q want=%q", sent.Method, MethodHostInvoke)
	}
	payload := decodePayloadObject(t, sent)
	assertNoIdentityFields(t, payload)
	for _, key := range []string{"sideEffect", "requestId"} {
		if _, exists := payload[key]; exists {
			t.Fatalf("unused host.invoke field %q must not be sent", key)
		}
	}
	if err := <-errC; err != nil {
		t.Fatalf("host.invoke: %v", err)
	}
	result := <-resultC
	var output map[string]any
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("decode nested output: %v", err)
	}
	if result.Status != HostAPISuccess || result.Method != "host.runtime.health" || output["moduleId"] != "module-1" {
		t.Fatalf("unexpected host.invoke result: %+v output=%v", result, output)
	}
}

func TestArtifactSDKMatchesCanonicalHostWireContract(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("artifact-list")), WithPendingTimeout(time.Second))
	resultC := make(chan ArtifactListResult, 1)
	errC := make(chan error, 1)
	go func() {
		result, err := client.ListArtifacts(context.Background(), ArtifactRequest{TargetRoot: "game-root", CompatibilityVersion: "1.0"})
		resultC <- result
		errC <- err
	}()
	sent := waitForReservedRequest(t, client, transport, "artifact-list", `{"items":[]}`)
	if sent.Method != MethodArtifactList {
		t.Fatalf("method=%q want=%q", sent.Method, MethodArtifactList)
	}
	payload := decodePayloadObject(t, sent)
	assertNoIdentityFields(t, payload)
	if _, ok := payload["targetRoot"]; !ok {
		t.Fatal("artifact request must carry targetRoot")
	}
	if err := <-errC; err != nil {
		t.Fatalf("artifact.list: %v", err)
	}
	if result := <-resultC; result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("unexpected artifact.list result: %+v", result)
	}
}

func TestChannelAndAgentEventSDKUseReservedNotifications(t *testing.T) {
	ctx := context.Background()

	transport := NewMockTransport()
	client := NewClient(transport, WithIDGenerator(NewFixedIDGenerator("channel-publish")))
	if _, err := client.ChannelPublish(ctx, ChannelPublishInput{
		ChannelID: "events",
		Payload:   json.RawMessage(`{"type":"tick"}`),
	}); err != nil {
		t.Fatalf("channel.publish: %v", err)
	}
	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one channel notification, got %d", len(messages))
	}
	if messages[0].Type != protocol.MessageTypeNotification || messages[0].Method != MethodChannelPublish {
		t.Fatalf("unexpected channel envelope: %+v", messages[0])
	}
	channelPayload := decodePayloadObject(t, messages[0])
	assertNoIdentityFields(t, channelPayload)
	if _, ok := channelPayload["channelId"]; !ok {
		t.Fatal("channel.publish must carry channelId")
	}

	transport = NewMockTransport()
	client = NewClient(transport, WithIDGenerator(NewFixedIDGenerator("agent-event")))
	event := protocol.PluginEvent{ID: "event-1", Type: "test.event"}
	if _, err := client.PublishAgentEvent(ctx, event, nil); err != nil {
		t.Fatalf("plugin.event.publish: %v", err)
	}
	messages = transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one agent-event notification, got %d", len(messages))
	}
	if messages[0].Type != protocol.MessageTypeNotification || messages[0].Method != MethodAgentEventPublish {
		t.Fatalf("unexpected agent-event envelope: %+v", messages[0])
	}
	eventPayload := decodePayloadObject(t, messages[0])
	assertNoIdentityFields(t, eventPayload)
	if _, exists := eventPayload["channelId"]; exists {
		t.Fatal("plugin.event.publish is an Agent wake-up hint and must not expose a channelId")
	}
	var eventID string
	if err := json.Unmarshal(eventPayload["eventId"], &eventID); err != nil || eventID != PluginAgentEventID {
		t.Fatalf("eventId=%q err=%v", eventID, err)
	}
}
