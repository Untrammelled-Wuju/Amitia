package ui_ordering

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type ContributionID string
type SlotID string

type OrderingRule struct {
	Group     string           `json:"group,omitempty"`
	Priority  int              `json:"priority"`
	Before    []ContributionID `json:"before,omitempty"`
	After     []ContributionID `json:"after,omitempty"`
	Placement string           `json:"placement,omitempty"`
}

type ContributionEntry struct {
	ContributionID   ContributionID
	ExtensionID      string
	SlotID           SlotID
	Group            string
	Priority         int
	Before           []ContributionID
	After            []ContributionID
	HostReserved     bool
	HostReservedRank int
	Enabled          bool
	Effective        bool
	RuntimeReady     bool
	Generation       int64
	InstalledAt      time.Time
}

type UserLayoutPreference struct {
	UserPinned map[ContributionID]int  `json:"userPinned,omitempty"`
	UserHidden map[ContributionID]bool `json:"userHidden,omitempty"`
	UserOrder  map[ContributionID]int  `json:"userOrder,omitempty"`
	Collapsed  bool                    `json:"collapsed,omitempty"`
}

type SlotCapacity struct {
	SlotID            SlotID
	MaxVisible        int
	OverflowPolicy    string
	ReservedHostCount int
}

const (
	OverflowPolicyHide    = "hide"
	OverflowPolicyMenu    = "menu"
	OverflowPolicyScroll  = "scroll"
	OverflowPolicyReplace = "replace"
)

type OrderingEngine struct {
	mu              sync.RWMutex
	capacities      map[SlotID]*SlotCapacity
	reservedSlots   map[SlotID]map[ContributionID]int
	defaultCapacity int
}

func NewOrderingEngine() *OrderingEngine {
	return &OrderingEngine{
		capacities:      make(map[SlotID]*SlotCapacity),
		reservedSlots:   make(map[SlotID]map[ContributionID]int),
		defaultCapacity: 8,
	}
}

