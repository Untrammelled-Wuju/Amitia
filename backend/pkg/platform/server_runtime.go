package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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

	_, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return fmt.Errorf("parse addr failed: %w", splitErr)
	}

	if pid, pidErr := s.ReadPidFile(s.DefaultDataDir()); pidErr == nil && pid > 0 {
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
	}

	if path, lookupErr := exec.LookPath("lsof"); lookupErr == nil {
		out, _ := exec.Command(path, "-ti", ":"+port).Output()
		for _, raw := range strings.Fields(string(out)) {
			if pid, err2 := strconv.Atoi(raw); err2 == nil {
				if proc, findErr := os.FindProcess(pid); findErr == nil {
					_ = proc.Kill()
				}
			}
		}
		time.Sleep(2 * time.Second)
		return nil
	}

	if path, lookupErr := exec.LookPath("fuser"); lookupErr == nil {
		_ = exec.Command(path, "-k", port+"/tcp").Run()
		time.Sleep(2 * time.Second)
		return nil
	}

	return nil
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
