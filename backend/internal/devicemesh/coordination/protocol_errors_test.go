package coordination

import (
	"errors"
	"fmt"
	"testing"
)

func TestOwnedProtocolErrorsRetainPermanentRejectionIdentity(t *testing.T) {
	for _, entry := range protocolErrors {
		code := ProtocolErrorCode(fmt.Errorf("source: %w", entry.err))
		if code != entry.code || !errors.Is(ErrorFromProtocol(code), entry.err) {
			t.Fatalf("lost typed owner error: %s", entry.code)
		}
	}
	if ErrorFromProtocol("device_execution_unknown") != nil || ProtocolErrorCode(errors.New("connection unavailable")) != "" {
		t.Fatal("unknown execution or connection loss classified as acknowledged rejection")
	}
}
