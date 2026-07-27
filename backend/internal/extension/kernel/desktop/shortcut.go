package desktop

import (
	"fmt"
	"runtime"
	"strings"
)

var reservedHostShortcuts = []string{
	"CmdOrCtrl+Q", "CmdOrCtrl+Shift+Q",
	"CmdOrCtrl+W", "CmdOrCtrl+Shift+W",
	"CmdOrCtrl+N", "CmdOrCtrl+Shift+N",
	"CmdOrCtrl+T", "CmdOrCtrl+Shift+T",
	"CmdOrCtrl+R", "CmdOrCtrl+Shift+R",
	"CmdOrCtrl+Shift+I", "CmdOrCtrl+Alt+I", "F12",
	"Alt+F4",
	"CmdOrCtrl+Tab", "CmdOrCtrl+Shift+Tab",
	"CmdOrCtrl+M", "CmdOrCtrl+Shift+M",
	"CmdOrCtrl+B", "CmdOrCtrl+Shift+B",
	"CmdOrCtrl+H", "CmdOrCtrl+Shift+H",
	"CommandOrControl+Shift+H",
}

var osReservedShortcuts = map[string][]string{
	"darwin": {
		"Cmd+Q", "Cmd+W", "Cmd+N", "Cmd+T", "Cmd+R",
		"Cmd+Shift+I", "Cmd+Option+I",
		"Cmd+Tab", "Cmd+Shift+Tab",
		"Cmd+Space", "Cmd+Shift+Space",
		"Cmd+Option+Esc",
	},
	"windows": {
		"Ctrl+Shift+Esc", "Win+D", "Win+E", "Win+L",
		"Win+R", "Win+Tab", "Alt+Tab", "Alt+Shift+Tab",
		"Ctrl+Alt+Del", "Ctrl+Shift+Esc",
	},
	"linux": {
		"Ctrl+Alt+Del", "Ctrl+Alt+Backspace",
		"Alt+F1", "Alt+F2", "Alt+F3",
		"Ctrl+Alt+L", "Super+D",
	},
}

var validModifiers = map[string]bool{
	"cmdorcontrol": true, "cmdorctrl": true, "cmd": true, "ctrl": true, "control": true,
	"alt": true, "option": true, "shift": true, "super": true,
	"meta": true, "command": true,
}

var validKeys = map[string]bool{
	"0": true, "1": true, "2": true, "3": true, "4": true,
	"5": true, "6": true, "7": true, "8": true, "9": true,
	"a": true, "b": true, "c": true, "d": true, "e": true, "f": true,
	"g": true, "h": true, "i": true, "j": true, "k": true, "l": true,
	"m": true, "n": true, "o": true, "p": true, "q": true, "r": true,
	"s": true, "t": true, "u": true, "v": true, "w": true, "x": true,
	"y": true, "z": true,
	"f1": true, "f2": true, "f3": true, "f4": true, "f5": true,
	"f6": true, "f7": true, "f8": true, "f9": true, "f10": true,
	"f11": true, "f12": true, "f13": true, "f14": true, "f15": true,
	"f16": true, "f17": true, "f18": true, "f19": true, "f20": true,
	"f21": true, "f22": true, "f23": true, "f24": true,
	"space": true, "enter": true, "return": true, "tab": true,
	"escape": true, "esc": true, "backspace": true, "delete": true,
	"insert": true, "home": true, "end": true,
	"pageup": true, "pagedown": true,
	"up": true, "down": true, "left": true, "right": true,
	"plus": true, "minus": true, "equal": true,
	"comma": true, "period": true, "slash": true, "backslash": true,
	"semicolon": true, "quote": true, "backquote": true,
	"leftbracket": true, "rightbracket": true,
	"numpad0": true, "numpad1": true, "numpad2": true, "numpad3": true,
	"numpad4": true, "numpad5": true, "numpad6": true, "numpad7": true,
	"numpad8": true, "numpad9": true,
	"numpadmultiply": true, "numpadadd": true, "numpadsubtract": true,
	"numpaddecimal": true, "numpaddivide": true,
}

type ShortcutValidationResult struct {
	Valid       bool
	Normalized  string
	Reason      string
	IsReserved  bool
	IsModifierOnly bool
}

