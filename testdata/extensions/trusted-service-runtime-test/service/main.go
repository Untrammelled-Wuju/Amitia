package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	protocolVersion = "amitia_jsonrpc_v1"
	jsonrpcVersion  = "2.0"
	frameHeaderSize = 8
	maxFrameBytes   = 16 * 1024 * 1024
	childSleeperArg = "--amitia-child-sleeper"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type helloPayload struct {
	ProtocolVersion string            `json:"protocol_version"`
	RuntimeType     string            `json:"runtime_type"`
	InstanceID      string            `json:"instance_id"`
	Generation      int64             `json:"generation"`
	DefinitionHash  string            `json:"definition_hash"`
	Nonce           string            `json:"nonce"`
	Features       map[string]bool    `json:"features"`
	SDKVersion     string            `json:"sdk_version"`
	Metadata       map[string]string `json:"metadata"`
}

type welcomePayload struct {
	ProtocolVersion string         `json:"protocol_version"`
	SessionID      string         `json:"session_id"`
	SessionToken   string         `json:"session_token"`
	HostAPIVersion string         `json:"host_api_version"`
	Features       map[string]bool `json:"features"`
	InstanceID     string          `json:"instance_id"`
	Generation     int64           `json:"generation"`
}

type invokeParams struct {
	Capability string          `json:"capability"`
	Input      json.RawMessage `json:"input,omitempty"`
}

type shutdownParams struct {
	Reason  string `json:"reason"`
	GraceMS int64  `json:"grace_ms"`
}

type service struct {
	in           io.Reader
	out          io.Writer
	writeMu      sync.Mutex
	instanceID   string
	nonce        string
	generation   int64
	tempDir      string
	secretLease  string
	sessionID    string
	sessionToken string
	hostFeatures map[string]bool
	startedAt    time.Time
	pendingMu    sync.Mutex
	pending      map[string]chan *rpcMessage
	nextID       int64
	listenersMu  sync.Mutex
	listeners    []io.Closer
	logMu        sync.Mutex
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == childSleeperArg {
		runChildSleeper()
		return
	}
	s := &service{
		in:         os.Stdin,
		out:        os.Stdout,
		pending:    make(map[string]chan *rpcMessage),
		startedAt:  time.Now().UTC(),
		shutdownCh: make(chan struct{}),
	}
	s.loadEnv()
	if err := s.run(); err != nil {
		s.logf("fatal: %v", err)
		os.Exit(1)
	}
}

func runChildSleeper() {
	sleepMS := envInt("AMITIA_CHILD_SLEEP_MS", 60000)
	notifyParent := os.Getenv("AMITIA_CHILD_NOTIFY")
	if notifyParent != "" {
		f, err := os.OpenFile(notifyParent, os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			_, _ = f.WriteString(fmt.Sprintf("child:%d:started\n", os.Getpid()))
			_ = f.Close()
		}
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	timer := time.NewTimer(time.Duration(sleepMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-sigCh:
	}
	os.Exit(0)
}

func (s *service) loadEnv() {
	s.nonce = envOr("AMITIA_SESSION_NONCE", os.Getenv("AMITIA_SESSION"))
	s.instanceID = envOr("AMITIA_RUNTIME_INSTANCE_ID", os.Getenv("AMITIA_INSTANCE"))
	s.generation = envInt64("AMITIA_RUNTIME_GENERATION", envInt64("AMITIA_GENERATION", 1))
	s.tempDir = envOr("AMITIA_TEMP_DIR", os.TempDir())
	s.secretLease = os.Getenv("AMITIA_SECRET_LEASE")
	if s.instanceID == "" {
		s.instanceID = "tsrt-" + randHex(8)
	}
	if s.nonce == "" {
		s.nonce = randHex(16)
	}
}

func (s *service) run() error {
	if err := s.sendHello(); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}
	if err := s.awaitWelcome(); err != nil {
		return fmt.Errorf("await welcome: %w", err)
	}
	initID, err := s.sendInitialize()
	if err != nil {
		return fmt.Errorf("send initialize: %w", err)
	}
	if err := s.awaitInitResponse(initID); err != nil {
		return fmt.Errorf("await initialize response: %w", err)
	}
	if err := s.sendReady(); err != nil {
		return fmt.Errorf("send ready: %w", err)
	}
	s.logf("service ready: instance=%s generation=%d", s.instanceID, s.generation)
	msgCh := make(chan *rpcMessage, 32)
	errCh := make(chan error, 1)
	go s.readLoop(msgCh, errCh)
	go s.handleLoop(msgCh)
	select {
	case <-s.shutdownCh:
		return nil
	case err := <-errCh:
		if err != nil && err != io.EOF {
			return err
		}
		return nil
	}
}

func (s *service) readLoop(msgCh chan<- *rpcMessage, errCh chan<- error) {
	for {
		msg, err := s.readMessage()
		if err != nil {
			errCh <- err
			close(msgCh)
			return
		}
		select {
		case msgCh <- msg:
		case <-s.shutdownCh:
			return
		}
	}
}

func (s *service) handleLoop(msgCh <-chan *rpcMessage) {
	for msg := range msgCh {
		s.handleMessage(msg)
	}
}

func (s *service) handleMessage(msg *rpcMessage) {
	if msg.Method != "" && len(msg.ID) > 0 {
		s.handleRequest(msg)
		return
	}
	if msg.Method != "" {
		s.logf("notification: %s", msg.Method)
		return
	}
	if len(msg.ID) > 0 {
		s.deliverResponse(msg)
	}
}

func (s *service) handleRequest(msg *rpcMessage) {
	switch msg.Method {
	case "service.invoke":
		s.handleInvoke(msg)
	case "service.shutdown":
		s.handleShutdown(msg)
	case "service.health":
		s.respond(msg.ID, s.healthResult(), nil)
	default:
		s.respond(msg.ID, nil, newError("method_not_found", "method not found: "+msg.Method))
	}
}

func (s *service) handleInvoke(msg *rpcMessage) {
	var p invokeParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		s.respond(msg.ID, nil, newError("invalid_params", err.Error()))
		return
	}
	result, err := s.dispatchCapability(p.Capability, p.Input)
	if err != nil {
		s.respond(msg.ID, nil, err)
		return
	}
	s.respond(msg.ID, result, nil)
}

