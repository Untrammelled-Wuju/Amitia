package integration

import (
	"github.com/u-ai/backend/internal/gamehost/control"
)

var _ control.HostAPIWorkCanceller = (*HostAPIInvocationTracker)(nil)
