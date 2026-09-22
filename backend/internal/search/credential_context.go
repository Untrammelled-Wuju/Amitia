package search

import "context"

type providerCredentialContextKey struct{}

func ContextWithProviderCredential(ctx context.Context, credential string) context.Context {
	if credential == "" {
		return ctx
	}
	return context.WithValue(ctx, providerCredentialContextKey{}, credential)
}

func ProviderCredentialFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(providerCredentialContextKey{}).(string)
	return value
}
