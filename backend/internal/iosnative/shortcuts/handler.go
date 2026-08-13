package shortcuts

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ShortcutsHandler struct {
	bridge nativebridge.Bridge
}

func NewShortcutsHandler(bridge nativebridge.Bridge) *ShortcutsHandler {
	return &ShortcutsHandler{bridge: bridge}
}

func (h *ShortcutsHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationIntentRegister:
		return h.handleIntentRegister(ctx, request)
	case OperationIntentRevoke:
		return h.handleIntentRevoke(ctx, request)
	case OperationIntentDonate:
		return h.handleIntentDonate(ctx, request)
	case OperationEntitiesCharacters:
		return h.handleEntitiesCharacters(ctx, request)
	case OperationEntitiesConversations:
		return h.handleEntitiesConversations(ctx, request)
	case OperationEntitiesAlarms:
		return h.handleEntitiesAlarms(ctx, request)
	case OperationEntitiesReminders:
		return h.handleEntitiesReminders(ctx, request)
	case OperationEntitiesActions:
		return h.handleEntitiesActions(ctx, request)
	case OperationEntityResolve:
		return h.handleEntityResolve(ctx, request)
	case OperationEntitySuggestions:
		return h.handleEntitySuggestions(ctx, request)
	case OperationActionsCatalog:
		return h.handleActionsCatalog(ctx, request)
	case OperationActionDescribe:
		return h.handleActionDescribe(ctx, request)
	case OperationActionExecute:
		return h.handleActionExecute(ctx, request)
	case OperationActionConfirm:
		return h.handleActionConfirm(ctx, request)
	case OperationRuntimeReadiness:
		return h.handleRuntimeReadiness(ctx, request)
	case OperationRuntimeEnsure:
		return h.handleRuntimeEnsure(ctx, request)
	case OperationSnapshotGet:
		return h.handleSnapshotGet(ctx, request)
	case OperationSnapshotRefresh:
		return h.handleSnapshotRefresh(ctx, request)
	case OperationShortcutsProvider:
		return h.handleShortcutsProvider(ctx, request)
	case OperationShortcutsPhrase:
		return h.handleShortcutsPhrase(ctx, request)
	case OperationShortcutsUpdate:
		return h.handleShortcutsUpdate(ctx, request)
	case OperationSettingsGet:
		return h.handleSettingsGet(ctx, request)
	case OperationSettingsUpdate:
		return h.handleSettingsUpdate(ctx, request)
	default:
		return NewShortcutsError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *ShortcutsHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewShortcutsError(request, ErrShortcutsNativeBridgeUnavailable, "ios native bridge is not available")
	}
	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- NewShortcutsError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewShortcutsError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getInt(m map[string]any, key string, defaultVal int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return defaultVal
}

func getStringSlice(m map[string]any, key string) []string {
	var result []string
	if arr, ok := m[key].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
	}
	return result
}

func (h *ShortcutsHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *ShortcutsHandler) handleIntentRegister(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	intentID := getString(request.Payload, "intentId")
	if intentID == "" {
		return NewShortcutsError(request, ErrShortcutsParameterRequired, "intentId is required")
	}
	return h.bridgeCall(ctx, request, OperationIntentRegister, map[string]any{
		"intentId": intentID,
	})
}

func (h *ShortcutsHandler) handleIntentRevoke(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	intentID := getString(request.Payload, "intentId")
	if intentID == "" {
		return NewShortcutsError(request, ErrShortcutsParameterRequired, "intentId is required")
	}
	return h.bridgeCall(ctx, request, OperationIntentRevoke, map[string]any{
		"intentId": intentID,
	})
}

func (h *ShortcutsHandler) handleIntentDonate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	donation := IntentDonationRequest{
		IntentID:  getString(request.Payload, "intentId"),
		ActionID:  getString(request.Payload, "actionId"),
		Timestamp: getString(request.Payload, "timestamp"),
	}
	if params, ok := request.Payload["parameters"].(map[string]any); ok {
		donation.Parameters = params
	}
	if err := ValidateDonation(donation); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}
	payload := map[string]any{
		"intentId":  donation.IntentID,
		"actionId":  donation.ActionID,
		"timestamp": donation.Timestamp,
	}
	if len(donation.Parameters) > 0 {
		payload["parameters"] = donation.Parameters
	}
	return h.bridgeCall(ctx, request, OperationIntentDonate, payload)
}

