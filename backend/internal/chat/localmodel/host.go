// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package localmodel

import (
	"sync"

	"github.com/u-ai/backend/internal/media"
	"github.com/u-ai/backend/internal/runtimehost"
)

var (
	globalHost       runtimehost.RuntimeHost
	globalHostMu     sync.RWMutex
	globalMaterializer media.ResourceMaterializer
	globalMaterializerMu sync.RWMutex
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

func SetGlobalMediaMaterializer(m media.ResourceMaterializer) {
	globalMaterializerMu.Lock()
	defer globalMaterializerMu.Unlock()
	globalMaterializer = m
}

func GetGlobalMediaMaterializer() media.ResourceMaterializer {
	globalMaterializerMu.RLock()
	defer globalMaterializerMu.RUnlock()
	return globalMaterializer
}
