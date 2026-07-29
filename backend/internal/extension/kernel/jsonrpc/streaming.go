package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	StreamStateOpen    = "open"
	StreamStateClosing = "closing"
	StreamStateClosed  = "closed"
	StreamStateErrored = "errored"
)

var (
	ErrStreamClosed       = errors.New("jsonrpc: stream closed")
	ErrStreamNotFound     = errors.New("jsonrpc: stream not found")
	ErrStreamLimitReached = errors.New("jsonrpc: stream limit reached")
	ErrNoCredit           = errors.New("jsonrpc: no credit available")
)

type StreamID string

type StreamOpenRequest struct {
	StreamID      StreamID        `json:"stream_id"`
	Method        string          `json:"method"`
	Params        json.RawMessage `json:"params,omitempty"`
	InitialCredit int             `json:"initial_credit"`
	ChunkMax      int             `json:"chunk_max"`
	Direction     string          `json:"direction"`
}

type StreamChunk struct {
	StreamID StreamID `json:"stream_id"`
	Sequence int64    `json:"sequence"`
	Data     []byte   `json:"data"`
	Last     bool     `json:"last,omitempty"`
}

type StreamClose struct {
	StreamID StreamID `json:"stream_id"`
	Reason   string   `json:"reason,omitempty"`
}

type StreamError struct {
	StreamID StreamID  `json:"stream_id"`
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
}

type StreamCredit struct {
	StreamID StreamID `json:"stream_id"`
	Credit   int      `json:"credit"`
}

type Stream struct {
	ID         StreamID
	Method     string
	Direction  string
	ChunkMax   int
	Credit     int64
	Consumed   int64
	Produced   int64
	State      string
	CreatedAt  time.Time
	ClosedAt   *time.Time
	mu         sync.Mutex
	ch         chan []byte
	errCh      chan *Error
	doneCh     chan struct{}
	onChunk    func(chunk StreamChunk) error
	onClose    func(reason string)
	creditCond *sync.Cond
	cancel     context.CancelFunc
}

func NewStream(id StreamID, method, direction string, chunkMax, initialCredit int, bufferSize int) *Stream {
	s := &Stream{
		ID:        id,
		Method:    method,
		Direction: direction,
		ChunkMax:  chunkMax,
		Credit:    int64(initialCredit),
		State:     StreamStateOpen,
		CreatedAt: time.Now().UTC(),
		ch:        make(chan []byte, bufferSize),
		errCh:     make(chan *Error, 1),
		doneCh:    make(chan struct{}),
	}
	s.creditCond = sync.NewCond(&s.mu)
	return s
}

func (s *Stream) ConsumeCredit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StreamStateOpen {
		return false
	}
	if s.Credit <= 0 {
		return false
	}
	s.Credit--
	return true
}

func (s *Stream) AddCredit(amount int) {
	s.mu.Lock()
	s.Credit += int64(amount)
	s.creditCond.Broadcast()
	s.mu.Unlock()
}

func (s *Stream) WaitCredit(ctx context.Context) error {
	for {
		s.mu.Lock()
		if s.State != StreamStateOpen {
			s.mu.Unlock()
			return ErrStreamClosed
		}
		if s.Credit > 0 {
			s.Credit--
			s.mu.Unlock()
			return nil
		}
		waitCh := make(chan struct{})
		go func() {
			s.creditCond.Wait()
			close(waitCh)
		}()
		s.mu.Unlock()
		select {
		case <-waitCh:
		case <-ctx.Done():
			return ctx.Err()
		case <-s.doneCh:
			return ErrStreamClosed
		}
	}
}

func (s *Stream) SendChunk(data []byte) error {
	if len(data) > s.ChunkMax {
		return fmt.Errorf("jsonrpc: chunk size %d exceeds max %d", len(data), s.ChunkMax)
	}
	if !s.ConsumeCredit() {
		return ErrNoCredit
	}
	select {
	case s.ch <- data:
		atomic.AddInt64(&s.Produced, 1)
		return nil
	case <-s.doneCh:
		return ErrStreamClosed
	}
}

