package hook

import (
	"sort"
)

type OrderingInput struct {
	Contributions []HookContributionDefinition
}

type OrderingResult struct {
	Ordered      []HookContributionDefinition
	CycleDetected bool
	CyclesRemoved int
	Warnings      []string
}

type orderingNode struct {
	Contribution HookContributionDefinition
	Before       []string
	After        []string
}

func OrderHooks(input OrderingInput) OrderingResult {
	if len(input.Contributions) == 0 {
		return OrderingResult{}
	}

	byPhase := groupByPhase(input.Contributions)
	var ordered []HookContributionDefinition
	var warnings []string
	cycleDetected := false
	cyclesRemoved := 0

	phaseOrder := []HookPhase{PhaseBefore, PhaseFilter, PhaseTransform, PhaseAfter, PhaseObserve}
	for _, phase := range phaseOrder {
		contribs := byPhase[phase]
		if len(contribs) == 0 {
			continue
		}
		phaseOrdered, phaseCycle, phaseRemoved, phaseWarnings := orderWithinPhase(contribs)
		ordered = append(ordered, phaseOrdered...)
		if phaseCycle {
			cycleDetected = true
		}
		cyclesRemoved += phaseRemoved
		warnings = append(warnings, phaseWarnings...)
	}

	return OrderingResult{
		Ordered:       ordered,
		CycleDetected: cycleDetected,
		CyclesRemoved: cyclesRemoved,
		Warnings:      warnings,
	}
}

func groupByPhase(contribs []HookContributionDefinition) map[HookPhase][]HookContributionDefinition {
	out := make(map[HookPhase][]HookContributionDefinition)
	for _, c := range contribs {
		out[c.Phase] = append(out[c.Phase], c)
	}
	return out
}

func orderWithinPhase(contribs []HookContributionDefinition) ([]HookContributionDefinition, bool, int, []string) {
	stableSorted := make([]HookContributionDefinition, len(contribs))
	copy(stableSorted, contribs)
	sort.SliceStable(stableSorted, func(i, j int) bool {
		a, b := stableSorted[i], stableSorted[j]
		if a.SystemReserved != b.SystemReserved {
			return a.SystemReserved
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.ExtensionID != b.ExtensionID {
			return a.ExtensionID < b.ExtensionID
		}
		return a.ContributionID < b.ContributionID
	})

	contribSet := make(map[string]bool)
	for _, c := range stableSorted {
		contribSet[c.ContributionID] = true
	}

	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, c := range stableSorted {
		inDegree[c.ContributionID] = 0
	}
	for _, c := range stableSorted {
		for _, beforeID := range c.Before {
			if !contribSet[beforeID] {
				continue
			}
			if beforeID == c.ContributionID {
				continue
			}
			adj[c.ContributionID] = append(adj[c.ContributionID], beforeID)
			inDegree[beforeID]++
		}
		for _, afterID := range c.After {
			if !contribSet[afterID] {
				continue
			}
			if afterID == c.ContributionID {
				continue
			}
			adj[afterID] = append(adj[afterID], c.ContributionID)
			inDegree[c.ContributionID]++
		}
	}

	queue := make([]string, 0)
	for _, c := range stableSorted {
		if inDegree[c.ContributionID] == 0 {
			queue = append(queue, c.ContributionID)
		}
	}

	var topoOrder []string
	var warnings []string

	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, head)
		for _, neighbor := range adj[head] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(topoOrder) < len(stableSorted) {
		cyclicNodes := make([]string, 0)
		for _, c := range stableSorted {
			if inDegree[c.ContributionID] > 0 {
				cyclicNodes = append(cyclicNodes, c.ContributionID)
			}
		}
		warnings = append(warnings, "cycle detected among: "+joinIDs(cyclicNodes)+", falling back to stable sort")
		return stableSorted, true, len(cyclicNodes), warnings
	}

	posMap := make(map[string]int)
	for i, id := range topoOrder {
		posMap[id] = i
	}
	result := make([]HookContributionDefinition, len(stableSorted))
	for i, id := range topoOrder {
		for _, c := range stableSorted {
			if c.ContributionID == id {
				result[i] = c
				break
			}
		}
	}
	_ = posMap
	return result, false, 0, warnings
}

func joinIDs(ids []string) string {
	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ", "
		}
		result += id
	}
	return result
}
