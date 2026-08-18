package alarms

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type AlarmHandler struct {
	bridge nativebridge.Bridge
}

func NewAlarmHandler(bridge nativebridge.Bridge) *AlarmHandler {
	return &AlarmHandler{bridge: bridge}
}

func (h *AlarmHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationAuthorizationStatus:
		return h.handleAuthorizationStatus(ctx, request)
	case OperationAuthorizationRequest:
		return h.handleAuthorizationRequest(ctx, request)
	case OperationList:
		return h.handleList(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	case OperationSchedule:
		return h.handleSchedule(ctx, request)
	case OperationStop:
		return h.handleStop(ctx, request)
	case OperationCancel:
		return h.handleCancel(ctx, request)
	default:
		return NewAlarmError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *AlarmHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewAlarmError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- NewAlarmError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewAlarmError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *AlarmHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *AlarmHandler) handleAuthorizationStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationAuthorizationStatus, map[string]any{})
}

func (h *AlarmHandler) handleAuthorizationRequest(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationAuthorizationRequest, map[string]any{})
}

func (h *AlarmHandler) handleList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationList, map[string]any{})
}

func (h *AlarmHandler) handleGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	alarmID, ok := request.Payload["alarmId"].(string)
	if !ok || alarmID == "" {
		return NewAlarmError(request, ErrAlarmsNotFound, "missing required field: alarmId")
	}
	return h.bridgeCall(ctx, request, OperationGet, map[string]any{
		"alarmId": alarmID,
	})
}

func (h *AlarmHandler) handleSchedule(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	kind, ok := request.Payload["kind"].(string)
	if !ok || !IsValidKind(kind) {
		return NewAlarmError(request, ErrAlarmsScheduleInvalid, "invalid or missing field: kind")
	}

	title, ok := request.Payload["title"].(string)
	if !ok || title == "" {
		return NewAlarmError(request, ErrAlarmsScheduleInvalid, "missing required field: title")
	}

	payload := map[string]any{
		"kind":  kind,
		"title": title,
	}

	var schedule *IOSAlarmSchedule
	if scheduleRaw, ok := request.Payload["schedule"].(map[string]any); ok && scheduleRaw != nil {
		schedule = parseSchedule(scheduleRaw)
		if err := ValidateSchedule(schedule); err != nil {
			return NewAlarmError(request, ErrAlarmsScheduleInvalid, err.Error())
		}
		payload["schedule"] = schedule
	} else if kind == "alarm" || kind == "countdown_alarm" {
		return NewAlarmError(request, ErrAlarmsScheduleInvalid, "schedule is required for alarm kind")
	}

	var countdown *IOSAlarmCountdown
	if countdownRaw, ok := request.Payload["countdown"].(map[string]any); ok && countdownRaw != nil {
		countdown = parseCountdown(countdownRaw)
		if err := ValidateCountdown(countdown); err != nil {
			return NewAlarmError(request, ErrAlarmsCountdownInvalid, err.Error())
		}
		payload["countdown"] = countdown
	} else if kind == "timer" {
		return NewAlarmError(request, ErrAlarmsCountdownInvalid, "countdown is required for timer kind")
	}

	presentationRaw, ok := request.Payload["presentation"].(map[string]any)
	if !ok || presentationRaw == nil {
		return NewAlarmError(request, ErrAlarmsPresentationInvalid, "missing required field: presentation")
	}
	presentation := parsePresentation(presentationRaw)
	if err := ValidatePresentation(presentation); err != nil {
		return NewAlarmError(request, ErrAlarmsPresentationInvalid, err.Error())
	}
	payload["presentation"] = presentation

	if soundRaw, ok := request.Payload["sound"].(map[string]any); ok && soundRaw != nil {
		sound := parseSound(soundRaw)
		if err := ValidateSound(sound); err != nil {
			return NewAlarmError(request, ErrAlarmsSoundInvalid, err.Error())
		}
		payload["sound"] = sound
	}

	if action, ok := request.Payload["action"].(string); ok && action != "" {
		if !IsValidAlarmIntentAction(action) {
			return NewAlarmError(request, ErrAlarmsActionInvalid, "invalid action: "+action)
		}
		payload["action"] = action
	}

	if metadataRaw, ok := request.Payload["metadata"].(map[string]any); ok && metadataRaw != nil {
		metadata := parseMetadata(metadataRaw)
		payload["metadata"] = metadata
	}

	return h.bridgeCall(ctx, request, OperationSchedule, payload)
}