func (s *service) dispatchCapability(capability string, input json.RawMessage) (any, *rpcError) {
	switch capability {
	case "health":
		return s.healthResult(), nil
	case "echo":
		var in struct {
			Message any `json:"message"`
		}
		_ = json.Unmarshal(input, &in)
		return map[string]any{"echo": in.Message}, nil
	case "sleep":
		var in struct {
			DurationMS int64 `json:"duration_ms"`
		}
		_ = json.Unmarshal(input, &in)
		if in.DurationMS <= 0 {
			in.DurationMS = 100
		}
		select {
		case <-time.After(time.Duration(in.DurationMS) * time.Millisecond):
		case <-s.shutdownCh:
			return nil, newError("cancelled", "sleep cancelled by shutdown")
		}
		return map[string]any{"slept_ms": in.DurationMS}, nil
	case "crash":
		s.logf("crash requested, exiting with non-zero status")
		go func() {
			time.Sleep(10 * time.Millisecond)
			os.Exit(1)
		}()
		return map[string]any{"crashing": true, "exit_code": 1}, nil
	case "spawn_child":
		return s.spawnChild(input)
	case "write_temp":
		return s.writeTemp(input)
	case "listen_loopback":
		return s.listenLoopback(input)
	case "request_secret":
		return s.requestSecret(input)
	case "resource_usage":
		return s.resourceUsage(), nil
	case "graceful_shutdown":
		s.triggerShutdown("graceful_shutdown capability")
		return map[string]any{"shutting_down": true}, nil
	default:
		return nil, newError("method_not_found", "unknown capability: "+capability)
	}
}

func (s *service) spawnChild(input json.RawMessage) (any, *rpcError) {
	var in struct {
		Count   int    `json:"count"`
		SleepMS int64  `json:"sleep_ms"`
		Args    string `json:"args"`
	}
	_ = json.Unmarshal(input, &in)
	if in.Count <= 0 {
		in.Count = 1
	}
	if in.SleepMS <= 0 {
		in.SleepMS = 60000
	}
	exePath, err := os.Executable()
	if err != nil {
		return nil, newError("internal", "resolve executable: "+err.Error())
	}
	pids := make([]int, 0, in.Count)
	for i := 0; i < in.Count; i++ {
		cmd := exec.Command(exePath, childSleeperArg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("AMITIA_CHILD_SLEEP_MS=%d", in.SleepMS))
		if err := cmd.Start(); err != nil {
			return nil, newError("internal", "spawn child: "+err.Error())
		}
		pids = append(pids, cmd.Process.Pid)
		go func(c *exec.Cmd) {
			_ = c.Wait()
		}(cmd)
	}
	return map[string]any{"child_pids": pids, "count": len(pids)}, nil
}

