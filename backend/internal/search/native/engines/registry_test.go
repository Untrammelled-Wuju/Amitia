package engines

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
)

func TestDefaultRegistryCoversCoreKinds(t *testing.T) {
	registry := DefaultRegistry(nil)
	require.GreaterOrEqual(t, registry.Count(), 84)

	ids := map[string]bool{}
	for _, engine := range registry.All() {
		ids[engine.Descriptor().ID] = true
	}
	require.True(t, ids["duckduckgo_html"])
	require.False(t, ids["brave_api"])
	require.False(t, ids["exa"])
	require.False(t, ids["tavily"])
	covered := make(map[search.SearchKind]bool)
	for _, engine := range registry.All() {
		caps := engine.Descriptor().Capabilities
		if caps.GeneralWeb {
			covered[search.SearchKindWeb] = true
		}
		for _, kind := range caps.SearchKinds {
			covered[kind] = true
		}
	}
	for _, kind := range []search.SearchKind{
		search.SearchKindWeb,
		search.SearchKindNews,
		search.SearchKindAcademic,
		search.SearchKindCode,
		search.SearchKindImage,
		search.SearchKindVideo,
		search.SearchKindPlaces,
		search.SearchKindProduct,
		search.SearchKindMusic,
		search.SearchKindFiles,
		search.SearchKindSocial,
		search.SearchKindSoftware,
		search.SearchKindTranslation,
		search.SearchKindDictionary,
		search.SearchKindWeather,
		search.SearchKindCurrency,
	} {
		require.Truef(t, covered[kind], "missing engine coverage for %s", kind)
	}
}
