package workbenchterminal

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamTicketOneTimeScopeAndOrigin(t *testing.T) {
	var r ticketRegistry
	token, err := r.issue("owner", "session", "https://desktop.example")
	if err != nil || len(token) != 48 {
		t.Fatalf("issue failed: %v", err)
	}
	if _, err := r.consume(token, "https://foreign.example"); !errors.Is(err, ErrTicket) {
		t.Fatalf("origin mismatch accepted: %v", err)
	}
	if _, err := r.consume(token, "https://desktop.example"); !errors.Is(err, ErrTicket) {
		t.Fatalf("replayed ticket accepted: %v", err)
	}
	valid, err := r.issue("owner", "session", "https://desktop.example")
	if err != nil {
		t.Fatal(err)
	}
	grant, err := r.consume(valid, "https://desktop.example")
	if err != nil || grant.owner != "owner" || grant.sessionID != "session" {
		t.Fatalf("scope damaged: %+v %v", grant, err)
	}
}

func TestStreamTicketExpiryAndBounds(t *testing.T) {
	var r ticketRegistry
	for _, origin := range []string{"", "*", "ftp://site.example", "https://site.example/path", "https://user@site.example", "http://site.example?ticket=x"} {
		if _, err := r.issue("a", "s", origin); !errors.Is(err, ErrInvalid) {
			t.Fatalf("unsafe origin %q: %v", origin, err)
		}
	}
	r.tickets = map[string]streamTicket{"expired": {owner: "a", sessionID: "s", origin: "null", expires: time.Now().Add(-time.Second)}}
	if _, err := r.consume("expired", "null"); !errors.Is(err, ErrTicket) {
		t.Fatalf("expired accepted: %v", err)
	}
	for i := 0; i < maxPendingTickets; i++ {
		if _, err := r.issue("a", "s", "null"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.issue("a", "s", "null"); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit bypassed: %v", err)
	}
	if len(r.tickets) != maxPendingTickets {
		t.Fatal("wrong ticket count")
	}
}

func TestStreamTicketConcurrentSingleConsumption(t *testing.T) {
	var r ticketRegistry
	ticket, err := r.issue("a", "s", "http://localhost:18899")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	successful := make(chan struct{}, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.consume(ticket, "http://localhost:18899"); err == nil {
				successful <- struct{}{}
			}
		}()
	}
	wg.Wait()
	if len(successful) != 1 {
		t.Fatalf("ticket used %d times", len(successful))
	}
	if _, err := r.issue("a", strings.Repeat("s", 16), "null"); err != nil {
		t.Fatal(err)
	}
}
