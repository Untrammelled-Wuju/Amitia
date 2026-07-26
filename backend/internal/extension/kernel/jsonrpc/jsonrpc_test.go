package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestEncodeDecodeRequest(t *testing.T) {
	id := NewStringID("req-1")
	req, err := EncodeRequest(id, "runtime.invoke", map[string]any{"entry": "tool.foo"})
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if req.JSONRPC != ProtocolVersion {
		t.Fatalf("expected jsonrpc %s, got %s", ProtocolVersion, req.JSONRPC)
	}
	if req.Method != "runtime.invoke" {
		t.Fatalf("expected method runtime.invoke, got %s", req.Method)
	}
	data, err := MarshalMessage(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Kind != KindRequest {
		t.Fatalf("expected request kind, got %s", env.Kind)
	}
	if env.Request.Method != "runtime.invoke" {
		t.Fatalf("method mismatch: %s", env.Request.Method)
	}
	if env.Request.ID.String() != "req-1" {
		t.Fatalf("id mismatch: %s", env.Request.ID.String())
	}
}

func TestEncodeDecodeResponse(t *testing.T) {
	id := NewNumberID(42)
	resp, err := EncodeResponse(id, map[string]any{"output": "ok"})
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	data, err := MarshalMessage(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Kind != KindResponse {
		t.Fatalf("expected response kind, got %s", env.Kind)
	}
	if env.Response.Error != nil {
		t.Fatalf("unexpected error: %v", env.Response.Error)
	}
}

func TestEncodeErrorResponse(t *testing.T) {
	id := NewStringID("err-1")
	resp := EncodeErrorResponse(id, PermissionDeniedError("not allowed"))
	data, err := MarshalMessage(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Kind != KindError {
		t.Fatalf("expected error kind, got %s", env.Kind)
	}
	if env.Response.Error == nil {
		t.Fatalf("missing error")
	}
	if env.Response.Error.Code != ErrCodePermissionDenied {
		t.Fatalf("code mismatch: %s", env.Response.Error.Code)
	}
	var data2 ErrorData
	if err := json.Unmarshal(env.Response.Error.Data, &data2); err != nil {
		t.Fatalf("unmarshal error data: %v", err)
	}
	if data2.Retryable {
		t.Fatalf("permission_denied should not be retryable")
	}
	if data2.Category != CategoryPermission {
		t.Fatalf("category mismatch: %s", data2.Category)
	}
}

func TestEncodeNotification(t *testing.T) {
	n, err := EncodeNotification("runtime.hello", map[string]any{"instance_id": "i1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	data, err := MarshalMessage(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Kind != KindNotification {
		t.Fatalf("expected notification kind, got %s", env.Kind)
	}
}

func TestFramingRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	framer := NewFramer(&buf, &buf)
	payload := []byte(`{"jsonrpc":"2.0","method":"diagnostic.ping","id":1}`)
	if err := framer.WriteFrame(payload); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("payload mismatch: got %s want %s", frame.Payload, payload)
	}
}

func TestFramingTooLarge(t *testing.T) {
	var buf bytes.Buffer
	framer := NewFramer(&buf, &buf)
	framer.SetMaxFrameBytes(8)
	err := framer.WriteFrame([]byte("this is too long"))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected frame too large error, got %v", err)
	}
}

func TestTransportRoundtrip(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	server := NewTransport(r2, w1)
	client := NewTransport(r1, w2)
	req, _ := EncodeRequest(NewStringID("ping-1"), "diagnostic.ping", nil)
	type result struct {
		env *Envelope
		err error
	}
	ch := make(chan result, 1)
	go func() {
		env, err := server.Read()
		ch <- result{env, err}
	}()
	if err := client.Write(req); err != nil {
		t.Fatalf("client write: %v", err)
	}
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("server read: %v", r.err)
		}
		if r.env.Kind != KindRequest || r.env.Request.Method != "diagnostic.ping" {
			t.Fatalf("unexpected envelope: %+v", r.env)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server read timed out")
	}
}

