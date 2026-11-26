//go:build rsi_quota && rsi_profiles

package task_runtime

import (
	"errors"
	"fmt"
)

var (
	ErrQuotaProfileUnknown      = errors.New("task_runtime: quota profile unknown")
	ErrQuotaProfileDenied       = errors.New("task_runtime: quota profile denied for device")
	ErrQuotaProfileUnauthorized = errors.New("task_runtime: quota profile change unauthorized")
	ErrQuotaProfileInvalid      = errors.New("task_runtime: quota profile invalid")
)

type QuotaProfile string

const (
	ProfileMobileLite QuotaProfile = "mobile-lite"
	ProfileDesktop    QuotaProfile = "desktop"
	ProfileServer     QuotaProfile = "server"
)

func (p QuotaProfile) valid() bool {
	switch p {
	case ProfileMobileLite, ProfileDesktop, ProfileServer:
		return true
	default:
		return false
	}
}

type DeviceClass string

const (
	DeviceMobile  DeviceClass = "mobile"
	DeviceDesktop DeviceClass = "desktop"
	DeviceServer  DeviceClass = "server"
)

func (d DeviceClass) valid() bool {
	switch d {
	case DeviceMobile, DeviceDesktop, DeviceServer:
		return true
	default:
		return false
	}
}

func (d DeviceClass) defaultProfile() QuotaProfile {
	switch d {
	case DeviceMobile:
		return ProfileMobileLite
	case DeviceServer:
		return ProfileServer
	default:
		return ProfileDesktop
	}
}

type ProfileGrant struct {
	Trusted bool
	Actor   string
}

func (g ProfileGrant) authorize() error {
	if !g.Trusted {
		return fmt.Errorf("%w: actor=%q", ErrQuotaProfileUnauthorized, g.Actor)
	}
	return nil
}

type QuotaProfileSpec struct {
	Profile QuotaProfile
	Device  DeviceClass
	Quota   ResourceQuota
	Ceiling QuotaCeiling
}

func AbsoluteQuotaMax() QuotaCeiling {
	return QuotaCeiling{
		MemoryBytes: 8 << 30,
		CPUPercent:  800,
		Processes:   256,
		Concurrency: 64,
		DiskBytes:   4 << 30,
	}
}

func DefaultQuotaProfiles() map[QuotaProfile]QuotaProfileSpec {
	return map[QuotaProfile]QuotaProfileSpec{
		ProfileMobileLite: {
			Profile: ProfileMobileLite,
			Device:  DeviceMobile,
			Quota:   ResourceQuota{MemoryBytes: 128 << 20, CPUPercent: 25, Processes: 4, Concurrency: 1, DiskBytes: 64 << 20},
			Ceiling: QuotaCeiling{MemoryBytes: 256 << 20, CPUPercent: 50, Processes: 8, Concurrency: 2, DiskBytes: 128 << 20},
		},
		ProfileDesktop: desktopQuotaProfile(),
		ProfileServer: {
			Profile: ProfileServer,
			Device:  DeviceServer,
			Quota:   ResourceQuota{MemoryBytes: 2 << 30, CPUPercent: 200, Processes: 64, Concurrency: 16, DiskBytes: 1 << 30},
			Ceiling: QuotaCeiling{MemoryBytes: 4 << 30, CPUPercent: 400, Processes: 128, Concurrency: 32, DiskBytes: 2 << 30},
		},
	}
}

func desktopQuotaProfile() QuotaProfileSpec {
	ceiling := DefaultQuotaCeiling()
	return QuotaProfileSpec{
		Profile: ProfileDesktop,
		Device:  DeviceDesktop,
		Quota: ResourceQuota{
			MemoryBytes: ceiling.MemoryBytes,
			CPUPercent:  ceiling.CPUPercent,
			Processes:   ceiling.Processes,
			Concurrency: ceiling.Concurrency,
			DiskBytes:   ceiling.DiskBytes,
		},
		Ceiling: ceiling,
	}
}

