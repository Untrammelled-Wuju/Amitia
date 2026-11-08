package interaction

import (
	"encoding/json"
	"testing"
)

func TestOwnedMemoryNumberTreatsMissingOptionalFieldsAsZero(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage(""), json.RawMessage("null")} {
		value, err := ownedMemoryNumber(raw)
		if err != nil || value != 0 {
			t.Fatalf("optional memory number failed for %q: value=%v err=%v", raw, value, err)
		}
	}
	value, err := ownedMemoryNumber(json.RawMessage("0.75"))
	if err != nil || value != 0.75 {
		t.Fatalf("valid memory number lost: value=%v err=%v", value, err)
	}
}

func TestOwnedMemoryNumberReturnsMalformedFieldError(t *testing.T) {
	if _, err := ownedMemoryNumber(json.RawMessage(`"not-a-number"`)); err == nil {
		t.Fatal("malformed optional memory number was silently accepted")
	}
}
