// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/pkg/resourceuri"
	"github.com/u-ai/backend/pkg/util"
)

type Resolver interface {
	Resolve(context.Context, qdrantenv.Environment) (Layout, error)
}

type ResolveContext struct {
	Config    *config.Config
	Host      runtimehost.RuntimeHost
	Inspector FileInspector
}

type resolver struct {
	config    *config.Config
	host      runtimehost.RuntimeHost
	inspector FileInspector
}

func NewResolver(ctx ResolveContext) (Resolver, error) {
	if ctx.Config == nil {
		return nil, fmt.Errorf("qdrantlayout: config is required")
	}
	if ctx.Host == nil {
		return nil, fmt.Errorf("qdrantlayout: host is required")
	}
	inspector := ctx.Inspector
	if inspector == nil {
		inspector = newDefaultFileInspector()
	}
	return &resolver{
		config:    ctx.Config,
		host:      ctx.Host,
		inspector: inspector,
	}, nil
}

func (r *resolver) Resolve(ctx context.Context, env qdrantenv.Environment) (Layout, error) {
	if err := ctx.Err(); err != nil {
		return Layout{}, err
	}
	if !env.Installed {
		return Layout{}, ErrQdrantNotInstalled
	}
	if env.DistributionRoot == "" || env.BinaryPath == "" {
		return Layout{}, ErrQdrantNotInstalled
	}

	qdrantCfg := r.config.Providers.VectorStore.Qdrant
	paths := r.host.Paths()
	physicalRoots := resourceuri.PhysicalRootsFromRuntimePaths(paths)
	physicalResolver, err := resourceuri.NewPhysicalResolver(physicalRoots)
	if err != nil {
		return Layout{}, newResourceRootUnavailable("physical", err.Error())
	}

	configRoot, err := r.resolveConfigRoot(qdrantCfg.ConfigDir, paths, physicalResolver, env.DistributionRoot)
	if err != nil {
		return Layout{}, err
	}

	dataRoot, err := r.resolveDataRoot(qdrantCfg.DataDir, paths, physicalResolver, configRoot, env.DistributionRoot)
	if err != nil {
		return Layout{}, err
	}

	snapshotsDir, err := r.resolveSnapshotsDir(qdrantCfg.SnapshotsDir, dataRoot)
	if err != nil {
		return Layout{}, err
	}

	layout := Layout{
		DistributionRoot: env.DistributionRoot,
		BinaryPath:       env.BinaryPath,
		ConfigRoot:       configRoot,
		ConfigPath:       filepath.Join(configRoot, "config.yaml"),
		DataRoot:         dataRoot,
		StorageDir:       filepath.Join(dataRoot, "storage"),
		SnapshotsDir:     snapshotsDir,
		MigrationDir:     filepath.Join(dataRoot, "migration"),
	}

	if err := layout.Validate(); err != nil {
		return Layout{}, err
	}
	return layout, nil
}

func (r *resolver) resolveConfigRoot(explicit string, paths util.RuntimePaths, presolver *resourceuri.PhysicalResolver, distRoot string) (string, error) {
	var root string
	if explicit == "" {
		uri := resourceuri.MustParse("amitia://config/providers/qdrant")
		resolved, err := presolver.Resolve(uri)
		if err != nil {
			return "", newResourceRootUnavailable("config/providers/qdrant", err.Error())
		}
		root = resolved.LocalPath
	} else {
		cleaned := strings.TrimSpace(explicit)
		if filepath.IsAbs(cleaned) {
			root = filepath.Clean(cleaned)
		} else {
			root = filepath.Join(paths.ConfigDir, cleaned)
		}
	}
	if containsPath(distRoot, root) || containsPath(root, distRoot) {
		return "", newPathOverlap(distRoot, root)
	}
	return root, nil
}

func (r *resolver) resolveDataRoot(explicit string, paths util.RuntimePaths, presolver *resourceuri.PhysicalResolver, configRoot, distRoot string) (string, error) {
	var root string
	if explicit == "" {
		uri := resourceuri.MustParse("amitia://data/providers/qdrant")
		resolved, err := presolver.Resolve(uri)
		if err != nil {
			return "", newResourceRootUnavailable("data/providers/qdrant", err.Error())
		}
		root = resolved.LocalPath
	} else {
		cleaned := strings.TrimSpace(explicit)
		if filepath.IsAbs(cleaned) {
			root = filepath.Clean(cleaned)
		} else {
			root = filepath.Join(paths.DataDir, cleaned)
		}
	}
	if containsPath(distRoot, root) || containsPath(root, distRoot) {
		return "", newPathOverlap(distRoot, root)
	}
	if containsPath(configRoot, root) || containsPath(root, configRoot) {
		return "", newPathOverlap(configRoot, root)
	}
	return root, nil
}

func (r *resolver) resolveSnapshotsDir(explicit, dataRoot string) (string, error) {
	if explicit == "" {
		return filepath.Join(dataRoot, "snapshots"), nil
	}
	cleaned := strings.TrimSpace(explicit)
	var result string
	if filepath.IsAbs(cleaned) {
		result = filepath.Clean(cleaned)
	} else {
		result = filepath.Join(dataRoot, cleaned)
	}
	if !containsPath(dataRoot, result) {
		return "", newInvalidLayout("snapshots dir must be within data root")
	}
	return result, nil
}
