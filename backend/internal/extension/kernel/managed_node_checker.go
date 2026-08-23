package kernel

import (
	"context"
	"path/filepath"

	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type managedNodeChecker struct {
	resolver script_host.NodeEnvironmentResolver
}

func newManagedNodeChecker(resolver script_host.NodeEnvironmentResolver) *managedNodeChecker {
	return &managedNodeChecker{resolver: resolver}
}

func (c *managedNodeChecker) IsManagedNode(exePath string) bool {
	if c == nil || c.resolver == nil || exePath == "" {
		return false
	}
	env, err := c.resolver.Resolve(context.Background())
	if err != nil || env.NodeBinary == "" {
		return false
	}
	got, err := filepath.Abs(exePath)
	if err != nil {
		return false
	}
	want, err := filepath.Abs(env.NodeBinary)
	if err != nil {
		return false
	}
	return filepath.Clean(got) == filepath.Clean(want)
}
