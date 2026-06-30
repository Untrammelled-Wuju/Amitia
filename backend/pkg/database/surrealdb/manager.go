// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/util"
)

var surrealCmd *exec.Cmd

type surrealWriter struct{}

func (w *surrealWriter) Write(p []byte) (int, error) {
	lines := string(p)
	for len(lines) > 0 && (lines[len(lines)-1] == '\n' || lines[len(lines)-1] == '\r') {
		lines = lines[:len(lines)-1]
	}
	if lines != "" {
		log.Info("[SurrealDB]", lines)
	}
	return len(p), nil
}

func killExistingSurreal(port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return
	}
	conn.Close()

	log.Warn("检测到旧SurrealDB进程，正在终止...")
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "surreal.exe").Run()
	} else {
		exec.Command("pkill", "-9", "surreal").Run()
	}
	time.Sleep(2 * time.Second)

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			log.Info("旧SurrealDB已释放端口", port)
			return
		}
		conn.Close()
		time.Sleep(1 * time.Second)
	}
	log.Warn("旧SurrealDB未能在10秒内释放端口，继续启动...")
}

func StartSurreal() error {
	cfg := config.AppCfg.Surreal
	workDir := util.RuntimeRoot()

	killExistingSurreal(cfg.Port)

	surrealDir := filepath.Join(workDir, "surrealdb")

	var surrealPath string
	switch runtime.GOOS {
	case "windows":
		surrealPath = filepath.Join(surrealDir, "surreal.exe")
	case "linux":
		surrealPath = filepath.Join(surrealDir, "surreal")
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	if _, err := os.Stat(surrealPath); os.IsNotExist(err) {
		if err := ensureSurrealBinary(surrealPath, surrealDir); err != nil {
			return err
		}
	}

	bindAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	storagePath := cfg.DataPath
	if storagePath != "" && storagePath != "memory" {
		absPath := util.ResolveRuntimePath(workDir, storagePath)
		os.MkdirAll(absPath, 0755)
		storagePath = "surrealkv:" + absPath
	}

	cmd := exec.Command(surrealPath, "start",
		"--log", "info",
		"--user", cfg.Username,
		"--pass", cfg.Password,
		"--bind", bindAddr,
		storagePath,
	)
	cmd.Dir = surrealDir
	cmd.Stdout = &surrealWriter{}
	cmd.Stderr = &surrealWriter{}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动SurrealDB失败: %w", err)
	}

	surrealCmd = cmd
	log.Info("SurrealDB已启动", "port", cfg.Port, "pid", cmd.Process.Pid)
	return nil
}

func ensureSurrealBinary(surrealPath, surrealDir string) error {
	if _, err := os.Stat(surrealPath); err == nil {
		return nil
	}

	for _, name := range []string{"surreal.exe.zip", "surreal.zip"} {
		zipPath := filepath.Join(surrealDir, name)
		if _, err := os.Stat(zipPath); err == nil {
			log.Info("正在解压SurrealDB程序", "zip", zipPath)
			if err := unzipSurreal(zipPath, surrealDir); err != nil {
				return fmt.Errorf("解压SurrealDB程序失败: %w", err)
			}
			if _, err := os.Stat(surrealPath); err == nil {
				return nil
			}
			return fmt.Errorf("SurrealDB压缩包中未找到程序: %s", surrealPath)
		}
	}

	return fmt.Errorf("SurrealDB程序不存在: %s", surrealPath)
}

func unzipSurreal(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		targetPath := filepath.Join(targetDir, file.Name)
		cleanPath, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}
		if cleanPath != targetDir && !strings.HasPrefix(cleanPath, targetDir+string(os.PathSeparator)) {
			return fmt.Errorf("非法压缩包路径: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func StopSurreal() {
	if surrealCmd == nil || surrealCmd.Process == nil {
		return
	}
	_ = surrealCmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = surrealCmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = surrealCmd.Process.Kill()
		log.Warn("强制终止SurrealDB进程(超时)")
	}
	log.Info("SurrealDB已停止")
}

func WaitForSurreal(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Info("SurrealDB端口就绪", "port", port)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return fmt.Errorf("等待SurrealDB启动超时(30s)")
}
