package contracts

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HostServices interface {
	Invoke(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
		method string,
		payload []byte,
	) ([]byte, error)
}
