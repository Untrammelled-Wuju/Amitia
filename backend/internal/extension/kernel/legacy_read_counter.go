package kernel

import (
	"context"
	"sync/atomic"
)

type LegacyReadCounter struct {
	previewUninstall atomic.Int64
	dependencies     atomic.Int64
	listVersions     atomic.Int64
	compareVersions  atomic.Int64
	export           atomic.Int64
	dependenciesList atomic.Int64
	packageReadCalls atomic.Int64
	store            *LegacyCounterStore
}

func NewLegacyReadCounter() *LegacyReadCounter {
	return &LegacyReadCounter{}
}

func NewLegacyReadCounterWithStore(store *LegacyCounterStore) *LegacyReadCounter {
	return &LegacyReadCounter{store: store}
}

func (c *LegacyReadCounter) SetStore(store *LegacyCounterStore) {
	c.store = store
}

func (c *LegacyReadCounter) LoadFromStore(ctx context.Context) error {
	if c.store == nil {
		return nil
	}
	return c.store.LoadAll(ctx)
}

func (c *LegacyReadCounter) syncToStore(metricName string) {
	if c.store == nil {
		return
	}
	_, _ = c.store.Increment(context.Background(), metricName)
}

func (c *LegacyReadCounter) storeGet(metricName string) int64 {
	if c.store == nil {
		return 0
	}
	return c.store.Get(context.Background(), metricName)
}

func (c *LegacyReadCounter) IncPreviewUninstall() {
	c.previewUninstall.Add(1)
	c.syncToStore("legacy_read_preview_uninstall")
}

func (c *LegacyReadCounter) IncDependencies() {
	c.dependencies.Add(1)
	c.syncToStore("legacy_read_dependencies")
}

func (c *LegacyReadCounter) IncListVersions() {
	c.listVersions.Add(1)
	c.syncToStore("legacy_read_list_versions")
}

func (c *LegacyReadCounter) IncCompareVersions() {
	c.compareVersions.Add(1)
	c.syncToStore("legacy_read_compare_versions")
}

func (c *LegacyReadCounter) IncExport() {
	c.export.Add(1)
	c.syncToStore("legacy_read_export")
}

func (c *LegacyReadCounter) IncDependenciesList() {
	c.dependenciesList.Add(1)
	c.syncToStore("legacy_read_dependencies_list")
}

func (c *LegacyReadCounter) IncPackageReadCalls() {
	c.packageReadCalls.Add(1)
	c.syncToStore("legacy_package_read_calls")
}

func (c *LegacyReadCounter) PreviewUninstallFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_preview_uninstall")
	}
	return c.previewUninstall.Load()
}

func (c *LegacyReadCounter) DependenciesFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_dependencies")
	}
	return c.dependencies.Load()
}

func (c *LegacyReadCounter) ListVersionsFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_list_versions")
	}
	return c.listVersions.Load()
}

func (c *LegacyReadCounter) CompareVersionsFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_compare_versions")
	}
	return c.compareVersions.Load()
}

func (c *LegacyReadCounter) ExportFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_export")
	}
	return c.export.Load()
}

func (c *LegacyReadCounter) DependenciesListFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_read_dependencies_list")
	}
	return c.dependenciesList.Load()
}

func (c *LegacyReadCounter) PackageReadCallsFallbacks() int64 {
	if c.store != nil {
		return c.storeGet("legacy_package_read_calls")
	}
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
	if c.store != nil {
		snap := c.store.Snapshot(context.Background())
		result := make(map[string]int64, len(snap)+1)
		for k, v := range snap {
			result[k] = v
		}
		result["legacy_read_total"] = c.Total()
		return result
	}
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
