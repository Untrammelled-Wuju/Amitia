package agent

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/lan"
)

func TestLANEventsReportsChangesAndUnsubscribes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &LocalHandler{}
	router := gin.New()
	router.GET("/events", h.handleLANEvents)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	readEvent := func() string {
		t.Helper()
		var event strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\n" {
				return event.String()
			}
			event.WriteString(line)
		}
	}
	if initial := readEvent(); !strings.Contains(initial, `"revision":0`) {
		t.Fatalf("initial event: %s", initial)
	}
	h.SetLANEndpoints([]lan.Endpoint{{URL: "https://192.168.1.2:54796"}})
	if changed := readEvent(); !strings.Contains(changed, `"revision":1`) {
		t.Fatalf("changed event: %s", changed)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		h.mu.RLock()
		count := len(h.lanSubscribers)
		h.mu.RUnlock()
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("subscriber was not removed")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestLANEventsTrackRealListenerReplacement(t *testing.T) {
	addresses, err := lan.PrivateAddresses()
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) == 0 {
		t.Skip("no private IPv4 interface")
	}
	address := addresses[0]
	allocatePort := func() int {
		t.Helper()
		listener, err := net.Listen("tcp", net.JoinHostPort(address.String(), "0"))
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return port
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "ready"); err != nil {
			t.Error(err)
		}
	})
	dataDir := t.TempDir()
	first, err := lan.Start(handler, dataDir, "test-core", allocatePort(), []net.IP{address})
	if err != nil {
		t.Fatal(err)
	}
	second, err := lan.Start(handler, dataDir, "test-core", allocatePort(), []net.IP{address})
	if err != nil {
		if closeErr := first.Close(context.Background()); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	firstClosed := false
	t.Cleanup(func() {
		if !firstClosed {
			if err := first.Close(context.Background()); err != nil {
				t.Error(err)
			}
		}
		if err := second.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if first.Endpoints[0].URL == second.Endpoints[0].URL {
		t.Fatal("listener replacement retained the same endpoint")
	}
	if first.Endpoints[0].Fingerprint != second.Endpoints[0].Fingerprint {
		t.Fatal("listener replacement changed the persistent key")
	}
	h := &LocalHandler{}
	h.SetLANEndpoints(first.Endpoints)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events", h.handleLANEvents)
	eventServer := httptest.NewServer(router)
	defer eventServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventServer.URL+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	readEvent := func(expected string) {
		t.Helper()
		var value strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\n" {
				break
			}
			value.WriteString(line)
		}
		if !strings.Contains(value.String(), expected) {
			t.Fatalf("event %q does not contain %q", value.String(), expected)
		}
	}
	readEvent(`"revision":1`)
	h.SetLANEndpoints(nil)
	readEvent(`"revision":2`)
	if err := first.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstClosed = true
	oldConnection, err := net.DialTimeout("tcp", strings.TrimPrefix(first.Endpoints[0].URL, "https://"), time.Second)
	if err == nil {
		if closeErr := oldConnection.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("old listener remains reachable")
	}
	h.SetLANEndpoints(second.Endpoints)
	readEvent(`"revision":3`)
	config, err := lan.PinnedTLS(second.Endpoints[0])
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{TLSClientConfig: config}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	result, err := client.Get(second.Endpoints[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil || result.StatusCode != http.StatusOK || string(body) != "ready" {
		t.Fatalf("replacement response status=%d body=%q error=%v", result.StatusCode, body, err)
	}
}
