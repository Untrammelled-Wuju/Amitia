package hostapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	kernelhostapi "github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func restrictedSocketDefinition(port int, transports ...string) *trusted_service.ServiceRuntimeDefinition {
	return &trusted_service.ServiceRuntimeDefinition{Network: trusted_service.ServiceNetworkPolicy{
		Mode:              "restricted",
		Enforce:           true,
		RequireProxy:      true,
		AllowedPorts:      []int{port},
		AllowedTransports: append([]string(nil), transports...),
		AllowHostLoopback: true,
		MaxConnections:    4,
	}}
}

func socketRouteHost(definition *trusted_service.ServiceRuntimeDefinition) *networkRouteHost {
	return &networkRouteHost{
		topology:    networkTestTopology{serviceID: "runtime-service", definitionID: "definition-1"},
		supervisor:  networkTestDefinitions{definition: definition},
		connections: newNetworkConnectionManager(),
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decodeCallOutput[T any](t *testing.T, result kernelhostapi.CallResult) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("decode call output: %v; raw=%s", err, result.Output)
	}
	return out
}

func startTCPEcho(t *testing.T) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buffer := make([]byte, 4096)
				for {
					n, readErr := c.Read(buffer)
					if n > 0 {
						_, _ = c.Write(append([]byte("tcp:"), buffer[:n]...))
					}
					if readErr != nil {
						return
					}
				}
			}(conn)
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	return port, func() {
		_ = listener.Close()
		select {
		case <-closed:
		case <-time.After(time.Second):
		}
	}
}

