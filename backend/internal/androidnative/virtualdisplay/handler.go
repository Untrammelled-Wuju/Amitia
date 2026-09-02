package virtualdisplay

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	OperationStatus  = "virtual_display.status"
	OperationCreate  = "virtual_display.create"
	OperationGet     = "virtual_display.get"
	OperationList    = "virtual_display.list"
	OperationResize  = "virtual_display.resize"
	OperationRelease = "virtual_display.release"
)

const (
	PermissionVirtualDisplayInspect = "android.virtual_display.inspect"
	PermissionVirtualDisplayManage  = "android.virtual_display.manage"
)

type Service struct {
	store    *Store
	bridge   VirtualBridge
	policy   Policy
	resolver DisplayTargetResolver
}

func NewService(store *Store, bridge VirtualBridge, policy Policy, resolver DisplayTargetResolver) *Service {
	return &Service{
		store:    store,
		bridge:   bridge,
		policy:   policy,
		resolver: resolver,
	}
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationCreate:
		return h.handleCreate(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	case OperationList:
		return h.handleList(ctx, request)
	case OperationResize:
		return h.handleResize(ctx, request)
	case OperationRelease:
		return h.handleRelease(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unsupported virtual display operation: " + request.Operation,
			},
		}
	}
}

func (h *Handler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	result := h.service.Status(ctx)
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"supported":                 result.Supported,
			"featureSecondaryDisplays":  result.FeatureSecondaryDisplays,
			"canCreate":                 result.CanCreate,
			"active":                    result.Active,
			"activeCount":               result.ActiveCount,
			"display":                   result.Display,
			"displays":                  result.Displays,
			"frameSourceSupported":      result.FrameSourceSupported,
			"uiTreeSupported":           result.UITreeSupported,
			"gestureSupported":          result.GestureSupported,
			"thirdPartyLaunchSupported": result.ThirdPartyLaunchSupported,
			"state":                     result.State,
			"reason":                    result.Reason,
		},
	}
}

func (h *Handler) handleCreate(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	var req CreateRequest
	if request.Payload != nil {
		b, err := json.Marshal(request.Payload)
		if err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid payload")
		}
		if err := json.Unmarshal(b, &req); err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid create payload")
		}
	}
	result, err := h.service.Create(ctx, req)
	if err != nil {
		if ve, ok := err.(*Error); ok {
			return errorResponse(request, ve.Code, ve.Message)
		}
		return errorResponse(request, ErrVirtualDisplayCreate, err.Error())
	}
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"display":                   result.Display,
			"frameSourceReady":          result.FrameSourceReady,
			"thirdPartyLaunchSupported": result.ThirdPartyLaunchSupported,
			"uiTreeSupported":           result.UITreeSupported,
			"gestureSupported":          result.GestureSupported,
		},
	}
}

func (h *Handler) handleGet(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	var req GetRequest
	if request.Payload != nil {
		b, err := json.Marshal(request.Payload)
		if err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid payload")
		}
		if err := json.Unmarshal(b, &req); err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid get payload")
		}
	}
	result, err := h.service.Get(ctx, req)
	if err != nil {
		if ve, ok := err.(*Error); ok {
			return errorResponse(request, ve.Code, ve.Message)
		}
		return errorResponse(request, ErrVirtualDisplayNotFound, err.Error())
	}
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"display": result,
		},
	}
}

func (h *Handler) handleList(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	displays := h.service.List(ctx)
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"displays": displays,
			"count":    len(displays),
		},
	}
}

func (h *Handler) handleResize(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	var req ResizeRequest
	if request.Payload != nil {
		b, err := json.Marshal(request.Payload)
		if err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid payload")
		}
		if err := json.Unmarshal(b, &req); err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid resize payload")
		}
	}
	result, err := h.service.Resize(ctx, req)
	if err != nil {
		if ve, ok := err.(*Error); ok {
			return errorResponse(request, ve.Code, ve.Message)
		}
		return errorResponse(request, ErrVirtualDisplayResize, err.Error())
	}
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"display": result,
		},
	}
}

