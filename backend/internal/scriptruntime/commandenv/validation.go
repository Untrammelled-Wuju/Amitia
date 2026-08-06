// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func validateCommandString(cmd string) error {
	if strings.TrimSpace(cmd) == "" {
		return newCommandError(ErrCommandRequired, "", "")
	}
	if !utf8.ValidString(cmd) {
		return newCommandError(ErrCommandInvalid, "", baseName(cmd))
	}
	if strings.ContainsRune(cmd, 0) {
		return newCommandError(ErrCommandInvalid, "", baseName(cmd))
	}
	return nil
}

func validateArgs(args []string) error {
	for _, a := range args {
		if strings.ContainsRune(a, 0) {
			return newCommandError(ErrCommandInvalid, "", "")
		}
	}
	return nil
}

func (i Invocation) Validate() error {
	if i.Executable == "" {
		return newCommandError(ErrCommandRequired, "", "")
	}
	if !filepath.IsAbs(i.Executable) {
		return newCommandError(ErrExecutableInvalid, i.Kind, i.Executable)
	}
	if !knownKind(i.Kind) {
		return newCommandError(ErrCommandInvalid, i.Kind, baseName(i.Executable))
	}
	if !knownSource(i.Source) {
		return newCommandError(ErrCommandInvalid, i.Kind, baseName(i.Executable))
	}
	for _, a := range i.Args {
		if strings.ContainsRune(a, 0) {
			return newCommandError(ErrCommandInvalid, i.Kind, baseName(i.Executable))
		}
	}
	if i.Source == SourceManagedNode {
		if i.Kind == KindNode && !samePathBase(i.Executable, "node") {
			return newCommandError(ErrExecutableInvalid, i.Kind, baseName(i.Executable))
		}
	}
	return nil
}

func normalizeCommand(cmd string) string {
	return strings.TrimSpace(cmd)
}

func classifyCommand(cmd string) (Kind, bool) {
	lower := strings.ToLower(cmd)
	switch lower {
	case "node", "node.exe":
		return KindNode, true
	case "npm", "npm.cmd", "npm.exe":
		return KindNPM, true
	case "npx", "npx.cmd", "npx.exe":
		return KindNPX, true
	}
	return KindNative, false
}

var shellCommands = map[string]bool{
	"sh":            true,
	"bash":          true,
	"zsh":           true,
	"fish":          true,
	"cmd":           true,
	"cmd.exe":       true,
	"powershell":    true,
	"powershell.exe": true,
	"pwsh":          true,
	"pwsh.exe":      true,
}

func isShellCommand(cmd string) bool {
	base := strings.ToLower(baseName(cmd))
	return shellCommands[base]
}

func baseName(cmd string) string {
	idx := strings.LastIndexAny(cmd, "\\/")
	if idx < 0 {
		return cmd
	}
	return cmd[idx+1:]
}

func copyArgs(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func samePathBase(path, name string) bool {
	return strings.EqualFold(baseName(path), name)
}

func filepath_IsAbs(path string) bool {
	return filepath.IsAbs(path)
}
