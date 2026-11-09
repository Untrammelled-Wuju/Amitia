package plugincontract

import "encoding/json"

var builtinAdapterSchema = json.RawMessage(`{"type":"object"}`)

func BuiltinAdapterDescriptors() []AdapterDescriptor {
	return []AdapterDescriptor{
		builtinAdapter("tool", []string{"tool"}),
		builtinAdapter("agent_skill", []string{"agent_skill"}),
		builtinAdapter("workflow", []string{"workflow"}),
		builtinAdapter("mcp_server", []string{"mcp_server"}),
		builtinAdapter("hook", []string{"hook"}),
		builtinAdapter("event", []string{"event_subscription"}),
		builtinAdapter("schedule", []string{"schedule"}),
		builtinAdapter("ui", []string{"ui_page", "ui_panel", "ui_chat", "ui_context_action", "ui_desktop"}),
		builtinAdapter("ui_provider", []string{"ui_provider"}),
		builtinAdapter("ui_slot", []string{"ui_slot"}),
		builtinAdapter("background", []string{"background_task"}),
		builtinAdapter("provider", []string{"provider"}),
		builtinAdapter("game", []string{"gamex"}),
		builtinAdapter("desktop_pet", []string{"petx"}),
	}
}

func NewBuiltinAdapterRegistry() (*AdapterRegistry, error) {
	registry := NewAdapterRegistry(AdapterRegistrationPolicy{
		MinTrust:      TrustUserTrusted,
		AllowExternal: true,
	})
	for _, descriptor := range BuiltinAdapterDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func builtinAdapter(id string, kinds []string) AdapterDescriptor {
	return AdapterDescriptor{
		ID:                 id,
		ContractVersion:    1,
		SupportedKinds:     kinds,
		ContributionSchema: append(json.RawMessage(nil), builtinAdapterSchema...),
		Trust:              TrustSystem,
	}
}
