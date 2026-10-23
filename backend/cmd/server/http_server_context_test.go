// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"testing"
)

func TestHTTPServerBaseContextFollowsApplicationCancellation(t *testing.T) {
	appCtx, cancel := context.WithCancel(context.Background())
	srv := newHTTPServer("127.0.0.1:0", appCtx)
	baseCtx := srv.BaseContext(nil)
	cancel()
	select {
	case <-baseCtx.Done():
	default:
		t.Fatal("HTTP base context remained active after application cancellation")
	}
}
