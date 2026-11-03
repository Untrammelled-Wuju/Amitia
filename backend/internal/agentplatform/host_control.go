package agentplatform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

type HostControlAction struct {
	Action string `json:"action"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button"`
	Text   string `json:"text"`
	Key    string `json:"key"`
	Window string `json:"window"`
	Amount int    `json:"amount"`
}

type HostControlResult struct {
	Action      string `json:"action"`
	Platform    string `json:"platform"`
	Output      string `json:"output,omitempty"`
	ResourceURI string `json:"resourceUri,omitempty"`
	Base64      string `json:"base64,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

func (m *RuntimeManager) ExecuteHostControl(ctx context.Context, spaceID, sessionID string, action HostControlAction) (HostControlResult, error) {
	var session HostControlSession
	if err := m.db.WithContext(ctx).Where("id = ? AND space_id = ?", sessionID, spaceID).First(&session).Error; err != nil {
		return HostControlResult{}, fmt.Errorf("读取桌面接管会话: %w", err)
	}
	if session.Status != "active" || !session.ExpiresAt.After(time.Now().UTC()) {
		return HostControlResult{}, errors.New("桌面接管会话未激活或已经过期")
	}
	var capabilities []string
	if err := json.Unmarshal([]byte(session.Capabilities), &capabilities); err != nil {
		return HostControlResult{}, fmt.Errorf("解析桌面接管能力: %w", err)
	}
	required := hostCapabilityForAction(action.Action)
	if required != "" && !containsString(capabilities, required) {
		return HostControlResult{}, fmt.Errorf("桌面接管未授权能力 %s", required)
	}
	switch action.Action {
	case "screen.capture":
		return m.captureHostScreen(ctx, session, action)
	case "mouse.move", "mouse.click", "mouse.scroll":
		return m.controlHostMouse(ctx, action)
	case "keyboard.type", "keyboard.key":
		return m.controlHostKeyboard(ctx, action)
	case "window.list", "window.activate":
		return m.controlHostWindow(ctx, action)
	default:
		return HostControlResult{}, fmt.Errorf("不支持的桌面控制动作: %s", action.Action)
	}
}

func hostCapabilityForAction(action string) string {
	switch action {
	case "screen.capture":
		return "screen.capture"
	case "mouse.move", "mouse.click", "mouse.scroll":
		return "input.mouse"
	case "keyboard.type", "keyboard.key":
		return "input.keyboard"
	case "window.list", "window.activate":
		return "window.control"
	default:
		return ""
	}
}

func (m *RuntimeManager) captureHostScreen(ctx context.Context, session HostControlSession, action HostControlAction) (HostControlResult, error) {
	root := filepath.Join(m.dataDir, "agent-platform", "host-control", session.ID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return HostControlResult{}, fmt.Errorf("创建桌面截图目录: %w", err)
	}
	path := filepath.Join(root, newID("screen")+".png")
	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "screencapture", "-x", path)
	case "windows":
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms,System.Drawing; $bounds=[System.Windows.Forms.SystemInformation]::VirtualScreen; $bitmap=New-Object System.Drawing.Bitmap $bounds.Width,$bounds.Height; $graphics=[System.Drawing.Graphics]::FromImage($bitmap); $graphics.CopyFromScreen($bounds.Location,[System.Drawing.Point]::Empty,$bounds.Size); $bitmap.Save('%s',[System.Drawing.Imaging.ImageFormat]::Png); $graphics.Dispose(); $bitmap.Dispose()`, strings.ReplaceAll(path, "'", "''"))
		command = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
	default:
		command = exec.CommandContext(ctx, "gnome-screenshot", "-f", path)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return HostControlResult{}, fmt.Errorf("捕获桌面画面: %w: %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return HostControlResult{}, fmt.Errorf("读取桌面截图: %w", err)
	}
	if len(data) > 20<<20 {
		return HostControlResult{}, fmt.Errorf("桌面截图超过 20 MiB: %d", len(data))
	}
	return HostControlResult{
		Action: action.Action, Platform: goruntime.GOOS,
		ResourceURI: "file://" + filepath.ToSlash(path), Base64: base64.StdEncoding.EncodeToString(data),
		MimeType: "image/png",
	}, nil
}

