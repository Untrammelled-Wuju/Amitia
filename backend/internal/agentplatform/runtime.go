package agentplatform

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RuntimeManager struct {
	db                 *gorm.DB
	dataDir            string
	workspaceRegistrar RuntimeWorkspaceRegistrar
}

type RuntimeWorkspace struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	RootURI string `json:"rootUri"`
}

type RuntimeWorkspaceRegistrar interface {
	RegisterRuntimeWorkspace(ctx context.Context, agentID, name, localRoot string) (RuntimeWorkspace, error)
}

type CreateRuntimeInput struct {
	AgentID            string  `json:"agentId"`
	Provider           string  `json:"provider"`
	Image              string  `json:"image"`
	NetworkPolicyID    string  `json:"networkPolicyId"`
	CPUQuota           float64 `json:"cpuQuota"`
	MemoryBytes        int64   `json:"memoryBytes"`
	DiskBytes          int64   `json:"diskBytes"`
	ProcessLimit       int     `json:"processLimit"`
	IdleTimeoutSeconds int     `json:"idleTimeoutSeconds"`
}

type CreateHostControlInput struct {
	AgentID          string         `json:"agentId"`
	Capabilities     []string       `json:"capabilities"`
	Permission       map[string]any `json:"permission"`
	ExpiresInSeconds int            `json:"expiresInSeconds"`
}

type RuntimeExecInput struct {
	Command        string `json:"command"`
	WorkingDir     string `json:"workingDir"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type RuntimeExecResult struct {
	RuntimeID  string `json:"runtimeId"`
	Provider   string `json:"provider"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
}

func NewRuntimeManager(db *gorm.DB, dataDir string) *RuntimeManager {
	return &RuntimeManager{db: db, dataDir: strings.TrimSpace(dataDir)}
}

