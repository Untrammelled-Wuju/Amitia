package iosnative

import (
	"context"

	"github.com/u-ai/backend/internal/nativebridge"
)

type Handler interface {
	Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response
}
