package extension_page_host

import (
	"context"
	"testing"
)

func TestOpenPageReturnsContributionGeneration(t *testing.T) {
	registry := NewPageRegistry()
	def := NewExtensionPageDefinition(PageRegistrationInput{
		PageID:          "page",
		ExtensionID:     "com.example/extension",
		ModuleID:        "module",
		ContributionID:  "page",
		Generation:      7,
		ContractVersion: 1,
		EntryKind:       PageKindWeb,
		EntryPath:       "index.html",
		Title:           LocalizedText{Default: "Page"},
		Description:     LocalizedText{Default: "Page"},
	})
	if err := registry.Register(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	host := NewPageHost(registry, NewSessionManager())
	result, err := host.OpenPage(context.Background(), OpenPageRequest{
		ExtensionID: "com.example/extension",
		PageID:      "page",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Generation != 7 {
		t.Fatalf("generation = %d, want 7", result.Generation)
	}
}