func (m *RuntimeManager) Create(ctx context.Context, spaceID string, input CreateRuntimeInput) (RuntimeInstance, error) {
	if m == nil || m.db == nil {
		return RuntimeInstance{}, errors.New("Agent 运行时服务不可用")
	}
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = "local"
	}
	if provider == "isolated" {
		provider = "oci"
		if strings.TrimSpace(input.Image) == "" {
			input.Image = "ubuntu:24.04"
		}
	}
	switch provider {
	case "local", "oci", "remote", "android":
	default:
		return RuntimeInstance{}, fmt.Errorf("不支持的 Agent 运行时 Provider: %s", provider)
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return RuntimeInstance{}, errors.New("Agent ID 不能为空")
	}
	if err := m.requireAgent(ctx, spaceID, input.AgentID); err != nil {
		return RuntimeInstance{}, err
	}
	if provider == "oci" && strings.TrimSpace(input.Image) == "" {
		return RuntimeInstance{}, errors.New("OCI 运行时必须指定镜像")
	}
	if provider == "remote" {
		return RuntimeInstance{}, errors.New("远程运行时必须通过 endpoint 注册")
	}
	if input.CPUQuota <= 0 {
		input.CPUQuota = 1
	}
	if input.MemoryBytes <= 0 {
		input.MemoryBytes = 1 << 30
	}
	if input.DiskBytes <= 0 {
		input.DiskBytes = 10 << 30
	}
	if input.ProcessLimit <= 0 {
		input.ProcessLimit = 64
	}
	if input.IdleTimeoutSeconds <= 0 {
		input.IdleTimeoutSeconds = 1800
	}
	now := time.Now().UTC()
	instance := RuntimeInstance{
		ID: newID("runtime"), SpaceID: spaceID, AgentID: input.AgentID,
		Provider: provider, Image: strings.TrimSpace(input.Image), State: "created",
		NetworkPolicyID: input.NetworkPolicyID, CPUQuota: input.CPUQuota,
		MemoryBytes: input.MemoryBytes, DiskBytes: input.DiskBytes, ProcessLimit: input.ProcessLimit,
		IdleTimeoutSeconds: input.IdleTimeoutSeconds, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	instance.RootPath = filepath.Join(m.runtimeRoot(), instance.ID)
	if err := os.MkdirAll(instance.RootPath, 0o700); err != nil {
		return RuntimeInstance{}, fmt.Errorf("创建 Agent 电脑目录: %w", err)
	}
	workspace := filepath.Join(instance.RootPath, "workspace")
	home := filepath.Join(instance.RootPath, "home")
	for _, path := range []string{workspace, home} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return RuntimeInstance{}, fmt.Errorf("初始化 Agent 电脑目录 %s: %w", path, err)
		}
	}
	if provider == "local" && m.workspaceRegistrar != nil {
		registered, registerErr := m.workspaceRegistrar.RegisterRuntimeWorkspace(ctx, input.AgentID, "Agent Computer "+input.AgentID, workspace)
		if registerErr != nil {
			return RuntimeInstance{}, fmt.Errorf("注册 Agent 电脑工作区: %w", registerErr)
		}
		instance.WorkspaceID = registered.ID
		instance.WorkspaceRootURI = registered.RootURI
	}
	if err := m.db.WithContext(ctx).Create(&instance).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("创建 Agent 运行时: %w", err)
	}
	if err := m.record(ctx, instance, "runtime.lifecycle", "created", "Agent 电脑已创建", ""); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) RegisterRemote(ctx context.Context, spaceID string, input CreateRuntimeInput, endpoint string) (RuntimeInstance, error) {
	endpoint, err := normalizeRemoteRuntimeEndpoint(endpoint)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return RuntimeInstance{}, errors.New("Agent ID 不能为空")
	}
	if err := m.requireAgent(ctx, spaceID, input.AgentID); err != nil {
		return RuntimeInstance{}, err
	}
	input.Provider = "remote"
	now := time.Now().UTC()
	instance := RuntimeInstance{
		ID: newID("runtime"), SpaceID: spaceID, AgentID: input.AgentID, Provider: "remote",
		State: "running", Endpoint: endpoint, Revision: 1, CreatedAt: now, UpdatedAt: now,
		CPUQuota: input.CPUQuota, MemoryBytes: input.MemoryBytes, DiskBytes: input.DiskBytes,
		ProcessLimit: input.ProcessLimit, IdleTimeoutSeconds: input.IdleTimeoutSeconds,
		NetworkPolicyID: input.NetworkPolicyID,
	}
	if instance.CPUQuota <= 0 {
		instance.CPUQuota = 1
	}
	if instance.MemoryBytes <= 0 {
		instance.MemoryBytes = 1 << 30
	}
	if instance.DiskBytes <= 0 {
		instance.DiskBytes = 10 << 30
	}
	if instance.ProcessLimit <= 0 {
		instance.ProcessLimit = 64
	}
	if instance.IdleTimeoutSeconds <= 0 {
		instance.IdleTimeoutSeconds = 1800
	}
	if err := m.db.WithContext(ctx).Create(&instance).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("注册远程 Agent 运行时: %w", err)
	}
	if err := m.record(ctx, instance, "runtime.lifecycle", "running", "远程 Agent 电脑已注册", endpoint); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func normalizeRemoteRuntimeEndpoint(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", errors.New("远程运行时 endpoint 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("解析远程运行时 endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("远程运行时 endpoint 必须使用 http 或 https")
	}
	if parsed.Host == "" {
		return "", errors.New("远程运行时 endpoint 缺少主机名")
	}
	if parsed.User != nil {
		return "", errors.New("远程运行时 endpoint 不允许包含用户凭据")
	}
	if parsed.Fragment != "" {
		return "", errors.New("远程运行时 endpoint 不允许包含片段")
	}
	return value, nil
}