func (s *service) writeTemp(input json.RawMessage) (any, *rpcError) {
	var in struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(input, &in)
	if in.Name == "" {
		in.Name = "amitia-test-" + randHex(8) + ".tmp"
	}
	dir := s.tempDir
	if dir == "" {
		dir = os.TempDir()
	}
	path := dir + string(os.PathSeparator) + in.Name
	if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
		return nil, newError("internal", "write temp file: "+err.Error())
	}
	return map[string]any{"path": path, "bytes_written": len(in.Content)}, nil
}

func (s *service) listenLoopback(input json.RawMessage) (any, *rpcError) {
	var in struct {
		Port int `json:"port"`
	}
	_ = json.Unmarshal(input, &in)
	addr := fmt.Sprintf("127.0.0.1:%d", in.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, newError("internal", "listen loopback: "+err.Error())
	}
	s.listenersMu.Lock()
	s.listeners = append(s.listeners, ln)
	s.listenersMu.Unlock()
	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}(ln)
	tcpAddr := ln.Addr().(*net.TCPAddr)
	return map[string]any{"address": "127.0.0.1", "port": tcpAddr.Port}, nil
}

func (s *service) requestSecret(input json.RawMessage) (any, *rpcError) {
	var in struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(input, &in)
	if in.Name == "" {
		in.Name = "default"
	}
	params := map[string]any{"name": in.Name, "reason": in.Reason}
	resp, err := s.callHost("host.request_secret", params, 5*time.Second)
	if err != nil {
		return nil, newError("internal", "request secret: "+err.Error())
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	var out any
	if len(resp.Result) > 0 {
		_ = json.Unmarshal(resp.Result, &out)
	}
	return map[string]any{"secret": out, "lease": s.secretLease}, nil
}

func (s *service) resourceUsage() any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"pid":              os.Getpid(),
		"goroutines":       runtime.NumGoroutine(),
		"cpu_count":        runtime.NumCPU(),
		"alloc_bytes":      m.Alloc,
		"sys_bytes":        m.Sys,
		"heap_objects":     m.HeapObjects,
		"uptime_seconds":   time.Since(s.startedAt).Seconds(),
		"instance_id":      s.instanceID,
	}
}

func (s *service) healthResult() any {
	return map[string]any{
		"healthy":        true,
		"instance_id":    s.instanceID,
		"generation":    s.generation,
		"session_id":    s.sessionID,
		"uptime_seconds": time.Since(s.startedAt).Seconds(),
		"now":            time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (s *service) handleShutdown(msg *rpcMessage) {
	var p shutdownParams
	_ = json.Unmarshal(msg.Params, &p)
	if p.GraceMS <= 0 {
		p.GraceMS = 5000
	}
	s.respond(msg.ID, map[string]any{"accepted": true, "grace_ms": p.GraceMS}, nil)
	s.triggerShutdown("service.shutdown: " + p.Reason)
}

func (s *service) triggerShutdown(reason string) {
	s.shutdownOnce.Do(func() {
		s.logf("shutdown triggered: %s", reason)
		s.closeListeners()
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(s.shutdownCh)
			os.Exit(0)
		}()
	})
}

func (s *service) closeListeners() {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for _, l := range s.listeners {
		_ = l.Close()
	}
	s.listeners = nil
}

func (s *service) sendHello() error {
	payload := helloPayload{
		ProtocolVersion: protocolVersion,
		RuntimeType:     "service",
		InstanceID:      s.instanceID,
		Generation:      s.generation,
		DefinitionHash:  "sha256:trusted-service-runtime-test",
		Nonce:           s.nonce,
		Features: map[string]bool{
			"invoke":       true,
			"health":       true,
			"shutdown":     true,
			"cancellation": true,
			"diagnostics":  true,
		},
		SDKVersion: "amitia-test/1.0.0",
		Metadata: map[string]string{
			"service_id": "trusted-service-runtime-test",
			"build":      "test",
		},
	}
	return s.sendNotification("runtime.hello", payload)
}

func (s *service) awaitWelcome() error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			return err
		}
		if msg.Method == "host.welcome" {
			var w welcomePayload
			_ = json.Unmarshal(msg.Params, &w)
			s.sessionID = w.SessionID
			s.sessionToken = w.SessionToken
			s.hostFeatures = w.Features
			s.logf("received host.welcome: session=%s", s.sessionID)
			return nil
		}
		s.logf("ignoring pre-welcome message: %s", msg.Method)
	}
}

