package dependency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var (
	ErrCycleDetected   = errors.New("dependency: cycle detected")
	ErrMissingRequired = errors.New("dependency: missing required dependency")
	ErrNoCandidate     = errors.New("dependency: no candidate available")
)

type DefaultResolver struct {
	mu              sync.RWMutex
	candidate       CandidateProvider
	graphCache      map[string]Graph
	snapshotCache   map[string]Snapshot
	affectedIndex   map[string][]AffectedSubject
	platform        string
	hostVersion     domain.SemanticVersion
	hostFeatures    map[string]bool
	dependencyGens  map[string]int64
}

func NewDefaultResolver(candidate CandidateProvider) *DefaultResolver {
	return &DefaultResolver{
		candidate:      candidate,
		graphCache:     make(map[string]Graph),
		snapshotCache:  make(map[string]Snapshot),
		affectedIndex:  make(map[string][]AffectedSubject),
		hostFeatures:   make(map[string]bool),
		dependencyGens: make(map[string]int64),
	}
}

func (r *DefaultResolver) SetPlatform(platform string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.platform = platform
}

func (r *DefaultResolver) SetHostVersion(v domain.SemanticVersion) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hostVersion = v
}

func (r *DefaultResolver) SetHostFeatures(features map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hostFeatures = features
}

func (r *DefaultResolver) Resolve(ctx context.Context, request ResolveRequest) ResolveResult {
	result := ResolveResult{
		SourceID:   request.SourceID,
		Phase:      request.Phase,
		Generation: r.nextGeneration(request.SourceID),
	}
	nodes := map[string]Node{request.SourceID: {ID: request.SourceID, Type: TargetExtension}}
	var edges []Edge
	for _, req := range request.Requests {
		req.Phase = request.Phase
		if req.Policy == "" {
			req.Policy = PolicyInstalledPreferred
		}
		if req.Scope == "" {
			req.Scope = ScopeShared
		}
		resolution := r.resolveSingle(ctx, req)
		result.Resolutions = append(result.Resolutions, resolution)
		result.Conflicts = append(result.Conflicts, resolution.Conflicts...)
		result.Warnings = append(result.Warnings, resolution.Warnings...)
		edges = append(edges, Edge{
			From:     request.SourceID,
			To:       req.Target,
			Phase:    request.Phase,
			Required: req.Required,
			Range:    req.VersionRange,
			Owner:    request.SourceID,
		})
		if resolution.Selected != nil {
			nodes[req.Target] = Node{ID: req.Target, Type: req.Type, Owner: request.SourceID}
			r.recordAffected(req.Target, request.SourceID, request.Phase, req.Required)
		}
	}
	graph := Graph{Nodes: nodes, Edges: edges}
	graph.Hash = r.graphHash(graph)
	result.Graph = graph
	r.mu.Lock()
	r.graphCache[request.SourceID] = graph
	r.mu.Unlock()
	return result
}

