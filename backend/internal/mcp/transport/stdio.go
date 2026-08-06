package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type StdioConfig struct {
	Command              string
	Executable           string
	OriginalCommand      string
	Args                 []string
	WorkDir              string
	Environment          map[string]string
	StartTimeout         time.Duration
	ShutdownTimeout      time.Duration
	MaxMessageBytes      int64
	StderrBytesPerMinute int
	OnStderr             func(string)
}

type Stdio struct {
	config        StdioConfig
	mu            sync.RWMutex
	writeMu       sync.Mutex
	state         State
	command       *exec.Cmd
	stdin         io.WriteCloser
	receive       chan protocol.Message
	waitDone      chan error
	processCancel context.CancelFunc
	processTree   uintptr
	done          chan struct{}
	doneOnce      sync.Once
}

func NewStdio(config StdioConfig) *Stdio {
	if config.StartTimeout <= 0 {
		config.StartTimeout = 10 * time.Second
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 3 * time.Second
	}
	if config.MaxMessageBytes <= 0 {
		config.MaxMessageBytes = 4 << 20
	}
	if config.StderrBytesPerMinute <= 0 {
		config.StderrBytesPerMinute = 64 << 10
	}
	return &Stdio{config: config, state: StateStopped, receive: make(chan protocol.Message, 64), waitDone: make(chan error, 1), done: make(chan struct{})}
}

func (t *Stdio) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.state != StateStopped {
		t.mu.Unlock()
		return fmt.Errorf("MCP transport already started")
	}
	t.state = StateStarting
	t.mu.Unlock()
	cmdStr := t.config.OriginalCommand
	if cmdStr == "" {
		cmdStr = t.config.Command
	}
	if err := validateStdioCommand(cmdStr); err != nil {
		t.setState(StateError)
		return err
	}
	if err := validateStdioExecutable(t.config.Executable); err != nil {
		t.setState(StateError)
		return err
	}
	if t.config.WorkDir != "" {
		info, statErr := os.Stat(t.config.WorkDir)
		if statErr != nil || !info.IsDir() {
			t.setState(StateError)
			return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: invalid working directory")
		}
	}
	processCtx, processCancel := context.WithCancel(context.Background())
	command := exec.CommandContext(processCtx, t.config.Executable, append([]string(nil), t.config.Args...)...)
	command.Dir = t.config.WorkDir
	envStrs, envErr := minimalEnvironment(t.config.Environment)
	if envErr != nil {
		processCancel()
		t.setState(StateError)
		return envErr
	}
	command.Env = envStrs
	configureProcess(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		processCancel()
		t.setState(StateError)
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		processCancel()
		t.setState(StateError)
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		processCancel()
		t.setState(StateError)
		return err
	}
	started := make(chan error, 1)
	go func() { started <- command.Start() }()
	startTimer := time.NewTimer(t.config.StartTimeout)
	defer startTimer.Stop()
	select {
	case err = <-started:
		if err != nil {
			processCancel()
			t.setState(StateError)
			return fmt.Errorf("MCP_TRANSPORT_START_FAILED: %w", err)
		}
	case <-startTimer.C:
		processCancel()
		<-started
		t.setState(StateError)
		return fmt.Errorf("MCP_TRANSPORT_TIMEOUT: stdio start")
	case <-ctx.Done():
		processCancel()
		<-started
		t.setState(StateError)
		return ctx.Err()
	}
	processTree, err := attachProcessTree(command)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		processCancel()
		t.setState(StateError)
		return fmt.Errorf("MCP_TRANSPORT_START_FAILED: process tree: %w", err)
	}
	t.mu.Lock()
	t.command = command
	t.stdin = stdin
	t.processCancel = processCancel
	t.processTree = processTree
	t.state = StateRunning
	t.mu.Unlock()
	go t.readStdout(stdout)
	go t.readStderr(stderr)
	go t.waitProcess(command)
	return nil
}

