//go:build linux

package platform

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const flatpakAppID = "com.tencent.WeChat"

func candidates() []string {
	r := []string{}
	if v := strings.TrimSpace(os.Getenv("AMITIA_WECHAT_CLIENT_PATH")); v != "" {
		r = append(r, v)
	}
	return append(r,
		"/usr/bin/wechat",
		"/opt/wechat/wechat",
		"/usr/local/bin/wechat",
	)
}

func findNativePath() string {
	for _, p := range candidates() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func flatpakInstalled() bool {
	roots := []string{
		"/var/lib/flatpak/app/" + flatpakAppID,
		filepath.Join(os.Getenv("HOME"), ".local", "share", "flatpak", "app", flatpakAppID),
	}
	for _, p := range roots {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

func findPID(path string) int {
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		procRoot := filepath.Join("/proc", entry.Name())
		exe, _ := os.Readlink(filepath.Join(procRoot, "exe"))
		exe = strings.TrimSuffix(exe, " (deleted)")
		base := strings.ToLower(filepath.Base(exe))
		comm, _ := os.ReadFile(filepath.Join(procRoot, "comm"))
		commName := strings.ToLower(strings.TrimSpace(string(comm)))
		if (path != "" && exe == path) || base == "wechat" || base == "weixin" || commName == "wechat" || commName == "weixin" {
			return pid
		}
	}
	return 0
}

func commandOutput(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func packageVersion(strategy string) string {
	if strategy == "flatpak" {
		return commandOutput(1200*time.Millisecond, "flatpak", "info", "--show-version", flatpakAppID)
	}
	for _, pkg := range []string{"wechat", "com.tencent.WeChat"} {
		if value := commandOutput(900*time.Millisecond, "dpkg-query", "-W", "-f=${Version}", pkg); value != "" {
			return value
		}
	}
	return ""
}

func ProbeClient() ClientStatus {
	if p := findNativePath(); p != "" {
		pid := findPID(p)
		return ClientStatus{
			Found:          true,
			Running:        pid > 0,
			PID:            pid,
			Path:           p,
			Version:        packageVersion("native"),
			Strategy:       "native",
			WindowManaged:  false,
			PreloadCapable: true,
			Message:        "official Linux WeChat detected",
		}
	}
	if flatpakInstalled() {
		pid := findPID("")
		return ClientStatus{
			Found:          true,
			Running:        pid > 0,
			PID:            pid,
			Path:           "flatpak:" + flatpakAppID,
			Version:        packageVersion("flatpak"),
			Strategy:       "flatpak",
			WindowManaged:  false,
			PreloadCapable: false,
			Message:        "Flatpak WeChat detected; host LD_PRELOAD companion cannot be injected into the sandbox",
		}
	}
	return ClientStatus{Message: "official Linux WeChat not found"}
}

func StartClient(opts StartOptions) (ClientStatus, error) {
	status := ProbeClient()
	if !status.Found {
		return status, fmt.Errorf("official Linux WeChat not found")
	}
	if status.Running {
		return status, nil
	}
	var cmd *exec.Cmd
	switch status.Strategy {
	case "flatpak":
		cmd = exec.Command("flatpak", "run", flatpakAppID)
	default:
		cmd = exec.Command(status.Path)
		if opts.DriverLibraryPath != "" && status.PreloadCapable {
			if st, err := os.Stat(opts.DriverLibraryPath); err == nil && !st.IsDir() {
				cmd.Env = append(os.Environ(),
					"LD_PRELOAD="+opts.DriverLibraryPath,
					"AMITIA_WECHAT_CLIENT_VERSION="+status.Version,
				)
			}
		}
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return status, err
	}
	_ = cmd.Process.Release()
	for i := 0; i < 30; i++ {
		time.Sleep(150 * time.Millisecond)
		status = ProbeClient()
		if status.Running {
			return status, nil
		}
	}
	return status, fmt.Errorf("Linux WeChat process did not become ready")
}

func HideClient(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("WeChat pid unavailable")
	}
	// X11/XWayland can be hidden without changing the channel contract. Pure
	// Wayland sessions intentionally fail closed because arbitrary foreign
	// window management is not generally permitted by the compositor.
	windowID, err := linuxUIWindowID(pid)
	if err != nil {
		return fmt.Errorf("window hiding unavailable: %w", err)
	}
	if !commandExists("xdotool") {
		return fmt.Errorf("xdotool unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "xdotool", "windowunmap", windowID).CombinedOutput(); err != nil {
		return fmt.Errorf("hide WeChat window: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func linuxSocketPath(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf("/tmp/amitia-wechat-hook-%d.sock", pid)
}

func preloadDriver(client ClientStatus) (DriverStatus, bool) {
	if !client.Running || client.PID <= 0 || client.Strategy == "flatpak" {
		return DriverStatus{}, false
	}
	socketPath := linuxSocketPath(client.PID)
	payload, err := callPreloadDriver(client, DriverRequest{Op: "driver.capabilities"})
	if err != nil {
		return DriverStatus{Endpoint: socketPath, Message: "Linux preload companion is not attached"}, false
	}
	var response struct {
		OK            bool            `json:"ok"`
		Kind          string          `json:"kind"`
		Version       string          `json:"version"`
		ClientVersion string          `json:"clientVersion"`
		Capabilities  map[string]bool `json:"capabilities"`
		Message       string          `json:"message"`
	}
	if err := json.Unmarshal(payload, &response); err != nil || !response.OK {
		return DriverStatus{Endpoint: socketPath, Message: "Linux preload companion returned invalid response"}, false
	}
	attached := response.Capabilities["attached"]
	verified := attached && client.Version != "" && response.ClientVersion == client.Version
	if !verified {
		for _, key := range []string{"qr", "loginStatus", "selfProfile", "receiveText", "sendText"} {
			response.Capabilities[key] = false
		}
		if attached {
			response.Message = fmt.Sprintf("driver/client version mismatch: driver=%q client=%q", response.ClientVersion, client.Version)
		}
	}
	return DriverStatus{
		Available:       attached,
		Attached:        attached,
		Kind:            response.Kind,
		Version:         response.Version,
		ClientVersion:   response.ClientVersion,
		VersionVerified: verified,
		Endpoint:        socketPath,
		Capabilities:    response.Capabilities,
		Message:         response.Message,
	}, attached
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func linuxUIWindowID(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("WeChat pid unavailable")
	}
	if strings.TrimSpace(os.Getenv("DISPLAY")) == "" {
		return "", fmt.Errorf("DISPLAY unavailable")
	}
	if !commandExists("xdotool") {
		return "", fmt.Errorf("xdotool unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "xdotool", "search", "--pid", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Fields(string(out)) {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line), nil
		}
	}
	return "", fmt.Errorf("WeChat X11 window not found")
}

func linuxUIWindowGeometry(windowID string) (int, int, error) {
	if !commandExists("xwininfo") {
		return 0, 0, fmt.Errorf("xwininfo unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "xwininfo", "-id", windowID).CombinedOutput()
	if err != nil {
		return 0, 0, err
	}
	var width, height int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Width:") {
			width, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Width:")))
		}
		if strings.HasPrefix(line, "Height:") {
			height, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Height:")))
		}
	}
	if width <= 0 || height <= 0 {
		return width, height, fmt.Errorf("invalid WeChat window geometry")
	}
	return width, height, nil
}

func linuxUICapture(windowID string) ([]byte, error) {
	if !commandExists("import") {
		return nil, fmt.Errorf("ImageMagick import unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "import", "-window", windowID, "png:-").Output()
}

func probeUIDriver(client ClientStatus) DriverStatus {
	caps := map[string]bool{
		"attached":    false,
		"qr":          false,
		"loginStatus": false,
		"selfProfile": false,
		"receiveText": false,
		"sendText":    false,
	}
	if !client.Running || client.PID <= 0 {
		return DriverStatus{Kind: "amitia-linux-ui", Capabilities: caps, Message: "WeChat client is not running"}
	}
	windowID, err := linuxUIWindowID(client.PID)
	if err != nil {
		return DriverStatus{Kind: "amitia-linux-ui", Capabilities: caps, Message: "Linux UI driver unavailable: " + err.Error()}
	}
	if !commandExists("import") || !commandExists("xwininfo") {
		return DriverStatus{Kind: "amitia-linux-ui", Endpoint: "x11:" + windowID, Capabilities: caps, Message: "Linux UI driver requires ImageMagick import and xwininfo"}
	}
	caps["attached"] = true
	caps["qr"] = true
	caps["loginStatus"] = true
	return DriverStatus{
		Available:       true,
		Attached:        true,
		Kind:            "amitia-linux-ui",
		Version:         "ui-v1",
		ClientVersion:   client.Version,
		VersionVerified: true,
		Endpoint:        "x11:" + windowID,
		Capabilities:    caps,
		Message:         "official Linux WeChat UI driver attached; QR/login are available, message transport requires a verified version adapter",
	}
}

func ProbeDriver(client ClientStatus) DriverStatus {
	if !client.Running || client.PID <= 0 {
		return DriverStatus{Message: "WeChat client is not running"}
	}
	if preload, attached := preloadDriver(client); attached {
		useful := preload.Capabilities["qr"] || preload.Capabilities["loginStatus"] || preload.Capabilities["receiveText"] || preload.Capabilities["sendText"]
		if useful {
			return preload
		}
		ui := probeUIDriver(client)
		if ui.Available {
			ui.Message = preload.Message + "; falling back to official-client UI for login"
			return ui
		}
		return preload
	}
	return probeUIDriver(client)
}

func uiDriverCall(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	status := probeUIDriver(client)
	if !status.Available {
		return nil, fmt.Errorf("Linux UI driver unavailable: %s", status.Message)
	}
	windowID, err := linuxUIWindowID(client.PID)
	if err != nil {
		return nil, err
	}
	switch req.Op {
	case "driver.capabilities":
		return mustJSON(map[string]any{
			"ok": true, "kind": status.Kind, "version": status.Version,
			"capabilities": status.Capabilities, "message": status.Message,
		}), nil
	case "login.qr":
		png, err := linuxUICapture(windowID)
		if err != nil {
			return nil, err
		}
		return mustJSON(map[string]any{
			"imageDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			"mode":         "official-client-window",
		}), nil
	case "login.status":
		width, height, err := linuxUIWindowGeometry(windowID)
		if err != nil {
			return nil, err
		}
		// The official Linux client's authenticated main window is materially
		// larger than the compact login window. This is deliberately reported as
		// a UI-derived heuristic, never as a protocol-authentication guarantee.
		logged := width >= 620 && height >= 460
		return mustJSON(map[string]any{
			"logged": logged, "status": map[bool]string{true: "online_ui_heuristic", false: "login_ui"}[logged],
			"windowWidth": width, "windowHeight": height, "derived": true,
		}), nil
	case "account.self":
		return mustJSON(map[string]any{"derived": true}), nil
	default:
		return nil, fmt.Errorf("Linux UI driver does not support %s", req.Op)
	}
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return json.RawMessage(data)
}

func callPreloadDriver(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	if !client.Running || client.PID <= 0 {
		return nil, fmt.Errorf("Linux WeChat is not running")
	}
	if client.Strategy == "flatpak" {
		return nil, fmt.Errorf("Flatpak WeChat cannot use host preload driver")
	}
	socketPath := linuxSocketPath(client.PID)
	conn, err := net.DialTimeout("unix", socketPath, 1200*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return json.RawMessage(line), nil
}

func callDriver(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	if preload, attached := preloadDriver(client); attached {
		capability := map[string]string{
			"login.qr":           "qr",
			"login.status":       "loginStatus",
			"account.self":       "selfProfile",
			"events.poll":        "receiveText",
			"messages.send_text": "sendText",
		}[req.Op]
		if req.Op == "driver.capabilities" || (capability != "" && preload.Capabilities[capability]) {
			if data, err := callPreloadDriver(client, req); err == nil {
				return data, nil
			}
		}
	}
	return uiDriverCall(client, req)
}