func (e *OrderingEngine) SetCapacity(cap *SlotCapacity) error {
	if cap == nil || cap.SlotID == "" {
		return ErrInvalidCapacity
	}
	if cap.MaxVisible <= 0 {
		cap.MaxVisible = e.defaultCapacity
	}
	if cap.OverflowPolicy == "" {
		cap.OverflowPolicy = OverflowPolicyHide
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.capacities[cap.SlotID] = cap
	return nil
}

func (e *OrderingEngine) GetCapacity(slotID SlotID) *SlotCapacity {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if cap, exists := e.capacities[slotID]; exists {
		return cap
	}
	return &SlotCapacity{
		SlotID:         slotID,
		MaxVisible:     e.defaultCapacity,
		OverflowPolicy: OverflowPolicyHide,
	}
}

func (e *OrderingEngine) MarkReserved(slotID SlotID, contributionID ContributionID, rank int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.reservedSlots[slotID]; !exists {
		e.reservedSlots[slotID] = make(map[ContributionID]int)
	}
	e.reservedSlots[slotID][contributionID] = rank
}

type SortInput struct {
	Contributions []*ContributionEntry
	Preferences   *UserLayoutPreference
	SlotID        SlotID
}

type SortResult struct {
	Visible   []*ContributionEntry
	Hidden    []*ContributionEntry
	Overflow  []*ContributionEntry
	Conflicts []ConflictRecord
}

type ConflictRecord struct {
	Type          string
	ContributionA ContributionID
	ContributionB ContributionID
	Reason        string
}

func (e *OrderingEngine) Sort(input SortInput) (*SortResult, error) {
	if input.Contributions == nil {
		return nil, ErrNilInput
	}
	e.mu.RLock()
	reserved := e.reservedSlots[input.SlotID]
	capacity := e.capacities[input.SlotID]
	e.mu.RUnlock()
	if capacity == nil {
		capacity = &SlotCapacity{
			SlotID:         input.SlotID,
			MaxVisible:     e.defaultCapacity,
			OverflowPolicy: OverflowPolicyHide,
		}
	}
	entries := make([]*ContributionEntry, 0, len(input.Contributions))
	for _, c := range input.Contributions {
		if c == nil {
			continue
		}
		if rank, isReserved := reserved[c.ContributionID]; isReserved {
			c.HostReserved = true
			c.HostReservedRank = rank
		}
		if input.Preferences != nil && input.Preferences.UserHidden[c.ContributionID] {
			continue
		}
		if !c.Enabled || !c.Effective {
			continue
		}
		entries = append(entries, c)
	}
	conflicts := e.detectConflicts(entries)
	graph := newDependencyGraph()
	for _, entry := range entries {
		graph.addNode(entry.ContributionID)
		for _, before := range entry.Before {
			graph.addEdge(entry.ContributionID, before)
		}
		for _, after := range entry.After {
			graph.addEdge(after, entry.ContributionID)
		}
	}
	topoOrder, cycleConflicts := graph.topologicalSort()
	conflicts = append(conflicts, cycleConflicts...)
	entryMap := make(map[ContributionID]*ContributionEntry, len(entries))
	for _, e := range entries {
		entryMap[e.ContributionID] = e
	}
	sortedEntries := make([]*ContributionEntry, 0, len(topoOrder))
	for _, id := range topoOrder {
		if entry, ok := entryMap[id]; ok {
			sortedEntries = append(sortedEntries, entry)
		}
	}
	sort.SliceStable(sortedEntries, func(i, j int) bool {
		return compareEntries(sortedEntries[i], sortedEntries[j], input.Preferences)
	})
	result := &SortResult{
		Conflicts: conflicts,
	}
	maxVisible := capacity.MaxVisible
	if len(sortedEntries) <= maxVisible {
		result.Visible = sortedEntries
		return result, nil
	}
	result.Visible = sortedEntries[:maxVisible]
	overflow := sortedEntries[maxVisible:]
	switch capacity.OverflowPolicy {
	case OverflowPolicyHide:
		result.Hidden = overflow
	case OverflowPolicyMenu, OverflowPolicyScroll, OverflowPolicyReplace:
		result.Overflow = overflow
	default:
		result.Hidden = overflow
	}
	return result, nil
}

func (e *OrderingEngine) detectConflicts(entries []*ContributionEntry) []ConflictRecord {
	conflicts := make([]ConflictRecord, 0)
	priorityMap := make(map[int][]ContributionID)
	for _, entry := range entries {
		priorityMap[entry.Priority] = append(priorityMap[entry.Priority], entry.ContributionID)
	}
	for _, ids := range priorityMap {
		if len(ids) > 1 {
			for i := 1; i < len(ids); i++ {
				conflicts = append(conflicts, ConflictRecord{
					Type:          "priority_tie",
					ContributionA: ids[0],
					ContributionB: ids[i],
					Reason:        fmt.Sprintf("priority tie resolved by extension_id"),
				})
			}
		}
	}
	return conflicts
}

func compareEntries(a, b *ContributionEntry, pref *UserLayoutPreference) bool {
	if a.HostReservedRank != b.HostReservedRank {
		return a.HostReservedRank < b.HostReservedRank
	}
	aPinned := getUserPinnedRank(a.ContributionID, pref)
	bPinned := getUserPinnedRank(b.ContributionID, pref)
	if aPinned != bPinned {
		if aPinned < 0 {
			return false
		}
		if bPinned < 0 {
			return true
		}
		return aPinned < bPinned
	}
	aOrder := getUserCustomOrder(a.ContributionID, pref)
	bOrder := getUserCustomOrder(b.ContributionID, pref)
	if aOrder != bOrder {
		if aOrder < 0 {
			return false
		}
		if bOrder < 0 {
			return true
		}
		return aOrder < bOrder
	}
	if a.Group != b.Group {
		return a.Group < b.Group
	}
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	if a.ExtensionID != b.ExtensionID {
		return a.ExtensionID < b.ExtensionID
	}
	return a.ContributionID < b.ContributionID
}

func getUserPinnedRank(id ContributionID, pref *UserLayoutPreference) int {
	if pref == nil || pref.UserPinned == nil {
		return -1
	}
	if rank, ok := pref.UserPinned[id]; ok {
		return rank
	}
	return -1
}

func getUserCustomOrder(id ContributionID, pref *UserLayoutPreference) int {
	if pref == nil || pref.UserOrder == nil {
		return -1
	}
	if order, ok := pref.UserOrder[id]; ok {
		return order
	}
	return -1
}

type dependencyGraph struct {
	nodes map[ContributionID]bool
	edges map[ContributionID][]ContributionID
	indeg map[ContributionID]int
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		nodes: make(map[ContributionID]bool),
		edges: make(map[ContributionID][]ContributionID),
		indeg: make(map[ContributionID]int),
	}
}

func (g *dependencyGraph) addNode(id ContributionID) {
	if !g.nodes[id] {
		g.nodes[id] = true
		g.indeg[id] = 0
	}
}

func (g *dependencyGraph) addEdge(from, to ContributionID) {
	g.addNode(from)
	g.addNode(to)
	g.edges[from] = append(g.edges[from], to)
	g.indeg[to]++
}