func startUDPEcho(t *testing.T) (int, func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, 4096)
		for {
			n, addr, readErr := conn.ReadFromUDP(buffer)
			if readErr != nil {
				return
			}
			_, _ = conn.WriteToUDP(append([]byte("udp:"), buffer[:n]...), addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port, func() {
		_ = conn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
}

func TestRestrictedTCPHostLoopbackRoundTripAndGenerationIsolation(t *testing.T) {
	port, stop := startTCPEcho(t)
	defer stop()
	h := socketRouteHost(restrictedSocketDefinition(port, "tcp"))
	identity := gameRuntimeIdentity()

	opened, err := h.tcpOpen(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketOpenInput{Target: "host-loopback", Port: port, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	handle := decodeCallOutput[NetworkSocketOpenOutput](t, opened)
	if handle.HandleID == "" || handle.Transport != "tcp" {
		t.Fatalf("unexpected open output: %+v", handle)
	}
	payload := base64.StdEncoding.EncodeToString([]byte("hello"))
	if _, err := h.tcpWrite(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketWriteInput{HandleID: handle.HandleID, DataBase64: payload, TimeoutMs: 2000}),
	}); err != nil {
		t.Fatal(err)
	}
	readResult, err := h.tcpRead(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketReadInput{HandleID: handle.HandleID, MaxBytes: 1024, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	read := decodeCallOutput[NetworkSocketReadOutput](t, readResult)
	decoded, _ := base64.StdEncoding.DecodeString(read.DataBase64)
	if string(decoded) != "tcp:hello" {
		t.Fatalf("tcp roundtrip = %q", decoded)
	}

	staleIdentity := identity
	staleIdentity.Generation++
	if _, err := h.tcpRead(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: staleIdentity,
		Input:           mustJSON(t, NetworkSocketReadInput{HandleID: handle.HandleID, MaxBytes: 1, TimeoutMs: 100}),
	}); err == nil {
		t.Fatal("next generation reused a previous-generation socket handle")
	}
}

func TestRestrictedUDPHostLoopbackRoundTrip(t *testing.T) {
	port, stop := startUDPEcho(t)
	defer stop()
	h := socketRouteHost(restrictedSocketDefinition(port, "udp"))
	identity := gameRuntimeIdentity()
	opened, err := h.udpOpen(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketOpenInput{Target: "host-loopback", Port: port, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	handle := decodeCallOutput[NetworkSocketOpenOutput](t, opened)
	if _, err := h.udpSend(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input: mustJSON(t, NetworkSocketWriteInput{HandleID: handle.HandleID,
			DataBase64: base64.StdEncoding.EncodeToString([]byte("hello")), TimeoutMs: 2000}),
	}); err != nil {
		t.Fatal(err)
	}
	result, err := h.udpReceive(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketReadInput{HandleID: handle.HandleID, MaxBytes: 1024, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := decodeCallOutput[NetworkSocketReadOutput](t, result)
	decoded, _ := base64.StdEncoding.DecodeString(out.DataBase64)
	if string(decoded) != "udp:hello" {
		t.Fatalf("udp roundtrip = %q", decoded)
	}
	if _, err := h.udpClose(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity, Input: mustJSON(t, NetworkSocketCloseInput{HandleID: handle.HandleID}),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedWebSocketHostLoopbackRoundTrip(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			messageType, payload, readErr := conn.ReadMessage()
			if readErr != nil {
				return
			}
			if writeErr := conn.WriteMessage(messageType, append([]byte("ws:"), payload...)); writeErr != nil {
				return
			}
		}
	}))
	server.Listener.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Listener = listener
	server.Start()
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	h := socketRouteHost(restrictedSocketDefinition(port, "websocket"))
	identity := gameRuntimeIdentity()
	wsURL := fmt.Sprintf("ws://host-loopback:%d/echo", port)
	opened, err := h.webSocketOpen(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkWebSocketOpenInput{URL: wsURL, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	handle := decodeCallOutput[NetworkWebSocketOpenOutput](t, opened)
	if _, err := h.webSocketSend(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input: mustJSON(t, NetworkWebSocketSendInput{HandleID: handle.HandleID, MessageType: "text",
			DataBase64: base64.StdEncoding.EncodeToString([]byte("hello")), TimeoutMs: 2000}),
	}); err != nil {
		t.Fatal(err)
	}
	result, err := h.webSocketReceive(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkWebSocketReceiveInput{HandleID: handle.HandleID, TimeoutMs: 2000}),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := decodeCallOutput[NetworkWebSocketReceiveOutput](t, result)
	decoded, _ := base64.StdEncoding.DecodeString(out.DataBase64)
	if out.MessageType != "text" || string(decoded) != "ws:hello" {
		t.Fatalf("websocket roundtrip type=%s data=%q", out.MessageType, decoded)
	}
	if _, err := h.webSocketClose(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity, Input: mustJSON(t, NetworkSocketCloseInput{HandleID: handle.HandleID}),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedSocketPolicyRejectsTransportTargetAndPort(t *testing.T) {
	port, stop := startTCPEcho(t)
	defer stop()
	client, err := newRestrictedHTTPClient(restrictedSocketDefinition(port, "tcp").Network)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.resolveSocketAddresses(context.Background(), "udp", "host-loopback", port); err == nil {
		t.Fatal("undeclared UDP transport was allowed")
	}
	if _, err := client.resolveSocketAddresses(context.Background(), "tcp", "host-loopback", port+1); err == nil {
		t.Fatal("undeclared port was allowed")
	}
	withoutLoopback := restrictedSocketDefinition(port, "tcp")
	withoutLoopback.Network.AllowHostLoopback = false
	withoutLoopback.Network.AllowedIPs = []string{"127.0.0.1"}
	client, err = newRestrictedHTTPClient(withoutLoopback.Network)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.resolveSocketAddresses(context.Background(), "tcp", "host-loopback", port); err == nil {
		t.Fatal("host-loopback abstraction bypassed allowHostLoopback=false")
	}
}

func TestWebSocketURLUsesPortableHostLoopbackWithoutDNS(t *testing.T) {
	policy := restrictedSocketDefinition(19003, "websocket").Network
	client, err := newRestrictedHTTPClient(policy)
	if err != nil {
		t.Fatal(err)
	}
	parsed, port, err := validateWebSocketURL(context.Background(), client, "ws://host-loopback:19003/game")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Hostname() != "host-loopback" || port != 19003 {
		t.Fatalf("parsed websocket target = %s:%d", parsed.Hostname(), port)
	}
	if _, err := url.Parse("ws://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.String(), "host-loopback") {
		t.Fatal("portable host-loopback target was rewritten into platform-specific metadata")
	}
}

func TestNetworkSocketReadCancellationInterruptsBlockedIO(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	manager := newNetworkConnectionManager()
	defer manager.shutdown()
	identity := gameRuntimeIdentity()
	handleID, err := manager.add(identity, networkConnectionTCP, client, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := manager.get(identity, handleID, networkConnectionTCP)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, readErr := readApprovedSocket(ctx, handle, NetworkSocketReadInput{MaxBytes: 32, TimeoutMs: 120000})
		result <- readErr
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case readErr := <-result:
		if !errors.Is(readErr, context.Canceled) {
			t.Fatalf("read error = %v, want context.Canceled", readErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt blocked host-mediated socket read")
	}
}

func TestNetworkLifecycleClosesServiceHandlesImmediately(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	manager := newNetworkConnectionManager()
	lifecycle := &NetworkLifecycle{
		topology:    networkTestTopology{serviceID: "runtime-service", moduleID: "runtime-module", definitionID: "definition-1"},
		connections: manager,
	}
	defer lifecycle.Shutdown()
	identity := gameRuntimeIdentity()
	handleID, err := manager.add(identity, networkConnectionTCP, client, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := lifecycle.CountRuntimeNetworkHandles(identity.InstanceID); got != 1 {
		t.Fatalf("handle count before stop = %d, want 1", got)
	}
	lifecycle.OnServiceStopped(identity.InstanceID, "runtime-service")
	if got := lifecycle.CountRuntimeNetworkHandles(identity.InstanceID); got != 0 {
		t.Fatalf("handle count after stop = %d, want 0", got)
	}
	if _, err := manager.get(identity, handleID, networkConnectionTCP); err == nil {
		t.Fatal("stopped service retained a host-mediated socket handle")
	}
}

func TestNetworkSocketOpenDistinguishesPolicyDenialFromUnavailableEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	h := socketRouteHost(restrictedSocketDefinition(port, "tcp"))
	identity := gameRuntimeIdentity()
	_, err = h.tcpOpen(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketOpenInput{Target: "host-loopback", Port: port, TimeoutMs: 500}),
	})
	if !errors.Is(err, kernelhostapi.ErrHostUnavailable) {
		t.Fatalf("allowed but unavailable endpoint error = %v, want ErrHostUnavailable", err)
	}
	if errors.Is(err, kernelhostapi.ErrPermissionDenied) {
		t.Fatalf("allowed but unavailable endpoint was misreported as permission denial: %v", err)
	}

	_, err = h.tcpOpen(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: identity,
		Input:           mustJSON(t, NetworkSocketOpenInput{Target: "host-loopback", Port: port + 1, TimeoutMs: 500}),
	})
	if !errors.Is(err, kernelhostapi.ErrPermissionDenied) {
		t.Fatalf("undeclared port error = %v, want ErrPermissionDenied", err)
	}
}
