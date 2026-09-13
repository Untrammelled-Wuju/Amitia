package conformance

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

const HelloMethod = "control.handshake.hello"

const (
	MethodSecretAcquire = "secret.acquire"
	MethodSecretRelease = "secret.release"
	MethodSecretQuery   = "secret.query"
	MethodSinkRegister  = "control.sink.register"
	MethodEffectSubmit  = "control.effect.submit"
	MethodAuthorityMode = "control.authority.mode"
	MethodEmergency     = "control.emergency.status"
)

type leaseRecord struct {
	leaseID string
	ref     string
	status  string
	granted bool
}

type PseudoHost struct {
	pluginPath string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	mu         sync.Mutex

	rpcSeq     atomic.Int64
	pending    map[string]chan rpcResult
	pendingMu  sync.Mutex
	generation atomic.Int64
	startTime  time.Time
	stderr     *bufio.Reader

	counter  atomic.Int64
	leasesMu sync.Mutex
	leases   map[string]leaseRecord
	sinksMu  sync.Mutex
	sinks    map[string]bool
}

type rpcResult struct {
	envelope protocol.Envelope
	err      error
}

type launchConfig struct {
	pluginPath    string
	env           []string
	binaryInflate bool
}

func NewPseudoHost(cfg launchConfig) *PseudoHost {
	return &PseudoHost{
		pluginPath: cfg.pluginPath,
		pending:    make(map[string]chan rpcResult),
		leases:     make(map[string]leaseRecord),
		sinks:      make(map[string]bool),
	}
}

func (h *PseudoHost) Start(ctx context.Context) error {
	h.cmd = exec.CommandContext(ctx, h.hydratePath())
	stdin, err := h.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := h.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := h.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	h.stdin = stdin
	h.stdout = bufio.NewReader(stdout)
	h.stderr = bufio.NewReader(stderr)

	if err := h.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	h.startTime = time.Now()
	h.generation.Add(1)
	fmt.Fprintf(os.Stderr, "[PH] STARTED pid=%d bin=%s\n", h.cmd.Process.Pid, h.hydratePath())

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[PH] readLoop panic: %v\n", r)
			}
		}()
		h.readLoop(ctx)
		fmt.Fprintf(os.Stderr, "[PH] readLoop exited pid=%d\n", h.cmd.Process.Pid)
	}()
	return nil
}

func (h *PseudoHost) hydratePath() string {
	return h.pluginPath
}

func (h *PseudoHost) WaitExit() error {
	if h.cmd == nil {
		return nil
	}
	return h.cmd.Wait()
}

func (h *PseudoHost) ExitCode() int {
	if h.cmd == nil || h.cmd.ProcessState == nil {
		return -1
	}
	return h.cmd.ProcessState.ExitCode()
}

func (h *PseudoHost) Generation() int64 {
	return h.generation.Load()
}

func (h *PseudoHost) Restart(ctx context.Context) error {
	_ = h.Kill()
	h.generation.Add(1)
	return h.Start(ctx)
}

func (h *PseudoHost) Kill() error {
	if h.cmd == nil {
		return nil
	}
	_ = h.cmd.Process.Kill()
	return h.cmd.Wait()
}

func (h *PseudoHost) writeFrame(env protocol.Envelope) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := binary.Write(h.stdin, binary.BigEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("write len: %w", err)
	}
	if _, err := h.stdin.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func (h *PseudoHost) readFrame() (*protocol.Envelope, error) {
	var length uint32
	if err := binary.Read(h.stdout, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("read len: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[PH] READ_HEADER len=%d\n", length)
	buf := make([]byte, length)
	if _, err := io.ReadFull(h.stdout, buf); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[PH] READ_PAYLOAD %d bytes\n", len(buf))
	var env protocol.Envelope
	if err := json.Unmarshal(buf, &env); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &env, nil
}

func (h *PseudoHost) CallRPC(ctx context.Context, service string, method string, payload json.RawMessage) (*protocol.Envelope, error) {
	seq := h.rpcSeq.Add(1)
	reqID := fmt.Sprintf("rpc-%d", seq)

	h.pendingMu.Lock()
	resultCh := make(chan rpcResult, 1)
	h.pending[reqID] = resultCh
	h.pendingMu.Unlock()

	env := protocol.Envelope{
		Protocol:  "amitia-game-host/1",
		Type:      "request",
		ID:        reqID,
		ServiceID: service,
		Method:    method,
		Payload:   payload,
	}

	if err := h.writeFrame(env); err != nil {
		h.pendingMu.Lock()
		delete(h.pending, reqID)
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("write: %w", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		return &result.envelope, nil
	case <-ctx.Done():
		h.pendingMu.Lock()
		delete(h.pending, reqID)
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("rpc timeout: %w", ctx.Err())
	}
}

func (h *PseudoHost) SendNotification(ctx context.Context, service string, name string, payload json.RawMessage) error {
	env := protocol.Envelope{
		Protocol:  "amitia-game-host/1",
		Type:      "notification",
		ServiceID: service,
		Method:    name,
		Payload:   payload,
	}
	return h.writeFrame(env)
}

func (h *PseudoHost) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		env, err := h.readFrame()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return
			}
			continue
		}

		switch env.Type {
		case "response":
			h.pendingMu.Lock()
			if ch, ok := h.pending[env.RequestID]; ok {
				ch <- rpcResult{envelope: *env}
				delete(h.pending, env.RequestID)
			}
			h.pendingMu.Unlock()
		case "request":
			h.handlePluginRequest(*env)
		case "notification":
			h.handleNotification(*env)
		}
	}
}

