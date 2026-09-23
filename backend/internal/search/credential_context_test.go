package search

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngineCredentialsAreScopedByEngineID(t *testing.T) {
	ctx := ContextWithEngineCredential(context.Background(), "Brave_API", "brave-key")
	require.Equal(t, "brave-key", EngineCredentialFromContext(ctx, "brave_api"))
	require.Empty(t, EngineCredentialFromContext(ctx, "github"))
}
