package share

import (
	"context"
	"errors"
	"sync"

	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type ShareHandler struct {
	client ShareClient
	policy SharePolicy
	mu     sync.RWMutex
}

type SharePolicy struct {
	MaxTextBytes           int
	MaxSubjectBytes        int
	MaxChooserTitleBytes   int
	MaxResources           int
	MaxSingleResourceBytes int64
	MaxTotalBytes          int64
	AllowedRoots           map[resourceuri.ResourceRoot]struct{}
}

func DefaultSharePolicy() SharePolicy {
	return SharePolicy{
		MaxTextBytes:         MaxShareTextBytes,
		MaxSubjectBytes:      MaxSubjectBytes,
		MaxChooserTitleBytes: ChooserTitleMaxBytes,
		MaxResources:         MaxResourcesCount,
		MaxSingleResourceBytes: MaxSingleResourceBytes,
		MaxTotalBytes:        MaxTotalBytes,
		AllowedRoots: map[resourceuri.ResourceRoot]struct{}{
			resourceuri.ResourceRootWorkspace:   {},
			resourceuri.ResourceRootAttachments: {},
			resourceuri.ResourceRootTemp:        {},
			resourceuri.ResourceRootCache:       {},
		},
	}
}

type ShareClient interface {
	Send(ctx context.Context, req ShareSendRequest) (ShareSendResult, error)
	Status(ctx context.Context) (ShareCapabilityState, error)
	Close()
}

func NewShareHandler(client ShareClient) *ShareHandler {
	if client == nil {
		client = NewBlockedShareClient()
	}
	return &ShareHandler{
		client: client,
		policy: DefaultSharePolicy(),
	}
}

func (h *ShareHandler) Execute(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationSend:
		return h.handleSend(ctx, request)
	default:
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    SHARE_UNAVAILABLE,
				Message: "unknown share operation: " + request.Operation,
			},
		}
	}
}

func (h *ShareHandler) handleStatus(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	state, err := h.client.Status(ctx)
	if err != nil {
		var se *shareError
		if errors.As(err, &se) {
			return androidsystem.NotificationResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.NotificationError{
					Code:    se.code,
					Message: se.message,
				},
			}
		}
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    SHARE_UNAVAILABLE,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"supported":              state.Supported,
			"canSend":                state.CanSend,
			"canReceive":             state.CanReceive,
			"nativeHostReady":        state.NativeHostReady,
			"maxResources":           state.MaxResources,
			"maxSingleResourceBytes": state.MaxSingleResourceBytes,
			"maxTotalBytes":          state.MaxTotalBytes,
			"state":                  state.State,
		},
	}
}

func (h *ShareHandler) handleSend(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	text, _ := request.Payload["text"].(string)
	subject, _ := request.Payload["subject"].(string)
	mimeType, _ := request.Payload["mimeType"].(string)
	chooserTitle, _ := request.Payload["chooserTitle"].(string)

	if err := h.validateTextInput(text); err != nil {
		return h.errorResponse(request.RequestID, err)
	}
	if err := h.validateSubjectInput(subject); err != nil {
		return h.errorResponse(request.RequestID, err)
	}
	if err := h.validateChooserTitle(chooserTitle); err != nil {
		return h.errorResponse(request.RequestID, err)
	}

	resources, _ := request.Payload["resources"].([]interface{})
	resourceURIs := make([]string, 0, len(resources))
	for _, r := range resources {
		if s, ok := r.(string); ok {
			resourceURIs = append(resourceURIs, s)
		}
	}

	if len(resourceURIs) > h.policy.MaxResources {
		return h.errorResponse(request.RequestID, &shareError{
			code:    SHARE_TOO_MANY_RESOURCES,
			message: "too many resources to share",
		})
	}

	for _, uri := range resourceURIs {
		if _, err := resourceuri.Parse(uri); err != nil {
			return h.errorResponse(request.RequestID, &shareError{
				code:    SHARE_INVALID_INPUT,
				message: "invalid resource URI: " + err.Error(),
			})
		}
	}

	SendReq := ShareSendRequest{
		Text:         text,
		Subject:      subject,
		Resources:    resourceURIs,
		MIMEType:     mimeType,
		ChooserTitle: chooserTitle,
	}

	result, err := h.client.Send(ctx, SendReq)
	if err != nil {
		return h.errorResponse(request.RequestID, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"status":              result.Status,
			"resourceCount":       result.ResourceCount,
			"mimeType":            result.MIMEType,
			"userActionRequired":  result.UserActionRequired,
		},
	}
}

func (h *ShareHandler) validateTextInput(text string) error {
	if len(text) > h.policy.MaxTextBytes {
		return &shareError{code: SHARE_TEXT_TOO_LARGE, message: "share text exceeds maximum size"}
	}
	return nil
}

func (h *ShareHandler) validateSubjectInput(subject string) error {
	if len(subject) > h.policy.MaxSubjectBytes {
		return &shareError{code: SHARE_SUBJECT_TOO_LARGE, message: "share subject exceeds maximum size"}
	}
	return nil
}

func (h *ShareHandler) validateChooserTitle(title string) error {
	if len(title) > h.policy.MaxChooserTitleBytes {
		return &shareError{code: SHARE_CHOOSER_TITLE_TOO_LARGE, message: "chooser title exceeds maximum size"}
	}
	return nil
}

func (h *ShareHandler) errorResponse(requestID string, err error) androidsystem.NotificationResponse {
	var se *shareError
	if errors.As(err, &se) {
		return androidsystem.NotificationResponse{
			RequestID: requestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    se.code,
				Message: se.message,
			},
		}
	}
	return androidsystem.NotificationResponse{
		RequestID: requestID,
		Status:    "error",
		Error: &androidsystem.NotificationError{
			Code:    SHARE_UNAVAILABLE,
			Message: err.Error(),
		},
	}
}

type shareError struct {
	code    string
	message string
}

func (e *shareError) Error() string { return e.message }

type blockedShareClient struct{}

func NewBlockedShareClient() ShareClient {
	return &blockedShareClient{}
}

func (b *blockedShareClient) Send(ctx context.Context, req ShareSendRequest) (ShareSendResult, error) {
	return ShareSendResult{}, &shareError{code: SHARE_UNAVAILABLE, message: "android native host source not available; share provider blocked"}
}

func (b *blockedShareClient) Status(ctx context.Context) (ShareCapabilityState, error) {
	return ShareCapabilityState{
		Supported: false,
		State:     StateUnsupported,
	}, nil
}

func (b *blockedShareClient) Close() {}
