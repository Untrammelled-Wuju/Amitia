package search

import "context"

const SearchPurpose = "search.api"

type LeaseIssuer interface {
	Issue(ctx context.Context, ref string, purpose string) (leaseID string, err error)
	Consume(ctx context.Context, leaseID string) (credential string, err error)
}

type SecretBridge struct {
	issuer LeaseIssuer
}

func NewSecretBridge(issuer LeaseIssuer) *SecretBridge {
	if issuer == nil {
		return nil
	}
	return &SecretBridge{issuer: issuer}
}

func (b *SecretBridge) Resolve(ctx context.Context, providerID, invocation string, credentialRef string) (string, func(), error) {
	if b == nil || credentialRef == "" {
		return "", func() {}, nil
	}
	leaseID, err := b.issuer.Issue(ctx, credentialRef, SearchPurpose)
	if err != nil {
		return "", func() {}, err
	}
	cred, cerr := b.issuer.Consume(ctx, leaseID)
	if cerr != nil {
		return "", func() {}, cerr
	}
	return cred, func() {}, nil
}
