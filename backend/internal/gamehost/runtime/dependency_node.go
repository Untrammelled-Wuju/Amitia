package runtime

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DependencyNode struct {
	ServiceID domain.ServiceID

	Dependencies []domain.ServiceID
	Dependents   []domain.ServiceID
}

func (n DependencyNode) IsRoot() bool {
	return len(n.Dependencies) == 0
}

func (n DependencyNode) IsLeaf() bool {
	return len(n.Dependents) == 0
}

func (n DependencyNode) Snapshot() DependencyNodeSnapshot {
	depsCopy := make([]domain.ServiceID, len(n.Dependencies))
	copy(depsCopy, n.Dependencies)

	dependentsCopy := make([]domain.ServiceID, len(n.Dependents))
	copy(dependentsCopy, n.Dependents)

	return DependencyNodeSnapshot{
		ServiceID:    n.ServiceID,
		Dependencies: depsCopy,
		Dependents:   dependentsCopy,
	}
}
