package uitree

import (
	"context"
	"sync"
	"time"
)

type SnapshotCache struct {
	mu         sync.RWMutex
	snapshots  map[string]*snapshotRecord
	policy     Policy
	counter    int64
}

func NewSnapshotCache(policy Policy) *SnapshotCache {
	return &SnapshotCache{
		snapshots: make(map[string]*snapshotRecord),
		policy:    policy,
	}
}

func (c *SnapshotCache) Put(snapshot UITreeSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.snapshots) >= c.policy.MaxSnapshots {
		var oldestKey string
		var oldestTime int64
		for k, v := range c.snapshots {
			if oldestTime == 0 || v.createdAt < oldestTime {
				oldestTime = v.createdAt
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(c.snapshots, oldestKey)
		}
	}

	nodeIndex := make(map[string]int, len(snapshot.Nodes))
	for i, node := range snapshot.Nodes {
		nodeIndex[node.NodeID] = i
	}

	counter := time.Now().UnixNano()
	c.snapshots[snapshot.SnapshotID] = &snapshotRecord{
		snapshot:   snapshot,
		createdAt:  counter,
		accessedAt: counter,
		nodeIndex:  nodeIndex,
	}
}

func (c *SnapshotCache) Get(snapshotID string) (UITreeSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.snapshots[snapshotID]
	if !ok {
		return UITreeSnapshot{}, false
	}

	age := time.Since(time.Unix(0, record.createdAt))
	if age > c.policy.SnapshotTTL {
		return UITreeSnapshot{}, false
	}

	return record.snapshot, true
}

func (c *SnapshotCache) GetNode(snapshotID string, nodeID string) (UINode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, ok := c.snapshots[snapshotID]
	if !ok {
		return UINode{}, false
	}

	age := time.Since(time.Unix(0, record.createdAt))
	if age > c.policy.SnapshotTTL {
		return UINode{}, false
	}

	idx, ok := record.nodeIndex[nodeID]
	if !ok {
		return UINode{}, false
	}

	if idx < 0 || idx >= len(record.snapshot.Nodes) {
		return UINode{}, false
	}

	return record.snapshot.Nodes[idx], true
}

func (c *SnapshotCache) Latest() (UITreeSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var latest *snapshotRecord
	latestCreatedAt := int64(0)
	for _, record := range c.snapshots {
		age := time.Since(time.Unix(0, record.createdAt))
		if age > c.policy.SnapshotTTL {
			continue
		}
		if latest == nil || record.createdAt > latestCreatedAt {
			latest = record
			latestCreatedAt = record.createdAt
		}
	}

	if latest == nil {
		return UITreeSnapshot{}, false
	}

	return latest.snapshot, true
}

func (c *SnapshotCache) Invalidate(snapshotID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.snapshots, snapshotID)
}

func (c *SnapshotCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshots = make(map[string]*snapshotRecord)
}

func (c *SnapshotCache) SnapshotResolver() SnapshotResolver {
	return &cacheSnapshotResolver{cache: c}
}

func (c *SnapshotCache) NodeResolver() NodeResolver {
	return &cacheNodeResolver{cache: c}
}

type cacheSnapshotResolver struct {
	cache *SnapshotCache
}

func (r *cacheSnapshotResolver) Latest(ctx context.Context) (UITreeSnapshot, error) {
	snapshot, ok := r.cache.Latest()
	if !ok {
		return UITreeSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_NOT_FOUND, Message: "no valid snapshot available"}
	}
	return snapshot, nil
}

func (r *cacheSnapshotResolver) GetSnapshot(ctx context.Context, snapshotID string) (UITreeSnapshot, error) {
	snapshot, ok := r.cache.Get(snapshotID)
	if !ok {
		return UITreeSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_NOT_FOUND, Message: "snapshot not found or expired"}
	}
	return snapshot, nil
}

type cacheNodeResolver struct {
	cache *SnapshotCache
}

