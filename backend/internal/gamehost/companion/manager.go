package companion

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/integration"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type InstallationRecord struct {
	ExtensionID   string    `json:"extensionId"`
	ArtifactID    string    `json:"artifactId"`
	GameRoot      string    `json:"gameRoot"`
	TargetPath    string    `json:"targetPath"`
	InstalledHash string    `json:"installedHash"`
	InstalledAt   time.Time `json:"installedAt"`
}

type ArtifactStatus struct {
	Artifact      gameprotocol.GameCompanionArtifact `json:"artifact"`
	Installed     bool                               `json:"installed"`
	Healthy       bool                               `json:"healthy"`
	TargetPath    string                             `json:"targetPath,omitempty"`
	InstalledHash string                             `json:"installedHash,omitempty"`
}

type Manager struct {
	mu          sync.Mutex
	source      integration.KernelContributionSource
	generations integration.InstalledGenerationResolver
	statePath   string
	records     map[string]InstallationRecord
}

func NewManager(dataRoot string, source integration.KernelContributionSource, generations integration.InstalledGenerationResolver) (*Manager, error) {
	if strings.TrimSpace(dataRoot) == "" || source == nil || generations == nil {
		return nil, fmt.Errorf("companion: data root, kernel source and generation resolver are required")
	}
	stateDir := filepath.Join(dataRoot, "gamehost", "companions")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	m := &Manager{source: source, generations: generations, statePath: filepath.Join(stateDir, "installations.json"), records: map[string]InstallationRecord{}}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func recordKey(extensionID, artifactID, gameRoot string) string {
	return extensionID + "\x00" + artifactID + "\x00" + filepath.Clean(gameRoot)
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var records []InstallationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("companion: decode state: %w", err)
	}
	for _, r := range records {
		m.records[recordKey(r.ExtensionID, r.ArtifactID, r.GameRoot)] = r
	}
	return nil
}

func (m *Manager) saveLocked() error {
	records := make([]InstallationRecord, 0, len(m.records))
	for _, r := range m.records {
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool {
		return recordKey(records[i].ExtensionID, records[i].ArtifactID, records[i].GameRoot) < recordKey(records[j].ExtensionID, records[j].ArtifactID, records[j].GameRoot)
	})
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return replaceStateFile(tmp, m.statePath)
}

// replaceStateFile replaces the persisted companion state without relying on
// POSIX rename-over-existing semantics. Windows does not allow os.Rename to
// replace an existing file, so keep a short-lived backup and restore it if the
// replacement fails.
func replaceStateFile(tmp, target string) error {
	if err := os.Rename(tmp, target); err == nil {
		return nil
	} else if runtimeGOOS() != "windows" {
		return err
	}

	backup := target + ".bak"
	_ = os.Remove(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		hadTarget = true
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("companion: backup state before replacement: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("companion: replace state: %w", err)
	}
	if hadTarget {
		_ = os.Remove(backup)
	}
	return nil
}

func (m *Manager) List(ctx context.Context, extensionID, gameRoot, gameVersion string) ([]ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, _, err := m.resolveArtifacts(ctx, extensionID, gameVersion)
	if err != nil {
		return nil, err
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return nil, err
	}
	out := make([]ArtifactStatus, 0, len(artifacts))
	for _, a := range artifacts {
		r, ok := m.records[recordKey(extensionID, a.ID, root)]
		status := ArtifactStatus{Artifact: a, Installed: ok}
		if ok {
			status.TargetPath, status.InstalledHash = r.TargetPath, r.InstalledHash
			h, hashErr := hashPath(r.TargetPath)
			status.Healthy = hashErr == nil && strings.EqualFold(h, r.InstalledHash)
		}
		out = append(out, status)
	}
	return out, nil
}

func (m *Manager) PrepareRequired(ctx context.Context, extensionID, gameRoot, gameVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	required, generation, err := m.resolveRequiredArtifacts(ctx, extensionID, gameVersion)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		return nil
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return fmt.Errorf("companion: required artifacts need a valid gameRoot: %w", err)
	}
	for _, artifact := range required {
		if _, err := m.installLocked(extensionID, root, generation.Path, artifact); err != nil {
			return err
		}
	}
	return m.saveLocked()
}