func (t *Stdio) Send(ctx context.Context, message protocol.Message) error {
	if t.State() != StateRunning {
		return protocol.ErrTransportClosed
	}
	payload, err := protocol.Encode(message, t.config.MaxMessageBytes)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	t.mu.RLock()
	stdin := t.stdin
	t.mu.RUnlock()
	if stdin == nil {
		return protocol.ErrTransportClosed
	}
	done := make(chan error, 1)
	go func() {
		_, writeErr := stdin.Write(append(payload, '\n'))
		done <- writeErr
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *Stdio) Receive() <-chan protocol.Message { return t.receive }

func (t *Stdio) Done() <-chan struct{} { return t.done }

func (t *Stdio) Close(ctx context.Context) error {
	t.mu.Lock()
	if t.state == StateStopped {
		t.mu.Unlock()
		return nil
	}
	t.state = StateClosing
	stdin := t.stdin
	command := t.command
	processTree := t.processTree
	processCancel := t.processCancel
	t.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	timer := time.NewTimer(t.config.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-t.waitDone:
	case <-timer.C:
		if command != nil && command.Process != nil {
			_ = terminateProcessTree(command, processTree)
		}
		if processCancel != nil {
			processCancel()
		}
		select {
		case <-t.waitDone:
		case <-ctx.Done():
		}
	case <-ctx.Done():
		if command != nil && command.Process != nil {
			_ = terminateProcessTree(command, processTree)
		}
		if processCancel != nil {
			processCancel()
		}
	}
	closeProcessTree(processTree)
	t.mu.Lock()
	t.command = nil
	t.stdin = nil
	t.processCancel = nil
	t.processTree = 0
	t.state = StateStopped
	t.mu.Unlock()
	return nil
}

func (t *Stdio) State() State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *Stdio) readStdout(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	max := int(t.config.MaxMessageBytes)
	if max > 16<<20 {
		max = 16 << 20
	}
	scanner.Buffer(make([]byte, 64<<10), max)
	for scanner.Scan() {
		message, err := protocol.Decode(append([]byte(nil), scanner.Bytes()...), t.config.MaxMessageBytes)
		if err != nil {
			t.setState(StateError)
			t.stopProcess()
			return
		}
		t.receive <- message
	}
	if scanner.Err() != nil {
		t.setState(StateError)
		t.stopProcess()
	}
}

func (t *Stdio) readStderr(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	windowStart := time.Now()
	remaining := t.config.StderrBytesPerMinute
	for scanner.Scan() {
		if time.Since(windowStart) >= time.Minute {
			windowStart = time.Now()
			remaining = t.config.StderrBytesPerMinute
		}
		line := scanner.Text()
		if len(line) > remaining {
			line = line[:max(remaining, 0)]
		}
		remaining -= len(line)
		if line != "" && t.config.OnStderr != nil {
			t.config.OnStderr(line)
		}
	}
}

func (t *Stdio) waitProcess(command *exec.Cmd) {
	err := command.Wait()
	t.waitDone <- err
	t.doneOnce.Do(func() { close(t.done) })
	t.mu.Lock()
	if t.state != StateClosing && t.state != StateStopped {
		t.state = StateError
	}
	t.mu.Unlock()
}

func (t *Stdio) stopProcess() {
	t.mu.RLock()
	command := t.command
	processTree := t.processTree
	cancel := t.processCancel
	t.mu.RUnlock()
	if command != nil && command.Process != nil {
		_ = terminateProcessTree(command, processTree)
	}
	if cancel != nil {
		cancel()
	}
}

func validateStdioCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" || strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: command is required")
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed)))
	forbidden := map[string]struct{}{"sh": {}, "bash": {}, "zsh": {}, "fish": {}, "cmd": {}, "powershell": {}, "pwsh": {}}
	if _, blocked := forbidden[base]; blocked {
		return fmt.Errorf("MCP_STDIO_COMMAND_NOT_ALLOWED: shell commands are forbidden")
	}
	return nil
}

func validateStdioExecutable(executable string) error {
	trimmed := strings.TrimSpace(executable)
	if trimmed == "" {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: executable path is required")
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: executable not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: executable is a directory")
	}
	return nil
}

func minimalEnvironment(explicit map[string]string) ([]string, error) {
	keys := []string{"PATH", "HOME", "TMP", "TEMP"}
	if runtime.GOOS == "windows" {
		keys = append(keys, "SystemRoot", "USERPROFILE", "ComSpec", "PATHEXT")
	}
	values := map[string]string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	for key, value := range explicit {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: invalid environment entry")
		}
		values[key] = value
	}
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result, nil
}

func (t *Stdio) setState(state State) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}
