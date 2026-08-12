//go:build linux && !android

package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type sessionFinalizer struct {
	mu       sync.Mutex
	done     bool
	exitCode int
	exited   bool
}

func newSessionFinalizer() *sessionFinalizer {
	return &sessionFinalizer{}
}

func (f *sessionFinalizer) MarkExited(code int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done {
		return false
	}
	f.done = true
	f.exited = true
	f.exitCode = code
	return true
}

func (f *sessionFinalizer) MarkClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done {
		return false
	}
	f.done = true
	return true
}

func (f *sessionFinalizer) IsDone() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

func validateShell(shell string, allowed []string) error {
	if shell == "" {
		return ErrStartFailed("shell is empty")
	}

	for _, s := range allowed {
		if shell == s {
			info, err := os.Stat(shell)
			if err != nil {
				return ErrStartFailed(fmt.Sprintf("shell not found: %s", shell))
			}
			if info.IsDir() {
				return ErrStartFailed(fmt.Sprintf("shell is a directory: %s", shell))
			}
			return nil
		}
	}

	return ErrStartFailed(fmt.Sprintf("shell not in allowlist: %s", shell))
}

func resolveWorkingDir(cwd string, workspaceRoot string) (string, error) {
	if cwd == "" {
		return workspaceRoot, nil
	}

	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(workspaceRoot, cwd)
	}

	cleaned := filepath.Clean(cwd)

	if workspaceRoot != "" {
		rel, err := filepath.Rel(workspaceRoot, cleaned)
		if err != nil {
			return "", ErrStartFailed("invalid working directory")
		}
		if rel == ".." || (len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator)) {
			return "", ErrStartFailed("working directory escapes workspace")
		}
	}

	return cleaned, nil
}

func buildEnvironment(homeDir string) []string {
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		fmt.Sprintf("HOME=%s", homeDir),
		"TERM=xterm-256color",
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
		fmt.Sprintf("TMPDIR=%s", os.TempDir()),
		fmt.Sprintf("PWD=%s", homeDir),
	}
	return env
}

func (s *Session) readLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closeCh:
			return
		default:
		}

		if s.ptmx == nil {
			return
		}

		s.ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := s.ptmx.Read(buf)
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			if err == io.EOF {
				s.handleExit()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-s.closeCh:
				return
			default:
				if !s.IsActive() {
					return
				}
			}
			continue
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			s.output.Append(TerminalStreamPTY, data, time.Now())
			s.RecordActivity()
		}
	}
}

func (s *Session) handleExit() {
	if s.cmd == nil {
		return
	}

	if s.cmd.ProcessState == nil {
		_ = s.cmd.Wait()
	}

	exitCode := 0
	if s.cmd.ProcessState != nil {
		exitCode = s.cmd.ProcessState.ExitCode()
	}

	now := time.Now()
	s.stateMu.Lock()
	s.ExitedAt = &now
	s.ExitCode = &exitCode
	s.State = SessionExited
	s.stateMu.Unlock()

	if s.ptmx != nil {
		_ = s.ptmx.Close()
		s.ptmx = nil
	}
}

func (s *Session) writeStdin(data []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if !s.IsActive() {
		return 0, ErrNotRunning()
	}

	if s.ptmx == nil {
		return 0, ErrIOFailed("ptmx closed")
	}

	n, err := s.ptmx.Write(data)
	if err != nil {
		return n, ErrIOFailed(err.Error())
	}

	s.RecordActivity()
	return n, nil
}

func (s *Session) resize(rows, cols uint16) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.State != SessionRunning {
		return ErrNotRunning()
	}

	s.Rows = rows
	s.Cols = cols

	if s.ptmx == nil {
		return ErrIOFailed("ptmx closed")
	}

	return resizePTY(s.ptmx, ptySize{Rows: rows, Cols: cols})
}

func (s *Session) status() SessionStatus {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()

	return SessionStatus{
		SessionID:    s.ID,
		State:        s.State,
		ExitCode:     s.ExitCode,
		CreatedAt:    s.CreatedAt,
		LastActivity: s.LastActivity,
		Rows:         s.Rows,
		Cols:         s.Cols,
	}
}

type SessionStatus struct {
	SessionID    SessionID
	State        SessionState
	ExitCode     *int
	CreatedAt    time.Time
	LastActivity time.Time
	Rows         uint16
	Cols         uint16
}