func (m *RuntimeManager) Start(ctx context.Context, spaceID, runtimeID string) (RuntimeInstance, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if instance.State == "running" {
		return instance, nil
	}
	if instance.State == "paused" {
		return m.Resume(ctx, spaceID, runtimeID)
	}
	if instance.State == "disabled" {
		return RuntimeInstance{}, errors.New("Agent 电脑已停用")
	}
	switch instance.Provider {
	case "local":
		if err := os.MkdirAll(filepath.Join(instance.RootPath, "workspace"), 0o700); err != nil {
			return RuntimeInstance{}, fmt.Errorf("启动本地 Agent 电脑: %w", err)
		}
	case "oci":
		args := []string{
			"run", "-d", "--name", m.containerName(instance.ID),
			"--cpus", strconv.FormatFloat(instance.CPUQuota, 'f', 2, 64),
			"--memory", strconv.FormatInt(instance.MemoryBytes, 10),
			"--pids-limit", strconv.Itoa(instance.ProcessLimit),
			"--network", "none",
			"-v", instance.RootPath + "/workspace:/workspace",
			"-e", "HOME=/home/agent",
			"-w", "/workspace",
			instance.Image, "sleep", "infinity",
		}
		output, runErr := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if runErr != nil {
			return RuntimeInstance{}, fmt.Errorf("启动 OCI Agent 电脑: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		instance.ProviderRef = m.containerName(instance.ID)
	case "remote":
		if instance.Endpoint == "" {
			return RuntimeInstance{}, errors.New("远程 Agent 电脑缺少 endpoint")
		}
	case "android":
		return RuntimeInstance{}, errors.New("Android Agent 电脑必须通过 Native Bridge 启动")
	}
	now := time.Now().UTC()
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ? AND revision = ?", instance.ID, instance.Revision).Updates(map[string]any{
		"state": "running", "started_at": now, "stopped_at": nil,
		"provider_ref": instance.ProviderRef, "revision": instance.Revision + 1,
		"updated_at": now, "last_error": "",
	}).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("保存 Agent 运行时启动状态: %w", err)
	}
	instance.State, instance.StartedAt, instance.Revision = "running", &now, instance.Revision+1
	if err := m.record(ctx, instance, "runtime.lifecycle", "running", "Agent 电脑已启动", ""); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) Pause(ctx context.Context, spaceID, runtimeID string) (RuntimeInstance, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if instance.State == "paused" {
		return instance, nil
	}
	if instance.State != "running" {
		return RuntimeInstance{}, fmt.Errorf("Agent 电脑状态为 %s，不能暂停", instance.State)
	}
	if instance.Provider == "oci" {
		output, pauseErr := exec.CommandContext(ctx, "docker", "pause", m.containerName(instance.ID)).CombinedOutput()
		if pauseErr != nil {
			return RuntimeInstance{}, fmt.Errorf("暂停 OCI Agent 电脑: %w: %s", pauseErr, strings.TrimSpace(string(output)))
		}
	}
	now := time.Now().UTC()
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ? AND revision = ?", instance.ID, instance.Revision).Updates(map[string]any{
		"state": "paused", "revision": instance.Revision + 1, "updated_at": now,
	}).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("保存 Agent 运行时暂停状态: %w", err)
	}
	instance.State, instance.Revision, instance.UpdatedAt = "paused", instance.Revision+1, now
	if err := m.record(ctx, instance, "runtime.lifecycle", "paused", "Agent 电脑已暂停", ""); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) Resume(ctx context.Context, spaceID, runtimeID string) (RuntimeInstance, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if instance.State == "running" {
		return instance, nil
	}
	if instance.State != "paused" {
		return RuntimeInstance{}, fmt.Errorf("Agent 电脑状态为 %s，不能恢复", instance.State)
	}
	if instance.Provider == "oci" {
		output, resumeErr := exec.CommandContext(ctx, "docker", "unpause", m.containerName(instance.ID)).CombinedOutput()
		if resumeErr != nil {
			return RuntimeInstance{}, fmt.Errorf("恢复 OCI Agent 电脑: %w: %s", resumeErr, strings.TrimSpace(string(output)))
		}
	}
	now := time.Now().UTC()
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ? AND revision = ?", instance.ID, instance.Revision).Updates(map[string]any{
		"state": "running", "revision": instance.Revision + 1, "updated_at": now,
	}).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("保存 Agent 运行时恢复状态: %w", err)
	}
	instance.State, instance.Revision, instance.UpdatedAt = "running", instance.Revision+1, now
	if err := m.record(ctx, instance, "runtime.lifecycle", "resumed", "Agent 电脑已恢复", ""); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) Stop(ctx context.Context, spaceID, runtimeID string) (RuntimeInstance, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if instance.Provider == "oci" && instance.ProviderRef != "" {
		output, stopErr := exec.CommandContext(ctx, "docker", "rm", "-f", instance.ProviderRef).CombinedOutput()
		if stopErr != nil && !strings.Contains(strings.ToLower(string(output)), "no such container") {
			return RuntimeInstance{}, fmt.Errorf("停止 OCI Agent 电脑: %w: %s", stopErr, strings.TrimSpace(string(output)))
		}
	}
	now := time.Now().UTC()
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ? AND revision = ?", instance.ID, instance.Revision).Updates(map[string]any{
		"state": "stopped", "stopped_at": now, "provider_ref": "", "revision": instance.Revision + 1, "updated_at": now,
	}).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("保存 Agent 运行时停止状态: %w", err)
	}
	instance.State, instance.StoppedAt, instance.ProviderRef = "stopped", &now, ""
	instance.Revision++
	instance.UpdatedAt = now
	if err := m.record(ctx, instance, "runtime.lifecycle", "stopped", "Agent 电脑已停止", ""); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) List(ctx context.Context, spaceID, agentID string) ([]RuntimeInstance, error) {
	query := m.db.WithContext(ctx).Where("space_id = ?", spaceID)
	if strings.TrimSpace(agentID) != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	instances := make([]RuntimeInstance, 0)
	if err := query.Order("updated_at DESC").Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("读取 Agent 运行时列表: %w", err)
	}
	return instances, nil
}

