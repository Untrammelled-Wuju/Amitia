// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
)

func androidProotProvider() DescriptorProvider {
	return fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformAndroid,
			Kind:         platform.RuntimeKindProot,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "arm64",
		},
		caps: androidCaps(),
	}
}

func windowsDesktopProvider() DescriptorProvider {
	return fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformWindows,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformWindows,
			Architecture: "amd64",
		},
		caps: androidCaps(),
	}
}

func linuxDesktopProvider() DescriptorProvider {
	return fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformLinux,
			Kind:         platform.RuntimeKindNativeProcess,
			Guest:        platform.GuestPlatformLinux,
			Architecture: "amd64",
		},
		caps: androidCaps(),
	}
}

func iosRestrictedProvider() DescriptorProvider {
	return fakeDescriptorProvider{
		desc: platform.RuntimeDescriptor{
			Host:         platform.HostPlatformIOS,
			Kind:         platform.RuntimeKindSandbox,
			Guest:        platform.GuestPlatformNone,
			Architecture: "arm64",
		},
		caps: runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
			runtimehost.CapProcessSpawn:    runtimehost.SupportUnsupported,
			runtimehost.CapFilesystemLocal: runtimehost.SupportUnsupported,
		}),
	}
}

func androidCaps() *runtimehost.HostCapabilities {
	return runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapProcessSpawn:    runtimehost.SupportSupported,
		runtimehost.CapFilesystemLocal: runtimehost.SupportSupported,
	})
}

func newTestResolver(t *testing.T, provider DescriptorProvider) Resolver {
	t.Helper()
	r, err := NewResolver(ResolveContext{DescriptorProvider: provider})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	return r
}

func TestResolver_AutoAndroidProotReturnsBalanced(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "auto")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileMobileBalanced {
		t.Errorf("ID = %q, want mobile-balanced", resolved.ID)
	}
	if !resolved.Mobile {
		t.Error("expected Mobile=true")
	}
	if resolved.Settings == nil {
		t.Error("expected non-nil settings")
	}
}

func TestResolver_AutoWindowsDesktopReturnsDesktop(t *testing.T) {
	r := newTestResolver(t, windowsDesktopProvider())
	resolved, err := r.Resolve(context.Background(), "auto")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileDesktopDefault {
		t.Errorf("ID = %q, want desktop-default", resolved.ID)
	}
	if resolved.Mobile {
		t.Error("expected Mobile=false")
	}
	if resolved.Settings != nil {
		t.Error("expected nil settings for desktop")
	}
}

func TestResolver_AutoLinuxDesktopReturnsDesktop(t *testing.T) {
	r := newTestResolver(t, linuxDesktopProvider())
	resolved, err := r.Resolve(context.Background(), "auto")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileDesktopDefault {
		t.Errorf("ID = %q, want desktop-default", resolved.ID)
	}
}

func TestResolver_AutoIOSRestrictedReturnsError(t *testing.T) {
	r := newTestResolver(t, iosRestrictedProvider())
	_, err := r.Resolve(context.Background(), "auto")
	if err == nil {
		t.Error("expected error for restricted runtime")
	}
}

func TestResolver_ExplicitCompact(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "mobile-compact")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileMobileCompact {
		t.Errorf("ID = %q, want mobile-compact", resolved.ID)
	}
	if !resolved.Mobile {
		t.Error("expected Mobile=true")
	}
	if resolved.Settings == nil {
		t.Error("expected non-nil settings")
	}
	if resolved.Settings.HNSWMemory != "cold" {
		t.Errorf("HNSWMemory = %q, want cold", resolved.Settings.HNSWMemory)
	}
}

func TestResolver_ExplicitBalanced(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "mobile-balanced")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileMobileBalanced {
		t.Errorf("ID = %q, want mobile-balanced", resolved.ID)
	}
	if resolved.Settings.WALCapacityMB != 16 {
		t.Errorf("WALCapacityMB = %d, want 16", resolved.Settings.WALCapacityMB)
	}
}

func TestResolver_ExplicitPerformance(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "mobile-performance")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileMobilePerformance {
		t.Errorf("ID = %q, want mobile-performance", resolved.ID)
	}
	if resolved.Settings.MaxIndexingThreads != 4 {
		t.Errorf("MaxIndexingThreads = %d, want 4", resolved.Settings.MaxIndexingThreads)
	}
}

func TestResolver_ExplicitDesktop(t *testing.T) {
	r := newTestResolver(t, windowsDesktopProvider())
	resolved, err := r.Resolve(context.Background(), "desktop-default")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileDesktopDefault {
		t.Errorf("ID = %q, want desktop-default", resolved.ID)
	}
	if resolved.Settings != nil {
		t.Error("expected nil settings for desktop-default")
	}
}

func TestResolver_RestrictedCantBypassExplicit(t *testing.T) {
	r := newTestResolver(t, iosRestrictedProvider())
	_, err := r.Resolve(context.Background(), "mobile-balanced")
	if err == nil {
		t.Error("expected error: restricted runtime cannot use explicit mobile profile")
	}
}

func TestResolver_UnknownProfileFails(t *testing.T) {
	r := newTestResolver(t, windowsDesktopProvider())
	_, err := r.Resolve(context.Background(), "invalid-profile")
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestResolver_EmptyStringIsAuto(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "  ")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != ProfileMobileBalanced {
		t.Errorf("empty string should resolve to auto behavior, got %q", resolved.ID)
	}
}

func TestResolver_ConstructorDoesNotReadDescriptor(t *testing.T) {
	_, err := NewResolver(ResolveContext{DescriptorProvider: androidProotProvider()})
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
}

func TestResolver_NilProviderConstructorFails(t *testing.T) {
	_, err := NewResolver(ResolveContext{DescriptorProvider: nil})
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestResolver_ConcurrentResolve(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = r.Resolve(context.Background(), "auto")
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestResolver_ContextCancel(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Resolve(ctx, "auto")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestResolver_SettingsDefensiveCopy(t *testing.T) {
	r := newTestResolver(t, androidProotProvider())
	resolved, err := r.Resolve(context.Background(), "mobile-balanced")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Settings == nil {
		t.Fatal("expected non-nil settings")
	}
	originalWAL := resolved.Settings.WALCapacityMB
	resolved.Settings.WALCapacityMB = 999

	resolved2, _ := r.Resolve(context.Background(), "mobile-balanced")
	if resolved2.Settings.WALCapacityMB != originalWAL {
		t.Error("settings should be defensive copy; mutation leaked")
	}
}
