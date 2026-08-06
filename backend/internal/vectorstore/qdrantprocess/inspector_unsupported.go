// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build !linux && !windows

package qdrantprocess

import (
	"context"
)

type unsupportedInspector struct{}

func newPlatformInspector() ProcessInspector {
	return unsupportedInspector{}
}

func (unsupportedInspector) Inspect(ctx context.Context, pid int) (ProcessIdentity, error) {
	return ProcessIdentity{}, ErrProcessInspectionUnsupported
}

func (unsupportedInspector) IsAlive(ctx context.Context, id ProcessIdentity) (bool, error) {
	return false, ErrProcessInspectionUnsupported
}
