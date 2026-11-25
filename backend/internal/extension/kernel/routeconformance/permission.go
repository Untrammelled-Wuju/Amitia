//go:build rsi_routeconformance

package routeconformance

import (
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	permissionEpoch    = "conformance-epoch"
	permissionRevision = 11
)

type PermissionCase struct {
	Name     string
	Request  plugincontract.BindingRequest
	Allow    bool
	Scope    plugincontract.BindingScope
	Sentinel error
}

func PermissionSnapshot() (*plugincontract.BindingSnapshot, error) {
	bindings := []plugincontract.Binding{
		{
			Key:                plugincontract.BindingKey{CapabilityID: "cap.global", Scope: plugincontract.BindingScopeGlobal},
			ProviderID:         "provider.global",
			ProviderInstanceID: "provider.global:1",
		},
		{
			Key:                plugincontract.BindingKey{CapabilityID: "cap.space", SpaceID: "space-a", Scope: plugincontract.BindingScopeSpace},
			ProviderID:         "provider.space",
			ProviderInstanceID: "provider.space:1",
		},
		{
			Key:                plugincontract.BindingKey{CapabilityID: "cap.device", SpaceID: "space-a", DeviceID: "device-1", Scope: plugincontract.BindingScopeDevice},
			ProviderID:         "provider.device",
			ProviderInstanceID: "provider.device:1",
		},
		{
			Key:                plugincontract.BindingKey{CapabilityID: "cap.precedence", SpaceID: "space-a", DeviceID: "device-1", Scope: plugincontract.BindingScopeDevice},
			ProviderID:         "provider.precedence.device",
			ProviderInstanceID: "provider.precedence.device:1",
		},
		{
			Key:                plugincontract.BindingKey{CapabilityID: "cap.precedence", SpaceID: "space-a", Scope: plugincontract.BindingScopeSpace},
			ProviderID:         "provider.precedence.space",
			ProviderInstanceID: "provider.precedence.space:1",
		},
	}
	return plugincontract.NewBindingSnapshot(permissionEpoch, permissionRevision, bindings)
}

func PermissionCases() []PermissionCase {
	return []PermissionCase{
		{Name: "global-from-any-scope", Request: plugincontract.BindingRequest{CapabilityID: "cap.global", SpaceID: "space-b", DeviceID: "device-9"}, Allow: true, Scope: plugincontract.BindingScopeGlobal},
		{Name: "global-from-own-scope", Request: plugincontract.BindingRequest{CapabilityID: "cap.global", SpaceID: "space-a", DeviceID: "device-1"}, Allow: true, Scope: plugincontract.BindingScopeGlobal},
		{Name: "space-inside", Request: plugincontract.BindingRequest{CapabilityID: "cap.space", SpaceID: "space-a", DeviceID: "device-1"}, Allow: true, Scope: plugincontract.BindingScopeSpace},
		{Name: "space-outside", Request: plugincontract.BindingRequest{CapabilityID: "cap.space", SpaceID: "space-b", DeviceID: "device-1"}, Allow: false, Sentinel: plugincontract.ErrBindingNotFound},
		{Name: "device-inside", Request: plugincontract.BindingRequest{CapabilityID: "cap.device", SpaceID: "space-a", DeviceID: "device-1"}, Allow: true, Scope: plugincontract.BindingScopeDevice},
		{Name: "device-outside", Request: plugincontract.BindingRequest{CapabilityID: "cap.device", SpaceID: "space-a", DeviceID: "device-2"}, Allow: false, Sentinel: plugincontract.ErrBindingNotFound},
		{Name: "unknown-capability", Request: plugincontract.BindingRequest{CapabilityID: "cap.unknown", SpaceID: "space-a", DeviceID: "device-1"}, Allow: false, Sentinel: plugincontract.ErrBindingNotFound},
		{Name: "blank-capability", Request: plugincontract.BindingRequest{CapabilityID: "   ", SpaceID: "space-a", DeviceID: "device-1"}, Allow: false, Sentinel: plugincontract.ErrBindingInvalid},
		{Name: "device-beats-space", Request: plugincontract.BindingRequest{CapabilityID: "cap.precedence", SpaceID: "space-a", DeviceID: "device-1"}, Allow: true, Scope: plugincontract.BindingScopeDevice},
		{Name: "space-when-device-misses", Request: plugincontract.BindingRequest{CapabilityID: "cap.precedence", SpaceID: "space-a", DeviceID: "device-2"}, Allow: true, Scope: plugincontract.BindingScopeSpace},
	}
}

func CheckPermissions(snapshot *plugincontract.BindingSnapshot) (rows int, allowed int, denied int, problems []string) {
	problems = make([]string, 0)
	if snapshot == nil {
		return 0, 0, 0, append(problems, "permission snapshot is nil")
	}
	for _, item := range PermissionCases() {
		rows++
		resolution, err := snapshot.Resolve(item.Request)
		if item.Allow {
			allowed++
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s denied: %v", item.Name, err))
				continue
			}
			if resolution.Scope != item.Scope {
				problems = append(problems, fmt.Sprintf("%s scope=%s want %s", item.Name, resolution.Scope, item.Scope))
			}
			if resolution.Revision != permissionRevision || resolution.Epoch != permissionEpoch {
				problems = append(problems, fmt.Sprintf("%s revision=%d epoch=%s want %d/%s", item.Name, resolution.Revision, resolution.Epoch, permissionRevision, permissionEpoch))
			}
			if len(resolution.Trail) == 0 || !strings.HasSuffix(resolution.Trail[len(resolution.Trail)-1], "=hit") {
				problems = append(problems, fmt.Sprintf("%s trail=%s want hit", item.Name, strings.Join(resolution.Trail, ">")))
			}
			continue
		}
		denied++
		if err == nil {
			problems = append(problems, fmt.Sprintf("%s allowed but must be denied", item.Name))
			continue
		}
		if item.Sentinel != nil && !errors.Is(err, item.Sentinel) {
			problems = append(problems, fmt.Sprintf("%s error chain lost %v: %v", item.Name, item.Sentinel, err))
		}
	}
	return rows, allowed, denied, problems
}
