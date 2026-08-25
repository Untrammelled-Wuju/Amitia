package artifact

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

const (
	// Protocol v1 artifact deployment is host-local and bounded. These limits
	// protect the host from archive bombs and accidental unbounded tree copies.
	maxArtifactSingleFileBytes int64  = 512 << 20 // 512 MiB
	maxArtifactTotalBytes      int64  = 1 << 30   // 1 GiB
	maxArtifactFiles                  = 10000
	maxArtifactDepth                  = 32
	maxZipCompressionRatio     uint64 = 200
)

type DeploymentRecord struct {
	ExtensionID   string    `json:"extensionId"`
	ArtifactID    string    `json:"artifactId"`
	TargetRoot    string    `json:"targetRoot"`
	TargetPath    string    `json:"targetPath"`
	InstalledHash string    `json:"installedHash"`
	InstalledAt   time.Time `json:"installedAt"`
}

type ArtifactStatus struct {
	Artifact      gameprotocol.PluginArtifact `json:"artifact"`
	Installed     bool                        `json:"installed"`
	Healthy       bool                        `json:"healthy"`
	TargetPath    string                      `json:"targetPath,omitempty"`
	InstalledHash string                      `json:"installedHash,omitempty"`
}

type ArtifactManager struct {
	mu              sync.Mutex
	source          integration.KernelContributionSource
	generations     integration.InstalledGenerationResolver
	statePath       string
	legacyStatePath string
	targetGrantPath string
	records         map[string]DeploymentRecord
	targetGrants    map[string]TargetRootGrant
}

