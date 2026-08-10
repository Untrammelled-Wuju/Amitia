package secret

import "testing"

func TestParseRef_Canonical(t *testing.T) {
	ref, err := ParseRef("secret://mcp/server-1/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.String() != "secret://mcp/server-1/abc123" {
		t.Fatalf("unexpected ref: %s", ref)
	}
	if !ref.Valid() {
		t.Fatal("expected valid")
	}
}

func TestParseRef_Legacy(t *testing.T) {
	ref, err := ParseRef("mcp-secret://legacy/old-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ref.Valid() {
		t.Fatal("expected valid after conversion")
	}
	if !ref.IsLegacy() {
		t.Fatal("expected legacy detection")
	}
}

func TestParseRef_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"http://example.com",
		"secret://",
		"secret://noslash",
		"secret://trailing/",
	}
	for _, raw := range invalid {
		if _, err := ParseRef(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestSecretRef_StringDoesNotResolve(t *testing.T) {
	ref := SecretRef("secret://ns/id")
	s := ref.String()
	if s != "secret://ns/id" {
		t.Fatalf("expected canonical form, got %s", s)
	}
}

func TestReferenceMarshalUnmarshal(t *testing.T) {
	ref := Reference{Ref: SecretRef("secret://ns/id")}
	data, err := ref.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"$secret":"secret://ns/id"}` {
		t.Fatalf("unexpected json: %s", data)
	}
	var decoded Reference
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Ref != ref.Ref {
		t.Fatalf("expected %s, got %s", ref.Ref, decoded.Ref)
	}
}

func TestFingerprintRef(t *testing.T) {
	fp := FingerprintRef(SecretRef("secret://mcp/key/123"))
	if len(fp) != len("secret-ref:")+16 {
		t.Fatalf("unexpected fingerprint length: %s", fp)
	}
	if fp2 := FingerprintRef(SecretRef("secret://mcp/key/123")); fp2 != fp {
		t.Fatal("expected deterministic fingerprint")
	}
	if fp3 := FingerprintRef(SecretRef("secret://mcp/key/999")); fp3 == fp {
		t.Fatal("expected different fingerprint for different ref")
	}
}
