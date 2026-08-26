//go:build windows

package trusted_service

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestWindowsNetworkSandboxE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("AMITIA_TEST_WINDOWS_NETWORK_SANDBOX")) != "1" {
		t.Skip("set AMITIA_TEST_WINDOWS_NETWORK_SANDBOX=1 to run the native AppContainer network boundary")
	}
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		t.Fatal("SystemRoot is required for Windows network sandbox E2E")
	}
	curl := filepath.Join(systemRoot, "System32", "curl.exe")
	if info, err := os.Stat(curl); err != nil || info.IsDir() {
		t.Fatalf("Windows curl.exe is required for network sandbox E2E: %v", err)
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
		stdout, stderr, runErr := runWindowsSandboxProbe(t, ServiceNetworkPolicy{
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
		_, stderr, runErr := runWindowsSandboxProbe(t, ServiceNetworkPolicy{
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

	t.Run("forced launcher termination leaves recoverable durable state", func(t *testing.T) {
		ping := filepath.Join(systemRoot, "System32", "PING.EXE")
		if info, statErr := os.Stat(ping); statErr != nil || info.IsDir() {
			t.Fatalf("Windows ping.exe is required for residue recovery E2E: %v", statErr)
		}
		work := t.TempDir()
		stateRoot := t.TempDir()
		plan, planErr := prepareNetworkLaunchWithStateRoot(ServiceNetworkPolicy{
			Mode:          "loopback",
			Enforce:       true,
			AllowOutbound: true,
			LoopbackOnly:  true,
		}, ping, []string{"-n", "30", "127.0.0.1"}, work, work, stateRoot)
		if planErr != nil {
			t.Fatal(planErr)
		}
		defer plan.cleanup()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, plan.Path, plan.Args...)
		cmd.Dir = plan.WorkingDir
		process.ConfigureProcess(cmd)
		if startErr := cmd.Start(); startErr != nil {
			t.Fatal(startErr)
		}
		handle, attachErr := process.AttachProcessTreeWithLimits(cmd, process.ResourceLimits{})
		if attachErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatal(attachErr)
		}
		if plan.AfterStart != nil {
			if startErr := plan.AfterStart(ctx, cmd); startErr != nil {
				_ = process.TerminateProcessTree(cmd.Process.Pid, handle)
				process.CloseProcessTree(handle)
				_ = cmd.Wait()
				t.Fatal(startErr)
			}
		}
		time.Sleep(750 * time.Millisecond)
		// Simulate a host /F termination: the wrapper is killed before its finally
		// block can revoke ACL/loopback/profile state. Closing the job then removes
		// the AppContainer child, matching the production kill-on-close boundary.
		_ = cmd.Process.Kill()
		process.CloseProcessTree(handle)
		_ = cmd.Wait()

		stateDir, dirErr := windowsSandboxStateDir(stateRoot)
		if dirErr != nil {
			t.Fatal(dirErr)
		}
		stateFile := filepath.Join(stateDir, windowsSandboxProfileName(work, work)+".json")
		if _, statErr := os.Stat(stateFile); statErr != nil {
			t.Fatalf("forced termination did not preserve durable sandbox state: %v", statErr)
		}
		if recoverErr := recoverPlatformSandboxResidue(stateRoot); recoverErr != nil {
			t.Fatalf("recoverPlatformSandboxResidue() failed: %v", recoverErr)
		}
		if _, statErr := os.Stat(stateFile); !os.IsNotExist(statErr) {
			t.Fatalf("sandbox state remains after recovery: %v", statErr)
		}
	})
}

func runWindowsSandboxProbe(t *testing.T, policy ServiceNetworkPolicy, executable string, args []string) (string, string, error) {
	t.Helper()
	work := t.TempDir()
	stateRoot := t.TempDir()
	plan, err := prepareNetworkLaunchWithStateRoot(policy, executable, args, work, work, stateRoot)
	if err != nil {
		return "", "", err
	}
	defer plan.cleanup()
	if !plan.FilesystemIsolated || !plan.NetworkPolicyEnforced {
		t.Fatalf("sandbox launch plan is not fully enforced: %+v", plan)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	// Windows ConfigureProcess deliberately creates the trusted PowerShell
	// launcher suspended. Attach it to the same kill-on-close Job Object used by
	// production before resuming; otherwise this smoke test would never execute
	// the AppContainer child and could not validate the real production boundary.
	handle, err := process.AttachProcessTreeWithLimits(cmd, process.ResourceLimits{})
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return stdout.String(), stderr.String(), err
	}
	defer process.CloseProcessTree(handle)
	if plan.AfterStart != nil {
		if err := plan.AfterStart(ctx, cmd); err != nil {
			_ = process.TerminateProcessTree(cmd.Process.Pid, handle)
			_ = cmd.Wait()
			return stdout.String(), stderr.String(), err
		}
	}
	waitErr := cmd.Wait()
	stateDir, stateErr := windowsSandboxStateDir(stateRoot)
	if stateErr != nil {
		t.Fatalf("resolve sandbox state dir: %v", stateErr)
	}
	stateFile := filepath.Join(stateDir, windowsSandboxProfileName(work, work)+".json")
	if _, statErr := os.Stat(stateFile); !os.IsNotExist(statErr) {
		t.Fatalf("normal sandbox exit left durable state %q: %v", stateFile, statErr)
	}
	return stdout.String(), stderr.String(), waitErr
}