func TestRequestTrackerResolve(t *testing.T) {
	tracker := NewRequestTracker()
	pr, id, err := tracker.Track("test.method", 5*time.Second)
	if err != nil {
		t.Fatalf("track: %v", err)
	}
	if tracker.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", tracker.PendingCount())
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		resp, _ := EncodeResponse(id, map[string]any{"ok": true})
		_ = tracker.Resolve(resp)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := pr.Wait(ctx)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestRequestTrackerCancel(t *testing.T) {
	tracker := NewRequestTracker()
	pr, id, _ := tracker.Track("test.cancel", 5*time.Second)
	if err := tracker.Cancel(id, "user cancelled"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := pr.Wait(ctx)
	if err != nil {
		t.Fatalf("wait should return cancelled resp, got err: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != ErrCodeCancelled {
		t.Fatalf("expected cancelled error, got %+v", resp.Error)
	}
}

func TestRequestTrackerFailAll(t *testing.T) {
	tracker := NewRequestTracker()
	pr1, _, _ := tracker.Track("m1", 5*time.Second)
	pr2, _, _ := tracker.Track("m2", 5*time.Second)
	tracker.FailAll("transport closed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp1, _ := pr1.Wait(ctx)
	resp2, _ := pr2.Wait(ctx)
	if resp1.Error == nil || resp1.Error.Code != ErrCodeCancelled {
		t.Fatalf("expected cancelled for pr1, got %+v", resp1.Error)
	}
	if resp2.Error == nil || resp2.Error.Code != ErrCodeCancelled {
		t.Fatalf("expected cancelled for pr2, got %+v", resp2.Error)
	}
}

func TestCancellationRegistry(t *testing.T) {
	reg := NewCancellationRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	reg.Register("op-1", cancel)
	if !reg.Cancel("op-1", "test") {
		t.Fatalf("expected cancel to succeed")
	}
	if reg.Reason("op-1") != "test" {
		t.Fatalf("reason mismatch: %s", reg.Reason("op-1"))
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("context not cancelled")
	}
}

func TestStreamSendRecv(t *testing.T) {
	s := NewStream("stream-1", "stream.test", "duplex", 1024, 4, 8)
	if err := s.SendChunk([]byte("hello")); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := s.SendChunk([]byte("world")); err != nil {
		t.Fatalf("send: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	data, err := s.RecvChunk(ctx)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %s", data)
	}
	data, err = s.RecvChunk(ctx)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if string(data) != "world" {
		t.Fatalf("expected world, got %s", data)
	}
}

func TestStreamNoCredit(t *testing.T) {
	s := NewStream("stream-2", "stream.test", "duplex", 1024, 0, 8)
	err := s.SendChunk([]byte("hello"))
	if !errors.Is(err, ErrNoCredit) {
		t.Fatalf("expected no credit, got %v", err)
	}
	s.AddCredit(2)
	if err := s.SendChunk([]byte("hello")); err != nil {
		t.Fatalf("send should succeed with credit: %v", err)
	}
}

func TestStreamRegistry(t *testing.T) {
	reg := NewStreamRegistry(2)
	s1 := NewStream("a", "m", "duplex", 1024, 1, 8)
	s2 := NewStream("b", "m", "duplex", 1024, 1, 8)
	s3 := NewStream("c", "m", "duplex", 1024, 1, 8)
	if err := reg.Open(s1); err != nil {
		t.Fatalf("open s1: %v", err)
	}
	if err := reg.Open(s2); err != nil {
		t.Fatalf("open s2: %v", err)
	}
	if err := reg.Open(s3); !errors.Is(err, ErrStreamLimitReached) {
		t.Fatalf("expected stream limit, got %v", err)
	}
	if reg.Count() != 2 {
		t.Fatalf("expected 2 streams, got %d", reg.Count())
	}
	if err := reg.Close("a", "test"); err != nil {
		t.Fatalf("close a: %v", err)
	}
	if reg.Count() != 1 {
		t.Fatalf("expected 1 stream, got %d", reg.Count())
	}
}

func TestBackpressureMeter(t *testing.T) {
	m := NewBackpressureMeter(BackpressureConfig{
		MaxInflightBytes: 100,
		MaxStreams:       4,
		ChunkMax:         64,
		CreditLowMark:    2,
		CreditRefill:     4,
	})
	if err := m.Reserve(50); err != nil {
		t.Fatalf("reserve 50: %v", err)
	}
	if err := m.Reserve(60); err == nil {
		t.Fatalf("expected reserve 60 to fail")
	}
	m.Release(50)
	if err := m.Reserve(60); err != nil {
		t.Fatalf("reserve 60 after release: %v", err)
	}
}

func TestMethodRegistry(t *testing.T) {
	r := NewMethodRegistry()
	called := false
	r.Register("test.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		called = true
		return map[string]any{"echo": true}, nil
	})
	result, err := r.Dispatch(context.Background(), "test.echo", nil)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Fatalf("handler not called")
	}
	m, _ := result.(map[string]any)
	if m["echo"] != true {
		t.Fatalf("unexpected result: %v", result)
	}
	_, err = r.Dispatch(context.Background(), "missing.method", nil)
	if err == nil {
		t.Fatalf("expected method not found")
	}
	rpcErr, ok := err.(*Error)
	if !ok || rpcErr.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected method_not_found, got %v", err)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3)
	for i := 0; i < 3; i++ {
		if !rl.Allow("k") {
			t.Fatalf("expected allow %d", i)
		}
	}
	if rl.Allow("k") {
		t.Fatalf("expected rate limit to trigger")
	}
}

func TestHandshakeHostRuntime(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	hostTransport := NewTransport(r2, w1)
	runtimeTransport := NewTransport(r1, w2)
	nonceStore := NewDefaultNonceStore()
	nonce, err := nonceStore.Issue("inst-1")
	if err != nil {
		t.Fatalf("issue nonce: %v", err)
	}
	hostHandshaker := NewHandshaker(HandshakeConfig{
		HostAPIVersion: "1.0",
		NonceStore:     nonceStore,
		Limits:         SessionLimits{MaxFrameBytes: 1024 * 1024, MaxConcurrent: 8},
	})
	runtimeHandshaker := NewHandshaker(HandshakeConfig{
		HostAPIVersion: "1.0",
	})
	hello := HelloMessage{
		ProtocolVersion: RPCVersion,
		RuntimeType:     RuntimeTypeMain,
		InstanceID:      "inst-1",
		Generation:      1,
		DefinitionHash:  "sha256:abc",
		Nonce:           nonce,
		Features:        map[string]bool{"streaming": true, "cancellation": true},
	}
	var (
		wg          sync.WaitGroup
		hostSess    *Session
		runtimeSess *Session
		hostErr     error
		runtimeErr  error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		hostSess, _, hostErr = hostHandshaker.HostHandshake(context.Background(), hostTransport, HelloMessage{})
	}()
	go func() {
		defer wg.Done()
		runtimeSess, _, runtimeErr = runtimeHandshaker.RuntimeHandshake(context.Background(), runtimeTransport, hello)
	}()
	wg.Wait()
	if hostErr != nil {
		t.Fatalf("host handshake: %v", hostErr)
	}
	if runtimeErr != nil {
		t.Fatalf("runtime handshake: %v", runtimeErr)
	}
	if hostSess == nil || runtimeSess == nil {
		t.Fatalf("sessions nil")
	}
	if hostSess.ID != runtimeSess.ID {
		t.Fatalf("session id mismatch: %s != %s", hostSess.ID, runtimeSess.ID)
	}
	if hostSess.InstanceID != "inst-1" {
		t.Fatalf("instance id mismatch: %s", hostSess.InstanceID)
	}
	if !hostSess.HasFeature("streaming") {
		t.Fatalf("streaming feature not negotiated")
	}
}

func TestHandshakeInvalidNonce(t *testing.T) {
	nonceStore := NewDefaultNonceStore()
	_, _ = nonceStore.Issue("inst-2")
	h := NewHandshaker(HandshakeConfig{
		NonceStore: nonceStore,
	})
	hello := HelloMessage{
		ProtocolVersion: RPCVersion,
		RuntimeType:     RuntimeTypeMain,
		InstanceID:      "inst-2",
		Generation:      1,
		DefinitionHash:  "sha256:abc",
		Nonce:           "wrong",
	}
	if err := h.validateHello(hello); err != nil {
		t.Fatalf("validateHello should not check nonce: %v", err)
	}
}

func TestHandshakeVersionMismatch(t *testing.T) {
	h := NewHandshaker(HandshakeConfig{})
	hello := HelloMessage{
		ProtocolVersion: "amitia-runtime-rpc/2",
		RuntimeType:     RuntimeTypeMain,
		InstanceID:      "inst-x",
		Nonce:           "n",
		DefinitionHash:  "h",
	}
	err := h.validateHello(hello)
	if err == nil {
		t.Fatalf("expected version mismatch error")
	}
}

func TestClientServerCall(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	server := NewServer(NewTransport(r2, w1), DefaultServerConfig())
	client := NewClient(NewTransport(r1, w2), DefaultClientConfig())
	server.Registry().Register("test.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Msg string `json:"msg"`
		}
		_ = json.Unmarshal(params, &p)
		return map[string]any{"echo": p.Msg}, nil
	})
	go server.Serve(context.Background())
	go client.Serve(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := client.Call(ctx, "test.echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	var resp struct {
		Echo string `json:"echo"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Echo != "hello" {
		t.Fatalf("expected hello, got %s", resp.Echo)
	}
}

func TestClientServerNotification(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	server := NewServer(NewTransport(r2, w1), DefaultServerConfig())
	client := NewClient(NewTransport(r1, w2), DefaultClientConfig())
	received := make(chan struct{})
	server.Registry().RegisterNotification("test.notify", func(ctx context.Context, params json.RawMessage) error {
		close(received)
		return nil
	})
	go server.Serve(context.Background())
	go client.Serve(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Notify(ctx, "test.notify", map[string]any{"x": 1}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatalf("notification not received")
	}
}

func TestCoreMethodsHealth(t *testing.T) {
	registry := NewMethodRegistry()
	streams := NewStreamRegistry(8)
	tracker := NewRequestTracker()
	bp := NewBackpressureMeter(DefaultBackpressureConfig())
	cancelReg := NewCancellationRegistry()
	session := &Session{
		ID:           "sess-1",
		InstanceID:   "inst-1",
		Generation:   1,
		RuntimeType:  RuntimeTypeMain,
		Features:     map[string]bool{"streaming": true},
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		LastActivity: time.Now().UTC(),
	}
	core := NewCoreMethods(session, registry, streams, tracker, bp, cancelReg)
	core.RegisterAll()
	_, ok := registry.Lookup("runtime.health")
	if !ok {
		t.Fatalf("runtime.health not registered")
	}
	result, err := registry.Dispatch(context.Background(), "runtime.health", nil)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	hr, ok := result.(*HealthResponse)
	if !ok {
		t.Fatalf("expected HealthResponse, got %T", result)
	}
	if !hr.Healthy {
		t.Fatalf("expected healthy")
	}
	if hr.InstanceID != "inst-1" {
		t.Fatalf("instance mismatch: %s", hr.InstanceID)
	}
}

func TestCoreMethodsPing(t *testing.T) {
	registry := NewMethodRegistry()
	core := NewCoreMethods(&Session{}, registry, NewStreamRegistry(4), NewRequestTracker(), NewBackpressureMeter(DefaultBackpressureConfig()), NewCancellationRegistry())
	core.RegisterAll()
	result, err := registry.Dispatch(context.Background(), "diagnostic.ping", nil)
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || !m["pong"].(bool) {
		t.Fatalf("unexpected ping result: %v", result)
	}
}

func TestCoreMethodsStreamOpen(t *testing.T) {
	registry := NewMethodRegistry()
	streams := NewStreamRegistry(8)
	core := NewCoreMethods(&Session{}, registry, streams, NewRequestTracker(), NewBackpressureMeter(DefaultBackpressureConfig()), NewCancellationRegistry())
	core.RegisterAll()
	params, _ := json.Marshal(StreamOpenRequest{
		StreamID:      "s1",
		Method:        "stream.test",
		InitialCredit: 4,
		ChunkMax:      1024,
		Direction:     "duplex",
	})
	_, err := registry.Dispatch(context.Background(), "stream.open", params)
	if err != nil {
		t.Fatalf("stream.open: %v", err)
	}
	if streams.Count() != 1 {
		t.Fatalf("expected 1 stream, got %d", streams.Count())
	}
}

func TestProtocolSchema(t *testing.T) {
	schema := BuildProtocolSchema()
	if schema.Version != RPCVersion {
		t.Fatalf("version mismatch: %s", schema.Version)
	}
	hasInvoke := false
	for _, m := range schema.Methods {
		if m.Name == "runtime.invoke" {
			hasInvoke = true
		}
	}
	if !hasInvoke {
		t.Fatalf("schema missing runtime.invoke")
	}
	if len(schema.Errors) == 0 {
		t.Fatalf("schema missing errors")
	}
}

func TestClientCallNotFound(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	server := NewServer(NewTransport(r2, w1), DefaultServerConfig())
	client := NewClient(NewTransport(r1, w2), DefaultClientConfig())
	go server.Serve(context.Background())
	go client.Serve(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := client.Call(ctx, "missing.method", nil)
	if err == nil {
		t.Fatalf("expected method not found error")
	}
	rpcErr, ok := err.(*Error)
	if !ok || rpcErr.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected method_not_found, got %v", err)
	}
}

func TestConcurrentRequests(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	defer r1.Close()
	defer w1.Close()
	defer r2.Close()
	defer w2.Close()
	server := NewServer(NewTransport(r2, w1), DefaultServerConfig())
	client := NewClient(NewTransport(r1, w2), DefaultClientConfig())
	server.Registry().Register("test.slow", func(ctx context.Context, params json.RawMessage) (any, error) {
		time.Sleep(20 * time.Millisecond)
		var p struct {
			N int `json:"n"`
		}
		_ = json.Unmarshal(params, &p)
		return map[string]any{"n": p.N * 2}, nil
	})
	go server.Serve(context.Background())
	go client.Serve(context.Background())
	const N = 8
	var wg sync.WaitGroup
	errs := make(chan error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			result, err := client.Call(ctx, "test.slow", map[string]any{"n": i})
			if err != nil {
				errs <- fmt.Errorf("call %d: %w", i, err)
				return
			}
			var resp struct {
				N int `json:"n"`
			}
			_ = json.Unmarshal(result, &resp)
			if resp.N != i*2 {
				errs <- fmt.Errorf("call %d: expected %d, got %d", i, i*2, resp.N)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
