// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/config"
	securitymode "github.com/u-ai/backend/internal/security"
)

const localCredentialMinLength = 32

// prepareSecurityMaterial resolves the device-local credential path before
// SecurityConfig validation. Local single-user runtimes are allowed to create
// their own persistent credential because they are loopback-only; network
// runtimes remain fail-closed and never auto-generate externally meaningful
// secrets.
func prepareSecurityMaterial(dataDir string) (string, error) {
	dataDir = filepath.Clean(strings.TrimSpace(dataDir))
	if dataDir == "" || dataDir == "." {
		return "", errors.New("security bootstrap: data directory is required")
	}

	tokenPath, err := resolveLocalCredentialPath(dataDir, config.AppCfg.Security.LocalTokenFile)
	if err != nil {
		return "", err
	}
	config.AppCfg.Security.LocalTokenFile = tokenPath

	mode := securitymode.SecurityMode(strings.TrimSpace(config.AppCfg.Security.Mode))
	configuredToken := strings.TrimSpace(config.AppCfg.Security.LocalToken)

	switch mode {
	case securitymode.SecurityModeLocalSingle:
		token, tokenErr := ensureLocalCredential(tokenPath, configuredToken)
		if tokenErr != nil {
			return "", tokenErr
		}
		config.AppCfg.Security.LocalToken = token

		return token, nil

	case securitymode.SecurityModeMaintenance:
		if configuredToken != "" {
			if len(configuredToken) < localCredentialMinLength {
				return "", errors.New("security bootstrap: configured local token is too short")
			}
			return configuredToken, nil
		}
		data, readErr := os.ReadFile(tokenPath)
		if readErr == nil {
			resolved := strings.TrimSpace(string(data))
			if len(resolved) >= localCredentialMinLength {
				config.AppCfg.Security.LocalToken = resolved
				return resolved, nil
			}
		}
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return "", fmt.Errorf("security bootstrap: read local token: %w", readErr)
		}
		return "", nil

	case securitymode.SecurityModeNetwork:
		// Network security is explicitly provisioned. Never create or derive a
		// credential here, otherwise a deployment could silently become reachable
		// with a secret that the operator never received.
		return configuredToken, nil

	default:
		return configuredToken, nil
	}
}

func resolveLocalCredentialPath(dataDir, configuredPath string) (string, error) {
	base, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("security bootstrap: resolve data directory: %w", err)
	}
	base = filepath.Clean(base)

	configuredPath = strings.TrimSpace(configuredPath)
	var candidate string
	if configuredPath == "" {
		candidate = filepath.Join(base, "security", "local-token")
	} else if filepath.IsAbs(configuredPath) {
		candidate = filepath.Clean(configuredPath)
	} else {
		candidate = filepath.Join(base, configuredPath)
	}

	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("security bootstrap: resolve local token path: %w", err)
	}
	candidateAbs = filepath.Clean(candidateAbs)

	rel, err := filepath.Rel(base, candidateAbs)
	if err != nil {
		return "", fmt.Errorf("security bootstrap: validate local token path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("security bootstrap: local token path must stay within data directory: %s", candidateAbs)
	}
	return candidateAbs, nil
}

func ensureLocalCredential(tokenPath, configuredToken string) (string, error) {
	configuredToken = strings.TrimSpace(configuredToken)
	if configuredToken != "" {
		if len(configuredToken) < localCredentialMinLength {
			return "", errors.New("security bootstrap: configured local token is too short")
		}
		if err := atomicWriteCredentialIfDifferent(tokenPath, configuredToken); err != nil {
			return "", err
		}
		return configuredToken, nil
	}

	if data, err := os.ReadFile(tokenPath); err == nil {
		existing := strings.TrimSpace(string(data))
		if len(existing) >= localCredentialMinLength {
			return existing, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("security bootstrap: read local token: %w", err)
	}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("security bootstrap: generate local token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(randomBytes)
	if err := atomicWriteCredentialIfDifferent(tokenPath, token); err != nil {
		return "", err
	}
	return token, nil
}

func atomicWriteCredentialIfDifferent(path, value string) error {
	if existing, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(existing)) == value {
		_ = os.Chmod(path, 0o600)
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("security bootstrap: create credential directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("security bootstrap: secure credential directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".local-token-*")
	if err != nil {
		return fmt.Errorf("security bootstrap: create credential temp file: %w", err)
	}
	tmpPath := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("security bootstrap: chmod credential: %w", err)
	}
	if _, err := file.WriteString(value + "\n"); err != nil {
		return fmt.Errorf("security bootstrap: write credential: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("security bootstrap: sync credential: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("security bootstrap: close credential: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("security bootstrap: publish credential: %w", err)
	}
	cleanup = false
	return nil
}

func configuredLocalCredentialPath(dataDir string) string {
	path, err := resolveLocalCredentialPath(dataDir, config.AppCfg.Security.LocalTokenFile)
	if err != nil {
		return filepath.Join(dataDir, "security", "local-token")
	}
	return path
}
