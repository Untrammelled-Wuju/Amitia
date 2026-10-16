package workbenchterminal

import (
	"io"
	"os"
	"runtime"
	"strings"
)

type process interface {
	io.Reader
	io.Writer
	Close() error
	Resize(rows, cols uint16) error
	Wait() error
	Mode() string
}

func shellExecutable() string {
	if runtime.GOOS == "windows" {
		if candidate := os.Getenv("COMSPEC"); candidate != "" {
			return candidate
		}
		return "cmd.exe"
	}
	if candidate := strings.TrimSpace(os.Getenv("SHELL")); candidate != "" && strings.HasPrefix(candidate, "/") {
		return candidate
	}
	return "/bin/sh"
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if code, ok := err.(interface{ ExitCode() int }); ok {
		return code.ExitCode()
	}

	return -1
}