func (r *DefaultResolver) resolveSingle(ctx context.Context, req Request) Resolution {
	resolution := Resolution{Request: req}
	if r.candidate == nil {
		resolution.Status = StatusMissing
		if req.Required {
			resolution.Conflicts = append(resolution.Conflicts, Conflict{
				Kind: ConflictMissing, Request: req, Detail: "no candidate provider configured",
			})
		} else {
			resolution.Warnings = append(resolution.Warnings, Warning{
				Request: req, Message: "optional dependency missing (no provider)",
			})
			resolution.Status = StatusDowngraded
		}
		return resolution
	}
	candidates, err := r.candidate.FindCandidates(ctx, req.Target, req.Type)
	if err != nil {
		resolution.Status = StatusMissing
		if req.Required {
			resolution.Conflicts = append(resolution.Conflicts, Conflict{
				Kind: ConflictMissing, Request: req, Detail: err.Error(),
			})
		} else {
			resolution.Warnings = append(resolution.Warnings, Warning{
				Request: req, Message: err.Error(),
			})
			resolution.Status = StatusDowngraded
		}
		return resolution
	}
	resolution.Candidates = candidates
	rangeFiltered := r.filterByRange(candidates, req.VersionRange)
	if len(rangeFiltered) == 0 {
		resolution.Status = StatusMissing
		if req.Required {
			resolution.Conflicts = append(resolution.Conflicts, Conflict{
				Kind: ConflictVersion, Request: req,
				Detail: fmt.Sprintf("no candidate satisfies %s", req.VersionRange),
			})
		} else {
			resolution.Warnings = append(resolution.Warnings, Warning{
				Request: req, Message: "no candidate satisfies range",
			})
			resolution.Status = StatusDowngraded
		}
		return resolution
	}
	filtered := r.filterByPlatform(rangeFiltered)
	if len(filtered) == 0 {
		resolution.Status = StatusConflict
		resolution.Conflicts = append(resolution.Conflicts, Conflict{
			Kind: ConflictPlatform, Request: req, Detail: "no candidate for platform",
		})
		return resolution
	}
	selected := r.selectByPolicy(filtered, req.Policy)
	resolution.Selected = &selected
	resolution.Status = StatusResolved
	if req.Type == TargetHostFeature {
		if !r.hostFeatureAvailable(req.Target) {
			resolution.Status = StatusConflict
			resolution.Conflicts = append(resolution.Conflicts, Conflict{
				Kind: ConflictHostFeatureMissing, Request: req, Detail: "host feature not available",
			})
		}
	}
	if req.Type == TargetPlatform {
		if r.platform != "" && req.Target != "" && req.Target != r.platform {
			resolution.Status = StatusConflict
			resolution.Conflicts = append(resolution.Conflicts, Conflict{
				Kind: ConflictPlatform, Request: req, Detail: fmt.Sprintf("platform mismatch: %s != %s", req.Target, r.platform),
			})
		}
	}
	return resolution
}

func (r *DefaultResolver) filterByRange(candidates []Candidate, raw string) []Candidate {
	if strings.TrimSpace(raw) == "" || raw == "*" {
		return candidates
	}
	rng, err := ParseRange(raw)
	if err != nil {
		return candidates
	}
	var out []Candidate
	for _, c := range candidates {
		if rng.Satisfies(c.Version) {
			out = append(out, c)
		}
	}
	return out
}

func (r *DefaultResolver) filterByPlatform(candidates []Candidate) []Candidate {
	r.mu.RLock()
	platform := r.platform
	r.mu.RUnlock()
	if platform == "" {
		return candidates
	}
	var out []Candidate
	for _, c := range candidates {
		if c.Platform == "" || c.Platform == platform {
			out = append(out, c)
		}
	}
	return out
}

func (r *DefaultResolver) selectByPolicy(candidates []Candidate, policy ResolutionPolicy) Candidate {
	if len(candidates) == 0 {
		return Candidate{}
	}
	sorted := make([]Candidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Version.Compare(sorted[j].Version) > 0
	})
	switch policy {
	case PolicyHighestCompatible:
		return sorted[0]
	case PolicyLowestCompatible:
		return sorted[len(sorted)-1]
	case PolicyUserSelected:
		for _, c := range sorted {
			if c.UserSelected {
				return c
			}
		}
		return sorted[0]
	case PolicyInstalledPreferred:
		for _, c := range sorted {
			if c.Origin == "installed" || c.Origin == "system" {
				return c
			}
		}
		return sorted[0]
	case PolicySystemPreferred:
		for _, c := range sorted {
			if c.Origin == "system" {
				return c
			}
		}
		return sorted[0]
	case PolicyExact, PolicyIsolated:
		return sorted[0]
	}
	return sorted[0]
}

func (r *DefaultResolver) hostFeatureAvailable(feature string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	available, ok := r.hostFeatures[feature]
	if !ok {
		return true
	}
	return available
}

func (r *DefaultResolver) recordAffected(targetID, sourceID string, phase Phase, required bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.affectedIndex[targetID] = append(r.affectedIndex[targetID], AffectedSubject{
		SubjectID: sourceID, Type: TargetExtension, Phase: phase, Required: required,
	})
}

func (r *DefaultResolver) nextGeneration(sourceID string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dependencyGens[sourceID]++
	return r.dependencyGens[sourceID]
}

func (r *DefaultResolver) BuildGraph(_ context.Context, roots []string) Graph {
	r.mu.RLock()
	defer r.mu.RUnlock()
	visited := make(map[string]bool)
	var nodes []Node
	var edges []Edge
	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		if g, ok := r.graphCache[id]; ok {
			for _, n := range g.Nodes {
				if !visited[n.ID] {
					nodes = append(nodes, n)
					visit(n.ID)
				}
			}
			edges = append(edges, g.Edges...)
		} else {
			nodes = append(nodes, Node{ID: id, Type: TargetExtension})
		}
	}
	for _, root := range roots {
		visit(root)
	}
	nodeMap := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	g := Graph{Nodes: nodeMap, Edges: edges}
	g.Hash = r.graphHash(g)
	return g
}

