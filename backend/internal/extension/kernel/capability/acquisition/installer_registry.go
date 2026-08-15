package acquisition

import (
	"fmt"
	"sync"
)

// InstallerRegistry 管理所有 Installer 的注册与查找。
// 按 InstallMethod 分发到对应的 Installer 实现。
type InstallerRegistry struct {
	mu        sync.RWMutex
	installers map[InstallMethod]Installer
}

// NewInstallerRegistry 创建 InstallerRegistry 并注册所有内置 Installer。
func NewInstallerRegistry() *InstallerRegistry {
	r := &InstallerRegistry{
		installers: make(map[InstallMethod]Installer),
	}
	r.registerDefaults()
	return r
}

// Register 注册或覆盖指定方法的 Installer。
func (r *InstallerRegistry) Register(method InstallMethod, installer Installer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.installers[method] = installer
}

// Resolve 按 InstallMethod 查找对应的 Installer。
func (r *InstallerRegistry) Resolve(method InstallMethod) (Installer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	installer, ok := r.installers[method]
	if !ok {
		return nil, fmt.Errorf("no installer registered for method %q", method)
	}
	return installer, nil
}

// Methods 返回所有已注册的 InstallMethod 列表。
func (r *InstallerRegistry) Methods() []InstallMethod {
	r.mu.RLock()
	defer r.mu.RUnlock()
	methods := make([]InstallMethod, 0, len(r.installers))
	for m := range r.installers {
		methods = append(methods, m)
	}
	return methods
}

// registerDefaults 注册所有内置 Installer 实现。
func (r *InstallerRegistry) registerDefaults() {
	r.installers[InstallExtension] = &ExtensionPackageInstaller{}
	r.installers[InstallMCP] = &MCPInstaller{}
	r.installers[InstallSkill] = &SkillInstaller{}
	r.installers[InstallGeneratedSkill] = &GeneratedSkillInstaller{}
	r.installers[InstallEnableExisting] = &EnableExistingInstaller{}
}
