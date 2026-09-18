package kernel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ResourceLinkScopePackage = "package"
	ResourceLinkScopeData    = "data"
)

type ResourceLinkTarget struct {
	ExtensionID string `json:"extensionId"`
	Scope       string `json:"scope"`
	Path        string `json:"path"`
}

type ResourceLinkManager struct {
	root string
	key  []byte
}

func NewResourceLinkManager(root string) (*ResourceLinkManager, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("extension root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(root, "resource-links.key")
	key, err := os.ReadFile(keyPath)
	if err == nil && len(key) >= 32 {
		return &ResourceLinkManager{root: root, key: append([]byte(nil), key...)}, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, err
	}
	return &ResourceLinkManager{root: root, key: key}, nil
}

func (m *ResourceLinkManager) DataPath(extensionID, relativePath string, create bool) (string, error) {
	if m == nil {
		return "", errors.New("resource link manager is unavailable")
	}
	return m.resolvePath(extensionID, ResourceLinkScopeData, relativePath, create)
}

func (m *ResourceLinkManager) Sign(extensionID, scope, relativePath string) (string, error) {
	fullPath, err := m.resolvePath(extensionID, scope, relativePath, false)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(fullPath); err != nil || info.IsDir() {
		if err != nil {
			return "", err
		}
		return "", errors.New("resource path is a directory")
	}
	target := ResourceLinkTarget{
		ExtensionID: strings.TrimSpace(extensionID),
		Scope:       strings.ToLower(strings.TrimSpace(scope)),
		Path:        filepath.ToSlash(filepath.Clean(relativePath)),
	}
	payload, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "/api/extension/resources/" + encoded + "." + signature, nil
}

func (m *ResourceLinkManager) Resolve(token string) (ResourceLinkTarget, string, error) {
	if m == nil {
		return ResourceLinkTarget{}, "", errors.New("resource link manager is unavailable")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return ResourceLinkTarget{}, "", errors.New("invalid resource token")
	}
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return ResourceLinkTarget{}, "", errors.New("invalid resource signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ResourceLinkTarget{}, "", err
	}
	var target ResourceLinkTarget
	if err := json.Unmarshal(payload, &target); err != nil {
		return ResourceLinkTarget{}, "", err
	}
	fullPath, err := m.resolvePath(target.ExtensionID, target.Scope, target.Path, false)
	if err != nil {
		return ResourceLinkTarget{}, "", err
	}
	return target, fullPath, nil
}

func (m *ResourceLinkManager) ResolveResourceLink(token string) (string, string, string, error) {
	target, fullPath, err := m.Resolve(token)
	if err != nil {
		return "", "", "", err
	}
	return target.ExtensionID, target.Scope, fullPath, nil
}

func (m *ResourceLinkManager) resolvePath(extensionID, scope, relativePath string, create bool) (string, error) {
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return "", errors.New("extension id is required")
	}
	cleanPath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relativePath)))
	if cleanPath == "." || cleanPath == "" || filepath.IsAbs(cleanPath) {
		return "", errors.New("relative resource path is required")
	}
	var root string
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case ResourceLinkScopeData:
		root = filepath.Join(m.root, "data", safeDirectoryName(extensionID))
	case ResourceLinkScopePackage:
		root = resolveExtensionBundlePath(m.root, extensionID)
	default:
		return "", fmt.Errorf("unsupported resource scope %q", scope)
	}
	if root == "" {
		return "", errors.New("extension resource root not found")
	}
	target := filepath.Join(root, cleanPath)
	if !isPathSafe(root, target) {
		return "", errors.New("resource path escapes extension root")
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
	}
	return target, nil
}

func (m *ResourceLinkManager) KeyFingerprint() string {
	if m == nil {
		return ""
	}
	sum := sha256.Sum256(m.key)
	return hex.EncodeToString(sum[:8])
}
