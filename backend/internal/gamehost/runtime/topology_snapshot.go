package runtime

import (
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ServiceInstanceSnapshot struct {
	ID        ServiceInstanceID
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID
	ServiceID domain.ServiceID

	State    ServiceRuntimeState
	Required bool

	ServiceKind  domain.ServiceKind
	Dependencies []domain.ServiceID

	CreatedAt time.Time
	UpdatedAt time.Time

	StartedAt *time.Time
	StoppedAt *time.Time
	FailedAt  *time.Time

	Metadata map[string]string
}

type RuntimeTopologySnapshot struct {
	RuntimeID   domain.RuntimeInstanceID
	PluginID    domain.PluginID
	ExtensionID string

	Services []ServiceInstanceSnapshot

	CreatedAt time.Time
	UpdatedAt time.Time
}

func sortServicesByID(services []ServiceInstanceSnapshot) {
	sort.SliceStable(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
}
