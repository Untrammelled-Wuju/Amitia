package storage

import (
	"path/filepath"
)

type PluginPaths struct {
	Root   string
	Data   string
	Cache  string
	Shared string
}

type RuntimePaths struct {
	Root     string
	Data     string
	Temp     string
	Cache    string
	Services string
}

type ServicePaths struct {
	Root  string
	Data  string
	Temp  string
	Cache string
}

type RuntimeDirectoryContext struct {
	Plugin  PluginPaths
	Runtime RuntimePaths
}

type ServiceDirectoryContext struct {
	Plugin  PluginPaths
	Runtime RuntimePaths
	Service ServicePaths
}

func (p PluginPaths) Contains(target string) (bool, error) {
	return pathWithinRoot(p.Root, target)
}

func (r RuntimePaths) Contains(target string) (bool, error) {
	return pathWithinRoot(r.Root, target)
}

func (s ServicePaths) Contains(target string) (bool, error) {
	return pathWithinRoot(s.Root, target)
}

func pathWithinRoot(root, target string) (bool, error) {
	if root == "" || target == "" {
		return false, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	absRoot = filepath.Clean(absRoot)

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	absTarget = filepath.Clean(absTarget)

	if absRoot == absTarget {
		return true, nil
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false, nil
	}
	if rel == "." {
		return true, nil
	}
	if rel == ".." {
		return false, nil
	}
	if len(rel) > 2 && rel[0] == '.' && rel[1] == '.' && (rel[2] == filepath.Separator || rel[2] == '/') {
		return false, nil
	}
	if len(rel) > 1 && rel[0] == '.' && (rel[1] == filepath.Separator || rel[1] == '/') {
		return true, nil
	}
	return true, nil
}
