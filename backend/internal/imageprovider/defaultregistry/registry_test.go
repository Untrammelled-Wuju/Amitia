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
	for _, alias := range []string{"dall-e", "dashscope", "stability_ai", "zhipu"} {
		if descriptor, ok := registry.Describe(alias); ok {
			t.Fatalf("legacy alias %s remained registered: %#v", alias, descriptor)
		}
	}
}
