package reminders

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type RemindersHandler struct {
	bridge nativebridge.Bridge
}

func NewRemindersHandler(bridge nativebridge.Bridge) *RemindersHandler {
	return &RemindersHandler{bridge: bridge}
}

func (h *RemindersHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationAuthorizationStatus:
		return h.handleAuthorizationStatus(ctx, request)
	case OperationAuthorizationRequest:
		return h.handleAuthorizationRequest(ctx, request)
	case OperationListsList:
		return h.handleListsList(ctx, request)
	case OperationQuery:
		return h.handleQuery(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	case OperationCreate:
		return h.handleCreate(ctx, request)
	case OperationUpdate:
		return h.handleUpdate(ctx, request)
	case OperationComplete:
		return h.handleComplete(ctx, request)
	case OperationUncomplete:
		return h.handleUncomplete(ctx, request)
	case OperationDelete:
		return h.handleDelete(ctx, request)
	default:
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrOperationNotSupported,
				Message: "unknown reminders operation: " + request.Operation,
			},
		}
	}
}

func (h *RemindersHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationStatus,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders status query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleAuthorizationStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationAuthorizationStatus,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders authorization status query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleAuthorizationRequest(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationAuthorizationRequest,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders authorization request cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleListsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationListsList,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders lists list cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	status, _ := request.Payload["status"].(string)
	if status == "" {
		status = "incomplete"
	}
	if !IsValidQueryStatus(status) {
		return h.errorResponse(request, ErrInvalidResponse, "invalid status filter: "+status)
	}

	if dueStart, ok := request.Payload["dueStart"].(string); ok && dueStart != "" {
		if _, err := time.Parse(time.RFC3339, dueStart); err != nil {
			return h.errorResponse(request, ErrInvalidDateRange, "invalid dueStart format: "+err.Error())
		}
	}
	if dueEnd, ok := request.Payload["dueEnd"].(string); ok && dueEnd != "" {
		if _, err := time.Parse(time.RFC3339, dueEnd); err != nil {
			return h.errorResponse(request, ErrInvalidDateRange, "invalid dueEnd format: "+err.Error())
		}
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationQuery,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	reminderID, ok := request.Payload["reminderId"].(string)
	if !ok || reminderID == "" {
		return h.errorResponse(request, ErrReminderNotFound, "missing required field: reminderId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationGet,
			Payload:         map[string]any{"reminderId": reminderID},
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders get cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleCreate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	title, ok := request.Payload["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return h.errorResponse(request, ErrTitleEmpty, "title is required and cannot be empty")
	}
	if len([]rune(title)) > TitleMaxLengthRunes {
		return h.errorResponse(request, ErrTitleTooLong, "title exceeds maximum length")
	}

	if notes, ok := request.Payload["notes"].(string); ok && len(notes) > NotesMaxLength {
		return h.errorResponse(request, ErrNotesTooLong, "notes exceed maximum length")
	}

	if url, ok := request.Payload["url"].(string); ok && url != "" {
		if !isValidURL(url) {
			return h.errorResponse(request, ErrInvalidURL, "invalid URL scheme")
		}
	}

	if priority, ok := request.Payload["priority"].(string); ok && priority != "" {
		if !IsValidPriority(priority) {
			return h.errorResponse(request, ErrInvalidPriority, "invalid priority: "+priority)
		}
	}

	if tz, ok := request.Payload["timeZone"].(string); ok && tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return h.errorResponse(request, ErrInvalidTimezone, "invalid timezone: "+err.Error())
		}
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationCreate,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders create cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	reminderID, ok := request.Payload["reminderId"].(string)
	if !ok || reminderID == "" {
		return h.errorResponse(request, ErrReminderNotFound, "missing required field: reminderId")
	}

	if title, ok := request.Payload["title"].(string); ok && len([]rune(title)) > TitleMaxLengthRunes {
		return h.errorResponse(request, ErrTitleTooLong, "title exceeds maximum length")
	}

	if notes, ok := request.Payload["notes"].(string); ok && len(notes) > NotesMaxLength {
		return h.errorResponse(request, ErrNotesTooLong, "notes exceed maximum length")
	}

	if url, ok := request.Payload["url"].(string); ok && url != "" {
		if !isValidURL(url) {
			return h.errorResponse(request, ErrInvalidURL, "invalid URL scheme")
		}
	}

	if priority, ok := request.Payload["priority"].(string); ok && priority != "" {
		if !IsValidPriority(priority) {
			return h.errorResponse(request, ErrInvalidPriority, "invalid priority: "+priority)
		}
	}

	if tz, ok := request.Payload["timeZone"].(string); ok && tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return h.errorResponse(request, ErrInvalidTimezone, "invalid timezone: "+err.Error())
		}
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationUpdate,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders update cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleComplete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	reminderID, ok := request.Payload["reminderId"].(string)
	if !ok || reminderID == "" {
		return h.errorResponse(request, ErrReminderNotFound, "missing required field: reminderId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationComplete,
			Payload:         request.Payload,
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders complete cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleUncomplete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	reminderID, ok := request.Payload["reminderId"].(string)
	if !ok || reminderID == "" {
		return h.errorResponse(request, ErrReminderNotFound, "missing required field: reminderId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationUncomplete,
			Payload:         map[string]any{"reminderId": reminderID},
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders uncomplete cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) handleDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	reminderID, ok := request.Payload["reminderId"].(string)
	if !ok || reminderID == "" {
		return h.errorResponse(request, ErrReminderNotFound, "missing required field: reminderId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       OperationDelete,
			Payload:         map[string]any{"reminderId": reminderID},
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "reminders delete cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *RemindersHandler) errorResponse(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:       code,
			Message:    message,
			DomainCode: "REMINDERS_DOMAIN",
		},
	}
}

func isValidURL(url string) bool {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}
	return true
}