func (s *Stream) RecvChunk(ctx context.Context) ([]byte, error) {
	select {
	case data, ok := <-s.ch:
		if !ok {
			return nil, ErrStreamClosed
		}
		atomic.AddInt64(&s.Consumed, 1)
		return data, nil
	case err := <-s.errCh:
		return nil, err
	case <-s.doneCh:
		return nil, ErrStreamClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Stream) Close(reason string) {
	s.mu.Lock()
	if s.State == StreamStateClosed || s.State == StreamStateErrored {
		s.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	s.ClosedAt = &now
	s.State = StreamStateClosed
	close(s.doneCh)
	if s.onClose != nil {
		s.onClose(reason)
	}
	s.mu.Unlock()
}

func (s *Stream) Error(err *Error) {
	s.mu.Lock()
	if s.State == StreamStateClosed || s.State == StreamStateErrored {
		s.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	s.ClosedAt = &now
	s.State = StreamStateErrored
	select {
	case s.errCh <- err:
	default:
	}
	close(s.doneCh)
	if s.onClose != nil {
		s.onClose(err.Message)
	}
	s.mu.Unlock()
}

func (s *Stream) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State == StreamStateClosed || s.State == StreamStateErrored
}

type StreamRegistry struct {
	mu         sync.Mutex
	streams    map[StreamID]*Stream
	maxStreams int
	closed     bool
}

func NewStreamRegistry(maxStreams int) *StreamRegistry {
	if maxStreams <= 0 {
		maxStreams = 16
	}
	return &StreamRegistry{
		streams:    make(map[StreamID]*Stream),
		maxStreams: maxStreams,
	}
}

func (r *StreamRegistry) Open(s *Stream) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrStreamClosed
	}
	if len(r.streams) >= r.maxStreams {
		return ErrStreamLimitReached
	}
	if _, exists := r.streams[s.ID]; exists {
		return fmt.Errorf("jsonrpc: stream %s already exists", s.ID)
	}
	r.streams[s.ID] = s
	return nil
}

func (r *StreamRegistry) Get(id StreamID) (*Stream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.streams[id]
	if !ok {
		return nil, ErrStreamNotFound
	}
	return s, nil
}

func (r *StreamRegistry) Close(id StreamID, reason string) error {
	r.mu.Lock()
	s, ok := r.streams[id]
	if !ok {
		r.mu.Unlock()
		return ErrStreamNotFound
	}
	delete(r.streams, id)
	r.mu.Unlock()
	s.Close(reason)
	return nil
}

func (r *StreamRegistry) CloseAll(reason string) {
	r.mu.Lock()
	r.closed = true
	streams := r.streams
	r.streams = make(map[StreamID]*Stream)
	r.mu.Unlock()
	for _, s := range streams {
		s.Close(reason)
	}
}

func (r *StreamRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.streams)
}

func (r *StreamRegistry) ActiveStreams() []StreamID {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]StreamID, 0, len(r.streams))
	for id := range r.streams {
		ids = append(ids, id)
	}
	return ids
}

type BackpressureConfig struct {
	MaxInflightBytes int
	MaxStreams       int
	ChunkMax         int
	CreditLowMark    int
	CreditRefill     int
	CallTimeout      time.Duration
}

func DefaultBackpressureConfig() BackpressureConfig {
	return BackpressureConfig{
		MaxInflightBytes: DefaultMaxPendingBytes,
		MaxStreams:       16,
		ChunkMax:         DefaultMaxStreamChunk,
		CreditLowMark:    2,
		CreditRefill:     8,
		CallTimeout:      30 * time.Second,
	}
}

type BackpressureMeter struct {
	cfg       BackpressureConfig
	mu        sync.Mutex
	inflight  int64
	totalSent int64
	totalRecv int64
	refills   int64
}

func NewBackpressureMeter(cfg BackpressureConfig) *BackpressureMeter {
	return &BackpressureMeter{cfg: cfg}
}

func (m *BackpressureMeter) Reserve(size int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inflight+int64(size) > int64(m.cfg.MaxInflightBytes) {
		return NewError(
			ErrCodeStreamBackpressure,
			fmt.Sprintf("inflight bytes %d + %d exceeds limit %d", m.inflight, size, m.cfg.MaxInflightBytes),
			true,
			CategoryStream,
		)
	}
	m.inflight += int64(size)
	return nil
}

func (m *BackpressureMeter) Release(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inflight -= int64(size)
	if m.inflight < 0 {
		m.inflight = 0
	}
}

func (m *BackpressureMeter) RecordSent(bytes int) { atomic.AddInt64(&m.totalSent, int64(bytes)) }
func (m *BackpressureMeter) RecordRecv(bytes int) { atomic.AddInt64(&m.totalRecv, int64(bytes)) }
func (m *BackpressureMeter) RecordRefill()        { atomic.AddInt64(&m.refills, 1) }

func (m *BackpressureMeter) Stats() (inflight, sent, recv, refills int64) {
	return atomic.LoadInt64(&m.inflight), atomic.LoadInt64(&m.totalSent),
		atomic.LoadInt64(&m.totalRecv), atomic.LoadInt64(&m.refills)
}

func (m *BackpressureMeter) ShouldRefill(currentCredit int) bool {
	return currentCredit <= m.cfg.CreditLowMark
}

func (m *BackpressureMeter) RefillAmount() int {
	return m.cfg.CreditRefill
}

func (m *BackpressureMeter) Config() BackpressureConfig { return m.cfg }
