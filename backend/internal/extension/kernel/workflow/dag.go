package workflow

import "fmt"

func ValidateDAG(nodes []WorkflowNode) error {
	if len(nodes) == 0 {
		return nil
	}

	idSet := make(map[string]bool)
	for _, node := range nodes {
		if node.ID == "" {
			return fmt.Errorf("%w: empty node id", ErrInvalidNodeID)
		}
		if idSet[node.ID] {
			return fmt.Errorf("%w: %s", ErrDuplicateNodeID, node.ID)
		}
		idSet[node.ID] = true
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if !idSet[dep] {
				return fmt.Errorf("%w: node %s depends on non-existent node %s", ErrInvalidNodeID, node.ID, dep)
			}
			if dep == node.ID {
				return fmt.Errorf("%w: node %s depends on itself", ErrCycleDetected, node.ID)
			}
		}
	}

	if _, err := TopologicalSort(nodes); err != nil {
		return err
	}

	hasEntry := false
	for _, node := range nodes {
		if len(node.DependsOn) == 0 {
			hasEntry = true
			break
		}
	}
	if !hasEntry {
		return fmt.Errorf("%w: no entry node (all nodes have dependencies)", ErrCycleDetected)
	}

	return nil
}

func TopologicalSort(nodes []WorkflowNode) ([]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	idSet := make(map[string]bool)

	for _, node := range nodes {
		idSet[node.ID] = true
		inDegree[node.ID] = 0
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if !idSet[dep] {
				return nil, fmt.Errorf("%w: node %s depends on non-existent node %s", ErrInvalidNodeID, node.ID, dep)
			}
			adj[dep] = append(adj[dep], node.ID)
			inDegree[node.ID]++
		}
	}

	var queue []string
	for _, node := range nodes {
		if inDegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}

	result := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, curr)
		for _, next := range adj[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(result) != len(nodes) {
		return nil, ErrCycleDetected
	}

	return result, nil
}
