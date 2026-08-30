// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

type Policy struct {
	Profile Profile

	CoreBusinessServices bool
	FullHTTPAPI          bool

	ExtensionKernel bool
	TaskRuntime     bool

	VectorStore     bool
	GraphStore      bool
	ChannelSidecars bool

	DesktopPet bool

	DeviceExecutionPlane bool
	DevicePluginRuntime  bool

	LocalUIEndpoints bool

	DurableEvents bool
}

func (p Policy) AllowsCoreBusiness() bool {
	return p.CoreBusinessServices
}

func (p Policy) AllowsFullHTTPAPI() bool {
	return p.FullHTTPAPI
}

func (p Policy) AllowsDeviceExecution() bool {
	return p.DeviceExecutionPlane
}

func (p Policy) AllowsLocalUIEndpoints() bool {
	return p.LocalUIEndpoints
}

func PolicyFor(profile Profile) Policy {
	switch profile {
	case ProfileLocal:
		return Policy{
			Profile: ProfileLocal,

			CoreBusinessServices: true,
			FullHTTPAPI:          true,

			ExtensionKernel: true,
			TaskRuntime:     true,

			VectorStore:     true,
			GraphStore:      true,
			ChannelSidecars: true,

			DesktopPet: true,

			DeviceExecutionPlane: true,
			DevicePluginRuntime:  true,

			LocalUIEndpoints: true,

			DurableEvents: true,
		}
	case ProfileCloudCore:
		return Policy{
			Profile: ProfileCloudCore,

			CoreBusinessServices: true,
			FullHTTPAPI:          true,

			ExtensionKernel: true,
			TaskRuntime:     true,

			VectorStore:     true,
			GraphStore:      true,
			ChannelSidecars: true,

			DesktopPet: false,

			DeviceExecutionPlane: false,
			DevicePluginRuntime:  false,

			LocalUIEndpoints: false,

			DurableEvents: true,
		}
	case ProfileDeviceAgent:
		return Policy{
			Profile: ProfileDeviceAgent,

			CoreBusinessServices: false,
			FullHTTPAPI:          false,

			ExtensionKernel: true,
			TaskRuntime:     true,

			VectorStore:     false,
			GraphStore:      false,
			ChannelSidecars: false,

			// The pet body is a device capability. In cloud deployments the
			// Business Core remains remote, while package installation, renderer
			// state and Runtime v2 are hosted by this device-agent.
			DesktopPet: true,

			DeviceExecutionPlane: true,
			DevicePluginRuntime:  true,

			LocalUIEndpoints: true,

			DurableEvents: true,
		}
	default:
		return Policy{Profile: profile}
	}
}
