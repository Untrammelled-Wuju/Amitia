package lifecycle

import (
	"fmt"
	"sort"
)

type DependencyGraph struct {
	nodes map[string][]string
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{nodes: make(map[string][]string)}
}

func (g *DependencyGraph) AddNode(id string, deps []string) {
	g.nodes[id] = append([]string{}, deps...)
}

func (g *DependencyGraph) Dependencies(id string) []string {
	deps, ok := g.nodes[id]
	if !ok {
		return nil
	}
	return append([]string{}, deps...)
}

func (g *DependencyGraph) DetectCycle() []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var path []string
	var cycle []string

	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		path = append(path, id)
		for _, dep := range g.nodes[id] {
			if color[dep] == gray {
				for i, p := range path {
					if p == dep {
						cycle = append([]string{}, path[i:]...)
						cycle = append(cycle, dep)
						return true
					}
				}
				cycle = []string{dep, id, dep}
				return true
			}
			if color[dep] == white {
				if visit(dep) {
					return true
				}
			}
		}
		color[id] = black
		path = path[:len(path)-1]
		return false
	}

	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if color[id] == white {
			if visit(id) {
				return cycle
			}
		}
	}
	return nil
}

func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	if cycle := g.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("%w: %v", ErrCircularDependency, cycle)
	}
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, deps := range g.nodes {
		for _, dep := range deps {
			if _, ok := g.nodes[dep]; ok {
				inDegree[dep] = inDegree[dep] + 0
			}
		}
	}
	for id, deps := range g.nodes {
		for _, dep := range deps {
			if _, ok := g.nodes[dep]; !ok {
				return nil, fmt.Errorf("%w: component %q depends on missing %q", ErrMissingDependency, id, dep)
			}
		}
		inDegree[id] = len(deps)
	}
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	var result []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		result = append(result, id)
		var newReady []string
		for other, deps := range g.nodes {
			for i, dep := range deps {
				if dep == id {
					inDegree[other]--
					if inDegree[other] == 0 {
						newReady = append(newReady, other)
					}
					_ = i
					break
				}
			}
		}
		sort.Strings(newReady)
		queue = append(queue, newReady...)
	}
	if len(result) != len(g.nodes) {
		return nil, ErrCircularDependency
	}
	return result, nil
}

type Planner struct {
	registry *ComponentRegistry
}

func NewPlanner(registry *ComponentRegistry) *Planner {
	return &Planner{registry: registry}
}

func (p *Planner) BuildPlan(startupID string) (*BootstrapPlan, error) {
	metas := p.registry.AllMetadata()
	graph := NewDependencyGraph()
	phaseSet := make(map[StartupPhase]struct{})
	components := make([]BootstrapComponent, 0, len(metas))
	for _, m := range metas {
		graph.AddNode(m.ID, m.Dependencies)
		phaseSet[m.Phase] = struct{}{}
		components = append(components, m)
	}
	if cycle := graph.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("%w: %v", ErrCircularDependency, cycle)
	}
	for _, m := range metas {
		for _, dep := range m.Dependencies {
			if _, ok := p.registry.Metadata(dep); !ok {
				return nil, fmt.Errorf("%w: %q depends on missing %q", ErrMissingDependency, m.ID, dep)
			}
			depMeta, _ := p.registry.Metadata(dep)
			if phaseOrder(depMeta.Phase) > phaseOrder(m.Phase) {
				return nil, fmt.Errorf("%w: %q (%s) depends on %q (%s) in later phase",
					ErrIllegalCrossPhase, m.ID, m.Phase, dep, depMeta.Phase)
			}
		}
	}
	phases := make([]StartupPhase, 0, len(phaseSet))
	for ph := range phaseSet {
		phases = append(phases, ph)
	}
	sort.Slice(phases, func(i, j int) bool {
		return phaseOrder(phases[i]) < phaseOrder(phases[j])
	})
	plan := &BootstrapPlan{
		Components: components,
		Phases:     phases,
		StartupID:  startupID,
		CreatedAt:  now(),
	}
	plan.PlanHash = computePlanHash(plan)
	return plan, nil
}

func (p *Planner) OrderByPhase(plan *BootstrapPlan) [][]BootstrapComponent {
	groups := make(map[StartupPhase][]BootstrapComponent)
	for _, c := range plan.Components {
		groups[c.Phase] = append(groups[c.Phase], c)
	}
	for ph := range groups {
		g := groups[ph]
		sort.Slice(g, func(i, j int) bool { return g[i].ID < g[j].ID })
		groups[ph] = g
	}
	out := make([][]BootstrapComponent, 0, len(plan.Phases))
	for _, ph := range plan.Phases {
		out = append(out, groups[ph])
	}
	return out
}
