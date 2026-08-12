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
			"supported":           result.Supported,
			"featureSecondaryDisplays": result.FeatureSecondaryDisplays,
			"canCreate":           result.CanCreate,
			"active":              result.Active,
			"display":             result.Display,
			"frameSourceSupported": result.FrameSourceSupported,
			"uiTreeSupported":     result.UITreeSupported,
			"gestureSupported":    result.GestureSupported,
			"thirdPartyLaunchSupported": result.ThirdPartyLaunchSupported,
			"state":               result.State,
			"reason":              result.Reason,
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
			"display":                 result.Display,
			"frameSourceReady":        result.FrameSourceReady,
			"thirdPartyLaunchSupported": result.ThirdPartyLaunchSupported,
			"uiTreeSupported":         result.UITreeSupported,
			"visualTapSupported":       result.VisualTapSupported,
			"keyboardInjectSupported": result.KeyboardInjectSupported,
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
	rec := s.store.Get()
	if rec == nil {
		return StatusResult{
			Supported:           true,
			FeatureSecondaryDisplays: true,
			CanCreate:           true,
			Active:              false,
			FrameSourceSupported: true,
			UITreeSupported:      true,
			GestureSupported:    true,
			ThirdPartyLaunchSupported: true,
			State:               "none",
			Reason:              "no active virtual display",
		}
	}
	info := recordToInfo(rec)
	return StatusResult{
		Supported:           true,
		FeatureSecondaryDisplays: true,
		CanCreate:           false,
		Active:              true,
		Display:             &info,
		FrameSourceSupported: true,
		UITreeSupported:      true,
		GestureSupported:    true,
		ThirdPartyLaunchSupported: true,
		State:               string(rec.State),
		Reason:              "",
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	if s.store.HasActive() {
		return nil, NewError(ErrVirtualDisplayAlreadyExists, "virtual display already exists")
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
		"width":      req.Width,
		"height":     req.Height,
		"densityDpi": req.DensityDPI,
		"refreshRate": req.RefreshRate,
		"flags": int64(0x00000001 | 0x00000004),
	}
	if s.bridge != nil {
		result, err := s.bridge.Execute(ctx, "create_virtual_display", bridgeReq)
		if err != nil {
			return nil, err
		}
		displayID, _ := result["displayId"].(float64)
		rec := &VirtualDisplayRecord{
			DisplayID:  int(displayID),
			Name:       "amitia_virtual",
			Width:      req.Width,
			Height:     req.Height,
			DensityDPI: req.DensityDPI,
			RefreshRate: req.RefreshRate,
			State:      StateReady,
			CreatedAt:  time.Now(),
		}
		s.store.Insert(rec)
		info := recordToInfo(rec)
		return &CreateResult{
			Display:        info,
			FrameSourceReady: true,
			ThirdPartyLaunchSupported: true,
			UITreeSupported:      true,
			VisualTapSupported:   true,
			KeyboardInjectSupported: true,
		}, nil
	}
	rec := &VirtualDisplayRecord{
		DisplayID:  9999,
		Name:       "amitia_virtual",
		Width:      req.Width,
		Height:     req.Height,
		DensityDPI: req.DensityDPI,
		State:      StateReady,
		CreatedAt:  time.Now(),
	}
	s.store.Insert(rec)
	info := recordToInfo(rec)
	return &CreateResult{
		Display:        info,
		FrameSourceReady: true,
		ThirdPartyLaunchSupported: true,
		UITreeSupported:      true,
		VisualTapSupported:   true,
		KeyboardInjectSupported: true,
	}, nil
}

func (s *Service) Get(ctx context.Context, req GetRequest) (*VirtualDisplayInfo, error) {
	rec := s.store.Get()
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "no active virtual display")
	}
	if !req.Ref.IsEmpty() && req.Ref != rec.Ref {
		return nil, NewError(ErrVirtualDisplayIdMismatch, "reference mismatch")
	}
	info := recordToInfo(rec)
	return &info, nil
}

func (s *Service) Resize(ctx context.Context, req ResizeRequest) (*VirtualDisplayInfo, error) {
	rec := s.store.Get()
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "no active virtual display")
	}
	if !req.Ref.IsEmpty() && req.Ref != rec.Ref {
		return nil, NewError(ErrVirtualDisplayIdMismatch, "reference mismatch")
	}
	p := s.policy
	w, h := p.ClampSize(req.Width, req.Height)
	dpi := p.ClampDensity(req.DensityDPI)
	if s.bridge != nil {
		bridgeReq := map[string]any{
			"ref":        req.Ref.String(),
			"width":      w,
			"height":     h,
			"densityDpi": dpi,
		}
		_, err := s.bridge.Execute(ctx, "resize_virtual_display", bridgeReq)
		if err != nil {
			return nil, err
		}
	}
	err := s.store.Update(req.Ref, func(r *VirtualDisplayRecord) error {
		r.Width = w
		r.Height = h
		r.DensityDPI = dpi
		r.Generation++
		return nil
	})
	if err != nil {
		return nil, err
	}
	rec = s.store.Get()
	info := recordToInfo(rec)
	return &info, nil
}

func (s *Service) Release(ctx context.Context, req ReleaseRequest) (*ReleaseResult, error) {
	rec := s.store.Get()
	if rec == nil {
		return &ReleaseResult{
			Released:  false,
			WasActive: false,
			State:     string(StateReleased),
			Status:    "already_released",
		}, nil
	}
	if !req.Ref.IsEmpty() && req.Ref != rec.Ref {
		return nil, NewError(ErrVirtualDisplayIdMismatch, "reference mismatch")
	}
	wasActive := rec.State.IsActive()
	if s.bridge != nil {
		bridgeReq := map[string]any{
			"ref": req.Ref.String(),
		}
		if _, err := s.bridge.Execute(ctx, "release_virtual_display", bridgeReq); err != nil {
			return nil, err
		}
	}
	removed, err := s.store.Remove(req.Ref)
	if err != nil {
		return nil, err
	}
	_ = removed
	return &ReleaseResult{
		Released:  true,
		WasActive: wasActive,
		State:     string(StateReleased),
		Status:    "released",
	}, nil
}

func recordToInfo(rec *VirtualDisplayRecord) VirtualDisplayInfo {
	return VirtualDisplayInfo{
		Ref:        rec.Ref,
		DisplayID:  rec.DisplayID,
		Generation: rec.Generation,
		Name:       rec.Name,
		Width:      rec.Width,
		Height:     rec.Height,
		DensityDPI: rec.DensityDPI,
		Rotation:   rec.Rotation,
		RefreshRate: rec.RefreshRate,
		SurfaceAttached: rec.SurfaceAttached,
		State:      string(rec.State),
		CreatedAt:  rec.CreatedAt.UnixMilli(),
	}
}

var _ androidnative.Handler = (*Handler)(nil)
