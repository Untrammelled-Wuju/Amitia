package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DirectoryManager interface {
	Root() string
	ResolvePluginPaths(pluginID domain.PluginID) (PluginPaths, error)
	ResolveRuntimePaths(runtimeID domain.RuntimeInstanceID) (RuntimePaths, error)
	ResolveServicePaths(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServicePaths, error)
	EnsurePluginPaths(ctx context.Context, pluginID domain.PluginID) (PluginPaths, error)
	EnsureRuntimePaths(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimePaths, error)
	EnsureServicePaths(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServicePaths, error)
	BuildRuntimeContext(pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID) (RuntimeDirectoryContext, error)
	BuildServiceContext(pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServiceDirectoryContext, error)
	RemoveRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	RemoveRuntimeTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	RemovePluginCache(ctx context.Context, pluginID domain.PluginID) error
	ValidatePath(path string) error
}

type directoryManagerImpl struct {
	mu       sync.Mutex
	dataRoot string
}

func NewDirectoryManager(dataRoot string) (DirectoryManager, error) {
	if dataRoot == "" {
		return nil, errors.New("data root must not be empty")
	}
	abs, err := filepath.Abs(dataRoot)
	if err != nil {
		return nil, err
	}
	return &directoryManagerImpl{dataRoot: filepath.Clean(abs)}, nil
}

func (m *directoryManagerImpl) Root() string {
	return m.dataRoot
}

func (m *directoryManagerImpl) ResolvePluginPaths(pluginID domain.PluginID) (PluginPaths, error) {
	if err := checkPluginID(pluginID); err != nil {
		return PluginPaths{}, err
	}
	key, err := StorageKeyForPluginID(pluginID)
	if err != nil {
		return PluginPaths{}, err
	}
	root := filepath.Join(m.dataRoot, "gamehost", "plugins", key.String())
	cleaned := filepath.Clean(root)
	if !strings.HasPrefix(cleaned, m.dataRoot+string(filepath.Separator)) && cleaned != m.dataRoot {
		return PluginPaths{}, newPathEscapeError("resolve_plugin", string(pluginID))
	}
	return PluginPaths{
		Root:   cleaned,
		Data:   filepath.Join(cleaned, "data"),
		Cache:  filepath.Join(cleaned, "cache"),
		Shared: filepath.Join(cleaned, "shared"),
	}, nil
}

func (m *directoryManagerImpl) ResolveRuntimePaths(runtimeID domain.RuntimeInstanceID) (RuntimePaths, error) {
	if err := checkRuntimeID(runtimeID); err != nil {
		return RuntimePaths{}, err
	}
	key, err := StorageKeyForRuntimeID(runtimeID)
	if err != nil {
		return RuntimePaths{}, err
	}
	root := filepath.Join(m.dataRoot, "gamehost", "runtimes", key.String())
	if err := m.ValidatePath(root); err != nil {
		return RuntimePaths{}, err
	}
	return RuntimePaths{
		Root:     root,
		Data:     filepath.Join(root, "data"),
		Temp:     filepath.Join(root, "temp"),
		Cache:    filepath.Join(root, "cache"),
		Services: filepath.Join(root, "services"),
	}, nil
}

func (m *directoryManagerImpl) ResolveServicePaths(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServicePaths, error) {
	if err := checkRuntimeID(runtimeID); err != nil {
		return ServicePaths{}, err
	}
	if err := checkServiceID(serviceID); err != nil {
		return ServicePaths{}, err
	}
	runtimePaths, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return ServicePaths{}, err
	}
	serviceKey, err := StorageKeyForServiceID(runtimeID, serviceID)
	if err != nil {
		return ServicePaths{}, err
	}
	root := filepath.Join(runtimePaths.Services, serviceKey.String())
	if err := m.ValidatePath(root); err != nil {
		return ServicePaths{}, err
	}
	return ServicePaths{
		Root:  root,
		Data:  filepath.Join(root, "data"),
		Temp:  filepath.Join(root, "temp"),
		Cache: filepath.Join(root, "cache"),
	}, nil
}

func (m *directoryManagerImpl) EnsurePluginPaths(ctx context.Context, pluginID domain.PluginID) (PluginPaths, error) {
	if err := ctx.Err(); err != nil {
		return PluginPaths{}, err
	}
	paths, err := m.ResolvePluginPaths(pluginID)
	if err != nil {
		return PluginPaths{}, err
	}
	dirs := []string{paths.Root, paths.Data, paths.Cache, paths.Shared}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return PluginPaths{}, newCreateFailedError("ensure_plugin", dir, err)
		}
	}
	return paths, nil
}

