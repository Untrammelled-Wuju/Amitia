// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build !linux && !windows

package qdrantprocess

import (
	"context"
	"time"
)

type unsupportedTerminator struct{}

func newPlatformTerminator() ProcessTerminator {
	return unsupportedTerminator{}
}

func (unsupportedTerminator) Terminate(ctx context.Context, id ProcessIdentity, gracePeriod time.Duration) (TerminationResult, error) {
	return "", ErrOrphanTerminationUnsupported
}
