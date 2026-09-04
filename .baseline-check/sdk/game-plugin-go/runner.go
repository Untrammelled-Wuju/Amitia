package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

const HelloMethod = "control.handshake.hello"

const DefaultWorkerPoolSize = 8

type SDKInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type HelloConfiguration struct {
	SupportedProtocols []string
	Capabilities       []string
	RPCNamespaces      []string
	Channels           []ChannelHelloDescriptor
	Sinks              []SinkHelloDescriptor
	SDK                *SDKInfo
	Metadata           map[string]json.RawMessage
}

type ChannelHelloDescriptor struct {
	ID string `json:"id"`
}

type SinkHelloDescriptor struct {
	SinkID    string `json:"sinkId"`
	Kind      string `json:"kind"`
	ServiceID string `json:"serviceId,omitempty"`
}

type RunnerConfig struct {
	PluginID         string
	DefaultServiceID string
	Hello            HelloConfiguration
	OnReady          func(ctx context.Context, client *Client) func(ctx context.Context)
	WorkerPoolSize   int
}

type ServiceRegistry struct {
	ServiceID string
	Registry  *HandlerRegistry
}

type handlerTask struct {
	ctx      context.Context
	client   *Client
	envelope protocol.Envelope
	registry *HandlerRegistry
}

type Runner struct {
	client     *Client
	config     RunnerConfig
	services   map[string]*HandlerRegistry
	running    bool
	workerSize int
	workerSem  chan struct{}
	workerWg   sync.WaitGroup
}

func NewRunner(client *Client, config RunnerConfig) *Runner {
	workerSize := config.WorkerPoolSize
	if workerSize <= 0 {
		workerSize = DefaultWorkerPoolSize
	}
	return &Runner{
		client:     client,
		config:     config,
		services:   make(map[string]*HandlerRegistry),
		workerSize: workerSize,
		workerSem:  make(chan struct{}, workerSize),
	}
}

func (r *Runner) AddService(serviceID string, registry *HandlerRegistry) {
	r.services[serviceID] = registry
}

func (r *Runner) findRegistryForRequest(request protocol.Envelope) *HandlerRegistry {
	if request.ServiceID != "" {
		if reg, ok := r.services[request.ServiceID]; ok {
			return reg
		}
	}
	return nil
}

func (r *Runner) performHandshake(ctx context.Context) (*HelloResponse, error) {
	helloReq := map[string]any{
		"supportedProtocols": r.config.Hello.SupportedProtocols,
		"capabilities":       r.config.Hello.Capabilities,
		"rpcNamespaces":      r.config.Hello.RPCNamespaces,
	}

	if len(r.config.Hello.Channels) > 0 {
		helloReq["channels"] = r.config.Hello.Channels
	}
	if len(r.config.Hello.Sinks) > 0 {
		helloReq["sinks"] = r.config.Hello.Sinks
	}

	if r.config.Hello.SDK != nil {
		helloReq["sdk"] = r.config.Hello.SDK
	}
	if r.config.Hello.Metadata != nil {
		helloReq["metadata"] = r.config.Hello.Metadata
	}

	envelope, err := r.client.NewRequest(HelloMethod, helloReq)
	if err != nil {
		return nil, fmt.Errorf("build hello request failed: %w", err)
	}

	if err := r.client.Transport().Send(ctx, envelope); err != nil {
		return nil, fmt.Errorf("send hello request failed: %w", err)
	}

	respEnvelope, err := r.client.Transport().Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("receive hello response failed: %w", err)
	}

	if respEnvelope.Error != nil {
		return nil, fmt.Errorf("handshake failed: %s - %s", respEnvelope.Error.Code, respEnvelope.Error.Message)
	}
	if respEnvelope.Type == protocol.MessageTypeError {
		return nil, fmt.Errorf("handshake failed with unknown error")
	}

	if respEnvelope.Type != protocol.MessageTypeResponse {
		return nil, fmt.Errorf("unexpected handshake response type: %s", respEnvelope.Type)
	}
	if respEnvelope.Protocol != protocol.ProtocolVersion {
		return nil, fmt.Errorf("handshake envelope protocol mismatch: got %s, expected %s", respEnvelope.Protocol, protocol.ProtocolVersion)
	}
	if respEnvelope.RequestID != envelope.ID {
		return nil, fmt.Errorf("handshake response requestId mismatch: got %q, expected %q", respEnvelope.RequestID, envelope.ID)
	}
	if err := r.client.AdoptPeerRouting(respEnvelope); err != nil {
		return nil, fmt.Errorf("bind handshake peer route: %w", err)
	}

	var respPayload HelloResponse
	if len(respEnvelope.Payload) > 0 {
		if err := json.Unmarshal(respEnvelope.Payload, &respPayload); err != nil {
			return nil, fmt.Errorf("unmarshal handshake response failed: %w", err)
		}
	}

	if respPayload.Protocol != protocol.ProtocolVersion {
		return nil, fmt.Errorf("protocol mismatch: got %s, expected %s", respPayload.Protocol, protocol.ProtocolVersion)
	}

	return &respPayload, nil
}

