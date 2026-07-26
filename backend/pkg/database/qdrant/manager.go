package qdrant

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

var qdrantCmd *exec.Cmd

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

func killExistingQdrant(port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return
	}
	conn.Close()

	log.Warn("检测到旧Qdrant进程，正在终止...")
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "qdrant.exe").Run()
	} else {
		exec.Command("pkill", "-9", "qdrant").Run()
	}
	time.Sleep(2 * time.Second)

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			log.Info("旧Qdrant已释放端口", port)
			return
		}
		conn.Close()
		time.Sleep(1 * time.Second)
	}
	log.Warn("旧Qdrant未能在10秒内释放端口，继续启动...")
}

func resolveQdrantBinaryPath(qdrantDir string) string {
	if cfgPath := config.AppCfg.Qdrant.BinaryPath; cfgPath != "" {
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
	if cfgDir := config.AppCfg.Qdrant.DataDir; cfgDir != "" {
		if filepath.IsAbs(cfgDir) {
			return cfgDir
		}
		return filepath.Join(qdrantDir, cfgDir)
	}
	return filepath.Join(qdrantDir, "data")
}

func StartQdrant() error {
	cfg := config.AppCfg.Qdrant
	workDir := util.RuntimeRoot()

	killExistingQdrant(cfg.Port)

	qdrantDir := filepath.Join(workDir, "qdrant")
	configDir := filepath.Join(qdrantDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")

	_ = os.MkdirAll(configDir, 0755)

	configContent := fmt.Sprintf("service:\n  http_port: %d\n  grpc_port: %d\nstorage:\n  storage_path: %s\n", cfg.Port, cfg.Port+1, resolveQdrantDataDir(qdrantDir))
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	qdrantPath := resolveQdrantBinaryPath(qdrantDir)

	if _, err := os.Stat(qdrantPath); os.IsNotExist(err) {
		if err := ensureQdrantBinary(qdrantPath, qdrantDir); err != nil {
			return err
		}
	}

	dataDir := resolveQdrantDataDir(qdrantDir)
	_ = os.MkdirAll(dataDir, 0755)

	cmd := exec.Command(qdrantPath, "--config-path", configPath)
	cmd.Dir = resolveQdrantWorkDir(qdrantDir)
	cmd.Stdout = &qdrantWriter{}
	cmd.Stderr = &qdrantWriter{}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动Qdrant失败: %w", err)
	}

	qdrantCmd = cmd
	log.Info("Qdrant已启动", "port", cfg.Port, "pid", cmd.Process.Pid)
	return nil
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

func StopQdrant() {
	if qdrantCmd == nil || qdrantCmd.Process == nil {
		return
	}
	_ = qdrantCmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = qdrantCmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = qdrantCmd.Process.Kill()
		log.Warn("强制终止Qdrant进程(超时)")
	}
	log.Info("Qdrant已停止")
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
