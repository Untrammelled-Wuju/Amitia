package qdrant

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func SetQdrantShuttingDown() {}

func IsQdrantShuttingDown() bool { return false }

// StartQdrant is deprecated. Use BuildQdrantProcessSpec and the runtimehost.ProcessSupervisor.
func StartQdrant() error {
	return errors.New("qdrant: use BuildQdrantProcessSpec instead")
}

// StopQdrant is deprecated. Use runtimehost.ProcessSupervisor.Stop.
func StopQdrant() {}

type qdrantWriter struct{}

func (w *qdrantWriter) Write(p []byte) (int, error) {
	lines := string(p)
	for len(lines) > 0 && (lines[len(lines)-1] == '\n' || lines[len(lines)-1] == '\r') {
		lines = lines[:len(lines)-1]
	}
	if lines != "" {
		log.Info("[Qdrant]", lines)
	}
	return len(p), nil
}

func resolveQdrantBinaryPath(qdrantDir string) string {
	if cfgPath := config.AppCfg.Providers.VectorStore.Qdrant.BinaryPath; cfgPath != "" {
		if filepath.IsAbs(cfgPath) {
			return cfgPath
		}
		return filepath.Join(qdrantDir, cfgPath)
	}

	p := platform.Get()
	if rootfs := p.RootFSDir(); rootfs != "" && !p.IsWindows() {
		binName := "qdrant" + p.BinarySuffix()
		rootfsPath := filepath.Join(rootfs, "bin", binName)
		if _, err := os.Stat(rootfsPath); err == nil {
			return rootfsPath
		}
		if IsLinuxARM64() {
			candidates := []string{
				filepath.Join(rootfs, "bin", "qdrant_linux_aarch64"),
				filepath.Join(rootfs, "bin", "qdrant"),
				filepath.Join(rootfs, "qdrant"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
		if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
			candidates := []string{
				filepath.Join(rootfs, "bin", "qdrant_linux_x86"),
				filepath.Join(rootfs, "bin", "qdrant"),
				filepath.Join(rootfs, "qdrant"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
	}

	suffix := p.ExecutableSuffix()
	defaultPath := filepath.Join(qdrantDir, "qdrant"+suffix)
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	if IsLinuxARM64() {
		candidates := []string{
			filepath.Join(qdrantDir, "qdrant_linux_aarch64"),
			filepath.Join(qdrantDir, "qdrant"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		candidates := []string{
			filepath.Join(qdrantDir, "qdrant_linux_x86"),
			filepath.Join(qdrantDir, "qdrant"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	return defaultPath
}

func resolveQdrantWorkDir(qdrantDir string) string {
	p := platform.Get()
	if rootfs := p.RootFSDir(); rootfs != "" && !p.IsWindows() {
		binDir := filepath.Join(rootfs, "bin")
		if info, err := os.Stat(binDir); err == nil && info.IsDir() {
			return binDir
		}
	}
	return qdrantDir
}

func resolveQdrantDataDir(qdrantDir string) string {
	if cfgDir := config.AppCfg.Providers.VectorStore.Qdrant.DataDir; cfgDir != "" {
		if filepath.IsAbs(cfgDir) {
			return cfgDir
		}
		return filepath.Join(qdrantDir, cfgDir)
	}
	return filepath.Join(qdrantDir, "data")
}

func isQdrantAlive(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", port)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func IsLinuxARM64() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "arm64"
}

func ensureQdrantBinary(qdrantPath, qdrantDir string) error {
	if _, err := os.Stat(qdrantPath); err == nil {
		return nil
	}

	candidates := []string{"qdrant.exe.zip", "qdrant.zip"}
	if IsLinuxARM64() {
		candidates = []string{"qdrant_linux_aarch64.zip", "qdrant-aarch64-unknown-linux-gnu.zip", "qdrant-arm64.zip", "qdrant.zip"}
	} else if runtime.GOOS == "linux" {
		candidates = []string{"qdrant_linux_x86.zip", "qdrant-x86_64-unknown-linux-gnu.zip", "qdrant.zip"}
	}

	for _, name := range candidates {
		zipPath := filepath.Join(qdrantDir, name)
		if _, err := os.Stat(zipPath); err == nil {
			log.Info("正在解压Qdrant程序", "zip", zipPath)
			if err := util.UnzipFile(zipPath, qdrantDir); err != nil {
				return fmt.Errorf("解压Qdrant程序失败: %w", err)
			}
			if _, err := os.Stat(qdrantPath); err == nil {
				return nil
			}
			return fmt.Errorf("Qdrant压缩包中未找到程序: %s", qdrantPath)
		}
	}

	return fmt.Errorf("Qdrant程序不存在: %s", qdrantPath)
}

func WaitForQdrant(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Info("Qdrant端口就绪", "port", port)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return fmt.Errorf("等待Qdrant启动超时(30s)")
}
