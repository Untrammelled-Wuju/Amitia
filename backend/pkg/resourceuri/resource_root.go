// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import (
	"fmt"
	"strings"
)

type ResourceRoot string

const (
	ResourceRootWorkspace   ResourceRoot = "workspace"
	ResourceRootAttachments ResourceRoot = "attachments"
	ResourceRootData        ResourceRoot = "data"
	ResourceRootCache       ResourceRoot = "cache"
	ResourceRootRuntime     ResourceRoot = "runtime"
	ResourceRootConfig      ResourceRoot = "config"
	ResourceRootExtensions  ResourceRoot = "extensions"
	ResourceRootLogs        ResourceRoot = "logs"
	ResourceRootTemp        ResourceRoot = "temp"
	ResourceRootNative      ResourceRoot = "native"
)

var allResourceRoots = []ResourceRoot{
	ResourceRootWorkspace,
	ResourceRootAttachments,
	ResourceRootData,
	ResourceRootCache,
	ResourceRootRuntime,
	ResourceRootConfig,
	ResourceRootExtensions,
	ResourceRootLogs,
	ResourceRootTemp,
	ResourceRootNative,
}

var resourceRootSet = func() map[ResourceRoot]struct{} {
	m := make(map[ResourceRoot]struct{}, len(allResourceRoots))
	for _, r := range allResourceRoots {
		m[r] = struct{}{}
	}
	return m
}()

func (r ResourceRoot) IsFilesystem() bool {
	return r != ResourceRootNative
}

func (r ResourceRoot) IsVirtual() bool {
	return r == ResourceRootNative
}

func parseResourceRoot(value string) (ResourceRoot, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", fmt.Errorf("%q: %w", value, ErrInvalidRoot)
	}
	candidate := ResourceRoot(trimmed)
	if _, ok := resourceRootSet[candidate]; !ok {
		return "", fmt.Errorf("%q: %w", trimmed, ErrInvalidRoot)
	}
	return candidate, nil
}
