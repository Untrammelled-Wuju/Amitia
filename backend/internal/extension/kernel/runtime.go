package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
)

type InstalledExtension struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Publisher   string    `json:"publisher"`
	Path        string    `json:"path"`
	ModuleCount int       `json:"moduleCount"`
	InstalledAt time.Time `json:"installedAt"`
}

type Runtime struct {
	mu          sync.RWMutex
	root        string
	installer   *amitiax.Installer
	installed   map[string]InstalledExtension
	container   *Container
	enableLocks sync.Map
}

func NewRuntime(root string) (*Runtime, error) {
	if root == "" {
		return nil, errors.New("extension kernel root required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	runtime := &Runtime{root: root, installer: amitiax.NewInstaller(), installed: make(map[string]InstalledExtension)}
	if err := runtime.Recover(); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) Root() string {
	return r.root
}

func (r *Runtime) SetContainer(c *Container) {
	r.mu.Lock()
	r.container = c
	r.mu.Unlock()
}

func (r *Runtime) Container() *Container {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.container
}

func (r *Runtime) getEnableLock(extensionID string) *sync.Mutex {
	actual, _ := r.enableLocks.LoadOrStore(extensionID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (r *Runtime) Recover() error {
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return err
	}
	recovered := make(map[string]InstalledExtension)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(r.root, entry.Name())
		manifestPath := filepath.Join(dir, amitiax.ManifestFile)
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			continue
		}
		manifest, parseErr := manifest_v2.Parse(data)
		if parseErr != nil {
			continue
		}
		report := manifest.Validate()
		if report.HasErrors() {
			continue
		}
		info, statErr := os.Stat(dir)
		if statErr != nil {
			continue
		}
		recovered[manifest.Extension.ID] = installedFromManifest(manifest, dir, info.ModTime())
	}
	r.mu.Lock()
	r.installed = recovered
	r.mu.Unlock()
	return nil
}

func (r *Runtime) List() []InstalledExtension {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]InstalledExtension, 0, len(r.installed))
	for _, item := range r.installed {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *Runtime) Install(ctx context.Context, archivePath string) (InstalledExtension, error) {
	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		return InstalledExtension{}, err
	}
	if report := pkg.Manifest.Validate(); report.HasErrors() {
		data, _ := json.Marshal(report.Errors)
		return InstalledExtension{}, fmt.Errorf("manifest validation failed: %s", data)
	}
	name := safeDirectoryName(pkg.Manifest.Extension.ID)
	staging, err := os.MkdirTemp(r.root, ".install-"+name+"-")
	if err != nil {
		return InstalledExtension{}, err
	}
	defer os.RemoveAll(staging)
	result := r.installer.Install(ctx, amitiax.InstallRequest{ArchivePath: archivePath, TargetDir: staging, ExtensionID: domain.ExtensionID(pkg.Manifest.Extension.ID)})
	if result.Status != amitiax.InstallSucceeded {
		return InstalledExtension{}, fmt.Errorf("installation failed: %v", result.Errors)
	}
	target := filepath.Join(r.root, name)
	backup := target + ".previous"
	_ = os.RemoveAll(backup)
	if _, statErr := os.Stat(target); statErr == nil {
		if err := os.Rename(target, backup); err != nil {
			return InstalledExtension{}, err
		}
	}
	if err := os.Rename(staging, target); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, target)
		}
		return InstalledExtension{}, err
	}
	_ = os.RemoveAll(backup)
	item := installedFromManifest(pkg.Manifest, target, time.Now().UTC())
	r.mu.Lock()
	r.installed[item.ID] = item
	r.mu.Unlock()
	return item, nil
}

func installedFromManifest(manifest manifest_v2.Manifest, path string, installedAt time.Time) InstalledExtension {
	return InstalledExtension{ID: manifest.Extension.ID, Name: manifest.Extension.Name.Default, Version: manifest.Extension.Version, Publisher: manifest.Publisher.ID, Path: path, ModuleCount: len(manifest.Modules), InstalledAt: installedAt.UTC()}
}

func safeDirectoryName(id string) string {
	replacer := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_")
	return replacer.Replace(id)
}
