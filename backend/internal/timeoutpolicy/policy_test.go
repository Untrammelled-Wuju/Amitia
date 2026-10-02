package timeoutpolicy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOperationTimeoutContexts(t *testing.T) {
	restore := Configure(Settings{Seconds: 60})
	defer restore()
	ctx, cancel := WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) < 59*time.Second {
		t.Fatal("global duration was not applied")
	}
	Configure(Settings{Disabled: true, Seconds: 60})
	parent, stop := context.WithCancel(context.Background())
	unlimited, finish := WithTimeout(parent, time.Millisecond)
	defer finish()
	if _, ok := unlimited.Deadline(); ok {
		t.Fatal("disabled policy retained a deadline")
	}
	stop()
	if unlimited.Err() != context.Canceled {
		t.Fatal("user cancellation was lost")
	}
	short, end := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer end()
	child, done := WithTimeout(short, time.Minute)
	defer done()
	if d, ok := child.Deadline(); !ok || time.Until(d) > 20*time.Millisecond {
		t.Fatal("parent deadline was lost")
	}
}

func TestOperationTimeoutHTTPHotReload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(25 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	}))
	defer server.Close()
	client := Client(&http.Client{Timeout: time.Millisecond})
	restore := Configure(Settings{Disabled: true, Seconds: 30})
	defer restore()
	for _, disabled := range []bool{true, false} {
		Configure(Settings{Disabled: disabled, Seconds: 30})
		response, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != "done" {
			t.Fatalf("stream body canceled early: %q %v", body, err)
		}
	}
	Configure(Settings{Disabled: true, Seconds: 30})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("canceled HTTP request was accepted")
	}
}

func TestOperationTimeoutValidation(t *testing.T) {
	for _, seconds := range []int{-1, 0, 29, 31, 1801} {
		if Validate(Settings{Seconds: seconds}) == nil {
			t.Fatalf("accepted invalid duration %d", seconds)
		}
	}
	for _, seconds := range []int{30, 180, 1800} {
		if err := Validate(Settings{Seconds: seconds}); err != nil {
			t.Fatal(err)
		}
	}
}
