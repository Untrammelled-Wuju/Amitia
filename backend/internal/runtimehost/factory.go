// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"fmt"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type HostBuildContext struct {
	Descriptor     platform.RuntimeDescriptor
	Paths          util.RuntimePaths
	ProcessManager *process.DefaultProcessManager
}

func NewRuntimeHost(ctx HostBuildContext) (RuntimeHost, error) {
	switch ctx.Descriptor.Kind {
	case platform.RuntimeKindEmbedded, platform.RuntimeKindSandbox:
		return newRestrictedHost(ctx.Descriptor, ctx.Paths), nil
	}

	switch ctx.Descriptor.Guest {
	case platform.GuestPlatformAndroid, platform.GuestPlatformIOS, platform.GuestPlatformNone:
		return newRestrictedHost(ctx.Descriptor, ctx.Paths), nil
	}

	switch ctx.Descriptor.Guest {
	case platform.GuestPlatformLinux, platform.GuestPlatformWindows, platform.GuestPlatformMacOS:
		return newNativeProcessHost(ctx.Descriptor, ctx.Paths), nil
	}

	if ctx.Descriptor.Host == platform.HostPlatformUnknown &&
		ctx.Descriptor.Kind == platform.RuntimeKindUnknown &&
		ctx.Descriptor.Guest == platform.GuestPlatformUnknown {
		return nil, fmt.Errorf("%w: unknown descriptor: host=%s kind=%s guest=%s",
			ErrHostUnknownDescriptor, ctx.Descriptor.Host, ctx.Descriptor.Kind, ctx.Descriptor.Guest)
	}

	return newRestrictedHost(ctx.Descriptor, ctx.Paths), nil
}
var _ RuntimeHost = (*nativeProcessHost)(nil)
var _ RuntimeHost = (*restrictedHost)(nil)