type HelloResponse struct {
	Protocol      string                     `json:"protocol"`
	Capabilities  []string                   `json:"capabilities"`
	RPCNamespaces []string                   `json:"rpcNamespaces,omitempty"`
	Channels      []string                   `json:"channels,omitempty"`
	Metadata      map[string]json.RawMessage `json:"metadata,omitempty"`
}

type HandlerRegistry struct {
	requestHandlers      map[string]RequestHandler
	notificationHandlers map[string]NotificationHandler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		requestHandlers:      make(map[string]RequestHandler),
		notificationHandlers: make(map[string]NotificationHandler),
	}
}

func (r *HandlerRegistry) RegisterRequest(method string, handler RequestHandler) {
	r.requestHandlers[method] = handler
}

func (r *HandlerRegistry) RegisterNotification(method string, handler NotificationHandler) {
	r.notificationHandlers[method] = handler
}

// MergeFrom merges handlers from another registry into this registry. Request
// methods must be unique. Notification handlers with the same method are
// chained in registration order so a single process-backed service can host
// multiple logical namespaces without pretending to be multiple services.
func (r *HandlerRegistry) MergeFrom(other *HandlerRegistry) error {
	if other == nil || other == r {
		return nil
	}
	for method, handler := range other.requestHandlers {
		if _, exists := r.requestHandlers[method]; exists {
			return fmt.Errorf("duplicate request handler: %s", method)
		}
		r.requestHandlers[method] = handler
	}
	for method, handler := range other.notificationHandlers {
		if existing, exists := r.notificationHandlers[method]; exists {
			first := existing
			second := handler
			r.notificationHandlers[method] = func(ctx context.Context, notification protocol.Envelope) error {
				if err := first(ctx, notification); err != nil {
					return err
				}
				return second(ctx, notification)
			}
			continue
		}
		r.notificationHandlers[method] = handler
	}
	return nil
}

func (r *HandlerRegistry) HandleRequest(ctx context.Context, client *Client, request protocol.Envelope) {
	handler, ok := r.requestHandlers[request.Method]
	if !ok {
		r.sendErrorResponse(ctx, client, request, protocol.ErrorNotFound, fmt.Sprintf("unknown method: %s", request.Method), false)
		return
	}

	resp, err := handler(ctx, request)
	if err != nil {
		r.sendErrorResponse(ctx, client, request, protocol.ErrorInternal, err.Error(), false)
		return
	}

	if _, err := client.SendResponse(ctx, request, resp); err != nil {
		fmt.Fprintf(os.Stderr, "send response failed: %v\n", err)
	}
}

