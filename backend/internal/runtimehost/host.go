// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type RuntimeHost interface {
	Descriptor() platform.RuntimeDescriptor
	Capabilities() *HostCapabilities
	Paths() util.RuntimePaths
	Processes() ProcessSupervisor
	RuntimeInstanceID() string
}
