package plugincontract

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func TestStdioAdapterProviderTransportRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	transport := NewStdioAdapterProviderTransport(client, client)
	defer transport.Close()
	runFakeAdapterServer(t, server)
	var result struct {
		ID string `json:"id"`
	}
	if err := transport.Call(context.Background(), ExternalAdapterMethodBuild, map[string]any{"id": "tool"}, &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != "tool" {
		t.Fatalf("result id = %s", result.ID)
	}
}

func TestStdioAdapterProviderTransportHonorsContextCancellation(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	transport := NewStdioAdapterProviderTransport(client, client)
	defer transport.Close()
	runSlowAdapterServer(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := transport.Call(ctx, ExternalAdapterMethodBuild, map[string]any{}, &map[string]any{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
}

func runFakeAdapterServer(t *testing.T, server net.Conn) {
	t.Helper()
	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var request externalAdapterRPCRequest
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				continue
			}
			var params struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(request.Params, &params)
			response, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result":  map[string]any{"id": params.ID},
			})
			_, _ = server.Write(append(response, '\n'))
		}
	}()
}

func runSlowAdapterServer(t *testing.T, server net.Conn) {
	t.Helper()
	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var request externalAdapterRPCRequest
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				continue
			}
			time.Sleep(30 * time.Millisecond)
			response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{}})
			_, _ = server.Write(append(response, '\n'))
		}
	}()
}