func (r *HandlerRegistry) HandleNotification(ctx context.Context, client *Client, notification protocol.Envelope) {
	handler, ok := r.notificationHandlers[notification.Method]
	if !ok {
		return
	}

	if err := handler(ctx, notification); err != nil {
		fmt.Fprintf(os.Stderr, "notification handler error: %v\n", err)
	}
}

func (r *HandlerRegistry) sendErrorResponse(ctx context.Context, client *Client, request protocol.Envelope, code protocol.ErrorCode, message string, retryable bool) {
	if _, err := client.SendError(ctx, request, code, message, retryable, nil); err != nil {
		fmt.Fprintf(os.Stderr, "send error response failed: %v\n", err)
	}
}

func (r *Runner) findRegistryForNotification(notification protocol.Envelope) *HandlerRegistry {
	if notification.ServiceID != "" {
		if reg, ok := r.services[notification.ServiceID]; ok {
			return reg
		}
	}
	return nil
}

func (r *Runner) dispatchHandlerTask(task handlerTask) {
	r.workerWg.Add(1)
	go func() {
		defer r.workerWg.Done()
		switch task.envelope.Type {
		case protocol.MessageTypeRequest:
			task.registry.HandleRequest(task.ctx, task.client, task.envelope)
		case protocol.MessageTypeNotification:
			task.registry.HandleNotification(task.ctx, task.client, task.envelope)
		}
		<-r.workerSem
	}()
}

func (r *Runner) acquireWorker() bool {
	select {
	case r.workerSem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *Runner) Run(ctx context.Context, defaultRegistry *HandlerRegistry) error {
	if _, err := r.performHandshake(ctx); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	r.running = true

	if r.config.OnReady != nil {
		shutdownFn := r.config.OnReady(ctx, r.client)
		if shutdownFn != nil {
			defer shutdownFn(ctx)
		}
	}

	for {
		select {
		case <-ctx.Done():
			r.workerWg.Wait()
			return nil
		default:
		}

		envelope, err := r.client.Transport().Receive(ctx)
		if err != nil {
			if err == io.EOF {
				r.workerWg.Wait()
				return nil
			}
			if ctx.Err() != nil {
				r.workerWg.Wait()
				return nil
			}
			fmt.Fprintf(os.Stderr, "receive failed: %v\n", err)
			continue
		}

		switch envelope.Type {
		case protocol.MessageTypeResponse, protocol.MessageTypeError:
			r.client.DispatchIncomingResponse(envelope)
		case protocol.MessageTypeRequest:
			registry := r.findRegistryForRequest(envelope)
			if registry == nil {
				registry = defaultRegistry
			}
			if registry != nil {
				if r.acquireWorker() {
					r.dispatchHandlerTask(handlerTask{
						ctx:      ctx,
						client:   r.client,
						envelope: envelope,
						registry: registry,
					})
				} else {
					r.sendPoolBusyErrorResponse(ctx, r.client, envelope)
				}
			}
		case protocol.MessageTypeNotification:
			registry := r.findRegistryForNotification(envelope)
			if registry == nil {
				registry = defaultRegistry
			}
			if registry != nil {
				if r.acquireWorker() {
					r.dispatchHandlerTask(handlerTask{
						ctx:      ctx,
						client:   r.client,
						envelope: envelope,
						registry: registry,
					})
				}
			}
		default:
			fmt.Fprintf(os.Stderr, "unexpected message type: %s\n", envelope.Type)
		}
	}
}

func (r *Runner) sendPoolBusyErrorResponse(ctx context.Context, client *Client, request protocol.Envelope) {
	if _, err := client.SendError(ctx, request, protocol.ErrorResourceExhausted, "handler pool exhausted, try again later", true, nil); err != nil {
		fmt.Fprintf(os.Stderr, "send pool busy error failed: %v\n", err)
	}
}

func (r *Runner) Stop() {
	r.running = false
	r.workerWg.Wait()
}
