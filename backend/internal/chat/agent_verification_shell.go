package chat

import (
	"encoding/json"
	"strings"
)

func agentToolMutatesWorkspace(name, arguments string) bool {
	if agentIsWorkspaceMutation(name) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "execute_host_command", "execute_terminal", "execute_in_terminal_session_streaming":
	default:
		return false
	}
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(arguments), &args) != nil || strings.TrimSpace(args.Command) == "" {
		return false
	}
	if agentShellHasOutputRedirection(args.Command) {
		return true
	}
	for _, segment := range agentVerificationCommandSegments(args.Command) {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(segment)))
		if len(fields) == 0 {
			continue
		}
		command := strings.Trim(fields[0], string([]byte{34, 39}))
		command = strings.ReplaceAll(command, "\\", "/")
		if idx := strings.LastIndexByte(command, '/'); idx >= 0 {
			command = command[idx+1:]
		}
		switch command {
		case "tee", "tee.exe", "touch", "mkdir", "rmdir", "rm", "mv", "cp", "install",
			"set-content", "add-content", "out-file", "remove-item", "move-item",
			"copy-item", "rename-item", "new-item", "clear-content",
			"apply_patch", "truncate", "chmod", "chown", "patch", "patch.exe":
			return true
		case "git", "git.exe":
			if len(fields) > 1 {
				switch fields[1] {
				case "apply", "checkout", "switch", "restore", "reset", "clean", "merge", "rebase", "cherry-pick":
					return true
				}
			}
		case "go", "go.exe":
			if len(fields) > 1 && (fields[1] == "fmt" || fields[1] == "generate") {
				return true
			}
		case "dart", "dart.exe", "cargo", "cargo.exe":
			if len(fields) > 1 && fields[1] == "fmt" {
				return true
			}
			if command == "dart" && len(fields) > 1 && fields[1] == "format" {
				return true
			}
		case "npm", "npm.cmd", "pnpm", "pnpm.cmd", "yarn", "yarn.cmd":
			if len(fields) > 1 && (fields[1] == "install" || fields[1] == "add" || fields[1] == "remove") {
				return true
			}
		case "sed", "sed.exe", "perl", "perl.exe", "clang-format", "clang-format.exe", "prettier", "prettier.exe":
			for _, arg := range fields[1:] {
				if arg == "-i" || strings.HasPrefix(arg, "-i.") || arg == "--in-place" || arg == "--write" || arg == "-w" {
					return true
				}
			}
		case "eslint", "eslint.exe":
			for _, arg := range fields[1:] {
				if arg == "--fix" || arg == "--fix-dry-run" {
					return true
				}
			}
		case "gofmt", "gofmt.exe":
			for _, arg := range fields[1:] {
				if arg == "-w" {
					return true
				}
			}
		case "python", "python.exe", "python3", "python3.exe", "node", "node.exe", "ruby", "ruby.exe", "pwsh", "pwsh.exe", "powershell", "powershell.exe", "bash", "sh":
			for _, arg := range fields[1:] {
				if arg == "-c" || arg == "-command" || arg == "-e" || arg == "-encodedcommand" {
					return true
				}
			}
		}
	}
	return false
}

func agentShellHasOutputRedirection(command string) bool {
	var quoted rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			escaped = false
		case quoted != 0 && r == '\\':
			escaped = true
		case quoted != 0 && r == quoted:
			quoted = 0
		case quoted == 0 && (r == '\'' || r == '"'):
			quoted = r
		case quoted == 0 && r == '>':
			return true
		}
	}
	return false
}
