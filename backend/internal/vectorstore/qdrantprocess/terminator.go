// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"context"
	"time"
)

type ProcessTerminator interface {
	Terminate(ctx context.Context, id ProcessIdentity, gracePeriod time.Duration) (TerminationResult, error)
}

func NewProcessTerminator() ProcessTerminator {
	return newPlatformTerminator()
}
