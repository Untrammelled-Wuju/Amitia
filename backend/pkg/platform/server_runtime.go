package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

func (s *ServerRuntime) KillExistingServer(addr, dataDir string) error {
	if dataDir == "" {
		dataDir = s.DefaultDataDir()
	}
	return killExistingServer(addr, dataDir, s.ReadPidFile, s.RemovePidFile)
}

func (s *ServerRuntime) WritePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = s.DefaultDataDir()
	}
	return writePidFile(dataDir)
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
