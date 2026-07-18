package client

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/mcp/transport"
)

type memoryTransport struct {
	mu      sync.RWMutex
	state   transport.State
	receive chan protocol.Message
	sent    chan protocol.Message
}

func newMemoryTransport() *memoryTransport {
	return &memoryTransport{state: transport.StateStopped, receive: make(chan protocol.Message, 32), sent: make(chan protocol.Message, 32)}
}

func (t *memoryTransport) Start(context.Context) error {
	t.mu.Lock()
	t.state = transport.StateRunning
	t.mu.Unlock()
	return nil
}

func (t *memoryTransport) Send(ctx context.Context, message protocol.Message) error {
	select {
	case t.sent <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *memoryTransport) Receive() <-chan protocol.Message { return t.receive }

func (t *memoryTransport) Close(context.Context) error {
	t.mu.Lock()
	t.state = transport.StateStopped
	t.mu.Unlock()
	return nil
}

func (t *memoryTransport) State() transport.State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func TestConnectionInitializeNegotiatesCapabilities(t *testing.T) {
	target := newMemoryTransport()
	connection := NewConnection(target, Config{ClientInfo: protocol.Implementation{Name: "amitia-test", Version: "1.0.0"}, Capabilities: protocol.ClientCapabilities{Roots: map[string]any{"listChanged": true}}})
	go respondToInitialize(t, target, protocol.LatestProtocolVersion)
	if err := connection.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if connection.State() != StateReady {
		t.Fatalf("unexpected state: %s", connection.State())
	}
	result := connection.InitializeResult()
	if result.ProtocolVersion != protocol.LatestProtocolVersion || result.ServerInfo.Name != "test-server" {
		t.Fatalf("unexpected initialize result: %#v", result)
	}
	if tools, ok := result.Capabilities["tools"].(map[string]any); !ok || tools["listChanged"] != true {
		t.Fatalf("capabilities were not negotiated: %#v", result.Capabilities)
	}
	initialized := receiveSent(t, target)
	if initialized.Method != "notifications/initialized" {
		t.Fatalf("expected initialized notification, got %#v", initialized)
	}
}

func TestConnectionRejectsUnsupportedVersion(t *testing.T) {
	target := newMemoryTransport()
	connection := NewConnection(target, Config{})
	go respondToInitialize(t, target, "2099-01-01")
	err := connection.Connect(context.Background())
	if !errors.Is(err, protocol.ErrUnsupportedVersion) {
		t.Fatalf("expected unsupported version, got %v", err)
	}
	if connection.State() != StateDisconnected {
		t.Fatalf("unexpected state: %s", connection.State())
	}
}

func TestRequestManagerResponseRemoteErrorAndUnknownResponse(t *testing.T) {
	target := newMemoryTransport()
	manager := NewRequestManager(target)
	target.Start(context.Background())

	result := make(chan error, 1)
	go func() {
		raw, err := manager.Call(context.Background(), "tools/list", map[string]any{}, CallOptions{})
		if err == nil {
			var response map[string]any
			err = json.Unmarshal(raw, &response)
			if response["ok"] != true {
				err = errors.New("unexpected result")
			}
		}
		result <- err
	}()
	request := receiveSent(t, target)
	response, _ := protocol.Response(request.ID, map[string]any{"ok": true})
	if err := manager.HandleResponse(response); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := manager.HandleResponse(response); !errors.Is(err, protocol.ErrUnknownResponse) {
		t.Fatalf("expected unknown response, got %v", err)
	}

	remoteResult := make(chan error, 1)
	go func() {
		_, err := manager.Call(context.Background(), "tools/call", map[string]any{}, CallOptions{})
		remoteResult <- err
	}()
	remoteRequest := receiveSent(t, target)
	remoteResponse, _ := protocol.ErrorResponse(remoteRequest.ID, protocol.NewError(protocol.ErrorInvalidParams, "bad tool input", nil))
	if err := manager.HandleResponse(remoteResponse); err != nil {
		t.Fatal(err)
	}
	var rpcErr *protocol.RPCError
	if err := <-remoteResult; !errors.As(err, &rpcErr) || rpcErr.Code != protocol.ErrorInvalidParams {
		t.Fatalf("expected remote JSON-RPC error, got %v", err)
	}
}

func TestRequestManagerTimeoutCancelAndProgress(t *testing.T) {
	target := newMemoryTransport()
	manager := NewRequestManager(target)
	target.Start(context.Background())

	timedOut := make(chan error, 1)
	go func() {
		_, err := manager.Call(context.Background(), "resources/read", map[string]any{"uri": "test://slow"}, CallOptions{Timeout: 20 * time.Millisecond})
		timedOut <- err
	}()
	request := receiveSent(t, target)
	if request.Method != "resources/read" {
		t.Fatalf("unexpected request: %s", request.Method)
	}
	if err := <-timedOut; !errors.Is(err, protocol.ErrRequestTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
	cancelled := receiveSent(t, target)
	if cancelled.Method != "notifications/cancelled" {
		t.Fatalf("expected cancellation, got %s", cancelled.Method)
	}

	progressReceived := make(chan protocol.ProgressParams, 1)
	callDone := make(chan error, 1)
	go func() {
		_, err := manager.Call(context.Background(), "tools/call", map[string]any{"name": "slow"}, CallOptions{ProgressToken: "progress-1", OnProgress: func(progress protocol.ProgressParams) { progressReceived <- progress }})
		callDone <- err
	}()
	progressRequest := receiveSent(t, target)
	var params map[string]any
	if err := json.Unmarshal(progressRequest.Params, &params); err != nil {
		t.Fatal(err)
	}
	meta, _ := params["_meta"].(map[string]any)
	if meta["progressToken"] != "progress-1" {
		t.Fatalf("progress token missing: %#v", params)
	}
	if !manager.HandleProgress(protocol.ProgressParams{ProgressToken: "progress-1", Progress: 1, Total: 2, Message: "working"}) {
		t.Fatal("progress was not routed")
	}
	if progress := <-progressReceived; progress.Message != "working" {
		t.Fatalf("unexpected progress: %#v", progress)
	}
	progressResponse, _ := protocol.Response(progressRequest.ID, map[string]any{"done": true})
	if err := manager.HandleResponse(progressResponse); err != nil {
		t.Fatal(err)
	}
	if err := <-callDone; err != nil {
		t.Fatal(err)
	}
}

func TestConnectionHandlesServerRequestsNotificationsAndDuplicateIDs(t *testing.T) {
	target := newMemoryTransport()
	connection := NewConnection(target, Config{})
	go respondToInitialize(t, target, protocol.LatestProtocolVersion)
	if err := connection.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = receiveSent(t, target)

	notified := make(chan string, 1)
	connection.RegisterNotificationHandler("notifications/tools/list_changed", func(context.Context, json.RawMessage) { notified <- "changed" })
	notification, _ := protocol.Notification("notifications/tools/list_changed", nil)
	target.receive <- notification
	if value := <-notified; value != "changed" {
		t.Fatalf("unexpected notification: %s", value)
	}
	unknown, _ := protocol.Notification("notifications/unknown", nil)
	target.receive <- unknown

	connection.RegisterRequestHandler("sampling/createMessage", func(context.Context, json.RawMessage) (any, *protocol.RPCError) {
		return map[string]any{"model": "safe-model", "content": "ok"}, nil
	})
	serverRequest, _ := protocol.Request("server-1", "sampling/createMessage", map[string]any{"maxTokens": 10})
	target.receive <- serverRequest
	serverResponse := receiveSent(t, target)
	if kind, _ := serverResponse.Kind(); kind != protocol.MessageResponse {
		t.Fatalf("unexpected server response: %#v", serverResponse)
	}

	unknownRequest, _ := protocol.Request("server-2", "unknown/method", nil)
	target.receive <- unknownRequest
	unknownResponse := receiveSent(t, target)
	if unknownResponse.Error == nil || unknownResponse.Error.Code != protocol.ErrorMethodNotFound {
		t.Fatalf("expected method not found, got %#v", unknownResponse)
	}

	release := make(chan struct{})
	connection.RegisterRequestHandler("elicitation/create", func(context.Context, json.RawMessage) (any, *protocol.RPCError) {
		<-release
		return map[string]any{"action": "decline"}, nil
	})
	duplicate, _ := protocol.Request("duplicate", "elicitation/create", nil)
	target.receive <- duplicate
	target.receive <- duplicate
	duplicateResponse := receiveSent(t, target)
	if duplicateResponse.Error == nil || duplicateResponse.Error.Code != protocol.ErrorInvalidRequest {
		t.Fatalf("expected duplicate id rejection, got %#v", duplicateResponse)
	}
	close(release)
	_ = receiveSent(t, target)
}

func TestConnectionCancelsInboundServerRequest(t *testing.T) {
	target := newMemoryTransport()
	connection := NewConnection(target, Config{})
	go respondToInitialize(t, target, protocol.LatestProtocolVersion)
	if err := connection.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = receiveSent(t, target)
	cancelled := make(chan struct{}, 1)
	connection.RegisterRequestHandler("sampling/createMessage", func(ctx context.Context, _ json.RawMessage) (any, *protocol.RPCError) {
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, protocol.NewError(protocol.ErrorInternal, "cancelled", nil)
	})
	request, _ := protocol.Request("cancel-me", "sampling/createMessage", map[string]any{"maxTokens": 10})
	target.receive <- request
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		connection.mu.RLock()
		active := len(connection.inbound)
		connection.mu.RUnlock()
		if active == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	notification, _ := protocol.Notification("notifications/cancelled", protocol.CancelledParams{RequestID: "cancel-me", Reason: "server cancelled"})
	target.receive <- notification
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("inbound request was not cancelled")
	}
	response := receiveSent(t, target)
	if response.Error == nil {
		t.Fatalf("expected cancellation error response: %#v", response)
	}
}

func respondToInitialize(t *testing.T, target *memoryTransport, version string) {
	t.Helper()
	request := receiveSent(t, target)
	if request.Method != "initialize" {
		t.Errorf("expected initialize, got %s", request.Method)
		return
	}
	var params protocol.InitializeParams
	if err := json.Unmarshal(request.Params, &params); err != nil {
		t.Errorf("decode initialize: %v", err)
		return
	}
	if params.ProtocolVersion != protocol.LatestProtocolVersion {
		t.Errorf("unexpected requested version: %s", params.ProtocolVersion)
		return
	}
	response, err := protocol.Response(request.ID, protocol.InitializeResult{ProtocolVersion: version, Capabilities: map[string]any{"tools": map[string]any{"listChanged": true}}, ServerInfo: protocol.Implementation{Name: "test-server", Version: "1.0.0"}})
	if err != nil {
		t.Errorf("build initialize response: %v", err)
		return
	}
	target.receive <- response
}

func receiveSent(t *testing.T, target *memoryTransport) protocol.Message {
	t.Helper()
	select {
	case message := <-target.sent:
		return message
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent message")
		return protocol.Message{}
	}
}
