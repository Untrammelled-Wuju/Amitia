package contracts

import "context"

type Transport interface {
	Close(ctx context.Context) error
}
