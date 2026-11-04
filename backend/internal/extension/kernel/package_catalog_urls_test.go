package kernel

import (
	"reflect"
	"testing"
)

func TestExtensionCatalogURLsCombinesAndDeduplicatesSources(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_CATALOG_URL", "https://one.example/catalog")
	t.Setenv("AMITIA_EXTENSION_CATALOG_URLS", "https://two.example/catalog,https://one.example/catalog\nhttps://three.example/catalog")
	want := []string{
		"https://one.example/catalog",
		"https://two.example/catalog",
		"https://three.example/catalog",
	}
	if got := extensionCatalogURLs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog urls = %#v, want %#v", got, want)
	}
}
