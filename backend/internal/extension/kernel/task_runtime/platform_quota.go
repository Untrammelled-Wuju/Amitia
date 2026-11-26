//go:build rsi_platformquota

package task_runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const MaxPlatformQuotaProcesses = 256

var ErrPlatformQuotaInvalid = errors.New("task_runtime: platform quota invalid")

type PlatformQuotaPlatform string

const (
	PlatformAndroid PlatformQuotaPlatform = "android"
	PlatformIOS     PlatformQuotaPlatform = "ios"
	PlatformDesktop PlatformQuotaPlatform = "desktop"
)

type PlatformQuotaLevel string

const (
	QuotaLevelFull    PlatformQuotaLevel = "full"
	QuotaLevelLimited PlatformQuotaLevel = "limited"
	QuotaLevelRemote  PlatformQuotaLevel = "remote"
)

type PlatformQuotaCapability struct {
	Memory          bool
	CPU             bool
	Processes       bool
	FileDescriptors bool
	Background      bool
}

type PlatformQuotaRequest struct {
	MemoryBytes     int64
	CPUPercent      uint32
	ActiveProcesses uint32
}

type PlatformQuotaDecision struct {
	Platform    PlatformQuotaPlatform
	Level       PlatformQuotaLevel
	Delegated   bool
	Unsupported []string
	Conditions  []string
}

func (p PlatformQuotaPlatform) valid() bool {
	switch p {
	case PlatformAndroid, PlatformIOS, PlatformDesktop:
		return true
	default:
		return false
	}
}

func (l PlatformQuotaLevel) valid() bool {
	switch l {
	case QuotaLevelFull, QuotaLevelLimited, QuotaLevelRemote:
		return true
	default:
		return false
	}
}

func NegotiatePlatformQuota(platform PlatformQuotaPlatform, requested PlatformQuotaLevel, capability PlatformQuotaCapability, request PlatformQuotaRequest) (PlatformQuotaDecision, error) {
	if !platform.valid() {
		return PlatformQuotaDecision{}, fmt.Errorf("%w: platform=%q", ErrPlatformQuotaInvalid, platform)
	}
	if requested == "" {
		requested = QuotaLevelFull
	}
	if !requested.valid() {
		return PlatformQuotaDecision{}, fmt.Errorf("%w: level=%q", ErrPlatformQuotaInvalid, requested)
	}
	if request.MemoryBytes < 0 || request.CPUPercent > 100 || request.ActiveProcesses > MaxPlatformQuotaProcesses {
		return PlatformQuotaDecision{}, fmt.Errorf("%w: request memory=%d cpu=%d processes=%d", ErrPlatformQuotaInvalid, request.MemoryBytes, request.CPUPercent, request.ActiveProcesses)
	}
	if platform == PlatformIOS {
		return PlatformQuotaDecision{
			Platform:    platform,
			Level:       QuotaLevelRemote,
			Delegated:   true,
			Unsupported: []string{"activeProcesses", "cpuPercent", "memoryBytes"},
			Conditions:  []string{"network-required", "owner-consent"},
		}, nil
	}
	unsupported := make([]string, 0, 5)
	if request.MemoryBytes > 0 && !capability.Memory {
		unsupported = append(unsupported, "memoryBytes")
	}
	if request.CPUPercent > 0 && !capability.CPU {
		unsupported = append(unsupported, "cpuPercent")
	}
	if request.ActiveProcesses > 0 && !capability.Processes {
		unsupported = append(unsupported, "activeProcesses")
	}
	if !capability.FileDescriptors {
		unsupported = append(unsupported, "fileDescriptors")
	}
	if !capability.Background {
		unsupported = append(unsupported, "background")
	}
	sort.Strings(unsupported)
	decision := PlatformQuotaDecision{Platform: platform, Unsupported: unsupported}
	switch {
	case requested == QuotaLevelRemote:
		decision.Level = QuotaLevelRemote
		decision.Delegated = true
		decision.Conditions = []string{"network-required", "owner-consent"}
	case len(unsupported) == 0:
		decision.Level = QuotaLevelFull
	case capability.Memory || capability.CPU || capability.Processes:
		decision.Level = QuotaLevelLimited
	default:
		decision.Level = QuotaLevelRemote
		decision.Delegated = true
		decision.Conditions = []string{"network-required", "owner-consent"}
	}
	return decision, nil
}

func (d PlatformQuotaDecision) ConditionString() string {
	return strings.Join(d.Conditions, ",")
}
