//go:build rsi_quota && rsi_profiles

package task_runtime

import (
	"errors"
	"fmt"
	"testing"
)

func TestQuotaProfileDefaultsPerDevice(t *testing.T) {
	mobile, err := DefaultQuotaProfile(ProfileMobileLite)
	if err != nil {
		t.Fatalf("mobile: %v", err)
	}
	desktop, err := DefaultQuotaProfile(ProfileDesktop)
	if err != nil {
		t.Fatalf("desktop: %v", err)
	}
	server, err := DefaultQuotaProfile(ProfileServer)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	if mobile.Quota.MemoryBytes != 128<<20 || mobile.ConcurrencyLimit() != 1 {
		t.Fatalf("mobile=%+v", mobile.Quota)
	}
	if desktop.Quota.MemoryBytes != 512<<20 || desktop.ConcurrencyLimit() != 4 {
		t.Fatalf("desktop=%+v", desktop.Quota)
	}
	if server.Quota.MemoryBytes != 2<<30 || server.ConcurrencyLimit() != 16 {
		t.Fatalf("server=%+v", server.Quota)
	}
	fmt.Printf("m07-s05 mobile memory_mb=128 concurrency=1\n")
	fmt.Printf("m07-s05 desktop memory_mb=512 concurrency=4\n")
	fmt.Printf("m07-s05 server memory_mb=2048 concurrency=16\n")
}

func TestQuotaProfileResolveDefaults(t *testing.T) {
	spec, err := ResolveQuotaProfile(DeviceMobile, "", ProfileGrant{})
	if err != nil {
		t.Fatalf("resolve mobile: %v", err)
	}
	if spec.Profile != ProfileMobileLite || spec.Device != DeviceMobile {
		t.Fatalf("spec=%+v", spec)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	fmt.Printf("m07-s05 resolve mobile=mobile-lite\n")
}

func TestQuotaProfileMobileDeniesServer(t *testing.T) {
	_, err := ResolveQuotaProfile(DeviceMobile, ProfileServer, ProfileGrant{Trusted: true, Actor: "admin"})
	if !errors.Is(err, ErrQuotaProfileDenied) {
		t.Fatalf("mobile server err=%v", err)
	}
	fmt.Printf("m07-s05 deny mobile_server=true\n")
}

func TestQuotaProfileRequiresGrantForNonDefault(t *testing.T) {
	if _, err := ResolveQuotaProfile(DeviceDesktop, ProfileServer, ProfileGrant{Actor: "plugin"}); !errors.Is(err, ErrQuotaProfileUnauthorized) {
		t.Fatalf("ungranted err=%v", err)
	}
	spec, err := ResolveQuotaProfile(DeviceDesktop, ProfileServer, ProfileGrant{Trusted: true, Actor: "admin"})
	if err != nil {
		t.Fatalf("granted: %v", err)
	}
	if spec.Profile != ProfileServer || spec.Device != DeviceDesktop {
		t.Fatalf("spec=%+v", spec)
	}
	fmt.Printf("m07-s05 unauthorized=true granted server=true\n")
}

func TestQuotaProfileRejectsUnknownDeviceAndProfile(t *testing.T) {
	if _, err := ResolveQuotaProfile(DeviceClass("watch"), "", ProfileGrant{}); !errors.Is(err, ErrQuotaProfileUnknown) {
		t.Fatalf("device err=%v", err)
	}
	if _, err := DefaultQuotaProfile(QuotaProfile("gpu")); !errors.Is(err, ErrQuotaProfileUnknown) {
		t.Fatalf("profile err=%v", err)
	}
	fmt.Printf("m07-s05 unknown device=true profile=true\n")
}

func TestQuotaProfileConfigureRequiresGrant(t *testing.T) {
	spec, err := DefaultQuotaProfile(ProfileDesktop)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	before := spec.Ceiling
	_, err = ConfigureQuotaCeiling(spec, QuotaCeiling{MemoryBytes: 256 << 20, CPUPercent: 50, Processes: spec.Ceiling.Processes, Concurrency: 2, DiskBytes: 128 << 20}, ProfileGrant{})
	if !errors.Is(err, ErrQuotaProfileUnauthorized) {
		t.Fatalf("configure err=%v", err)
	}
	if spec.Ceiling != before {
		t.Fatalf("spec mutated=%+v", spec.Ceiling)
	}
	fmt.Printf("m07-s05 configure unauthorized=true unchanged=true\n")
}

func TestQuotaProfileConfigureClampsQuota(t *testing.T) {
	spec, err := DefaultQuotaProfile(ProfileDesktop)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	configured, err := ConfigureQuotaCeiling(spec, QuotaCeiling{MemoryBytes: 256 << 20, CPUPercent: 50, Processes: spec.Ceiling.Processes, Concurrency: 2, DiskBytes: 128 << 20}, ProfileGrant{Trusted: true, Actor: "admin"})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if configured.Quota.MemoryBytes != 256<<20 || configured.Quota.Processes != spec.Ceiling.Processes || configured.Quota.Concurrency != 2 || configured.Quota.DiskBytes != 128<<20 {
		t.Fatalf("quota=%+v", configured.Quota)
	}
	if configured.Ceiling.MemoryBytes != 256<<20 {
		t.Fatalf("ceiling=%+v", configured.Ceiling)
	}
	fmt.Printf("m07-s05 configured quota_mb=256 concurrency=2\n")
}

func TestQuotaProfileConfigureRejectsAboveAbsolute(t *testing.T) {
	spec, err := DefaultQuotaProfile(ProfileDesktop)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	_, err = ConfigureQuotaCeiling(spec, QuotaCeiling{MemoryBytes: 9 << 30, CPUPercent: 50, Processes: 8, Concurrency: 2, DiskBytes: 128 << 20}, ProfileGrant{Trusted: true, Actor: "admin"})
	if !errors.Is(err, ErrQuotaProfileInvalid) {
		t.Fatalf("oversize err=%v", err)
	}
	fmt.Printf("m07-s05 oversize rejected=true\n")
}

func TestQuotaProfileValidateRejectsQuotaAboveCeiling(t *testing.T) {
	spec := QuotaProfileSpec{
		Profile: ProfileDesktop,
		Device:  DeviceDesktop,
		Quota:   ResourceQuota{MemoryBytes: 1 << 30, CPUPercent: 50, Processes: 8, Concurrency: 2, DiskBytes: 128 << 20},
		Ceiling: QuotaCeiling{MemoryBytes: 512 << 20, CPUPercent: 50, Processes: 8, Concurrency: 2, DiskBytes: 128 << 20},
	}
	if err := spec.Validate(); !errors.Is(err, ErrQuotaProfileInvalid) {
		t.Fatalf("validate err=%v", err)
	}
	fmt.Printf("m07-s05 validate quota_exceeds=true\n")
}

func TestQuotaProfileMobileCeilingBounds(t *testing.T) {
	spec, err := ResolveQuotaProfile(DeviceMobile, "", ProfileGrant{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if spec.Ceiling.Concurrency > 2 || spec.Quota.Concurrency > spec.Ceiling.Concurrency {
		t.Fatalf("mobile ceiling=%+v quota=%+v", spec.Ceiling, spec.Quota)
	}
	fmt.Printf("m07-s05 mobile_ceiling concurrency<=2\n")
}
