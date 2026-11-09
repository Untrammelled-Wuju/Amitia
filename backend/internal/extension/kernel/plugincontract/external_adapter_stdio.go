package plugincontract

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var ErrExternalAdapterTransportClosed = errors.New("plugincontract: external adapter transport closed")

type externalAdapterRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type externalAdapterRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type externalAdapterPending struct {
	result chan json.RawMessage
	err    chan error
}

type StdioAdapterProviderTransport struct {
	reader  *bufio.Scanner
	writer  io.WriteCloser
	closeFn func() error

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  atomic.Uint64
	pending map[uint64]externalAdapterPending
	closed  bool
	done    chan struct{}
}

func NewStdioAdapterProviderTransport(reader io.ReadCloser, writer io.WriteCloser) *StdioAdapterProviderTransport {
	transport := &StdioAdapterProviderTransport{
		reader:  bufio.NewScanner(reader),
		writer:  writer,
		pending: make(map[uint64]externalAdapterPending),
		done:    make(chan struct{}),
	}
	transport.reader.Buffer(make([]byte, 64*1024), 8*1024*1024)
	transport.closeFn = func() error {
		return errors.Join(reader.Close(), writer.Close())
	}
	go transport.readLoop()
	return transport
}

func (t *StdioAdapterProviderTransport) Call(ctx context.Context, method string, params any, result any) error {
	if t == nil {
		return ErrExternalAdapterTransportClosed
	}
	select {
	case <-t.done:
		return ErrExternalAdapterTransportClosed
	default:
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("plugincontract: encode adapter request: %w", err)
	}
	id := t.nextID.Add(1)
	pending := externalAdapterPending{
		result: make(chan json.RawMessage, 1),
		err:    make(chan error, 1),
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrExternalAdapterTransportClosed
	}
	t.pending[id] = pending
	t.mu.Unlock()
	request := externalAdapterRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.removePending(id)
		return err
	}
	t.writeMu.Lock()
	_, writeErr := t.writer.Write(append(data, '\n'))
	t.writeMu.Unlock()
	if writeErr != nil {
		t.removePending(id)
		return writeErr
	}
	select {
	case <-ctx.Done():
		t.removePending(id)
		return ctx.Err()
	case raw := <-pending.result:
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("plugincontract: decode adapter response: %w", err)
		}
		return nil
	case err := <-pending.err:
		return err
	}
}

func (t *StdioAdapterProviderTransport) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.done)
	pending := t.pending
	t.pending = make(map[uint64]externalAdapterPending)
	t.mu.Unlock()
	for _, call := range pending {
		call.err <- ErrExternalAdapterTransportClosed
	}
	if t.closeFn != nil {
		return t.closeFn()
	}
	return nil
}

func (t *StdioAdapterProviderTransport) readLoop() {
	for t.reader.Scan() {
		var response externalAdapterRPCResponse
		if err := json.Unmarshal(t.reader.Bytes(), &response); err != nil {
			continue
		}
		if response.ID == 0 {
			continue
		}
		t.mu.Lock()
		pending, ok := t.pending[response.ID]
		if ok {
			delete(t.pending, response.ID)
		}
		t.mu.Unlock()
		if !ok {
			continue
		}
		if response.Error != nil {
			pending.err <- fmt.Errorf("plugincontract: adapter rpc %d: %s", response.Error.Code, response.Error.Message)
			continue
		}
		pending.result <- response.Result
	}
	err := t.reader.Err()
	if err == nil {
		err = io.EOF
	}
	t.failPending(err)
}

func (t *StdioAdapterProviderTransport) failPending(cause error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.done)
	pending := t.pending
	t.pending = make(map[uint64]externalAdapterPending)
	t.mu.Unlock()
	err := fmt.Errorf("%w: %v", ErrExternalAdapterTransportClosed, cause)
	for _, call := range pending {
		call.err <- err
	}
}

func (t *StdioAdapterProviderTransport) removePending(id uint64) {
	t.mu.Lock()
	delete(t.pending, id)
	t.mu.Unlock()
}
