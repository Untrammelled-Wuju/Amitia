package search

import (
	"context"
	"strings"
)

type providerCredentialContextKey struct{}
type engineCredentialContextKey struct{}

type engineCredentialMap map[string]string

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

func ContextWithEngineCredential(ctx context.Context, engineID, credential string) context.Context {
	if ctx == nil || credential == "" {
		return ctx
	}
	engineID = normalizeEngineCredentialID(engineID)
	if engineID == "" {
		return ctx
	}
	values, _ := ctx.Value(engineCredentialContextKey{}).(engineCredentialMap)
	cloned := make(engineCredentialMap, len(values)+1)
	for key, value := range values {
		cloned[key] = value
	}
	cloned[engineID] = credential
	return context.WithValue(ctx, engineCredentialContextKey{}, cloned)
}

func EngineCredentialFromContext(ctx context.Context, engineID string) string {
	if ctx == nil {
		return ""
	}
	values, _ := ctx.Value(engineCredentialContextKey{}).(engineCredentialMap)
	return values[normalizeEngineCredentialID(engineID)]
}

func normalizeEngineCredentialID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
