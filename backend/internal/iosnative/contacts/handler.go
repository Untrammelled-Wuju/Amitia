package contacts

import (
	"context"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ContactsHandler struct {
	bridge nativebridge.Bridge
}

func NewContactsHandler(bridge nativebridge.Bridge) *ContactsHandler {
	return &ContactsHandler{bridge: bridge}
}

func (h *ContactsHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationAuthorizationStatus:
		return h.handleAuthorizationStatus(ctx, request)
	case OperationAuthorizationRequest:
		return h.handleAuthorizationRequest(ctx, request)
	case OperationSearch:
		return h.handleSearch(ctx, request)
	case OperationList:
		return h.handleList(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	case OperationCreate:
		return h.handleCreate(ctx, request)
	case OperationUpdate:
		return h.handleUpdate(ctx, request)
	case OperationDelete:
		return h.handleDelete(ctx, request)
	case OperationContainersList:
		return h.handleContainersList(ctx, request)
	case OperationGroupsList:
		return h.handleGroupsList(ctx, request)
	case OperationPhotoGet:
		return h.handlePhotoGet(ctx, request)
	case OperationPhotoSet:
		return h.handlePhotoSet(ctx, request)
	case OperationPhotoRemove:
		return h.handlePhotoRemove(ctx, request)
	default:
		return h.errorResponse(request, nativebridge.ErrOperationNotSupported, "unknown contacts operation: "+request.Operation)
	}
}

func (h *ContactsHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
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
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return h.errorResponse(request, nativebridge.ErrBridgeTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *ContactsHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	return h.bridgeCall(ctx, request, OperationStatus, nil)
}

func (h *ContactsHandler) handleAuthorizationStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	return h.bridgeCall(ctx, request, OperationAuthorizationStatus, nil)
}

func (h *ContactsHandler) handleAuthorizationRequest(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	return h.bridgeCall(ctx, request, OperationAuthorizationRequest, nil)
}

func (h *ContactsHandler) handleSearch(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	query, ok := request.Payload["query"].(string)
	if !ok || query == "" {
		return h.errorResponse(request, ErrInvalidName, "missing required field: query")
	}

	payload := map[string]any{
		"query": query,
	}

	if field, ok := request.Payload["field"].(string); ok && field != "" {
		if !SupportedSearchFields[field] {
			return h.errorResponse(request, ErrInvalidSearchField, "unsupported search field: "+field)
		}
		payload["field"] = field
	}

	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampSearchLimit(int(limit))
	}

	if includePhones, ok := request.Payload["includePhones"].(bool); ok {
		payload["includePhones"] = includePhones
	}
	if includeEmails, ok := request.Payload["includeEmails"].(bool); ok {
		payload["includeEmails"] = includeEmails
	}

	return h.bridgeCall(ctx, request, OperationSearch, payload)
}

func (h *ContactsHandler) handleList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	payload := map[string]any{}

	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampListLimit(int(limit))
	}
	if cursor, ok := request.Payload["cursor"].(string); ok {
		payload["cursor"] = cursor
	}
	if includeOrganization, ok := request.Payload["includeOrganization"].(bool); ok {
		payload["includeOrganization"] = includeOrganization
	}
	if includePhones, ok := request.Payload["includePhones"].(bool); ok {
		payload["includePhones"] = includePhones
	}
	if includeEmails, ok := request.Payload["includeEmails"].(bool); ok {
		payload["includeEmails"] = includeEmails
	}

	return h.bridgeCall(ctx, request, OperationList, payload)
}

func (h *ContactsHandler) handleGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	payload := map[string]any{
		"contactId": contactID,
	}

	if includePhones, ok := request.Payload["includePhones"].(bool); ok {
		payload["includePhones"] = includePhones
	}
	if includeEmails, ok := request.Payload["includeEmails"].(bool); ok {
		payload["includeEmails"] = includeEmails
	}
	if includeAddresses, ok := request.Payload["includeAddresses"].(bool); ok {
		payload["includeAddresses"] = includeAddresses
	}
	if includeDates, ok := request.Payload["includeDates"].(bool); ok {
		payload["includeDates"] = includeDates
	}
	if includeSocial, ok := request.Payload["includeSocial"].(bool); ok {
		payload["includeSocial"] = includeSocial
	}
	if includePhoto, ok := request.Payload["includePhoto"].(bool); ok {
		payload["includePhoto"] = includePhoto
	}

	return h.bridgeCall(ctx, request, OperationGet, payload)
}

func (h *ContactsHandler) handleCreate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	name := extractContactNameInput(request.Payload)
	if !hasAnyIdentity(request.Payload, name) {
		return h.errorResponse(request, ErrInvalidName, "contact must have at least a name, organization, phone, or email")
	}

	payload := map[string]any{
		"name": name,
	}

	if org, ok := request.Payload["organization"].(string); ok {
		payload["organization"] = TruncateStringRunes(org, OrganizationMaxLength)
	}
	if dept, ok := request.Payload["department"].(string); ok {
		payload["department"] = TruncateStringRunes(dept, DepartmentMaxLength)
	}
	if jobTitle, ok := request.Payload["jobTitle"].(string); ok {
		payload["jobTitle"] = TruncateStringRunes(jobTitle, JobTitleMaxLength)
	}

	if phones, ok := request.Payload["phoneNumbers"].([]any); ok {
		payload["phoneNumbers"] = truncateLabeledStrings(phones, MaxPhoneNumbers)
	}
	if emails, ok := request.Payload["emailAddresses"].([]any); ok {
		payload["emailAddresses"] = truncateLabeledStrings(emails, MaxEmailAddresses)
	}
	if urls, ok := request.Payload["urls"].([]any); ok {
		payload["urls"] = truncateLabeledStrings(urls, MaxURLs)
	}
	if photoURI, ok := request.Payload["photoResourceUri"].(string); ok && photoURI != "" {
		payload["photoResourceUri"] = photoURI
	}
	if containerID, ok := request.Payload["containerId"].(string); ok && containerID != "" {
		payload["containerId"] = containerID
	}

	return h.bridgeCall(ctx, request, OperationCreate, payload)
}

