package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/lan"
)

type realtimeOwnedModel struct {
	*threeCoreMemoryModel
	source    *threeCoreFixture
	asr       atomic.Int32
	frames    atomic.Int32
	started   chan struct{}
	cancelled chan struct{}
}

func (m *realtimeOwnedModel) TranscribeOwnedAudio(ctx context.Context, input business.Inference, item business.Attachment) (string, error) {
	m.asr.Add(1)
	if m.started != nil {
		close(m.started)
		<-ctx.Done()
		close(m.cancelled)
		return "迟到转写", nil
	}
	return "已确认实时语音", nil
}

func (m *realtimeOwnedModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	if len(input.Attachments) == 2 {
		m.frames.Add(1)
	}
	return m.threeCoreMemoryModel.GenerateOwnedReply(ctx, input)
}

func (m *realtimeOwnedModel) GenerateOwnedSpeech(context.Context, business.SpeechInference) ([]byte, error) {
	return []byte("ID3owned realtime"), nil
}

func realtimeTicketHTTP(t *testing.T, provider, caller *threeCoreFixture, scope coordination.ExecutionScope, origin string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(business.Request{RoleID: "one", RequestID: uuid.NewString(), ExpectedScope: &scope})
	request, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, provider.endpoint.URL+"/api/device-mesh/v1/business/realtime/tickets", bytes.NewReader(body))
	credential, err := caller.local.LoadCredential()
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "AmitiaDevice "+credential.Credential)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", origin)
	if err := caller.identity.SignRequest(request, provider.core); err != nil {
		t.Fatal(err)
	}
	response, err := provider.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		t.Fatalf("ticket status %d: %s", response.StatusCode, data)
	}
	var result struct {
		Data map[string]any `json:"data"`
	}
	if json.Unmarshal(data, &result) != nil {
		t.Fatal("invalid ticket response")
	}
	return result.Data
}

