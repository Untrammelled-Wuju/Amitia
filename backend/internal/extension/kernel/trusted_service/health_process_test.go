package trusted_service

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestTrustedServiceHealthRecognizesLivePlatformProcess(t *testing.T) {
	monitor := NewHealthMonitor()
	instance := &ServiceInstance{PID: os.Getpid()}
	for _, kind := range []string{"process", "rpc", "unknown"} {
		definition := &ServiceRuntimeDefinition{HealthCheck: ServiceHealthCheck{Type: kind}}
		if !monitor.check(t.Context(), instance, definition) {
			t.Fatalf("live host process was declared unhealthy for %s", kind)
		}
	}
	for _, pid := range []int{0, -1} {
		if procIsAlive(pid) {
			t.Fatal("invalid process was declared alive")
		}
	}
}

func TestTrustedServiceExitedHealthProcess(t *testing.T) {
	if os.Getenv("AMITIA_TRUSTED_HEALTH_HELPER") == "1" {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestTrustedServiceExitedHealthProcess$")
	command.Env = append(os.Environ(), "AMITIA_TRUSTED_HEALTH_HELPER=1")
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	monitor := NewHealthMonitor()
	instance := &ServiceInstance{PID: command.Process.Pid}
	if monitor.check(t.Context(), instance, &ServiceRuntimeDefinition{HealthCheck: ServiceHealthCheck{Type: "process"}}) {
		t.Fatal("reaped process was declared healthy")
	}
}