func (h *ContactsHandler) handleUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	payload := map[string]any{
		"contactId": contactID,
	}

	if v, ok := request.Payload["name"].(map[string]any); ok {
		payload["name"] = v
	}
	if v, ok := request.Payload["organization"].(string); ok {
		payload["organization"] = TruncateStringRunes(v, OrganizationMaxLength)
	}
	if v, ok := request.Payload["department"].(string); ok {
		payload["department"] = TruncateStringRunes(v, DepartmentMaxLength)
	}
	if v, ok := request.Payload["jobTitle"].(string); ok {
		payload["jobTitle"] = TruncateStringRunes(v, JobTitleMaxLength)
	}
	if v, ok := request.Payload["phoneNumbers"].([]any); ok {
		payload["phoneNumbers"] = truncateLabeledStrings(v, MaxPhoneNumbers)
	}
	if v, ok := request.Payload["emailAddresses"].([]any); ok {
		payload["emailAddresses"] = truncateLabeledStrings(v, MaxEmailAddresses)
	}
	if v, ok := request.Payload["urls"].([]any); ok {
		payload["urls"] = truncateLabeledStrings(v, MaxURLs)
	}
	if v, ok := request.Payload["photoResourceUri"].(string); ok {
		payload["photoResourceUri"] = v
	}
	if v, ok := request.Payload["containerId"].(string); ok {
		payload["containerId"] = v
	}

	return h.bridgeCall(ctx, request, OperationUpdate, payload)
}

func (h *ContactsHandler) handleDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	payload := map[string]any{
		"contactId": contactID,
	}

	return h.bridgeCall(ctx, request, OperationDelete, payload)
}

func (h *ContactsHandler) handleContainersList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	return h.bridgeCall(ctx, request, OperationContainersList, nil)
}

func (h *ContactsHandler) handleGroupsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	payload := map[string]any{}
	if containerID, ok := request.Payload["containerId"].(string); ok && containerID != "" {
		payload["containerId"] = containerID
	}

	return h.bridgeCall(ctx, request, OperationGroupsList, payload)
}

func (h *ContactsHandler) handlePhotoGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	payload := map[string]any{
		"contactId": contactID,
	}

	return h.bridgeCall(ctx, request, OperationPhotoGet, payload)
}

func (h *ContactsHandler) handlePhotoSet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	resourceURI, ok := request.Payload["resourceUri"].(string)
	if !ok || resourceURI == "" {
		return h.errorResponse(request, ErrPhotoInvalid, "missing required field: resourceUri")
	}

	payload := map[string]any{
		"contactId":   contactID,
		"resourceUri": resourceURI,
	}

	return h.bridgeCall(ctx, request, OperationPhotoSet, payload)
}

func (h *ContactsHandler) handlePhotoRemove(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	contactID, ok := request.Payload["contactId"].(string)
	if !ok || contactID == "" {
		return h.errorResponse(request, ErrInvalidContactID, "missing required field: contactId")
	}

	payload := map[string]any{
		"contactId": contactID,
	}

	return h.bridgeCall(ctx, request, OperationPhotoRemove, payload)
}

func (h *ContactsHandler) errorResponse(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:       code,
			Message:    message,
			DomainCode: "CONTACTS_DOMAIN",
		},
	}
}

func extractContactNameInput(payload map[string]any) ContactNameInput {
	name := ContactNameInput{}
	if v, ok := payload["name"].(map[string]any); ok {
		if s, ok := v["prefix"].(string); ok {
			name.Prefix = s
		}
		if s, ok := v["given"].(string); ok {
			name.Given = s
		}
		if s, ok := v["middle"].(string); ok {
			name.Middle = s
		}
		if s, ok := v["family"].(string); ok {
			name.Family = s
		}
		if s, ok := v["suffix"].(string); ok {
			name.Suffix = s
		}
		if s, ok := v["nickname"].(string); ok {
			name.Nickname = s
		}
		if s, ok := v["phoneticGiven"].(string); ok {
			name.PhoneticGiven = s
		}
		if s, ok := v["phoneticMiddle"].(string); ok {
			name.PhoneticMiddle = s
		}
		if s, ok := v["phoneticFamily"].(string); ok {
			name.PhoneticFamily = s
		}
	}
	return name
}

func hasAnyIdentity(payload map[string]any, name ContactNameInput) bool {
	if !name.IsEmpty() {
		return true
	}
	if org, ok := payload["organization"].(string); ok && org != "" {
		return true
	}
	if phones, ok := payload["phoneNumbers"].([]any); ok && len(phones) > 0 {
		return true
	}
	if emails, ok := payload["emailAddresses"].([]any); ok && len(emails) > 0 {
		return true
	}
	return false
}

func truncateLabeledStrings(items []any, max int) []any {
	if len(items) > max {
		return items[:max]
	}
	return items
}