func (m *RuntimeManager) Exec(ctx context.Context, spaceID, runtimeID string, input RuntimeExecInput) (RuntimeExecResult, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeExecResult{}, err
	}
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return RuntimeExecResult{}, errors.New("Agent 电脑命令不能为空")
	}
	if instance.State != "running" {
		return RuntimeExecResult{}, fmt.Errorf("Agent 电脑状态为 %s，不能执行命令", instance.State)
	}
	timeout := input.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 600 {
		timeout = 600
	}
	input.TimeoutSeconds = timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	startedAt := time.Now()
	result := RuntimeExecResult{RuntimeID: instance.ID, Provider: instance.Provider}
	switch instance.Provider {
	case "local":
		output, exitCode, runErr := runRuntimeCommand(execCtx, "", filepath.Join(instance.RootPath, "workspace"), input.WorkingDir, command)
		result.Output, result.ExitCode = output, exitCode
		if runErr != nil {
			return result, fmt.Errorf("执行 Agent 本机命令: %w", runErr)
		}
	case "oci":
		if instance.ProviderRef == "" {
			return RuntimeExecResult{}, errors.New("OCI Agent 电脑没有运行容器")
		}
		workdir, workdirErr := containerWorkingDir(input.WorkingDir)
		if workdirErr != nil {
			return RuntimeExecResult{}, workdirErr
		}
		output, exitCode, runErr := runRuntimeCommand(execCtx, "docker", "", workdir, command, "exec", "-w", workdir, instance.ProviderRef, "/bin/sh", "-lc", command)
		result.Output, result.ExitCode = output, exitCode
		if runErr != nil {
			return result, fmt.Errorf("执行 OCI Agent 电脑命令: %w", runErr)
		}
	case "remote":
		output, exitCode, runErr := executeRemoteRuntime(execCtx, instance, input)
		result.Output, result.ExitCode = output, exitCode
		if runErr != nil {
			return result, fmt.Errorf("执行远程 Agent 电脑命令: %w", runErr)
		}
	default:
		return RuntimeExecResult{}, fmt.Errorf("Agent 电脑 Provider %s 不支持终端执行", instance.Provider)
	}
	result.DurationMS = time.Since(startedAt).Milliseconds()
	if err := m.record(ctx, instance, "runtime.exec", "completed", "Agent 电脑命令已执行", fmt.Sprintf("exit=%d command=%s", result.ExitCode, command)); err != nil {
		return result, err
	}
	return result, nil
}

func runRuntimeCommand(ctx context.Context, executable, directory, workingDir, command string, prefix ...string) (string, int, error) {
	binary := executableOrShell(executable)
	args := append([]string(nil), prefix...)
	if len(prefix) == 0 {
		if goruntime.GOOS == "windows" {
			args = []string{"/C", command}
		} else {
			args = []string{"-lc", command}
		}
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	if directory != "" {
		cmd.Dir = directory
	}
	if strings.TrimSpace(workingDir) != "" && len(prefix) == 0 {
		candidate := filepath.Join(directory, filepath.Clean(workingDir))
		relative, err := filepath.Rel(directory, candidate)
		if err != nil || strings.HasPrefix(relative, "..") {
			return "", -1, errors.New("Agent 电脑工作目录越界")
		}
		cmd.Dir = candidate
	}
	var output limitedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output.String(), -1, fmt.Errorf("命令执行超时: %w", ctx.Err())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return output.String(), exitErr.ExitCode(), nil
		}
		return output.String(), -1, err
	}
	return output.String(), 0, nil
}

func containerWorkingDir(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "/workspace", nil
	}
	clean := path.Clean("/workspace/" + strings.TrimPrefix(strings.TrimSpace(value), "/"))
	if clean != "/workspace" && !strings.HasPrefix(clean, "/workspace/") {
		return "", errors.New("Agent 电脑工作目录越界")
	}
	return clean, nil
}

func executableOrShell(executable string) string {
	if strings.TrimSpace(executable) != "" {
		return executable
	}
	if goruntime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/sh"
}

type limitedOutput struct {
	buffer strings.Builder
}

func (w *limitedOutput) Write(value []byte) (int, error) {
	const limit = 16 << 20
	if w.buffer.Len()+len(value) > limit {
		return 0, errors.New("Agent 电脑命令输出超过 16 MiB")
	}
	return w.buffer.Write(value)
}

func (w *limitedOutput) String() string {
	return w.buffer.String()
}

