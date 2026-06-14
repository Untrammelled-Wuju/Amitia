package qdrant

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
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

func StartQdrant() error {
	cfg := config.AppCfg.Qdrant
	workDir, err := os.Getwd()
	if err != nil {
		log.Error("获取工作目录失败:", err)
		return err
	}

	qdrantDir := filepath.Join(workDir, "qdrant")
	configDir := filepath.Join(qdrantDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")

	_ = os.MkdirAll(configDir, 0755)

	configContent := fmt.Sprintf("service:\n  http_port: %d\n  grpc_port: %d\n", cfg.Port, cfg.Port+1)
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	var qdrantPath string
	switch runtime.GOOS {
	case "windows":
		qdrantPath = filepath.Join(qdrantDir, "qdrant.exe")
	case "linux":
		if runtime.GOARCH == "arm64" {
			qdrantPath = filepath.Join(qdrantDir, "qdrant_linux_aarch64")
		} else {
			qdrantPath = filepath.Join(qdrantDir, "qdrant_linux_x86")
		}
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	if _, err := os.Stat(qdrantPath); os.IsNotExist(err) {
		return fmt.Errorf("Qdrant程序不存在: %s", qdrantPath)
	}

	cmd := exec.Command(qdrantPath, "--config-path", configPath)
	cmd.Dir = qdrantDir
	cmd.Stdout = &qdrantWriter{}
	cmd.Stderr = &qdrantWriter{}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动Qdrant失败: %w", err)
	}

	qdrantCmd = cmd
	log.Info("Qdrant已启动", "port", cfg.Port, "pid", cmd.Process.Pid)
	return nil
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