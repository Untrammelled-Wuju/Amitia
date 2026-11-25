package plugincontract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func baseDigest() ContractDigest {
	return ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       "media.transcribe",
		Version:         "1.2.0",
		Placement:       ServicePlacementCore,
		Scopes:          []ServiceScope{ServiceScopeGlobal, ServiceScopeConversation},
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"audio":{"type":"string"},"lang":{"type":"string"}}}`),
		OutputSchema:    json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
	}
}

func TestContractHashIgnoresPropertyOrder(t *testing.T) {
	left := baseDigest()
	right := baseDigest()
	right.InputSchema = json.RawMessage(`{"properties":{"lang":{"type":"string"},"audio":{"type":"string"}},"type":"object"}`)
	leftHash, err := CanonicalContractHash(left)
	if err != nil {
		t.Fatalf("hash left: %v", err)
	}
	rightHash, err := CanonicalContractHash(right)
	if err != nil {
		t.Fatalf("hash right: %v", err)
	}
	if leftHash != rightHash {
		t.Fatalf("same semantics with different property order must hash equal: %s vs %s", leftHash, rightHash)
	}
	if !hashPattern.MatchString(leftHash) {
		t.Fatalf("hash must match existing pattern, got %q", leftHash)
	}
	if err := ValidateContractHash(leftHash); err != nil {
		t.Fatalf("produced hash must pass ValidateContractHash: %v", err)
	}
}

func TestContractHashIgnoresWhitespaceAndScopeOrder(t *testing.T) {
	left := baseDigest()
	right := baseDigest()
	right.InputSchema = json.RawMessage("{\n  \"type\": \"object\",\n  \"properties\": {\"lang\": {\"type\": \"string\"}, \"audio\": {\"type\": \"string\"}}\n}")
	right.Scopes = []ServiceScope{ServiceScopeConversation, ServiceScopeGlobal, ServiceScopeConversation}
	leftHash, err := CanonicalContractHash(left)
	if err != nil {
		t.Fatalf("hash left: %v", err)
	}
	rightHash, err := CanonicalContractHash(right)
	if err != nil {
		t.Fatalf("hash right: %v", err)
	}
	if leftHash != rightHash {
		t.Fatalf("whitespace/scope order must not change hash: %s vs %s", leftHash, rightHash)
	}
	payload, err := CanonicalContractBytes(right)
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	if strings.Count(string(payload), `"global"`) != 1 {
		t.Fatalf("scopes must be deduplicated: %s", string(payload))
	}
}

func TestContractHashChangesWithSemantics(t *testing.T) {
	left := baseDigest()
	right := baseDigest()
	right.InputSchema = json.RawMessage(`{"type":"object","properties":{"audio":{"type":"string"}}}`)
	leftHash, err := CanonicalContractHash(left)
	if err != nil {
		t.Fatalf("hash left: %v", err)
	}
	rightHash, err := CanonicalContractHash(right)
	if err != nil {
		t.Fatalf("hash right: %v", err)
	}
	if leftHash == rightHash {
		t.Fatalf("removing a property must change the hash")
	}
}

func TestContractHashRejectsWrongProtocolVersion(t *testing.T) {
	digest := baseDigest()
	digest.ProtocolVersion = ContractProtocolVersion + 1
	if _, err := CanonicalContractHash(digest); !errors.Is(err, ErrUnsupportedContractProtocol) {
		t.Fatalf("wrong protocol version must be rejected, got %v", err)
	}
	if err := ValidateContractProtocolVersion(ContractProtocolVersion); err != nil {
		t.Fatalf("current version must pass: %v", err)
	}
}

func TestContractHashRejectsWrongServiceIdentity(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ContractDigest)
	}{
		{"uppercase service id", func(d *ContractDigest) { d.ServiceID = "Media.Transcribe" }},
		{"empty service id", func(d *ContractDigest) { d.ServiceID = "" }},
		{"space in service id", func(d *ContractDigest) { d.ServiceID = "media transcribe" }},
		{"empty version", func(d *ContractDigest) { d.Version = " " }},
		{"unknown placement", func(d *ContractDigest) { d.Placement = ServicePlacement("edge") }},
		{"unknown scope", func(d *ContractDigest) { d.Scopes = []ServiceScope{"galaxy"} }},
	}
	for _, item := range cases {
		digest := baseDigest()
		item.mutate(&digest)
		if _, err := CanonicalContractHash(digest); err == nil {
			t.Fatalf("%s must be rejected", item.name)
		}
	}
}

func TestContractHashRejectsMalformedSchema(t *testing.T) {
	digest := baseDigest()
	digest.InputSchema = json.RawMessage(`{"type":"object"`)
	if _, err := CanonicalContractHash(digest); err == nil {
		t.Fatalf("broken schema must be rejected")
	}
	digest = baseDigest()
	digest.InputSchema = json.RawMessage(`{"type":"object"} {"extra":true}`)
	if _, err := CanonicalContractHash(digest); err == nil {
		t.Fatalf("trailing content must be rejected")
	}
}

func TestContractHashForOfferMatchesDigest(t *testing.T) {
	digest := baseDigest()
	offer := ServiceOffer{
		ServiceID: digest.ServiceID,
		Version:   digest.Version,
		Placement: digest.Placement,
		Scopes:    digest.Scopes,
	}
	direct, err := CanonicalContractHash(digest)
	if err != nil {
		t.Fatalf("hash digest: %v", err)
	}
	viaOffer, err := ContractHashForOffer(offer, digest.InputSchema, digest.OutputSchema)
	if err != nil {
		t.Fatalf("hash offer: %v", err)
	}
	if direct != viaOffer {
		t.Fatalf("offer shortcut must equal digest hash: %s vs %s", direct, viaOffer)
	}
}

func TestValidateContractHash(t *testing.T) {
	if err := ValidateContractHash(strings.Repeat("a", 64)); err != nil {
		t.Fatalf("64 hex must pass: %v", err)
	}
	if err := ValidateContractHash(" " + strings.Repeat("A", 64) + " "); err != nil {
		t.Fatalf("surrounding whitespace must be tolerated: %v", err)
	}
	for _, raw := range []string{"", "abc", strings.Repeat("a", 63), strings.Repeat("z", 64)} {
		if err := ValidateContractHash(raw); err == nil {
			t.Fatalf("%q must be rejected", raw)
		}
	}
}
