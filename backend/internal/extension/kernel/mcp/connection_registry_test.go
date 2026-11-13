package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/mcpruntime/protocol"
)

func TestCanonicalRemoteRegistryIsolatesServiceOperations(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})
	var release sync.Once
	var slowStarts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var message protocol.Message
		if err := json.NewDecoder(req.Body).Decode(&message); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if message.Method != "initialize" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if req.URL.Path == "/slow" {
			if slowStarts.Add(1) == 1 {
				close(started)
			}
			select {
			case <-finish:
			case <-req.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": message.ID,
			"result": protocol.InitializeResult{ProtocolVersion: protocol.LatestProtocolVersion,
				ServerInfo: protocol.Implementation{Name: "qa-http", Version: "1"}},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	defer release.Do(func() { close(finish) })
	registry := NewCanonicalRemoteRegistry(NewCanonicalRemoteFactory())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t.Cleanup(func() {
		if err := registry.CloseAll(context.Background()); err != nil {
			t.Error(err)
		}
	})
	fast := MCPRemoteSpec{ServerID: "fast", Endpoint: server.URL + "/fast", AllowLoopback: true}
	slow := MCPRemoteSpec{ServerID: "slow", Endpoint: server.URL + "/slow", AllowLoopback: true}
	if _, err := registry.StartOrGet(ctx, fast); err != nil {
		t.Fatal(err)
	}
	type startResult struct {
		conn *CanonicalRemoteConnection
		err  error
	}
	start := func() <-chan startResult {
		result := make(chan startResult, 1)
		go func() {
			conn, err := registry.StartOrGet(ctx, slow)
			result <- startResult{conn: conn, err: err}
		}()
		return result
	}
	first := start()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	closed := make(chan error, 1)
	go func() { closed <- registry.Close(ctx, fast.ServerID) }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("closing an unrelated MCP service waited for initialization")
	}
	if _, ok := registry.Get(fast.ServerID); ok {
		t.Fatal("closed connection still registered")
	}
	waiting, stopWaiting := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stopWaiting()
	if _, err := registry.StartOrGet(waiting, slow); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("same-service wait lost cancellation: %v", err)
	}
	second := start()
	release.Do(func() { close(finish) })
	one, two := <-first, <-second
	if one.err != nil || two.err != nil || one.conn != two.conn || slowStarts.Load() != 1 {
		t.Fatalf("handshakes=%d, first=%+v, second=%+v", slowStarts.Load(), one, two)
	}
}

func TestCanonicalRemoteRegistryDrainCancelsInFlightStart(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	var signal sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var message protocol.Message
		if err := json.NewDecoder(req.Body).Decode(&message); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		signal.Do(func() { close(started) })
		select {
		case <-req.Context().Done():
			close(canceled)
		case <-time.After(4 * time.Second):
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}))
	defer server.Close()
	registry := NewCanonicalRemoteRegistry(NewCanonicalRemoteFactory())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := registry.StartOrGet(ctx, MCPRemoteSpec{ServerID: "slow", Endpoint: server.URL, AllowLoopback: true})
		result <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	drainCtx, cancelDrain := context.WithTimeout(ctx, time.Second)
	defer cancelDrain()
	if err := registry.CloseAll(drainCtx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("initialization lost cancellation: %v", err)
	}
	if len(registry.List()) != 0 {
		t.Fatal("canceled initialization left a registered connection")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("loopback MCP server did not observe the canceled handshake")
	}
}
