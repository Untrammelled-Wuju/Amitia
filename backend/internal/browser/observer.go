package browser

import (
	"context"
	"strings"
	"sync"
)

type productionObserver struct {
	tabs     TabResolver
	dom      DOMBackend
	elements *elementStore
	policy   *InteractionPolicy
	tabMgr   *productionTabManager
	mu       sync.RWMutex
}

func NewProductionObserver(tabs TabResolver, dom DOMBackend, elements *elementStore, policy *InteractionPolicy, tabMgr *productionTabManager) BrowserObserver {
	return &productionObserver{
		tabs:     tabs,
		dom:      dom,
		elements: elements,
		policy:   policy,
		tabMgr:   tabMgr,
	}
}

func (o *productionObserver) GetDOMSnapshot(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, maxDepth int) (*BrowserDOMSnapshot, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	if tabID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	resolved, err := o.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	if err := o.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeDOMSnapshotFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	maxDepth = o.policy.NormalizeMaxDepth(maxDepth)

	document, domErr := o.dom.GetDocument(ctx, resolved.TargetID, maxDepth)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeDOMSnapshotFailed,
			Message: "failed to get document",
			Cause:   domErr,
		}
	}

	content := buildCompactDOM(document, maxDepth, 0, o.policy)
	nodeCount := countNodes(document)
	truncated := nodeCount > o.policy.MaxDOMNodes

	if o.tabMgr != nil {
		if record, ok := o.tabMgr.store.get(tabID); ok {
			return &BrowserDOMSnapshot{
				SessionID:          sessionID,
				TabID:              tabID,
				URL:                record.info.URL,
				Title:              record.info.Title,
				Content:            content,
				Truncated:          truncated,
				MaxDepth:           maxDepth,
				RuntimeGeneration:  resolved.RuntimeGeneration,
				DocumentGeneration: record.documentGeneration,
				NodeCount:          nodeCount,
			}, nil
		}
	}

	return &BrowserDOMSnapshot{
		SessionID:          sessionID,
		TabID:              tabID,
		Content:            content,
		Truncated:          truncated,
		MaxDepth:           maxDepth,
		RuntimeGeneration:  resolved.RuntimeGeneration,
		NodeCount:          nodeCount,
	}, nil
}

func (o *productionObserver) FindElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, selector string) (*BrowserElementRef, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "session ID is required",
		}
	}

	if tabID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "tab ID is required",
		}
	}

	if err := o.policy.ValidateSelector(selector); err != nil {
		return nil, err
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	resolved, err := o.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	if err := o.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeDOMSnapshotFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	document, domErr := o.dom.GetDocument(ctx, resolved.TargetID, 1)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeDOMSnapshotFailed,
			Message: "failed to get document",
			Cause:   domErr,
		}
	}

	nodeID, domErr := o.dom.QuerySelector(ctx, resolved.TargetID, document.NodeID, selector)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element not found: " + selector,
		}
	}

	described, domErr := o.dom.DescribeNode(ctx, resolved.TargetID, nodeID)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "failed to describe element",
			Cause:   domErr,
		}
	}

	var docGen uint64
	if o.tabMgr != nil {
		if record, ok := o.tabMgr.store.get(tabID); ok {
			docGen = record.documentGeneration
		}
	}

	stableID := o.elements.generateStableID()
	record := &elementRecord{
		stableID:           stableID,
		sessionID:          sessionID,
		tabID:              tabID,
		runtimeGeneration:  resolved.RuntimeGeneration,
		documentGeneration: docGen,
		targetID:           resolved.TargetID,
		frameID:            FrameID(described.FrameID),
		backendNodeID:      described.BackendNodeID,
		selector:           selector,
	}
	o.elements.put(record)

	return &BrowserElementRef{
		SessionID:          sessionID,
		TabID:              tabID,
		Selector:           selector,
		StableID:           stableID,
		RuntimeGeneration:  resolved.RuntimeGeneration,
		DocumentGeneration: docGen,
		FrameID:            described.FrameID,
	}, nil
}

func (o *productionObserver) ScrollToElement(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) *BrowserError {
	if err := ctx.Err(); err != nil {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if element.StableID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	resolved, err := o.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return err
	}

	record, ok := o.elements.get(element.StableID)
	if !ok {
		return &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID || record.tabID != tabID {
		return &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		o.elements.remove(element.StableID)
		return &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	if err := o.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	if err := o.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to scroll element into view",
			Cause:   err,
		}
	}

	_, domErr := o.dom.GetContentQuads(ctx, resolved.TargetID, record.backendNodeID)
	if domErr != nil {
		return &BrowserError{
			Code:    ErrCodeElementNotInteractable,
			Message: "element has no visible layout",
			Cause:   domErr,
		}
	}

	return nil
}

func buildCompactDOM(node *domNode, maxDepth, currentDepth int, policy *InteractionPolicy) string {
	if node == nil {
		return ""
	}
	if currentDepth >= maxDepth {
		return ""
	}

	var sb strings.Builder
	renderNode(&sb, node, currentDepth, 0, policy)
	return sb.String()
}

func renderNode(sb *strings.Builder, node *domNode, depth, indent int, policy *InteractionPolicy) {
	if node == nil || depth > policy.MaxDOMMaxDepth {
		return
	}

	nodeName := strings.ToLower(node.NodeName)
	if nodeName == "script" || nodeName == "style" || nodeName == "noscript" {
		return
	}

	for i := 0; i < indent; i++ {
		sb.WriteString("  ")
	}
	sb.WriteString(node.NodeName)

	if id, ok := node.Attributes["id"]; ok && id != "" {
		sb.WriteString("#")
		sb.WriteString(id)
	}
	if class, ok := node.Attributes["class"]; ok && class != "" {
		classes := strings.Fields(class)
		for _, c := range classes {
			if len(sb.String()) > policy.MaxTextBytesPerNode {
				break
			}
			sb.WriteString(".")
			sb.WriteString(c)
		}
	}

	if nodeValue := node.NodeValue; nodeValue != "" {
		if len(nodeValue) > policy.MaxTextBytesPerNode {
			nodeValue = nodeValue[:policy.MaxTextBytesPerNode] + "..."
		}
		sb.WriteString(" \"")
		sb.WriteString(nodeValue)
		sb.WriteString("\"")
	}
	sb.WriteString("\n")

	for _, child := range node.Children {
		renderNode(sb, child, depth+1, indent+1, policy)
	}
}

func countNodes(node *domNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}
