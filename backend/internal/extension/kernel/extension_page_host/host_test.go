package extension_page_host

import (
	"context"
	"errors"
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

func TestEnsurePageIsIdempotentAndRejectsDifferentRegistration(t *testing.T) {
	host := NewPageHost(NewPageRegistry(), NewSessionManager())
	def := NewExtensionPageDefinition(PageRegistrationInput{
		PageID: "page", ExtensionID: "com.example/extension", ModuleID: "module",
		ContributionID: "page", Generation: 7, ContractVersion: 1,
		EntryKind: PageKindWeb, EntryPath: "index.html",
		Title: LocalizedText{Default: "Page"}, Description: LocalizedText{Default: "Page"},
	})
	if err := host.EnsurePage(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	if err := host.EnsurePage(context.Background(), def); err != nil {
		t.Fatalf("identical page must be idempotent: %v", err)
	}
	changed := NewExtensionPageDefinition(PageRegistrationInput{
		PageID: "page", ExtensionID: "com.example/extension", ModuleID: "module",
		ContributionID: "page", Generation: 8, ContractVersion: 1,
		EntryKind: PageKindWeb, EntryPath: "changed.html",
		Title: LocalizedText{Default: "Page"}, Description: LocalizedText{Default: "Page"},
	})
	if err := host.EnsurePage(context.Background(), changed); !errors.Is(err, ErrPageExists) {
		t.Fatalf("different page registration error=%v, want ErrPageExists", err)
	}
}
