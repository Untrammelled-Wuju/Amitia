package system

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/secret"
)

type searchCredentialVault struct {
	broker *secret.Broker
}

func newSearchCredentialVault(broker *secret.Broker) *searchCredentialVault {
	if broker == nil {
		return nil
	}
	return &searchCredentialVault{broker: broker}
}

func (v *searchCredentialVault) Store(ctx context.Context, namespace string, value []byte) (string, error) {
	if v == nil || v.broker == nil {
		return "", fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := v.broker.Store(ctx, namespace, value)
	if err != nil {
		return "", err
	}
	return ref.String(), nil
}

func (v *searchCredentialVault) Resolve(ctx context.Context, rawRef string) ([]byte, error) {
	if v == nil || v.broker == nil {
		return nil, fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := secret.ParseRef(rawRef)
	if err != nil {
		return nil, err
	}
	lease, err := v.broker.Issue(ctx, secret.LeaseRequest{
		Ref: ref, Purpose: "search-engine-admin", RuntimeInstanceID: "system-search-credentials", TTL: 30 * time.Second, MaxUses: 1,
	})
	if err != nil {
		return nil, err
	}
	return v.broker.Consume(ctx, lease.ID, secret.LeaseUseContext{RuntimeInstanceID: "system-search-credentials"})
}

func (v *searchCredentialVault) Delete(ctx context.Context, rawRef string) error {
	if v == nil || v.broker == nil {
		return fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := secret.ParseRef(rawRef)
	if err != nil {
		return err
	}
	return v.broker.Delete(ctx, ref)
}
