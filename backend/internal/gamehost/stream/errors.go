package stream

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

func eventAdapterFailure() *domain.HostError {
	return domain.NewHostError(domain.ErrInternal, "stream: event adapter failure")
}

func stateStoreFailure() *domain.HostError {
	return domain.NewHostError(domain.ErrInternal, "stream: state store failure")
}
