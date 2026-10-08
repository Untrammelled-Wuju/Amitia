package secret

import (
	"context"

	"github.com/google/uuid"
)

type ReferenceStore interface {
	PutReference(context.Context, string, []byte) error
}

func (b *Broker) ReserveReference(namespace string) (SecretRef, error) {
	if b == nil || b.store == nil {
		return "", ErrSecretStoreUnavailable
	}
	return ParseRef(schemeCanonical + sanitizeNamespace(namespace) + "/" + uuid.NewString())
}

func (b *Broker) StoreReference(ctx context.Context, ref SecretRef, value []byte) error {
	if b == nil || b.store == nil {
		return ErrSecretStoreUnavailable
	}
	if !ref.Valid() || ref != ref.Canonical() || len(value) == 0 {
		return ErrSecretRefInvalid
	}
	store, ok := b.store.(ReferenceStore)
	if !ok {
		return ErrSecretStoreUnavailable
	}
	if err := store.PutReference(ctx, ref.String(), value); err != nil {
		return err
	}
	b.redactor.Add(value)
	return nil
}
