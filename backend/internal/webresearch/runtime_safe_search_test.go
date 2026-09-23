package webresearch

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
)

func TestParseSafeSearchPreservesUnspecifiedValue(t *testing.T) {
	require.Empty(t, parseSafeSearch(""))
	require.Equal(t, search.SafeSearchStrict, parseSafeSearch(" strict "))
	require.Equal(t, search.SafeSearchModerate, parseSafeSearch("invalid"))
}
