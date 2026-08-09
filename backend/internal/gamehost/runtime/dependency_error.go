package runtime

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func NewDependencyNotFoundError(message string) *TopologyError {
	return NewTopologyError(ErrDependencyNotFound, message)
}

type DependencyCycleError struct {
	Message string
	Path    []domain.ServiceID
}

func (e *DependencyCycleError) Error() string {
	if len(e.Path) > 0 {
		strs := make([]string, len(e.Path))
		for i, sid := range e.Path {
			strs[i] = string(sid)
		}
		return fmt.Sprintf("%s: %s", e.Message, strings.Join(strs, " -> "))
	}
	return e.Message
}

func NewDependencyCycleError(message string, path []domain.ServiceID) *DependencyCycleError {
	return &DependencyCycleError{
		Message: message,
		Path:    path,
	}
}

func IsDependencyCycleError(err error) bool {
	_, ok := err.(*DependencyCycleError)
	return ok
}
