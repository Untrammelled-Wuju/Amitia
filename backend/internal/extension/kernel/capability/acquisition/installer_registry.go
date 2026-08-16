package acquisition

import (
	"fmt"
	"sync"
)

// InstallerRegistryOpts holds optional dependencies for constructing Installers.
// When provided, InstallerRegistry will create real implementations instead of placeholders.
type InstallerRegistryOpts struct {
	PackageInstallPort PackageInstallPort
	MCPInstallPort     MCPInstallPort
	SkillInstallPort   SkillInstallPort
	EnableExistingPort EnableExistingPort
	WorkshopPort       WorkshopGeneratePort
}

// InstallerRegistry 管理所有 Installer 的注册与查找。
// 按 InstallMethod 分发到对应的 Installer 实现。
type InstallerRegistry struct {
	mu         sync.RWMutex
	installers map[InstallMethod]Installer
}

// NewInstallerRegistry 创建 InstallerRegistry。如果 opts 提供了 port，则创建真实实现。
// 启动后若缺这四类正式 Installer，直接 Hard Gate 失败。
func NewInstallerRegistry(opts *InstallerRegistryOpts) (*InstallerRegistry, error) {
	r := &InstallerRegistry{
		installers: make(map[InstallMethod]Installer),
	}
	r.registerDefaults(opts)
	if err := r.validateAllInstallersRegistered(); err != nil {
		return nil, err
	}
	return r, nil
}

// validateAllInstallersRegistered 验证所有必需的 Installer 都已注册
func (r *InstallerRegistry) validateAllInstallersRegistered() error {
	required := map[InstallMethod]string{
		InstallExtension:      "PackageInstallPort (Extension)",
		InstallMCP:            "MCPInstallPort",
		InstallSkill:          "SkillInstallPort",
		InstallEnableExisting: "EnableExistingPort",
	}
	return r.checkRequiredInstallers(required)
}

// checkRequiredInstallers 检查必需的 Installer 是否都已注册
func (r *InstallerRegistry) checkRequiredInstallers(required map[InstallMethod]string) error {
	var missing []string
	for method, name := range required {
		if _, ok := r.installers[method]; !ok {
			missing = append(missing, fmt.Sprintf("%s (%s)", method, name))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("installer registry: missing required installers: %v", missing)
	}
	return nil
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
func (r *InstallerRegistry) registerDefaults(opts *InstallerRegistryOpts) {
	if opts != nil {
		if opts.PackageInstallPort != nil {
			r.installers[InstallExtension] = NewExtensionPackageInstaller(opts.PackageInstallPort)
		}
		if opts.MCPInstallPort != nil {
			r.installers[InstallMCP] = NewMCPInstaller(opts.MCPInstallPort)
		}
		if opts.SkillInstallPort != nil {
			r.installers[InstallSkill] = NewSkillInstaller(opts.SkillInstallPort)
			if opts.WorkshopPort != nil {
				r.installers[InstallGeneratedSkill] = NewGeneratedSkillInstallerWithWorkshop(opts.SkillInstallPort, opts.WorkshopPort)
			}
		}
		if opts.EnableExistingPort != nil {
			r.installers[InstallEnableExisting] = NewEnableExistingInstaller(opts.EnableExistingPort)
		}
	}
}
