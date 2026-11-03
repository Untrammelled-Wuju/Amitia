package defaultregistry

import "testing"

func TestDefaultRegistryContainsAllConfiguredProviders(t *testing.T) {
	registry, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"seedream", "openai", "stability", "tongyi", "cogview", "siliconflow"} {
		if _, ok := registry.Resolve(name); !ok {
			t.Fatalf("provider is not registered: %s", name)
		}
	}
	for alias, expected := range map[string]string{
		"dall-e":       "openai",
		"dashscope":    "tongyi",
		"stability_ai": "stability",
		"zhipu":        "cogview",
	} {
		descriptor, ok := registry.Describe(alias)
		if !ok || descriptor.Name != expected {
			t.Fatalf("alias %s resolved to %#v", alias, descriptor)
		}
	}
}
