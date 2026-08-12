package display

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	OperationStatus  = "display.status"
	OperationList    = "display.list"
	OperationGet     = "display.get"
	OperationResolve = "display.resolve"
)

type DisplayService struct {
	store       *DisplayStore
	classifier  *DisplayClassifier
	listener    *Listener
	resolver    *DefaultResolver
	topology    *TopologyAdapter
	policy      DisplaySelectionPolicy
	startedAt   time.Time
	eventLog    []DisplayEvent
	nativecap   DisplayCapability
}

type DisplayCapability interface {
	FetchDisplays(ctx context.Context) ([]DisplayInfo, error)
	FetchDisplay(ctx context.Context, displayID int) (DisplayInfo, error)
	AddExists(displayID int) bool
	NotifyDisplayAdded(displayID int)
	NotifyDisplayRemoved(displayID int)
	NotifyDisplayChanged(displayID int)
}

func NewDisplayService(
	store *DisplayStore,
	classifier *DisplayClassifier,
	listener *Listener,
	resolver *DefaultResolver,
	topology *TopologyAdapter,
	policy DisplaySelectionPolicy,
	capability DisplayCapability,
) *DisplayService {
	s := &DisplayService{
		store:      store,
		classifier: classifier,
		listener:   listener,
		resolver:   resolver,
		topology:   topology,
		policy:     policy,
		startedAt:  time.Now(),
		nativecap:  capability,
	}
	return s
}

func (s *DisplayService) Handle(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch req.Operation {
	case OperationStatus:
		return s.handleStatus(ctx, req)
	case OperationList:
		return s.handleList(ctx, req)
	case OperationGet:
		return s.handleGet(ctx, req)
	case OperationResolve:
		return s.handleResolve(ctx, req)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "display operation not supported: " + req.Operation,
			},
		}
	}
}

func (s *DisplayService) handleStatus(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	var reqData DisplayStatusRequest
	if len(req.Payload) > 0 {
		_ = decodePayloadDisplay(req.Payload, &reqData)
	}

	if err := s.refreshDisplays(ctx); err != nil {
		return errorResponseDisplay(req, ErrDisplayInvalidResponse, err.Error())
	}

	snapshot := s.store.Snapshot()
	presentationCount := 0
	managedCount := 0
	for _, d := range snapshot.Displays {
		if d.Presentation {
			presentationCount++
		}
		if d.ManagedByAmitia {
			managedCount++
		}
	}

	status := MultiDisplayStatus{
		Supported:                        true,
		DisplayCount:                     len(snapshot.Displays),
		DefaultDisplayID:                 snapshot.DefaultDisplayID,
		SecondaryDisplaySupported:        len(snapshot.Displays) > 1,
		PresentationDisplayCount:         presentationCount,
		ManagedVirtualDisplayCount:       managedCount,
		UITreeMultiDisplaySupported:      true,
		GestureMultiDisplaySupported:     true,
		ScreenshotMultiDisplaySupported:  true,
		ScreenFrameMultiDisplaySupported: true,
		TopologySupported:                s.topology.IsSupported(),
		Generation:                       s.store.GlobalGeneration(),
		State:                            "ready",
	}

	return successResponseDisplay(req, map[string]any{
		"supported":                          status.Supported,
		"displayCount":                       status.DisplayCount,
		"defaultDisplayId":                   status.DefaultDisplayID,
		"secondaryDisplaySupported":          status.SecondaryDisplaySupported,
		"presentationDisplayCount":           status.PresentationDisplayCount,
		"managedVirtualDisplayCount":         status.ManagedVirtualDisplayCount,
		"uiTreeMultiDisplaySupported":        status.UITreeMultiDisplaySupported,
		"gestureMultiDisplaySupported":       status.GestureMultiDisplaySupported,
		"screenshotMultiDisplaySupported":    status.ScreenshotMultiDisplaySupported,
		"screenFrameMultiDisplaySupported":   status.ScreenFrameMultiDisplaySupported,
		"topologySupported":                  status.TopologySupported,
		"generation":                         status.Generation,
		"state":                              status.State,
		"reason":                             status.Reason,
	})
}

func (s *DisplayService) handleList(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	var reqData DisplayListRequest
	if len(req.Payload) > 0 {
		_ = decodePayloadDisplay(req.Payload, &reqData)
	}

	if err := s.refreshDisplays(ctx); err != nil {
		return errorResponseDisplay(req, ErrDisplayInvalidResponse, err.Error())
	}

	snapshot := s.store.Snapshot()
	filtered := s.applyFilter(snapshot.Displays, reqData)

	return successResponseDisplay(req, map[string]any{
		"generation":       s.store.GlobalGeneration(),
		"defaultDisplayId": snapshot.DefaultDisplayID,
		"displays":         filtered,
		"capturedAt":       snapshot.CapturedAt,
	})
}