func NewArtifactManager(dataRoot string, source integration.KernelContributionSource, generations integration.InstalledGenerationResolver) (*ArtifactManager, error) {
	if strings.TrimSpace(dataRoot) == "" || source == nil || generations == nil {
		return nil, fmt.Errorf("artifact: data root, kernel source and generation resolver are required")
	}
	stateDir := filepath.Join(dataRoot, "gamehost", "artifacts")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	m := &ArtifactManager{
		source:          source,
		generations:     generations,
		statePath:       filepath.Join(stateDir, "deployments.json"),
		legacyStatePath: filepath.Join(dataRoot, "gamehost", "companions", "installations.json"),
		targetGrantPath: filepath.Join(stateDir, "target-roots.json"),
		records:         map[string]DeploymentRecord{},
		targetGrants:    map[string]TargetRootGrant{},
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	if err := m.loadTargetGrants(); err != nil {
		return nil, err
	}
	return m, nil
}

func recordKey(extensionID, artifactID, targetRoot string) string {
	return extensionID + "\x00" + artifactID + "\x00" + filepath.Clean(targetRoot)
}

func (m *ArtifactManager) load() error {
	data, err := os.ReadFile(m.statePath)
	legacy := false
	if os.IsNotExist(err) && strings.TrimSpace(m.legacyStatePath) != "" {
		data, err = os.ReadFile(m.legacyStatePath)
		legacy = err == nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	type persistedRecord struct {
		DeploymentRecord
		LegacyGameRoot string `json:"gameRoot,omitempty"`
	}
	var persisted []persistedRecord
	if err := json.Unmarshal(data, &persisted); err != nil {
		return fmt.Errorf("artifact: decode state: %w", err)
	}
	for _, item := range persisted {
		r := item.DeploymentRecord
		if strings.TrimSpace(r.TargetRoot) == "" {
			r.TargetRoot = item.LegacyGameRoot
		}
		if strings.TrimSpace(r.TargetRoot) == "" {
			continue
		}
		m.records[recordKey(r.ExtensionID, r.ArtifactID, r.TargetRoot)] = r
	}
	if legacy {
		return m.saveLocked()
	}
	return nil
}

func (m *ArtifactManager) saveLocked() error {
	records := make([]DeploymentRecord, 0, len(m.records))
	for _, r := range m.records {
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool {
		return recordKey(records[i].ExtensionID, records[i].ArtifactID, records[i].TargetRoot) < recordKey(records[j].ExtensionID, records[j].ArtifactID, records[j].TargetRoot)
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

// replaceStateFile replaces persisted artifact deployment state without relying on
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
			return fmt.Errorf("artifact: backup state before replacement: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("artifact: replace state: %w", err)
	}
	if hadTarget {
		_ = os.Remove(backup)
	}
	return nil
}

func (m *ArtifactManager) List(ctx context.Context, extensionID, targetRoot, compatibilityVersion string) ([]ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, _, err := m.resolveArtifacts(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return nil, err
	}
	root, err := canonicalTargetRoot(targetRoot)
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

func (m *ArtifactManager) DeployRequired(ctx context.Context, extensionID, targetRoot, compatibilityVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	required, generation, err := m.resolveRequiredArtifacts(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		return nil
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return fmt.Errorf("artifact: required artifacts need a valid targetRoot: %w", err)
	}
	original := cloneDeploymentRecords(m.records)
	txs := make([]artifactReplacement, 0, len(required))
	for _, artifact := range required {
		_, tx, deployErr := m.deployLocked(extensionID, root, generation.Path, artifact)
		if deployErr != nil {
			return m.rollbackDeploymentBatch(txs, original, deployErr)
		}
		txs = append(txs, tx)
	}
	return m.persistDeploymentBatch(txs, original)
}

func (m *ArtifactManager) DeployRequiredArtifacts(ctx context.Context, extensionID, targetRoot, compatibilityVersion string) ([]ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, generation, err := m.resolveArtifacts(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return nil, err
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return nil, err
	}
	original := cloneDeploymentRecords(m.records)
	txs := make([]artifactReplacement, 0)
	for _, a := range artifacts {
		if !a.Required {
			continue
		}
		_, tx, deployErr := m.deployLocked(extensionID, root, generation.Path, a)
		if deployErr != nil {
			return nil, m.rollbackDeploymentBatch(txs, original, deployErr)
		}
		txs = append(txs, tx)
	}
	if err := m.persistDeploymentBatch(txs, original); err != nil {
		return nil, err
	}
	return m.statusesLocked(extensionID, root, artifacts), nil
}

func (m *ArtifactManager) Deploy(ctx context.Context, extensionID, artifactID, targetRoot, compatibilityVersion string) (ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, generation, err := m.resolveArtifacts(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return ArtifactStatus{}, err
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return ArtifactStatus{}, err
	}
	a, ok := findArtifact(artifacts, artifactID)
	if !ok {
		return ArtifactStatus{}, fmt.Errorf("artifact: artifact %q not found", artifactID)
	}
	original := cloneDeploymentRecords(m.records)
	r, tx, err := m.deployLocked(extensionID, root, generation.Path, a)
	if err != nil {
		return ArtifactStatus{}, err
	}
	if err := m.persistDeploymentBatch([]artifactReplacement{tx}, original); err != nil {
		return ArtifactStatus{}, err
	}
	return ArtifactStatus{Artifact: a, Installed: true, Healthy: true, TargetPath: r.TargetPath, InstalledHash: r.InstalledHash}, nil
}

func (m *ArtifactManager) Verify(ctx context.Context, extensionID, artifactID, targetRoot, compatibilityVersion string) (ArtifactStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts, _, err := m.resolveArtifacts(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return ArtifactStatus{}, err
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return ArtifactStatus{}, err
	}
	a, ok := findArtifact(artifacts, artifactID)
	if !ok {
		return ArtifactStatus{}, fmt.Errorf("artifact: artifact %q not found", artifactID)
	}
	r, ok := m.records[recordKey(extensionID, artifactID, root)]
	if !ok {
		return ArtifactStatus{Artifact: a}, nil
	}
	h, hashErr := hashPath(r.TargetPath)
	return ArtifactStatus{Artifact: a, Installed: true, Healthy: hashErr == nil && strings.EqualFold(h, r.InstalledHash), TargetPath: r.TargetPath, InstalledHash: r.InstalledHash}, nil
}

func (m *ArtifactManager) Remove(ctx context.Context, extensionID, artifactID, targetRoot string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return err
	}
	key := recordKey(extensionID, artifactID, root)
	r, ok := m.records[key]
	if !ok {
		return nil
	}
	if !isWithin(root, r.TargetPath) {
		return fmt.Errorf("artifact: managed target escaped target root")
	}
	h, err := hashPath(r.TargetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !strings.EqualFold(h, r.InstalledHash) {
		return fmt.Errorf("artifact: refusing to remove modified artifact %s", r.TargetPath)
	}

	quarantine := ""
	if err == nil {
		quarantine = r.TargetPath + fmt.Sprintf(".amitia-remove-%d", time.Now().UnixNano())
		_ = os.RemoveAll(quarantine)
		if err := os.Rename(r.TargetPath, quarantine); err != nil {
			return fmt.Errorf("artifact: quarantine deployment before removal: %w", err)
		}
	}
	delete(m.records, key)
	if err := m.saveLocked(); err != nil {
		m.records[key] = r
		_ = os.Remove(m.statePath + ".tmp")
		if quarantine != "" {
			if restoreErr := os.Rename(quarantine, r.TargetPath); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("artifact: restore deployment after state failure: %w", restoreErr))
			}
		}
		return err
	}
	if quarantine != "" {
		if err := os.RemoveAll(quarantine); err != nil {
			return fmt.Errorf("artifact: state committed but cleanup of removed deployment failed: %w", err)
		}
	}
	return nil
}

func (m *ArtifactManager) deployLocked(extensionID, targetRoot, generationRoot string, artifact gameprotocol.PluginArtifact) (DeploymentRecord, artifactReplacement, error) {
	if strings.TrimSpace(artifact.Target) == "" {
		return DeploymentRecord{}, artifactReplacement{}, fmt.Errorf("artifact: artifact %s requires target", artifact.ID)
	}
	source, err := resolveContained(generationRoot, artifact.Source)
	if err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}
	if err := validateArtifactSource(source, artifact.Type); err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}
	if artifact.SHA256 != "" {
		h, err := hashPath(source)
		if err != nil {
			return DeploymentRecord{}, artifactReplacement{}, err
		}
		if !strings.EqualFold(h, normalizeHash(artifact.SHA256)) {
			return DeploymentRecord{}, artifactReplacement{}, fmt.Errorf("artifact: source hash mismatch for %s", artifact.ID)
		}
	}
	target, err := resolveTarget(targetRoot, artifact.Target)
	if err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}

	key := recordKey(extensionID, artifact.ID, targetRoot)
	existing, managed := m.records[key]
	if managed {
		if !isWithin(targetRoot, existing.TargetPath) {
			return DeploymentRecord{}, artifactReplacement{}, fmt.Errorf("artifact: existing managed target escaped target root")
		}
		h, hashErr := hashPath(existing.TargetPath)
		if hashErr != nil {
			if !os.IsNotExist(hashErr) {
				return DeploymentRecord{}, artifactReplacement{}, hashErr
			}
		} else if !strings.EqualFold(h, existing.InstalledHash) {
			return DeploymentRecord{}, artifactReplacement{}, fmt.Errorf("artifact: managed target was modified; refusing overwrite")
		}
	}
	if !managed || filepath.Clean(existing.TargetPath) != filepath.Clean(target) {
		if _, err := os.Lstat(target); err == nil {
			return DeploymentRecord{}, artifactReplacement{}, fmt.Errorf("artifact: target already exists and is not managed by this artifact: %s", target)
		} else if !os.IsNotExist(err) {
			return DeploymentRecord{}, artifactReplacement{}, err
		}
	}

	stage := target + fmt.Sprintf(".amitia-stage-%d", time.Now().UnixNano())
	_ = os.RemoveAll(stage)
	if err := deployArtifact(source, stage, artifact.Type); err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}
	defer os.RemoveAll(stage)

	h, err := hashPath(stage)
	if err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}
	tx, err := commitArtifactReplacement(stage, target, existing, managed)
	if err != nil {
		return DeploymentRecord{}, artifactReplacement{}, err
	}

	r := DeploymentRecord{
		ExtensionID: extensionID, ArtifactID: artifact.ID, TargetRoot: targetRoot,
		TargetPath: target, InstalledHash: h, InstalledAt: time.Now().UTC(),
	}
	m.records[key] = r
	return r, tx, nil
}