func (m *Manager) InstallRequired(ctx context.Context, extensionID, gameRoot, gameVersion string) ([]ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, generation, err := m.resolveArtifacts(ctx, extensionID, gameVersion)
	if err != nil {
		return nil, err
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return nil, err
	}
	for _, a := range artifacts {
		if a.Required {
			if _, err := m.installLocked(extensionID, root, generation.Path, a); err != nil {
				return nil, err
			}
		}
	}
	if err := m.saveLocked(); err != nil {
		return nil, err
	}
	return m.statusesLocked(extensionID, root, artifacts), nil
}

func (m *Manager) Install(ctx context.Context, extensionID, artifactID, gameRoot, gameVersion string) (ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, generation, err := m.resolveArtifacts(ctx, extensionID, gameVersion)
	if err != nil {
		return ArtifactStatus{}, err
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return ArtifactStatus{}, err
	}
	a, ok := findArtifact(artifacts, artifactID)
	if !ok {
		return ArtifactStatus{}, fmt.Errorf("companion: artifact %q not found", artifactID)
	}
	r, err := m.installLocked(extensionID, root, generation.Path, a)
	if err != nil {
		return ArtifactStatus{}, err
	}
	if err := m.saveLocked(); err != nil {
		return ArtifactStatus{}, err
	}
	return ArtifactStatus{Artifact: a, Installed: true, Healthy: true, TargetPath: r.TargetPath, InstalledHash: r.InstalledHash}, nil
}

func (m *Manager) Verify(ctx context.Context, extensionID, artifactID, gameRoot, gameVersion string) (ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, _, err := m.resolveArtifacts(ctx, extensionID, gameVersion)
	if err != nil {
		return ArtifactStatus{}, err
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return ArtifactStatus{}, err
	}
	a, ok := findArtifact(artifacts, artifactID)
	if !ok {
		return ArtifactStatus{}, fmt.Errorf("companion: artifact %q not found", artifactID)
	}
	r, ok := m.records[recordKey(extensionID, artifactID, root)]
	if !ok {
		return ArtifactStatus{Artifact: a}, nil
	}
	h, hashErr := hashPath(r.TargetPath)
	return ArtifactStatus{Artifact: a, Installed: true, Healthy: hashErr == nil && strings.EqualFold(h, r.InstalledHash), TargetPath: r.TargetPath, InstalledHash: r.InstalledHash}, nil
}

func (m *Manager) Remove(ctx context.Context, extensionID, artifactID, gameRoot string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return err
	}
	key := recordKey(extensionID, artifactID, root)
	r, ok := m.records[key]
	if !ok {
		return nil
	}
	if !isWithin(root, r.TargetPath) {
		return fmt.Errorf("companion: managed target escaped game root")
	}
	h, err := hashPath(r.TargetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !strings.EqualFold(h, r.InstalledHash) {
		return fmt.Errorf("companion: refusing to remove modified artifact %s", r.TargetPath)
	}
	if err == nil {
		if err := os.RemoveAll(r.TargetPath); err != nil {
			return err
		}
	}
	delete(m.records, key)
	return m.saveLocked()
}