func (s *service) sendInitialize() (string, error) {
	params := map[string]any{
		"service_id":   "trusted-service-runtime-test",
		"instance_id":  s.instanceID,
		"generation":   s.generation,
		"capabilities": []string{
			"health", "echo", "sleep", "crash", "spawn_child",
			"write_temp", "listen_loopback", "request_secret",
			"resource_usage", "graceful_shutdown",
		},
		"protocol_version": protocolVersion,
	}
	id, _, err := s.sendRequest("service.initialize", params)
	return id, err
}

func (s *service) awaitInitResponse(initID string) error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			return err
		}
		if len(msg.ID) > 0 && msg.Method == "" {
			var idStr string
			if err := json.Unmarshal(msg.ID, &idStr); err == nil && idStr == initID {
				if msg.Error != nil {
					return fmt.Errorf("initialize rejected: %s", msg.Error.Message)
				}
				s.logf("service.initialize accepted")
				return nil
			}
			s.deliverResponse(msg)
			continue
		}
		s.logf("ignoring pre-ready message: %s", msg.Method)
	}
}

func (s *service) sendReady() error {
	return s.sendNotification("runtime.ready", map[string]any{
		"ready":        true,
		"instance_id":  s.instanceID,
		"session_id":   s.sessionID,
		"started_at":   s.startedAt.Format(time.RFC3339Nano),
	})
}

func (s *service) callHost(method string, params any, timeout time.Duration) (*rpcMessage, error) {
	id, ch, err := s.sendRequest(method, params)
	if err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("host call timeout: %s", method)
	case <-s.shutdownCh:
		return nil, fmt.Errorf("shutdown during host call: %s", method)
	}
}

func (s *service) readMessage() (*rpcMessage, error) {
	var header [frameHeaderSize]byte
	if _, err := io.ReadFull(s.in, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint64(header[:])
	if size == 0 {
		return nil, fmt.Errorf("empty frame")
	}
	if size > maxFrameBytes {
		return nil, fmt.Errorf("frame too large: %d", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(s.in, payload); err != nil {
		return nil, err
	}
	var msg rpcMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	if msg.JSONRPC != jsonrpcVersion {
		return nil, fmt.Errorf("unsupported jsonrpc version: %s", msg.JSONRPC)
	}
	return &msg, nil
}

func (s *service) writeMessage(msg *rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var header [frameHeaderSize]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(data)))
	if _, err := s.out.Write(header[:]); err != nil {
		return err
	}
	_, err = s.out.Write(data)
	return err
}

func (s *service) sendNotification(method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = b
	}
	return s.writeMessage(&rpcMessage{JSONRPC: jsonrpcVersion, Method: method, Params: raw})
}

func (s *service) sendRequest(method string, params any) (string, chan *rpcMessage, error) {
	id := s.nextRequestID()
	ch := make(chan *rpcMessage, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			s.pendingMu.Lock()
			delete(s.pending, id)
			s.pendingMu.Unlock()
			return "", nil, err
		}
		raw = b
	}
	idRaw, _ := json.Marshal(id)
	if err := s.writeMessage(&rpcMessage{JSONRPC: jsonrpcVersion, ID: idRaw, Method: method, Params: raw}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return "", nil, err
	}
	return id, ch, nil
}

func (s *service) nextRequestID() string {
	s.pendingMu.Lock()
	id := s.nextID
	s.nextID++
	s.pendingMu.Unlock()
	return strconv.FormatInt(id, 10)
}

func (s *service) respond(id json.RawMessage, result any, err *rpcError) {
	var raw json.RawMessage
	if result != nil {
		b, merr := json.Marshal(result)
		if merr != nil {
			err = newError("internal", "marshal result: "+merr.Error())
			raw = nil
		} else {
			raw = b
		}
	}
	if mErr := s.writeMessage(&rpcMessage{JSONRPC: jsonrpcVersion, ID: id, Result: raw, Error: err}); mErr != nil {
		s.logf("write response failed: %v", mErr)
	}
}

func (s *service) deliverResponse(msg *rpcMessage) {
	var idStr string
	if err := json.Unmarshal(msg.ID, &idStr); err != nil {
		return
	}
	s.pendingMu.Lock()
	ch, ok := s.pending[idStr]
	if ok {
		delete(s.pending, idStr)
	}
	s.pendingMu.Unlock()
	if ok {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *service) logf(format string, args ...any) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	fmt.Fprintf(os.Stderr, "[tsrt] "+format+"\n", args...)
}

func newError(code, message string) *rpcError {
	return &rpcError{Code: code, Message: message}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
