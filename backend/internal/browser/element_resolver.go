package browser

import (
	"context"
	"sync"
)

type productionElementResolver struct {
	tabs  TabResolver
	store *elementStore
	mu    sync.RWMutex
}

func NewProductionElementResolver(tabs TabResolver, store *elementStore) ElementResolver {
	return &productionElementResolver{
		tabs:  tabs,
		store: store,
	}
}

func (r *productionElementResolver) ResolveElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, ref BrowserElementRef) (ResolvedBrowserElement, *BrowserError) {
	if ref.StableID == "" {
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.store.get(ref.StableID)
	if !ok {
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID {
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to session",
		}
	}

	if record.tabID != tabID {
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to tab",
		}
	}

	resolved, err := r.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return ResolvedBrowserElement{}, err
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		r.store.remove(ref.StableID)
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	currentDocGen, ok := getDocumentGenerationFromStore(tabID)
	if ok && record.documentGeneration != currentDocGen {
		r.store.remove(ref.StableID)
		return ResolvedBrowserElement{}, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale document generation",
		}
	}

	r.store.touch(ref.StableID)

	return ResolvedBrowserElement{
		SessionID:          record.sessionID,
		TabID:              record.tabID,
		RuntimeGeneration:  record.runtimeGeneration,
		DocumentGeneration: record.documentGeneration,
		TargetID:           record.targetID,
		CDPSessionID:       record.cdpSessionID,
		FrameID:            record.frameID,
		BackendNodeID:      record.backendNodeID,
		Selector:           record.selector,
	}, nil
}

func getDocumentGenerationFromStore(tabID BrowserTabID) (uint64, bool) {
	return 0, false
}
