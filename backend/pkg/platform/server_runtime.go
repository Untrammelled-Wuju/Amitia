package platform

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ServerRuntime struct {
	dataDir string
	once    sync.Once
}

func (s *ServerRuntime) Name() string {
	return "server-remote"
}

func (s *ServerRuntime) Descriptor() RuntimeDescriptor {
	return newRuntimeDescriptor(hostPlatformFromGOOS(runtime.GOOS), RuntimeKindRemote, guestPlatformFromGOOS(runtime.GOOS))
}

var _ RuntimePlatform = (*ServerRuntime)(nil)

func (s *ServerRuntime) ExecutableSuffix() string {
	return ""
}

func (s *ServerRuntime) BinarySuffix() string {
	return ""
}

func (s *ServerRuntime) RootFSDir() string {
	if v := os.Getenv("AMITIA_ROOTFS_DIR"); v != "" {
		return v
	}
	return ""
}

func (s *ServerRuntime) DefaultDataDir() string {
	s.once.Do(func() {
		if env := os.Getenv("AMITIA_DATA_DIR"); env != "" {
			s.dataDir = env
		}
		if s.dataDir == "" {
			s.dataDir = "data"
		}
	})
	return s.dataDir
}

func (s *ServerRuntime) IsWindows() bool {
	return false
}

func (s *ServerRuntime) IsLinux() bool {
	return true
}

func (s *ServerRuntime) IsAndroid() bool {
	return false
}

func (s *ServerRuntime) IsAndroidEmbedded() bool {
	return false
}

func (s *ServerRuntime) KillExistingServer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	if _, _, splitErr := net.SplitHostPort(addr); splitErr != nil {
		return fmt.Errorf("parse addr failed: %w", splitErr)
	}

	if pid, pidErr := s.ReadPidFile(s.DefaultDataDir()); pidErr == nil && pid > 0 {
		if pid == os.Getpid() {
			return fmt.Errorf("port occupied by current process pid=%d", pid)
		}
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() {
				_, _ = proc.Wait()
				close(done)
			}()
			select {
			case <-done:
				time.Sleep(1 * time.Second)
				return nil
			case <-time.After(2 * time.Second):
				_ = proc.Kill()
				time.Sleep(1 * time.Second)
				return nil
			}
		}

		_ = s.RemovePidFile(s.DefaultDataDir())
		return fmt.Errorf("port occupied by pid=%d (process not responsive)", pid)
	}

	return fmt.Errorf("port occupied by unknown process (no valid pid file found)")
}

func (s *ServerRuntime) WritePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = s.DefaultDataDir()
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (s *ServerRuntime) ReadPidFile(dataDir string) (int, error) {
	if dataDir == "" {
		dataDir = s.DefaultDataDir()
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (s *ServerRuntime) RemovePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = s.DefaultDataDir()
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.Remove(pidPath)
}