func executeRemoteRuntime(ctx context.Context, instance RuntimeInstance, input RuntimeExecInput) (string, int, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(instance.Endpoint), "/")
	if endpoint == "" {
		return "", -1, errors.New("远程 Agent 电脑缺少 endpoint")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return "", -1, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/exec", bytes.NewReader(body))
	if err != nil {
		return "", -1, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: time.Duration(input.TimeoutSeconds+5) * time.Second}).Do(request)
	if err != nil {
		return "", -1, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return "", -1, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return string(responseBody), -1, fmt.Errorf("远程 Agent 电脑返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var remote RuntimeExecResult
	if err := json.Unmarshal(responseBody, &remote); err != nil {
		return string(responseBody), -1, fmt.Errorf("解析远程 Agent 电脑响应: %w", err)
	}
	return remote.Output, remote.ExitCode, nil
}

func (m *RuntimeManager) Snapshot(ctx context.Context, spaceID, runtimeID, name string) (RuntimeSnapshot, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snapshotDir := filepath.Join(m.snapshotRoot(), instance.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("创建 Agent 快照目录: %w", err)
	}
	snapshotID := newID("snapshot")
	path := filepath.Join(snapshotDir, snapshotID+".tar.gz")
	switch instance.Provider {
	case "local":
		if err := archiveDirectory(instance.RootPath, path); err != nil {
			return RuntimeSnapshot{}, err
		}
	case "oci":
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if createErr != nil {
			return RuntimeSnapshot{}, fmt.Errorf("创建 OCI Agent 快照文件: %w", createErr)
		}
		command := exec.CommandContext(ctx, "docker", "export", m.containerName(instance.ID))
		command.Stdout = file
		output, exportErr := command.StderrPipe()
		if exportErr != nil {
			file.Close()
			return RuntimeSnapshot{}, fmt.Errorf("读取 OCI 导出错误: %w", exportErr)
		}
		runErr := command.Run()
		errorOutput, _ := io.ReadAll(output)
		closeErr := file.Close()
		if runErr != nil {
			return RuntimeSnapshot{}, fmt.Errorf("导出 OCI Agent 快照: %w: %s", runErr, strings.TrimSpace(string(errorOutput)))
		}
		if closeErr != nil {
			return RuntimeSnapshot{}, fmt.Errorf("关闭 OCI Agent 快照: %w", closeErr)
		}
	default:
		return RuntimeSnapshot{}, fmt.Errorf("运行时 Provider %s 不支持本地快照", instance.Provider)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = "snapshot-" + time.Now().UTC().Format("20060102T150405Z")
	}
	manifestJSON, _, err := hashJSON(map[string]any{
		"runtimeId": instance.ID, "provider": instance.Provider, "path": path,
		"sha256": hash, "createdAt": time.Now().UTC(),
	})
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snapshot := RuntimeSnapshot{ID: snapshotID, SpaceID: spaceID, AgentID: instance.AgentID, RuntimeID: instance.ID, Name: name, Path: path, ManifestJSON: manifestJSON, ContentHash: hash, CreatedAt: time.Now().UTC()}
	if err := m.db.WithContext(ctx).Create(&snapshot).Error; err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("保存 Agent 快照: %w", err)
	}
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ?", instance.ID).Update("snapshot_id", snapshot.ID).Error; err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("记录 Agent 当前快照: %w", err)
	}
	if err := m.record(ctx, instance, "runtime.snapshot", "created", "Agent 电脑快照已创建", name); err != nil {
		return RuntimeSnapshot{}, err
	}
	return snapshot, nil
}