func (g *dependencyGraph) topologicalSort() ([]ContributionID, []ConflictRecord) {
	queue := make([]ContributionID, 0)
	for node := range g.nodes {
		if g.indeg[node] == 0 {
			queue = append(queue, node)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	result := make([]ContributionID, 0, len(g.nodes))
	conflicts := make([]ConflictRecord, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		nextNodes := make([]ContributionID, len(g.edges[node]))
		copy(nextNodes, g.edges[node])
		sort.Slice(nextNodes, func(i, j int) bool { return nextNodes[i] < nextNodes[j] })
		for _, next := range nextNodes {
			g.indeg[next]--
			if g.indeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(result) < len(g.nodes) {
		for node := range g.nodes {
			found := false
			for _, r := range result {
				if r == node {
					found = true
					break
				}
			}
			if !found {
				conflicts = append(conflicts, ConflictRecord{
					Type:   "cycle",
					Reason: fmt.Sprintf("cycle detected involving %s", node),
				})
			}
		}
		allNodes := make([]ContributionID, 0, len(g.nodes))
		for node := range g.nodes {
			allNodes = append(allNodes, node)
		}
		sort.Slice(allNodes, func(i, j int) bool { return allNodes[i] < allNodes[j] })
		for _, node := range allNodes {
			found := false
			for _, r := range result {
				if r == node {
					found = true
					break
				}
			}
			if !found {
				result = append(result, node)
			}
		}
	}
	return result, conflicts
}

type LayoutEngine struct {
	mu sync.RWMutex
}

type LayoutInput struct {
	SlotID   SlotID
	Entries  []*ContributionEntry
	Layout   string
	Capacity *SlotCapacity
	Width    int
	Density  string
	Platform string
}

type LayoutOutput struct {
	SlotID      SlotID
	Rows        [][]*ContributionEntry
	Overflow    []*ContributionEntry
	Hidden      []*ContributionEntry
	ColumnCount int
	Wrapped     bool
}

func NewLayoutEngine() *LayoutEngine {
	return &LayoutEngine{}
}

func (le *LayoutEngine) Layout(input LayoutInput) (*LayoutOutput, error) {
	if input.Entries == nil {
		return nil, ErrNilInput
	}
	le.mu.RLock()
	defer le.mu.RUnlock()
	output := &LayoutOutput{
		SlotID: input.SlotID,
	}
	visible := input.Entries
	if input.Capacity != nil && len(visible) > input.Capacity.MaxVisible {
		visible = visible[:input.Capacity.MaxVisible]
		output.Overflow = input.Entries[input.Capacity.MaxVisible:]
	}
	switch input.Layout {
	case "inline", "row":
		output.Rows = le.layoutRow(visible, input.Width)
		output.ColumnCount = 1
	case "stack":
		output.Rows = le.layoutStack(visible)
		output.ColumnCount = 1
	case "grid":
		cols := le.calculateGridColumns(input.Width, input.Density)
		output.ColumnCount = cols
		output.Rows = le.layoutGrid(visible, cols)
	case "tabs":
		output.Rows = [][]*ContributionEntry{visible}
		output.ColumnCount = len(visible)
	default:
		output.Rows = [][]*ContributionEntry{visible}
		output.ColumnCount = 1
	}
	if len(output.Rows) > 1 {
		output.Wrapped = true
	}
	return output, nil
}

func (le *LayoutEngine) layoutRow(entries []*ContributionEntry, width int) [][]*ContributionEntry {
	if width <= 0 {
		return [][]*ContributionEntry{entries}
	}
	rows := make([][]*ContributionEntry, 0)
	current := make([]*ContributionEntry, 0)
	currentWidth := 0
	approxItemWidth := 120
	for _, e := range entries {
		if currentWidth+approxItemWidth > width && len(current) > 0 {
			rows = append(rows, current)
			current = make([]*ContributionEntry, 0)
			currentWidth = 0
		}
		current = append(current, e)
		currentWidth += approxItemWidth
	}
	if len(current) > 0 {
		rows = append(rows, current)
	}
	return rows
}

func (le *LayoutEngine) layoutStack(entries []*ContributionEntry) [][]*ContributionEntry {
	rows := make([][]*ContributionEntry, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []*ContributionEntry{e})
	}
	return rows
}

func (le *LayoutEngine) layoutGrid(entries []*ContributionEntry, cols int) [][]*ContributionEntry {
	if cols <= 0 {
		cols = 1
	}
	rows := make([][]*ContributionEntry, 0)
	for i := 0; i < len(entries); i += cols {
		end := i + cols
		if end > len(entries) {
			end = len(entries)
		}
		rows = append(rows, entries[i:end])
	}
	return rows
}

func (le *LayoutEngine) calculateGridColumns(width int, density string) int {
	minColWidth := 280
	switch density {
	case "compact":
		minColWidth = 220
	case "comfortable":
		minColWidth = 320
	}
	if width <= 0 {
		return 1
	}
	cols := width / minColWidth
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}
	return cols
}

type DegradationPolicy struct {
	MaxConsecutiveFailures int
	CooldownDuration       time.Duration
	HideAfterFailures      int
}

type FailureTracker struct {
	mu           sync.RWMutex
	failureCount map[ContributionID]int
	lastFailure  map[ContributionID]time.Time
	degraded     map[ContributionID]bool
	policy       DegradationPolicy
}

func NewFailureTracker(policy DegradationPolicy) *FailureTracker {
	if policy.MaxConsecutiveFailures <= 0 {
		policy.MaxConsecutiveFailures = 5
	}
	if policy.CooldownDuration <= 0 {
		policy.CooldownDuration = 5 * time.Minute
	}
	if policy.HideAfterFailures <= 0 {
		policy.HideAfterFailures = 10
	}
	return &FailureTracker{
		failureCount: make(map[ContributionID]int),
		lastFailure:  make(map[ContributionID]time.Time),
		degraded:     make(map[ContributionID]bool),
		policy:       policy,
	}
}

func (f *FailureTracker) RecordFailure(id ContributionID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failureCount[id]++
	f.lastFailure[id] = time.Now().UTC()
	if f.failureCount[id] >= f.policy.HideAfterFailures {
		f.degraded[id] = true
		return true
	}
	if f.failureCount[id] >= f.policy.MaxConsecutiveFailures {
		f.degraded[id] = true
	}
	return f.degraded[id]
}

func (f *FailureTracker) RecordSuccess(id ContributionID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.failureCount, id)
	delete(f.lastFailure, id)
	delete(f.degraded, id)
}

func (f *FailureTracker) IsDegraded(id ContributionID) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.degraded[id] {
		return false
	}
	if last, ok := f.lastFailure[id]; ok {
		if time.Since(last) > f.policy.CooldownDuration {
			return false
		}
	}
	return true
}