func (r *cacheNodeResolver) ResolveNode(ctx context.Context, snapshotID string, nodeID string) (ResolvedUINode, error) {
	snapshot, ok := r.cache.Get(snapshotID)
	if !ok {
		return ResolvedUINode{}, &Error{Code: UI_TREE_SNAPSHOT_NOT_FOUND, Message: "snapshot not found or expired"}
	}

	node, ok := r.cache.GetNode(snapshotID, nodeID)
	if !ok {
		return ResolvedUINode{}, &Error{Code: UI_NODE_NOT_FOUND, Message: "node not found in snapshot"}
	}

	return ResolvedUINode{
		SnapshotID: snapshot.SnapshotID,
		Generation: snapshot.Generation,
		Node:       node,
		Source:     SourceType(snapshot.Source),
		NativeRef:  node.SourceRef,
	}, nil
}

type Service struct {
	sources SourceSet
	policy  Policy
	cache   *SnapshotCache
}

func NewService(sources SourceSet, policy Policy) *Service {
	return &Service{
		sources: sources,
		policy:  policy,
		cache:   NewSnapshotCache(policy),
	}
}

func (s *Service) Status(ctx context.Context) StatusResult {
	result := StatusResult{
		AvailableSources: []string{},
	}

	if s.sources.Accessibility != nil {
		status := s.sources.Accessibility.Status(ctx)
		result.AccessibilityReady = status.Available
		if status.Available {
			result.AvailableSources = append(result.AvailableSources, string(SourceTypeAccessibility))
		}
	}

	if s.sources.Root != nil {
		status := s.sources.Root.Status(ctx)
		result.RootAvailable = status.Available
		if status.Available {
			result.AvailableSources = append(result.AvailableSources, string(SourceTypeRoot))
		}
	}

	if s.sources.ADB != nil {
		status := s.sources.ADB.Status(ctx)
		result.ADBAvailable = status.Available
		if status.Available {
			result.AvailableSources = append(result.AvailableSources, string(SourceTypeADB))
		}
	}

	result.Available = len(result.AvailableSources) > 0
	result.PreferredSource = s.preferredSource(ctx)

	return result
}

func (s *Service) preferredSource(ctx context.Context) string {
	if s.sources.Accessibility != nil {
		status := s.sources.Accessibility.Status(ctx)
		if status.Available {
			return string(SourceTypeAccessibility)
		}
	}
	if s.sources.ADB != nil {
		status := s.sources.ADB.Status(ctx)
		if status.Available {
			return string(SourceTypeADB)
		}
	}
	if s.sources.Root != nil {
		status := s.sources.Root.Status(ctx)
		if status.Available {
			return string(SourceTypeRoot)
		}
	}
	return ""
}

func (s *Service) Snapshot(ctx context.Context, req SnapshotRequest) (UITreeSnapshot, error) {
	source, sourceType, err := s.sources.SelectSource(req, req.AllowRootFallback)
	if err != nil {
		return UITreeSnapshot{}, err
	}

	rawSnapshot, err := source.Snapshot(ctx, req)
	if err != nil {
		return UITreeSnapshot{}, err
	}

	snapshot := s.buildSnapshot(rawSnapshot, sourceType)

	s.cache.Put(snapshot)

	return snapshot, nil
}

func (s *Service) Find(ctx context.Context, req FindRequest) (FindResult, error) {
	var snapshot UITreeSnapshot
	if req.SnapshotID != "" {
		var ok bool
		snapshot, ok = s.cache.Get(req.SnapshotID)
		if !ok {
			return FindResult{}, &Error{Code: UI_TREE_SNAPSHOT_NOT_FOUND, Message: "snapshot not found or expired"}
		}
	} else {
		var err error
		snapshot, err = s.cache.SnapshotResolver().Latest(ctx)
		if err != nil {
			return FindResult{}, err
		}
	}

	nodeIDs := FilterNodes(snapshot.Nodes, req)

	return FindResult{
		SnapshotID: snapshot.SnapshotID,
		NodeIDs:    nodeIDs,
		Count:      len(nodeIDs),
	}, nil
}

