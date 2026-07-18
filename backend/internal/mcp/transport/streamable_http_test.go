package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

func TestValidateEndpointPolicy(t *testing.T) {
	loopback, err := ValidateEndpoint(context.Background(), "http://127.0.0.1:18899/mcp", EndpointPolicy{AllowLoopback: true})
	if err != nil || loopback.Class != EndpointLoopbackHTTP {
		t.Fatalf("unexpected loopback result: %#v %v", loopback, err)
	}
	if _, err := ValidateEndpoint(context.Background(), "http://127.0.0.1:18899/mcp", EndpointPolicy{}); err == nil {
		t.Fatal("expected loopback confirmation requirement")
	}
	if _, err := ValidateEndpoint(context.Background(), "http://169.254.169.254/latest", EndpointPolicy{AllowPrivate: true}); err == nil {
		t.Fatal("expected metadata endpoint rejection")
	}
	if _, err := ValidateEndpoint(context.Background(), "file:///tmp/mcp", EndpointPolicy{}); err == nil {
		t.Fatal("expected non-http scheme rejection")
	}
	if _, err := ValidateEndpoint(context.Background(), "https://user:password@example.com/mcp", EndpointPolicy{}); err == nil {
		t.Fatal("expected embedded credentials rejection")
	}
	if _, err := ValidateEndpoint(context.Background(), "http://8.8.8.8/mcp", EndpointPolicy{}); err == nil {
		t.Fatal("expected public HTTP rejection")
	}
	remote, err := ValidateEndpoint(context.Background(), "https://8.8.8.8/mcp", EndpointPolicy{})
	if err != nil || remote.Class != EndpointRemoteHTTPS {
		t.Fatalf("unexpected HTTPS result: %#v %v", remote, err)
	}
}

