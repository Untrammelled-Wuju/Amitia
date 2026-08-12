package display

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

type DisplayTargetResolver interface {
	Resolve(ctx context.Context, req DisplayResolveRequest) (*DisplayResolveResult, error)
}

type DefaultResolver struct {
	mu     sync.RWMutex
	store  *DisplayStore
	policy DisplaySelectionPolicy
}

func NewDefaultResolver(store *DisplayStore, policy DisplaySelectionPolicy) *DefaultResolver {
	return &DefaultResolver{
		store:  store,
		policy: policy,
	}
}

func (r *DefaultResolver) Resolve(ctx context.Context, req DisplayResolveRequest) (*DisplayResolveResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if req.DisplayID >= 0 {
		return r.resolveByDisplayID(req.DisplayID)
	}

	if !isEmptyStringRef(req.VirtualRef) {
		return r.resolveByVirtualRef(req.VirtualRef)
	}

	if req.Ref != "" {
		return r.resolveByRef(req.Ref)
	}

	if r.policy.AllowDefaultFallback {
		return r.resolveByDisplayID(DefaultDisplayID)
	}

	if r.policy.RejectAmbiguous {
		return nil, NewError(ErrDisplayAmbiguous, "no target display specified and ambiguous")
	}

	return r.resolveByDisplayID(DefaultDisplayID)
}

func (r *DefaultResolver) resolveByDisplayID(displayID int) (*DisplayResolveResult, error) {
	rec, ok := r.store.Get(displayID)
	if !ok {
		return nil, NewError(ErrDisplayNotFound, fmt.Sprintf("display %d not found", displayID))
	}
	return r.buildResult(rec.Info, true), nil
}

func (r *DefaultResolver) resolveByVirtualRef(refStr string) (*DisplayResolveResult, error) {
	ref := virtualdisplay.VirtualDisplayRef(refStr)
	all := r.store.GetAll()
	for _, info := range all {
		if info.VirtualRef != nil && *info.VirtualRef == ref {
			rec, _ := r.store.Get(info.DisplayID)
			return r.buildResult(rec.Info, true), nil
		}
	}
	return nil, NewError(ErrDisplayNotFound, "managed virtual display not found for ref: "+refStr)
}

func (r *DefaultResolver) resolveByRef(ref string) (*DisplayResolveResult, error) {
	all := r.store.GetAll()
	for id, info := range all {
		if info.Ref == ref {
			rec, _ := r.store.Get(id)
			return r.buildResult(rec.Info, true), nil
		}
	}
	return nil, NewError(ErrDisplayNotFound, "display not found for ref: "+ref)
}

func (r *DefaultResolver) buildResult(info DisplayInfo, found bool) *DisplayResolveResult {
	return &DisplayResolveResult{
		Target: DisplayTarget{
			DisplayID:              info.DisplayID,
			Ref:                    info.Ref,
			Generation:             info.Generation,
			Type:                   info.Type,
			ManagedVirtualRef:      info.VirtualRef,
			Width:                  info.Width,
			Height:                 info.Height,
			DensityDPI:             info.DensityDPI,
			Rotation:               info.Rotation,
			State:                  info.State,
			CoordinateSpace:        info.CoordinateSpace,
			UITreeSupported:        info.UITreeSupported,
			GestureSupported:       info.GestureSupported,
			ScreenshotSupported:    info.ScreenshotSupported,
			ScreenFrameSupported:   info.ScreenFrameSupported,
			ActivityLaunchSupported: info.ActivityLaunchSupported,
		},
		FromCache:  true,
		Generation: info.Generation,
		Found:      found,
	}
}

func isEmptyStringRef(s string) bool {
	return s == ""
}

func (r *DefaultResolver) ValidateGeneration(displayID int, expectedGen uint64) error {
	rec, ok := r.store.Get(displayID)
	if !ok {
		return NewError(ErrDisplayRemoved, fmt.Sprintf("display %d removed", displayID))
	}
	if rec.Info.Generation != expectedGen {
		return NewError(ErrDisplayTargetStale, fmt.Sprintf("display %d generation mismatch: expected %d, got %d", displayID, expectedGen, rec.Info.Generation))
	}
	return nil
}
