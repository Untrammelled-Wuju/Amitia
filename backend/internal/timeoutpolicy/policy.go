package timeoutpolicy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type Settings struct {
	Disabled bool `json:"disabled"`
	Seconds  int  `json:"seconds"`
}

var current atomic.Pointer[Settings]

func Default() Settings { return Settings{Seconds: 180} }

func Validate(s Settings) error {
	if s.Seconds < 30 || s.Seconds > 1800 || s.Seconds%30 != 0 {
		return errors.New("超时时间必须为 30 至 1800 秒，步长为 30 秒")
	}
	return nil
}

func Configure(s Settings) func() {
	previous := current.Swap(&s)
	return func() { current.Store(previous) }
}

func Current() (Settings, bool) {
	s := current.Load()
	if s == nil {
		return Default(), false
	}
	return *s, true
}

func Duration(fallback time.Duration) time.Duration {
	s, active := Current()
	if !active {
		return fallback
	}
	if s.Disabled {
		return 0
	}
	return time.Duration(s.Seconds) * time.Second
}

func WithTimeout(ctx context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	d := Duration(fallback)
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

func After(fallback time.Duration) <-chan time.Time {
	d := Duration(fallback)
	if d <= 0 {
		return nil
	}
	return time.After(d)
}

type transport struct {
	base     http.RoundTripper
	fallback time.Duration
}
type responseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *responseBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && strings.Contains(req.Header.Get("Accept"), "text/event-stream") {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := WithTimeout(req.Context(), t.fallback)
	response, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	if response.Body == nil {
		cancel()
	} else {
		response.Body = &responseBody{ReadCloser: response.Body, cancel: cancel}
	}
	return response, nil
}

func (t transport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func Client(client *http.Client) *http.Client {
	copy := *client
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = transport{base: base, fallback: copy.Timeout}
	copy.Timeout = 0
	return &copy
}
