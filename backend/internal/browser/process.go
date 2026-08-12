package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type browserProcess struct {
	execInfo    BrowserExecutable
	config      BrowserConfig
	profileDir  string
	cmd         *exec.Cmd
	mu          sync.Mutex
	alive       bool
	cdpEndpoint string
	connected   bool
}

func newBrowserProcess(execInfo BrowserExecutable, config BrowserConfig, profileDir string) *browserProcess {
	return &browserProcess{
		execInfo:   execInfo,
		config:     config,
		profileDir: profileDir,
	}
}

func (p *browserProcess) start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureProfileDir(); err != nil {
		return &BrowserError{
			Code:    ErrCodeBrowserStartFailed,
			Message: "failed to create browser profile directory",
			Cause:   err,
		}
	}

	args := p.buildArgs()
	cmd := exec.CommandContext(ctx, p.execInfo.Path, args...)

	cmd.Env = p.sanitizeEnv(cmd.Env)
	cmd.Dir = p.profileDir

	if err := cmd.Start(); err != nil {
		return &BrowserError{
			Code:    ErrCodeBrowserStartFailed,
			Message: "failed to start browser process",
			Cause:   err,
		}
	}

	p.cmd = cmd
	p.alive = true
	p.cdpEndpoint = ""
	p.connected = false

	return nil
}

func (p *browserProcess) connectCDP(ctx context.Context) error {
	p.mu.Lock()
	endpoint := p.cdpEndpoint
	p.mu.Unlock()

	if endpoint == "" {
		return fmt.Errorf("browser CDP endpoint not available")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}

	if time.Now().After(deadline) {
		return fmt.Errorf("CDP connection deadline exceeded")
	}

	p.mu.Lock()
	p.connected = true
	p.mu.Unlock()

	return nil
}

func (p *browserProcess) gracefulClose(ctx context.Context) error {
	p.mu.Lock()
	cmd := p.cmd
	alive := p.alive
	p.mu.Unlock()

	if cmd == nil || !alive {
		return nil
	}

	if runtime.GOOS == "windows" {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
	} else {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		return cmd.Process.Kill()
	case <-done:
		p.mu.Lock()
		p.alive = false
		p.connected = false
		p.mu.Unlock()
		return nil
	}
}

func (p *browserProcess) kill() {
	p.mu.Lock()
	cmd := p.cmd
	alive := p.alive
	p.mu.Unlock()

	if cmd == nil || !alive {
		return
	}

	if cmd.Process != nil {
		killProcessTree(cmd.Process)
	}

	p.mu.Lock()
	p.alive = false
	p.connected = false
	p.mu.Unlock()
}

func (p *browserProcess) isAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || !p.alive {
		return false
	}
	if p.cmd.Process == nil {
		return false
	}
	return isProcessAlive(p.cmd.Process)
}

func (p *browserProcess) cdpConnected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected && p.alive
}

func (p *browserProcess) ping() bool {
	return p.isAlive() && p.cdpConnected()
}

func (p *browserProcess) browserName() string {
	return p.execInfo.Kind
}

func (p *browserProcess) browserVersion() string {
	return p.execInfo.Version
}

func (p *browserProcess) cleanupProfile() {
	if p.profileDir == "" {
		return
	}
	if !isAmitiaOwnedPath(p.profileDir, p.config.UserDataRoot) {
		return
	}
	os.RemoveAll(p.profileDir)
}

func (p *browserProcess) ensureProfileDir() error {
	if p.profileDir == "" {
		return nil
	}
	return os.MkdirAll(p.profileDir, 0700)
}

func (p *browserProcess) buildArgs() []string {
	args := []string{
		"--disable-background-networking",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-sync",
		"--disable-translate",
		"--metrics-recording-only",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-features=TranslateUI",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--safebrowsing-disable-auto-update",
		"--disable-breakpad",
		"--disable-component-update",
		"--disable-hang-monitor",
		"--disable-popup-blocking",
		"--disable-prompt-on-postredirection",
		"--disable-client-side-phishing-detection",
		"--disable-ipc-flooding-protection",
		"--password-store=basic",
		"--use-mock-keychain",
		"--disable-dev-shm-usage",
	}

	if p.config.Headless {
		args = append(args, "--headless=new")
	}

	if p.profileDir != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", p.profileDir))
	}

	if p.config.MaxBrowserMemoryBytes > 0 {
		args = append(args,
			fmt.Sprintf("--js-flags=--max-old-space-size=%d", p.config.MaxBrowserMemoryBytes/(1024*1024)),
		)
	}

	args = append(args, "--remote-debugging-pipe")

	return args
}

func (p *browserProcess) sanitizeEnv(env []string) []string {
	safeEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !isForbiddenEnvVar(e) {
			safeEnv = append(safeEnv, e)
		}
	}
	return safeEnv
}

func isForbiddenEnvVar(env string) bool {
	forbidden := []string{
		"HTTP_PROXY=", "HTTPS_PROXY=", "NO_PROXY=",
		"http_proxy=", "https_proxy=", "no_proxy=",
	}
	for _, prefix := range forbidden {
		if len(env) >= len(prefix) && env[:len(prefix)+1] == prefix {
			return true
		}
	}
	return false
}

func profilePathFor(root string, generation uint64) string {
	return filepath.Join(root, fmt.Sprintf("runtime-%d", generation))
}