func (s *DisplayService) handleGet(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	var reqData DisplayGetRequest
	hasDisplayID := false
	if len(req.Payload) > 0 {
		_ = decodePayloadDisplay(req.Payload, &reqData)
		_, hasDisplayID = req.Payload["displayId"]
	}

	if err := s.refreshDisplays(ctx); err != nil {
		return errorResponseDisplay(req, ErrDisplayInvalidResponse, err.Error())
	}

	if hasDisplayID {
		rec, ok := s.store.Get(reqData.DisplayID)
		if !ok {
			return errorResponseDisplay(req, ErrDisplayNotFound, fmt.Sprintf("display %d not found", reqData.DisplayID))
		}
		return successResponseDisplay(req, map[string]any{
			"display": rec.Info,
		})
	}

	if reqData.Ref != "" {
		all := s.store.GetAll()
		for _, info := range all {
			if info.Ref == reqData.Ref {
				return successResponseDisplay(req, map[string]any{
					"display": info,
				})
			}
		}
		return errorResponseDisplay(req, ErrDisplayNotFound, "display not found for ref: "+reqData.Ref)
	}

	return errorResponseDisplay(req, ErrDisplayInvalidID, "either displayId or ref must be provided")
}

func (s *DisplayService) handleResolve(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	var reqData DisplayResolveRequest
	hasDisplayID := false
	if len(req.Payload) > 0 {
		_ = decodePayloadDisplay(req.Payload, &reqData)
		_, hasDisplayID = req.Payload["displayId"]
	}

	if err := s.refreshDisplays(ctx); err != nil {
		return errorResponseDisplay(req, ErrDisplayInvalidResponse, err.Error())
	}

	resolveReq := DisplayResolveRequest{
		DisplayID:        -1,
		Ref:              reqData.Ref,
		VirtualRef:       reqData.VirtualRef,
		PreferredDisplay: reqData.PreferredDisplay,
	}
	if hasDisplayID {
		resolveReq.DisplayID = reqData.DisplayID
	}

	result, err := s.resolver.Resolve(ctx, resolveReq)
	if err != nil {
		if de, ok := err.(*Error); ok {
			return errorResponseDisplay(req, de.Code, de.Message)
		}
		return errorResponseDisplay(req, ErrDisplayInvalidResponse, err.Error())
	}

	return successResponseDisplay(req, map[string]any{
		"target":     result.Target,
		"fromCache":  result.FromCache,
		"generation": result.Generation,
		"found":      result.Found,
	})
}

func (s *DisplayService) refreshDisplays(ctx context.Context) error {
	if s.nativecap == nil {
		return nil
	}
	displays, err := s.nativecap.FetchDisplays(ctx)
	if err != nil {
		return err
	}
	for _, d := range displays {
		isNew, _ := s.store.Put(d)
		if isNew {
			s.nativecap.NotifyDisplayAdded(d.DisplayID)
		}
	}
	return nil
}

func (s *DisplayService) applyFilter(displays []DisplayInfo, filter DisplayListRequest) []DisplayInfo {
	includeAll := !filter.IncludeDefault && !filter.IncludeSecondary
	result := []DisplayInfo{}
	for _, d := range displays {
		if filter.Type != "" && d.Type != filter.Type {
			continue
		}
		if filter.PresentationOnly && !d.Presentation {
			continue
		}
		if filter.ManagedOnly && !d.ManagedByAmitia {
			continue
		}
		if filter.InteractiveOnly && !(d.UITreeSupported || d.GestureSupported || d.ScreenshotSupported) {
			continue
		}
		if !includeAll {
			if d.IsDefault && !filter.IncludeDefault {
				continue
			}
			if !d.IsDefault && !filter.IncludeSecondary {
				continue
			}
		}
		result = append(result, d)
	}
	return result
}

func (s *DisplayService) RecordDisplayEvent(evt DisplayEvent) {
	s.listener.Emit(evt)
	s.eventLog = append(s.eventLog, evt)
}

func (s *DisplayService) NotifyManagedVirtual(displayID int, ref *virtualdisplay.VirtualDisplayRef) {
	s.store.SetManagedVirtual(displayID, ref)
}

func (s *DisplayService) NotifyVirtualReleased(displayID int) {
	s.store.RemoveManagedVirtual(displayID)
}

func successResponseDisplay(req capability.AndroidBridgeRequest, result map[string]any) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
		Result:          result,
	}
}

func errorResponseDisplay(req capability.AndroidBridgeRequest, code, message string) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "error",
		Error: &capability.AndroidError{
			Code:    code,
			Message: message,
		},
	}
}

func decodePayloadDisplay(payload map[string]any, target interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
