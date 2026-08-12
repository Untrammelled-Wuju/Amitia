package root

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	MaxArgCount           = 128
	MaxInputBytes         = 1048576
	MaxOutputBytes        = 2097152
	MaxArgBytes           = 65536
	MaxTotalArgBytes      = 524288
	MaxEnvironmentEntries = 32
	MaxEnvironmentBytes   = 16384
	MinTimeoutMS          = 100
	DefaultTimeoutMS      = 15000
	HardTimeoutMS         = 60000
)

var allowedEnvKeys = map[string]bool{
	"PATH":    true,
	"TMPDIR":  true,
	"LANG":    true,
	"LC_ALL":  true,
	"HOME":    true,
	"USER":    true,
}

var shellExecutables = []string{
	"sh", "bash", "zsh", "cmd", "powershell",
}

type Policy struct {
	MaxArgCount           int
	MaxInputBytes         int
	MaxOutputBytes        int
	MaxArgBytes           int
	MaxTotalArgBytes      int
	MaxEnvironmentEntries int
	MaxEnvironmentBytes   int
	MinTimeoutMS          int
	DefaultTimeoutMS      int
	HardTimeoutMS         int
}

func DefaultPolicy() Policy {
	return Policy{
		MaxArgCount:           MaxArgCount,
		MaxInputBytes:         MaxInputBytes,
		MaxOutputBytes:        MaxOutputBytes,
		MaxArgBytes:           MaxArgBytes,
		MaxTotalArgBytes:      MaxTotalArgBytes,
		MaxEnvironmentEntries: MaxEnvironmentEntries,
		MaxEnvironmentBytes:   MaxEnvironmentBytes,
		MinTimeoutMS:          MinTimeoutMS,
		DefaultTimeoutMS:      DefaultTimeoutMS,
		HardTimeoutMS:         HardTimeoutMS,
	}
}

type PolicyError struct {
	Code    string
	Message string
}

func (e *PolicyError) Error() string {
	return e.Code + ": " + e.Message
}

func (p Policy) ValidateExecute(req *ExecuteRequest) *PolicyError {
	if req.Executable == "" {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "executable is required"}
	}

	if isShellExecutable(req.Executable) {
		if req.Mode == "shell" {
			return nil
		}
		return &PolicyError{Code: ROOT_COMMAND_NOT_ALLOWED, Message: "shell executable requires shell mode"}
	}

	if len(req.Args) > p.MaxArgCount {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "too many arguments"}
	}

	totalArgBytes := 0
	for _, arg := range req.Args {
		if len(arg) > p.MaxArgBytes {
			return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "argument too long"}
		}
		totalArgBytes += len(arg)
	}
	if totalArgBytes > p.MaxTotalArgBytes {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "total arguments too large"}
	}

	if len(req.Stdin) > p.MaxInputBytes {
		return &PolicyError{Code: ROOT_INPUT_TOO_LARGE, Message: "stdin too large"}
	}

	if len(req.Env) > p.MaxEnvironmentEntries {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "too many environment variables"}
	}

	envBytes := 0
	for k, v := range req.Env {
		if !isValidEnvKey(k) {
			return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "invalid environment key: " + k}
		}
		envBytes += len(k) + len(v)
	}
	if envBytes > p.MaxEnvironmentBytes {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "environment too large"}
	}

	if req.Mode != "" && req.Mode != "structured" && req.Mode != "shell" {
		return &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: "invalid mode: " + req.Mode}
	}

	return nil
}

func (p Policy) ValidateTimeout(timeoutMS int) int {
	if timeoutMS <= 0 {
		return p.DefaultTimeoutMS
	}
	if timeoutMS < p.MinTimeoutMS {
		return p.MinTimeoutMS
	}
	if timeoutMS > p.HardTimeoutMS {
		return p.HardTimeoutMS
	}
	return timeoutMS
}

func isShellExecutable(executable string) bool {
	for _, name := range shellExecutables {
		if strings.EqualFold(executable, name) {
			return true
		}
	}
	return false
}

func isValidEnvKey(key string) bool {
	if allowedEnvKeys[key] {
		return true
	}
	for _, r := range key {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return len(key) > 0 && len(key) <= 256
}

func ValidateWorkDir(workDir string) error {
	if workDir == "" {
		return nil
	}
	if !strings.HasPrefix(workDir, "/") {
		return fmt.Errorf("workDir must be an absolute path")
	}
	return nil
}
