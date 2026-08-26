//go:build darwin && !ios

package trusted_service

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestDarwinNetworkSandboxE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("AMITIA_TEST_DARWIN_NETWORK_SANDBOX")) != "1" {
		t.Skip("set AMITIA_TEST_DARWIN_NETWORK_SANDBOX=1 to run the native Seatbelt network boundary")
	}
	if firstTrustedLauncher("/usr/bin/sandbox-exec") == "" {
		t.Fatal("/usr/bin/sandbox-exec is required for macOS network sandbox E2E")
	}
	curl := firstTrustedLauncher("/usr/bin/curl")
	if curl == "" {
		t.Fatal("/usr/bin/curl is required for macOS network sandbox E2E")
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("sandbox-ok"))
	})}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		_ = server.Close()
		select {
		case serveErr := <-serveDone:
			if serveErr != nil && serveErr != http.ErrServerClosed {
				t.Errorf("test server failed: %v", serveErr)
			}
		case <-time.After(2 * time.Second):
			t.Error("test server did not stop")
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)

	t.Run("unrestricted permits outbound", func(t *testing.T) {
		stdout, stderr, runErr := runDarwinSandboxProbe(t, ServiceNetworkPolicy{
			Mode:          "unrestricted",
			Enforce:       true,
			AllowOutbound: true,
		}, curl, []string{"--fail", "--silent", "--show-error", "--max-time", "5", url})
		if runErr != nil {
			t.Fatalf("unrestricted sandboxed curl failed: %v; stderr=%s", runErr, stderr)
		}
		if strings.TrimSpace(stdout) != "sandbox-ok" {
			t.Fatalf("unexpected unrestricted response %q; stderr=%s", stdout, stderr)
		}
	})

	t.Run("restricted denies ambient sockets", func(t *testing.T) {
		_, stderr, runErr := runDarwinSandboxProbe(t, ServiceNetworkPolicy{
			Mode:         "restricted",
			Enforce:      true,
			RequireProxy: true,
			AllowedIPs:   []string{"127.0.0.1"},
			AllowedPorts: []int{port},
		}, curl, []string{"--fail", "--silent", "--show-error", "--max-time", "3", url})
		if runErr == nil {
			t.Fatalf("restricted sandbox unexpectedly allowed a direct socket; stderr=%s", stderr)
		}
	})
}

func runDarwinSandboxProbe(t *testing.T, policy ServiceNetworkPolicy, executable string, args []string) (string, string, error) {
	t.Helper()
	work := t.TempDir()
	plan, err := prepareNetworkLaunch(policy, executable, args, work, work)
	if err != nil {
		return "", "", err
	}
	defer plan.cleanup()
	if !plan.FilesystemIsolated || !plan.NetworkPolicyEnforced {
		t.Fatalf("sandbox launch plan is not fully enforced: %+v", plan)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, plan.Path, plan.Args...)
	cmd.Dir = plan.WorkingDir
	process.ConfigureProcess(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return stdout.String(), stderr.String(), err
	}
	if plan.AfterStart != nil {
		if err := plan.AfterStart(ctx, cmd); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return stdout.String(), stderr.String(), err
		}
	}
	return stdout.String(), stderr.String(), cmd.Wait()
}