func (h *AlarmHandler) handleStop(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	alarmID, ok := request.Payload["alarmId"].(string)
	if !ok || alarmID == "" {
		return NewAlarmError(request, ErrAlarmsNotFound, "missing required field: alarmId")
	}
	return h.bridgeCall(ctx, request, OperationStop, map[string]any{
		"alarmId": alarmID,
	})
}

func (h *AlarmHandler) handleCancel(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	alarmID, ok := request.Payload["alarmId"].(string)
	if !ok || alarmID == "" {
		return NewAlarmError(request, ErrAlarmsNotFound, "missing required field: alarmId")
	}
	return h.bridgeCall(ctx, request, OperationCancel, map[string]any{
		"alarmId": alarmID,
	})
}

func parseSchedule(raw map[string]any) *IOSAlarmSchedule {
	s := &IOSAlarmSchedule{}
	if fireAt, ok := raw["fireAt"].(string); ok {
		s.FireAt = &fireAt
	}
	if hour, ok := raw["hour"].(float64); ok {
		h := int(hour)
		s.Hour = &h
	}
	if minute, ok := raw["minute"].(float64); ok {
		m := int(minute)
		s.Minute = &m
	}
	if recurrence, ok := raw["recurrence"].(string); ok {
		s.Recurrence = recurrence
	}
	if weekdays, ok := raw["weekdays"].([]any); ok {
		ws := make([]string, 0, len(weekdays))
		for _, w := range weekdays {
			if s, ok := w.(string); ok {
				ws = append(ws, s)
			}
		}
		s.Weekdays = ws
	}
	return s
}

func parseCountdown(raw map[string]any) *IOSAlarmCountdown {
	c := &IOSAlarmCountdown{}
	if pre, ok := raw["preAlertSeconds"].(float64); ok {
		v := int64(pre)
		c.PreAlertSeconds = &v
	}
	if post, ok := raw["postAlertSeconds"].(float64); ok {
		v := int64(post)
		c.PostAlertSeconds = &v
	}
	return c
}

func parsePresentation(raw map[string]any) IOSAlarmPresentation {
	p := IOSAlarmPresentation{}
	if alertTitle, ok := raw["alertTitle"].(string); ok {
		p.AlertTitle = alertTitle
	}
	if countdownTitle, ok := raw["countdownTitle"].(string); ok {
		p.CountdownTitle = countdownTitle
	}
	if pausedTitle, ok := raw["pausedTitle"].(string); ok {
		p.PausedTitle = pausedTitle
	}
	if tintColor, ok := raw["tintColor"].(string); ok {
		p.TintColor = tintColor
	}
	if secondaryAction, ok := raw["secondaryAction"].(string); ok {
		p.SecondaryAction = secondaryAction
	}
	return p
}

func parseSound(raw map[string]any) IOSAlarmSound {
	s := IOSAlarmSound{}
	if kind, ok := raw["kind"].(string); ok {
		s.Kind = kind
	}
	if soundID, ok := raw["soundId"].(string); ok {
		s.SoundID = soundID
	}
	return s
}

func parseMetadata(raw map[string]any) *IOSAlarmMetadata {
	m := &IOSAlarmMetadata{}
	if kind, ok := raw["kind"].(string); ok {
		m.Kind = kind
	}
	if icon, ok := raw["icon"].(string); ok {
		m.Icon = icon
	}
	if ownerRef, ok := raw["ownerRef"].(string); ok {
		m.OwnerRef = ownerRef
	}
	return m
}
