// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh"
	devicemeshserver "github.com/u-ai/backend/internal/devicemesh/server"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func runtimeCapability(t *testing.T, payload gin.H, key string) bool {
	t.Helper()
	caps, ok := payload["capabilities"].(gin.H)
	if !ok {
		t.Fatalf("capabilities has unexpected type %T", payload["capabilities"])
	}
	value, ok := caps[key].(bool)
	if !ok {
		t.Fatalf("capabilities[%q] has unexpected type %T", key, caps[key])
	}
	return value
}

func TestPublicRuntimeCapabilitiesCloudCoreExposesRemoteGameModeWhenMeshGatewayReady(t *testing.T) {
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileCloudCore),
		DeviceMesh: &devicemesh.Runtime{
			Hub:                devicemeshserver.NewConnectionHub(),
			PendingInvocations: capability.NewPendingInvocationManager(),
		},
	}
	payload := publicRuntimeCapabilities(services)
	if !runtimeCapability(t, payload, "gameMode") {
		t.Fatal("gameMode = false, want true for cloud-core with remote GameHost gateway")
	}
}

func TestPublicRuntimeCapabilitiesCloudCoreFailsClosedWithoutMeshGateway(t *testing.T) {
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileCloudCore),
	}
	payload := publicRuntimeCapabilities(services)
	if runtimeCapability(t, payload, "gameMode") {
		t.Fatal("gameMode = true, want false when cloud device mesh gateway is unavailable")
	}
}

func TestPublicRuntimeCapabilitiesLocalRequiresLiveGameHost(t *testing.T) {
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileLocal,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileLocal),
	}
	payload := publicRuntimeCapabilities(services)
	if runtimeCapability(t, payload, "gameMode") {
		t.Fatal("gameMode = true, want false without GameHost")
	}

	services.KernelContainer = &extensionkernel.Container{GameHost: &gamehost.GameHostContainer{}}
	payload = publicRuntimeCapabilities(services)
	if !runtimeCapability(t, payload, "gameMode") {
		t.Fatal("gameMode = false, want true for local with GameHost")
	}
}

func TestPublicRuntimeCapabilitiesDeviceAgentDoesNotExposeWebGameMode(t *testing.T) {
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileDeviceAgent,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileDeviceAgent),
		KernelContainer: &extensionkernel.Container{
			GameHost: &gamehost.GameHostContainer{},
		},
	}
	payload := publicRuntimeCapabilities(services)
	if runtimeCapability(t, payload, "gameMode") {
		t.Fatal("gameMode = true, want false for device-agent HTTP surface")
	}
}

func TestPublicRuntimeCapabilitiesFailClosedOnProfilePolicyMismatch(t *testing.T) {
	services := &AppServices{
		RuntimeProfile: runtimeprofile.ProfileCloudCore,
		RuntimePolicy:  runtimeprofile.PolicyFor(runtimeprofile.ProfileLocal),
		KernelContainer: &extensionkernel.Container{
			GameHost: &gamehost.GameHostContainer{},
		},
	}
	payload := publicRuntimeCapabilities(services)
	if payload["runtimeProfile"] != "unknown" {
		t.Fatalf("runtimeProfile = %v, want unknown for inconsistent profile/policy", payload["runtimeProfile"])
	}
	for _, key := range []string{"gameMode", "devicePluginRuntime", "deviceExecutionPlane", "localUIEndpoints"} {
		if runtimeCapability(t, payload, key) {
			t.Fatalf("capabilities[%q] = true, want fail-closed false", key)
		}
	}
}
