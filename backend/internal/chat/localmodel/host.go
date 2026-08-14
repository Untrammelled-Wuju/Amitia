// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package localmodel

import (
	"sync"

	"github.com/u-ai/backend/internal/runtimehost"
)

var (
	globalHost   runtimehost.RuntimeHost
	globalHostMu sync.RWMutex
)

func SetGlobalRuntimeHost(host runtimehost.RuntimeHost) {
	globalHostMu.Lock()
	defer globalHostMu.Unlock()
	globalHost = host
}

func GetGlobalRuntimeHost() runtimehost.RuntimeHost {
	globalHostMu.RLock()
	defer globalHostMu.RUnlock()
	return globalHost
}
