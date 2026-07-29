package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	ErrNotConnected = errors.New("jsonrpc: not connected")
	ErrCallTimeout  = errors.New("jsonrpc: call timeout")
)

type Client struct {
	transport     *ReadWriteCloser
	tracker       *RequestTracker
	registry      *MethodRegistry
	notifications *NotificationDispatcher
	streams       *StreamRegistry
	bp            *BackpressureMeter
	session       *Session
	mu            sync.RWMutex
	closed        bool
	closeCh       chan struct{}
	logger        func(level, msg string, fields map[string]any)
}

type ClientConfig struct {
	CallTimeout      time.Duration
	MaxFrameBytes    int
	NotificationRate int
	Backpressure     BackpressureConfig
	Logger           func(level, msg string, fields map[string]any)
}

func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		CallTimeout:      30 * time.Second,
		MaxFrameBytes:    DefaultMaxFrameBytes,
		NotificationRate: 100,
		Backpressure:     DefaultBackpressureConfig(),
		Logger:           func(level, msg string, fields map[string]any) {},
	}
}

func NewClient(transport *ReadWriteCloser, cfg ClientConfig) *Client {
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = 30 * time.Second
	}
	if cfg.MaxFrameBytes > 0 {
		transport.SetMaxFrameBytes(cfg.MaxFrameBytes)
	}
	registry := NewMethodRegistry()
	limiter := NewRateLimiter(cfg.NotificationRate)
	tracker := NewRequestTracker()
	return &Client{
		transport:     transport,
		tracker:       tracker,
		registry:      registry,
		notifications: NewNotificationDispatcher(registry, limiter),
		streams:       NewStreamRegistry(cfg.Backpressure.MaxStreams),
		bp:            NewBackpressureMeter(cfg.Backpressure),
		closeCh:       make(chan struct{}),
		logger:        cfg.Logger,
	}
}

func (c *Client) SetSession(session *Session) {
	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
}

func (c *Client) Session() *Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

func (c *Client) Registry() *MethodRegistry { return c.registry }
func (c *Client) Streams() *StreamRegistry  { return c.streams }
func (c *Client) Tracker() *RequestTracker  { return c.tracker }

func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.isClosed() {
		return nil, ErrNotConnected
	}
	timeout := c.callTimeout(ctx)
	pr, id, err := c.tracker.Track(method, timeout)
	if err != nil {
		return nil, err
	}
	defer c.tracker.Cancel(id, "call completed")
	req, err := EncodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}
	if err := c.transport.Write(req); err != nil {
		c.tracker.Cancel(id, "transport error: "+err.Error())
		return nil, fmt.Errorf("jsonrpc: write request: %w", err)
	}
	resp, err := pr.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (c *Client) Notify(ctx context.Context, method string, params any) error {
	if c.isClosed() {
		return ErrNotConnected
	}
	n, err := EncodeNotification(method, params)
	if err != nil {
		return err
	}
	return c.transport.Write(n)
}

func (c *Client) callTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if ok {
		return time.Until(deadline)
	}
	return 30 * time.Second
}

func (c *Client) Serve(ctx context.Context) error {
	for {
		env, err := c.transport.Read()
		if err != nil {
			if errors.Is(err, ErrClosed) || errors.Is(err, io.EOF) {
				c.close("transport closed")
				return err
			}
			c.log("warn", "read error", map[string]any{"error": err.Error()})
			continue
		}
		switch env.Kind {
		case KindResponse, KindError:
			if err := c.tracker.Resolve(env.Response); err != nil {
				c.log("warn", "resolve failed", map[string]any{"error": err.Error()})
			}
		case KindNotification:
			if err := c.notifications.Handle(ctx, env.Notification); err != nil {
				c.log("warn", "notification handler failed", map[string]any{"method": env.Notification.Method, "error": err.Error()})
			}
		case KindRequest:
			go c.handleRequest(ctx, env.Request)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, req *Request) {
	result, err := c.registry.Dispatch(ctx, req.Method, req.Params)
	if err != nil {
		rpcErr, ok := err.(*Error)
		if !ok {
			rpcErr = InternalError(err.Error())
		}
		_ = c.transport.Write(EncodeErrorResponse(req.ID, rpcErr))
		return
	}
	resp, err := EncodeResponse(req.ID, result)
	if err != nil {
		_ = c.transport.Write(EncodeErrorResponse(req.ID, InternalError(err.Error())))
		return
	}
	_ = c.transport.Write(resp)
}

func (c *Client) Close() error {
	c.close("client closed")
	return c.transport.Close()
}

func (c *Client) close(reason string) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.closeCh)
	c.mu.Unlock()
	c.tracker.FailAll(reason)
	c.streams.CloseAll(reason)
}

func (c *Client) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