func (m *RuntimeManager) Reset(ctx context.Context, spaceID, runtimeID, snapshotID string) (RuntimeInstance, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeInstance{}, err
	}
	var snapshot RuntimeSnapshot
	if err := m.db.WithContext(ctx).Where("id = ? AND runtime_id = ? AND space_id = ?", snapshotID, runtimeID, spaceID).First(&snapshot).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("读取 Agent 快照: %w", err)
	}
	if instance.State == "running" {
		if _, err := m.Stop(ctx, spaceID, runtimeID); err != nil {
			return RuntimeInstance{}, err
		}
		instance, err = m.get(ctx, spaceID, runtimeID)
		if err != nil {
			return RuntimeInstance{}, err
		}
	}
	if instance.Provider == "oci" {
		imageRef := m.containerName(instance.ID) + ":snapshot-" + snapshot.ID
		output, importErr := exec.CommandContext(ctx, "docker", "import", snapshot.Path, imageRef).CombinedOutput()
		if importErr != nil {
			return RuntimeInstance{}, fmt.Errorf("导入 OCI Agent 快照: %w: %s", importErr, strings.TrimSpace(string(output)))
		}
		args := []string{
			"run", "-d", "--name", m.containerName(instance.ID),
			"--cpus", strconv.FormatFloat(instance.CPUQuota, 'f', 2, 64),
			"--memory", strconv.FormatInt(instance.MemoryBytes, 10),
			"--pids-limit", strconv.Itoa(instance.ProcessLimit),
			"--network", "none",
			imageRef, "sleep", "infinity",
		}
		output, runErr := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if runErr != nil {
			return RuntimeInstance{}, fmt.Errorf("启动恢复后的 OCI Agent 电脑: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		now := time.Now().UTC()
		if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ?", instance.ID).Updates(map[string]any{
			"state": "running", "snapshot_id": snapshot.ID, "provider_ref": m.containerName(instance.ID),
			"started_at": now, "stopped_at": nil, "updated_at": now, "revision": instance.Revision + 1, "last_error": "",
		}).Error; err != nil {
			return RuntimeInstance{}, fmt.Errorf("保存 OCI Agent 快照恢复状态: %w", err)
		}
		instance.State, instance.SnapshotID, instance.ProviderRef = "running", snapshot.ID, m.containerName(instance.ID)
		instance.StartedAt, instance.Revision, instance.UpdatedAt = &now, instance.Revision+1, now
		if err := m.record(ctx, instance, "runtime.snapshot", "restored", "Agent 电脑快照已恢复", snapshot.Name); err != nil {
			return RuntimeInstance{}, err
		}
		return instance, nil
	}
	if instance.Provider != "local" {
		return RuntimeInstance{}, fmt.Errorf("运行时 Provider %s 暂不支持原地恢复快照", instance.Provider)
	}
	tempRoot := instance.RootPath + ".restore-" + stableID("restore", time.Now().UTC().String())
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return RuntimeInstance{}, fmt.Errorf("创建 Agent 恢复目录: %w", err)
	}
	if err := extractArchive(snapshot.Path, tempRoot); err != nil {
		return RuntimeInstance{}, err
	}
	oldRoot := instance.RootPath + ".old-" + stableID("old", time.Now().UTC().String())
	if err := os.Rename(instance.RootPath, oldRoot); err != nil {
		return RuntimeInstance{}, fmt.Errorf("备份当前 Agent 电脑目录: %w", err)
	}
	if err := os.Rename(tempRoot, instance.RootPath); err != nil {
		if rollbackErr := os.Rename(oldRoot, instance.RootPath); rollbackErr != nil {
			return RuntimeInstance{}, fmt.Errorf("恢复 Agent 电脑目录: %w; 回滚原目录失败: %v", err, rollbackErr)
		}
		return RuntimeInstance{}, fmt.Errorf("恢复 Agent 电脑目录: %w", err)
	}
	if err := os.RemoveAll(oldRoot); err != nil {
		return RuntimeInstance{}, fmt.Errorf("清理 Agent 快照恢复备份: %w", err)
	}
	now := time.Now().UTC()
	if err := m.db.WithContext(ctx).Model(&RuntimeInstance{}).Where("id = ?", instance.ID).Updates(map[string]any{
		"state": "stopped", "snapshot_id": snapshot.ID, "updated_at": now, "revision": instance.Revision + 1,
	}).Error; err != nil {
		return RuntimeInstance{}, fmt.Errorf("保存 Agent 电脑恢复状态: %w", err)
	}
	instance.State, instance.SnapshotID, instance.Revision, instance.UpdatedAt = "stopped", snapshot.ID, instance.Revision+1, now
	if err := m.record(ctx, instance, "runtime.snapshot", "restored", "Agent 电脑快照已恢复", snapshot.Name); err != nil {
		return RuntimeInstance{}, err
	}
	return instance, nil
}

