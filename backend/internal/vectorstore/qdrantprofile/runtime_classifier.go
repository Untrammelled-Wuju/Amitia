// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"fmt"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
)

type RuntimeClass string

const (
	RuntimeClassDesktopProcess RuntimeClass = "desktop-process"
	RuntimeClassAndroidProot   RuntimeClass = "android-proot"
	RuntimeClassRestricted     RuntimeClass = "restricted"
	RuntimeClassUnsupported    RuntimeClass = "unsupported"
)

type DescriptorProvider interface {
	Descriptor() platform.RuntimeDescriptor
	Capabilities() *runtimehost.HostCapabilities
}

type runtimeClassifier struct{}

func NewRuntimeClassifier() *runtimeClassifier {
	return &runtimeClassifier{}
}

func (c *runtimeClassifier) Classify(provider DescriptorProvider) (RuntimeClass, error) {
	if provider == nil {
		return "", ErrRuntimeDescriptorUnavailable
	}
	desc := provider.Descriptor()
	caps := provider.Capabilities()

	if caps == nil {
		return "", ErrRuntimeDescriptorUnavailable
	}

	if !caps.Supports(runtimehost.CapProcessSpawn) ||
		!caps.Supports(runtimehost.CapFilesystemLocal) {
		return RuntimeClassRestricted, nil
	}

	if desc.Host == platform.HostPlatformAndroid &&
		(desc.Kind == platform.RuntimeKindProot || desc.Kind == platform.RuntimeKindEmbedded) &&
		desc.Guest == platform.GuestPlatformLinux &&
		desc.Architecture == "arm64" {
		return RuntimeClassAndroidProot, nil
	}

	if desc.Host == platform.HostPlatformIOS {
		return RuntimeClassRestricted, nil
	}

	if desc.Host == platform.HostPlatformWindows ||
		desc.Host == platform.HostPlatformLinux ||
		desc.Host == platform.HostPlatformMacOS {
		if desc.Guest == platform.GuestPlatformWindows ||
			desc.Guest == platform.GuestPlatformLinux ||
			desc.Guest == platform.GuestPlatformMacOS ||
			desc.Guest == platform.GuestPlatformNone {
			return RuntimeClassDesktopProcess, nil
		}
		return RuntimeClassUnsupported, fmt.Errorf("%w: unsupported guest platform: %s", ErrUnsupportedGuestPlatform, desc.Guest)
	}

	if desc.Host == platform.HostPlatformAndroid {
		if desc.Guest != platform.GuestPlatformLinux {
			return RuntimeClassUnsupported, fmt.Errorf("%w: unsupported guest platform: %s", ErrUnsupportedGuestPlatform, desc.Guest)
		}
		if desc.Architecture != "arm64" {
			return RuntimeClassUnsupported, fmt.Errorf("%w: unsupported architecture: %s", ErrUnsupportedGuestArchitecture, desc.Architecture)
		}
		return RuntimeClassUnsupported, fmt.Errorf("%w: android non-proot runtime", ErrRuntimeClassificationFailed)
	}

	return RuntimeClassUnsupported, fmt.Errorf("%w: unsupported host platform: %s", ErrRuntimeClassificationFailed, desc.Host)
}