func (m *Manager) installLocked(extensionID, gameRoot, generationRoot string, artifact gameprotocol.GameCompanionArtifact) (InstallationRecord, error) {
	if strings.TrimSpace(artifact.EffectiveTarget()) == "" {
		return InstallationRecord{}, fmt.Errorf("companion: artifact %s requires installTarget", artifact.ID)
	}
	source, err := resolveContained(generationRoot, artifact.Source)
	if err != nil {
		return InstallationRecord{}, err
	}
	if artifact.SHA256 != "" {
		h, err := hashPath(source)
		if err != nil {
			return InstallationRecord{}, err
		}
		if !strings.EqualFold(h, normalizeHash(artifact.SHA256)) {
			return InstallationRecord{}, fmt.Errorf("companion: source hash mismatch for %s", artifact.ID)
		}
	}
	target, err := resolveTarget(gameRoot, artifact.EffectiveTarget())
	if err != nil {
		return InstallationRecord{}, err
	}
	key := recordKey(extensionID, artifact.ID, gameRoot)
	if existing, ok := m.records[key]; ok {
		h, hashErr := hashPath(existing.TargetPath)
		if hashErr == nil && strings.EqualFold(h, existing.InstalledHash) {
			_ = os.RemoveAll(existing.TargetPath)
		} else if hashErr == nil {
			return InstallationRecord{}, fmt.Errorf("companion: managed target was modified; refusing overwrite")
		}
	} else if _, err := os.Lstat(target); err == nil {
		return InstallationRecord{}, fmt.Errorf("companion: target already exists and is not managed: %s", target)
	} else if !os.IsNotExist(err) {
		return InstallationRecord{}, err
	}
	if err := installArtifact(source, target, artifact.Type); err != nil {
		return InstallationRecord{}, err
	}
	h, err := hashPath(target)
	if err != nil {
		_ = os.RemoveAll(target)
		return InstallationRecord{}, err
	}
	r := InstallationRecord{ExtensionID: extensionID, ArtifactID: artifact.ID, GameRoot: gameRoot, TargetPath: target, InstalledHash: h, InstalledAt: time.Now().UTC()}
	m.records[key] = r
	return r, nil
}

func (m *Manager) statusesLocked(extensionID, gameRoot string, artifacts []gameprotocol.GameCompanionArtifact) []ArtifactStatus {
	out := make([]ArtifactStatus, 0, len(artifacts))
	for _, a := range artifacts {
		r, ok := m.records[recordKey(extensionID, a.ID, gameRoot)]
		st := ArtifactStatus{Artifact: a, Installed: ok}
		if ok {
			st.TargetPath = r.TargetPath
			st.InstalledHash = r.InstalledHash
			h, e := hashPath(r.TargetPath)
			st.Healthy = e == nil && strings.EqualFold(h, r.InstalledHash)
		}
		out = append(out, st)
	}
	return out
}