func (h *PseudoHost) nextLeaseID() string {
	n := h.counter.Add(1)
	return fmt.Sprintf("lease-stub-%d", n)
}

func (h *PseudoHost) handlePluginRequest(env protocol.Envelope) {
	switch env.Method {
	case HelloMethod:
		h.respondToHello(env)
		return
	case MethodSecretAcquire:
		h.respondToAcquireSecret(env)
		return
	case MethodSecretRelease:
		h.respondToReleaseSecret(env)
		return
	case MethodSecretQuery:
		h.respondToQuerySecret(env)
		return
	case MethodSinkRegister, MethodEffectSubmit:
		h.respondOk(env, `{"registered":true,"stub":true}`)
		return
	case MethodAuthorityMode, MethodEmergency:
		h.respondOk(env, `{"mode":"stub","state":"stub"}`)
		return
	}
	h.respondOk(env, `{"stub":true}`)
}

func (h *PseudoHost) respondToHello(helloEnv protocol.Envelope) {
	resp := protocol.Envelope{
		Protocol:  "amitia-game-host/1",
		Type:      "response",
		RequestID: helloEnv.ID,
		Payload:   json.RawMessage(`{"protocol":"amitia-game-host/1","capabilities":["realtime_control","state_streaming","event_streaming","custom_rpc","host_api","shared_control","multi_service"],"rpcNamespaces":["mock.core","mock.state","mock.data","mock.control","mock.security","mock.fault"]}`),
	}
	_ = h.writeFrame(resp)
}

func (h *PseudoHost) respondToAcquireSecret(env protocol.Envelope) {
	var payload struct {
		Ref       string `json:"ref"`
		Purpose   string `json:"purpose"`
		ServiceID string `json:"serviceId"`
	}
	_ = json.Unmarshal(env.Payload, &payload)

	h.leasesMu.Lock()
	r := leaseRecord{
		leaseID: h.nextLeaseID(),
		ref:     payload.Ref,
		status:  "granted",
		granted: true,
	}
	h.leases[payload.Ref] = r
	if payload.ServiceID != "" {
		h.leases[payload.ServiceID] = r
	}
	h.leasesMu.Unlock()

	h.respondOk(env, mustJSON(map[string]any{
		"leaseId": r.leaseID,
		"ref":     payload.Ref,
		"status":  "granted",
		"granted": true,
	}))
}

func (h *PseudoHost) respondToReleaseSecret(env protocol.Envelope) {
	var payload struct {
		LeaseID string `json:"leaseId"`
		Ref     string `json:"ref"`
	}
	_ = json.Unmarshal(env.Payload, &payload)

	h.leasesMu.Lock()
	if payload.Ref != "" {
		delete(h.leases, payload.Ref)
	}
	h.leasesMu.Unlock()

	h.respondOk(env, `{"released":true}`)
}

func (h *PseudoHost) respondToQuerySecret(env protocol.Envelope) {
	var payload struct {
		LeaseID   string `json:"leaseId"`
		Ref       string `json:"ref"`
		ServiceID string `json:"serviceId"`
	}
	_ = json.Unmarshal(env.Payload, &payload)

	h.leasesMu.Lock()
	var r leaseRecord
	var ok bool
	if payload.Ref != "" {
		r, ok = h.leases[payload.Ref]
	} else if payload.ServiceID != "" {
		r, ok = h.leases[payload.ServiceID]
	} else if payload.LeaseID != "" {
		for _, v := range h.leases {
			if v.leaseID == payload.LeaseID {
				r = v
				ok = true
				break
			}
		}
	}
	h.leasesMu.Unlock()

	if !ok {
		r = leaseRecord{leaseID: payload.LeaseID, ref: payload.Ref, status: "revoked", granted: false}
	}
	h.respondOk(env, mustJSON(map[string]any{
		"leaseId": r.leaseID,
		"ref":     r.ref,
		"status":  r.status,
		"granted": r.granted,
		"valid":   r.granted,
	}))
}

func (h *PseudoHost) respondOk(env protocol.Envelope, payload string) {
	resp := protocol.Envelope{
		Protocol:  "amitia-game-host/1",
		Type:      "response",
		RequestID: env.ID,
		Payload:   json.RawMessage(payload),
	}
	_ = h.writeFrame(resp)
}

func (h *PseudoHost) handleNotification(env protocol.Envelope) {
	_ = env
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