func (f *FailureTracker) FilterDegraded(entries []*ContributionEntry) ([]*ContributionEntry, []*ContributionEntry) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	active := make([]*ContributionEntry, 0, len(entries))
	degraded := make([]*ContributionEntry, 0)
	for _, e := range entries {
		if f.degraded[e.ContributionID] {
			if last, ok := f.lastFailure[e.ContributionID]; ok {
				if time.Since(last) <= f.policy.CooldownDuration {
					degraded = append(degraded, e)
					continue
				}
			}
		}
		active = append(active, e)
	}
	return active, degraded
}

type LayoutPreferencesStore struct {
	mu    sync.RWMutex
	prefs map[SlotID]*UserLayoutPreference
}

func NewLayoutPreferencesStore() *LayoutPreferencesStore {
	return &LayoutPreferencesStore{
		prefs: make(map[SlotID]*UserLayoutPreference),
	}
}

func (s *LayoutPreferencesStore) Get(slotID SlotID) *UserLayoutPreference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pref, exists := s.prefs[slotID]; exists {
		return pref
	}
	return &UserLayoutPreference{
		UserPinned: make(map[ContributionID]int),
		UserHidden: make(map[ContributionID]bool),
		UserOrder:  make(map[ContributionID]int),
	}
}

func (s *LayoutPreferencesStore) Set(slotID SlotID, pref *UserLayoutPreference) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefs[slotID] = pref
}

func (s *LayoutPreferencesStore) Pin(slotID SlotID, id ContributionID, rank int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.prefs[slotID]; !exists {
		s.prefs[slotID] = &UserLayoutPreference{
			UserPinned: make(map[ContributionID]int),
			UserHidden: make(map[ContributionID]bool),
			UserOrder:  make(map[ContributionID]int),
		}
	}
	s.prefs[slotID].UserPinned[id] = rank
}

func (s *LayoutPreferencesStore) Hide(slotID SlotID, id ContributionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.prefs[slotID]; !exists {
		s.prefs[slotID] = &UserLayoutPreference{
			UserPinned: make(map[ContributionID]int),
			UserHidden: make(map[ContributionID]bool),
			UserOrder:  make(map[ContributionID]int),
		}
	}
	s.prefs[slotID].UserHidden[id] = true
}

func (s *LayoutPreferencesStore) Reorder(slotID SlotID, id ContributionID, order int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.prefs[slotID]; !exists {
		s.prefs[slotID] = &UserLayoutPreference{
			UserPinned: make(map[ContributionID]int),
			UserHidden: make(map[ContributionID]bool),
			UserOrder:  make(map[ContributionID]int),
		}
	}
	s.prefs[slotID].UserOrder[id] = order
}

func (s *LayoutPreferencesStore) Clear(slotID SlotID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.prefs, slotID)
}

var (
	ErrInvalidCapacity = errors.New("ui_ordering: invalid capacity")
	ErrNilInput        = errors.New("ui_ordering: nil input")
)

var _ = context.Background
var _ = strings.Contains
