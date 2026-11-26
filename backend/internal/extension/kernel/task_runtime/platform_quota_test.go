//go:build rsi_platformquota

package task_runtime

import (
	"errors"
	"fmt"
	"testing"
)

func TestPlatformQuotaAndroidFull(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{Memory: true, CPU: true, Processes: true, FileDescriptors: true, Background: true}, PlatformQuotaRequest{MemoryBytes: 256 << 20, CPUPercent: 50, ActiveProcesses: 4})
	if err != nil || decision.Level != QuotaLevelFull || len(decision.Unsupported) != 0 || decision.Delegated {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 android level=full\n")
}

func TestPlatformQuotaAndroidLimitedCPU(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{Memory: true, Processes: true, FileDescriptors: true, Background: true}, PlatformQuotaRequest{CPUPercent: 50})
	if err != nil || decision.Level != QuotaLevelLimited || len(decision.Unsupported) != 1 || decision.Unsupported[0] != "cpuPercent" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 android level=limited unsupported=cpuPercent\n")
}

func TestPlatformQuotaAndroidRemote(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{}, PlatformQuotaRequest{MemoryBytes: 64 << 20, CPUPercent: 25})
	if err != nil || decision.Level != QuotaLevelRemote || !decision.Delegated || decision.ConditionString() != "network-required,owner-consent" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 android level=remote delegated=true\n")
}

func TestPlatformQuotaIOSAlwaysRemote(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformIOS, QuotaLevelFull, PlatformQuotaCapability{Memory: true, CPU: true, Processes: true, FileDescriptors: true, Background: true}, PlatformQuotaRequest{MemoryBytes: 256 << 20, CPUPercent: 50, ActiveProcesses: 4})
	if err != nil || decision.Level != QuotaLevelRemote || len(decision.Unsupported) != 3 {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 ios level=remote unsupported=3\n")
}

func TestPlatformQuotaIOSRemoteRequest(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformIOS, QuotaLevelRemote, PlatformQuotaCapability{}, PlatformQuotaRequest{})
	if err != nil || decision.Level != QuotaLevelRemote || decision.ConditionString() != "network-required,owner-consent" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 ios conditions=network-required,owner-consent\n")
}

func TestPlatformQuotaDesktopFull(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformDesktop, QuotaLevelFull, PlatformQuotaCapability{Memory: true, CPU: true, Processes: true, FileDescriptors: true, Background: true}, PlatformQuotaRequest{MemoryBytes: 512 << 20})
	if err != nil || decision.Level != QuotaLevelFull || decision.Delegated {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 desktop level=full\n")
}

func TestPlatformQuotaInvalidPlatform(t *testing.T) {
	if _, err := NegotiatePlatformQuota(PlatformQuotaPlatform("watchos"), QuotaLevelFull, PlatformQuotaCapability{}, PlatformQuotaRequest{}); !errors.Is(err, ErrPlatformQuotaInvalid) {
		t.Fatalf("platform err=%v", err)
	}
	fmt.Printf("m07-s12 platform rejected=true\n")
}

func TestPlatformQuotaInvalidLevel(t *testing.T) {
	if _, err := NegotiatePlatformQuota(PlatformDesktop, PlatformQuotaLevel("turbo"), PlatformQuotaCapability{}, PlatformQuotaRequest{}); !errors.Is(err, ErrPlatformQuotaInvalid) {
		t.Fatalf("level err=%v", err)
	}
	fmt.Printf("m07-s12 level rejected=true\n")
}

func TestPlatformQuotaZeroRequestFull(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformAndroid, "", PlatformQuotaCapability{Memory: true, CPU: true, Processes: true, FileDescriptors: true, Background: true}, PlatformQuotaRequest{})
	if err != nil || decision.Level != QuotaLevelFull {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 zero_request level=full\n")
}

func TestPlatformQuotaBackgroundOnlyMissing(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{Memory: true, CPU: true, Processes: true, FileDescriptors: true}, PlatformQuotaRequest{})
	if err != nil || decision.Level != QuotaLevelLimited || decision.Unsupported[0] != "background" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 background unsupported=true\n")
}

func TestPlatformQuotaRemoteRequestOnDesktop(t *testing.T) {
	decision, err := NegotiatePlatformQuota(PlatformDesktop, QuotaLevelRemote, PlatformQuotaCapability{Memory: true}, PlatformQuotaRequest{})
	if err != nil || decision.Level != QuotaLevelRemote || !decision.Delegated {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	fmt.Printf("m07-s12 remote_requested delegated=true\n")
}

func TestPlatformQuotaInvalidRequest(t *testing.T) {
	if _, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{}, PlatformQuotaRequest{MemoryBytes: -1}); !errors.Is(err, ErrPlatformQuotaInvalid) {
		t.Fatalf("request err=%v", err)
	}
	if _, err := NegotiatePlatformQuota(PlatformAndroid, QuotaLevelFull, PlatformQuotaCapability{}, PlatformQuotaRequest{CPUPercent: 101}); !errors.Is(err, ErrPlatformQuotaInvalid) {
		t.Fatalf("cpu err=%v", err)
	}
	fmt.Printf("m07-s12 request rejected=true\n")
}