func (s *Service) Get(ctx context.Context, req GetRequest) (GetResult, error) {
	snapshot, ok := s.cache.Get(req.SnapshotID)
	if !ok {
		return GetResult{}, &Error{Code: UI_TREE_SNAPSHOT_NOT_FOUND, Message: "snapshot not found or expired"}
	}

	node, ok := s.cache.GetNode(req.SnapshotID, req.NodeID)
	if !ok {
		return GetResult{}, &Error{Code: UI_NODE_NOT_FOUND, Message: "node not found in snapshot"}
	}

	return GetResult{
		SnapshotID: snapshot.SnapshotID,
		Generation: snapshot.Generation,
		Source:     snapshot.Source,
		Node:       node,
	}, nil
}

func (s *Service) NodeResolver() NodeResolver {
	return s.cache.NodeResolver()
}

func (s *Service) SnapshotResolver() SnapshotResolver {
	return s.cache.SnapshotResolver()
}

func (s *Service) buildSnapshot(raw RawSnapshot, sourceType SourceType) UITreeSnapshot {
	snapshot := UITreeSnapshot{
		Source:    string(sourceType),
		Generation: raw.Generation,
		CapturedAt: raw.CapturedAt,
		Truncated: raw.Truncated,
	}

	snapshot.Capability = UITreeCapabilityState{
		Available:           true,
		Source:              string(sourceType),
		MultiWindow:         raw.MultiWindow,
		StableNodeReference: raw.StableRef,
		CanReadText:         true,
		CanReadActions:      true,
		SupportsFind:        true,
	}

	if raw.CapturedAt == 0 {
		snapshot.CapturedAt = time.Now().UnixMilli()
	}

	t := time.Now()
	snapshot.SnapshotID = GenerateSnapshotID(raw.Generation, t.UnixNano())

	nodeIDSet := make(map[string]*UINode)
	for _, rawNode := range raw.RawNodes {
		node := s.mapRawNode(rawNode, sourceType, snapshot.SnapshotID)
		if _, exists := nodeIDSet[node.NodeID]; exists {
			continue
		}
		nodeIDSet[node.NodeID] = &node
		snapshot.Nodes = append(snapshot.Nodes, node)
		if len(snapshot.Nodes) >= s.policy.MaxNodes {
			snapshot.Truncated = true
			break
		}
	}

	for _, rawWindow := range raw.RawWindows {
		window := s.mapRawWindow(rawWindow, sourceType, snapshot.SnapshotID)
		snapshot.Windows = append(snapshot.Windows, window)
	}

	snapshot.NodeCount = len(snapshot.Nodes)

	for i, node := range snapshot.Nodes {
		if node.ParentID != "" {
			if parent, ok := nodeIDSet[node.ParentID]; ok {
				parent.ChildIDs = append(parent.ChildIDs, snapshot.Nodes[i].NodeID)
			}
		}
	}

	for i := range snapshot.Nodes {
		RedactSensitiveNode(&snapshot.Nodes[i],
			s.policy.MaxNodeTextRunes,
			s.policy.MaxDescriptionRunes,
			s.policy.MaxResourceIDRunes,
			s.policy.MaxClassNameRunes)
	}

	return snapshot
}

