package workbenchterminal

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRevokedSessionPointerCannotWriteOrResize(t *testing.T) {
	m, p, id := sessionFixture(t)
	stale := m.sessions[id]
	if err := m.RevokeSession("first", id); err != nil {
		t.Fatal(err)
	}
	if err := stale.input([]byte("must not execute")); !errors.Is(err, ErrExited) {
		t.Fatalf("stale input accepted: %v", err)
	}
	if err := stale.resize(35, 100); !errors.Is(err, ErrExited) {
		t.Fatalf("stale resize accepted: %v", err)
	}
	if stale.snapshot(0).Running {
		t.Fatal("revoked session still reports running")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.writes) != 0 {
		t.Fatalf("revoked session wrote %q", p.writes)
	}
}

func TestShutdownAndPruneFenceOldSessionReferences(t *testing.T) {
	for _, mode := range []string{"close", "prune"} {
		t.Run(mode, func(t *testing.T) {
			m, _, id := sessionFixture(t)
			stale := m.sessions[id]
			if mode == "close" {
				m.Close()
			} else {
				stale.mu.Lock()
				stale.lastUsed = time.Now().Add(-sessionTTL - time.Second)
				stale.mu.Unlock()
				m.prune()
			}
			if err := stale.input([]byte("late input")); !errors.Is(err, ErrExited) {
				t.Fatalf("late input accepted: %v", err)
			}
			if err := stale.resize(35, 100); !errors.Is(err, ErrExited) {
				t.Fatalf("late resize accepted: %v", err)
			}
		})
	}
}

func TestSessionCloseNotifiesConnectedObservers(t *testing.T) {
	m, _, id := sessionFixture(t)
	stale := m.sessions[id]
	_, notification := stale.snapshotAndListen(0)
	if err := m.RevokeSession("first", id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-notification:
	case <-time.After(time.Second):
		t.Fatal("observer did not receive revoke notification")
	}
	snapshot, _ := stale.snapshotAndListen(0)
	if snapshot.Running {
		t.Fatal("observer received a running session after revoke")
	}
}

func TestConcurrentCloseMakesStaleWritesFail(t *testing.T) {
	m, _, id := sessionFixture(t)
	stale := m.sessions[id]
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				_ = stale.input([]byte("x"))
				_ = stale.resize(30, 100)
			}
		}()
	}
	if err := m.RevokeSession("first", id); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err := stale.input([]byte("revoked")); !errors.Is(err, ErrExited) {
		t.Fatalf("post-close input returned: %v", err)
	}
}
