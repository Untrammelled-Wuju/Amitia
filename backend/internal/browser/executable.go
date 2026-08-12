package browser

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	browserKindChromium = "chromium"
	browserKindChrome   = "chrome"
	browserKindEdge     = "edge"
	browserKindUnknown  = "unknown"
)

type BrowserExecutableResolver interface {
	Resolve(ctx context.Context) (BrowserExecutable, error)
}

type executableResolver struct {
	config            BrowserConfig
	packagedPaths     []string
	osStandardPaths   []string
}

func NewBrowserExecutableResolver(config BrowserConfig) BrowserExecutableResolver {
	return &executableResolver{
		config:          config,
		packagedPaths:   collectPackagedPaths(config),
		osStandardPaths: collectOSSpecificPaths(),
	}
}

func (r *executableResolver) Resolve(ctx context.Context) (BrowserExecutable, error) {
	if !r.config.Enabled {
		return BrowserExecutable{}, &BrowserError{
			Code:    ErrCodeBrowserDisabled,
			Message: "browser runtime is disabled by configuration",
		}
	}

	var candidatePaths []string

	if r.config.ExecutablePath != "" {
		candidatePaths = append(candidatePaths, r.config.ExecutablePath)
	}

	candidatePaths = append(candidatePaths, r.packagedPaths...)
	candidatePaths = append(candidatePaths, r.osStandardPaths...)

	seen := make(map[string]bool)
	for _, path := range candidatePaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		normalized := strings.ToLower(path)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true

		if _, err := exec.LookPath(path); err == nil {
			return r.probeExecutable(ctx, path)
		}
	}

	return BrowserExecutable{}, &BrowserError{
		Code:    ErrCodeBrowserExecNotFound,
		Message: "no compatible chromium browser executable found",
	}
}

func (r *executableResolver) probeExecutable(ctx context.Context, path string) (BrowserExecutable, error) {
	kind := classifyExecutable(path)
	if kind == browserKindUnknown {
		return BrowserExecutable{}, &BrowserError{
			Code:    ErrCodeBrowserExecNotFound,
			Message: fmt.Sprintf("executable is not a supported browser: %s", filepath.Base(path)),
		}
	}

	execInfo := BrowserExecutable{
		Path:    path,
		Kind:    kind,
		Version: "",
	}

	return execInfo, nil
}

func classifyExecutable(path string) string {
	name := strings.ToLower(filepath.Base(path))
	name = strings.TrimSuffix(name, ".exe")

	switch {
	case strings.Contains(name, "chrome") && !strings.Contains(name, "chromium"):
		return browserKindChrome
	case strings.Contains(name, "edge"):
		return browserKindEdge
	case strings.Contains(name, "chromium"):
		return browserKindChromium
	default:
		return browserKindUnknown
	}
}

func collectPackagedPaths(config BrowserConfig) []string {
	var paths []string
	if config.UserDataRoot != "" {
		paths = append(paths,
			filepath.Join(config.UserDataRoot, "chrome-headless-shell"),
			filepath.Join(config.UserDataRoot, "chrome-headless-shell.exe"),
			filepath.Join(config.UserDataRoot, "chrome"),
			filepath.Join(config.UserDataRoot, "chrome.exe"),
		)
	}
	return paths
}

func collectOSSpecificPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Chromium\Application\chrome.exe`,
			"chrome",
			"chrome.exe",
			"msedge",
			"msedge.exe",
		}
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"google-chrome",
			"chrome",
			"chromium",
		}
	default:
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"google-chrome",
			"chrome",
			"chromium",
			"chromium-browser",
			"microsoft-edge",
		}
	}
}