func (m *Manager) resolveRequiredArtifacts(ctx context.Context, extensionID, gameVersion string) ([]gameprotocol.GameCompanionArtifact, integration.InstalledGeneration, error) {
	plugins, err := m.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	foundExtension := false
	requiredDeclared := false
	var required []gameprotocol.GameCompanionArtifact
	for _, kp := range plugins {
		if string(kp.Extension.ID) != extensionID {
			continue
		}
		foundExtension = true
		spec, err := gameprotocol.ParseGamePluginSpec(kp.Contribution.Definition)
		if err != nil {
			return nil, integration.InstalledGeneration{}, err
		}
		for _, artifact := range spec.EffectiveArtifacts() {
			if !artifact.Required {
				continue
			}
			requiredDeclared = true
			if platformMatches(artifact.Platforms) && versionMatches(artifact.EffectiveCompatibilityVersions(), gameVersion) {
				required = append(required, artifact)
			}
		}
	}
	if !foundExtension {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("companion: enabled game extension %s not found", extensionID)
	}
	if !requiredDeclared {
		return nil, integration.InstalledGeneration{}, nil
	}
	if len(required) == 0 {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("companion: no required artifact is compatible with platform %s and game version %q", currentPlatform(), strings.TrimSpace(gameVersion))
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	return required, generation, nil
}

func (m *Manager) resolveArtifactsOptional(ctx context.Context, extensionID, gameVersion string) ([]gameprotocol.GameCompanionArtifact, integration.InstalledGeneration, error) {
	plugins, err := m.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	var artifacts []gameprotocol.GameCompanionArtifact
	foundExtension := false
	for _, kp := range plugins {
		if string(kp.Extension.ID) != extensionID {
			continue
		}
		foundExtension = true
		spec, err := gameprotocol.ParseGamePluginSpec(kp.Contribution.Definition)
		if err != nil {
			return nil, integration.InstalledGeneration{}, err
		}
		for _, artifact := range spec.EffectiveArtifacts() {
			if platformMatches(artifact.Platforms) && versionMatches(artifact.EffectiveCompatibilityVersions(), gameVersion) {
				artifacts = append(artifacts, artifact)
			}
		}
	}
	if !foundExtension {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("companion: enabled game extension %s not found", extensionID)
	}
	if len(artifacts) == 0 {
		return nil, integration.InstalledGeneration{}, nil
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	return artifacts, generation, nil
}

func (m *Manager) resolveArtifacts(ctx context.Context, extensionID, gameVersion string) ([]gameprotocol.GameCompanionArtifact, integration.InstalledGeneration, error) {
	artifacts, generation, err := m.resolveArtifactsOptional(ctx, extensionID, gameVersion)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	if len(artifacts) == 0 {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("companion: no compatible artifacts for extension %s", extensionID)
	}
	return artifacts, generation, nil
}

func canonicalGameRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("companion: gameRoot is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("companion: gameRoot unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("companion: gameRoot must be a directory")
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(real), nil
}
func resolveContained(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("companion: source must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("companion: invalid source path")
	}
	p := filepath.Join(root, clean)
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !isWithin(base, real) {
		return "", fmt.Errorf("companion: source escaped generation")
	}
	return real, nil
}
func resolveTarget(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("companion: installTarget must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("companion: invalid installTarget")
	}
	target := filepath.Join(root, clean)
	if !isWithin(root, target) {
		return "", fmt.Errorf("companion: target escaped game root")
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !isWithin(root, realParent) {
		return "", fmt.Errorf("companion: target parent escaped game root")
	}
	return target, nil
}
func isWithin(root, path string) bool {
	r, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
func findArtifact(items []gameprotocol.GameCompanionArtifact, id string) (gameprotocol.GameCompanionArtifact, bool) {
	for _, a := range items {
		if a.ID == id {
			return a, true
		}
	}
	return gameprotocol.GameCompanionArtifact{}, false
}
func platformMatches(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	p := currentPlatform()
	for _, x := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(x))
		if normalized == "all" || normalized == p || (p == "darwin" && (normalized == "macos" || normalized == "osx")) {
			return true
		}
	}
	return false
}
func currentPlatform() string { return runtimeGOOS() }
func versionMatches(versions []string, version string) bool {
	if len(versions) == 0 || strings.TrimSpace(version) == "" {
		return true
	}
	for _, v := range versions {
		if v == "*" || strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(version)) {
			return true
		}
	}
	return false
}
func normalizeHash(v string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), "sha256:")
}

func installArtifact(source, target, kind string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("companion: symlink source rejected")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if info.IsDir() {
		return copyTreeAtomic(source, target)
	}
	if (kind == "archive" || kind == "zip") && strings.EqualFold(filepath.Ext(source), ".zip") {
		return extractZipAtomic(source, target)
	}
	return copyFileAtomic(source, target, info.Mode().Perm())
}
func copyFileAtomic(source, target string, mode os.FileMode) error {
	tmp := target + ".amitia-tmp"
	_ = os.RemoveAll(tmp)
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, target)
}
func copyTreeAtomic(source, target string) error {
	tmp := target + ".amitia-tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("companion: symlink rejected: %s", path)
		}
		rel, _ := filepath.Rel(source, path)
		if rel == "." {
			return nil
		}
		dst := filepath.Join(tmp, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		return copyFileAtomic(path, dst, info.Mode().Perm())
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, target)
}
func extractZipAtomic(source, target string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer r.Close()
	tmp := target + ".amitia-tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		clean := filepath.Clean(f.Name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("companion: unsafe zip entry %q", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("companion: zip symlink rejected")
		}
		dst := filepath.Join(tmp, clean)
		if !isWithin(tmp, dst) {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("companion: zip entry escaped target")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, f.Mode().Perm()); err != nil {
				_ = os.RemoveAll(tmp)
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
		src, err := f.Open()
		if err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, f.Mode().Perm())
		if err != nil {
			src.Close()
			_ = os.RemoveAll(tmp)
			return err
		}
		_, err = io.Copy(out, src)
		src.Close()
		out.Close()
		if err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
	}
	return os.Rename(tmp, target)
}
func hashPath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("companion: symlink rejected")
	}
	h := sha256.New()
	if !info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	var paths []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("companion: symlink rejected")
		}
		if !info.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(path, p)
		h.Write([]byte(filepath.ToSlash(rel)))
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