func (c *Client) Closed() <-chan struct{} { return c.closeCh }

func (c *Client) log(level, msg string, fields map[string]any) {
	if c.logger != nil {
		c.logger(level, msg, fields)
	}
}

type Server struct {
	transport     *ReadWriteCloser
	registry      *MethodRegistry
	notifications *NotificationDispatcher
	tracker       *RequestTracker
	streams       *StreamRegistry
	bp            *BackpressureMeter
	session       *Session
	mu            sync.RWMutex
	closed        bool
	closeCh       chan struct{}
	logger        func(level, msg string, fields map[string]any)
}

type ServerConfig struct {
	MaxFrameBytes    int
	NotificationRate int
	Backpressure     BackpressureConfig
	Logger           func(level, msg string, fields map[string]any)
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MaxFrameBytes:    DefaultMaxFrameBytes,
		NotificationRate: 100,
		Backpressure:     DefaultBackpressureConfig(),
		Logger:           func(level, msg string, fields map[string]any) {},
	}
}

func NewServer(transport *ReadWriteCloser, cfg ServerConfig) *Server {
	if cfg.MaxFrameBytes > 0 {
		transport.SetMaxFrameBytes(cfg.MaxFrameBytes)
	}
	registry := NewMethodRegistry()
	limiter := NewRateLimiter(cfg.NotificationRate)
	return &Server{
		transport:     transport,
		registry:      registry,
		notifications: NewNotificationDispatcher(registry, limiter),
		tracker:       NewRequestTracker(),
		streams:       NewStreamRegistry(cfg.Backpressure.MaxStreams),
		bp:            NewBackpressureMeter(cfg.Backpressure),
		closeCh:       make(chan struct{}),
		logger:        cfg.Logger,
	}
}

func (s *Server) Registry() *MethodRegistry { return s.registry }
func (s *Server) Streams() *StreamRegistry  { return s.streams }
func (s *Server) Tracker() *RequestTracker  { return s.tracker }
func (s *Server) SetSession(session *Session) {
	s.mu.Lock()
	s.session = session
	s.mu.Unlock()
}
func (s *Server) Session() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

func (s *Server) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if s.isClosed() {
		return nil, ErrNotConnected
	}
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	pr, id, err := s.tracker.Track(method, timeout)
	if err != nil {
		return nil, err
	}
	defer s.tracker.Cancel(id, "call completed")
	req, err := EncodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}
	if err := s.transport.Write(req); err != nil {
		s.tracker.Cancel(id, "transport error: "+err.Error())
		return nil, fmt.Errorf("jsonrpc: write request: %w", err)
	}
	resp, err := pr.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (s *Server) Notify(ctx context.Context, method string, params any) error {
	if s.isClosed() {
		return ErrNotConnected
	}
	n, err := EncodeNotification(method, params)
	if err != nil {
		return err
	}
	return s.transport.Write(n)
}

func (s *Server) Serve(ctx context.Context) error {
	for {
		env, err := s.transport.Read()
		if err != nil {
			if errors.Is(err, ErrClosed) || errors.Is(err, io.EOF) {
				s.close("transport closed")
				return err
			}
			s.log("warn", "read error", map[string]any{"error": err.Error()})
			continue
		}
		switch env.Kind {
		case KindResponse, KindError:
			if err := s.tracker.Resolve(env.Response); err != nil {
				s.log("warn", "resolve failed", map[string]any{"error": err.Error()})
			}
		case KindNotification:
			if err := s.notifications.Handle(ctx, env.Notification); err != nil {
				s.log("warn", "notification handler failed", map[string]any{"method": env.Notification.Method, "error": err.Error()})
			}
		case KindRequest:
			go s.handleRequest(ctx, env.Request)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req *Request) {
	result, err := s.registry.Dispatch(ctx, req.Method, req.Params)
	if err != nil {
		rpcErr, ok := err.(*Error)
		if !ok {
			rpcErr = InternalError(err.Error())
		}
		_ = s.transport.Write(EncodeErrorResponse(req.ID, rpcErr))
		return
	}
	resp, err := EncodeResponse(req.ID, result)
	if err != nil {
		_ = s.transport.Write(EncodeErrorResponse(req.ID, InternalError(err.Error())))
		return
	}
	_ = s.transport.Write(resp)
}

func (s *Server) Close() error {
	s.close("server closed")
	return s.transport.Close()
}

func (s *Server) close(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.closeCh)
	s.mu.Unlock()
	s.tracker.FailAll(reason)
	s.streams.CloseAll(reason)
}

func (s *Server) isClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *Server) Closed() <-chan struct{} { return s.closeCh }

func (s *Server) log(level, msg string, fields map[string]any) {
	if s.logger != nil {
		s.logger(level, msg, fields)
	}
}