func (h *ShortcutsHandler) handleEntitiesCharacters(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", DefaultListLimit)
	limit = ClampLimit(limit)
	return h.bridgeCall(ctx, request, OperationEntitiesCharacters, map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
		"query":  getString(request.Payload, "query"),
	})
}

func (h *ShortcutsHandler) handleEntitiesConversations(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", DefaultListLimit)
	limit = ClampLimit(limit)
	payload := map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
		"query":  getString(request.Payload, "query"),
	}
	if charID := getString(request.Payload, "characterId"); charID != "" {
		payload["characterId"] = charID
	}
	return h.bridgeCall(ctx, request, OperationEntitiesConversations, payload)
}

func (h *ShortcutsHandler) handleEntitiesAlarms(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", DefaultListLimit)
	limit = ClampLimit(limit)
	return h.bridgeCall(ctx, request, OperationEntitiesAlarms, map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
	})
}

func (h *ShortcutsHandler) handleEntitiesReminders(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", DefaultListLimit)
	limit = ClampLimit(limit)
	return h.bridgeCall(ctx, request, OperationEntitiesReminders, map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
	})
}

func (h *ShortcutsHandler) handleEntitiesActions(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", MaxCatalogActions)
	limit = ClampLimitWithMax(limit, MaxCatalogActions)
	return h.bridgeCall(ctx, request, OperationEntitiesActions, map[string]any{
		"limit":  limit,
		"offset": getInt(request.Payload, "offset", 0),
	})
}

func (h *ShortcutsHandler) handleEntityResolve(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	entityID := getString(request.Payload, "entityId")
	if err := ValidateEntityID(entityID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationEntityResolve, map[string]any{
		"entityId": entityID,
		"entityType": getString(request.Payload, "entityType"),
	})
}

func (h *ShortcutsHandler) handleEntitySuggestions(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	limit := getInt(request.Payload, "limit", MaxSuggestedEntities)
	limit = ClampLimitWithMax(limit, MaxSuggestedEntities)
	return h.bridgeCall(ctx, request, OperationEntitySuggestions, map[string]any{
		"limit":      limit,
		"entityType": getString(request.Payload, "entityType"),
	})
}

func (h *ShortcutsHandler) handleActionsCatalog(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationActionsCatalog, map[string]any{
		"includeHighRisk": getBool(request.Payload, "includeHighRisk"),
	})
}

func (h *ShortcutsHandler) handleActionDescribe(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	actionID := getString(request.Payload, "actionId")
	if err := ValidateActionID(actionID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationActionDescribe, map[string]any{
		"actionId": actionID,
	})
}

func (h *ShortcutsHandler) handleActionExecute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	actionID := getString(request.Payload, "actionId")
	if err := ValidateActionID(actionID); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}

	idempotencyKey := getString(request.Payload, "idempotencyKey")
	if idempotencyKey != "" && len(idempotencyKey) < MinIdempotencyKeyLength {
		return NewShortcutsError(request, ErrShortcutsIdempotencyInvalid, "idempotencyKey too short")
	}

	parameters := map[string]any{}
	if p, ok := request.Payload["parameters"].(map[string]any); ok {
		parameters = p
	}
	if err := ValidateParameters(parameters); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}

	payload := map[string]any{
		"actionId": actionID,
	}
	if len(parameters) > 0 {
		payload["parameters"] = parameters
	}
	if idempotencyKey != "" {
		payload["idempotencyKey"] = idempotencyKey
	}
	if invocationID := getString(request.Payload, "invocationId"); invocationID != "" {
		payload["invocationId"] = invocationID
	}
	if executionMode := getString(request.Payload, "executionMode"); executionMode != "" {
		payload["executionMode"] = executionMode
	}
	if userID := getString(request.Payload, "userId"); userID != "" {
		payload["userId"] = userID
	}
	if sessionID := getString(request.Payload, "sessionId"); sessionID != "" {
		payload["sessionId"] = sessionID
	}
	if characterID := getString(request.Payload, "characterId"); characterID != "" {
		payload["characterId"] = characterID
	}
	if conversationID := getString(request.Payload, "conversationId"); conversationID != "" {
		payload["conversationId"] = conversationID
	}

	return h.bridgeCall(ctx, request, OperationActionExecute, payload)
}