func TestStreamableHTTPJSONSessionAndHeaders(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requests++
		current := requests
		mu.Unlock()
		if request.Header.Get("Accept") != "application/json, text/event-stream" {
			t.Errorf("unexpected accept header: %s", request.Header.Get("Accept"))
		}
		var message protocol.Message
		if err := json.NewDecoder(request.Body).Decode(&message); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if current == 1 {
			writer.Header().Set("MCP-Session-Id", "session-1")
			writer.Header().Set("Content-Type", "application/json")
			response, _ := protocol.Response(message.ID, protocol.InitializeResult{ProtocolVersion: protocol.LatestProtocolVersion, Capabilities: map[string]any{}, ServerInfo: protocol.Implementation{Name: "http-server", Version: "1.0.0"}})
			json.NewEncoder(writer).Encode(response)
			return
		}
		if request.Header.Get("MCP-Session-Id") != "session-1" || request.Header.Get("MCP-Protocol-Version") != protocol.LatestProtocolVersion {
			t.Errorf("session or protocol header missing")
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	target := NewStreamableHTTP(HTTPConfig{Endpoint: server.URL, Policy: EndpointPolicy{AllowLoopback: true}, Timeout: time.Second})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	initialize, _ := protocol.Request(1, "initialize", protocol.InitializeParams{ProtocolVersion: protocol.LatestProtocolVersion})
	if err := target.Send(context.Background(), initialize); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-target.Receive():
		if response.Error != nil {
			t.Fatalf("unexpected response: %#v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("missing initialize response")
	}
	if target.SessionID() != "session-1" {
		t.Fatalf("session was not stored: %s", target.SessionID())
	}
	target.SetProtocolVersion(protocol.LatestProtocolVersion)
	initialized, _ := protocol.Notification("notifications/initialized", nil)
	if err := target.Send(context.Background(), initialized); err != nil {
		t.Fatal(err)
	}
}

func TestStreamableHTTPSSEAndResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var message protocol.Message
		json.NewDecoder(request.Body).Decode(&message)
		writer.Header().Set("Content-Type", "text/event-stream")
		response, _ := protocol.Response(message.ID, map[string]any{"tools": []any{}})
		data, _ := json.Marshal(response)
		fmt.Fprintf(writer, "event: message\ndata: %s\n\n", data)
	}))
	defer server.Close()
	target := NewStreamableHTTP(HTTPConfig{Endpoint: server.URL, Policy: EndpointPolicy{AllowLoopback: true}, Timeout: time.Second, MaxMessageBytes: 2048})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	request, _ := protocol.Request("tools", "tools/list", nil)
	if err := target.Send(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-target.Receive():
		if key, _ := protocol.CanonicalID(message.ID, false); key != "s:tools" {
			t.Fatalf("unexpected SSE response: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("missing SSE response")
	}

	largeServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"jsonrpc":"2.0","id":1,"result":{"value":"%s"}}`, strings.Repeat("x", 256))
	}))
	defer largeServer.Close()
	limited := NewStreamableHTTP(HTTPConfig{Endpoint: largeServer.URL, Policy: EndpointPolicy{AllowLoopback: true}, MaxMessageBytes: 64})
	if err := limited.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	smallRequest, _ := protocol.Request(1, "ping", nil)
	if err := limited.Send(context.Background(), smallRequest); !errors.Is(err, protocol.ErrMessageTooLarge) {
		t.Fatalf("expected response limit, got %v", err)
	}
}

func TestStreamableHTTPServerStreamResumesWithLastEventID(t *testing.T) {
	var mu sync.Mutex
	getCount := 0
	resumed := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			var message protocol.Message
			json.NewDecoder(request.Body).Decode(&message)
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("MCP-Session-Id", "resume-session")
			response, _ := protocol.Response(message.ID, protocol.InitializeResult{ProtocolVersion: protocol.LatestProtocolVersion, Capabilities: map[string]any{}, ServerInfo: protocol.Implementation{Name: "resume", Version: "1"}})
			json.NewEncoder(writer).Encode(response)
			return
		}
		mu.Lock()
		getCount++
		current := getCount
		mu.Unlock()
		writer.Header().Set("Content-Type", "text/event-stream")
		if current == 1 {
			message, _ := protocol.Notification("notifications/tools/list_changed", nil)
			data, _ := json.Marshal(message)
			fmt.Fprintf(writer, "id: event-1\nretry: 100\ndata: %s\n\n", data)
			return
		}
		resumed <- request.Header.Get("Last-Event-ID")
		<-request.Context().Done()
	}))
	defer server.Close()
	target := NewStreamableHTTP(HTTPConfig{Endpoint: server.URL, Policy: EndpointPolicy{AllowLoopback: true}, Timeout: time.Second})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	initialize, _ := protocol.Request(1, "initialize", protocol.InitializeParams{ProtocolVersion: protocol.LatestProtocolVersion})
	if err := target.Send(context.Background(), initialize); err != nil {
		t.Fatal(err)
	}
	<-target.Receive()
	target.SetProtocolVersion(protocol.LatestProtocolVersion)
	if err := target.StartServerStream(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-target.Receive():
	case <-time.After(time.Second):
		t.Fatal("missing server notification")
	}
	select {
	case value := <-resumed:
		if value != "event-1" {
			t.Fatalf("expected Last-Event-ID event-1, got %q", value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server stream did not reconnect")
	}
	if err := target.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStreamableHTTPSessionExpirySignalsDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			var message protocol.Message
			json.NewDecoder(request.Body).Decode(&message)
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("MCP-Session-Id", "expired-session")
			response, _ := protocol.Response(message.ID, protocol.InitializeResult{ProtocolVersion: protocol.LatestProtocolVersion, Capabilities: map[string]any{}, ServerInfo: protocol.Implementation{Name: "expired", Version: "1"}})
			json.NewEncoder(writer).Encode(response)
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	target := NewStreamableHTTP(HTTPConfig{Endpoint: server.URL, Policy: EndpointPolicy{AllowLoopback: true}, Timeout: time.Second})
	if err := target.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	initialize, _ := protocol.Request(1, "initialize", protocol.InitializeParams{ProtocolVersion: protocol.LatestProtocolVersion})
	if err := target.Send(context.Background(), initialize); err != nil {
		t.Fatal(err)
	}
	<-target.Receive()
	if err := target.StartServerStream(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-target.Done():
		if target.State() != StateError {
			t.Fatalf("expected error state, got %s", target.State())
		}
	case <-time.After(time.Second):
		t.Fatal("session expiry did not signal transport failure")
	}
}
