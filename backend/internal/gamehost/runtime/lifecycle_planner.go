package runtime

import (
	"sort"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type LifecyclePlanner struct{}

func NewLifecyclePlanner() *LifecyclePlanner {
	return &LifecyclePlanner{}
}

func (p *LifecyclePlanner) topologicalSortFromGraph(graph DependencyGraphSnapshot) ([]domain.ServiceID, error) {
	indegree := make(map[domain.ServiceID]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		indegree[node.ServiceID] = len(node.Dependencies)
	}

	sortedIDs := make([]domain.ServiceID, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		sortedIDs = append(sortedIDs, node.ServiceID)
	}
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i] < sortedIDs[j]
	})

	queue := make([]domain.ServiceID, 0)
	for _, serviceID := range sortedIDs {
		if indegree[serviceID] == 0 {
			queue = append(queue, serviceID)
		}
	}

	head := 0
	var result []domain.ServiceID
	nodeMap := make(map[domain.ServiceID]DependencyNodeSnapshot, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeMap[node.ServiceID] = node
	}

	for head < len(queue) {
		current := queue[head]
		head++
		result = append(result, current)

		if node, exists := nodeMap[current]; exists {
			dependents := make([]domain.ServiceID, len(node.Dependents))
			copy(dependents, node.Dependents)
			sort.Slice(dependents, func(i, j int) bool {
				return dependents[i] < dependents[j]
			})
			for _, dependent := range dependents {
				indegree[dependent]--
				if indegree[dependent] == 0 {
					queue = append(queue, dependent)
				}
			}
		}
	}

	if len(result) != len(graph.Nodes) {
		return nil, NewDependencyCycleError("dependency cycle detected", nil)
	}

	return result, nil
}
