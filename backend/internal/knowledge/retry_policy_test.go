package knowledge

import (
	"testing"
	"time"
)

func TestOutboxRetryDelayUsesConfiguredBounds(t *testing.T) {
	settings := runtimeSettings{OutboxRetryBase: 2 * time.Second, OutboxRetryMax: 10 * time.Second}
	if got := settings.outboxRetryDelay(0); got != 2*time.Second {
		t.Fatalf("retry 0 = %v", got)
	}
	if got := settings.outboxRetryDelay(2); got != 8*time.Second {
		t.Fatalf("retry 2 = %v", got)
	}
	if got := settings.outboxRetryDelay(8); got != 10*time.Second {
		t.Fatalf("retry cap = %v", got)
	}
}