func (m *RuntimeManager) ImportSnapshot(ctx context.Context, spaceID, agentID, runtimeID, name, sourcePath, expectedHash string) (RuntimeSnapshot, error) {
	instance, err := m.get(ctx, spaceID, runtimeID)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if instance.AgentID != agentID {
		return RuntimeSnapshot{}, errors.New("导入快照与 Agent 不匹配")
	}
	actualHash, err := fileSHA256(sourcePath)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if expectedHash != "" && actualHash != expectedHash {
		return RuntimeSnapshot{}, errors.New("导入 Agent 快照哈希校验失败")
	}
	snapshotDir := filepath.Join(m.snapshotRoot(), instance.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("创建导入快照目录: %w", err)
	}
	snapshotID := newID("snapshot")
	target := filepath.Join(snapshotDir, snapshotID+".tar.gz")
	source, err := os.Open(sourcePath)
	if err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("打开导入 Agent 快照: %w", err)
	}
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		source.Close()
		return RuntimeSnapshot{}, fmt.Errorf("创建导入 Agent 快照: %w", err)
	}
	_, copyErr := io.Copy(destination, source)
	sourceCloseErr := source.Close()
	destinationCloseErr := destination.Close()
	if copyErr != nil {
		return RuntimeSnapshot{}, fmt.Errorf("复制导入 Agent 快照: %w", copyErr)
	}
	if sourceCloseErr != nil {
		return RuntimeSnapshot{}, fmt.Errorf("关闭导入 Agent 快照源文件: %w", sourceCloseErr)
	}
	if destinationCloseErr != nil {
		return RuntimeSnapshot{}, fmt.Errorf("关闭导入 Agent 快照目标文件: %w", destinationCloseErr)
	}
	manifestJSON, _, err := hashJSON(map[string]any{
		"runtimeId": runtimeID, "agentId": agentID, "provider": instance.Provider,
		"sha256": actualHash, "importedAt": time.Now().UTC(),
	})
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snapshot := RuntimeSnapshot{
		ID: snapshotID, SpaceID: spaceID, AgentID: agentID, RuntimeID: runtimeID,
		Name: name, Path: target, ManifestJSON: manifestJSON, ContentHash: actualHash,
		CreatedAt: time.Now().UTC(),
	}
	if err := m.db.WithContext(ctx).Create(&snapshot).Error; err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("保存导入 Agent 快照: %w", err)
	}
	return snapshot, nil
}

func (m *RuntimeManager) CreateHostControl(ctx context.Context, spaceID string, input CreateHostControlInput) (HostControlSession, error) {
	if strings.TrimSpace(input.AgentID) == "" {
		return HostControlSession{}, errors.New("Agent ID 不能为空")
	}
	if err := m.requireAgent(ctx, spaceID, input.AgentID); err != nil {
		return HostControlSession{}, err
	}
	allowedCapabilities := map[string]bool{
		"screen.capture": true, "input.mouse": true, "input.keyboard": true, "window.control": true,
	}
	capabilities := make([]string, 0, len(input.Capabilities))
	seen := map[string]bool{}
	for _, capability := range input.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" || seen[capability] {
			continue
		}
		if !allowedCapabilities[capability] {
			return HostControlSession{}, fmt.Errorf("不支持的桌面接管能力: %s", capability)
		}
		seen[capability] = true
		capabilities = append(capabilities, capability)
	}
	if len(capabilities) == 0 {
		return HostControlSession{}, errors.New("桌面接管至少需要一项能力")
	}
	platforms := map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"}
	platform := platforms[goruntime.GOOS]
	if platform == "" {
		platform = goruntime.GOOS
	}
	expires := input.ExpiresInSeconds
	if expires <= 0 {
		expires = 1800
	}
	if expires > 86400 {
		expires = 86400
	}
	capabilitiesJSON, _, err := hashJSON(capabilities)
	if err != nil {
		return HostControlSession{}, err
	}
	permissionJSON, _, err := hashJSON(input.Permission)
	if err != nil {
		return HostControlSession{}, err
	}
	now := time.Now().UTC()
	session := HostControlSession{
		ID: newID("hostcontrol"), SpaceID: spaceID, AgentID: input.AgentID, Platform: platform,
		Capabilities: capabilitiesJSON, PermissionJSON: permissionJSON, Status: "pending",
		ExpiresAt: now.Add(time.Duration(expires) * time.Second), CreatedAt: now,
	}
	if err := m.db.WithContext(ctx).Create(&session).Error; err != nil {
		return HostControlSession{}, fmt.Errorf("创建桌面接管会话: %w", err)
	}
	if err := m.record(ctx, RuntimeInstance{SpaceID: spaceID, AgentID: input.AgentID}, "host.control", "pending", "等待桌面接管授权", ""); err != nil {
		return HostControlSession{}, err
	}
	return session, nil
}

func (m *RuntimeManager) ApproveHostControl(ctx context.Context, spaceID, sessionID, actorID string) (HostControlSession, error) {
	var session HostControlSession
	if err := m.db.WithContext(ctx).Where("id = ? AND space_id = ? AND status = ?", sessionID, spaceID, "pending").First(&session).Error; err != nil {
		return HostControlSession{}, fmt.Errorf("读取桌面接管会话: %w", err)
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return HostControlSession{}, errors.New("桌面接管授权已经过期")
	}
	if err := m.db.WithContext(ctx).Model(&HostControlSession{}).Where("id = ?", session.ID).Updates(map[string]any{"status": "active", "approved_by": actorID}).Error; err != nil {
		return HostControlSession{}, fmt.Errorf("批准桌面接管: %w", err)
	}
	session.Status = "active"
	session.ApprovedBy = actorID
	return session, nil
}