func (s QuotaProfileSpec) Validate() error {
	if !s.Profile.valid() {
		return fmt.Errorf("%w: profile=%q", ErrQuotaProfileInvalid, s.Profile)
	}
	if !s.Device.valid() {
		return fmt.Errorf("%w: device=%q", ErrQuotaProfileInvalid, s.Device)
	}
	if err := s.Ceiling.Validate(); err != nil {
		return fmt.Errorf("%w: ceiling: %v", ErrQuotaProfileInvalid, err)
	}
	absolute := AbsoluteQuotaMax()
	if s.Ceiling.MemoryBytes > absolute.MemoryBytes || s.Ceiling.CPUPercent > absolute.CPUPercent || s.Ceiling.Processes > absolute.Processes || s.Ceiling.Concurrency > absolute.Concurrency || s.Ceiling.DiskBytes > absolute.DiskBytes {
		return fmt.Errorf("%w: ceiling exceeds absolute maximum", ErrQuotaProfileInvalid)
	}
	if s.Quota.MemoryBytes > s.Ceiling.MemoryBytes || s.Quota.CPUPercent > s.Ceiling.CPUPercent || s.Quota.Processes > s.Ceiling.Processes || s.Quota.Concurrency > s.Ceiling.Concurrency || s.Quota.DiskBytes > s.Ceiling.DiskBytes {
		return fmt.Errorf("%w: quota exceeds profile ceiling", ErrQuotaProfileInvalid)
	}
	if violations := ValidateQuota(s.Quota); len(violations) > 0 {
		return &QuotaRejection{Violations: violations}
	}
	return nil
}

func DefaultQuotaProfile(profile QuotaProfile) (QuotaProfileSpec, error) {
	spec, ok := DefaultQuotaProfiles()[profile]
	if !ok {
		return QuotaProfileSpec{}, fmt.Errorf("%w: profile=%q", ErrQuotaProfileUnknown, profile)
	}
	if err := spec.Validate(); err != nil {
		return QuotaProfileSpec{}, err
	}
	return spec, nil
}

func ResolveQuotaProfile(device DeviceClass, requested QuotaProfile, grant ProfileGrant) (QuotaProfileSpec, error) {
	if !device.valid() {
		return QuotaProfileSpec{}, fmt.Errorf("%w: device=%q", ErrQuotaProfileUnknown, device)
	}
	target := requested
	if target == "" {
		target = device.defaultProfile()
	}
	if !target.valid() {
		return QuotaProfileSpec{}, fmt.Errorf("%w: profile=%q", ErrQuotaProfileUnknown, target)
	}
	if device == DeviceMobile && target == ProfileServer {
		return QuotaProfileSpec{}, fmt.Errorf("%w: device=%s profile=%s", ErrQuotaProfileDenied, device, target)
	}
	if target != device.defaultProfile() {
		if err := grant.authorize(); err != nil {
			return QuotaProfileSpec{}, err
		}
	}
	spec, err := DefaultQuotaProfile(target)
	if err != nil {
		return QuotaProfileSpec{}, err
	}
	spec.Device = device
	if err := spec.Validate(); err != nil {
		return QuotaProfileSpec{}, err
	}
	return spec, nil
}

func ConfigureQuotaCeiling(spec QuotaProfileSpec, configured QuotaCeiling, grant ProfileGrant) (QuotaProfileSpec, error) {
	if err := grant.authorize(); err != nil {
		return QuotaProfileSpec{}, err
	}
	if err := spec.Validate(); err != nil {
		return QuotaProfileSpec{}, err
	}
	absolute := AbsoluteQuotaMax()
	if configured.MemoryBytes > absolute.MemoryBytes || configured.CPUPercent > absolute.CPUPercent || configured.Processes > absolute.Processes || configured.Concurrency > absolute.Concurrency || configured.DiskBytes > absolute.DiskBytes {
		return QuotaProfileSpec{}, fmt.Errorf("%w: configured ceiling exceeds absolute maximum", ErrQuotaProfileInvalid)
	}
	if err := configured.Validate(); err != nil {
		return QuotaProfileSpec{}, fmt.Errorf("%w: configured ceiling: %v", ErrQuotaProfileInvalid, err)
	}
	normalized, _, err := NormalizeQuota(spec.Quota, configured)
	if err != nil {
		return QuotaProfileSpec{}, err
	}
	updated := spec
	updated.Ceiling = configured
	updated.Quota = normalized
	if err := updated.Validate(); err != nil {
		return QuotaProfileSpec{}, err
	}
	return updated, nil
}

func (s QuotaProfileSpec) ConcurrencyLimit() int64 {
	return s.Quota.Concurrency
}