// artifactReplacement keeps a previous deployment recoverable until the
// deployment state file is durably committed.
type artifactReplacement struct {
	target       string
	previousPath string
	backup       string
}

func commitArtifactReplacement(stage, target string, existing DeploymentRecord, managed bool) (artifactReplacement, error) {
	tx := artifactReplacement{target: target}
	if managed {
		old := filepath.Clean(existing.TargetPath)
		tx.previousPath = old
		if _, err := os.Lstat(old); err == nil {
			tx.backup = old + fmt.Sprintf(".amitia-backup-%d", time.Now().UnixNano())
			_ = os.RemoveAll(tx.backup)
			if err := os.Rename(old, tx.backup); err != nil {
				return artifactReplacement{}, fmt.Errorf("artifact: backup previous deployment: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return artifactReplacement{}, err
		}
	}

	if err := os.Rename(stage, target); err != nil {
		if tx.backup != "" {
			_ = os.Rename(tx.backup, tx.previousPath)
		}
		return artifactReplacement{}, fmt.Errorf("artifact: commit deployment: %w", err)
	}
	return tx, nil
}

func (tx artifactReplacement) rollback() error {
	var errs []error
	if tx.target != "" {
		if err := os.RemoveAll(tx.target); err != nil {
			errs = append(errs, fmt.Errorf("remove new deployment %s: %w", tx.target, err))
		}
	}
	if tx.backup != "" {
		if err := os.Rename(tx.backup, tx.previousPath); err != nil {
			errs = append(errs, fmt.Errorf("restore previous deployment %s: %w", tx.previousPath, err))
		}
	}
	return errors.Join(errs...)
}

func (tx artifactReplacement) finalize() error {
	if tx.backup == "" {
		return nil
	}
	if err := os.RemoveAll(tx.backup); err != nil {
		return fmt.Errorf("remove deployment backup %s: %w", tx.backup, err)
	}
	return nil
}

func cloneDeploymentRecords(records map[string]DeploymentRecord) map[string]DeploymentRecord {
	copyOf := make(map[string]DeploymentRecord, len(records))
	for key, value := range records {
		copyOf[key] = value
	}
	return copyOf
}

func (m *ArtifactManager) rollbackDeploymentBatch(txs []artifactReplacement, original map[string]DeploymentRecord, cause error) error {
	m.records = original
	var errs []error
	if cause != nil {
		errs = append(errs, cause)
	}
	for i := len(txs) - 1; i >= 0; i-- {
		if err := txs[i].rollback(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *ArtifactManager) persistDeploymentBatch(txs []artifactReplacement, original map[string]DeploymentRecord) error {
	if err := m.saveLocked(); err != nil {
		_ = os.Remove(m.statePath + ".tmp")
		return m.rollbackDeploymentBatch(txs, original, err)
	}
	var errs []error
	for _, tx := range txs {
		if err := tx.finalize(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *ArtifactManager) statusesLocked(extensionID, targetRoot string, artifacts []gameprotocol.PluginArtifact) []ArtifactStatus {
	out := make([]ArtifactStatus, 0, len(artifacts))
	for _, a := range artifacts {
		r, ok := m.records[recordKey(extensionID, a.ID, targetRoot)]
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

func (m *ArtifactManager) resolveRequiredArtifacts(ctx context.Context, extensionID, compatibilityVersion string) ([]gameprotocol.PluginArtifact, integration.InstalledGeneration, error) {
	plugins, err := m.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	foundExtension := false
	requiredDeclared := false
	var required []gameprotocol.PluginArtifact
	for _, kp := range plugins {
		if string(kp.Extension.ID) != extensionID {
			continue
		}
		foundExtension = true
		spec, err := gameprotocol.ParsePluginHostSpec(kp.Contribution.Definition)
		if err != nil {
			return nil, integration.InstalledGeneration{}, err
		}
		if err := spec.Validate(); err != nil {
			return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: invalid game plugin host spec for %s: %w", extensionID, err)
		}
		for _, artifact := range spec.Artifacts {
			if !artifact.Required {
				continue
			}
			requiredDeclared = true
			if platformMatches(artifact.Platforms) && architectureMatches(artifact.Architectures) && versionMatches(artifact.CompatibilityVersions, compatibilityVersion) {
				required = append(required, artifact)
			}
		}
	}
	if !foundExtension {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: enabled game plugin extension %s not found", extensionID)
	}
	if !requiredDeclared {
		return nil, integration.InstalledGeneration{}, nil
	}
	if len(required) == 0 {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: no required artifact is compatible with host platform %s/%s and compatibility version %q", currentPlatform(), currentArchitecture(), strings.TrimSpace(compatibilityVersion))
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	return required, generation, nil
}

func (m *ArtifactManager) resolveArtifactsOptional(ctx context.Context, extensionID, compatibilityVersion string) ([]gameprotocol.PluginArtifact, integration.InstalledGeneration, error) {
	plugins, err := m.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	var artifacts []gameprotocol.PluginArtifact
	foundExtension := false
	for _, kp := range plugins {
		if string(kp.Extension.ID) != extensionID {
			continue
		}
		foundExtension = true
		spec, err := gameprotocol.ParsePluginHostSpec(kp.Contribution.Definition)
		if err != nil {
			return nil, integration.InstalledGeneration{}, err
		}
		if err := spec.Validate(); err != nil {
			return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: invalid game plugin host spec for %s: %w", extensionID, err)
		}
		for _, artifact := range spec.Artifacts {
			if platformMatches(artifact.Platforms) && architectureMatches(artifact.Architectures) && versionMatches(artifact.CompatibilityVersions, compatibilityVersion) {
				artifacts = append(artifacts, artifact)
			}
		}
	}
	if !foundExtension {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: enabled game plugin extension %s not found", extensionID)
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

func (m *ArtifactManager) resolveArtifacts(ctx context.Context, extensionID, compatibilityVersion string) ([]gameprotocol.PluginArtifact, integration.InstalledGeneration, error) {
	artifacts, generation, err := m.resolveArtifactsOptional(ctx, extensionID, compatibilityVersion)
	if err != nil {
		return nil, integration.InstalledGeneration{}, err
	}
	if len(artifacts) == 0 {
		return nil, integration.InstalledGeneration{}, fmt.Errorf("artifact: no compatible artifacts for extension %s", extensionID)
	}
	return artifacts, generation, nil
}

func canonicalTargetRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("artifact: targetRoot is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("artifact: targetRoot unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("artifact: targetRoot must be a directory")
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(real), nil
}
func resolveContained(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("artifact: source must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact: invalid source path")
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
		return "", fmt.Errorf("artifact: source escaped generation")
	}
	return real, nil
}
func resolveTarget(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("artifact: target must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact: invalid target")
	}
	target := filepath.Join(root, clean)
	if !isWithin(root, target) {
		return "", fmt.Errorf("artifact: target escaped target root")
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
		return "", fmt.Errorf("artifact: target parent escaped target root")
	}
	return target, nil
}
func isWithin(root, path string) bool {
	r, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
func findArtifact(items []gameprotocol.PluginArtifact, id string) (gameprotocol.PluginArtifact, bool) {
	for _, a := range items {
		if a.ID == id {
			return a, true
		}
	}
	return gameprotocol.PluginArtifact{}, false
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
func currentPlatform() string     { return runtimeGOOS() }
func currentArchitecture() string { return runtimeGOARCH() }
func architectureMatches(architectures []string) bool {
	if len(architectures) == 0 {
		return true
	}
	arch := currentArchitecture()
	for _, value := range architectures {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "all" || normalized == "*" || normalized == arch {
			return true
		}
		switch arch {
		case "amd64":
			if normalized == "x86_64" || normalized == "x64" {
				return true
			}
		case "arm64":
			if normalized == "aarch64" {
				return true
			}
		}
	}
	return false
}
func normalizeHash(v string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), "sha256:")
}

func validateArtifactSource(source, kind string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("artifact: symlink source rejected")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case gameprotocol.PluginArtifactTypeFile:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact: type file requires a regular file source")
		}
		if info.Size() > maxArtifactSingleFileBytes {
			return fmt.Errorf("artifact: source file exceeds %d bytes", maxArtifactSingleFileBytes)
		}
	case gameprotocol.PluginArtifactTypeDirectory:
		if !info.IsDir() {
			return fmt.Errorf("artifact: type directory requires a directory source")
		}
		_, _, err := scanArtifactTree(source)
		return err
	case gameprotocol.PluginArtifactTypeZIP:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact: type zip requires a regular file source")
		}
		if info.Size() > maxArtifactSingleFileBytes {
			return fmt.Errorf("artifact: zip source exceeds %d bytes", maxArtifactSingleFileBytes)
		}
		return validateZipBudget(source)
	default:
		return fmt.Errorf("artifact: unsupported type %q", kind)
	}
	return nil
}

func deployArtifact(source, target, kind string) error {
	if err := validateArtifactSource(source, kind); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case gameprotocol.PluginArtifactTypeFile:
		return copyFileAtomic(source, target, info.Mode().Perm())
	case gameprotocol.PluginArtifactTypeDirectory:
		return copyTreeAtomic(source, target)
	case gameprotocol.PluginArtifactTypeZIP:
		return extractZipAtomic(source, target)
	default:
		return fmt.Errorf("artifact: unsupported type %q", kind)
	}
}

func copyFileAtomic(source, target string, mode os.FileMode) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("artifact: only regular files can be copied")
	}
	if info.Size() > maxArtifactSingleFileBytes {
		return fmt.Errorf("artifact: file exceeds %d bytes", maxArtifactSingleFileBytes)
	}
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
	written, copyErr := io.Copy(out, io.LimitReader(in, maxArtifactSingleFileBytes+1))
	closeErr := out.Close()
	if copyErr == nil && written > maxArtifactSingleFileBytes {
		copyErr = fmt.Errorf("artifact: file exceeded %d bytes during copy", maxArtifactSingleFileBytes)
	}
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

func scanArtifactTree(source string) (int, int64, error) {
	files := 0
	var total int64
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact: symlink rejected: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		depth := len(strings.FieldsFunc(filepath.ToSlash(rel), func(r rune) bool { return r == '/' }))
		if depth > maxArtifactDepth {
			return fmt.Errorf("artifact: directory depth exceeds %d", maxArtifactDepth)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact: unsupported non-regular file: %s", path)
		}
		files++
		if files > maxArtifactFiles {
			return fmt.Errorf("artifact: file count exceeds %d", maxArtifactFiles)
		}
		if info.Size() > maxArtifactSingleFileBytes {
			return fmt.Errorf("artifact: file %s exceeds %d bytes", path, maxArtifactSingleFileBytes)
		}
		total += info.Size()
		if total > maxArtifactTotalBytes {
			return fmt.Errorf("artifact: total tree size exceeds %d bytes", maxArtifactTotalBytes)
		}
		return nil
	})
	return files, total, err
}

func copyTreeAtomic(source, target string) error {
	if _, _, err := scanArtifactTree(source); err != nil {
		return err
	}
	tmp := target + ".amitia-tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
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

func validateZipBudget(source string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer r.Close()
	if len(r.File) > maxArtifactFiles {
		return fmt.Errorf("artifact: zip entry count exceeds %d", maxArtifactFiles)
	}
	var total uint64
	for _, f := range r.File {
		clean := filepath.Clean(f.Name)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("artifact: unsafe zip entry %q", f.Name)
		}
		depth := len(strings.FieldsFunc(filepath.ToSlash(clean), func(r rune) bool { return r == '/' }))
		if depth > maxArtifactDepth {
			return fmt.Errorf("artifact: zip entry depth exceeds %d", maxArtifactDepth)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact: zip symlink rejected")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return fmt.Errorf("artifact: zip contains unsupported non-regular entry %q", f.Name)
		}
		if f.UncompressedSize64 > uint64(maxArtifactSingleFileBytes) {
			return fmt.Errorf("artifact: zip entry %q exceeds %d bytes", f.Name, maxArtifactSingleFileBytes)
		}
		total += f.UncompressedSize64
		if total > uint64(maxArtifactTotalBytes) {
			return fmt.Errorf("artifact: zip uncompressed size exceeds %d bytes", maxArtifactTotalBytes)
		}
		if f.UncompressedSize64 >= 1<<20 && f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > maxZipCompressionRatio {
			return fmt.Errorf("artifact: zip entry %q compression ratio exceeds %d:1", f.Name, maxZipCompressionRatio)
		}
	}
	return nil
}

func extractZipAtomic(source, target string) error {
	if err := validateZipBudget(source); err != nil {
		return err
	}
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
	var extracted int64
	for _, f := range r.File {
		clean := filepath.Clean(f.Name)
		dst := filepath.Join(tmp, clean)
		if !isWithin(tmp, dst) {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("artifact: zip entry escaped target")
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
		limit := int64(f.UncompressedSize64) + 1
		if limit > maxArtifactSingleFileBytes+1 {
			limit = maxArtifactSingleFileBytes + 1
		}
		written, copyErr := io.Copy(out, io.LimitReader(src, limit))
		srcErr := src.Close()
		outErr := out.Close()
		if copyErr == nil && written != int64(f.UncompressedSize64) {
			copyErr = fmt.Errorf("artifact: zip entry %q size mismatch", f.Name)
		}
		if copyErr == nil && written > maxArtifactSingleFileBytes {
			copyErr = fmt.Errorf("artifact: zip entry %q exceeds per-file limit", f.Name)
		}
		if copyErr != nil {
			_ = os.RemoveAll(tmp)
			return copyErr
		}
		if srcErr != nil {
			_ = os.RemoveAll(tmp)
			return srcErr
		}
		if outErr != nil {
			_ = os.RemoveAll(tmp)
			return outErr
		}
		extracted += written
		if extracted > maxArtifactTotalBytes {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("artifact: zip extraction exceeds %d bytes", maxArtifactTotalBytes)
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
		return "", fmt.Errorf("artifact: symlink rejected")
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
			return fmt.Errorf("artifact: symlink rejected")
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
