// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

type ResourceKind int

const (
	ResourceKindFilesystem ResourceKind = iota
	ResourceKindVirtual
)

type ResolvedResource struct {
	URI       ResourceURI
	Kind      ResourceKind
	LocalPath string
}

type ResourceResolver interface {
	Resolve(uri ResourceURI) (ResolvedResource, error)
	Reverse(localPath string) (ResourceURI, error)
}
