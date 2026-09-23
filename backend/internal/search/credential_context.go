package search

import (
	"context"
	"strings"
)

type providerCredentialContextKey struct{}
type engineCredentialContextKey struct{}
type engineCredentialSourceContextKey struct{}

type engineCredentialMap map[string]string

// EngineCredentialSource resolves native-engine credentials lazily. The search
// service attaches the source to the request context; the native planner checks
// availability before scheduling an engine, and only the selected engine secret
// is resolved for the duration of that engine call.
type EngineCredentialSource interface {
	Available(ctx context.Context, engineID string) bool
	Resolve(ctx context.Context, engineID string) (credential string, release func(), err error)
}

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

func ContextWithEngineCredentialSource(ctx context.Context, source EngineCredentialSource) context.Context {
	if ctx == nil || source == nil {
		return ctx
	}
	return context.WithValue(ctx, engineCredentialSourceContextKey{}, source)
}

func EngineCredentialSourceFromContext(ctx context.Context) EngineCredentialSource {
	if ctx == nil {
		return nil
	}
	source, _ := ctx.Value(engineCredentialSourceContextKey{}).(EngineCredentialSource)
	return source
}

func EngineCredentialAvailable(ctx context.Context, engineID string) bool {
	if EngineCredentialFromContext(ctx, engineID) != "" {
		return true
	}
	source := EngineCredentialSourceFromContext(ctx)
	return source != nil && source.Available(ctx, normalizeEngineCredentialID(engineID))
}

func ResolveEngineCredential(ctx context.Context, engineID string) (string, func(), error) {
	if credential := EngineCredentialFromContext(ctx, engineID); credential != "" {
		return credential, func() {}, nil
	}
	source := EngineCredentialSourceFromContext(ctx)
	if source == nil {
		return "", func() {}, nil
	}
	return source.Resolve(ctx, normalizeEngineCredentialID(engineID))
}

func normalizeEngineCredentialID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
