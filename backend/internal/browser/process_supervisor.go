package browser

import (
	"sync"

	"github.com/u-ai/backend/internal/runtimehost"
)

var (
	globalSupervisor     runtimehost.ProcessSupervisor
	globalSupervisorMu   sync.RWMutex
)

func SetGlobalRuntimeSupervisor(s runtimehost.ProcessSupervisor) {
	globalSupervisorMu.Lock()
	defer globalSupervisorMu.Unlock()
	globalSupervisor = s
}

func GetGlobalRuntimeSupervisor() runtimehost.ProcessSupervisor {
	globalSupervisorMu.RLock()
	defer globalSupervisorMu.RUnlock()
	return globalSupervisor
}
