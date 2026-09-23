package native_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
	nativeengines "github.com/u-ai/backend/internal/search/native/engines"
)

func TestLiveNativeSearchKinds(t *testing.T) {
	if os.Getenv("AMITIA_SEARCH_LIVE") != "1" {
		t.Skip("set AMITIA_SEARCH_LIVE=1 to run live search")
	}
	provider := native.NewProvider(nativeengines.DefaultRegistry(nil), true, 4*1024*1024)
	tests := []struct {
		name  string
		query string
		kind  search.SearchKind
	}{
		{name: "web", query: "amitia search", kind: search.SearchKindWeb},
		{name: "academic", query: "retrieval augmented generation", kind: search.SearchKindAcademic},
		{name: "code", query: "http client go", kind: search.SearchKindCode},
		{name: "news", query: "artificial intelligence", kind: search.SearchKindNews},
		{name: "image", query: "mount fuji", kind: search.SearchKindImage},
		{name: "video", query: "golang tutorial", kind: search.SearchKindVideo},
		{name: "places", query: "Shanghai", kind: search.SearchKindPlaces},
		{name: "product", query: "coca cola", kind: search.SearchKindProduct},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			response, err := provider.Search(ctx, search.SearchRequest{
				Query: test.query,
				Kind:  test.kind,
				Limit: 5,
			})
			require.NoError(t, err)
			require.NotEmpty(t, response.Results)
		})
	}
}

func TestLiveNativeExtendedKinds(t *testing.T) {
	if os.Getenv("AMITIA_SEARCH_LIVE") != "1" {
		t.Skip("set AMITIA_SEARCH_LIVE=1 to run live search")
	}
	tests := []struct {
		name  string
		query string
		kind  search.SearchKind
	}{
		{name: "software", query: "react", kind: search.SearchKindSoftware},
		{name: "music", query: "jazz", kind: search.SearchKindMusic},
		{name: "files", query: "climate research data", kind: search.SearchKindFiles},
		{name: "social", query: "golang", kind: search.SearchKindSocial},
		{name: "academic_extended", query: "quantum computing", kind: search.SearchKindAcademic},
		{name: "image_extended", query: "space telescope", kind: search.SearchKindImage},
		{name: "places_extended", query: "Tokyo", kind: search.SearchKindPlaces},
		{name: "translation", query: "hello to Chinese", kind: search.SearchKindTranslation},
		{name: "dictionary", query: "define serendipity", kind: search.SearchKindDictionary},
		{name: "weather", query: "weather in Shanghai", kind: search.SearchKindWeather},
		{name: "currency", query: "10 USD to CNY", kind: search.SearchKindCurrency},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := native.NewProvider(nativeengines.DefaultRegistry(nil), true, 4*1024*1024)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			response, err := provider.Search(ctx, search.SearchRequest{
				Query: test.query,
				Kind:  test.kind,
				Limit: 3,
			})
			if err != nil {
				t.Logf("kind=%s unavailable: %v", test.kind, err)
				return
			}
			t.Logf("kind=%s results=%d", test.kind, len(response.Results))
			require.NotEmpty(t, response.Results)
		})
	}
}
