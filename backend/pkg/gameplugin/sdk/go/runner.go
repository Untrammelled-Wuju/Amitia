package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const HelloMethod = "control.handshake.hello"

type SDKInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type HelloConfiguration struct {
	SupportedProtocols []string
	Capabilities       []string
	RPCNamespaces      []string
	Services           []ServiceHelloDescriptor
	Sinks              []SinkHelloDescriptor
	SDK                *SDKInfo
	Metadata           map[string]json.RawMessage
}

type ServiceHelloDescriptor struct {
	ServiceID    string   `json:"serviceId"`
	Capabilities []string `json:"capabilities,omitempty"`
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
}

type ServiceRegistry struct {
	ServiceID string
	Registry  *HandlerRegistry
}

type Runner struct {
	client    *Client
	config    RunnerConfig
	services  map[string]*HandlerRegistry
	running   bool
}

func NewRunner(client *Client, config RunnerConfig) *Runner {
	return &Runner{
		client:   client,
		config:   config,
		services: make(map[string]*HandlerRegistry),
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

	if len(r.config.Hello.Services) > 0 {
		helloReq["services"] = r.config.Hello.Services
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

	if respEnvelope.Type == protocol.MessageTypeError {
		if respEnvelope.Error != nil {
			return nil, fmt.Errorf("handshake failed: %s - %s", respEnvelope.Error.Code, respEnvelope.Error.Message)
		}
		return nil, fmt.Errorf("handshake failed with unknown error")
	}

	if respEnvelope.Type != protocol.MessageTypeResponse {
		return nil, fmt.Errorf("unexpected handshake response type: %s", respEnvelope.Type)
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
	Protocol      string            `json:"protocol"`
	Capabilities  []string          `json:"capabilities"`
	RPCNamespaces []string          `json:"rpcNamespaces,omitempty"`
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
			return nil
		default:
		}

		envelope, err := r.client.Transport().Receive(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			if ctx.Err() != nil {
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
				registry.HandleRequest(ctx, r.client, envelope)
			}
		case protocol.MessageTypeNotification:
			registry := r.findRegistryForNotification(envelope)
			if registry == nil {
				registry = defaultRegistry
			}
			if registry != nil {
				registry.HandleNotification(ctx, r.client, envelope)
			}
		default:
			fmt.Fprintf(os.Stderr, "unexpected message type: %s\n", envelope.Type)
		}
	}
}

func (r *Runner) Stop() {
	r.running = false
}
