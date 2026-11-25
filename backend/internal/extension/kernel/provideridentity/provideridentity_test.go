package provideridentity

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

const pinnedCatalogKey = "com.amitia/emote/emote-runtime/com.amitia/emote/ai"

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestCatalogProviderKeyIsStable(t *testing.T) {
	key := CatalogProviderKey("com.amitia/emote", "emote-runtime", "com.amitia/emote/ai")
	if key != pinnedCatalogKey {
		t.Fatalf("catalog key = %s", key)
	}
	if CatalogProviderKey(" com.amitia/emote ", " emote-runtime ", "com.amitia/emote/ai") != pinnedCatalogKey {
		t.Fatalf("catalog key module segments are not trimmed")
	}
	if CatalogProviderKey("a", "b", " svc.one ") != "a/b/ svc.one " {
		t.Fatalf("catalog key trimmed the service segment")
	}
	if CatalogProviderKey("", "", "svc.one") != "//svc.one" {
		t.Fatalf("catalog key shape changed for empty module")
	}
}

func TestExtensionProviderIDIsStable(t *testing.T) {
	withProvider := ExtensionProviderID("com.amitia/emote", "emote-runtime", "emote.ai", "com.amitia/emote/ai")
	withoutProvider := ExtensionProviderID("com.amitia/emote", "emote-runtime", "", "com.amitia/emote/ai")
	if withProvider == withoutProvider {
		t.Fatalf("provider key ignored in seed")
	}
	if withProvider != ExtensionProviderID("com.amitia/emote", "emote-runtime", "emote.ai", "com.amitia/emote/ai") {
		t.Fatalf("extension provider id is not deterministic")
	}
	if len(withProvider) != len("extprov_")+64 {
		t.Fatalf("extension provider id length = %d", len(withProvider))
	}
	if withProvider != "extprov_"+digest("com.amitia/emote|emote-runtime|emote.ai|com.amitia/emote/ai") {
		t.Fatalf("extension provider id = %s", withProvider)
	}
	if withoutProvider != "extprov_"+digest("com.amitia/emote|emote-runtime|com.amitia/emote/ai") {
		t.Fatalf("extension provider id without provider key = %s", withoutProvider)
	}
	if ExtensionProviderID("a", "b", "", "c") == ExtensionProviderID("a", "b", "c", "") {
		t.Fatalf("seed field boundaries collapsed")
	}
}