func (h *ShortcutsHandler) handleActionConfirm(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	confirmReq := ConfirmationRequest{
		ActionID:   getString(request.Payload, "actionId"),
		Title:      getString(request.Payload, "title"),
		Message:    getString(request.Payload, "message"),
		ObjectName: getString(request.Payload, "objectName"),
		Consequence: getString(request.Payload, "consequence"),
	}
	if err := ValidateConfirmationRequest(confirmReq); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}
	payload := map[string]any{
		"actionId": confirmReq.ActionID,
		"title":    confirmReq.Title,
		"message":  confirmReq.Message,
	}
	if confirmReq.ObjectName != "" {
		payload["objectName"] = confirmReq.ObjectName
	}
	if confirmReq.Consequence != "" {
		payload["consequence"] = confirmReq.Consequence
	}
	return h.bridgeCall(ctx, request, OperationActionConfirm, payload)
}

func (h *ShortcutsHandler) handleRuntimeReadiness(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationRuntimeReadiness, map[string]any{})
}

func (h *ShortcutsHandler) handleRuntimeEnsure(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	requirement := getString(request.Payload, "requirement")
	if requirement == "" {
		requirement = string(ShortcutRuntimeRequirementNativeOnly)
	}
	payload := map[string]any{
		"requirement": requirement,
	}
	if timeoutMs := getInt(request.Payload, "timeoutMs", 0); timeoutMs > 0 {
		payload["timeoutMs"] = timeoutMs
	}
	return h.bridgeCall(ctx, request, OperationRuntimeEnsure, payload)
}

func (h *ShortcutsHandler) handleSnapshotGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationSnapshotGet, map[string]any{
		"entityType": getString(request.Payload, "entityType"),
	})
}

func (h *ShortcutsHandler) handleSnapshotRefresh(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationSnapshotRefresh, map[string]any{
		"entityType": getString(request.Payload, "entityType"),
	})
}

func (h *ShortcutsHandler) handleShortcutsProvider(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationShortcutsProvider, map[string]any{
		"includeHighRisk": getBool(request.Payload, "includeHighRisk"),
	})
}

func (h *ShortcutsHandler) handleShortcutsPhrase(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	phrase := getString(request.Payload, "phrase")
	if err := ValidateShortcutPhrase(phrase); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShortcutsError(request, code, msg)
	}
	return h.bridgeCall(ctx, request, OperationShortcutsPhrase, map[string]any{
		"phrase": phrase,
	})
}

func (h *ShortcutsHandler) handleShortcutsUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	actionIDs := getStringSlice(request.Payload, "actionIds")
	return h.bridgeCall(ctx, request, OperationShortcutsUpdate, map[string]any{
		"refreshParams": getBool(request.Payload, "refreshParams"),
		"actionIds":     actionIDs,
	})
}

func (h *ShortcutsHandler) handleSettingsGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationSettingsGet, map[string]any{})
}

func (h *ShortcutsHandler) handleSettingsUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	settings := map[string]any{}
	for _, key := range []string{
		"enabled", "askAmitiaEnabled", "voiceEnabled", "alarmEnabled",
		"reminderEnabled", "calendarEnabled", "shareEnabled",
		"exposeConversationTitles", "safeToolModeDefault", "backgroundAutomationSafe",
	} {
		if v, ok := request.Payload[key]; ok {
			settings[key] = v
		}
	}
	return h.bridgeCall(ctx, request, OperationSettingsUpdate, settings)
}
