package runtime

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ServiceDefinitionBindingResolver interface {
	ResolveDefinitionID(
		runtimeID domain.RuntimeInstanceID,
		serviceID domain.ServiceID,
	) (string, error)
}