func (r *DefaultResolver) Snapshot(_ context.Context, sourceID string) (Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.graphCache[sourceID]
	if !ok {
		return Snapshot{}, fmt.Errorf("dependency: no graph for %s", sourceID)
	}
	snap := Snapshot{
		SnapshotID: fmt.Sprintf("snap-%s-%d", sourceID, time.Now().UnixNano()),
		SourceID:   sourceID,
		GraphHash:  g.Hash,
		Generation: r.dependencyGens[sourceID],
		CreatedAt:  time.Now().UTC(),
	}
	for _, e := range g.Edges {
		if e.From != sourceID {
			continue
		}
		snap.Resolutions = append(snap.Resolutions, ResolutionRef{
			TargetID: e.To,
			Version:  e.Range,
			Status:   StatusResolved,
		})
	}
	r.snapshotCache[sourceID] = snap
	return snap, nil
}

func (r *DefaultResolver) AffectedBy(_ context.Context, targetID string) ([]AffectedSubject, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	subjects := append([]AffectedSubject{}, r.affectedIndex[targetID]...)
	sort.SliceStable(subjects, func(i, j int) bool {
		if subjects[i].Required != subjects[j].Required {
			return subjects[i].Required
		}
		return subjects[i].SubjectID < subjects[j].SubjectID
	})
	return subjects, nil
}

func (r *DefaultResolver) graphHash(g Graph) string {
	var sb strings.Builder
	keys := make([]string, 0, len(g.Nodes))
	for k := range g.Nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("|")
	}
	sortedEdges := make([]Edge, len(g.Edges))
	copy(sortedEdges, g.Edges)
	sort.SliceStable(sortedEdges, func(i, j int) bool {
		if sortedEdges[i].From != sortedEdges[j].From {
			return sortedEdges[i].From < sortedEdges[j].From
		}
		return sortedEdges[i].To < sortedEdges[j].To
	})
	for _, e := range sortedEdges {
		sb.WriteString(fmt.Sprintf("%s->%s:%s:%v;", e.From, e.To, e.Phase, e.Required))
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

func (r *DefaultResolver) DetectCycle(g Graph) []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var path []string
	var cycle []string
	adj := make(map[string][]Edge)
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e)
	}
	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		path = append(path, id)
		for _, e := range adj[id] {
			if !e.Required {
				continue
			}
			if color[e.To] == gray {
				for i, p := range path {
					if p == e.To {
						cycle = append([]string{}, path[i:]...)
						cycle = append(cycle, e.To)
						return true
					}
				}
			}
			if color[e.To] == white {
				if visit(e.To) {
					return true
				}
			}
		}
		color[id] = black
		path = path[:len(path)-1]
		return false
	}
	keys := make([]string, 0, len(g.Nodes))
	for k := range g.Nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if color[k] == white {
			if visit(k) {
				return cycle
			}
		}
	}
	return nil
}

func (r *DefaultResolver) TopologicalOrder(g Graph) ([]string, error) {
	if cycle := r.DetectCycle(g); cycle != nil {
		return nil, fmt.Errorf("%w: %v", ErrCycleDetected, cycle)
	}
	inDegree := make(map[string]int)
	for k := range g.Nodes {
		inDegree[k] = 0
	}
	for _, e := range g.Edges {
		if !e.Required {
			continue
		}
		if _, ok := g.Nodes[e.To]; ok {
			inDegree[e.To]++
		}
	}
	var queue []string
	for k, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, k)
		}
	}
	sort.Strings(queue)
	var result []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		result = append(result, id)
		var next []string
		for _, e := range g.Edges {
			if !e.Required {
				continue
			}
			if e.From != id {
				continue
			}
			if _, ok := g.Nodes[e.To]; !ok {
				continue
			}
			inDegree[e.To]--
			if inDegree[e.To] == 0 {
				next = append(next, e.To)
			}
		}
		sort.Strings(next)
		queue = append(queue, next...)
	}
	return result, nil
}

var _ Resolver = (*DefaultResolver)(nil)
