// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

import "testing"

func TestPolicyForDeviceAgentKeepsDesktopPetDeviceLocal(t *testing.T) {
	policy := PolicyFor(ProfileDeviceAgent)
	if !policy.DesktopPet {
		t.Fatal("device-agent must host the local desktop-pet runtime")
	}
	if policy.FullHTTPAPI || policy.CoreBusinessServices {
		t.Fatal("device-agent desktop-pet capability must not enable cloud business HTTP services")
	}
	if !policy.DeviceExecutionPlane || !policy.LocalUIEndpoints {
		t.Fatal("device-agent desktop-pet runtime requires device execution and local UI endpoints")
	}
}

func TestPolicyForCloudCoreDoesNotHostDesktopPetBody(t *testing.T) {
	policy := PolicyFor(ProfileCloudCore)
	if policy.DesktopPet {
		t.Fatal("cloud-core must not host desktop-pet packages, renderer state, or Runtime v2")
	}
	if !policy.FullHTTPAPI || !policy.CoreBusinessServices {
		t.Fatal("cloud-core must retain business API authority")
	}
}