func realtimeDial(t *testing.T, provider *threeCoreFixture, ticket map[string]any, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	configuration, err := lan.PinnedTLS(provider.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	dialer := websocket.Dialer{TLSClientConfig: configuration, HandshakeTimeout: 5 * time.Second}
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return dialer.DialContext(t.Context(), strings.Replace(provider.endpoint.URL, "https://", "wss://", 1)+"/api/device-mesh/v1/business/realtime/session?ticket="+ticket["ticket"].(string), header)
}

func TestOwnedRealtimeRealTLSRejectsReplayForeignCoreOriginAndSavesEachMode(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "realtime-a", schemas), newThreeCoreFixture(t, "realtime-b", schemas), newThreeCoreFixture(t, "realtime-c", schemas)
	for _, fixture := range []*threeCoreFixture{a, b, c} {
		registerMeshRealtimePublicRoutes(fixture.router, fixture.services)
	}
	model := &realtimeOwnedModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: b.core}, source: a}
	b.services.OwnedBusiness = business.NewEngine(b.services.DeviceMesh.Coordination, b.services.DeviceMesh, model)
	pairThreeCoreFixtures(t, a, b)
	scope := threeCoreSpeechScope(t, b, a)
	for _, wrong := range []string{"foreignCore", "origin"} {
		ticket := realtimeTicketHTTP(t, b, a, scope, "https://app.example")
		provider, origin := b, "https://foreign.example"
		if wrong == "foreignCore" {
			provider, origin = c, "https://app.example"
		}
		conn, response, err := realtimeDial(t, provider, ticket, origin)
		if conn != nil {
			conn.Close()
		}
		if err == nil || response == nil || response.StatusCode < 400 {
			t.Fatal("foreign Core or Origin ticket accepted")
		}
	}
	for _, coordinated := range []bool{false, true} {
		if coordinated {
			policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
			if err != nil {
				t.Fatal(err)
			}
			b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, 200)
			scope = threeCoreSpeechScope(t, b, a)
		}
		ticket := realtimeTicketHTTP(t, b, a, scope, "")
		conn, _, err := realtimeDial(t, b, ticket, "")
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		var ready map[string]any
		if conn.ReadJSON(&ready) != nil || ready["type"] != "ready" {
			t.Fatal("missing original scope readiness")
		}
		requestID := uuid.NewString()
		if err := conn.WriteJSON(map[string]any{"type": "turn_start", "requestId": requestID, "expectedExecutionScope": scope}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0, 1}); err != nil {
			t.Fatal(err)
		}
		var frame bytes.Buffer
		if err := png.Encode(&frame, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(frame.Bytes())
		if err := conn.WriteJSON(map[string]any{"type": "visual", "attachment": business.Attachment{Kind: "image", Name: "frame.png", MIME: "image/png", Data: base64.StdEncoding.EncodeToString(frame.Bytes()), Hash: hex.EncodeToString(digest[:])}}); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(map[string]any{"type": "turn_end"}); err != nil {
			t.Fatal(err)
		}
		completed, audio := false, false
		for !audio {
			var event struct {
				Type      string          `json:"type"`
				RequestID string          `json:"requestId"`
				Data      json.RawMessage `json:"data"`
				Message   string          `json:"message"`
			}
			if err := conn.ReadJSON(&event); err != nil {
				t.Fatal(err)
			}
			if event.Type == "error" {
				t.Fatal(event.Message)
			}
			if event.Type == "completed" {
				var reply business.Response
				json.Unmarshal(event.Data, &reply)
				if !reply.Saved || reply.UserRevision != 2 || reply.Transcription == "" || reply.Scope.ResourceOwnerID != scope.ResourceOwnerID {
					t.Fatal("reply escaped owner ACK")
				}
				completed = true
			}
			if event.Type == "audio" {
				var speech business.SpeechResponse
				json.Unmarshal(event.Data, &speech)
				if !completed || !speech.Saved || speech.RequestID != requestID || speech.Acknowledgement.RequestID != "speech/"+requestID || speech.Acknowledgement.OwnerID != scope.ResourceOwnerID || len(speech.Acknowledgement.Versions) != 2 {
					t.Fatal("audio escaped original owner ACK")
				}
				audio = true
			}
		}
		conn.Close()
		if reused, response, err := realtimeDial(t, b, ticket, ""); err == nil || response == nil || response.StatusCode != 401 {
			if reused != nil {
				reused.Close()
			}
			t.Fatal("single use ticket replay accepted")
		}
	}
	if model.asr.Load() != 2 || model.frames.Load() != 2 {
		t.Fatal("realtime bypassed Core ASR")
	}
}

func TestOwnedRealtimeRealTLSDisconnectAndModeChangeCancelASR(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "mode", true: "disconnect"}[disconnect], func(t *testing.T) {
			schemas := newThreeCoreSchemas(t)
			a, b := newThreeCoreFixture(t, "cancel-realtime-a", schemas), newThreeCoreFixture(t, "cancel-realtime-b", schemas)
			registerMeshRealtimePublicRoutes(b.router, b.services)
			model := &realtimeOwnedModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: b.core}, source: a, started: make(chan struct{}), cancelled: make(chan struct{})}
			b.services.OwnedBusiness = business.NewEngine(b.services.DeviceMesh.Coordination, b.services.DeviceMesh, model)
			pairThreeCoreFixtures(t, a, b)
			scope := threeCoreSpeechScope(t, b, a)
			conn, _, err := realtimeDial(t, b, realtimeTicketHTTP(t, b, a, scope, ""), "")
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			var ready map[string]any
			if conn.ReadJSON(&ready) != nil {
				t.Fatal("no ready")
			}
			id := uuid.NewString()
			conn.WriteJSON(map[string]any{"type": "turn_start", "requestId": id, "expectedExecutionScope": scope})
			conn.WriteMessage(websocket.BinaryMessage, []byte{0, 1})
			conn.WriteJSON(map[string]any{"type": "turn_end"})
			select {
			case <-model.started:
			case <-time.After(10 * time.Second):
				t.Fatal("ASR did not start")
			}
			if disconnect {
				conn.Close()
			} else {
				b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": scope.ModeRevision, "selectedRole": "one"}, 200)
			}
			select {
			case <-model.cancelled:
			case <-time.After(time.Second):
				t.Fatal("authority change or disconnect did not promptly cancel Core ASR")
			}
		})
	}
}
