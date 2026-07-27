package migration

import (
	"fmt"
	"sort"
)

type MigrationGraphResolver struct {
	defMap map[string]MigrationDefinition
}

func NewMigrationGraphResolver() *MigrationGraphResolver {
	return &MigrationGraphResolver{
		defMap: make(map[string]MigrationDefinition),
	}
}

func (r *MigrationGraphResolver) BuildGraph(migrations []MigrationDefinition) (*MigrationGraph, error) {
	if len(migrations) == 0 {
		return &MigrationGraph{}, nil
	}
	r.defMap = make(map[string]MigrationDefinition)
	for _, m := range migrations {
		r.defMap[m.MigrationID] = m
	}
	graph := &MigrationGraph{
		ExtensionID: migrations[0].ExtensionID,
	}
	nodeMap := make(map[MigrationNodeID]bool)
	for _, m := range migrations {
		nodeID := MigrationNodeID(m.MigrationID)
		if nodeMap[nodeID] {
			continue
		}
		node := MigrationNode{
			NodeID:      nodeID,
			MigrationID: m.MigrationID,
			Stage:       r.inferStage(m),
			DependsOn:   r.inferDependsOn(m),
		}
		graph.Nodes = append(graph.Nodes, node)
		nodeMap[nodeID] = true
	}
	r.buildEdges(graph)
	if err := r.DetectCycle(graph); err != nil {
		return nil, err
	}
	return graph, nil
}

func (r *MigrationGraphResolver) inferStage(m MigrationDefinition) string {
	for _, dd := range m.DataDomains {
		switch dd.Domain {
		case "schema":
			return "schema"
		case "data":
			return "data"
		case "resource_index":
			return "index"
		}
	}
	if m.Direction == DirectionRepair {
		return "prepare"
	}
	return "data"
}

func (r *MigrationGraphResolver) inferDependsOn(m MigrationDefinition) []MigrationNodeID {
	if m.ForwardMigrationID != nil && *m.ForwardMigrationID != "" {
		return []MigrationNodeID{MigrationNodeID(*m.ForwardMigrationID)}
	}
	return nil
}

func (r *MigrationGraphResolver) buildEdges(graph *MigrationGraph) {
	for _, node := range graph.Nodes {
		for _, dep := range node.DependsOn {
			graph.Edges = append(graph.Edges, MigrationEdge{
				From: dep,
				To:   node.NodeID,
				Type: "depends_on",
			})
		}
	}
	stageOrder := map[string]int{
		"prepare":  0,
		"schema":   1,
		"data":     2,
		"index":    3,
		"validate": 4,
	}
	for _, n := range graph.Nodes {
		for _, other := range graph.Nodes {
			if n.NodeID == other.NodeID {
				continue
			}
			if stageOrder[n.Stage] < stageOrder[other.Stage] {
				exists := false
				for _, e := range graph.Edges {
					if e.From == n.NodeID && e.To == other.NodeID {
						exists = true
						break
					}
				}
				if !exists {
					graph.Edges = append(graph.Edges, MigrationEdge{
						From: n.NodeID,
						To:   other.NodeID,
						Type: "before",
					})
				}
			}
		}
	}
}

func (r *MigrationGraphResolver) DetectCycle(graph *MigrationGraph) error {
	adj := make(map[MigrationNodeID][]MigrationNodeID)
	for _, n := range graph.Nodes {
		adj[n.NodeID] = nil
	}
	for _, e := range graph.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	white, gray, black := 0, 1, 2
	color := make(map[MigrationNodeID]int)
	for _, n := range graph.Nodes {
		color[n.NodeID] = white
	}
	var hasCycle func(node MigrationNodeID) bool
	hasCycle = func(node MigrationNodeID) bool {
		color[node] = gray
		for _, neighbor := range adj[node] {
			if color[neighbor] == gray {
				return true
			}
			if color[neighbor] == white && hasCycle(neighbor) {
				return true
			}
		}
		color[node] = black
		return false
	}
	for _, n := range graph.Nodes {
		if color[n.NodeID] == white {
			if hasCycle(n.NodeID) {
				return fmt.Errorf("migration: cycle detected involving node %s", n.NodeID)
			}
		}
	}
	return nil
}

