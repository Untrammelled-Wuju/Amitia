package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type InstanceLock struct {
	mu       sync.Mutex
	path     string
	id       string
	pid      int
	startedAt time.Time
	dataDir  string
	host     string
	released bool
}

var (
	ErrInstanceLockHeld = errors.New("lifecycle: instance lock already held")
	ErrInstanceLockStale = errors.New("lifecycle: instance lock stale")
)

func NewInstanceLock(path string) *InstanceLock {
	return &InstanceLock{path: path, pid: os.Getpid()}
}

func (l *InstanceLock) Acquire(ctx context.Context, dataDir, host string) (string, error) {
	if err := validateContext(ctx); err != nil {
		return "", err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.id != "" && !l.released {
		return l.id, nil
	}
	if existing, err := os.ReadFile(l.path); err == nil {
		parts := strings.SplitN(string(existing), "\n", 5)
		if len(parts) >= 2 {
			oldPID, _ := strconv.Atoi(parts[1])
			if oldPID > 0 && isProcessAlive(oldPID) {
				return "", fmt.Errorf("%w: pid=%d", ErrInstanceLockHeld, oldPID)
			}
		}
	}
	l.id = newID("instance")
	l.pid = os.Getpid()
	l.startedAt = now()
	l.dataDir = dataDir
	l.host = host
	content := fmt.Sprintf("%s\n%d\n%s\n%s\n%s\n", l.id, l.pid, l.startedAt.Format(time.RFC3339), dataDir, host)
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return "", err
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, l.path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	l.released = false
	return l.id, nil
}

func (l *InstanceLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return nil
	}
	_ = os.Remove(l.path)
	l.released = true
	return nil
}

func (l *InstanceLock) ID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.id
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		return isWindowsProcessAlive(pid)
	}
	return isUnixProcessAlive(pid)
}
