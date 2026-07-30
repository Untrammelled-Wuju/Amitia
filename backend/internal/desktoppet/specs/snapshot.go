// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type Snapshotter interface {
	Freeze(spec contracts.ActionSpec, frozenAt string) (contracts.ActionSpecSnapshot, error)
	Verify(snapshot contracts.ActionSpecSnapshot) error
}

type snapshotter struct{}

func NewSnapshotter() Snapshotter {
	return &snapshotter{}
}

func (s *snapshotter) Freeze(spec contracts.ActionSpec, frozenAt string) (contracts.ActionSpecSnapshot, error) {
	normalized := contracts.NormalizeSpec(spec)
	data, err := json.Marshal(normalized)
	if err != nil {
		return contracts.ActionSpecSnapshot{}, fmt.Errorf("marshal spec: %w", err)
	}
	jsonStr := string(data)
	return contracts.ActionSpecSnapshot{
		Spec:     normalized,
		JSON:     jsonStr,
		SHA256:   sha256Hex(jsonStr),
		FrozenAt: frozenAt,
	}, nil
}

func (s *snapshotter) Verify(snapshot contracts.ActionSpecSnapshot) error {
	if snapshot.JSON == "" {
		return fmt.Errorf("snapshot json is empty")
	}
	if snapshot.SHA256 == "" {
		return fmt.Errorf("snapshot sha256 is empty")
	}

	var spec contracts.ActionSpec
	if err := json.Unmarshal([]byte(snapshot.JSON), &spec); err != nil {
		return fmt.Errorf("unmarshal snapshot json: %w", err)
	}

	computed := sha256Hex(snapshot.JSON)
	if computed != snapshot.SHA256 {
		return fmt.Errorf("sha256 mismatch: expected %s got %s", snapshot.SHA256, computed)
	}

	return nil
}

func FreezeMany(specs []contracts.ActionSpec, frozenAt string) ([]contracts.ActionSpecSnapshot, error) {
	s := NewSnapshotter()
	snapshots := make([]contracts.ActionSpecSnapshot, 0, len(specs))
	for _, spec := range specs {
		snap, err := s.Freeze(spec, frozenAt)
		if err != nil {
			return nil, fmt.Errorf("freeze %s: %w", spec.Identity.Key, err)
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

func SnapshotFromJSON(jsonStr string, frozenAt string) (contracts.ActionSpecSnapshot, error) {
	var spec contracts.ActionSpec
	if err := json.Unmarshal([]byte(jsonStr), &spec); err != nil {
		return contracts.ActionSpecSnapshot{}, fmt.Errorf("unmarshal json: %w", err)
	}
	return contracts.ActionSpecSnapshot{
		Spec:     spec,
		JSON:     jsonStr,
		SHA256:   sha256Hex(jsonStr),
		FrozenAt: frozenAt,
	}, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func HashSpec(spec contracts.ActionSpec) string {
	normalized := contracts.NormalizeSpec(spec)
	b, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return sha256Hex(string(b))
}