func NormalizeAccelerator(accelerator string) string {
	if accelerator == "" {
		return ""
	}
	parts := strings.Split(accelerator, "+")
	var normalized []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch lower {
		case "cmdorcontrol", "commandorcontrol", "cmdorctrl":
			normalized = append(normalized, "CmdOrCtrl")
		case "ctrl", "control":
			normalized = append(normalized, "Ctrl")
		case "cmd", "command":
			normalized = append(normalized, "Cmd")
		case "alt":
			normalized = append(normalized, "Alt")
		case "option":
			normalized = append(normalized, "Alt")
		case "shift":
			normalized = append(normalized, "Shift")
		case "super", "meta":
			normalized = append(normalized, "Super")
		default:
			if len(trimmed) == 1 {
				normalized = append(normalized, strings.ToUpper(trimmed))
			} else {
				normalized = append(normalized, trimmed)
			}
		}
	}
	return strings.Join(normalized, "+")
}

func ValidateAccelerator(accelerator string) ShortcutValidationResult {
	if accelerator == "" {
		return ShortcutValidationResult{Reason: "empty accelerator"}
	}
	normalized := NormalizeAccelerator(accelerator)
	parts := strings.Split(normalized, "+")
	if len(parts) < 2 {
		return ShortcutValidationResult{
			Reason:      "accelerator must have at least one modifier and one key",
			IsModifierOnly: isAllModifiers(parts),
		}
	}
	var hasModifier bool
	var hasKey bool
	for _, part := range parts {
		lower := strings.ToLower(part)
		if validModifiers[lower] {
			hasModifier = true
		} else if validKeys[lower] {
			hasKey = true
		} else {
			return ShortcutValidationResult{
				Reason: fmt.Sprintf("invalid key or modifier: %s", part),
			}
		}
	}
	if !hasModifier {
		return ShortcutValidationResult{
			Reason:      "accelerator must contain at least one modifier",
			IsModifierOnly: true,
		}
	}
	if !hasKey {
		return ShortcutValidationResult{
			Reason:      "accelerator must contain at least one non-modifier key",
			IsModifierOnly: true,
		}
	}
	if isReservedShortcut(normalized) {
		return ShortcutValidationResult{
			Valid:      false,
			Normalized: normalized,
			Reason:     "reserved host shortcut",
			IsReserved: true,
		}
	}
	if isOSReservedShortcut(normalized) {
		return ShortcutValidationResult{
			Valid:      false,
			Normalized: normalized,
			Reason:     "reserved OS shortcut",
			IsReserved: true,
		}
	}
	return ShortcutValidationResult{
		Valid:      true,
		Normalized: normalized,
	}
}

func isAllModifiers(parts []string) bool {
	for _, part := range parts {
		if !validModifiers[strings.ToLower(part)] {
			return false
		}
	}
	return true
}

func isReservedShortcut(accelerator string) bool {
	normalized := NormalizeAccelerator(accelerator)
	for _, reserved := range reservedHostShortcuts {
		if NormalizeAccelerator(reserved) == normalized {
			return true
		}
	}
	return false
}

func isOSReservedShortcut(accelerator string) bool {
	normalized := NormalizeAccelerator(accelerator)
	osName := runtime.GOOS
	var reserved []string
	switch osName {
	case "darwin":
		reserved = osReservedShortcuts["darwin"]
	case "windows":
		reserved = osReservedShortcuts["windows"]
	default:
		reserved = osReservedShortcuts["linux"]
	}
	for _, r := range reserved {
		if NormalizeAccelerator(r) == normalized {
			return true
		}
	}
	return false
}

func IsShortcutReserved(accelerator string) bool {
	return isReservedShortcut(accelerator) || isOSReservedShortcut(accelerator)
}

func AcceleratorsConflict(a, b string) bool {
	return NormalizeAccelerator(a) == NormalizeAccelerator(b)
}

type PlatformAccelerators struct {
	Darwin  string `json:"darwin,omitempty"`
	Windows string `json:"windows,omitempty"`
	Linux   string `json:"linux,omitempty"`
}

type LogicalShortcut struct {
	LogicalAccelerator string             `json:"logicalAccelerator"`
	PlatformAccelerators PlatformAccelerators `json:"platformAccelerators,omitempty"`
}

func (ls *LogicalShortcut) ResolveForPlatform(platform string) string {
	switch platform {
	case "darwin":
		if ls.PlatformAccelerators.Darwin != "" {
			return ls.PlatformAccelerators.Darwin
		}
	case "windows":
		if ls.PlatformAccelerators.Windows != "" {
			return ls.PlatformAccelerators.Windows
		}
	case "linux":
		if ls.PlatformAccelerators.Linux != "" {
			return ls.PlatformAccelerators.Linux
		}
	}
	return ls.LogicalAccelerator
}

func IsPlatformSupported(platforms []string, targetPlatform string) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, p := range platforms {
		if p == targetPlatform {
			return true
		}
	}
	return false
}
