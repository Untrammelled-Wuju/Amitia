package main

import (
	"testing"

	"github.com/u-ai/backend/internal/chat/localmodel"
)

func TestLocalModelProductionRegistrationIncludesMNNAndLlamaCpp(t *testing.T) {
	for _, provider := range []string{"mnn", "llama_cpp"} {
		descriptor, ok := localmodel.Descriptor(provider)
		if !ok {
			t.Fatalf("provider %s is not registered", provider)
		}
		if descriptor.RuntimeKind != "embedded" || len(descriptor.ConfigFields) == 0 {
			t.Fatalf("provider %s descriptor is incomplete: %#v", provider, descriptor)
		}
	}
}
