// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import "path/filepath"

type Document struct {
	HTTPPort     int
	GRPCPort     int
	StoragePath  string
	SnapshotPath string
}

func (d Document) Validate() error {
	if d.HTTPPort < 1 || d.HTTPPort > 65535 {
		return newInvalidDocument("http port out of range")
	}
	if d.GRPCPort < 1 || d.GRPCPort > 65535 {
		return newInvalidDocument("grpc port out of range")
	}
	if d.HTTPPort == d.GRPCPort {
		return newInvalidDocument("http and grpc ports must be different")
	}
	if d.StoragePath == "" {
		return newInvalidDocument("storage path is empty")
	}
	if !filepath.IsAbs(d.StoragePath) {
		return newInvalidDocument("storage path is not absolute")
	}
	if d.SnapshotPath == "" {
		return newInvalidDocument("snapshot path is empty")
	}
	if !filepath.IsAbs(d.SnapshotPath) {
		return newInvalidDocument("snapshot path is not absolute")
	}
	if d.StoragePath == d.SnapshotPath {
		return newInvalidDocument("storage and snapshot paths must be different")
	}
	return nil
}
