package realtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestHandleSessionCleansVoiceSessionOnStop(t *testing.T) {
	clearActiveVoiceSessions()
	t.Cleanup(clearActiveVoiceSessions)

	upstreamDone := make(chan struct{})
	upstreamErr := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			upstreamErr <- err
			return
		}
		defer close(upstreamDone)
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			upstreamErr <- err
			return
		}
		if frame, err := parseFrame(data); err != nil || frame.EventCode != EvtStartConnection {
			upstreamErr <- err
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, buildEventFrame(MsgTypeFullServer, 50, "", []byte("{}"))); err != nil {
			upstreamErr <- err
			return
		}

		_, data, err = conn.ReadMessage()
		if err != nil {
			upstreamErr <- err
			return
		}
		frame, err := parseFrame(data)
		if err != nil {
			upstreamErr <- err
			return
		}
		if frame.EventCode != EvtStartSession || frame.SessionID == "" {
			upstreamErr <- errUnexpectedFrame
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, buildEventFrame(MsgTypeFullServer, 150, frame.SessionID, []byte(`{"dialog_id":"dialog-test"}`))); err != nil {
			upstreamErr <- err
			return
		}

		for {
			_, data, err = conn.ReadMessage()
			if err != nil {
				return
			}
			frame, err = parseFrame(data)
			if err != nil {
				upstreamErr <- err
				return
			}
			if frame.EventCode == EvtFinishConnection {
				return
			}
		}
	}))
	defer upstream.Close()

	originalURI := volcanoRealtimeURI
	volcanoRealtimeURI = func() string {
		return "ws" + strings.TrimPrefix(upstream.URL, "http")
	}
	t.Cleanup(func() {
		volcanoRealtimeURI = originalURI
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/realtime/session", HandleSession)
	server := httptest.NewServer(router)
	defer server.Close()

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/realtime/session?apiKey=test", nil)
	if err != nil {
		t.Fatalf("dial realtime session: %v", err)
	}
	defer client.Close()

	var connected struct {
		Event    string `json:"event"`
		Data     string `json:"data"`
		DialogID string `json:"dialogId"`
	}
	if err := client.ReadJSON(&connected); err != nil {
		t.Fatalf("read connected event: %v", err)
	}
	if connected.Event != "connected" || connected.DialogID != "dialog-test" {
		b, _ := json.Marshal(connected)
		t.Fatalf("unexpected connected event: %s", string(b))
	}
	if countActiveVoiceSessions() != 1 {
		t.Fatalf("expected active voice session while websocket is open")
	}
	if err := client.WriteJSON(map[string]string{"event": "stop"}); err != nil {
		t.Fatalf("write stop event: %v", err)
	}

	select {
	case <-upstreamDone:
	case err := <-upstreamErr:
		t.Fatalf("upstream failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not receive finish connection")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if countActiveVoiceSessions() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected voice session registry to be empty, got %d", countActiveVoiceSessions())
}

var errUnexpectedFrame = &unexpectedFrameError{}

type unexpectedFrameError struct{}

func (e *unexpectedFrameError) Error() string {
	return "unexpected frame"
}

func clearActiveVoiceSessions() {
	activeVoiceSessions.Range(func(key, value interface{}) bool {
		activeVoiceSessions.Delete(key)
		return true
	})
}

func countActiveVoiceSessions() int {
	count := 0
	activeVoiceSessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