func (h *Handler) handleRelease(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return errorResponse(request, ErrVirtualDisplayUnavailable, "service not initialized")
	}
	var req ReleaseRequest
	if request.Payload != nil {
		b, err := json.Marshal(request.Payload)
		if err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid payload")
		}
		if err := json.Unmarshal(b, &req); err != nil {
			return errorResponse(request, ErrVirtualDisplayProperty, "invalid release payload")
		}
	}
	result, err := h.service.Release(ctx, req)
	if err != nil {
		if ve, ok := err.(*Error); ok {
			return errorResponse(request, ve.Code, ve.Message)
		}
		return errorResponse(request, ErrVirtualDisplayNative, err.Error())
	}
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"released":  result.Released,
			"wasActive": result.WasActive,
		},
	}
}

func errorResponse(request capability.AndroidBridgeRequest, code, message string) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &capability.AndroidError{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Service) Status(ctx context.Context) StatusResult {
	result := StatusResult{
		Supported:                 s.bridge != nil,
		FeatureSecondaryDisplays:  s.bridge != nil,
		CanCreate:                 s.bridge != nil,
		FrameSourceSupported:      s.bridge != nil,
		UITreeSupported:           s.bridge != nil,
		GestureSupported:          s.bridge != nil,
		ThirdPartyLaunchSupported: s.bridge != nil,
		State:                     "unavailable",
		Reason:                    "native bridge not configured",
	}
	if s.bridge == nil {
		return result
	}

	if native, err := s.bridge.Execute(ctx, OperationStatus, map[string]any{}); err == nil {
		if supported, ok := native["supported"].(bool); ok {
			result.Supported = supported
		}
		if canCreate, ok := native["canCreate"].(bool); ok {
			result.CanCreate = canCreate
		}
		if state, ok := native["state"].(string); ok && state != "" {
			result.State = state
		}
		if reason, ok := native["reason"].(string); ok {
			result.Reason = reason
		} else {
			result.Reason = ""
		}
	} else if err != nil {
		result.State = "failed"
		result.Reason = err.Error()
		result.CanCreate = false
	}

	result.Displays = s.List(ctx)
	result.ActiveCount = len(result.Displays)
	result.Active = result.ActiveCount > 0
	if result.Active {
		latest := s.store.Get()
		if latest != nil {
			info := recordToInfo(latest)
			result.Display = &info
		}
		if result.State == "unavailable" || result.State == "none" {
			result.State = "ready"
		}
	}
	return result
}

func (s *Service) List(ctx context.Context) []VirtualDisplayInfo {
	records := s.store.List()
	out := make([]VirtualDisplayInfo, 0, len(records))
	for i := range records {
		out = append(out, recordToInfo(&records[i]))
	}
	return out
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	if s.bridge == nil {
		return nil, NewError(ErrVirtualDisplayUnavailable, "native bridge not configured; refusing to fabricate a display ID")
	}
	p := s.policy
	if req.Width == 0 {
		req.Width = p.DefaultWidth
	}
	if req.Height == 0 {
		req.Height = p.DefaultHeight
	}
	if req.DensityDPI == 0 {
		req.DensityDPI = p.DefaultDensityDPI
	}
	if err := p.ValidateSize(req.Width, req.Height); err != nil {
		return nil, err
	}
	if err := p.ValidateDensity(req.DensityDPI); err != nil {
		return nil, err
	}
	if err := p.ValidateRefreshRate(req.RefreshRate); err != nil {
		return nil, err
	}
	bridgeReq := map[string]any{
		"name":        "amitia_virtual",
		"width":       req.Width,
		"height":      req.Height,
		"densityDpi":  req.DensityDPI,
		"refreshRate": req.RefreshRate,
	}
	result, err := s.bridge.Execute(ctx, OperationCreate, bridgeReq)
	if err != nil {
		return nil, err
	}
	displayID := numberAsInt(result["displayId"])
	if displayID < 0 {
		return nil, NewError(ErrVirtualDisplayCreate, "native layer did not return a real display ID")
	}
	rec := &VirtualDisplayRecord{
		DisplayID:       displayID,
		Name:            stringValue(result["name"], "amitia_virtual"),
		Width:           positiveInt(result["width"], req.Width),
		Height:          positiveInt(result["height"], req.Height),
		DensityDPI:      positiveInt(result["densityDpi"], req.DensityDPI),
		RefreshRate:     req.RefreshRate,
		SurfaceAttached: boolValue(result["surfaceAttached"], true),
		State:           StateReady,
		CreatedAt:       time.Now(),
	}
	stored := s.store.Insert(rec)
	info := recordToInfo(stored)
	return &CreateResult{
		Display:                   info,
		FrameSourceReady:          rec.SurfaceAttached,
		ThirdPartyLaunchSupported: true,
		UITreeSupported:           true,
		GestureSupported:          true,
	}, nil
}

