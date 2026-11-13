// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

// BuildSurrealProcessSpec prepares the SurrealDB ProcessSpec but does NOT start SurrealDB.
// The caller (typically a runtimehost.ProcessSupervisor-based provider) handles actual execution.
func BuildSurrealProcessSpec(instanceID string) (runtimehost.ProcessSpec, error) {
	if instanceID == "" {
		instanceID = "surrealdb-unsigned"
	}
	cfg := config.AppCfg.Providers.GraphStore.SurrealDB
	bindHost, err := normalizeLoopbackHost(cfg.Host)
	if err != nil {
		return runtimehost.ProcessSpec{}, err
	}
	if err := validateSurrealCredentials(cfg.Username, cfg.Password); err != nil {
		return runtimehost.ProcessSpec{}, err
	}
	workDir := util.RuntimeRoot()
	surrealDir := filepath.Join(workDir, "surrealdb")

	p := platform.Get()
	binaryName := "surreal" + p.ExecutableSuffix()
	surrealPath := filepath.Join(surrealDir, binaryName)

	if err := ensureSurrealExecutable(surrealDir, binaryName); err != nil {
		return runtimehost.ProcessSpec{}, err
	}

	// Ensure data directory exists for file-based storage
	dataPath := filepath.Join(surrealDir, "data")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create surrealdb data dir: %w", err)
	}

	if !filepath.IsAbs(surrealDir) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("working_dir must be absolute: %s", surrealDir)
	}
	if !filepath.IsAbs(surrealPath) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("executable must be absolute: %s", surrealPath)
	}

	bindAddress := net.JoinHostPort(bindHost, fmt.Sprintf("%d", cfg.Port))
	healthURL := "http://" + bindAddress + "/health"

	args := []string{
		"start",
		"surrealkv:" + dataPath,
		"--bind", bindAddress,
	}

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDSurrealDB,
		Executable: surrealPath,
		Args:       args,
		WorkingDir: surrealDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
			Values: map[string]string{
				"AMITIA_PROCESS_ID":          "amitia.surrealdb",
				"AMITIA_RUNTIME_INSTANCE_ID": instanceID,
				"SURREAL_USER":               cfg.Username,
				"SURREAL_PASS":               cfg.Password,
			},
			SensitiveKeys: []string{"SURREAL_PASS"},
		},
		Ports: []runtimehost.LoopbackPortClaim{
			{Host: bindHost, Port: cfg.Port, Protocol: "tcp"},
		},
		StartupTimeout:  runtimehost.DefaultStartupTimeout,
		StopGracePeriod: runtimehost.DefaultStopGracePeriod,
		HealthProbe:     runtimehost.NewHTTPHealthProbe(healthURL, 5*time.Second),
		HealthInterval:  runtimehost.DefaultHealthInterval,
		RestartPolicy: runtimehost.RestartPolicy{
			Mode:        runtimehost.RestartOnFailure,
			MaxRestarts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
			ResetAfter:  5 * time.Minute,
		},
	}, nil
}

func ensureSurrealExecutable(dir, binaryName string) error {
	path := filepath.Join(dir, binaryName)
	var failures []error
	if _, err := os.Stat(path); err == nil {
		if verifyErr := verifySurrealExecutable(path); verifyErr == nil {
			return nil
		} else {
			failures = append(failures, fmt.Errorf("existing executable %s: %w", path, verifyErr))
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat surrealdb executable %s: %w", path, err)
	}
	for _, zip := range []string{"surreal.zip", "surreal.exe.zip"} {
		zipPath := filepath.Join(dir, zip)
		if _, err := os.Stat(zipPath); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", zipPath, err))
			continue
		}
		if err := util.UnzipFile(zipPath, dir); err != nil {
			failures = append(failures, fmt.Errorf("unzip %s: %w", zipPath, err))
			continue
		}
		if err := verifySurrealExecutable(path); err != nil {
			failures = append(failures, fmt.Errorf("extract %s: %w", zipPath, err))
			if detail := describeSurrealArchiveCandidates(dir, binaryName); detail != "" {
				failures = append(failures, fmt.Errorf("extract %s: %s", zipPath, detail))
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("surrealdb executable %s is unavailable for %s/%s: %w", path, runtime.GOOS, runtime.GOARCH, errors.Join(failures...))
}

func verifySurrealExecutable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	want := executableFormatFor(runtime.GOOS)
	got := detectExecutableFormat(header)
	if want != "" && got != want {
		return fmt.Errorf("%s carries %s executable header, want %s for %s/%s", path, got, want, runtime.GOOS, runtime.GOARCH)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		if err := os.Chmod(path, info.Mode().Perm()|0o755); err != nil {
			return fmt.Errorf("%s lacks execute permission (%s): %w", path, info.Mode().Perm(), err)
		}
	}
	return nil
}

func describeSurrealArchiveCandidates(dir, binaryName string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Sprintf("list %s: %v", dir, err)
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".zip") || !strings.HasPrefix(name, "surreal") {
			continue
		}
		if name == binaryName {
			continue
		}
		found = append(found, fmt.Sprintf("%s carries %s executable header", name, executableFormatAt(filepath.Join(dir, name))))
	}
	if len(found) == 0 {
		return ""
	}
	return fmt.Sprintf("archive provided [%s], want %s with %s header for %s/%s", strings.Join(found, "; "), binaryName, executableFormatFor(runtime.GOOS), runtime.GOOS, runtime.GOARCH)
}

func executableFormatAt(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("unreadable(%v)", err)
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Sprintf("unreadable(%v)", err)
	}
	return detectExecutableFormat(header)
}

func executableFormatFor(goos string) string {
	switch goos {
	case "windows":
		return "pe"
	case "darwin":
		return "mach-o"
	case "linux", "android":
		return "elf"
	default:
		return ""
	}
}

func detectExecutableFormat(header []byte) string {
	if len(header) < 4 {
		return "truncated"
	}
	if header[0] == 'M' && header[1] == 'Z' {
		return "pe"
	}
	if header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F' {
		return "elf"
	}
	be := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	switch be {
	case 0xfeedface, 0xfeedfacf, 0xcefaedfe, 0xcffaedfe, 0xcafebabe, 0xbebafeca:
		return "mach-o"
	}
	return fmt.Sprintf("unknown(0x%08x)", be)
}

func normalizeLoopbackHost(raw string) (string, error) {
	host := strings.TrimSpace(strings.Trim(raw, "[]"))
	if host == "" || strings.EqualFold(host, "localhost") {
		return "127.0.0.1", nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("surrealdb host must be loopback, got %q", raw)
	}
	return ip.String(), nil
}

func validateSurrealCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("surrealdb username must not be empty")
	}
	trimmed := strings.TrimSpace(password)
	if len(trimmed) == 0 {
		return fmt.Errorf("surrealdb password must not be empty")
	}
	return nil
}
