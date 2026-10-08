package main

import (
	"testing"

	"github.com/u-ai/backend/internal/auth"
)

func TestMeshTaskCallerRequiresCurrentCoreAndAuthenticatedConsoleOrDevice(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		actor   *auth.ActorContext
		allowed bool
	}{
		{"missing", nil, false},
		{"device", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "device"}, true},
		{"console", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "console", IsLocalTrusted: true, Permissions: []string{auth.PermSystemAdmin}}, true},
		{"untrusted-console", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "console", Permissions: []string{auth.PermSystemAdmin}}, false},
		{"console-without-admin", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "console", IsLocalTrusted: true}, false},
		{"foreign-core", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "foreign", DeviceID: "device"}, false},
		{"missing-device", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core"}, false},
		{"extension", &auth.ActorContext{PrincipalType: auth.PrincipalExtension, SpaceID: "core", DeviceID: "device", IsLocalTrusted: true, Permissions: []string{auth.PermSystemAdmin}}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if meshTaskCallerAllowed(scenario.actor, "core") != scenario.allowed {
				t.Fatal("设备任务调用者边界错误")
			}
		})
	}
}
