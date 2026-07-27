package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const defaultDevHost = "localhost:18899"

type devWorkspaceRegistration struct {
	WorkspacePath string `json:"workspacePath"`
	ManifestPath  string `json:"manifestPath,omitempty"`
	Watch         bool   `json:"watch"`
	AutoReload    bool   `json:"autoReload"`
}

type devWorkspaceResponse struct {
	WorkspaceID string `json:"workspaceId"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type devReloadRequest struct {
	WorkspaceID  string   `json:"workspaceId"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
}

type devReloadResponse struct {
	Success  bool     `json:"success"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Duration string   `json:"duration,omitempty"`
}

func runDev(args []string, output *Output) int {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	manifestPath := fs.String("manifest", "", "manifest.json 路径（默认为工作区下的 manifest.json）")
	watch := fs.Bool("watch", true, "启用文件监听")
	autoReload := fs.Bool("auto-reload", true, "变更后自动重载")
	host := fs.String("host", defaultDevHost, "Developer Host 地址（host:port）")
	fs.Parse(args)

	if fs.NArg() < 1 {
		output.fail(ExitConfig, "用法: amitia-ext dev <workspace-path> [--manifest <path>] [--watch] [--auto-reload] [--host host:port]")
	}

	workspacePath, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析工作区路径失败: %v", err))
	}
	if info, err := os.Stat(workspacePath); err != nil || !info.IsDir() {
		output.fail(ExitEnv, fmt.Sprintf("工作区目录不存在: %s", workspacePath))
	}

	manifestAbs := *manifestPath
	if manifestAbs == "" {
		manifestAbs = filepath.Join(workspacePath, "manifest.json")
	}
	if _, err := os.Stat(manifestAbs); err != nil {
		output.fail(ExitConfig, fmt.Sprintf("manifest.json 不存在: %s", manifestAbs))
	}
	manifestAbs, err = filepath.Abs(manifestAbs)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析 manifest 路径失败: %v", err))
	}

	hostAddr := normalizeHost(*host)
	baseURL := fmt.Sprintf("http://%s", hostAddr)

	reg := devWorkspaceRegistration{
		WorkspacePath: workspacePath,
		ManifestPath:  manifestAbs,
		Watch:         *watch,
		AutoReload:    *autoReload,
	}

	workspaceID, err := registerDevWorkspace(baseURL, reg)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("注册工作区失败: %v", err))
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("开发模式已启动: %s", workspacePath),
		Data: map[string]any{
			"workspaceId": workspaceID,
			"workspace":   workspacePath,
			"manifest":    manifestAbs,
			"host":        hostAddr,
			"watch":       *watch,
			"autoReload":  *autoReload,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	if *watch {
		go watchAndReload(ctx, baseURL, workspaceID, workspacePath, *autoReload, output)
	}

	output.infof("按 Ctrl+C 退出并清理工作区...\n")

	select {
	case <-sigCh:
	case <-ctx.Done():
	}

	output.infof("\n正在清理工作区...\n")
	if err := unregisterDevWorkspace(baseURL, workspaceID); err != nil {
		output.infof("清理工作区失败: %v\n", err)
	} else {
		output.infof("工作区已清理: %s\n", workspaceID)
	}
	return ExitSuccess
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return defaultDevHost
	}
	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	} else if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}
	return host
}

func registerDevWorkspace(baseURL string, reg devWorkspaceRegistration) (string, error) {
	body, err := json.Marshal(reg)
	if err != nil {
		return "", fmt.Errorf("序列化注册请求失败: %w", err)
	}
	url := baseURL + "/api/extensions/dev-mode/workspaces"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建注册请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接 Developer Host 失败 (%s): %w", url, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("注册工作区返回状态 %d: %s", resp.StatusCode, string(respBody))
	}
	var out devWorkspaceResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("解析注册响应失败: %w", err)
	}
	if out.WorkspaceID == "" {
		return "", fmt.Errorf("注册响应缺少 workspaceId")
	}
	return out.WorkspaceID, nil
}

func unregisterDevWorkspace(baseURL, workspaceID string) error {
	url := fmt.Sprintf("%s/api/extensions/dev-mode/workspaces/%s", baseURL, workspaceID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("创建清理请求失败: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("清理工作区请求失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("清理工作区返回状态 %d", resp.StatusCode)
	}
	return nil
}

func watchAndReload(ctx context.Context, baseURL, workspaceID, workspacePath string, autoReload bool, output *Output) {
	watchDirs := []string{"modules", "resources", "assets", "migrations", "licenses", "docs"}
	prevSnapshot := snapshotWorkspace(workspacePath, watchDirs)

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currSnapshot := snapshotWorkspace(workspacePath, watchDirs)
			changed := diffSnapshot(prevSnapshot, currSnapshot)
			if len(changed) == 0 {
				continue
			}
			prevSnapshot = currSnapshot
			output.infof("检测到变更: %s\n", strings.Join(changed, ", "))
			if !autoReload {
				continue
			}
			result, err := triggerReload(baseURL, workspaceID, changed)
			if err != nil {
				output.infof("重载失败: %v\n", err)
				continue
			}
			if result.Success {
				output.infof("重载成功 (%s)\n", result.Duration)
			} else {
				output.infof("重载完成但存在错误:\n")
				for _, e := range result.Errors {
					output.infof("  错误: %s\n", e)
				}
				for _, w := range result.Warnings {
					output.infof("  警告: %s\n", w)
				}
			}
		}
	}
}

func snapshotWorkspace(root string, dirs []string) map[string]int64 {
	snapshot := make(map[string]int64)
	for _, d := range dirs {
		dirPath := filepath.Join(root, d)
		filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			snapshot[filepath.ToSlash(rel)] = info.ModTime().UnixNano()
			return nil
		})
	}
	manifestPath := filepath.Join(root, "manifest.json")
	if info, err := os.Stat(manifestPath); err == nil {
		snapshot["manifest.json"] = info.ModTime().UnixNano()
	}
	return snapshot
}

func diffSnapshot(prev, curr map[string]int64) []string {
	var changed []string
	for k, v := range curr {
		if pv, ok := prev[k]; !ok || pv != v {
			changed = append(changed, k)
		}
	}
	for k := range prev {
		if _, ok := curr[k]; !ok {
			changed = append(changed, k+" (deleted)")
		}
	}
	return changed
}

func triggerReload(baseURL, workspaceID string, changedFiles []string) (*devReloadResponse, error) {
	reqBody := devReloadRequest{
		WorkspaceID:  workspaceID,
		ChangedFiles: changedFiles,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化重载请求失败: %w", err)
	}
	url := fmt.Sprintf("%s/api/extensions/dev-mode/workspaces/%s/reload", baseURL, workspaceID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建重载请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("重载请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("重载返回状态 %d: %s", resp.StatusCode, string(respBody))
	}
	var out devReloadResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析重载响应失败: %w", err)
	}
	return &out, nil
}