func (m *RuntimeManager) EndHostControl(ctx context.Context, spaceID, sessionID string) (HostControlSession, error) {
	now := time.Now().UTC()
	result := m.db.WithContext(ctx).Model(&HostControlSession{}).Where("id = ? AND space_id = ? AND status = ?", sessionID, spaceID, "active").Updates(map[string]any{"status": "ended", "ended_at": now})
	if result.Error != nil {
		return HostControlSession{}, fmt.Errorf("结束桌面接管: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return HostControlSession{}, errors.New("桌面接管会话不存在或未激活")
	}
	var session HostControlSession
	if err := m.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error; err != nil {
		return HostControlSession{}, fmt.Errorf("读取桌面接管会话: %w", err)
	}
	return session, nil
}

func (m *RuntimeManager) get(ctx context.Context, spaceID, runtimeID string) (RuntimeInstance, error) {
	var instance RuntimeInstance
	if err := m.db.WithContext(ctx).Where("id = ? AND space_id = ?", runtimeID, spaceID).First(&instance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RuntimeInstance{}, errors.New("Agent 电脑不存在")
		}
		return RuntimeInstance{}, fmt.Errorf("读取 Agent 电脑: %w", err)
	}
	return instance, nil
}

func (m *RuntimeManager) requireAgent(ctx context.Context, spaceID, agentID string) error {
	var agent AgentProfile
	if err := m.db.WithContext(ctx).Where(
		"id = ? AND space_id = ? AND deleted_at IS NULL", agentID, spaceID,
	).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Agent 不存在")
		}
		return fmt.Errorf("读取 Agent 电脑所属 Agent: %w", err)
	}
	return nil
}

func (m *RuntimeManager) runtimeRoot() string {
	root := m.dataDir
	if root == "" {
		root = os.TempDir()
	}
	return filepath.Join(root, "agent-platform", "runtimes")
}

func (m *RuntimeManager) snapshotRoot() string {
	root := m.dataDir
	if root == "" {
		root = os.TempDir()
	}
	return filepath.Join(root, "agent-platform", "snapshots")
}

func (m *RuntimeManager) containerName(runtimeID string) string {
	clean := strings.NewReplacer("_", "-", "/", "-", ":", "-").Replace(runtimeID)
	if len(clean) > 48 {
		clean = clean[:48]
	}
	return "amitia-agent-" + clean
}

func (m *RuntimeManager) record(ctx context.Context, instance RuntimeInstance, kind, state, title, detail string) error {
	event := ActivityEvent{
		ID: newID("activity"), SpaceID: instance.SpaceID, AgentID: instance.AgentID,
		RuntimeID: instance.ID, Kind: kind, State: state, Title: title, Detail: detail,
		OccurredAt: time.Now().UTC(), PayloadJSON: "{}",
	}
	applyActivityActor(ctx, &event)
	return m.db.WithContext(ctx).Create(&event).Error
}

func archiveDirectory(root, target string) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("创建 Agent 快照文件: %w", err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == "." {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	tarErr := tarWriter.Close()
	gzipErr := gzipWriter.Close()
	closeErr := file.Close()
	if walkErr != nil {
		return fmt.Errorf("归档 Agent 电脑: %w", walkErr)
	}
	if tarErr != nil {
		return fmt.Errorf("完成 Agent 快照归档: %w", tarErr)
	}
	if gzipErr != nil {
		return fmt.Errorf("压缩 Agent 快照: %w", gzipErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭 Agent 快照: %w", closeErr)
	}
	return nil
}

func extractArchive(path, target string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开 Agent 快照: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("解压 Agent 快照: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 Agent 快照条目: %w", err)
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return fmt.Errorf("Agent 快照包含非法路径: %s", header.Name)
		}
		destination := filepath.Join(target, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("恢复 Agent 快照目录: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				return fmt.Errorf("创建 Agent 快照父目录: %w", err)
			}
			output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("创建 Agent 快照文件: %w", err)
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return fmt.Errorf("恢复 Agent 快照文件: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("关闭 Agent 快照文件: %w", closeErr)
			}
		default:
			return fmt.Errorf("Agent 快照包含不支持的条目类型: %d", header.Typeflag)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开 Agent 快照校验文件: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算 Agent 快照哈希: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func RuntimeMetadataJSON(instance RuntimeInstance) string {
	data, err := json.Marshal(instance)
	if err != nil {
		return "{}"
	}
	return string(data)
}
