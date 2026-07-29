package kernel

import "sync/atomic"

type LegacyReadCounter struct {
	previewUninstall  atomic.Int64
	dependencies      atomic.Int64
	listVersions      atomic.Int64
	compareVersions   atomic.Int64
	export            atomic.Int64
	dependenciesList  atomic.Int64
	packageReadCalls  atomic.Int64
}

func NewLegacyReadCounter() *LegacyReadCounter {
	return &LegacyReadCounter{}
}

func (c *LegacyReadCounter) IncPreviewUninstall() {
	c.previewUninstall.Add(1)
}

func (c *LegacyReadCounter) IncDependencies() {
	c.dependencies.Add(1)
}

func (c *LegacyReadCounter) IncListVersions() {
	c.listVersions.Add(1)
}

func (c *LegacyReadCounter) IncCompareVersions() {
	c.compareVersions.Add(1)
}

func (c *LegacyReadCounter) IncExport() {
	c.export.Add(1)
}

func (c *LegacyReadCounter) IncDependenciesList() {
	c.dependenciesList.Add(1)
}

func (c *LegacyReadCounter) IncPackageReadCalls() {
	c.packageReadCalls.Add(1)
}

func (c *LegacyReadCounter) PreviewUninstallFallbacks() int64 {
	return c.previewUninstall.Load()
}

func (c *LegacyReadCounter) DependenciesFallbacks() int64 {
	return c.dependencies.Load()
}

func (c *LegacyReadCounter) ListVersionsFallbacks() int64 {
	return c.listVersions.Load()
}

func (c *LegacyReadCounter) CompareVersionsFallbacks() int64 {
	return c.compareVersions.Load()
}

func (c *LegacyReadCounter) ExportFallbacks() int64 {
	return c.export.Load()
}

func (c *LegacyReadCounter) DependenciesListFallbacks() int64 {
	return c.dependenciesList.Load()
}

func (c *LegacyReadCounter) PackageReadCallsFallbacks() int64 {
	return c.packageReadCalls.Load()
}

func (c *LegacyReadCounter) Total() int64 {
	return c.previewUninstall.Load() +
		c.dependencies.Load() +
		c.listVersions.Load() +
		c.compareVersions.Load() +
		c.export.Load() +
		c.dependenciesList.Load() +
		c.packageReadCalls.Load()
}

func (c *LegacyReadCounter) Snapshot() map[string]int64 {
	return map[string]int64{
		"legacy_read_preview_uninstall": c.previewUninstall.Load(),
		"legacy_read_dependencies":      c.dependencies.Load(),
		"legacy_read_list_versions":     c.listVersions.Load(),
		"legacy_read_compare_versions":  c.compareVersions.Load(),
		"legacy_read_export":            c.export.Load(),
		"legacy_read_dependencies_list": c.dependenciesList.Load(),
		"legacy_package_read_calls":     c.packageReadCalls.Load(),
		"legacy_read_total":             c.Total(),
	}
}

var globalLegacyReadCounter = NewLegacyReadCounter()

func GlobalLegacyReadCounter() *LegacyReadCounter {
	return globalLegacyReadCounter
}
