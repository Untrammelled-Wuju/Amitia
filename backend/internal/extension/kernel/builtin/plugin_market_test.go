package builtin

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/pluginmarket"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
)

func TestPluginMarketIsPreinstalledOrdinaryPlugin(t *testing.T) {
	definition := BuildPluginMarketExtension("1.0.0")
	if definition.Extension.ID != pluginmarket.ExtensionID || !definition.ShouldEnable() || !definition.DisableAllowed {
		t.Fatalf("plugin market bootstrap definition invalid: %+v", definition)
	}
	contributions := definition.Extension.AllContributions()
	adapters := map[domain.ContributionAdapter]bool{}
	for _, contribution := range contributions {
		adapters[contribution.EffectiveAdapter()] = true
		if contribution.Kind == domain.ContributionKind("com.amitia.plugin-market/page") {
			if contribution.Adapter != domain.ContributionAdapterUI || contribution.ContractVersion != 1 || len(contribution.Schema) == 0 {
				t.Fatalf("custom market contribution contract invalid: %+v", contribution)
			}
			schema, _ := contribution.Definition["schema"].(map[string]any)
			if schema == nil || schema["type"] != "document" {
				t.Fatalf("inline schema missing: %+v", contribution.Definition)
			}
		}
	}
	if !adapters[domain.ContributionAdapterTool] || !adapters[domain.ContributionAdapterUI] || !adapters[domain.ContributionAdapterUIProvider] {
		t.Fatalf("market plugin does not use ordinary contribution adapters: %+v", adapters)
	}
}

func TestPluginMarketSchemaIsValidJSON(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(pluginMarketSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["schemaVersion"] != "schema-ui/1" || schema["type"] != "document" {
		t.Fatalf("unexpected schema document: %+v", schema)
	}
	registry := schema_ui.NewSchemaRegistry(schema_ui.NewValidator(), nil)
	if err := registry.LoadFromBytes("com.amitia.builtin.plugin-market", "plugin-market-page", pluginMarketSchema); err != nil {
		t.Fatal(err)
	}
}