func (s *Service) mapRawNode(raw map[string]any, sourceType SourceType, snapshotID string) UINode {
	node := UINode{
		Bounds:             Rect{},
		Actions:            nil,
		ChildIDs:           nil,
	}

	if v, ok := raw["nodeId"].(string); ok {
		node.NodeID = v
	}
	if v, ok := raw["parentId"].(string); ok {
		node.ParentID = v
	}
	if v, ok := raw["windowId"].(string); ok {
		node.WindowID = v
	}
	if v, ok := raw["className"].(string); ok {
		node.ClassName = v
	}
	if v, ok := raw["packageName"].(string); ok {
		node.PackageName = v
	}
	if v, ok := raw["text"].(string); ok {
		node.Text = v
	}
	if v, ok := raw["contentDescription"].(string); ok {
		node.ContentDescription = v
	}
	if v, ok := raw["resourceId"].(string); ok {
		node.ResourceID = v
	}
	if v, ok := raw["sourceRef"].(string); ok {
		node.SourceRef = v
	}

	if v, ok := raw["left"].(float64); ok {
		node.Bounds.Left = int(v)
	}
	if v, ok := raw["top"].(float64); ok {
		node.Bounds.Top = int(v)
	}
	if v, ok := raw["right"].(float64); ok {
		node.Bounds.Right = int(v)
	}
	if v, ok := raw["bottom"].(float64); ok {
		node.Bounds.Bottom = int(v)
	}

	if v, ok := raw["visibleToUser"].(bool); ok {
		node.VisibleToUser = v
	}
	if v, ok := raw["enabled"].(bool); ok {
		node.Enabled = v
	}
	if v, ok := raw["focusable"].(bool); ok {
		node.Focusable = v
	}
	if v, ok := raw["focused"].(bool); ok {
		node.Focused = v
	}
	if v, ok := raw["selected"].(bool); ok {
		node.Selected = v
	}
	if v, ok := raw["checked"].(bool); ok {
		node.Checked = v
	}
	if v, ok := raw["checkable"].(bool); ok {
		node.Checkable = v
	}
	if v, ok := raw["clickable"].(bool); ok {
		node.Clickable = v
	}
	if v, ok := raw["longClickable"].(bool); ok {
		node.LongClickable = v
	}
	if v, ok := raw["scrollable"].(bool); ok {
		node.Scrollable = v
	}
	if v, ok := raw["editable"].(bool); ok {
		node.Editable = v
	}
	if v, ok := raw["password"].(bool); ok {
		node.Password = v
	}
	if v, ok := raw["depth"].(float64); ok {
		node.Depth = int(v)
	}

	if actions, ok := raw["actions"].([]any); ok {
		rawActions := make([]string, 0, len(actions))
		for _, a := range actions {
			if as, ok := a.(string); ok {
				rawActions = append(rawActions, as)
			}
		}
		node.Actions = MapActions(rawActions)
	}

	node.Role = MapClassToRole(node.ClassName, node.Editable, node.Clickable, node.Checkable)

	if node.NodeID == "" {
		node.NodeID = GenerateNodeID(snapshotID, node.WindowID, node.SourceRef, node.ResourceID, node.ClassName, node.Bounds, node.Depth)
	}

	return node
}

func (s *Service) mapRawWindow(raw map[string]any, sourceType SourceType, snapshotID string) UIWindow {
	window := UIWindow{
		Bounds: Rect{},
	}

	if v, ok := raw["windowId"].(string); ok {
		window.WindowID = v
	}
	if v, ok := raw["type"].(string); ok {
		window.Type = WindowType(v)
	}
	if v, ok := raw["packageName"].(string); ok {
		window.PackageName = v
	}
	if v, ok := raw["title"].(string); ok {
		window.Title = v
	}
	if v, ok := raw["active"].(bool); ok {
		window.Active = v
	}
	if v, ok := raw["focused"].(bool); ok {
		window.Focused = v
	}
	if v, ok := raw["displayId"].(float64); ok {
		window.DisplayID = int(v)
	}
	if v, ok := raw["rootNodeId"].(string); ok {
		window.RootNodeID = v
	}

	if v, ok := raw["left"].(float64); ok {
		window.Bounds.Left = int(v)
	}
	if v, ok := raw["top"].(float64); ok {
		window.Bounds.Top = int(v)
	}
	if v, ok := raw["right"].(float64); ok {
		window.Bounds.Right = int(v)
	}
	if v, ok := raw["bottom"].(float64); ok {
		window.Bounds.Bottom = int(v)
	}

	if window.WindowID == "" && window.PackageName != "" {
		window.WindowID = GenerateWindowID(string(sourceType)+"_"+window.PackageName, window.Type, window.PackageName)
	}

	return window
}