func (m *directoryManagerImpl) EnsureRuntimePaths(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimePaths, error) {
	if err := ctx.Err(); err != nil {
		return RuntimePaths{}, err
	}
	paths, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return RuntimePaths{}, err
	}
	dirs := []string{paths.Root, paths.Data, paths.Temp, paths.Cache, paths.Services}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return RuntimePaths{}, newCreateFailedError("ensure_runtime", dir, err)
		}
	}
	return paths, nil
}

func (m *directoryManagerImpl) EnsureServicePaths(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServicePaths, error) {
	if err := ctx.Err(); err != nil {
		return ServicePaths{}, err
	}
	paths, err := m.ResolveServicePaths(runtimeID, serviceID)
	if err != nil {
		return ServicePaths{}, err
	}
	dirs := []string{paths.Root, paths.Data, paths.Temp, paths.Cache}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ServicePaths{}, newCreateFailedError("ensure_service", dir, err)
		}
	}
	return paths, nil
}

func (m *directoryManagerImpl) BuildRuntimeContext(pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID) (RuntimeDirectoryContext, error) {
	plugin, err := m.ResolvePluginPaths(pluginID)
	if err != nil {
		return RuntimeDirectoryContext{}, err
	}
	runtime, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return RuntimeDirectoryContext{}, err
	}
	return RuntimeDirectoryContext{
		Plugin:  plugin,
		Runtime: runtime,
	}, nil
}

func (m *directoryManagerImpl) BuildServiceContext(pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServiceDirectoryContext, error) {
	plugin, err := m.ResolvePluginPaths(pluginID)
	if err != nil {
		return ServiceDirectoryContext{}, err
	}
	runtime, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return ServiceDirectoryContext{}, err
	}
	service, err := m.ResolveServicePaths(runtimeID, serviceID)
	if err != nil {
		return ServiceDirectoryContext{}, err
	}
	return ServiceDirectoryContext{
		Plugin:  plugin,
		Runtime: runtime,
		Service: service,
	}, nil
}

func (m *directoryManagerImpl) RemoveRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return err
	}
	if err := safeRemoveAll(paths.Root); err != nil {
		return err
	}
	return safeRemoveEmptyParents(paths.Root, m.dataRoot)
}

func (m *directoryManagerImpl) RemoveRuntimeTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := m.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return err
	}
	return safeRemoveAll(paths.Temp)
}

func (m *directoryManagerImpl) RemovePluginCache(ctx context.Context, pluginID domain.PluginID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := m.ResolvePluginPaths(pluginID)
	if err != nil {
		return err
	}
	return safeRemoveAll(paths.Cache)
}

func (m *directoryManagerImpl) ValidatePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	if abs != m.dataRoot && !strings.HasPrefix(abs, m.dataRoot+string(filepath.Separator)) {
		return &PathEscapesRootError{Path: path, Root: m.dataRoot}
	}
	return nil
}

func checkPluginID(pluginID domain.PluginID) error {
	if pluginID == "" {
		return newInvalidPathError("resolve_plugin", "", domain.NewHostError(domain.ErrInvalidArgument, "plugin id must not be empty"))
	}
	return nil
}

func checkRuntimeID(runtimeID domain.RuntimeInstanceID) error {
	if runtimeID == "" {
		return newInvalidPathError("resolve_runtime", "", domain.NewHostError(domain.ErrInvalidArgument, "runtime id must not be empty"))
	}
	return nil
}

func checkServiceID(serviceID domain.ServiceID) error {
	if serviceID == "" {
		return newInvalidPathError("resolve_service", "", domain.NewHostError(domain.ErrInvalidArgument, "service id must not be empty"))
	}
	return nil
}

func safeRemoveAll(path string) error {
	if path == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return newRemoveFailedError("remove", path, err)
	}
	return nil
}

func safeRemoveEmptyParents(childRoot, stopRoot string) error {
	parent := filepath.Dir(childRoot)
	for parent != stopRoot && len(parent) > len(stopRoot) {
		err := os.Remove(parent)
		if err != nil {
			break
		}
		parent = filepath.Dir(parent)
	}
	return nil
}

func EnsureWithinRoot(root, candidate string) error {
	if root == "" {
		return errors.New("root must not be empty")
	}
	if candidate == "" {
		return errors.New("candidate path must not be empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absRoot = filepath.Clean(absRoot)
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	absCandidate = filepath.Clean(absCandidate)
	if absCandidate == absRoot {
		return nil
	}
	if !strings.HasPrefix(absCandidate, absRoot+string(filepath.Separator)) {
		return errors.New("path escapes root: " + candidate)
	}
	return nil
}

type PathEscapesRootError struct {
	Path string
	Root string
}

func (e *PathEscapesRootError) Error() string {
	return "path escapes root: " + e.Path
}
