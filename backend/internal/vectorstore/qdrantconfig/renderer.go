// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Renderer interface {
	Render(Document) ([]byte, error)
}

type yamlRenderer struct{}

func NewRenderer() Renderer {
	return &yamlRenderer{}
}

func (r *yamlRenderer) Render(doc Document) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, newRenderFailed(err)
	}

	httpPortJSON, err := json.Marshal(doc.HTTPPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	grpcPortJSON, err := json.Marshal(doc.GRPCPort)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	storagePathJSON, err := json.Marshal(doc.StoragePath)
	if err != nil {
		return nil, newRenderFailed(err)
	}
	snapshotPathJSON, err := json.Marshal(doc.SnapshotPath)
	if err != nil {
		return nil, newRenderFailed(err)
	}

	var sb strings.Builder
	sb.WriteString("service:\n")
	sb.WriteString(fmt.Sprintf("  http_port: %s\n", string(httpPortJSON)))
	sb.WriteString(fmt.Sprintf("  grpc_port: %s\n", string(grpcPortJSON)))
	sb.WriteString("storage:\n")
	sb.WriteString(fmt.Sprintf("  storage_path: %s\n", string(storagePathJSON)))
	sb.WriteString(fmt.Sprintf("  snapshots_path: %s\n", string(snapshotPathJSON)))

	return []byte(sb.String()), nil
}