func (s *Service) Get(ctx context.Context, req GetRequest) (*VirtualDisplayInfo, error) {
	rec := s.store.GetByRef(req.Ref)
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "virtual display not found")
	}
	info := recordToInfo(rec)
	return &info, nil
}

func (s *Service) Resize(ctx context.Context, req ResizeRequest) (*VirtualDisplayInfo, error) {
	rec := s.store.GetByRef(req.Ref)
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "virtual display not found")
	}
	if s.bridge == nil {
		return nil, NewError(ErrVirtualDisplayUnavailable, "native bridge not configured")
	}
	p := s.policy
	w, h := p.ClampSize(req.Width, req.Height)
	dpi := p.ClampDensity(req.DensityDPI)
	bridgeReq := map[string]any{
		"displayId":  rec.DisplayID,
		"width":      w,
		"height":     h,
		"densityDpi": dpi,
	}
	if _, err := s.bridge.Execute(ctx, OperationResize, bridgeReq); err != nil {
		return nil, err
	}
	if err := s.store.Update(rec.Ref, func(r *VirtualDisplayRecord) error {
		r.Width = w
		r.Height = h
		r.DensityDPI = dpi
		r.Generation++
		return nil
	}); err != nil {
		return nil, err
	}
	rec = s.store.GetByRef(rec.Ref)
	info := recordToInfo(rec)
	return &info, nil
}

func (s *Service) Release(ctx context.Context, req ReleaseRequest) (*ReleaseResult, error) {
	rec := s.store.GetByRef(req.Ref)
	if rec == nil {
		return &ReleaseResult{Released: false, WasActive: false, State: string(StateReleased), Status: "already_released"}, nil
	}
	if s.bridge == nil {
		return nil, NewError(ErrVirtualDisplayUnavailable, "native bridge not configured")
	}
	wasActive := rec.State.IsActive()
	if _, err := s.bridge.Execute(ctx, OperationRelease, map[string]any{"displayId": rec.DisplayID}); err != nil {
		return nil, err
	}
	if _, err := s.store.Remove(rec.Ref); err != nil {
		return nil, err
	}
	return &ReleaseResult{Released: true, WasActive: wasActive, State: string(StateReleased), Status: "released"}, nil
}

func numberAsInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return -1
	}
}

func positiveInt(v any, fallback int) int {
	n := numberAsInt(v)
	if n <= 0 {
		return fallback
	}
	return n
}

func stringValue(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

func boolValue(v any, fallback bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

func recordToInfo(rec *VirtualDisplayRecord) VirtualDisplayInfo {
	return VirtualDisplayInfo{
		Ref:             rec.Ref,
		DisplayID:       rec.DisplayID,
		Generation:      rec.Generation,
		Name:            rec.Name,
		Width:           rec.Width,
		Height:          rec.Height,
		DensityDPI:      rec.DensityDPI,
		Rotation:        rec.Rotation,
		RefreshRate:     rec.RefreshRate,
		SurfaceAttached: rec.SurfaceAttached,
		State:           string(rec.State),
		CreatedAt:       rec.CreatedAt.UnixMilli(),
	}
}

var _ androidnative.Handler = (*Handler)(nil)
