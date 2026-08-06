// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
)

type ProcessInspector interface {
	Inspect(ctx context.Context, pid int) (ProcessIdentity, error)
	IsAlive(ctx context.Context, id ProcessIdentity) (bool, error)
}

func NewProcessInspector() ProcessInspector {
	return newPlatformInspector()
}
