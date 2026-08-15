package browser

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	proc "github.com/u-ai/backend/internal/platform/process"
)

type browserProcess struct {
	execInfo   BrowserExecutable
	config     BrowserConfig
	profileDir string
	cdpClient  *cdpClient
	mu         sync.Mutex
	alive      bool
	connected  bool
	pid        int
	procHandle proc.ProcessTreeHandle
	reader     io.ReadCloser
	writer     io.WriteCloser
}

func newBrowserProcess(execInfo BrowserExecutable, config BrowserConfig, profileDir string) *browserProcess {
	return &browserProcess{
		execInfo:   execInfo,
		config:     config,
		profileDir: profileDir,
	}
}

func (p *browserProcess) connectCDP(ctx context.Context) error {
	p.mu.Lock()
	reader := p.reader
	writer := p.writer
	p.mu.Unlock()

	if reader == nil || writer == nil {
		return fmt.Errorf("browser CDP pipes not available")
	}

	transport := newCDPPipeTransport(reader, writer)
	client := newCDPClient(transport)

	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			client.Close()
			return fmt.Errorf("CDP health check timed out")
		}

		var versionResult struct {
			UserAgent string `json:"userAgent"`
			Version   string `json:"version"`
		}
		hctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := client.Call(hctx, "Browser.getVersion", "", nil, &versionResult)
		cancel()

		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			client.Close()
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	p.mu.Lock()
	p.cdpClient = client
	p.connected = true
	p.mu.Unlock()

	client.OnDisconnect(func(err error) {
		p.mu.Lock()
		p.connected = false
		p.alive = false
		p.mu.Unlock()
	})

	return nil
}

func (p *browserProcess) cdp() *cdpClient {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cdpClient
}

func (p *browserProcess) gracefulClose(ctx context.Context) error {
	p.mu.Lock()
	client := p.cdpClient
	alive := p.alive
	reader := p.reader
	writer := p.writer
	p.mu.Unlock()

	if !alive {
		return nil
	}

	if client != nil {
		client.Close()
	}
	if reader != nil {
		reader.Close()
	}
	if writer != nil {
		writer.Close()
	}

	p.mu.Lock()
	p.alive = false
	p.connected = false
	p.mu.Unlock()

	return nil
}

func (p *browserProcess) kill() {
	p.mu.Lock()
	client := p.cdpClient
	reader := p.reader
	writer := p.writer
	handle := p.procHandle
	alive := p.alive
	p.mu.Unlock()

	if !alive {
		return
	}

	if client != nil {
		client.Close()
	}
	if reader != nil {
		reader.Close()
	}
	if writer != nil {
		writer.Close()
	}

	if handle != 0 {
		killProcessTreeHandle(handle)
	}

	p.mu.Lock()
	p.alive = false
	p.connected = false
	p.mu.Unlock()
}

func (p *browserProcess) isAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.alive || p.pid == 0 {
		return false
	}
	return isProcessAliveByPID(p.pid)
}

func (p *browserProcess) cdpConnected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected && p.alive
}

func (p *browserProcess) ping() bool {
	p.mu.Lock()
	client := p.cdpClient
	connected := p.connected
	p.mu.Unlock()

	if !connected || client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result struct {
		UserAgent string `json:"userAgent"`
	}
	return client.Call(ctx, "Browser.getVersion", "", nil, &result) == nil
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
	removeProfileDir(p.profileDir)
}

func (p *browserProcess) ensureProfileDir() error {
	if p.profileDir == "" {
		return nil
	}
	return makeProfileDir(p.profileDir)
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
		if len(env) >= len(prefix) && env[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func profilePathFor(root string, generation uint64) string {
	return fmt.Sprintf("%s%cruntime-%d", root, os.PathSeparator, generation)
}