func (m *RuntimeManager) controlHostMouse(ctx context.Context, action HostControlAction) (HostControlResult, error) {
	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		var script string
		switch action.Action {
		case "mouse.move":
			script = fmt.Sprintf(`import CoreGraphics; let point = CGPoint(x: %d, y: %d); CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left)?.post(tap: .cghidEventTap)`, action.X, action.Y)
		case "mouse.click":
			button := ".left"
			if action.Button == "right" {
				button = ".right"
			}
			script = fmt.Sprintf(`import CoreGraphics; let point = CGPoint(x: %d, y: %d); CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: %s)?.post(tap: .cghidEventTap); CGEvent(mouseEventSource: nil, mouseType: .%sMouseDown, mouseCursorPosition: point, mouseButton: %s)?.post(tap: .cghidEventTap); CGEvent(mouseEventSource: nil, mouseType: .%sMouseUp, mouseCursorPosition: point, mouseButton: %s)?.post(tap: .cghidEventTap)`, action.X, action.Y, button, strings.TrimPrefix(button, "."), button, strings.TrimPrefix(button, "."), button)
		case "mouse.scroll":
			delta := action.Amount
			if delta == 0 {
				delta = 1
			}
			script = fmt.Sprintf(`import CoreGraphics; CGEvent(scrollWheelEvent2Source: nil, units: .pixel, wheelCount: 1, wheel1: %d, wheel2: 0, wheel3: 0)?.post(tap: .cghidEventTap)`, delta*30)
		}
		command = exec.CommandContext(ctx, "swift", "-e", script)
	case "windows":
		script := windowsMouseScript(action)
		command = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", script)
	default:
		switch action.Action {
		case "mouse.move":
			command = exec.CommandContext(ctx, "xdotool", "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y))
		case "mouse.click":
			command = exec.CommandContext(ctx, "xdotool", "mousemove", fmt.Sprint(action.X), fmt.Sprint(action.Y), "click", mouseButton(action.Button))
		case "mouse.scroll":
			button := "4"
			if action.Amount < 0 {
				button = "5"
			}
			command = exec.CommandContext(ctx, "xdotool", "click", "--repeat", fmt.Sprint(maxInt(absInt(action.Amount), 1)), button)
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return HostControlResult{}, fmt.Errorf("执行桌面鼠标动作: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return HostControlResult{Action: action.Action, Platform: goruntime.GOOS, Output: strings.TrimSpace(string(output))}, nil
}

func (m *RuntimeManager) controlHostKeyboard(ctx context.Context, action HostControlAction) (HostControlResult, error) {
	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		text := escapeAppleScript(action.Text)
		if action.Action == "keyboard.key" {
			key := normalizeKey(action.Key)
			command = exec.CommandContext(ctx, "osascript", "-e", fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(key)))
		} else {
			command = exec.CommandContext(ctx, "osascript", "-e", fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, text))
		}
	case "windows":
		command = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", windowsKeyboardScript(action))
	default:
		if action.Action == "keyboard.key" {
			command = exec.CommandContext(ctx, "xdotool", "key", normalizeKey(action.Key))
		} else {
			command = exec.CommandContext(ctx, "xdotool", "type", "--clearmodifiers", action.Text)
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return HostControlResult{}, fmt.Errorf("执行桌面键盘动作: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return HostControlResult{Action: action.Action, Platform: goruntime.GOOS, Output: strings.TrimSpace(string(output))}, nil
}

func (m *RuntimeManager) controlHostWindow(ctx context.Context, action HostControlAction) (HostControlResult, error) {
	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		if action.Action == "window.list" {
			command = exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to get name of every process whose background only is false`)
		} else {
			command = exec.CommandContext(ctx, "osascript", "-e", fmt.Sprintf(`tell application "System Events" to set frontmost of first process whose name is "%s" to true`, escapeAppleScript(action.Window)))
		}
	case "windows":
		command = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", windowsWindowScript(action))
	default:
		if action.Action == "window.list" {
			command = exec.CommandContext(ctx, "wmctrl", "-l")
		} else {
			command = exec.CommandContext(ctx, "wmctrl", "-a", action.Window)
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return HostControlResult{}, fmt.Errorf("执行桌面窗口动作: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return HostControlResult{Action: action.Action, Platform: goruntime.GOOS, Output: strings.TrimSpace(string(output))}, nil
}

func windowsMouseScript(action HostControlAction) string {
	if action.Action == "mouse.move" {
		return fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Cursor]::Position=New-Object System.Drawing.Point(%d,%d)`, action.X, action.Y)
	}
	if action.Action == "mouse.scroll" {
		delta := -120
		if action.Amount > 0 {
			delta = 120
		}
		return fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Cursor]::Position=New-Object System.Drawing.Point(%d,%d); Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class M{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,int d,int e);}'; [M]::SetCursorPos(%d,%d); [M]::mouse_event(0x0800,0,0,%d,0)`, action.X, action.Y, action.X, action.Y, delta)
	}
	if action.Button == "right" {
		return fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Cursor]::Position=New-Object System.Drawing.Point(%d,%d); Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class M{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,int d,int e);}'; [M]::SetCursorPos(%d,%d); [M]::mouse_event(0x0008,0,0,0,0);[M]::mouse_event(0x0010,0,0,0,0)`, action.X, action.Y, action.X, action.Y)
	}
	return fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Cursor]::Position=New-Object System.Drawing.Point(%d,%d); Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class M{[DllImport("user32.dll")]public static extern bool SetCursorPos(int X,int Y);[DllImport("user32.dll")]public static extern void mouse_event(uint f,uint x,uint y,int d,int e);}'; [M]::SetCursorPos(%d,%d); [M]::mouse_event(0x0002,0,0,0,0);[M]::mouse_event(0x0004,0,0,0,0)`, action.X, action.Y, action.X, action.Y)
}

func windowsKeyboardScript(action HostControlAction) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(action.Text))
	return fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $value=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); [System.Windows.Forms.SendKeys]::SendWait($value)`, encoded)
}

func windowsWindowScript(action HostControlAction) string {
	if action.Action == "window.list" {
		return `Get-Process | Where-Object {$_.MainWindowTitle -ne ''} | Select-Object Id,ProcessName,MainWindowTitle | ConvertTo-Json -Compress`
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(action.Window))
	return fmt.Sprintf(`Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class W{[DllImport("user32.dll")]public static extern bool SetForegroundWindow(IntPtr h);}'; $name=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); Get-Process | Where-Object {$_.MainWindowTitle -eq $name} | Select-Object -First 1 | ForEach-Object {[W]::SetForegroundWindow($_.MainWindowHandle)}`, encoded)
}

func escapeAppleScript(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "\"", "\\\"")
}

func mouseButton(value string) string {
	if value == "right" {
		return "3"
	}
	if value == "middle" {
		return "2"
	}
	return "1"
}

func normalizeKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Return"
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
