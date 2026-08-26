//go:build linux && !android

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

func TestLinuxUnrestrictedNetworkSandboxE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("AMITIA_TEST_LINUX_UNRESTRICTED_NETWORK")) != "1" {
		t.Skip("set AMITIA_TEST_LINUX_UNRESTRICTED_NETWORK=1 to run the real bwrap+slirp4netns boundary")
	}
	if firstTrustedLauncher("/usr/bin/bwrap", "/bin/bwrap") == "" {
		t.Fatal("bubblewrap is required for unrestricted network sandbox E2E")
	}
	if firstTrustedLauncher("/usr/bin/slirp4netns", "/bin/slirp4netns") == "" {
		t.Fatal("slirp4netns is required for unrestricted network sandbox E2E")
	}
	curl := firstTrustedLauncher("/usr/bin/curl", "/bin/curl")
	if curl == "" {
		t.Fatal("curl is required for unrestricted network sandbox E2E")
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
	work := t.TempDir()
	plan, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:          "unrestricted",
		Enforce:       true,
		AllowOutbound: true,
	}, curl, []string{"--fail", "--silent", "--show-error", "--max-time", "10", fmt.Sprintf("http://10.0.2.2:%d/", port)}, work, work)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, plan.Path, plan.Args...)
	cmd.Dir = plan.WorkingDir
	cmd.ExtraFiles = append(cmd.ExtraFiles, plan.ExtraFiles...)
	process.ConfigureProcess(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sandbox: %v", err)
	}
	if plan.AfterStart != nil {
		if err := plan.AfterStart(ctx, cmd); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("activate sandbox: %v; stderr=%s", err, stderr.String())
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("sandboxed curl failed: %v; stderr=%s", err, stderr.String())
	}
	cancel() // terminate the slirp4netns sidecar after the sandboxed child exits.
	if strings.TrimSpace(stdout.String()) != "sandbox-ok" {
		t.Fatalf("unexpected sandbox response %q; stderr=%s", stdout.String(), stderr.String())
	}
}
