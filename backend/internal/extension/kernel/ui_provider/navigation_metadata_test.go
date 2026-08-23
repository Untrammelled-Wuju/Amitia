package ui_provider

import (
	"strings"
	"testing"
)

func TestValidateProviderMetadataRouteRegistryNavigationMustReferenceDeclaredRoute(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityRouteRegistry,
		Metadata: map[string]any{
			"routes": []any{
				map[string]any{"id": "main", "path": "/games/minecraft", "providerId": "minecraft.page"},
			},
			"navigationItems": []any{
				map[string]any{"id": "main", "label": "Minecraft", "route": "/games/other"},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "references undeclared route") {
		t.Fatalf("expected undeclared route error, got %v", err)
	}
}

func TestValidateProviderMetadataRejectsDuplicateRoutePaths(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityRouteRegistry,
		Metadata: map[string]any{
			"routes": []any{
				map[string]any{"id": "a", "path": "/games/shared", "providerId": "page.a"},
				map[string]any{"id": "b", "path": "/games/shared", "providerId": "page.b"},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "duplicate route.registry path") {
		t.Fatalf("expected duplicate path error, got %v", err)
	}
}

func TestValidateProviderMetadataAcceptsWellFormedNavigation(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityRouteRegistry,
		Metadata: map[string]any{
			"routes": []any{
				map[string]any{
					"id": "main", "path": "/games/minecraft", "providerId": "minecraft.page",
					"capability": "page.provider", "priority": float64(100), "title": "Minecraft",
				},
			},
			"navigationItems": []any{
				map[string]any{
					"id": "main", "label": "Minecraft", "route": "/games/minecraft", "order": float64(100),
					"mobile": true, "panel": "main", "match": []any{"/games/minecraft"},
					"routePrefixes": []any{"/games/minecraft"}, "icon": "extension", "group": "games",
				},
			},
		},
	}
	if err := validateProviderMetadata(def); err != nil {
		t.Fatalf("expected metadata to validate, got %v", err)
	}
}

func TestValidateProviderMetadataRejectsMalformedNavigationOptionalFields(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityAppNavigation,
		Metadata: map[string]any{
			"navigationItems": []any{
				map[string]any{"id": "main", "label": "Minecraft", "route": "/games/minecraft", "mobile": "yes"},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "mobile must be a boolean") {
		t.Fatalf("expected mobile type error, got %v", err)
	}
}

func TestValidateProviderMetadataRejectsRootCatchAllRoute(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityRouteRegistry,
		Metadata: map[string]any{
			"routes": []any{
				map[string]any{"id": "catch-all", "path": "/:pathMatch(.*)*", "providerId": "plugin.page"},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "safe extension path") {
		t.Fatalf("expected unsafe extension path error, got %v", err)
	}
}

func TestValidateProviderMetadataRejectsParameterizedNavigationTarget(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityAppNavigation,
		Metadata: map[string]any{
			"navigationItems": []any{
				map[string]any{"id": "detail", "label": "Detail", "route": "/games/:id"},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "concrete absolute route") {
		t.Fatalf("expected concrete navigation route error, got %v", err)
	}
}

func TestValidateProviderMetadataRejectsFractionalOrdering(t *testing.T) {
	def := ProviderDefinition{
		Capability: CapabilityAppNavigation,
		Metadata: map[string]any{
			"navigationItems": []any{
				map[string]any{"id": "main", "label": "Minecraft", "route": "/games/minecraft", "order": float64(1.5)},
			},
		},
	}
	if err := validateProviderMetadata(def); err == nil || !strings.Contains(err.Error(), "finite integer") {
		t.Fatalf("expected integer ordering error, got %v", err)
	}
}
