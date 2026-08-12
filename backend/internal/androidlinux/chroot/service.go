//go:build linux && !android

package chroot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	policy     Policy
	knownRoots map[string]*RootFSEntry
}

func NewService(policy Policy) *Service {
	return &Service{
		policy:     policy,
		knownRoots: make(map[string]*RootFSEntry),
	}
}

func (s *Service) Status(ctx context.Context, workspace string) ChrootStatus {
	envs := make([]string, 0)
	for name := range s.knownRoots {
		envs = append(envs, name)
	}

	execBackends := []string{"proot"}
	if s.policy.AllowProotExec {
		execBackends = append(execBackends, "proot_exec")
	}

	return ChrootStatus{
		Enabled:          s.policy.Enabled,
		DefaultRootFSP:   workspace,
		KnownFSPs:        envs,
		MaxFSBytes:       s.policy.MaxFSBytes,
		MaxEnvironments:  s.policy.MaxEnvironments,
		AvailableEnvironments: envs,
		ExecBackends:     execBackends,
	}
}

func (s *Service) Inspect(ctx context.Context, req ChrootInspectRequest) (*ChrootInspectResult, error) {
	if req.RootFSPath == "" {
		return nil, ErrInvalidRequest("rootfsPath is required")
	}

	result := &ChrootInspectResult{
		RootFSPath: req.RootFSPath,
		Exists:     false,
		Valid:      false,
	}

	info, err := os.Stat(req.RootFSPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Error = fmt.Sprintf("rootfs path does not exist: %s", req.RootFSPath)
			return result, nil
		}
		return nil, ErrInternal(fmt.Sprintf("stat rootfs: %v", err))
	}

	if !info.IsDir() {
		result.Error = fmt.Sprintf("rootfs path is not a directory: %s", req.RootFSPath)
		return result, nil
	}

	result.Exists = true
	result.TotalBytes = 0
	result.FileCount = 0

	err = filepath.Walk(req.RootFSPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			result.TotalBytes += info.Size()
			result.FileCount++
		}
		if info.Name() == "sh" && strings.Contains(path, "/bin/") {
			result.HasBinSH = true
		}
		if info.Name() == "bash" && strings.Contains(path, "/bin/") {
			result.HasBinBash = true
		}
		return nil
	})

	if err != nil {
		result.Error = fmt.Sprintf("walk failed: %v", err)
		return result, nil
	}

	result.Valid = true

	if !s.verifyBinariesPath(req.RootFSPath) {
		result.Error = "rootfs missing dynamic library path"
	}

	return result, nil
}

func (s *Service) Exec(ctx context.Context, req ChrootExecRequest) (*ChrootExecResult, error) {
	if !s.policy.Enabled {
		return nil, ErrExecDenied("chroot is disabled")
	}
	if req.RootFSPath == "" {
		return nil, ErrInvalidRequest("rootfsPath is required")
	}
	if req.Command == "" {
		return nil, ErrInvalidRequest("command is required")
	}

	info, err := os.Stat(req.RootFSPath)
	if err != nil || !info.IsDir() {
		return nil, ErrRootFSNotFound(req.RootFSPath)
	}

	if s.policy.RequireBinSH {
		binSH := filepath.Join(req.RootFSPath, "bin", "sh")
		if _, err := os.Stat(binSH); err != nil {
			return nil, ErrRootFSInvalid(fmt.Sprintf("rootfs missing /bin/sh: %v", err))
		}
	}

	maxOutput := s.policy.MaxOutputBytes
	if req.MaxOutputBytes > 0 && req.MaxOutputBytes < maxOutput {
		maxOutput = req.MaxOutputBytes
	}

	timeout := time.Duration(s.policy.DefaultTimeoutSec) * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	if timeout > time.Duration(s.policy.MaxTimeoutSec)*time.Second {
		timeout = time.Duration(s.policy.MaxTimeoutSec) * time.Second
	}

	execResult, err := s.execProot(ctx, req, timeout, maxOutput)
	if err != nil {
		return nil, err
	}

	execResult.RootFSPath = req.RootFSPath
	execResult.Environment = s.policy.DefaultExecBackend

	return execResult, nil
}

func (s *Service) execProot(ctx context.Context, req ChrootExecRequest, timeout time.Duration, maxOutput int64) (*ChrootExecResult, error) {
	rootFS := req.RootFSPath

	args := []string{"-r", rootFS, "-w", "/"}
	if req.WorkingDir != "" {
		args = append(args, "-w", req.WorkingDir)
	}

	args = append(args, "--bind", "/proc", "/proc")
	args = append(args, "--bind", "/dev", "/dev")
	args = append(args, "--bind", "/sys", "/sys")
	args = append(args, "--bind", "/tmp", "/tmp")

	filesystems := []string{"/data", "/sdcard", "/system"}
	for _, fs := range filesystems {
		if _, err := os.Stat(fs); err == nil {
			args = append(args, "--bind", fs, fs)
		}
	}

	args = append(args, "sh", "-c", req.Command)

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "proot", args...)
	cmd.Dir = rootFS

	if req.Environment != nil {
		env := os.Environ()
		for k, v := range req.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout, stderr []byte
	var exitCode int

	startTime := time.Now()
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, ErrCommandFailed(fmt.Sprintf("stdout pipe: %v", err))
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, ErrCommandFailed(fmt.Sprintf("stderr pipe: %v", err))
	}

	if err := cmd.Start(); err != nil {
		return nil, ErrCommandFailed(fmt.Sprintf("start: %v", err))
	}

	stdout = readLimitedBytes(stdoutPipe, int(maxOutput)/2)
	stderr = readLimitedBytes(stderrPipe, int(maxOutput)/2)

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if cmdCtx.Err() == context.DeadlineExceeded {
			exitCode = 124
		}
	}

	durationMs := time.Since(startTime).Milliseconds()

	result := &ChrootExecResult{
		ExitCode:    exitCode,
		Stdout:      string(stdout),
		Stderr:      string(stderr),
		StdoutBytes: int64(len(stdout)),
		StderrBytes: int64(len(stderr)),
		DurationMs:  durationMs,
	}

	if cmdCtx.Err() == context.DeadlineExceeded {
		result.ExitCode = 124
		result.Stderr += "\n[TIMEOUT]"
		result.StderrTruncated = true
	}

	return result, nil
}

func readLimitedBytes(r interface{ Read([]byte) (int, error) }, maxBytes int) []byte {
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024
	}

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	var totalRead int

	for totalRead < maxBytes {
		n, err := r.Read(tmp)
		if n > 0 {
			remaining := maxBytes - totalRead
			if n > remaining {
				n = remaining
			}
			buf = append(buf, tmp[:n]...)
			totalRead += n
		}
		if err != nil {
			break
		}
	}

	return buf
}

func (s *Service) verifyBinariesPath(rootfs string) bool {
	binDirs := []string{"bin", "usr/bin", "usr/local/bin"}
	for _, dir := range binDirs {
		path := filepath.Join(rootfs, dir)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) Close() {
}
