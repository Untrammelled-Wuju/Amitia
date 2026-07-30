// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/specs"
)

func buildPresets(_ []string) []PresetResponse {
	builtin := specs.BuiltinPresets()
	out := make([]PresetResponse, 0, len(builtin))
	for _, p := range builtin {
		out = append(out, presetToResponse(p))
	}
	return out
}

func presetToResponse(p contracts.ActionPreset) PresetResponse {
	return PresetResponse{
		Key:         p.Key,
		Name:        p.Name,
		Description: p.Description,
		ActionKeys:  p.ActionKeys,
	}
}
