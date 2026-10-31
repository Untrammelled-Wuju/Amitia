package task_runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeprofile"
	amitialog "log"
)

type taskProcessMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type taskDiagnosticWriter struct {
	writer    io.Writer
	remaining int
}

func (w *taskDiagnosticWriter) Write(value []byte) (int, error) {
	count := len(value)
	if len(value) > w.remaining {
		value = value[:w.remaining]
	}
	w.remaining -= len(value)
	if len(value) > 0 {
		_, err := w.writer.Write(value)
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

func (h *TaskProcessHost) runProcess(ctx context.Context, input, checkpoint json.RawMessage, deadline *time.Time, attempt, maxAttempts int, callbacks ProcessCallbacks) (int, error) {
	profile := runtimeprofile.CurrentProcessProfile()
	if err := validateHostExecutionIdentity(profile, os.Geteuid()); err != nil {
		return 1, err
	}
	resourceSupport := process.ResourceLimitsSupported()
	resourcePolicy := os.Getenv(cloudTaskResourcePolicyEnv)
	if err := validateCloudHostResourceLimits(profile, h.config.RequireSandbox, h.config.NativeLimits, resourceSupport, resourcePolicy); err != nil {
		return 1, err
	}
	if profile == runtimeprofile.ProfileCloudCore && !h.config.RequireSandbox && strings.TrimSpace(resourcePolicy) == "allow-degraded" {
		if missing := cloudHostMissingResourceLimits(h.config.NativeLimits, resourceSupport); len(missing) > 0 {
			slog.Warn("Cloud Core 任务宿主运行在资源隔离降级模式", "task_run_id", h.config.TaskRunID, "missing_hard_limits", strings.Join(missing, ","))
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return 1, err
	}
	if len(input) > 1<<20 || !json.Valid(input) || len(checkpoint) > 1<<20 || len(checkpoint) > 0 && !json.Valid(checkpoint) {
		return 1, errors.New("任务输入或检查点无效")
	}
	for _, path := range []string{h.config.NodePath, h.config.HostPath, h.config.EntryPath} {
		if !filepath.IsAbs(path) {
			return 1, errors.New("任务运行时和入口必须使用已解析的绝对路径")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return 1, errors.New("任务运行时或入口不可用")
		}
	}
	processCtx, stop := context.WithCancel(ctx)
	defer stop()
	if deadline != nil {
		var deadlineStop context.CancelFunc
		processCtx, deadlineStop = context.WithDeadline(processCtx, *deadline)
		defer deadlineStop()
	}
	nonce := uuid.NewString()
	nodeArgs, err := taskNodeLaunchArgs(profile, h.config)
	if err != nil {
		return 1, err
	}
	cloudNetworkMode, err := cloudHostNetworkMode(profile, h.config.RequireSandbox, os.Getenv(cloudTaskNetworkPolicyEnv))
	if err != nil {
		return 1, err
	}
	cmd := exec.CommandContext(processCtx, h.config.NodePath, nodeArgs...)
	cmd.Dir = h.config.WorkDir

	cmd.WaitDelay = 3 * time.Second
	limits := h.config.NativeLimits
	if h.config.RequireSandbox {
		args := []string{"--max-old-space-size=128", "--preserve-symlinks", "--preserve-symlinks-main", "--permission", "--allow-fs-read=" + filepath.Dir(h.config.HostPath), "--allow-fs-read=" + h.config.BundleRoot, "--allow-fs-read=" + h.config.WorkDir, "--allow-fs-write=" + h.config.WorkDir, h.config.HostPath}
		plan, err := trusted_service.PrepareTaskSandbox(h.config.NodePath, args, h.config.WorkDir, filepath.Dir(h.config.HostPath), h.config.BundleRoot, limits)
		if err != nil {
			return 1, err
		}
		if plan.Cleanup != nil {
			defer plan.Cleanup()
		}
		cmd = exec.CommandContext(processCtx, plan.Path, plan.Args...)
		cmd.WaitDelay = 3 * time.Second
		cmd.Dir = plan.WorkingDir
		cmd.ExtraFiles = plan.ExtraFiles
		limits = plan.SupervisorLimits
	} else if cloudNetworkMode == cloudTaskNetworkIsolated {

		bundleRoot := h.config.BundleRoot
		if bundleRoot == "" {
			bundleRoot = filepath.Dir(h.config.EntryPath)
		}
		plan, err := trusted_service.PrepareIsolatedTaskNetworkLaunch(
			h.config.NodePath, nodeArgs, h.config.WorkDir,
			filepath.Dir(h.config.HostPath), bundleRoot,
		)
		if err != nil {
			return 1, fmt.Errorf("云端任务无法获得网络隔离: %w", err)
		}
		if plan.Cleanup != nil {
			defer plan.Cleanup()
		}
		cmd = exec.CommandContext(processCtx, plan.Path, plan.Args...)
		cmd.Dir = plan.WorkingDir
		cmd.ExtraFiles = plan.ExtraFiles
		cmd.WaitDelay = 3 * time.Second

		if plan.AfterStart != nil {
			return 1, NewTaskError(ErrTaskScopeDenied, "云端任务网络沙箱不得要求启动后激活")
		}
	} else if cloudNetworkMode == cloudTaskNetworkDegraded {
		slog.Warn("Cloud Core 任务在管理员显式授权的网络隔离降级模式运行", "task_run_id", h.config.TaskRunID)
	}
	environment := process.NewEnvironmentBuilder()
	if profile == runtimeprofile.ProfileCloudCore && !h.config.RequireSandbox {
		environment.Set("HOME", h.config.WorkDir)
		environment.Set("TMP", h.config.WorkDir)
		environment.Set("TEMP", h.config.WorkDir)
		environment.Set("TMPDIR", h.config.WorkDir)
		environment.Set("XDG_CACHE_HOME", h.config.WorkDir)
	}
	cmd.Env = environment.Build()
	cmd.Env = append(cmd.Env, "AMITIA_GENERATION="+strconv.FormatInt(h.config.Generation, 10))
	cmd.Env = append(cmd.Env, "AMITIA_ENTRY_HASH="+h.config.EntryHash)
	cmd.Env = append(cmd.Env, "AMITIA_BUNDLE_ROOT="+h.config.BundleRoot, "AMITIA_BUNDLE_HASH="+h.config.BundleHash)
	cmd.Env = append(cmd.Env, "AMITIA_INSTANCE_ID="+h.config.InstanceID, "AMITIA_TASK_RUN_ID="+h.config.TaskRunID, "AMITIA_EXTENSION_ID="+h.config.ExtensionID, "AMITIA_MODULE_ID="+h.config.ModuleID, "AMITIA_NONCE="+nonce, "AMITIA_DEFINITION_HASH="+h.config.DefHash, "AMITIA_WORKSPACE_PATH="+h.config.WorkDir)
	process.ConfigureProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 1, err
	}
	defer stdin.Close()

	stdoutReader, stdoutWriter := io.Pipe()
	cmd.Stdout = stdoutWriter
	defer stdoutReader.Close()
	defer stdoutWriter.Close()
	cmd.Stderr = io.Discard
	if h.config.Diagnostics != nil {
		cmd.Stderr = &taskDiagnosticWriter{writer: h.config.Diagnostics, remaining: 8192}
	}

	tree, err := process.StartProcessTreeWithLimits(cmd, limits)
	if err != nil {
		return 1, err
	}
	defer process.CloseProcessTree(tree)
	type processExit struct {
		code int
		err  error
	}

	exited := make(chan processExit, 1)
	go func() {
		waitErr := cmd.Wait()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		if err := stdoutWriter.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		exited <- processExit{code: code, err: waitErr}
	}()
	var writeMu sync.Mutex
	write := func(value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err = stdin.Write(append(encoded, '\n'))
		return err
	}
	watchDone := make(chan struct{})
	watchExited := make(chan struct{})
	defer func() { close(watchDone); <-watchExited }()
	go func() {
		defer close(watchExited)
		guardTicker := time.NewTicker(200 * time.Millisecond)
		defer guardTicker.Stop()
		for {
			select {
			case <-guardTicker.C:
				if err := coordination.ValidateCurrent(ctx); err != nil {
					stop()
					if err := process.TerminateProcessTree(cmd.Process.Pid, tree); err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
					return
				}
			case <-h.pauseCh:
				if err := write(map[string]any{"jsonrpc": "2.0", "method": "task.pause", "params": map[string]any{"task_run_id": h.config.TaskRunID, "timeout_ms": 10000}}); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			case <-processCtx.Done():
				if err := process.TerminateProcessTree(cmd.Process.Pid, tree); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				return
			case <-h.cancelCh:
				if err := write(map[string]any{"jsonrpc": "2.0", "method": "task.cancel", "params": map[string]string{"task_run_id": h.config.TaskRunID, "reason": "cancelled"}}); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				timer := time.NewTimer(2 * time.Second)
				defer timer.Stop()
				select {
				case <-timer.C:
					stop()
					if err := process.TerminateProcessTree(cmd.Process.Pid, tree); err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
				case <-processCtx.Done():
					if err := process.TerminateProcessTree(cmd.Process.Pid, tree); err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
				case <-watchDone:
				}
				return
			case <-watchDone:
				return
			}
		}
	}()
	hello, ready, finished := false, false, false
	paused := false
	var checkpointVersion int64
	var finalStatus, finalCode, finalMessage, finalArtifactID string
	var finalResult json.RawMessage
	sessionID := uuid.NewString()
	scanner := bufio.NewScanner(stdoutReader)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var protocolErr error
	for scanner.Scan() {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			protocolErr = err
			break
		}
		var message taskProcessMessage
		if json.Unmarshal(scanner.Bytes(), &message) != nil || message.JSONRPC != "2.0" {
			protocolErr = errors.New("任务进程协议无效")
			break
		}
		if message.Method == "" {
			continue
		}
		if message.Method == "runtime.hello" {
			var value struct {
				Protocol   string   `json:"protocol_version"`
				Generation int64    `json:"generation"`
				InstanceID string   `json:"instance_id"`
				Nonce      string   `json:"nonce"`
				Hash       string   `json:"definition_hash"`
				Runtime    string   `json:"runtime_type"`
				Features   []string `json:"features"`
			}
			if hello || json.Unmarshal(message.Params, &value) != nil || value.Protocol != "2.0" || value.Generation != h.config.Generation || value.InstanceID != h.config.InstanceID || value.Nonce != nonce || value.Hash != h.config.DefHash || value.Runtime != "task" {
				protocolErr = errors.New("任务进程身份验证失败")
				break
			}
			if h.config.EntryHash != "" {
				entryPin, checkpointAck, bundlePin := false, false, false
				for _, feature := range value.Features {
					entryPin = entryPin || feature == "entry_pin"
					checkpointAck = checkpointAck || feature == "checkpoint_ack"
					bundlePin = bundlePin || feature == "bundle_pin"
				}
				if !entryPin || !checkpointAck || h.config.BundleHash != "" && !bundlePin {
					protocolErr = errors.New("任务运行时缺少源码校验或检查点保存确认能力")
					break
				}
			}
			hello = true
			protocolErr = write(map[string]any{"jsonrpc": "2.0", "method": "host.welcome", "params": map[string]any{"session_id": sessionID, "session_token": uuid.NewString(), "limits": map[string]any{}, "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}})
		} else if message.Method == "runtime.ready" {
			var value struct {
				SessionID string `json:"session_id"`
			}
			if !hello || ready || json.Unmarshal(message.Params, &value) != nil || value.SessionID != sessionID {
				protocolErr = errors.New("任务进程就绪确认无效")
				break
			}
			ready = true
			var deadlineMillis int64
			if deadline != nil {
				deadlineMillis = deadline.UnixMilli()
			}
			checkpointValue := checkpoint
			if len(checkpointValue) == 0 {
				checkpointValue = json.RawMessage("null")
			}
			protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": "execute", "method": "task.execute", "params": map[string]any{"task_run_id": h.config.TaskRunID, "entry": h.config.EntryPath, "input": input, "checkpoint": checkpointValue, "deadline": deadlineMillis, "attempt": attempt, "max_attempts": maxAttempts, "host_capabilities": h.config.HostCapabilities}})
			if protocolErr == nil {
				close(h.readyCh)
			}
		} else {
			if !ready || finished {
				protocolErr = errors.New("任务进程发送了越序事件")
				break
			}
			var value struct {
				TaskRunID                     string `json:"task_run_id"`
				Sequence                      int64  `json:"sequence"`
				Current, Total, Percentage    *float64
				Stage, Message, Level, Status string
				Fields                        map[string]interface{} `json:"fields"`
				Version                       int64                  `json:"version"`
				CheckpointVersion             int64                  `json:"checkpoint_version"`
				Paused                        bool                   `json:"paused"`
				Payload                       json.RawMessage        `json:"payload"`
				Result                        struct {
					Mode       string          `json:"mode"`
					Data       json.RawMessage `json:"data"`
					ArtifactID string          `json:"artifact_id"`
				} `json:"result"`
				Error struct{ Code, Message string } `json:"error"`
			}
			if json.Unmarshal(message.Params, &value) != nil || value.TaskRunID != h.config.TaskRunID {
				protocolErr = errors.New("任务事件归属不一致")
				break
			}
			switch message.Method {
			case "task.progress":
				if callbacks.OnProgress != nil {
					callbacks.OnProgress(value.Sequence, value.Current, value.Total, value.Percentage, value.Stage, value.Message)
				}
			case "task.checkpoint", "task.checkpoint.save":
				if value.Version < 1 || !json.Valid(value.Payload) || len(value.Payload) > 1<<20 {
					protocolErr = errors.New("任务检查点无效")
					break
				}
				if message.Method == "task.checkpoint.save" {
					var checkpoint struct {
						Cursor int64 `json:"cursor"`
					}
					if json.Unmarshal(value.Payload, &checkpoint) != nil || checkpoint.Cursor != value.Version {
						protocolErr = errors.New("检查点内容与保存版本不一致")
						break
					}
					if len(message.ID) == 0 || len(message.ID) > 128 || callbacks.OnCheckpointConfirmed == nil {
						protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32601, "message": "检查点所有者确认端口不可用"}})
						break
					}
					if err := callbacks.OnCheckpointConfirmed(value.Version, value.Payload, hashBytes(value.Payload)); err != nil {
						protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32001, "message": err.Error()}})
						break
					}
					if err := coordination.ValidateCurrent(ctx); err != nil {
						protocolErr = err
						break
					}
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "result": map[string]any{"version": value.Version}})
				} else if callbacks.OnCheckpointConfirmed != nil {
					if err := callbacks.OnCheckpointConfirmed(value.Version, value.Payload, hashBytes(value.Payload)); err != nil {
						protocolErr = err
						break
					}
				} else if callbacks.OnCheckpoint != nil {
					callbacks.OnCheckpoint(value.Version, value.Payload, hashBytes(value.Payload))
				}
				checkpointVersion = value.Version
			case "task.pause_ack":
				h.mu.Lock()
				pausing := h.state == "pausing"
				h.mu.Unlock()
				if !pausing || paused || !value.Paused || value.CheckpointVersion < 1 || value.CheckpointVersion != checkpointVersion {
					protocolErr = errors.New("任务暂停缺少有效检查点确认")
					break
				}
				paused = true
				h.mu.Lock()
				h.pauseVersion = value.CheckpointVersion
				h.mu.Unlock()
			case "log.write":
				if callbacks.OnLog != nil && len(message.Params) <= 16<<10 {
					callbacks.OnLog(value.Level, value.Message, value.Fields)
				}
			case "task.finished":
				if paused {
					protocolErr = errors.New("任务暂停后仍发送结果")
					break
				}
				validInline := value.Result.Mode == "inline_json" && len(value.Result.Data) <= 64<<10 && json.Valid(value.Result.Data) && value.Result.ArtifactID == ""
				validArtifact := value.Result.Mode == "artifact" && len(value.Result.ArtifactID) == 73 && strings.HasPrefix(value.Result.ArtifactID, "artifact-") && len(value.Result.Data) == 0
				if value.Status != "succeeded" && value.Status != "failed" || value.Status == "succeeded" && !validInline && !validArtifact {
					protocolErr = errors.New("任务结果无效或需要所有者产物端口")
					break
				}
				finished = true
				finalStatus, finalResult, finalCode, finalMessage = value.Status, value.Result.Data, value.Error.Code, value.Error.Message
				finalArtifactID = value.Result.ArtifactID
			case "task.shutdown":
			case "task.host.executeTool", "task.host.emitEvent":
				if len(message.ID) == 0 || len(message.ID) > 128 {
					protocolErr = errors.New("当前任务运行时未提供已授权的工具执行或事件发布端口")
					break
				}
				if callbacks.OnRequest == nil {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32601, "message": "当前任务运行时未提供已授权的工具执行或事件发布端口"}})
					break
				}
				response, err := callbacks.OnRequest(processCtx, nonce+"/"+string(message.ID), message.Method, message.Params)
				if err != nil {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32001, "message": err.Error()}})
				} else if !json.Valid(response) || len(response) > 80<<10 {
					protocolErr = errors.New("任务Native确认无效或超过限制")
				} else {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "result": response})
				}
			case "task.storage.get", "task.storage.set", "task.storage.delete", "task.artifact.saveData", "task.artifact.saveFile", "task.artifact.list":
				if len(message.ID) == 0 || len(message.ID) > 128 || callbacks.OnRequest == nil {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32601, "message": "任务所有者接口不可用"}})
					break
				}
				response, err := callbacks.OnRequest(processCtx, nonce+"/"+string(message.ID), message.Method, message.Params)
				if err != nil {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32001, "message": err.Error()}})
				} else if !json.Valid(response) || len(response) > 512<<10 {
					protocolErr = errors.New("任务接口返回值无效或超过限制")
				} else {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "result": response})
				}
			default:
				if len(message.ID) > 0 {
					protocolErr = write(map[string]any{"jsonrpc": "2.0", "id": message.ID, "error": map[string]any{"code": -32601, "message": "任务接口尚未授予执行端口"}})
				} else {
					protocolErr = fmt.Errorf("未知任务事件: %s", message.Method)
				}
			}
		}
		if protocolErr != nil {
			break
		}
	}
	if protocolErr == nil {
		protocolErr = scanner.Err()
	}
	if protocolErr != nil {
		stop()
		if err := process.TerminateProcessTree(cmd.Process.Pid, tree); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	wait := <-exited
	err = wait.err
	exitCode := wait.code
	if protocolErr != nil {
		return 1, protocolErr
	}
	if err != nil && !(finished && finalStatus == "failed" && exitCode == 1) {
		return exitCode, err
	}
	if paused && processCtx.Err() == nil {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return 1, err
		}
		return 0, nil
	}
	if !finished {
		return 1, errors.New("任务进程未确认结果，不能报告成功")
	}
	if processCtx.Err() != nil {
		return 1, processCtx.Err()
	}
	h.mu.Lock()
	if h.state == "cancelling" {
		h.mu.Unlock()
		return 1, context.Canceled
	}
	h.state = "confirmed"
	h.mu.Unlock()
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return 1, err
	}
	if callbacks.OnFinished != nil {
		callbacks.OnFinished(finalStatus, finalResult, finalArtifactID, finalCode, finalMessage)
	}
	return 0, nil
}
