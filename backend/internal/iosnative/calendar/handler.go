package calendar

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type CalendarHandler struct {
	bridge nativebridge.Bridge
}

func NewCalendarHandler(bridge nativebridge.Bridge) *CalendarHandler {
	return &CalendarHandler{bridge: bridge}
}

func (h *CalendarHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationAuthorizationStatus:
		return h.handleAuthorizationStatus(ctx, request)
	case OperationAuthorizationRequest:
		return h.handleAuthorizationRequest(ctx, request)
	case OperationCalendarsList:
		return h.handleCalendarsList(ctx, request)
	case OperationEventsQuery:
		return h.handleEventsQuery(ctx, request)
	case OperationEventsGet:
		return h.handleEventsGet(ctx, request)
	case OperationEventsCreate:
		return h.handleEventsCreate(ctx, request)
	case OperationEventsUpdate:
		return h.handleEventsUpdate(ctx, request)
	case OperationEventsDelete:
		return h.handleEventsDelete(ctx, request)
	default:
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrOperationNotSupported,
				Message: "unknown calendar operation: " + request.Operation,
			},
		}
	}
}

func (h *CalendarHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
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
		return h.errorResponse(request, ErrCancelled, "calendar status query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleAuthorizationStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
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
		return h.errorResponse(request, ErrCancelled, "calendar authorization status query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleAuthorizationRequest(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	access, ok := request.Payload["access"].(string)
	if !ok || access == "" {
		return h.errorResponse(request, ErrInvalidResponse, "missing required field: access")
	}

	if !IsValidAccessLevel(access) {
		return h.errorResponse(request, ErrInvalidResponse, "invalid access level: "+access)
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationAuthorizationRequest,
			Payload:         map[string]any{"access": access},
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "calendar authorization request cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleCalendarsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationCalendarsList,
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
		return h.errorResponse(request, ErrCancelled, "calendar list query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleEventsQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	startAt, ok := request.Payload["startAt"].(string)
	if !ok || startAt == "" {
		return h.errorResponse(request, ErrInvalidDateRange, "missing required field: startAt")
	}

	endAt, ok := request.Payload["endAt"].(string)
	if !ok || endAt == "" {
		return h.errorResponse(request, ErrInvalidDateRange, "missing required field: endAt")
	}

	startTime, err := time.Parse(time.RFC3339, startAt)
	if err != nil {
		return h.errorResponse(request, ErrInvalidDateRange, "invalid startAt format: "+err.Error())
	}

	endTime, err := time.Parse(time.RFC3339, endAt)
	if err != nil {
		return h.errorResponse(request, ErrInvalidDateRange, "invalid endAt format: "+err.Error())
	}

	if !endTime.After(startTime) {
		return h.errorResponse(request, ErrInvalidDateRange, "endAt must be after startAt")
	}

	rangeDays := endTime.Sub(startTime).Hours() / 24
	if rangeDays > MaxQueryRangeDays {
		return h.errorResponse(request, ErrQueryRangeTooLarge, "query range exceeds maximum allowed days")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationEventsQuery,
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
		return h.errorResponse(request, ErrCancelled, "events query cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleEventsGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	eventID, ok := request.Payload["eventId"].(string)
	if !ok || eventID == "" {
		return h.errorResponse(request, ErrEventNotFound, "missing required field: eventId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationEventsGet,
			Payload:         map[string]any{"eventId": eventID},
		})
		if err != nil {
			done <- h.errorResponse(request, ErrTimeout, err.Error())
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, ErrCancelled, "events get cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleEventsCreate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	title, ok := request.Payload["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return h.errorResponse(request, ErrInvalidResponse, "title is required and cannot be empty")
	}

	if len([]rune(title)) > TitleMaxLengthRunes {
		return h.errorResponse(request, ErrInvalidResponse, "title exceeds maximum length")
	}

	startAt, ok := request.Payload["startAt"].(string)
	if !ok || startAt == "" {
		return h.errorResponse(request, ErrInvalidDateRange, "missing required field: startAt")
	}

	endAt, ok := request.Payload["endAt"].(string)
	if !ok || endAt == "" {
		return h.errorResponse(request, ErrInvalidDateRange, "missing required field: endAt")
	}

	startTime, err := time.Parse(time.RFC3339, startAt)
	if err != nil {
		return h.errorResponse(request, ErrInvalidDateRange, "invalid startAt format: "+err.Error())
	}

	endTime, err := time.Parse(time.RFC3339, endAt)
	if err != nil {
		return h.errorResponse(request, ErrInvalidDateRange, "invalid endAt format: "+err.Error())
	}

	if !endTime.After(startTime) {
		return h.errorResponse(request, ErrInvalidDateRange, "endAt must be after startAt")
	}

	if tz, ok := request.Payload["timeZone"].(string); ok && tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return h.errorResponse(request, ErrInvalidTimezone, "invalid timezone: "+err.Error())
		}
	}

	if notes, ok := request.Payload["notes"].(string); ok && len(notes) > NotesMaxLength {
		return h.errorResponse(request, ErrInvalidResponse, "notes exceed maximum length")
	}

	if url, ok := request.Payload["url"].(string); ok && url != "" {
		if !isValidURL(url) {
			return h.errorResponse(request, ErrInvalidResponse, "invalid URL scheme")
		}
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationEventsCreate,
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
		return h.errorResponse(request, ErrCancelled, "events create cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleEventsUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	eventID, ok := request.Payload["eventId"].(string)
	if !ok || eventID == "" {
		return h.errorResponse(request, ErrEventNotFound, "missing required field: eventId")
	}

	if title, ok := request.Payload["title"].(string); ok {
		if len([]rune(title)) > TitleMaxLengthRunes {
			return h.errorResponse(request, ErrInvalidResponse, "title exceeds maximum length")
		}
	}

	if tz, ok := request.Payload["timeZone"].(string); ok && tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return h.errorResponse(request, ErrInvalidTimezone, "invalid timezone: "+err.Error())
		}
	}

	if notes, ok := request.Payload["notes"].(string); ok && len(notes) > NotesMaxLength {
		return h.errorResponse(request, ErrInvalidResponse, "notes exceed maximum length")
	}

	if url, ok := request.Payload["url"].(string); ok && url != "" {
		if !isValidURL(url) {
			return h.errorResponse(request, ErrInvalidResponse, "invalid URL scheme")
		}
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationEventsUpdate,
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
		return h.errorResponse(request, ErrCancelled, "events update cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) handleEventsDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	eventID, ok := request.Payload["eventId"].(string)
	if !ok || eventID == "" {
		return h.errorResponse(request, ErrEventNotFound, "missing required field: eventId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       OperationEventsDelete,
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
		return h.errorResponse(request, ErrCancelled, "events delete cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *CalendarHandler) errorResponse(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:       code,
			Message:    message,
			DomainCode: "CALENDAR_DOMAIN",
		},
	}
}

func isValidURL(url string) bool {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}
	return true
}