func (r *MigrationGraphResolver) TopologicalSort(graph *MigrationGraph) ([]MigrationNodeID, error) {
	if err := r.DetectCycle(graph); err != nil {
		return nil, err
	}
	inDegree := make(map[MigrationNodeID]int)
	adj := make(map[MigrationNodeID][]MigrationNodeID)
	for _, n := range graph.Nodes {
		inDegree[n.NodeID] = 0
		adj[n.NodeID] = nil
	}
	for _, e := range graph.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}
	var queue []MigrationNodeID
	for _, n := range graph.Nodes {
		if inDegree[n.NodeID] == 0 {
			queue = append(queue, n.NodeID)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return queue[i] < queue[j]
	})
	var result []MigrationNodeID
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		neighbors := adj[node]
		sort.Slice(neighbors, func(i, j int) bool {
			return neighbors[i] < neighbors[j]
		})
		for _, neighbor := range neighbors {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}
	return result, nil
}

func (r *MigrationGraphResolver) ResolvePath(graph *MigrationGraph, fromVersion, toVersion string) (*MigrationPath, error) {
	if fromVersion == toVersion {
		return &MigrationPath{
			FromVersion: fromVersion,
			ToVersion:   toVersion,
			IsDirect:    true,
		}, nil
	}
	direct := r.findDirectMigration(fromVersion, toVersion)
	if direct != nil {
		return &MigrationPath{
			FromVersion: fromVersion,
			ToVersion:   toVersion,
			IsDirect:    true,
			Steps:       []MigrationPathStep{*direct},
		}, nil
	}
	chainPath := r.findChainMigration(fromVersion, toVersion)
	if chainPath != nil {
		return chainPath, nil
	}
	return nil, fmt.Errorf("migration: no path from %s to %s", fromVersion, toVersion)
}

func (r *MigrationGraphResolver) findDirectMigration(fromVersion, toVersion string) *MigrationPathStep {
	for mid, def := range r.defMap {
		if r.matchVersionRange(def.FromVersionRange, fromVersion) && def.ToVersion == toVersion {
			return &MigrationPathStep{
				StepID:      1,
				NodeID:      MigrationNodeID(mid),
				MigrationID: mid,
				FromVersion: fromVersion,
				ToVersion:   toVersion,
				Direction:   def.Direction,
			}
		}
	}
	return nil
}

func (r *MigrationGraphResolver) findChainMigration(fromVersion, toVersion string) *MigrationPath {
	visited := make(map[string]bool)
	var currentPath []MigrationPathStep
	var result *MigrationPath
	var dfs func(version string) bool
	dfs = func(version string) bool {
		if version == toVersion {
			if len(currentPath) > 0 {
				steps := make([]MigrationPathStep, len(currentPath))
				for i, s := range currentPath {
					s.StepID = i + 1
					steps[i] = s
				}
				result = &MigrationPath{
					Steps:       steps,
					FromVersion: fromVersion,
					ToVersion:   toVersion,
					IsDirect:    false,
				}
			}
			return true
		}
		if visited[version] {
			return false
		}
		visited[version] = true
		defer func() { visited[version] = false }()
		for mid, def := range r.defMap {
			if r.matchVersionRange(def.FromVersionRange, version) && def.ToVersion != version {
				step := MigrationPathStep{
					NodeID:      MigrationNodeID(mid),
					MigrationID: mid,
					FromVersion: version,
					ToVersion:   def.ToVersion,
					Direction:   def.Direction,
				}
				currentPath = append(currentPath, step)
				if dfs(def.ToVersion) {
					return true
				}
				currentPath = currentPath[:len(currentPath)-1]
			}
		}
		return false
	}
	dfs(fromVersion)
	return result
}

func (r *MigrationGraphResolver) matchVersionRange(versionRange, version string) bool {
	if versionRange == "" || versionRange == "*" {
		return true
	}
	if versionRange == version {
		return true
	}
	if len(versionRange) > 0 && versionRange[0] == '[' {
		return true
	}
	if len(versionRange) > 2 && versionRange[:2] == ">=" {
		return true
	}
	return false
}
